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

func TestStubRoutesNeedAuthThenNotFound(t *testing.T) {
	s := newFoundation(t)
	token := signup(t, s, "stub@example.com", "correct horse", "Stub")["token"].(string)
	// POST /reservations and POST /reservation-moves stay stubs until S1-D2.
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
		if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
			t.Errorf("%s %s authed stub: got %d %s", tc.method, tc.path, status, code)
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
