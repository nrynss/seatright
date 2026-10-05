package service

import (
	"net/http"
	"net/url"
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
