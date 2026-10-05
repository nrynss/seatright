package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testFixture = `{
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}
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
        {"id": "t_2", "label": "2", "capacity": 4}
      ]
    }
  ],
  "reservations": []
}`

func newFoundation(t *testing.T) *Service {
	t.Helper()
	s := New()
	res := s.Reset([]byte(testFixture))
	if res.Status != 204 {
		t.Fatalf("reset fixture: status %d body %v", res.Status, res.Body)
	}
	return s
}

func serveRequest(s *Service, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("response is not a JSON object: %v (body %q)", err, rec.Body.String())
	}
	return v
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	v := decodeBody(t, rec)
	errObj, ok := v["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope in %q", rec.Body.String())
	}
	code, _ := errObj["code"].(string)
	return rec.Code, code
}

func authHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func signup(t *testing.T, s *Service, email, password, display string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password, "display_name": display})
	rec := serveRequest(s, http.MethodPost, "/auth/signup", body, nil)
	if rec.Code != 201 {
		t.Fatalf("signup: status %d body %q", rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)
}

func TestHealth(t *testing.T) {
	s := New()
	rec := serveRequest(s, http.MethodGet, "/health", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("health status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("health content-type = %q", ct)
	}
	v := decodeBody(t, rec)
	if v["status"] != "ok" || len(v) != 1 {
		t.Fatalf("health body = %q", rec.Body.String())
	}
}

func TestIdempotentWritesNeedAuthThenKey(t *testing.T) {
	s := newFoundation(t)
	token := signup(t, s, "stub@example.com", "correct horse", "Stub")["token"].(string)
	// POST /reservations and POST /reservation-moves are live idempotent
	// writes: 401 without a valid token, 400 without a key.
	for _, tc := range []struct{ method, path string }{
		{"POST", "/reservations"},
		{"POST", "/reservation-moves"},
	} {
		rec := serveRequest(s, tc.method, tc.path, []byte(`{}`), nil)
		if status, code := errorCode(t, rec); status != 401 || code != "unauthenticated" {
			t.Errorf("%s %s without token: got %d %s", tc.method, tc.path, status, code)
		}
		rec = serveRequest(s, tc.method, tc.path, []byte(`{}`), authHeader("bogus"))
		if status, code := errorCode(t, rec); status != 401 || code != "unauthenticated" {
			t.Errorf("%s %s bogus token: got %d %s", tc.method, tc.path, status, code)
		}
		rec = serveRequest(s, tc.method, tc.path, []byte(`{}`), authHeader(token))
		if status, code := errorCode(t, rec); status != 400 || code != "missing_idempotency_key" {
			t.Errorf("%s %s authed without key: got %d %s", tc.method, tc.path, status, code)
		}
	}
	// Live reservation reads authenticate, then report unknown references.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/reservations"},
		{"GET", "/reservations/K3P7QW"},
		{"POST", "/reservations/K3P7QW/cancel"},
		{"PATCH", "/reservations/K3P7QW"},
	} {
		rec := serveRequest(s, tc.method, tc.path, []byte(`{}`), nil)
		if status, code := errorCode(t, rec); status != 401 || code != "unauthenticated" {
			t.Errorf("%s %s without token: got %d %s", tc.method, tc.path, status, code)
		}
	}
	rec := serveRequest(s, http.MethodGet, "/reservations", nil, authHeader(token))
	if rec.Code != 200 {
		t.Fatalf("authed empty list: %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticFailsCleanlyWithoutWebBuild(t *testing.T) {
	s := newFoundation(t)
	t.Setenv("WEB_DIR", t.TempDir()+"/missing-dist")
	for _, path := range []string{"/", "/signup", "/login", "/lookup", "/no-such-page", "/api/nope"} {
		rec := serveRequest(s, http.MethodGet, path, nil, nil)
		if rec.Code == 500 {
			t.Fatalf("GET %s: 500", path)
		}
		if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
			t.Fatalf("GET %s: got %d %s", path, status, code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Fatalf("GET %s: content-type %q", path, ct)
		}
	}
}

func TestUnknownMethodIsNotFound(t *testing.T) {
	s := newFoundation(t)
	rec := serveRequest(s, http.MethodDelete, "/restaurants", nil, nil)
	if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
		t.Fatalf("DELETE /restaurants: got %d %s", status, code)
	}
}

func TestPolicyRoutesHTTP(t *testing.T) {
	s := New()
	if res := s.Reset([]byte(policyFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	publish := func(token, key string, body []byte) *httptest.ResponseRecorder {
		headers := map[string]string{"Idempotency-Key": key}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
		return serveRequest(s, http.MethodPost, "/restaurants/r_anker/policies", body, headers)
	}
	// Unknown-field body is accepted; response carries the flat policy.
	rec := publish(mgr, "http-01", []byte(`{"effective_from":"2027-06-01","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4},"zzz":1}`))
	if rec.Code != 201 {
		t.Fatalf("publish: %d %q", rec.Code, rec.Body.String())
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["policy_version"] != float64(1) || v["effective_from"] != "2027-06-01" {
		t.Fatalf("publish body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	// Replay returns the identical bytes.
	rec2 := publish(mgr, "http-01", []byte(`{"effective_from":"2027-06-01","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4},"zzz":1}`))
	if rec2.Code != 200 || rec2.Body.String() != rec.Body.String() {
		t.Fatalf("replay = %d %q, want 200 %q", rec2.Code, rec2.Body.String(), rec.Body.String())
	}
	// Auth matrix over HTTP.
	if status, code := errorCode(t, publish("", "http-02", validPolicyBody("2027-06-02"))); status != 401 || code != "unauthenticated" {
		t.Fatalf("no token = %d %s", status, code)
	}
	if status, code := errorCode(t, publish(diner, "http-03", validPolicyBody("2027-06-03"))); status != 403 || code != "forbidden" {
		t.Fatalf("non-manager = %d %s", status, code)
	}
	rec = serveRequest(s, http.MethodPost, "/restaurants/r_nope/policies", validPolicyBody("2027-06-04"), map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "http-04"})
	if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant = %d %s", status, code)
	}
	// Missing key and malformed bodies.
	rec = serveRequest(s, http.MethodPost, "/restaurants/r_anker/policies", validPolicyBody("2027-06-05"), map[string]string{"Authorization": "Bearer " + mgr})
	if status, code := errorCode(t, rec); status != 400 || code != "missing_idempotency_key" {
		t.Fatalf("missing key = %d %s", status, code)
	}
	rec = publish(mgr, "http-05", []byte(`oops`))
	if status, code := errorCode(t, rec); status != 400 || code != "malformed_request" {
		t.Fatalf("malformed = %d %s", status, code)
	}
	// Public list; exact suffix matching with no catch-all leaks.
	rec = serveRequest(s, http.MethodGet, "/restaurants/r_anker/policies", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %q", rec.Code, rec.Body.String())
	}
	rec = serveRequest(s, http.MethodGet, "/restaurants/r_anker/policies/extra", nil, nil)
	if status, _ := errorCode(t, rec); status != 404 {
		t.Fatalf("deep policies path = %d", status)
	}
	rec = serveRequest(s, http.MethodPost, "/restaurants/r_anker", validPolicyBody("2027-06-06"), map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "http-06"})
	if status, _ := errorCode(t, rec); status != 404 {
		t.Fatalf("POST detail = %d", status)
	}
	rec = serveRequest(s, http.MethodGet, "/restaurants/r_anker/policies/", nil, nil)
	if status, _ := errorCode(t, rec); status != 404 {
		t.Fatalf("trailing slash = %d", status)
	}
	// Explain over HTTP: matrix shape and guard codes.
	rec = serveRequest(s, http.MethodGet, "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2&explain=true", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("explain: %d %q", rec.Code, rec.Body.String())
	}
	var avail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &avail); err != nil {
		t.Fatal(err)
	}
	slot := avail["slots"].([]any)[0].(map[string]any)
	if _, ok := slot["explain"]; !ok {
		t.Fatalf("explain missing: %q", rec.Body.String())
	}
	for _, bad := range []string{"false", "1", ""} {
		rec = serveRequest(s, http.MethodGet, "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2&explain="+bad, nil, nil)
		if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
			t.Fatalf("explain=%q: %d %s", bad, status, code)
		}
	}
	rec = serveRequest(s, http.MethodGet, "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2", nil, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &avail); err != nil {
		t.Fatal(err)
	}
	if _, ok := avail["slots"].([]any)[0].(map[string]any)["explain"]; ok {
		t.Fatalf("no-explain response carries explain")
	}
}

func TestSeriesHTTPRoutes(t *testing.T) {
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}],"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	login, _ := json.Marshal(map[string]any{"email": "a@b", "password": "password1"})
	lr := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	var lb map[string]any
	if err := json.Unmarshal(lr.Body.Bytes(), &lb); err != nil {
		t.Fatal(err)
	}
	tok := lb["token"].(string)
	auth := map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-anchor"}
	ar := serveRequest(s, http.MethodPost, "/reservations", []byte(`{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T19:00","party_size":1}`), auth)
	if ar.Code != 201 {
		t.Fatalf("anchor: %d %s", ar.Code, ar.Body.String())
	}
	var ab map[string]any
	if err := json.Unmarshal(ar.Body.Bytes(), &ab); err != nil {
		t.Fatal(err)
	}
	ref := ab["reference"].(string)
	// POST /series through the router.
	sr := serveRequest(s, http.MethodPost, "/series", []byte(`{"anchor_reference":`+quote(ref)+`,"count":2,"interval_weeks":1}`),
		map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-series"})
	if sr.Code != 201 {
		t.Fatalf("series: %d %s", sr.Code, sr.Body.String())
	}
	var sb map[string]any
	if err := json.Unmarshal(sr.Body.Bytes(), &sb); err != nil {
		t.Fatal(err)
	}
	sid, _ := sb["series_id"].(string)
	if sid == "" {
		t.Fatal("no series_id")
	}
	// Replay through the router is 200 identical.
	rp := serveRequest(s, http.MethodPost, "/series", []byte(`{"anchor_reference":`+quote(ref)+`,"count":2,"interval_weeks":1}`),
		map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-series"})
	if rp.Code != 200 || rp.Body.String() != sr.Body.String() {
		t.Fatalf("replay: %d", rp.Code)
	}
	// GET /series/{id} through the router.
	gr := serveRequest(s, http.MethodGet, "/series/"+sid, nil, map[string]string{"Authorization": "Bearer " + tok})
	if gr.Code != 200 {
		t.Fatalf("get: %d %s", gr.Code, gr.Body.String())
	}
	// No token is 404, not 401.
	nt := serveRequest(s, http.MethodGet, "/series/"+sid, nil, nil)
	if nt.Code != 404 {
		t.Fatalf("no token = %d", nt.Code)
	}
	// Unknown series is 404.
	un := serveRequest(s, http.MethodGet, "/series/nosuch", nil, map[string]string{"Authorization": "Bearer " + tok})
	if un.Code != 404 {
		t.Fatalf("unknown = %d", un.Code)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestReplanPreviewHTTPRoutes(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(replanFixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	login, _ := json.Marshal(map[string]any{"email": "ada@example.com", "password": "correct horse"})
	lr := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	var lb map[string]any
	if err := json.Unmarshal(lr.Body.Bytes(), &lb); err != nil {
		t.Fatal(err)
	}
	tok := lb["token"].(string)
	auth := map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-rp-1"}
	first := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure), auth)
	if first.Code != 201 {
		t.Fatalf("preview route: %d %s", first.Code, first.Body.String())
	}
	var fb map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &fb); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"plan_id", "restaurant_revision", "closure", "assignments", "moved_count", "unused_seats"} {
		if _, ok := fb[k]; !ok {
			t.Fatalf("missing key %q in %v", k, fb)
		}
	}
	if len(fb) != 6 {
		t.Fatalf("shape must have exactly 6 keys: %v", fb)
	}
	// Replay through the router is 200 with identical bytes.
	rp := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure), auth)
	if rp.Code != 200 || rp.Body.String() != first.Body.String() {
		t.Fatalf("replay: %d", rp.Code)
	}
	// Routing matrix: wrong method, trailing slash, deeper paths, empty id
	// and the future apply path are all 404.
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/restaurants/r_anker/replans"},
		{http.MethodPut, "/restaurants/r_anker/replans"},
		{http.MethodPost, "/restaurants/r_anker/replans/"},
		{http.MethodPost, "/restaurants/r_anker/replans/abc/apply"},
		{http.MethodPost, "/restaurants//replans"},
		{http.MethodPost, "/restaurants/r_anker/replans/abc"},
	} {
		rec := serveRequest(s, tc.method, tc.path, []byte(replanClosure), auth)
		if rec.Code != 404 {
			t.Fatalf("%s %s = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
	// Missing key through the router is 400.
	nk := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure),
		map[string]string{"Authorization": "Bearer " + tok})
	if nk.Code != 400 {
		t.Fatalf("missing key = %d", nk.Code)
	}
	// Missing token through the router is 401.
	nt := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure),
		map[string]string{"Idempotency-Key": "http-rp-2"})
	if nt.Code != 401 {
		t.Fatalf("missing token = %d", nt.Code)
	}
}

func TestReplanApplyHTTPRoutes(t *testing.T) {
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(replanFixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	login, _ := json.Marshal(map[string]any{"email": "ada@example.com", "password": "correct horse"})
	lr := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	var lb map[string]any
	if err := json.Unmarshal(lr.Body.Bytes(), &lb); err != nil {
		t.Fatal(err)
	}
	tok := lb["token"].(string)
	pvauth := map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-rp-pv"}
	pv := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans", []byte(replanClosure), pvauth)
	if pv.Code != 201 {
		t.Fatalf("preview: %d %s", pv.Code, pv.Body.String())
	}
	var pb map[string]any
	if err := json.Unmarshal(pv.Body.Bytes(), &pb); err != nil {
		t.Fatal(err)
	}
	pid, _ := pb["plan_id"].(string)
	if pid == "" {
		t.Fatal("no plan_id")
	}
	// Apply through the router: exact 3-key shape, reference order.
	auth := map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "http-rp-ap"}
	ap := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans/"+pid+"/apply", []byte(`{}`), auth)
	if ap.Code != 201 {
		t.Fatalf("apply: %d %s", ap.Code, ap.Body.String())
	}
	var ab map[string]any
	if err := json.Unmarshal(ap.Body.Bytes(), &ab); err != nil {
		t.Fatal(err)
	}
	if len(ab) != 3 {
		t.Fatalf("apply shape keys: %v", ab)
	}
	recs, _ := ab["reservations"].([]any)
	if len(recs) != 3 {
		t.Fatalf("reservations: %v", ab)
	}
	r0, _ := recs[0].(map[string]any)
	r1, _ := recs[1].(map[string]any)
	r2, _ := recs[2].(map[string]any)
	if r0["reference"] != "BKAAAA" || r1["reference"] != "BKBBBB" || r2["reference"] != "BKCCCC" {
		t.Fatalf("reference order: %v", ab)
	}
	// Replay through the router is 200 identical.
	rp := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans/"+pid+"/apply", []byte(`{}`), auth)
	if rp.Code != 200 || rp.Body.String() != ap.Body.String() {
		t.Fatalf("replay: %d", rp.Code)
	}
	// Routing matrix: wrong method, trailing slash, deeper paths, empty
	// ids and unknown plans are 404 without touching state.
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/restaurants/r_anker/replans/" + pid + "/apply"},
		{http.MethodPut, "/restaurants/r_anker/replans/" + pid + "/apply"},
		{http.MethodPost, "/restaurants/r_anker/replans/" + pid + "/apply/"},
		{http.MethodPost, "/restaurants/r_anker/replans/" + pid + "/apply/extra"},
		{http.MethodPost, "/restaurants/r_anker/replans//apply"},
		{http.MethodPost, "/restaurants//replans/" + pid + "/apply"},
		{http.MethodPost, "/restaurants/r_anker/replans/nosuchplan/apply"},
	} {
		rec := serveRequest(s, tc.method, tc.path, []byte(`{}`), auth)
		if rec.Code != 404 {
			t.Fatalf("%s %s = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
	// Missing key is 400, missing token is 401.
	nk := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans/"+pid+"/apply", []byte(`{}`),
		map[string]string{"Authorization": "Bearer " + tok})
	if nk.Code != 400 {
		t.Fatalf("missing key = %d", nk.Code)
	}
	nt := serveRequest(s, http.MethodPost, "/restaurants/r_anker/replans/"+pid+"/apply", []byte(`{}`),
		map[string]string{"Idempotency-Key": "http-rp-ap2"})
	if nt.Code != 401 {
		t.Fatalf("missing token = %d", nt.Code)
	}
}
