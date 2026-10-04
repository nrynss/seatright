package service

import (
	"fmt"
	"time"
)

// This file holds fixture parsing/validation and seed reservation time
// normalization. The time helper below is a TEMPORARY stdlib implementation
// for seeded reservations only; reservation write paths (S1-C) will use the
// frozen clock contract, which this helper already mirrors (first-occurrence
// resolution, absolute durations).

var weekdays = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true,
	"fri": true, "sat": true, "sun": true,
}

// parseBareLocal validates a bare local YYYY-MM-DDTHH:MM string and returns
// its components.
func parseBareLocal(s string) (y, mo, d, h, mi int, err error) {
	if len(s) != 16 || s[10] != 'T' {
		return 0, 0, 0, 0, 0, fmt.Errorf("not bare local")
	}
	parsed, perr := time.Parse("2006-01-02T15:04", s)
	if perr != nil {
		return 0, 0, 0, 0, 0, perr
	}
	// time.Parse normalizes overflows (e.g. month 13); reject those.
	if parsed.Format("2006-01-02T15:04") != s {
		return 0, 0, 0, 0, 0, fmt.Errorf("nonexistent calendar value")
	}
	return parsed.Year(), int(parsed.Month()), parsed.Day(), parsed.Hour(), parsed.Minute(), nil
}

// parseHHMM validates a local HH:MM string.
func parseHHMM(s string) (h, mi int, err error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, fmt.Errorf("not HH:MM")
	}
	parsed, perr := time.Parse("15:04", s)
	if perr != nil {
		return 0, 0, perr
	}
	if parsed.Format("15:04") != s {
		return 0, 0, fmt.Errorf("bad clock value")
	}
	return parsed.Hour(), parsed.Minute(), nil
}

// resolveSeedInstant resolves a bare local wall time in tz to its first
// occurrence instant (repeated fall-back times resolve to the earlier
// occurrence) and returns start and end (start + durationMinutes of absolute
// time). Skipped spring-forward times resolve forward with the
// post-transition offset. An error is returned only for malformed input or an
// unknown zone.
func resolveSeedInstant(local, tz string, durationMinutes int) (start, end time.Time, err error) {
	y, mo, d, h, mi, err := parseBareLocal(local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	wall := time.Date(y, time.Month(mo), d, h, mi, 0, 0, time.UTC)
	want := wall.Format("2006-01-02T15:04")
	offsets := map[int]bool{}
	for _, delta := range []time.Duration{-14, -8, -3, -1, 0, 1, 3, 8, 14} {
		_, off := wall.Add(delta * time.Hour).In(loc).Zone()
		offsets[off] = true
	}
	var first *time.Time
	for off := range offsets {
		cand := wall.Add(-time.Duration(off) * time.Second)
		if cand.In(loc).Format("2006-01-02T15:04") != want {
			continue
		}
		if first == nil || cand.Before(*first) {
			c := cand
			first = &c
		}
	}
	if first == nil {
		// Skipped local time: resolve forward like the runtime does.
		fwd := time.Date(y, time.Month(mo), d, h, mi, 0, 0, loc)
		first = &fwd
	}
	// Render in the restaurant's zone so stored timestamps carry the local
	// offset; the instant is what matters for overlap and cutoff checks.
	start = first.In(loc)
	end = start.Add(time.Duration(durationMinutes) * time.Minute)
	return start, end, nil
}

// fixtureInt interprets a decoded JSON value as an integer. Strings, booleans
// and null are the wrong JSON type; non-integral numbers are numbers but not
// integers (an invalid value, not a malformed body).
func fixtureInt(v any) (n int, isNumber bool, isInteger bool) {
	f, ok := v.(float64)
	if !ok {
		return 0, false, false
	}
	if f != float64(int(f)) {
		return 0, true, false
	}
	return int(f), true, true
}
