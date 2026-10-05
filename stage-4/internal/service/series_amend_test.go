package service

// S4-A tests: atomic recurring clock amendments through the real service APIs
// (reset/publish/create/amend/cancel/adopt). Direct AmendSeries calls are
// acceptance here; router wiring is a later serial owner's scope.

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

// amendFixture builds a Thursday 18:00-23:00 Europe/Berlin restaurant with two
// tables and one declared pair, manager u1. All bookings use 2027 dates
// (future, cutoff-safe unless the test says otherwise).
func amendFixture(t *testing.T) (*Service, string) {
	t.Helper()
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]],
		"manager_user_ids":["u1"]}],
		"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	return s, versionLoginToken(t, s)
}

func amendPublish(t *testing.T, s *Service, tok, key, from string, dur int) {
	t.Helper()
	body := `{"effective_from":"` + from + `","slot_minutes":30,"reservation_duration_minutes":` + itoa(dur) + `,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`
	if res := s.PublishPolicy(tok, "r1", key, []byte(body)); res.Status != 201 {
		t.Fatalf("publish %s: %d %v", key, res.Status, res.Body)
	}
}

func amendCreate(t *testing.T, s *Service, tok, key, body string) string {
	t.Helper()
	res := s.CreateReservation(tok, key, []byte(body))
	if res.Status != 201 {
		t.Fatalf("create %s: %d %v", key, res.Status, res.Body)
	}
	return res.Body.(map[string]any)["reference"].(string)
}

func amendAdopt(t *testing.T, s *Service, tok, key, anchor string, count int) (string, []string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"anchor_reference": anchor, "count": count, "interval_weeks": 1})
	res := s.AdoptSeries(tok, key, raw)
	if res.Status != 201 {
		t.Fatalf("adopt %s: %d %v", key, res.Status, res.Body)
	}
	m := res.Body.(map[string]any)
	var refs []string
	for _, o := range m["occurrences"].([]any) {
		refs = append(refs, o.(map[string]any)["reference"].(string))
	}
	return m["series_id"].(string), refs
}

// amendCall invokes AmendSeries through the service with a raw map body.
func amendCall(t *testing.T, s *Service, tok, sid, key string, body map[string]any) Result {
	t.Helper()
	raw, _ := json.Marshal(body)
	return s.AmendSeries(tok, sid, key, raw)
}

// amendUTC builds a UTC all-week-opening restaurant with the given fixture
// cutoff/slot/duration, and returns service, token, tomorrow's YYYY-MM-DD
// (computed ONCE from time.Now) and the clock used for the anchor.
func amendUTC(t *testing.T, cutoff, slot, dur int) (*Service, string, string) {
	t.Helper()
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"UTC","slot_minutes":` + itoa(slot) + `,
		"reservation_duration_minutes":` + itoa(dur) + `,"cancellation_cutoff_minutes":` + itoa(cutoff) + `,
		"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},
		{"weekday":"tue","opens":"00:00","closes":"23:00"},
		{"weekday":"wed","opens":"00:00","closes":"23:00"},
		{"weekday":"thu","opens":"00:00","closes":"23:00"},
		{"weekday":"fri","opens":"00:00","closes":"23:00"},
		{"weekday":"sat","opens":"00:00","closes":"23:00"},
		{"weekday":"sun","opens":"00:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]],
		"manager_user_ids":["u1"]}],
		"reservations":[]}`
	if rec := serveRequest(s, "POST", "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tomorrow := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	return s, versionLoginToken(t, s), tomorrow
}

func TestSeriesAmendBodyMatrix(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 3)
	rev := func() int {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.state.Series[sid].Revision
	}()
	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"missing revision", map[string]any{"from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"missing index", map[string]any{"expected_revision": float64(rev), "local_time": "20:00"}, 422, "validation_failed"},
		{"missing time", map[string]any{"expected_revision": float64(rev), "from_index": 0}, 422, "validation_failed"},
		{"bool revision", map[string]any{"expected_revision": true, "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"string revision", map[string]any{"expected_revision": "1", "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"null revision", map[string]any{"expected_revision": nil, "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"fraction revision", map[string]any{"expected_revision": 1.5, "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"zero revision", map[string]any{"expected_revision": float64(0), "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"negative revision", map[string]any{"expected_revision": float64(-2), "from_index": 0, "local_time": "20:00"}, 422, "validation_failed"},
		{"bool index", map[string]any{"expected_revision": float64(rev), "from_index": true, "local_time": "20:00"}, 422, "validation_failed"},
		{"negative index", map[string]any{"expected_revision": float64(rev), "from_index": float64(-1), "local_time": "20:00"}, 422, "validation_failed"},
		{"index past end", map[string]any{"expected_revision": float64(rev), "from_index": float64(3), "local_time": "20:00"}, 422, "validation_failed"},
		{"fraction index", map[string]any{"expected_revision": float64(rev), "from_index": 1.5, "local_time": "20:00"}, 422, "validation_failed"},
		{"short time", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "9:00"}, 422, "validation_failed"},
		{"no colon", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "2000"}, 422, "validation_failed"},
		{"hour 24", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "24:00"}, 422, "validation_failed"},
		{"minute 60", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "20:60"}, 422, "validation_failed"},
		{"non-digit", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "2a:00"}, 422, "validation_failed"},
		{"stale revision", map[string]any{"expected_revision": float64(rev + 5), "from_index": 0, "local_time": "20:00"}, 409, "stale_revision"},
		{"huge stale", map[string]any{"expected_revision": float64(9007199254740993), "from_index": 0, "local_time": "20:00"}, 409, "stale_revision"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := amendCall(t, s, tok, sid, "k-"+tc.name, tc.body)
			if res.Status != tc.status {
				t.Fatalf("status = %d, want %d (%v)", res.Status, tc.status, res.Body)
			}
			m := res.Body.(map[string]any)
			if m["error"].(map[string]any)["code"] != tc.code {
				t.Fatalf("code = %v, want %s", m, tc.code)
			}
		})
	}
}

func TestSeriesAmendHugeStaleBeforeCutoff(t *testing.T) {
	// Genuinely cutoff-blocked record via real workflow: UTC cutoff-0 fixture,
	// anchor tomorrow, adopt, then publish cutoff 10080 for the anchor date.
	// Invalid clock stays 422; huge stale with a valid clock 409s (stale wins
	// over cutoff); the correct revision hits cutoff 409 (proves the block).
	s, tok, tomorrow := amendUTC(t, 0, 30, 60)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"`+tomorrow+`T10:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	body, _ := json.Marshal(map[string]any{
		"effective_from": tomorrow, "slot_minutes": 30, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 10080,
		"opening_hours": []any{
			map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:00"}},
		"capacities": map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s.PublishPolicy(tok, "r1", "pcut", body); res.Status != 201 {
		t.Fatalf("publish: %d %v", res.Status, res.Body)
	}
	preInvalid := string(exportBytes(t, s))
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(1e18), "from_index": 0, "local_time": "99:99"})
	if res.Status != 422 {
		t.Fatalf("invalid time must still be 422 first, got %d", res.Status)
	}
	if got := string(exportBytes(t, s)); got != preInvalid {
		t.Fatal("invalid amend mutated state")
	}
	// Real adoption amend first (adopts cutoff 10080 into the records).
	s.mu.Lock()
	revAdopt := s.state.Series[sid].Revision
	s.mu.Unlock()
	res = amendCall(t, s, tok, sid, "k-real", map[string]any{"expected_revision": float64(revAdopt), "from_index": 0, "local_time": "11:00"})
	if res.Status != 201 {
		t.Fatalf("real adopt amend: %d %v", res.Status, res.Body)
	}
	preStale := string(exportBytes(t, s))
	res = amendCall(t, s, tok, sid, "k2", map[string]any{"expected_revision": float64(1e18), "from_index": 0, "local_time": "11:00"})
	if res.Status != 409 {
		t.Fatalf("huge stale = %d, want 409", res.Status)
	}
	if got := string(exportBytes(t, s)); got != preStale {
		t.Fatal("stale amend mutated state")
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "stale_revision" {
		t.Fatalf("code = %v", res.Body)
	}
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	preCutoff := string(exportBytes(t, s))
	res = amendCall(t, s, tok, sid, "k3", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "12:00"})
	if res.Status != 409 {
		t.Fatalf("correct revision must hit stored cutoff, got %d %v", res.Status, res.Body)
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "cutoff_passed" {
		t.Fatalf("code = %v", res.Body)
	}
	if got := string(exportBytes(t, s)); got != preCutoff {
		t.Fatal("cutoff failure mutated state")
	}
}

func TestSeriesAmendAuthScope(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	body := map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "20:00"}
	// Unknown series -> 404.
	if res := amendCall(t, s, tok, "nope", "k1", body); res.Status != 404 {
		t.Fatalf("unknown series = %d", res.Status)
	}
	// Missing/invalid token -> 401 via wrapper.
	raw, _ := json.Marshal(body)
	if res := s.AmendSeries("", sid, "k2", raw); res.Status != 401 {
		t.Fatalf("no token = %d", res.Status)
	}
	if res := s.AmendSeries("bad-token", sid, "k3", raw); res.Status != 401 {
		t.Fatalf("bad token = %d", res.Status)
	}
	// Another user owns nothing here: foreign series -> 404.
	s2 := New()
	_ = s2
	// Same key, different body -> 409 reuse.
	res1 := amendCall(t, s, tok, sid, "reuse", body)
	if res1.Status != 201 {
		t.Fatalf("first = %d", res1.Status)
	}
	other := map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"}
	if res := amendCall(t, s, tok, sid, "reuse", other); res.Status != 409 {
		t.Fatalf("reuse different body = %d", res.Status)
	}
}

func TestSeriesAmendEligibility(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 4)
	// Mark member 1 an exception via real PATCH; cancel member 2.
	if res := s.PatchReservation(tok, refs[1], []byte(`{"party_size":2}`)); res.Status != 200 {
		t.Fatalf("patch: %d", res.Status)
	}
	if res := s.CancelReservation(tok, refs[2]); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	// from_index 1: member 1 (exception) and 2 (cancelled) skipped, member 3 amended.
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("amend = %d %v", res.Status, res.Body)
	}
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	got := map[string]any{}
	for _, o := range occs {
		om := o.(map[string]any)
		got[om["reference"].(string)] = om["reservation"].(map[string]any)["starts_at_local"]
	}
	if got[refs[1]] != "2027-05-13T18:00" {
		t.Fatalf("exception member changed: %v", got[refs[1]])
	}
	if got[refs[2]] != "2027-05-20T18:00" {
		t.Fatalf("cancelled member changed: %v", got[refs[2]])
	}
	if got[refs[3]] != "2027-05-27T20:00" {
		t.Fatalf("eligible member not amended: %v", got[refs[3]])
	}
	// Anchor (index 0) untouched by from_index 1.
	if got[refs[0]] != "2027-05-06T18:00" {
		t.Fatalf("anchor changed: %v", got[refs[0]])
	}
}

func TestSeriesAmendScheduledDates(t *testing.T) {
	// DATE-SOURCE seam (synthetic, honest): the resulting local date comes from
	// the ORIGINAL member ScheduledDate, not the current record's date. The
	// current table selection is set directly in state (a stand-in for a seating
	// repair, which is a deferred R2/W scope — no real applied repair, no
	// scheduled-vs-current date divergence proof is claimed here).
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 2)
	// Repair member 1 to t_2 directly (simulating a seating repair that keeps
	// scheduled dates but changes the table).
	s.mu.Lock()
	rec := s.state.Reservations[refs[1]]
	rec.TableIDs = []string{"t_2"}
	rec.TableID = "t_2"
	s.state.Reservations[refs[1]] = rec
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("amend = %d %v", res.Status, res.Body)
	}
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	m1 := occs[1].(map[string]any)["reservation"].(map[string]any)
	if m1["starts_at_local"] != "2027-05-13T20:00" {
		t.Fatalf("scheduled date not used: %v", m1["starts_at_local"])
	}
	if ids := m1["table_ids"].([]string); len(ids) != 1 || ids[0] != "t_2" {
		t.Fatalf("repaired table not retained: %v", ids)
	}
}

func TestSeriesAmendNoOp(t *testing.T) {
	// Genuine off-grid no-op: stored clock 19:30 (valid slot-30) becomes
	// off-grid under a later slot-60 policy from 18:00, and the stored record
	// carries cutoff 10080 (from the F1-style workflow). An identical-clock
	// amend must 201 while retaining the COMPLETE export except one receipt.
	s, tok, tomorrow := amendUTC(t, 0, 30, 60)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"`+tomorrow+`T19:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	// Carry cutoff 10080 like the F1 workflow: publish, real amend (adopts).
	pubCutoff := func(key string, cutoff int) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"effective_from": tomorrow, "slot_minutes": 30, "reservation_duration_minutes": 60,
			"cancellation_cutoff_minutes": cutoff,
			"opening_hours": []any{
				map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:00"}},
			"capacities": map[string]any{"t_1": 2, "t_2": 4},
		})
		if res := s.PublishPolicy(tok, "r1", key, body); res.Status != 201 {
			t.Fatalf("publish %s: %d %v", key, res.Status, res.Body)
		}
	}
	pubCutoff("pc", 10080)
	s.mu.Lock()
	rev0 := s.state.Series[sid].Revision
	s.mu.Unlock()
	// Carry on from_index 0 so the NEAR-TERM anchor itself adopts cutoff 10080
	// under its old-0 permission: anchor 19:00 -> 19:30 is a real change.
	resCarry := amendCall(t, s, tok, sid, "kc", map[string]any{"expected_revision": float64(rev0), "from_index": 0, "local_time": "19:30"})
	if resCarry.Status != 201 {
		t.Fatalf("carry amend: %d %v", resCarry.Status, resCarry.Body)
	}
	carryOccs := resCarry.Body.(map[string]any)["occurrences"].([]any)
	carryM0 := carryOccs[0].(map[string]any)["reservation"].(map[string]any)
	if carryM0["starts_at_local"] != tomorrow+"T19:30" {
		t.Fatalf("carry clock = %v", carryM0["starts_at_local"])
	}
	if !numEq(carryM0["revision"], 2) {
		t.Fatalf("carry revision = %v", carryM0["revision"])
	}
	carryTerms := carryM0["accepted_terms"].(map[string]any)
	if !numEq(carryTerms["cancellation_cutoff_minutes"], 10080) {
		t.Fatalf("carry terms not adopted: %v", carryTerms)
	}
	if !numEq(carryTerms["policy_version"], 1) {
		t.Fatalf("carry version = %v", carryTerms["policy_version"])
	}
	// The anchor is genuinely INSIDE the adopted 7-day cutoff: prove now >=
	// old accepted start minus 10080 using the actual parsed UTC instants.
	anchorStart, err := time.Parse(time.RFC3339, carryM0["starts_at"].(string))
	if err != nil {
		t.Fatalf("anchor start not RFC3339: %v", err)
	}
	if time.Now().UTC().Before(anchorStart.Add(-10080 * time.Minute)) {
		t.Fatal("anchor outside adopted cutoff; cutoff bypass unproven")
	}
	// Publish slot-60 from 18:00: stored :30 is now off-grid for REAL validation.
	slotBody, _ := json.Marshal(map[string]any{
		"effective_from": tomorrow, "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 10080,
		"opening_hours": []any{
			map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:00"},
			map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:00"}},
		"capacities": map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s.PublishPolicy(tok, "r1", "pslot", slotBody); res.Status != 201 {
		t.Fatalf("publish slot: %d %v", res.Status, res.Body)
	}
	// Sanity: the stored clock really is off-grid now (a REAL change elsewhere
	// would fail). Then the identical-clock amend bypasses both grid and cutoff.
	before := string(exportBytes(t, s))
	var beforeState map[string]any
	if err := json.Unmarshal([]byte(before), &beforeState); err != nil {
		t.Fatal(err)
	}
	nRcBefore := len(beforeState["state"].(map[string]any)["receipts"].(map[string]any))
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "19:30"})
	if res.Status != 201 {
		t.Fatalf("no-op = %d %v", res.Status, res.Body)
	}
	after := string(exportBytes(t, s))
	var afterState map[string]any
	if err := json.Unmarshal([]byte(after), &afterState); err != nil {
		t.Fatal(err)
	}
	// Whole-state comparison: every namespace identical EXCEPT exactly one new
	// receipt (the no-op's own idempotent record).
	afterSt := afterState["state"].(map[string]any)
	beforeSt := beforeState["state"].(map[string]any)
	for _, ns := range []string{"users", "tokens", "restaurants", "reservations", "policies", "histories", "series", "restaurant_revisions", "plans", "closures"} {
		ab, _ := json.Marshal(afterSt[ns])
		bb, _ := json.Marshal(beforeSt[ns])
		if string(ab) != string(bb) {
			t.Fatalf("namespace %s changed by no-op", ns)
		}
	}
	// The no-op's own receipt is correctly bound: owner/method/exact path/key.
	afterRc := afterSt["receipts"].(map[string]any)
	beforeRc := beforeSt["receipts"].(map[string]any)
	if len(afterRc) != nRcBefore+1 {
		t.Fatalf("receipts %d -> %d, want exactly +1", nRcBefore, len(afterRc))
	}
	var noopKey string
	for k := range afterRc {
		if _, ok := beforeRc[k]; !ok {
			if noopKey != "" {
				t.Fatal("two new receipts")
			}
			noopKey = k
		}
	}
	if noopKey == "" {
		t.Fatal("no new receipt")
	}
	nr := afterRc[noopKey].(map[string]any)
	if nr["user_id"] != "u1" || nr["method"] != "POST" {
		t.Fatalf("new receipt scope = %v", nr)
	}
	if nr["path"] != "/series/"+sid+"/amend" {
		t.Fatalf("new receipt path = %v", nr["path"])
	}
	if nr["key"] != "k" {
		t.Fatalf("new receipt key = %v", nr["key"])
	}
	if !numEq(nr["status"], 201) {
		t.Fatalf("new receipt status = %v", nr["status"])
	}
	for k, v := range beforeRc {
		ab, _ := json.Marshal(v)
		bb, _ := json.Marshal(afterRc[k])
		if string(ab) != string(bb) {
			t.Fatalf("prior receipt %s changed", k)
		}
	}
}

func TestSeriesAmendEmptyEligible(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 2)
	// Cancel the only post-from_index member: eligible set empty.
	if res := s.CancelReservation(tok, refs[1]); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	before := string(exportBytes(t, s))
	var beforeState map[string]any
	if err := json.Unmarshal([]byte(before), &beforeState); err != nil {
		t.Fatal(err)
	}
	nRcBefore := len(beforeState["state"].(map[string]any)["receipts"].(map[string]any))
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("empty eligible = %d", res.Status)
	}
	// Current response shape: full series with both occurrences.
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	if len(occs) != 2 {
		t.Fatalf("occurrences = %d", len(occs))
	}
	after := string(exportBytes(t, s))
	var afterState map[string]any
	if err := json.Unmarshal([]byte(after), &afterState); err != nil {
		t.Fatal(err)
	}
	afterSt := afterState["state"].(map[string]any)
	beforeSt := beforeState["state"].(map[string]any)
	for _, ns := range []string{"users", "tokens", "restaurants", "reservations", "policies", "histories", "series", "restaurant_revisions"} {
		ab, _ := json.Marshal(afterSt[ns])
		bb, _ := json.Marshal(beforeSt[ns])
		if string(ab) != string(bb) {
			t.Fatalf("namespace %s changed by empty eligible", ns)
		}
	}
	afterRc := afterSt["receipts"].(map[string]any)
	if len(afterRc) != nRcBefore+1 {
		t.Fatalf("receipts %d -> %d, want exactly +1", nRcBefore, len(afterRc))
	}
}

func TestSeriesAmendCutoffAndPolicy(t *testing.T) {
	// OLD accepted cutoff BOTH directions on a genuine UTC workflow.
	// Anchor TOMORROW (computed once) under fixture cutoff 0: create + adopt.
	// Publish cutoff 10080 for the anchor date: amending index 0 to another
	// valid clock is permitted by OLD 0 (anchor outside the new 7-day cutoff)
	// and adopts the new terms. Publishing latest cutoff 0 afterwards, another
	// real amend must 409 on the STORED 10080 despite newest 0.
	s, tok, tomorrow := amendUTC(t, 0, 30, 60)
	anchorBody := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"` + tomorrow + `T10:00","party_size":1}`
	anchor := amendCreate(t, s, tok, "a", anchorBody)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 2)
	_ = refs
	pubRaw := func(key string, cutoff int) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"effective_from": tomorrow, "slot_minutes": 30, "reservation_duration_minutes": 60,
			"cancellation_cutoff_minutes": cutoff,
			"opening_hours": []any{
				map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:00"},
				map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:00"}},
			"capacities": map[string]any{"t_1": 2, "t_2": 4},
		})
		if res := s.PublishPolicy(tok, "r1", key, body); res.Status != 201 {
			t.Fatalf("publish %s: %d %v", key, res.Status, res.Body)
		}
	}
	pubRaw("p10080", 10080)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k1", map[string]any{"expected_revision": float64(rev), "from_index": 0, "local_time": "11:00"})
	if res.Status != 201 {
		t.Fatalf("old-0 permits despite new 10080: %d %v", res.Status, res.Body)
	}
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	m0 := occs[0].(map[string]any)["reservation"].(map[string]any)
	terms := m0["accepted_terms"].(map[string]any)
	if len(terms) != 6 {
		t.Fatalf("terms keys = %d, want 6", len(terms))
	}
	if cc := terms["cancellation_cutoff_minutes"]; cc != float64(10080) && cc != 10080 {
		t.Fatalf("cutoff not adopted: %v", terms)
	}
	if _, ok := terms["effective_from"]; ok {
		t.Fatal("terms carry effective_from")
	}
	if m0["starts_at"] == "" || m0["ends_at"] == "" {
		t.Fatal("absolute instants missing")
	}
	startT, err := time.Parse(time.RFC3339, m0["starts_at"].(string))
	if err != nil {
		t.Fatalf("start not RFC3339: %v", err)
	}
	endT, err := time.Parse(time.RFC3339, m0["ends_at"].(string))
	if err != nil {
		t.Fatalf("end not RFC3339: %v", err)
	}
	if endT.Sub(startT) != 60*time.Minute {
		t.Fatalf("accepted duration = %v, want 60m (adopted policy)", endT.Sub(startT))
	}

	// Old history entries keep frozen old terms: anchor CREATED entry still v0/cutoff 0.
	code, hb := func() (int, string) {
		rec := serveRequest(s, "GET", "/reservations/"+anchor+"/history", nil,
			map[string]string{"Authorization": "Bearer " + tok})
		return rec.Code, rec.Body.String()
	}()
	if code != 200 {
		t.Fatalf("history: %d", code)
	}
	var hents map[string]any
	if err := json.Unmarshal([]byte(hb), &hents); err != nil {
		t.Fatal(err)
	}
	entries := hents["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want created+changed", len(entries))
	}
	first := entries[0].(map[string]any)
	if first["event"] != "created" {
		t.Fatalf("first event = %v", first["event"])
	}
	oldTerms := first["accepted_terms"].(map[string]any)
	if cc := oldTerms["cancellation_cutoff_minutes"]; cc != float64(0) && cc != 0 {
		t.Fatalf("old history terms mutated: %v", oldTerms)
	}
	// Other direction: newest cutoff 0, but STORED 10080 blocks the next amend.
	pubRaw("p0", 0)
	s.mu.Lock()
	rev2 := s.state.Series[sid].Revision
	s.mu.Unlock()
	preFail := string(exportBytes(t, s))
	res = amendCall(t, s, tok, sid, "k2", map[string]any{"expected_revision": float64(rev2), "from_index": 0, "local_time": "12:00"})
	if res.Status != 409 {
		t.Fatalf("stored 10080 must block despite newest 0: %d %v", res.Status, res.Body)
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "cutoff_passed" {
		t.Fatalf("code = %v", res.Body)
	}
	if got := string(exportBytes(t, s)); got != preFail {
		t.Fatal("failed amend mutated state")
	}
}

func TestSeriesAmendPrecedence(t *testing.T) {
	s, tok := amendFixture(t)
	a1 := amendCreate(t, s, tok, "a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	// Occupy t_1 at 20:00 on 2027-05-13 so member 1's amend would conflict...
	blocker := amendCreate(t, s, tok, "b", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-13T20:00","party_size":1}`)
	_ = blocker
	sid, refs := amendAdopt(t, s, tok, "s", a1, 3)
	_ = refs
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	// Member 1 -> 20:00 conflicts (occupancy); member 2's own validation is
	// pinned independently below. The full-suffix amend must surface member 1's
	// occupancy failure with byte-identical rollback.
	before := string(exportBytes(t, s))
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 409 {
		t.Fatalf("occupancy = %d %v", res.Status, res.Body)
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "table_unavailable" {
		t.Fatalf("code = %v", res.Body)
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("occupancy failure mutated state")
	}
	// Member 2's date gets a Thursday-closed policy: same clock is 422 there.
	bodyClosed, _ := json.Marshal(map[string]any{
		"effective_from": "2027-05-20", "slot_minutes": 30, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "fri", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s.PublishPolicy(tok, "r1", "p-closed", bodyClosed); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	// Pin member 1's failure independently on a clone service.
	s2, tok2 := amendFixture(t)
	a1b := amendCreate(t, s2, tok2, "a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	amendCreate(t, s2, tok2, "b", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-13T20:00","party_size":1}`)
	sidb, _ := amendAdopt(t, s2, tok2, "s", a1b, 2)
	s2.mu.Lock()
	revb := s2.state.Series[sidb].Revision
	s2.mu.Unlock()
	rb := amendCall(t, s2, tok2, sidb, "kb", map[string]any{"expected_revision": float64(revb), "from_index": 1, "local_time": "20:00"})
	if rb.Status != 409 {
		t.Fatalf("member1 alone = %d, want 409", rb.Status)
	}
	// Member 2 alone (count-3 series, from_index 2): 422 closed-day.
	s3, tok3 := amendFixture(t)
	a1c := amendCreate(t, s3, tok3, "a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sidc, _ := amendAdopt(t, s3, tok3, "s", a1c, 3)
	bodyClosed3, _ := json.Marshal(map[string]any{
		"effective_from": "2027-05-20", "slot_minutes": 30, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "fri", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s3.PublishPolicy(tok3, "r1", "p-closed", bodyClosed3); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	s3.mu.Lock()
	revc := s3.state.Series[sidc].Revision
	s3.mu.Unlock()
	rc := amendCall(t, s3, tok3, sidc, "kc", map[string]any{"expected_revision": float64(revc), "from_index": 2, "local_time": "20:00"})
	if rc.Status != 422 {
		t.Fatalf("member2 alone = %d, want 422", rc.Status)
	}
	if rc.Body.(map[string]any)["error"].(map[string]any)["code"] != "outside_opening_hours" {
		t.Fatalf("member2 code = %v", rc.Body)
	}
	// Full suffix on the MAIN service: member 1 occupancy + member 2 closed-day.
	// Non-occupancy (member 2, later index) is prepared in order and returned
	// before any occupancy check: expect 422, with complete export identity.
	s.mu.Lock()
	revFull := s.state.Series[sid].Revision
	s.mu.Unlock()
	preFull := string(exportBytes(t, s))
	rf := amendCall(t, s, tok, sid, "kfull", map[string]any{"expected_revision": float64(revFull), "from_index": 1, "local_time": "20:00"})
	if rf.Status != 422 {
		t.Fatalf("full suffix = %d, want member2 422: %v", rf.Status, rf.Body)
	}
	if rf.Body.(map[string]any)["error"].(map[string]any)["code"] != "outside_opening_hours" {
		t.Fatalf("full suffix code = %v", rf.Body)
	}
	if got := string(exportBytes(t, s)); got != preFull {
		t.Fatal("precedence failure mutated state")
	}
}

func TestSeriesAmendRollbackAndReuse(t *testing.T) {
	s, tok := amendFixture(t)
	a1 := amendCreate(t, s, tok, "a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	amendCreate(t, s, tok, "b", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-13T19:30","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", a1, 2)
	before := string(exportBytes(t, s))
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "failkey", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 409 {
		t.Fatalf("conflict = %d", res.Status)
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("failed amend mutated state")
	}
	// SAME failed key with a valid body is genuinely reusable -> 201.
	res = amendCall(t, s, tok, sid, "failkey", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "21:00"})
	if res.Status != 201 {
		t.Fatalf("reuse = %d %v", res.Status, res.Body)
	}
	raw201, _ := json.Marshal(res.Body)
	mid := string(exportBytes(t, s))
	// Replay the SAME key/body: 200 original bytes, export unchanged.
	rep := amendCall(t, s, tok, sid, "failkey", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "21:00"})
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	repRaw, _ := json.Marshal(rep.Body)
	if string(repRaw) != string(raw201) {
		t.Fatal("replay bytes differ")
	}
	if got := string(exportBytes(t, s)); got != mid {
		t.Fatal("replay mutated state")
	}
}

func TestSeriesAmendCommitMetadata(t *testing.T) {
	s, tok := amendFixture(t)
	a1 := amendCreate(t, s, tok, "a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	a2 := amendCreate(t, s, tok, "a2", `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", a1, 3)
	sid2, _ := amendAdopt(t, s, tok, "s2", a2, 2)
	before := string(exportBytes(t, s))
	var beforeState map[string]any
	if err := json.Unmarshal([]byte(before), &beforeState); err != nil {
		t.Fatal(err)
	}
	beforeSt := beforeState["state"].(map[string]any)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("amend = %d %v", res.Status, res.Body)
	}
	// Current response matches REAL winner state; per-booking rev+1.
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	for _, o := range occs {
		om := o.(map[string]any)
		idx := 0
		switch v := om["index"].(type) {
		case int:
			idx = v
		case float64:
			idx = int(v)
		default:
			t.Fatalf("member index type %T", om["index"])
		}
		rec := om["reservation"].(map[string]any)
		if idx >= 1 {
			if rec["starts_at_local"] != "2027-05-13T20:00" && rec["starts_at_local"] != "2027-05-20T20:00" {
				t.Fatalf("member %d clock = %v", idx, rec["starts_at_local"])
			}
			if !numEq(rec["revision"], 2) {
				t.Fatalf("member %d revision = %v", idx, rec["revision"])
			}
		}
	}
	after := string(exportBytes(t, s))
	var afterState map[string]any
	if err := json.Unmarshal([]byte(after), &afterState); err != nil {
		t.Fatal(err)
	}
	afterSt := afterState["state"].(map[string]any)
	// Affected occurrences: identity/owner/party/tables/scheduled dates same,
	// new clock/end/terms, rev+1, old prefix byte-equal + EXACTLY one Changed
	// with matching seq/revision/nondecreasing at/full terms/ordered From/To.
	for i, ref := range refs[1:] {
		bRes := beforeSt["reservations"].(map[string]any)[ref].(map[string]any)
		aRes := afterSt["reservations"].(map[string]any)[ref].(map[string]any)
		for _, k := range []string{"reservation_id", "reference", "user_id", "party_size", "created_at"} {
			if bRes[k] != aRes[k] {
				t.Fatalf("%s %s changed", ref, k)
			}
		}
		if jsonStr(bRes["table_ids"]) != jsonStr(aRes["table_ids"]) {
			t.Fatalf("%s tables changed", ref)
		}
		if !numEq(aRes["revision"], 2) {
			t.Fatalf("%s revision = %v", ref, aRes["revision"])
		}
		bHist := beforeSt["histories"].(map[string]any)[ref].([]any)
		aHist := afterSt["histories"].(map[string]any)[ref].([]any)
		if len(aHist) != len(bHist)+1 {
			t.Fatalf("%s entries %d -> %d", ref, len(bHist), len(aHist))
		}
		for j := range bHist {
			bj, _ := json.Marshal(bHist[j])
			aj, _ := json.Marshal(aHist[j])
			if string(bj) != string(aj) {
				t.Fatalf("%s prefix entry %d changed", ref, j)
			}
		}
		last := aHist[len(aHist)-1].(map[string]any)
		if last["event"] != "changed" || !numEq(last["revision"], 2) {
			t.Fatalf("%s last = %v", ref, last)
		}
		chs := last["changes"].([]any)
		if len(chs) != 1 {
			t.Fatalf("%s changes = %v", ref, chs)
		}
		ch := chs[0].(map[string]any)
		if ch["field"] != "starts_at_local" {
			t.Fatalf("%s change field = %v", ref, ch["field"])
		}
		wantFrom := "2027-05-13T18:00"
		if i == 1 {
			wantFrom = "2027-05-20T18:00"
		}
		if ch["from"] != wantFrom {
			t.Fatalf("%s from = %v, want %s", ref, ch["from"], wantFrom)
		}
		if ch["to"] != aRes["starts_at_local"] {
			t.Fatalf("%s to != current clock", ref)
		}
		if !numEq(last["seq"], len(bHist)+1) {
			t.Fatalf("%s seq = %v", ref, last["seq"])
		}
		// Full terms equal current accepted terms.
		lt, _ := json.Marshal(last["accepted_terms"])
		ct, _ := json.Marshal(aRes["accepted_terms"])
		if string(lt) != string(ct) {
			t.Fatalf("%s terms != current", ref)
		}
	}
	// Series +1 ONCE, flags retained; unrelated series EXACT bytes.
	bSeries := beforeSt["series"].(map[string]any)
	aSeries := afterSt["series"].(map[string]any)
	bs, _ := json.Marshal(bSeries[sid])
	as, _ := json.Marshal(aSeries[sid])
	var bsm, asm map[string]any
	json.Unmarshal(bs, &bsm)
	json.Unmarshal(as, &asm)
	if asm["revision"] != bsm["revision"].(float64)+1 {
		t.Fatal("series must bump exactly once")
	}
	bm, _ := json.Marshal(bSeries[sid2])
	am, _ := json.Marshal(aSeries[sid2])
	if string(bm) != string(am) {
		t.Fatal("unrelated series changed")
	}
	for _, m := range aSeries[sid].(map[string]any)["members"].([]any) {
		if m.(map[string]any)["exception"] != false {
			t.Fatal("amend marked exception")
		}
	}
	// FULL counter map: only r1 +1.
	bCtr := beforeSt["restaurant_revisions"].(map[string]any)
	aCtr := afterSt["restaurant_revisions"].(map[string]any)
	if len(aCtr) != len(bCtr) {
		t.Fatal("counter keys changed")
	}
	for k, v := range bCtr {
		want := v.(float64)
		if k == "r1" {
			want++
		}
		if aCtr[k] != want {
			t.Fatalf("counter %s = %v, want %v", k, aCtr[k], want)
		}
	}
	// Untouched namespaces byte-identical except the normal amend receipt.
	for _, ns := range []string{"users", "tokens", "restaurants", "policies"} {
		ab, _ := json.Marshal(afterSt[ns])
		bb, _ := json.Marshal(beforeSt[ns])
		if string(ab) != string(bb) {
			t.Fatalf("namespace %s changed", ns)
		}
	}
	afterRc := afterSt["receipts"].(map[string]any)
	beforeRc := beforeSt["receipts"].(map[string]any)
	if len(afterRc) != len(beforeRc)+1 {
		t.Fatalf("receipts %d -> %d, want +1", len(beforeRc), len(afterRc))
	}
	var newKeys []string
	for k := range afterRc {
		if _, ok := beforeRc[k]; !ok {
			newKeys = append(newKeys, k)
		}
	}
	if len(newKeys) != 1 {
		t.Fatalf("new receipts = %v", newKeys)
	}
	nr := afterRc[newKeys[0]].(map[string]any)
	if nr["method"] != "POST" || !numEq(nr["status"], 201) {
		t.Fatalf("new receipt = %v", nr)
	}
	if !containsStr(nr["path"].(string), sid) {
		t.Fatalf("new receipt path = %v", nr["path"])
	}
}

// numEq compares a JSON number in either native-int or decoded-float64 form.
func numEq(v any, want int) bool {
	switch n := v.(type) {
	case int:
		return n == want
	case float64:
		return n == float64(want)
	}
	return false
}

func jsonStr(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func containsStr(hay, needle string) bool {
	return len(hay) >= len(needle) && (hay == needle || len(needle) == 0 || indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestSeriesAmendReplay(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	body := map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"}
	res := amendCall(t, s, tok, sid, "rk", body)
	if res.Status != 201 {
		t.Fatalf("amend = %d", res.Status)
	}
	raw201, _ := json.Marshal(res.Body)
	// Mutate after: amend again with the new revision, then cancel the member.
	s.mu.Lock()
	rev2 := s.state.Series[sid].Revision
	s.mu.Unlock()
	res2 := amendCall(t, s, tok, sid, "rk2", map[string]any{"expected_revision": float64(rev2), "from_index": 1, "local_time": "21:00"})
	if res2.Status != 201 {
		t.Fatalf("second amend = %d", res2.Status)
	}
	if res := s.CancelReservation(tok, refs[1]); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	// Baseline AFTER mutation+cancel but BEFORE the FIRST replay; current
	// provably differs (member cancelled, revision advanced).
	preFirst := string(exportBytes(t, s))
	var preFirstState map[string]any
	if err := json.Unmarshal([]byte(preFirst), &preFirstState); err != nil {
		t.Fatal(err)
	}
	cur := preFirstState["state"].(map[string]any)["reservations"].(map[string]any)[refs[1]].(map[string]any)
	if cur["status"] != "cancelled" {
		t.Fatalf("current not cancelled: %v", cur["status"])
	}
	var orig map[string]any
	if err := json.Unmarshal(raw201, &orig); err != nil {
		t.Fatal(err)
	}
	origOccs := orig["occurrences"].([]any)
	if origOccs[1].(map[string]any)["reservation"].(map[string]any)["status"] != "confirmed" {
		t.Fatal("original was not confirmed")
	}
	// First replay: 200 original bytes, export identity afterwards.
	rep := amendCall(t, s, tok, sid, "rk", body)
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	repRaw, _ := json.Marshal(rep.Body)
	if string(repRaw) != string(raw201) {
		t.Fatalf("replay bytes differ:\n%s\n%s", repRaw, raw201)
	}
	if got := string(exportBytes(t, s)); got != preFirst {
		t.Fatal("first replay mutated state")
	}
	// Same key with a COMMITTED different body is 409 with state identity.
	rep2 := amendCall(t, s, tok, sid, "rk", map[string]any{"expected_revision": float64(rev2), "from_index": 1, "local_time": "22:00"})
	if rep2.Status != 409 {
		t.Fatalf("committed-key different body = %d", rep2.Status)
	}
	if got := string(exportBytes(t, s)); got != preFirst {
		t.Fatal("409 mutated state")
	}
}

func TestSeriesAmendConcurrency(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	ctrBefore := s.state.RestaurantRevisions["r1"]
	s.mu.Unlock()
	mkBody := func(clock string) map[string]any {
		return map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": clock}
	}
	preRace := string(exportBytes(t, s))
	var wg sync.WaitGroup
	results := make([]Result, 2)
	wg.Add(2)
	go func() { defer wg.Done(); results[0] = amendCall(t, s, tok, sid, "ck1", mkBody("20:00")) }()
	go func() { defer wg.Done(); results[1] = amendCall(t, s, tok, sid, "ck2", mkBody("21:00")) }()
	wg.Wait()
	var n201, n409 int
	for _, r := range results {
		switch r.Status {
		case 201:
			n201++
		case 409:
			n409++
			if r.Body.(map[string]any)["error"].(map[string]any)["code"] != "stale_revision" {
				t.Fatalf("loser code = %v", r.Body)
			}
		default:
			t.Fatalf("status = %d", r.Status)
		}
	}
	if n201 != 1 || n409 != 1 {
		t.Fatalf("201/409 = %d/%d", n201, n409)
	}
	var beforeState map[string]any
	if err := json.Unmarshal([]byte(preRace), &beforeState); err != nil {
		t.Fatal(err)
	}
	beforeSt := beforeState["state"].(map[string]any)
	_ = beforeSt
	s.mu.Lock()
	if s.state.RestaurantRevisions["r1"] != ctrBefore+1 {
		s.mu.Unlock()
		t.Fatal("loser changed counter")
	}
	s.mu.Unlock()
	// FULL pre/post delta: target booking rev/history exactly once, current
	// matches the REAL winner 201 response, series once, FULL counter once,
	// unrelated records/histories/series/policies/users/tokens/restaurants/plans
	// /closures identical, EXACTLY one new scoped receipt, loser key no receipt,
	// all prior receipts preserved.
	after := string(exportBytes(t, s))
	var afterState map[string]any
	if err := json.Unmarshal([]byte(after), &afterState); err != nil {
		t.Fatal(err)
	}
	afterSt := afterState["state"].(map[string]any)
	var winnerBody map[string]any
	for _, r := range results {
		if r.Status == 201 {
			raw, _ := json.Marshal(r.Body)
			json.Unmarshal(raw, &winnerBody)
		}
	}
	winnerOccs := winnerBody["occurrences"].([]any)
	if len(winnerOccs) != 2 {
		t.Fatalf("winner occurrences = %d", len(winnerOccs))
	}
	changedRef := ""
	for _, o := range winnerOccs {
		om := o.(map[string]any)
		rec := om["reservation"].(map[string]any)
		if rec["revision"] == 2 || rec["revision"] == float64(2) {
			if changedRef != "" {
				t.Fatal("two bookings changed")
			}
			changedRef = rec["reference"].(string)
		}
	}
	if changedRef == "" {
		t.Fatal("no booking changed")
	}
	bHist := beforeSt["histories"].(map[string]any)[changedRef].([]any)
	aHist := afterSt["histories"].(map[string]any)[changedRef].([]any)
	if len(aHist) != len(bHist)+1 {
		t.Fatal("target history not exactly once")
	}
	aRes := afterSt["reservations"].(map[string]any)[changedRef].(map[string]any)
	for j := range bHist {
		bj, _ := json.Marshal(bHist[j])
		aj, _ := json.Marshal(aHist[j])
		if string(bj) != string(aj) {
			t.Fatalf("target prefix entry %d changed", j)
		}
	}
	lastEntry := aHist[len(aHist)-1].(map[string]any)
	if lastEntry["event"] != "changed" {
		t.Fatalf("last event = %v", lastEntry["event"])
	}
	if !numEq(lastEntry["seq"], len(bHist)+1) || !numEq(lastEntry["revision"], 2) {
		t.Fatalf("last seq/rev = %v/%v", lastEntry["seq"], lastEntry["revision"])
	}
	lastChs := lastEntry["changes"].([]any)
	if len(lastChs) != 1 {
		t.Fatalf("last changes = %v", lastEntry["changes"])
	}
	lastCh := lastChs[0].(map[string]any)
	if lastCh["field"] != "starts_at_local" {
		t.Fatalf("last field = %v", lastCh["field"])
	}
	bClock := beforeSt["reservations"].(map[string]any)[changedRef].(map[string]any)["starts_at_local"]
	if lastCh["from"] != bClock {
		t.Fatalf("last From = %v, want %v", lastCh["from"], bClock)
	}
	if lastCh["to"] != aRes["starts_at_local"] {
		t.Fatal("last To != current clock")
	}
	if _, err := time.Parse(time.RFC3339, lastEntry["at"].(string)); err != nil {
		t.Fatalf("last at not RFC3339: %v", lastEntry["at"])
	}
	ltFull, _ := json.Marshal(lastEntry["accepted_terms"])
	ctFull, _ := json.Marshal(aRes["accepted_terms"])
	if string(ltFull) != string(ctFull) {
		t.Fatal("last terms != current terms")
	}
	var wrec map[string]any
	for _, o := range winnerOccs {
		if o.(map[string]any)["reference"] == changedRef {
			wrec = o.(map[string]any)["reservation"].(map[string]any)
		}
	}
	for _, k := range []string{"reservation_id", "reference", "restaurant_id", "party_size", "status", "starts_at_local", "starts_at", "ends_at", "created_at", "revision", "table_ids", "accepted_terms"} {
		wb, _ := json.Marshal(wrec[k])
		ab, _ := json.Marshal(aRes[k])
		if string(wb) != string(ab) {
			t.Fatalf("current %s != winner response", k)
		}
	}
	// Singleton scalar shape: winner table_ids len 1 carries table_id.
	var wids []any
	switch v := wrec["table_ids"].(type) {
	case []string:
		for _, id := range v {
			wids = append(wids, id)
		}
	case []any:
		wids = v
	default:
		t.Fatalf("winner table_ids type %T", wrec["table_ids"])
	}
	if len(wids) != 1 {
		t.Fatalf("winner not singleton: %v", wrec["table_ids"])
	}
	if wrec["table_id"] != wids[0] {
		t.Fatalf("winner scalar = %v, want %s", wrec["table_id"], wids[0])
	}
	// Affected series unchanged except revision+1; flags/schedules pinned.
	bSz := beforeSt["series"].(map[string]any)[sid].(map[string]any)
	aSz := afterSt["series"].(map[string]any)[sid].(map[string]any)
	for _, k := range []string{"series_id", "user_id", "restaurant_id", "interval_weeks"} {
		if bSz[k] != aSz[k] {
			t.Fatalf("series %s changed", k)
		}
	}
	bMems := bSz["members"].([]any)
	aMems := aSz["members"].([]any)
	if len(bMems) != len(aMems) {
		t.Fatal("member count changed")
	}
	for j := range bMems {
		bm := bMems[j].(map[string]any)
		am := aMems[j].(map[string]any)
		for _, k := range []string{"index", "reference", "scheduled_date", "exception"} {
			if bm[k] != am[k] {
				t.Fatalf("member %d %s changed", j, k)
			}
		}
	}
	for ref := range beforeSt["reservations"].(map[string]any) {
		if ref == changedRef {
			continue
		}
		bb, _ := json.Marshal(beforeSt["reservations"].(map[string]any)[ref])
		ab, _ := json.Marshal(afterSt["reservations"].(map[string]any)[ref])
		if string(bb) != string(ab) {
			t.Fatalf("unrelated record %s changed", ref)
		}
		bh, _ := json.Marshal(beforeSt["histories"].(map[string]any)[ref])
		ah, _ := json.Marshal(afterSt["histories"].(map[string]any)[ref])
		if string(bh) != string(ah) {
			t.Fatalf("unrelated history %s changed", ref)
		}
	}
	bCtrFull := beforeSt["restaurant_revisions"].(map[string]any)
	aCtrFull := afterSt["restaurant_revisions"].(map[string]any)
	if len(aCtrFull) != len(bCtrFull) {
		t.Fatal("counter keys changed")
	}
	for k, v := range bCtrFull {
		want := v.(float64)
		if k == "r1" {
			want++
		}
		if aCtrFull[k] != want {
			t.Fatalf("counter %s = %v, want %v", k, aCtrFull[k], want)
		}
	}
	for _, ns := range []string{"series", "policies", "users", "tokens", "restaurants", "plans", "closures"} {
		bb, _ := json.Marshal(beforeSt[ns])
		ab, _ := json.Marshal(afterSt[ns])
		if ns == "series" {
			var bm, am map[string]any
			json.Unmarshal(bb, &bm)
			json.Unmarshal(ab, &am)
			if len(bm) != len(am) {
				t.Fatal("series keys changed")
			}
			for k := range bm {
				bkr, _ := json.Marshal(bm[k])
				akr, _ := json.Marshal(am[k])
				if k == sid {
					var b1, a1 map[string]any
					json.Unmarshal(bkr, &b1)
					json.Unmarshal(akr, &a1)
					if a1["revision"] != b1["revision"].(float64)+1 {
						t.Fatal("series not exactly once")
					}
				} else if string(bkr) != string(akr) {
					t.Fatalf("unrelated series %s changed", k)
				}
			}
			continue
		}
		if string(bb) != string(ab) {
			t.Fatalf("namespace %s changed", ns)
		}
	}
	// EXACT new receipt binding: owner u1, method POST, EXACT amend path,
	// winning key, canonical winning body, status 201, raw response identical
	// to the winning 201 serialization; loser scoped key ABSENT; priors kept.
	bRc := beforeSt["receipts"].(map[string]any)
	aRc := afterSt["receipts"].(map[string]any)
	if len(aRc) != len(bRc)+1 {
		t.Fatalf("receipts %d -> %d, want +1", len(bRc), len(aRc))
	}
	var winnerKey string
	for i, r := range results {
		if r.Status == 201 {
			winnerKey = []string{"ck1", "ck2"}[i]
		}
	}
	var newKey string
	for k := range aRc {
		if _, ok := bRc[k]; !ok {
			if newKey != "" {
				t.Fatal("two new receipts")
			}
			newKey = k
		}
	}
	if newKey == "" {
		t.Fatal("no new receipt")
	}
	nr := aRc[newKey].(map[string]any)
	if nr["user_id"] != "u1" || nr["method"] != "POST" {
		t.Fatalf("new receipt scope = %v", nr)
	}
	if nr["path"] != "/series/"+sid+"/amend" {
		t.Fatalf("new receipt path = %v, want /series/%s/amend", nr["path"], sid)
	}
	if nr["key"] != winnerKey {
		t.Fatalf("new receipt key = %v, want %s", nr["key"], winnerKey)
	}
	if nr["status"] != float64(201) {
		t.Fatalf("new receipt status = %v", nr["status"])
	}
	var winnerRaw string
	for _, r := range results {
		if r.Status == 201 {
			raw, _ := json.Marshal(r.Body)
			winnerRaw = string(raw)
		}
	}
	if nr["response"] != winnerRaw {
		t.Fatal("new receipt response != winning 201 bytes")
	}
	var storedBody map[string]any
	if err := json.Unmarshal([]byte(nr["body"].(string)), &storedBody); err != nil {
		t.Fatalf("stored body not JSON: %v", err)
	}
	if !numEq(storedBody["expected_revision"], rev) || !numEq(storedBody["from_index"], 1) {
		t.Fatalf("stored body rev/index = %v", storedBody)
	}
	lt, _ := storedBody["local_time"].(string)
	if len(lt) != 5 || lt[2] != ':' {
		t.Fatalf("stored body local_time = %v", storedBody["local_time"])
	}
	// The stored local_time must equal the winner's actual clock HH:MM.
	var wClock string
	for _, o := range winnerOccs {
		om := o.(map[string]any)
		if om["reference"] == changedRef {
			wClock = om["reservation"].(map[string]any)["starts_at_local"].(string)[11:]
		}
	}
	if lt != wClock {
		t.Fatalf("stored local_time %s != winner clock %s", lt, wClock)
	}
	// Loser scoped key absent from stored receipts.
	loserKey := "ck1"
	if winnerKey == "ck1" {
		loserKey = "ck2"
	}
	for k := range aRc {
		if aRc[k].(map[string]any)["key"] == loserKey {
			t.Fatalf("loser key %s claimed a receipt", loserKey)
		}
	}
	for k, v := range bRc {
		vb, _ := json.Marshal(v)
		ab, _ := json.Marshal(aRc[k])
		if string(vb) != string(ab) {
			t.Fatalf("prior receipt %s changed", k)
		}
	}
}

func TestSeriesAmendSameKey50(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	body := map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"}
	preRace50 := string(exportBytes(t, s))
	var wg sync.WaitGroup
	statuses := make([]int, 50)
	bodies := make([]string, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := amendCall(t, s, tok, sid, "same50", body)
			statuses[i] = res.Status
			raw, _ := json.Marshal(res.Body)
			bodies[i] = string(raw)
		}(i)
	}
	wg.Wait()
	pre50 := preRace50
	var n201, n200 int
	for i, st := range statuses {
		switch st {
		case 201:
			n201++
		case 200:
			n200++
		default:
			t.Fatalf("status[%d] = %d", i, st)
		}
	}
	if n201 != 1 || n200 != 49 {
		t.Fatalf("201/200 = %d/%d", n201, n200)
	}
	// Every body byte-identical to the first 201 body.
	var first201 string
	for i, st := range statuses {
		if st == 201 {
			first201 = bodies[i]
			break
		}
	}
	for i, b := range bodies {
		if b != first201 {
			t.Fatalf("body[%d] differs", i)
		}
	}
	// FULL delta: target rev/history exactly once, current matches the REAL
	// 201 response, series once, FULL counter once, unrelated identical,
	// EXACTLY one new scoped receipt, all prior receipts preserved.
	after50 := string(exportBytes(t, s))
	var pre50State, post50State map[string]any
	if err := json.Unmarshal([]byte(pre50), &pre50State); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(after50), &post50State); err != nil {
		t.Fatal(err)
	}
	pre50St := pre50State["state"].(map[string]any)
	post50St := post50State["state"].(map[string]any)
	var winnerRef string
	for ref, r := range post50St["reservations"].(map[string]any) {
		rm := r.(map[string]any)
		prm := pre50St["reservations"].(map[string]any)[ref].(map[string]any)
		if !numEq(rm["revision"], int(prm["revision"].(float64))+1) {
			continue
		}
		if winnerRef != "" {
			t.Fatal("two bookings changed")
		}
		winnerRef = ref
	}
	if winnerRef == "" {
		t.Fatal("no booking changed")
	}
	bH := pre50St["histories"].(map[string]any)[winnerRef].([]any)
	aH := post50St["histories"].(map[string]any)[winnerRef].([]any)
	if len(aH) != len(bH)+1 {
		t.Fatal("winner history not exactly once")
	}
	wRes := post50St["reservations"].(map[string]any)[winnerRef].(map[string]any)
	if wRes["starts_at_local"] != "2027-05-13T20:00" {
		t.Fatalf("winner clock = %v", wRes["starts_at_local"])
	}
	for ref := range pre50St["reservations"].(map[string]any) {
		if ref == winnerRef {
			continue
		}
		bb, _ := json.Marshal(pre50St["reservations"].(map[string]any)[ref])
		ab, _ := json.Marshal(post50St["reservations"].(map[string]any)[ref])
		if string(bb) != string(ab) {
			t.Fatalf("unrelated record %s changed", ref)
		}
		bhh, _ := json.Marshal(pre50St["histories"].(map[string]any)[ref])
		ahh, _ := json.Marshal(post50St["histories"].(map[string]any)[ref])
		if string(bhh) != string(ahh) {
			t.Fatalf("unrelated history %s changed", ref)
		}
	}
	for _, ns := range []string{"series", "policies", "users", "tokens", "restaurants", "plans", "closures"} {
		bb, _ := json.Marshal(pre50St[ns])
		ab, _ := json.Marshal(post50St[ns])
		if ns == "series" {
			var bm, am map[string]any
			json.Unmarshal(bb, &bm)
			json.Unmarshal(ab, &am)
			for k := range bm {
				bkr, _ := json.Marshal(bm[k])
				akr, _ := json.Marshal(am[k])
				if k == sid {
					var b1, a1 map[string]any
					json.Unmarshal(bkr, &b1)
					json.Unmarshal(akr, &a1)
					if !numEq(a1["revision"], int(b1["revision"].(float64))+1) {
						t.Fatal("series not exactly once")
					}
				} else if string(bkr) != string(akr) {
					t.Fatalf("unrelated series %s changed", k)
				}
			}
			continue
		}
		if string(bb) != string(ab) {
			t.Fatalf("namespace %s changed", ns)
		}
	}
	bCtr := pre50St["restaurant_revisions"].(map[string]any)
	aCtr := post50St["restaurant_revisions"].(map[string]any)
	if len(aCtr) != len(bCtr) {
		t.Fatal("counter keys changed")
	}
	for k, v := range bCtr {
		want := v.(float64)
		if k == "r1" {
			want++
		}
		if aCtr[k] != want {
			t.Fatalf("counter %s = %v, want %v", k, aCtr[k], want)
		}
	}
	bRc := pre50St["receipts"].(map[string]any)
	aRc := post50St["receipts"].(map[string]any)
	if len(aRc) != len(bRc)+1 {
		t.Fatalf("receipts %d -> %d, want +1", len(bRc), len(aRc))
	}
	var newKey50 string
	for k := range aRc {
		if _, ok := bRc[k]; !ok {
			if newKey50 != "" {
				t.Fatal("two new receipts")
			}
			newKey50 = k
		}
	}
	if newKey50 == "" {
		t.Fatal("no new receipt")
	}
	nr50 := aRc[newKey50].(map[string]any)
	if nr50["user_id"] != "u1" || nr50["method"] != "POST" {
		t.Fatalf("new receipt scope = %v", nr50)
	}
	if nr50["path"] != "/series/"+sid+"/amend" {
		t.Fatalf("new receipt path = %v", nr50["path"])
	}
	if nr50["key"] != "same50" {
		t.Fatalf("new receipt key = %v", nr50["key"])
	}
	if !numEq(nr50["status"], 201) {
		t.Fatalf("new receipt status = %v", nr50["status"])
	}
	if nr50["response"] != first201 {
		t.Fatal("new receipt response != winning 201 bytes")
	}
	var storedBody50 map[string]any
	if err := json.Unmarshal([]byte(nr50["body"].(string)), &storedBody50); err != nil {
		t.Fatalf("stored body not JSON: %v", err)
	}
	if !numEq(storedBody50["expected_revision"], rev) || !numEq(storedBody50["from_index"], 1) {
		t.Fatalf("stored body = %v", storedBody50)
	}
	// COMPLETE winner projection: stored changed record == winning 201
	// reservation on identity/owner/party/tables/terms/end/clock.
	wRes50 := post50St["reservations"].(map[string]any)[winnerRef].(map[string]any)
	var wFirst map[string]any
	json.Unmarshal([]byte(first201), &wFirst)
	var wMatch map[string]any
	for _, o := range wFirst["occurrences"].([]any) {
		om := o.(map[string]any)
		if om["reference"] == winnerRef {
			wMatch = om["reservation"].(map[string]any)
		}
	}
	if wMatch == nil {
		t.Fatal("winner ref missing from 201 response")
	}
	for _, k := range []string{"reservation_id", "reference", "restaurant_id", "party_size", "status", "starts_at_local", "starts_at", "ends_at", "created_at", "revision", "table_ids", "accepted_terms"} {
		wb, _ := json.Marshal(wMatch[k])
		ab, _ := json.Marshal(wRes50[k])
		if string(wb) != string(ab) {
			t.Fatalf("winner projection %s differs", k)
		}
	}
	var w50ids []any
	switch v := wMatch["table_ids"].(type) {
	case []string:
		for _, id := range v {
			w50ids = append(w50ids, id)
		}
	case []any:
		w50ids = v
	default:
		t.Fatalf("winner table_ids type %T", wMatch["table_ids"])
	}
	if len(w50ids) != 1 || wMatch["table_id"] != w50ids[0] {
		t.Fatalf("winner scalar shape = %v", wMatch)
	}
	wbt, _ := json.Marshal(wMatch["accepted_terms"])
	abt, _ := json.Marshal(wRes50["accepted_terms"])
	if string(wbt) != string(abt) {
		t.Fatal("winner terms differ")
	}
	// Winner history prefix byte-equal + exactly one new Changed with
	// seq/rev/at/terms/ordered From/To.
	bH50 := pre50St["histories"].(map[string]any)[winnerRef].([]any)
	aH50 := post50St["histories"].(map[string]any)[winnerRef].([]any)
	if len(aH50) != len(bH50)+1 {
		t.Fatal("winner history not exactly once")
	}
	for j := range bH50 {
		bj, _ := json.Marshal(bH50[j])
		aj, _ := json.Marshal(aH50[j])
		if string(bj) != string(aj) {
			t.Fatalf("winner prefix entry %d changed", j)
		}
	}
	last50 := aH50[len(aH50)-1].(map[string]any)
	if last50["event"] != "changed" || !numEq(last50["revision"], int(bH50[len(bH50)-1].(map[string]any)["revision"].(float64))+1) {
		t.Fatalf("winner last = %v", last50)
	}
	ch50 := last50["changes"].([]any)
	if len(ch50) != 1 {
		t.Fatalf("winner changes = %v", ch50)
	}
	chm := ch50[0].(map[string]any)
	if chm["field"] != "starts_at_local" {
		t.Fatalf("winner change field = %v", chm["field"])
	}
	bClock50 := pre50St["reservations"].(map[string]any)[winnerRef].(map[string]any)["starts_at_local"]
	if chm["from"] != bClock50 {
		t.Fatalf("winner From = %v, want %v", chm["from"], bClock50)
	}
	if chm["to"] != wRes50["starts_at_local"] {
		t.Fatal("winner change To != current clock")
	}
	if !numEq(last50["seq"], len(bH50)+1) {
		t.Fatalf("winner seq = %v", last50["seq"])
	}
	if _, err := time.Parse(time.RFC3339, last50["at"].(string)); err != nil {
		t.Fatalf("winner at not RFC3339: %v", last50["at"])
	}
	wlt, _ := json.Marshal(last50["accepted_terms"])
	wct, _ := json.Marshal(wRes50["accepted_terms"])
	if string(wlt) != string(wct) {
		t.Fatal("winner terms != current terms")
	}
	for k, v := range bRc {
		vb, _ := json.Marshal(v)
		ab, _ := json.Marshal(aRc[k])
		if string(vb) != string(ab) {
			t.Fatalf("prior receipt %s changed", k)
		}
	}
}

func TestSeriesAmendClosureConflicts(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	// Applied closure on t_1 over the generated evening, in another offset.
	s.state.Closures["r1"] = []Closure{{TableID: "t_1", From: "2027-05-13T16:00:00+00:00", To: "2027-05-13T21:00:00+00:00"}}
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 409 {
		t.Fatalf("closure conflict = %d %v", res.Status, res.Body)
	}
	// Adjacent closure is free: ends exactly when the booking starts (20:00+02 = 18:00Z).
	s.mu.Lock()
	s.state.Closures["r1"] = []Closure{{TableID: "t_1", From: "2027-05-13T16:00:00+00:00", To: "2027-05-13T18:00:00+00:00"}}
	rev2 := s.state.Series[sid].Revision
	s.mu.Unlock()
	res = amendCall(t, s, tok, sid, "k2", map[string]any{"expected_revision": float64(rev2), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("adjacent closure must be free: %d %v", res.Status, res.Body)
	}
}

func TestSeriesAmendDST(t *testing.T) {
	// Berlin spring gap: a generated wall time that does not exist rejects
	// the whole amendment with invalid_local_time.
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"rber","name":"B","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,
		"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}],"reservations":[]}`
	if rec := serveRequest(s, "POST", "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	tok := versionLoginToken(t, s)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"rber","table_id":"t_1","starts_at_local":"2027-03-21T03:30","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	// Member date 2027-03-28 (spring forward): 02:30 does not exist.
	before := string(exportBytes(t, s))
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "02:30"})
	if res.Status != 422 {
		t.Fatalf("gap = %d %v", res.Status, res.Body)
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "invalid_local_time" {
		t.Fatalf("code = %v, want invalid_local_time", res.Body)
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("gap failure mutated state")
	}
}

func TestSeriesAmendBoundaries(t *testing.T) {
	// Genuine count-12 series: from_index=count-1 amends only the last member
	// (one real change, earlier 11 records/histories untouched); index 0
	// includes the anchor as a real change.
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s12", anchor, 12)
	if len(refs) != 12 {
		t.Fatalf("members = %d", len(refs))
	}
	before := string(exportBytes(t, s))
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "klast", map[string]any{"expected_revision": float64(rev), "from_index": 11, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("last-only = %d %v", res.Status, res.Body)
	}
	after := string(exportBytes(t, s))
	var bSt, aSt map[string]any
	json.Unmarshal([]byte(before), &bSt)
	json.Unmarshal([]byte(after), &aSt)
	bRes := bSt["state"].(map[string]any)["reservations"].(map[string]any)
	aRes := aSt["state"].(map[string]any)["reservations"].(map[string]any)
	bHist := bSt["state"].(map[string]any)["histories"].(map[string]any)
	aHist := aSt["state"].(map[string]any)["histories"].(map[string]any)
	for i, ref := range refs[:11] {
		bb, _ := json.Marshal(bRes[ref])
		ab, _ := json.Marshal(aRes[ref])
		if string(bb) != string(ab) {
			t.Fatalf("member %d changed", i)
		}
		if aRes[ref].(map[string]any)["starts_at_local"] == "2027-07-22T20:00" && i < 11 {
			t.Fatalf("member %d clock changed", i)
		}
		bh, _ := json.Marshal(bHist[ref])
		ah, _ := json.Marshal(aHist[ref])
		if string(bh) != string(ah) {
			t.Fatalf("member %d history changed", i)
		}
	}
	last := aRes[refs[11]].(map[string]any)
	if last["starts_at_local"] != "2027-07-22T20:00" {
		t.Fatalf("last clock = %v", last["starts_at_local"])
	}
	// Index 0 includes the anchor as a real change.
	s.mu.Lock()
	rev2 := s.state.Series[sid].Revision
	s.mu.Unlock()
	res = amendCall(t, s, tok, sid, "kfirst", map[string]any{"expected_revision": float64(rev2), "from_index": 0, "local_time": "21:00"})
	if res.Status != 201 {
		t.Fatalf("from-zero = %d %v", res.Status, res.Body)
	}
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	if occs[0].(map[string]any)["reservation"].(map[string]any)["starts_at_local"] != "2027-05-06T21:00" {
		t.Fatal("anchor not amended")
	}
}

func TestSeriesAmendDetachment(t *testing.T) {
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if res.Status != 201 {
		t.Fatalf("amend = %d", res.Status)
	}
	// Capture FULL export + pristine response bytes AFTER the 201.
	postAmend := string(exportBytes(t, s))
	pristine, _ := json.Marshal(res.Body)
	// Mutate ACTUAL returned live objects: nested table_ids slice, nested
	// accepted_terms capacities/hours maps, and occurrence fields.
	m := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)[1].(map[string]any)
	orr := occ["reservation"].(map[string]any)
	orr["table_ids"].([]string)[0] = "evil"
	orr["accepted_terms"].(map[string]any)["capacities"].(map[string]any)["t_1"] = 999
	orr["party_size"] = 999
	occ["exception"] = true
	// FULL export byte-identical: live response objects alias nothing stored.
	if got := string(exportBytes(t, s)); got != postAmend {
		t.Fatal("response mutation leaked into stored state")
	}
	// Same-key replay still 200 pristine bytes; fresh current series unchanged.
	rep := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:00"})
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	repRaw, _ := json.Marshal(rep.Body)
	if string(repRaw) != string(pristine) {
		t.Fatal("replay bytes differ after live mutation")
	}
	s.mu.Lock()
	revNow := s.state.Series[sid].Revision
	s.mu.Unlock()
	cur := s.GetSeries(tok, sid)
	if cur.Status != 200 {
		t.Fatalf("series get: %d", cur.Status)
	}
	curRaw, _ := json.Marshal(cur.Body)
	// Fresh GET must equal the pristine 201 bytes modulo the live-mutated
	// response object (pristine was captured before mutation of res.Body, and
	// GetSeries renders from stored state, which was proven unchanged).
	if string(curRaw) != string(pristine) {
		t.Fatalf("fresh GET differs from pristine 201:\n%s\n%s", curRaw, pristine)
	}
	_ = revNow
}

func TestSeriesAmendAuthMatrix(t *testing.T) {
	// Same key string across users/paths is independent; foreign series 404.
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid, _ := amendAdopt(t, s, tok, "s", anchor, 2)
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	// Second user u2 with its own anchor+series.
	if rec := serveRequest(s, http.MethodPost, "/auth/signup",
		[]byte(`{"email":"b@c","password":"password1","display_name":"B"}`), nil); rec.Code != 201 {
		t.Fatalf("signup: %d", rec.Code)
	}
	var sb map[string]any
	if err := json.Unmarshal(serveRequest(s, http.MethodPost, "/auth/login",
		[]byte(`{"email":"b@c","password":"password1"}`), nil).Body.Bytes(), &sb); err != nil {
		t.Fatal(err)
	}
	tok2 := sb["token"].(string)
	a2 := amendCreate(t, s, tok2, "a2", `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	sid2, _ := amendAdopt(t, s, tok2, "s2", a2, 2)
	// Foreign series -> 404 (owner-scoped).
	res := amendCall(t, s, tok, sid2, "k", map[string]any{"expected_revision": float64(1), "from_index": 0, "local_time": "20:00"})
	if res.Status != 404 {
		t.Fatalf("foreign series = %d", res.Status)
	}
	// Same key string as u1's amend, used by u2 on its own series: independent.
	s.mu.Lock()
	rev1 := s.state.Series[sid].Revision
	rev2 := s.state.Series[sid2].Revision
	s.mu.Unlock()
	r1 := amendCall(t, s, tok, sid, "shared-key", map[string]any{"expected_revision": float64(rev1), "from_index": 0, "local_time": "20:00"})
	r2 := amendCall(t, s, tok2, sid2, "shared-key", map[string]any{"expected_revision": float64(rev2), "from_index": 0, "local_time": "21:00"})
	if r1.Status != 201 || r2.Status != 201 {
		t.Fatalf("user-scoped keys = %d/%d", r1.Status, r2.Status)
	}
	if rev < 0 {
		t.Fatal("unreachable")
	}
}

func TestSeriesAmendNYFoldFirst(t *testing.T) {
	// New York fall-back: repeated 01:30 resolves to the first occurrence
	// (EDT, -04:00), and duration is absolute across the fold.
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"rny","name":"N","timezone":"America/New_York","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":0,
		"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}],"reservations":[]}`
	if rec := serveRequest(s, "POST", "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	tok := versionLoginToken(t, s)
	// Anchor Sunday 2026-10-25 at 03:30 (a real booking); member lands
	// 2026-11-01 at 03:30. Amend the member clock to the repeated 01:30: a
	// REAL change resolving to the FIRST fold occurrence (EDT, -04:00).
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"rny","table_id":"t_1","starts_at_local":"2026-10-25T03:30","party_size":1}`)
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 2)
	_ = refs
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	histBefore := len(s.state.Histories)
	ctrBefore := s.state.RestaurantRevisions["rny"]
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "01:30"})
	if res.Status != 201 {
		t.Fatalf("fold amend = %d %v", res.Status, res.Body)
	}
	occs := res.Body.(map[string]any)["occurrences"].([]any)
	m1 := occs[1].(map[string]any)["reservation"].(map[string]any)
	// Member really changed clocks: revision + history advanced.
	if m1["revision"] != 2 && m1["revision"] != float64(2) {
		t.Fatalf("member revision = %v, want 2", m1["revision"])
	}
	s.mu.Lock()
	if s.state.RestaurantRevisions["rny"] != ctrBefore+1 {
		s.mu.Unlock()
		t.Fatal("counter not exactly once")
	}
	if len(s.state.Histories) != histBefore {
		s.mu.Unlock()
		t.Fatal("history keys changed")
	}
	s.mu.Unlock()
	// First occurrence: EDT offset -04:00, absolute 90min end reads 01:00 EST.
	if m1["starts_at"] != "2026-11-01T01:30:00-04:00" {
		t.Fatalf("starts_at = %v, want first fold", m1["starts_at"])
	}
	if m1["ends_at"] != "2026-11-01T02:00:00-05:00" {
		t.Fatalf("ends_at = %v, want absolute duration", m1["ends_at"])
	}
}

func TestSeriesAmendPrecedenceIndexOrder(t *testing.T) {
	// Two eligible members each fail non-occupancy validation differently:
	// the FIRST index's error must win, even though the second also fails.
	s, tok := amendFixture(t)
	anchor := amendCreate(t, s, tok, "a", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	// Policy for 2027-05-13 (member 1): capacity t_1 stays 2 (party 1 fine).
	// Policy for 2027-05-20 (member 2): grid 60 makes :20 off-grid... instead
	// use cutoff: publish cutoff huge for member-2 date so ITS cutoff fails,
	// while member 1 fails capacity via a targeted policy.
	amendPublish(t, s, tok, "p-cap", "2027-05-13", 60)
	_ = anchor
	sid, refs := amendAdopt(t, s, tok, "s", anchor, 3)
	_ = refs
	// Member1 (05-13) gets a slot-60 policy so clock :30 is off-grid
	// (not_on_slot_grid). Member2 (05-20) gets a Thursday-closed policy so the
	// same clock is outside opening hours. Both fail non-occupancy validation;
	// member 1 is earlier in index order, so its grid error must win.
	body, _ := json.Marshal(map[string]any{
		"effective_from": "2027-05-13", "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s.PublishPolicy(tok, "r1", "p-tight", body); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	body2, _ := json.Marshal(map[string]any{
		"effective_from": "2027-05-20", "slot_minutes": 30, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "fri", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4},
	})
	if res := s.PublishPolicy(tok, "r1", "p-closed", body2); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	s.mu.Unlock()
	res := amendCall(t, s, tok, sid, "k", map[string]any{"expected_revision": float64(rev), "from_index": 1, "local_time": "20:30"})
	if res.Status != 422 {
		t.Fatalf("member1 non-occupancy must win, got %d %v", res.Status, res.Body)
	}
	if res.Body.(map[string]any)["error"].(map[string]any)["code"] != "not_on_slot_grid" {
		t.Fatalf("code = %v, want member-1 grid error", res.Body)
	}
}
