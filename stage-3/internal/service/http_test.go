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
