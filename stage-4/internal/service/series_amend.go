package service

// Atomic recurring clock amendments: POST /series/{id}/amend changes the
// clock time of eligible occurrences on their original scheduled dates.
// AmendSeries is the idempotent-write wrapper; amendSeriesLocked is the pure
// callback (no locks/reentry) running against the Idempotent cloned working
// state, committed atomically only on 201. Router wiring is a later serial
// owner's scope; service-direct calls are acceptance here.

import (
	"math"
	"time"
)

// AmendSeries binds POST /series/{seriesID}/amend to the amendment callback.
func (s *Service) AmendSeries(token, seriesID, key string, raw []byte) Result {
	return s.Idempotent(token, "POST", "/series/"+seriesID+"/amend", key, raw,
		func(st *State, userID string, obj map[string]any) Result {
			return amendSeriesLocked(st, userID, seriesID, obj)
		})
}

// amendExpectedRevision reads the required expected_revision: a positive
// integral JSON number. Booleans, strings, null, fractions and nonpositive
// values are 422. Arbitrarily large integral values stay valid (compared in
// float64 against the series revision, never cast to int).
func amendExpectedRevision(obj map[string]any) (float64, *codedError) {
	raw, ok := obj["expected_revision"]
	if !ok {
		return 0, invalidErr("expected_revision is required")
	}
	f, ok := raw.(float64)
	if !ok || f < 1 || f != math.Trunc(f) {
		return 0, invalidErr("expected_revision must be a positive integer")
	}
	return f, nil
}

// amendFromIndex reads the required from_index: a strict integer in
// 0..count-1. Booleans and other non-integral values are 422.
func amendFromIndex(obj map[string]any, count int) (int, *codedError) {
	raw, ok := obj["from_index"]
	if !ok {
		return 0, invalidErr("from_index is required")
	}
	f, ok := raw.(float64)
	if !ok || f != math.Trunc(f) || int(f) < 0 || int(f) > count-1 {
		return 0, invalidErr("from_index is invalid")
	}
	return int(f), nil
}

// amendLocalTime reads the required local_time: exactly 5 bytes HH:MM with a
// valid 00:00..23:59 clock. Anything else is 422.
func amendLocalTime(obj map[string]any) (string, *codedError) {
	raw, ok := obj["local_time"]
	if !ok {
		return "", invalidErr("local_time is required")
	}
	clock, ok := raw.(string)
	if !ok || len(clock) != 5 || clock[2] != ':' {
		return "", invalidErr("local_time must be HH:MM")
	}
	for i := 0; i < 5; i++ {
		if i == 2 {
			continue
		}
		if clock[i] < '0' || clock[i] > '9' {
			return "", invalidErr("local_time must be HH:MM")
		}
	}
	hh := int(clock[0]-'0')*10 + int(clock[1]-'0')
	mm := int(clock[3]-'0')*10 + int(clock[4]-'0')
	if hh > 23 || mm > 59 {
		return "", invalidErr("local_time must be 00:00..23:59")
	}
	return clock, nil
}

// amendSeriesLocked executes one atomic clock amendment on the working state.
func amendSeriesLocked(st *State, userID, seriesID string, obj map[string]any) Result {
	wantRev, cerr := amendExpectedRevision(obj)
	if cerr != nil {
		return cerr.Result()
	}
	sz, ok := st.Series[seriesID]
	if !ok || sz.UserID != userID {
		return notFound("unknown series")
	}
	fromIndex, cerr := amendFromIndex(obj, len(sz.Members))
	if cerr != nil {
		return cerr.Result()
	}
	localTime, cerr := amendLocalTime(obj)
	if cerr != nil {
		return cerr.Result()
	}
	if wantRev != float64(sz.Revision) {
		cerr := &codedError{status: 409, code: "stale_revision", msg: "the series has changed"}
		return cerr.Result()
	}
	// Eligible members in original occurrence-index order: index >=
	// from_index, current record confirmed, not a permanent exception.
	// Cancelled and exception members are ignored entirely.
	type target struct {
		member SeriesMember
		before Reservation
	}
	var eligible []target
	for _, m := range sz.Members {
		if m.Index < fromIndex {
			continue
		}
		rec, ok := st.Reservations[m.Reference]
		if !ok || rec.Status != StatusConfirmed {
			continue
		}
		if m.Exception {
			continue
		}
		eligible = append(eligible, target{member: m, before: rec})
	}
	// Prepare ALL eligible indices first: no-op detection before the shared
	// helper (which would check cutoff first), real changes through the
	// actual prepareAmendment (old accepted cutoff, then resulting-date
	// policy). First non-occupancy error in occurrence-index order wins,
	// even ahead of an earlier candidate's occupancy conflict.
	type prepared struct {
		t         target
		candidate Reservation
		real      bool
	}
	var ready []prepared
	for _, t := range eligible {
		resultLocal := t.member.ScheduledDate + "T" + localTime
		if resultLocal == t.before.StartsAtLocal {
			// Identical operation: retain every stored field, terms, end,
			// revision and history; bypass grid/capacity and cutoff.
			ready = append(ready, prepared{t: t, candidate: t.before, real: false})
			continue
		}
		cand, cerr := prepareAmendment(st, t.before, map[string]any{"starts_at_local": resultLocal})
		if cerr != nil {
			return cerr.Result()
		}
		ready = append(ready, prepared{t: t, candidate: cand, real: true})
	}
	// Final occupancy over detached WORK candidates: publish all, then check
	// each real candidate against everything else (excluding only itself),
	// plus applied closures. Unchanged, cancelled and excluded members keep
	// their stored occupancy as usual.
	for _, r := range ready {
		st.Reservations[r.candidate.Reference] = r.candidate
	}
	for _, r := range ready {
		if !r.real {
			continue
		}
		if conflictingReservation(st, r.candidate, map[string]bool{r.candidate.Reference: true}) {
			return conflict("table_unavailable", "resulting bookings overlap")
		}
		start, err := parseStoredInstant(r.candidate.StartsAt)
		if err != nil {
			return internalErr()
		}
		end, err := parseStoredInstant(r.candidate.EndsAt)
		if err != nil {
			return internalErr()
		}
		if closureBlocks(st, r.candidate.RestaurantID, reservationTableIDs(r.candidate), start, end) {
			return conflict("table_unavailable", "resulting bookings overlap")
		}
	}
	// Commit real changes against ORIGINAL befores: one ordinary ordered
	// Changed entry and rev+1 each; no-ops untouched.
	now := time.Now()
	var changedRefs []string
	for _, r := range ready {
		if !r.real {
			continue
		}
		if _, changed := commitReservationAmendment(st, r.t.before, r.candidate, now); changed {
			changedRefs = append(changedRefs, r.t.before.Reference)
		}
	}
	if len(changedRefs) > 0 {
		touchSeriesForChanges(st, changedRefs, false)
		if st.RestaurantRevisions == nil {
			st.RestaurantRevisions = map[string]int{}
		}
		st.RestaurantRevisions[sz.RestaurantID]++
	}
	out, cerr := seriesPublic(st, st.Series[seriesID])
	if cerr != nil {
		return internalErr()
	}
	return created(out)
}
