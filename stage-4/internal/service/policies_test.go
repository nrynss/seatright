package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

const policyFixture = `{
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}
  ],
  "restaurants": [
    {
      "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [
        {"weekday": "thu", "opens": "18:00", "closes": "23:00"},
        {"weekday": "fri", "opens": "18:00", "closes": "23:30"},
        {"weekday": "sun", "opens": "00:00", "closes": "05:00"}
      ],
      "tables": [
        {"id": "t_1", "label": "1", "capacity": 2},
        {"id": "t_2", "label": "2", "capacity": 4},
        {"id": "t_3", "label": "3", "capacity": 4}
      ],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
      "manager_user_ids": ["u_ada"]
    },
    {
      "id": "r_baar", "name": "Baar", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_9", "label": "9", "capacity": 6}],
      "manager_user_ids": ["u_ada"]
    },
    {
      "id": "r_ny", "name": "New York", "timezone": "America/New_York",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "sun", "opens": "00:00", "closes": "05:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
      "manager_user_ids": ["u_ada"]
    }
  ],
  "reservations": []
}`

func newPolicyService(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(policyFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	return s, mgr, diner
}

func policyLogin(t *testing.T, s *Service, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password})
	res := s.Login(body)
	if res.Status != 200 {
		t.Fatalf("login %s: %d %v", email, res.Status, res.Body)
	}
	token, _ := res.Body.(map[string]any)["token"].(string)
	if token == "" {
		t.Fatalf("login %s returned no token", email)
	}
	return token
}

func validPolicyBody(effective string) []byte {
	body, _ := json.Marshal(map[string]any{
		"effective_from":               effective,
		"slot_minutes":                 60,
		"reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes":  60,
		"opening_hours":                []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                   map[string]any{"t_1": 4, "t_2": 6, "t_3": 4},
	})
	return body
}

func resultCode(res Result) (int, string) {
	m, ok := res.Body.(map[string]any)
	if !ok {
		return res.Status, ""
	}
	errObj, _ := m["error"].(map[string]any)
	code, _ := errObj["code"].(string)
	return res.Status, code
}

func marshalBody(t *testing.T, res Result) string {
	t.Helper()
	raw, err := json.Marshal(res.Body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(raw)
}

func TestPolicyPublishPermissions(t *testing.T) {
	s, mgr, diner := newPolicyService(t)
	res := s.PublishPolicy(mgr, "r_anker", "perm-01", validPolicyBody("2027-06-01"))
	if res.Status != 201 {
		t.Fatalf("manager publish: %d %v", res.Status, res.Body)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(marshalBody(t, res)), &v); err != nil {
		t.Fatal(err)
	}
	if v["policy_version"] != float64(1) || v["effective_from"] != "2027-06-01" {
		t.Fatalf("publish response = %v", v)
	}
	for _, k := range []string{"slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"} {
		if _, ok := v[k]; !ok {
			t.Fatalf("publish response missing %s: %v", k, v)
		}
	}
	if status, code := resultCode(s.PublishPolicy(diner, "r_anker", "perm-02", validPolicyBody("2027-06-02"))); status != 403 || code != "forbidden" {
		t.Fatalf("non-manager publish = %d %s", status, code)
	}
	if status, code := resultCode(s.PublishPolicy("", "r_anker", "perm-03", validPolicyBody("2027-06-03"))); status != 401 || code != "unauthenticated" {
		t.Fatalf("missing token publish = %d %s", status, code)
	}
	if status, code := resultCode(s.PublishPolicy("nope", "r_anker", "perm-04", validPolicyBody("2027-06-04"))); status != 401 || code != "unauthenticated" {
		t.Fatalf("bad token publish = %d %s", status, code)
	}
	if status, code := resultCode(s.PublishPolicy(mgr, "r_nope", "perm-05", validPolicyBody("2027-06-05"))); status != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant publish = %d %s", status, code)
	}
	// A manager of one restaurant is a non-manager everywhere else, and gains
	// no private lookup through publication.
	if status, code := resultCode(s.ListPolicies("r_nope")); status != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant list = %d %s", status, code)
	}
}

func TestPolicyValidationMatrix(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	base := func() map[string]any {
		return map[string]any{
			"effective_from":               "2027-06-01",
			"slot_minutes":                 30,
			"reservation_duration_minutes": 90,
			"cancellation_cutoff_minutes":  120,
			"opening_hours":                []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
			"capacities":                   map[string]any{"t_1": 2, "t_2": 4, "t_3": 4},
		}
	}
	badHours := func(h []any) map[string]any { m := base(); m["opening_hours"] = h; return m }
	badCaps := func(c map[string]any) map[string]any { m := base(); m["capacities"] = c; return m }
	cases := []struct {
		name string
		mut  func(map[string]any) map[string]any
		raw  []byte
	}{
		{"missing effective_from", func(m map[string]any) map[string]any { delete(m, "effective_from"); return m }, nil},
		{"missing slot", func(m map[string]any) map[string]any { delete(m, "slot_minutes"); return m }, nil},
		{"missing duration", func(m map[string]any) map[string]any { delete(m, "reservation_duration_minutes"); return m }, nil},
		{"missing cutoff", func(m map[string]any) map[string]any { delete(m, "cancellation_cutoff_minutes"); return m }, nil},
		{"missing hours", func(m map[string]any) map[string]any { delete(m, "opening_hours"); return m }, nil},
		{"missing capacities", func(m map[string]any) map[string]any { delete(m, "capacities"); return m }, nil},
		{"bool slot", func(m map[string]any) map[string]any { m["slot_minutes"] = true; return m }, nil},
		{"string slot", func(m map[string]any) map[string]any { m["slot_minutes"] = "30"; return m }, nil},
		{"slot zero", func(m map[string]any) map[string]any { m["slot_minutes"] = 0; return m }, nil},
		{"slot too big", func(m map[string]any) map[string]any { m["slot_minutes"] = 1441; return m }, nil},
		{"duration bool", func(m map[string]any) map[string]any { m["reservation_duration_minutes"] = false; return m }, nil},
		{"cutoff negative", func(m map[string]any) map[string]any { m["cancellation_cutoff_minutes"] = -1; return m }, nil},
		{"cutoff too big", func(m map[string]any) map[string]any { m["cancellation_cutoff_minutes"] = 10081; return m }, nil},
		{"cutoff bool", func(m map[string]any) map[string]any { m["cancellation_cutoff_minutes"] = true; return m }, nil},
		{"bad date format", func(m map[string]any) map[string]any { m["effective_from"] = "06/01/2027"; return m }, nil},
		{"non date", func(m map[string]any) map[string]any { m["effective_from"] = "not-a-date"; return m }, nil},
		{"non leap feb29", func(m map[string]any) map[string]any { m["effective_from"] = "2027-02-29"; return m }, nil},
		{"month 13", func(m map[string]any) map[string]any { m["effective_from"] = "2027-13-01"; return m }, nil},
		{"duplicate weekday", func(m map[string]any) map[string]any {
			return badHours([]any{
				map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
				map[string]any{"weekday": "thu", "opens": "19:00", "closes": "22:00"},
			})
		}, nil},
		{"bad weekday", func(m map[string]any) map[string]any {
			return badHours([]any{map[string]any{"weekday": "funday", "opens": "18:00", "closes": "23:00"}})
		}, nil},
		{"overnight hours", func(m map[string]any) map[string]any {
			return badHours([]any{map[string]any{"weekday": "thu", "opens": "22:00", "closes": "06:00"}})
		}, nil},
		{"bad opens", func(m map[string]any) map[string]any {
			return badHours([]any{map[string]any{"weekday": "thu", "opens": "6pm", "closes": "23:00"}})
		}, nil},
		{"capacities missing table", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": 4})
		}, nil},
		{"capacities extra table", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": 4, "t_3": 4, "t_9": 1})
		}, nil},
		{"capacity zero", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": 4, "t_3": 0})
		}, nil},
		{"capacity too big", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": 4, "t_3": 101})
		}, nil},
		{"capacity bool", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": true, "t_3": 4})
		}, nil},
		{"capacity string", func(m map[string]any) map[string]any {
			return badCaps(map[string]any{"t_1": 2, "t_2": "4", "t_3": 4})
		}, nil},
		{"malformed json", nil, []byte(`{"effective_from":`)},
		{"json array", nil, []byte(`[1,2]`)},
		{"empty body", nil, []byte(``)},
	}
	for i, tc := range cases {
		raw := tc.raw
		if raw == nil {
			var err error
			raw, err = json.Marshal(tc.mut(base()))
			if err != nil {
				t.Fatal(err)
			}
		}
		res := s.PublishPolicy(mgr, "r_anker", fmt.Sprintf("bad-%02d", i), raw)
		want := 422
		wantCode := "validation_failed"
		if tc.name == "malformed json" || tc.name == "json array" || tc.name == "empty body" {
			want, wantCode = 400, "malformed_request"
		}
		if status, code := resultCode(res); status != want || code != wantCode {
			t.Errorf("%s: got %d %s, want %d %s", tc.name, status, code, want, wantCode)
		}
	}
	if res := s.ListPolicies("r_anker"); marshalBody(t, res) != `{"policies":[]}` {
		t.Fatalf("invalid publications changed state: %s", marshalBody(t, res))
	}
	s.mu.Lock()
	n := s.state.RestaurantRevisions["r_anker"]
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("restaurant revision after failures = %d, want 0", n)
	}
	// A caller-supplied version is ignored: allocation starts at 1.
	m := base()
	m["policy_version"] = 99
	raw, _ := json.Marshal(m)
	res := s.PublishPolicy(mgr, "r_anker", "version-ignored", raw)
	if res.Status != 201 {
		t.Fatalf("publish with version field: %d %v", res.Status, res.Body)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(marshalBody(t, res)), &v); err != nil {
		t.Fatal(err)
	}
	if v["policy_version"] != float64(1) {
		t.Fatalf("caller version honored: %v", v)
	}
}

func TestPolicyVersionsAndSelection(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	pub := func(rest, key, eff string, caps map[string]any) Result {
		body, _ := json.Marshal(map[string]any{
			"effective_from": eff, "slot_minutes": 30, "reservation_duration_minutes": 90,
			"cancellation_cutoff_minutes": 120,
			"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
			"capacities":                  caps,
		})
		return s.PublishPolicy(mgr, rest, key, body)
	}
	ankerCaps := map[string]any{"t_1": 2, "t_2": 4, "t_3": 4}
	// Publication order differs from effective-date order.
	if res := pub("r_anker", "v-a", "2027-07-01", ankerCaps); res.Status != 201 {
		t.Fatalf("publish v1: %d", res.Status)
	}
	if res := pub("r_anker", "v-b", "2027-06-01", ankerCaps); res.Status != 201 {
		t.Fatalf("publish v2: %d", res.Status)
	}
	// Per-restaurant versions are independent.
	if res := pub("r_baar", "v-c", "2027-06-01", map[string]any{"t_9": 6}); res.Status != 201 {
		t.Fatalf("publish baar: %d", res.Status)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(marshalBody(t, s.ListPolicies("r_baar"))), &v); err != nil {
		t.Fatal(err)
	}
	pols := v["policies"].([]any)
	if len(pols) != 1 || pols[0].(map[string]any)["policy_version"] != float64(1) {
		t.Fatalf("baar policies = %v", pols)
	}
	s.mu.Lock()
	st := &s.state
	anker := restaurantByID(st, "r_anker")
	before, err := selectedTerms(st, anker, "2027-05-01")
	mid, err2 := selectedTerms(st, anker, "2027-06-15")
	late, err3 := selectedTerms(st, anker, "2027-07-15")
	s.mu.Unlock()
	if err != nil || err2 != nil || err3 != nil {
		t.Fatalf("selection errors: %v %v %v", err, err2, err3)
	}
	if before.PolicyVersion != 0 || before.SlotMinutes != 30 || before.Capacities["t_1"] != 2 {
		t.Fatalf("policy0 before first = %+v", before)
	}
	if mid.PolicyVersion != 2 {
		t.Fatalf("2027-06-15 selected v%d, want 2 (greatest date <= day)", mid.PolicyVersion)
	}
	if late.PolicyVersion != 1 {
		t.Fatalf("2027-07-15 selected v%d, want 1", late.PolicyVersion)
	}
	// Same-date tie: the greatest version wins.
	if res := pub("r_anker", "v-d", "2027-06-01", ankerCaps); res.Status != 201 {
		t.Fatalf("publish v3: %d", res.Status)
	}
	s.mu.Lock()
	tied, err := selectedTerms(st, anker, "2027-06-15")
	s.mu.Unlock()
	if err != nil || tied.PolicyVersion != 3 {
		t.Fatalf("same-date tie selected %+v, %v", tied, err)
	}
	// Past effective dates are accepted.
	if res := pub("r_anker", "v-e", "2020-01-02", ankerCaps); res.Status != 201 {
		t.Fatalf("past effective date: %d %v", res.Status, res.Body)
	}
	// Publication order is preserved in the list.
	var list map[string]any
	if err := json.Unmarshal([]byte(marshalBody(t, s.ListPolicies("r_anker"))), &list); err != nil {
		t.Fatal(err)
	}
	got := list["policies"].([]any)
	if len(got) != 4 {
		t.Fatalf("policies = %v", got)
	}
	for i, want := range []float64{1, 2, 3, 4} {
		if got[i].(map[string]any)["policy_version"] != want {
			t.Fatalf("policy order = %v", got)
		}
	}
}

func TestPublishNoRetroactiveChange(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	create := map[string]any{"restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 4}
	raw, _ := json.Marshal(create)
	res := s.CreateReservation(policyLogin(t, s, "bea@example.com", "correct horse bea"), "retro-01", raw)
	if res.Status != 201 {
		t.Fatalf("create: %d %v", res.Status, res.Body)
	}
	before := marshalBody(t, res)
	ref := res.Body.(map[string]any)["reference"].(string)
	if res := s.PublishPolicy(mgr, "r_anker", "retro-pol", validPolicyBody("2027-06-01")); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	got := s.GetReservation(policyLogin(t, s, "bea@example.com", "correct horse bea"), ref)
	if got.Status != 200 || marshalBody(t, got) != before {
		t.Fatalf("booking mutated by publication:\nbefore %s\nafter  %s", before, marshalBody(t, got))
	}
	s.mu.Lock()
	histLen := len(s.state.Histories[ref])
	terms := s.state.Reservations[ref].AcceptedTerms
	s.mu.Unlock()
	if histLen != 1 {
		t.Fatalf("history length after publication = %d, want 1", histLen)
	}
	if terms.PolicyVersion != 0 || terms.ReservationDurationMinutes != 90 {
		t.Fatalf("accepted terms mutated: %+v", terms)
	}
}

func TestPolicyReplayAndScope(t *testing.T) {
	s, mgr, diner := newPolicyService(t)
	body := validPolicyBody("2027-06-01")
	first := s.PublishPolicy(mgr, "r_anker", "replay-01", body)
	if first.Status != 201 {
		t.Fatalf("first publish: %d", first.Status)
	}
	original := marshalBody(t, first)
	replay := s.PublishPolicy(mgr, "r_anker", "replay-01", body)
	if replay.Status != 200 || marshalBody(t, replay) != original {
		t.Fatalf("replay = %d %s, want 200 %s", replay.Status, marshalBody(t, replay), original)
	}
	// Unknown fields are ignored by validation but bind receipt identity.
	withExtra, _ := json.Marshal(map[string]any{
		"effective_from": "2027-06-01", "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 4, "t_2": 6, "t_3": 4}, "zzz": 1,
	})
	if status, code := resultCode(s.PublishPolicy(mgr, "r_anker", "replay-01", withExtra)); status != 409 || code != "idempotency_key_reuse" {
		t.Fatalf("different body on used key = %d %s", status, code)
	}
	// Same key string on another restaurant path is independent.
	baarBody, _ := json.Marshal(map[string]any{
		"effective_from": "2027-06-01", "slot_minutes": 30, "reservation_duration_minutes": 90,
		"cancellation_cutoff_minutes": 120,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_9": 6},
	})
	if res := s.PublishPolicy(mgr, "r_baar", "replay-01", baarBody); res.Status != 201 {
		t.Fatalf("same key other path: %d %v", res.Status, res.Body)
	}
	// Same key string by another user does not hit the first receipt.
	if status, code := resultCode(s.PublishPolicy(diner, "r_anker", "replay-01", body)); status != 403 || code != "forbidden" {
		t.Fatalf("same key other user = %d %s", status, code)
	}
	// A key from a failed 4xx is reusable.
	bad, _ := json.Marshal(map[string]any{"slot_minutes": 30})
	if status, _ := resultCode(s.PublishPolicy(mgr, "r_anker", "reuse-01", bad)); status != 422 {
		t.Fatalf("bad publish = %d", status)
	}
	if res := s.PublishPolicy(mgr, "r_anker", "reuse-01", body); res.Status != 201 {
		t.Fatalf("reused key: %d %v", res.Status, res.Body)
	}
	if replay := s.PublishPolicy(mgr, "r_anker", "reuse-01", body); replay.Status != 200 || marshalBody(t, replay) != marshalBody(t, s.PublishPolicy(mgr, "r_anker", "reuse-01", body)) {
		t.Fatalf("replay after reuse mismatch")
	}
}

func TestPolicyConcurrentPublish(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	body := validPolicyBody("2027-06-01")
	const n = 50
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			statuses[idx] = s.PublishPolicy(mgr, "r_anker", "race-01", body).Status
		}(i)
	}
	wg.Wait()
	created, replayed := 0, 0
	for _, st := range statuses {
		switch st {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Fatalf("concurrent status = %d", st)
		}
	}
	if created != 1 || replayed != n-1 {
		t.Fatalf("concurrent publish: %d x201 + %d x200, want 1+49", created, replayed)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Policies["r_anker"]) != 1 {
		t.Fatalf("versions = %d, want 1", len(s.state.Policies["r_anker"]))
	}
	if s.state.RestaurantRevisions["r_anker"] != 1 {
		t.Fatalf("counter = %d, want 1", s.state.RestaurantRevisions["r_anker"])
	}
	count := 0
	for rk := range s.state.Receipts {
		if len(rk) > 0 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("receipts = %d, want 1", count)
	}
}

func TestPolicyListAndDetail(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	if body := marshalBody(t, s.ListPolicies("r_anker")); body != `{"policies":[]}` {
		t.Fatalf("empty list = %s", body)
	}
	before := marshalBody(t, s.GetRestaurant("r_anker"))
	if res := s.PublishPolicy(mgr, "r_anker", "list-01", validPolicyBody("2027-06-01")); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	after := marshalBody(t, s.GetRestaurant("r_anker"))
	if before != after {
		t.Fatalf("detail changed by publication:\n%s\n%s", before, after)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(after), &detail); err != nil {
		t.Fatal(err)
	}
	if _, ok := detail["manager_user_ids"]; ok {
		t.Fatalf("detail exposes managers: %s", after)
	}
	if detail["slot_minutes"] != float64(30) || detail["reservation_duration_minutes"] != float64(90) {
		t.Fatalf("detail mutated: %s", after)
	}
	listed := marshalBody(t, s.ListPolicies("r_anker"))
	if res := s.PublishPolicy(mgr, "r_anker", "list-02", validPolicyBody("2027-07-01")); res.Status != 201 {
		t.Fatalf("publish 2: %d", res.Status)
	}
	if again := marshalBody(t, s.ListPolicies("r_anker")); again == listed {
		t.Fatalf("second publication missing from list")
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(marshalBody(t, s.ListPolicies("r_anker"))), &v); err != nil {
		t.Fatal(err)
	}
	pols := v["policies"].([]any)
	if len(pols) != 2 || pols[0].(map[string]any)["policy_version"] != float64(1) || pols[1].(map[string]any)["policy_version"] != float64(2) {
		t.Fatalf("list order = %v", pols)
	}
	if _, ok := pols[0].(map[string]any)["effective_from"]; !ok {
		t.Fatalf("list entry missing effective_from: %v", pols[0])
	}
}
