// Package clock is the independent stage-1 time/grid engine for Tablekeeper.
//
// Pure standard library only: no service state, no module dependencies.
// Wall times resolve against IANA zones generically by enumerating the real
// instants that display as a given wall time; skipped walls resolve to none
// and repeated walls resolve to the earliest (first occurrence).
package clock

import (
	"fmt"
	"time"
)

// Hours is one weekday's opening interval, local HH:MM, never crossing midnight.
type Hours struct {
	Weekday string
	Opens   string
	Closes  string
}

// Rules carries everything the grid needs: zone, step, absolute duration,
// and per-weekday opening hours.
type Rules struct {
	Timezone        string
	SlotMinutes     int
	DurationMinutes int
	OpeningHours    []Hours
}

// Slot is one bookable wall-clock start with its resolved absolute bounds.
// End is absolute: Start plus the duration in real elapsed time.
type Slot struct {
	Local string
	Start time.Time
	End   time.Time
}

// Error is the package's only error shape; Code is one of validation_failed,
// invalid_local_time, not_on_slot_grid, outside_opening_hours.
type Error struct {
	Code string
}

func (e *Error) Error() string { return e.Code }

func fail(code string) *Error { return &Error{Code: code} }

// Overlap reports whether the half-open intervals [aStart,aEnd) and
// [bStart,bEnd) share any instant. Adjacent intervals do not overlap.
func Overlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// CutoffPassed reports whether now is at or past start minus cutoffMinutes.
// The boundary is inclusive: now exactly at start-cutoff has passed.
func CutoffPassed(now, start time.Time, cutoffMinutes int) bool {
	return !now.Before(start.Add(-time.Duration(cutoffMinutes) * time.Minute))
}

// ParseDate validates a strict YYYY-MM-DD calendar date.
func ParseDate(s string) (time.Time, error) {
	y, m, d, ok := splitDate(s)
	if !ok {
		return time.Time{}, fail("validation_failed")
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
}

// ResolveLocal resolves a strict bare local YYYY-MM-DDTHH:MM against an IANA
// zone, always choosing the first occurrence of a repeated wall time.
// A wall time that never occurs (spring-forward skip) is invalid_local_time;
// any malformed input or unknown zone is validation_failed.
func ResolveLocal(local, timezone string) (time.Time, error) {
	y, mo, d, hh, mm, ok := splitLocal(local)
	if !ok {
		return time.Time{}, fail("validation_failed")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fail("validation_failed")
	}
	// Wall time viewed as UTC; the true instant is wall minus its own offset.
	wall := time.Date(y, time.Month(mo), d, hh, mm, 0, 0, time.UTC)
	var first *time.Time
	for _, off := range offsetsAround(wall, loc) {
		u := wall.Add(-time.Duration(off) * time.Second)
		lt := u.In(loc)
		if lt.Year() == y && int(lt.Month()) == mo && lt.Day() == d &&
			lt.Hour() == hh && lt.Minute() == mm {
			c := u
			if first == nil || c.Before(*first) {
				first = &c
			}
		}
	}
	if first == nil {
		return time.Time{}, fail("invalid_local_time")
	}
	return first.In(loc), nil
}

// ValidateSlot resolves local against rules, checking in order: the wall time
// exists, the day is open and the start is inside it, the start sits on the
// slot grid measured from opening, and the absolute end finishes by closing.
func ValidateSlot(local string, rules Rules) (Slot, error) {
	_, slotMin, dur, hours, err := prepareRules(rules)
	if err != nil {
		return Slot{}, err
	}
	start, err := ResolveLocal(local, rules.Timezone)
	if err != nil {
		return Slot{}, err
	}
	y, mo, d, hh, mm, ok := splitLocal(local)
	if !ok {
		return Slot{}, fail("validation_failed")
	}
	day := time.Date(y, time.Month(mo), d, 12, 0, 0, 0, time.UTC)
	h, ok := hoursFor(weekdayName(day.Weekday()), hours)
	if !ok {
		return Slot{}, fail("outside_opening_hours")
	}
	openMin, _ := splitHM(h.Opens)
	closes, err := resolveClose(fmt.Sprintf("%04d-%02d-%02d", y, mo, d), h.Closes, rules.Timezone)
	if err != nil {
		return Slot{}, err
	}
	startMin := hh*60 + mm
	if startMin < openMin {
		return Slot{}, fail("outside_opening_hours")
	}
	if (startMin-openMin)%slotMin != 0 {
		return Slot{}, fail("not_on_slot_grid")
	}
	end := start.Add(time.Duration(dur) * time.Minute)
	if end.After(closes) {
		return Slot{}, fail("outside_opening_hours")
	}
	return Slot{Local: local, Start: start, End: end}, nil
}

// Slots produces every wall-grid slot for a local date: from opens in
// SlotMinutes steps, skipping nonexistent walls, keeping only slots whose
// absolute end finishes by closing. A closed day yields an empty slice.
func Slots(date string, rules Rules) ([]Slot, error) {
	y, m, d, ok := splitDate(date)
	if !ok {
		return nil, fail("validation_failed")
	}
	_, slotMin, dur, hours, err := prepareRules(rules)
	if err != nil {
		return nil, err
	}
	day := time.Date(y, time.Month(m), d, 12, 0, 0, 0, time.UTC)
	h, ok := hoursFor(weekdayName(day.Weekday()), hours)
	if !ok {
		return []Slot{}, nil
	}
	openMin, _ := splitHM(h.Opens)
	closeMin, _ := splitHM(h.Closes)
	closes, err := resolveClose(date, h.Closes, rules.Timezone)
	if err != nil {
		return nil, err
	}
	out := []Slot{}
	for wallMin := openMin; wallMin < closeMin; wallMin += slotMin {
		local := fmt.Sprintf("%sT%02d:%02d", date, wallMin/60, wallMin%60)
		start, err := ResolveLocal(local, rules.Timezone)
		if err != nil {
			if ce, ok := err.(*Error); ok && ce.Code == "invalid_local_time" {
				continue // spring-forward skip: never appears
			}
			return nil, err
		}
		end := start.Add(time.Duration(dur) * time.Minute)
		if end.After(closes) {
			break // absolute ends only grow with the wall clock
		}
		out = append(out, Slot{Local: local, Start: start, End: end})
	}
	return out, nil
}

// resolveClose resolves a closing wall time. A closes value landing in a
// spring-forward gap (e.g. 02:30 on a transition day) reads as the first
// existing instant at or after it, so day bounds never fail on real fixtures.
func resolveClose(date, hm, timezone string) (time.Time, error) {
	if t, err := ResolveLocal(date+"T"+hm, timezone); err == nil {
		return t, nil
	} else if ce, ok := err.(*Error); !ok || ce.Code != "invalid_local_time" {
		return time.Time{}, err
	}
	mins, _ := splitHM(hm)
	for m := mins + 1; m < 24*60; m++ {
		if t, err := ResolveLocal(fmt.Sprintf("%sT%02d:%02d", date, m/60, m%60), timezone); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fail("invalid_local_time")
}

// prepareRules validates the shared rule fields once per call.
func prepareRules(r Rules) (*time.Location, int, int, []Hours, error) {
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return nil, 0, 0, nil, fail("validation_failed")
	}
	if r.SlotMinutes <= 0 || r.DurationMinutes <= 0 {
		return nil, 0, 0, nil, fail("validation_failed")
	}
	seen := map[string]bool{}
	for _, h := range r.OpeningHours {
		if !validWeekday(h.Weekday) || seen[h.Weekday] {
			return nil, 0, 0, nil, fail("validation_failed")
		}
		seen[h.Weekday] = true
		o, ok1 := splitHM(h.Opens)
		c, ok2 := splitHM(h.Closes)
		if !ok1 || !ok2 || c <= o {
			return nil, 0, 0, nil, fail("validation_failed")
		}
	}
	return loc, r.SlotMinutes, r.DurationMinutes, r.OpeningHours, nil
}

// offsetsAround samples the distinct UTC offsets in effect near wall, viewed
// as UTC, so ResolveLocal can test every candidate real instant. Sampling
// both sides of any transition keeps this free of zone-specific branches.
func offsetsAround(wall time.Time, loc *time.Location) []int {
	seen := map[int]bool{}
	var out []int
	for t := wall.Add(-16 * time.Hour); t.Before(wall.Add(16 * time.Hour)); t = t.Add(5 * time.Minute) {
		_, off := t.In(loc).Zone()
		if !seen[off] {
			seen[off] = true
			out = append(out, off)
		}
	}
	return out
}

func hoursFor(wd string, hs []Hours) (Hours, bool) {
	for _, h := range hs {
		if h.Weekday == wd {
			return h, true
		}
	}
	return Hours{}, false
}

func weekdayName(wd time.Weekday) string {
	switch wd {
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

func validWeekday(s string) bool {
	switch s {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		return true
	}
	return false
}

func isDigits(s string) bool {
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func daysIn(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	}
	return 0
}

// splitDate parses strict YYYY-MM-DD with a real calendar day.
func splitDate(s string) (y, m, d int, ok bool) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return 0, 0, 0, false
	}
	for _, part := range []string{s[:4], s[5:7], s[8:10]} {
		if !isDigits(part) {
			return 0, 0, 0, false
		}
	}
	y = atoi4(s[:4])
	m = atoi2(s[5:7])
	d = atoi2(s[8:10])
	if m < 1 || m > 12 || d < 1 || d > daysIn(y, m) {
		return 0, 0, 0, false
	}
	return y, m, d, true
}

// splitHM parses strict HH:MM in 00:00..23:59.
func splitHM(s string) (mins int, ok bool) {
	if len(s) != 5 || s[2] != ':' || !isDigits(s[:2]) || !isDigits(s[3:5]) {
		return 0, false
	}
	h, m := atoi2(s[:2]), atoi2(s[3:5])
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// splitLocal parses strict bare YYYY-MM-DDTHH:MM with a real calendar day.
func splitLocal(s string) (y, mo, d, hh, mm int, ok bool) {
	if len(s) != 16 || s[10] != 'T' {
		return 0, 0, 0, 0, 0, false
	}
	var dok bool
	if y, mo, d, dok = splitDate(s[:10]); !dok {
		return 0, 0, 0, 0, 0, false
	}
	mins, hok := splitHM(s[11:16])
	if !hok {
		return 0, 0, 0, 0, 0, false
	}
	return y, mo, d, mins / 60, mins % 60, true
}

func atoi2(s string) int { return int(s[0]-'0')*10 + int(s[1]-'0') }

func atoi4(s string) int {
	return int(s[0]-'0')*1000 + int(s[1]-'0')*100 + int(s[2]-'0')*10 + int(s[3]-'0')
}
