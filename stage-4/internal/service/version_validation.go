package service

import (
	"encoding/json"
	"errors"
	"time"

	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// This file owns exhaustive modern stored-state consistency for import:
// policy, history, series and counter coherence on top of the base identity,
// configuration, instant and receipt checks. validateVersionState is pure
// and read-only: it never mutates state, rewrites receipts, histories,
// terms, policies, series or counters. It runs offside after normalization
// and base validation, before import replacement.
//
// Producer contracts honored here:
//   - Published policy versions follow Parse constraints, but fixture
//     policy0 and accepted historical terms may exceed publication maxima
//     (1441/1441/10081/cap101, long durations) and import unchanged.
//   - Nil/empty hours/capacities shapes match the current producer and old
//     normalization (nil and empty are both transmissible; what matters is
//     internal consistency, not a mandated shape).
//   - Already-cancelled legacy records carry revision1 with ONE reconstructed
//     CREATED entry at retained created_at; no Cancelled entry is fabricated
//     and none is required.
//   - Seeds may be past, off-grid, outside hours/end-by-close, over capacity
//     or overlapping: no booking business rules are applied here.
//   - Old receipt bytes stay opaque and are never compared to current records.

// validateVersionState checks modern stored-state consistency. Every failure
// is a plain error surfaced as 422 validation_failed without mutating the
// destination.
func validateVersionState(st *State) error {
	if err := validateVersionPolicies(st); err != nil {
		return err
	}
	if err := validateVersionHistories(st); err != nil {
		return err
	}
	if err := validateVersionSeries(st); err != nil {
		return err
	}
	if err := validateVersionCounters(st); err != nil {
		return err
	}
	return nil
}

// versionRestaurantIDs returns the set of known restaurant ids.
func versionRestaurantIDs(st *State) map[string]bool {
	ids := make(map[string]bool, len(st.Restaurants))
	for _, r := range st.Restaurants {
		ids[r.ID] = true
	}
	return ids
}

// versionRestaurantTables maps restaurant id to its fixture table ids.
func versionRestaurantTables(st *State) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(st.Restaurants))
	for _, r := range st.Restaurants {
		tabs := make(map[string]bool, len(r.Tables))
		for _, t := range r.Tables {
			tabs[t.ID] = true
		}
		out[r.ID] = tabs
	}
	return out
}

// validateVersionPolicies checks published policies: known restaurant,
// positive versions following Parse constraints (exact capacities
// membership, weekday/hours rules, integer ranges), publication order kept
// as stored, and accepted terms bound to a stored version when applicable.
// Fixture policy0 and historical accepted terms may exceed publication
// maxima and are never re-restricted here.
func validateVersionPolicies(st *State) error {
	restIDs := versionRestaurantIDs(st)
	tables := versionRestaurantTables(st)
	for rid, pubs := range st.Policies {
		if !restIDs[rid] {
			return errors.New("policies for unknown restaurant")
		}
		for i, p := range pubs {
			// Publication order allocates versions 1..N in append order
			// (independent of effective-date order).
			if p.PolicyVersion != i+1 {
				return errors.New("published policy versions must be 1..N in publication order")
			}
			if !validVersionDate(p.EffectiveFrom) {
				return errors.New("published policy date invalid")
			}
			if err := checkPublishedTerms(p.Terms, tables[rid]); err != nil {
				return err
			}
		}
	}
	for ref, res := range st.Reservations {
		if res.Revision < 1 {
			return errors.New("reservation revision invalid")
		}
		terms := res.AcceptedTerms
		if terms.PolicyVersion < 0 {
			return errors.New("accepted policy version invalid")
		}
		if terms.PolicyVersion == 0 {
			continue
		}
		found := false
		for _, p := range st.Policies[res.RestaurantID] {
			if p.PolicyVersion == terms.PolicyVersion {
				if !termsEqualForBinding(terms, p.Terms) {
					return errors.New("accepted terms do not match stored policy")
				}
				found = true
				break
			}
		}
		if !found {
			return errors.New("accepted terms bind to no stored policy")
		}
		_ = ref
	}
	return nil
}

// checkPublishedTerms enforces Parse-level constraints on a stored published
// policy's terms: integer ranges, weekday/hours rules and exact capacities
// membership for the restaurant's tables.
func checkPublishedTerms(t policy.Terms, tables map[string]bool) error {
	if t.SlotMinutes < 1 || t.SlotMinutes > 1440 {
		return errors.New("published slot_minutes out of range")
	}
	if t.ReservationDurationMinutes < 1 || t.ReservationDurationMinutes > 1440 {
		return errors.New("published duration out of range")
	}
	if t.CancellationCutoffMinutes < 0 || t.CancellationCutoffMinutes > 10080 {
		return errors.New("published cutoff out of range")
	}
	seen := map[string]bool{}
	for _, h := range t.OpeningHours {
		if !validVersionWeekday(h.Weekday) || seen[h.Weekday] {
			return errors.New("published opening_hours weekday invalid")
		}
		seen[h.Weekday] = true
		om, ok1 := splitVersionHM(h.Opens)
		cm, ok2 := splitVersionHM(h.Closes)
		if !ok1 || !ok2 || cm <= om {
			return errors.New("published opening_hours interval invalid")
		}
	}
	if len(t.Capacities) != len(tables) {
		return errors.New("published capacities membership invalid")
	}
	for id, c := range t.Capacities {
		if !tables[id] {
			return errors.New("published capacities name unknown table")
		}
		if c < 1 || c > 100 {
			return errors.New("published capacity out of range")
		}
	}
	return nil
}

// termsEqualForBinding compares accepted terms against their stored policy:
// all selected fields must match (policy_version, grid, duration, cutoff,
// hours, capacities). Effective_from is never stored in terms.
func termsEqualForBinding(a, b policy.Terms) bool {
	if a.PolicyVersion != b.PolicyVersion || a.SlotMinutes != b.SlotMinutes ||
		a.ReservationDurationMinutes != b.ReservationDurationMinutes ||
		a.CancellationCutoffMinutes != b.CancellationCutoffMinutes {
		return false
	}
	if len(a.OpeningHours) != len(b.OpeningHours) {
		return false
	}
	for i := range a.OpeningHours {
		if a.OpeningHours[i] != b.OpeningHours[i] {
			return false
		}
	}
	if len(a.Capacities) != len(b.Capacities) {
		return false
	}
	for k, v := range a.Capacities {
		if bv, ok := b.Capacities[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func validVersionWeekday(w string) bool {
	switch w {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		return true
	}
	return false
}

func splitVersionHM(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func validVersionDate(s string) bool {
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
	return d <= versionDaysIn(y, m)
}

func versionDaysIn(y, m int) int {
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

// validateVersionHistories checks history coherence: every entry belongs to
// a stored reservation (no orphans), seq starts at 1 and increments by one,
// events are known, instants are RFC3339 numeric-offset and nondecreasing,
// changed entries name only real ordered fields, retained terms snapshots
// match their own entry version (never the current record), and the final
// entry's revision/terms agree with the stored record. A lone CREATED entry
// on a cancelled revision1 record is the legitimate reconstructed legacy
// shape and is accepted.
func validateVersionHistories(st *State) error {
	for ref, entries := range st.Histories {
		res, ok := st.Reservations[ref]
		if !ok {
			return errors.New("history for unknown reservation")
		}
		if len(entries) == 0 {
			return errors.New("empty reservation history")
		}
		var prev time.Time
		var running *versionSnapshot
		for i, e := range entries {
			if e.Seq != i+1 {
				return errors.New("history seq must increment by one")
			}
			if i == 0 && e.Event != history.EventCreated {
				return errors.New("first history event must be created")
			}
			if i > 0 && entries[i-1].Event == history.EventCancelled {
				return errors.New("entry after cancellation")
			}
			switch e.Event {
			case history.EventCreated, history.EventChanged, history.EventCancelled, history.EventReassigned:
			default:
				return errors.New("history event unknown")
			}
			at, err := time.Parse(time.RFC3339, e.At)
			if err != nil {
				return errors.New("history instant invalid")
			}
			if i > 0 && at.Before(prev) {
				return errors.New("history instants decreasing")
			}
			prev = at
			if e.Revision != i+1 {
				return errors.New("history revision must increment by one")
			}
			if err := checkHistoryTerms(e.AcceptedTerms, e.Revision, st, res); err != nil {
				return err
			}
			var next versionSnapshot
			var prevTerms []byte
			if i > 0 {
				prevTerms = entries[i-1].AcceptedTerms
			}
			if err := checkHistoryTransition(e, running, &next, st, res, prevTerms); err != nil {
				return err
			}
			running = &next
			if e.Event == history.EventCancelled && i != len(entries)-1 {
				return errors.New("entry after cancellation")
			}
		}
		last := entries[len(entries)-1]
		if last.Revision != res.Revision {
			return errors.New("history final revision diverges from record")
		}
		if !termsJSONEqual(last.AcceptedTerms, res.AcceptedTerms) {
			return errors.New("history final terms diverge from record")
		}
		if !running.matchesRecord(res) {
			return errors.New("history replay diverges from record")
		}
		// Status/final-event agreement: a Cancelled final requires a
		// cancelled record; a confirmed record must not end cancelled.
		// A cancelled record with only Changed entries is rejected — only
		// the genuine already-cancelled seed/legacy shape (rev1, ONE
		// CREATED entry) is accepted without a Cancelled final.
		if last.Event == history.EventCancelled && res.Status != StatusCancelled {
			return errors.New("cancelled history on confirmed record")
		}
		if res.Status == StatusCancelled && last.Event != history.EventCancelled {
			if !(res.Revision == 1 && len(entries) == 1 && entries[0].Event == history.EventCreated) {
				return errors.New("cancelled record without cancelled history")
			}
		}
		if last.Event == history.EventCancelled && entries[len(entries)-1].Changes == nil {
			return errors.New("cancelled changes must be allocated")
		}
	}
	for ref, res := range st.Reservations {
		if len(st.Histories[ref]) == 0 {
			return errors.New("reservation without history")
		}
		_ = res
	}
	return nil
}

// versionSnapshot is the running reservation state reconstructed from
// history: canonical table set, local start, party and revision.
type versionSnapshot struct {
	tableIDs      []string
	startsAtLocal string
	partySize     int
	revision      int
}

// checkHistoricalSelector validates a historical table set against the
// restaurant config: known ids, no duplicates, 1..2 cardinality, declared
// pair membership and declared canonical order. Singleton==scalar shape is
// enforced by the caller via the scalar flag.
func checkHistoricalSelector(st *State, res Reservation, ids []string, scalar bool) error {
	if len(ids) == 0 || len(ids) > 2 {
		return errors.New("historical table set invalid")
	}
	seen := map[string]bool{}
	var restaurant *Restaurant
	for i := range st.Restaurants {
		if st.Restaurants[i].ID == res.RestaurantID {
			restaurant = &st.Restaurants[i]
			break
		}
	}
	if restaurant == nil {
		return errors.New("historical restaurant unknown")
	}
	known := map[string]bool{}
	for _, t := range restaurant.Tables {
		known[t.ID] = true
	}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return errors.New("historical table unknown or duplicate")
		}
		seen[id] = true
	}
	if len(ids) == 1 {
		if !scalar {
			return errors.New("historical singleton must use table_id")
		}
		return nil
	}
	if scalar {
		return errors.New("historical pair must use table_ids")
	}
	if !pairDeclared(st, res.RestaurantID, ids) {
		return errors.New("historical pair not declared")
	}
	if !isCanonicalPairOrder(st, res.RestaurantID, ids) {
		return errors.New("historical pair order invalid")
	}
	return nil
}

// validVersionLocal reports bare YYYY-MM-DDTHH:MM syntax with a real date
// and 00:00..23:59 clock. No grid/availability/capacity/past/cutoff business
// rules are applied to historical values.
func validVersionLocal(local string) bool {
	y, mo, d, hh, mm, ok := splitVersionLocalParts(local)
	if !ok {
		return false
	}
	if mo < 1 || mo > 12 || d < 1 || d > versionDaysIn(y, mo) {
		return false
	}
	return hh <= 23 && mm <= 59
}

// splitVersionLocalParts parses a strict bare local wall time.
func splitVersionLocalParts(local string) (y, mo, d, hh, mm int, ok bool) {
	if len(local) != 16 || local[4] != '-' || local[7] != '-' || local[10] != 'T' || local[13] != ':' {
		return 0, 0, 0, 0, 0, false
	}
	for i, c := range local {
		if i == 4 || i == 7 || i == 10 || i == 13 {
			continue
		}
		if c < '0' || c > '9' {
			return 0, 0, 0, 0, 0, false
		}
		switch {
		case i < 4:
			y = y*10 + int(c-'0')
		case i < 7:
			mo = mo*10 + int(c-'0')
		case i < 10:
			d = d*10 + int(c-'0')
		case i < 13:
			hh = hh*10 + int(c-'0')
		default:
			mm = mm*10 + int(c-'0')
		}
	}
	return y, mo, d, hh, mm, true
}

// matchesRecord reports whether the replayed running snapshot equals the
// stored record's selection, time and party.
func (v *versionSnapshot) matchesRecord(res Reservation) bool {
	if v == nil {
		return false
	}
	if !versionSetsEqual(v.tableIDs, reservationTableIDs(res)) {
		return false
	}
	return v.startsAtLocal == res.StartsAtLocal && v.partySize == res.PartySize
}

// checkHistoryTerms validates a frozen terms snapshot: six flat known fields
// with usable types/values, no effective_from, bound to its OWN recorded
// policy version (fixture0 exempt from publication maxima and ranges).
// Historical values are checked for coherence, never against newest policy.
func checkHistoryTerms(raw json.RawMessage, revision int, st *State, res Reservation) error {
	if len(raw) == 0 {
		return errors.New("history terms missing")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return errors.New("history terms invalid")
	}
	want := map[string]bool{"policy_version": true, "slot_minutes": true, "reservation_duration_minutes": true, "cancellation_cutoff_minutes": true, "opening_hours": true, "capacities": true}
	for k := range m {
		if !want[k] {
			return errors.New("history terms carry extra field")
		}
	}
	for k := range want {
		if _, ok := m[k]; !ok {
			return errors.New("history terms incomplete")
		}
	}
	ver, ok := m["policy_version"].(float64)
	if !ok || ver != float64(int(ver)) || int(ver) < 0 {
		return errors.New("history policy version invalid")
	}
	slot, ok := versionNumField(m, "slot_minutes")
	if !ok || slot <= 0 {
		return errors.New("history slot_minutes invalid")
	}
	dur, ok := versionNumField(m, "reservation_duration_minutes")
	if !ok || dur <= 0 {
		return errors.New("history duration invalid")
	}
	cut, ok := versionNumField(m, "cancellation_cutoff_minutes")
	if !ok || cut < 0 {
		return errors.New("history cutoff invalid")
	}
	caps, ok := m["capacities"].(map[string]any)
	if !ok {
		return errors.New("history capacities invalid")
	}
	for _, c := range caps {
		f, ok := c.(float64)
		if !ok || f != float64(int(f)) || int(f) < 1 {
			return errors.New("history capacity invalid")
		}
	}
	hours, ok := m["opening_hours"].([]any)
	if !ok {
		return errors.New("history opening_hours invalid")
	}
	seen := map[string]bool{}
	for _, h := range hours {
		hm, ok := h.(map[string]any)
		if !ok {
			return errors.New("history opening_hours invalid")
		}
		wd, _ := hm["weekday"].(string)
		op, _ := hm["opens"].(string)
		cl, _ := hm["closes"].(string)
		if !validVersionWeekday(wd) || seen[wd] {
			return errors.New("history opening_hours weekday invalid")
		}
		seen[wd] = true
		om, ok1 := splitVersionHM(op)
		cm, ok2 := splitVersionHM(cl)
		if !ok1 || !ok2 || cm <= om {
			return errors.New("history opening_hours interval invalid")
		}
	}
	if int(ver) == 0 {
		want := fixtureTerms(restaurantForTerms(st, res))
		if !historyTermsMatchFixture(m, want) {
			return errors.New("history fixture terms diverge from original config")
		}
		return nil
	}
	for _, q := range st.Policies[res.RestaurantID] {
		if q.PolicyVersion == int(ver) {
			if !historyTermsMatchPolicy(m, q) {
				return errors.New("history terms diverge from stored policy")
			}
			return nil
		}
	}
	return errors.New("history terms bind to no stored policy")
}

// restaurantForTerms finds the stored restaurant for terms binding. The
// caller guarantees restaurant existence via base validation.
func restaurantForTerms(st *State, res Reservation) Restaurant {
	for _, r := range st.Restaurants {
		if r.ID == res.RestaurantID {
			return r
		}
	}
	return Restaurant{}
}

// historyTermsMatchFixture compares a v0 snapshot map against the original
// fixture terms: grid, duration, cutoff, hours and every table capacity.
func historyTermsMatchFixture(m map[string]any, want policy.Terms) bool {
	if int(m["slot_minutes"].(float64)) != want.SlotMinutes ||
		int(m["reservation_duration_minutes"].(float64)) != want.ReservationDurationMinutes ||
		int(m["cancellation_cutoff_minutes"].(float64)) != want.CancellationCutoffMinutes {
		return false
	}
	qh, _ := json.Marshal(want.OpeningHours)
	var qhm any
	json.Unmarshal(qh, &qhm)
	qhr, _ := json.Marshal(qhm)
	mhr, _ := json.Marshal(m["opening_hours"])
	if string(qhr) != string(mhr) {
		return false
	}
	qc, _ := json.Marshal(want.Capacities)
	var qcm any
	json.Unmarshal(qc, &qcm)
	qcr, _ := json.Marshal(qcm)
	mcr, _ := json.Marshal(m["capacities"])
	return string(qcr) == string(mcr)
}

// numField reads a JSON number field.
func versionNumField(m map[string]any, k string) (int, bool) {
	f, ok := m[k].(float64)
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

// historyTermsMatchPolicy compares a frozen snapshot map against a stored
// published policy's complete terms.
func historyTermsMatchPolicy(m map[string]any, q policy.Policy) bool {
	if int(m["policy_version"].(float64)) != q.PolicyVersion ||
		int(m["slot_minutes"].(float64)) != q.SlotMinutes ||
		int(m["reservation_duration_minutes"].(float64)) != q.ReservationDurationMinutes ||
		int(m["cancellation_cutoff_minutes"].(float64)) != q.CancellationCutoffMinutes {
		return false
	}
	qh, _ := json.Marshal(q.OpeningHours)
	var qhm any
	json.Unmarshal(qh, &qhm)
	qhr, _ := json.Marshal(qhm)
	mhr, _ := json.Marshal(m["opening_hours"])
	if string(qhr) != string(mhr) {
		return false
	}
	qc, _ := json.Marshal(q.Capacities)
	var qcm any
	json.Unmarshal(qc, &qcm)
	qcr, _ := json.Marshal(qcm)
	mcr, _ := json.Marshal(m["capacities"])
	return string(qcr) == string(mcr)
}

// checkHistoryTransition validates one entry against the running snapshot and
// advances it. The first (CREATED) entry establishes the snapshot from its
// own To values — never from the current record, so a singleton→pair history
// stays valid after amendment. Later entries must transition from actual
// previous values with real changes only, ordered selection/time/party
// fields, coherent From/To types and values, and matching revision.
func checkHistoryTransition(e history.Entry, prev *versionSnapshot, next *versionSnapshot, st *State, res Reservation, prevTerms []byte) error {
	switch e.Event {
	case history.EventCreated:
		if prev != nil {
			return errors.New("created entry must be first")
		}
		if e.PlanID != "" {
			return errors.New("plan id on non-repair entry")
		}
		if len(e.Changes) != 3 {
			return errors.New("created entry must name three fields")
		}
		sel := e.Changes[0]
		var ids []string
		var scalar bool
		switch sel.Field {
		case "table_id":
			to, ok := sel.To.(string)
			if !ok || to == "" || sel.From != nil {
				return errors.New("created table value invalid")
			}
			ids = []string{to}
			scalar = true
		case "table_ids":
			arr, ok := versionStringList(sel.To)
			if !ok || len(arr) != 2 || sel.From != nil {
				return errors.New("created table value invalid")
			}
			for _, id := range arr {
				if id == "" {
					return errors.New("created table value invalid")
				}
				ids = append(ids, id)
			}
			scalar = false
		default:
			return errors.New("created fields misordered")
		}
		if e.Changes[1].Field != "starts_at_local" || e.Changes[2].Field != "party_size" {
			return errors.New("created fields misordered")
		}
		loc, ok := e.Changes[1].To.(string)
		if !ok || loc == "" || e.Changes[1].From != nil {
			return errors.New("created time value invalid")
		}
		party, ok := versionNumAny(e.Changes[2].To)
		if !ok || party < 1 || e.Changes[2].From != nil {
			return errors.New("created party value invalid")
		}
		if err := checkHistoricalSelector(st, res, ids, scalar); err != nil {
			return err
		}
		if !validVersionLocal(loc) {
			return errors.New("created time value invalid")
		}
		*next = versionSnapshot{tableIDs: ids, startsAtLocal: loc, partySize: party, revision: e.Revision}
		return nil
	case history.EventChanged:
		if prev == nil {
			return errors.New("changed entry without creation")
		}
		if e.PlanID != "" {
			return errors.New("plan id on non-repair entry")
		}
		if len(e.Changes) == 0 || len(e.Changes) > 3 {
			return errors.New("changed entry fields invalid")
		}
		order := map[string]int{"table_id": 0, "table_ids": 0, "starts_at_local": 1, "party_size": 2}
		last := -1
		seen := map[string]bool{}
		cur := *prev
		real := false
		for _, c := range e.Changes {
			o, ok := order[c.Field]
			if !ok || seen[c.Field] || o <= last {
				return errors.New("changed fields misordered")
			}
			seen[c.Field] = true
			last = o
			switch c.Field {
			case "table_id":
				from, fok := c.From.(string)
				to, tok := c.To.(string)
				if !fok || !tok || from == "" || to == "" || from == to {
					return errors.New("changed table values invalid")
				}
				if len(cur.tableIDs) != 1 || cur.tableIDs[0] != from {
					return errors.New("changed table from diverges")
				}
				cur.tableIDs = []string{to}
				if err := checkHistoricalSelector(st, res, cur.tableIDs, true); err != nil {
					return err
				}
				real = true
			case "table_ids":
				from, fok := versionStringList(c.From)
				to, tok := versionStringList(c.To)
				if !fok || !tok || len(from) == 0 || len(from) > 2 || len(to) == 0 || len(to) > 2 ||
					(len(from) == 1 && len(to) == 1) || versionSetsEqual(from, to) {
					return errors.New("changed table values invalid")
				}
				if !versionSetsEqual(from, cur.tableIDs) {
					return errors.New("changed table from diverges")
				}
				// The recorded From must itself be a canonical legal set:
				// singleton From is fine, but a pair From must be declared
				// and in canonical order (reversed/illegal arrays rejected).
				if err := checkHistoricalSelector(st, res, from, len(from) == 1); err != nil {
					return errors.New("changed table from invalid")
				}
				cur.tableIDs = append([]string(nil), to...)
				if len(cur.tableIDs) == 1 {
					if err := checkHistoricalSelector(st, res, cur.tableIDs, true); err != nil {
						return err
					}
				} else if err := checkHistoricalSelector(st, res, cur.tableIDs, false); err != nil {
					return err
				}
				real = true
			case "starts_at_local":
				from, fok := c.From.(string)
				to, tok := c.To.(string)
				if !fok || !tok || from == "" || to == "" || from == to {
					return errors.New("changed time values invalid")
				}
				if cur.startsAtLocal != from {
					return errors.New("changed time from diverges")
				}
				if !validVersionLocal(to) {
					return errors.New("changed time values invalid")
				}
				cur.startsAtLocal = to
				real = true
			case "party_size":
				from, fok := versionNumAny(c.From)
				to, tok := versionNumAny(c.To)
				if !fok || !tok || from < 1 || to < 1 || from == to {
					return errors.New("changed party values invalid")
				}
				if cur.partySize != from {
					return errors.New("changed party from diverges")
				}
				cur.partySize = to
				real = true
			}
		}
		if !real {
			return errors.New("changed entry without real change")
		}
		cur.revision = e.Revision
		*next = cur
		return nil
	case history.EventCancelled:
		if prev == nil {
			return errors.New("cancelled entry without creation")
		}
		if len(e.Changes) != 0 {
			return errors.New("cancelled entry must be empty")
		}
		if e.PlanID != "" {
			return errors.New("plan id on non-repair entry")
		}
		*next = *prev
		next.revision = e.Revision
		return nil
	case history.EventReassigned:
		if prev == nil {
			return errors.New("reassigned entry without creation")
		}
		return checkReassignedTransition(e, prev, next, st, res, prevTerms)
	}
	return errors.New("history event unknown")
}

// numAny reads a JSON number as an int, accepting both decoded float64 and
// in-process int shapes.
func versionNumAny(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// stringList reads a string array in decoded ([]any) or in-process
// ([]string) shape.
func versionStringList(v any) ([]string, bool) {
	switch arr := v.(type) {
	case []string:
		return append([]string(nil), arr...), true
	case []any:
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			id, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, id)
		}
		return out, true
	}
	return nil, false
}

// setsEqual reports order-independent set equality.
func versionSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, id := range a {
		counts[id]++
	}
	for _, id := range b {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

// termsJSONEqual compares a frozen history terms blob against stored terms
// via the canonical public rendering.
func termsJSONEqual(raw json.RawMessage, t policy.Terms) bool {
	if len(raw) == 0 {
		return false
	}
	want, _ := json.Marshal(termsToPublic(t))
	var a, b any
	if err := json.Unmarshal(raw, &a); err != nil {
		return false
	}
	if err := json.Unmarshal(want, &b); err != nil {
		return false
	}
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	return string(ra) == string(rb)
}

// validateVersionSeries checks series coherence: map key/ID agreement,
// known owner and restaurant, positive revision, count 2..12 with matching
// member length, interval 1..4, index order 0..n-1, unique references with
// unique membership across all series, stable scheduled calendar dates, and
// current reservation mapping (cancelled allowed; exception flags permanent
// and current local dates may differ from scheduled dates after edits).
func validateVersionSeries(st *State) error {
	used := map[string]string{}
	for key, sz := range st.Series {
		if key == "" || key != sz.ID {
			return errors.New("series id and map key diverge")
		}
		if sz.ID == "" || len(sz.ID) > 64 {
			return errors.New("series id invalid")
		}
		if _, ok := st.Users[sz.UserID]; !ok {
			return errors.New("series owner unknown")
		}
		knownRest := false
		for _, r := range st.Restaurants {
			if r.ID == sz.RestaurantID {
				knownRest = true
				break
			}
		}
		if !knownRest {
			return errors.New("series restaurant unknown")
		}
		if sz.Revision < 1 {
			return errors.New("series revision invalid")
		}
		if sz.IntervalWeeks < 1 || sz.IntervalWeeks > 4 {
			return errors.New("series interval invalid")
		}
		if len(sz.Members) < 2 || len(sz.Members) > 12 {
			return errors.New("series member count invalid")
		}
		var baseY, baseM, baseD int
		for i, m := range sz.Members {
			if m.Index != i {
				return errors.New("series index order invalid")
			}
			if m.Reference == "" {
				return errors.New("series member reference invalid")
			}
			if !validVersionDate(m.ScheduledDate) {
				return errors.New("series scheduled date invalid")
			}
			y, mo, d := parseVersionYMD(m.ScheduledDate)
			if i == 0 {
				baseY, baseM, baseD = y, mo, d
			} else if want := addVersionDays(baseY, baseM, baseD, i*sz.IntervalWeeks*7); want != m.ScheduledDate {
				return errors.New("series scheduled date breaks calendar sequence")
			}
			if other, dup := used[m.Reference]; dup {
				_ = other
				return errors.New("reservation in two series")
			}
			used[m.Reference] = key
			res, ok := st.Reservations[m.Reference]
			if !ok {
				return errors.New("series member reservation missing")
			}
			if res.UserID != sz.UserID || res.RestaurantID != sz.RestaurantID {
				return errors.New("series member ownership diverges")
			}
			// Unexceptioned members keep their immutable scheduled date as
			// the current local date; true exceptions and cancelled records
			// may differ after edits.
			if !m.Exception && res.StartsAtLocal[:10] != m.ScheduledDate {
				return errors.New("series member date diverges from schedule")
			}
		}
	}
	return nil
}

// parseVersionYMD splits a validated YYYY-MM-DD date.
func parseVersionYMD(date string) (int, int, int) {
	y, m, d, _ := splitVersionYMD(date)
	return y, m, d
}

func splitVersionYMD(date string) (int, int, int, bool) {
	y, m, d, ok := 0, 0, 0, true
	for i, c := range date {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return 0, 0, 0, false
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
	_ = ok
	return y, m, d, true
}

// addVersionDays adds calendar days with UTC date-only arithmetic (never
// zone-normalized), matching the series generation contract.
func addVersionDays(y, m, d, delta int) string {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, delta)
	return t.Format("2006-01-02")
}

// validateVersionCounters checks counters are nonnegative for known
// restaurants. History lengths never imply counter values and no past
// actions are fabricated: unknown counters legitimately start at 0.
func validateVersionCounters(st *State) error {
	known := map[string]bool{}
	for _, r := range st.Restaurants {
		known[r.ID] = true
	}
	for rid, c := range st.RestaurantRevisions {
		if !known[rid] {
			return errors.New("counter for unknown restaurant")
		}
		if c < 0 {
			return errors.New("negative restaurant revision")
		}
	}
	return nil
}
