package clock

import (
	"strings"
	"testing"
	"time"
)

func mustClockErr(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error code %q, got nil", code)
	}
	ce, ok := err.(*Error)
	if !ok {
		t.Fatalf("want *Error, got %T (%v)", err, err)
	}
	if ce.Code != code {
		t.Fatalf("want code %q, got %q", code, ce.Code)
	}
}

func berlinRules() Rules {
	return Rules{
		Timezone:        "Europe/Berlin",
		SlotMinutes:     30,
		DurationMinutes: 90,
		OpeningHours: []Hours{
			{Weekday: "thu", Opens: "18:00", Closes: "23:00"},
			{Weekday: "fri", Opens: "18:00", Closes: "23:30"},
		},
	}
}

// R29/R56: strict calendar date validation.
func TestParseDateTable(t *testing.T) {
	valid := []string{"2026-09-24", "2024-02-29", "2020-01-02", "2026-10-25"}
	for _, d := range valid {
		if _, err := ParseDate(d); err != nil {
			t.Errorf("ParseDate(%q): want nil, got %v", d, err)
		}
	}
	invalid := []string{
		"", "2026-9-24", "2026-09-4", "24-09-2026", "2026/09/24",
		"2026-13-01", "2026-00-10", "2026-02-30", "2023-02-29",
		"2026-04-31", "2026-09-24T19:00", "2026-09-24 ", " 2026-09-24",
		"abcd-ef-gh", "2026-09-2x",
	}
	for _, d := range invalid {
		mustClockErr(t, func() error { _, err := ParseDate(d); return err }(), "validation_failed")
	}
	// R26: past dates are accepted, never rejected for being past.
	if _, err := ParseDate("2020-01-02"); err != nil {
		t.Fatalf("past date rejected: %v", err)
	}
}

// R29/R31: only bare YYYY-MM-DDTHH:MM is accepted.
func TestResolveLocalRejectsNonBare(t *testing.T) {
	bad := []string{
		"", "2026-09-24", "19:00",
		"2026-09-24T19:00:00", "2026-09-24T19:00:00+02:00",
		"2026-09-24T19:00Z", "2026-09-24 19:00",
		"2026-09-24T7:00", "2026-9-24T19:00", "2026-09-24T19:0",
		"2026-09-24T24:00", "2026-09-24T19:60", "2026-13-01T19:00",
		"2026-02-30T19:00", "2026-09-24T19:00 ", "X026-09-24T19:00",
	}
	for _, s := range bad {
		_, err := ResolveLocal(s, "Europe/Berlin")
		mustClockErr(t, err, "validation_failed")
	}
	if _, err := ResolveLocal("2026-09-24T19:00", "Mars/Olympus"); err == nil {
		t.Fatal("want error for unknown zone")
	} else {
		mustClockErr(t, err, "validation_failed")
	}
}

// R80: IANA offsets for both named zones, summer and winter.
func TestResolveLocalOffsets(t *testing.T) {
	cases := []struct {
		local string
		zone  string
		want  int // seconds east of UTC
	}{
		{"2026-01-15T19:00", "Europe/Berlin", 3600},
		{"2026-07-15T19:00", "Europe/Berlin", 7200},
		{"2026-09-24T19:00", "Europe/Berlin", 7200},
		{"2026-01-15T19:00", "America/New_York", -5 * 3600},
		{"2026-07-15T19:00", "America/New_York", -4 * 3600},
	}
	for _, c := range cases {
		got, err := ResolveLocal(c.local, c.zone)
		if err != nil {
			t.Errorf("ResolveLocal(%q,%q): %v", c.local, c.zone, err)
			continue
		}
		_, off := got.Zone()
		if off != c.want {
			t.Errorf("ResolveLocal(%q,%q) offset = %d, want %d", c.local, c.zone, off, c.want)
		}
	}
}

// R81: spring-forward skipped hours do not exist.
func TestSpringForwardSkipped(t *testing.T) {
	for _, c := range []struct{ local, zone string }{
		{"2026-03-29T02:00", "Europe/Berlin"},
		{"2026-03-29T02:30", "Europe/Berlin"},
		{"2026-03-08T02:00", "America/New_York"},
		{"2026-03-08T02:30", "America/New_York"},
	} {
		_, err := ResolveLocal(c.local, c.zone)
		mustClockErr(t, err, "invalid_local_time")
	}
	// Valid neighbours on the same day still resolve.
	for _, c := range []struct {
		local, zone string
		off         int
	}{
		{"2026-03-29T01:30", "Europe/Berlin", 3600},
		{"2026-03-29T03:30", "Europe/Berlin", 7200},
		{"2026-03-08T01:30", "America/New_York", -5 * 3600},
		{"2026-03-08T03:30", "America/New_York", -4 * 3600},
	} {
		got, err := ResolveLocal(c.local, c.zone)
		if err != nil {
			t.Errorf("ResolveLocal(%q): %v", c.local, err)
			continue
		}
		if _, off := got.Zone(); off != c.off {
			t.Errorf("ResolveLocal(%q) offset = %d, want %d", c.local, off, c.off)
		}
	}
}

// R82: fall-back repeated times resolve to the first occurrence.
func TestFallBackFirstOccurrence(t *testing.T) {
	berlin, err := ResolveLocal("2026-10-25T02:30", "Europe/Berlin")
	if err != nil {
		t.Fatalf("berlin repeated: %v", err)
	}
	if _, off := berlin.Zone(); off != 7200 {
		t.Fatalf("berlin repeated offset = %d, want 7200 (CEST first)", off)
	}
	if want := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC); !berlin.UTC().Equal(want) {
		t.Fatalf("berlin repeated UTC = %v, want %v", berlin.UTC(), want)
	}

	ny, err := ResolveLocal("2026-11-01T01:30", "America/New_York")
	if err != nil {
		t.Fatalf("ny repeated: %v", err)
	}
	if _, off := ny.Zone(); off != -4*3600 {
		t.Fatalf("ny repeated offset = %d, want -14400 (EDT first)", off)
	}
	if want := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC); !ny.UTC().Equal(want) {
		t.Fatalf("ny repeated UTC = %v, want %v", ny.UTC(), want)
	}
}

// R83: duration is absolute elapsed time across the repeated hour.
func TestAbsoluteDurationAcrossFallback(t *testing.T) {
	rules := Rules{
		Timezone:        "Europe/Berlin",
		SlotMinutes:     30,
		DurationMinutes: 90,
		OpeningHours:    []Hours{{Weekday: "sun", Opens: "00:00", Closes: "05:00"}},
	}
	slot, err := ValidateSlot("2026-10-25T01:30", rules)
	if err != nil {
		t.Fatalf("ValidateSlot: %v", err)
	}
	if d := slot.End.Sub(slot.Start); d != 90*time.Minute {
		t.Fatalf("absolute duration = %v, want 90m", d)
	}
	// Spec example: local ends_at reads 02:00, not 03:00.
	if got := slot.End.Format("2006-01-02T15:04"); got != "2026-10-25T02:00" {
		t.Fatalf("ends_at wall = %s, want 2026-10-25T02:00", got)
	}
	if _, off := slot.End.Zone(); off != 3600 {
		t.Fatalf("ends_at offset = %d, want 3600 (CET)", off)
	}
}

// R57/R58: wall grid from opens, absolute end finishing by closing; ordering.
func TestSlotsGridBounds(t *testing.T) {
	slots, err := Slots("2026-09-24", berlinRules())
	if err != nil {
		t.Fatalf("Slots: %v", err)
	}
	want := []string{"18:00", "18:30", "19:00", "19:30", "20:00", "20:30", "21:00", "21:30"}
	if len(slots) != len(want) {
		t.Fatalf("got %d slots, want %d", len(slots), len(want))
	}
	for i, s := range slots {
		if wantLocal := "2026-09-24T" + want[i]; s.Local != wantLocal {
			t.Errorf("slot %d Local = %q, want %q", i, s.Local, wantLocal)
		}
		if i > 0 && !slots[i-1].Start.Before(s.Start) {
			t.Errorf("slots not ascending at %d", i)
		}
		if d := s.End.Sub(s.Start); d != 90*time.Minute {
			t.Errorf("slot %s duration = %v, want 90m", s.Local, d)
		}
		if got := s.Start.Format("2006-01-02T15:04"); got != s.Local {
			t.Errorf("slot Start wall = %s, want %s", got, s.Local)
		}
	}
	// Slot ending exactly at closes is included; anything later is not.
	last, err := ValidateSlot("2026-09-24T21:30", berlinRules())
	if err != nil {
		t.Fatalf("21:30 must be valid: %v", err)
	}
	if got := last.End.Format("15:04"); got != "23:00" {
		t.Fatalf("21:30 end = %s, want 23:00", got)
	}
	if _, err := ValidateSlot("2026-09-24T22:00", berlinRules()); err == nil {
		t.Fatal("22:00 must be outside opening hours")
	} else {
		mustClockErr(t, err, "outside_opening_hours")
	}
}

// R60/R81/R82: skipped walls absent, repeated walls appear exactly once.
func TestSlotsDSTDayShapes(t *testing.T) {
	night := func() Rules {
		return Rules{
			Timezone:        "Europe/Berlin",
			SlotMinutes:     30,
			DurationMinutes: 30,
			OpeningHours:    []Hours{{Weekday: "sun", Opens: "00:00", Closes: "05:00"}},
		}
	}
	spring, err := Slots("2026-03-29", night())
	if err != nil {
		t.Fatalf("spring Slots: %v", err)
	}
	if len(spring) != 8 {
		t.Fatalf("spring slots = %d, want 8 (11 walls minus 2 skipped minus 05:00 over close)", len(spring))
	}
	for _, s := range spring {
		if hhmm := s.Local[11:]; hhmm == "02:00" || hhmm == "02:30" {
			t.Fatalf("skipped wall %q appears in availability", s.Local)
		}
	}
	fall, err := Slots("2026-10-25", night())
	if err != nil {
		t.Fatalf("fall Slots: %v", err)
	}
	if len(fall) != 10 {
		t.Fatalf("fall slots = %d, want 10 (11 walls minus 05:00 over close)", len(fall))
	}
	seen := map[string]int{}
	for _, s := range fall {
		seen[s.Local[11:]]++
	}
	if seen["02:00"] != 1 || seen["02:30"] != 1 {
		t.Fatalf("repeated walls must appear once: %v", seen)
	}
	// New York spring day: same skip behaviour, different zone.
	nyNight := Rules{
		Timezone:        "America/New_York",
		SlotMinutes:     60,
		DurationMinutes: 60,
		OpeningHours:    []Hours{{Weekday: "sun", Opens: "00:00", Closes: "06:00"}},
	}
	ny, err := Slots("2026-03-08", nyNight)
	if err != nil {
		t.Fatalf("ny spring Slots: %v", err)
	}
	for _, s := range ny {
		if hhmm := s.Local[11:]; hhmm == "02:00" {
			t.Fatalf("skipped wall %q appears in ny availability", s.Local)
		}
	}
}

// A closes wall in the spring gap still bounds the day instead of failing.
func TestSlotsClosesInSkippedHour(t *testing.T) {
	rules := Rules{
		Timezone:        "Europe/Berlin",
		SlotMinutes:     30,
		DurationMinutes: 30,
		OpeningHours:    []Hours{{Weekday: "sun", Opens: "00:00", Closes: "02:30"}},
	}
	slots, err := Slots("2026-03-29", rules)
	if err != nil {
		t.Fatalf("Slots: %v", err)
	}
	if len(slots) != 4 {
		t.Fatalf("slots = %d, want 4 (00:00,00:30,01:00,01:30)", len(slots))
	}
	// Ending exactly at the resolved 02:30->03:00 bound is inside hours.
	if _, err := ValidateSlot("2026-03-29T01:30", rules); err != nil {
		t.Fatalf("01:30 + 30m ends exactly at bound: %v", err)
	}
}

// R60: closed day yields an empty (non-nil) slot list.
func TestSlotsClosedDay(t *testing.T) {
	thuOnly := Rules{
		Timezone:        "Europe/Berlin",
		SlotMinutes:     30,
		DurationMinutes: 90,
		OpeningHours:    []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
	}
	closed, err := Slots("2026-09-25", thuOnly) // Friday: closed
	if err != nil {
		t.Fatalf("closed day: %v", err)
	}
	if closed == nil || len(closed) != 0 {
		t.Fatalf("closed day slots = %v, want empty non-nil", closed)
	}
	if _, err := ValidateSlot("2026-09-25T19:00", thuOnly); err == nil {
		t.Fatal("booking on closed day must fail")
	} else {
		mustClockErr(t, err, "outside_opening_hours")
	}
}

// R64/R65/R29: ValidateSlot error codes and precedence of existence first.
func TestValidateSlotErrors(t *testing.T) {
	rules := berlinRules()
	cases := []struct {
		name  string
		local string
		r     Rules
		code  string
	}{
		{"off grid inside hours", "2026-09-24T19:15", rules, "not_on_slot_grid"},
		{"before opens", "2026-09-24T17:30", rules, "outside_opening_hours"},
		{"end after closes", "2026-09-24T22:00", rules, "outside_opening_hours"},
		{"skipped wall", "2026-03-29T02:30", Rules{
			Timezone: "Europe/Berlin", SlotMinutes: 30, DurationMinutes: 30,
			OpeningHours: []Hours{{Weekday: "sun", Opens: "00:00", Closes: "05:00"}},
		}, "invalid_local_time"},
		{"malformed", "2026-09-24 19:00", rules, "validation_failed"},
		{"bad zone", "2026-09-24T19:00", Rules{
			Timezone: "No/Such_Zone", SlotMinutes: 30, DurationMinutes: 90,
			OpeningHours: []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
		}, "validation_failed"},
		{"zero grid", "2026-09-24T19:00", Rules{
			Timezone: "Europe/Berlin", SlotMinutes: 0, DurationMinutes: 90,
			OpeningHours: []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
		}, "validation_failed"},
		{"bad hours", "2026-09-24T19:00", Rules{
			Timezone: "Europe/Berlin", SlotMinutes: 30, DurationMinutes: 90,
			OpeningHours: []Hours{{Weekday: "thu", Opens: "23:00", Closes: "18:00"}},
		}, "validation_failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ValidateSlot(c.local, c.r)
			mustClockErr(t, err, c.code)
		})
	}
}

// R26: past start dates alone are never rejected.
func TestValidateSlotPastDate(t *testing.T) {
	rules := Rules{
		Timezone:        "Europe/Berlin",
		SlotMinutes:     30,
		DurationMinutes: 90,
		OpeningHours:    []Hours{{Weekday: "thu", Opens: "18:00", Closes: "23:00"}},
	}
	s, err := ValidateSlot("2020-01-02T19:00", rules) // a Thursday
	if err != nil {
		t.Fatalf("past date rejected: %v", err)
	}
	if s.Local != "2020-01-02T19:00" {
		t.Fatalf("Local echo = %q", s.Local)
	}
}

// R2: half-open overlap, adjacency is not overlap.
func TestOverlapTable(t *testing.T) {
	mk := func(h, m int) time.Time {
		return time.Date(2026, 9, 24, h, m, 0, 0, time.UTC)
	}
	cases := []struct {
		name string
		a    [2]time.Time
		b    [2]time.Time
		want bool
	}{
		{"identical", [2]time.Time{mk(19, 0), mk(20, 30)}, [2]time.Time{mk(19, 0), mk(20, 30)}, true},
		{"partial", [2]time.Time{mk(19, 0), mk(20, 30)}, [2]time.Time{mk(20, 0), mk(21, 30)}, true},
		{"contained", [2]time.Time{mk(19, 0), mk(21, 0)}, [2]time.Time{mk(19, 30), mk(20, 0)}, true},
		{"adjacent after", [2]time.Time{mk(19, 0), mk(20, 30)}, [2]time.Time{mk(20, 30), mk(22, 0)}, false},
		{"adjacent before", [2]time.Time{mk(20, 30), mk(22, 0)}, [2]time.Time{mk(19, 0), mk(20, 30)}, false},
		{"disjoint", [2]time.Time{mk(18, 0), mk(19, 0)}, [2]time.Time{mk(20, 0), mk(21, 0)}, false},
		{"touch at start", [2]time.Time{mk(18, 0), mk(19, 0)}, [2]time.Time{mk(19, 0), mk(19, 30)}, false},
	}
	for _, c := range cases {
		if got := Overlap(c.a[0], c.a[1], c.b[0], c.b[1]); got != c.want {
			t.Errorf("%s: Overlap = %v, want %v", c.name, got, c.want)
		}
		if got := Overlap(c.b[0], c.b[1], c.a[0], c.a[1]); got != c.want {
			t.Errorf("%s (swapped): Overlap = %v, want %v", c.name, got, c.want)
		}
	}
}

// R73: cutoff boundary is inclusive: now at start-cutoff is already passed.
func TestCutoffPassedTable(t *testing.T) {
	start, err := ResolveLocal("2026-09-24T19:00", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		now    time.Time
		cutoff int
		want   bool
	}{
		{"well before", start.Add(-121 * time.Minute), 120, false},
		{"one second early", start.Add(-120*time.Minute - time.Second), 120, false},
		{"exactly at boundary", start.Add(-120 * time.Minute), 120, true},
		{"within", start.Add(-60 * time.Minute), 120, true},
		{"at start", start, 120, true},
		{"after start", start.Add(time.Hour), 120, true},
		{"zero cutoff at start", start, 0, true},
		{"zero cutoff before", start.Add(-time.Second), 0, false},
	}
	for _, c := range cases {
		if got := CutoffPassed(c.now, start, c.cutoff); got != c.want {
			t.Errorf("%s: CutoffPassed = %v, want %v", c.name, got, c.want)
		}
	}
}

// R19: resolved instants format as RFC 3339 with explicit offset.
func TestRFC3339ExplicitOffset(t *testing.T) {
	s, err := ValidateSlot("2026-09-24T19:00", berlinRules())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Start.Format(time.RFC3339); got != "2026-09-24T19:00:00+02:00" {
		t.Fatalf("starts_at = %q, want explicit +02:00 offset", got)
	}
	if got := s.End.Format(time.RFC3339); got != "2026-09-24T20:30:00+02:00" {
		t.Fatalf("ends_at = %q", got)
	}
	if strings.Contains(s.Start.Format(time.RFC3339), "Z") {
		t.Fatal("offset must be explicit, not Z")
	}
}

// Generality: a southern-hemisphere zone with a non-hour DST shape works
// without any zone-specific branching.
func TestGeneralZoneSydney(t *testing.T) {
	if _, err := ResolveLocal("2026-10-04T02:30", "Australia/Sydney"); err == nil {
		t.Fatal("Sydney spring skipped wall must not resolve")
	} else {
		mustClockErr(t, err, "invalid_local_time")
	}
	got, err := ResolveLocal("2026-04-05T02:30", "Australia/Sydney")
	if err != nil {
		t.Fatalf("Sydney repeated: %v", err)
	}
	if _, off := got.Zone(); off != 11*3600 {
		t.Fatalf("Sydney repeated offset = %d, want 39600 (AEDT first)", off)
	}
	if want := time.Date(2026, 4, 4, 15, 30, 0, 0, time.UTC); !got.UTC().Equal(want) {
		t.Fatalf("Sydney repeated UTC = %v, want %v", got.UTC(), want)
	}
}

// Slots rejects malformed dates with validation_failed (R56).
func TestSlotsMalformedDate(t *testing.T) {
	for _, d := range []string{"", "2026-02-30", "24-09-2026", "2026-09-24T00:00"} {
		_, err := Slots(d, berlinRules())
		mustClockErr(t, err, "validation_failed")
	}
}
