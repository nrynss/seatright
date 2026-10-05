// Package planner is a pure seating optimizer for table closures.
//
// It assigns each considered booking a single table or a declared pair such
// that capacities (from each booking's own accepted terms), fixed-booking
// occupancy, prior closures and the proposed closure all hold, minimizing in
// order: the number of changed table sets, total unused seats across all
// considered bookings, and the option-rank vector in ascending reference
// order.
//
// The package is pure: no service imports, locks, ids, clock reads, state
// mutation or input aliasing. Callers supply parsed instants and valid ids.
package planner

import (
	"sort"
	"time"
)

// Booking is one considered or fixed confirmed booking. Capacities maps table
// id to the capacity under that booking's own accepted terms; fixture or
// publication maxima never apply here.
type Booking struct {
	Reference  string
	TableIDs   []string
	PartySize  int
	StartsAt   time.Time
	EndsAt     time.Time
	Capacities map[string]int
}

// Closure blocks one table over a half-open instant interval [From, To).
type Closure struct {
	TableID string
	From    time.Time
	To      time.Time
}

// Request is one planning problem. TableIDs lists every table in fixture
// order; Pairs lists every declared pair in declared order (pairs only, no
// transitivity). Considered holds the bookings to assign; Fixed holds
// confirmed non-considered bookings at the same restaurant that occupy
// tables. Closures holds previously applied closures; Proposed is the new
// closure under review.
type Request struct {
	TableIDs   []string
	Pairs      [][]string
	Considered []Booking
	Fixed      []Booking
	Closures   []Closure
	Proposed   Closure
}

// Assignment is the planned table set for one booking, in canonical order
// (single id, or declared pair order). Changed reports set-based inequality
// with the booking's original table set.
type Assignment struct {
	Reference string
	TableIDs  []string
	Changed   bool
}

// Plan is the optimal assignment: one entry per considered booking in
// ascending reference order.
type Plan struct {
	Assignments []Assignment
	MovedCount  int
	UnusedSeats int
}

// Error is a planning failure with a stable machine code.
type Error struct {
	Code string
}

// Error implements error.
func (e Error) Error() string {
	return e.Code
}

// Planning limits from the specification. Larger inputs may be rejected.
const (
	maxTables     = 6
	maxPairs      = 4
	maxConsidered = 6
)

// Codes returned in Error.Code.
const (
	CodePlanningLimit = "planning_limit"
	CodeNoFeasible    = "no_feasible_plan"
)

// option is one bookable table set with its global rank.
type option struct {
	tables []string
	rank   int
}

// Solve returns the optimal plan for req, or a planning *Error.
func Solve(req Request) (Plan, *Error) {
	if len(req.TableIDs) > maxTables || len(req.Pairs) > maxPairs ||
		len(req.Considered) > maxConsidered {
		return Plan{}, &Error{Code: CodePlanningLimit}
	}
	// Clone and sort considered bookings by reference; never mutate input.
	sorted := append([]Booking(nil), req.Considered...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Reference < sorted[j].Reference
	})
	opts := buildOptions(req.TableIDs, req.Pairs)
	fixedSets := make([]map[string]bool, len(req.Fixed))
	for i, f := range req.Fixed {
		fixedSets[i] = tableSet(f.TableIDs)
	}
	closures := append([]Closure(nil), req.Closures...)
	closures = append(closures, req.Proposed)
	// Per-booking feasible options under capacity, fixed occupancy and
	// closures (mutual considered conflicts are checked per combination).
	feasible := make([][]int, len(sorted))
	for i, b := range sorted {
		for oi, o := range opts {
			if capacityOf(b, o.tables) < b.PartySize {
				continue
			}
			if blockedByFixed(b, o.tables, req.Fixed, fixedSets) {
				continue
			}
			if blockedByClosures(b, o.tables, closures) {
				continue
			}
			feasible[i] = append(feasible[i], oi)
		}
		if len(feasible[i]) == 0 {
			return Plan{}, &Error{Code: CodeNoFeasible}
		}
	}
	best := search(sorted, opts, feasible)
	if !best.found {
		return Plan{}, &Error{Code: CodeNoFeasible}
	}
	plan := Plan{Assignments: make([]Assignment, 0, len(sorted))}
	for i, b := range sorted {
		o := opts[best.pick[i]]
		tables := append([]string(nil), o.tables...)
		plan.Assignments = append(plan.Assignments, Assignment{
			Reference: b.Reference,
			TableIDs:  tables,
			Changed:   !setsEqual(tableSet(b.TableIDs), tableSet(tables)),
		})
		plan.UnusedSeats += capacityOf(b, o.tables) - b.PartySize
		if !setsEqual(tableSet(b.TableIDs), tableSet(tables)) {
			plan.MovedCount++
		}
	}
	return plan, nil
}

// buildOptions ranks singles in fixture order, then declared pairs.
func buildOptions(tables []string, pairs [][]string) []option {
	opts := make([]option, 0, len(tables)+len(pairs))
	for _, t := range tables {
		opts = append(opts, option{tables: []string{t}, rank: len(opts)})
	}
	for _, p := range pairs {
		cp := append([]string(nil), p...)
		opts = append(opts, option{tables: cp, rank: len(opts)})
	}
	return opts
}

// tableSet copies ids into a membership set.
func tableSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

// setsEqual reports set equality of two membership sets.
func setsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// capacityOf sums the booking's own capacities over an option's members.
// Members absent from the booking's terms contribute zero.
func capacityOf(b Booking, tables []string) int {
	sum := 0
	for _, t := range tables {
		sum += b.Capacities[t]
	}
	return sum
}

// halfOpenOverlap reports whether [aStart,aEnd) and [bStart,bEnd) overlap.
func halfOpenOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// sharesMember reports whether tables intersect a membership set.
func sharesMember(tables []string, set map[string]bool) bool {
	for _, t := range tables {
		if set[t] {
			return true
		}
	}
	return false
}

// blockedByFixed reports occupancy conflicts with non-considered bookings:
// overlapping half-open intervals with a shared member table.
func blockedByFixed(b Booking, tables []string, fixed []Booking, sets []map[string]bool) bool {
	for i, f := range fixed {
		if halfOpenOverlap(b.StartsAt, b.EndsAt, f.StartsAt, f.EndsAt) &&
			sharesMember(tables, sets[i]) {
			return true
		}
	}
	return false
}

// blockedByClosures reports closure conflicts: an option containing the
// closed table is excluded exactly when the booking interval overlaps the
// closure interval. A closed single and every containing pair are excluded.
func blockedByClosures(b Booking, tables []string, closures []Closure) bool {
	for _, c := range closures {
		if halfOpenOverlap(b.StartsAt, b.EndsAt, c.From, c.To) {
			for _, t := range tables {
				if t == c.TableID {
					return true
				}
			}
		}
	}
	return false
}

// result is one complete candidate combination.
type result struct {
	found  bool
	pick   []int
	moved  int
	unused int
	ranks  []int
}

// better reports whether candidate beats the incumbent on the objective
// tuple (changed sets, total unused seats, rank vector lex order).
func better(candMoved, candUnused int, candRanks []int, inc result) bool {
	if !inc.found {
		return true
	}
	if candMoved != inc.moved {
		return candMoved < inc.moved
	}
	if candUnused != inc.unused {
		return candUnused < inc.unused
	}
	for i := range candRanks {
		if candRanks[i] != inc.ranks[i] {
			return candRanks[i] < inc.ranks[i]
		}
	}
	return false
}

// search exhaustively enumerates every feasible combination in reference
// order; no greedy shortcut. Bounds are tiny (<=10 options, <=6 bookings).
func search(sorted []Booking, opts []option, feasible [][]int) result {
	n := len(sorted)
	best := result{}
	pick := make([]int, n)
	ranks := make([]int, n)
	var rec func(i, moved, unused int)
	rec = func(i, moved, unused int) {
		if i == n {
			for a := 0; a < n; a++ {
				for c := a + 1; c < n; c++ {
					if halfOpenOverlap(sorted[a].StartsAt, sorted[a].EndsAt,
						sorted[c].StartsAt, sorted[c].EndsAt) &&
						sharesMember(opts[pick[a]].tables, tableSet(opts[pick[c]].tables)) {
						return
					}
				}
			}
			rc := append([]int(nil), ranks...)
			if better(moved, unused, rc, best) {
				best = result{found: true, pick: append([]int(nil), pick...),
					moved: moved, unused: unused, ranks: rc}
			}
			return
		}
		b := sorted[i]
		orig := tableSet(b.TableIDs)
		for _, oi := range feasible[i] {
			o := opts[oi]
			pick[i] = oi
			ranks[i] = o.rank
			dm := 0
			if !setsEqual(orig, tableSet(o.tables)) {
				dm = 1
			}
			rec(i+1, moved+dm, unused+capacityOf(b, o.tables)-b.PartySize)
		}
	}
	rec(0, 0, 0)
	return best
}
