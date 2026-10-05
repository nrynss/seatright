package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

func instant(h, m int) time.Time {
	return time.Date(2026, 9, 28, h, m, 0, 0, time.UTC)
}

func bk(ref string, tables []string, party int, caps map[string]int, sH, sM, eH, eM int) Booking {
	cp := make(map[string]int, len(caps))
	for k, v := range caps {
		cp[k] = v
	}
	return Booking{
		Reference:  ref,
		TableIDs:   append([]string(nil), tables...),
		PartySize:  party,
		StartsAt:   instant(sH, sM),
		EndsAt:     instant(eH, eM),
		Capacities: cp,
	}
}

func baseTables() []string { return []string{"t_1", "t_2", "t_3"} }

func baseCaps() map[string]int { return map[string]int{"t_1": 2, "t_2": 4, "t_3": 4} }

func mustSolve(t *testing.T, req Request) Plan {
	t.Helper()
	plan, perr := Solve(req)
	if perr != nil {
		t.Fatalf("Solve error: %v", perr.Code)
	}
	return plan
}

func mustErr(t *testing.T, req Request, code string) {
	t.Helper()
	_, perr := Solve(req)
	if perr == nil {
		t.Fatalf("expected error %q, got plan", code)
	}
	if perr.Code != code {
		t.Fatalf("error code = %q, want %q", perr.Code, code)
	}
	if perr.Error() == "" {
		t.Fatal("empty Error() string")
	}
}

func TestSolveEmpty(t *testing.T) {
	plan := mustSolve(t, Request{
		TableIDs:   baseTables(),
		Considered: nil,
		Proposed:   Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
	})
	if plan.Assignments == nil {
		t.Fatal("assignments must be allocated, not nil")
	}
	if len(plan.Assignments) != 0 || plan.MovedCount != 0 || plan.UnusedSeats != 0 {
		t.Fatalf("unexpected empty plan: %+v", plan)
	}
}

func TestSolveLimits(t *testing.T) {
	mk := func(nT, nP, nC int) Request {
		tables := make([]string, nT)
		for i := range tables {
			tables[i] = fmt.Sprintf("t_%d", i+1)
		}
		pairs := make([][]string, nP)
		for i := range pairs {
			pairs[i] = []string{tables[0], tables[1]}
		}
		considered := make([]Booking, nC)
		caps := map[string]int{}
		for _, tb := range tables {
			caps[tb] = 4
		}
		for i := range considered {
			considered[i] = bk(fmt.Sprintf("R%02d", i), []string{tables[0]}, 1, caps, 18, 0, 19, 0)
		}
		return Request{TableIDs: tables, Pairs: pairs, Considered: considered,
			Proposed: Closure{TableID: tables[0], From: instant(20, 0), To: instant(21, 0)}}
	}
	mustErr(t, mk(7, 0, 0), CodePlanningLimit)
	mustErr(t, mk(6, 5, 0), CodePlanningLimit)
	mustErr(t, mk(6, 4, 7), CodePlanningLimit)
	// Exact maxima are accepted.
	plan := mustSolve(t, mk(6, 4, 6))
	if len(plan.Assignments) != 6 {
		t.Fatalf("max-limit plan has %d assignments", len(plan.Assignments))
	}
}

func TestSolveInfeasible(t *testing.T) {
	// Single table closed over the whole booking; no alternative.
	mustErr(t, Request{
		TableIDs:   []string{"t_1"},
		Considered: []Booking{bk("A", []string{"t_1"}, 2, map[string]int{"t_1": 2}, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
	}, CodeNoFeasible)
	// Capacity alone infeasible.
	mustErr(t, Request{
		TableIDs:   []string{"t_1"},
		Considered: []Booking{bk("A", []string{"t_1"}, 5, map[string]int{"t_1": 2}, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_9", From: instant(18, 0), To: instant(19, 0)},
	}, CodeNoFeasible)
	// Mutual considered conflict with no alternative.
	mustErr(t, Request{
		TableIDs: []string{"t_1"},
		Considered: []Booking{
			bk("A", []string{"t_1"}, 1, map[string]int{"t_1": 2}, 18, 0, 19, 0),
			bk("B", []string{"t_1"}, 1, map[string]int{"t_1": 2}, 18, 30, 19, 30),
		},
		Proposed: Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
	}, CodeNoFeasible)
}

func TestSolveHalfOpenAdjacency(t *testing.T) {
	caps := baseCaps()
	// Booking [18:00,19:00); closure [19:00,20:00): adjacent, allowed.
	plan := mustSolve(t, Request{
		TableIDs:   baseTables(),
		Considered: []Booking{bk("A", []string{"t_1"}, 2, caps, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_1", From: instant(19, 0), To: instant(20, 0)},
	})
	if len(plan.Assignments) != 1 || plan.Assignments[0].Changed {
		t.Fatalf("adjacent closure must not move booking: %+v", plan)
	}
	// Booking [19:00,20:00); closure [18:00,19:00): adjacent, allowed.
	plan = mustSolve(t, Request{
		TableIDs:   baseTables(),
		Considered: []Booking{bk("A", []string{"t_1"}, 2, caps, 19, 0, 20, 0)},
		Proposed:   Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
	})
	if plan.Assignments[0].Changed {
		t.Fatalf("adjacent closure must not move booking: %+v", plan)
	}
	// Short closure strictly inside a long booking still blocks the member.
	plan = mustSolve(t, Request{
		TableIDs:   baseTables(),
		Considered: []Booking{bk("A", []string{"t_1"}, 2, caps, 18, 0, 21, 0)},
		Proposed:   Closure{TableID: "t_1", From: instant(19, 0), To: instant(19, 30)},
	})
	if len(plan.Assignments) != 1 || !plan.Assignments[0].Changed {
		t.Fatalf("inner short closure must move booking: %+v", plan)
	}
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_2"}) {
		t.Fatalf("expected cheapest feasible single t_2, got %v", got)
	}
}

func TestSolveFixedPairBlocks(t *testing.T) {
	caps := baseCaps()
	// Fixed pair on t_2+t_3 blocks both members for an overlapping booking.
	plan := mustSolve(t, Request{
		TableIDs: baseTables(),
		Pairs:    [][]string{{"t_2", "t_3"}},
		Considered: []Booking{
			bk("A", []string{"t_2"}, 2, caps, 18, 0, 19, 0),
		},
		Fixed: []Booking{
			bk("F", []string{"t_2", "t_3"}, 6, caps, 18, 30, 19, 30),
		},
		Proposed: Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
	})
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_1"}) {
		t.Fatalf("shared member blocked, expected t_1, got %v", got)
	}
}

func TestSolvePreviousAndProposedClosure(t *testing.T) {
	caps := baseCaps()
	mk := func(closures []Closure) Request {
		return Request{
			TableIDs:   []string{"t_1", "t_2"},
			Considered: []Booking{bk("A", []string{"t_1"}, 2, caps, 18, 0, 19, 0)},
			Closures:   closures,
			Proposed:   Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
		}
	}
	// A prior overlapping closure on t_1 forces the move even though the
	// proposed closure is elsewhere.
	plan := mustSolve(t, mk([]Closure{{TableID: "t_1", From: instant(18, 30), To: instant(19, 30)}}))
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_2"}) {
		t.Fatalf("prior closure must move booking to t_2, got %v", got)
	}
	// A prior non-overlapping closure changes nothing.
	plan = mustSolve(t, mk([]Closure{{TableID: "t_1", From: instant(20, 0), To: instant(21, 0)}}))
	if plan.Assignments[0].Changed {
		t.Fatalf("non-overlapping prior closure must not move: %+v", plan)
	}
}

func TestSolveOwnAcceptedCapacities(t *testing.T) {
	// Booking's own terms exceed publication maxima: still usable.
	own := map[string]int{"t_1": 101, "t_2": 4}
	plan := mustSolve(t, Request{
		TableIDs:   []string{"t_1", "t_2"},
		Considered: []Booking{bk("A", []string{"t_2"}, 90, own, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_2", From: instant(18, 0), To: instant(19, 0)},
	})
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_1"}) {
		t.Fatalf("above-max accepted capacity must apply, got %v", got)
	}
	if plan.UnusedSeats != 101-90 {
		t.Fatalf("unused = %d, want 11", plan.UnusedSeats)
	}
	// Fixed bookings block by occupancy; their own capacities are irrelevant
	// (a fixed booking with tiny capacities still occupies its tables).
	fixedCaps := map[string]int{"t_1": 1}
	mustErr(t, Request{
		TableIDs:   []string{"t_1"},
		Considered: []Booking{bk("A", []string{"t_1"}, 1, map[string]int{"t_1": 4}, 18, 0, 19, 0)},
		Fixed:      []Booking{bk("F", []string{"t_1"}, 1, fixedCaps, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
	}, CodeNoFeasible)
}

func TestSolveObjectiveTier1Changed(t *testing.T) {
	// Genuine tradeoff: staying on t_1 has moved 0 but waste 2 (own cap 4,
	// party 2); moving to t_2 has moved 1 but waste 0. Tier 1 (changed
	// count) dominates tier 2 (waste): the booking must stay.
	caps := map[string]int{"t_1": 4, "t_2": 2, "t_3": 4}
	plan := mustSolve(t, Request{
		TableIDs:   []string{"t_1", "t_2", "t_3"},
		Considered: []Booking{bk("A", []string{"t_1"}, 2, caps, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_3", From: instant(18, 0), To: instant(19, 0)},
	})
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_1"}) {
		t.Fatalf("tier1 must keep booking on t_1, got %v", got)
	}
	if plan.Assignments[0].Changed || plan.MovedCount != 0 {
		t.Fatalf("stay must be unmoved: %+v", plan)
	}
	if plan.UnusedSeats != 2 {
		t.Fatalf("unused = %d, want 2: %+v", plan.UnusedSeats, plan)
	}
}

func TestSolveObjectiveTier2Unused(t *testing.T) {
	// Forced move off t_3 (closed). Both remaining singles move exactly
	// once, but the earlier-ranked t_1 (own cap 6, waste 4) loses to the
	// later-ranked tight t_2 (own cap 3, waste 1): waste beats rank.
	caps := map[string]int{"t_1": 6, "t_2": 3, "t_3": 4}
	plan := mustSolve(t, Request{
		TableIDs:   []string{"t_1", "t_2", "t_3"},
		Considered: []Booking{bk("A", []string{"t_3"}, 2, caps, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_3", From: instant(18, 0), To: instant(19, 0)},
	})
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_2"}) {
		t.Fatalf("tier2 must pick tight t_2 over ranked t_1, got %v", got)
	}
	if plan.MovedCount != 1 || plan.UnusedSeats != 1 {
		t.Fatalf("want moved=1 unused=1: %+v", plan)
	}
}

func TestSolveObjectiveTier3LexTie(t *testing.T) {
	// Symmetric tables: A and B interchangeable. References sorted: A < B.
	// Rank vector lex-minimal unique: A->t_1(rank0), B->t_2(rank1), not the
	// reverse.
	caps := map[string]int{"t_1": 4, "t_2": 4}
	plan := mustSolve(t, Request{
		TableIDs: []string{"t_1", "t_2"},
		Considered: []Booking{
			bk("B", []string{"t_9"}, 2, caps, 18, 0, 19, 0),
			bk("A", []string{"t_9"}, 2, caps, 18, 0, 19, 0),
		},
		Proposed: Closure{TableID: "t_9", From: instant(18, 0), To: instant(19, 0)},
	})
	if len(plan.Assignments) != 2 {
		t.Fatalf("want 2 assignments: %+v", plan)
	}
	if plan.Assignments[0].Reference != "A" || plan.Assignments[1].Reference != "B" {
		t.Fatalf("assignments must be reference-ordered: %+v", plan)
	}
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_1"}) {
		t.Fatalf("lex tie: A must take rank0 t_1, got %v", got)
	}
	if got := plan.Assignments[1].TableIDs; !reflect.DeepEqual(got, []string{"t_2"}) {
		t.Fatalf("lex tie: B must take rank1 t_2, got %v", got)
	}
}

func TestSolveSetEqualityUnchanged(t *testing.T) {
	// Reversed declared pair input names the same set: no change.
	plan := mustSolve(t, Request{
		TableIDs:   baseTables(),
		Pairs:      [][]string{{"t_1", "t_2"}},
		Considered: []Booking{bk("A", []string{"t_2", "t_1"}, 6, map[string]int{"t_1": 2, "t_2": 4}, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
	})
	if plan.Assignments[0].Changed || plan.MovedCount != 0 {
		t.Fatalf("reversed pair must be unchanged: %+v", plan)
	}
	if got := plan.Assignments[0].TableIDs; !reflect.DeepEqual(got, []string{"t_1", "t_2"}) {
		t.Fatalf("canonical declared order expected, got %v", got)
	}
}

func TestSolveNonTransitive(t *testing.T) {
	caps := map[string]int{"t_1": 3, "t_2": 3, "t_3": 3}
	// Pairs [t_1,t_2] and [t_2,t_3] do not authorize {t_1,t_3} for party 5:
	// t_1+t_3 capacity would be 6 and neither member is blocked, yet the
	// pair is undeclared. The other declared pair is occupied, so nothing
	// remains feasible.
	mustErr(t, Request{
		TableIDs:   []string{"t_1", "t_2", "t_3"},
		Pairs:      [][]string{{"t_1", "t_2"}, {"t_2", "t_3"}},
		Considered: []Booking{bk("A", []string{"t_1"}, 5, caps, 18, 0, 19, 0)},
		Fixed:      []Booking{bk("F", []string{"t_2"}, 1, caps, 18, 0, 19, 0)},
		Proposed:   Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
	}, CodeNoFeasible)
}

func TestSolveGreedyTrapExhaustive(t *testing.T) {
	// Greedy "keep the first booking, place the second cheapest" fails:
	// keeping A on t_1 leaves B (party 4, only t_1 fits among free singles)
	// with no feasible table, so the global optimum must move the keepable
	// A to t_2 to free t_1 for B. Both change; total waste is zero.
	capsA := map[string]int{"t_1": 2, "t_2": 2, "t_3": 4}
	capsB := map[string]int{"t_1": 4, "t_2": 1, "t_3": 4}
	plan := mustSolve(t, Request{
		TableIDs: []string{"t_1", "t_2", "t_3"},
		Considered: []Booking{
			bk("A", []string{"t_1"}, 2, capsA, 18, 0, 19, 0),
			bk("B", []string{"t_3"}, 4, capsB, 18, 0, 19, 0),
		},
		Proposed: Closure{TableID: "t_3", From: instant(18, 0), To: instant(19, 0)},
	})
	want := map[string][]string{"A": {"t_2"}, "B": {"t_1"}}
	for _, a := range plan.Assignments {
		if !reflect.DeepEqual(a.TableIDs, want[a.Reference]) {
			t.Fatalf("assignment mismatch: %+v, want %v", plan, want)
		}
		if !a.Changed {
			t.Fatalf("both bookings must change: %+v", plan)
		}
	}
	if plan.MovedCount != 2 || plan.UnusedSeats != 0 {
		t.Fatalf("want moved=2 unused=0: %+v", plan)
	}
}

func TestSolveDeterminismShuffled(t *testing.T) {
	caps := map[string]int{"t_1": 2, "t_2": 2, "t_3": 2, "t_4": 2}
	mk := func(order []Booking) Request {
		return Request{
			TableIDs:   []string{"t_1", "t_2", "t_3", "t_4"},
			Pairs:      [][]string{{"t_1", "t_2"}, {"t_3", "t_4"}},
			Considered: order,
			Fixed:      []Booking{bk("F", []string{"t_1"}, 1, caps, 20, 0, 21, 0)},
			Proposed:   Closure{TableID: "t_2", From: instant(18, 0), To: instant(19, 0)},
		}
	}
	a := []Booking{
		bk("C", []string{"t_3"}, 2, caps, 18, 0, 19, 0),
		bk("A", []string{"t_2"}, 2, caps, 18, 0, 19, 0),
		bk("B", []string{"t_1"}, 2, caps, 18, 0, 19, 0),
	}
	b := []Booking{a[1], a[2], a[0]}
	p1 := mustSolve(t, mk(a))
	p2 := mustSolve(t, mk(b))
	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("shuffled input changed plan:\n%+v\n%+v", p1, p2)
	}
}

func TestSolveSnapshotIsolation(t *testing.T) {
	caps := baseCaps()
	mkReq := func() Request {
		return Request{
			TableIDs:   append([]string(nil), baseTables()...),
			Pairs:      [][]string{{"t_1", "t_2"}},
			Considered: []Booking{bk("A", []string{"t_2"}, 2, caps, 18, 0, 19, 0)},
			Proposed:   Closure{TableID: "t_2", From: instant(18, 0), To: instant(19, 0)},
		}
	}
	req := mkReq()
	before := mkReq()
	plan := mustSolve(t, req)
	// Solve must leave the complete input structure unchanged.
	if !reflect.DeepEqual(req, before) {
		t.Fatalf("Solve mutated its input:\ngot  %+v\nwant %+v", req, before)
	}
	// Mutating returned outputs must not affect the original inputs.
	plan.Assignments[0].TableIDs[0] = "MUT"
	if !reflect.DeepEqual(req, before) {
		t.Fatalf("output aliases input: %+v vs %+v", req, before)
	}
	// Mutating original inputs after Solve must not affect saved outputs:
	// solve a NAMED source, snapshot the complete output bytes, mutate that
	// same source, then require the saved output still byte-matches.
	source := mkReq()
	saved, perr := Solve(source)
	if perr != nil {
		t.Fatalf("source solve error: %v", perr.Code)
	}
	savedJSON, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal saved output: %v", err)
	}
	source.Considered[0].TableIDs[0] = "MUT"
	source.Considered[0].Capacities["t_1"] = 999
	source.TableIDs[0] = "MUT"
	source.Pairs[0][0] = "MUT"
	afterJSON, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal saved output after mutation: %v", err)
	}
	if !bytes.Equal(savedJSON, afterJSON) {
		t.Fatalf("saved output changed after input mutation:\n%s\n%s", savedJSON, afterJSON)
	}
	// Nil vs empty considered shapes both solve to allocated empties.
	for _, c := range [][]Booking{nil, {}} {
		p, perr := Solve(Request{
			TableIDs:   baseTables(),
			Considered: c,
			Proposed:   Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
		})
		if perr != nil {
			t.Fatalf("empty shape error: %v", perr.Code)
		}
		if p.Assignments == nil || len(p.Assignments) != 0 {
			t.Fatalf("empty shape must yield allocated []: %+v", p)
		}
	}
	// Repeatability on a pristine equivalent.
	if again := mustSolve(t, mkReq()); !reflect.DeepEqual(saved, again) {
		t.Fatalf("solve not repeatable: %+v vs %+v", saved, again)
	}
}

// Independent oracle: brute-force over the option product with its own
// option construction, capacity sums, membership tests, interval
// predicates, set-change detection and lexicographic tuple comparison. It
// shares no helpers with the production package.
type oracleOption struct {
	tables []string
	rank   int
}

func oracleOverlaps(aS, aE, bS, bE time.Time) bool {
	return aS.Before(bE) && bS.Before(aE)
}

func oracleContains(tables []string, id string) bool {
	for _, t := range tables {
		if t == id {
			return true
		}
	}
	return false
}

func oracleIntersect(a, b []string) bool {
	for _, t := range a {
		if oracleContains(b, t) {
			return true
		}
	}
	return false
}

func oracleCap(caps map[string]int, tables []string) int {
	sum := 0
	for _, t := range tables {
		sum += caps[t]
	}
	return sum
}

func oracleSameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, t := range a {
		if !oracleContains(b, t) {
			return false
		}
	}
	return true
}

func oracleBest(req Request) (Plan, bool) {
	var opts []oracleOption
	for _, t := range req.TableIDs {
		opts = append(opts, oracleOption{tables: []string{t}, rank: len(opts)})
	}
	for _, p := range req.Pairs {
		cp := append([]string(nil), p...)
		opts = append(opts, oracleOption{tables: cp, rank: len(opts)})
	}
	idx := make([]Booking, len(req.Considered))
	copy(idx, req.Considered)
	sort.Slice(idx, func(i, j int) bool { return idx[i].Reference < idx[j].Reference })
	var closures []Closure
	closures = append(closures, req.Closures...)
	closures = append(closures, req.Proposed)
	n := len(idx)
	if n == 0 {
		return Plan{Assignments: []Assignment{}}, true
	}
	base := len(opts)
	total := 1
	for i := 0; i < n; i++ {
		total *= base
	}
	bestFound := false
	var bestPick []int
	bestMoved, bestUnused := 0, 0
	var bestRanks []int
	for code := 0; code < total; code++ {
		pick := make([]int, n)
		x := code
		for i := 0; i < n; i++ {
			pick[i] = x % base
			x /= base
		}
		moved, unused := 0, 0
		ranks := make([]int, n)
		feasible := true
		for i, b := range idx {
			o := opts[pick[i]]
			if oracleCap(b.Capacities, o.tables) < b.PartySize {
				feasible = false
				break
			}
			for _, f := range req.Fixed {
				if oracleOverlaps(b.StartsAt, b.EndsAt, f.StartsAt, f.EndsAt) &&
					oracleIntersect(o.tables, f.TableIDs) {
					feasible = false
					break
				}
			}
			if !feasible {
				break
			}
			for _, c := range closures {
				if oracleOverlaps(b.StartsAt, b.EndsAt, c.From, c.To) &&
					oracleContains(o.tables, c.TableID) {
					feasible = false
					break
				}
			}
			if !feasible {
				break
			}
			ranks[i] = o.rank
			unused += oracleCap(b.Capacities, o.tables) - b.PartySize
			if !oracleSameSet(b.TableIDs, o.tables) {
				moved++
			}
		}
		if !feasible {
			continue
		}
		for i := 0; i < n && feasible; i++ {
			for j := i + 1; j < n && feasible; j++ {
				if oracleOverlaps(idx[i].StartsAt, idx[i].EndsAt, idx[j].StartsAt, idx[j].EndsAt) &&
					oracleIntersect(opts[pick[i]].tables, opts[pick[j]].tables) {
					feasible = false
				}
			}
		}
		if !feasible {
			continue
		}
		take := false
		if !bestFound {
			take = true
		} else if moved != bestMoved {
			take = moved < bestMoved
		} else if unused != bestUnused {
			take = unused < bestUnused
		} else {
			for i := range ranks {
				if ranks[i] != bestRanks[i] {
					take = ranks[i] < bestRanks[i]
					break
				}
			}
		}
		if take {
			bestFound = true
			bestPick = append([]int(nil), pick...)
			bestMoved, bestUnused = moved, unused
			bestRanks = append([]int(nil), ranks...)
		}
	}
	if !bestFound {
		return Plan{}, false
	}
	out := Plan{Assignments: make([]Assignment, 0, n)}
	for i, b := range idx {
		ts := append([]string(nil), opts[bestPick[i]].tables...)
		out.Assignments = append(out.Assignments, Assignment{
			Reference: b.Reference,
			TableIDs:  ts,
			Changed:   !oracleSameSet(b.TableIDs, ts),
		})
	}
	out.MovedCount, out.UnusedSeats = bestMoved, bestUnused
	return out, true
}

func TestSolveBruteForceOracle(t *testing.T) {
	worlds := []struct {
		req      Request
		feasible bool
	}{
		{req: Request{
			TableIDs: []string{"t_1", "t_2", "t_3"},
			Pairs:    [][]string{{"t_1", "t_2"}, {"t_2", "t_3"}},
			Considered: []Booking{
				bk("A", []string{"t_1"}, 2, map[string]int{"t_1": 2, "t_2": 3, "t_3": 5}, 18, 0, 19, 30),
				bk("B", []string{"t_2", "t_3"}, 6, map[string]int{"t_1": 4, "t_2": 4, "t_3": 8}, 18, 0, 19, 0),
				bk("C", []string{"t_3"}, 1, map[string]int{"t_1": 2, "t_2": 2, "t_3": 2}, 19, 30, 21, 0),
			},
			Fixed:    []Booking{bk("F", []string{"t_1"}, 1, map[string]int{"t_1": 2}, 20, 0, 21, 0)},
			Closures: []Closure{{TableID: "t_2", From: instant(17, 0), To: instant(18, 0)}},
			Proposed: Closure{TableID: "t_1", From: instant(18, 0), To: instant(19, 0)},
		}, feasible: true},
		{req: Request{
			TableIDs: []string{"t_1", "t_2"},
			Pairs:    [][]string{{"t_1", "t_2"}},
			Considered: []Booking{
				bk("M", []string{"t_1"}, 2, map[string]int{"t_1": 5, "t_2": 5}, 18, 0, 20, 0),
				bk("N", []string{"t_2"}, 2, map[string]int{"t_1": 2, "t_2": 2}, 18, 30, 19, 30),
			},
			Fixed:    []Booking{bk("G", []string{"t_1", "t_2"}, 4, map[string]int{"t_1": 4, "t_2": 4}, 21, 0, 22, 0)},
			Closures: []Closure{{TableID: "t_2", From: instant(17, 0), To: instant(18, 0)}},
			Proposed: Closure{TableID: "t_9", From: instant(10, 0), To: instant(11, 0)},
		}, feasible: true},
		{req: Request{
			TableIDs: []string{"t_1", "t_2"},
			Pairs:    [][]string{{"t_1", "t_2"}},
			Considered: []Booking{
				bk("M", []string{"t_2"}, 3, map[string]int{"t_1": 5, "t_2": 5}, 18, 0, 20, 0),
				bk("N", []string{"t_1"}, 2, map[string]int{"t_1": 2, "t_2": 2}, 19, 0, 21, 0),
			},
			Fixed: []Booking{
				bk("G", []string{"t_1", "t_2"}, 4, map[string]int{"t_1": 4, "t_2": 4}, 21, 0, 22, 0),
			},
			Closures: []Closure{
				{TableID: "t_1", From: instant(19, 0), To: instant(19, 30)},
			},
			Proposed: Closure{TableID: "t_2", From: instant(18, 30), To: instant(19, 30)},
		}, feasible: false},
	}
	for wi, w := range worlds {
		got, gerr := Solve(w.req)
		want, found := oracleBest(w.req)
		if (gerr == nil) != w.feasible {
			t.Fatalf("world %d: Solve err=%v, expected feasible=%v", wi, gerr, w.feasible)
		}
		if found != w.feasible {
			t.Fatalf("world %d: oracle found=%v, expected feasible=%v", wi, found, w.feasible)
		}
		if !w.feasible {
			if gerr == nil || gerr.Code != CodeNoFeasible {
				t.Fatalf("world %d: want no_feasible_plan, got %v", wi, gerr)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("world %d mismatch:\ngot  %+v\nwant %+v", wi, got, want)
		}
		// A shuffled input permutation must yield the identical full plan.
		shuffled := w.req
		shuffled.Considered = append([]Booking(nil), w.req.Considered...)
		for i, j := 0, len(shuffled.Considered)-1; i < j; i, j = i+1, j-1 {
			shuffled.Considered[i], shuffled.Considered[j] = shuffled.Considered[j], shuffled.Considered[i]
		}
		again, aerr := Solve(shuffled)
		if aerr != nil {
			t.Fatalf("world %d shuffled: unexpected error %v", wi, aerr.Code)
		}
		if !reflect.DeepEqual(again, want) {
			t.Fatalf("world %d shuffled mismatch:\ngot  %+v\nwant %+v", wi, again, want)
		}
	}
}
