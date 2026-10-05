package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// wire round-trips a Result body through JSON so assertions observe wire
// types regardless of calling the service directly or over HTTP.
func wire(t *testing.T, res Result) map[string]any {
	t.Helper()
	raw, err := json.Marshal(res.Body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	return v
}

// comboFixture is the working restaurant for combination tests: three tables
// and two declared pairs sharing t_2.
const comboFixture = `{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin",
	"slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
	"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
	"combinable":[["t_1","t_2"],["t_2","t_3"]]}`

const pairDate = "2027-06-17"

func comboSetup(t *testing.T) (*Service, string) {
	t.Helper()
	s := New()
	resetWith(t, s, seedAda+","+seedBob, comboFixture, "")
	return s, loginToken(t, s, "ada@example.com")
}

func comboBodyJ(ids []string, local string, party int) map[string]any {
	raw := make([]any, 0, len(ids))
	for _, id := range ids {
		raw = append(raw, id)
	}
	return map[string]any{
		"restaurant_id": "r_anker", "table_ids": raw,
		"starts_at_local": local, "party_size": float64(party),
	}
}

func comboJSON(ids string, local string, party int) string {
	return fmt.Sprintf(`{"restaurant_id":"r_anker","table_ids":[%s],"starts_at_local":%q,"party_size":%d}`,
		ids, local, party)
}

func comboCreate(t *testing.T, s *Service, token, key, ids, local string, party int) map[string]any {
	t.Helper()
	rec := postWrite(t, s, "/reservations", token, key, comboJSON(ids, local, party))
	if rec.Code != 201 {
		t.Fatalf("pair create: %d %q", rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)
}

func TestPairCreateSchema(t *testing.T) {
	s, token := comboSetup(t)
	v := comboCreate(t, s, token, "k-p1", `"t_1","t_2"`, pairDate+"T18:00", 6)
	ids, ok := v["table_ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("table_ids = %v", v["table_ids"])
	}
	if _, present := v["table_id"]; present {
		t.Fatalf("pair response must omit table_id: %v", v)
	}
	if v["status"] != StatusConfirmed {
		t.Fatalf("status = %v", v["status"])
	}
	// Singleton responses carry both table_ids and table_id.
	rec := postWrite(t, s, "/reservations", token, "k-s1",
		`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T20:30","party_size":2}`)
	if rec.Code != 201 {
		t.Fatalf("singleton: %d %q", rec.Code, rec.Body.String())
	}
	w := decodeBody(t, rec)
	if ids := w["table_ids"].([]any); len(ids) != 1 || ids[0] != "t_3" {
		t.Fatalf("singleton table_ids = %v", w["table_ids"])
	}
	if w["table_id"] != "t_3" {
		t.Fatalf("singleton table_id = %v", w["table_id"])
	}
}

func TestPairSelectionValidation(t *testing.T) {
	s, token := comboSetup(t)
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"both formats", `{"restaurant_id":"r_anker","table_id":"t_1","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "validation_failed"},
		{"missing both", `{"restaurant_id":"r_anker","starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "validation_failed"},
		{"empty array", `{"restaurant_id":"r_anker","table_ids":[],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "validation_failed"},
		{"duplicates", `{"restaurant_id":"r_anker","table_ids":["t_1","t_1"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "validation_failed"},
		{"three tables", `{"restaurant_id":"r_anker","table_ids":["t_1","t_2","t_3"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "combination_not_allowed"},
		{"undeclared pair", `{"restaurant_id":"r_anker","table_ids":["t_1","t_3"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "combination_not_allowed"},
		{"unknown member", `{"restaurant_id":"r_anker","table_ids":["t_1","t_9"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 404, "not_found"},
		{"ids not array", `{"restaurant_id":"r_anker","table_ids":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}`, 400, "malformed_request"},
		{"member not string", `{"restaurant_id":"r_anker","table_ids":["t_1",4],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 400, "malformed_request"},
		{"single not string", `{"restaurant_id":"r_anker","table_id":["t_1"],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 400, "malformed_request"},
		{"empty member", `{"restaurant_id":"r_anker","table_ids":["t_1",""],"starts_at_local":"2027-06-17T18:00","party_size":2}`, 422, "validation_failed"},
		{"over summed capacity", `{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":7}`, 422, "party_exceeds_capacity"},
		{"over single capacity", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":3}`, 422, "party_exceeds_capacity"},
	}
	for i, tc := range cases {
		rec := postWrite(t, s, "/reservations", token, fmt.Sprintf("k-pv-%d", i), tc.body)
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
}

func TestPairCanonicalOrder(t *testing.T) {
	s, token := comboSetup(t)
	v := comboCreate(t, s, token, "k-rev", `"t_2","t_1"`, pairDate+"T18:00", 6)
	if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("reversed input not canonicalized: %v", v["table_ids"])
	}
	// A merely reversed PATCH names the same set: no-op with identical values.
	ref := v["reference"].(string)
	before := serveRequest(s, http.MethodGet, "/reservations/"+ref, nil, authHeader(token)).Body.String()
	rec := serveRequest(s, http.MethodPatch, "/reservations/"+ref,
		[]byte(`{"table_ids":["t_2","t_1"]}`), authHeader(token))
	if rec.Code != 200 || rec.Body.String() != before {
		t.Fatalf("reversed pair PATCH changed the record: %d %q vs %q", rec.Code, rec.Body.String(), before)
	}
}

func TestPairOccupancy(t *testing.T) {
	s, token := comboSetup(t)
	comboCreate(t, s, token, "k-occ", `"t_1","t_2"`, pairDate+"T18:00", 6)
	conflicts := []struct{ name, body string }{
		{"single member t_2", `{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T18:30","party_size":2}`},
		{"single member t_1", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}`},
		{"shared pair", comboJSON(`"t_2","t_3"`, pairDate+"T18:00", 4)},
		{"same pair overlap", comboJSON(`"t_1","t_2"`, pairDate+"T19:00", 4)},
	}
	for i, tc := range conflicts {
		rec := postWrite(t, s, "/reservations", token, fmt.Sprintf("k-oc-%d", i), tc.body)
		if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
	// Adjacent pair booking succeeds; disjoint single succeeds.
	if rec := postWrite(t, s, "/reservations", token, "k-adj", comboJSON(`"t_1","t_2"`, pairDate+"T19:30", 4)); rec.Code != 201 {
		t.Fatalf("adjacent pair: %d %q", rec.Code, rec.Body.String())
	}
	if rec := postWrite(t, s, "/reservations", token, "k-free",
		`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T18:00","party_size":2}`); rec.Code != 201 {
		t.Fatalf("disjoint single: %d %q", rec.Code, rec.Body.String())
	}
}

func TestPairAvailabilityOptions(t *testing.T) {
	s, _ := comboSetup(t)
	slotsOf := func(party string) []any {
		res := s.Availability(queryOf("r_anker", pairDate, party))
		if res.Status != 200 {
			t.Fatalf("availability: %d %v", res.Status, res.Body)
		}
		return wire(t, res)["slots"].([]any)
	}
	slots := slotsOf("6")
	first := slots[0].(map[string]any)
	if len(first["available_table_ids"].([]any)) != 0 {
		t.Fatalf("party 6 singles = %v", first["available_table_ids"])
	}
	opts := first["available_options"].([]any)
	if len(opts) != 2 {
		t.Fatalf("party 6 options = %v", first["available_options"])
	}
	o1 := opts[0].(map[string]any)
	if ids := o1["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" || o1["capacity"] != 6.0 {
		t.Fatalf("first option = %v", o1)
	}
	o2 := opts[1].(map[string]any)
	if ids := o2["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_2" || ids[1] != "t_3" || o2["capacity"] != 8.0 {
		t.Fatalf("second option = %v", o2)
	}
	// Occupy [t_1,t_2] at 18:00: both pair options vanish from intersecting
	// slots (shared t_2), singles keep original semantics.
	s2, token := comboSetup(t)
	comboCreate(t, s2, token, "k-av", `"t_1","t_2"`, pairDate+"T18:00", 6)
	res := s2.Availability(queryOf("r_anker", pairDate, "6"))
	slots = wire(t, res)["slots"].([]any)
	byLocal := map[string]map[string]any{}
	for _, sl := range slots {
		m := sl.(map[string]any)
		byLocal[m["starts_at_local"].(string)] = m
		if _, ok := m["available_options"]; !ok {
			t.Fatalf("slot lacks available_options: %v", m)
		}
	}
	if n := len(byLocal[pairDate+"T18:00"]["available_options"].([]any)); n != 0 {
		t.Fatalf("18:00 options = %v", byLocal[pairDate+"T18:00"]["available_options"])
	}
	if n := len(byLocal[pairDate+"T18:30"]["available_options"].([]any)); n != 0 {
		t.Fatalf("18:30 options = %v", byLocal[pairDate+"T18:30"]["available_options"])
	}
	if n := len(byLocal[pairDate+"T19:30"]["available_options"].([]any)); n != 2 {
		t.Fatalf("19:30 options = %v", byLocal[pairDate+"T19:30"]["available_options"])
	}
	// Singles for a small party still list every free table in fixture order.
	res = s2.Availability(queryOf("r_anker", pairDate, "2"))
	slots = wire(t, res)["slots"].([]any)
	for _, sl := range slots {
		m := sl.(map[string]any)
		if m["starts_at_local"] == pairDate+"T18:00" {
			got := []string{}
			for _, id := range m["available_table_ids"].([]any) {
				got = append(got, id.(string))
			}
			if len(got) != 1 || got[0] != "t_3" {
				t.Fatalf("18:00 party-2 tables = %v", got)
			}
		}
	}
}

func TestPairPatchCancel(t *testing.T) {
	s, token := comboSetup(t)
	// Single -> pair.
	rec := postWrite(t, s, "/reservations", token, "k-sp",
		`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}`)
	if rec.Code != 201 {
		t.Fatalf("setup: %d", rec.Code)
	}
	ref := decodeBody(t, rec)["reference"].(string)
	patch := serveRequest(s, http.MethodPatch, "/reservations/"+ref,
		[]byte(`{"table_ids":["t_2","t_1"],"party_size":6}`), authHeader(token))
	if patch.Code != 200 {
		t.Fatalf("single->pair: %d %q", patch.Code, patch.Body.String())
	}
	v := decodeBody(t, patch)
	if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("patched tables = %v", v)
	}
	if _, present := v["table_id"]; present {
		t.Fatalf("pair PATCH response keeps table_id: %v", v)
	}
	// Pair -> single retains table_id.
	patch = serveRequest(s, http.MethodPatch, "/reservations/"+ref,
		[]byte(`{"table_id":"t_3","party_size":2}`), authHeader(token))
	if patch.Code != 200 {
		t.Fatalf("pair->single: %d %q", patch.Code, patch.Body.String())
	}
	if v := decodeBody(t, patch); v["table_id"] != "t_3" {
		t.Fatalf("back to single: %q", patch.Body.String())
	}
	// Conflicting pair PATCH rolls back atomically.
	comboCreate(t, s, token, "k-block", `"t_1","t_2"`, pairDate+"T20:30", 4)
	before := serveRequest(s, http.MethodGet, "/reservations/"+ref, nil, authHeader(token)).Body.String()
	patch = serveRequest(s, http.MethodPatch, "/reservations/"+ref,
		[]byte(`{"table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T20:30"}`), authHeader(token))
	if status, code := errorCode(t, patch); status != 409 || code != "table_unavailable" {
		t.Fatalf("conflicting PATCH: %d %s", status, code)
	}
	after := serveRequest(s, http.MethodGet, "/reservations/"+ref, nil, authHeader(token)).Body.String()
	if before != after {
		t.Fatalf("failed PATCH mutated:\n%s\n%s", before, after)
	}
	// Cancelling a pair frees both members.
	pairRef := comboCreate(t, s, token, "k-cancel", `"t_1","t_2"`, pairDate+"T18:00", 4)["reference"].(string)
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+pairRef+"/cancel", nil, authHeader(token)); rec.Code != 200 {
		t.Fatalf("cancel pair: %d", rec.Code)
	}
	res := s.Availability(queryOf("r_anker", pairDate, "4"))
	for _, sl := range wire(t, res)["slots"].([]any) {
		m := sl.(map[string]any)
		if m["starts_at_local"] == pairDate+"T18:00" {
			got := map[string]bool{}
			for _, id := range m["available_table_ids"].([]any) {
				got[id.(string)] = true
			}
			if !got["t_2"] {
				t.Fatalf("cancelled pair member t_2 not freed: %v", m)
			}
			found := false
			for _, opt := range m["available_options"].([]any) {
				ids := opt.(map[string]any)["table_ids"].([]any)
				if len(ids) == 2 && ids[0] == "t_1" && ids[1] == "t_2" {
					found = true
				}
			}
			if !found {
				t.Fatalf("cancelled pair option missing: %v", m)
			}
		}
	}
}

func TestPairSeededLookup(t *testing.T) {
	s := New()
	seed := `{"id":"sP","reference":"SEEDP1","user_id":"u_ada","restaurant_id":"r_anker",` +
		`"table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T18:00","party_size":6}`
	resetWith(t, s, seedAda, comboFixture, seed)
	token := loginToken(t, s, "ada@example.com")
	rec := serveRequest(s, http.MethodGet, "/reservations/SEEDP1", nil, authHeader(token))
	if rec.Code != 200 {
		t.Fatalf("lookup: %d %q", rec.Code, rec.Body.String())
	}
	v := decodeBody(t, rec)
	if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_2" || ids[1] != "t_3" {
		t.Fatalf("seeded pair tables = %v", v)
	}
	if _, present := v["table_id"]; present {
		t.Fatalf("seeded pair lookup keeps table_id: %v", v)
	}
	// The seeded pair blocks an intersecting pair and the shared single.
	rec = postWrite(t, s, "/reservations", token, "k-sb", comboJSON(`"t_1","t_2"`, pairDate+"T18:00", 4))
	if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
		t.Fatalf("intersecting pair: %d %s", status, code)
	}
	rec = postWrite(t, s, "/reservations", token, "k-ss",
		`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T18:00","party_size":2}`)
	if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
		t.Fatalf("shared single: %d %s", status, code)
	}
}

func TestPairMoves(t *testing.T) {
	s, _ := comboSetup(t)
	// Seed: singleton A on t_1@19:00, pair B on [t_2,t_3]@19:00.
	resetWith(t, s, seedAda, comboFixture,
		`{"id":"sA","reference":"MVPA01","user_id":"u_ada","restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2},`+
			`{"id":"sB","reference":"MVPB02","user_id":"u_ada","restaurant_id":"r_anker","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T19:00","party_size":6}`)
	token := loginToken(t, s, "ada@example.com")
	// Move singleton onto the pair's shared member -> conflict, rollback.
	rec := postWrite(t, s, "/reservation-moves", token, "m-pc",
		`{"moves":[{"reference":"MVPA01","table_ids":["t_1","t_2"]}]}`)
	if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
		t.Fatalf("shared member move: %d %s", status, code)
	}
	// Swap: A takes [t_2,t_3]... occupied; instead move B to t_1-adjacent time
	// and A onto the freed pair in one batch.
	rec = postWrite(t, s, "/reservation-moves", token, "m-ps",
		`{"moves":[{"reference":"MVPB02","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T20:30"},{"reference":"MVPA01","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T19:00"}]}`)
	if rec.Code != 201 {
		t.Fatalf("pair swap batch: %d %q", rec.Code, rec.Body.String())
	}
	items := decodeBody(t, rec)["reservations"].([]any)
	if items[0].(map[string]any)["reference"] != "MVPB02" || items[1].(map[string]any)["reference"] != "MVPA01" {
		t.Fatalf("input order lost: %q", rec.Body.String())
	}
	// Input-order error precedence with table_ids.
	rec = postWrite(t, s, "/reservation-moves", token, "m-po2",
		`{"moves":[{"reference":"NOPE01"},{"reference":"MVPB02","table_ids":["t_9"]}]}`)
	if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
		t.Fatalf("input order: %d %s", status, code)
	}
}

func TestPairConcurrency(t *testing.T) {
	s, token := comboSetup(t)
	body := comboJSON(`"t_1","t_2"`, pairDate+"T18:00", 6)
	const n = 50
	var wg sync.WaitGroup
	type outcome struct {
		status  int
		payload string
	}
	results := make([]outcome, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := postWrite(t, s, "/reservations", token, "k-prace", body)
			results[i] = outcome{rec.Code, rec.Body.String()}
		}(i)
	}
	wg.Wait()
	created, replayed := 0, 0
	var want string
	for _, r := range results {
		switch r.status {
		case 201:
			created++
			want = r.payload
		case 200:
			replayed++
		default:
			t.Fatalf("status %d", r.status)
		}
	}
	if created != 1 || replayed != n-1 {
		t.Fatalf("created=%d replayed=%d", created, replayed)
	}
	for _, r := range results {
		if r.payload != want {
			t.Fatal("concurrent pair bodies differ")
		}
	}
	// Competing pair vs single on a shared member: exactly one 201.
	s2, token2 := comboSetup(t)
	var wg2 sync.WaitGroup
	got := make([]int, 2)
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		got[0] = postWrite(t, s2, "/reservations", token2, "k-ca", comboJSON(`"t_1","t_2"`, pairDate+"T18:00", 6)).Code
	}()
	go func() {
		defer wg2.Done()
		got[1] = postWrite(t, s2, "/reservations", token2, "k-cb",
			`{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T18:00","party_size":2}`).Code
	}()
	wg2.Wait()
	if !((got[0] == 201 && got[1] == 409) || (got[0] == 409 && got[1] == 201)) {
		t.Fatalf("competing pair/single: %v", got)
	}
}

func TestPairNonTransitive(t *testing.T) {
	s, token := comboSetup(t)
	// [t_1,t_2] and [t_2,t_3] declared; [t_1,t_3] still forbidden.
	rec := postWrite(t, s, "/reservations", token, "k-nt", comboJSON(`"t_1","t_3"`, pairDate+"T18:00", 6))
	if status, code := errorCode(t, rec); status != 422 || code != "combination_not_allowed" {
		t.Fatalf("transitive pair: %d %s", status, code)
	}
	if !strings.Contains(comboFixture, `"combinable":[["t_1","t_2"],["t_2","t_3"]]`) {
		t.Fatal("fixture drift")
	}
}

func TestPairOmittedSelectionRetention(t *testing.T) {
	s, token := comboSetup(t)
	ref := comboCreate(t, s, token, "k-ret", `"t_1","t_2"`, pairDate+"T18:00", 4)["reference"].(string)
	original := serveRequest(s, http.MethodGet, "/reservations/"+ref, nil, authHeader(token)).Body.String()

	patch := func(body string) *httptest.ResponseRecorder {
		return serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(body), authHeader(token))
	}
	// Empty PATCH retains everything byte-identically.
	if rec := patch(`{}`); rec.Code != 200 || rec.Body.String() != original {
		t.Fatalf("PATCH {}: %d %q vs %q", rec.Code, rec.Body.String(), original)
	}
	// Party-only PATCH keeps the canonical pair.
	rec := patch(`{"party_size":3}`)
	if rec.Code != 200 {
		t.Fatalf("party-only PATCH: %d %q", rec.Code, rec.Body.String())
	}
	if v := decodeBody(t, rec); v["party_size"] != 3.0 {
		t.Fatalf("party not applied: %q", rec.Body.String())
	} else if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("pair lost on party-only PATCH: %q", rec.Body.String())
	} else if v["reference"] == nil || v["reservation_id"] == nil {
		t.Fatalf("identity lost: %q", rec.Body.String())
	}
	// Time-only PATCH keeps the canonical pair and moves occupancy.
	rec = patch(`{"starts_at_local":"` + pairDate + `T20:30"}`)
	if rec.Code != 200 {
		t.Fatalf("time-only PATCH: %d %q", rec.Code, rec.Body.String())
	}
	v := decodeBody(t, rec)
	if v["starts_at_local"] != pairDate+"T20:30" || v["ends_at"] != pairDate+"T22:00:00+02:00" {
		t.Fatalf("time not applied: %q", rec.Body.String())
	}
	if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("pair lost on time-only PATCH: %q", rec.Body.String())
	}
	// Old slot freed, new slot occupied on both members.
	res := s.Availability(queryOf("r_anker", pairDate, "2"))
	byLocal := map[string][]string{}
	for _, sl := range wire(t, res)["slots"].([]any) {
		m := sl.(map[string]any)
		got := []string{}
		for _, id := range m["available_table_ids"].([]any) {
			got = append(got, id.(string))
		}
		byLocal[m["starts_at_local"].(string)] = got
	}
	if got := byLocal[pairDate+"T18:00"]; len(got) != 3 {
		t.Fatalf("old slot not freed: %v", got)
	}
	if got := byLocal[pairDate+"T20:30"]; len(got) != 1 || got[0] != "t_3" {
		t.Fatalf("new slot not occupied on both members: %v", got)
	}
	// Time+party without selection keeps the pair.
	rec = patch(`{"starts_at_local":"` + pairDate + `T19:30","party_size":5}`)
	if rec.Code != 200 {
		t.Fatalf("time+party PATCH: %d %q", rec.Code, rec.Body.String())
	}
	if v := decodeBody(t, rec); v["party_size"] != 5.0 {
		t.Fatalf("fields not applied: %q", rec.Body.String())
	} else if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("pair lost on time+party PATCH: %q", rec.Body.String())
	}
	// Strictness preserved: both formats in one PATCH is still 422.
	rec = patch(`{"table_id":"t_1","table_ids":["t_1","t_2"]}`)
	if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
		t.Fatalf("both formats PATCH: %d %s", status, code)
	}
}

func TestPairBatchRetention(t *testing.T) {
	s, token := comboSetup(t)
	refPair := comboCreate(t, s, token, "k-bp", `"t_1","t_2"`, pairDate+"T18:00", 4)["reference"].(string)
	rec := postWrite(t, s, "/reservations", token, "k-bs",
		`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T20:30","party_size":2}`)
	if rec.Code != 201 {
		t.Fatalf("singleton setup: %d", rec.Code)
	}
	refSingle := decodeBody(t, rec)["reference"].(string)
	originalPair := serveRequest(s, http.MethodGet, "/reservations/"+refPair, nil, authHeader(token)).Body.String()

	batch := func(key, moves string) *httptest.ResponseRecorder {
		return postWrite(t, s, "/reservation-moves", token, key, `{"moves":[`+moves+`]}`)
	}
	// No-op pair batch returns current state unchanged.
	rec = batch("m-noop", `{"reference":"`+refPair+`"}`)
	if rec.Code != 201 || rec.Body.String() != `{"reservations":[`+originalPair+`]}` {
		t.Fatalf("no-op batch: %d %q", rec.Code, rec.Body.String())
	}
	// Time-only and party-only batch moves retain the pair.
	rec = batch("m-time", `{"reference":"`+refPair+`","starts_at_local":"2027-06-17T20:30"}`)
	if rec.Code != 201 {
		t.Fatalf("time-only batch: %d %q", rec.Code, rec.Body.String())
	}
	if v := decodeBody(t, rec)["reservations"].([]any)[0].(map[string]any); v["starts_at_local"] != pairDate+"T20:30" {
		t.Fatalf("batch time not applied: %q", rec.Body.String())
	} else if ids := v["table_ids"].([]any); len(ids) != 2 || ids[0] != "t_1" || ids[1] != "t_2" {
		t.Fatalf("pair lost on time-only batch: %q", rec.Body.String())
	}
	rec = batch("m-party", `{"reference":"`+refPair+`","party_size":5}`)
	if rec.Code != 201 {
		t.Fatalf("party-only batch: %d %q", rec.Code, rec.Body.String())
	}
	// Mixed singleton+pair batch: singleton keeps table_id, pair keeps set.
	rec = batch("m-mix", `{"reference":"`+refSingle+`","party_size":1},{"reference":"`+refPair+`"}`)
	if rec.Code != 201 {
		t.Fatalf("mixed batch: %d %q", rec.Code, rec.Body.String())
	}
	items := decodeBody(t, rec)["reservations"].([]any)
	if items[0].(map[string]any)["reference"] != refSingle || items[1].(map[string]any)["reference"] != refPair {
		t.Fatalf("input order lost: %q", rec.Body.String())
	}
	if items[0].(map[string]any)["table_id"] != "t_3" {
		t.Fatalf("singleton lost table_id: %q", rec.Body.String())
	}
	if ids := items[1].(map[string]any)["table_ids"].([]any); len(ids) != 2 {
		t.Fatalf("pair lost set: %q", rec.Body.String())
	}
	// Batch replay returns the immutable original JSON.
	rec = batch("m-mix", `{"reference":"`+refSingle+`","party_size":1},{"reference":"`+refPair+`"}`)
	if rec.Code != 200 {
		t.Fatalf("batch replay: %d %q", rec.Code, rec.Body.String())
	}
	// Conflicting batch rolls back records and receipt.
	s.mu.Lock()
	receiptsBefore := len(s.state.Receipts)
	s.mu.Unlock()
	rec = batch("m-conf", `{"reference":"`+refPair+`","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T20:30"}`)
	if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
		t.Fatalf("conflicting batch: %d %s", status, code)
	}
	lookup := serveRequest(s, http.MethodGet, "/reservations/"+refPair, nil, authHeader(token)).Body.String()
	if !strings.Contains(lookup, `"table_ids":["t_1","t_2"]`) {
		t.Fatalf("failed batch mutated: %q", lookup)
	}
	s.mu.Lock()
	receiptsAfter := len(s.state.Receipts)
	s.mu.Unlock()
	if receiptsAfter != receiptsBefore {
		t.Fatalf("failed batch claimed a receipt")
	}
}
