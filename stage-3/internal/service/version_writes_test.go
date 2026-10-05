package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func utcToday() string {
	switch time.Now().UTC().Weekday() {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	default:
		return "sun"
	}
}

func utcPlus(minutes int) string {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute).Format("2006-01-02T15:04")
}

func writeFixture() string {
	return policyFixture
}

func newWriteService(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(writeFixture())); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	return s, mgr, diner
}

func createRef(t *testing.T, s *Service, token, key string, body map[string]any) (string, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	res := s.CreateReservation(token, key, raw)
	if res.Status != 201 {
		t.Fatalf("create: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	ref, _ := v["reference"].(string)
	if ref == "" {
		t.Fatalf("create returned no reference: %v", v)
	}
	return ref, v
}

func patchBody(fields map[string]any) []byte {
	raw, _ := json.Marshal(fields)
	return raw
}

func counterOf(s *Service, restaurant string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.RestaurantRevisions[restaurant]
}

func historyLen(s *Service, ref string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.state.Histories[ref])
}

func TestPolicyWriteCreateDatedTerms(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	if res := s.PublishPolicy(mgr, "r_anker", "w-01", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	ref, v := createRef(t, s, diner, "w-create-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	if v["revision"] != 1 {
		t.Fatalf("create revision = %v", v["revision"])
	}
	terms, ok := v["accepted_terms"].(map[string]any)
	if !ok {
		t.Fatalf("create has no accepted_terms: %v", v)
	}
	if pv, ok := terms["policy_version"].(int); !ok || pv != 1 {
		t.Fatalf("create did not adopt selected terms: %v", terms)
	}
	if dm, ok := terms["reservation_duration_minutes"].(int); !ok || dm != 60 {
		t.Fatalf("create duration not adopted: %v", terms)
	}
	if _, hasEff := terms["effective_from"]; hasEff {
		t.Fatalf("accepted_terms carries effective_from: %v", terms)
	}
	if v["ends_at"] != date+"T20:00:00+02:00" {
		t.Fatalf("end uses selected duration: %v", v["ends_at"])
	}
	if counterOf(s, "r_anker") != 2 {
		t.Fatalf("counter after publish+create = %d", counterOf(s, "r_anker"))
	}
	if historyLen(s, ref) != 1 {
		t.Fatalf("history after create = %d", historyLen(s, ref))
	}
	// A pre-policy date still creates under policy0 with the 90-minute end.
	_, v0 := createRef(t, s, diner, "w-create-02", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": "2027-06-10T19:00", "party_size": 4,
	})
	t0 := v0["accepted_terms"].(map[string]any)
	if pv, ok := t0["policy_version"].(int); !ok || pv != 0 {
		t.Fatalf("pre-policy create = %v", v0)
	}
	if v0["ends_at"] != "2027-06-10T20:30:00+02:00" {
		t.Fatalf("pre-policy end = %v", v0)
	}
}

func utcGridPlus(minutes int) string {
	now := time.Now().UTC()
	floor := now.Truncate(30 * time.Minute)
	return floor.Add(time.Duration(minutes) * time.Minute).Format("2006-01-02T15:04")
}

func TestPolicyWriteOldVsNewCutoff(t *testing.T) {
	// All-week hours keep every computed wall time inside opening hours;
	// seeds bypass grid rules while merged PATCH validation does not.
	allHours := `[{"weekday":"mon","opens":"00:00","closes":"23:59"},{"weekday":"tue","opens":"00:00","closes":"23:59"},{"weekday":"wed","opens":"00:00","closes":"23:59"},{"weekday":"thu","opens":"00:00","closes":"23:59"},{"weekday":"fri","opens":"00:00","closes":"23:59"},{"weekday":"sat","opens":"00:00","closes":"23:59"},{"weekday":"sun","opens":"00:00","closes":"23:59"}]`
	build := func(cutoff int) string {
		return fmt.Sprintf(`{
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_utc", "name": "UTC", "timezone": "UTC",
    "slot_minutes": 30, "reservation_duration_minutes": 30, "cancellation_cutoff_minutes": %d,
    "opening_hours": %s,
    "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
    "manager_user_ids": ["u_ada"]
  }],
  "reservations": [
    {"id": "seed-near", "reference": "NEARCUT", "user_id": "u_ada",
     "restaurant_id": "r_utc", "table_id": "t_1",
     "starts_at_local": %q, "party_size": 2}
  ]
}`, cutoff, allHours, utcPlus(75))
	}
	publishCutoff := func(t *testing.T, s *Service, mgr string, cutoff int) {
		t.Helper()
		pol, _ := json.Marshal(map[string]any{
			"effective_from": "2020-01-01", "slot_minutes": 30, "reservation_duration_minutes": 30,
			"cancellation_cutoff_minutes": cutoff,
			"opening_hours": []any{
				map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:59"},
			},
			"capacities": map[string]any{"t_1": 4},
		})
		if res := s.PublishPolicy(mgr, "r_utc", "cut-pol", pol); res.Status != 201 {
			t.Fatalf("publish: %d", res.Status)
		}
	}
	t.Run("old large cutoff still blocks", func(t *testing.T) {
		s := New()
		if res := s.Reset([]byte(build(120))); res.Status != 204 {
			t.Fatalf("reset: %d %v", res.Status, res.Body)
		}
		mgr := policyLogin(t, s, "ada@example.com", "correct horse")
		publishCutoff(t, s, mgr, 30)
		res := s.PatchReservation(mgr, "NEARCUT", patchBody(map[string]any{"party_size": 3}))
		if status, code := resultCode(res); status != 409 || code != "cutoff_passed" {
			t.Fatalf("amend under old cutoff = %d %s (new cutoff 30 would allow)", status, code)
		}
	})
	t.Run("old zero cutoff still allows", func(t *testing.T) {
		s := New()
		if res := s.Reset([]byte(build(0))); res.Status != 204 {
			t.Fatalf("reset: %d %v", res.Status, res.Body)
		}
		mgr := policyLogin(t, s, "ada@example.com", "correct horse")
		publishCutoff(t, s, mgr, 10080)
		// Move to a grid-aligned future slot: fully merged validation must
		// pass under the new rules while the old zero cutoff allows it.
		res := s.PatchReservation(mgr, "NEARCUT", patchBody(map[string]any{"starts_at_local": utcGridPlus(120)}))
		if res.Status != 200 {
			t.Fatalf("amend under old cutoff = %d %v (new cutoff 10080 would block)", res.Status, res.Body)
		}
	})
}
func TestPolicyWriteFullyMerged(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "m-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:30", "party_size": 4,
	})
	// New 60-minute grid: the retained 19:30 start is off-grid, so even a
	// party-only change must fail with the merged time validated.
	if res := s.PublishPolicy(mgr, "r_anker", "m-pol", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	res := s.PatchReservation(diner, ref, patchBody(map[string]any{"party_size": 3}))
	if status, code := resultCode(res); status != 422 || code != "not_on_slot_grid" {
		t.Fatalf("merged validation = %d %s", status, code)
	}
	// The failed write changed nothing.
	got := s.GetReservation(diner, ref)
	if got.Body.(map[string]any)["party_size"] != 4 {
		t.Fatalf("failed amend mutated record: %v", got.Body)
	}
	if historyLen(s, ref) != 1 || counterOf(s, "r_anker") != 2 {
		t.Fatalf("failed amend leaked metadata")
	}
}

func TestPolicyWriteDateCrossing(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	if res := s.PublishPolicy(mgr, "r_anker", "x-01", validPolicyBody("2027-06-17")); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	ref, _ := createRef(t, s, diner, "x-create", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": "2027-06-10T19:00", "party_size": 4,
	})
	// Move across the policy boundary: new terms, new 60-minute end,
	// revision 2, one changed history entry carrying the new terms.
	res := s.PatchReservation(diner, ref, patchBody(map[string]any{"starts_at_local": "2027-06-17T19:00"}))
	if res.Status != 200 {
		t.Fatalf("cross-date amend: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	if v["revision"] != 2 || v["ends_at"] != "2027-06-17T20:00:00+02:00" {
		t.Fatalf("cross-date result = %v", v)
	}
	terms := v["accepted_terms"].(map[string]any)
	if pv, ok := terms["policy_version"].(int); !ok || pv != 1 {
		t.Fatalf("terms not adopted: %v", terms)
	}
	if dm, ok := terms["reservation_duration_minutes"].(int); !ok || dm != 60 {
		t.Fatalf("duration not adopted: %v", terms)
	}
	if historyLen(s, ref) != 2 {
		t.Fatalf("history = %d, want 2", historyLen(s, ref))
	}
	hres := s.ReservationHistory(diner, ref)
	entries := historyEntries(t, hres)
	if len(entries) != 2 {
		t.Fatalf("history entries = %v", hres.Body)
	}
	second := entries[1].(map[string]any)
	if second["event"] != "changed" || second["revision"] != 2 {
		t.Fatalf("second entry = %v", second)
	}
	if second["accepted_terms"].(map[string]any)["policy_version"] != float64(1) {
		t.Fatalf("changed entry terms = %v", second)
	}
	first := entries[0].(map[string]any)
	if first["accepted_terms"].(map[string]any)["policy_version"] != float64(0) {
		t.Fatalf("old entry acquired new terms: %v", first)
	}
}

func TestPolicyWriteSameDateSupersession(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	if res := s.PublishPolicy(mgr, "r_anker", "ss-01", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish 1: %d", res.Status)
	}
	other, _ := json.Marshal(map[string]any{
		"effective_from": date, "slot_minutes": 30, "reservation_duration_minutes": 120,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 2, "t_3": 2},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "ss-02", other); res.Status != 201 {
		t.Fatalf("publish 2: %d", res.Status)
	}
	_, v := createRef(t, s, diner, "ss-create", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 2,
	})
	terms := v["accepted_terms"].(map[string]any)
	if pv, ok := terms["policy_version"].(int); !ok || pv != 2 {
		t.Fatalf("same-date supersession not selected: %v", terms)
	}
	if dm, ok := terms["reservation_duration_minutes"].(int); !ok || dm != 120 {
		t.Fatalf("superseded duration not selected: %v", terms)
	}
}

func TestRevisionNoOpRetention(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "noop-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	before := marshalBody(t, s.GetReservation(diner, ref))
	if res := s.PublishPolicy(mgr, "r_anker", "noop-pol", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	names := []string{"empty", "unknown-only", "equal-fields", "reversed-legacy"}
	bodies := []map[string]any{
		{},
		{"zzz": 1},
		{"table_id": "t_2", "starts_at_local": date + "T19:00", "party_size": 4},
		{"table_id": "t_2"},
	}
	for i, body := range bodies {
		res := s.PatchReservation(diner, ref, patchBody(body))
		if res.Status != 200 || marshalBody(t, res) != before {
			t.Fatalf("no-op %s: %d %s, want 200 %s", names[i], res.Status, marshalBody(t, res), before)
		}
	}
	// A no-op on a pair via reversed input retains everything too.
	pref, _ := createRef(t, s, diner, "noop-02", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 6,
	})
	pbefore := marshalBody(t, s.GetReservation(diner, pref))
	rev, _ := json.Marshal(map[string]any{"table_ids": []string{"t_2", "t_1"}})
	if res := s.PatchReservation(diner, pref, rev); res.Status != 200 || marshalBody(t, res) != pbefore {
		t.Fatalf("reversed pair no-op: %d %s", res.Status, marshalBody(t, res))
	}
	if historyLen(s, ref) != 1 || historyLen(s, pref) != 1 {
		t.Fatalf("no-ops recorded history")
	}
	if counterOf(s, "r_anker") != 3 {
		t.Fatalf("counter = %d, want 3 (2 creates + 1 publication)", counterOf(s, "r_anker"))
	}
}

func TestRevisionExpectedPrecedence(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "exp-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	for _, bad := range []any{"2", true, 1.5, 0.0, -1.0, nil} {
		body := map[string]any{"party_size": 3}
		if bad != nil {
			body["expected_revision"] = bad
		} else {
			body["expected_revision"] = nil
		}
		res := s.PatchReservation(diner, ref, patchBody(body))
		if status, code := resultCode(res); status != 422 || code != "validation_failed" {
			t.Fatalf("expected_revision %v: got %d %s", bad, status, code)
		}
	}
	// Stale revision precedes cutoff and field validation: use a past
	// booking (cutoff long passed) with a wrong revision and a bad table.
	pastRef, _ := createRef(t, s, diner, "exp-02", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": "2020-01-02T19:00", "party_size": 2,
	})
	res := s.PatchReservation(diner, pastRef, patchBody(map[string]any{"expected_revision": 99, "table_id": "t_nope"}))
	if status, code := resultCode(res); status != 409 || code != "stale_revision" {
		t.Fatalf("stale precedence = %d %s", status, code)
	}
	// Matching revision succeeds.
	res = s.PatchReservation(diner, ref, patchBody(map[string]any{"expected_revision": 1, "party_size": 3}))
	if res.Status != 200 || res.Body.(map[string]any)["revision"] != 2 {
		t.Fatalf("matching revision: %d %v", res.Status, res.Body)
	}
	// The old revision is now stale.
	res = s.PatchReservation(diner, ref, patchBody(map[string]any{"expected_revision": 1, "party_size": 2}))
	if status, code := resultCode(res); status != 409 || code != "stale_revision" {
		t.Fatalf("used revision = %d %s", status, code)
	}
}

func TestRevisionConcurrentAmendments(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "race-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	var wg sync.WaitGroup
	results := make([]Result, 2)
	parties := []int{2, 3}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = s.PatchReservation(diner, ref, patchBody(map[string]any{
				"expected_revision": 1, "party_size": parties[idx],
			}))
		}(i)
	}
	wg.Wait()
	ok, stale := 0, 0
	for _, res := range results {
		switch status, code := resultCode(res); {
		case status == 200:
			ok++
		case status == 409 && code == "stale_revision":
			stale++
		default:
			t.Fatalf("concurrent amend = %d %s", status, code)
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("one expected revision gave %d successes + %d stale", ok, stale)
	}
	final := s.GetReservation(diner, ref).Body.(map[string]any)
	if final["revision"] != 2 || (final["party_size"] != 2 && final["party_size"] != 3) {
		t.Fatalf("final = %v", final)
	}
	if historyLen(s, ref) != 2 || counterOf(s, "r_anker") != 2 {
		t.Fatalf("history/counter wrong after race")
	}
}

func TestRevisionFailuresAddNothing(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "fail-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": 2,
	})
	// Occupancy failure.
	if res := s.PatchReservation(diner, ref, patchBody(map[string]any{"table_id": "t_1", "starts_at_local": date + "T19:00"})); res.Status != 200 {
		t.Fatalf("setup sanity: %d", res.Status)
	}
	other, _ := createRef(t, s, diner, "fail-02", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 2,
	})
	beforeOther := marshalBody(t, s.GetReservation(diner, other))
	res := s.PatchReservation(diner, other, patchBody(map[string]any{"table_id": "t_1"}))
	if status, code := resultCode(res); status != 409 || code != "table_unavailable" {
		t.Fatalf("overlap amend = %d %s", status, code)
	}
	if marshalBody(t, s.GetReservation(diner, other)) != beforeOther {
		t.Fatalf("failed amend mutated record")
	}
	// Validation failure.
	res = s.PatchReservation(diner, other, patchBody(map[string]any{"party_size": 99}))
	if status, _ := resultCode(res); status != 422 {
		t.Fatalf("over-capacity amend = %d", status)
	}
	// Failed create claims nothing.
	bad, _ := json.Marshal(map[string]any{"restaurant_id": "r_anker", "table_id": "t_1", "starts_at_local": date + "T19:00", "party_size": 2})
	if res := s.CreateReservation(diner, "fail-create", bad); res.Status != 409 {
		t.Fatalf("conflicting create = %d", res.Status)
	}
	if historyLen(s, other) != 1 || counterOf(s, "r_anker") != 2 {
		t.Fatalf("failures leaked metadata: history %d counter %d", historyLen(s, other), counterOf(s, "r_anker"))
	}
	// Create replay adds no history or counter.
	again, _ := json.Marshal(map[string]any{"restaurant_id": "r_anker", "table_id": "t_3", "starts_at_local": date + "T19:00", "party_size": 2})
	first := s.CreateReservation(diner, "fail-replay", again)
	replay := s.CreateReservation(diner, "fail-replay", again)
	if first.Status != 201 || replay.Status != 200 {
		t.Fatalf("replay statuses %d/%d", first.Status, replay.Status)
	}
	rref := first.Body.(map[string]any)["reference"].(string)
	if historyLen(s, rref) != 1 || counterOf(s, "r_anker") != 3 {
		t.Fatalf("replay leaked metadata")
	}
}

func TestRevisionExpectedLargeIntegers(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "bigexp-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	before := marshalBody(t, s.GetReservation(diner, ref))
	counterBefore := counterOf(s, "r_anker")
	// 9000000000000001 is a positive integer (decodes to integral float64);
	// it must be 409 stale_revision, not 422.
	raw := []byte(`{"expected_revision":9000000000000001,"party_size":3}`)
	res := s.PatchReservation(diner, ref, raw)
	if status, code := resultCode(res); status != 409 || code != "stale_revision" {
		t.Fatalf("huge revision = %d %s", status, code)
	}
	// Stale precedes field validation: unknown table would be 404 alone.
	raw = []byte(`{"expected_revision":9000000000000001,"table_id":"t_nope"}`)
	res = s.PatchReservation(diner, ref, raw)
	if status, code := resultCode(res); status != 409 || code != "stale_revision" {
		t.Fatalf("huge revision before fields = %d %s", status, code)
	}
	// A larger decoded integral value behaves the same.
	res = s.PatchReservation(diner, ref, patchBody(map[string]any{"expected_revision": 1e21, "party_size": 3}))
	if status, code := resultCode(res); status != 409 || code != "stale_revision" {
		t.Fatalf("1e21 revision = %d %s", status, code)
	}
	// The rejected attempts changed nothing.
	if marshalBody(t, s.GetReservation(diner, ref)) != before {
		t.Fatalf("rejected stale attempts mutated the record")
	}
	if counterOf(s, "r_anker") != counterBefore || historyLen(s, ref) != 1 {
		t.Fatalf("rejected stale attempts leaked metadata")
	}
	// Exact current revision still succeeds.
	res = s.PatchReservation(diner, ref, patchBody(map[string]any{"expected_revision": 1.0, "party_size": 3}))
	if res.Status != 200 {
		t.Fatalf("matching revision: %d %v", res.Status, res.Body)
	}
}

func TestRevisionRepeatCancelUnchanged(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "rc-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	first := s.CancelReservation(diner, ref)
	if first.Status != 200 {
		t.Fatalf("cancel: %d", first.Status)
	}
	firstBody := marshalBody(t, first)
	firstHist := historyLen(s, ref)
	firstCounter := counterOf(s, "r_anker")
	firstExport, _ := json.Marshal(s.Export().Body)
	second := s.CancelReservation(diner, ref)
	if second.Status != 200 || marshalBody(t, second) != firstBody {
		t.Fatalf("repeat cancel: %d %s, want 200 %s", second.Status, marshalBody(t, second), firstBody)
	}
	if historyLen(s, ref) != firstHist {
		t.Fatalf("repeat cancel added history")
	}
	if counterOf(s, "r_anker") != firstCounter {
		t.Fatalf("repeat cancel incremented counter")
	}
	secondExport, _ := json.Marshal(s.Export().Body)
	if string(secondExport) != string(firstExport) {
		t.Fatalf("repeat cancel changed exported state")
	}
	rev := s.GetReservation(diner, ref).Body.(map[string]any)["revision"]
	if rev != 2 {
		t.Fatalf("revision after repeat cancel = %v", rev)
	}
}

func TestRevisionCancelAcceptedCutoff(t *testing.T) {
	allHours := `[{"weekday":"mon","opens":"00:00","closes":"23:59"},{"weekday":"tue","opens":"00:00","closes":"23:59"},{"weekday":"wed","opens":"00:00","closes":"23:59"},{"weekday":"thu","opens":"00:00","closes":"23:59"},{"weekday":"fri","opens":"00:00","closes":"23:59"},{"weekday":"sat","opens":"00:00","closes":"23:59"},{"weekday":"sun","opens":"00:00","closes":"23:59"}]`
	build := func(cutoff int) string {
		return fmt.Sprintf(`{
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_utc", "name": "UTC", "timezone": "UTC",
    "slot_minutes": 30, "reservation_duration_minutes": 30, "cancellation_cutoff_minutes": %d,
    "opening_hours": %s,
    "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
    "manager_user_ids": ["u_ada"]
  }],
  "reservations": [
    {"id": "seed-near", "reference": "NEARCUT", "user_id": "u_ada",
     "restaurant_id": "r_utc", "table_id": "t_1",
     "starts_at_local": %q, "party_size": 2}
  ]
}`, cutoff, allHours, utcPlus(75))
	}
	publishCutoff := func(t *testing.T, s *Service, mgr string, cutoff int) {
		t.Helper()
		pol, _ := json.Marshal(map[string]any{
			"effective_from": "2020-01-01", "slot_minutes": 30, "reservation_duration_minutes": 30,
			"cancellation_cutoff_minutes": cutoff,
			"opening_hours": []any{
				map[string]any{"weekday": "mon", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "tue", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "wed", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "thu", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "fri", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "sat", "opens": "00:00", "closes": "23:59"},
				map[string]any{"weekday": "sun", "opens": "00:00", "closes": "23:59"},
			},
			"capacities": map[string]any{"t_1": 4},
		})
		if res := s.PublishPolicy(mgr, "r_utc", "cut-pol", pol); res.Status != 201 {
			t.Fatalf("publish: %d", res.Status)
		}
	}
	t.Run("old large cutoff blocks cancel", func(t *testing.T) {
		s := New()
		if res := s.Reset([]byte(build(120))); res.Status != 204 {
			t.Fatalf("reset: %d %v", res.Status, res.Body)
		}
		mgr := policyLogin(t, s, "ada@example.com", "correct horse")
		publishCutoff(t, s, mgr, 30)
		res := s.CancelReservation(mgr, "NEARCUT")
		if status, code := resultCode(res); status != 409 || code != "cutoff_passed" {
			t.Fatalf("cancel under old cutoff = %d %s (new cutoff 30 would allow)", status, code)
		}
	})
	t.Run("old zero cutoff allows cancel", func(t *testing.T) {
		s := New()
		if res := s.Reset([]byte(build(0))); res.Status != 204 {
			t.Fatalf("reset: %d %v", res.Status, res.Body)
		}
		mgr := policyLogin(t, s, "ada@example.com", "correct horse")
		publishCutoff(t, s, mgr, 10080)
		res := s.CancelReservation(mgr, "NEARCUT")
		if res.Status != 200 {
			t.Fatalf("cancel under old cutoff = %d %v (new cutoff 10080 would block)", res.Status, res.Body)
		}
	})
}

func TestPolicyWriteSelectedPairCapacity(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	if res := s.PublishPolicy(mgr, "r_anker", "pc-01", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	// Selected pair capacity 4+6=10 permits party 8; the original fixture
	// sum 2+4=6 would reject it.
	ref, v := createRef(t, s, diner, "pc-create", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 8,
	})
	if v["revision"] != 1 {
		t.Fatalf("revision = %v", v["revision"])
	}
	terms := v["accepted_terms"].(map[string]any)
	if pv, ok := terms["policy_version"].(int); !ok || pv != 1 {
		t.Fatalf("terms = %v", terms)
	}
	if v["ends_at"] != date+"T22:00:00+02:00" {
		t.Fatalf("end = %v", v["ends_at"])
	}
	if historyLen(s, ref) != 1 || counterOf(s, "r_anker") != 2 {
		t.Fatalf("create metadata wrong")
	}
	// A superseding lower-capacity policy rejects what the fixture sum and
	// the old policy would both allow.
	lower, _ := json.Marshal(map[string]any{
		"effective_from": date, "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 1, "t_2": 1, "t_3": 1},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "pc-02", lower); res.Status != 201 {
		t.Fatalf("publish 2: %d", res.Status)
	}
	counterBefore := counterOf(s, "r_anker")
	bad, _ := json.Marshal(map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T18:00", "party_size": 3,
	})
	if res := s.CreateReservation(diner, "pc-reject", bad); res.Status != 422 {
		t.Fatalf("selected-capacity rejection = %d %v", res.Status, res.Body)
	} else if code := res.Body.(map[string]any)["error"].(map[string]any)["code"]; code != "party_exceeds_capacity" {
		t.Fatalf("rejection code = %v", code)
	}
	if counterOf(s, "r_anker") != counterBefore {
		t.Fatalf("rejected create incremented counter")
	}
}
