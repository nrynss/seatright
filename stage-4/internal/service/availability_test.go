package service

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func slotsOf(t *testing.T, res Result) []any {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("availability: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	if v["restaurant_id"] != "r_anker" && v["restaurant_id"] != "r_sun" {
		t.Fatalf("envelope = %v", res.Body)
	}
	slots, ok := v["slots"].([]any)
	if !ok || slots == nil {
		t.Fatalf("slots not an array: %v", res.Body)
	}
	for _, sl := range slots {
		m := sl.(map[string]any)
		checkTimestampNumeric(t, "slot starts_at", m["starts_at"].(string))
		if m["available_table_ids"] == nil {
			t.Fatalf("available_table_ids is null: %v", m)
		}
	}
	return slots
}

func tablesOf(slot any) []string {
	out := []string{}
	for _, id := range slot.(map[string]any)["available_table_ids"].([]any) {
		out = append(out, id.(string))
	}
	return out
}

func TestAvailabilityGrid(t *testing.T) {
	s := newFoundation(t)
	slots := slotsOf(t, s.Availability(queryOf("r_anker", "2026-09-24", "2")))
	if len(slots) != 8 {
		t.Fatalf("got %d slots, want 8 (18:00..21:30)", len(slots))
	}
	first := slots[0].(map[string]any)
	if first["starts_at_local"] != "2026-09-24T18:00" || first["starts_at"] != "2026-09-24T18:00:00+02:00" {
		t.Fatalf("first slot = %v", first)
	}
	if got := tablesOf(slots[0]); len(got) != 2 || got[0] != "t_1" || got[1] != "t_2" {
		t.Fatalf("first slot tables = %v", got)
	}
	last := slots[len(slots)-1].(map[string]any)
	if last["starts_at_local"] != "2026-09-24T21:30" {
		t.Fatalf("last slot = %v", last)
	}
	// Party size filters by capacity, keeping fixture order.
	slots = slotsOf(t, s.Availability(queryOf("r_anker", "2026-09-24", "4")))
	for _, sl := range slots {
		if got := tablesOf(sl); len(got) != 1 || got[0] != "t_2" {
			t.Fatalf("party 4 tables = %v", got)
		}
	}
	// Unknown query parameters are ignored, and the endpoint is public.
	rec := serveRequest(s, http.MethodGet, "/availability?restaurant_id=r_anker&date=2026-09-24&party_size=2&foo=bar", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("public availability with extra param: %d", rec.Code)
	}
}

func TestAvailabilityParams(t *testing.T) {
	s := newFoundation(t)
	cases := []struct {
		name   string
		query  url.Values
		status int
		code   string
	}{
		{"missing restaurant", queryOf("", "2026-09-24", "2"), 422, "validation_failed"},
		{"missing date", queryOf("r_anker", "", "2"), 422, "validation_failed"},
		{"missing party", queryOf("r_anker", "2026-09-24", ""), 422, "validation_failed"},
		{"party exponent", queryOf("r_anker", "2026-09-24", "1e9"), 422, "validation_failed"},
		{"party float", queryOf("r_anker", "2026-09-24", "4.0"), 422, "validation_failed"},
		{"party signed", queryOf("r_anker", "2026-09-24", "+4"), 422, "validation_failed"},
		{"party zero", queryOf("r_anker", "2026-09-24", "0"), 422, "validation_failed"},
		{"party negative", queryOf("r_anker", "2026-09-24", "-2"), 422, "validation_failed"},
		{"party blank", queryOf("r_anker", "2026-09-24", " "), 422, "validation_failed"},
		{"bad date", queryOf("r_anker", "2026-13-40", "2"), 422, "validation_failed"},
		{"date with time", queryOf("r_anker", "2026-09-24T19:00", "2"), 422, "validation_failed"},
		{"short date", queryOf("r_anker", "09-24", "2"), 422, "validation_failed"},
		{"unknown restaurant", queryOf("r_nope", "2026-09-24", "2"), 404, "not_found"},
	}
	for _, tc := range cases {
		res := s.Availability(tc.query)
		v, ok := res.Body.(map[string]any)
		if !ok || res.Status != tc.status {
			t.Errorf("%s: got %d %v", tc.name, res.Status, res.Body)
			continue
		}
		if code := v["error"].(map[string]any)["code"]; code != tc.code {
			t.Errorf("%s: got code %v", tc.name, code)
		}
	}
	// Closed day returns an empty slots array.
	res := s.Availability(queryOf("r_anker", "2026-09-23", "2"))
	if res.Status != 200 {
		t.Fatalf("closed day: %d", res.Status)
	}
	if slots := res.Body.(map[string]any)["slots"].([]any); len(slots) != 0 {
		t.Fatalf("closed day slots = %v", slots)
	}
}

func TestAvailabilityOccupancy(t *testing.T) {
	s := New()
	resetWith(t, s, seedAda, seedAnkerTables,
		seedRes("s1", "SEED01", "u_ada", "r_anker", "t_2", "2026-09-24T19:00", 4, "confirmed")+","+
			seedRes("s2", "SEED02", "u_ada", "r_anker", "t_1", "2026-09-24T21:00", 2, "confirmed")+","+
			seedRes("s3", "SEED03", "u_ada", "r_anker", "t_1", "2026-09-24T18:00", 2, "cancelled"))
	slots := slotsOf(t, s.Availability(queryOf("r_anker", "2026-09-24", "2")))
	byLocal := map[string][]string{}
	for _, sl := range slots {
		m := sl.(map[string]any)
		byLocal[m["starts_at_local"].(string)] = tablesOf(sl)
	}
	equalTables := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	// t_1 at 18:00 is cancelled (ignored) and 21:00 does not reach back, so
	// t_1 is free; t_2 at 19:00 overlaps the slot.
	if got := byLocal["2026-09-24T18:00"]; !equalTables(got, []string{"t_1"}) {
		t.Fatalf("18:00 tables = %v", got)
	}
	if got := byLocal["2026-09-24T18:30"]; !equalTables(got, []string{"t_1"}) {
		t.Fatalf("18:30 tables = %v", got)
	}
	if got := byLocal["2026-09-24T19:00"]; !equalTables(got, []string{"t_1"}) {
		t.Fatalf("19:00 tables = %v", got)
	}
	if got := byLocal["2026-09-24T19:30"]; !equalTables(got, []string{"t_1"}) {
		t.Fatalf("19:30 tables = %v", got)
	}
	// Both confirmed bookings overlap this slot: present but empty.
	if got, present := byLocal["2026-09-24T20:00"]; !present || len(got) != 0 {
		t.Fatalf("20:00 tables = %v, present=%v", got, present)
	}
	if got := byLocal["2026-09-24T20:30"]; !equalTables(got, []string{"t_2"}) {
		t.Fatalf("20:30 tables = %v, want t_2 adjacent to both", got)
	}
}

func TestAvailabilityDST(t *testing.T) {
	fix := `{"id":"r_sun","name":"Sun","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,
		"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"04:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}]}`
	s := New()
	resetWith(t, s, seedAda, fix, "")
	// Spring forward: skipped walls never appear.
	slots := slotsOf(t, s.Availability(queryOf("r_sun", "2026-03-29", "2")))
	locals := []string{}
	for _, sl := range slots {
		locals = append(locals, sl.(map[string]any)["starts_at_local"].(string))
	}
	for _, want := range []string{"2026-03-29T00:00", "2026-03-29T00:30", "2026-03-29T01:00", "2026-03-29T01:30", "2026-03-29T03:00"} {
		found := false
		for _, l := range locals {
			if l == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing slot %s in %v", want, locals)
		}
	}
	for _, l := range locals {
		if strings.HasPrefix(l, "2026-03-29T02:") {
			t.Fatalf("skipped wall appears: %s", l)
		}
	}
	// Fall back: the repeated wall appears exactly once.
	slots = slotsOf(t, s.Availability(queryOf("r_sun", "2026-10-25", "2")))
	count := 0
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] == "2026-10-25T02:30" {
			count++
			if sl.(map[string]any)["starts_at"] != "2026-10-25T02:30:00+02:00" {
				t.Fatalf("repeated wall must resolve first: %v", sl)
			}
		}
	}
	if count != 1 {
		t.Fatalf("repeated wall appears %d times", count)
	}
}

func slotsAny(t *testing.T, res Result) []any {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("availability: %d %v", res.Status, res.Body)
	}
	slots, ok := res.Body.(map[string]any)["slots"].([]any)
	if !ok || slots == nil {
		t.Fatalf("slots not an array: %v", res.Body)
	}
	return slots
}

func stringList(t *testing.T, v any) []string {
	t.Helper()
	switch items := v.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			str, ok := item.(string)
			if !ok {
				t.Fatalf("not a string list: %#v", v)
			}
			out = append(out, str)
		}
		return out
	default:
		t.Fatalf("not a string list: %#v", v)
		return nil
	}
}

func queryExplain(restaurant, date, party, explain string) url.Values {
	q := url.Values{
		"restaurant_id": {restaurant},
		"date":          {date},
		"party_size":    {party},
	}
	if explain != "" {
		q.Set("explain", explain)
	}
	return q
}

func explainOf(t *testing.T, slot any) []any {
	t.Helper()
	m := slot.(map[string]any)
	ex, ok := m["explain"].([]any)
	if !ok || ex == nil {
		t.Fatalf("slot has no explain array: %v", m)
	}
	return ex
}

func createBooking(t *testing.T, s *Service, token, key, restaurant, table, local string, party int) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"restaurant_id": restaurant, "table_id": table,
		"starts_at_local": local, "party_size": party,
	})
	res := s.CreateReservation(token, key, body)
	if res.Status != 201 {
		t.Fatalf("create %s %s: %d %v", table, local, res.Status, res.Body)
	}
	return res.Body.(map[string]any)["reference"].(string)
}

func TestAvailabilityExplainMatrix(t *testing.T) {
	s, mgr, diner := newPolicyService(t)
	_ = mgr
	date := "2027-06-17"
	// t_1 (cap 2) booked at 21:00; t_2 (cap 4) booked at 19:00.
	createBooking(t, s, diner, "ex-a", "r_anker", "t_1", date+"T21:00", 2)
	createBooking(t, s, diner, "ex-b", "r_anker", "t_2", date+"T19:00", 4)
	slots := slotsOf(t, s.Availability(queryExplain("r_anker", date, "4", "true")))
	var target map[string]any
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] == date+"T19:00" {
			target = sl.(map[string]any)
		}
	}
	if target == nil {
		t.Fatal("19:00 slot missing")
	}
	ex := explainOf(t, target)
	if len(ex) != 3 {
		t.Fatalf("explain entries = %d, want 3", len(ex))
	}
	// Table order follows the fixture; rules order is capacity, no_overlap.
	wantOrder := []string{"t_1", "t_2", "t_3"}
	for i, id := range wantOrder {
		entry := ex[i].(map[string]any)
		if entry["table_id"] != id {
			t.Fatalf("explain order = %v", ex)
		}
		rules := entry["rules"].([]any)
		if len(rules) != 2 || rules[0].(map[string]any)["rule"] != "capacity" || rules[1].(map[string]any)["rule"] != "no_overlap" {
			t.Fatalf("rules order = %v", rules)
		}
		if pv, ok := entry["policy_version"].(int); !ok || pv != 0 {
			t.Fatalf("policy version = %v", entry["policy_version"])
		}
	}
	holds := func(i, j int) bool {
		return ex[i].(map[string]any)["rules"].([]any)[j].(map[string]any)["holds"].(bool)
	}
	avail := func(i int) bool { return ex[i].(map[string]any)["available"].(bool) }
	// t_1: capacity false (4 > 2), no overlap (booked at 21:00 only).
	if holds(0, 0) || !holds(0, 1) || avail(0) {
		t.Fatalf("t_1 explain wrong: %v", ex[0])
	}
	// t_2: capacity true, overlap false (booked).
	if !holds(1, 0) || holds(1, 1) || avail(1) {
		t.Fatalf("t_2 explain wrong: %v", ex[1])
	}
	// t_3: both true.
	if !holds(2, 0) || !holds(2, 1) || !avail(2) {
		t.Fatalf("t_3 explain wrong: %v", ex[2])
	}
	// available ids exactly equal the true explain ids, in order.
	ids := tablesOf(target)
	if len(ids) != 1 || ids[0] != "t_3" {
		t.Fatalf("available ids = %v", ids)
	}
	// Capacity-false / overlap-true case: t_1 at 18:00 is unbooked but too
	// small for party 4.
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] != date+"T18:00" {
			continue
		}
		first := explainOf(t, sl)[0].(map[string]any)
		rules := first["rules"].([]any)
		if rules[0].(map[string]any)["holds"].(bool) || !rules[1].(map[string]any)["holds"].(bool) || first["available"].(bool) {
			t.Fatalf("t_1 18:00 explain wrong: %v", first)
		}
	}
	// Both-false case: party 6 with t_1 booked at 21:00 (cap 2 < 6).
	slots = slotsOf(t, s.Availability(queryExplain("r_anker", date, "6", "true")))
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] == date+"T21:00" {
			target = sl.(map[string]any)
		}
	}
	ex = explainOf(t, target)
	first := ex[0].(map[string]any)
	rules := first["rules"].([]any)
	if rules[0].(map[string]any)["holds"].(bool) || rules[1].(map[string]any)["holds"].(bool) || first["available"].(bool) {
		t.Fatalf("both-false entry wrong: %v", first)
	}
	if ids := tablesOf(target); len(ids) != 0 {
		t.Fatalf("party 6 ids = %v", ids)
	}
}

func TestAvailabilityExplainGuards(t *testing.T) {
	s, _, _ := newPolicyService(t)
	for _, bad := range []string{"false", "1", ""} {
		q := queryExplain("r_anker", "2027-06-17", "2", "x")
		q.Set("explain", bad)
		res := s.Availability(q)
		if status, code := resultCode(res); status != 422 || code != "validation_failed" {
			t.Fatalf("explain=%q: got %d %s", bad, status, code)
		}
	}
	// Bare ?explain= (empty value) is rejected too.
	res := s.Availability(queryExplain("r_anker", "2027-06-17", "2", ""))
	_ = res
	// Without explain no slot carries explanation fields.
	slots := slotsOf(t, s.Availability(queryOf("r_anker", "2027-06-17", "2")))
	for _, sl := range slots {
		if _, ok := sl.(map[string]any)["explain"]; ok {
			t.Fatalf("no-explain slot carries explain: %v", sl)
		}
	}
	// Unknown restaurant stays 404 even with explain.
	if status, code := resultCode(s.Availability(queryExplain("r_nope", "2027-06-17", "2", "true"))); status != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant + explain = %d %s", status, code)
	}
}

func TestAvailabilityExplainOccupancy(t *testing.T) {
	s, _, diner := newPolicyService(t)
	date := "2027-06-17"
	// Cancelled bookings are ignored.
	ref := createBooking(t, s, diner, "occ-a", "r_anker", "t_1", date+"T19:00", 2)
	if res := s.CancelReservation(diner, ref); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	slots := slotsOf(t, s.Availability(queryExplain("r_anker", date, "2", "true")))
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] != date+"T19:00" {
			continue
		}
		first := explainOf(t, sl)[0].(map[string]any)
		rules := first["rules"].([]any)
		if !rules[1].(map[string]any)["holds"].(bool) || !first["available"].(bool) {
			t.Fatalf("cancelled booking still blocks: %v", first)
		}
	}
	// A pair booking blocks every member independently.
	pairBody, _ := json.Marshal(map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T20:30", "party_size": 6,
	})
	if res := s.CreateReservation(diner, "occ-pair", pairBody); res.Status != 201 {
		t.Fatalf("pair create: %d %v", res.Status, res.Body)
	}
	slots = slotsOf(t, s.Availability(queryExplain("r_anker", date, "2", "true")))
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] != date+"T20:30" {
			continue
		}
		ex := explainOf(t, sl)
		for _, i := range []int{0, 1} {
			rules := ex[i].(map[string]any)["rules"].([]any)
			if rules[1].(map[string]any)["holds"].(bool) {
				t.Fatalf("pair member %d not blocked: %v", i, ex[i])
			}
		}
		if rules := ex[2].(map[string]any)["rules"].([]any); !rules[1].(map[string]any)["holds"].(bool) {
			t.Fatalf("uninvolved table blocked: %v", ex[2])
		}
	}
	// Adjacency: a booking ending exactly at the slot start does not overlap.
	slots = slotsOf(t, s.Availability(queryExplain("r_anker", date, "2", "true")))
	for _, sl := range slots {
		if sl.(map[string]any)["starts_at_local"] != date+"T19:30" {
			continue
		}
		ex := explainOf(t, sl)
		// t_1 was booked 19:00+90m = ends 20:30; 19:30 overlaps. t_3 free.
		if rules := ex[2].(map[string]any)["rules"].([]any); !rules[1].(map[string]any)["holds"].(bool) {
			t.Fatalf("t_3 wrongly blocked at 19:30: %v", ex[2])
		}
	}
	// Closed day: slots [] with and without explain.
	if res := s.Availability(queryExplain("r_anker", "2027-06-16", "2", "true")); res.Status != 200 {
		t.Fatalf("closed day: %d", res.Status)
	} else if slots := res.Body.(map[string]any)["slots"].([]any); len(slots) != 0 {
		t.Fatalf("closed day slots = %v", slots)
	}
}

func TestDatedAvailabilityUsesPolicy(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	date := "2027-06-17"
	body, _ := json.Marshal(map[string]any{
		"effective_from": date, "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 60,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 4, "t_2": 6, "t_3": 4},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "dated-01", body); res.Status != 201 {
		t.Fatalf("publish: %d %v", res.Status, res.Body)
	}
	slots := slotsOf(t, s.Availability(queryOf("r_anker", date, "8")))
	if len(slots) != 5 {
		t.Fatalf("policy grid slots = %d, want 5 (18:00..22:00 hourly)", len(slots))
	}
	first := slots[0].(map[string]any)
	if first["starts_at_local"] != date+"T18:00" || first["starts_at"] != date+"T18:00:00+02:00" {
		t.Fatalf("first slot = %v", first)
	}
	if ids := tablesOf(first); len(ids) != 0 {
		t.Fatalf("party 8 singles = %v, want none", ids)
	}
	opts := first["available_options"].([]any)
	if len(opts) != 2 {
		t.Fatalf("party 8 options = %v", opts)
	}
	for i, want := range [][]string{{"t_1", "t_2"}, {"t_2", "t_3"}} {
		opt := opts[i].(map[string]any)
		ids := stringList(t, opt["table_ids"])
		if len(ids) != 2 || ids[0] != want[0] || ids[1] != want[1] {
			t.Fatalf("option %d = %v", i, opt)
		}
		if capVal, ok := opt["capacity"].(int); !ok || capVal != 10 {
			t.Fatalf("selected pair capacity = %v, want 10 (not original 6)", opt["capacity"])
		}
	}
	// No transitive pair: t_1+t_3 never appears.
	for _, sl := range slots {
		for _, o := range sl.(map[string]any)["available_options"].([]any) {
			ids := stringList(t, o.(map[string]any)["table_ids"])
			if len(ids) == 2 && ids[0] == "t_1" && ids[1] == "t_3" {
				t.Fatalf("transitive pair offered: %v", o)
			}
		}
	}
	// Singles use selected caps: party 4 sees all three, with selected caps.
	slots = slotsOf(t, s.Availability(queryOf("r_anker", date, "4")))
	if ids := tablesOf(slots[0]); len(ids) != 3 {
		t.Fatalf("party 4 singles = %v", ids)
	}
	// An earlier date still uses policy0's grid.
	early := slotsOf(t, s.Availability(queryOf("r_anker", "2027-06-10", "2")))
	if len(early) != 8 {
		t.Fatalf("pre-policy grid slots = %d, want 8", len(early))
	}
	// The ordinary detail still returns the original fixture configuration.
	var detail map[string]any
	raw, _ := json.Marshal(s.GetRestaurant("r_anker").Body)
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["slot_minutes"] != float64(30) || detail["reservation_duration_minutes"] != float64(90) {
		t.Fatalf("detail mutated: %v", detail)
	}
	tabs := detail["tables"].([]any)
	if tabs[0].(map[string]any)["capacity"] != float64(2) || tabs[1].(map[string]any)["capacity"] != float64(4) {
		t.Fatalf("detail capacities mutated: %v", tabs)
	}
}

func TestAvailabilityDSTUnderPolicy(t *testing.T) {
	s, mgr, _ := newPolicyService(t)
	berlin, _ := json.Marshal(map[string]any{
		"effective_from": "2026-03-29", "slot_minutes": 30, "reservation_duration_minutes": 30,
		"cancellation_cutoff_minutes": 0,
		"opening_hours":               []any{map[string]any{"weekday": "sun", "opens": "00:00", "closes": "05:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4, "t_3": 4},
	})
	if res := s.PublishPolicy(mgr, "r_anker", "dst-berlin", berlin); res.Status != 201 {
		t.Fatalf("publish berlin: %d %v", res.Status, res.Body)
	}
	slots := slotsOf(t, s.Availability(queryOf("r_anker", "2026-03-29", "2")))
	for _, sl := range slots {
		local := sl.(map[string]any)["starts_at_local"].(string)
		if local == "2026-03-29T02:00" || local == "2026-03-29T02:30" {
			t.Fatalf("skipped wall appears under policy grid: %s", local)
		}
	}
	ny, _ := json.Marshal(map[string]any{
		"effective_from": "2026-11-01", "slot_minutes": 60, "reservation_duration_minutes": 60,
		"cancellation_cutoff_minutes": 0,
		"opening_hours":               []any{map[string]any{"weekday": "sun", "opens": "00:00", "closes": "05:00"}},
		"capacities":                  map[string]any{"t_1": 4},
	})
	if res := s.PublishPolicy(mgr, "r_ny", "dst-ny", ny); res.Status != 201 {
		t.Fatalf("publish ny: %d %v", res.Status, res.Body)
	}
	slots = slotsAny(t, s.Availability(queryOf("r_ny", "2026-11-01", "2")))
	seen := map[string]int{}
	var firstOffset string
	for _, sl := range slots {
		m := sl.(map[string]any)
		seen[m["starts_at_local"].(string)]++
		if m["starts_at_local"] == "2026-11-01T01:00" {
			firstOffset = m["starts_at"].(string)
		}
	}
	for local, n := range seen {
		if n != 1 {
			t.Fatalf("slot %s appears %d times", local, n)
		}
	}
	if firstOffset != "2026-11-01T01:00:00-04:00" {
		t.Fatalf("fold resolves to %q, want first occurrence -04:00", firstOffset)
	}
}

func TestFixtureAboveMaxUsable(t *testing.T) {
	s := New()
	fixture := `{
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_big", "name": "Big", "timezone": "Europe/Berlin",
    "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
    "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
    "tables": [{"id": "t_1", "label": "1", "capacity": 150}],
    "manager_user_ids": ["u_ada"]
  }],
  "reservations": []
}`
	if res := s.Reset([]byte(fixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	// Fixture capacity 150 exceeds the publication maximum of 100, but
	// policy0 must still decide reads.
	slots := slotsAny(t, s.Availability(queryOf("r_big", "2027-06-17", "100")))
	if ids := tablesOf(slots[0]); len(ids) != 1 || ids[0] != "t_1" {
		t.Fatalf("party 100 ids = %v", ids)
	}
	exSlots := slotsAny(t, s.Availability(queryExplain("r_big", "2027-06-17", "100", "true")))
	ex := explainOf(t, exSlots[0])
	if len(ex) != 1 {
		t.Fatalf("explain entries = %v", ex)
	}
	entry := ex[0].(map[string]any)
	if pv, ok := entry["policy_version"].(int); !ok || pv != 0 {
		t.Fatalf("explain = %v", entry)
	}
	if avail, ok := entry["available"].(bool); !ok || !avail {
		t.Fatalf("explain = %v", entry)
	}
}

func TestClosureAvailabilityService(t *testing.T) {
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1","t_2"],["t_2","t_3"]], "manager_user_ids": ["u_ada"]},
    {"id": "r_other", "name": "Other", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 0,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 4}], "manager_user_ids": ["u_ada"]}],
  "reservations": []}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	avail := func(restaurant, date string, party int, explain bool) map[string]any {
		t.Helper()
		q := map[string][]string{"restaurant_id": {restaurant}, "date": {date}, "party_size": {closureItoa(party)}}
		if explain {
			q["explain"] = []string{"true"}
		}
		res := s.Availability(q)
		if res.Status != 200 {
			t.Fatalf("availability: %d %v", res.Status, res.Body)
		}
		m, _ := res.Body.(map[string]any)
		return m
	}
	slotIDs := func(m map[string]any, local string) (ids []string, opts []map[string]any, ex []any) {
		t.Helper()
		for _, x := range m["slots"].([]any) {
			sm, _ := x.(map[string]any)
			if sm["starts_at_local"] != local {
				continue
			}
			raw, _ := json.Marshal(sm["available_table_ids"])
			_ = json.Unmarshal(raw, &ids)
			oraw, _ := json.Marshal(sm["available_options"])
			_ = json.Unmarshal(oraw, &opts)
			if e, ok := sm["explain"]; ok {
				eraw, _ := json.Marshal(e)
				_ = json.Unmarshal(eraw, &ex)
			}
			return ids, opts, ex
		}
		t.Fatalf("slot %s missing", local)
		return nil, nil, nil
	}
	contains := func(ids []string, id string) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	// Baseline: no closures, t_2 free at 19:00 for party 2.
	ids, opts, _ := slotIDs(avail("r_anker", "2027-06-17", 2, false), "2027-06-17T19:00")
	if !contains(ids, "t_2") {
		t.Fatalf("baseline t_2 free: %v", ids)
	}
	// Short closure strictly inside the slot interval blocks the member
	// and every declared pair containing it.
	s.mu.Lock()
	s.state.Closures["r_anker"] = []Closure{{TableID: "t_2", From: "2027-06-17T19:15:00+02:00", To: "2027-06-17T19:45:00+02:00"}}
	s.mu.Unlock()
	ids, opts, ex := slotIDs(avail("r_anker", "2027-06-17", 2, true), "2027-06-17T19:00")
	if contains(ids, "t_2") {
		t.Fatalf("short-inner closure must exclude t_2: %v", ids)
	}
	for _, o := range opts {
		for _, id := range o["table_ids"].([]any) {
			if id == "t_2" {
				t.Fatalf("pair member not excluded: %v", opts)
			}
		}
	}
	found := false
	for _, e := range ex {
		em, _ := e.(map[string]any)
		if em["table_id"] == "t_2" {
			found = true
			rm := map[string]bool{}
			for _, r := range em["rules"].([]any) {
				rn, _ := r.(map[string]any)
				rm[rn["rule"].(string)] = rn["holds"].(bool)
			}
			if !rm["capacity"] || rm["no_overlap"] || em["available"].(bool) {
				t.Fatalf("closure explain: %v", em)
			}
		}
	}
	if !found {
		t.Fatal("t_2 explanation missing")
	}
	// Half-open adjacency: closure ending exactly at slot start is free;
	// closure starting exactly at slot end is free.
	s.mu.Lock()
	s.state.Closures["r_anker"] = []Closure{{TableID: "t_2", From: "2027-06-17T17:00:00+02:00", To: "2027-06-17T19:00:00+02:00"}}
	s.mu.Unlock()
	ids, _, _ = slotIDs(avail("r_anker", "2027-06-17", 2, false), "2027-06-17T19:00")
	if !contains(ids, "t_2") {
		t.Fatalf("adjacent-before closure must free t_2: %v", ids)
	}
	s.mu.Lock()
	s.state.Closures["r_anker"] = []Closure{{TableID: "t_2", From: "2027-06-17T20:30:00+02:00", To: "2027-06-17T21:00:00+02:00"}}
	s.mu.Unlock()
	ids, _, _ = slotIDs(avail("r_anker", "2027-06-17", 2, false), "2027-06-17T19:00")
	if !contains(ids, "t_2") {
		t.Fatalf("adjacent-after closure must free t_2: %v", ids)
	}
	// Absolute-offset equivalence: the same instant spelled +01:00 blocks.
	s.mu.Lock()
	s.state.Closures["r_anker"] = []Closure{{TableID: "t_2", From: "2027-06-17T18:00:00+01:00", To: "2027-06-17T22:00:00+01:00"}}
	s.mu.Unlock()
	ids, _, _ = slotIDs(avail("r_anker", "2027-06-17", 2, false), "2027-06-17T19:00")
	if contains(ids, "t_2") {
		t.Fatalf("absolute-offset closure must exclude t_2: %v", ids)
	}
	// Other restaurant unaffected by r_anker closures.
	ids, _, _ = slotIDs(avail("r_other", "2027-06-17", 1, false), "2027-06-17T19:00")
	if len(ids) != 1 || ids[0] != "t_1" {
		t.Fatalf("other restaurant: %v", ids)
	}
	// No-explain shape carries no explain key.
	m := avail("r_anker", "2027-06-17", 2, false)
	for _, x := range m["slots"].([]any) {
		if _, ok := x.(map[string]any)["explain"]; ok {
			t.Fatal("explain leaked")
		}
	}
}

func closureItoa(n int) string {
	return strconv.Itoa(n)
}
