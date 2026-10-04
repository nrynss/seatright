package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// exportEnvelope marshals the live export body for mutation helpers.
func exportEnvelope(t *testing.T, s *Service) map[string]any {
	t.Helper()
	rec := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("export: %d %q", rec.Code, rec.Body.String())
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	return v
}

func importEnvelope(t *testing.T, s *Service, v map[string]any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal mutated export: %v", err)
	}
	rec := serveRequest(s, http.MethodPost, "/_test/import", raw, nil)
	if rec.Code == 204 {
		return rec.Code, ""
	}
	status, code := errorCode(t, rec)
	return status, code
}

func exportBytes(t *testing.T, s *Service) []byte {
	t.Helper()
	rec := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("export: %d %q", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

// seededSource builds a source service whose export is a legitimate,
// unchanged stage-1 producer snapshot covering the portable cases:
// signup + empty-display-name accounts, tokens, a cancelled and a past
// reservation, and one idempotent receipt over a complete record.
func seededSource(t *testing.T) (*Service, string) {
	t.Helper()
	s := newFoundation(t)
	tok := idemToken(t, s)
	// Cancelled seeded reservation via reset keeps producer-consistent times.
	seed := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":10080,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[
			{"id":"s1","reference":"SEED01","user_id":"u1","restaurant_id":"r1",
			"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":2,"status":"cancelled"},
			{"id":"s2","reference":"PAST02","user_id":"u1","restaurant_id":"r1",
			"table_id":"t1","starts_at_local":"2020-01-02T19:00","party_size":1}]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(seed), nil); rec.Code != 204 {
		t.Fatalf("seed reset: %d %q", rec.Code, rec.Body.String())
	}
	tok = idemToken(t, s)
	_ = tok
	blank, _ := json.Marshal(map[string]any{"email": "blank@example.com", "password": "correct horse", "display_name": ""})
	if rec := serveRequest(s, http.MethodPost, "/auth/signup", blank, nil); rec.Code != 201 {
		t.Fatalf("blank signup: %d %q", rec.Code, rec.Body.String())
	}
	first := s.Idempotent(tok, "POST", "/reservations", "ship", []byte(`{"v":1}`), func(st *State, uid string, obj map[string]any) Result {
		st.Reservations["ORIG01"] = Reservation{ReservationID: "res_9", Reference: "ORIG01", UserID: uid, RestaurantID: "r1", TableID: "t1", PartySize: 2, Status: StatusConfirmed, StartsAtLocal: "2026-09-24T19:00", StartsAt: "2026-09-24T19:00:00+02:00", EndsAt: "2026-09-24T20:30:00+02:00", CreatedAt: "2026-09-21T11:04:03+00:00"}
		return created(map[string]any{"reference": "ORIG01", "status": StatusConfirmed, "created_at": "2026-09-21T11:04:03+00:00"})
	})
	if first.Status != 201 {
		t.Fatalf("ship first use: %d", first.Status)
	}
	return s, tok
}

func stateOf(v map[string]any) map[string]any {
	return v["state"].(map[string]any)
}

// Corrupted receipts must not import: every case asserts 422 and a
// byte-equivalent destination export afterwards.
func TestImportRejectsCorruptReceipts(t *testing.T) {
	s, _ := seededSource(t)
	cases := []struct {
		name   string
		mutate func(st map[string]any)
	}{
		{"response not JSON", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["response"] = "{oops"
		}},
		{"response JSON scalar", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["response"] = `42`
		}},
		{"body not canonical", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["body"] = `{ "v" : 1 }`
		}},
		{"body not object", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["body"] = `[1,2]`
		}},
		{"body not JSON", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["body"] = `v`
		}},
		{"key too long", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["key"] = strings.Repeat("k", 256)
		}},
		{"owner ghost", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["user_id"] = "u_ghost"
		}},
		{"empty method", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["method"] = ""
		}},
		{"status not 201", func(st map[string]any) {
			rc := firstReceipt(t, st)
			rc["status"] = float64(200)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := newFoundation(t)
			before := string(exportBytes(t, dst))
			v := exportEnvelope(t, s)
			tc.mutate(stateOf(v))
			if status, code := importEnvelope(t, dst, v); status != 422 || code != "validation_failed" {
				t.Fatalf("got %d %s, want 422 validation_failed", status, code)
			}
			if got := string(exportBytes(t, dst)); got != before {
				t.Fatal("failed import mutated destination state")
			}
		})
	}
	// Map key inconsistent with the scoped receipt identity.
	dst := newFoundation(t)
	before := string(exportBytes(t, dst))
	v := exportEnvelope(t, s)
	st := stateOf(v)
	rcs := st["receipts"].(map[string]any)
	for k, v := range rcs {
		rc := v.(map[string]any)
		rc["key"] = "renamed"
		delete(rcs, k)
		rcs["WRONG\x00KEY"] = rc
		break
	}
	if status, code := importEnvelope(t, dst, v); status != 422 || code != "validation_failed" {
		t.Fatalf("renamed key: got %d %s, want 422", status, code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("failed import mutated destination state")
	}
}

func firstReceipt(t *testing.T, st map[string]any) map[string]any {
	t.Helper()
	rcs, ok := st["receipts"].(map[string]any)
	if !ok || len(rcs) == 0 {
		t.Fatal("seeded export has no receipts")
	}
	for _, v := range rcs {
		return v.(map[string]any)
	}
	return nil
}

// Corrupted configuration, identities and timestamps must not import.
func TestImportRejectsCorruptConfigAndRecords(t *testing.T) {
	s, _ := seededSource(t)
	cases := []struct {
		name   string
		mutate func(v map[string]any)
	}{
		{"bad timezone", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["timezone"] = "Mars/Olympus"
		}},
		{"zero grid", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["slot_minutes"] = float64(0)
		}},
		{"zero duration", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["reservation_duration_minutes"] = float64(0)
		}},
		{"negative cutoff", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["cancellation_cutoff_minutes"] = float64(-1)
		}},
		{"bad weekday", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["opening_hours"] = []any{map[string]any{"weekday": "funday", "opens": "18:00", "closes": "23:00"}}
		}},
		{"duplicate weekday", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["opening_hours"] = []any{
				map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
				map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
			}
		}},
		{"closes before opens", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["opening_hours"] = []any{map[string]any{"weekday": "thu", "opens": "23:00", "closes": "18:00"}}
		}},
		{"duplicate tables", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["tables"] = []any{
				map[string]any{"id": "t1", "label": "1", "capacity": float64(2)},
				map[string]any{"id": "t1", "label": "1b", "capacity": float64(2)},
			}
		}},
		{"zero capacity", func(v map[string]any) {
			rest := stateOf(v)["restaurants"].([]any)[0].(map[string]any)
			rest["tables"] = []any{map[string]any{"id": "t1", "label": "1", "capacity": float64(0)}}
		}},
		{"user id mismatch", func(v map[string]any) {
			users := stateOf(v)["users"].(map[string]any)
			for k, u := range users {
				u.(map[string]any)["id"] = "u_other"
				users["u_other"] = u
				delete(users, k)
				break
			}
		}},
		{"user id too long", func(v map[string]any) {
			users := stateOf(v)["users"].(map[string]any)
			for _, u := range users {
				u.(map[string]any)["id"] = strings.Repeat("x", 65)
				break
			}
		}},
		{"bad email", func(v map[string]any) {
			users := stateOf(v)["users"].(map[string]any)
			for _, u := range users {
				u.(map[string]any)["email"] = "not-an-email"
				break
			}
		}},
		{"plaintext credentials", func(v map[string]any) {
			users := stateOf(v)["users"].(map[string]any)
			for _, u := range users {
				u.(map[string]any)["password_hash"] = "correct horse"
				break
			}
		}},
		{"token ghost owner", func(v map[string]any) {
			stateOf(v)["tokens"].(map[string]any)["tok-ghost"] = "u_ghost"
		}},
		{"reservation bad reference", func(v map[string]any) {
			res := firstReservation(t, stateOf(v))
			res["reference"] = "lower01"
		}},
		{"reservation map key mismatch", func(v map[string]any) {
			ress := stateOf(v)["reservations"].(map[string]any)
			for k, r := range ress {
				delete(ress, k)
				ress["ELSEWHERE"] = r
				break
			}
		}},
		{"reservation ghost owner", func(v map[string]any) {
			firstReservation(t, stateOf(v))["user_id"] = "u_ghost"
		}},
		{"reservation unknown table", func(v map[string]any) {
			firstReservation(t, stateOf(v))["table_id"] = "t_ghost"
		}},
		{"reservation party zero", func(v map[string]any) {
			firstReservation(t, stateOf(v))["party_size"] = float64(0)
		}},
		{"reservation bad status", func(v map[string]any) {
			firstReservation(t, stateOf(v))["status"] = "pending"
		}},
		{"reservation bad local", func(v map[string]any) {
			firstReservation(t, stateOf(v))["starts_at_local"] = "2026-09-24 19:00"
		}},
		{"reservation shifted absolute start", func(v map[string]any) {
			firstReservation(t, stateOf(v))["starts_at"] = "2026-09-24T19:00:00+01:00"
		}},
		{"reservation wrong end", func(v map[string]any) {
			firstReservation(t, stateOf(v))["ends_at"] = "2026-09-24T21:30:00+02:00"
		}},
		{"reservation bad created", func(v map[string]any) {
			firstReservation(t, stateOf(v))["created_at"] = "not-a-time"
		}},
		{"reservation skipped local", func(v map[string]any) {
			res := firstReservation(t, stateOf(v))
			res["starts_at_local"] = "2026-03-29T02:30"
			res["starts_at"] = "2026-03-29T03:30:00+02:00"
			res["ends_at"] = "2026-03-29T05:00:00+02:00"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := newFoundation(t)
			before := string(exportBytes(t, dst))
			v := exportEnvelope(t, s)
			tc.mutate(v)
			if status, code := importEnvelope(t, dst, v); status != 422 || code != "validation_failed" {
				t.Fatalf("got %d %s, want 422 validation_failed", status, code)
			}
			if got := string(exportBytes(t, dst)); got != before {
				t.Fatal("failed import mutated destination state")
			}
		})
	}
}

func firstReservation(t *testing.T, st map[string]any) map[string]any {
	t.Helper()
	ress, ok := st["reservations"].(map[string]any)
	if !ok || len(ress) == 0 {
		t.Fatal("seeded export has no reservations")
	}
	for _, v := range ress {
		return v.(map[string]any)
	}
	return nil
}

// Legitimate producer exports stay accepted: cancelled, past, empty display
// names, empty lists and large stage-1 cutoffs all import verbatim.
func TestImportAcceptsProducerSnapshots(t *testing.T) {
	s, tok := seededSource(t)
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("valid import: %d %q", rec.Code, rec.Body.String())
	}
	after := string(exportBytes(t, dst))
	if after != before {
		t.Fatal("import did not preserve the snapshot verbatim")
	}
	// Tokens, logins, receipts and repeat imports survive.
	if _, ok := dst.Authenticate(tok); !ok {
		t.Fatal("token not preserved")
	}
	replay := dst.Idempotent(tok, "POST", "/reservations", "ship", []byte(`{"v":1}`), func(*State, string, map[string]any) Result {
		t.Error("callback must not run after import replay")
		return created(nil)
	})
	if replay.Status != 200 {
		t.Fatalf("replay = %d, want 200", replay.Status)
	}
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("repeat import: %d", rec.Code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("repeat import changed state")
	}
	// Reset clears imported state.
	if rec := serveRequest(dst, http.MethodPost, "/_test/reset", []byte(testFixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	if _, ok := dst.Authenticate(tok); ok {
		t.Fatal("reset did not clear imported credentials")
	}
	// Empty-collection states remain valid.
	empty := `{"track":"tablekeeper","format_version":1,"state":{"users":{},"tokens":{},"restaurants":[],"reservations":{},"receipts":{}}}`
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(empty), nil); rec.Code != 204 {
		t.Fatalf("empty import: %d %q", rec.Code, rec.Body.String())
	}
}

// F1: snapshot-clone null collections. A closed restaurant (opening_hours:[])
// or a table-less restaurant exports null slices after the clone; the
// unchanged export must reimport.
func TestImportAcceptsNullCollections(t *testing.T) {
	for _, fixture := range []string{
		`{"users":[],"restaurants":[{"id":"r_closed","name":"Closed","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[],"tables":[{"id":"t1","label":"1","capacity":2}]}],"reservations":[]}`,
		`{"users":[],"restaurants":[{"id":"r_notables","name":"NoTables","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[]}],"reservations":[]}`,
	} {
		s := New()
		if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
			t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
		}
		before := string(exportBytes(t, s))
		dst := New()
		if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
			t.Fatalf("unchanged null-collection import: %d %q", rec.Code, rec.Body.String())
		}
		if got := string(exportBytes(t, dst)); got != before {
			t.Fatal("null-collection round trip changed state")
		}
	}
}

// F2: numeric UTC offsets. A UTC-zone booking stored with explicit +00:00
// (the integrated producer spelling) compares by instant and imports
// verbatim; strings are retained, never rewritten.
func TestImportAcceptsNumericUTC(t *testing.T) {
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r_utc","name":"UTC Diner","timezone":"UTC","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[{"id":"s1","reference":"UTC001","user_id":"u1","restaurant_id":"r_utc",
		"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":2}]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("numeric-UTC import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("numeric-UTC round trip changed state")
	}
	// Stored-record regression independent of the local seed formatter:
	// explicit +00:00 spellings compare by instant, never rewritten.
	v := exportEnvelope(t, s)
	ress := stateOf(v)["reservations"].(map[string]any)
	for _, r := range ress {
		rec := r.(map[string]any)
		rec["starts_at"] = "2026-09-24T19:00:00+00:00"
		rec["ends_at"] = "2026-09-24T20:30:00+00:00"
	}
	raw, _ := json.Marshal(v)
	dst2 := New()
	if rec := serveRequest(dst2, http.MethodPost, "/_test/import", raw, nil); rec.Code != 204 {
		t.Fatalf("rewritten +00:00 import: %d %q", rec.Code, rec.Body.String())
	}
	var check map[string]any
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatal(err)
	}
	got := exportEnvelope(t, dst2)
	wantState, _ := json.Marshal(check["state"])
	gotState, _ := json.Marshal(got["state"])
	if string(gotState) != string(wantState) {
		t.Fatal("numeric spelling was not retained verbatim")
	}
}

// F3: blank table labels. The fixture requires label presence, not
func TestImportAcceptsBlankTableLabel(t *testing.T) {
	fixture := `{"users":[],"restaurants":[{"id":"r_blank","name":"Blank","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t1","label":"","capacity":2}]}],"reservations":[]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("blank-label import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("blank-label round trip changed state")
	}
}

// F4: producer-accepted seeds reimport unchanged. Reset enforces
// resolvable local times and duration consistency but not booking business
// rules, so off-grid / outside-hours seeds with consistent instants stay
// importable (past and cancelled alike).
func TestImportAcceptsOffBusinessRuleSeeds(t *testing.T) {
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[
			{"id":"s1","reference":"OFFG01","user_id":"u1","restaurant_id":"r1",
			"table_id":"t1","starts_at_local":"2026-09-24T19:15","party_size":2},
			{"id":"s2","reference":"OFFH02","user_id":"u1","restaurant_id":"r1",
			"table_id":"t1","starts_at_local":"2026-09-24T17:00","party_size":1,"status":"cancelled"}]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("off-rule seed import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("off-rule seed round trip changed state")
	}
}

// Reset accepts party_size above table capacity (positive integer + known
// table only), so an unchanged export of such a seed must reimport
// verbatim. Ordinary booking capacity checks live in reservation core.
func TestImportAcceptsOverCapacitySeed(t *testing.T) {
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":4}]}],
		"reservations":[{"id":"s1","reference":"OVERC1","user_id":"u1","restaurant_id":"r1",
		"table_id":"t1","starts_at_local":"2026-09-24T19:00","party_size":5}]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("over-capacity seed import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("over-capacity seed round trip changed state")
	}
}

// Corrupt envelope bodies through HTTP: invalid JSON is 400, everything else
// 422, and the destination stays byte-identical.
func TestImportHTTPAtomicity(t *testing.T) {
	s, _ := seededSource(t)
	before := string(exportBytes(t, s))
	for _, tc := range []struct {
		name       string
		body       string
		status     int
		code       string
		checkAfter bool
	}{
		{"unparseable", `{`, 400, "malformed_request", true},
		{"missing state", `{"track":"tablekeeper","format_version":1}`, 422, "validation_failed", true},
		{"corrupt receipt response", corruptFirstReceipt(t, s, "response", "{oops"), 422, "validation_failed", true},
	} {
		rec := serveRequest(s, http.MethodPost, "/_test/import", []byte(tc.body), nil)
		if tc.status == 400 {
			if status, code := errorCode(t, rec); status != 400 || code != "malformed_request" {
				t.Errorf("%s: got %d %s", tc.name, status, code)
			}
		} else if status, code := errorCode(t, rec); status != tc.status || code != tc.code {
			t.Errorf("%s: got %d %s", tc.name, status, code)
		}
		if tc.checkAfter {
			if got := string(exportBytes(t, s)); got != before {
				t.Fatalf("%s mutated destination", tc.name)
			}
		}
	}
}

func corruptFirstReceipt(t *testing.T, s *Service, field, val string) string {
	t.Helper()
	v := exportEnvelope(t, s)
	firstReceipt(t, stateOf(v))[field] = val
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
