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

// postWrite issues an idempotent POST with optional key ("-" means no header,
// "" means an empty header value).
func postWrite(t *testing.T, s *Service, path, token, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	if key != "-" {
		headers["Idempotency-Key"] = key
	}
	return serveRequest(s, http.MethodPost, path, []byte(body), headers)
}

func loginToken(t *testing.T, s *Service, email string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": "correct horse"})
	rec := serveRequest(s, http.MethodPost, "/auth/login", body, nil)
	if rec.Code != 200 {
		t.Fatalf("login %s: %d %q", email, rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)["token"].(string)
}

func createJSON(restaurant, table, local string, party int) string {
	return fmt.Sprintf(`{"restaurant_id":%q,"table_id":%q,"starts_at_local":%q,"party_size":%d}`,
		restaurant, table, local, party)
}

func TestCreateHTTPHappyPath(t *testing.T) {
	s := newFoundation(t)
	token := loginToken(t, s, "ada@example.com")
	rec := postWrite(t, s, "/reservations", token, "k-happy", createJSON("r_anker", "t_2", "2026-09-24T19:00", 4))
	if rec.Code != 201 {
		t.Fatalf("create: %d %q", rec.Code, rec.Body.String())
	}
	v := decodeBody(t, rec)
	if v["status"] != StatusConfirmed || v["table_id"] != "t_2" || v["party_size"] != 4.0 {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if v["starts_at"] != "2026-09-24T19:00:00+02:00" || v["ends_at"] != "2026-09-24T20:30:00+02:00" {
		t.Fatalf("bounds = %q", rec.Body.String())
	}
	checkTimestampNumeric(t, "created_at", v["created_at"].(string))
	if _, present := v["user_id"]; present {
		t.Fatal("create response leaks user_id")
	}
	if !validReference(v["reference"].(string)) {
		t.Fatalf("bad reference %v", v["reference"])
	}
	// Occupancy is visible in availability.
	avail := s.Availability(queryOf("r_anker", "2026-09-24", "4"))
	slots := avail.Body.(map[string]any)["slots"].([]any)
	for _, sl := range slots {
		m := sl.(map[string]any)
		if m["starts_at_local"] == "2026-09-24T19:00" && len(m["available_table_ids"].([]any)) != 0 {
			t.Fatalf("booked table still offered: %v", m)
		}
	}
}

func TestCreateHTTPValidation(t *testing.T) {
	s := newFoundation(t)
	token := loginToken(t, s, "ada@example.com")
	// Occupy t_2 at 19:00 first.
	if rec := postWrite(t, s, "/reservations", token, "k-seed", createJSON("r_anker", "t_2", "2026-09-24T19:00", 4)); rec.Code != 201 {
		t.Fatalf("setup create: %d", rec.Code)
	}
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"overlap", createJSON("r_anker", "t_2", "2026-09-24T19:00", 2), 409, "table_unavailable"},
		{"adjacent ok is 201", createJSON("r_anker", "t_2", "2026-09-24T20:30", 2), 201, ""},
		{"off grid", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T19:15","party_size":2}`, 422, "not_on_slot_grid"},
		{"before open", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T17:30","party_size":2}`, 422, "outside_opening_hours"},
		{"ends after close", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T22:30","party_size":2}`, 422, "outside_opening_hours"},
		{"over capacity", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":3}`, 422, "party_exceeds_capacity"},
		{"party string", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":"2"}`, 422, "validation_failed"},
		{"party bool", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":true}`, 422, "validation_failed"},
		{"party float", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":2.5}`, 422, "validation_failed"},
		{"party zero", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":0}`, 422, "validation_failed"},
		{"party missing", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00"}`, 422, "validation_failed"},
		{"local spaced", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24 18:00","party_size":2}`, 422, "validation_failed"},
		{"local seconds", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00:00","party_size":2}`, 422, "validation_failed"},
		{"local offset", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00:00+02:00","party_size":2}`, 422, "validation_failed"},
		{"local number", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":42,"party_size":2}`, 400, "malformed_request"},
		{"table bool", `{"restaurant_id":"r_anker","table_id":true,"starts_at_local":"2026-09-24T18:00","party_size":2}`, 400, "malformed_request"},
		{"unknown restaurant", `{"restaurant_id":"r_no","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":2}`, 404, "not_found"},
		{"unknown table", `{"restaurant_id":"r_anker","table_id":"t_no","starts_at_local":"2026-09-24T18:00","party_size":2}`, 404, "not_found"},
		{"gap time", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-03-29T02:30","party_size":2}`, 422, "invalid_local_time"},
		{"unparseable", `{`, 400, "malformed_request"},
		{"array", `[]`, 400, "malformed_request"},
	}
	for i, tc := range cases {
		rec := postWrite(t, s, "/reservations", token, fmt.Sprintf("k-v-%d", i), tc.body)
		if tc.status == 201 {
			if rec.Code != 201 {
				t.Errorf("%s: got %d %q", tc.name, rec.Code, rec.Body.String())
			}
			continue
		}
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
	// Past bookings are accepted over HTTP too.
	if rec := postWrite(t, s, "/reservations", token, "k-past", createJSON("r_anker", "t_1", "2020-01-02T19:00", 2)); rec.Code != 201 {
		t.Fatalf("past create: %d %q", rec.Code, rec.Body.String())
	}
}

func TestCreateKeySemantics(t *testing.T) {
	setup := func(t *testing.T) (*Service, string, string) {
		s := New()
		resetWith(t, s, seedAda+","+seedBob, seedAnkerTables, "")
		return s, loginToken(t, s, "ada@example.com"), loginToken(t, s, "bob@example.com")
	}
	t.Run("missing and empty key", func(t *testing.T) {
		s, ada, _ := setup(t)
		body := createJSON("r_anker", "t_1", "2026-09-24T18:00", 2)
		if status, code := errorCode(t, postWrite(t, s, "/reservations", ada, "-", body)); status != 400 || code != "missing_idempotency_key" {
			t.Fatalf("no header: %d %s", status, code)
		}
		if status, code := errorCode(t, postWrite(t, s, "/reservations", ada, "", body)); status != 400 || code != "missing_idempotency_key" {
			t.Fatalf("empty header: %d %s", status, code)
		}
		if status, code := errorCode(t, postWrite(t, s, "/reservations", "", "k", body)); status != 401 || code != "unauthenticated" {
			t.Fatalf("no token: %d %s", status, code)
		}
	})
	t.Run("key length bounds", func(t *testing.T) {
		s, ada, _ := setup(t)
		long := strings.Repeat("k", 256)
		rec := postWrite(t, s, "/reservations", ada, long, createJSON("r_anker", "t_1", "2026-09-24T18:00", 2))
		if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
			t.Fatalf("256-char key: %d %s", status, code)
		}
		rec = postWrite(t, s, "/reservations", ada, strings.Repeat("k", 255), createJSON("r_anker", "t_1", "2026-09-24T18:00", 2))
		if rec.Code != 201 {
			t.Fatalf("255-char key: %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("replay before validation", func(t *testing.T) {
		s, ada, _ := setup(t)
		first := postWrite(t, s, "/reservations", ada, "k-re", createJSON("r_anker", "t_1", "2026-09-24T18:00", 2))
		if first.Code != 201 {
			t.Fatalf("first: %d", first.Code)
		}
		orig := first.Body.String()
		// Same logical body, different order/whitespace: replay.
		replay := postWrite(t, s, "/reservations", ada, "k-re", `{"party_size" : 2 , "table_id" : "t_1" , "starts_at_local" : "2026-09-24T18:00" , "restaurant_id" : "r_anker"}`)
		if replay.Code != 200 || replay.Body.String() != orig {
			t.Fatalf("replay: %d %q vs %q", replay.Code, replay.Body.String(), orig)
		}
		// Same key, different body that is also invalid: reuse wins.
		other := postWrite(t, s, "/reservations", ada, "k-re", `{"party_size":"many"}`)
		if status, code := errorCode(t, other); status != 409 || code != "idempotency_key_reuse" {
			t.Fatalf("reuse: %d %s", status, code)
		}
	})
	t.Run("scoped by user and path", func(t *testing.T) {
		s, ada, bob := setup(t)
		if rec := postWrite(t, s, "/reservations", ada, "k-shared", createJSON("r_anker", "t_1", "2026-09-24T18:00", 2)); rec.Code != 201 {
			t.Fatalf("ada: %d", rec.Code)
		}
		if rec := postWrite(t, s, "/reservations", bob, "k-shared", createJSON("r_anker", "t_1", "2026-09-24T20:30", 2)); rec.Code != 201 {
			t.Fatalf("bob same key: %d %q", rec.Code, rec.Body.String())
		}
		// Same key on the moves path is independent.
		moves := `{"moves":[]}`
		rec := postWrite(t, s, "/reservation-moves", ada, "k-shared", moves)
		if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
			t.Fatalf("same key other path should run independently: %d %s", status, code)
		}
	})
	t.Run("unknown fields and identity", func(t *testing.T) {
		s, ada, _ := setup(t)
		withExtra := `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2026-09-24T18:00","party_size":2,"vip":true}`
		first := postWrite(t, s, "/reservations", ada, "k-x", withExtra)
		if first.Code != 201 {
			t.Fatalf("extra fields: %d %q", first.Code, first.Body.String())
		}
		replay := postWrite(t, s, "/reservations", ada, "k-x", withExtra)
		if replay.Code != 200 || replay.Body.String() != first.Body.String() {
			t.Fatalf("replay with extras: %d", replay.Code)
		}
		other := postWrite(t, s, "/reservations", ada, "k-x", withExtra[:len(withExtra)-1]+`,"vip":false}`)
		if status, code := errorCode(t, other); status != 409 || code != "idempotency_key_reuse" {
			t.Fatalf("differing unknown field: %d %s", status, code)
		}
	})
	t.Run("failed key reusable", func(t *testing.T) {
		s, ada, _ := setup(t)
		bad := postWrite(t, s, "/reservations", ada, "k-fail", createJSON("r_anker", "t_1", "2026-09-24T18:00", 99))
		if bad.Code != 422 {
			t.Fatalf("bad: %d", bad.Code)
		}
		if rec := postWrite(t, s, "/reservations", ada, "k-fail", createJSON("r_anker", "t_1", "2026-09-24T18:00", 2)); rec.Code != 201 {
			t.Fatalf("reuse after failure: %d %q", rec.Code, rec.Body.String())
		}
	})
}

func TestCreateReplayAfterMutationAndImport(t *testing.T) {
	s := newFoundation(t)
	token := loginToken(t, s, "ada@example.com")
	body := createJSON("r_anker", "t_1", futureThursday(t)+"T18:00", 2)
	first := postWrite(t, s, "/reservations", token, "k-mut", body)
	if first.Code != 201 {
		t.Fatalf("create: %d", first.Code)
	}
	orig, ref := first.Body.String(), decodeBody(t, first)["reference"].(string)
	serveRequest(s, http.MethodPost, "/reservations/"+ref+"/cancel", nil, authHeader(token))
	replay := postWrite(t, s, "/reservations", token, "k-mut", body)
	if replay.Code != 200 || replay.Body.String() != orig {
		t.Fatalf("replay after cancel: %d %q vs %q", replay.Code, replay.Body.String(), orig)
	}
	if v := decodeBody(t, replay); v["status"] != StatusConfirmed {
		t.Fatalf("replay must carry the original response: %q", replay.Body.String())
	}
	// The replay changed nothing: still cancelled on lookup.
	lookup := serveRequest(s, http.MethodGet, "/reservations/"+ref, nil, authHeader(token))
	if decodeBody(t, lookup)["status"] != StatusCancelled {
		t.Fatalf("replay mutated: %q", lookup.Body.String())
	}
	// And across export/import into a fresh process state.
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	after := postWrite(t, dst, "/reservations", token, "k-mut", body)
	if after.Code != 200 || after.Body.String() != orig {
		t.Fatalf("replay after import: %d %q", after.Code, orig)
	}
}

func TestCreateConcurrentIdentical(t *testing.T) {
	s := newFoundation(t)
	token := loginToken(t, s, "ada@example.com")
	body := createJSON("r_anker", "t_2", "2026-09-24T19:00", 2)
	const n = 50
	var wg sync.WaitGroup
	type outcome struct {
		status int
		body   string
	}
	results := make([]outcome, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := postWrite(t, s, "/reservations", token, "k-race", body)
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
			want = r.body
		case 200:
			replayed++
			if want != "" && r.body != want {
				t.Fatalf("replay body differs")
			}
		default:
			t.Fatalf("status %d", r.status)
		}
	}
	if created != 1 || replayed != n-1 {
		t.Fatalf("created=%d replayed=%d", created, replayed)
	}
	for _, r := range results {
		if r.body != want {
			t.Fatal("bodies differ across concurrent identical creates")
		}
	}
	s.mu.Lock()
	count := len(s.state.Reservations)
	s.mu.Unlock()
	if count != 1 {
		t.Fatalf("state holds %d reservations", count)
	}
}

func TestCreateConcurrentCompeting(t *testing.T) {
	s := New()
	resetWith(t, s, seedAda+","+seedBob, seedAnkerTables, "")
	ada, bob := loginToken(t, s, "ada@example.com"), loginToken(t, s, "bob@example.com")
	body := createJSON("r_anker", "t_2", "2026-09-24T19:00", 2)
	var wg sync.WaitGroup
	results := make([]int, 2)
	wg.Add(2)
	go func() { defer wg.Done(); results[0] = postWrite(t, s, "/reservations", ada, "k-a", body).Code }()
	go func() { defer wg.Done(); results[1] = postWrite(t, s, "/reservations", bob, "k-b", body).Code }()
	wg.Wait()
	has201, has409 := false, false
	for _, st := range results {
		switch st {
		case 201:
			has201 = true
		case 409:
			has409 = true
		default:
			t.Fatalf("status %d", st)
		}
	}
	if !has201 || !has409 {
		t.Fatalf("results = %v, want one 201 and one 409", results)
	}
}
