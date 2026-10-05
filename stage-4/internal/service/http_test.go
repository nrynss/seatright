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

func amendRouteSetup(t *testing.T) (*Service, map[string]string, string, []string) {
	t.Helper()
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"},
		{"id":"u2","email":"c@d","password":"password22","display_name":"C"},
		{"id":"u3","email":"e@f","password":"password333","display_name":"E"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]],
		"manager_user_ids":["u1","u2"]}],
		"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	toks := map[string]string{}
	for u, em := range map[string]string{"u1": "a@b", "u2": "c@d", "u3": "e@f"} {
		pw := map[string]string{"u1": "password1", "u2": "password22", "u3": "password333"}[u]
		lb, _ := json.Marshal(map[string]any{"email": em, "password": pw})
		lr := serveRequest(s, http.MethodPost, "/auth/login", lb, nil)
		if lr.Code != 200 {
			t.Fatalf("login %s: %d %q", u, lr.Code, lr.Body.String())
		}
		var lm map[string]any
		if err := json.Unmarshal(lr.Body.Bytes(), &lm); err != nil {
			t.Fatal(err)
		}
		toks[u] = lm["token"].(string)
	}
	anchor := `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}`
	ar := serveRequest(s, http.MethodPost, "/reservations", []byte(anchor),
		map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-anchor"})
	if ar.Code != 201 {
		t.Fatalf("anchor: %d %q", ar.Code, ar.Body.String())
	}
	var ab map[string]any
	if err := json.Unmarshal(ar.Body.Bytes(), &ab); err != nil {
		t.Fatal(err)
	}
	adopt, _ := json.Marshal(map[string]any{"anchor_reference": ab["reference"], "count": 3, "interval_weeks": 1})
	sr := serveRequest(s, http.MethodPost, "/series", adopt,
		map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-adopt"})
	if sr.Code != 201 {
		t.Fatalf("adopt: %d %q", sr.Code, sr.Body.String())
	}
	var sb map[string]any
	if err := json.Unmarshal(sr.Body.Bytes(), &sb); err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, o := range sb["occurrences"].([]any) {
		refs = append(refs, o.(map[string]any)["reference"].(string))
	}
	return s, toks, sb["series_id"].(string), refs
}

func httpExportState(t *testing.T, s *Service) map[string]any {
	t.Helper()
	rec := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("export: %d", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	return env["state"].(map[string]any)
}

func amendPostHTTP(s *Service, tok, sid, key, body string, withKey bool) *httptest.ResponseRecorder {
	h := map[string]string{}
	if tok != "" {
		h["Authorization"] = "Bearer " + tok
	}
	if withKey {
		h["Idempotency-Key"] = key
	}
	return serveRequest(s, http.MethodPost, "/series/"+sid+"/amend", []byte(body), h)
}

func amendReceipt(t *testing.T, st map[string]any, path, key string) map[string]any {
	t.Helper()
	for _, v := range st["receipts"].(map[string]any) {
		rm, _ := v.(map[string]any)
		if rm["method"] == "POST" && rm["path"] == path && rm["key"] == key {
			return rm
		}
	}
	t.Fatalf("no receipt for %s %s", path, key)
	return nil
}

func TestSeriesAmendHTTPRoutes(t *testing.T) {
	s, toks, sid, _ := amendRouteSetup(t)
	valid := `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`
	if rec := amendPostHTTP(s, toks["u1"], sid, "rt-r1", valid, true); rec.Code != 201 {
		t.Fatalf("valid POST = %d %q", rec.Code, rec.Body.String())
	}
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := serveRequest(s, m, "/series/"+sid+"/amend", []byte(valid),
			map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-rx"})
		if rec.Code != 404 {
			t.Fatalf("%s amend = %d, want 404", m, rec.Code)
		}
	}
	for _, p := range []string{
		"/series/" + sid + "/amend/",
		"/series/" + sid + "/amend/x",
		"/series//amend",
	} {
		rec := serveRequest(s, http.MethodPost, p, []byte(valid),
			map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-rx"})
		if rec.Code != 404 {
			t.Fatalf("POST %s = %d, want 404", p, rec.Code)
		}
	}
	// Ordinary GET /series/{id} still works through the router.
	if rec := serveRequest(s, http.MethodGet, "/series/"+sid, nil,
		map[string]string{"Authorization": "Bearer " + toks["u1"]}); rec.Code != 200 {
		t.Fatalf("GET series = %d", rec.Code)
	}
	// GET on the amend leaf is an unknown series, still 404.
	if rec := serveRequest(s, http.MethodGet, "/series/"+sid+"/amend", nil,
		map[string]string{"Authorization": "Bearer " + toks["u1"]}); rec.Code != 404 {
		t.Fatalf("GET amend leaf = %d", rec.Code)
	}
}

func TestSeriesAmendHTTPAuth(t *testing.T) {
	s, toks, sid, _ := amendRouteSetup(t)
	valid := `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`
	if rec := amendPostHTTP(s, "", sid, "rt-a1", valid, true); rec.Code != 401 {
		t.Fatalf("no token = %d", rec.Code)
	}
	if rec := amendPostHTTP(s, "bogus", sid, "rt-a2", valid, true); rec.Code != 401 {
		t.Fatalf("bogus token = %d", rec.Code)
	}
	// Plain diner who owns nothing here.
	if st, code := errorCode(t, amendPostHTTP(s, toks["u3"], sid, "rt-a3", valid, true)); st != 404 || code != "not_found" {
		t.Fatalf("other owner = %d %s", st, code)
	}
	// Manager who does not own the series.
	if st, code := errorCode(t, amendPostHTTP(s, toks["u2"], sid, "rt-a4", valid, true)); st != 404 || code != "not_found" {
		t.Fatalf("manager non-owner = %d %s", st, code)
	}
	// Unknown series with an otherwise valid body and token.
	if st, code := errorCode(t, amendPostHTTP(s, toks["u1"], "nosuch", "rt-a5", valid, true)); st != 404 || code != "not_found" {
		t.Fatalf("unknown series = %d %s", st, code)
	}
	// Header idempotency scope is the exact amend path: the same key
	// string on the adoption path is an independent request, not a replay.
	anchor2 := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T21:30","party_size":1}`
	ar := serveRequest(s, http.MethodPost, "/reservations", []byte(anchor2),
		map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-scope-a"})
	if ar.Code != 201 {
		t.Fatalf("second anchor: %d %q", ar.Code, ar.Body.String())
	}
	var ab map[string]any
	if err := json.Unmarshal(ar.Body.Bytes(), &ab); err != nil {
		t.Fatal(err)
	}
	scopeAmend := amendPostHTTP(s, toks["u1"], sid, "rt-scope", valid, true)
	if scopeAmend.Code != 201 {
		t.Fatalf("scoped amend: %d %q", scopeAmend.Code, scopeAmend.Body.String())
	}
	adopt2, _ := json.Marshal(map[string]any{"anchor_reference": ab["reference"], "count": 2, "interval_weeks": 1})
	rec := serveRequest(s, http.MethodPost, "/series", adopt2,
		map[string]string{"Authorization": "Bearer " + toks["u1"], "Idempotency-Key": "rt-scope"})
	if rec.Code != 201 {
		t.Fatalf("same key other path = %d, want fresh 201", rec.Code)
	}
}

func TestSeriesAmendHTTPValidation(t *testing.T) {
	s, toks, sid, _ := amendRouteSetup(t)
	pre, _ := json.Marshal(httpExportState(t, s))
	bad := []struct {
		name string
		body string
	}{
		{"rev-bool", `{"expected_revision":true,"from_index":0,"local_time":"20:00"}`},
		{"rev-string", `{"expected_revision":"1","from_index":0,"local_time":"20:00"}`},
		{"rev-null", `{"expected_revision":null,"from_index":0,"local_time":"20:00"}`},
		{"rev-fraction", `{"expected_revision":1.5,"from_index":0,"local_time":"20:00"}`},
		{"rev-zero", `{"expected_revision":0,"from_index":0,"local_time":"20:00"}`},
		{"rev-negative", `{"expected_revision":-2,"from_index":0,"local_time":"20:00"}`},
		{"rev-missing", `{"from_index":0,"local_time":"20:00"}`},
		{"from-missing", `{"expected_revision":1,"local_time":"20:00"}`},
		{"from-negative", `{"expected_revision":1,"from_index":-1,"local_time":"20:00"}`},
		{"from-count", `{"expected_revision":1,"from_index":3,"local_time":"20:00"}`},
		{"from-bool", `{"expected_revision":1,"from_index":true,"local_time":"20:00"}`},
		{"from-string", `{"expected_revision":1,"from_index":"1","local_time":"20:00"}`},
		{"from-fraction", `{"expected_revision":1,"from_index":0.5,"local_time":"20:00"}`},
		{"clock-missing", `{"expected_revision":1,"from_index":0}`},
		{"clock-short", `{"expected_revision":1,"from_index":0,"local_time":"9:00"}`},
		{"clock-hour", `{"expected_revision":1,"from_index":0,"local_time":"24:00"}`},
		{"clock-min", `{"expected_revision":1,"from_index":0,"local_time":"20:60"}`},
		{"clock-bare", `{"expected_revision":1,"from_index":0,"local_time":"2000"}`},
		{"clock-empty", `{"expected_revision":1,"from_index":0,"local_time":""}`},
		{"clock-number", `{"expected_revision":1,"from_index":0,"local_time":2000}`},
	}
	for i, tc := range bad {
		rec := amendPostHTTP(s, toks["u1"], sid, "rt-vb", tc.body, true)
		_ = i
		if st, code := errorCode(t, rec); st != 422 || code != "validation_failed" {
			t.Fatalf("%s = %d %s", tc.name, st, code)
		}
	}
	if st, code := errorCode(t, amendPostHTTP(s, toks["u1"], sid, "rt-vm", "{oops", true)); st != 400 || code != "malformed_request" {
		t.Fatalf("invalid JSON = %d %s", st, code)
	}
	if st, code := errorCode(t, amendPostHTTP(s, toks["u1"], sid, "", `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`, false)); st != 400 || code != "missing_idempotency_key" {
		t.Fatalf("missing key = %d %s", st, code)
	}
	post, _ := json.Marshal(httpExportState(t, s))
	if string(pre) != string(post) {
		t.Fatal("failed validation changed state")
	}
	// A huge positive integral revision is well-formed: it fails stale
	// before any occurrence cutoff or booking-field validation.
	huge := amendPostHTTP(s, toks["u1"], sid, "rt-huge", `{"expected_revision":1000000000000000000,"from_index":0,"local_time":"20:00"}`, true)
	if st, code := errorCode(t, huge); st != 409 || code != "stale_revision" {
		t.Fatalf("huge revision = %d %s", st, code)
	}
	// Make the series move once, then replay the old revision: stale, with
	// the failed key leaving no claim.
	first := amendPostHTTP(s, toks["u1"], sid, "rt-ok1", `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`, true)
	if first.Code != 201 {
		t.Fatalf("first amend = %d %q", first.Code, first.Body.String())
	}
	base, _ := json.Marshal(httpExportState(t, s))
	stale := amendPostHTTP(s, toks["u1"], sid, "rt-stale", `{"expected_revision":1,"from_index":0,"local_time":"21:00"}`, true)
	if st, code := errorCode(t, stale); st != 409 || code != "stale_revision" {
		t.Fatalf("stale = %d %s", st, code)
	}
	after, _ := json.Marshal(httpExportState(t, s))
	if string(base) != string(after) {
		t.Fatal("stale failure changed state")
	}
	// The unclaimed failed key succeeds genuinely with the current revision.
	reuse := amendPostHTTP(s, toks["u1"], sid, "rt-stale", `{"expected_revision":2,"from_index":0,"local_time":"21:00"}`, true)
	if reuse.Code != 201 {
		t.Fatalf("failed-key reuse = %d %q", reuse.Code, reuse.Body.String())
	}
	// Unknown fields are ignored in behavior but change canonical-body
	// identity: a fresh key with an extra field amends identically, while
	// the same committed key with a changed body is a reuse conflict.
	extra := amendPostHTTP(s, toks["u1"], sid, "rt-extra", `{"expected_revision":3,"from_index":0,"local_time":"21:30","zzz":9}`, true)
	if extra.Code != 201 {
		t.Fatalf("unknown-field amend = %d %q", extra.Code, extra.Body.String())
	}
	changed := amendPostHTTP(s, toks["u1"], sid, "rt-ok1", `{"expected_revision":1,"from_index":0,"local_time":"20:00","zzz":1}`, true)
	if st, code := errorCode(t, changed); st != 409 || code != "idempotency_key_reuse" {
		t.Fatalf("changed body = %d %s", st, code)
	}
}

func TestSeriesAmendHTTPReceipts(t *testing.T) {
	s, toks, sid, refs := amendRouteSetup(t)
	first := amendPostHTTP(s, toks["u1"], sid, "rc-1", `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`, true)
	if first.Code != 201 {
		t.Fatalf("first = %d %q", first.Code, first.Body.String())
	}
	firstRaw := first.Body.String()
	var fm map[string]any
	if err := json.Unmarshal([]byte(firstRaw), &fm); err != nil {
		t.Fatal(err)
	}
	if fm["series_id"] != sid || fm["revision"] != 2.0 {
		t.Fatalf("first shape: %q", firstRaw)
	}
	// The stored receipt binds owner, exact method/path/key, canonical body,
	// 201 status and the raw original response bytes.
	st := httpExportState(t, s)
	rc := amendReceipt(t, st, "/series/"+sid+"/amend", "rc-1")
	if rc["user_id"] != "u1" || rc["method"] != "POST" {
		t.Fatalf("receipt binding: %v", rc)
	}
	if rc["status"] != 201.0 {
		t.Fatalf("receipt status: %v", rc)
	}
	if rc["body"] != `{"expected_revision":1,"from_index":0,"local_time":"20:00"}` {
		t.Fatalf("receipt body: %v", rc["body"])
	}
	if rc["response"] != firstRaw {
		t.Fatal("receipt response != original raw bytes")
	}
	// GET current matches the first response while nothing else moved.
	gr := serveRequest(s, http.MethodGet, "/series/"+sid, nil,
		map[string]string{"Authorization": "Bearer " + toks["u1"]})
	if gr.Code != 200 || gr.Body.String() != firstRaw {
		t.Fatalf("GET current != first 201: %d", gr.Code)
	}
	// A real later PATCH changes the world; the replay still returns the
	// original bytes and changes no state.
	pr := serveRequest(s, http.MethodPatch, "/reservations/"+refs[0], []byte(`{"party_size":1}`),
		map[string]string{"Authorization": "Bearer " + toks["u1"]})
	if pr.Code != 200 {
		t.Fatalf("later patch = %d %q", pr.Code, pr.Body.String())
	}
	base, _ := json.Marshal(httpExportState(t, s))
	rp := amendPostHTTP(s, toks["u1"], sid, "rc-1", `{"expected_revision":1,"from_index":0,"local_time":"20:00"}`, true)
	if rp.Code != 200 || rp.Body.String() != firstRaw {
		t.Fatalf("replay = %d", rp.Code)
	}
	after, _ := json.Marshal(httpExportState(t, s))
	if string(base) != string(after) {
		t.Fatal("replay changed state")
	}
	// Committed key with a different body is a conflict and atomic.
	cbase, _ := json.Marshal(httpExportState(t, s))
	cf := amendPostHTTP(s, toks["u1"], sid, "rc-1", `{"expected_revision":2,"from_index":0,"local_time":"21:00"}`, true)
	if st, code := errorCode(t, cf); st != 409 || code != "idempotency_key_reuse" {
		t.Fatalf("reuse conflict = %d %s", st, code)
	}
	cafter, _ := json.Marshal(httpExportState(t, s))
	if string(cbase) != string(cafter) {
		t.Fatal("reuse conflict changed state")
	}
}

func TestSeriesAmendHTTPNoop(t *testing.T) {
	s, toks, sid, refs := amendRouteSetup(t)
	pre, _ := json.Marshal(httpExportState(t, s))
	// Identical clock is a no-op success: one new receipt, nothing else.
	noop := amendPostHTTP(s, toks["u1"], sid, "rc-noop", `{"expected_revision":1,"from_index":0,"local_time":"19:00"}`, true)
	if noop.Code != 201 {
		t.Fatalf("noop = %d %q", noop.Code, noop.Body.String())
	}
	var preS, postS map[string]any
	_ = json.Unmarshal(pre, &preS)
	post, _ := json.Marshal(httpExportState(t, s))
	_ = json.Unmarshal(post, &postS)
	for k, v := range preS {
		if k == "receipts" {
			continue
		}
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(postS[k])
		if string(xv) != string(yv) {
			t.Fatalf("no-op changed namespace %q", k)
		}
	}
	if len(postS["receipts"].(map[string]any)) != len(preS["receipts"].(map[string]any))+1 {
		t.Fatal("no-op must store exactly one receipt")
	}
	if rp := amendPostHTTP(s, toks["u1"], sid, "rc-noop", `{"expected_revision":1,"from_index":0,"local_time":"19:00"}`, true); rp.Code != 200 || rp.Body.String() != noop.Body.String() {
		t.Fatalf("noop replay = %d", rp.Code)
	}
	// Empty eligible set (every member cancelled) also succeeds 201 with no
	// record, history, counter or series drift beyond its receipt.
	for _, ref := range refs {
		cr := serveRequest(s, http.MethodPost, "/reservations/"+ref+"/cancel", nil,
			map[string]string{"Authorization": "Bearer " + toks["u1"]})
		if cr.Code != 200 {
			t.Fatalf("cancel %s = %d %q", ref, cr.Code, cr.Body.String())
		}
	}
	mid, _ := json.Marshal(httpExportState(t, s))
	em := amendPostHTTP(s, toks["u1"], sid, "rc-empty", `{"expected_revision":4,"from_index":0,"local_time":"20:00"}`, true)
	if em.Code != 201 {
		t.Fatalf("empty eligible = %d %q", em.Code, em.Body.String())
	}
	var midS, endS map[string]any
	_ = json.Unmarshal(mid, &midS)
	endB, _ := json.Marshal(httpExportState(t, s))
	_ = json.Unmarshal(endB, &endS)
	for k, v := range midS {
		if k == "receipts" {
			continue
		}
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(endS[k])
		if string(xv) != string(yv) {
			t.Fatalf("empty-eligible changed namespace %q", k)
		}
	}
}
