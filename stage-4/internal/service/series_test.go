package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"tablekeeper/internal/history"
	"testing"
	"time"
)

// Requirement map (S3-S adoption + current lookup; R277-R282/C propagation
// helper-only, ordinary caller integration remains C):
// R259/R260/R285 auth + idempotency path + unknown fields -> TestSeriesValidationMatrix
// R261/R262 anchor errors + cutoff -> TestSeriesAnchorErrors
// R263 counts 2..12/1..4 strict -> TestSeriesValidationMatrix + TestSeriesBoundaries
// R264 anchor identity + receipts -> TestSeriesAnchorPreserved
// R265 calendar local dates -> TestSeriesCalendarDST
// R266 dated policies -> TestSeriesDatedPolicies
// R267 ordinary validation -> TestSeriesFailurePrecedence + occupancy tests
// R268 DST skip/fold -> TestSeriesCalendarDST
// R269 inherited party/tables -> TestSeriesSingletonAndPair
// R270 atomic failure/rollback + byte-identical state -> TestSeriesFailureRollback
// R271 first-failing-index precedence -> TestSeriesFailurePrecedence
// R272 response shape -> TestSeriesAdoptShape
// R273 stable identities -> TestSeriesStableIdentities
// R274 lists/occupancy/history -> TestSeriesOrdinaryBehavior
// R275 current GET -> TestSeriesCurrentLookup
// R276 privacy incl no-token -> TestSeriesPrivacy
// R277-R282 helper-only (no caller integration): TestSeriesTouchHelper
// R283 counter once -> TestSeriesCounterOnce
// R284 immutable replay -> TestSeriesReplayImmutable
// concurrency distinct-key + same-key50 -> TestSeriesConcurrency

const seriesFixture = `{"users":[
	{"id":"u1","email":"a@b","password":"password1","display_name":"A"},
	{"id":"u2","email":"c@d","password":"password1","display_name":"C"}],
	"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
	"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[
		{"weekday":"thu","opens":"18:00","closes":"23:00"},
		{"weekday":"fri","opens":"18:00","closes":"23:00"},
		{"weekday":"sun","opens":"00:00","closes":"23:30"}],
	"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
	"combinable":[["t_1","t_2"]]}],
	"reservations":[]}`

// seriesAnchor creates one anchor booking via the ordinary path and returns
// its reference and owner token.
func seriesAnchor(t *testing.T, s *Service, tok, local, tableID string, party int) string {
	t.Helper()
	body := fmt.Sprintf(`{"restaurant_id":"r1","table_id":%q,"starts_at_local":%q,"party_size":%d}`, tableID, local, party)
	res := s.CreateReservation(tok, "anchor-"+local+"-"+tableID, []byte(body))
	if res.Status != 201 {
		t.Fatalf("anchor %s: %d %v", local, res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	ref, _ := m["reference"].(string)
	if ref == "" {
		t.Fatal("anchor has no reference")
	}
	return ref
}

func seriesLogin(t *testing.T, s *Service, email string) string {
	t.Helper()
	login, _ := json.Marshal(map[string]any{"email": email, "password": "password1"})
	rec := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	if rec.Code != 200 {
		t.Fatalf("login %s: %d", email, rec.Code)
	}
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	tok, _ := b["token"].(string)
	if tok == "" {
		t.Fatal("no token")
	}
	return tok
}

func seriesSetup(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(seriesFixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	return s, seriesLogin(t, s, "a@b"), seriesLogin(t, s, "c@d")
}

func adoptBody(anchor string, count, interval int) []byte {
	return []byte(fmt.Sprintf(`{"anchor_reference":%q,"count":%d,"interval_weeks":%d}`, anchor, count, interval))
}

func TestSeriesValidationMatrix(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	cases := []struct {
		name string
		body string
		key  string
		want int
		code string
	}{
		{"missing key", `{"anchor_reference":"` + ref + `","count":2,"interval_weeks":1}`, "", 400, "missing_idempotency_key"},
		{"malformed", `{`, "m1", 400, "malformed_request"},
		{"non-object", `[1]`, "m2", 400, "malformed_request"},
		{"missing anchor", `{"count":2,"interval_weeks":1}`, "m3", 422, "validation_failed"},
		{"anchor non-string", `{"anchor_reference":5,"count":2,"interval_weeks":1}`, "m4", 400, "malformed_request"},
		{"missing count", `{"anchor_reference":"` + ref + `","interval_weeks":1}`, "m5", 422, "validation_failed"},
		{"count bool", `{"anchor_reference":"` + ref + `","count":true,"interval_weeks":1}`, "m6", 422, "validation_failed"},
		{"count string", `{"anchor_reference":"` + ref + `","count":"2","interval_weeks":1}`, "m7", 422, "validation_failed"},
		{"count null", `{"anchor_reference":"` + ref + `","count":null,"interval_weeks":1}`, "m8", 422, "validation_failed"},
		{"count fraction", `{"anchor_reference":"` + ref + `","count":2.5,"interval_weeks":1}`, "m9", 422, "validation_failed"},
		{"count 1", `{"anchor_reference":"` + ref + `","count":1,"interval_weeks":1}`, "m10", 422, "validation_failed"},
		{"count 13", `{"anchor_reference":"` + ref + `","count":13,"interval_weeks":1}`, "m11", 422, "validation_failed"},
		{"interval bool", `{"anchor_reference":"` + ref + `","count":2,"interval_weeks":false}`, "m12", 422, "validation_failed"},
		{"interval 0", `{"anchor_reference":"` + ref + `","count":2,"interval_weeks":0}`, "m13", 422, "validation_failed"},
		{"interval 5", `{"anchor_reference":"` + ref + `","count":2,"interval_weeks":5}`, "m14", 422, "validation_failed"},
		{"unknown anchor", `{"anchor_reference":"NOPE01","count":2,"interval_weeks":1}`, "m15", 404, "not_found"},
		{"unknown fields ok", `{"anchor_reference":"` + ref + `","count":2,"interval_weeks":1,"zzz":9}`, "m16", 201, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := s.AdoptSeries(tok, tc.key, []byte(tc.body))
			if res.Status != tc.want {
				t.Fatalf("status=%d want %d (%v)", res.Status, tc.want, res.Body)
			}
			if tc.code != "" {
				m, _ := res.Body.(map[string]any)
				e, _ := m["error"].(map[string]any)
				if e["code"] != tc.code {
					t.Fatalf("code=%v want %s", e["code"], tc.code)
				}
			}
		})
	}
	// No token is 401.
	if res := s.AdoptSeries("", "k401", adoptBody(ref, 2, 1)); res.Status != 401 {
		t.Fatalf("no token = %d", res.Status)
	}
	// Bad token is 401.
	if res := s.AdoptSeries("bad", "k401b", adoptBody(ref, 2, 1)); res.Status != 401 {
		t.Fatalf("bad token = %d", res.Status)
	}
}

func TestSeriesBoundaries(t *testing.T) {
	for _, tc := range []struct{ count, interval int }{{2, 1}, {12, 4}, {12, 1}, {2, 4}} {
		s, tok, _ := seriesSetup(t)
		ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
		res := s.AdoptSeries(tok, fmt.Sprintf("b-%d-%d", tc.count, tc.interval), adoptBody(ref, tc.count, tc.interval))
		if res.Status != 201 {
			t.Fatalf("count=%d interval=%d: %d %v", tc.count, tc.interval, res.Status, res.Body)
		}
		m, _ := res.Body.(map[string]any)
		occ, _ := m["occurrences"].([]any)
		if len(occ) != tc.count {
			t.Fatalf("occurrences=%d want %d", len(occ), tc.count)
		}
	}
}

func TestSeriesAnchorErrors(t *testing.T) {
	s, tok, tok2 := seriesSetup(t)
	// Foreign anchor is 404.
	foreign := seriesAnchor(t, s, tok2, "2027-05-06T19:00", "t_1", 1)
	if res := s.AdoptSeries(tok, "f1", adoptBody(foreign, 2, 1)); res.Status != 404 {
		t.Fatalf("foreign = %d", res.Status)
	}
	// Cancelled anchor is 409 reservation_cancelled.
	ref := seriesAnchor(t, s, tok, "2027-05-06T20:30", "t_1", 1)
	s.mu.Lock()
	r := s.state.Reservations[ref]
	r.Status = StatusCancelled
	s.state.Reservations[ref] = r
	s.mu.Unlock()
	// Cancel via API to keep history consistent instead of direct mutation.
	s.mu.Lock()
	r = s.state.Reservations[ref]
	r.Status = StatusConfirmed
	s.state.Reservations[ref] = r
	s.mu.Unlock()
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+ref+"/cancel", []byte("{}"),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel: %d", rec.Code)
	}
	if res := s.AdoptSeries(tok, "f2", adoptBody(ref, 2, 1)); res.Status != 409 {
		t.Fatalf("cancelled = %d %v", res.Status, res.Body)
	}
	// Already adopted is 409 already_in_series.
	ref2 := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_2", 1)
	if res := s.AdoptSeries(tok, "f3", adoptBody(ref2, 2, 1)); res.Status != 201 {
		t.Fatalf("first adopt = %d", res.Status)
	}
	if res := s.AdoptSeries(tok, "f4", adoptBody(ref2, 2, 1)); res.Status != 409 {
		t.Fatalf("re-adopt = %d %v", res.Status, res.Body)
	} else {
		m, _ := res.Body.(map[string]any)
		if m["error"].(map[string]any)["code"] != "already_in_series" {
			t.Fatalf("code = %v", res.Body)
		}
	}
}

func TestSeriesAdoptShape(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "shape1", adoptBody(ref, 3, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d %v", res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	if _, ok := m["series_id"]; !ok {
		t.Fatalf("no series_id: %v", m)
	}
	if m["revision"] != 1 || m["interval_weeks"] != 1 {
		t.Fatalf("header = %v", m)
	}
	occ, ok := m["occurrences"].([]any)
	if !ok || len(occ) != 3 {
		t.Fatalf("occurrences = %v", m["occurrences"])
	}
	seen := map[string]bool{}
	for i, o := range occ {
		om := o.(map[string]any)
		if int(om["index"].(int)) != i {
			t.Fatalf("index = %v", om)
		}
		if om["exception"] != false {
			t.Fatalf("exception = %v", om)
		}
		rm := om["reservation"].(map[string]any)
		if rm["reference"] == "" {
			t.Fatalf("no reference: %v", om)
		}
		seen[rm["reference"].(string)] = true
		if i == 0 && rm["reference"] != ref {
			t.Fatalf("occurrence 0 = %v want anchor %s", rm["reference"], ref)
		}
	}
	if len(seen) != 3 {
		t.Fatal("references not distinct")
	}
}

func TestSeriesAnchorPreserved(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	anchorBody := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T19:00","party_size":1}`
	anchorKey := "pres-anchor"
	ares := s.CreateReservation(tok, anchorKey, []byte(anchorBody))
	if ares.Status != 201 {
		t.Fatalf("anchor = %d", ares.Status)
	}
	ab, _ := json.Marshal(ares.Body)
	ref := ares.Body.(map[string]any)["reference"].(string)
	s.mu.Lock()
	beforeRec := s.state.Reservations[ref]
	beforeHist := history.CloneEntries(s.state.Histories[ref])
	s.mu.Unlock()
	res := s.AdoptSeries(tok, "pres1", adoptBody(ref, 2, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d", res.Status)
	}
	s.mu.Lock()
	afterRec := s.state.Reservations[ref]
	afterHist := s.state.Histories[ref]
	s.mu.Unlock()
	if !reflect.DeepEqual(beforeRec, afterRec) {
		t.Fatalf("anchor mutated:\n%+v\n%+v", beforeRec, afterRec)
	}
	rb, _ := json.Marshal(beforeHist)
	ab2, _ := json.Marshal(afterHist)
	if string(rb) != string(ab2) {
		t.Fatalf("anchor history mutated:\n%s\n%s", rb, ab2)
	}
	// Original anchor create receipt replays 200 with complete original bytes.
	rep := s.CreateReservation(tok, anchorKey, []byte(anchorBody))
	if rep.Status != 200 {
		t.Fatalf("anchor replay = %d", rep.Status)
	}
	rb2b, _ := json.Marshal(rep.Body)
	if string(ab) != string(rb2b) {
		t.Fatalf("anchor replay differs:\n%s\n%s", ab, rb2b)
	}
}

func TestSeriesSingletonAndPair(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	pairBody := `{"restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T19:00","party_size":5}`
	pres := s.CreateReservation(tok, "pair-anchor", []byte(pairBody))
	if pres.Status != 201 {
		t.Fatalf("pair anchor = %d %v", pres.Status, pres.Body)
	}
	pref := pres.Body.(map[string]any)["reference"].(string)
	res := s.AdoptSeries(tok, "pair-series", adoptBody(pref, 2, 1))
	if res.Status != 201 {
		t.Fatalf("pair adopt = %d %v", res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	rm := occ[1].(map[string]any)["reservation"].(map[string]any)
	if !reflect.DeepEqual(rm["table_ids"], []any{"t_1", "t_2"}) && !reflect.DeepEqual(rm["table_ids"], []string{"t_1", "t_2"}) {
		t.Fatalf("generated pair = %v", rm["table_ids"])
	}
	if _, ok := rm["table_id"]; ok {
		t.Fatalf("pair must omit table_id: %v", rm)
	}
}

func TestSeriesCalendarDST(t *testing.T) {
	// Spring gap: anchor Sunday 2027-03-21 02:30 Berlin; +7d lands
	// 2027-03-28 (spring forward), whose 02:30 wall does not exist ->
	// invalid_local_time with full rollback.
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-03-21T02:30", "t_1", 1)
	before := string(exportBytes(t, s))
	res := s.AdoptSeries(tok, "gap1", adoptBody(ref, 2, 1))
	if res.Status != 422 {
		t.Fatalf("gap = %d %v", res.Status, res.Body)
	}
	if m, ok := res.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "invalid_local_time" {
			t.Fatalf("gap code = %v", res.Body)
		}
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("gap failure mutated state")
	}
	// Fall fold: anchor Sunday 2027-10-17 02:30 Berlin; +7d lands 2027-10-24?
	// Berlin 2027 fall-back is 2027-10-31 (last Sunday). Anchor 2027-10-24
	// 02:30 +1wk lands on the repeated 02:30 wall and must take the first
	// (CEST, +02:00) occurrence with local clock preserved.
	s2, tok2, _ := seriesSetup(t)
	ref2 := seriesAnchor(t, s2, tok2, "2027-10-24T02:30", "t_1", 1)
	res2 := s2.AdoptSeries(tok2, "fold1", adoptBody(ref2, 2, 1))
	if res2.Status != 201 {
		t.Fatalf("fold = %d %v", res2.Status, res2.Body)
	}
	m := res2.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	rm := occ[1].(map[string]any)["reservation"].(map[string]any)
	if rm["starts_at_local"] != "2027-10-31T02:30" {
		t.Fatalf("fold local = %v", rm["starts_at_local"])
	}
	if !strings.HasSuffix(rm["starts_at"].(string), "+02:00") {
		t.Fatalf("fold first occurrence offset = %v (want +02:00 CEST)", rm["starts_at"])
	}
}

func TestSeriesSantiagoCalendar(t *testing.T) {
	// F2: timezone-independent calendar arithmetic. Santiago 2027-08-29 is a
	// Sunday; +7d must be 2027-09-05 (seven calendar days), not 2027-09-04
	// from local-midnight AddDate normalization in IANA zones.
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"America/Santiago","slot_minutes":30,
		"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,
		"opening_hours":[
			{"weekday":"sun","opens":"18:00","closes":"23:00"},
			{"weekday":"mon","opens":"18:00","closes":"23:00"},
			{"weekday":"tue","opens":"18:00","closes":"23:00"},
			{"weekday":"wed","opens":"18:00","closes":"23:00"},
			{"weekday":"thu","opens":"18:00","closes":"23:00"},
			{"weekday":"fri","opens":"18:00","closes":"23:00"},
			{"weekday":"sat","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}],"reservations":[]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := seriesLogin(t, s, "a@b")
	body := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-08-29T19:00","party_size":1}`
	ares := s.CreateReservation(tok, "sanchor", []byte(body))
	if ares.Status != 201 {
		t.Fatalf("anchor = %d %v", ares.Status, ares.Body)
	}
	ref := ares.Body.(map[string]any)["reference"].(string)
	res := s.AdoptSeries(tok, "scal1", adoptBody(ref, 2, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d %v", res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	rm := occ[1].(map[string]any)["reservation"].(map[string]any)
	if rm["starts_at_local"] != "2027-09-05T19:00" {
		t.Fatalf("scheduled local = %v, want 2027-09-05T19:00", rm["starts_at_local"])
	}
	got := s.GetSeries(tok, m["series_id"].(string))
	gm, _ := got.Body.(map[string]any)
	gocc := gm["occurrences"].([]any)
	grm := gocc[1].(map[string]any)["reservation"].(map[string]any)
	if grm["starts_at_local"] != "2027-09-05T19:00" {
		t.Fatalf("stored local = %v", grm["starts_at_local"])
	}
}

func TestSeriesDatedPolicies(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	// Publish a policy effective 2027-05-13 with duration 60 and capacities.
	mgr := seriesManager(t, s)
	pub := `{"effective_from":"2027-05-13","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`
	pres := serveRequest(s, http.MethodPost, "/restaurants/r1/policies", []byte(pub),
		map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "pol1"})
	if pres.Code != 201 {
		t.Fatalf("publish = %d %s", pres.Code, pres.Body.String())
	}
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "dated1", adoptBody(ref, 2, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d %v", res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	r0 := occ[0].(map[string]any)["reservation"].(map[string]any)
	r1 := occ[1].(map[string]any)["reservation"].(map[string]any)
	if r0["revision"] != 1 || r1["revision"] != 1 {
		t.Fatalf("revisions = %v %v", r0["revision"], r1["revision"])
	}
	t0 := r0["accepted_terms"].(map[string]any)
	t1 := r1["accepted_terms"].(map[string]any)
	if t0["policy_version"] != 0 || t1["policy_version"] != 1 {
		t.Fatalf("versions = %v %v", t0["policy_version"], t1["policy_version"])
	}
	if t0["reservation_duration_minutes"] != 90 || t1["reservation_duration_minutes"] != 60 {
		t.Fatalf("durations = %v %v", t0["reservation_duration_minutes"], t1["reservation_duration_minutes"])
	}
	el0, _ := time.Parse(time.RFC3339, r0["ends_at"].(string))
	st0, _ := time.Parse(time.RFC3339, r0["starts_at"].(string))
	el1, _ := time.Parse(time.RFC3339, r1["ends_at"].(string))
	st1, _ := time.Parse(time.RFC3339, r1["starts_at"].(string))
	if el0.Sub(st0) != 90*time.Minute || el1.Sub(st1) != 60*time.Minute {
		t.Fatalf("elapsed = %v %v", el0.Sub(st0), el1.Sub(st1))
	}
	switch v := t1["capacities"].(map[string]any)["t_1"]; v {
	case float64(2), 2:
	default:
		t.Fatalf("selected capacities = %v", t1["capacities"])
	}
}

func seriesManager(t *testing.T, s *Service) string {
	t.Helper()
	s.mu.Lock()
	for i := range s.state.Restaurants {
		if s.state.Restaurants[i].ID == "r1" {
			s.state.Restaurants[i].ManagerUserIDs = []string{"u1"}
		}
	}
	s.mu.Unlock()
	return seriesLogin(t, s, "a@b")
}

func TestSeriesFailurePrecedence(t *testing.T) {
	// R271: TWO independent failures — index 1 externally occupied (409
	// table_unavailable) while index 2 is independently invalid under a
	// dated policy effective only at index 2's date (closed Thursday hours
	// removed -> outside_opening_hours). The FIRST failing index determines
	// the error: 409 table_unavailable, with complete export equality.
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	// External occupant for index 1 (2027-05-13), built BEFORE publication
	// under plain fixture terms.
	ext := seriesAnchor(t, s, tok, "2027-05-13T19:00", "t_1", 1)
	_ = ext
	mgr := seriesManager(t, s)
	closed := `{"effective_from":"2027-05-20","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"fri","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`
	pres := serveRequest(s, http.MethodPost, "/restaurants/r1/policies", []byte(closed),
		map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "precpol"})
	if pres.Code != 201 {
		t.Fatalf("publish = %d %s", pres.Code, pres.Body.String())
	}
	before := string(exportBytes(t, s))
	res := s.AdoptSeries(tok, "prec1", adoptBody(ref, 3, 1))
	if res.Status != 409 {
		t.Fatalf("precedence = %d %v", res.Status, res.Body)
	}
	if m, ok := res.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "table_unavailable" {
			t.Fatalf("first-index code = %v", res.Body)
		}
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("precedence failure mutated state")
	}
	// Reverse: occupy only index 2 (2027-05-20); index 1 succeeds then
	// index 2 reports the occupancy conflict — still index-ordered.
	s2, tok2, _ := seriesSetup(t)
	refB := seriesAnchor(t, s2, tok2, "2027-05-06T19:00", "t_1", 1)
	_ = seriesAnchor(t, s2, tok2, "2027-05-20T19:00", "t_1", 1)
	resB := s2.AdoptSeries(tok2, "prec2", adoptBody(refB, 3, 1))
	if resB.Status != 409 {
		t.Fatalf("late-index occupancy = %d %v", resB.Status, resB.Body)
	}
}

func TestSeriesLongAnchorConflict(t *testing.T) {
	// F1: producer-valid long anchor (fixture duration 20160 = 14 days)
	// occupies the generated date under a shorter future policy. The
	// generated 2027-06-24T19:00 (60min) falls inside the anchor interval
	// 2027-06-17T19:00 + 14d, so adoption must be 409 with full rollback.
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":20160,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}],
		"reservations":[
			{"id":"s9","reference":"LONGSEED","user_id":"u1","restaurant_id":"r1",
			"table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":1}]}`
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := seriesLogin(t, s, "a@b")
	mgr := seriesManager(t, s)
	pub := `{"effective_from":"2027-06-18","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2}}`
	pres := serveRequest(s, http.MethodPost, "/restaurants/r1/policies", []byte(pub),
		map[string]string{"Authorization": "Bearer " + mgr, "Idempotency-Key": "longpol"})
	if pres.Code != 201 {
		t.Fatalf("publish: %d %s", pres.Code, pres.Body.String())
	}
	before := string(exportBytes(t, s))
	res := s.AdoptSeries(tok, "long1", adoptBody("LONGSEED", 2, 1))
	if res.Status != 409 {
		t.Fatalf("long anchor conflict = %d %v, want 409", res.Status, res.Body)
	}
	if m, ok := res.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "table_unavailable" {
			t.Fatalf("code = %v", res.Body)
		}
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("long-anchor failure mutated state")
	}
	// Failed key reusable: a fresh anchor after the 14-day shadow adopts anew.
	// LONGSEED covers t_1 until 2027-07-01T19:00; 2027-07-01T19:00 is adjacent.
	freshBody := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-07-01T19:00","party_size":1}`
	fres := s.CreateReservation(tok, "long-fresh", []byte(freshBody))
	if fres.Status != 201 {
		t.Fatalf("fresh anchor = %d %v", fres.Status, fres.Body)
	}
	freshRef := fres.Body.(map[string]any)["reference"].(string)
	if res2 := s.AdoptSeries(tok, "long1", adoptBody(freshRef, 2, 1)); res2.Status != 201 {
		t.Fatalf("failed key reuse = %d %v", res2.Status, res2.Body)
	}
}

func TestSeriesLaterIndexRollback(t *testing.T) {
	// F1: index 1 provisionally creates a record + history, then index 2
	// conflicts: the whole export (records/histories/receipts/counters)
	// must be byte-identical and the failed key reusable.
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	_ = seriesAnchor(t, s, tok, "2027-05-20T19:00", "t_1", 1)
	before := string(exportBytes(t, s))
	res := s.AdoptSeries(tok, "later1", adoptBody(ref, 3, 1))
	if res.Status != 409 {
		t.Fatalf("later-index conflict = %d %v", res.Status, res.Body)
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("later-index failure leaked provisional creation/history")
	}
	fresh := seriesAnchor(t, s, tok, "2027-05-06T20:30", "t_1", 1)
	if res2 := s.AdoptSeries(tok, "later1", adoptBody(fresh, 2, 1)); res2.Status != 201 {
		t.Fatalf("failed key reuse = %d %v", res2.Status, res2.Body)
	}
}

func TestSeriesFailureRollback(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	_ = seriesAnchor(t, s, tok, "2027-05-13T19:00", "t_1", 1)
	before := string(exportBytes(t, s))
	res := s.AdoptSeries(tok, "roll1", adoptBody(ref, 2, 1))
	if res.Status != 409 {
		t.Fatalf("occupancy failure = %d %v", res.Status, res.Body)
	}
	if m, ok := res.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "table_unavailable" {
			t.Fatalf("code = %v", res.Body)
		}
	}
	after := string(exportBytes(t, s))
	if after != before {
		t.Fatal("whole export changed on failure")
	}
	// Failed key reusable with a fresh anchor.
	fresh := seriesAnchor(t, s, tok, "2027-05-06T20:30", "t_1", 1)
	before2 := string(exportBytes(t, s))
	res2 := s.AdoptSeries(tok, "roll1", adoptBody(fresh, 2, 1))
	if res2.Status != 201 {
		t.Fatalf("failed key reuse = %d %v", res2.Status, res2.Body)
	}
	_ = before2
}

func TestSeriesStableIdentities(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "stab1", adoptBody(ref, 3, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d", res.Status)
	}
	m, _ := res.Body.(map[string]any)
	sid := m["series_id"].(string)
	got := s.GetSeries(tok, sid)
	if got.Status != 200 {
		t.Fatalf("get = %d", got.Status)
	}
	gm, _ := got.Body.(map[string]any)
	if gm["series_id"] != sid || gm["revision"] != 1 {
		t.Fatalf("series = %v", gm)
	}
}

func TestSeriesCurrentLookup(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "cur1", adoptBody(ref, 2, 1))
	m, _ := res.Body.(map[string]any)
	sid := m["series_id"].(string)
	// Owner list contains generated members; each has history.
	got := s.GetSeries(tok, sid)
	gm, _ := got.Body.(map[string]any)
	for _, o := range gm["occurrences"].([]any) {
		om := o.(map[string]any)
		rm := om["reservation"].(map[string]any)
		lr := s.GetReservation(tok, rm["reference"].(string))
		if lr.Status != 200 {
			t.Fatalf("lookup %v = %d", rm["reference"], lr.Status)
		}
		hr := s.ReservationHistory(tok, rm["reference"].(string))
		if hr.Status != 200 {
			t.Fatalf("history %v = %d", rm["reference"], hr.Status)
		}
	}
	// Anchor occurrence keeps original revision/terms.
	ar := s.GetReservation(tok, ref)
	am, _ := ar.Body.(map[string]any)
	if am["revision"] != 1 {
		t.Fatalf("anchor revision = %v", am["revision"])
	}
}

func TestSeriesOrdinaryBehavior(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "ord1", adoptBody(ref, 2, 1))
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	genRef := occ[1].(map[string]any)["reservation"].(map[string]any)["reference"].(string)
	// Generated occurrence appears in the owner list and occupies its table.
	lr := s.ListReservations(tok)
	lm, _ := lr.Body.(map[string]any)
	found := false
	for _, r := range lm["reservations"].([]any) {
		if r.(map[string]any)["reference"] == genRef {
			found = true
		}
	}
	if !found {
		t.Fatal("generated occurrence missing from list")
	}
	// The generated occurrence occupies its table: a conflicting ordinary
	// booking on the same member/slot is 409 with unchanged state.
	before := string(exportBytes(t, s))
	dupBody := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-13T19:00","party_size":1}`
	dup := s.CreateReservation(tok, "dup-conflict", []byte(dupBody))
	if dup.Status != 409 {
		t.Fatalf("conflicting booking = %d, want 409", dup.Status)
	}
	if m, ok := dup.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "table_unavailable" {
			t.Fatalf("code = %v", dup.Body)
		}
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("conflicting booking mutated state")
	}
}

func TestSeriesPrivacy(t *testing.T) {
	s, tok, tok2 := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "priv1", adoptBody(ref, 2, 1))
	sid := res.Body.(map[string]any)["series_id"].(string)
	if got := s.GetSeries(tok2, sid); got.Status != 404 {
		t.Fatalf("foreign = %d", got.Status)
	}
	if got := s.GetSeries("", sid); got.Status != 404 {
		t.Fatalf("no token = %d", got.Status)
	}
	if got := s.GetSeries(tok, "nosuch"); got.Status != 404 {
		t.Fatalf("unknown = %d", got.Status)
	}
}

func TestSeriesTouchHelper(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	res := s.AdoptSeries(tok, "touch1", adoptBody(ref, 3, 1))
	m, _ := res.Body.(map[string]any)
	sid := m["series_id"].(string)
	occ := m["occurrences"].([]any)
	refs := []string{
		occ[0].(map[string]any)["reservation"].(map[string]any)["reference"].(string),
		occ[1].(map[string]any)["reservation"].(map[string]any)["reference"].(string),
	}
	// Coalescing: repeated refs + two members in one batch increment once.
	s.withLock(func(st *State) {
		touchSeriesForChanges(st, []string{refs[0], refs[0], refs[1]}, true)
	})
	s.mu.Lock()
	rev := s.state.Series[sid].Revision
	e0 := s.state.Series[sid].Members[0].Exception
	e1 := s.state.Series[sid].Members[1].Exception
	e2 := s.state.Series[sid].Members[2].Exception
	s.mu.Unlock()
	if rev != 2 {
		t.Fatalf("revision = %d want 2", rev)
	}
	if !e0 || !e1 || e2 {
		t.Fatalf("flags = %v %v %v", e0, e1, e2)
	}
	// markException=false retains flags and still increments once.
	s.withLock(func(st *State) {
		touchSeriesForChanges(st, []string{refs[2%len(refs)]}, false)
	})
	s.mu.Lock()
	rev2 := s.state.Series[sid].Revision
	f0 := s.state.Series[sid].Members[0].Exception
	s.mu.Unlock()
	if rev2 != 3 || !f0 {
		t.Fatalf("retain = rev %d flag %v", rev2, f0)
	}
	// Unrelated refs do nothing.
	s.withLock(func(st *State) {
		touchSeriesForChanges(st, []string{"NOPE01"}, true)
	})
	s.mu.Lock()
	rev3 := s.state.Series[sid].Revision
	s.mu.Unlock()
	if rev3 != 3 {
		t.Fatalf("unrelated touched revision: %d", rev3)
	}
	// Membership lookup.
	gotID, idx, found := seriesMembershipOf(s, refs[1])
	if !found || gotID != sid || idx != 1 {
		t.Fatalf("membership = %s %d %v", gotID, idx, found)
	}
	if _, _, found := seriesMembershipOf(s, "NOPE01"); found {
		t.Fatal("phantom membership")
	}
}

func seriesMembershipOf(s *Service, ref string) (string, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return seriesMembership(&s.state, ref)
}

func TestSeriesCounterOnce(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	s.mu.Lock()
	before := s.state.RestaurantRevisions["r1"]
	s.mu.Unlock()
	res := s.AdoptSeries(tok, "cnt1", adoptBody(ref, 4, 1))
	if res.Status != 201 {
		t.Fatalf("adopt = %d", res.Status)
	}
	s.mu.Lock()
	after := s.state.RestaurantRevisions["r1"]
	s.mu.Unlock()
	if after-before != 1 {
		t.Fatalf("counter delta = %d", after-before)
	}
}

func TestSeriesReplayImmutable(t *testing.T) {
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	raw := adoptBody(ref, 2, 1)
	res := s.AdoptSeries(tok, "rep1", raw)
	if res.Status != 201 {
		t.Fatalf("adopt = %d", res.Status)
	}
	rb1, _ := json.Marshal(res.Body)
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	genRef := occ[1].(map[string]any)["reservation"].(map[string]any)["reference"].(string)
	// Mutate, then really cancel the generated occurrence after adoption.
	if rec := serveRequest(s, http.MethodPatch, "/reservations/"+genRef, []byte(`{"party_size":2}`),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("mutate: %d", rec.Code)
	}
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+genRef+"/cancel", []byte("{}"),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	// Current GET is cancelled and differs from the original receipt.
	cur := s.GetReservation(tok, genRef)
	cm, _ := cur.Body.(map[string]any)
	if cm["status"] != StatusCancelled || cm["party_size"] != 2 {
		t.Fatalf("current = %v, want cancelled party 2", cur.Body)
	}
	// Same key/body replays 200 with the original complete bytes, even after
	// the amendment and cancellation. Capture full state immediately BEFORE
	// replay; AFTER must be byte-identical: no metadata, revision, counter
	// or receipt growth from the replay itself.
	preReplay := string(exportBytes(t, s))
	rep := s.AdoptSeries(tok, "rep1", raw)
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	rb2, _ := json.Marshal(rep.Body)
	if string(rb1) != string(rb2) {
		t.Fatalf("replay differs:\n%s\n%s", rb1, rb2)
	}
	if post := string(exportBytes(t, s)); post != preReplay {
		t.Fatal("replay grew state")
	}
	// Committed key with a DIFFERENT body is 409 idempotency_key_reuse with
	// unchanged export — not a failed-key reuse.
	ref2 := seriesAnchor(t, s, tok, "2027-05-06T20:30", "t_1", 1)
	beforeReuse := string(exportBytes(t, s))
	reuse := s.AdoptSeries(tok, "rep1", adoptBody(ref2, 2, 1))
	if reuse.Status != 409 {
		t.Fatalf("committed-key reuse = %d, want 409", reuse.Status)
	}
	if m, ok := reuse.Body.(map[string]any); ok {
		if m["error"].(map[string]any)["code"] != "idempotency_key_reuse" {
			t.Fatalf("reuse code = %v", reuse.Body)
		}
	}
	if got := string(exportBytes(t, s)); got != beforeReuse {
		t.Fatal("key-reuse check mutated state")
	}
}

func TestSeriesConcurrency(t *testing.T) {
	// Distinct keys on the same anchor: exactly one adoption wins.
	s, tok, _ := seriesSetup(t)
	ref := seriesAnchor(t, s, tok, "2027-05-06T19:00", "t_1", 1)
	const n = 10
	var wg sync.WaitGroup
	type out struct {
		status int
		body   string
	}
	results := make([]out, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := s.AdoptSeries(tok, fmt.Sprintf("conc-%d", i), adoptBody(ref, 2, 1))
			rb, _ := json.Marshal(r.Body)
			results[i] = out{r.Status, string(rb)}
		}(i)
	}
	wg.Wait()
	var wins int
	for _, r := range results {
		if r.status == 201 {
			wins++
		} else if r.status != 409 {
			t.Fatalf("status = %d %s", r.status, r.body)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d", wins)
	}
	// Same key x50: exactly one 201 + 49x200, all byte-identical, counter once.
	s2, tok2, _ := seriesSetup(t)
	ref2 := seriesAnchor(t, s2, tok2, "2027-05-06T19:00", "t_1", 1)
	const m = 50
	results2 := make([]out, m)
	var wg2 sync.WaitGroup
	for i := 0; i < m; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			r := s2.AdoptSeries(tok2, "samekey", adoptBody(ref2, 2, 1))
			rb, _ := json.Marshal(r.Body)
			results2[i] = out{r.Status, string(rb)}
		}(i)
	}
	wg2.Wait()
	var c201, c200 int
	for _, r := range results2 {
		switch r.status {
		case 201:
			c201++
		case 200:
			c200++
		default:
			t.Fatalf("status = %d %s", r.status, r.body)
		}
	}
	if c201 != 1 || c200 != m-1 {
		t.Fatalf("201=%d 200=%d", c201, c200)
	}
	for i := 1; i < m; i++ {
		if results2[i].body != results2[0].body {
			t.Fatal("replay bodies differ")
		}
	}
	s2.mu.Lock()
	rev := s2.state.RestaurantRevisions["r1"]
	nseries := len(s2.state.Series)
	s2.mu.Unlock()
	if nseries != 1 || rev != 2 {
		t.Fatalf("series=%d counter=%d (anchor 1 + adoption 1)", nseries, rev)
	}
}
