// Package history is the pure stage-3 reservation-history value engine.
//
// It builds immutable history entries (created/changed/cancelled) from
// caller-supplied snapshots: table sets in canonical declared order,
// local start times, party sizes, resulting revisions and complete raw
// accepted terms. The package never locks, mutates service state, reads the
// wall clock (all instants are explicit inputs), or imports any policy or
// service type; terms stay opaque json.RawMessage blobs that are always
// deep-copied so old snapshots never acquire newer terms.
package history

import (
	"encoding/json"
	"time"
)

// Change is one field mutation inside an entry: the field name plus complete
// old and new JSON values (null renders as JSON null).
type Change struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// Entry is one history record. AcceptedTerms is a frozen copy of the
// reservation's complete terms at this entry; Changes is always allocated
// (an empty cancelled entry renders "changes":[] rather than null).
type Entry struct {
	Seq           int             `json:"seq"`
	At            string          `json:"at"`
	Event         string          `json:"event"`
	Changes       []Change        `json:"changes"`
	Revision      int             `json:"revision"`
	AcceptedTerms json.RawMessage `json:"accepted_terms"`
}

// Snapshot is the reservation state an entry is built from. TableIDs are the
// canonical stored set in declared combinable order; AcceptedTerms is the
// complete raw terms the resulting entry freezes.
type Snapshot struct {
	TableIDs      []string
	StartsAtLocal string
	PartySize     int
	Revision      int
	AcceptedTerms json.RawMessage
}

// timestampLayout renders RFC 3339 with an always-numeric offset: UTC reads
// +00:00, never a bare Z.
const timestampLayout = "2006-01-02T15:04:05-07:00"

// Events named in entries.
const (
	EventCreated   = "created"
	EventChanged   = "changed"
	EventCancelled = "cancelled"
)

// Next derives the next sequence number and instant for a well-formed
// producer history: seq is len(entries)+1 (so the first entry is seq 1 and
// every entry increments by exactly 1), and at is now rendered as numeric
// offset RFC3339 UTC. If the wall clock moved backward past the last entry's
// instant, at clamps to that instant so seq order stays at order. An
// unparseable last instant falls back to now.
func Next(entries []Entry, now time.Time) (seq int, at string) {
	seq = len(entries) + 1
	at = now.UTC().Format(timestampLayout)
	if len(entries) == 0 {
		return seq, at
	}
	last, err := time.Parse(time.RFC3339, entries[len(entries)-1].At)
	if err != nil {
		return seq, at
	}
	if now.Before(last) {
		return seq, last.UTC().Format(timestampLayout)
	}
	return seq, at
}

// copyTerms freezes a raw terms blob.
func copyTerms(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

// copyStrings freezes a table set, never returning nil so JSON renders []
// rather than null.
func copyStrings(ids []string) []string {
	out := make([]string, 0, len(ids))
	return append(out, ids...)
}

// selectionChange renders the table-set part of a change: the scalar table_id
// field when the set is a singleton, else the complete table_ids list. The
// returned value aliases nothing mutable owned by the caller.
func selectionChange(ids []string) (field string, value any) {
	if len(ids) == 1 {
		return "table_id", ids[0]
	}
	return "table_ids", copyStrings(ids)
}

// Created builds the seq-1 creation entry for a new reservation: all three
// fields in selection, starts_at_local, party_size order, every From null,
// and the resulting revision plus a frozen copy of the complete terms.
func Created(after Snapshot, at string) Entry {
	field, to := selectionChange(after.TableIDs)
	return Entry{
		Seq:   1,
		At:    at,
		Event: EventCreated,
		Changes: []Change{
			{Field: field, From: nil, To: to},
			{Field: "starts_at_local", From: nil, To: after.StartsAtLocal},
			{Field: "party_size", From: nil, To: after.PartySize},
		},
		Revision:      after.Revision,
		AcceptedTerms: copyTerms(after.AcceptedTerms),
	}
}

// tableSetsEqual reports whether two canonical sets hold the same members,
// independent of order, so a merely reversed equivalent pair is a no-op.
func tableSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
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

// Changed builds the amendment entry for before→after, reporting only the
// fields that actually changed in table-selection, starts_at_local,
// party_size order, with the resulting revision and frozen complete terms.
// It returns false (no entry) for a no-op: identical values, a merely
// reversed equivalent pair, or terms-only differences, since terms alone do
// not make an ordinary diner amendment. A table change uses table_id iff
// both sides are singletons, otherwise complete canonical before/after
// lists in the supplied declared order.
func Changed(before, after Snapshot, seq int, at string) (Entry, bool) {
	var changes []Change
	if !tableSetsEqual(before.TableIDs, after.TableIDs) {
		var field string
		var from, to any
		if len(before.TableIDs) == 1 && len(after.TableIDs) == 1 {
			field = "table_id"
			from, to = before.TableIDs[0], after.TableIDs[0]
		} else {
			field = "table_ids"
			from = copyStrings(before.TableIDs)
			to = copyStrings(after.TableIDs)
		}
		changes = append(changes, Change{Field: field, From: from, To: to})
	}
	if before.StartsAtLocal != after.StartsAtLocal {
		changes = append(changes, Change{Field: "starts_at_local", From: before.StartsAtLocal, To: after.StartsAtLocal})
	}
	if before.PartySize != after.PartySize {
		changes = append(changes, Change{Field: "party_size", From: before.PartySize, To: after.PartySize})
	}
	if len(changes) == 0 {
		return Entry{}, false
	}
	return Entry{
		Seq:           seq,
		At:            at,
		Event:         EventChanged,
		Changes:       changes,
		Revision:      after.Revision,
		AcceptedTerms: copyTerms(after.AcceptedTerms),
	}, true
}

// Cancelled builds the cancellation entry: an allocated empty Changes slice
// (JSON "changes":[]), with the resulting revision and frozen complete terms.
func Cancelled(after Snapshot, seq int, at string) Entry {
	return Entry{
		Seq:           seq,
		At:            at,
		Event:         EventCancelled,
		Changes:       []Change{},
		Revision:      after.Revision,
		AcceptedTerms: copyTerms(after.AcceptedTerms),
	}
}

// deepCopyValue deep-copies JSON-shaped change values: raw terms blobs,
// string slices, generic arrays and objects (recursively), leaving immutable
// scalars shared. Exact nil versus non-nil empty shape is preserved for every
// copied kind, so a clone serializes identically to its source.
func deepCopyValue(v any) any {
	switch t := v.(type) {
	case json.RawMessage:
		return copyTerms(t)
	case []string:
		if t == nil {
			return nil
		}
		out := make([]string, len(t))
		copy(out, t)
		return out
	case []any:
		if t == nil {
			return nil
		}
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = deepCopyValue(item)
		}
		return out
	case map[string]any:
		if t == nil {
			return nil
		}
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = deepCopyValue(item)
		}
		return out
	default:
		return v
	}
}

// CloneEntries deep-copies entries: frozen terms bytes plus any array or map
// values inside changes, so mutating the originals leaves the clone intact
// and old snapshots never acquire newer terms.
func CloneEntries(entries []Entry) []Entry {
	if entries == nil {
		return nil
	}
	out := make([]Entry, len(entries))
	for i, e := range entries {
		var changes []Change
		if e.Changes != nil {
			changes = make([]Change, len(e.Changes))
			for j, c := range e.Changes {
				changes[j] = Change{
					Field: c.Field,
					From:  deepCopyValue(c.From),
					To:    deepCopyValue(c.To),
				}
			}
		}
		out[i] = Entry{
			Seq:           e.Seq,
			At:            e.At,
			Event:         e.Event,
			Changes:       changes,
			Revision:      e.Revision,
			AcceptedTerms: copyTerms(e.AcceptedTerms),
		}
	}
	return out
}
