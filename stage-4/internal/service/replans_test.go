package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const replanFixture = `{
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}
  ],
  "restaurants": [
    {
      "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [
        {"id": "t_1", "label": "1", "capacity": 2},
        {"id": "t_2", "label": "2", "capacity": 4},
        {"id": "t_3", "label": "3", "capacity": 4}
      ],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
      "manager_user_ids": ["u_ada"]
    },
    {
      "id": "r_baar", "name": "Baar", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_9", "label": "9", "capacity": 6}],
      "manager_user_ids": ["u_bea"]
    }
  ],
  "reservations": [
    {"id": "seed-a", "reference": "BKAAAA", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_2",
     "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "seed-b", "reference": "BKBBBB", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "seed-c", "reference": "BKCCCC", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_3",
     "starts_at_local": "2027-06-17T21:30", "party_size": 2},
    {"id": "seed-f", "reference": "BKFFFF", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": "2027-06-24T19:00", "party_size": 2},
    {"id": "seed-d", "reference": "BKDDDD", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_2",
     "starts_at_local": "2027-06-17T19:00", "party_size": 1, "status": "cancelled"}
  ]
}`

const replanClosure = `{"table_id": "t_2", "from": "2027-06-17T18:00:00+02:00", "to": "2027-06-17T23:00:00+02:00"}`

func newReplanService(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(replanFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	return s, mgr, diner
}

func previewReplan(t *testing.T, s *Service, token, restaurant, key, body string) Result {
	t.Helper()
	return s.PreviewReplan(token, restaurant, key, []byte(body))
}

func exportStateMap(t *testing.T, s *Service) map[string]any {
	t.Helper()
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	st, _ := env["state"].(map[string]any)
	if st == nil {
		t.Fatal("export has no state")
	}
	return st
}

func TestReplanPreviewAuth(t *testing.T) {
	s, mgr, diner := newReplanService(t)
	if res := previewReplan(t, s, "", "r_anker", "k1", replanClosure); res.Status != 401 {
		t.Fatalf("missing token: %d %v", res.Status, res.Body)
	}
	if res := previewReplan(t, s, "nope", "r_anker", "k1", replanClosure); res.Status != 401 {
		t.Fatalf("bad token: %d %v", res.Status, res.Body)
	}
	if st, code := resultCode(previewReplan(t, s, diner, "r_anker", "k1", replanClosure)); st != 403 || code != "forbidden" {
		t.Fatalf("diner publish: %d %s", st, code)
	}
	// u_bea manages r_baar only: forbidden on r_anker.
	if st, _ := resultCode(previewReplan(t, s, diner, "r_baar", "k2", `{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)); st != 201 {
		t.Fatalf("own-restaurant manager preview: %d", st)
	}
	if st, code := resultCode(previewReplan(t, s, mgr, "r_baar", "k3", `{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)); st != 403 || code != "forbidden" {
		t.Fatalf("foreign manager: %d %s", st, code)
	}
	if st, code := resultCode(previewReplan(t, s, mgr, "r_nope", "k4", replanClosure)); st != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant: %d %s", st, code)
	}
	if res := previewReplan(t, s, mgr, "r_anker", "k5", replanClosure); res.Status != 201 {
		t.Fatalf("manager preview: %d %v", res.Status, res.Body)
	}
}

func TestReplanPreviewBodyValidation(t *testing.T) {
	s, mgr, diner := newReplanService(t)
	cases := []struct {
		name string
		body string
	}{
		{"missing-table", `{"from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`},
		{"number-table", `{"table_id":2,"from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`},
		{"missing-from", `{"table_id":"t_2","to":"2027-06-17T23:00:00+02:00"}`},
		{"number-from", `{"table_id":"t_2","from":42,"to":"2027-06-17T23:00:00+02:00"}`},
		{"missing-to", `{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00"}`},
		{"zoneless-from", `{"table_id":"t_2","from":"2027-06-17T18:00","to":"2027-06-17T23:00:00+02:00"}`},
		{"zoneless-to", `{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00"}`},
		{"bad-offset", `{"table_id":"t_2","from":"2027-06-17T18:00:00+0200","to":"2027-06-17T23:00:00+02:00"}`},
		{"from-after-to", `{"table_id":"t_2","from":"2027-06-17T23:00:00+02:00","to":"2027-06-17T18:00:00+02:00"}`},
		{"from-eq-to", `{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:00:00+02:00"}`},
	}
	for i, c := range cases {
		res := previewReplan(t, s, mgr, "r_anker", fmt.Sprintf("bv-%d", i), c.body)
		if st, code := resultCode(res); st != 422 || code != "validation_failed" {
			t.Fatalf("%s: got %d %s", c.name, st, code)
		}
	}
	if res := s.PreviewReplan(mgr, "r_anker", "badjson", []byte(`{"table_id":`)); res.Status != 400 {
		t.Fatalf("malformed body: %d %v", res.Status, res.Body)
	}
	if st, code := resultCode(previewReplan(t, s, mgr, "r_anker", "unkt", `{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)); st != 404 || code != "not_found" {
		t.Fatalf("unknown table: %d %s", st, code)
	}
	// Unknown fields are ignored: extra member still previews 201.
	res := previewReplan(t, s, mgr, "r_anker", "extra1", `{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00","zzz":1}`)
	if res.Status != 201 {
		t.Fatalf("unknown fields ignored: %d %v", res.Status, res.Body)
	}
	// Z is an explicit UTC offset (u_bea manages r_baar).
	res = previewReplan(t, s, diner, "r_baar", "zulu1", `{"table_id":"t_9","from":"2027-06-17T16:00:00Z","to":"2027-06-17T17:00:00Z"}`)
	if res.Status != 201 {
		t.Fatalf("Z offset: %d %v", res.Status, res.Body)
	}
}

func TestReplanPreviewSuccess(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	before := exportStateMap(t, s)
	revBefore := before["restaurant_revisions"].(map[string]any)["r_anker"]
	res := previewReplan(t, s, mgr, "r_anker", "ok-1", replanClosure)
	if res.Status != 201 {
		t.Fatalf("preview: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	var keys []string
	for k := range body {
		keys = append(keys, k)
	}
	if len(keys) != 6 {
		t.Fatalf("public shape must have exactly 6 keys, got %v", keys)
	}
	for _, k := range []string{"plan_id", "restaurant_revision", "closure", "assignments", "moved_count", "unused_seats"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing public key %q in %v", k, keys)
		}
	}
	if fmt.Sprintf("%v", body["restaurant_revision"]) != fmt.Sprintf("%v", revBefore) {
		t.Fatalf("captured revision %v, want current %v", body["restaurant_revision"], revBefore)
	}
	cl, _ := body["closure"].(map[string]any)
	if cl["table_id"] != "t_2" || cl["from"] != "2027-06-17T18:00:00+02:00" || cl["to"] != "2027-06-17T23:00:00+02:00" {
		t.Fatalf("closure echo: %v", cl)
	}
	assign, _ := body["assignments"].([]any)
	if len(assign) != 3 {
		t.Fatalf("want 3 considered assignments (cancelled excluded), got %d", len(assign))
	}
	want := []map[string]any{
		{"reference": "BKAAAA", "table_ids": []any{"t_3"}, "changed": true},
		{"reference": "BKBBBB", "table_ids": []any{"t_1"}, "changed": false},
		{"reference": "BKCCCC", "table_ids": []any{"t_3"}, "changed": false},
	}
	for i, w := range want {
		got, _ := assign[i].(map[string]any)
		if got["reference"] != w["reference"] || got["changed"] != w["changed"] {
			t.Fatalf("assignment %d: got %v want %v", i, got, w)
		}
		ids, _ := json.Marshal(got["table_ids"])
		exp, _ := json.Marshal(w["table_ids"])
		if string(ids) != string(exp) {
			t.Fatalf("assignment %d tables: got %s want %s", i, ids, exp)
		}
	}
	if body["moved_count"] != 1 || body["unused_seats"] != 4 {
		t.Fatalf("totals: %v", body)
	}
	// State delta is exactly one plan plus one receipt; counters and all
	// other namespaces byte-identical.
	after := exportStateMap(t, s)
	for k, bv := range before {
		if k == "plans" || k == "receipts" {
			continue
		}
		av, _ := json.Marshal(after[k])
		xv, _ := json.Marshal(bv)
		if string(av) != string(xv) {
			t.Fatalf("namespace %q changed by preview", k)
		}
	}
	bPlans, _ := before["plans"].(map[string]any)
	aPlans, _ := after["plans"].(map[string]any)
	if len(aPlans) != len(bPlans)+1 {
		t.Fatalf("exactly one plan stored: %d -> %d", len(bPlans), len(aPlans))
	}
	bRec, _ := before["receipts"].(map[string]any)
	aRec, _ := after["receipts"].(map[string]any)
	if len(aRec) != len(bRec)+1 {
		t.Fatalf("exactly one receipt stored: %d -> %d", len(bRec), len(aRec))
	}
	planID, _ := body["plan_id"].(string)
	if planID == "" {
		t.Fatal("empty plan_id")
	}
	if _, ok := aPlans[planID]; !ok {
		t.Fatalf("stored plan %q missing", planID)
	}
}

func TestReplanPreviewEmpty(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	res := previewReplan(t, s, mgr, "r_anker", "empty-1",
		`{"table_id":"t_1","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}`)
	if res.Status != 201 {
		t.Fatalf("empty preview: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	assign, _ := body["assignments"].([]any)
	if assign == nil || len(assign) != 0 {
		t.Fatalf("empty considered must yield allocated []: %v", body["assignments"])
	}
	if body["moved_count"] != 0 || body["unused_seats"] != 0 {
		t.Fatalf("empty totals: %v", body)
	}
}

func TestReplanPreviewLimits(t *testing.T) {
	seeds := ""
	for i := 0; i < 7; i++ {
		seeds += fmt.Sprintf(`{"id": "s%d", "reference": "LIM%03d", "user_id": "u_bea",
     "restaurant_id": "r_baar", "table_id": "t_9",
     "starts_at_local": "2027-06-17T19:00", "party_size": 1},`, i, i)
	}
	fx := `{"users": [{"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}],
  "restaurants": [{"id": "r_baar", "name": "Baar", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_9", "label": "9", "capacity": 6}], "manager_user_ids": ["u_bea"]}],
  "reservations": [` + seeds[:len(seeds)-1] + `]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "bea@example.com", "correct horse bea")
	body := `{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T20:00:00+02:00"}`
	before, _ := json.Marshal(s.Export().Body)
	if st, code := resultCode(s.PreviewReplan(tok, "r_baar", "lim-7", []byte(body))); st != 422 || code != "planning_limit" {
		t.Fatalf("7 considered: got %d %s", st, code)
	}
	after, _ := json.Marshal(s.Export().Body)
	if string(before) != string(after) {
		t.Fatal("limit failure changed state")
	}
}

func TestReplanPreviewMaxBoundary(t *testing.T) {
	// Exact supported maxima: 6 tables, 4 declared pairs, 6 considered.
	// Closure t_1 spans the evening; staggered bookings rotate tables so
	// only the t_1 occupant must move (to t_2, adjacent in time).
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_max", "name": "Max", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 2},{"id": "t_3", "label": "3", "capacity": 2},{"id": "t_4", "label": "4", "capacity": 2},{"id": "t_5", "label": "5", "capacity": 2},{"id": "t_6", "label": "6", "capacity": 2}],
      "combinable": [["t_1","t_2"],["t_2","t_3"],["t_4","t_5"],["t_5","t_6"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "w1", "reference": "MXAAAA", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_1", "starts_at_local": "2027-06-17T18:00", "party_size": 2},
    {"id": "w2", "reference": "MXBBBB", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_2", "starts_at_local": "2027-06-17T19:30", "party_size": 2},
    {"id": "w3", "reference": "MXCCCC", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_3", "starts_at_local": "2027-06-17T18:00", "party_size": 2},
    {"id": "w4", "reference": "MXDDDD", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_4", "starts_at_local": "2027-06-17T19:30", "party_size": 2},
    {"id": "w5", "reference": "MXEEEE", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_5", "starts_at_local": "2027-06-17T18:00", "party_size": 2},
    {"id": "w6", "reference": "MXFFFF", "user_id": "u_ada", "restaurant_id": "r_max", "table_id": "t_6", "starts_at_local": "2027-06-17T19:30", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	res := s.PreviewReplan(tok, "r_max", "mx-1",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("max boundary: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	assign, _ := body["assignments"].([]any)
	if len(assign) != 6 {
		t.Fatalf("all 6 considered: %v", body)
	}
	refs := make([]string, 0, 6)
	got := map[string]map[string]any{}
	for _, a := range assign {
		am, _ := a.(map[string]any)
		refs = append(refs, am["reference"].(string))
		got[am["reference"].(string)] = am
	}
	for i := 1; i < len(refs); i++ {
		if refs[i-1] >= refs[i] {
			t.Fatalf("assignments not reference-sorted: %v", refs)
		}
	}
	ids := func(m map[string]any) string {
		raw, _ := json.Marshal(m["table_ids"])
		return string(raw)
	}
	if ids(got["MXAAAA"]) != `["t_2"]` || got["MXAAAA"]["changed"] != true {
		t.Fatalf("t_1 occupant must move to t_2: %v", got["MXAAAA"])
	}
	for _, r := range []string{"MXBBBB", "MXCCCC", "MXDDDD", "MXEEEE", "MXFFFF"} {
		if got[r]["changed"] != false {
			t.Fatalf("%s must stay: %v", r, got[r])
		}
	}
	if body["moved_count"] != 1 || body["unused_seats"] != 0 {
		t.Fatalf("totals: %v", body)
	}
	// Each limit exceeded individually: 7 tables and 5 pairs.
	for _, tc := range []struct {
		name string
		fx   string
	}{
		{"seven-tables", `{"users": [{"id": "u", "email": "u@e.com", "password": "correct horse", "display_name": "U"}],
  "restaurants": [{"id": "r7", "name": "R", "timezone": "Europe/Berlin", "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 0,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 2},{"id": "t_3", "label": "3", "capacity": 2},{"id": "t_4", "label": "4", "capacity": 2},{"id": "t_5", "label": "5", "capacity": 2},{"id": "t_6", "label": "6", "capacity": 2},{"id": "t_7", "label": "7", "capacity": 2}],
      "manager_user_ids": ["u"]}], "reservations": []}`},
		{"five-pairs", `{"users": [{"id": "u", "email": "u@e.com", "password": "correct horse", "display_name": "U"}],
  "restaurants": [{"id": "r5", "name": "R", "timezone": "Europe/Berlin", "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 0,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 2},{"id": "t_3", "label": "3", "capacity": 2},{"id": "t_4", "label": "4", "capacity": 2},{"id": "t_5", "label": "5", "capacity": 2},{"id": "t_6", "label": "6", "capacity": 2}],
      "combinable": [["t_1","t_2"],["t_2","t_3"],["t_3","t_4"],["t_4","t_5"],["t_5","t_6"]], "manager_user_ids": ["u"]}], "reservations": []}`},
	} {
		s2 := New()
		if res := s2.Reset([]byte(tc.fx)); res.Status != 204 {
			t.Fatalf("%s reset: %d", tc.name, res.Status)
		}
		tok2 := policyLogin(t, s2, "u@e.com", "correct horse")
		rid := "r7"
		if tc.name == "five-pairs" {
			rid = "r5"
		}
		before, _ := json.Marshal(s2.Export().Body)
		res := s2.PreviewReplan(tok2, rid, "lim",
			[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`))
		if st, code := resultCode(res); st != 422 || code != "planning_limit" {
			t.Fatalf("%s: got %d %s", tc.name, st, code)
		}
		after, _ := json.Marshal(s2.Export().Body)
		if string(before) != string(after) {
			t.Fatalf("%s changed state", tc.name)
		}
	}
}

func TestReplanPreviewNoFeasible(t *testing.T) {
	fx := `{"users": [{"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]], "manager_user_ids": ["u_bea"]}],
  "reservations": [
    {"id": "a", "reference": "NFAAAA", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "b", "reference": "NFBBBB", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_1", "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "c", "reference": "NFCCCC", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_3", "starts_at_local": "2027-06-17T19:00", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "bea@example.com", "correct horse bea")
	before, _ := json.Marshal(s.Export().Body)
	res := s.PreviewReplan(tok, "r_anker", "nf-1",
		[]byte(`{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T20:00:00+02:00"}`))
	if st, code := resultCode(res); st != 409 || code != "no_feasible_plan" {
		t.Fatalf("infeasible: got %d %s", st, code)
	}
	after, _ := json.Marshal(s.Export().Body)
	if string(before) != string(after) {
		t.Fatal("failed preview changed state")
	}
	// The same failed key stays reusable.
	res = s.PreviewReplan(tok, "r_anker", "nf-1",
		[]byte(`{"table_id":"t_1","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("failed key genuine reuse: %d %v", res.Status, res.Body)
	}
	genuine, _ := json.Marshal(res.Body)
	again := s.PreviewReplan(tok, "r_anker", "nf-1",
		[]byte(`{"table_id":"t_1","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}`))
	if again.Status != 200 {
		t.Fatalf("replay after genuine reuse: %d", again.Status)
	}
	againJSON, _ := json.Marshal(again.Body)
	if string(genuine) != string(againJSON) {
		t.Fatal("replay bytes differ after genuine reuse")
	}
}

func TestReplanPreviewReplay(t *testing.T) {
	s, mgr, diner := newReplanService(t)
	first := previewReplan(t, s, mgr, "r_anker", "rp-1", replanClosure)
	if first.Status != 201 {
		t.Fatalf("first: %d", first.Status)
	}
	firstJSON, _ := json.Marshal(first.Body)
	fm, _ := first.Body.(map[string]any)
	revAtPreview := fm["restaurant_revision"]
	// A real later booking at the same restaurant really moves the counter
	// and the export.
	mid, _ := json.Marshal(s.Export().Body)
	mk := s.CreateReservation(diner, "rp-mid",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T21:30","party_size":1}`))
	if mk.Status != 201 {
		t.Fatalf("mid booking: %d %v", mk.Status, mk.Body)
	}
	post, _ := json.Marshal(s.Export().Body)
	if string(mid) == string(post) {
		t.Fatal("mid booking did not change export")
	}
	var midEnv, postEnv map[string]any
	_ = json.Unmarshal(mid, &midEnv)
	_ = json.Unmarshal(post, &postEnv)
	mr := midEnv["state"].(map[string]any)["restaurant_revisions"].(map[string]any)["r_anker"]
	pr := postEnv["state"].(map[string]any)["restaurant_revisions"].(map[string]any)["r_anker"]
	if mr == pr {
		t.Fatal("mid booking did not increase counter")
	}
	// Original key/body replays 200 with EXACT original JSON bytes; the
	// captured revision stays old; pre/post replay exports are identical.
	preReplay, _ := json.Marshal(s.Export().Body)
	second := previewReplan(t, s, mgr, "r_anker", "rp-1", replanClosure)
	if second.Status != 200 {
		t.Fatalf("replay status: %d", second.Status)
	}
	secondJSON, _ := json.Marshal(second.Body)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("replay bytes differ:\n%s\n%s", firstJSON, secondJSON)
	}
	sm, _ := second.Body.(map[string]any)
	if fmt.Sprintf("%v", sm["restaurant_revision"]) != fmt.Sprintf("%v", revAtPreview) {
		t.Fatalf("replay revision %v, want captured %v", sm["restaurant_revision"], revAtPreview)
	}
	postReplay, _ := json.Marshal(s.Export().Body)
	if string(preReplay) != string(postReplay) {
		t.Fatal("replay changed export")
	}
	if st, code := resultCode(previewReplan(t, s, mgr, "r_anker", "rp-1", `{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)); st != 409 || code != "idempotency_key_reuse" {
		t.Fatalf("key reuse: %d %s", st, code)
	}
	// A failed key stays reusable.
	if st, _ := resultCode(previewReplan(t, s, mgr, "r_anker", "rp-bad", `{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)); st != 404 {
		t.Fatalf("unknown table first: %d", st)
	}
	if res := previewReplan(t, s, mgr, "r_anker", "rp-bad", replanClosure); res.Status != 201 {
		t.Fatalf("failed key reuse: %d", res.Status)
	}
}

func TestReplanPreviewReceiptBeforeValidation(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	first := previewReplan(t, s, mgr, "r_anker", "rbv-1", replanClosure)
	if first.Status != 201 {
		t.Fatalf("first: %d", first.Status)
	}
	firstJSON, _ := json.Marshal(first.Body)
	// Remove manager membership afterwards: the stored receipt must still
	// resolve before callback validation, returning the original 200.
	s.mu.Lock()
	for i := range s.state.Restaurants {
		if s.state.Restaurants[i].ID == "r_anker" {
			s.state.Restaurants[i].ManagerUserIDs = []string{"u_nobody"}
		}
	}
	s.mu.Unlock()
	second := previewReplan(t, s, mgr, "r_anker", "rbv-1", replanClosure)
	if second.Status != 200 {
		t.Fatalf("replay after demotion: %d %v", second.Status, second.Body)
	}
	secondJSON, _ := json.Marshal(second.Body)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("replay bytes differ after demotion")
	}
	// And a fresh key with the demoted manager is now 403.
	if st, _ := resultCode(previewReplan(t, s, mgr, "r_anker", "rbv-2", replanClosure)); st != 403 {
		t.Fatalf("fresh key after demotion: %d", st)
	}
}

func TestReplanPreviewScopes(t *testing.T) {
	s, mgr, diner := newReplanService(t)
	// Same key string, different users: independent scopes.
	a := previewReplan(t, s, mgr, "r_anker", "shared", replanClosure)
	if a.Status != 201 {
		t.Fatalf("manager scope: %d", a.Status)
	}
	b := previewReplan(t, s, diner, "r_baar", "shared",
		`{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`)
	if b.Status != 201 {
		t.Fatalf("other-user scope (u_bea manages r_baar): %d %v", b.Status, b.Body)
	}
	// Same user, same key, different path: independent scope.
	cbody := `{"restaurant_id":"r_baar","table_id":"t_9","starts_at_local":"2027-06-17T19:00","party_size":2}`
	c := s.CreateReservation(mgr, "shared", []byte(cbody))
	if c.Status != 201 {
		t.Fatalf("other-path scope: %d %v", c.Status, c.Body)
	}
}

func TestReplanPreviewRace50(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	// Prepopulate a previous preview so pre-existing entries exist.
	if res := previewReplan(t, s, mgr, "r_anker", "race-pre", replanClosure); res.Status != 201 {
		t.Fatalf("prepopulate: %d", res.Status)
	}
	before, _ := json.Marshal(s.Export().Body)
	const n = 50
	codes := make([]int, n)
	outs := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := s.PreviewReplan(mgr, "r_anker", "race50", []byte(replanClosure))
			codes[i] = res.Status
			raw, _ := json.Marshal(res.Body)
			outs[i] = string(raw)
		}(i)
	}
	wg.Wait()
	ones, zeros := 0, 0
	for _, c := range codes {
		switch c {
		case 201:
			ones++
		case 200:
			zeros++
		}
	}
	if ones != 1 || zeros != n-1 {
		t.Fatalf("race: 201=%d 200=%d", ones, zeros)
	}
	for i := 1; i < n; i++ {
		if outs[i] != outs[0] {
			t.Fatal("race response bytes differ")
		}
	}
	// Exactly one plan and one receipt added; counters and every other
	// namespace unchanged.
	afterRaw, _ := json.Marshal(s.Export().Body)
	var beforeEnv, afterEnv map[string]any
	_ = json.Unmarshal(before, &beforeEnv)
	_ = json.Unmarshal(afterRaw, &afterEnv)
	bs := beforeEnv["state"].(map[string]any)
	as := afterEnv["state"].(map[string]any)
	for k, bv := range bs {
		if k == "plans" || k == "receipts" {
			continue
		}
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(as[k])
		if string(xv) != string(yv) {
			t.Fatalf("race changed namespace %q", k)
		}
	}
	bp := bs["plans"].(map[string]any)
	ap := as["plans"].(map[string]any)
	br := bs["receipts"].(map[string]any)
	ar := as["receipts"].(map[string]any)
	if len(ap) != len(bp)+1 || len(ar) != len(br)+1 {
		t.Fatalf("race must add one plan+receipt: plans %d->%d receipts %d->%d",
			len(bp), len(ap), len(br), len(ar))
	}
	// Full new-plan value: ID/rest/applied/revision/closure/assignments/
	// totals match the public winner response; the pre-existing plan entry
	// is byte-identical.
	var winBody map[string]any
	for _, o := range outs {
		if o != "" {
			if err := json.Unmarshal([]byte(o), &winBody); err != nil {
				t.Fatalf("winner parses: %v", err)
			}
			break
		}
	}
	if winBody == nil {
		t.Fatal("no winner response")
	}
	newPlans := map[string]any{}
	for k, v := range ap {
		if _, ok := bp[k]; !ok {
			newPlans[k] = v
		}
	}
	if len(newPlans) != 1 {
		t.Fatalf("one new plan: %v", newPlans)
	}
	for pid, pv := range newPlans {
		pm, _ := pv.(map[string]any)
		if pid != winBody["plan_id"] {
			t.Fatalf("stored plan id %q != public %v", pid, winBody["plan_id"])
		}
		if pm["applied"] != false {
			t.Fatalf("stored plan applied flag: %v", pm)
		}
		if pm["restaurant_id"] != "r_anker" {
			t.Fatalf("stored restaurant: %v", pm["restaurant_id"])
		}
		for _, k := range []string{"restaurant_revision", "moved_count", "unused_seats"} {
			pj, _ := json.Marshal(pm[k])
			wj, _ := json.Marshal(winBody[k])
			if string(pj) != string(wj) {
				t.Fatalf("stored %s %s != public %s", k, pj, wj)
			}
		}
		pc, _ := json.Marshal(pm["closure"])
		wc, _ := json.Marshal(winBody["closure"])
		wantClosure := `{"from":"2027-06-17T18:00:00+02:00","table_id":"t_2","to":"2027-06-17T23:00:00+02:00"}`
		if string(pc) != wantClosure || string(wc) != wantClosure {
			t.Fatalf("closure mismatch stored=%s public=%s", pc, wc)
		}
		pa, _ := json.Marshal(pm["assignments"])
		wa, _ := json.Marshal(winBody["assignments"])
		// Stored assignments carry the same reference/table set/order as
		// the public shape (public adds no extra fields).
		var paList, waList []map[string]any
		_ = json.Unmarshal(pa, &paList)
		_ = json.Unmarshal(wa, &waList)
		if len(paList) != len(waList) {
			t.Fatalf("assignment counts: %s vs %s", pa, wa)
		}
		for i := range paList {
			if paList[i]["reference"] != waList[i]["reference"] ||
				fmt.Sprintf("%v", paList[i]["changed"]) != fmt.Sprintf("%v", waList[i]["changed"]) {
				t.Fatalf("assignment %d diverges: %v vs %v", i, paList[i], waList[i])
			}
			px, _ := json.Marshal(paList[i]["table_ids"])
			wx, _ := json.Marshal(waList[i]["table_ids"])
			if string(px) != string(wx) {
				t.Fatalf("assignment %d tables: %s vs %s", i, px, wx)
			}
		}
	}
	for k, v := range bp {
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(ap[k])
		if string(xv) != string(yv) {
			t.Fatalf("pre-existing plan %q changed", k)
		}
	}
	for k, v := range br {
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(ar[k])
		if string(xv) != string(yv) {
			t.Fatalf("pre-existing receipt changed: %q", k)
		}
	}
	// The single new receipt binds manager/method/path/key/canonical body
	// with status 201 and the original raw response bytes.
	var newRec map[string]any
	for k, v := range ar {
		if _, ok := br[k]; !ok {
			rm, _ := v.(map[string]any)
			newRec = rm
		}
	}
	if newRec == nil {
		t.Fatal("no new receipt")
	}
	if newRec["key"] != "race50" || newRec["method"] != "POST" ||
		newRec["path"] != "/restaurants/r_anker/replans" || newRec["status"] != float64(201) {
		t.Fatalf("receipt binding: %v", newRec)
	}
	var adaID string
	for id, u := range as["users"].(map[string]any) {
		um, _ := u.(map[string]any)
		if um["email"] == "ada@example.com" {
			adaID = id
		}
	}
	if adaID == "" {
		t.Fatal("ada user missing from export")
	}
	if newRec["user_id"] != adaID {
		t.Fatalf("receipt owner %v, want %v", newRec["user_id"], adaID)
	}
	canon, err := CanonicalBody(map[string]any{"table_id": "t_2", "from": "2027-06-17T18:00:00+02:00", "to": "2027-06-17T23:00:00+02:00"})
	if err != nil {
		t.Fatal(err)
	}
	if newRec["body"] != canon {
		t.Fatalf("receipt body %v != canonical %v", newRec["body"], canon)
	}
	resp, _ := newRec["response"].(string)
	if resp != outs[0] {
		t.Fatalf("receipt response != original raw response")
	}
}

func TestReplanPreviewDetachment(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	res := previewReplan(t, s, mgr, "r_anker", "det-1", replanClosure)
	if res.Status != 201 {
		t.Fatalf("preview: %d", res.Status)
	}
	m, _ := res.Body.(map[string]any)
	pristine, _ := json.Marshal(res.Body)
	storedBefore, _ := json.Marshal(s.Export().Body)
	// Mutate the actual nested structures in place (not slot replacement).
	as, _ := m["assignments"].([]any)
	first, _ := as[0].(map[string]any)
	first["table_ids"].([]string)[0] = "MUT"
	first["changed"] = "MUT"
	m["closure"].(map[string]any)["from"] = "MUT"
	m["moved_count"] = "MUT"
	again := previewReplan(t, s, mgr, "r_anker", "det-1", replanClosure)
	if again.Status != 200 {
		t.Fatalf("replay: %d", again.Status)
	}
	replayed, _ := json.Marshal(again.Body)
	if string(replayed) != string(pristine) {
		t.Fatalf("replay bytes changed after caller mutation:\n%s\n%s", replayed, pristine)
	}
	storedAfter, _ := json.Marshal(s.Export().Body)
	if string(storedBefore) != string(storedAfter) {
		t.Fatal("stored export changed after caller mutation")
	}
	var env map[string]any
	_ = json.Unmarshal(storedAfter, &env)
	plans, _ := env["state"].(map[string]any)["plans"].(map[string]any)
	for _, p := range plans {
		pm, _ := p.(map[string]any)
		for _, a := range pm["assignments"].([]any) {
			am, _ := a.(map[string]any)
			for _, id := range am["table_ids"].([]any) {
				if id == "MUT" {
					t.Fatal("stored plan aliases caller map")
				}
			}
		}
	}
}

func TestReplanPreviewPair(t *testing.T) {
	// A pair booking stays on its declared pair (seeded reversed, stored
	// canonical) while the closed-table occupant relocates.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_pair", "name": "Pair", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1","t_2"],["t_2","t_3"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "a", "reference": "PRAAAA", "user_id": "u_ada", "restaurant_id": "r_pair", "table_ids": ["t_2","t_1"], "starts_at_local": "2027-06-17T19:00", "party_size": 6},
    {"id": "b", "reference": "PRBBBB", "user_id": "u_ada", "restaurant_id": "r_pair", "table_id": "t_3", "starts_at_local": "2027-06-17T21:30", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	res := s.PreviewReplan(tok, "r_pair", "pair-1",
		[]byte(`{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("pair preview: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	assign, _ := body["assignments"].([]any)
	if len(assign) != 2 {
		t.Fatalf("both considered: %v", body)
	}
	got := map[string]map[string]any{}
	for _, a := range assign {
		am, _ := a.(map[string]any)
		got[am["reference"].(string)] = am
	}
	ids := func(m map[string]any) string {
		raw, _ := json.Marshal(m["table_ids"])
		return string(raw)
	}
	if ids(got["PRAAAA"]) != `["t_1","t_2"]` || got["PRAAAA"]["changed"] != false {
		t.Fatalf("pair retained canonical+unchanged: %v", got["PRAAAA"])
	}
	if ids(got["PRBBBB"]) != `["t_1"]` || got["PRBBBB"]["changed"] != true {
		t.Fatalf("closed occupant relocates: %v", got["PRBBBB"])
	}
	for _, a := range assign {
		am, _ := a.(map[string]any)
		if ids(am) == `["t_1","t_3"]` {
			t.Fatalf("nontransitive pair assigned: %v", body)
		}
	}
	if body["moved_count"] != 1 || body["unused_seats"] != 0 {
		t.Fatalf("totals: %v", body)
	}
}

func TestReplanPreviewConsideredVsFixed(t *testing.T) {
	// Four tables: A (t_2, considered) must avoid t_1 (B, considered,
	// overlapping) and t_3 (F, fixed outside the closure but overlapping
	// A's tail), landing on t_4. B (t_1, overlapping the closure on an
	// unrelated table) is considered and stays put.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_fix", "name": "Fix", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 2},{"id": "t_3", "label": "3", "capacity": 2},{"id": "t_4", "label": "4", "capacity": 2}],
      "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "a", "reference": "CFAAAA", "user_id": "u_ada", "restaurant_id": "r_fix", "table_id": "t_2", "starts_at_local": "2027-06-17T18:30", "party_size": 2},
    {"id": "b", "reference": "CFBBBB", "user_id": "u_ada", "restaurant_id": "r_fix", "table_id": "t_1", "starts_at_local": "2027-06-17T18:30", "party_size": 2},
    {"id": "f", "reference": "CFFFFF", "user_id": "u_ada", "restaurant_id": "r_fix", "table_id": "t_3", "starts_at_local": "2027-06-17T19:30", "party_size": 1}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	res := s.PreviewReplan(tok, "r_fix", "cf-1",
		[]byte(`{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("preview: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	assign, _ := body["assignments"].([]any)
	if len(assign) != 2 {
		t.Fatalf("want A+B considered (F fixed), got %d", len(assign))
	}
	got := map[string]map[string]any{}
	for _, a := range assign {
		am, _ := a.(map[string]any)
		got[am["reference"].(string)] = am
	}
	ids := func(m map[string]any) string {
		raw, _ := json.Marshal(m["table_ids"])
		return string(raw)
	}
	if ids(got["CFAAAA"]) != `["t_4"]` || got["CFAAAA"]["changed"] != true {
		t.Fatalf("A must move to t_4 past fixed F: %v", got["CFAAAA"])
	}
	if ids(got["CFBBBB"]) != `["t_1"]` || got["CFBBBB"]["changed"] != false {
		t.Fatalf("B stays on unrelated t_1: %v", got["CFBBBB"])
	}
	if body["moved_count"] != 1 || body["unused_seats"] != 0 {
		t.Fatalf("totals: %v", body)
	}
}

func TestReplanPreviewOwnAcceptedCapacities(t *testing.T) {
	s, mgr, diner := newReplanService(t)
	// Publish selected caps where t_1 shrinks to 1 (fixture says 2).
	pol, _ := json.Marshal(map[string]any{
		"effective_from": "2027-06-01", "slot_minutes": 30,
		"reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
		"opening_hours": []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":    map[string]any{"t_1": 1, "t_2": 4, "t_3": 4},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "cap-pol", pol); res.Status != 201 {
		t.Fatalf("publish: %d %v", res.Status, res.Body)
	}
	// A (party 2) books t_2 under the published terms; B (party 1) books t_1.
	// A Thursday 06-24 keeps clear of the seeded 06-17 bookings.
	a := s.CreateReservation(diner, "cap-a",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-24T19:00","party_size":2}`))
	if a.Status != 201 {
		t.Fatalf("create A: %d %v", a.Status, a.Body)
	}
	am, _ := a.Body.(map[string]any)
	aref, _ := am["reference"].(string)
	b := s.CreateReservation(diner, "cap-b",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-24T21:30","party_size":1}`))
	if b.Status != 201 {
		t.Fatalf("create B: %d %v", b.Status, b.Body)
	}
	res := previewReplan(t, s, mgr, "r_anker", "cap-rp",
		`{"table_id":"t_2","from":"2027-06-24T18:00:00+02:00","to":"2027-06-24T23:00:00+02:00"}`)
	if res.Status != 201 {
		t.Fatalf("preview: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	assign, _ := body["assignments"].([]any)
	// Under fixture caps A could take t_1 (cap 2); under its accepted caps
	// t_1 holds only 1, so A must take t_3.
	found := false
	for _, x := range assign {
		xm, _ := x.(map[string]any)
		if xm["reference"] == aref {
			found = true
			ids, _ := json.Marshal(xm["table_ids"])
			if string(ids) != `["t_3"]` || xm["changed"] != true {
				t.Fatalf("A must move to t_3 under accepted caps: %v", xm)
			}
		}
	}
	if !found {
		t.Fatalf("anchor booking %s not considered", aref)
	}
	if body["moved_count"] != 1 {
		t.Fatalf("only A moves: %v", body)
	}
	// Publish a DIFFERENT same-date policy (superseding version 2 with
	// t_1 back to 4 and t_3 down to 1): date selection for the 06-24
	// bookings now yields version 2, but the bookings keep their old
	// accepted version-1 terms, which the preview must still use.
	pol2, _ := json.Marshal(map[string]any{
		"effective_from": "2027-06-01", "slot_minutes": 30,
		"reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
		"opening_hours": []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":    map[string]any{"t_1": 4, "t_2": 4, "t_3": 1},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "cap-pol2", pol2); res.Status != 201 {
		t.Fatalf("publish 2: %d %v", res.Status, res.Body)
	}
	s.mu.Lock()
	rest := restaurantByID(&s.state, "r_anker")
	sel, serr := selectedTerms(&s.state, rest, "2027-06-24")
	s.mu.Unlock()
	if serr != nil {
		t.Fatalf("selectedTerms: %v", serr)
	}
	if sel.PolicyVersion != 2 || sel.Capacities["t_1"] != 4 || sel.Capacities["t_3"] != 1 {
		t.Fatalf("date selection must be v2 {t_1:4,t_3:1}: %+v", sel)
	}
	recBefore, _ := json.Marshal(s.Export().Body)
	var recEnv map[string]any
	_ = json.Unmarshal(recBefore, &recEnv)
	recState := recEnv["state"].(map[string]any)
	refs := map[string]bool{aref: true}
	for _, r := range recState["reservations"].(map[string]any) {
		rm, _ := r.(map[string]any)
		if refs[rm["reference"].(string)] {
			if rm["revision"] != float64(1) {
				t.Fatalf("record revision moved: %v", rm)
			}
			at, _ := rm["accepted_terms"].(map[string]any)
			caps, _ := at["capacities"].(map[string]any)
			if at["policy_version"] != float64(1) || caps["t_1"] != float64(1) || caps["t_3"] != float64(4) {
				t.Fatalf("stored booking must keep v1 {t_1:1,t_3:4} terms: %v", at)
			}
		}
	}
	res = previewReplan(t, s, mgr, "r_anker", "cap-rp2",
		`{"table_id":"t_2","from":"2027-06-24T18:00:00+02:00","to":"2027-06-24T23:00:00+02:00"}`)
	if res.Status != 201 {
		t.Fatalf("preview after republication: %d %v", res.Status, res.Body)
	}
	bm2, _ := res.Body.(map[string]any)
	as2, _ := bm2["assignments"].([]any)
	occ := 0
	for _, x := range as2 {
		xm, _ := x.(map[string]any)
		if xm["reference"] == aref {
			occ++
			ids, _ := json.Marshal(xm["table_ids"])
			if string(ids) != `["t_3"]` {
				t.Fatalf("newest caps must not apply to old booking: %v", xm)
			}
		}
	}
	if occ != 1 {
		t.Fatalf("booking %s occurs %d times, want exactly once", aref, occ)
	}
	recAfter, _ := json.Marshal(s.Export().Body)
	var a2, b2 map[string]any
	_ = json.Unmarshal(recBefore, &a2)
	_ = json.Unmarshal(recAfter, &b2)
	as_, _ := json.Marshal(a2["state"].(map[string]any)["reservations"])
	bs_, _ := json.Marshal(b2["state"].(map[string]any)["reservations"])
	ah, _ := json.Marshal(a2["state"].(map[string]any)["histories"])
	bh, _ := json.Marshal(b2["state"].(map[string]any)["histories"])
	if string(as_) != string(bs_) || string(ah) != string(bh) {
		t.Fatal("preview changed stored records/terms/history")
	}
	// Above publication maxima: fixture capacity 101 is accepted as-is.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_big", "name": "Big", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 101},{"id": "t_2", "label": "2", "capacity": 101}],
      "manager_user_ids": ["u_ada"]}],
  "reservations": [{"id": "g", "reference": "BIGAAA", "user_id": "u_ada",
     "restaurant_id": "r_big", "table_id": "t_1", "starts_at_local": "2027-06-17T19:00", "party_size": 90}]}`
	s2 := New()
	if res := s2.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset big: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s2, "ada@example.com", "correct horse")
	res = s2.PreviewReplan(tok, "r_big", "big-1",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("big preview: %d %v", res.Status, res.Body)
	}
	bm, _ := res.Body.(map[string]any)
	as, _ := bm["assignments"].([]any)
	if len(as) != 1 {
		t.Fatalf("one considered: %v", bm)
	}
	am2, _ := as[0].(map[string]any)
	ids, _ := json.Marshal(am2["table_ids"])
	if string(ids) != `["t_2"]` || bm["unused_seats"] != 11 {
		t.Fatalf("above-max caps must apply: %v", bm)
	}
}

func TestReplanPreviewSeriesUntouched(t *testing.T) {
	fx := `{"users": [{"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]], "manager_user_ids": ["u_bea"]}],
  "reservations": [
    {"id": "anch", "reference": "SRANCH", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_1", "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "oth", "reference": "SROTHR", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "bea@example.com", "correct horse bea")
	adopt := s.AdoptSeries(tok, "ser-1", []byte(`{"anchor_reference":"SRANCH","count":2,"interval_weeks":1}`))
	if adopt.Status != 201 {
		t.Fatalf("adopt: %d %v", adopt.Status, adopt.Body)
	}
	am, _ := adopt.Body.(map[string]any)
	sid, _ := am["series_id"].(string)
	res := s.PreviewReplan(tok, "r_anker", "ser-rp",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("preview: %d %v", res.Status, res.Body)
	}
	// Preview moves nothing in stored state: series revision, exception
	// flags, reservation revision and history length all unchanged.
	cur := s.GetSeries(tok, sid)
	if cur.Status != 200 {
		t.Fatalf("series get: %d", cur.Status)
	}
	cm, _ := cur.Body.(map[string]any)
	if cm["revision"] != 1 {
		t.Fatalf("series revision changed: %v", cm["revision"])
	}
	occs, _ := cm["occurrences"].([]any)
	if len(occs) != 2 {
		t.Fatalf("occurrences: %v", cm)
	}
	o0, _ := occs[0].(map[string]any)
	if o0["exception"] != false || o0["reference"] != "SRANCH" {
		t.Fatalf("anchor occurrence: %v", o0)
	}
	hist := s.ReservationHistory(tok, "SRANCH")
	if hist.Status != 200 {
		t.Fatalf("history: %d", hist.Status)
	}
	hm, _ := hist.Body.(map[string]any)
	he, _ := json.Marshal(hm["entries"])
	var entries []any
	if err := json.Unmarshal(he, &entries); err != nil || len(entries) != 1 {
		t.Fatalf("anchor history grew: %v", hm)
	}
	lk := s.GetReservation(tok, "SRANCH")
	lm, _ := lk.Body.(map[string]any)
	if lk.Status != 200 || lm["revision"] != 1 {
		t.Fatalf("anchor record: %d %v", lk.Status, lm)
	}
}

func TestReplanPreviewPriorClosures(t *testing.T) {
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "a", "reference": "PCAAAA", "user_id": "u_ada", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	setClosures := func(cs []Closure) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.state.Closures["r_anker"] = cs
	}
	// Adjacent prior closure (ends exactly at booking start): absolute
	// half-open adjacency leaves t_1 free, so A takes rank-0 t_1.
	setClosures([]Closure{{TableID: "t_1", From: "2027-06-17T18:00:00+02:00", To: "2027-06-17T19:00:00+02:00"}})
	res := s.PreviewReplan(tok, "r_anker", "pc-adj",
		[]byte(`{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("adjacent prior: %d %v", res.Status, res.Body)
	}
	bm, _ := res.Body.(map[string]any)
	as, _ := bm["assignments"].([]any)
	am, _ := as[0].(map[string]any)
	ids, _ := json.Marshal(am["table_ids"])
	if am["reference"] != "PCAAAA" || string(ids) != `["t_1"]` {
		t.Fatalf("adjacent prior must leave t_1 free: %v", bm)
	}
	// Overlapping prior closure on t_1 blocks it (any member): A takes t_3.
	setClosures([]Closure{{TableID: "t_1", From: "2027-06-17T19:00:00+02:00", To: "2027-06-17T20:00:00+02:00"}})
	res = s.PreviewReplan(tok, "r_anker", "pc-over",
		[]byte(`{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("overlapping prior: %d %v", res.Status, res.Body)
	}
	bm, _ = res.Body.(map[string]any)
	as, _ = bm["assignments"].([]any)
	if len(as) != 1 {
		t.Fatalf("one considered: %v", bm)
	}
	am, _ = as[0].(map[string]any)
	ids, _ = json.Marshal(am["table_ids"])
	if am["reference"] != "PCAAAA" || string(ids) != `["t_3"]` || am["changed"] != true {
		t.Fatalf("overlapping prior must force t_3: %v", bm)
	}
	if bm["moved_count"] != 1 || bm["unused_seats"] != 2 {
		t.Fatalf("totals: %v", bm)
	}
}

func TestReplanPreviewPastCutoff(t *testing.T) {
	// A booking long past its cancellation cutoff is still repairable by an
	// operator: no cutoff check applies to preview.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "p", "reference": "PSTAAA", "user_id": "u_ada", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2020-05-07T19:00", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	res := s.PreviewReplan(tok, "r_anker", "past-1",
		[]byte(`{"table_id":"t_2","from":"2020-05-07T18:00:00+02:00","to":"2020-05-07T23:00:00+02:00"}`))
	if res.Status != 201 {
		t.Fatalf("past-cutoff repair: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	as, _ := body["assignments"].([]any)
	if len(as) != 1 {
		t.Fatalf("past booking considered: %v", body)
	}
	am, _ := as[0].(map[string]any)
	ids, _ := json.Marshal(am["table_ids"])
	if am["reference"] != "PSTAAA" || string(ids) != `["t_1"]` || am["changed"] != true {
		t.Fatalf("past booking moves to t_1: %v", body)
	}
}

func TestReplanPreviewRouting(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	h := map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "http-1"}
	rec := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure), h)
	if rec.Code != 201 {
		t.Fatalf("route preview: %d %s", rec.Code, rec.Body.String())
	}
}

func TestReplanPreviewStrictInstants(t *testing.T) {
	s, mgr, _ := newReplanService(t)
	before, _ := json.Marshal(s.Export().Body)
	badFrom := []string{
		"2027-06-18T18:00:00+24:00",
		"2027-06-18T18:00:00+02:60",
		"2027-06-18T8:00:00+02:00",
		"2027-06-18T18:00:00,1+02:00",
	}
	for i, f := range badFrom {
		body := fmt.Sprintf(`{"table_id":"t_2","from":%q,"to":"2027-06-18T19:00:00+02:00"}`, f)
		res := previewReplan(t, s, mgr, "r_anker", fmt.Sprintf("sif-%d", i), body)
		if st, code := resultCode(res); st != 422 || code != "validation_failed" {
			t.Fatalf("bad from %q: got %d %s", f, st, code)
		}
		body = fmt.Sprintf(`{"table_id":"t_2","from":"2027-06-18T18:00:00+02:00","to":%q}`, strings.Replace(f, "18:00", "19:00", 1))
		res = previewReplan(t, s, mgr, "r_anker", fmt.Sprintf("sit-%d", i), body)
		if st, code := resultCode(res); st != 422 || code != "validation_failed" {
			t.Fatalf("bad to %q: got %d %s", f, st, code)
		}
	}
	after, _ := json.Marshal(s.Export().Body)
	if string(before) != string(after) {
		t.Fatal("invalid intervals changed state")
	}
	// Failed strict keys stay reusable for a genuine valid 201.
	res := previewReplan(t, s, mgr, "r_anker", "sif-0",
		`{"table_id":"t_2","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}`)
	if res.Status != 201 {
		t.Fatalf("failed-key reuse after strict reject: %d", res.Status)
	}
	// Valid fractional seconds accepted, original text preserved.
	res = previewReplan(t, s, mgr, "r_anker", "sif-frac",
		`{"table_id":"t_2","from":"2027-06-18T18:00:00.1+02:00","to":"2027-06-18T19:00:00.1+02:00"}`)
	if res.Status != 201 {
		t.Fatalf("fractional instants: %d %v", res.Status, res.Body)
	}
	bm, _ := res.Body.(map[string]any)
	cl, _ := bm["closure"].(map[string]any)
	if cl["from"] != "2027-06-18T18:00:00.1+02:00" || cl["to"] != "2027-06-18T19:00:00.1+02:00" {
		t.Fatalf("fractional text not preserved: %v", cl)
	}
	// Absolute ordering across offset spellings: 17:00+02 (=15:00Z) is
	// before 18:00+01 (=17:00Z).
	res = previewReplan(t, s, mgr, "r_anker", "sif-ord",
		`{"table_id":"t_2","from":"2027-06-18T17:00:00+02:00","to":"2027-06-18T18:00:00+01:00"}`)
	if res.Status != 201 {
		t.Fatalf("offset ordering: %d %v", res.Status, res.Body)
	}
}
