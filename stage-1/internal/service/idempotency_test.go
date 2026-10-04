package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func idemToken(t *testing.T, s *Service) string {
	t.Helper()
	v := signup(t, s, "idem@example.com", "correct horse", "Idem")
	tok, _ := v["token"].(string)
	if tok == "" {
		t.Fatal("signup returned no token")
	}
	return tok
}

func callIdem(s *Service, tok, method, path, key, body string) Result {
	return s.Idempotent(tok, method, path, key, []byte(body), func(st *State, uid string, obj map[string]any) Result {
		return created(map[string]any{"echo": obj["v"]})
	})
}

// Auth and malformed-object errors precede key handling.
func TestIdemAuthAndMalformed(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	if res := s.Idempotent("bad-token", "POST", "/reservations", "k1", []byte(`{"v":1}`), func(*State, string, map[string]any) Result {
		t.Error("callback must not run on auth failure")
		return created(nil)
	}); res.Status != 401 {
		t.Fatalf("bad token status = %d", res.Status)
	}
	if res := s.Idempotent("", "POST", "/reservations", "k1", []byte(`{"v":1}`), func(*State, string, map[string]any) Result {
		t.Error("callback must not run on auth failure")
		return created(nil)
	}); res.Status != 401 {
		t.Fatalf("empty token status = %d", res.Status)
	}
	for _, raw := range []string{"", "   ", "{oops", "[1,2]", `"str"`, "42", "null", `{"a":1} {"b":2}`} {
		if res := s.Idempotent(tok, "POST", "/reservations", "k-mal", []byte(raw), func(*State, string, map[string]any) Result {
			t.Errorf("callback must not run for malformed %q", raw)
			return created(nil)
		}); res.Status != 400 {
			t.Errorf("malformed %q status = %d, want 400", raw, res.Status)
		}
	}
}

// Header absent/empty is missing_idempotency_key 400; length validated as
// characters (not bytes), untrimmed.
func TestIdemKeyHeaderRules(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	if res := callIdem(s, tok, "POST", "/reservations", "", `{"v":1}`); res.Status != 400 {
		t.Fatalf("empty key status = %d, want 400", res.Status)
	} else if m := res.Body.(map[string]any)["error"].(map[string]any); m["code"] != "missing_idempotency_key" {
		t.Fatalf("empty key code = %v", m["code"])
	}
	ok255 := strings.Repeat("a", 255)
	if res := callIdem(s, tok, "POST", "/reservations", ok255, `{"v":1}`); res.Status != 201 {
		t.Fatalf("255-char key status = %d, want 201", res.Status)
	}
	if res := callIdem(s, tok, "POST", "/reservations", strings.Repeat("a", 256), `{"v":2}`); res.Status != 422 {
		t.Fatalf("256-char key status = %d, want 422", res.Status)
	}
	// 255 multibyte runes (510 bytes) are valid: length is 1..255 characters.
	multi255 := strings.Repeat("é", 255)
	if res := callIdem(s, tok, "POST", "/reservations", multi255, `{"v":3}`); res.Status != 201 {
		t.Fatalf("255-rune key status = %d, want 201", res.Status)
	}
	if res := callIdem(s, tok, "POST", "/reservations", strings.Repeat("é", 256), `{"v":4}`); res.Status != 422 {
		t.Fatalf("256-rune key status = %d, want 422", res.Status)
	}
	// Leading/trailing spaces are significant and must not be trimmed.
	if res := callIdem(s, tok, "POST", "/reservations", " k ", `{"v":5}`); res.Status != 201 {
		t.Fatalf("spaced key status = %d, want 201", res.Status)
	}
	if res := callIdem(s, tok, "POST", "/reservations", "k", `{"v":5}`); res.Status != 201 {
		t.Fatalf("trimmed key must be a different key: status = %d", res.Status)
	}
}

// Scope: same key string works independently per user and per path.
func TestIdemScopeUserAndPath(t *testing.T) {
	s := newFoundation(t)
	a := idemToken(t, s)
	bV := signup(t, s, "idem-b@example.com", "correct horse", "B")
	b, _ := bV["token"].(string)

	if res := callIdem(s, a, "POST", "/reservations", "shared", `{"v":1}`); res.Status != 201 {
		t.Fatalf("user A first use = %d", res.Status)
	}
	if res := callIdem(s, b, "POST", "/reservations", "shared", `{"v":1}`); res.Status != 201 {
		t.Fatalf("user B same key+body must be independent: %d", res.Status)
	}
	// Same key with a different body under user B is B's own 409: A's
	// receipt does not leak across users, B's governs B.
	if res := callIdem(s, b, "POST", "/reservations", "shared", `{"v":2}`); res.Status != 409 {
		t.Fatalf("user B different body must be B's own reuse conflict: %d", res.Status)
	}
	if res := callIdem(s, a, "POST", "/reservation-moves", "shared", `{"v":1}`); res.Status != 201 {
		t.Fatalf("same key+body on different path must succeed: %d", res.Status)
	}
}

// Canonical identity: key order and whitespace do not matter; unknown fields
// are part of the compared value.
func TestIdemCanonicalBody(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	inc := 0
	first := s.Idempotent(tok, "POST", "/reservations", "canon", []byte(`{"a":1,"b":[1, 2],"x":"y"}`), func(st *State, uid string, obj map[string]any) Result {
		inc++
		return created(map[string]any{"n": inc})
	})
	if first.Status != 201 {
		t.Fatalf("first = %d", first.Status)
	}
	replay := s.Idempotent(tok, "POST", "/reservations", "canon", []byte("  { \"x\" : \"y\" , \"b\" : [1,2] , \"a\" : 1 }  "), func(*State, string, map[string]any) Result {
		t.Error("callback must not run on replay")
		return created(map[string]any{"n": 99})
	})
	if replay.Status != 200 {
		t.Fatalf("replay status = %d, want 200", replay.Status)
	}
	want, _ := json.Marshal(first.Body)
	got, _ := json.Marshal(replay.Body)
	if string(want) != string(got) {
		t.Fatalf("replay body %s != original %s", got, want)
	}
	// Unknown-field difference is a different body.
	if res := s.Idempotent(tok, "POST", "/reservations", "canon", []byte(`{"a":1,"b":[1,2]}`), func(*State, string, map[string]any) Result {
		return created(nil)
	}); res.Status != 409 {
		t.Fatalf("dropped unknown field status = %d, want 409", res.Status)
	}
}

// Receipt resolution precedes callback validation: a different body on a used
// key is 409 even when the new body would be invalid in the callback.
func TestIdemReusePrecedesCallback(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	validate := func(st *State, uid string, obj map[string]any) Result {
		v, ok := obj["v"]
		if !ok {
			return validationFailed("missing v")
		}
		return created(map[string]any{"v": v})
	}
	if res := s.Idempotent(tok, "POST", "/reservations", "prec", []byte(`{"v":1}`), validate); res.Status != 201 {
		t.Fatalf("first = %d", res.Status)
	}
	res := s.Idempotent(tok, "POST", "/reservations", "prec", []byte(`{}`), validate)
	if res.Status != 409 {
		t.Fatalf("different invalid body status = %d, want 409", res.Status)
	}
	if m := res.Body.(map[string]any)["error"].(map[string]any); m["code"] != "idempotency_key_reuse" {
		t.Fatalf("code = %v", m["code"])
	}
}

// Failed callbacks store no receipt and mutate nothing: the key is reusable,
// including after the callback dirtied the working state.
func TestIdemFailureDiscardsStateAndClaim(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	fail := s.Idempotent(tok, "POST", "/reservations", "fk", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["tentative"] = Reservation{ReservationID: "x", Reference: "tentative", UserID: uid}
		st.Receipts["tentative"] = Receipt{UserID: uid}
		return validationFailed("nope")
	})
	if fail.Status != 422 {
		t.Fatalf("fail status = %d", fail.Status)
	}
	s.mu.Lock()
	_, resKept := s.state.Reservations["tentative"]
	_, rcKept := s.state.Receipts["tentative"]
	n := len(s.state.Receipts)
	s.mu.Unlock()
	if resKept || rcKept {
		t.Fatal("tentative working-state changes leaked on failure")
	}
	if n != 0 {
		t.Fatalf("receipts = %d after failure, want 0", n)
	}
	ok := s.Idempotent(tok, "POST", "/reservations", "fk", []byte(`{"v":2}`), func(st *State, uid string, obj map[string]any) Result {
		return created(map[string]any{"v": obj["v"]})
	})
	if ok.Status != 201 {
		t.Fatalf("failed key must be reusable: %d", ok.Status)
	}
	// Non-201 callback statuses (e.g. 409 from the domain) also leave no claim.
	conf := s.Idempotent(tok, "POST", "/reservations", "ck", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["tent"] = Reservation{Reference: "tent"}
		return conflict("table_unavailable", "taken")
	})
	if conf.Status != 409 {
		t.Fatalf("conflict status = %d", conf.Status)
	}
	if again := s.Idempotent(tok, "POST", "/reservations", "ck", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		return created(map[string]any{"ok": true})
	}); again.Status != 201 {
		t.Fatalf("409 key must be reusable even with same body: %d", again.Status)
	}
}

// 50 identical concurrent writes: exactly one 201, rest 200, effect once.
func TestIdemConcurrentIdentical(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	var calls int
	var mu sync.Mutex
	const n = 50
	statuses := make([]int, n)
	bodies := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := s.Idempotent(tok, "POST", "/reservations", "race", []byte(`{"v":7}`), func(st *State, uid string, obj map[string]any) Result {
				mu.Lock()
				calls++
				c := calls
				mu.Unlock()
				st.Reservations[fmt.Sprintf("r-%d", c)] = Reservation{ReservationID: "x", Reference: "r", UserID: uid}
				return created(map[string]any{"effect": c})
			})
			raw, _ := json.Marshal(res.Body)
			statuses[i], bodies[i] = res.Status, string(raw)
		}(i)
	}
	wg.Wait()
	n201, n200 := 0, 0
	for i, st := range statuses {
		switch st {
		case 201:
			n201++
		case 200:
			n200++
		default:
			t.Fatalf("worker %d status = %d", i, st)
		}
		if bodies[i] != bodies[0] {
			t.Fatalf("worker %d body %s != %s", i, bodies[i], bodies[0])
		}
	}
	if n201 != 1 || n200 != n-1 {
		t.Fatalf("statuses: %d x 201, %d x 200; want 1 and %d", n201, n200, n-1)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("callback ran %d times, want exactly once", calls)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Reservations) != 1 || len(s.state.Receipts) != 1 {
		t.Fatalf("state has %d reservations %d receipts, want 1/1", len(s.state.Reservations), len(s.state.Receipts))
	}
}

// Replay after the response-owned data mutates still returns the frozen
// original, with no callback and no state change.
func TestIdemReplayAfterMutation(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	first := s.Idempotent(tok, "POST", "/reservations", "mut", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["ORIG"] = Reservation{ReservationID: "res_1", Reference: "ORIG", UserID: uid, Status: StatusConfirmed}
		return created(map[string]any{"reference": "ORIG", "status": StatusConfirmed})
	})
	if first.Status != 201 {
		t.Fatalf("first = %d", first.Status)
	}
	// Later resource change: cancel the underlying booking.
	s.mu.Lock()
	r := s.state.Reservations["ORIG"]
	r.Status = StatusCancelled
	s.state.Reservations["ORIG"] = r
	s.mu.Unlock()

	replay := s.Idempotent(tok, "POST", "/reservations", "mut", []byte(`{"v":1}`), func(*State, string, map[string]any) Result {
		t.Error("callback must not run on replay")
		return created(map[string]any{"changed": true})
	})
	if replay.Status != 200 {
		t.Fatalf("replay status = %d, want 200", replay.Status)
	}
	m, ok := replay.Body.(map[string]any)
	if !ok || m["status"] != StatusConfirmed || m["reference"] != "ORIG" {
		t.Fatalf("replay body = %v, want frozen original", replay.Body)
	}
}

// Export/import preserves receipts, statuses and timestamps; replays survive
// import; import replaces destination receipts; reset clears imported state.
func TestIdemExportImport(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	first := s.Idempotent(tok, "POST", "/reservations", "ship", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["ORIG"] = Reservation{ReservationID: "res_9", Reference: "ORIG", UserID: uid, RestaurantID: "r_anker", Status: StatusConfirmed, CreatedAt: "2026-09-21T11:04:03+00:00"}
		return created(map[string]any{"reference": "ORIG", "status": StatusConfirmed, "created_at": "2026-09-21T11:04:03+00:00"})
	})
	if first.Status != 201 {
		t.Fatalf("first = %d", first.Status)
	}
	exp := s.Export()
	raw, _ := json.Marshal(exp.Body)

	dst := New()
	if res := dst.Import(raw); res.Status != 204 {
		t.Fatalf("import = %d (%v)", res.Status, res.Body)
	}
	replay := dst.Idempotent(tok, "POST", "/reservations", "ship", []byte(`{"v":1}`), func(*State, string, map[string]any) Result {
		t.Error("callback must not run after import replay")
		return created(map[string]any{"changed": true})
	})
	if replay.Status != 200 {
		t.Fatalf("post-import replay = %d, want 200", replay.Status)
	}
	got, _ := json.Marshal(replay.Body)
	want, _ := json.Marshal(first.Body)
	if string(got) != string(want) {
		t.Fatalf("post-import replay %s != original %s", got, want)
	}
	// Failed keys stay reusable after import (no receipt stored for them).
	if res := dst.Idempotent(tok, "POST", "/reservations", "ship", []byte(`{"v":2}`), func(*State, string, map[string]any) Result {
		return created(nil)
	}); res.Status != 409 {
		t.Fatalf("mismatch after import = %d, want 409", res.Status)
	}
	// Import is replacement, not merge: destination-only data disappears.
	// Receipts scope by user ID, so resolve the original owner for checks.
	uidOrig, ok := s.Authenticate(tok)
	if !ok {
		t.Fatal("original token must still authenticate")
	}
	dst2 := newFoundation(t)
	tok2 := idemToken(t, dst2)
	if res := dst2.Idempotent(tok2, "POST", "/reservations", "local", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		return created(map[string]any{"v": 1})
	}); res.Status != 201 {
		t.Fatalf("dst setup = %d", res.Status)
	}
	if res := dst2.Import(raw); res.Status != 204 {
		t.Fatalf("re-import = %d", res.Status)
	}
	dst2.mu.Lock()
	uid2, ok2 := dst2.state.Tokens[tok2]
	_, localKept := dst2.state.Receipts[ReceiptKey(uid2, "POST", "/reservations", "local")]
	_, shippedKept := dst2.state.Receipts[ReceiptKey(uidOrig, "POST", "/reservations", "ship")]
	n := len(dst2.state.Receipts)
	dst2.mu.Unlock()
	if ok2 {
		t.Fatal("import must remove previous destination credentials")
	}
	if localKept || !shippedKept || n != 1 {
		t.Fatalf("import must replace receipts: localKept=%v shippedKept=%v n=%d", localKept, shippedKept, n)
	}
	// Repeating import restores state without duplicating anything.
	if res := dst2.Import(raw); res.Status != 204 {
		t.Fatalf("repeat import = %d", res.Status)
	}
	dst2.mu.Lock()
	n2 := len(dst2.state.Receipts)
	dst2.mu.Unlock()
	if n2 != 1 {
		t.Fatalf("repeat import receipts = %d, want 1", n2)
	}
	// Reset clears imported receipts: the shipped key is usable again.
	if res := dst2.Reset([]byte(testFixture)); res.Status != 204 {
		t.Fatalf("reset = %d", res.Status)
	}
	tok3 := idemToken(t, dst2)
	if res := dst2.Idempotent(tok3, "POST", "/reservations", "ship", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		return created(map[string]any{"fresh": true})
	}); res.Status != 201 {
		t.Fatalf("post-reset reuse = %d, want 201", res.Status)
	}
}

// Unmarshallable callback responses never leak tentative state.
func TestIdemUnmarshallableResponse(t *testing.T) {
	s := newFoundation(t)
	tok := idemToken(t, s)

	res := s.Idempotent(tok, "POST", "/reservations", "bad", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["x"] = Reservation{Reference: "x"}
		return created(map[string]any{"ch": make(chan int)})
	})
	if res.Status != 500 {
		t.Fatalf("status = %d, want 500", res.Status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Reservations) != 0 || len(s.state.Receipts) != 0 {
		t.Fatal("unmarshallable response must not commit state or receipt")
	}
}
