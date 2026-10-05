package service

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/nrynss/keel/id"

	"tablekeeper/internal/planner"
)

// This file owns the stage-4 seating-repair preview surface: manager-gated
// POST /restaurants/{id}/replans with real idempotent receipts. Preview
// stores only a plan (plus its normal receipt); closures, occupancy,
// bookings, histories, terms, series and restaurant counters are untouched.
// Application and closure write enforcement belong to a later item.

// PreviewReplan previews a seating repair for a table closure. It runs
// through the real Idempotent engine on the actual
// POST /restaurants/{id}/replans path, so canonical replay-before-
// validation, 200 original-response replays, user/path scoping and single-
// commit concurrency hold exactly as for every other idempotent write.
func (s *Service) PreviewReplan(token, restaurantID, key string, raw []byte) Result {
	path := "/restaurants/" + restaurantID + "/replans"
	return s.Idempotent(token, http.MethodPost, path, key, raw, func(st *State, uid string, obj map[string]any) Result {
		return previewReplanLocked(st, uid, restaurantID, obj)
	})
}

// strictInstant matches RFC3339 instants with an explicit offset: exactly
// four-digit date, two-digit hour/minute/second, literal T and separators,
// optional DOT-digit fractional seconds only, and Z or a signed two-digit
// hour:minute offset. Go's time.Parse accepts out-of-range offsets and lax
// spellings here, so this gate runs first.
var strictInstant = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)

// parseStrictInstant validates an RFC3339 instant with an explicit offset
// and returns its absolute time. Bounds-checked fields that time.Parse
// would otherwise forgive (offset hour 24, minute 60, single-digit clock
// parts, comma fractions) are rejected before parsing.
func parseStrictInstant(s string) (time.Time, bool) {
	m := strictInstant.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	atoi := func(x string) int {
		n, _ := strconv.Atoi(x)
		return n
	}
	month, day, hour, minute, second := atoi(m[2]), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6])
	if month < 1 || month > 12 || day < 1 || day > 31 ||
		hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, false
	}
	zone := m[8]
	if zone != "Z" {
		zh, zm := atoi(zone[1:3]), atoi(zone[4:6])
		if zh > 23 || zm > 59 {
			return time.Time{}, false
		}
	}
	// Real calendar bounds (leap days, month lengths), then absolute time.
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	y, mo, d := parsed.Date()
	if y != atoi(m[1]) || int(mo) != month || d != day {
		return time.Time{}, false
	}
	return parsed, true
}

func previewReplanLocked(st *State, userID, restaurantID string, obj map[string]any) Result {
	restaurant := restaurantByID(st, restaurantID)
	if restaurant == nil {
		return notFound("unknown restaurant")
	}
	manager := false
	for _, mid := range restaurant.ManagerUserIDs {
		if mid == userID {
			manager = true
			break
		}
	}
	if !manager {
		return Result{Status: http.StatusForbidden, Body: errorBody("forbidden", "restaurant manager permission is required")}
	}
	tableID, ok := obj["table_id"].(string)
	if !ok {
		return validationFailed("closure table_id must be a string")
	}
	known := false
	for _, t := range restaurant.Tables {
		if t.ID == tableID {
			known = true
			break
		}
	}
	if !known {
		return notFound("unknown table")
	}
	fromText, ok := obj["from"].(string)
	if !ok {
		return validationFailed("closure from must be a string")
	}
	toText, ok := obj["to"].(string)
	if !ok {
		return validationFailed("closure to must be a string")
	}
	from, ok := parseStrictInstant(fromText)
	if !ok {
		return validationFailed("closure from must be an RFC3339 instant with explicit offset")
	}
	to, ok := parseStrictInstant(toText)
	if !ok {
		return validationFailed("closure to must be an RFC3339 instant with explicit offset")
	}
	if !from.Before(to) {
		return validationFailed("closure from must be before to")
	}
	// Fixture table ids and declared pairs in their stored order.
	tableIDs := make([]string, 0, len(restaurant.Tables))
	for _, t := range restaurant.Tables {
		tableIDs = append(tableIDs, t.ID)
	}
	pairs := make([][]string, 0, len(restaurant.Combinable))
	for _, p := range restaurant.Combinable {
		pairs = append(pairs, append([]string(nil), p...))
	}
	// Previously applied closures of this restaurant (absolute parsed).
	var prior []planner.Closure
	for _, c := range st.Closures[restaurantID] {
		cf, ferr := time.Parse(time.RFC3339, c.From)
		ct, terr := time.Parse(time.RFC3339, c.To)
		if ferr != nil || terr != nil {
			continue
		}
		prior = append(prior, planner.Closure{TableID: c.TableID, From: cf, To: ct})
	}
	// Every confirmed booking at this restaurant overlapping [from,to) is
	// considered (including bookings not on the closed table); every other
	// confirmed same-restaurant booking is fixed occupancy. Cancelled and
	// foreign bookings are excluded. No cutoff checks: operator repair.
	var considered, fixed []planner.Booking
	for _, r := range st.Reservations {
		if r.RestaurantID != restaurantID || r.Status != StatusConfirmed {
			continue
		}
		start, serr := time.Parse(time.RFC3339, r.StartsAt)
		end, eerr := time.Parse(time.RFC3339, r.EndsAt)
		if serr != nil || eerr != nil {
			continue
		}
		caps := make(map[string]int, len(r.AcceptedTerms.Capacities))
		for k, v := range r.AcceptedTerms.Capacities {
			caps[k] = v
		}
		pb := planner.Booking{
			Reference:  r.Reference,
			TableIDs:   append([]string(nil), reservationTableIDs(r)...),
			PartySize:  r.PartySize,
			StartsAt:   start,
			EndsAt:     end,
			Capacities: caps,
		}
		if start.Before(to) && from.Before(end) {
			considered = append(considered, pb)
		} else {
			fixed = append(fixed, pb)
		}
	}
	solved, perr := planner.Solve(planner.Request{
		TableIDs:   tableIDs,
		Pairs:      pairs,
		Considered: considered,
		Fixed:      fixed,
		Closures:   prior,
		Proposed:   planner.Closure{TableID: tableID, From: from, To: to},
	})
	if perr != nil {
		switch perr.Code {
		case planner.CodePlanningLimit:
			return Result{Status: http.StatusUnprocessableEntity, Body: errorBody("planning_limit", "replan input exceeds supported limits")}
		default:
			return conflict("no_feasible_plan", "no feasible seating plan")
		}
	}
	// Only a feasible solve allocates an opaque plan id.
	planID, err := id.New()
	if err != nil {
		return internalErr()
	}
	stored := Replan{
		ID:                 planID,
		RestaurantID:       restaurantID,
		RestaurantRevision: st.RestaurantRevisions[restaurantID],
		Closure:            Closure{TableID: tableID, From: fromText, To: toText},
		MovedCount:         solved.MovedCount,
		UnusedSeats:        solved.UnusedSeats,
		Applied:            false,
	}
	stored.Assignments = make([]ReplanAssignment, 0, len(solved.Assignments))
	for _, a := range solved.Assignments {
		stored.Assignments = append(stored.Assignments, ReplanAssignment{
			Reference: a.Reference,
			TableIDs:  append([]string(nil), a.TableIDs...),
			Changed:   a.Changed,
		})
	}
	if st.Plans == nil {
		st.Plans = map[string]Replan{}
	}
	st.Plans[planID] = stored
	return created(replanPublic(stored))
}
