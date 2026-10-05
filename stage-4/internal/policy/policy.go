// Package policy is the pure stage-3 booking-policy engine for Tablekeeper.
//
// It owns complete-policy parsing, effective-date selection, snapshot cloning,
// clock-grid delegation and selected-set capacity. It never locks, allocates
// opaque ids, publishes policies, reads the system clock or touches service
// state. Fixture policy0 terms may carry original stage-1 values outside the
// new-publication maxima; Select/Clone/Rules/Capacity never reapply the
// publication restrictions to already-selected terms.
package policy

import (
	"tablekeeper/internal/clock"
)

// Hours is one weekday's opening interval, local HH:MM, never crossing midnight.
type Hours struct {
	Weekday string `json:"weekday"`
	Opens   string `json:"opens"`
	Closes  string `json:"closes"`
}

// Terms is a complete snapshot of booking rules. Capacities maps every table
// id of the restaurant to its integer capacity.
type Terms struct {
	PolicyVersion              int            `json:"policy_version"`
	SlotMinutes                int            `json:"slot_minutes"`
	ReservationDurationMinutes int            `json:"reservation_duration_minutes"`
	CancellationCutoffMinutes  int            `json:"cancellation_cutoff_minutes"`
	OpeningHours               []Hours        `json:"opening_hours"`
	Capacities                 map[string]int `json:"capacities"`
}

// Policy is a dated published policy. Terms is embedded anonymously so the
// JSON encoding stays flat: there is no nested "terms" property.
type Policy struct {
	EffectiveFrom string `json:"effective_from"`
	Terms
}

// Error is the package's only error shape. Every Parse/Select error carries
// Code "validation_failed"; the future HTTP integration maps it to 422.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func fail(msg string) *Error { return &Error{Code: "validation_failed", Message: msg} }

// Parse validates a complete strict documented policy from its decoded JSON
// object. obj holds ordinary decoded values (maps, []any, float64, string,
// bool). tableIDs are the restaurant's known fixture table ids; capacities
// must name exactly that membership. All documented fields are required;
// unknown fields are ignored; policy_version is allocated by the future
// service and never set by the caller (defaults 0). Every failure returns
// Code "validation_failed". Invalid JSON text itself remains the HTTP
// ParseBody responsibility and never reaches here.
func Parse(obj map[string]any, tableIDs []string) (Policy, error) {
	var p Policy
	if obj == nil {
		return p, fail("policy must be an object")
	}
	effRaw, ok := obj["effective_from"]
	if !ok {
		return p, fail("effective_from is required")
	}
	eff, ok := effRaw.(string)
	if !ok || !validDate(eff) {
		return p, fail("effective_from must be a real YYYY-MM-DD date")
	}
	slot, err := policyInt(obj, "slot_minutes", 1, 1440)
	if err != nil {
		return p, err
	}
	dur, err := policyInt(obj, "reservation_duration_minutes", 1, 1440)
	if err != nil {
		return p, err
	}
	cut, err := policyInt(obj, "cancellation_cutoff_minutes", 0, 10080)
	if err != nil {
		return p, err
	}
	hoursRaw, ok := obj["opening_hours"]
	if !ok {
		return p, fail("opening_hours is required")
	}
	hoursList, ok := hoursRaw.([]any)
	if !ok {
		return p, fail("opening_hours must be an array")
	}
	hours := make([]Hours, 0, len(hoursList))
	seen := map[string]bool{}
	for _, h := range hoursList {
		hm, ok := h.(map[string]any)
		if !ok {
			return p, fail("opening_hours entry must be an object")
		}
		wd, o, c, ok := hoursFields(hm)
		if !ok {
			return p, fail("opening_hours entry needs weekday/opens/closes strings")
		}
		if !validWeekday(wd) || seen[wd] {
			return p, fail("opening_hours has an invalid or duplicate weekday")
		}
		seen[wd] = true
		om, ok1 := splitHM(o)
		cm, ok2 := splitHM(c)
		if !ok1 || !ok2 || cm <= om {
			return p, fail("opening_hours interval must satisfy opens < closes")
		}
		hours = append(hours, Hours{Weekday: wd, Opens: o, Closes: c})
	}
	capRaw, ok := obj["capacities"]
	if !ok {
		return p, fail("capacities is required")
	}
	capMap, ok := capRaw.(map[string]any)
	if !ok {
		return p, fail("capacities must be an object")
	}
	if len(capMap) != len(tableIDs) {
		return p, fail("capacities must name exactly the restaurant tables")
	}
	caps := make(map[string]int, len(capMap))
	known := map[string]bool{}
	for _, id := range tableIDs {
		known[id] = true
	}
	for id, v := range capMap {
		if !known[id] {
			return p, fail("capacities names an unknown table")
		}
		n, ok := intValue(v)
		if !ok || n < 1 || n > 100 {
			return p, fail("capacity must be an integer 1..100")
		}
		caps[id] = n
	}
	p.EffectiveFrom = eff
	p.Terms = Terms{
		PolicyVersion:              0,
		SlotMinutes:                slot,
		ReservationDurationMinutes: dur,
		CancellationCutoffMinutes:  cut,
		OpeningHours:               hours,
		Capacities:                 caps,
	}
	return p, nil
}

// policyInt reads a required integer field: JSON numbers only (float64 with
// integral value), never booleans or strings, within [lo,hi].
func policyInt(obj map[string]any, field string, lo, hi int) (int, error) {
	raw, ok := obj[field]
	if !ok {
		return 0, fail(field + " is required")
	}
	n, ok := intValue(raw)
	if !ok || n < lo || n > hi {
		return 0, fail(field + " out of range")
	}
	return n, nil
}

// intValue accepts only JSON numbers with integral values. Booleans,
// strings and fractional values are rejected.
func intValue(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	n := int(f)
	if float64(n) != f {
		return 0, false
	}
	return n, true
}

func hoursFields(hm map[string]any) (wd, o, c string, ok bool) {
	wdr, okr := hm["weekday"]
	opr, oko := hm["opens"]
	clr, okc := hm["closes"]
	if !okr || !oko || !okc {
		return "", "", "", false
	}
	wds, ok1 := wdr.(string)
	ops, ok2 := opr.(string)
	cls, ok3 := clr.(string)
	if !ok1 || !ok2 || !ok3 {
		return "", "", "", false
	}
	return wds, ops, cls, true
}

func validWeekday(w string) bool {
	switch w {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		return true
	}
	return false
}

// splitHM parses a strict HH:MM 24-hour wall time.
func splitHM(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' ||
		s[3] < '0' || s[3] > '9' || s[4] < '0' || s[4] > '9' {
		return 0, false
	}
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// validDate reports whether s is a real strict YYYY-MM-DD calendar date,
// including leap-day validity.
func validDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	y, m, d := 0, 0, 0
	for i, c := range s {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		switch {
		case i < 4:
			y = y*10 + int(c-'0')
		case i < 7:
			m = m*10 + int(c-'0')
		default:
			d = d*10 + int(c-'0')
		}
	}
	if m < 1 || m > 12 || d < 1 {
		return false
	}
	dim := daysIn(y, m)
	return d <= dim
}

func daysIn(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// Select chooses the effective terms for a booking's local start date: the
// eligible published policy with the greatest effective_from not later than
// date wins; ties choose the greatest policy_version. Publication order is
// irrelevant. With no eligible policy it returns a deep copy of base
// (fixture policy0). date must be a real strict YYYY-MM-DD date. The result
// is a deep snapshot; inputs are never mutated. Versions are never
// allocated here. Fixture terms are used as-is without reapplying the
// publication maxima.
func Select(base Terms, published []Policy, date string) (Terms, error) {
	if !validDate(date) {
		return Terms{}, fail("date must be a real YYYY-MM-DD date")
	}
	best := -1
	for i, p := range published {
		if !validDate(p.EffectiveFrom) || p.EffectiveFrom > date {
			continue
		}
		if best < 0 || p.EffectiveFrom > published[best].EffectiveFrom ||
			(p.EffectiveFrom == published[best].EffectiveFrom &&
				p.PolicyVersion > published[best].PolicyVersion) {
			best = i
		}
	}
	if best < 0 {
		return CloneTerms(base), nil
	}
	return CloneTerms(published[best].Terms), nil
}

// CloneTerms deep-copies hours and the capacities map, preserving nil vs
// non-nil empty shape: a non-nil empty OpeningHours still encodes as []
// rather than null, keeping the complete policy snapshot byte-stable.
func CloneTerms(t Terms) Terms {
	out := t
	if t.OpeningHours != nil {
		out.OpeningHours = append([]Hours{}, t.OpeningHours...)
	}
	if t.Capacities != nil {
		out.Capacities = make(map[string]int, len(t.Capacities))
		for k, v := range t.Capacities {
			out.Capacities[k] = v
		}
	}
	return out
}

// ClonePolicy deep-copies a published policy's terms.
func ClonePolicy(p Policy) Policy {
	out := p
	out.Terms = CloneTerms(p.Terms)
	return out
}

// Rules converts selected terms to the generic clock grid rules for a zone.
// Timezone handling (including DST transitions) stays entirely in clock.
func Rules(t Terms, timezone string) clock.Rules {
	hours := make([]clock.Hours, 0, len(t.OpeningHours))
	for _, h := range t.OpeningHours {
		hours = append(hours, clock.Hours{Weekday: h.Weekday, Opens: h.Opens, Closes: h.Closes})
	}
	return clock.Rules{
		Timezone:        timezone,
		SlotMinutes:     t.SlotMinutes,
		DurationMinutes: t.ReservationDurationMinutes,
		OpeningHours:    hours,
	}
}

// Capacity sums exactly the selected policy's capacities for the given table
// ids. Selection and canonical pair legality stay with the existing service
// seating helpers; unknown ids contribute nothing here but callers only pass
// validated selected sets.
func Capacity(t Terms, tableIDs []string) int {
	sum := 0
	for _, id := range tableIDs {
		sum += t.Capacities[id]
	}
	return sum
}
