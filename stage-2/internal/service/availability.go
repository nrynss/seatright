package service

import (
	"net/url"
	"strconv"
	"time"

	"tablekeeper/internal/clock"
)

// Availability renders the public slot grid for a restaurant, date and party
// size. Every opening-grid slot appears, including slots with no free table,
// in wall-clock order. available_table_ids stays singles-only in fixture
// table order; available_options lists every eligible singleton in fixture
// order followed by every eligible declared pair in combinable order. A
// closed day returns an empty slots array.
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
	slots, err := clock.Slots(date, rulesFor(restaurant))
	if err != nil {
		if ce, ok := err.(*clock.Error); ok {
			return Result{Status: 422, Body: errorBody(ce.Code, ce.Code)}
		}
		return validationFailed("invalid availability request")
	}
	confirmed := confirmedOccupancy(&s.state, restaurantID)
	rendered := make([]any, 0, len(slots))
	for _, slot := range slots {
		options := eligibleOptions(restaurant, confirmed, party, slot.Start, slot.End)
		rendered = append(rendered, map[string]any{
			"starts_at_local":     slot.Local,
			"starts_at":           formatTimestamp(slot.Start),
			"available_table_ids": singlesOf(options, restaurant),
			"available_options":   renderOptions(options),
		})
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

// eligibleOptions returns the seating options with sufficient capacity and no
// member overlapping [start, end): fixture singles first, then declared pairs
// in combinable order (the order seatingOptions produces).
func eligibleOptions(restaurant *Restaurant, confirmed []occupancy, party int, start, end time.Time) []SeatingOption {
	var out []SeatingOption
	for _, opt := range seatingOptions(restaurant) {
		if opt.Capacity < party {
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
			out = append(out, opt)
		}
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
