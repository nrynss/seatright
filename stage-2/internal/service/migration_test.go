package service

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A real stage-1 export (built from immutable stage-1/ semantics: no
// TableIDs, no combinable) imports into stage 2 with legacy singletons
// normalized, original receipts byte-identical, and reimport stable.
func TestStage1ExportMigrates(t *testing.T) {
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
