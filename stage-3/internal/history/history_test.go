package history

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testTerms0() json.RawMessage {
	return json.RawMessage(`{"policy_version":0,"slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`)
}

func testTerms1() json.RawMessage {
	return json.RawMessage(`{"policy_version":1,"slot_minutes":60,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6}}`)
}

// R214: singleton creation names table_id, starts_at_local, party_size in
// order, every From null, seq 1, resulting revision and frozen terms.
func TestCreatedSingleton(t *testing.T) {
	after := Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
	e := Created(after, "2026-09-17T12:00:00+02:00")
	if e.Seq != 1 || e.Event != EventCreated || e.At != "2026-09-17T12:00:00+02:00" {
		t.Fatalf("bad created header: %+v", e)
	}
	if e.Revision != 1 {
		t.Fatalf("bad revision: %+v", e)
	}
	want := []Change{
		{Field: "table_id", From: nil, To: "t_2"},
		{Field: "starts_at_local", From: nil, To: "2026-09-24T19:00"},
		{Field: "party_size", From: nil, To: 4},
	}
	if !reflect.DeepEqual(e.Changes, want) {
		t.Fatalf("bad created changes: %#v", e.Changes)
	}
	if string(e.AcceptedTerms) != string(testTerms0()) {
		t.Fatalf("bad terms: %s", e.AcceptedTerms)
	}
	raw, err := json.Marshal(e.Changes[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"field":"table_id","from":null,"to":"t_2"}` {
		t.Fatalf("bad change JSON: %s", raw)
	}
}

// R291: pair creation replaces table_id with table_ids from null to the full
// canonical pair; inputs are frozen so later caller mutation is invisible.
func TestCreatedPairFreezesInputs(t *testing.T) {
	tables := []string{"t_1", "t_2"}
	terms := testTerms0()
	after := Snapshot{TableIDs: tables, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: terms}
	e := Created(after, "2026-09-17T12:00:00+02:00")
	if len(e.Changes) != 3 || e.Changes[0].Field != "table_ids" || e.Changes[0].From != nil {
		t.Fatalf("bad pair creation: %#v", e.Changes)
	}
	if !reflect.DeepEqual(e.Changes[0].To, []string{"t_1", "t_2"}) {
		t.Fatalf("bad pair to: %#v", e.Changes[0].To)
	}
	tables[0] = "t_9"
	terms[2] = 'X'
	if !reflect.DeepEqual(e.Changes[0].To, []string{"t_1", "t_2"}) {
		t.Fatalf("creation aliases caller slice: %#v", e.Changes[0].To)
	}
	if string(e.AcceptedTerms) != string(testTerms0()) {
		t.Fatalf("creation aliases terms bytes: %s", e.AcceptedTerms)
	}
}

// R290: singleton-to-singleton history retains scalar table_id before/after.
func TestChangedSingletonToSingleton(t *testing.T) {
	before := Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
	after := Snapshot{TableIDs: []string{"t_3"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 2, AcceptedTerms: testTerms1()}
	e, ok := Changed(before, after, 2, "2026-09-17T12:05:00+02:00")
	if !ok {
		t.Fatal("expected a changed entry")
	}
	if e.Event != EventChanged || e.Seq != 2 || e.Revision != 2 {
		t.Fatalf("bad header: %+v", e)
	}
	want := []Change{{Field: "table_id", From: "t_2", To: "t_3"}}
	if !reflect.DeepEqual(e.Changes, want) {
		t.Fatalf("bad changes: %#v", e.Changes)
	}
	if string(e.AcceptedTerms) != string(testTerms1()) {
		t.Fatalf("entry must carry resulting terms: %s", e.AcceptedTerms)
	}
}

// R292: any table-selection change involving a pair records complete
// canonical before/after table_ids lists, in supplied declared order.
func TestChangedPairTransitions(t *testing.T) {
	cases := []struct {
		name   string
		before []string
		after  []string
		from   []string
		to     []string
	}{
		{"singleton-to-pair", []string{"t_3"}, []string{"t_1", "t_2"}, []string{"t_3"}, []string{"t_1", "t_2"}},
		{"pair-to-singleton", []string{"t_1", "t_2"}, []string{"t_3"}, []string{"t_1", "t_2"}, []string{"t_3"}},
		{"pair-to-pair", []string{"t_1", "t_2"}, []string{"t_2", "t_3"}, []string{"t_1", "t_2"}, []string{"t_2", "t_3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := Snapshot{TableIDs: append([]string(nil), c.before...), StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
			after := Snapshot{TableIDs: append([]string(nil), c.after...), StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 2, AcceptedTerms: testTerms0()}
			e, ok := Changed(before, after, 2, "2026-09-17T12:05:00+02:00")
			if !ok {
				t.Fatal("expected a changed entry")
			}
			if len(e.Changes) != 1 || e.Changes[0].Field != "table_ids" {
				t.Fatalf("bad changes: %#v", e.Changes)
			}
			if !reflect.DeepEqual(e.Changes[0].From, c.from) || !reflect.DeepEqual(e.Changes[0].To, c.to) {
				t.Fatalf("from/to must be complete canonical lists: %#v", e.Changes[0])
			}
			// Emitted arrays retain supplied canonical order, even when the
			// caller's canonical order is not sorted.
			if c.name == "pair-to-pair" {
				raw, _ := json.Marshal(e.Changes[0])
				if !strings.Contains(string(raw), `"to":["t_2","t_3"]`) {
					t.Fatalf("order not retained: %s", raw)
				}
			}
		})
	}
}

// R293: a reversed input pair names the same set and is a no-op on its own.
func TestChangedReversedPairIsNoOp(t *testing.T) {
	before := Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: testTerms0()}
	after := Snapshot{TableIDs: []string{"t_2", "t_1"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: testTerms0()}
	if _, ok := Changed(before, after, 2, "2026-09-17T12:05:00+02:00"); ok {
		t.Fatal("reversed equivalent pair must be a no-op")
	}
}

// R215: changed entries name only actually changed fields, in
// table-selection, starts_at_local, party_size order, with complete values.
func TestChangedMultiFieldOrder(t *testing.T) {
	before := Snapshot{TableIDs: []string{"t_1"}, StartsAtLocal: "2026-09-24T18:00", PartySize: 2, Revision: 1, AcceptedTerms: testTerms0()}
	after := Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 2, AcceptedTerms: testTerms1()}
	e, ok := Changed(before, after, 2, "2026-09-17T12:05:00+02:00")
	if !ok {
		t.Fatal("expected a changed entry")
	}
	if len(e.Changes) != 3 {
		t.Fatalf("expected 3 changes: %#v", e.Changes)
	}
	if e.Changes[0].Field != "table_ids" || e.Changes[1].Field != "starts_at_local" || e.Changes[2].Field != "party_size" {
		t.Fatalf("wrong order: %#v", e.Changes)
	}
	if !reflect.DeepEqual(e.Changes[0].From, []string{"t_1"}) || !reflect.DeepEqual(e.Changes[0].To, []string{"t_1", "t_2"}) {
		t.Fatalf("bad selection change: %#v", e.Changes[0])
	}
	if e.Changes[1].From != "2026-09-24T18:00" || e.Changes[1].To != "2026-09-24T19:00" {
		t.Fatalf("bad time change: %#v", e.Changes[1])
	}
	if e.Changes[2].From != 2 || e.Changes[2].To != 6 {
		t.Fatalf("bad party change: %#v", e.Changes[2])
	}
}

func TestChangedSingleFieldSubsets(t *testing.T) {
	base := Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
	timeOnly := base
	timeOnly.StartsAtLocal = "2026-09-24T20:00"
	timeOnly.Revision = 2
	e, ok := Changed(base, timeOnly, 2, "2026-09-17T12:05:00+02:00")
	if !ok || len(e.Changes) != 1 || e.Changes[0].Field != "starts_at_local" {
		t.Fatalf("bad time-only change: %+v %#v", e, e.Changes)
	}
	partyOnly := base
	partyOnly.PartySize = 3
	partyOnly.Revision = 2
	e, ok = Changed(base, partyOnly, 2, "2026-09-17T12:05:00+02:00")
	if !ok || len(e.Changes) != 1 || e.Changes[0].Field != "party_size" {
		t.Fatalf("bad party-only change: %+v %#v", e, e.Changes)
	}
}

// R216 + terms rule: identical values, and terms-only differences, record no
// entry at all.
func TestChangedNoOps(t *testing.T) {
	base := Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
	if _, ok := Changed(base, base, 2, "2026-09-17T12:05:00+02:00"); ok {
		t.Fatal("identical snapshots must be a no-op")
	}
	termsOnly := base
	termsOnly.Revision = 2
	termsOnly.AcceptedTerms = testTerms1()
	if _, ok := Changed(base, termsOnly, 2, "2026-09-17T12:05:00+02:00"); ok {
		t.Fatal("terms-only differences must not make an ordinary amendment")
	}
	dup := Snapshot{TableIDs: []string{"t_1", "t_1"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}
	if _, ok := Changed(dup, dup, 2, "2026-09-17T12:05:00+02:00"); ok {
		t.Fatal("identical multisets must be a no-op")
	}
}

// Constructors must never alter either Snapshot.
func TestChangedDoesNotAlterSnapshots(t *testing.T) {
	before := Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: testTerms0()}
	after := Snapshot{TableIDs: []string{"t_2", "t_3"}, StartsAtLocal: "2026-09-24T20:00", PartySize: 5, Revision: 2, AcceptedTerms: testTerms1()}
	beforeCopy := Snapshot{TableIDs: append([]string(nil), before.TableIDs...), StartsAtLocal: before.StartsAtLocal, PartySize: before.PartySize, Revision: before.Revision, AcceptedTerms: append(json.RawMessage(nil), before.AcceptedTerms...)}
	afterCopy := Snapshot{TableIDs: append([]string(nil), after.TableIDs...), StartsAtLocal: after.StartsAtLocal, PartySize: after.PartySize, Revision: after.Revision, AcceptedTerms: append(json.RawMessage(nil), after.AcceptedTerms...)}
	if _, ok := Changed(before, after, 2, "2026-09-17T12:05:00+02:00"); !ok {
		t.Fatal("expected a changed entry")
	}
	Created(after, "2026-09-17T12:00:00+02:00")
	Cancelled(after, 3, "2026-09-17T12:09:00+02:00")
	if !reflect.DeepEqual(before.TableIDs, beforeCopy.TableIDs) || !reflect.DeepEqual(after.TableIDs, afterCopy.TableIDs) {
		t.Fatal("constructors mutated caller snapshots")
	}
	if string(before.AcceptedTerms) != string(beforeCopy.AcceptedTerms) || string(after.AcceptedTerms) != string(afterCopy.AcceptedTerms) {
		t.Fatal("constructors mutated caller terms")
	}
}

// R212/R213: seq starts at 1 and increments by exactly 1; at is numeric-offset
// RFC3339 UTC and never moves backward past the last entry instant.
func TestNextSequenceAndClock(t *testing.T) {
	seq, at := Next(nil, time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC))
	if seq != 1 {
		t.Fatalf("first seq must be 1, got %d", seq)
	}
	if at != "2026-09-17T10:00:00+00:00" {
		t.Fatalf("UTC must render numeric +00:00, got %s", at)
	}
	if strings.Contains(at, "Z") {
		t.Fatalf("offset must not use Z: %s", at)
	}
	if _, err := time.Parse(time.RFC3339, at); err != nil {
		t.Fatalf("at must parse as RFC3339: %v", err)
	}
	hist := []Entry{Created(Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}, "2026-09-17T12:00:00+02:00")}
	seq, _ = Next(hist, time.Now())
	if seq != 2 {
		t.Fatalf("seq must increment by 1, got %d", seq)
	}
	five := append([]Entry(nil), hist...)
	for i := 2; i <= 5; i++ {
		five = append(five, Entry{Seq: i, At: "2026-09-17T12:00:00+02:00", Event: EventChanged, Changes: []Change{{Field: "party_size", From: i, To: i + 1}}, Revision: i, AcceptedTerms: testTerms0()})
	}
	if seq, _ := Next(five, time.Now()); seq != 6 {
		t.Fatalf("seq must be len+1, got %d", seq)
	}
}

func TestNextSameSecondAndBackward(t *testing.T) {
	last := "2026-09-17T12:05:00+02:00"
	hist := []Entry{{Seq: 1, At: last, Event: EventCreated, Changes: []Change{}, Revision: 1, AcceptedTerms: testTerms0()}}
	lastInstant, _ := time.Parse(time.RFC3339, last)
	// Same instant: nondecreasing.
	_, at := Next(hist, lastInstant)
	if at != lastInstant.UTC().Format("2006-01-02T15:04:05-07:00") {
		t.Fatalf("same-second at must equal last instant: %s", at)
	}
	// Backward clock: clamp to the last entry instant.
	_, at = Next(hist, lastInstant.Add(-time.Hour))
	if at != lastInstant.UTC().Format("2006-01-02T15:04:05-07:00") {
		t.Fatalf("backward clock must clamp to last instant: %s", at)
	}
	// Forward clock: use wall clock.
	later := lastInstant.Add(time.Hour)
	_, at = Next(hist, later)
	if at != later.UTC().Format("2006-01-02T15:04:05-07:00") {
		t.Fatalf("forward clock must use now: %s", at)
	}
	if _, err := time.Parse(time.RFC3339, at); err != nil {
		t.Fatalf("clamped at must stay RFC3339: %v", err)
	}
}

// R217: cancelled carries an allocated empty changes slice rendering [].
func TestCancelledEmptyChanges(t *testing.T) {
	after := Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 3, AcceptedTerms: testTerms1()}
	e := Cancelled(after, 3, "2026-09-17T12:09:00+02:00")
	if e.Event != EventCancelled || e.Seq != 3 || e.Revision != 3 {
		t.Fatalf("bad header: %+v", e)
	}
	if e.Changes == nil || len(e.Changes) != 0 {
		t.Fatalf("cancelled changes must be allocated empty: %#v", e.Changes)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"changes":[]`) {
		t.Fatalf("cancelled changes must render [] not null: %s", raw)
	}
	if string(e.AcceptedTerms) != string(testTerms1()) {
		t.Fatalf("cancelled entry must freeze terms: %s", e.AcceptedTerms)
	}
}

// Full entry JSON shape, including nested accepted terms object.
func TestEntryJSONShape(t *testing.T) {
	e := Created(Snapshot{TableIDs: []string{"t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 4, Revision: 1, AcceptedTerms: testTerms0()}, "2026-09-17T12:00:00+02:00")
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"seq", "at", "event", "changes", "revision", "accepted_terms"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %s in %s", key, raw)
		}
	}
	if decoded["seq"] != float64(1) || decoded["event"] != "created" || decoded["revision"] != float64(1) {
		t.Fatalf("bad scalars: %s", raw)
	}
	terms, ok := decoded["accepted_terms"].(map[string]any)
	if !ok || terms["policy_version"] != float64(0) || terms["slot_minutes"] != float64(30) {
		t.Fatalf("accepted_terms must be the complete nested object: %s", raw)
	}
	if _, hasEffective := terms["effective_from"]; hasEffective {
		t.Fatalf("accepted_terms must exclude effective_from: %s", raw)
	}
	changes := decoded["changes"].([]any)
	if len(changes) != 3 {
		t.Fatalf("bad changes: %s", raw)
	}
	first := changes[0].(map[string]any)
	if first["field"] != "table_id" || first["from"] != nil || first["to"] != "t_2" {
		t.Fatalf("bad first change: %s", raw)
	}
}

// R257 + deep-clone isolation: mutating originals (terms bytes, change
// slices, map/array change values) must not touch the clone, so old snapshots
// never acquire newer terms.
func TestCloneEntriesIsolation(t *testing.T) {
	e1 := Created(Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: testTerms0()}, "2026-09-17T12:00:00+02:00")
	e2, _ := Changed(
		Snapshot{TableIDs: []string{"t_1", "t_2"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 1, AcceptedTerms: testTerms0()},
		Snapshot{TableIDs: []string{"t_2", "t_3"}, StartsAtLocal: "2026-09-24T19:00", PartySize: 6, Revision: 2, AcceptedTerms: testTerms1()},
		2, "2026-09-17T12:05:00+02:00")
	entries := []Entry{e1, e2}
	clone := CloneEntries(entries)
	// Mutate every mutable surface of the originals.
	entries[0].AcceptedTerms[2] = 'X'
	entries[1].AcceptedTerms[2] = 'X'
	entries[1].Changes[0].To.([]string)[0] = "t_9"
	entries[1].Changes[0].From.([]string)[1] = "t_9"
	if string(clone[0].AcceptedTerms) != string(testTerms0()) {
		t.Fatalf("old snapshot acquired newer terms: %s", clone[0].AcceptedTerms)
	}
	if string(clone[1].AcceptedTerms) != string(testTerms1()) {
		t.Fatalf("clone terms mutated: %s", clone[1].AcceptedTerms)
	}
	if !reflect.DeepEqual(clone[1].Changes[0].To, []string{"t_2", "t_3"}) {
		t.Fatalf("clone slice mutated: %#v", clone[1].Changes[0].To)
	}
	if !reflect.DeepEqual(clone[1].Changes[0].From, []string{"t_1", "t_2"}) {
		t.Fatalf("clone slice mutated: %#v", clone[1].Changes[0].From)
	}
}

func TestCloneEntriesMapsAndArrays(t *testing.T) {
	entries := []Entry{{
		Seq: 1, At: "2026-09-17T12:00:00+02:00", Event: EventChanged, Revision: 1, AcceptedTerms: testTerms0(),
		Changes: []Change{
			{Field: "table_ids", From: []any{"t_1", "t_2"}, To: []any{"t_2", "t_3"}},
			{Field: "meta", From: map[string]any{"a": []any{"x"}}, To: map[string]any{"b": "y"}},
		},
	}}
	clone := CloneEntries(entries)
	entries[0].Changes[0].To.([]any)[0] = "t_9"
	entries[0].Changes[1].From.(map[string]any)["a"].([]any)[0] = "z"
	if !reflect.DeepEqual(clone[0].Changes[0].To, []any{"t_2", "t_3"}) {
		t.Fatalf("clone array mutated: %#v", clone[0].Changes[0].To)
	}
	if !reflect.DeepEqual(clone[0].Changes[1].From, map[string]any{"a": []any{"x"}}) {
		t.Fatalf("clone map mutated: %#v", clone[0].Changes[1].From)
	}
}

func TestCloneEntriesNil(t *testing.T) {
	if CloneEntries(nil) != nil {
		t.Fatal("nil must clone to nil")
	}
	out := CloneEntries([]Entry{})
	if out == nil || len(out) != 0 {
		t.Fatalf("empty must stay empty non-nil: %#v", out)
	}
}

// CloneEntries must preserve exact nil versus non-nil empty shape for every
// JSON-shaped value, so the clone serializes byte-identically to its source:
// nil slices render null, empty ones render [], nil maps render null and
// empty maps render {}.
func TestCloneEntriesNilEmptyShape(t *testing.T) {
	entries := []Entry{
		{Seq: 1, At: "2026-09-17T12:00:00+02:00", Event: EventChanged, Revision: 1, AcceptedTerms: testTerms0(), Changes: nil},
		{Seq: 2, At: "2026-09-17T12:01:00+02:00", Event: EventChanged, Revision: 1, AcceptedTerms: testTerms0(), Changes: []Change{}},
		{Seq: 3, At: "2026-09-17T12:02:00+02:00", Event: EventChanged, Revision: 1, AcceptedTerms: testTerms0(), Changes: []Change{
			{Field: "table_ids", From: []string(nil), To: []string{}},
			{Field: "mix", From: []any(nil), To: []any{}},
			{Field: "obj", From: map[string]any(nil), To: map[string]any{}},
		}},
	}
	clone := CloneEntries(entries)
	if clone[0].Changes != nil {
		t.Fatalf("nil changes must stay nil: %#v", clone[0].Changes)
	}
	if clone[1].Changes == nil {
		t.Fatal("empty non-nil changes must stay non-nil (renders [])")
	}
	got := clone[2].Changes
	if got[0].From != nil {
		t.Fatalf("nil []string must stay nil: %#v", got[0].From)
	}
	if to, ok := got[0].To.([]string); !ok || to == nil {
		t.Fatalf("empty []string must stay non-nil: %#v", got[0].To)
	}
	if got[1].From != nil {
		t.Fatalf("nil []any must stay nil: %#v", got[1].From)
	}
	if to, ok := got[1].To.([]any); !ok || to == nil {
		t.Fatalf("empty []any must stay non-nil: %#v", got[1].To)
	}
	if got[2].From != nil {
		t.Fatalf("nil map must stay nil: %#v", got[2].From)
	}
	if to, ok := got[2].To.(map[string]any); !ok || to == nil {
		t.Fatalf("empty map must stay non-nil: %#v", got[2].To)
	}
	for i := range entries {
		want, err := json.Marshal(entries[i])
		if err != nil {
			t.Fatal(err)
		}
		actual, err := json.Marshal(clone[i])
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != string(actual) {
			t.Fatalf("clone %d serializes differently:\n%s\n%s", i, want, actual)
		}
	}
}

// Nil terms bytes stay nil (JSON null); nonempty terms are frozen copies, so
// mutating the source never touches the clone.
func TestCloneEntriesTermsShape(t *testing.T) {
	entries := []Entry{
		{Seq: 1, At: "2026-09-17T12:00:00+02:00", Event: EventCancelled, Revision: 2, AcceptedTerms: nil, Changes: []Change{}},
		{Seq: 2, At: "2026-09-17T12:01:00+02:00", Event: EventChanged, Revision: 3, AcceptedTerms: testTerms1(), Changes: []Change{{Field: "party_size", From: 4, To: 6}}},
	}
	clone := CloneEntries(entries)
	if clone[0].AcceptedTerms != nil {
		t.Fatalf("nil terms must stay nil: %q", clone[0].AcceptedTerms)
	}
	entries[1].AcceptedTerms[2] = 'X'
	if string(clone[1].AcceptedTerms) != string(testTerms1()) {
		t.Fatalf("clone terms mutated: %s", clone[1].AcceptedTerms)
	}
	for i := range entries {
		if i == 1 {
			continue // source was deliberately mutated above
		}
		want, _ := json.Marshal(entries[i])
		actual, _ := json.Marshal(clone[i])
		if string(want) != string(actual) {
			t.Fatalf("clone %d serializes differently:\n%s\n%s", i, want, actual)
		}
	}
	fresh, _ := json.Marshal(Entry{Seq: 2, At: "2026-09-17T12:01:00+02:00", Event: EventChanged, Revision: 3, AcceptedTerms: testTerms1(), Changes: []Change{{Field: "party_size", From: 4, To: 6}}})
	actual, _ := json.Marshal(clone[1])
	if string(fresh) != string(actual) {
		t.Fatalf("mutated source leaked into clone:\n%s\n%s", fresh, actual)
	}
}
