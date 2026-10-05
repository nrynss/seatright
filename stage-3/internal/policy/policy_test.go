package policy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Requirement map (engine portions only; no HTTP/permissions/versions/counters):
// R230/R236 wrong-type + missing-field matrices -> TestParseInvalidMatrix
// R231 strict real dates incl leap days -> TestParseEffectiveDates
// R232/R233 int ranges, bool/fraction rejection -> TestParseInvalidMatrix
// R234 weekday/HH:MM/non-overnight/duplicates -> TestParseHoursRules
// R235 exact capacities membership -> TestParseCapacities
// R238 unknown fields ignored, version caller-unset -> TestParseUnknownFields
// R226/R227/R228/R229 selection incl unsorted/equal-date/past/supercession -> TestSelect
// R243 snapshot excludes effective_from, deep copy -> TestSelectSnapshot + TestClones
// R241/R266/R267/R268 Rules delegate DST/grid to clock -> TestRulesClockIntegration
// R237/R243 clone nil-vs-empty snapshot shape -> TestCloneEmptyShape
// R244 actual fixture0 beyond maxima usable -> TestFixtureBeyondMaxima
// R289/R294/R295 pair sums from selected policy -> TestCapacityPairs

var testTables = []string{"t_1", "t_2", "t_3"}

func validObj() map[string]any {
	return map[string]any{
		"effective_from":               "2026-09-28",
		"slot_minutes":                 float64(30),
		"reservation_duration_minutes": float64(120),
		"cancellation_cutoff_minutes":  float64(60),
		"opening_hours": []any{
			map[string]any{"weekday": "mon", "opens": "18:00", "closes": "23:00"},
		},
		"capacities": map[string]any{
			"t_1": float64(2), "t_2": float64(4), "t_3": float64(6),
		},
	}
}

func parseErr(t *testing.T, obj map[string]any) *Error {
	t.Helper()
	_, err := Parse(obj, testTables)
	if err == nil {
		t.Fatalf("expected validation_failed, got nil")
	}
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if pe.Code != "validation_failed" {
		t.Fatalf("error code = %q, want validation_failed", pe.Code)
	}
	return pe
}

func TestParseValid(t *testing.T) {
	p, err := Parse(validObj(), testTables)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.EffectiveFrom != "2026-09-28" || p.PolicyVersion != 0 {
		t.Fatalf("header = %+v", p)
	}
	if p.SlotMinutes != 30 || p.ReservationDurationMinutes != 120 || p.CancellationCutoffMinutes != 60 {
		t.Fatalf("terms = %+v", p.Terms)
	}
	if len(p.OpeningHours) != 1 || p.OpeningHours[0].Weekday != "mon" {
		t.Fatalf("hours = %+v", p.OpeningHours)
	}
	if !reflect.DeepEqual(p.Capacities, map[string]int{"t_1": 2, "t_2": 4, "t_3": 6}) {
		t.Fatalf("capacities = %+v", p.Capacities)
	}
	// Flat JSON: no nested "terms" property.
	raw, _ := json.Marshal(p)
	var flat map[string]any
	if err := json.Unmarshal(raw, &flat); err != nil {
		t.Fatal(err)
	}
	if _, ok := flat["terms"]; ok {
		t.Fatalf("policy JSON must be flat, got %s", raw)
	}
	for _, k := range []string{"effective_from", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"} {
		if _, ok := flat[k]; !ok {
			t.Fatalf("flat JSON missing %s: %s", k, raw)
		}
	}
}

func TestParseInvalidMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"nil obj", func(o map[string]any) { o = nil }}, // handled separately below
		{"missing effective_from", func(o map[string]any) { delete(o, "effective_from") }},
		{"missing slot", func(o map[string]any) { delete(o, "slot_minutes") }},
		{"missing duration", func(o map[string]any) { delete(o, "reservation_duration_minutes") }},
		{"missing cutoff", func(o map[string]any) { delete(o, "cancellation_cutoff_minutes") }},
		{"missing hours", func(o map[string]any) { delete(o, "opening_hours") }},
		{"missing capacities", func(o map[string]any) { delete(o, "capacities") }},
		{"effective_from non-string", func(o map[string]any) { o["effective_from"] = float64(20260928) }},
		{"slot bool", func(o map[string]any) { o["slot_minutes"] = true }},
		{"slot string", func(o map[string]any) { o["slot_minutes"] = "30" }},
		{"slot fraction", func(o map[string]any) { o["slot_minutes"] = 30.5 }},
		{"slot zero", func(o map[string]any) { o["slot_minutes"] = float64(0) }},
		{"slot 1441", func(o map[string]any) { o["slot_minutes"] = float64(1441) }},
		{"slot negative", func(o map[string]any) { o["slot_minutes"] = float64(-30) }},
		{"duration bool", func(o map[string]any) { o["reservation_duration_minutes"] = false }},
		{"duration zero", func(o map[string]any) { o["reservation_duration_minutes"] = float64(0) }},
		{"duration 1441", func(o map[string]any) { o["reservation_duration_minutes"] = float64(1441) }},
		{"cutoff bool", func(o map[string]any) { o["cancellation_cutoff_minutes"] = true }},
		{"cutoff negative", func(o map[string]any) { o["cancellation_cutoff_minutes"] = float64(-1) }},
		{"cutoff 10081", func(o map[string]any) { o["cancellation_cutoff_minutes"] = float64(10081) }},
		{"cutoff fraction", func(o map[string]any) { o["cancellation_cutoff_minutes"] = 1.5 }},
		{"hours non-array", func(o map[string]any) { o["opening_hours"] = "mon" }},
		{"hours entry non-object", func(o map[string]any) { o["opening_hours"] = []any{"mon"} }},
		{"hours entry missing closes", func(o map[string]any) {
			o["opening_hours"] = []any{map[string]any{"weekday": "mon", "opens": "18:00"}}
		}},
		{"hours entry non-string opens", func(o map[string]any) {
			o["opening_hours"] = []any{map[string]any{"weekday": "mon", "opens": float64(18), "closes": "23:00"}}
		}},
		{"capacities non-object", func(o map[string]any) { o["capacities"] = float64(4) }},
		{"capacity bool", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": true, "t_2": float64(4), "t_3": float64(6)}
		}},
		{"capacity string", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": "2", "t_2": float64(4), "t_3": float64(6)}
		}},
		{"capacity fraction", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": 2.5, "t_2": float64(4), "t_3": float64(6)}
		}},
		{"capacity zero", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": float64(0), "t_2": float64(4), "t_3": float64(6)}
		}},
		{"capacity 101", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": float64(101), "t_2": float64(4), "t_3": float64(6)}
		}},
		{"capacity negative", func(o map[string]any) {
			o["capacities"] = map[string]any{"t_1": float64(-2), "t_2": float64(4), "t_3": float64(6)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil obj" {
				if _, err := Parse(nil, testTables); err == nil {
					t.Fatal("expected error for nil obj")
				} else if err.(*Error).Code != "validation_failed" {
					t.Fatalf("code = %v", err)
				}
				return
			}
			o := validObj()
			tc.mutate(o)
			pe := parseErr(t, o)
			if pe.Message == "" {
				t.Fatal("empty message")
			}
		})
	}
	// Boundary acceptance: minima and maxima parse.
	for _, tc := range []struct {
		field string
		value float64
	}{
		{"slot_minutes", 1}, {"slot_minutes", 1440},
		{"reservation_duration_minutes", 1}, {"reservation_duration_minutes", 1440},
		{"cancellation_cutoff_minutes", 0}, {"cancellation_cutoff_minutes", 10080},
	} {
		o := validObj()
		o[tc.field] = tc.value
		if _, err := Parse(o, testTables); err != nil {
			t.Fatalf("%s=%v: %v", tc.field, tc.value, err)
		}
	}
}

func TestParseEffectiveDates(t *testing.T) {
	valid := []string{"2026-09-28", "2024-02-29", "2000-02-29", "2027-06-17", "1999-12-31"}
	for _, d := range valid {
		o := validObj()
		o["effective_from"] = d
		if _, err := Parse(o, testTables); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
	}
	invalid := []string{
		"", "2026-9-28", "2026-09-8", "2026/09/28", "28-09-2026",
		"2026-02-29", "2023-02-29", "1900-02-29", "2026-13-01", "2026-00-10",
		"2026-04-31", "2026-06-31", "2026-09-00", "2026-09-32",
		"2026-09-28T00:00", "2026-09-28Z", "abcd-ef-gh", "2026-09-2x",
	}
	for _, d := range invalid {
		o := validObj()
		o["effective_from"] = d
		parseErr(t, o)
	}
}

func TestParseHoursRules(t *testing.T) {
	cases := []struct {
		name  string
		hours []any
	}{
		{"bad weekday", []any{map[string]any{"weekday": "monday", "opens": "18:00", "closes": "23:00"}}},
		{"uppercase weekday", []any{map[string]any{"weekday": "MON", "opens": "18:00", "closes": "23:00"}}},
		{"duplicate weekday", []any{
			map[string]any{"weekday": "mon", "opens": "18:00", "closes": "20:00"},
			map[string]any{"weekday": "mon", "opens": "20:00", "closes": "23:00"},
		}},
		{"overnight closes<=opens", []any{map[string]any{"weekday": "mon", "opens": "23:00", "closes": "02:00"}}},
		{"equal opens closes", []any{map[string]any{"weekday": "mon", "opens": "18:00", "closes": "18:00"}}},
		{"bad opens format", []any{map[string]any{"weekday": "mon", "opens": "6pm", "closes": "23:00"}}},
		{"bad closes hour", []any{map[string]any{"weekday": "mon", "opens": "18:00", "closes": "24:00"}}},
		{"bad opens minute", []any{map[string]any{"weekday": "mon", "opens": "18:60", "closes": "23:00"}}},
		{"missing colon", []any{map[string]any{"weekday": "mon", "opens": "1800", "closes": "23:00"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validObj()
			o["opening_hours"] = tc.hours
			parseErr(t, o)
		})
	}
	// All seven weekdays parse; empty hours parse (closed-every-day policy).
	o := validObj()
	all := []any{}
	for _, wd := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		all = append(all, map[string]any{"weekday": wd, "opens": "09:00", "closes": "17:00"})
	}
	o["opening_hours"] = all
	if _, err := Parse(o, testTables); err != nil {
		t.Fatalf("all weekdays: %v", err)
	}
	o["opening_hours"] = []any{}
	if _, err := Parse(o, testTables); err != nil {
		t.Fatalf("empty hours: %v", err)
	}
}

func TestParseCapacities(t *testing.T) {
	t.Run("missing table", func(t *testing.T) {
		o := validObj()
		o["capacities"] = map[string]any{"t_1": float64(2), "t_2": float64(4)}
		parseErr(t, o)
	})
	t.Run("extra table", func(t *testing.T) {
		o := validObj()
		o["capacities"] = map[string]any{"t_1": float64(2), "t_2": float64(4), "t_3": float64(6), "t_4": float64(1)}
		parseErr(t, o)
	})
	t.Run("unknown id", func(t *testing.T) {
		o := validObj()
		o["capacities"] = map[string]any{"t_1": float64(2), "t_2": float64(4), "t_X": float64(6)}
		parseErr(t, o)
	})
	t.Run("empty tables exact empty", func(t *testing.T) {
		o := validObj()
		o["capacities"] = map[string]any{}
		if _, err := Parse(o, nil); err != nil {
			t.Fatalf("empty/exact: %v", err)
		}
	})
}

func TestParseUnknownFields(t *testing.T) {
	o := validObj()
	o["policy_version"] = float64(99)
	o["whatever"] = "ignored"
	o["effective_from_extra"] = "x"
	p, err := Parse(o, testTables)
	if err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
	if p.PolicyVersion != 0 {
		t.Fatalf("caller must not set version, got %d", p.PolicyVersion)
	}
}

func baseTerms() Terms {
	return Terms{
		PolicyVersion: 0, SlotMinutes: 30, ReservationDurationMinutes: 90,
		CancellationCutoffMinutes: 120,
		OpeningHours:              []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
		Capacities:                map[string]int{"t_1": 2, "t_2": 4},
	}
}

func TestSelect(t *testing.T) {
	mk := func(from string, ver, slot int) Policy {
		b := baseTerms()
		b.SlotMinutes = slot
		return Policy{EffectiveFrom: from, Terms: Terms{
			PolicyVersion: ver, SlotMinutes: slot,
			ReservationDurationMinutes: b.ReservationDurationMinutes,
			CancellationCutoffMinutes:  b.CancellationCutoffMinutes,
			OpeningHours:               append([]Hours(nil), b.OpeningHours...),
			Capacities:                 map[string]int{"t_1": 2, "t_2": 4},
		}}
	}
	t.Run("none eligible returns base copy", func(t *testing.T) {
		got, err := Select(baseTerms(), []Policy{mk("2026-09-28", 1, 60)}, "2026-09-27")
		if err != nil {
			t.Fatal(err)
		}
		if got.SlotMinutes != 30 || got.PolicyVersion != 0 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("empty published returns base", func(t *testing.T) {
		got, err := Select(baseTerms(), nil, "2027-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, baseTerms()) {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("greatest date wins unsorted", func(t *testing.T) {
		pubs := []Policy{mk("2026-10-01", 1, 60), mk("2026-09-28", 1, 15), mk("2026-09-20", 1, 45)}
		got, err := Select(baseTerms(), pubs, "2026-12-31")
		if err != nil {
			t.Fatal(err)
		}
		if got.SlotMinutes != 60 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("equal date greatest version", func(t *testing.T) {
		pubs := []Policy{mk("2026-09-28", 3, 60), mk("2026-09-28", 1, 15), mk("2026-09-28", 2, 45)}
		got, err := Select(baseTerms(), pubs, "2026-09-28")
		if err != nil {
			t.Fatal(err)
		}
		if got.PolicyVersion != 3 || got.SlotMinutes != 60 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("effective order beats publication order", func(t *testing.T) {
		// Published newest-first: version 2 effective later, version 1 earlier.
		// A booking between them must select version 1 despite version 2
		// being published later.
		pubs := []Policy{mk("2026-10-05", 2, 60), mk("2026-09-28", 1, 15)}
		got, err := Select(baseTerms(), pubs, "2026-10-01")
		if err != nil {
			t.Fatal(err)
		}
		if got.PolicyVersion != 1 || got.SlotMinutes != 15 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("past effective accepted", func(t *testing.T) {
		got, err := Select(baseTerms(), []Policy{mk("2020-01-15", 1, 60)}, "2026-09-28")
		if err != nil {
			t.Fatal(err)
		}
		if got.SlotMinutes != 60 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("same-date supersession future only", func(t *testing.T) {
		pubs := []Policy{mk("2026-09-28", 1, 15), mk("2026-09-28", 2, 60)}
		before, err := Select(baseTerms(), pubs[:1], "2026-09-28")
		if err != nil {
			t.Fatal(err)
		}
		after, err := Select(baseTerms(), pubs, "2026-09-28")
		if err != nil {
			t.Fatal(err)
		}
		if before.SlotMinutes != 15 || after.SlotMinutes != 60 {
			t.Fatalf("before=%+v after=%+v", before, after)
		}
		// Earlier date unaffected by either.
		old, err := Select(baseTerms(), pubs, "2026-09-27")
		if err != nil {
			t.Fatal(err)
		}
		if old.SlotMinutes != 30 {
			t.Fatalf("old=%+v", old)
		}
	})
	t.Run("leap date selection", func(t *testing.T) {
		pubs := []Policy{mk("2024-02-29", 1, 60)}
		got, err := Select(baseTerms(), pubs, "2024-02-29")
		if err != nil {
			t.Fatal(err)
		}
		if got.SlotMinutes != 60 {
			t.Fatalf("got %+v", got)
		}
		if _, err := Select(baseTerms(), pubs, "2023-02-29"); err == nil {
			t.Fatal("non-leap date must fail")
		}
	})
	t.Run("invalid date", func(t *testing.T) {
		for _, d := range []string{"", "2026-13-01", "2026-02-30", "not-a-date"} {
			if _, err := Select(baseTerms(), nil, d); err == nil {
				t.Fatalf("%q: expected error", d)
			} else if err.(*Error).Code != "validation_failed" {
				t.Fatalf("%q: code=%v", d, err)
			}
		}
	})
	t.Run("result is snapshot", func(t *testing.T) {
		pub := mk("2026-09-28", 5, 60)
		pubs := []Policy{pub}
		got, err := Select(baseTerms(), pubs, "2026-10-01")
		if err != nil {
			t.Fatal(err)
		}
		got.SlotMinutes = 999
		got.Capacities["t_1"] = 999
		got.OpeningHours[0].Opens = "00:00"
		if pubs[0].SlotMinutes != 60 || pubs[0].Capacities["t_1"] != 2 || pubs[0].OpeningHours[0].Opens != "18:00" {
			t.Fatalf("Select mutated input: %+v", pubs[0])
		}
	})
	t.Run("no version allocation", func(t *testing.T) {
		base := baseTerms()
		if _, err := Select(base, nil, "2026-10-01"); err != nil {
			t.Fatal(err)
		}
		if base.PolicyVersion != 0 {
			t.Fatal("Select must not allocate versions")
		}
	})
}

func TestSelectSnapshot(t *testing.T) {
	// accepted_terms shape: all selected fields, no effective_from.
	got, err := Select(baseTerms(), nil, "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"policy_version", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("terms missing %s: %s", k, raw)
		}
	}
	if _, ok := m["effective_from"]; ok {
		t.Fatalf("terms must exclude effective_from: %s", raw)
	}
}

func TestCloneEmptyShape(t *testing.T) {
	// R237/R243: cloning must preserve nil vs non-nil empty shape so the
	// complete policy snapshot keeps opening_hours:[] instead of null.
	obj := map[string]any{
		"effective_from": "2026-09-28", "slot_minutes": float64(30),
		"reservation_duration_minutes": float64(120), "cancellation_cutoff_minutes": float64(60),
		"opening_hours": []any{}, "capacities": map[string]any{},
	}
	p, err := Parse(obj, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	mustEncode := func(name string, v any, wantHours, wantCaps string) {
		t.Helper()
		raw, _ := json.Marshal(v)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		h, _ := json.Marshal(m["opening_hours"])
		c, _ := json.Marshal(m["capacities"])
		if string(h) != wantHours {
			t.Fatalf("%s opening_hours = %s, want %s (full %s)", name, h, wantHours, raw)
		}
		if string(c) != wantCaps {
			t.Fatalf("%s capacities = %s, want %s (full %s)", name, c, wantCaps, raw)
		}
	}
	mustEncode("parsed", p, "[]", "{}")
	mustEncode("clone-terms", CloneTerms(p.Terms), "[]", "{}")
	mustEncode("clone-policy", ClonePolicy(p), "[]", "{}")
	sel, err := Select(p.Terms, nil, "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	mustEncode("selected", sel, "[]", "{}")
	// Published-policy path preserves shape too.
	pub := ClonePolicy(p)
	pub.EffectiveFrom = "2026-09-28"
	pub.PolicyVersion = 1
	sel2, err := Select(baseTerms(), []Policy{pub}, "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	mustEncode("selected-published", sel2, "[]", "{}")
	// Nil source stays nil (null), not manufactured into [].
	var nilT Terms
	mustEncode("clone-nil", CloneTerms(nilT), "null", "null")
	selNil, err := Select(nilT, nil, "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	mustEncode("select-nil", selNil, "null", "null")
	// Deep-copy isolation still holds for empty shapes.
	cp := CloneTerms(p.Terms)
	if cp.OpeningHours == nil {
		t.Fatal("non-nil empty hours became nil")
	}
}

func TestClones(t *testing.T) {
	src := baseTerms()
	src.PolicyVersion = 3
	c := CloneTerms(src)
	c.Capacities["t_1"] = 99
	c.OpeningHours[0].Opens = "00:00"
	if src.Capacities["t_1"] != 2 || src.OpeningHours[0].Opens != "18:00" {
		t.Fatalf("CloneTerms aliases input: %+v", src)
	}
	p := Policy{EffectiveFrom: "2026-09-28", Terms: src}
	q := ClonePolicy(p)
	q.Capacities["t_2"] = 99
	q.OpeningHours[0].Closes = "00:00"
	q.EffectiveFrom = "2000-01-01"
	if p.Capacities["t_2"] != 4 || p.OpeningHours[0].Closes != "23:00" || p.EffectiveFrom != "2026-09-28" {
		t.Fatalf("ClonePolicy aliases input: %+v", p)
	}
	var nilCap Terms
	n := CloneTerms(nilCap)
	if n.Capacities != nil {
		t.Fatalf("nil map must stay nil: %+v", n)
	}
}

func TestRulesClockIntegration(t *testing.T) {
	// Timezone-general: Rules must drive the real clock grid for arbitrary
	// zones, not fixture branches. Cover Berlin spring-forward skip and
	// New York fall-back first-occurrence via selected terms.
	terms := Terms{
		SlotMinutes: 30, ReservationDurationMinutes: 90,
		OpeningHours: []Hours{
			{Weekday: "sun", Opens: "00:00", Closes: "23:30"},
			{Weekday: "mon", Opens: "18:00", Closes: "23:00"},
		},
		Capacities: map[string]int{"t_1": 2},
	}
	// Berlin spring forward 2026-03-29: 02:30 does not exist.
	berlin := Rules(terms, "Europe/Berlin")
	if _, err := clockResolve("2026-03-29T02:30", berlin); err == nil {
		t.Fatal("skipped Berlin wall must not resolve")
	} else if !strings.Contains(err.Error(), "invalid_local_time") {
		t.Fatalf("code = %v", err)
	}
	// New York fall back 2026-11-01: repeated 01:30 occurs once and resolves
	// to the first (EDT, UTC-4) instant; its 90-minute absolute end is
	// 07:00 UTC, i.e. 02:00 local EST. Delegated generic clock behavior.
	ny := Rules(terms, "America/New_York")
	slots, err := clockSlots("2026-11-01", ny)
	if err != nil {
		t.Fatalf("Slots: %v", err)
	}
	var found int
	for _, sl := range slots {
		if sl.Local == "2026-11-01T01:30" {
			found++
			if off := sl.Start.Format("-07:00"); off != "-04:00" {
				t.Fatalf("fold start offset = %s, want -04:00", off)
			}
			if got := sl.Start.UTC().Format("15:04"); got != "05:30" {
				t.Fatalf("fold start UTC = %s, want 05:30", got)
			}
			if got := sl.End.UTC().Format("15:04"); got != "07:00" {
				t.Fatalf("fold end UTC = %s, want 07:00", got)
			}
			if got := sl.End.Format("15:04-07:00"); got != "02:00-05:00" {
				t.Fatalf("fold end local = %s, want 02:00-05:00", got)
			}
		}
	}
	if found != 1 {
		t.Fatalf("01:30 occurs %d times, want exactly once", found)
	}
	// Mon 18:00 grid under selected slot 30 from 18:00 opening.
	mon, err := clockSlots("2026-10-05", berlin)
	if err != nil {
		t.Fatalf("Slots: %v", err)
	}
	if len(mon) == 0 || mon[0].Local != "2026-10-05T18:00" {
		t.Fatalf("first slot = %+v", mon)
	}
	// Selected slot width changes the grid: 60-minute terms step hourly.
	terms60 := terms
	terms60.SlotMinutes = 60
	mon60, err := clockSlots("2026-10-05", Rules(terms60, "Europe/Berlin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mon60)*2 != len(mon) && len(mon) > 0 {
		t.Fatalf("60-min grid=%d vs 30-min grid=%d", len(mon60), len(mon))
	}
	// Rules field mapping is exact.
	r := Rules(terms, "Europe/Berlin")
	if r.Timezone != "Europe/Berlin" || r.SlotMinutes != 30 || r.DurationMinutes != 90 {
		t.Fatalf("rules = %+v", r)
	}
	if len(r.OpeningHours) != 2 || r.OpeningHours[1].Opens != "18:00" {
		t.Fatalf("rules hours = %+v", r.OpeningHours)
	}
}

func TestFixtureBeyondMaxima(t *testing.T) {
	// Genuine producer-valid fixture0 values ABOVE the publication maxima
	// (slot/duration cap 1440, cutoff cap 10080, capacity cap 100).
	// Selection, Clone, Rules and Capacity must retain them without
	// reapplying Parse restrictions; Parse of the same values must fail.
	fix := Terms{
		PolicyVersion: 0, SlotMinutes: 1441, ReservationDurationMinutes: 1441,
		CancellationCutoffMinutes: 10081,
		OpeningHours:              []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
		Capacities:                map[string]int{"t_1": 101, "t_2": 4},
	}
	got, err := Select(fix, nil, "2027-06-17")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, fix) {
		t.Fatalf("fixture terms altered: %+v", got)
	}
	cloned := CloneTerms(fix)
	if !reflect.DeepEqual(cloned, fix) {
		t.Fatalf("clone altered fixture terms: %+v", cloned)
	}
	r := Rules(fix, "Europe/Berlin")
	if r.SlotMinutes != 1441 || r.DurationMinutes != 1441 {
		t.Fatalf("rules = %+v", r)
	}
	if Capacity(fix, []string{"t_1"}) != 101 {
		t.Fatal("capacity must use fixture values")
	}
	if Capacity(fix, []string{"t_1", "t_2"}) != 105 {
		t.Fatal("pair capacity must use fixture values")
	}
	for _, tc := range []struct {
		field string
		value float64
	}{
		{"slot_minutes", 1441},
		{"reservation_duration_minutes", 1441},
		{"cancellation_cutoff_minutes", 10081},
	} {
		o := validObj()
		o[tc.field] = tc.value
		if _, err := Parse(o, testTables); err == nil {
			t.Fatalf("%s=%v: Parse must reject publication value", tc.field, tc.value)
		} else if err.(*Error).Code != "validation_failed" {
			t.Fatalf("%s: code=%v", tc.field, err)
		}
	}
	o := validObj()
	o["capacities"] = map[string]any{"t_1": float64(101), "t_2": float64(4), "t_3": float64(6)}
	if _, err := Parse(o, testTables); err == nil {
		t.Fatal("capacity 101: Parse must reject publication value")
	}
}

func TestCapacityPairs(t *testing.T) {
	terms := Terms{Capacities: map[string]int{"t_1": 4, "t_2": 6, "t_3": 4}}
	if got := Capacity(terms, []string{"t_1", "t_2"}); got != 10 {
		t.Fatalf("pair = %d", got)
	}
	if got := Capacity(terms, []string{"t_2", "t_3"}); got != 10 {
		t.Fatalf("pair = %d", got)
	}
	if got := Capacity(terms, []string{"t_1"}); got != 4 {
		t.Fatalf("single = %d", got)
	}
	if got := Capacity(terms, nil); got != 0 {
		t.Fatalf("empty = %d", got)
	}
	// Selected-policy capacities win over stale fixture values: same pair
	// under different selected terms sums differently.
	old := Terms{Capacities: map[string]int{"t_1": 2, "t_2": 4, "t_3": 4}}
	if Capacity(old, []string{"t_1", "t_2"}) != 6 {
		t.Fatal("stale sum wrong")
	}
	if Capacity(terms, []string{"t_1", "t_2"}) == Capacity(old, []string{"t_1", "t_2"}) {
		t.Fatal("selected pair sum must differ from stale fixture sum")
	}
}

func TestErrorShape(t *testing.T) {
	_, err := Parse(map[string]any{}, testTables)
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("type = %T", err)
	}
	if pe.Code != "validation_failed" || !strings.Contains(pe.Error(), "validation_failed") {
		t.Fatalf("err = %+v", pe)
	}
}
