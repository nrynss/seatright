package service

// Pure stage-4 replan state: stored plan/closure values, cloning with
// nil-vs-empty fidelity, public preview rendering, and read-only closure
// occupancy. No locks, ids, clocks, counters or endpoint operations.

import (
	"time"
)

// cloneReplanMap deep-copies plans preserving nil-vs-empty at the map level
// and per-plan assignment slices plus table-id slices.
func cloneReplanMap(in map[string]Replan) map[string]Replan {
	if in == nil {
		return nil
	}
	out := make(map[string]Replan, len(in))
	for k, p := range in {
		out[k] = cloneReplan(p)
	}
	return out
}

// cloneReplan deep-copies one plan with detached assignment table slices,
// preserving nil-vs-empty assignment shape.
func cloneReplan(p Replan) Replan {
	out := Replan{
		ID:                 p.ID,
		RestaurantID:       p.RestaurantID,
		RestaurantRevision: p.RestaurantRevision,
		Closure:            p.Closure,
		MovedCount:         p.MovedCount,
		UnusedSeats:        p.UnusedSeats,
		Applied:            p.Applied,
	}
	if p.Assignments == nil {
		out.Assignments = nil
	} else {
		out.Assignments = make([]ReplanAssignment, len(p.Assignments))
		for i, a := range p.Assignments {
			out.Assignments[i] = ReplanAssignment{
				Reference: a.Reference,
				TableIDs:  cloneStringsPreserveNil(a.TableIDs),
				Changed:   a.Changed,
			}
		}
	}
	return out
}

// cloneClosureMap deep-copies closures preserving nil-vs-empty at the map
// and per-restaurant slice levels.
func cloneClosureMap(in map[string][]Closure) map[string][]Closure {
	if in == nil {
		return nil
	}
	out := make(map[string][]Closure, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]Closure, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// normalizeReplanState initializes absent plan/closure maps to empty without
// dropping existing modern data. Called by normalizeVersionState.
func normalizeReplanState(st *State) {
	if st.Plans == nil {
		st.Plans = map[string]Replan{}
	}
	if st.Closures == nil {
		st.Closures = map[string][]Closure{}
	}
}

// replanPublic renders the EXACT preview shape: plan_id, restaurant_revision,
// closure{table_id/from/to}, assignments allocated[] each with
// reference/table_ids/changed, moved_count and unused_seats.
// RestaurantID and Applied stay internal. Values are detached.
func replanPublic(p Replan) map[string]any {
	assign := make([]any, 0, len(p.Assignments))
	for _, a := range p.Assignments {
		ids := append([]string(nil), a.TableIDs...)
		if ids == nil {
			ids = []string{}
		}
		assign = append(assign, map[string]any{
			"reference": a.Reference,
			"table_ids": ids,
			"changed":   a.Changed,
		})
	}
	return map[string]any{
		"plan_id":             p.ID,
		"restaurant_revision": p.RestaurantRevision,
		"closure":             map[string]any{"table_id": p.Closure.TableID, "from": p.Closure.From, "to": p.Closure.To},
		"assignments":         assign,
		"moved_count":         p.MovedCount,
		"unused_seats":        p.UnusedSeats,
	}
}

// closureBlocks reports whether any stored closure of the restaurant blocks
// the given table set over the half-open interval [start,end): a stored
// RFC3339-explicit-offset closure interval overlapping [start,end) in the
// half-open sense on any shared table (any pair member) blocks. Other
// restaurants are harmless; adjacent intervals are free. Read-only: no
// counter, id or clock mutation. Unparseable stored closures never block.
func closureBlocks(st *State, restaurantID string, ids []string, start, end time.Time) bool {
	for _, c := range st.Closures[restaurantID] {
		hit := false
		for _, id := range ids {
			if id == c.TableID {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		from, err := time.Parse(time.RFC3339, c.From)
		if err != nil {
			continue
		}
		to, err := time.Parse(time.RFC3339, c.To)
		if err != nil {
			continue
		}
		if from.Before(end) && start.Before(to) {
			return true
		}
	}
	return false
}
