package service

import (
	"net/url"
	"strconv"
	"time"

	"tablekeeper/internal/clock"
	"tablekeeper/internal/policy"
)

// Availability renders the public slot grid for a restaurant, date and party
// size. Terms come from the queried local date's selected policy (fixture
// policy0 before any publication): the grid and duration follow
// policy.Rules, and single/pair capacities follow policy.Capacity, while
// table geometry, fixture order and declared pairs stay original. Every
// opening-grid slot appears, including slots with no free table, in
// wall-clock order. available_table_ids stays singles-only in fixture
// table order; available_options lists every eligible singleton in fixture
// order followed by every eligible declared pair in combinable order. A
// closed day returns an empty slots array.
//
// explain is optional and accepts only the value "true". Without it the
// response keeps the ordinary stage-2 shape with no explanation fields.
// With it every slot carries an explain array: every fixture-order table
// exactly once with its selected policy_version, available as the
// conjunction of the independently evaluated capacity and no_overlap rules,
// and the rules array (capacity then no_overlap) reporting both outcomes.
func (s *Service) Availability(q url.Values) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	restaurantID := q.Get("restaurant_id")
	date := q.Get("date")
	partyRaw := q.Get("party_size")
	if restaurantID == "" || date == "" || partyRaw == "" {
		return validationFailed("restaurant_id, date and party_size are required")
	}
	party, ok := parseDecimalParam(partyRaw)
	if !ok || party < 1 {
		return validationFailed("party_size must be a positive integer")
	}
	if _, err := clock.ParseDate(date); err != nil {
		return validationFailed("date must be a local YYYY-MM-DD calendar date")
	}
	restaurant := restaurantByID(&s.state, restaurantID)
	if restaurant == nil {
		return notFound("unknown restaurant")
	}
	explain := false
	if _, present := q["explain"]; present {
		if q.Get("explain") != "true" {
			return validationFailed("explain accepts only true")
		}
		explain = true
	}
	terms, err := selectedTerms(&s.state, restaurant, date)
	if err != nil {
		return validationFailed("invalid availability date")
	}
	slots, err := clock.Slots(date, policy.Rules(terms, restaurant.Timezone))
	if err != nil {
		if ce, ok := err.(*clock.Error); ok {
			return Result{Status: 422, Body: errorBody(ce.Code, ce.Code)}
		}
		return validationFailed("invalid availability request")
	}
	confirmed := confirmedOccupancy(&s.state, restaurantID)
	rendered := make([]any, 0, len(slots))
	for _, slot := range slots {
		options := eligibleOptions(restaurant, terms, confirmed, party, slot.Start, slot.End)
		entry := map[string]any{
			"starts_at_local":     slot.Local,
			"starts_at":           formatTimestamp(slot.Start),
			"available_table_ids": singlesOf(options, restaurant),
			"available_options":   renderOptions(options),
		}
		if explain {
			entry["explain"] = explainSlot(restaurant, terms, confirmed, party, slot.Start, slot.End)
		}
		rendered = append(rendered, entry)
	}
	return okResult(map[string]any{
		"restaurant_id": restaurant.ID,
		"date":          date,
		"timezone":      restaurant.Timezone,
		"slots":         rendered,
	})
}

// parseDecimalParam accepts only plain decimal digits: 1e9, 4.0 and +4 are
// rejected whatever their numeric value.
func parseDecimalParam(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// occupancy is one confirmed booking's table set and absolute interval.
type occupancy struct {
	tables []string
	start  time.Time
	end    time.Time
}

// confirmedOccupancy collects every confirmed booking of a restaurant with
// parsable bounds. Pairs contribute all of their members.
func confirmedOccupancy(st *State, restaurantID string) []occupancy {
	var out []occupancy
	for _, r := range st.Reservations {
		if r.Status != StatusConfirmed || r.RestaurantID != restaurantID {
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
		out = append(out, occupancy{tables: reservationTableIDs(r), start: s, end: e})
	}
	return out
}

// eligibleOptions returns the seating options with sufficient selected-policy
// capacity and no member overlapping [start, end): fixture singles first,
// then declared pairs in combinable order (the order seatingOptions
// produces). Pair capacity is the sum of the selected policy's capacities.
func eligibleOptions(restaurant *Restaurant, terms policy.Terms, confirmed []occupancy, party int, start, end time.Time) []SeatingOption {
	var out []SeatingOption
	for _, opt := range seatingOptions(restaurant) {
		capacity := policy.Capacity(terms, opt.TableIDs)
		if capacity < party {
			continue
		}
		blocked := false
		for _, occ := range confirmed {
			if !tableSetsIntersect(opt.TableIDs, occ.tables) {
				continue
			}
			if clock.Overlap(start, end, occ.start, occ.end) {
				blocked = true
				break
			}
		}
		if !blocked {
			out = append(out, SeatingOption{TableIDs: opt.TableIDs, Capacity: capacity})
		}
	}
	return out
}

// explainSlot reports every fixture-order table exactly once for a slot: the
// selected policy_version, available as capacity && no_overlap, and both
// rules evaluated independently in capacity, no_overlap order.
func explainSlot(restaurant *Restaurant, terms policy.Terms, confirmed []occupancy, party int, start, end time.Time) []any {
	out := make([]any, 0, len(restaurant.Tables))
	for _, t := range restaurant.Tables {
		capHolds := party <= policy.Capacity(terms, []string{t.ID})
		overlap := false
		for _, occ := range confirmed {
			if !tableSetsIntersect([]string{t.ID}, occ.tables) {
				continue
			}
			if clock.Overlap(start, end, occ.start, occ.end) {
				overlap = true
				break
			}
		}
		available := capHolds && !overlap
		out = append(out, map[string]any{
			"table_id":       t.ID,
			"policy_version": terms.PolicyVersion,
			"available":      available,
			"rules": []any{
				map[string]any{"rule": "capacity", "holds": capHolds},
				map[string]any{"rule": "no_overlap", "holds": !overlap},
			},
		})
	}
	return out
}

// singlesOf renders available_table_ids: the eligible singleton ids in
// fixture order.
func singlesOf(options []SeatingOption, restaurant *Restaurant) []any {
	eligible := map[string]bool{}
	for _, opt := range options {
		if len(opt.TableIDs) == 1 {
			eligible[opt.TableIDs[0]] = true
		}
	}
	out := make([]any, 0, len(restaurant.Tables))
	for _, t := range restaurant.Tables {
		if eligible[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}

// renderOptions renders available_options entries.
func renderOptions(options []SeatingOption) []any {
	out := make([]any, 0, len(options))
	for _, opt := range options {
		out = append(out, map[string]any{
			"table_ids": append([]string(nil), opt.TableIDs...),
			"capacity":  opt.Capacity,
		})
	}
	return out
}
