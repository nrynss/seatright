package history

// S4-M foundation tests for the reassigned entry: single/pair transitions,
// unchanged sets, canonical arrays, plan id, frozen terms, field order and
// sequence, plus inherited constructor outputs unchanged.

import (
	"encoding/json"
	"testing"
)

func reassignedTerms(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"policy_version": 0, "slot_minutes": 30, "reservation_duration_minutes": 90,
		"cancellation_cutoff_minutes": 120,
		"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
		"capacities":                  map[string]any{"t_1": 2, "t_2": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReassignedSingleToSingle(t *testing.T) {
	terms := reassignedTerms(t)
	before := Snapshot{TableIDs: []string{"t_1"}, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: 1, AcceptedTerms: terms}
	after := Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: 2, AcceptedTerms: terms}
	e, ok := Reassigned(before, after, 2, "2027-05-01T12:00:00+00:00", "plan1")
	if !ok {
		t.Fatal("real set change must emit an entry")
	}
	if e.Event != EventReassigned {
		t.Fatalf("event = %s", e.Event)
	}
	if e.PlanID != "plan1" {
		t.Fatalf("plan_id = %s", e.PlanID)
	}
	if e.Seq != 2 || e.Revision != 2 || e.At != "2027-05-01T12:00:00+00:00" {
		t.Fatalf("seq/rev/at = %d/%d/%s", e.Seq, e.Revision, e.At)
	}
	if len(e.Changes) != 1 {
		t.Fatalf("changes = %d, want exactly one", len(e.Changes))
	}
	c := e.Changes[0]
	if c.Field != "table_ids" {
		t.Fatalf("field = %s, want table_ids even for singletons", c.Field)
	}
	from, _ := c.From.([]string)
	to, _ := c.To.([]string)
	if len(from) != 1 || from[0] != "t_1" || len(to) != 1 || to[0] != "t_2" {
		t.Fatalf("from/to = %v/%v", c.From, c.To)
	}
	if string(e.AcceptedTerms) != string(terms) {
		t.Fatal("terms not frozen")
	}
	// Inputs not modified.
	if before.TableIDs[0] != "t_1" || after.TableIDs[0] != "t_2" {
		t.Fatal("inputs modified")
	}
}

func TestReassignedPairTransitions(t *testing.T) {
	terms := reassignedTerms(t)
	mk := func(ids []string, rev int) Snapshot {
		return Snapshot{TableIDs: ids, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: rev, AcceptedTerms: terms}
	}
	// Singleton -> pair uses full canonical arrays.
	e, ok := Reassigned(mk([]string{"t_1"}, 1), mk([]string{"t_1", "t_2"}, 2), 3, "2027-05-01T12:00:00+00:00", "p")
	if !ok {
		t.Fatal("singleton->pair must emit")
	}
	if from := e.Changes[0].From.([]string); len(from) != 1 || from[0] != "t_1" {
		t.Fatalf("from = %v", e.Changes[0].From)
	}
	if to := e.Changes[0].To.([]string); len(to) != 2 || to[0] != "t_1" || to[1] != "t_2" {
		t.Fatalf("to = %v", e.Changes[0].To)
	}
	// Pair -> singleton likewise.
	e, ok = Reassigned(mk([]string{"t_1", "t_2"}, 2), mk([]string{"t_2"}, 3), 4, "2027-05-01T12:00:00+00:00", "p")
	if !ok {
		t.Fatal("pair->singleton must emit")
	}
	if to := e.Changes[0].To.([]string); len(to) != 1 || to[0] != "t_2" {
		t.Fatalf("to = %v", e.Changes[0].To)
	}
	// Pair -> different pair: exact full From/To in declared caller order,
	// resulting revision/seq/at/plan_id and unchanged complete terms.
	e, ok = Reassigned(mk([]string{"t_1", "t_2"}, 3), mk([]string{"t_2", "t_3"}, 4), 5, "2027-05-02T12:00:00+00:00", "plan7")
	if !ok {
		t.Fatal("pair->pair must emit")
	}
	if e.Event != EventReassigned || e.Seq != 5 || e.Revision != 4 ||
		e.At != "2027-05-02T12:00:00+00:00" || e.PlanID != "plan7" {
		t.Fatalf("meta = %+v", e)
	}
	if len(e.Changes) != 1 || e.Changes[0].Field != "table_ids" {
		t.Fatalf("changes = %+v", e.Changes)
	}
	from := e.Changes[0].From.([]string)
	to := e.Changes[0].To.([]string)
	if len(from) != 2 || from[0] != "t_1" || from[1] != "t_2" {
		t.Fatalf("from = %v", e.Changes[0].From)
	}
	if len(to) != 2 || to[0] != "t_2" || to[1] != "t_3" {
		t.Fatalf("to = %v", e.Changes[0].To)
	}
	if string(e.AcceptedTerms) != string(terms) {
		t.Fatal("terms not carried")
	}
}

func TestReassignedUnchangedSets(t *testing.T) {
	terms := reassignedTerms(t)
	mk := func(ids []string) Snapshot {
		return Snapshot{TableIDs: ids, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: 2, AcceptedTerms: terms}
	}
	// Identical sets: no entry.
	if _, ok := Reassigned(mk([]string{"t_1"}), mk([]string{"t_1"}), 2, "2027-05-01T12:00:00+00:00", "p"); ok {
		t.Fatal("identical singletons must be a no-op")
	}
	// Same unordered pair set: no entry (order-insensitive set comparison).
	if _, ok := Reassigned(mk([]string{"t_1", "t_2"}), mk([]string{"t_2", "t_1"}), 2, "2027-05-01T12:00:00+00:00", "p"); ok {
		t.Fatal("same pair set must be a no-op")
	}
}

func TestReassignedTermsFrozenAndClone(t *testing.T) {
	terms := reassignedTerms(t)
	before := Snapshot{TableIDs: []string{"t_1"}, Revision: 1, AcceptedTerms: terms}
	after := Snapshot{TableIDs: []string{"t_2"}, Revision: 2, AcceptedTerms: terms}
	e, ok := Reassigned(before, after, 2, "2027-05-01T12:00:00+00:00", "plan9")
	if !ok {
		t.Fatal("must emit")
	}
	// Mutating the source terms and snapshot arrays must not affect the entry.
	terms[2] = '9'
	before.TableIDs[0] = "evil"
	if string(e.AcceptedTerms) == string(terms) {
		t.Fatal("entry aliases source terms")
	}
	if e.Changes[0].From.([]string)[0] != "t_1" {
		t.Fatal("entry aliases source snapshot arrays")
	}
	// Capture the COMPLETE original Entry JSON BEFORE mutating the clone;
	// after all cloned From/To/accepted_terms mutations the ORIGINAL Entry
	// must marshal byte-identical (binds clone-terms and every field).
	completeBefore, _ := json.Marshal(e)
	cl0 := CloneEntries([]Entry{e})
	cl0[0].Changes[0].From.([]string)[0] = "evil"
	cl0[0].Changes[0].To.([]string)[0] = "evil"
	cl0[0].AcceptedTerms[2] = '8'
	completeAfter, _ := json.Marshal(e)
	if string(completeBefore) != string(completeAfter) {
		t.Fatalf("original changed via clone:\n%s\n%s", completeBefore, completeAfter)
	}
	var origM map[string]any
	if err := json.Unmarshal(completeAfter, &origM); err != nil {
		t.Fatal(err)
	}
	chs := origM["changes"].([]any)[0].(map[string]any)
	if chs["from"].([]any)[0] != "t_1" || chs["to"].([]any)[0] != "t_2" {
		t.Fatalf("original mutated via clone: %v", chs)
	}
	if origM["plan_id"] != "plan9" || origM["event"] != "reassigned" || origM["revision"] != float64(2) {
		t.Fatalf("original meta changed: %v", origM)
	}
	// CloneEntries preserves the reassigned shape including PlanID.
	cl := CloneEntries([]Entry{e})
	if len(cl) != 1 || cl[0].PlanID != "plan9" || cl[0].Event != EventReassigned {
		t.Fatalf("clone = %+v", cl)
	}
	if len(cl[0].Changes) != 1 || cl[0].Changes[0].Field != "table_ids" {
		t.Fatalf("clone changes = %+v", cl[0].Changes)
	}
	// JSON shape: plan_id present on reassigned, absent (omitted) on others.
	raw, _ := json.Marshal(e)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["plan_id"] != "plan9" || m["event"] != "reassigned" {
		t.Fatalf("json = %v", m)
	}
	created := Created(after, "2027-05-01T12:00:00+00:00")
	raw, _ = json.Marshal(created)
	m = nil
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["plan_id"]; ok {
		t.Fatalf("created must omit plan_id: %v", m)
	}
}

func TestReassignedInheritedOutputsUnchanged(t *testing.T) {
	terms := reassignedTerms(t)
	after := Snapshot{TableIDs: []string{"t_1"}, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: 1, AcceptedTerms: terms}
	c := Created(after, "2027-05-01T12:00:00+00:00")
	if c.Event != EventCreated || c.Seq != 1 || len(c.Changes) != 3 {
		t.Fatalf("created = %+v", c)
	}
	if c.Changes[0].Field != "table_id" {
		t.Fatalf("created singleton field = %s", c.Changes[0].Field)
	}
	before := Snapshot{TableIDs: []string{"t_1"}, StartsAtLocal: "2027-05-06T19:00", PartySize: 2, Revision: 1, AcceptedTerms: terms}
	ch, ok := Changed(before, Snapshot{TableIDs: []string{"t_1"}, StartsAtLocal: "2027-05-06T20:00", PartySize: 2, Revision: 2, AcceptedTerms: terms}, 2, "2027-05-01T12:00:00+00:00")
	if !ok || ch.Event != EventChanged || len(ch.Changes) != 1 || ch.Changes[0].Field != "starts_at_local" {
		t.Fatalf("changed = %+v %v", ch, ok)
	}
	x := Cancelled(after, 2, "2027-05-01T12:00:00+00:00")
	if x.Event != EventCancelled || x.Changes == nil || len(x.Changes) != 0 {
		t.Fatalf("cancelled = %+v", x)
	}
}
