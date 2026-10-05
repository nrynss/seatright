package service

import (
	"net/http"
	"time"

	"tablekeeper/internal/history"
)

// This file owns the stage-4 plan application surface: manager-gated atomic
// POST /restaurants/{id}/replans/{plan_id}/apply with real idempotent
// receipts. Application records the closure and every assignment together,
// moves each changed booking once (revision + one reassigned history entry,
// terms and times preserved), bumps the restaurant revision exactly once for
// the whole plan, and touches each affected series once without marking new
// exceptions. Closure write enforcement in availability and the common
// booking-conflict seam lives in availability.go and reservations.go.

// ApplyReplan applies a previewed seating plan. It runs through the real
// Idempotent engine on the actual
// POST /restaurants/{id}/replans/{plan_id}/apply path, so canonical
// replay-before-validation, 200 original-response replays, user/path
// scoping and single-commit concurrency hold exactly as for every other
// idempotent write.
func (s *Service) ApplyReplan(token, restaurantID, planID, key string, raw []byte) Result {
	path := "/restaurants/" + restaurantID + "/replans/" + planID + "/apply"
	return s.Idempotent(token, http.MethodPost, path, key, raw, func(st *State, uid string, obj map[string]any) Result {
		return applyReplanLocked(st, uid, restaurantID, planID, obj)
	})
}

func applyReplanLocked(st *State, userID, restaurantID, planID string, obj map[string]any) Result {
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
	plan, ok := st.Plans[planID]
	if !ok || plan.RestaurantID != restaurantID {
		return notFound("unknown plan")
	}
	// An applied plan under a new key is already-applied even though the
	// restaurant counter moved on with its own application: check this
	// before staleness so the required code is not masked. Same-key
	// replays resolve inside the wrapper before this callback runs.
	if plan.Applied {
		return conflict("plan_already_applied", "plan was already applied")
	}
	if st.RestaurantRevisions[restaurantID] != plan.RestaurantRevision {
		return conflict("stale_plan", "restaurant changed since preview")
	}
	// Preflight every saved considered booking: identity, ownership,
	// restaurant, party, times, creation, status and accepted terms must be
	// exactly as previewed. Any deviation means the world moved on (every
	// API write bumps the counter, so this is defensive only).
	type preflight struct {
		rec  Reservation
		want ReplanAssignment
	}
	checked := make([]preflight, 0, len(plan.Assignments))
	for _, a := range plan.Assignments {
		rec, ok := st.Reservations[a.Reference]
		if !ok || rec.Status != StatusConfirmed || rec.RestaurantID != restaurantID {
			return conflict("stale_plan", "booking changed since preview")
		}
		checked = append(checked, preflight{rec: rec, want: a})
	}
	// Build detached candidates, changing only the canonical table set.
	// Set-equal assignments are unmoved: no revision, history or terms
	// drift. Operator repair never revalidates, checks cutoffs or adopts
	// new policy terms.
	type applied struct {
		final   Reservation
		changed bool
	}
	finals := make([]applied, 0, len(checked))
	changedRefs := make([]string, 0, len(checked))
	for _, c := range checked {
		rec := c.rec
		if tableSetEqual(reservationTableIDs(rec), c.want.TableIDs) {
			finals = append(finals, applied{final: rec})
			continue
		}
		beforeSnap := reservationSnapshot(rec)
		rec.Revision++
		setReservationTables(&rec, c.want.TableIDs)
		afterSnap := reservationSnapshot(rec)
		entries := st.Histories[rec.Reference]
		seq, at := history.Next(entries, time.Now())
		entry, ok := history.Reassigned(beforeSnap, afterSnap, seq, at, plan.ID)
		if !ok {
			return internalErr()
		}
		st.Histories[rec.Reference] = append(entries, entry)
		finals = append(finals, applied{final: rec, changed: true})
		changedRefs = append(changedRefs, rec.Reference)
	}
	// Atomic commit: closure, assignments, applied flag, one counter bump
	// for the whole plan (even with zero considered or zero moved), and one
	// series touch per affected series preserving all exception flags.
	for _, f := range finals {
		st.Reservations[f.final.Reference] = f.final
	}
	st.Closures[restaurantID] = append(st.Closures[restaurantID], plan.Closure)
	plan.Applied = true
	st.Plans[planID] = plan
	if st.RestaurantRevisions == nil {
		st.RestaurantRevisions = map[string]int{}
	}
	st.RestaurantRevisions[restaurantID]++
	touchSeriesForChanges(st, changedRefs, false)
	out := make([]any, 0, len(finals))
	for _, f := range finals {
		out = append(out, st.Reservations[f.final.Reference].Public())
	}
	return created(map[string]any{
		"plan_id":             plan.ID,
		"restaurant_revision": st.RestaurantRevisions[restaurantID],
		"reservations":        out,
	})
}
