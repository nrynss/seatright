package service

import (
	"encoding/json"
	"errors"

	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// changeTo reads a history change target value.
func changeTo(c history.Change) any {
	return c.To
}

// This file owns storage consistency for seating-repair state: Reassigned
// history transitions and plan/closure coherence. Everything here is pure
// and read-only: no state mutation, clock reads, locks, id/receipt/counter
// changes or normalization. It runs offside during import after
// normalizeVersionState, validateState and validateVersionState.
//
// Producer contracts honored:
//   - Reassigned entries carry exactly one table_ids change with full
//     canonical From/To arrays (even singleton-to-singleton), a nonempty
//     plan_id, unchanged clock/party and frozen accepted terms.
//   - The referenced plan must exist, be applied, belong to the same
//     restaurant, list this ref exactly once with changed=true and target
//     TableIDs equal the entry's To at THAT event.
//   - Closures are validated for shape/interval only; historical occupancy,
//     cutoff, grid, hours and capacity business rules are never re-applied.
//   - Applied-plan closure multisets must match stored closures per
//     restaurant with exact multiplicity (duplicate identical closures from
//     two legitimate applies are allowed).

// checkReassignedTransition validates one Reassigned entry against the
// running snapshot and advances selection only. A repair changes selectors
// only: the entry must carry EXACTLY the previous entry's accepted terms
// (not merely terms matching some published version). Clock, party and
// revision follow the entry's own recorded values.
func checkReassignedTransition(e history.Entry, prev *versionSnapshot, next *versionSnapshot, st *State, res Reservation, prevTerms []byte) error {
	if len(prevTerms) == 0 {
		return errors.New("reassigned entry without prior terms")
	}
	if !termsRawEqual(e.AcceptedTerms, prevTerms) {
		return errors.New("reassigned terms diverge from prior entry")
	}
	if e.PlanID == "" {
		return errors.New("reassigned entry without plan")
	}
	if len(e.PlanID) > 64 {
		return errors.New("reassigned plan id invalid")
	}
	if len(e.Changes) != 1 {
		return errors.New("reassigned entry must name one field")
	}
	c := e.Changes[0]
	if c.Field != "table_ids" {
		return errors.New("reassigned field must be table_ids")
	}
	from, fok := versionStringList(c.From)
	to, tok := versionStringList(c.To)
	if !fok || !tok || len(from) == 0 || len(from) > 2 || len(to) == 0 || len(to) > 2 {
		return errors.New("reassigned table values invalid")
	}
	if versionSetsEqual(from, to) {
		return errors.New("reassigned entry without real change")
	}
	// From must equal the actual replayed previous selection, and both
	// sides must be canonical legal sets (1..2 known ids, no duplicates,
	// declared pair order for pairs). Singleton->singleton arrays are
	// valid here (ordinary Changed keeps its scalar table_id field).
	if !versionSetsEqual(from, prev.tableIDs) {
		return errors.New("reassigned from diverges")
	}
	if err := checkHistoricalSelector(st, res, from, len(from) == 1); err != nil {
		return errors.New("reassigned from invalid")
	}
	if err := checkHistoricalSelector(st, res, to, len(to) == 1); err != nil {
		return errors.New("reassigned table invalid")
	}
	// Cross-bind to the stored plan at THIS event: the plan must exist, be
	// applied, belong to this restaurant, contain this ref exactly once with
	// changed=true, and its saved target must equal the entry's To. Later
	// edits may move the booking elsewhere; only this event's To binds.
	plan, ok := st.Plans[e.PlanID]
	if !ok {
		return errors.New("reassigned plan unknown")
	}
	if !plan.Applied {
		return errors.New("reassigned plan not applied")
	}
	if plan.RestaurantID != res.RestaurantID {
		return errors.New("reassigned plan restaurant diverges")
	}
	hits := 0
	for _, a := range plan.Assignments {
		if a.Reference == res.Reference {
			hits++
			if !a.Changed {
				return errors.New("reassigned assignment not changed")
			}
			if !versionSetsEqual(a.TableIDs, to) {
				return errors.New("reassigned target diverges from plan")
			}
		}
	}
	if hits != 1 {
		return errors.New("reassigned plan assignment not unique")
	}
	cur := *prev
	cur.tableIDs = append([]string(nil), to...)
	cur.revision = e.Revision
	*next = cur
	return nil
}

// validateReplanState checks plan/closure storage consistency. Every failure
// is a plain error surfaced as 422 validation_failed without mutating the
// destination.
func validateReplanState(st *State) error {
	restIDs := versionRestaurantIDs(st)
	tables := versionRestaurantTables(st)
	for key, p := range st.Plans {
		if key == "" || key != p.ID {
			return errors.New("plan id and map key diverge")
		}
		if p.ID == "" || len(p.ID) > 64 {
			return errors.New("plan id invalid")
		}
		if !restIDs[p.RestaurantID] {
			return errors.New("plan restaurant unknown")
		}
		if p.RestaurantRevision < 0 {
			return errors.New("plan revision invalid")
		}
		cur := 0
		if st.RestaurantRevisions != nil {
			cur = st.RestaurantRevisions[p.RestaurantID]
		}
		if cur < 0 {
			return errors.New("negative restaurant revision")
		}
		if p.RestaurantRevision > cur {
			return errors.New("plan revision beyond current counter")
		}
		if p.Applied && cur < p.RestaurantRevision+1 {
			return errors.New("applied plan revision inconsistent")
		}
		// Assignments: reference-ascending, unique, same-restaurant known
		// refs, at most 6, canonical legal sets.
		if len(p.Assignments) > 6 {
			return errors.New("plan assignments exceed limit")
		}
		seen := map[string]bool{}
		last := ""
		moved := 0
		for _, a := range p.Assignments {
			if a.Reference == "" || seen[a.Reference] {
				return errors.New("plan assignment reference invalid")
			}
			seen[a.Reference] = true
			if last != "" && a.Reference <= last {
				return errors.New("plan assignments out of order")
			}
			last = a.Reference
			res, ok := st.Reservations[a.Reference]
			if !ok {
				return errors.New("plan assignment reservation missing")
			}
			if res.RestaurantID != p.RestaurantID {
				return errors.New("plan assignment restaurant diverges")
			}
			if len(a.TableIDs) == 0 || len(a.TableIDs) > 2 {
				return errors.New("plan assignment table set invalid")
			}
			known := tables[p.RestaurantID]
			dup := map[string]bool{}
			for _, id := range a.TableIDs {
				if id == "" || !known[id] || dup[id] {
					return errors.New("plan assignment table unknown")
				}
				dup[id] = true
			}
			if len(a.TableIDs) == 2 {
				if !pairDeclared(st, p.RestaurantID, a.TableIDs) {
					return errors.New("plan assignment pair not declared")
				}
				if !isCanonicalPairOrder(st, p.RestaurantID, a.TableIDs) {
					return errors.New("plan assignment pair order invalid")
				}
			}
			if a.Changed {
				moved++
			}
		}
		if p.MovedCount != moved {
			return errors.New("plan moved count diverges")
		}
		if p.MovedCount < 0 || p.UnusedSeats < 0 {
			return errors.New("plan totals negative")
		}
		// Closure: known table, strict real interval from<to.
		if !tables[p.RestaurantID][p.Closure.TableID] {
			return errors.New("plan closure table unknown")
		}
		from, ok := parseStrictInstant(p.Closure.From)
		if !ok {
			return errors.New("plan closure from invalid")
		}
		to, ok := parseStrictInstant(p.Closure.To)
		if !ok {
			return errors.New("plan closure to invalid")
		}
		if !from.Before(to) {
			return errors.New("plan closure interval invalid")
		}
	}
	// Stored closures: known namespace/restaurant/table, strict intervals.
	for rid, list := range st.Closures {
		if !restIDs[rid] {
			return errors.New("closure restaurant unknown")
		}
		for _, c := range list {
			if !tables[rid][c.TableID] {
				return errors.New("closure table unknown")
			}
			from, ok := parseStrictInstant(c.From)
			if !ok {
				return errors.New("closure from invalid")
			}
			to, ok := parseStrictInstant(c.To)
			if !ok {
				return errors.New("closure to invalid")
			}
			if !from.Before(to) {
				return errors.New("closure interval invalid")
			}
		}
	}
	// Applied-plan closure multiset must match stored closures per
	// restaurant with exact multiplicity. Unapplied plans contribute
	// nothing; zero-move applies still contribute one closure.
	want := map[string][]Closure{}
	for _, p := range st.Plans {
		if p.Applied {
			want[p.RestaurantID] = append(want[p.RestaurantID], p.Closure)
		}
	}
	for rid, list := range st.Closures {
		if len(list) != len(want[rid]) {
			return errors.New("applied closures diverge from stored")
		}
	}
	for rid, list := range want {
		if len(list) != len(st.Closures[rid]) {
			return errors.New("applied closures diverge from stored")
		}
		matched := make([]bool, len(st.Closures[rid]))
		for _, w := range list {
			found := false
			for i, c := range st.Closures[rid] {
				if !matched[i] && c == w {
					matched[i] = true
					found = true
					break
				}
			}
			if !found {
				return errors.New("applied closure missing from stored")
			}
		}
	}
	// Reassigned/plan cross-coverage: every changed assignment of an applied
	// plan has exactly one Reassigned entry naming this plan; unchanged
	// assignments and unapplied plans have none.
	counts := map[string]map[string]int{}
	for ref, entries := range st.Histories {
		for _, e := range entries {
			if e.Event != history.EventReassigned {
				continue
			}
			if counts[e.PlanID] == nil {
				counts[e.PlanID] = map[string]int{}
			}
			counts[e.PlanID][ref]++
		}
	}
	for key, p := range st.Plans {
		_ = key
		for _, a := range p.Assignments {
			got := 0
			if counts[p.ID] != nil {
				got = counts[p.ID][a.Reference]
			}
			if p.Applied && a.Changed {
				if got != 1 {
					return errors.New("changed assignment without exactly one reassigned entry")
				}
			} else if got != 0 {
				return errors.New("unexpected reassigned entry")
			}
		}
	}
	if err := validateReplanReceipts(st); err != nil {
		return err
	}
	return nil
}

// validateReplanReceipts binds saved plans to their genuine retained
// first-201 receipts in both directions. A stored preview receipt (POST
// /restaurants/{id}/replans with this plan_id in its six-key response)
// freezes the original producer decision: captured revision, closure,
// ordered assignments with flags, and both totals must equal the stored
// plan. An applied plan's apply receipt (POST .../replans/{pid}/apply)
// must carry the full original six-key preview values plus the frozen
// apply outcome: exact assignment count/order/references, canonical table
// sets with scalar shape, and per-reference historical identity/terms/
// revisions drawn from plan+history+receipt only. A retained native
// receipt naming an unknown plan (or no plan at all) cannot skip
// validation: orphan native receipts fail. Plans without any retained
// receipt (old exporters) keep the structural checks above. The original
// caller is never required to still be a manager, historical targets are
// never compared to current records, and Solve is never rerun.
func validateReplanReceipts(st *State) error {
	for key, p := range st.Plans {
		_ = key
		if rc := previewReceiptFor(st, p); rc != nil {
			if err := bindPreviewReceipt(p, rc); err != nil {
				return err
			}
		}
		if p.Applied {
			if rc := applyReceiptFor(st, p); rc != nil {
				if err := bindApplyReceipt(st, p, rc); err != nil {
					return err
				}
			}
		}
	}
	return validateOrphanNativeReceipts(st)
}

// validateOrphanNativeReceipts rejects retained native replan receipts
// that name no stored plan. Stage-1..3 exports carry no native receipts,
// so this only fires on contradictory stage-4 forgeries.
func validateOrphanNativeReceipts(st *State) error {
	for _, rc := range st.Receipts {
		kind, pid := nativeReceiptKind(st, rc)
		if kind == "" {
			continue
		}
		id := receiptPlanID(rc.Response)
		if id == "" {
			return errors.New("native receipt without plan id")
		}
		if kind == "preview" {
			if _, ok := st.Plans[id]; !ok {
				return errors.New("preview receipt names unknown plan")
			}
			continue
		}
		if id != pid {
			return errors.New("apply receipt plan diverges")
		}
		if _, ok := st.Plans[id]; !ok {
			return errors.New("apply receipt names unknown plan")
		}
	}
	return nil
}

// nativeReceiptKind classifies a retained receipt by its exact scoped path:
// "preview" for POST /restaurants/{known-id}/replans, "apply:{pid}" for
// POST /restaurants/{known-id}/replans/{pid}/apply, "" otherwise. Unknown
// restaurant ids never classify: only genuine routes bind.
func nativeReceiptKind(st *State, rc Receipt) (string, string) {
	const prefix = "/restaurants/"
	if rc.Method != "POST" || rc.Status != 201 {
		return "", ""
	}
	if len(rc.Path) <= len(prefix) || rc.Path[:len(prefix)] != prefix {
		return "", ""
	}
	rest := rc.Path[len(prefix):]
	parts := splitPath(rest)
	if len(parts) == 2 && parts[1] == "replans" && versionRestaurantIDs(st)[parts[0]] {
		return "preview", ""
	}
	if len(parts) == 4 && parts[1] == "replans" && parts[3] == "apply" && versionRestaurantIDs(st)[parts[0]] && parts[2] != "" {
		return "apply", parts[2]
	}
	return "", ""
}

func splitPath(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '/' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	return append(out, cur)
}

// receiptPlanID reads the plan_id from a stored receipt response.
func receiptPlanID(raw string) string {
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return ""
	}
	id, _ := body["plan_id"].(string)
	return id
}

// previewReceiptFor returns the stored preview receipt whose six-key
// response names this plan, or nil when no receipt is retained.
func previewReceiptFor(st *State, p Replan) *Receipt {
	want := "/restaurants/" + p.RestaurantID + "/replans"
	for _, rc := range st.Receipts {
		rc := rc
		if rc.Method != "POST" || rc.Path != want || rc.Status != 201 {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(rc.Response), &body); err != nil {
			continue
		}
		if id, _ := body["plan_id"].(string); id == p.ID {
			return &rc
		}
	}
	return nil
}

// bindPreviewReceipt requires the stored plan to equal its own original
// preview response: captured revision, closure text, ordered assignments
// with flags, and both totals.
func bindPreviewReceipt(p Replan, rc *Receipt) error {
	var body map[string]any
	if err := json.Unmarshal([]byte(rc.Response), &body); err != nil {
		return errors.New("preview receipt response invalid")
	}
	if rev, ok := versionNumAny(body["restaurant_revision"]); !ok || rev != p.RestaurantRevision {
		return errors.New("plan revision contradicts original receipt")
	}
	cl, ok := body["closure"].(map[string]any)
	if !ok || strField(cl, "table_id") != p.Closure.TableID ||
		strField(cl, "from") != p.Closure.From || strField(cl, "to") != p.Closure.To {
		return errors.New("plan closure contradicts original receipt")
	}
	rawAs, ok := body["assignments"].([]any)
	if !ok || len(rawAs) != len(p.Assignments) {
		return errors.New("plan assignments contradict original receipt")
	}
	for i, a := range p.Assignments {
		m, ok := rawAs[i].(map[string]any)
		if !ok || strField(m, "reference") != a.Reference {
			return errors.New("plan assignments contradict original receipt")
		}
		ids, ok := versionStringList(m["table_ids"])
		if !ok || !versionSetsEqual(ids, a.TableIDs) || len(ids) != len(a.TableIDs) {
			return errors.New("plan assignments contradict original receipt")
		}
		if flag, _ := m["changed"].(bool); flag != a.Changed {
			return errors.New("plan assignments contradict original receipt")
		}
	}
	if mv, ok := versionNumAny(body["moved_count"]); !ok || mv != p.MovedCount {
		return errors.New("plan totals contradict original receipt")
	}
	if us, ok := versionNumAny(body["unused_seats"]); !ok || us != p.UnusedSeats {
		return errors.New("plan totals contradict original receipt")
	}
	return nil
}

// applyReceiptFor returns the stored apply receipt naming this plan, or nil.
func applyReceiptFor(st *State, p Replan) *Receipt {
	want := "/restaurants/" + p.RestaurantID + "/replans/" + p.ID + "/apply"
	for _, rc := range st.Receipts {
		rc := rc
		if rc.Method != "POST" || rc.Path != want || rc.Status != 201 {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(rc.Response), &body); err != nil {
			continue
		}
		if id, _ := body["plan_id"].(string); id == p.ID {
			return &rc
		}
	}
	return nil
}

// bindApplyReceipt requires the apply response to name the stored applied
// plan with frozen response revision captured+1 and the EXACT considered
// outcome: same number of reservations in reference-ascending order with
// the same reference set as the saved assignments, canonical table sets
// with scalar shape equal to each saved assignment target, and historical
// identity/terms/revisions consistent with plan+history+receipt. Current
// (evolved) record values are never compared: the receipt is frozen at
// apply time, so targets bind to the saved assignment and the Reassigned
// entry, not to today's record.
func bindApplyReceipt(st *State, p Replan, rc *Receipt) error {
	var body map[string]any
	if err := json.Unmarshal([]byte(rc.Response), &body); err != nil {
		return errors.New("apply receipt response invalid")
	}
	if id, _ := body["plan_id"].(string); id != p.ID {
		return errors.New("apply receipt plan diverges")
	}
	if rev, ok := versionNumAny(body["restaurant_revision"]); !ok || rev != p.RestaurantRevision+1 {
		return errors.New("apply receipt revision contradicts plan")
	}
	recs, ok := body["reservations"].([]any)
	if !ok || len(recs) != len(p.Assignments) {
		return errors.New("apply receipt reservations diverge from plan")
	}
	last := ""
	for i, r := range recs {
		m, ok := r.(map[string]any)
		if !ok {
			return errors.New("apply receipt reservations invalid")
		}
		ref, _ := m["reference"].(string)
		if ref == "" || ref != p.Assignments[i].Reference {
			return errors.New("apply receipt reservations diverge from plan")
		}
		if last != "" && ref <= last {
			return errors.New("apply receipt reservations out of order")
		}
		last = ref
		want := p.Assignments[i]
		ids, ok := versionStringList(m["table_ids"])
		if !ok || len(ids) != len(want.TableIDs) || !versionSetsEqual(ids, want.TableIDs) {
			return errors.New("apply receipt target contradicts assignment")
		}
		_, hasScalar := m["table_id"]
		if hasScalar != (len(want.TableIDs) == 1) {
			return errors.New("apply receipt scalar shape invalid")
		}
		if hasScalar && strField(m, "table_id") != want.TableIDs[0] {
			return errors.New("apply receipt target contradicts assignment")
		}
		// Historical consistency against plan+history+receipt only: the
		// frozen receipt record must carry this booking's identity
		// (reservation_id, restaurant, clock, party) and accepted terms as
		// recorded in history, and its revision must be exactly one past
		// the pre-repair revision for changed assignments.
		if err := bindApplyRecord(st, p, want, m); err != nil {
			return err
		}
	}
	return nil
}

// bindApplyRecord checks one frozen apply-response record against the
// saved assignment and the booking's own history. Changed assignments
// must show a Reassigned entry naming this plan whose To equals both the
// receipt target and the saved assignment; the receipt terms must equal
// the Reassigned entry terms; the receipt revision must be the Reassigned
// revision. Clock/party bind to the history replay at the Reassigned
// entry (Created values plus earlier Changed updates), never to the
// evolved current record. Unchanged assignments bind clock/party/terms/
// revision to the full history replay at the final entry.
func bindApplyRecord(st *State, p Replan, want ReplanAssignment, m map[string]any) error {
	res, ok := st.Reservations[want.Reference]
	if !ok {
		return errors.New("apply receipt reference unknown")
	}
	if strField(m, "reservation_id") != res.ReservationID {
		return errors.New("apply receipt identity diverges")
	}
	if strField(m, "restaurant_id") != res.RestaurantID {
		return errors.New("apply receipt identity diverges")
	}
	entries := st.Histories[want.Reference]
	rev, ok := versionNumAny(m["revision"])
	if !ok {
		return errors.New("apply receipt revision invalid")
	}
	if want.Changed {
		clock, party, terms, rrev, found := replayToReassigned(entries, p.ID)
		if !found {
			return errors.New("apply receipt changed assignment without history")
		}
		if strField(m, "starts_at_local") != clock {
			return errors.New("apply receipt clock diverges from history")
		}
		if partyNow, ok := versionNumAny(m["party_size"]); !ok || partyNow != party {
			return errors.New("apply receipt party diverges from history")
		}
		if !termsRawEqual(rawTerms(m["accepted_terms"]), terms) {
			return errors.New("apply receipt terms diverge from history")
		}
		if rev != rrev {
			return errors.New("apply receipt revision diverges from history")
		}
		to := want.TableIDs
		var entryTo []string
		for _, e := range entries {
			if e.Event == history.EventReassigned && e.PlanID == p.ID && len(e.Changes) == 1 {
				if arr, ok := versionStringList(e.Changes[0].To); ok {
					entryTo = arr
				}
			}
		}
		if !versionSetsEqual(entryTo, to) || len(entryTo) != len(to) {
			return errors.New("apply receipt target diverges from history")
		}
		_ = to
	} else {
		// Unchanged assignments never gain history: the frozen receipt
		// must match the history replay at the record's revision, not
		// the evolved final state. Later PATCH/cancel/clock evolution
		// appends entries and bumps the record revision; the receipt
		// stays frozen at apply time, so bind it to the replay prefix
		// ending at the receipt's own revision.
		clock, party, terms, rrev, found := replayToRevision(entries, rev)
		if !found {
			return errors.New("apply receipt history missing")
		}
		if strField(m, "starts_at_local") != clock {
			return errors.New("apply receipt clock diverges from history")
		}
		if partyNow, ok := versionNumAny(m["party_size"]); !ok || partyNow != party {
			return errors.New("apply receipt party diverges from history")
		}
		if !termsRawEqual(rawTerms(m["accepted_terms"]), terms) {
			return errors.New("apply receipt terms diverge from history")
		}
		if rev != rrev {
			return errors.New("apply receipt revision diverges from history")
		}
	}
	return nil
}

// replayToReassigned walks history to the Reassigned entry naming planID,
// returning the clock/party/terms/revision then current. Changed entries
// update clock/party along the way.
func replayToReassigned(entries []history.Entry, planID string) (string, int, []byte, int, bool) {
	var clock string
	var party int
	started := false
	for _, e := range entries {
		switch e.Event {
		case history.EventCreated:
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					clock, _ = c.To.(string)
				case "party_size":
					party, _ = versionNumAny(c.To)
				}
			}
			started = true
		case history.EventChanged:
			if !started {
				return "", 0, nil, 0, false
			}
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					if s, ok := c.To.(string); ok {
						clock = s
					}
				case "party_size":
					if n, ok := versionNumAny(c.To); ok {
						party = n
					}
				}
			}
		case history.EventReassigned:
			if !started {
				return "", 0, nil, 0, false
			}
			if e.PlanID == planID {
				return clock, party, e.AcceptedTerms, e.Revision, true
			}
		}
	}
	return "", 0, nil, 0, false
}

// replayToRevision replays history entries with Revision <= want, returning
// the clock/party/terms then current. Unchanged repair assignments gain no
// entries, so the receipt revision selects exactly the apply-time prefix;
// later evolution entries (higher revisions) are excluded. Cancelled finals
// keep pre-cancel values with the final revision/terms.
func replayToRevision(entries []history.Entry, want int) (string, int, []byte, int, bool) {
	var clock string
	var party int
	var terms []byte
	var rev int
	started := false
	seen := false
	for _, e := range entries {
		if e.Revision > want {
			break
		}
		switch e.Event {
		case history.EventCreated:
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					clock, _ = c.To.(string)
				case "party_size":
					party, _ = versionNumAny(c.To)
				}
			}
			terms = e.AcceptedTerms
			rev = e.Revision
			started = true
			seen = true
		case history.EventChanged, history.EventReassigned:
			if !started {
				return "", 0, nil, 0, false
			}
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					if s, ok := c.To.(string); ok {
						clock = s
					}
				case "party_size":
					if n, ok := versionNumAny(c.To); ok {
						party = n
					}
				}
			}
			terms = e.AcceptedTerms
			rev = e.Revision
			seen = true
		case history.EventCancelled:
			if !started {
				return "", 0, nil, 0, false
			}
			terms = e.AcceptedTerms
			rev = e.Revision
			seen = true
		default:
			return "", 0, nil, 0, false
		}
	}
	if !seen || rev != want {
		return "", 0, nil, 0, false
	}
	return clock, party, terms, rev, true
}

// replayToFinal replays a full history to its final clock/party/terms/
// revision. Cancelled finals keep the pre-cancel values with the final
// revision/terms.
func replayToFinal(entries []history.Entry) (string, int, []byte, int, bool) {
	var clock string
	var party int
	var terms []byte
	var rev int
	started := false
	for _, e := range entries {
		switch e.Event {
		case history.EventCreated:
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					clock, _ = c.To.(string)
				case "party_size":
					party, _ = versionNumAny(c.To)
				}
			}
			terms = e.AcceptedTerms
			rev = e.Revision
			started = true
		case history.EventChanged, history.EventReassigned:
			if !started {
				return "", 0, nil, 0, false
			}
			for _, c := range e.Changes {
				switch c.Field {
				case "starts_at_local":
					if s, ok := c.To.(string); ok {
						clock = s
					}
				case "party_size":
					if n, ok := versionNumAny(c.To); ok {
						party = n
					}
				}
			}
			terms = e.AcceptedTerms
			rev = e.Revision
		case history.EventCancelled:
			if !started {
				return "", 0, nil, 0, false
			}
			terms = e.AcceptedTerms
			rev = e.Revision
		}
	}
	if !started {
		return "", 0, nil, 0, false
	}
	return clock, party, terms, rev, true
}

// rawTerms marshals an arbitrary decoded terms value to canonical bytes.
func rawTerms(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

// rawTermsOf renders stored policy terms through the public projection so
// receipt/record/history blobs compare in one shape.
func rawTermsOf(t policy.Terms) []byte {
	return rawTerms(termsToPublic(t))
}

// termsRawEqual compares two frozen terms blobs as JSON values.
// A repair must preserve the previous entry's terms exactly.
func termsRawEqual(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &y); err != nil {
		return false
	}
	rx, _ := json.Marshal(x)
	ry, _ := json.Marshal(y)
	return string(rx) == string(ry)
}

// strField reads an optional string field.
func strField(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}
