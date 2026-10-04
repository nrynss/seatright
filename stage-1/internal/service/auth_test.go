package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSignupHappyPath(t *testing.T) {
	s := New()
	got := signup(t, s, "a@example.com", "correct horse", "Ada")
	uid, _ := got["user_id"].(string)
	token, _ := got["token"].(string)
	if uid == "" || len(uid) > 64 {
		t.Fatalf("bad user_id %q", uid)
	}
	if got["display_name"] != "Ada" {
		t.Fatalf("display_name = %v", got["display_name"])
	}
	if token == "" || len(token) > 64 {
		t.Fatalf("bad token %q", token)
	}
	if _, ok := s.Authenticate(token); !ok {
		t.Fatal("signup token does not authenticate")
	}
}

func TestSignupValidationMatrix(t *testing.T) {
	s := newFoundation(t)
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"duplicate email", `{"email":"ada@example.com","password":"correct horse battery","display_name":"Ada2"}`, 409, "email_taken"},
		{"short password", `{"email":"new@example.com","password":"short","display_name":"N"}`, 422, "validation_failed"},
		{"seven chars", `{"email":"new@example.com","password":"1234567","display_name":"N"}`, 422, "validation_failed"},
		{"eight chars ok", `{"email":"eight@example.com","password":"12345678","display_name":"N"}`, 201, ""},
		{"no at", `{"email":"not-an-email","password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"empty local", `{"email":"@example.com","password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"empty domain", `{"email":"a@","password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"two ats", `{"email":"a@b@c","password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"space", `{"email":"a @b","password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"missing email", `{"password":"correct horse","display_name":"N"}`, 422, "validation_failed"},
		{"missing password", `{"email":"m@example.com","display_name":"N"}`, 422, "validation_failed"},
		{"missing display", `{"email":"m@example.com","password":"correct horse"}`, 422, "validation_failed"},
		{"email wrong type", `{"email":42,"password":"correct horse","display_name":"N"}`, 400, "malformed_request"},
		{"password wrong type", `{"email":"m@example.com","password":true,"display_name":"N"}`, 400, "malformed_request"},
		{"display wrong type", `{"email":"m@example.com","password":"correct horse","display_name":3}`, 400, "malformed_request"},
		{"password null", `{"email":"m@example.com","password":null,"display_name":"N"}`, 400, "malformed_request"},
		{"unparseable", `{"email":`, 400, "malformed_request"},
		{"array body", `[1,2]`, 400, "malformed_request"},
		{"empty body", ``, 400, "malformed_request"},
	}
	for _, tc := range cases {
		rec := serveRequest(s, http.MethodPost, "/auth/signup", []byte(tc.body), nil)
		if tc.status == 201 {
			if rec.Code != 201 {
				t.Errorf("%s: got %d %q", tc.name, rec.Code, rec.Body.String())
			}
			continue
		}
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s, want %d %s", tc.name, status, code, tc.status, tc.code)
		}
	}
}

func TestSignupIgnoresUnknownFields(t *testing.T) {
	s := New()
	rec := serveRequest(s, http.MethodPost, "/auth/signup",
		[]byte(`{"email":"x@example.com","password":"correct horse","display_name":"X","role":"admin"}`), nil)
	if rec.Code != 201 {
		t.Fatalf("unknown fields should be ignored: %d %q", rec.Code, rec.Body.String())
	}
}

func TestPasswordIsHashed(t *testing.T) {
	s := New()
	signup(t, s, "hash@example.com", "correct horse", "H")
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.state.Users {
		if u.Email == "hash@example.com" {
			if u.PasswordHash == "correct horse" {
				t.Fatal("password stored in plaintext")
			}
			if !strings.HasPrefix(u.PasswordHash, "$2a$") {
				t.Fatalf("password hash is not bcrypt: %q", u.PasswordHash)
			}
		}
	}
}

func TestLogin(t *testing.T) {
	s := newFoundation(t)
	body := []byte(`{"email":"ada@example.com","password":"correct horse"}`)
	rec := serveRequest(s, http.MethodPost, "/auth/login", body, nil)
	if rec.Code != 200 {
		t.Fatalf("login: %d %q", rec.Code, rec.Body.String())
	}
	v := decodeBody(t, rec)
	if v["user_id"] != "u_ada" || v["display_name"] != "Ada" {
		t.Fatalf("login body = %q", rec.Body.String())
	}
	token, _ := v["token"].(string)
	if token == "" {
		t.Fatal("login returned no token")
	}
	// Second login yields another concurrently valid token.
	rec2 := serveRequest(s, http.MethodPost, "/auth/login", body, nil)
	token2 := decodeBody(t, rec2)["token"].(string)
	if token2 == token {
		t.Fatal("concurrent logins should mint distinct tokens")
	}
	if _, ok := s.Authenticate(token); !ok {
		t.Fatal("first token invalidated by second login")
	}
	if _, ok := s.Authenticate(token2); !ok {
		t.Fatal("second token does not authenticate")
	}
}

func TestLoginFailures(t *testing.T) {
	s := newFoundation(t)
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"wrong password", `{"email":"ada@example.com","password":"wrong horse"}`, 401, "unauthenticated"},
		{"unknown email", `{"email":"nobody@example.com","password":"correct horse"}`, 401, "unauthenticated"},
		{"missing password", `{"email":"ada@example.com"}`, 422, "validation_failed"},
		{"password wrong type", `{"email":"ada@example.com","password":9}`, 400, "malformed_request"},
		{"unparseable", `not json`, 400, "malformed_request"},
	}
	for _, tc := range cases {
		rec := serveRequest(s, http.MethodPost, "/auth/login", []byte(tc.body), nil)
		if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
	}
}

func TestAuthHeaderForms(t *testing.T) {
	s := newFoundation(t)
	token := signup(t, s, "hdr@example.com", "correct horse", "Hdr")["token"].(string)
	protected := func(h map[string]string) int {
		return serveRequest(s, http.MethodGet, "/reservations", nil, h).Code
	}
	if got := protected(nil); got != 401 {
		t.Errorf("missing header: %d", got)
	}
	if got := protected(map[string]string{"Authorization": "Bearer"}); got != 401 {
		t.Errorf("scheme only: %d", got)
	}
	if got := protected(map[string]string{"Authorization": "Bearer "}); got != 401 {
		t.Errorf("empty token: %d", got)
	}
	if got := protected(map[string]string{"Authorization": "Token " + token}); got != 401 {
		t.Errorf("wrong scheme: %d", got)
	}
	if got := protected(map[string]string{"Authorization": "bearer " + token}); got != 401 {
		t.Errorf("lowercase scheme accepted: %d", got)
	}
	if got := protected(authHeader(token)); got != 404 { // authed stub
		t.Errorf("valid token: %d", got)
	}
}

func TestLongPasswordsHaveNoUpperLimit(t *testing.T) {
	s := New()
	long73 := strings.Repeat("a", 73)
	body, _ := json.Marshal(map[string]any{"email": "long@example.com", "password": long73, "display_name": "Long"})
	rec := serveRequest(s, http.MethodPost, "/auth/signup", body, nil)
	if rec.Code != 201 {
		t.Fatalf("73-char password signup: got %d %q", rec.Code, rec.Body.String())
	}
	login := serveRequest(s, http.MethodPost, "/auth/login", body, nil)
	if login.Code != 200 {
		t.Fatalf("73-char password login: got %d %q", login.Code, login.Body.String())
	}
	// Passwords differing only past bcrypt's 72-byte input limit must still
	// be distinguished: no truncation of distinguishing suffixes.
	prefix := strings.Repeat("b", 72)
	for i, pw := range []string{prefix + "X", prefix + "Y"} {
		b, _ := json.Marshal(map[string]any{
			"email":    "suffix" + string(rune('a'+i)) + "@example.com",
			"password": pw, "display_name": "S",
		})
		if rec := serveRequest(s, http.MethodPost, "/auth/signup", b, nil); rec.Code != 201 {
			t.Fatalf("suffix signup %d: %d %q", i, rec.Code, rec.Body.String())
		}
	}
	cross, _ := json.Marshal(map[string]any{"email": "suffixa@example.com", "password": prefix + "Y"})
	if rec := serveRequest(s, http.MethodPost, "/auth/login", cross, nil); rec.Code != 401 {
		t.Fatalf("cross-suffix login should fail: %d %q", rec.Code, rec.Body.String())
	}
}

func TestLongSeededPasswordLogsIn(t *testing.T) {
	long80 := strings.Repeat("c", 80)
	fixture := `{"users":[{"id":"u_long","email":"long@example.com","password":"` + long80 + `","display_name":"Long"}],
		"restaurants":[],"reservations":[]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset with long seeded password: %d %q", rec.Code, rec.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"email": "long@example.com", "password": long80})
	if rec := serveRequest(s, http.MethodPost, "/auth/login", body, nil); rec.Code != 200 {
		t.Fatalf("long seeded password login: %d %q", rec.Code, rec.Body.String())
	}
	// The hash survives an unchanged export/import round trip.
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	if rec := serveRequest(dst, http.MethodPost, "/auth/login", body, nil); rec.Code != 200 {
		t.Fatalf("long password login after import: %d %q", rec.Code, rec.Body.String())
	}
}
