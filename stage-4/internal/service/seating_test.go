package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func pairFixture(combinable string) string {
	return `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
		"combinable":` + combinable + `}],"reservations":[]}`
}

func restaurantOf(t *testing.T, s *Service) *Restaurant {
	t.Helper()
	s.mu.Lock()
	for i := range s.state.Restaurants {
		if s.state.Restaurants[i].ID == "r1" {
			out := s.state.Restaurants[i]
			out.OpeningHours = append([]OpeningHour(nil), out.OpeningHours...)
			out.Tables = append([]Table(nil), out.Tables...)
			out.Combinable = append([][]string(nil), out.Combinable...)
			s.mu.Unlock()
			return &out
		}
	}
	s.mu.Unlock()
	t.Fatal("restaurant r1 missing")
	return nil
}

// Canonical sets use declared order; reversed pairs name the same set.
func TestSeatingCanonicalOrder(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(`[["t_1","t_2"],["t_2","t_3"]]`)), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	r := restaurantOf(t, s)
	for _, tc := range []struct {
		in   []string
		want []string
	}{
		{[]string{"t_2", "t_1"}, []string{"t_1", "t_2"}},
		{[]string{"t_1", "t_2"}, []string{"t_1", "t_2"}},
		{[]string{"t_3", "t_2"}, []string{"t_2", "t_3"}},
	} {
		got, cerr := parseTableSelection(r, map[string]any{"table_ids": toAnySlice(tc.in)}, nil)
		if cerr != nil {
			t.Fatalf("%v: %v", tc.in, cerr)
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("%v -> %v, want %v", tc.in, got, tc.want)
		}
	}
	// Non-transitive: t_1+t_3 shares no declared pair.
	if _, cerr := parseTableSelection(r, map[string]any{"table_ids": toAnySlice([]string{"t_1", "t_3"})}, nil); cerr == nil || cerr.code != "combination_not_allowed" {
		t.Fatalf("undeclared pair cerr = %+v", cerr)
	}
}

func toAnySlice(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, id)
	}
	return out
}

// Selection matrix: distinct limits, wrong types, missing/empty, unknown and
// foreign ids, current retention, array copies.
func TestParseTableSelectionMatrix(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(`[["t_1","t_2"]]`)), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	r := restaurantOf(t, s)
	cur := []string{"t_1"}
	cases := []struct {
		name   string
		obj    map[string]any
		cur    []string
		status int
		code   string
	}{
		{"both formats", map[string]any{"table_id": "t_1", "table_ids": toAnySlice([]string{"t_1"})}, nil, 422, "validation_failed"},
		{"three members", map[string]any{"table_ids": toAnySlice([]string{"t_1", "t_2", "t_3"})}, nil, 422, "combination_not_allowed"},
		{"duplicate ids", map[string]any{"table_ids": toAnySlice([]string{"t_1", "t_1"})}, nil, 422, "validation_failed"},
		{"single wrong type", map[string]any{"table_id": 42}, nil, 400, "malformed_request"},
		{"multi wrong type", map[string]any{"table_ids": "t_1"}, nil, 400, "malformed_request"},
		{"item wrong type", map[string]any{"table_ids": []any{"t_1", 42}}, nil, 400, "malformed_request"},
		{"missing create", map[string]any{}, nil, 422, "validation_failed"},
		{"empty list", map[string]any{"table_ids": []any{}}, nil, 422, "validation_failed"},
		{"unknown table", map[string]any{"table_id": "t_ghost"}, nil, 404, "not_found"},
		{"foreign pair id", map[string]any{"table_ids": toAnySlice([]string{"t_1", "t_ghost"})}, nil, 404, "not_found"},
		{"undeclared pair", map[string]any{"table_ids": toAnySlice([]string{"t_1", "t_3"})}, nil, 422, "combination_not_allowed"},
		{"id too long", map[string]any{"table_id": strings.Repeat("x", 65)}, nil, 422, "validation_failed"},
	}
	for _, tc := range cases {
		_, cerr := parseTableSelection(r, tc.obj, tc.cur)
		if cerr == nil || cerr.status != tc.status || cerr.code != tc.code {
			t.Errorf("%s: cerr = %+v, want %d %s", tc.name, cerr, tc.status, tc.code)
		}
	}
	// PATCH with no selection fields retains current; copies never alias.
	got, cerr := parseTableSelection(r, map[string]any{"party_size": 2}, cur)
	if cerr != nil || strings.Join(got, ",") != "t_1" {
		t.Fatalf("retention = %v %+v", got, cerr)
	}
	got[0] = "MUT"
	if cur[0] != "t_1" {
		t.Fatal("retention aliased caller slice")
	}
	in := toAnySlice([]string{"t_2", "t_1"})
	got, cerr = parseTableSelection(r, map[string]any{"table_ids": in}, nil)
	if cerr != nil {
		t.Fatal(cerr)
	}
	in[0] = "MUT"
	if strings.Join(got, ",") != "t_1,t_2" {
		t.Fatalf("input aliased: %v", got)
	}
}

// Options order: fixture singles then declared pairs with summed capacity.
func TestSeatingOptionsOrder(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(`[["t_1","t_2"],["t_2","t_3"]]`)), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	r := restaurantOf(t, s)
	opts := seatingOptions(r)
	want := []SeatingOption{
		{TableIDs: []string{"t_1"}, Capacity: 2},
		{TableIDs: []string{"t_2"}, Capacity: 4},
		{TableIDs: []string{"t_3"}, Capacity: 4},
		{TableIDs: []string{"t_1", "t_2"}, Capacity: 6},
		{TableIDs: []string{"t_2", "t_3"}, Capacity: 8},
	}
	if len(opts) != len(want) {
		t.Fatalf("options = %+v", opts)
	}
	for i := range want {
		if strings.Join(opts[i].TableIDs, ",") != strings.Join(want[i].TableIDs, ",") || opts[i].Capacity != want[i].Capacity {
			t.Fatalf("option %d = %+v, want %+v", i, opts[i], want[i])
		}
	}
}

// Intersection is pure set overlap.
func TestTableSetsIntersect(t *testing.T) {
	if !tableSetsIntersect([]string{"t_1", "t_2"}, []string{"t_2", "t_3"}) {
		t.Fatal("shared t_2 must intersect")
	}
	if tableSetsIntersect([]string{"t_1"}, []string{"t_2", "t_3"}) {
		t.Fatal("disjoint sets must not intersect")
	}
	if tableSetsIntersect(nil, []string{"t_1"}) {
		t.Fatal("empty set must not intersect")
	}
}

// Public: table_ids always, table_id iff singleton.
func TestPublicTableIDSchema(t *testing.T) {
	single := Reservation{ReservationID: "a", Reference: "SINGLE1", TableID: "t_1"}
	pub := single.Public()
	ids, ok := pub["table_ids"].([]string)
	if !ok || len(ids) != 1 || ids[0] != "t_1" {
		t.Fatalf("single table_ids = %v", pub["table_ids"])
	}
	if pub["table_id"] != "t_1" {
		t.Fatalf("single table_id = %v", pub["table_id"])
	}
	ids[0] = "MUT"
	if single.TableID != "t_1" {
		t.Fatal("Public aliased stored set")
	}
	pair := Reservation{ReservationID: "b", Reference: "PAIR01X", TableIDs: []string{"t_1", "t_2"}}
	pub = pair.Public()
	if got := pub["table_ids"]; len(got.([]string)) != 2 {
		t.Fatalf("pair table_ids = %v", got)
	}
	if _, ok := pub["table_id"]; ok {
		t.Fatalf("pair must omit table_id: %v", pub)
	}
	if _, ok := pub["user_id"]; ok {
		t.Fatal("public must exclude user_id")
	}
	// Legacy singleton normalizes through the setter.
	leg := Reservation{ReservationID: "c", Reference: "LEGACY1", TableID: "t_2"}
	setReservationTables(&leg, reservationTableIDs(leg))
	if leg.TableID != "t_2" || len(leg.TableIDs) != 1 {
		t.Fatalf("legacy normalize = %+v", leg)
	}
}

// Fixture combinable: absent means none; entries need two known distinct
// tables; wrong shapes are malformed.
func TestFixtureCombinable(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(`[["t_2","t_1"]]`)), nil); rec.Code != 204 {
		t.Fatalf("declared order kept: %d %q", rec.Code, rec.Body.String())
	}
	r := restaurantOf(t, s)
	if len(r.Combinable) != 1 || r.Combinable[0][0] != "t_2" || r.Combinable[0][1] != "t_1" {
		t.Fatalf("declared order = %v", r.Combinable)
	}
	for _, tc := range []struct {
		name   string
		fix    string
		status int
	}{
		{"three members", `[["t_1","t_2","t_3"]]`, 422},
		{"same table", `[["t_1","t_1"]]`, 422},
		{"unknown table", `[["t_1","t_ghost"]]`, 422},
		{"non-array", `{"not":"array"}`, 400},
		{"pair non-string", `[["t_1",42]]`, 400},
	} {
		rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(tc.fix)), nil)
		if status, _ := errorCode(t, rec); status != tc.status {
			t.Errorf("%s: got %d", tc.name, status)
		}
	}
	// Absent combinable stays absent (nil), not an error.
	plain := `{"users":[],"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[],"tables":[{"id":"t1","label":"1","capacity":2}]}],"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(plain), nil); rec.Code != 204 {
		t.Fatalf("absent combinable: %d %q", rec.Code, rec.Body.String())
	}
}

// Seed table sets: singletons via table_id, pairs via table_ids with
// declared membership in declared order; both-keys and undeclared rejected.
func TestSeedTableSets(t *testing.T) {
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]]}],
		"reservations":[
			{"id":"s1","reference":"SINGLE1","user_id":"u1","restaurant_id":"r1",
			"table_id":"t_1","starts_at_local":"2027-05-06T19:00","party_size":2},
			{"id":"s2","reference":"PAIR01X","user_id":"u1","restaurant_id":"r1",
			"table_ids":["t_2","t_1"],"starts_at_local":"2027-05-06T19:00","party_size":4,"status":"cancelled"}]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	s.mu.Lock()
	single := s.state.Reservations["SINGLE1"]
	pair := s.state.Reservations["PAIR01X"]
	s.mu.Unlock()
	if single.TableID != "t_1" || strings.Join(reservationTableIDs(single), ",") != "t_1" {
		t.Fatalf("single = %+v", single)
	}
	if pair.TableID != "" || strings.Join(reservationTableIDs(pair), ",") != "t_1,t_2" {
		t.Fatalf("pair not canonical: %+v", pair)
	}
	if pair.Status != StatusCancelled {
		t.Fatalf("cancelled seed status = %q", pair.Status)
	}
	bad := strings.Replace(fix, `"table_ids":["t_2","t_1"]`, `"table_id":"t_1","table_ids":["t_1","t_2"]`, 1)
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(bad), nil); rec.Code != 422 {
		t.Fatalf("both keys: got %d", rec.Code)
	}
	undeclared := strings.Replace(fix, `[["t_1","t_2"]]`, `[]`, 1)
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(undeclared), nil); rec.Code != 422 {
		t.Fatalf("undeclared pair seed: got %d", rec.Code)
	}
}

// Detail carries combinable once the renderer (out of S2-M scope) includes
// it; state already stores declared order. Legacy receipt JSON stays
// byte-identical across export/import.
func TestDetailAndLegacyReceipt(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(pairFixture(`[["t_1","t_2"]]`)), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	s.mu.Lock()
	got := append([][]string(nil), s.state.Restaurants[0].Combinable...)
	s.mu.Unlock()
	if len(got) != 1 || got[0][0] != "t_1" || got[0][1] != "t_2" {
		t.Fatalf("stored combinable = %v", got)
	}
	rec := serveRequest(s, http.MethodGet, "/restaurants/r1", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("detail: %d", rec.Code)
	}
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	comb, ok := detail["combinable"].([]any)
	if !ok || len(comb) != 1 {
		t.Fatalf("detail combinable = %v", detail["combinable"])
	}
	pair, ok := comb[0].([]any)
	if !ok || len(pair) != 2 || pair[0] != "t_1" || pair[1] != "t_2" {
		t.Fatalf("detail pair = %v", comb[0])
	}
	// Legacy receipt: original stage-1 JSON without table_ids is untouched.
	s.withLock(func(st *State) {
		st.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")] = Receipt{
			UserID: "u1", Method: "POST", Path: "/reservations", Key: "k1",
			Body: `{"party_size":2}`, Response: `{"reference":"SINGLE1"}`, Status: 201,
		}
	})
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	rc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")]
	dst.mu.Unlock()
	if rc.Response != `{"reference":"SINGLE1"}` || rc.Body != `{"party_size":2}` {
		t.Fatalf("receipt rewritten: %+v", rc)
	}
}
