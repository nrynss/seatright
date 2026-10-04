package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

func TestResetReplacesAndRepeats(t *testing.T) {
	s := New()
	token := signup(t, s, "temp@example.com", "correct horse", "Temp")["token"].(string)
	rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(testFixture), nil)
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	if _, ok := s.Authenticate(token); ok {
		t.Fatal("reset did not clear pre-reset tokens")
	}
	// Seeded user logs in immediately.
	login := serveRequest(s, http.MethodPost, "/auth/login",
		[]byte(`{"email":"ada@example.com","password":"correct horse"}`), nil)
	if login.Code != 200 {
		t.Fatalf("seeded login: %d %q", login.Code, login.Body.String())
	}
	// Repeated reset leaves only the last fixture.
	other := `{"users":[],"restaurants":[],"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(other), nil); rec.Code != 204 {
		t.Fatalf("second reset: %d", rec.Code)
	}
	rec = serveRequest(s, http.MethodGet, "/restaurants", nil, nil)
	if got := decodeBody(t, rec)["restaurants"]; len(got.([]any)) != 0 {
		t.Fatalf("repeated reset left stale restaurants: %q", rec.Body.String())
	}
	if rec := serveRequest(s, http.MethodPost, "/auth/login",
		[]byte(`{"email":"ada@example.com","password":"correct horse"}`), nil); rec.Code != 401 {
		t.Fatalf("repeated reset left stale users: %d", rec.Code)
	}
}

func TestResetValidation(t *testing.T) {
	s := New()
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"unparseable", `{`, 400, "malformed_request"},
		{"users wrong type", `{"users":{},"restaurants":[],"reservations":[]}`, 400, "malformed_request"},
		{"user id too long", `{"users":[{"id":"` + string(make([]byte, 0)) + longID(65) + `","email":"a@b","password":"x","display_name":"d"}],"restaurants":[],"reservations":[]}`, 422, "validation_failed"},
		{"bad timezone", `{"users":[],"restaurants":[{"id":"r","name":"n","timezone":"Mars/Olympus","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[],"tables":[]}],"reservations":[]}`, 422, "validation_failed"},
		{"bad weekday", `{"users":[],"restaurants":[{"id":"r","name":"n","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"funday","opens":"18:00","closes":"23:00"}],"tables":[]}],"reservations":[]}`, 422, "validation_failed"},
		{"closes before opens", `{"users":[],"restaurants":[{"id":"r","name":"n","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"23:00","closes":"18:00"}],"tables":[]}],"reservations":[]}`, 422, "validation_failed"},
		{"slot wrong type", `{"users":[],"restaurants":[{"id":"r","name":"n","timezone":"Europe/Berlin","slot_minutes":"30","reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[],"tables":[]}],"reservations":[]}`, 400, "malformed_request"},
		{"seed unknown table", `{"users":[{"id":"u","email":"a@b","password":"x","display_name":"d"}],"restaurants":[{"id":"r","name":"n","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[],"tables":[]}],"reservations":[{"id":"s","reference":"REF123","user_id":"u","restaurant_id":"r","table_id":"t_x","starts_at_local":"2026-09-24T19:00","party_size":2}]}`, 422, "validation_failed"},
	}
	for _, tc := range cases {
		rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(tc.body), nil)
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
}

func longID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func TestRestaurantsPublic(t *testing.T) {
	s := newFoundation(t)
	rec := serveRequest(s, http.MethodGet, "/restaurants", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d", rec.Code)
	}
	v := decodeBody(t, rec)
	items, ok := v["restaurants"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("list body = %q", rec.Body.String())
	}
	first := items[0].(map[string]any)
	if first["id"] != "r_anker" || first["name"] != "Zum Anker" || first["timezone"] != "Europe/Berlin" || len(first) != 3 {
		t.Fatalf("list entry = %v", first)
	}
	rec = serveRequest(s, http.MethodGet, "/restaurants/r_anker", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("detail: %d", rec.Code)
	}
	d := decodeBody(t, rec)
	if d["slot_minutes"] != 30.0 || d["reservation_duration_minutes"] != 90.0 || d["cancellation_cutoff_minutes"] != 120.0 {
		t.Fatalf("detail rules = %q", rec.Body.String())
	}
	hours := d["opening_hours"].([]any)
	if len(hours) != 2 || hours[0].(map[string]any)["weekday"] != "thu" {
		t.Fatalf("detail hours = %v", hours)
	}
	tables := d["tables"].([]any)
	if len(tables) != 2 || tables[0].(map[string]any)["capacity"] != 2.0 {
		t.Fatalf("detail tables = %v", tables)
	}
	rec = serveRequest(s, http.MethodGet, "/restaurants/r_missing", nil, nil)
	if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant: %d %s", status, code)
	}
}

func TestSeededReservationsOccupy(t *testing.T) {
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[{"id":"s1","reference":"SEED01","user_id":"u1","restaurant_id":"r1",
		"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":2}]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	s.mu.Lock()
	res, ok := s.state.Reservations["SEED01"]
	s.mu.Unlock()
	if !ok {
		t.Fatal("seeded reservation missing from state")
	}
	if res.Status != StatusConfirmed || res.ReservationID != "s1" || res.UserID != "u1" {
		t.Fatalf("seed = %+v", res)
	}
	if res.StartsAt != "2026-09-24T19:00:00+02:00" {
		t.Fatalf("starts_at = %q", res.StartsAt)
	}
	if res.EndsAt != "2026-09-24T20:30:00+02:00" {
		t.Fatalf("ends_at = %q", res.EndsAt)
	}
	if res.CreatedAt == "" {
		t.Fatal("seeded created_at missing")
	}
	if _, ok := res.Public()["user_id"]; ok {
		t.Fatal("ordinary JSON must exclude user_id")
	}
}

func TestSeedDSTFirstOccurrence(t *testing.T) {
	loc := "Europe/Berlin"
	start, end, err := resolveSeedInstant("2026-10-25T02:30", loc, 90)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := start.Format("2006-01-02T15:04:05-07:00"); got != "2026-10-25T02:30:00+02:00" {
		t.Fatalf("ambiguous local must resolve to the first occurrence, got %s", got)
	}
	if got := end.Format("2006-01-02T15:04:05-07:00"); got != "2026-10-25T03:00:00+01:00" {
		t.Fatalf("duration is absolute across the fallback, got %s", got)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	s := newFoundation(t)
	alice := signup(t, s, "alice@example.com", "correct horse", "Alice")
	aliceToken := alice["token"].(string)
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	if export.Code != 200 {
		t.Fatalf("export: %d", export.Code)
	}
	v := decodeBody(t, export)
	if v["track"] != "tablekeeper" || v["format_version"] != 1.0 {
		t.Fatalf("export envelope = %q", export.Body.String())
	}
	state, ok := v["state"].(map[string]any)
	if !ok {
		t.Fatalf("export has no state object: %q", export.Body.String())
	}
	if _, ok := state["users"].(map[string]any); !ok {
		t.Fatalf("export state has no users object: %q", export.Body.String())
	}
	// Export is a snapshot: later writes do not change it.
	signup(t, s, "late@example.com", "correct horse", "Late")
	var before map[string]any
	if err := json.Unmarshal(export.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	beforeUsers := before["state"].(map[string]any)["users"].(map[string]any)
	if _, found := beforeUsers["late@example.com"]; found {
		t.Fatal("export snapshot changed after later writes")
	}

	// Import into a fresh destination preserves everything.
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	if _, ok := dst.Authenticate(aliceToken); !ok {
		t.Fatal("import did not preserve existing bearer token")
	}
	login := serveRequest(dst, http.MethodPost, "/auth/login",
		[]byte(`{"email":"alice@example.com","password":"correct horse"}`), nil)
	if login.Code != 200 {
		t.Fatalf("import did not preserve password hash login: %d", login.Code)
	}
	rec := serveRequest(dst, http.MethodGet, "/restaurants/r_anker", nil, nil)
	if rec.Code != 200 {
		t.Fatal("import did not preserve fixture configuration")
	}
	// Re-import restores without duplication.
	again, _ := json.Marshal(v)
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", again, nil); rec.Code != 204 {
		t.Fatalf("re-import: %d", rec.Code)
	}
	dst.mu.Lock()
	nUsers, nTokens := len(dst.state.Users), len(dst.state.Tokens)
	dst.mu.Unlock()
	if nUsers != 2 || nTokens != 1 {
		t.Fatalf("re-import duplicated state: users=%d tokens=%d", nUsers, nTokens)
	}
	// Import removes previous destination data.
	bogus := serveRequest(dst, http.MethodPost, "/_test/import",
		[]byte(`{"track":"tablekeeper","format_version":1,"state":{"users":{},"tokens":{},"restaurants":[],"reservations":{},"receipts":{}}}`), nil)
	if bogus.Code != 204 {
		t.Fatalf("wipe import: %d", bogus.Code)
	}
	if _, ok := dst.Authenticate(aliceToken); ok {
		t.Fatal("import did not remove previous credentials")
	}
	// Reset clears imported state.
	s2 := New()
	if rec := serveRequest(s2, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import for reset test: %d", rec.Code)
	}
	if rec := serveRequest(s2, http.MethodPost, "/_test/reset", []byte(`{"users":[],"restaurants":[],"reservations":[]}`), nil); rec.Code != 204 {
		t.Fatalf("reset after import: %d", rec.Code)
	}
	if _, ok := s2.Authenticate(aliceToken); ok {
		t.Fatal("reset did not clear imported credentials")
	}
}

func TestImportRejectsWithoutMutation(t *testing.T) {
	s := newFoundation(t)
	token := signup(t, s, "keep@example.com", "correct horse", "Keep")["token"].(string)
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"unparseable", `{`, 400, "malformed_request"},
		{"missing track", `{"format_version":1,"state":{}}`, 422, "validation_failed"},
		{"wrong track", `{"track":"other","format_version":1,"state":{}}`, 422, "validation_failed"},
		{"missing version", `{"track":"tablekeeper","state":{}}`, 422, "validation_failed"},
		{"wrong version", `{"track":"tablekeeper","format_version":2,"state":{}}`, 422, "validation_failed"},
		{"missing state", `{"track":"tablekeeper","format_version":1}`, 422, "validation_failed"},
		{"state wrong type", `{"track":"tablekeeper","format_version":1,"state":[]}`, 422, "validation_failed"},
		{"dangling token", `{"track":"tablekeeper","format_version":1,"state":{"users":{},"tokens":{"t":"ghost"},"restaurants":[],"reservations":{},"receipts":{}}}`, 422, "validation_failed"},
	}
	for _, tc := range cases {
		rec := serveRequest(s, http.MethodPost, "/_test/import", []byte(tc.body), nil)
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
	if _, ok := s.Authenticate(token); !ok {
		t.Fatal("failed import mutated destination credentials")
	}
	if rec := serveRequest(s, http.MethodGet, "/restaurants/r_anker", nil, nil); rec.Code != 200 {
		t.Fatal("failed import mutated destination fixtures")
	}
}

func TestImportPreservesReceiptsAndReservations(t *testing.T) {
	s := newFoundation(t)
	seed := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[{"id":"s1","reference":"SEED01","user_id":"u1","restaurant_id":"r1",
		"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":2,"status":"cancelled"}]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(seed), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	login := serveRequest(s, http.MethodPost, "/auth/login",
		[]byte(`{"email":"a@b","password":"password1"}`), nil)
	token := decodeBody(t, login)["token"].(string)
	s.withLock(func(st *State) {
		st.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")] = Receipt{
			UserID: "u1", Method: "POST", Path: "/reservations", Key: "k1",
			Body: `{"party_size":2}`, Response: `{"reference":"SEED01"}`, Status: 201,
		}
	})
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	res, ok := dst.state.Reservations["SEED01"]
	rc, okRc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")]
	dst.mu.Unlock()
	if !ok || res.Status != StatusCancelled || res.StartsAt == "" || res.CreatedAt == "" {
		t.Fatalf("reservation not preserved: %+v", res)
	}
	if !okRc || rc.Response != `{"reference":"SEED01"}` {
		t.Fatalf("receipt not preserved: %+v", rc)
	}
	if _, ok := dst.Authenticate(token); !ok {
		t.Fatal("token not preserved")
	}
}

func TestConcurrentReadsAndSignups(t *testing.T) {
	s := newFoundation(t)
	var wg sync.WaitGroup
	errs := make(chan string, 200)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := serveRequest(s, http.MethodGet, "/health", nil, nil)
			if rec.Code != 200 {
				errs <- "health failed under load"
			}
			rec = serveRequest(s, http.MethodGet, "/restaurants", nil, nil)
			if rec.Code != 200 {
				errs <- "list failed under load"
			}
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := []byte(`{"email":"load` + string(rune('a'+i%26)) + string(rune('a'+i/26)) + `@example.com","password":"correct horse","display_name":"L"}`)
			rec := serveRequest(s, http.MethodPost, "/auth/signup", body, nil)
			if rec.Code != 201 {
				errs <- "signup failed under load"
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func seedFixtureWithReference(reference string) string {
	return `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[{"id":"s1","reference":"` + reference + `","user_id":"u1","restaurant_id":"r1",
		"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":2}]}`
}

func TestSeedReferenceFormat(t *testing.T) {
	s := New()
	for _, ref := range []string{"x", "lower01", "TOO-LONG-WITH-DASH", "ABCDE", "ABCDEFGHIJKLM", "abc123", "AB CD1", ""} {
		rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(seedFixtureWithReference(ref)), nil)
		if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
			t.Errorf("reference %q: got %d %s, want 422 validation_failed", ref, status, code)
		}
	}
	for _, ref := range []string{"SEED01", "K3P7QW", "ABCDEF", "ABCDEFGHIJKL", "123456789012"} {
		rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(seedFixtureWithReference(ref)), nil)
		if rec.Code != 204 {
			t.Errorf("reference %q: got %d %q, want 204", ref, rec.Code, rec.Body.String())
		}
	}
}

func TestEmptyDisplayNameExportImport(t *testing.T) {
	s := New()
	body, _ := json.Marshal(map[string]any{"email": "blank@example.com", "password": "correct horse", "display_name": ""})
	rec := serveRequest(s, http.MethodPost, "/auth/signup", body, nil)
	if rec.Code != 201 {
		t.Fatalf("empty display_name signup: %d %q", rec.Code, rec.Body.String())
	}
	token := decodeBody(t, rec)["token"].(string)
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	if export.Code != 200 {
		t.Fatalf("export: %d", export.Code)
	}
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import of unchanged export: %d %q", rec.Code, rec.Body.String())
	}
	if got := serveRequest(dst, http.MethodGet, "/reservations", nil, authHeader(token)).Code; got != 404 {
		t.Fatalf("old token after import: %d, want 404 stub (alive)", got)
	}
	login, _ := json.Marshal(map[string]any{"email": "blank@example.com", "password": "correct horse"})
	if rec := serveRequest(dst, http.MethodPost, "/auth/login", login, nil); rec.Code != 200 {
		t.Fatalf("hash login after import: %d %q", rec.Code, rec.Body.String())
	}
}
