package service

import (
	"net/http"

	"tablekeeper/internal/policy"
)

// This file owns the stage-3 policy publication surface: dated selection,
// manager-gated publication with real idempotent receipts, and the public
// policy list. It performs no booking writes and mutates no reservation,
// history, term or end-time state; publication only appends an immutable
// policy and increments the per-restaurant revision once.

// selectedTerms resolves the complete isolated terms for a booking's local
// start date: the fixture policy0 snapshot when no published policy applies,
// else the eligible greatest effective date (ties to the greatest version).
// It delegates directly to policy.Select and returns an isolated snapshot.
// Frozen next-write seam for B2; no locking inside.
func selectedTerms(st *State, restaurant *Restaurant, localDate string) (policy.Terms, error) {
	return policy.Select(fixtureTerms(*restaurant), st.Policies[restaurant.ID], localDate)
}

// PublishPolicy publishes a complete dated policy for a restaurant. It runs
// through the real Idempotent engine on the actual
// POST /restaurants/{id}/policies path, so canonical replay-before-
// validation, 200 original-response replays, user/path scoping and single-
// commit concurrency all hold exactly as for every other idempotent write.
// The locked callback checks the restaurant, then manager permission, then
// parses the complete policy; only a successful callback allocates the next
// per-restaurant version, appends the cloned flat policy in publication
// order and increments the restaurant revision once. Failed publications
// and replays change no version, counter, receipt or booking state.
func (s *Service) PublishPolicy(token, restaurantID, key string, raw []byte) Result {
	path := "/restaurants/" + restaurantID + "/policies"
	return s.Idempotent(token, http.MethodPost, path, key, raw, func(st *State, uid string, obj map[string]any) Result {
		return publishPolicyLocked(st, uid, restaurantID, obj)
	})
}

func publishPolicyLocked(st *State, uid, restaurantID string, obj map[string]any) Result {
	restaurant := restaurantByID(st, restaurantID)
	if restaurant == nil {
		return notFound("unknown restaurant")
	}
	manager := false
	for _, id := range restaurant.ManagerUserIDs {
		if id == uid {
			manager = true
			break
		}
	}
	if !manager {
		return Result{Status: http.StatusForbidden, Body: errorBody("forbidden", "restaurant manager permission is required")}
	}
	tableIDs := make([]string, 0, len(restaurant.Tables))
	for _, t := range restaurant.Tables {
		tableIDs = append(tableIDs, t.ID)
	}
	parsed, err := policy.Parse(obj, tableIDs)
	if err != nil {
		if pe, ok := err.(*policy.Error); ok {
			return Result{Status: http.StatusUnprocessableEntity, Body: errorBody(pe.Code, pe.Message)}
		}
		return validationFailed("invalid policy")
	}
	parsed.PolicyVersion = len(st.Policies[restaurantID]) + 1
	stored := policy.ClonePolicy(parsed)
	if st.Policies == nil {
		st.Policies = map[string][]policy.Policy{}
	}
	st.Policies[restaurantID] = append(st.Policies[restaurantID], stored)
	if st.RestaurantRevisions == nil {
		st.RestaurantRevisions = map[string]int{}
	}
	st.RestaurantRevisions[restaurantID]++
	return created(policyToPublic(stored))
}

// policyToPublic renders a published policy as the flat specification JSON:
// the supplied fields plus the allocated policy_version. A plain map is used
// (rather than the policy struct) so the 201 response and its idempotent 200
// replay marshal byte-identically: both pass through sorted map encoding,
// while a struct would emit field order the first time and sorted order on
// replay.
func policyToPublic(p policy.Policy) map[string]any {
	hours := make([]any, 0, len(p.OpeningHours))
	for _, h := range p.OpeningHours {
		hours = append(hours, map[string]any{"weekday": h.Weekday, "opens": h.Opens, "closes": h.Closes})
	}
	caps := make(map[string]any, len(p.Capacities))
	for k, v := range p.Capacities {
		caps[k] = v
	}
	return map[string]any{
		"effective_from":               p.EffectiveFrom,
		"policy_version":               p.PolicyVersion,
		"slot_minutes":                 p.SlotMinutes,
		"reservation_duration_minutes": p.ReservationDurationMinutes,
		"cancellation_cutoff_minutes":  p.CancellationCutoffMinutes,
		"opening_hours":                hours,
		"capacities":                   caps,
	}
}

// ListPolicies renders the public policy list for a restaurant in publication
// order, omitting the fixture policy0. An empty list renders {"policies":[]}.
// Entries are detached clones; no caller can mutate published state.
func (s *Service) ListPolicies(restaurantID string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if restaurantByID(&s.state, restaurantID) == nil {
		return notFound("unknown restaurant")
	}
	out := make([]any, 0, len(s.state.Policies[restaurantID]))
	for _, p := range s.state.Policies[restaurantID] {
		out = append(out, policyToPublic(policy.ClonePolicy(p)))
	}
	return okResult(map[string]any{"policies": out})
}
