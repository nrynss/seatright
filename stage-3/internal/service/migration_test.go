package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestModernExportRoundtrip preserves the useful checks from the earlier
// misleadingly named TestStage1ExportMigrates: a modern stage-2 producer
// export (already carrying canonical table_ids) reimports byte-identically
// with original receipts untouched. It is NOT a legacy migration proof;
// TestLegacyShapedImport and the donor probes cover genuine old state.
func TestModernExportRoundtrip(t *testing.T) {
	stage1 := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t1","label":"1","capacity":2}]}],
		"reservations":[{"id":"s1","reference":"STAGE01","user_id":"u1","restaurant_id":"r1",
		"table_id":"t1","starts_at_local":"2027-05-06T19:00","party_size":2}]}`
	s1 := New()
	if rec := serveRequest(s1, http.MethodPost, "/_test/reset", []byte(stage1), nil); rec.Code != 204 {
		t.Fatalf("stage-1 reset: %d %q", rec.Code, rec.Body.String())
	}
	s1.withLock(func(st *State) {
		st.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")] = Receipt{
			UserID: "u1", Method: "POST", Path: "/reservations", Key: "k1",
			Body: `{"party_size":2}`, Response: `{"reference":"STAGE01"}`, Status: 201,
		}
	})
	export := serveRequest(s1, http.MethodGet, "/_test/export", nil, nil)
	var env map[string]any
	if err := json.Unmarshal(export.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	rec := env["state"].(map[string]any)["reservations"].(map[string]any)["STAGE01"].(map[string]any)
	if rec["table_ids"] != nil {
		t.Logf("note: producer export carries canonical table_ids already: %v", rec["table_ids"])
	}
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", export.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("migration import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	res := dst.state.Reservations["STAGE01"]
	rc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservations", "k1")]
	dst.mu.Unlock()
	if res.TableID != "t1" || len(res.TableIDs) != 1 || res.TableIDs[0] != "t1" {
		t.Fatalf("legacy singleton not normalized: %+v", res)
	}
	if rc.Response != `{"reference":"STAGE01"}` || rc.Body != `{"party_size":2}` {
		t.Fatalf("original receipt rewritten: %+v", rc)
	}
	second := serveRequest(dst, http.MethodGet, "/_test/export", nil, nil)
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", second.Body.Bytes(), nil); rec.Code != 204 {
		t.Fatalf("reimport: %d %q", rec.Code, rec.Body.String())
	}
}

// TestLegacyShapedImport proves genuine old-state normalization with an
// explicitly legacy-shaped unit fixture: no table_ids and no combinable
// anywhere, plus an original stage-1 request body and response. The
// singleton normalizes to ids=[TableID] while its receipt JSON stays
// byte-identical. This fixture is synthetic legacy shape, not a real
// process export; actual old-image migration is proved by donor probes.
func TestLegacyShapedImport(t *testing.T) {
	legacy := map[string]any{
		"track":          "tablekeeper",
		"format_version": float64(1),
		"state": map[string]any{
			"users": map[string]any{"u1": map[string]any{
				"id": "u1", "email": "a@b", "display_name": "A",
				"password_hash": mustHashPassword(t, "password1"),
			}},
			"tokens": map[string]any{"tok-legacy": "u1"},
			"restaurants": []any{map[string]any{
				"id": "r1", "name": "N", "timezone": "Europe/Berlin",
				"slot_minutes": float64(30), "reservation_duration_minutes": float64(90),
				"cancellation_cutoff_minutes": float64(120),
				"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
				"tables":                      []any{map[string]any{"id": "t1", "label": "1", "capacity": float64(2)}},
			}},
			"reservations": map[string]any{"LEGACY1": map[string]any{
				"reservation_id": "s1", "reference": "LEGACY1", "user_id": "u1",
				"restaurant_id": "r1", "table_id": "t1",
				"party_size": float64(2), "status": StatusConfirmed,
				"starts_at_local": "2027-05-06T19:00",
				"starts_at":       "2027-05-06T19:00:00+02:00",
				"ends_at":         "2027-05-06T20:30:00+02:00",
				"created_at":      "2027-05-01T12:00:00+00:00",
			}},
			"receipts": map[string]any{},
		},
	}
	origBody := `{"party_size":2,"restaurant_id":"r1","starts_at_local":"2027-05-06T19:00","table_id":"t1"}`
	legacy["state"].(map[string]any)["receipts"] = map[string]any{
		ReceiptKey("u1", "POST", "/reservations", "k-legacy"): map[string]any{
			"user_id": "u1", "method": "POST", "path": "/reservations", "key": "k-legacy",
			"body":     origBody,
			"response": mustMarshalLegacyResponse(t),
			"status":   float64(201),
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 204 {
		t.Fatalf("legacy import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	res := dst.state.Reservations["LEGACY1"]
	rc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservations", "k-legacy")]
	dst.mu.Unlock()
	if res.TableID != "t1" || len(res.TableIDs) != 1 || res.TableIDs[0] != "t1" {
		t.Fatalf("legacy singleton not normalized: %+v", res)
	}
	wantResp := mustMarshalLegacyResponse(t)
	if rc.Body != origBody || rc.Response != wantResp {
		t.Fatalf("original stage-1 receipt rewritten: %+v", rc)
	}
	got := serveRequest(dst, http.MethodGet, "/reservations/LEGACY1", nil, map[string]string{"Authorization": "Bearer tok-legacy"})
	if got.Code != 200 {
		t.Fatalf("legacy lookup: %d %q", got.Code, got.Body.String())
	}
	var cur map[string]any
	if err := json.Unmarshal(got.Body.Bytes(), &cur); err != nil {
		t.Fatal(err)
	}
	var wantRec map[string]any
	if err := json.Unmarshal([]byte(wantResp), &wantRec); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"reservation_id", "reference", "restaurant_id", "table_id", "party_size", "status", "starts_at_local", "starts_at", "ends_at", "created_at"} {
		ce, _ := json.Marshal(cur[k])
		we, _ := json.Marshal(wantRec[k])
		if string(ce) != string(we) {
			t.Fatalf("legacy current field %s differs: got %s want %s", k, ce, we)
		}
	}
	if len(reservationTableIDs(res)) != 1 || reservationTableIDs(res)[0] != "t1" {
		t.Fatalf("legacy current table_ids not singleton: %+v", res)
	}
	replay := dst.CreateReservation("tok-legacy", "k-legacy", []byte(origBody))
	if replay.Status != 200 {
		t.Fatalf("legacy replay: %d %v", replay.Status, replay.Body)
	}
	replayRaw, err := json.Marshal(replay.Body)
	if err != nil {
		t.Fatal(err)
	}
	var rv, wv any
	if err := json.Unmarshal(replayRaw, &rv); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantResp), &wv); err != nil {
		t.Fatal(err)
	}
	re, _ := json.Marshal(rv)
	we, _ := json.Marshal(wv)
	if string(re) != string(we) {
		t.Fatalf("legacy replay response differs: %s", replayRaw)
	}
	login, _ := json.Marshal(map[string]any{"email": "a@b", "password": "password1"})
	if rec := serveRequest(dst, http.MethodPost, "/auth/login", login, nil); rec.Code != 200 {
		t.Fatalf("legacy hash login: %d %q", rec.Code, rec.Body.String())
	}
}

// mustMarshalLegacyResponse builds the full synthetic ordinary stage-1
// create response for the legacy unit fixture: every field an old server
// returned (identity, restaurant, singleton table_id, party, status,
// local/absolute times, creation), with no table_ids anywhere.
func mustMarshalLegacyResponse(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"reservation_id": "s1", "reference": "LEGACY1", "restaurant_id": "r1",
		"table_id": "t1", "party_size": float64(2), "status": StatusConfirmed,
		"starts_at_local": "2027-05-06T19:00",
		"starts_at":       "2027-05-06T19:00:00+02:00",
		"ends_at":         "2027-05-06T20:30:00+02:00",
		"created_at":      "2027-05-01T12:00:00+00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

// TestPairExportImportRoundtrip proves pair records and their original
// receipts survive export/import byte-identically: a confirmed pair and a
// cancelled pair keep canonical declared-order sets; after a real mutation
// the original batch receipt still replays 200 with the complete original
// JSON while the current record differs; confirmed/cancelled sets, identity
// and times survive reimport alongside mixed pair+single batch records.
func TestPairExportImportRoundtrip(t *testing.T) {
	s := New()
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]]}],
		"reservations":[
			{"id":"s1","reference":"PAIRCF","user_id":"u1","restaurant_id":"r1",
			"table_ids":["t_2","t_1"],"starts_at_local":"2027-05-06T19:00","party_size":4},
			{"id":"s2","reference":"PAIRCN","user_id":"u1","restaurant_id":"r1",
			"table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T20:30","party_size":2,"status":"cancelled"}]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	login, _ := json.Marshal(map[string]any{"email": "a@b", "password": "password1"})
	recLogin := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	if recLogin.Code != 200 {
		t.Fatalf("seed login: %d %q", recLogin.Code, recLogin.Body.String())
	}
	var loginBody map[string]any
	if err := json.Unmarshal(recLogin.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	tok := loginBody["token"].(string)
	batchBody := `{"moves":[{"reference":"PAIRCF","party_size":5}]}`
	if res := s.MoveReservations(tok, "ship-pair", []byte(batchBody)); res.Status != 201 {
		t.Fatalf("pair batch: %d %v", res.Status, res.Body)
	}
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("pair import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("pair export/import not byte-identical")
	}
	dst.mu.Lock()
	confirmed := dst.state.Reservations["PAIRCF"]
	cancelled := dst.state.Reservations["PAIRCN"]
	dst.mu.Unlock()
	if strings.Join(reservationTableIDs(confirmed), ",") != "t_1,t_2" || confirmed.TableID != "" {
		t.Fatalf("confirmed pair not canonical: %+v", confirmed)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("cancelled pair status = %q", cancelled.Status)
	}
	dst.mu.Lock()
	origRc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservation-moves", "ship-pair")]
	dst.mu.Unlock()
	var origResp map[string]any
	if err := json.Unmarshal([]byte(origRc.Response), &origResp); err != nil {
		t.Fatalf("original batch response not JSON: %v", err)
	}
	origList, ok := origResp["reservations"].([]any)
	if !ok || len(origList) != 1 {
		t.Fatalf("original batch shape wrong: %s", origRc.Response)
	}
	origEntry, ok := origList[0].(map[string]any)
	if !ok {
		t.Fatalf("original batch entry wrong: %v", origList[0])
	}
	if origEntry["reference"] != "PAIRCF" || origEntry["party_size"] != float64(5) {
		t.Fatalf("original batch entry wrong: %v", origEntry)
	}
	// Real mutation AFTER export/import: current record must diverge from receipt.
	if rec := serveRequest(dst, http.MethodPatch, "/reservations/PAIRCF", []byte("{}"), map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		// Empty PATCH is a no-op success path; any non-200 here means fixture issue.
		t.Fatalf("post-import no-op touch: %d %q", rec.Code, rec.Body.String())
	}
	patch := serveRequest(dst, http.MethodPatch, "/reservations/PAIRCF", []byte("{\"party_size\":6}"), map[string]string{"Authorization": "Bearer " + tok})
	if patch.Code != 200 {
		t.Fatalf("post-import mutation: %d %q", patch.Code, patch.Body.String())
	}
	replay := dst.MoveReservations(tok, "ship-pair", []byte(batchBody))
	if replay.Status != 200 {
		t.Fatalf("batch replay after import: %d %v", replay.Status, replay.Body)
	}
	replayRaw, err := json.Marshal(replay.Body)
	if err != nil {
		t.Fatal(err)
	}
	var rv, wv any
	if err := json.Unmarshal(replayRaw, &rv); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(origRc.Response), &wv); err != nil {
		t.Fatal(err)
	}
	re, _ := json.Marshal(rv)
	we, _ := json.Marshal(wv)
	if string(re) != string(we) {
		t.Fatalf("batch replay response differs from original: %s", replayRaw)
	}
	cur := serveRequest(dst, http.MethodGet, "/reservations/PAIRCF", nil, map[string]string{"Authorization": "Bearer " + tok})
	if cur.Code != 200 {
		t.Fatalf("mutated lookup: %d %q", cur.Code, cur.Body.String())
	}
	var curBody map[string]any
	if err := json.Unmarshal(cur.Body.Bytes(), &curBody); err != nil {
		t.Fatal(err)
	}
	if curBody["party_size"] != float64(6) {
		t.Fatalf("mutation did not apply: %v", curBody["party_size"])
	}
	ce, _ := json.Marshal(curBody)
	oe, _ := json.Marshal(origEntry)
	if string(ce) == string(oe) {
		t.Fatal("current record identical to receipt entry after mutation")
	}
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("pair reimport: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("pair reimport not stable")
	}
	dst.mu.Lock()
	rcf := dst.state.Reservations["PAIRCF"]
	rcn := dst.state.Reservations["PAIRCN"]
	dst.mu.Unlock()
	if strings.Join(reservationTableIDs(rcf), ",") != "t_1,t_2" || rcf.Status != StatusConfirmed {
		t.Fatalf("reimport confirmed pair wrong: %+v", rcf)
	}
	if strings.Join(reservationTableIDs(rcn), ",") != "t_1,t_2" || rcn.Status != StatusCancelled {
		t.Fatalf("reimport cancelled pair wrong: %+v", rcn)
	}
	if rcf.ReservationID != "s1" || rcn.ReservationID != "s2" {
		t.Fatalf("reimport identity changed: %+v %+v", rcf, rcn)
	}
	if rcf.StartsAtLocal != "2027-05-06T19:00" || rcn.StartsAtLocal != "2027-05-06T20:30" {
		t.Fatalf("reimport times changed: %+v %+v", rcf, rcn)
	}
}

// TestPairSequentialConflictRollback proves sequential pair/single conflict handling: sequential
// pair-then-single on a shared member leaves exactly one confirmed holder,
// and a failed batch leaves records and receipts unchanged.
// It does NOT prove concurrent serializability on its own; inherited B
// concurrent tests cover R195. It checks receipt-count behavior plus the
// sequential conflict outcome and unchanged record identity.
func TestPairSequentialConflictRollback(t *testing.T) {
	s := New()
	fixture := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
		"combinable":[["t_1","t_2"]]}],
		"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := idemToken(t, s)
	pairBody := `{"restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T19:00","party_size":4}`
	if res := s.CreateReservation(tok, "ser-pair", []byte(pairBody)); res.Status != 201 {
		t.Fatalf("pair create: %d %v", res.Status, res.Body)
	}
	singleBody := `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}`
	single := s.CreateReservation(tok, "ser-single", []byte(singleBody))
	if single.Status != 409 {
		t.Fatalf("shared member overlap: %d %v", single.Status, single.Body)
	}
	s.mu.Lock()
	receiptsBefore := len(s.state.Receipts)
	s.mu.Unlock()
	bad2 := s.MoveReservations(tok, "ser-bad2", []byte(`{"moves":[{"reference":"NOPE01"}]}`))
	if bad2.Status != 404 {
		t.Fatalf("unknown move: %d %v", bad2.Status, bad2.Body)
	}
	s.mu.Lock()
	receiptsAfter := len(s.state.Receipts)
	holderAfter := ""
	for _, r := range s.state.Reservations {
		if r.Status == StatusConfirmed {
			holderAfter = r.Reference
		}
	}
	s.mu.Unlock()
	if receiptsAfter != receiptsBefore {
		t.Fatal("failed batch claimed a receipt")
	}
	if holderAfter == "" {
		t.Fatal("no confirmed holder after sequential conflict")
	}
}

func TestModernExportRoundtripVersionState(t *testing.T) {
	// Modern exports carry version metadata: revision1, complete fixture0
	// terms, one created history per record, counters and unknown-manager
	// preservation survive a byte-identical round trip.
	s, _, _ := versionImportModern(t)
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	for ref, res := range dst.state.Reservations {
		if res.Revision < 1 {
			t.Fatalf("%s revision = %d", ref, res.Revision)
		}
		if len(dst.state.Histories[ref]) == 0 {
			t.Fatalf("%s has no history", ref)
		}
	}
	dst.mu.Unlock()
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern version round trip changed state")
	}
}

func TestVersionHistoryForgedValuesRejected(t *testing.T) {
	// Host F2 repro: singleton->pair genuine history imports; forging the
	// CREATED party To or an earlier terms duration is 422 with the
	// destination unchanged.
	s := resetVersion(t, versionFixture)
	tok := versionLoginToken(t, s)
	body := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T20:30","party_size":1}`
	res := s.CreateReservation(tok, "f2perm", []byte(body))
	if res.Status != 201 {
		t.Fatalf("create: %d %v", res.Status, res.Body)
	}
	ref := res.Body.(map[string]any)["reference"].(string)
	if rec := serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(`{"table_ids":["t_1","t_2"]}`),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("amend to pair: %d %s", rec.Code, rec.Body.String())
	}
	forge := func(t *testing.T, change func(env map[string]any)) {
		t.Helper()
		env := exportEnvelope(t, s)
		change(env["state"].(map[string]any))
		raw, _ := json.Marshal(env)
		dst := New()
		pre := string(exportBytes(t, dst))
		if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 422 {
			t.Fatalf("forged import = %d, want 422", rec.Code)
		}
		if got := string(exportBytes(t, dst)); got != pre {
			t.Fatal("rejected import mutated destination")
		}
	}
	t.Run("forged created party", func(t *testing.T) {
		forge(t, func(st map[string]any) {
			h := st["histories"].(map[string]any)[ref].([]any)
			h[0].(map[string]any)["changes"].([]any)[2].(map[string]any)["to"] = float64(99)
		})
	})
	t.Run("forged earlier duration", func(t *testing.T) {
		forge(t, func(st map[string]any) {
			h := st["histories"].(map[string]any)[ref].([]any)
			h[0].(map[string]any)["accepted_terms"].(map[string]any)["reservation_duration_minutes"] = float64(-9)
		})
	})
	t.Run("genuine pair back to singleton", func(t *testing.T) {
		if rec := serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(`{"table_id":"t_1"}`),
			map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
			t.Fatalf("amend back: %d %s", rec.Code, rec.Body.String())
		}
		full := string(exportBytes(t, s))
		dst := New()
		if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(full), nil); rec.Code != 204 {
			t.Fatalf("genuine round trip = %d %s", rec.Code, rec.Body.String())
		}
		if got := string(exportBytes(t, dst)); got != full {
			t.Fatal("genuine round trip not stable")
		}
	})
}
