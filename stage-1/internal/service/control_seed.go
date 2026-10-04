package service

import (
	"fmt"
	"time"
)

// This file holds fixture parsing/validation helpers. Seed reservation
// times resolve through the frozen clock contract (clock.ResolveLocal);
// reservation write paths use the same contract, so seeds and writes agree
// on first-occurrence resolution and absolute durations.

var weekdays = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true,
	"fri": true, "sat": true, "sun": true,
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
