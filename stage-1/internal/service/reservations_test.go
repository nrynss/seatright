package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// futureThursday returns a Thursday safely beyond any fixture cutoff.
func futureThursday(t *testing.T) string {
	t.Helper()
	d := time.Now().Add(60 * 24 * time.Hour)
	for d.Weekday() != time.Thursday {
		d = d.Add(24 * time.Hour)
	}
	return d.Format("2006-01-02")
}

const (
	seedAda         = `{"id":"u_ada","email":"ada@example.com","password":"correct horse","display_name":"Ada"}`
	seedBob         = `{"id":"u_bob","email":"bob@example.com","password":"correct horse","display_name":"Bob"}`
	seedAnkerTables = `{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"},{"weekday":"fri","opens":"18:00","closes":"23:30"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}]}`
)

func seedRes(id, reference, user, restaurant, table, local string, party int, status string) string {
	return fmt.Sprintf(`{"id":%q,"reference":%q,"user_id":%q,"restaurant_id":%q,"table_id":%q,"starts_at_local":%q,"party_size":%d,"status":%q}`,
		id, reference, user, restaurant, table, local, party, status)
}

func resetWith(t *testing.T, s *Service, users, restaurants, reservations string) {
	t.Helper()
	body := `{"users":[` + users + `],"restaurants":[` + restaurants + `],"reservations":[` + reservations + `]}`
	rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(body), nil)
	if rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
}

// queryOf builds availability query values.
func queryOf(restaurant, date, party string) url.Values {
	q := url.Values{}
	q.Set("restaurant_id", restaurant)
	q.Set("date", date)
	q.Set("party_size", party)
	return q
}

// createLocked exercises the creation callback exactly as the future receipt
// wrapper will: under the caller's lock, against the passed state.
func createLocked(s *Service, userID string, body map[string]any) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createReservationLocked(&s.state, userID, body)
}

func createBody(restaurant string, table any, local any, party any) map[string]any {
	return map[string]any{
		"restaurant_id": restaurant, "table_id": table,
		"starts_at_local": local, "party_size": party,
	}
}

func checkTimestampNumeric(t *testing.T, label, value string) {
	t.Helper()
	if strings.HasSuffix(value, "Z") {
		t.Fatalf("%s uses bare Z: %q", label, value)
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		t.Fatalf("%s is not RFC3339: %q", label, value)
	}
}

func TestCreateCoreHappyPath(t *testing.T) {
	s := newFoundation(t)
	res := createLocked(s, "u_ada", createBody("r_anker", "t_2", "2026-09-24T19:00", 4.0))
	if res.Status != 201 {
		t.Fatalf("create: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	if v["status"] != StatusConfirmed || v["party_size"] != 4 {
		t.Fatalf("body = %v", res.Body)
	}
	if v["starts_at_local"] != "2026-09-24T19:00" {
		t.Fatalf("local echo = %v", v["starts_at_local"])
	}
	if v["starts_at"] != "2026-09-24T19:00:00+02:00" || v["ends_at"] != "2026-09-24T20:30:00+02:00" {
		t.Fatalf("bounds = %v %v", v["starts_at"], v["ends_at"])
	}
	checkTimestampNumeric(t, "created_at", v["created_at"].(string))
	if _, present := v["user_id"]; present {
		t.Fatal("create response must exclude user_id")
	}
	rid, _ := v["reservation_id"].(string)
	if rid == "" || len(rid) > 64 {
		t.Fatalf("bad reservation_id %q", rid)
	}
	ref, _ := v["reference"].(string)
	if !validReference(ref) {
		t.Fatalf("bad reference %q", ref)
	}
	// A second booking gets a distinct reference.
	res2 := createLocked(s, "u_ada", createBody("r_anker", "t_1", "2026-09-24T19:00", 2.0))
	if res2.Status != 201 || res2.Body.(map[string]any)["reference"] == ref {
		t.Fatalf("second create: %d %v", res2.Status, res2.Body)
	}
}

func TestCreateCoreValidation(t *testing.T) {
	newSeeded := func(t *testing.T) *Service {
		s := New()
		resetWith(t, s, seedAda,
			seedAnkerTables,
			seedRes("s1", "SEED01", "u_ada", "r_anker", "t_2", "2026-09-24T19:00", 4, "confirmed"))
		return s
	}
	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"overlap same slot", createBody("r_anker", "t_2", "2026-09-24T19:00", 2.0), 409, "table_unavailable"},
		{"overlap partial", createBody("r_anker", "t_2", "2026-09-24T18:30", 2.0), 409, "table_unavailable"},
		{"adjacent after", createBody("r_anker", "t_2", "2026-09-24T20:30", 2.0), 201, ""},
		{"free table same slot", createBody("r_anker", "t_1", "2026-09-24T19:00", 2.0), 201, ""},
		{"off grid", createBody("r_anker", "t_2", "2026-09-24T19:15", 2.0), 422, "not_on_slot_grid"},
		{"before open", createBody("r_anker", "t_2", "2026-09-24T17:30", 2.0), 422, "outside_opening_hours"},
		{"ends after close", createBody("r_anker", "t_2", "2026-09-24T22:30", 2.0), 422, "outside_opening_hours"},
		{"closed day", createBody("r_anker", "t_2", "2026-09-23T19:00", 2.0), 422, "outside_opening_hours"},
		{"gap time", createBody("r_anker", "t_2", "2026-03-29T02:30", 2.0), 422, "invalid_local_time"},
		{"over capacity", createBody("r_anker", "t_1", "2026-09-24T19:00", 3.0), 422, "party_exceeds_capacity"},
		{"party string", createBody("r_anker", "t_1", "2026-09-24T18:00", "2"), 422, "validation_failed"},
		{"party bool", createBody("r_anker", "t_1", "2026-09-24T18:00", true), 422, "validation_failed"},
		{"party float", createBody("r_anker", "t_1", "2026-09-24T18:00", 2.5), 422, "validation_failed"},
		{"party zero", createBody("r_anker", "t_1", "2026-09-24T18:00", 0.0), 422, "validation_failed"},
		{"party negative", createBody("r_anker", "t_1", "2026-09-24T18:00", -1.0), 422, "validation_failed"},
		{"party null", createBody("r_anker", "t_1", "2026-09-24T18:00", nil), 422, "validation_failed"},
		{"local with space", createBody("r_anker", "t_1", "2026-09-24 18:00", 2.0), 422, "validation_failed"},
		{"local with seconds", createBody("r_anker", "t_1", "2026-09-24T18:00:00", 2.0), 422, "validation_failed"},
		{"local with offset", createBody("r_anker", "t_1", "2026-09-24T18:00:00+02:00", 2.0), 422, "validation_failed"},
		{"local number", createBody("r_anker", "t_1", 42.0, 2.0), 400, "malformed_request"},
		{"table bool", createBody("r_anker", true, "2026-09-24T18:00", 2.0), 400, "malformed_request"},
		{"unknown restaurant", createBody("r_nope", "t_1", "2026-09-24T18:00", 2.0), 404, "not_found"},
		{"unknown table", createBody("r_anker", "t_nope", "2026-09-24T18:00", 2.0), 404, "not_found"},
		{"past booking allowed", createBody("r_anker", "t_1", "2020-01-02T19:00", 2.0), 201, ""},
	}
	for _, tc := range cases {
		s := newSeeded(t)
		res := createLocked(s, "u_ada", tc.body)
		if tc.status == 201 {
			if res.Status != 201 {
				t.Errorf("%s: got %d %v", tc.name, res.Status, res.Body)
			}
			continue
		}
		v, ok := res.Body.(map[string]any)
		if !ok || res.Status != tc.status {
			t.Errorf("%s: got %d %v", tc.name, res.Status, res.Body)
			continue
		}
		if code := v["error"].(map[string]any)["code"]; code != tc.code {
			t.Errorf("%s: got code %v, want %s", tc.name, code, tc.code)
		}
	}
	// Missing-field cases (built separately to avoid shared-map mutation).
	for _, tc := range []struct {
		name string
		drop string
	}{
		{"missing party", "party_size"},
		{"missing local", "starts_at_local"},
		{"missing table", "table_id"},
		{"missing restaurant", "restaurant_id"},
	} {
		s := newSeeded(t)
		body := createBody("r_anker", "t_1", "2026-09-24T18:00", 2.0)
		delete(body, tc.drop)
		res := createLocked(s, "u_ada", body)
		if v, ok := res.Body.(map[string]any); !ok || res.Status != 422 {
			t.Errorf("%s: got %d %v", tc.name, res.Status, res.Body)
		} else if code := v["error"].(map[string]any)["code"]; code != "validation_failed" {
			t.Errorf("%s: got code %v", tc.name, code)
		}
	}
	// Unknown fields are ignored.
	s := newSeeded(t)
	body := createBody("r_anker", "t_1", "2026-09-24T18:00", 2.0)
	body["vip"] = true
	if res := createLocked(s, "u_ada", body); res.Status != 201 {
		t.Fatalf("unknown fields must be ignored: %d %v", res.Status, res.Body)
	}
	// Failed creates leave no record.
	s2 := newSeeded(t)
	before := len(s2.state.Reservations)
	if res := createLocked(s2, "u_ada", createBody("r_anker", "t_2", "2026-09-24T19:00", 2.0)); res.Status != 409 {
		t.Fatalf("expected conflict, got %d", res.Status)
	}
	if len(s2.state.Reservations) != before {
		t.Fatal("failed create mutated state")
	}
}

func TestCreateForeignTable(t *testing.T) {
	s := New()
	other := `{"id":"r_zwei","name":"Zwei","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_9","label":"9","capacity":2}]}`
	resetWith(t, s, seedAda, seedAnkerTables+","+other, "")
	res := createLocked(s, "u_ada", createBody("r_anker", "t_9", "2026-09-24T19:00", 2.0))
	if v, ok := res.Body.(map[string]any); !ok || res.Status != 404 {
		t.Fatalf("foreign table: got %d %v", res.Status, res.Body)
	} else if code := v["error"].(map[string]any)["code"]; code != "not_found" {
		t.Fatalf("foreign table code = %v", code)
	}
}

func TestPrepareAmendmentAndConflict(t *testing.T) {
	future := futureThursday(t)
	past := "2020-01-02T19:00"
	newState := func(t *testing.T) *Service {
		s := New()
		resetWith(t, s, seedAda+","+seedBob, seedAnkerTables,
			seedRes("sA", "FUTURE1", "u_ada", "r_anker", "t_1", future+"T19:00", 2, "confirmed")+","+
				seedRes("sB", "FUTURE2", "u_ada", "r_anker", "t_2", future+"T19:00", 4, "confirmed")+","+
				seedRes("sP", "PAST001", "u_ada", "r_anker", "t_1", past, 2, "confirmed")+","+
				seedRes("sC", "CANC001", "u_ada", "r_anker", "t_2", future+"T20:30", 2, "cancelled"))
		return s
	}
	t.Run("prepared change keeps identity without mutation", func(t *testing.T) {
		s := newState(t)
		cur := s.state.Reservations["FUTURE1"]
		prepared, cerr := prepareAmendment(&s.state, cur, map[string]any{"party_size": 1.0})
		if cerr != nil {
			t.Fatalf("prepare: %v", cerr)
		}
		if prepared.ReservationID != cur.ReservationID || prepared.Reference != cur.Reference ||
			prepared.UserID != cur.UserID || prepared.CreatedAt != cur.CreatedAt {
			t.Fatal("identity/owner/created time changed")
		}
		if prepared.PartySize != 1 || prepared.TableID != "t_1" {
			t.Fatalf("prepared = %+v", prepared)
		}
		if s.state.Reservations["FUTURE1"].PartySize != 2 {
			t.Fatal("prepare mutated state")
		}
	})
	t.Run("cancelled rejected", func(t *testing.T) {
		s := newState(t)
		_, cerr := prepareAmendment(&s.state, s.state.Reservations["CANC001"], map[string]any{"party_size": 1.0})
		if cerr == nil || cerr.code != "reservation_cancelled" || cerr.status != 409 {
			t.Fatalf("cerr = %+v", cerr)
		}
	})
	t.Run("cutoff rejected before fields", func(t *testing.T) {
		s := newState(t)
		_, cerr := prepareAmendment(&s.state, s.state.Reservations["PAST001"], map[string]any{"table_id": "t_nope"})
		if cerr == nil || cerr.code != "cutoff_passed" {
			t.Fatalf("cerr = %+v", cerr)
		}
	})
	t.Run("noop retains values", func(t *testing.T) {
		s := newState(t)
		cur := s.state.Reservations["FUTURE1"]
		prepared, cerr := prepareAmendment(&s.state, cur, map[string]any{})
		if cerr != nil {
			t.Fatalf("noop prepare: %v", cerr)
		}
		if prepared != cur {
			t.Fatalf("noop changed values: %+v vs %+v", prepared, cur)
		}
	})
	t.Run("conflict sets", func(t *testing.T) {
		s := newState(t)
		overlap := s.state.Reservations["FUTURE1"]
		if !conflictingReservation(&s.state, overlap, nil) {
			t.Fatal("self without exclude should report overlap")
		}
		if conflictingReservation(&s.state, overlap, map[string]bool{"FUTURE1": true}) {
			t.Fatal("excluded self should not conflict")
		}
		cancelled := s.state.Reservations["CANC001"]
		other := Reservation{RestaurantID: "r_anker", TableID: "t_2", Status: StatusConfirmed,
			StartsAtLocal: cancelled.StartsAtLocal, StartsAt: cancelled.StartsAt, EndsAt: cancelled.EndsAt}
		if conflictingReservation(&s.state, other, nil) {
			t.Fatal("cancelled bookings hold no occupancy")
		}
		adjacent := overlap
		adjacent.Reference = "OTHER"
		adjacent.StartsAtLocal = future + "T20:30"
		adjacent.StartsAt = future + "T20:30:00+01:00"
		adjacent.EndsAt = future + "T22:00:00+01:00"
		if conflictingReservation(&s.state, adjacent, nil) {
			t.Fatal("adjacent intervals must not overlap")
		}
	})
}

func TestPatchCancelListLookupHTTP(t *testing.T) {
	future := futureThursday(t)
	setup := func(t *testing.T) (*Service, map[string]string) {
		s := New()
		resetWith(t, s, seedAda+","+seedBob, seedAnkerTables,
			seedRes("sA", "FUTURE1", "u_ada", "r_anker", "t_1", future+"T19:00", 2, "confirmed")+","+
				seedRes("sB", "FUTURE2", "u_ada", "r_anker", "t_2", future+"T19:00", 4, "confirmed")+","+
				seedRes("sP", "PAST001", "u_ada", "r_anker", "t_1", "2020-01-02T19:00", 2, "confirmed")+","+
				seedRes("sO", "BOB0001", "u_bob", "r_anker", "t_2", future+"T20:30", 2, "confirmed"))
		adaLogin, _ := json.Marshal(map[string]any{"email": "ada@example.com", "password": "correct horse"})
		ada := decodeBody(t, serveRequest(s, http.MethodPost, "/auth/login", adaLogin, nil))["token"].(string)
		bobLogin, _ := json.Marshal(map[string]any{"email": "bob@example.com", "password": "correct horse"})
		bob := decodeBody(t, serveRequest(s, http.MethodPost, "/auth/login", bobLogin, nil))["token"].(string)
		return s, map[string]string{"ada": ada, "bob": bob}
	}
	t.Run("list is owner-only descending", func(t *testing.T) {
		s, tok := setup(t)
		rec := serveRequest(s, http.MethodGet, "/reservations", nil, authHeader(tok["ada"]))
		if rec.Code != 200 {
			t.Fatalf("list: %d %q", rec.Code, rec.Body.String())
		}
		items := decodeBody(t, rec)["reservations"].([]any)
		if len(items) != 3 {
			t.Fatalf("ada sees %d, want 3", len(items))
		}
		refs := []string{items[0].(map[string]any)["reference"].(string),
			items[1].(map[string]any)["reference"].(string),
			items[2].(map[string]any)["reference"].(string)}
		if refs[0] == "PAST001" || refs[2] != "PAST001" {
			t.Fatalf("not starts_at descending: %v", refs)
		}
		for _, it := range items {
			if _, present := it.(map[string]any)["user_id"]; present {
				t.Fatal("list leaks user_id")
			}
		}
		rec = serveRequest(s, http.MethodGet, "/reservations", nil, authHeader(tok["bob"]))
		if got := decodeBody(t, rec)["reservations"].([]any); len(got) != 1 {
			t.Fatalf("bob sees %d", len(got))
		}
		rec = serveRequest(s, http.MethodGet, "/reservations", nil, nil)
		if status, code := errorCode(t, rec); status != 401 || code != "unauthenticated" {
			t.Fatalf("unauthed list: %d %s", status, code)
		}
	})
	t.Run("lookup hides others", func(t *testing.T) {
		s, tok := setup(t)
		rec := serveRequest(s, http.MethodGet, "/reservations/FUTURE1", nil, authHeader(tok["ada"]))
		if rec.Code != 200 || decodeBody(t, rec)["table_id"] != "t_1" {
			t.Fatalf("lookup: %d %q", rec.Code, rec.Body.String())
		}
		for _, path := range []string{"/reservations/BOB0001", "/reservations/NOPE000"} {
			rec = serveRequest(s, http.MethodGet, path, nil, authHeader(tok["ada"]))
			if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
				t.Fatalf("GET %s as ada: %d %s", path, status, code)
			}
		}
		rec = serveRequest(s, http.MethodGet, "/reservations/FUTURE1", nil, nil)
		if status, _ := errorCode(t, rec); status != 401 {
			t.Fatalf("unauthed lookup: %d", status)
		}
	})
	t.Run("cancel flows", func(t *testing.T) {
		s, tok := setup(t)
		rec := serveRequest(s, http.MethodPost, "/reservations/FUTURE1/cancel", nil, authHeader(tok["ada"]))
		if rec.Code != 200 || decodeBody(t, rec)["status"] != StatusCancelled {
			t.Fatalf("cancel: %d %q", rec.Code, rec.Body.String())
		}
		rec = serveRequest(s, http.MethodPost, "/reservations/FUTURE1/cancel", nil, authHeader(tok["ada"]))
		if rec.Code != 200 || decodeBody(t, rec)["status"] != StatusCancelled {
			t.Fatalf("recancel: %d %q", rec.Code, rec.Body.String())
		}
		rec = serveRequest(s, http.MethodPost, "/reservations/PAST001/cancel", nil, authHeader(tok["ada"]))
		if status, code := errorCode(t, rec); status != 409 || code != "cutoff_passed" {
			t.Fatalf("past cancel: %d %s", status, code)
		}
		rec = serveRequest(s, http.MethodPost, "/reservations/BOB0001/cancel", nil, authHeader(tok["ada"]))
		if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
			t.Fatalf("foreign cancel: %d %s", status, code)
		}
		// Freed occupancy reappears in availability.
		avail := s.Availability(queryOf("r_anker", future, "2"))
		if avail.Status != 200 {
			t.Fatalf("availability: %d", avail.Status)
		}
		found := false
		for _, sl := range avail.Body.(map[string]any)["slots"].([]any) {
			m := sl.(map[string]any)
			if m["starts_at_local"] == future+"T19:00" {
				for _, id := range m["available_table_ids"].([]any) {
					if id == "t_1" {
						found = true
					}
				}
			}
		}
		if !found {
			t.Fatal("cancelled table not offered again")
		}
	})
	t.Run("patch flows", func(t *testing.T) {
		s, tok := setup(t)
		patch := func(ref, body string) *httptest.ResponseRecorder {
			return serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(body), authHeader(tok["ada"]))
		}
		rec := patch("FUTURE1", `{"party_size":1}`)
		if rec.Code != 200 {
			t.Fatalf("patch party: %d %q", rec.Code, rec.Body.String())
		}
		v := decodeBody(t, rec)
		if v["party_size"] != 1.0 || v["table_id"] != "t_1" {
			t.Fatalf("patched = %q", rec.Body.String())
		}
		if v["reference"] != "FUTURE1" {
			t.Fatalf("reference changed: %q", rec.Body.String())
		}
		// Moving onto an occupied slot is table_unavailable and atomic.
		rec = patch("FUTURE1", `{"table_id":"t_2"}`)
		if status, code := errorCode(t, rec); status != 409 || code != "table_unavailable" {
			t.Fatalf("occupied patch: %d %s", status, code)
		}
		lookup := serveRequest(s, http.MethodGet, "/reservations/FUTURE1", nil, authHeader(tok["ada"]))
		if v := decodeBody(t, lookup); v["table_id"] != "t_1" || v["party_size"] != 1.0 {
			t.Fatalf("failed patch mutated record: %q", lookup.Body.String())
		}
		rec = patch("PAST001", `{"party_size":1}`)
		if status, code := errorCode(t, rec); status != 409 || code != "cutoff_passed" {
			t.Fatalf("past patch: %d %s", status, code)
		}
		// Cancelling first, then patching, is reservation_cancelled.
		serveRequest(s, http.MethodPost, "/reservations/FUTURE2/cancel", nil, authHeader(tok["ada"]))
		rec = patch("FUTURE2", `{"party_size":1}`)
		if status, code := errorCode(t, rec); status != 409 || code != "reservation_cancelled" {
			t.Fatalf("cancelled patch: %d %s", status, code)
		}
		// Wrong-type and invalid fields.
		rec = patch("FUTURE1", `{"table_id":4}`)
		if status, code := errorCode(t, rec); status != 400 || code != "malformed_request" {
			t.Fatalf("patch table type: %d %s", status, code)
		}
		rec = patch("FUTURE1", `{"party_size":"many"}`)
		if status, code := errorCode(t, rec); status != 422 || code != "validation_failed" {
			t.Fatalf("patch party type: %d %s", status, code)
		}
		rec = patch("FUTURE1", `{"table_id":"t_nope"}`)
		if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
			t.Fatalf("patch unknown table: %d %s", status, code)
		}
		// No-op patch succeeds with identical values.
		rec = patch("FUTURE1", `{}`)
		if rec.Code != 200 {
			t.Fatalf("noop patch: %d %q", rec.Code, rec.Body.String())
		}
		if v := decodeBody(t, rec); v["table_id"] != "t_1" || v["party_size"] != 1.0 {
			t.Fatalf("noop changed values: %q", rec.Body.String())
		}
		// Other owner's booking is invisible.
		rec = patch("BOB0001", `{"party_size":1}`)
		if status, code := errorCode(t, rec); status != 404 || code != "not_found" {
			t.Fatalf("foreign patch: %d %s", status, code)
		}
		// Unparseable body is malformed.
		rec = patch("FUTURE1", `{`)
		if status, code := errorCode(t, rec); status != 400 || code != "malformed_request" {
			t.Fatalf("unparseable patch: %d %s", status, code)
		}
	})
}

func TestConcurrentCreatesSerialize(t *testing.T) {
	s := newFoundation(t)
	const n = 20
	var wg sync.WaitGroup
	results := make([]Result, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = createLocked(s, "u_ada", createBody("r_anker", "t_2", "2026-09-24T19:00", 2.0))
		}(i)
	}
	wg.Wait()
	created, conflicted := 0, 0
	refs := map[string]bool{}
	for _, res := range results {
		switch res.Status {
		case 201:
			created++
			refs[res.Body.(map[string]any)["reference"].(string)] = true
		case 409:
			conflicted++
			if code := res.Body.(map[string]any)["error"].(map[string]any)["code"]; code != "table_unavailable" {
				t.Fatalf("conflict code = %v", code)
			}
		default:
			t.Fatalf("unexpected status %d", res.Status)
		}
	}
	if created != 1 || conflicted != n-1 || len(refs) != 1 {
		t.Fatalf("created=%d conflicted=%d refs=%v", created, conflicted, refs)
	}
	s.mu.Lock()
	count := len(s.state.Reservations)
	s.mu.Unlock()
	if count != 1 {
		t.Fatalf("state holds %d reservations", count)
	}
}

func TestConcurrentAmendCancelVsCreate(t *testing.T) {
	future := futureThursday(t)
	s := New()
	resetWith(t, s, seedAda, seedAnkerTables,
		seedRes("sA", "MOVING1", "u_ada", "r_anker", "t_1", future+"T19:00", 2, "confirmed"))
	adaLogin, _ := json.Marshal(map[string]any{"email": "ada@example.com", "password": "correct horse"})
	token := decodeBody(t, serveRequest(s, http.MethodPost, "/auth/login", adaLogin, nil))["token"].(string)
	var wg sync.WaitGroup
	var patchRes, cancelRes, createRes Result
	wg.Add(3)
	go func() {
		defer wg.Done()
		patchRes = s.PatchReservation(token, "MOVING1", []byte(`{"table_id":"t_2"}`))
	}()
	go func() {
		defer wg.Done()
		cancelRes = s.CancelReservation(token, "MOVING1")
	}()
	go func() {
		defer wg.Done()
		createRes = createLocked(s, "u_ada", createBody("r_anker", "t_2", future+"T20:30", 2.0))
	}()
	wg.Wait()
	for _, res := range []Result{patchRes, cancelRes, createRes} {
		if res.Status == 500 {
			t.Fatalf("5xx under concurrency: %v", res.Body)
		}
	}
	// The final record is one of the serial outcomes: cancelled, or confirmed
	// on t_1 or t_2 with no double occupancy anywhere.
	s.mu.Lock()
	defer s.mu.Unlock()
	final := s.state.Reservations["MOVING1"]
	switch {
	case final.Status == StatusCancelled:
	case final.Status == StatusConfirmed && (final.TableID == "t_1" || final.TableID == "t_2"):
		if conflictingReservation(&s.state, final, map[string]bool{final.Reference: true}) {
			t.Fatalf("final record overlaps another booking: %+v", final)
		}
	default:
		t.Fatalf("invalid final record: %+v", final)
	}
}
