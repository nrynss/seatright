package service

// S3-I2 focused source tests: complete modern C roundtrip (policies,
// fixture0 above-maxima seed, pair history, adopted series, PATCH exception
// then cancel on the SAME member, mixed-member collective batch) and
// detached-state equality. Names TestVersionTransfer*.

import (
	"encoding/json"
	"net/http"
	"testing"
)

// transferReceipt is one captured first-201 original: key, path, canonical
// request body and exact stored response bytes.
type transferReceipt struct {
	key      string
	path     string
	body     string
	response string
}

// transferModern builds a rich modern producer snapshot through the real APIs
// and returns the service, owner token, series ids, and the captured
// first-201 raw receipts (publish/create/adopt/batch, in call order).
func transferModern(t *testing.T) (*Service, string, []string, []transferReceipt) {
	t.Helper()
	var receipts []transferReceipt
	var tok string
	s := New()
	// capture performs the write through genuine first HTTP bytes (serveRequest
	// with Idempotency-Key) and records the exact 201 response bytes as the
	// immutable original receipt.
	capture := func(key, path string, raw []byte) Result {
		t.Helper()
		hdr := map[string]string{
			"Authorization":   "Bearer " + tok,
			"Idempotency-Key": key,
		}
		rec := serveRequest(s, http.MethodPost, path, raw, hdr)
		if rec.Code != 201 {
			t.Fatalf("write %s %s: %d %q", path, key, rec.Code, rec.Body.String())
		}
		receipts = append(receipts, transferReceipt{key: key, path: path, body: string(raw), response: rec.Body.String()})
		var res Result
		if err := json.Unmarshal(rec.Body.Bytes(), &res.Body); err != nil {
			var v any
			_ = json.Unmarshal(rec.Body.Bytes(), &v)
			res.Body = v
		}
		res.Status = 201
		return res
	}
	// rbig carries fixture0 values above publication maxima (cutoff 10081,
	// table cap 101): proves above-maxima fixture terms survive the roundtrip.
	// Existing I1 shapes are reused; no new product behavior is invented.
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[
			{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
			"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
			"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
			"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
			"combinable":[["t_1","t_2"]],
			"manager_user_ids":["u1"]},
			{"id":"rbig","name":"Big","timezone":"Europe/Berlin","slot_minutes":30,
			"reservation_duration_minutes":90,"cancellation_cutoff_minutes":10081,
			"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
			"tables":[{"id":"b1","label":"1","capacity":101}]}],
		"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok = versionLoginToken(t, s)
	pub := func(from, key string, dur int) {
		t.Helper()
		raw := []byte(`{"effective_from":"` + from + `","slot_minutes":30,"reservation_duration_minutes":` + itoa(dur) + `,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`)
		capture(key, "/restaurants/r1/policies", raw)
	}
	pub("2027-05-13", "pol1", 120)
	pub("2027-05-13", "pol2", 60)
	create := func(key, body string) string {
		t.Helper()
		raw := []byte(body)
		res := capture(key, "/reservations", raw)
		return res.Body.(map[string]any)["reference"].(string)
	}
	patch := func(ref, body string) {
		t.Helper()
		if rec := serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(body),
			map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
			t.Fatalf("patch %s: %d %s", ref, rec.Code, rec.Body.String())
		}
	}
	// Above-maxima booking on rbig (party 101, cutoff 10081): retained verbatim.
	bigRef := create("t-big", `{"restaurant_id":"rbig","table_id":"b1","starts_at_local":"2027-05-06T19:00","party_size":101}`)
	_ = bigRef
	// Pair history: pair -> singleton -> pair.
	pairRef := create("t-pair", `{"restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T19:00","party_size":2}`)
	patch(pairRef, `{"table_id":"t_1"}`)
	patch(pairRef, `{"table_ids":["t_1","t_2"]}`)
	// Two anchors adopted as two series.
	a1 := create("t-a1", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T20:30","party_size":1}`)
	a2 := create("t-a2", `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T21:30","party_size":1}`)
	adopt := func(key, anchor string) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"anchor_reference": anchor, "count": 2, "interval_weeks": 1})
		res := capture(key, "/series", raw)
		return res.Body.(map[string]any)["series_id"].(string)
	}
	sid1 := adopt("t-s1", a1)
	sid2 := adopt("t-s2", a2)
	members := func(sid string) []map[string]any {
		t.Helper()
		got := s.GetSeries(tok, sid)
		if got.Status != 200 {
			t.Fatalf("series GET: %d", got.Status)
		}
		var out []map[string]any
		for _, o := range got.Body.(map[string]any)["occurrences"].([]any) {
			out = append(out, o.(map[string]any))
		}
		return out
	}
	m1, m2 := members(sid1), members(sid2)
	gen1 := m1[1]["reference"].(string)
	gen2 := m2[1]["reference"].(string)
	// PATCH exception on gen1, then cancel the SAME member: the permanent
	// true flag must survive cancellation.
	patch(gen1, `{"party_size":2}`)
	s.mu.Lock()
	excBefore := false
	for _, m := range s.state.Series[sid1].Members {
		if m.Reference == gen1 {
			excBefore = m.Exception
		}
	}
	s.mu.Unlock()
	if !excBefore {
		t.Fatal("gen1 must be a permanent exception after PATCH")
	}
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+gen1+"/cancel", []byte("{}"),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel gen1: %d %s", rec.Code, rec.Body.String())
	}
	s.mu.Lock()
	excAfter := false
	for _, m := range s.state.Series[sid1].Members {
		if m.Reference == gen1 {
			excAfter = m.Exception
		}
	}
	s.mu.Unlock()
	if !excAfter {
		t.Fatal("permanent exception flag lost after cancel")
	}
	// Cancel gen2 (no prior exception): flag stays false, revision still bumps.
	s.mu.Lock()
	srev2Before := s.state.Series[sid2].Revision
	s.mu.Unlock()
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+gen2+"/cancel", []byte("{}"),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel gen2: %d %s", rec.Code, rec.Body.String())
	}
	s.mu.Lock()
	srev2After := s.state.Series[sid2].Revision
	excGen2 := false
	for _, m := range s.state.Series[sid2].Members {
		if m.Reference == gen2 {
			excGen2 = m.Exception
		}
	}
	s.mu.Unlock()
	if srev2After != srev2Before+1 {
		t.Fatalf("cancel must bump series revision %d -> %d", srev2Before, srev2After)
	}
	if excGen2 {
		t.Fatal("cancel must not mark a new exception")
	}
	// Mixed-member collective batch across both series: gen1 is cancelled, so
	// use the anchor a1 (member 0 of sid1) plus anchor a2 (member 0 of sid2).
	// Both become permanent exceptions; each series +1, restaurant +1 total.
	s.mu.Lock()
	rev1 := s.state.RestaurantRevisions["r1"]
	srev1 := s.state.Series[sid1].Revision
	srev2 := s.state.Series[sid2].Revision
	s.mu.Unlock()
	batchRaw := []byte(`{"moves":[{"reference":` + quoted(a1) + `,"party_size":2},{"reference":` + quoted(a2) + `,"party_size":2}]}`)
	capture("t-batch", "/reservation-moves", batchRaw)
	s.mu.Lock()
	rev2 := s.state.RestaurantRevisions["r1"]
	nrev1 := s.state.Series[sid1].Revision
	nrev2 := s.state.Series[sid2].Revision
	excA1, excA2 := false, false
	for _, m := range s.state.Series[sid1].Members {
		if m.Reference == a1 && m.Exception {
			excA1 = true
		}
	}
	for _, m := range s.state.Series[sid2].Members {
		if m.Reference == a2 && m.Exception {
			excA2 = true
		}
	}
	// Per-booking: each changed booking gains exactly one revision + one entry.
	hA1 := len(s.state.Histories[a1])
	hA2 := len(s.state.Histories[a2])
	s.mu.Unlock()
	if rev2 != rev1+1 {
		t.Fatalf("restaurant revision %d -> %d, want exactly +1", rev1, rev2)
	}
	if nrev1 != srev1+1 || nrev2 != srev2+1 {
		t.Fatalf("series revisions %d/%d -> %d/%d, want each +1", srev1, srev2, nrev1, nrev2)
	}
	if !excA1 || !excA2 {
		t.Fatal("batch members must be permanent exceptions")
	}
	// Later mutation AFTER all receipt captures: a1 party 2->1 (rev3), so every
	// captured original provably differs from current.
	patch(a1, `{"party_size":1}`)
	if hA1 != 2 || hA2 != 2 {
		t.Fatalf("changed bookings need created+changed entries, got %d/%d", hA1, hA2)
	}
	return s, tok, []string{sid1, sid2}, receipts
}

// transferReceiptPaths returns the distinct receipt paths in capture order.
func transferReceiptPaths(rs []transferReceipt) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range rs {
		if !seen[r.path] {
			seen[r.path] = true
			out = append(out, r.path)
		}
	}
	return out
}

// TestVersionTransferModernRoundtrip exports a complete modern C snapshot,
// imports it into a fresh service, and requires full byte identity plus
// stable reimport. It also asserts the captured first-201 receipts cover all
// four write paths and the above-maxima seed survives verbatim.
func TestVersionTransferModernRoundtrip(t *testing.T) {
	s, tok, sids, receipts := transferModern(t)
	if len(sids) != 2 {
		t.Fatalf("series = %d, want 2", len(sids))
	}
	paths := transferReceiptPaths(receipts)
	if len(paths) != 4 {
		t.Fatalf("receipt paths = %v, want 4 write paths", paths)
	}
	// Exactly 9 original receipts: 2 pubs + 4 creates (big/pair/a1/a2) + 2 adopts + 1 batch.
	if len(receipts) != 9 {
		t.Fatalf("receipts = %d, want exactly 9", len(receipts))
	}
	// Bind the mk-a1 original by reference: party1/rev1 at creation; after the
	// batch it is party2/rev2; after the later mutation party1/rev3. Same identity.
	var a1Ref string
	s.mu.Lock()
	for ref, res := range s.state.Reservations {
		if res.RestaurantID == "r1" && res.StartsAtLocal == "2027-05-06T20:30" {
			a1Ref = ref
		}
	}
	a1Cur := s.state.Reservations[a1Ref]
	s.mu.Unlock()
	var a1Orig *transferReceipt
	for i, r := range receipts {
		if r.path == "/reservations" && r.key == "t-a1" {
			a1Orig = &receipts[i]
		}
	}
	if a1Orig == nil {
		t.Fatal("mk-a1 original receipt not captured")
	}
	var origBody map[string]any
	if err := json.Unmarshal([]byte(a1Orig.response), &origBody); err != nil {
		t.Fatalf("original receipt not JSON: %v", err)
	}
	if origBody["reference"] != a1Ref || origBody["party_size"] != float64(1) || origBody["revision"] != float64(1) {
		t.Fatalf("mk-a1 original = party %v rev %v, want 1/1", origBody["party_size"], origBody["revision"])
	}
	if a1Cur.PartySize != 1 || a1Cur.Revision != 3 {
		t.Fatalf("a1 current = party %d rev %d, want 1/3 (batch 2 + later 1)", a1Cur.PartySize, a1Cur.Revision)
	}
	if a1Cur.ReservationID == "" || origBody["reservation_id"] != a1Cur.ReservationID {
		t.Fatal("a1 identity changed across batch/mutation")
	}
	var batchResp map[string]any
	for _, r := range receipts {
		if r.key == "t-batch" {
			if err := json.Unmarshal([]byte(r.response), &batchResp); err != nil {
				t.Fatalf("batch receipt not JSON: %v", err)
			}
		}
	}
	if batchResp == nil {
		t.Fatal("mk-batch original receipt not captured")
	}
	foundBatch := false
	for _, item := range batchResp["reservations"].([]any) {
		im := item.(map[string]any)
		if im["reference"] == a1Ref {
			foundBatch = true
			if im["party_size"] != float64(2) || im["revision"] != float64(2) {
				t.Fatalf("batch a1 = party %v rev %v, want 2/2", im["party_size"], im["revision"])
			}
		}
	}
	if !foundBatch {
		t.Fatal("a1 missing from batch receipt")
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern C export/import not byte-identical")
	}
	// Above-maxima seed terms survive verbatim (cutoff 10081, cap 101).
	dst.mu.Lock()
	bigOK := false
	for _, res := range dst.state.Reservations {
		if res.RestaurantID == "rbig" && res.AcceptedTerms.CancellationCutoffMinutes == 10081 &&
			res.AcceptedTerms.Capacities["b1"] == 101 {
			bigOK = true
		}
	}
	dst.mu.Unlock()
	if !bigOK {
		t.Fatal("above-maxima fixture terms lost in transfer")
	}
	// Replay every captured original via actual HTTP with Idempotency-Key:
	// 200 + exact original bytes, and the whole export unchanged by replays.
	preReplay := string(exportBytes(t, dst))
	for _, r := range receipts {
		hdr := map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": r.key}
		rec := serveRequest(dst, http.MethodPost, r.path, []byte(r.body), hdr)
		if rec.Code != 200 {
			t.Fatalf("replay %s %s = %d", r.path, r.key, rec.Code)
		}
		if rec.Body.String() != r.response {
			t.Fatalf("replay %s %s bytes differ", r.path, r.key)
		}
	}
	if got := string(exportBytes(t, dst)); got != preReplay {
		t.Fatal("replays mutated the import")
	}
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("reimport: %d", rec.Code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern C reimport not stable")
	}
}

// TestVersionTransferDetachedEquality proves the imported state is a detached
// clone: a real party-changing PATCH on the destination must change its
// record, revision, history length and restaurant counter plus its export
// bytes, while the source full raw export stays byte-identical.
func TestVersionTransferDetachedEquality(t *testing.T) {
	s, tok, _, _ := transferModern(t)
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	// Choose a guaranteed confirmed record whose party+1 fits its selected
	// capacity (deterministic under map order: prefer the pair record).
	var target string
	var newParty int
	dst.mu.Lock()
	for ref, res := range dst.state.Reservations {
		if res.Status != StatusConfirmed || res.RestaurantID != "r1" {
			continue
		}
		cap := 0
		for _, id := range reservationTableIDs(res) {
			cap += res.AcceptedTerms.Capacities[id]
		}
		if res.PartySize+1 <= cap && (target == "" || len(reservationTableIDs(res)) == 2) {
			target, newParty = ref, res.PartySize+1
			if len(reservationTableIDs(res)) == 2 {
				break
			}
		}
	}
	beforeRec := dst.state.Reservations[target]
	beforeHist := len(dst.state.Histories[target])
	beforeCounter := dst.state.RestaurantRevisions["r1"]
	dst.mu.Unlock()
	if target == "" {
		t.Fatal("no confirmed r1 record with headroom on destination")
	}
	// Party-only PATCH keeps occupancy; must be a real change (200 + metadata).
	rec := serveRequest(dst, http.MethodPatch, "/reservations/"+target,
		[]byte(`{"party_size":`+itoa(newParty)+`}`),
		map[string]string{"Authorization": "Bearer " + tok})
	if rec.Code != 200 {
		t.Fatalf("destination PATCH: %d %s", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	afterRec := dst.state.Reservations[target]
	afterHist := len(dst.state.Histories[target])
	afterCounter := dst.state.RestaurantRevisions["r1"]
	dst.mu.Unlock()
	if afterRec.PartySize != newParty {
		t.Fatalf("destination record unchanged: party %d", afterRec.PartySize)
	}
	if afterRec.Revision != beforeRec.Revision+1 {
		t.Fatalf("destination revision %d -> %d, want +1", beforeRec.Revision, afterRec.Revision)
	}
	if afterHist != beforeHist+1 {
		t.Fatalf("destination history %d -> %d, want +1", beforeHist, afterHist)
	}
	if afterCounter != beforeCounter+1 {
		t.Fatalf("PATCH must bump restaurant counter exactly once %d -> %d", beforeCounter, afterCounter)
	}
	if got := string(exportBytes(t, dst)); got == before {
		t.Fatal("destination export identical after a real write")
	}
	// Source full raw export is unchanged by the destination write.
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("source export changed by a destination write")
	}
}
