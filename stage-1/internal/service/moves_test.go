package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const seedZwei = `{"id":"r_zwei","name":"Zwei","timezone":"Europe/Berlin","slot_minutes":30,
	"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
	"tables":[{"id":"t_9","label":"9","capacity":2}]}`

// movesSetup seeds two owners and returns their tokens. Bookings A and B are
// ada's future bookings on t_1/t_2 at 19:00; P is ada's past booking; O is
// bob's future booking.
func movesSetup(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	future := futureThursday(t)
	s := New()
	resetWith(t, s, seedAda+","+seedBob, seedAnkerTables,
		seedRes("sA", "MV0001", "u_ada", "r_anker", "t_1", future+"T19:00", 2, "confirmed")+","+
			seedRes("sB", "MV0002", "u_ada", "r_anker", "t_2", future+"T19:00", 2, "confirmed")+","+
			seedRes("sP", "MV0003", "u_ada", "r_anker", "t_1", "2020-01-02T19:00", 2, "confirmed")+","+
			seedRes("sO", "MV0004", "u_bob", "r_anker", "t_2", future+"T20:30", 2, "confirmed"))
	return s, loginToken(t, s, "ada@example.com"), loginToken(t, s, "bob@example.com"), future
}

func movesBody(t *testing.T, s *Service, token, key string, moves string) (int, map[string]any) {
	t.Helper()
	rec := postWrite(t, s, "/reservation-moves", token, key, `{"moves":[`+moves+`]}`)
	var v map[string]any
	if rec.Code == 201 || rec.Code == 200 {
		v = decodeBody(t, rec)
	} else {
		status, code := errorCode(t, rec)
		v = map[string]any{"_status": float64(status), "_code": code}
	}
	return rec.Code, v
}

func TestMovesShapeBounds(t *testing.T) {
	s, ada, _, _ := movesSetup(t)
	mk := func(ref string) string { return `{"reference":` + fmt.Sprintf("%q", ref) + `}` }
	// One no-op move succeeds.
	if status, v := movesBody(t, s, ada, "m-1", mk("MV0001")); status != 201 {
		t.Fatalf("single: %d %v", status, v)
	} else if rs := v["reservations"].([]any); len(rs) != 1 || rs[0].(map[string]any)["reference"] != "MV0001" {
		t.Fatalf("single response = %v", v)
	}
	// Duplicates are rejected whatever the length.
	if status, _ := movesBody(t, s, ada, "m-8dup", mk("MV0001")+","+mk("MV0001")); status != 422 {
		t.Fatalf("duplicates: %d", status)
	}
	many := ""
	for i := 0; i < 9; i++ {
		if i > 0 {
			many += ","
		}
		many += mk("MV0001")
	}
	if status, v := movesBody(t, s, ada, "m-9", many); status != 422 || v["_code"] != "validation_failed" {
		t.Fatalf("nine: %d %v", status, v)
	}
	if rec := postWrite(t, s, "/reservation-moves", ada, "m-0", `{"moves":[]}`); rec.Code != 422 {
		t.Fatalf("zero: %d", rec.Code)
	}
	if rec := postWrite(t, s, "/reservation-moves", ada, "m-miss", `{}`); rec.Code != 422 {
		t.Fatalf("missing: %d", rec.Code)
	}
	for _, tc := range []struct{ name, body string }{
		{"non-array", `{"moves":{}}`},
		{"non-object", `{"moves":[42]}`},
		{"missing ref", `{"moves":[{}]}`},
		{"numeric ref", `{"moves":[{"reference":42}]}`},
		{"empty ref", `{"moves":[{"reference":""}]}`},
	} {
		if rec := postWrite(t, s, "/reservation-moves", ada, "m-"+tc.name, tc.body); rec.Code != 422 {
			t.Fatalf("%s: %d %q", tc.name, rec.Code, rec.Body.String())
		}
	}
	if rec := postWrite(t, s, "/reservation-moves", ada, "-", `{"moves":[{ "reference":"MV0001"}]}`); rec.Code != 400 {
		t.Fatalf("missing key: %d", rec.Code)
	}
	if rec := postWrite(t, s, "/reservation-moves", ada, "m-bad", `{`); rec.Code != 400 {
		t.Fatalf("unparseable: %d", rec.Code)
	}
}

func TestMovesEightDistinct(t *testing.T) {
	s := New()
	// Eight distinct references on non-overlapping slots (Thursday 18:00 /
	// 19:30 / 21:00 on both tables plus two Friday slots): the shape itself
	// must reach a 201, not a shape rejection.
	thu := futureThursday(t)
	d, _ := time.Parse("2006-01-02", thu)
	friday := d.Add(24 * time.Hour).Format("2006-01-02")
	placements := []struct{ table, local string }{
		{"t_1", thu + "T18:00"}, {"t_2", thu + "T18:00"},
		{"t_1", thu + "T19:30"}, {"t_2", thu + "T19:30"},
		{"t_1", thu + "T21:00"}, {"t_2", thu + "T21:00"},
		{"t_1", friday + "T18:00"}, {"t_2", friday + "T18:00"},
	}
	var many []string
	var seedList []string
	for i, p := range placements {
		ref := fmt.Sprintf("E8000%d", i)
		many = append(many, `{"reference":`+fmt.Sprintf("%q", ref)+`}`)
		seedList = append(seedList, seedRes(fmt.Sprintf("e%d", i), ref, "u_ada", "r_anker",
			p.table, p.local, 2, "confirmed"))
	}
	resetWith(t, s, seedAda, seedAnkerTables, strings.Join(seedList, ","))
	ada := loginToken(t, s, "ada@example.com")
	status, v := movesBody(t, s, ada, "m-eight", strings.Join(many, ","))
	if status != 201 {
		t.Fatalf("eight no-op moves: %d %v", status, v)
	}
	if rs := v["reservations"].([]any); len(rs) != 8 {
		t.Fatalf("want 8 in input order, got %v", v)
	}
}

func TestMovesOwnershipAndRestaurants(t *testing.T) {
	s, ada, _, future := movesSetup(t)
	if status, v := movesBody(t, s, ada, "m-unk", `{"reference":"NOPE000"}`); status != 404 || v["_code"] != "not_found" {
		t.Fatalf("unknown: %d %v", status, v)
	}
	if status, v := movesBody(t, s, ada, "m-for", `{"reference":"MV0004"}`); status != 404 || v["_code"] != "not_found" {
		t.Fatalf("foreign: %d %v", status, v)
	}
	// Mixed restaurants: seed a second-restaurant booking for ada.
	s2 := New()
	resetWith(t, s2, seedAda, seedAnkerTables+","+seedZwei,
		seedRes("sA", "MV0001", "u_ada", "r_anker", "t_1", future+"T19:00", 2, "confirmed")+","+
			seedRes("sZ", "MZ0001", "u_ada", "r_zwei", "t_9", future+"T19:00", 2, "confirmed"))
	ada2 := loginToken(t, s2, "ada@example.com")
	status, v := movesBody(t, s2, ada2, "m-mix", `{"reference":"MV0001"},{"reference":"MZ0001"}`)
	if status != 422 || v["_code"] != "validation_failed" {
		t.Fatalf("mixed: %d %v", status, v)
	}
}

func TestMovesInputOrderPrecedence(t *testing.T) {
	s, ada, _, _ := movesSetup(t)
	// Cancel MV0001 first for the cancelled-precedence case via a fresh state.
	s2, ada2, _, _ := movesSetup(t)
	serveRequest(s2, http.MethodPost, "/reservations/MV0001/cancel", nil, authHeader(ada2))
	if status, v := movesBody(t, s2, ada2, "m-cx", `{"reference":"MV0001"},{"reference":"NOPE000"}`); status != 409 || v["_code"] != "reservation_cancelled" {
		t.Fatalf("cancelled beats later unknown: %d %v", status, v)
	}
	ordered := []struct {
		name, moves string
		status      int
		code        string
	}{
		{"cutoff-then-unknown", `{"reference":"MV0003"},{"reference":"NOPE000"}`, 409, "cutoff_passed"},
		{"unknown-then-cutoff", `{"reference":"NOPE000"},{"reference":"MV0003"}`, 404, "not_found"},
		{"field-then-cutoff", `{"reference":"MV0002","table_id":"t_nope"},{"reference":"MV0003"}`, 404, "not_found"},
		{"cutoff-then-field", `{"reference":"MV0003"},{"reference":"MV0002","table_id":"t_nope"}`, 409, "cutoff_passed"},
	}
	for i, tc := range ordered {
		status, v := movesBody(t, s, ada, fmt.Sprintf("m-ord-%d", i), tc.moves)
		if status != tc.status || v["_code"] != tc.code {
			t.Errorf("%s: got %d %v", tc.name, status, v)
		}
	}
	// Failures change nothing.
	lookup := serveRequest(s, http.MethodGet, "/reservations/MV0002", nil, authHeader(ada))
	if v := decodeBody(t, lookup); v["table_id"] != "t_2" {
		t.Fatalf("failed batch mutated: %q", lookup.Body.String())
	}
}

func TestMovesSwapAndRotation(t *testing.T) {
	s, ada, _, future := movesSetup(t)
	status, v := movesBody(t, s, ada, "m-swap", `{"reference":"MV0001","table_id":"t_2"},{"reference":"MV0002","table_id":"t_1"}`)
	if status != 201 {
		t.Fatalf("swap: %d %v", status, v)
	}
	rs := v["reservations"].([]any)
	if rs[0].(map[string]any)["reference"] != "MV0001" || rs[1].(map[string]any)["reference"] != "MV0002" {
		t.Fatalf("input order not preserved: %v", v)
	}
	if rs[0].(map[string]any)["table_id"] != "t_2" || rs[1].(map[string]any)["table_id"] != "t_1" {
		t.Fatalf("swap values: %v", v)
	}
	// Identity preserved.
	a := rs[0].(map[string]any)
	if a["reservation_id"] != "sA" {
		t.Fatalf("identity changed: %v", a)
	}
	// Three-way rotation across slots.
	s2 := New()
	resetWith(t, s2, seedAda, seedAnkerTables,
		seedRes("sA", "MV0001", "u_ada", "r_anker", "t_1", future+"T18:00", 2, "confirmed")+","+
			seedRes("sB", "MV0002", "u_ada", "r_anker", "t_1", future+"T19:30", 2, "confirmed")+","+
			seedRes("sC", "MV0003", "u_ada", "r_anker", "t_1", future+"T21:00", 2, "confirmed"))
	ada2 := loginToken(t, s2, "ada@example.com")
	moves := fmt.Sprintf(`{"reference":"MV0001","starts_at_local":%q},{"reference":"MV0002","starts_at_local":%q},{"reference":"MV0003","starts_at_local":%q}`,
		future+"T19:30", future+"T21:00", future+"T18:00")
	status, v = movesBody(t, s2, ada2, "m-rot", moves)
	if status != 201 {
		t.Fatalf("rotation: %d %v", status, v)
	}
	got := map[string]string{}
	for _, r := range v["reservations"].([]any) {
		m := r.(map[string]any)
		got[m["reference"].(string)] = m["starts_at_local"].(string)
	}
	if got["MV0001"] != future+"T19:30" || got["MV0002"] != future+"T21:00" || got["MV0003"] != future+"T18:00" {
		t.Fatalf("rotation values = %v", got)
	}
}

func TestMovesConflictRollback(t *testing.T) {
	s, ada, _, future := movesSetup(t)
	// Unlisted blocker: MV0004 sits on t_2 at future 20:30.
	status, v := movesBody(t, s, ada, "m-blk", `{"reference":"MV0002","starts_at_local":"`+future+`T20:30"}`)
	if status != 409 || v["_code"] != "table_unavailable" {
		t.Fatalf("unlisted blocker: %d %v", status, v)
	}
	// Listed unchanged booking blocks too.
	status, v = movesBody(t, s, ada, "m-blk2", `{"reference":"MV0001","table_id":"t_2"},{"reference":"MV0002"}`)
	if status != 409 || v["_code"] != "table_unavailable" {
		t.Fatalf("listed blocker: %d %v", status, v)
	}
	// Two movers onto the same free slot conflict with each other.
	status, v = movesBody(t, s, ada, "m-blk3", `{"reference":"MV0001","starts_at_local":"`+future+`T20:30"},{"reference":"MV0002","starts_at_local":"`+future+`T20:30"}`)
	if status != 409 || v["_code"] != "table_unavailable" {
		t.Fatalf("mutual conflict: %d %v", status, v)
	}
	// Nothing changed: both bookings keep values and occupancy.
	for _, tc := range []struct{ ref, table, local string }{
		{"MV0001", "t_1", future + "T19:00"},
		{"MV0002", "t_2", future + "T19:00"},
	} {
		lookup := serveRequest(s, http.MethodGet, "/reservations/"+tc.ref, nil, authHeader(ada))
		if v := decodeBody(t, lookup); v["table_id"] != tc.table || v["starts_at_local"] != tc.local {
			t.Fatalf("%s changed: %q", tc.ref, lookup.Body.String())
		}
	}
	s.mu.Lock()
	n := len(s.state.Receipts)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("failed batches claimed %d receipts", n)
	}
}

func TestMovesNoopAndIdentity(t *testing.T) {
	s, ada, _, _ := movesSetup(t)
	before := serveRequest(s, http.MethodGet, "/reservations/MV0001", nil, authHeader(ada)).Body.String()
	status, v := movesBody(t, s, ada, "m-noop", `{"reference":"MV0001","color":"blue"}`)
	if status != 201 {
		t.Fatalf("noop: %d %v", status, v)
	}
	rs := v["reservations"].([]any)
	if len(rs) != 1 || rs[0].(map[string]any)["table_id"] != "t_1" {
		t.Fatalf("noop response = %v", v)
	}
	after := serveRequest(s, http.MethodGet, "/reservations/MV0001", nil, authHeader(ada)).Body.String()
	if before != after {
		t.Fatalf("noop changed record:\n%s\n%s", before, after)
	}
}

func TestMovesReplayAndImport(t *testing.T) {
	s, ada, _, future := movesSetup(t)
	_ = future
	status, v := movesBody(t, s, ada, "m-ok", `{"reference":"MV0001","party_size":1}`)
	if status != 201 {
		t.Fatalf("batch: %d %v", status, v)
	}
	orig, _ := json.Marshal(v)
	// Later edits and cancels do not change the replay.
	serveRequest(s, http.MethodPatch, "/reservations/MV0001", []byte(`{"party_size":2}`), authHeader(ada))
	serveRequest(s, http.MethodPost, "/reservations/MV0002/cancel", nil, authHeader(ada))
	status, v2 := movesBody(t, s, ada, "m-ok", `{"reference":"MV0001","party_size":1}`)
	if status != 200 {
		t.Fatalf("replay: %d %v", status, v2)
	}
	again, _ := json.Marshal(v2)
	if string(again) != string(orig) {
		t.Fatalf("replay differs:\n%s\n%s", again, orig)
	}
	// State untouched by the replay.
	lookup := serveRequest(s, http.MethodGet, "/reservations/MV0001", nil, authHeader(ada))
	if decodeBody(t, lookup)["party_size"] != 2.0 {
		t.Fatalf("replay mutated: %q", lookup.Body.String())
	}
	// Across export/import.
	export := serveRequest(s, http.MethodGet, "/_test/export", nil, nil)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	status, v3 := movesBody(t, dst, ada, "m-ok", `{"reference":"MV0001","party_size":1}`)
	if status != 200 {
		t.Fatalf("replay after import: %d %v", status, v3)
	}
	imported, _ := json.Marshal(v3)
	if string(imported) != string(orig) {
		t.Fatalf("import replay differs:\n%s\n%s", imported, orig)
	}
	// Batch results survived with identities.
	lookup = serveRequest(dst, http.MethodGet, "/reservations/MV0001", nil, authHeader(ada))
	if v := decodeBody(t, lookup); v["reservation_id"] != "sA" || v["status"] != StatusConfirmed {
		t.Fatalf("imported result = %q", lookup.Body.String())
	}
}

func TestMovesConcurrency(t *testing.T) {
	s, ada, _, future := movesSetup(t)
	_ = future
	t.Run("identical batches serialize", func(t *testing.T) {
		const n = 10
		var wg sync.WaitGroup
		type outcome struct {
			status int
			body   string
		}
		results := make([]outcome, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				rec := postWrite(t, s, "/reservation-moves", ada, "m-race", `{"moves":[{"reference":"MV0001","party_size":1}]}`)
				results[i] = outcome{rec.Code, rec.Body.String()}
			}(i)
		}
		wg.Wait()
		created, replayed := 0, 0
		for _, r := range results {
			switch r.status {
			case 201:
				created++
			case 200:
				replayed++
			default:
				t.Fatalf("status %d", r.status)
			}
		}
		if created != 1 || replayed != n-1 {
			t.Fatalf("created=%d replayed=%d", created, replayed)
		}
		first := ""
		for _, r := range results {
			if first == "" {
				first = r.body
			} else if r.body != first {
				t.Fatal("batch replay bodies differ")
			}
		}
	})
	t.Run("batch versus individual write stays serializable", func(t *testing.T) {
		s2, ada2, _, _ := movesSetup(t)
		var wg sync.WaitGroup
		var batchRes, patchRes, createRes int
		wg.Add(3)
		go func() {
			defer wg.Done()
			rec := postWrite(t, s2, "/reservation-moves", ada2, "m-mix", `{"moves":[{"reference":"MV0001","table_id":"t_2"},{"reference":"MV0002","table_id":"t_1"}]}`)
			batchRes = rec.Code
		}()
		go func() {
			defer wg.Done()
			rec := serveRequest(s2, http.MethodPatch, "/reservations/MV0001", []byte(`{"party_size":1}`), authHeader(ada2))
			patchRes = rec.Code
		}()
		go func() {
			defer wg.Done()
			rec := postWrite(t, s2, "/reservations", ada2, "m-mix2", createJSON("r_anker", "t_1", "2026-09-24T18:00", 2))
			createRes = rec.Code
		}()
		wg.Wait()
		for _, st := range []int{batchRes, patchRes, createRes} {
			if st == 500 {
				t.Fatal("5xx under mixed concurrency")
			}
		}
		// No double occupancy anywhere.
		s2.mu.Lock()
		defer s2.mu.Unlock()
		confirmed := []Reservation{}
		for _, r := range s2.state.Reservations {
			if r.Status == StatusConfirmed {
				confirmed = append(confirmed, r)
			}
		}
		// Direct pairwise overlap check: no double occupancy anywhere.
		for i := range confirmed {
			for j := i + 1; j < len(confirmed); j++ {
				a, b := confirmed[i], confirmed[j]
				if a.RestaurantID != b.RestaurantID || a.TableID != b.TableID {
					continue
				}
				as, _ := parseStoredInstant(a.StartsAt)
				ae, _ := parseStoredInstant(a.EndsAt)
				bs, _ := parseStoredInstant(b.StartsAt)
				be, _ := parseStoredInstant(b.EndsAt)
				if as.Before(be) && bs.Before(ae) {
					t.Fatalf("double occupancy: %v vs %v", a.Reference, b.Reference)
				}
			}
		}
	})
}
