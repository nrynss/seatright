package service

import (
	"time"

	"github.com/nrynss/keel/id"
	"tablekeeper/internal/clock"
)

// This file owns series adoption and current owner lookup: POST /series
// adopts an existing reservation as occurrence zero of a recurring agreement,
// and GET /series/{id} renders the current owner shape. Generation reuses
// the integrated prepareReservation/commitReservationCreation seams without
// per-member counters; the whole adoption increments the restaurant revision
// exactly once. Individual PATCH/cancel propagation is queued S3-C; the
// membership/touch seam below is built and unit-tested here but ordinary
// caller integration remains C.

// AdoptSeries binds POST /series to the integrated adoption callback.
func (s *Service) AdoptSeries(token, key string, raw []byte) Result {
	return s.Idempotent(token, "POST", "/series", key, raw, s.adoptSeriesLocked)
}

// seriesMembership scans stored members and returns the stable series
// identity and occurrence index for a booking reference. Produced state has
// unique membership.
func seriesMembership(st *State, reference string) (seriesID string, index int, found bool) {
	for id, sz := range st.Series {
		for _, m := range sz.Members {
			if m.Reference == reference {
				return id, m.Index, true
			}
		}
	}
	return "", 0, false
}

// touchSeriesForChanges advances series bookkeeping for successful real
// changes only. Each affected series revision increments once, even with
// repeated refs or multiple members in one batch. With markException=true
// the listed members are permanently flagged; otherwise every existing flag
// is retained. No restaurant counters, booking/history changes, receipts or
// action on unrelated refs.
func touchSeriesForChanges(st *State, references []string, markException bool) {
	seen := map[string]bool{}
	for _, ref := range references {
		sid, idx, ok := seriesMembership(st, ref)
		if !ok || seen[sid] {
			continue
		}
		seen[sid] = true
		sz := st.Series[sid]
		sz.Revision++
		if markException {
			for i := range sz.Members {
				for _, r := range references {
					if sz.Members[i].Reference == r {
						sz.Members[i].Exception = true
					}
				}
			}
		}
		_ = idx
		st.Series[sid] = sz
	}
}

// seriesInt reads a required strict integer field: decoded float64 with
// integral value in [lo,hi]. Booleans, strings, null and fractions fail.
func seriesInt(obj map[string]any, field string, lo, hi int) (int, *codedError) {
	raw, ok := obj[field]
	if !ok {
		return 0, invalidErr(field + " is required")
	}
	f, ok := raw.(float64)
	if !ok || f != float64(int(f)) || int(f) < lo || int(f) > hi {
		return 0, invalidErr(field + " is invalid")
	}
	return int(f), nil
}

// adoptSeriesLocked validates the anchor and generates occurrences. It locks
// nothing and calls no Service locked methods; the Idempotent wrapper runs it
// against a cloned working state and commits atomically on 201.
func (s *Service) adoptSeriesLocked(st *State, userID string, obj map[string]any) Result {
	anchorRef, present, wrongType := fieldString(obj, "anchor_reference")
	if wrongType {
		return malformed()
	}
	if !present || anchorRef == "" {
		return invalidErr("anchor_reference is required").Result()
	}
	count, cerr := seriesInt(obj, "count", 2, 12)
	if cerr != nil {
		return cerr.Result()
	}
	interval, cerr := seriesInt(obj, "interval_weeks", 1, 4)
	if cerr != nil {
		return cerr.Result()
	}
	anchor, ok := st.Reservations[anchorRef]
	if !ok || anchor.UserID != userID {
		return notFound("unknown reservation")
	}
	if anchor.Status == StatusCancelled {
		return conflict("reservation_cancelled", "the reservation is cancelled")
	}
	if _, _, found := seriesMembership(st, anchorRef); found {
		return conflict("already_in_series", "the reservation is already in a series")
	}
	restaurant := restaurantByID(st, anchor.RestaurantID)
	if restaurant == nil {
		return notFound("unknown restaurant")
	}
	start, err := parseStoredInstant(anchor.StartsAt)
	if err != nil {
		return internalErr()
	}
	if clock.CutoffPassed(time.Now(), start, anchor.AcceptedTerms.CancellationCutoffMinutes) {
		return conflict("cutoff_passed", "the cancellation cutoff has passed")
	}
	// Calendar: anchor local date plus i*interval*7 days, same clock text.
	y, mo, d, hh, mm, ok := splitSeriesLocal(anchor.StartsAtLocal)
	if !ok {
		return invalidErr("invalid anchor time").Result()
	}
	anchorSet := reservationTableIDs(anchor)
	members := make([]SeriesMember, 0, count)
	members = append(members, SeriesMember{Index: 0, Reference: anchorRef, ScheduledDate: anchor.StartsAtLocal[:10], Exception: false})
	// Timezone-independent calendar arithmetic: date-only math in UTC never
	// normalizes nonexistent zone midnights; per-occurrence clock resolution
	// stays with prepareReservation/clock. No fixed 168-hour instants.
	anchorDay := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	for i := 1; i < count; i++ {
		day := anchorDay.AddDate(0, 0, i*interval*7)
		local := day.Format("2006-01-02") + "T" + two(hh) + ":" + two(mm)
		genObj := map[string]any{
			"restaurant_id":   anchor.RestaurantID,
			"starts_at_local": local,
			"party_size":      float64(anchor.PartySize),
		}
		if len(anchorSet) == 1 {
			genObj["table_id"] = anchorSet[0]
		} else {
			arr := make([]any, 0, len(anchorSet))
			for _, t := range anchorSet {
				arr = append(arr, t)
			}
			genObj["table_ids"] = arr
		}
		cand, cerr := prepareReservation(st, userID, genObj)
		if cerr != nil {
			return cerr.Result()
		}
		// Occupancy on the transaction working state with NO exclusions:
		// the anchor and every previously committed generated record are
		// real confirmed occupants. Each success commits immediately, so a
		// later index conflicts with earlier provisional members and the
		// wrapper discards everything on any failure.
		if conflictingReservation(st, cand, nil) {
			return conflict("table_unavailable", "the table is taken for that interval")
		}
		commitReservationCreation(st, cand)
		members = append(members, SeriesMember{Index: i, Reference: cand.Reference, ScheduledDate: local[:10], Exception: false})
	}
	// All generated: store the series, then one restaurant counter increment.
	seriesID, err := id.New()
	if err != nil {
		return internalErr()
	}
	st.Series[seriesID] = Series{
		ID: seriesID, UserID: userID, RestaurantID: anchor.RestaurantID,
		Revision: 1, IntervalWeeks: interval, Members: members,
	}
	st.RestaurantRevisions[anchor.RestaurantID]++
	return created(seriesPublicMust(st, st.Series[seriesID]))
}

// seriesPublic renders the current owner series shape as detached map JSON:
// occurrences in index order, each with index/reference/exception and the
// ordinary current Public reservation. Struct encoding is avoided so 201 and
// replay-200 marshal identically through the receipt map path.
func seriesPublic(st *State, series Series) (map[string]any, *codedError) {
	occ := make([]any, 0, len(series.Members))
	for _, m := range series.Members {
		rec, ok := st.Reservations[m.Reference]
		if !ok {
			return nil, &codedError{status: 500, code: "internal", msg: "internal error"}
		}
		occ = append(occ, map[string]any{
			"index":       m.Index,
			"reference":   m.Reference,
			"exception":   m.Exception,
			"reservation": rec.Public(),
		})
	}
	return map[string]any{
		"series_id":      series.ID,
		"revision":       series.Revision,
		"interval_weeks": series.IntervalWeeks,
		"occurrences":    occ,
	}, nil
}

func seriesPublicMust(st *State, series Series) map[string]any {
	out, cerr := seriesPublic(st, series)
	if cerr != nil {
		return map[string]any{}
	}
	return out
}

// GetSeries returns the current owner series shape, or 404 for unknown,
// foreign or unauthenticated callers (missing/invalid token included).
func (s *Service) GetSeries(token, seriesID string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.state.Tokens[token]
	if !ok || token == "" {
		return notFound("unknown series")
	}
	sz, ok := s.state.Series[seriesID]
	if !ok || sz.UserID != uid {
		return notFound("unknown series")
	}
	out, cerr := seriesPublic(&s.state, sz)
	if cerr != nil {
		return internalErr()
	}
	return okResult(out)
}

// splitSeriesLocal parses a strict bare YYYY-MM-DDTHH:MM wall time.
func splitSeriesLocal(local string) (y, mo, d, hh, mm int, ok bool) {
	if len(local) != 16 || local[10] != 'T' || local[13] != ':' {
		return 0, 0, 0, 0, 0, false
	}
	date := local[:10]
	y, mo, d, ok = splitSeriesDate(date)
	if !ok {
		return 0, 0, 0, 0, 0, false
	}
	hh = int(local[11]-'0')*10 + int(local[12]-'0')
	mm = int(local[14]-'0')*10 + int(local[15]-'0')
	for _, c := range []byte{local[11], local[12], local[14], local[15]} {
		if c < '0' || c > '9' {
			return 0, 0, 0, 0, 0, false
		}
	}
	if hh > 23 || mm > 59 {
		return 0, 0, 0, 0, 0, false
	}
	return y, mo, d, hh, mm, true
}

func splitSeriesDate(date string) (y, m, d int, ok bool) {
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		return 0, 0, 0, false
	}
	for i, c := range date {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return 0, 0, 0, false
		}
		switch {
		case i < 4:
			y = y*10 + int(c-'0')
		case i < 7:
			m = m*10 + int(c-'0')
		default:
			d = d*10 + int(c-'0')
		}
	}
	if m < 1 || m > 12 || d < 1 || d > seriesDaysIn(y, m) {
		return 0, 0, 0, false
	}
	return y, m, d, true
}

func seriesDaysIn(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// two renders a clock component with a leading zero.
func two(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
