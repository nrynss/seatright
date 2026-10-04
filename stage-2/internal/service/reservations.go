package service

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/nrynss/keel/id"
	"tablekeeper/internal/clock"
)

// This file implements the reservation write core: creation, amendment
// preparation, overlap detection, cancellation, lookup and list. All helpers
// operate on a passed *State and never lock the Service, so a future
// idempotency wrapper can execute them against a cloned working state and
// commit atomically. HTTP entry points that lock are named
// ListReservations/GetReservation/CancelReservation/PatchReservation; the
// creation callback createReservationLocked is bound to POST /reservations by
// S1-D2 after the receipt wrapper integrates.

// timestampLayout renders RFC 3339 with an always-numeric offset: UTC reads
// +00:00, never a bare Z.
const timestampLayout = "2006-01-02T15:04:05-07:00"

// formatTimestamp renders t for API responses and stored records.
func formatTimestamp(t time.Time) string {
	return t.Format(timestampLayout)
}

// parseStoredInstant parses a timestamp previously produced by
// formatTimestamp.
func parseStoredInstant(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// rulesFor builds the clock rules for a restaurant's fixture configuration.
func rulesFor(r *Restaurant) clock.Rules {
	hours := make([]clock.Hours, 0, len(r.OpeningHours))
	for _, h := range r.OpeningHours {
		hours = append(hours, clock.Hours{Weekday: h.Weekday, Opens: h.Opens, Closes: h.Closes})
	}
	return clock.Rules{
		Timezone:        r.Timezone,
		SlotMinutes:     r.SlotMinutes,
		DurationMinutes: r.ReservationDurationMinutes,
		OpeningHours:    hours,
	}
}

// validatedBooking is the common validated outcome of create and amendment
// field checks: the restaurant, the table, the party size and the resolved
// slot. Occupancy is checked separately by the caller.
type validatedBooking struct {
	restaurant *Restaurant
	table      *Table
	party      int
	slot       clock.Slot
}

// partySize extracts party_size under the endpoint-specific rule: every
// invalid value, including strings and booleans, is 422 validation_failed.
// Only a missing field is reported separately from an invalid one, and both
// are 422.
func partySize(obj map[string]any) (int, *codedError) {
	raw, ok := obj["party_size"]
	if !ok {
		return 0, invalidErr("party_size is required")
	}
	n, isNumber, isInteger := fixtureInt(raw)
	if !isNumber || !isInteger || n < 1 {
		return 0, invalidErr("party_size must be an integer of at least 1")
	}
	return n, nil
}

// bookingTarget resolves restaurant_id and table_id: missing fields are 422,
// wrong JSON types are 400, and unknown restaurants, unknown tables or tables
// of another restaurant are 404.
func bookingTarget(st *State, obj map[string]any) (*Restaurant, *Table, *codedError) {
	restaurantID, present, wrongType := fieldString(obj, "restaurant_id")
	if wrongType {
		return nil, nil, malformedErr()
	}
	if !present {
		return nil, nil, invalidErr("restaurant_id is required")
	}
	tableID, present, wrongType := fieldString(obj, "table_id")
	if wrongType {
		return nil, nil, malformedErr()
	}
	if !present {
		return nil, nil, invalidErr("table_id is required")
	}
	restaurant := restaurantByID(st, restaurantID)
	if restaurant == nil {
		return nil, nil, &codedError{status: 404, code: "not_found", msg: "unknown restaurant"}
	}
	for i := range restaurant.Tables {
		if restaurant.Tables[i].ID == tableID {
			return restaurant, &restaurant.Tables[i], nil
		}
	}
	return nil, nil, &codedError{status: 404, code: "not_found", msg: "unknown table"}
}

// bookingSlot resolves starts_at_local: a missing field is 422, a wrong JSON
// type is 400, a non-bare-local string is 422, and clock errors map to their
// own codes (invalid_local_time, not_on_slot_grid, outside_opening_hours).
func bookingSlot(obj map[string]any, rules clock.Rules) (clock.Slot, *codedError) {
	local, present, wrongType := fieldString(obj, "starts_at_local")
	if wrongType {
		return clock.Slot{}, malformedErr()
	}
	if !present {
		return clock.Slot{}, invalidErr("starts_at_local is required")
	}
	slot, err := clock.ValidateSlot(local, rules)
	if err != nil {
		if ce, ok := err.(*clock.Error); ok {
			return clock.Slot{}, &codedError{status: 422, code: ce.Code, msg: ce.Code}
		}
		return clock.Slot{}, invalidErr("invalid starts_at_local")
	}
	return slot, nil
}

// validateBookingFields runs the identical field validation for create and
// amendment: target, time, party size and capacity. It performs no occupancy
// check and no mutation.
func validateBookingFields(st *State, obj map[string]any) (validatedBooking, *codedError) {
	var out validatedBooking
	restaurant, table, cerr := bookingTarget(st, obj)
	if cerr != nil {
		return out, cerr
	}
	slot, cerr := bookingSlot(obj, rulesFor(restaurant))
	if cerr != nil {
		return out, cerr
	}
	party, cerr := partySize(obj)
	if cerr != nil {
		return out, cerr
	}
	if party > table.Capacity {
		return out, &codedError{status: 422, code: "party_exceeds_capacity", msg: "party exceeds table capacity"}
	}
	out = validatedBooking{restaurant: restaurant, table: table, party: party, slot: slot}
	return out, nil
}

// conflictingReservation reports whether candidate overlaps a confirmed
// reservation on the same restaurant and table, ignoring the references in
// exclude (the candidate itself for amendments, or a whole batch for moves).
func conflictingReservation(st *State, candidate Reservation, exclude map[string]bool) bool {
	cStart, err := parseStoredInstant(candidate.StartsAt)
	if err != nil {
		return false
	}
	cEnd, err := parseStoredInstant(candidate.EndsAt)
	if err != nil {
		return false
	}
	for ref, r := range st.Reservations {
		if exclude[ref] {
			continue
		}
		if r.Status != StatusConfirmed {
			continue
		}
		if r.RestaurantID != candidate.RestaurantID || r.TableID != candidate.TableID {
			continue
		}
		s, err := parseStoredInstant(r.StartsAt)
		if err != nil {
			continue
		}
		e, err := parseStoredInstant(r.EndsAt)
		if err != nil {
			continue
		}
		if clock.Overlap(cStart, cEnd, s, e) {
			return true
		}
	}
	return false
}

// referenceAlphabet is the confirmation-reference alphabet: A-Z0-9.
const referenceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// newReference generates a unique 8-character confirmation reference.
func newReference(st *State) (string, *codedError) {
	for range 100 {
		raw := make([]byte, 8)
		for i := range raw {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(referenceAlphabet))))
			if err != nil {
				return "", &codedError{status: 500, code: "internal", msg: "internal error"}
			}
			raw[i] = referenceAlphabet[n.Int64()]
		}
		ref := string(raw)
		if _, taken := st.Reservations[ref]; !taken {
			return ref, nil
		}
	}
	return "", &codedError{status: 500, code: "internal", msg: "internal error"}
}

// createReservationLocked validates obj, checks occupancy and stores a new
// confirmed reservation in st. It locks nothing itself; callers hold the
// state lock (or operate on a clone for atomic wrappers).
func (s *Service) createReservationLocked(st *State, userID string, obj map[string]any) Result {
	vb, cerr := validateBookingFields(st, obj)
	if cerr != nil {
		return cerr.Result()
	}
	start := vb.slot.Start
	end := vb.slot.End
	reservationID, err := id.New()
	if err != nil {
		return internalErr()
	}
	reference, cerr := newReference(st)
	if cerr != nil {
		return cerr.Result()
	}
	candidate := Reservation{
		ReservationID: reservationID,
		Reference:     reference,
		UserID:        userID,
		RestaurantID:  vb.restaurant.ID,
		TableID:       vb.table.ID,
		PartySize:     vb.party,
		Status:        StatusConfirmed,
		StartsAtLocal: vb.slot.Local,
		StartsAt:      formatTimestamp(start),
		EndsAt:        formatTimestamp(end),
		CreatedAt:     formatTimestamp(time.Now().UTC()),
	}
	if conflictingReservation(st, candidate, nil) {
		return conflict("table_unavailable", "the table is taken for that interval")
	}
	st.Reservations[reference] = candidate
	return created(candidate.Public())
}

// prepareAmendment validates a PATCH-style change set against the current
// record: cancelled bookings and passed cutoffs are rejected before changed
// fields are examined. It returns the fully prepared record with preserved
// identity, owner and creation time, without any occupancy check or mutation.
// A no-op change set returns the current values unchanged, but only for an
// editable (confirmed, within-cutoff) booking.
func prepareAmendment(st *State, current Reservation, changes map[string]any) (Reservation, *codedError) {
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
	if clock.CutoffPassed(time.Now(), start, restaurant.CancellationCutoffMinutes) {
		return Reservation{}, &codedError{status: 409, code: "cutoff_passed", msg: "the cancellation cutoff has passed"}
	}
	merged := map[string]any{
		"restaurant_id":   current.RestaurantID,
		"table_id":        current.TableID,
		"starts_at_local": current.StartsAtLocal,
		// Stored party sizes are Go ints; validation consumes decoded JSON,
		// so convert to float64 exactly as a request body would carry it.
		"party_size": float64(current.PartySize),
	}
	for _, field := range []string{"table_id", "starts_at_local", "party_size"} {
		if raw, ok := changes[field]; ok {
			merged[field] = raw
		}
	}
	vb, cerr := validateBookingFields(st, merged)
	if cerr != nil {
		return Reservation{}, cerr
	}
	prepared := current
	prepared.TableID = vb.table.ID
	prepared.PartySize = vb.party
	prepared.StartsAtLocal = vb.slot.Local
	prepared.StartsAt = formatTimestamp(vb.slot.Start)
	prepared.EndsAt = formatTimestamp(vb.slot.End)
	return prepared, nil
}

// ownedReservation fetches a reservation by reference for an authenticated
// caller. Unknown references and other owners' bookings are indistinguishable:
// both are 404, leaking nothing.
func ownedReservation(st *State, userID, reference string) (Reservation, *codedError) {
	r, ok := st.Reservations[reference]
	if !ok || r.UserID != userID {
		return Reservation{}, &codedError{status: 404, code: "not_found", msg: "unknown reservation"}
	}
	return r, nil
}

// ListReservations returns the caller's reservations, starts_at descending,
// confirmed and cancelled alike.
func (s *Service) ListReservations(token string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.state.Tokens[token]
	if !ok {
		return unauthenticated()
	}
	list := make([]Reservation, 0)
	for _, r := range s.state.Reservations {
		if r.UserID == userID {
			list = append(list, r)
		}
	}
	sortReservationsDescending(list)
	items := make([]any, 0, len(list))
	for _, r := range list {
		items = append(items, r.Public())
	}
	return okResult(map[string]any{"reservations": items})
}

// sortReservationsDescending orders by start instant, newest first, with the
// reference as a deterministic tiebreak.
func sortReservationsDescending(list []Reservation) {
	instants := make(map[string]time.Time, len(list))
	for _, r := range list {
		if t, err := parseStoredInstant(r.StartsAt); err == nil {
			instants[r.Reference] = t
		}
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0; j-- {
			a, b := list[j-1], list[j]
			ta, oka := instants[a.Reference]
			tb, okb := instants[b.Reference]
			swap := false
			switch {
			case oka && okb:
				if tb.After(ta) || (tb.Equal(ta) && b.Reference < a.Reference) {
					swap = true
				}
			case okb && !oka:
				swap = true
			case !oka && !okb:
				if b.Reference < a.Reference {
					swap = true
				}
			}
			if !swap {
				break
			}
			list[j-1], list[j] = list[j], list[j-1]
		}
	}
}

// GetReservation returns one of the caller's reservations by reference.
func (s *Service) GetReservation(token, reference string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.state.Tokens[token]
	if !ok {
		return unauthenticated()
	}
	r, cerr := ownedReservation(&s.state, userID, reference)
	if cerr != nil {
		return cerr.Result()
	}
	return okResult(r.Public())
}

// CancelReservation cancels a booking, freeing its occupancy immediately.
// Cancelling an already-cancelled booking returns its current state.
func (s *Service) CancelReservation(token, reference string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.state.Tokens[token]
	if !ok {
		return unauthenticated()
	}
	r, cerr := ownedReservation(&s.state, userID, reference)
	if cerr != nil {
		return cerr.Result()
	}
	if r.Status == StatusCancelled {
		return okResult(r.Public())
	}
	restaurant := restaurantByID(&s.state, r.RestaurantID)
	if restaurant == nil {
		return notFound("unknown restaurant")
	}
	start, err := parseStoredInstant(r.StartsAt)
	if err != nil {
		return internalErr()
	}
	if clock.CutoffPassed(time.Now(), start, restaurant.CancellationCutoffMinutes) {
		return conflict("cutoff_passed", "the cancellation cutoff has passed")
	}
	r.Status = StatusCancelled
	s.state.Reservations[reference] = r
	return okResult(r.Public())
}

// PatchReservation amends time, table or party size atomically: prepare,
// final occupancy validation and commit happen under one lock. Failures leave
// the original record and its occupancy unchanged.
func (s *Service) PatchReservation(token, reference string, raw []byte) Result {
	obj, _, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.state.Tokens[token]
	if !ok {
		return unauthenticated()
	}
	current, cerr := ownedReservation(&s.state, userID, reference)
	if cerr != nil {
		return cerr.Result()
	}
	prepared, cerr := prepareAmendment(&s.state, current, obj)
	if cerr != nil {
		return cerr.Result()
	}
	if conflictingReservation(&s.state, prepared, map[string]bool{reference: true}) {
		return conflict("table_unavailable", "the table is taken for that interval")
	}
	s.state.Reservations[reference] = prepared
	return okResult(prepared.Public())
}
