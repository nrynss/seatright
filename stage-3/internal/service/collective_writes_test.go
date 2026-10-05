package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

const collectiveFixture = `{
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
    {"id": "u_bob", "email": "bob@example.com", "password": "correct horse", "display_name": "Bob"}
  ],
  "restaurants": [
    {
      "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [
        {"weekday": "thu", "opens": "18:00", "closes": "23:00"},
        {"weekday": "fri", "opens": "18:00", "closes": "23:30"}
      ],
      "tables": [
        {"id": "t_1", "label": "1", "capacity": 2},
        {"id": "t_2", "label": "2", "capacity": 4},
        {"id": "t_3", "label": "3", "capacity": 4}
      ],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
      "manager_user_ids": ["u_ada"]
    }
  ],
  "reservations": []
}`

func collectiveSetup(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(collectiveFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	token := loginToken(t, s, "ada@example.com")
	return s, token, futureThursday(t)
}

func batchRaw(t *testing.T, s *Service, token, key, moves string) (int, string) {
	t.Helper()
	rec := serveRequest(s, http.MethodPost, "/reservation-moves", []byte(`{"moves":[`+moves+`]}`),
		map[string]string{"Authorization": "Bearer " + token, "Idempotency-Key": key})
	return rec.Code, rec.Body.String()
}

func batchItem(t *testing.T, body string, i int) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("batch body not JSON: %v", err)
	}
	items := v["reservations"].([]any)
	if len(items) <= i {
		t.Fatalf("batch has %d items: %s", len(items), body)
	}
	return items[i].(map[string]any)
}

func stateInts(s *Service, refs ...string) (revs []int, histLens []int, counter int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ref := range refs {
		revs = append(revs, s.state.Reservations[ref].Revision)
		histLens = append(histLens, len(s.state.Histories[ref]))
	}
	return revs, histLens, s.state.RestaurantRevisions["r_anker"]
}

func exportStateBytes(t *testing.T, s *Service) string {
	t.Helper()
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatalf("export marshal: %v", err)
	}
	return string(raw)
}

func TestCollectiveRealChanges(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cc-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refB, _ := createRef(t, s, ada, "cc-b", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 2})
	moves := `{"reference":"` + refA + `","party_size":1},{"reference":"` + refB + `","party_size":3}`
	status, body := batchRaw(t, s, ada, "cc-01", moves)
	if status != 201 {
		t.Fatalf("batch: %d %s", status, body)
	}
	a := batchItem(t, body, 0)
	b := batchItem(t, body, 1)
	if a["reference"] != refA || b["reference"] != refB {
		t.Fatalf("input order not preserved: %s", body)
	}
	if a["revision"] != float64(2) || b["revision"] != float64(2) {
		t.Fatalf("revisions not incremented: %s", body)
	}
	if a["party_size"] != float64(1) || b["party_size"] != float64(3) {
		t.Fatalf("values not applied: %s", body)
	}
	revs, histLens, counter := stateInts(s, refA, refB)
	if revs[0] != 2 || revs[1] != 2 || histLens[0] != 2 || histLens[1] != 2 || counter != 3 {
		t.Fatalf("state rev=%v hist=%v counter=%d (want 2/2, 2/2, 3)", revs, histLens, counter)
	}
}

func TestCollectiveNoOpBatch(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cn-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refP, _ := createRef(t, s, ada, "cn-p", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 6})
	moves := `{"reference":"` + refA + `"},{"reference":"` + refP + `","table_ids":["t_2","t_1"]}`
	status, body := batchRaw(t, s, ada, "cn-01", moves)
	if status != 201 {
		t.Fatalf("no-op batch: %d %s", status, body)
	}
	revs, histLens, counter := stateInts(s, refA, refP)
	if revs[0] != 1 || revs[1] != 1 || histLens[0] != 1 || histLens[1] != 1 || counter != 2 {
		t.Fatalf("no-op batch mutated: rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
	status2, body2 := batchRaw(t, s, ada, "cn-01", moves)
	if status2 != 200 || body2 != body {
		t.Fatalf("no-op replay = %d %s, want 200 identical bytes", status2, body2)
	}
}

func TestCollectiveMixedRealNoop(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cm-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refB, _ := createRef(t, s, ada, "cm-b", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_3",
		"starts_at_local": date + "T19:00", "party_size": 2})
	noopBefore := marshalBody(t, s.GetReservation(ada, refB))
	moves := `{"reference":"` + refB + `"},{"reference":"` + refA + `","party_size":1}`
	status, body := batchRaw(t, s, ada, "cm-01", moves)
	if status != 201 {
		t.Fatalf("mixed batch: %d %s", status, body)
	}
	// Output order follows input order: noop first, real second.
	noop := batchItem(t, body, 0)
	real := batchItem(t, body, 1)
	if noop["reference"] != refB || real["reference"] != refA {
		t.Fatalf("order wrong: %s", body)
	}
	raw, _ := json.Marshal(noop)
	if string(raw) != noopBefore {
		t.Fatalf("noop item changed:\n%s\n%s", raw, noopBefore)
	}
	if real["revision"] != float64(2) || real["party_size"] != float64(1) {
		t.Fatalf("real item wrong: %v", real)
	}
	revs, histLens, counter := stateInts(s, refA, refB)
	if revs[0] != 2 || revs[1] != 1 || histLens[0] != 2 || histLens[1] != 1 || counter != 3 {
		t.Fatalf("mixed state rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
}

func TestCollectiveSwaps(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cs-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refB, _ := createRef(t, s, ada, "cs-b", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refP, _ := createRef(t, s, ada, "cs-p", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 6})
	moves := `{"reference":"` + refA + `","table_id":"t_2"},` +
		`{"reference":"` + refB + `","table_id":"t_1"},` +
		`{"reference":"` + refP + `","table_ids":["t_2","t_3"]}`
	status, body := batchRaw(t, s, ada, "cs-01", moves)
	if status != 201 {
		t.Fatalf("swap batch: %d %s", status, body)
	}
	a := batchItem(t, body, 0)
	b := batchItem(t, body, 1)
	p := batchItem(t, body, 2)
	if a["table_id"] != "t_2" || b["table_id"] != "t_1" {
		t.Fatalf("singleton swap wrong: %s", body)
	}
	raw, _ := json.Marshal(p["table_ids"])
	if string(raw) != `["t_2","t_3"]` {
		t.Fatalf("pair move wrong: %v", p["table_ids"])
	}
	if p["revision"] != float64(2) {
		t.Fatalf("pair revision wrong: %v", p)
	}
	revs, histLens, counter := stateInts(s, refA, refB, refP)
	for i, want := range []int{2, 2, 2} {
		if revs[i] != want || histLens[i] != want {
			t.Fatalf("swap state rev=%v hist=%v", revs, histLens)
		}
	}
	if counter != 4 {
		t.Fatalf("counter = %d, want 4 (3 creates + 1 batch)", counter)
	}
}

func TestCollectiveMixedPolicy(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cp-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": "2027-06-10T19:00", "party_size": 4})
	refB, _ := createRef(t, s, ada, "cp-b", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4})
	// Ada manages r_anker, so her own token publishes.
	if res := s.PublishPolicy(ada, "r_anker", "cp-pol", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	noopBefore := marshalBody(t, s.GetReservation(ada, refB))
	// A crosses into the published date (adopts v1 terms and the 60-minute
	// end); B is an untouched no-op retaining v0 terms.
	moves := `{"reference":"` + refA + `","table_id":"t_1","starts_at_local":"` + date + `T19:00"},` +
		`{"reference":"` + refB + `"}`
	status, body := batchRaw(t, s, ada, "cp-01", moves)
	if status != 201 {
		t.Fatalf("mixed-policy batch: %d %s", status, body)
	}
	a := batchItem(t, body, 0)
	b := batchItem(t, body, 1)
	if a["revision"] != float64(2) || a["table_id"] != "t_1" {
		t.Fatalf("moved item = %v", a)
	}
	if end, ok := a["ends_at"].(string); !ok || len(end) < 16 || end[:16] != date+"T20:00" {
		t.Fatalf("moved end not recomputed: %v", a["ends_at"])
	}
	if a["accepted_terms"].(map[string]any)["policy_version"] != float64(1) {
		t.Fatalf("moved terms not adopted: %v", a["accepted_terms"])
	}
	raw, _ := json.Marshal(b)
	if string(raw) != noopBefore {
		t.Fatalf("noop item changed:\n%s\n%s", raw, noopBefore)
	}
	if v := b["accepted_terms"].(map[string]any)["policy_version"]; v != float64(0) {
		t.Fatalf("noop adopted new terms: %v", b)
	}
}

func TestCollectiveExpectedRevision(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "ce-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	// Matching per-move revision succeeds.
	status, body := batchRaw(t, s, ada, "ce-01",
		`{"reference":"`+refA+`","party_size":1,"expected_revision":1}`)
	if status != 201 {
		t.Fatalf("matching revision batch: %d %s", status, body)
	}
	// Mismatched revision fails before field validation (unknown table
	// alone would be 404, cutoff/validation never runs).
	status, body = batchRaw(t, s, ada, "ce-02",
		`{"reference":"`+refA+`","table_id":"t_nope","expected_revision":1}`)
	if status != 409 || batchCode(t, body) != "stale_revision" {
		t.Fatalf("stale precedence = %d %s", status, body)
	}
	// Invalid types are 422, including a huge-but-valid stale integer going
	// 409 rather than tripping any ceiling.
	for _, tc := range []struct {
		name string
		frag string
		code int
		want string
	}{
		{"bool", `"expected_revision":true`, 422, "validation_failed"},
		{"null", `"expected_revision":null`, 422, "validation_failed"},
		{"string", `"expected_revision":"2"`, 422, "validation_failed"},
		{"fraction", `"expected_revision":2.5`, 422, "validation_failed"},
		{"zero", `"expected_revision":0`, 422, "validation_failed"},
	} {
		status, body := batchRaw(t, s, ada, "ce-"+tc.name,
			`{"reference":"`+refA+`",`+tc.frag+`}`)
		if status != tc.code || batchCode(t, body) != tc.want {
			t.Fatalf("%s = %d %s", tc.name, status, body)
		}
	}
	status, body = batchRaw(t, s, ada, "ce-huge",
		`{"reference":"`+refA+`","expected_revision":9000000000000001}`)
	if status != 409 || batchCode(t, body) != "stale_revision" {
		t.Fatalf("huge stale = %d %s", status, body)
	}
	if revs, _, _ := stateInts(s, refA); revs[0] != 2 {
		t.Fatalf("revision after revision tests = %v", revs)
	}
}

func batchCode(t *testing.T, body string) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("batch error body not JSON: %v", err)
	}
	return v["error"].(map[string]any)["code"].(string)
}

func TestCollectiveFailureRollback(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cf-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	refB, _ := createRef(t, s, ada, "cf-b", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 2})
	before := exportStateBytes(t, s)
	// The second move collides with the first move's result: whole batch
	// fails with no partial history, counter, flags or receipt.
	status, body := batchRaw(t, s, ada, "cf-01",
		`{"reference":"`+refA+`","table_id":"t_3"},`+
			`{"reference":"`+refB+`","table_id":"t_3"}`)
	if status != 409 || batchCode(t, body) != "table_unavailable" {
		t.Fatalf("conflicting batch = %d %s", status, body)
	}
	if exportStateBytes(t, s) != before {
		t.Fatalf("failed batch changed exported state")
	}
	revs, histLens, counter := stateInts(s, refA, refB)
	if revs[0] != 1 || revs[1] != 1 || histLens[0] != 1 || histLens[1] != 1 || counter != 2 {
		t.Fatalf("failed batch leaked: rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
	// The failed key stays reusable for a corrected body.
	status, body = batchRaw(t, s, ada, "cf-01",
		`{"reference":"`+refA+`","table_id":"t_3"}`)
	if status != 201 {
		t.Fatalf("reused key: %d %s", status, body)
	}
	if item := batchItem(t, body, 0); item["revision"] != float64(2) {
		t.Fatalf("reused item = %v", item)
	}
}

func TestCollectiveSameKey50(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "ck-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2})
	moves := `{"reference":"` + refA + `","party_size":1}`
	const n = 50
	codes := make([]int, n)
	bodies := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			codes[idx], bodies[idx] = batchRaw(t, s, ada, "ck-50", moves)
		}(i)
	}
	wg.Wait()
	created, replayed := 0, ""
	for i, code := range codes {
		switch code {
		case 201:
			created++
			replayed = bodies[i]
		case 200:
			if bodies[i] != replayed && replayed != "" {
				t.Fatalf("replay bytes differ")
			}
		default:
			t.Fatalf("concurrent status = %d", code)
		}
	}
	if created != 1 {
		t.Fatalf("concurrent batch: %d x201, want exactly 1", created)
	}
	for _, body := range bodies {
		if body != replayed {
			t.Fatalf("replay not byte-identical")
		}
	}
	revs, histLens, counter := stateInts(s, refA)
	if revs[0] != 2 || histLens[0] != 2 || counter != 2 {
		t.Fatalf("concurrent state rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
}

func TestCollectiveCompetingExpectedRevision(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	refA, _ := createRef(t, s, ada, "cc2-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4})
	var wg sync.WaitGroup
	type outcome struct {
		status int
		body   string
	}
	results := make([]outcome, 2)
	parties := []string{"1", "2"}
	_ = parties
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			frag := `{"reference":"` + refA + `","party_size":` + string(rune('1'+idx)) + `,"expected_revision":1}`
			st, bd := batchRaw(t, s, ada, "cc2-k"+string(rune('a'+idx)), frag)
			results[idx] = outcome{st, bd}
		}(i)
	}
	wg.Wait()
	ok, stale := 0, 0
	for _, r := range results {
		switch {
		case r.status == 201:
			ok++
		case r.status == 409 && batchCode(t, r.body) == "stale_revision":
			stale++
		default:
			t.Fatalf("competing batch = %d %s", r.status, r.body)
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("competing batches: %d ok + %d stale, want 1+1", ok, stale)
	}
	revs, histLens, counter := stateInts(s, refA)
	if revs[0] != 2 || histLens[0] != 2 || counter != 2 {
		t.Fatalf("loser leaked: rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
}

func TestCollectiveSeedNoOp(t *testing.T) {
	s := New()
	future := futureThursday(t)
	resetWith(t, s, seedAda, seedAnkerTables,
		seedRes("s1", "SEEDOFF", "u_ada", "r_anker", "t_1", future+"T18:07", 2, "confirmed"))
	ada := loginToken(t, s, "ada@example.com")
	beforeRaw, _ := json.Marshal(s.GetReservation(ada, "SEEDOFF").Body)
	status, body := batchRaw(t, s, ada, "seed-noop", `{"reference":"SEEDOFF"}`)
	if status != 201 {
		t.Fatalf("seed no-op batch: %d %s", status, body)
	}
	itemRaw, _ := json.Marshal(batchItem(t, body, 0))
	if string(itemRaw) != string(beforeRaw) {
		t.Fatalf("off-grid seed not retained:\n%s\n%s", itemRaw, beforeRaw)
	}
	if revs, histLens, counter := stateInts(s, "SEEDOFF"); revs[0] != 1 || histLens[0] != 1 || counter != 0 {
		t.Fatalf("seed no-op mutated: rev=%v hist=%v counter=%d", revs, histLens, counter)
	}
}
