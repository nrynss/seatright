package service

import (
	"net/url"
	"strconv"
	"time"

	"tablekeeper/internal/clock"
)

// Availability renders the public slot grid for a restaurant, date and party
// size. Every opening-grid slot appears, including slots with no free table,
// in wall-clock order; available_table_ids stays in fixture table order. A
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
	rendered := make([]any, 0, len(slots))
	for _, slot := range slots {
		rendered = append(rendered, map[string]any{
			"starts_at_local":     slot.Local,
			"starts_at":           formatTimestamp(slot.Start),
			"available_table_ids": availableTables(&s.state, restaurant, party, slot.Start, slot.End),
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

// availableTables lists the restaurant's tables with capacity for party and
// no overlapping confirmed reservation on the interval, in fixture order.
func availableTables(st *State, restaurant *Restaurant, party int, start, end time.Time) []any {
	out := make([]any, 0, len(restaurant.Tables))
	for _, t := range restaurant.Tables {
		if t.Capacity < party {
			continue
		}
		free := true
		for _, r := range st.Reservations {
			if r.Status != StatusConfirmed {
				continue
			}
			if r.RestaurantID != restaurant.ID || r.TableID != t.ID {
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
			if clock.Overlap(start, end, s, e) {
				free = false
				break
			}
		}
		if free {
			out = append(out, t.ID)
		}
	}
	return out
}
