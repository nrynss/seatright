package service

import (
	"math"
	"time"

	"github.com/nrynss/keel/id"
	"tablekeeper/internal/clock"
	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// This file owns the stage-3 ordinary-write version seams: policy-selected
// creation preparation/commit, expected-revision checks, amendment
// preparation with no-op detection, and atomic amendment commit with ordered
// history. All helpers are pure with respect to locking: callers supply
// locked or isolated working state. No helper touches restaurant/series
// counters or idempotency receipts; ordinary single-write callers increment
// the restaurant counter once per real change, and later collective moves
// will do the same after final occupancy over all candidates.

// prepareReservation validates the resulting date's policy, grid, capacity
// and party, then allocates identity, reference, creation timestamp and
// selected revision-1 terms. It inserts no state, history, counter or
// receipt, and it does not check occupancy; callers perform occupancy with
// conflictingReservation.
func prepareReservation(st *State, userID string, obj map[string]any) (Reservation, *codedError) {
	vb, cerr := validateBookingFields(st, obj)
	if cerr != nil {
		return Reservation{}, cerr
	}
	reservationID, err := id.New()
	if err != nil {
		return Reservation{}, &codedError{status: 500, code: "internal", msg: "internal error"}
	}
	reference, cerr := newReference(st)
	if cerr != nil {
		return Reservation{}, cerr
	}
	candidate := Reservation{
		ReservationID: reservationID,
		Reference:     reference,
		UserID:        userID,
		RestaurantID:  vb.restaurant.ID,
		PartySize:     vb.party,
		Status:        StatusConfirmed,
		StartsAtLocal: vb.slot.Local,
		StartsAt:      formatTimestamp(vb.slot.Start),
		EndsAt:        formatTimestamp(vb.slot.End),
		CreatedAt:     formatTimestamp(time.Now().UTC()),
		Revision:      1,
		AcceptedTerms: policy.CloneTerms(vb.terms),
	}
	setReservationTables(&candidate, vb.tableIDs)
	return candidate, nil
}

// commitReservationCreation stores a prepared record with exactly one created
// history entry at its creation instant. No restaurant/series counter or
// receipt is claimed here; the ordinary create caller increments the
// restaurant counter once, and later series adoption will increment once for
// the whole adoption.
func commitReservationCreation(st *State, candidate Reservation) {
	if st.Histories == nil {
		st.Histories = map[string][]history.Entry{}
	}
	snap := reservationSnapshot(candidate)
	st.Histories[candidate.Reference] = append(st.Histories[candidate.Reference], history.Created(snap, candidate.CreatedAt))
	st.Reservations[candidate.Reference] = candidate
}

// checkExpectedRevision applies the optional optimistic-concurrency guard:
// an absent field keeps prior semantics, any non-positive-integer JSON value
// (including booleans, strings, null and fractions) is 422, and a mismatch
// with the current revision is 409 stale_revision. Comparison stays in
// float64 against the small integer revision, so arbitrarily large integral
// values compare safely without any invented ceiling or lossy int cast.
func checkExpectedRevision(current Reservation, changes map[string]any) *codedError {
	raw, ok := changes["expected_revision"]
	if !ok {
		return nil
	}
	f, ok := raw.(float64)
	if !ok || f < 1 || f != math.Trunc(f) {
		return invalidErr("expected_revision must be a positive integer")
	}
	if f != float64(current.Revision) {
		return &codedError{status: 409, code: "stale_revision", msg: "the reservation has changed"}
	}
	return nil
}

// tableSetEqual reports whether two table sets hold the same members, so a
// merely reversed equivalent pair counts as no amendment on its own.
func tableSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, id := range a {
		counts[id]++
	}
	for _, id := range b {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

// prepareAmendment validates a PATCH-style change set with retained
// signature for ordinary writes and collective moves: expected revision,
// confirmed/editable status under the OLD accepted cutoff against the
// current start, provided-field parsing with canonical sets, and no-op
// detection. It returns a detached candidate with unchanged revision and no
// history/state/counter effects. A no-op (empty, unknown-only, equal fields
// or reversed pair) preserves every stored field exactly and never
// revalidates unchanged stored values under newly published rules. A real
// change validates ALL merged fields against the resulting date's policy and
// replaces selected terms and end time.
func prepareAmendment(st *State, current Reservation, changes map[string]any) (Reservation, *codedError) {
	if cerr := checkExpectedRevision(current, changes); cerr != nil {
		return Reservation{}, cerr
	}
	if current.Status == StatusCancelled {
		return Reservation{}, &codedError{status: 409, code: "reservation_cancelled", msg: "the reservation is cancelled"}
	}
	restaurant := restaurantByID(st, current.RestaurantID)
	if restaurant == nil {
		return Reservation{}, &codedError{status: 404, code: "not_found", msg: "unknown restaurant"}
	}
	start, err := parseStoredInstant(current.StartsAt)
	if err != nil {
		return Reservation{}, &codedError{status: 500, code: "internal", msg: "internal error"}
	}
	if clock.CutoffPassed(time.Now(), start, current.AcceptedTerms.CancellationCutoffMinutes) {
		return Reservation{}, &codedError{status: 409, code: "cutoff_passed", msg: "the cancellation cutoff has passed"}
	}
	// Resolve the effective selection/time/party, validating every provided
	// field: invalid provided input fails even when nothing else changes.
	currentSet := reservationTableIDs(current)
	var newSet []string
	if _, ok := changes["table_id"]; ok {
		ids, cerr := parseTableSelection(restaurant, changes, nil)
		if cerr != nil {
			return Reservation{}, cerr
		}
		newSet = ids
	} else if _, ok := changes["table_ids"]; ok {
		ids, cerr := parseTableSelection(restaurant, changes, nil)
		if cerr != nil {
			return Reservation{}, cerr
		}
		newSet = ids
	} else {
		newSet = append([]string(nil), currentSet...)
	}
	newLocal := current.StartsAtLocal
	if raw, ok := changes["starts_at_local"]; ok {
		local, ok := raw.(string)
		if !ok {
			return Reservation{}, malformedErr()
		}
		newLocal = local
	}
	newParty := current.PartySize
	if raw, ok := changes["party_size"]; ok {
		n, isNumber, isInteger := fixtureInt(raw)
		if !isNumber || !isInteger || n < 1 {
			return Reservation{}, invalidErr("party_size must be an integer of at least 1")
		}
		newParty = n
	}
	if tableSetEqual(newSet, currentSet) && newLocal == current.StartsAtLocal && newParty == current.PartySize {
		detached := current
		detached.TableIDs = append([]string(nil), current.TableIDs...)
		detached.AcceptedTerms = policy.CloneTerms(current.AcceptedTerms)
		return detached, nil
	}
	merged := map[string]any{
		"restaurant_id":   current.RestaurantID,
		"starts_at_local": newLocal,
		"party_size":      float64(newParty),
	}
	for field, raw := range amendmentTables(currentSet, changes) {
		merged[field] = raw
	}
	vb, cerr := validateBookingFields(st, merged)
	if cerr != nil {
		return Reservation{}, cerr
	}
	candidate := current
	setReservationTables(&candidate, vb.tableIDs)
	candidate.PartySize = vb.party
	candidate.StartsAtLocal = vb.slot.Local
	candidate.StartsAt = formatTimestamp(vb.slot.Start)
	candidate.EndsAt = formatTimestamp(vb.slot.End)
	candidate.AcceptedTerms = policy.CloneTerms(vb.terms)
	return candidate, nil
}

// commitReservationAmendment commits only a real mutation: revision+1 with
// one ordered changed-only history entry at a nondecreasing instant, and the
// final record with a changed flag. Counters stay with the caller. A pure
// no-op returns the before record unchanged with no state effects. Call only
// after occupancy passes, so a failed write can never leak metadata.
func commitReservationAmendment(st *State, before, candidate Reservation, now time.Time) (Reservation, bool) {
	if tableSetEqual(reservationTableIDs(before), reservationTableIDs(candidate)) &&
		before.StartsAtLocal == candidate.StartsAtLocal &&
		before.PartySize == candidate.PartySize {
		return before, false
	}
	final := candidate
	final.Revision = before.Revision + 1
	beforeSnap := reservationSnapshot(before)
	afterSnap := reservationSnapshot(final)
	if st.Histories == nil {
		st.Histories = map[string][]history.Entry{}
	}
	seq, at := history.Next(st.Histories[before.Reference], now)
	if entry, ok := history.Changed(beforeSnap, afterSnap, seq, at); ok {
		st.Histories[before.Reference] = append(st.Histories[before.Reference], entry)
	}
	st.Reservations[before.Reference] = final
	return final, true
}
