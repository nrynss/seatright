package service

// S4-M foundation tests: snapshot/map/array isolation, nil-vs-empty
// serialization, full preview public shape/privacy/detachment, empty old
// import normalization without dropping receipts, closure half-open/offset/
// cross-restaurant/member semantics. Names TestReplanState*/TestClosureBlocks*.

import (
	"encoding/json"
	"testing"
	"time"
)

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad instant %s: %v", s, err)
	}
	return ts
}

func TestReplanStateSnapshotIsolation(t *testing.T) {
	s := New()
	s.state.Plans["p1"] = Replan{
		ID: "p1", RestaurantID: "r1", RestaurantRevision: 3,
		Closure:     Closure{TableID: "t_1", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"},
		Assignments: []ReplanAssignment{{Reference: "A", TableIDs: []string{"t_2"}, Changed: true}},
		MovedCount:  1, UnusedSeats: 0,
	}
	s.state.Closures["r1"] = []Closure{{TableID: "t_1", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"}}
	snap := s.snapshot()
	snap.Plans["p1"] = Replan{ID: "evil"}
	snap.Closures["r1"][0].TableID = "evil"
	snap.Plans["p1"] = Replan{}
	delete(snap.Plans, "p1")
	if s.state.Plans["p1"].ID != "p1" {
		t.Fatal("snapshot aliases stored plan")
	}
	if s.state.Closures["r1"][0].TableID != "t_1" {
		t.Fatal("snapshot aliases stored closure")
	}
	// cloneState detachment: mutating a clone's assignment tables leaves the source.
	src := State{Plans: map[string]Replan{"p1": {ID: "p1", Assignments: []ReplanAssignment{{Reference: "A", TableIDs: []string{"t_1"}}}}}}
	cl := cloneState(&src)
	mut := cl.Plans["p1"]
	mut.Assignments[0].TableIDs[0] = "mut"
	if src.Plans["p1"].Assignments[0].TableIDs[0] != "t_1" {
		t.Fatal("cloneState aliases assignment tables")
	}
}

func TestReplanStateNilVsEmptySerialization(t *testing.T) {
	// Nil maps serialize as null; empty maps as {}. Both must survive a
	// JSON round trip without changing shape.
	var nilPlans map[string]Replan
	var nilClosures map[string][]Closure
	raw, _ := json.Marshal(map[string]any{"plans": nilPlans, "closures": nilClosures})
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back["plans"] != nil || back["closures"] != nil {
		t.Fatalf("nil maps = %v", back)
	}
	st := emptyState()
	if st.Plans == nil || st.Closures == nil {
		t.Fatal("emptyState must allocate plans/closures")
	}
	raw, _ = json.Marshal(st)
	var back2 map[string]any
	if err := json.Unmarshal(raw, &back2); err != nil {
		t.Fatal(err)
	}
	if _, ok := back2["plans"].(map[string]any); !ok {
		t.Fatalf("empty plans = %v", back2["plans"])
	}
	// Table of NIL / allocated-EMPTY / POPULATED shapes through the real
	// clone functions, with JSON equality before/after plus actual nil checks,
	// then nested-mutation detachment against a pristine capture.
	type replanCase struct {
		name string
		in   map[string]Replan
	}
	replanCases := []replanCase{
		{"nil", nil},
		{"empty", map[string]Replan{}},
		{"populated", map[string]Replan{"p1": {
			ID: "p1", RestaurantID: "r1", RestaurantRevision: 2, Applied: true,
			Closure:     Closure{TableID: "t_1", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"},
			Assignments: []ReplanAssignment{{Reference: "A", TableIDs: []string{"t_1", "t_2"}, Changed: true}},
			MovedCount:  1, UnusedSeats: 0,
		}}},
		{"nil-assignments", map[string]Replan{"p2": {ID: "p2"}}},
		{"empty-assignments", map[string]Replan{"p3": {ID: "p3", Assignments: []ReplanAssignment{}}}},
		{"nil-tableids", map[string]Replan{"p4": {ID: "p4", Assignments: []ReplanAssignment{{Reference: "A"}}}}},
		{"empty-tableids", map[string]Replan{"p5": {ID: "p5", Assignments: []ReplanAssignment{{Reference: "A", TableIDs: []string{}}}}}},
	}
	for _, tc := range replanCases {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := json.Marshal(tc.in)
			got := cloneReplanMap(tc.in)
			after, _ := json.Marshal(got)
			if string(before) != string(after) {
				t.Fatalf("JSON changed: %s -> %s", before, after)
			}
			if tc.in == nil && got != nil {
				t.Fatal("nil map not preserved")
			}
			if tc.in != nil && got == nil {
				t.Fatal("non-nil map became nil")
			}
		})
	}
	// Single-plan cloneReplan nil-vs-empty assignment slices.
	if got := cloneReplan(Replan{ID: "x"}); got.Assignments != nil {
		t.Fatal("nil assignments not preserved")
	}
	if got := cloneReplan(Replan{Assignments: []ReplanAssignment{}}); got.Assignments == nil {
		t.Fatal("empty assignments not preserved")
	}
	// Closure map shapes through cloneClosureMap and cloneState.
	closureCases := []struct {
		name string
		in   map[string][]Closure
	}{
		{"nil", nil},
		{"empty", map[string][]Closure{}},
		{"nil-slice", map[string][]Closure{"r1": nil}},
		{"empty-slice", map[string][]Closure{"r1": {}}},
		{"populated", map[string][]Closure{"r1": {{TableID: "t_1", From: "a", To: "b"}}}},
	}
	for _, tc := range closureCases {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := json.Marshal(tc.in)
			got := cloneClosureMap(tc.in)
			after, _ := json.Marshal(got)
			if string(before) != string(after) {
				t.Fatalf("JSON changed: %s -> %s", before, after)
			}
			if tc.in == nil && got != nil {
				t.Fatal("nil closure map not preserved")
			}
		})
	}
	// cloneState over a populated state: pristine capture, nested mutation of
	// the CLONE's maps/slices/arrays, source complete JSON unchanged.
	srcState := State{
		Plans:    map[string]Replan{"p1": {ID: "p1", Assignments: []ReplanAssignment{{Reference: "A", TableIDs: []string{"t_1"}}}}},
		Closures: map[string][]Closure{"r1": {{TableID: "t_1", From: "a", To: "b"}}},
	}
	pristine, _ := json.Marshal(srcState)
	cl := cloneState(&srcState)
	mut := cl.Plans["p1"]
	mut.Assignments[0].TableIDs[0] = "evil"
	mut.Assignments[0].Reference = "evil"
	cl.Closures["r1"][0].TableID = "evil"
	cl.Plans["p9"] = Replan{ID: "p9"}
	delete(cl.Closures, "r1")
	again, _ := json.Marshal(srcState)
	if string(pristine) != string(again) {
		t.Fatalf("source changed by clone mutation:\n%s\n%s", pristine, again)
	}
}

func TestReplanStatePreviewPublicShape(t *testing.T) {
	p := Replan{
		ID: "plan9", RestaurantID: "r1", RestaurantRevision: 4, Applied: true,
		Closure:     Closure{TableID: "t_2", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"},
		Assignments: []ReplanAssignment{{Reference: "B", TableIDs: []string{"t_1"}, Changed: false}, {Reference: "A", TableIDs: []string{"t_1", "t_2"}, Changed: true}},
		MovedCount:  1, UnusedSeats: 2,
	}
	got := replanPublic(p)
	// Pristine capture BEFORE any mutation: the complete expected preview.
	pristine, _ := json.Marshal(got)
	want := `{"assignments":[{"changed":false,"reference":"B","table_ids":["t_1"]},` +
		`{"changed":true,"reference":"A","table_ids":["t_1","t_2"]}],` +
		`"closure":{"from":"2026-09-28T18:00:00+02:00","table_id":"t_2","to":"2026-09-28T23:00:00+02:00"},` +
		`"moved_count":1,"plan_id":"plan9","restaurant_revision":4,"unused_seats":2}`
	var wantV, gotV any
	if err := json.Unmarshal([]byte(want), &wantV); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(pristine, &gotV); err != nil {
		t.Fatal(err)
	}
	wantN, _ := json.Marshal(wantV)
	gotN, _ := json.Marshal(gotV)
	if string(wantN) != string(gotN) {
		t.Fatalf("preview = %s, want %s", gotN, wantN)
	}
	var back map[string]any
	if err := json.Unmarshal(pristine, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 6 {
		t.Fatalf("top-level keys = %d, want exactly 6", len(back))
	}
	cl := back["closure"].(map[string]any)
	if len(cl) != 3 {
		t.Fatalf("closure keys = %d, want exactly 3", len(cl))
	}
	as := back["assignments"].([]any)
	if len(as) != 2 {
		t.Fatalf("assignments = %d", len(as))
	}
	for _, a := range as {
		if len(a.(map[string]any)) != 3 {
			t.Fatalf("assignment keys = %v", a)
		}
	}
	a1 := as[1].(map[string]any)
	if a1["reference"] != "A" || a1["changed"] != true {
		t.Fatalf("assignment 1 = %v", a1)
	}
	if ids := a1["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("assignment ids = %v", ids)
	}
	// Detachment BOTH directions on the ORIGINAL map values (not JSON-decoded
	// copies): mutate got's nested []string table_ids and closure map, then
	// compare the plan's pristine JSON and the plan itself.
	gotAssign := got["assignments"].([]any)
	gotAssign[0].(map[string]any)["table_ids"].([]string)[0] = "evil"
	got["closure"].(map[string]any)["table_id"] = "evil"
	if p.Assignments[0].TableIDs[0] != "t_1" {
		t.Fatal("public output aliases stored plan tables")
	}
	if p.Closure.TableID != "t_2" {
		t.Fatal("public output aliases stored plan closure")
	}
	// Conversely: pin the COMPLETE original plan JSON, obtain a FRESH live
	// public map from the SAME p, capture its pristine JSON, then mutate p and
	// re-serialize the SAME live map — it must still equal its pristine capture.
	planBefore, _ := json.Marshal(p)
	live := replanPublic(p)
	livePristine, _ := json.Marshal(live)
	p.Assignments[1].TableIDs[1] = "evil"
	p.Closure.From = "evil"
	again, _ := json.Marshal(live)
	if string(again) != string(livePristine) {
		t.Fatal("plan mutation leaked into live public map")
	}
	// And the plan itself really did change where mutated (guards vacuity),
	// while the earlier pristine public bytes still equal want.
	var planAfter map[string]any
	if err := json.Unmarshal(planBefore, &planAfter); err != nil {
		t.Fatal(err)
	}
	if p.Assignments[1].TableIDs[1] != "evil" || p.Closure.From != "evil" {
		t.Fatal("plan mutation did not apply")
	}
	if string(pristine) != string(wantN) {
		t.Fatal("saved pristine public JSON changed")
	}
	// Empty assignments render allocated [], never null.
	empty := replanPublic(Replan{ID: "e"})
	rawEmpty, _ := json.Marshal(empty)
	var backEmpty map[string]any
	if err := json.Unmarshal(rawEmpty, &backEmpty); err != nil {
		t.Fatal(err)
	}
	if as, ok := backEmpty["assignments"].([]any); !ok || len(as) != 0 {
		t.Fatalf("empty assignments = %v", backEmpty["assignments"])
	}
}

func TestReplanStateEmptyOldImport(t *testing.T) {
	// An old export without plans/closures normalizes to empty maps without
	// dropping modern receipts, histories or terms.
	s := New()
	legacy := map[string]any{
		"track": "tablekeeper", "format_version": float64(1),
		"state": map[string]any{
			"users": map[string]any{"u1": map[string]any{
				"id": "u1", "email": "a@b", "display_name": "A",
				"password_hash": mustTestHash(t, "password1"),
			}},
			"tokens":               map[string]any{"tok-old": "u1"},
			"restaurants":          []any{},
			"reservations":         map[string]any{},
			"receipts":             map[string]any{"u1\u0000POST\u0000/reservations\u0000k1": map[string]any{"user_id": "u1", "method": "POST", "path": "/reservations", "key": "k1", "body": "{}", "response": "{}", "status": float64(201)}},
			"histories":            map[string]any{},
			"series":               map[string]any{},
			"restaurant_revisions": map[string]any{},
		},
	}
	raw, _ := json.Marshal(legacy)
	if rec := serveRequest(s, "POST", "/_test/import", raw, nil); rec.Code != 204 {
		t.Fatalf("legacy import: %d %q", rec.Code, rec.Body.String())
	}
	// Pin whole original values BEFORE the import for byte-identical comparison.
	wantUser := map[string]any{"id": "u1", "email": "a@b", "display_name": "A",
		"password_hash": legacy["state"].(map[string]any)["users"].(map[string]any)["u1"].(map[string]any)["password_hash"]}
	wantReceiptKey := "u1\u0000POST\u0000/reservations\u0000k1"
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Plans == nil || s.state.Closures == nil {
		t.Fatal("plans/closures not normalized")
	}
	if len(s.state.Plans) != 0 || len(s.state.Closures) != 0 {
		t.Fatal("plans/closures must be empty, not fabricated")
	}
	// Whole token/user records byte-identical, not just counts.
	if s.state.Tokens["tok-old"] != "u1" || len(s.state.Tokens) != 1 {
		t.Fatalf("tokens = %v", s.state.Tokens)
	}
	gotUser := s.state.Users["u1"]
	rawUser, _ := json.Marshal(map[string]any{"id": gotUser.ID, "email": gotUser.Email,
		"display_name": gotUser.DisplayName, "password_hash": gotUser.PasswordHash})
	wantUserRaw, _ := json.Marshal(wantUser)
	if string(rawUser) != string(wantUserRaw) {
		t.Fatalf("user = %s, want %s", rawUser, wantUserRaw)
	}
	if len(s.state.Users) != 1 {
		t.Fatalf("users = %d", len(s.state.Users))
	}
	// Whole original receipt byte-identical (all scope fields + body/response).
	gotRc, ok := s.state.Receipts[wantReceiptKey]
	if !ok {
		t.Fatalf("receipt keys = %v", keysOf(s.state.Receipts))
	}
	if gotRc.UserID != "u1" || gotRc.Method != "POST" || gotRc.Path != "/reservations" ||
		gotRc.Key != "k1" || gotRc.Body != "{}" || gotRc.Response != "{}" || gotRc.Status != 201 {
		t.Fatalf("receipt = %+v", gotRc)
	}
	// NOTE (honest boundary): this synthetic absent-new-fields fixture proves
	// storage retention of one canonical receipt, not a genuine donor export or
	// a replay-proven endpoint receipt. Genuine replay proof belongs to S4-I/S4-P2.
}

func keysOf(m map[string]Receipt) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestClosureBlocksHalfOpen(t *testing.T) {
	s := New()
	s.state.Closures["r1"] = []Closure{
		{TableID: "t_1", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"},
	}
	at := func(s string) time.Time { return mustParseTime(t, s) }
	// Strict overlap blocks.
	if !closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T19:00:00+02:00"), at("2026-09-28T20:00:00+02:00")) {
		t.Fatal("overlap must block")
	}
	// Adjacency is free on both ends: [18:00,23:00) vs ending 18:00 / starting 23:00.
	if closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T17:00:00+02:00"), at("2026-09-28T18:00:00+02:00")) {
		t.Fatal("end-adjacent must be free")
	}
	if closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T23:00:00+02:00"), at("2026-09-28T23:30:00+02:00")) {
		t.Fatal("start-adjacent must be free")
	}
	// Other tables and other restaurants are harmless.
	if closureBlocks(&s.state, "r1", []string{"t_2"}, at("2026-09-28T19:00:00+02:00"), at("2026-09-28T20:00:00+02:00")) {
		t.Fatal("other table must be free")
	}
	if closureBlocks(&s.state, "r2", []string{"t_1"}, at("2026-09-28T19:00:00+02:00"), at("2026-09-28T20:00:00+02:00")) {
		t.Fatal("other restaurant must be free")
	}
	// Any pair member blocks the set.
	if !closureBlocks(&s.state, "r1", []string{"t_2", "t_1"}, at("2026-09-28T19:00:00+02:00"), at("2026-09-28T20:00:00+02:00")) {
		t.Fatal("pair member must block")
	}
}

func TestClosureBlocksOffsetsAbsolute(t *testing.T) {
	s := New()
	// Same absolute instant in different offsets: closure +02:00 18:00-23:00
	// blocks a query expressed in +00:00 (16:00-21:00Z overlaps).
	s.state.Closures["r1"] = []Closure{
		{TableID: "t_1", From: "2026-09-28T18:00:00+02:00", To: "2026-09-28T23:00:00+02:00"},
	}
	at := func(s string) time.Time { return mustParseTime(t, s) }
	if !closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T16:30:00+00:00"), at("2026-09-28T17:00:00+00:00")) {
		t.Fatal("absolute overlap across offsets must block")
	}
	if closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T21:00:00+00:00"), at("2026-09-28T22:00:00+00:00")) {
		t.Fatal("absolute disjoint across offsets must be free")
	}
	// Unparseable stored closures never block (validation is later I scope).
	s.state.Closures["r1"] = []Closure{{TableID: "t_1", From: "not-a-time", To: "also-bad"}}
	if closureBlocks(&s.state, "r1", []string{"t_1"}, at("2026-09-28T19:00:00+02:00"), at("2026-09-28T20:00:00+02:00")) {
		t.Fatal("unparseable closure must not block")
	}
}
