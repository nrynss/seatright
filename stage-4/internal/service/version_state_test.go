package service

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
	"testing"
)

// Requirement map (S3-M foundation only; no endpoint/policy-publication/
// series behavior claimed):
// R220 manager default/shape -> TestVersionManagerDefaults
// R237/R240 fixture config preserved -> TestVersionSeedDefaults
// R242/R243/R244 revision1 + complete policy0 terms, no effective_from -> TestVersionSeedDefaults
// R257 created entry null-from/ordered/one-entry + R290-R293 shape via history package -> TestVersionCreatedHistory
// R244 beyond-maxima fixture usable -> TestVersionBeyondMaximaProducer
// R245 receipts untouched + R91/R92 snapshot invariants -> TestVersionModernRoundtrip
// R286-R288/R21/R25/R43/R44 SYNTHETIC legacy normalization -> TestLegacyShapedVersionImport
// birth hook + counter + immutable retry + no-mutation-on-failure -> TestVersionBirthCounter
// deep-copy isolation across new nested types -> TestVersionSnapshotIsolation

const versionFixture = `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
	"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
	"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
	"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
	"combinable":[["t_1","t_2"]]}],
	"reservations":[
		{"id":"s1","reference":"SEEDCF","user_id":"u1","restaurant_id":"r1",
		"table_ids":["t_2","t_1"],"starts_at_local":"2027-05-06T19:00","party_size":4},
		{"id":"s2","reference":"SEEDCN","user_id":"u1","restaurant_id":"r1",
		"table_id":"t_2","starts_at_local":"2027-05-06T20:30","party_size":2,"status":"cancelled"},
		{"id":"s3","reference":"SEEDOFF","user_id":"u1","restaurant_id":"r1",
		"table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":99},
		{"id":"s4","reference":"SEEDOVER","user_id":"u1","restaurant_id":"r1",
		"table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":7}]}`

// Note: SEEDOFF sits off the 18:00 grid? No: 18:00 is on-grid from 18:00
// opening, but party 99 exceeds capacity — producer-valid off-rule seeds.

func resetVersion(t *testing.T, fixture string) *Service {
	t.Helper()
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	return s
}

func TestVersionSeedDefaults(t *testing.T) {
	s := resetVersion(t, versionFixture)
	s.mu.Lock()
	defer s.mu.Unlock()
	// Confirmed pair: canonical order, revision1, fixture0 terms.
	cf := s.state.Reservations["SEEDCF"]
	if strings.Join(reservationTableIDs(cf), ",") != "t_1,t_2" {
		t.Fatalf("seed pair not canonical: %+v", cf)
	}
	if cf.Revision != 1 {
		t.Fatalf("seed revision = %d", cf.Revision)
	}
	if cf.AcceptedTerms.PolicyVersion != 0 || cf.AcceptedTerms.SlotMinutes != 30 ||
		cf.AcceptedTerms.ReservationDurationMinutes != 90 || cf.AcceptedTerms.CancellationCutoffMinutes != 120 {
		t.Fatalf("seed terms = %+v", cf.AcceptedTerms)
	}
	if !reflect.DeepEqual(cf.AcceptedTerms.Capacities, map[string]int{"t_1": 2, "t_2": 4}) {
		t.Fatalf("seed capacities = %+v", cf.AcceptedTerms.Capacities)
	}
	// Cancelled singleton keeps cancelled/revision1.
	cn := s.state.Reservations["SEEDCN"]
	if cn.Status != StatusCancelled || cn.Revision != 1 {
		t.Fatalf("cancelled seed = %+v", cn)
	}
	// Off-rule seeds stay producer-valid: over-capacity accepted at seed.
	for _, ref := range []string{"SEEDOFF", "SEEDOVER"} {
		r := s.state.Reservations[ref]
		if r.Revision != 1 || r.Status != StatusConfirmed {
			t.Fatalf("%s = %+v", ref, r)
		}
	}
	// Public current record carries revision + complete terms, no effective_from.
	pub := cf.Public()
	if pub["revision"] != 1 {
		t.Fatalf("public revision = %v", pub["revision"])
	}
	raw, _ := json.Marshal(pub["accepted_terms"])
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"policy_version", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("terms missing %s: %s", k, raw)
		}
	}
	if _, ok := m["effective_from"]; ok {
		t.Fatalf("terms must exclude effective_from: %s", raw)
	}
	// Every seed has exactly one created history entry.
	for _, ref := range []string{"SEEDCF", "SEEDCN", "SEEDOFF", "SEEDOVER"} {
		if len(s.state.Histories[ref]) != 1 || s.state.Histories[ref][0].Seq != 1 {
			t.Fatalf("%s histories = %+v", ref, s.state.Histories[ref])
		}
	}
	// Counters begin at 0 after reset.
	if s.state.RestaurantRevisions["r1"] != 0 {
		t.Fatalf("counter = %d", s.state.RestaurantRevisions["r1"])
	}
}

func TestVersionManagerDefaults(t *testing.T) {
	s := resetVersion(t, versionFixture)
	s.mu.Lock()
	mgr := s.state.Restaurants[0].ManagerUserIDs
	s.mu.Unlock()
	if mgr == nil || len(mgr) != 0 {
		t.Fatalf("manager default must be allocated empty, got %#v", mgr)
	}
	raw, _ := json.Marshal(map[string]any{"m": mgr})
	if string(raw) != `{"m":[]}` {
		t.Fatalf("manager JSON = %s, want []", raw)
	}
	with := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}],
		"manager_user_ids":["u1"]}],
		"reservations":[]}`
	s2 := resetVersion(t, with)
	s2.mu.Lock()
	defer s2.mu.Unlock()
	if !reflect.DeepEqual(s2.state.Restaurants[0].ManagerUserIDs, []string{"u1"}) {
		t.Fatalf("managers = %#v", s2.state.Restaurants[0].ManagerUserIDs)
	}
	// Wrong manager type is malformed.
	bad := `{"users":[],"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[],"tables":[],"manager_user_ids":"u1"}],"reservations":[]}`
	if rec := serveRequest(New(), http.MethodPost, "/_test/reset", []byte(bad), nil); rec.Code != 400 {
		t.Fatalf("manager string: %d, want 400", rec.Code)
	}
	// Unknown manager ids are preserved verbatim (they simply cannot
	// authenticate as managers): producer contract, no invented ban.
	unknown := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}],
		"manager_user_ids":["ghost-1"]}],
		"reservations":[]}`
	s3 := resetVersion(t, unknown)
	s3.mu.Lock()
	kept := s3.state.Restaurants[0].ManagerUserIDs
	s3.mu.Unlock()
	if !reflect.DeepEqual(kept, []string{"ghost-1"}) {
		t.Fatalf("unknown managers not preserved: %#v", kept)
	}
	before := string(exportBytes(t, s3))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("unknown-manager import: %d", rec.Code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("unknown-manager round trip changed state")
	}
}

func TestVersionCreatedHistory(t *testing.T) {
	s := resetVersion(t, versionFixture)
	s.mu.Lock()
	defer s.mu.Unlock()
	// Singleton created entry: table_id scalar, ordered fields, from null.
	cn := s.state.Histories["SEEDCN"][0]
	if cn.Event != "created" || cn.Seq != 1 || cn.Revision != 1 {
		t.Fatalf("entry = %+v", cn)
	}
	if len(cn.Changes) != 3 || cn.Changes[0].Field != "table_id" ||
		cn.Changes[1].Field != "starts_at_local" || cn.Changes[2].Field != "party_size" {
		t.Fatalf("changes = %+v", cn.Changes)
	}
	for _, ch := range cn.Changes {
		if ch.From != nil {
			t.Fatalf("from must be null: %+v", ch)
		}
	}
	if cn.Changes[0].To != "t_2" {
		t.Fatalf("table change = %+v", cn.Changes[0])
	}
	// Pair created entry uses table_ids full canonical list.
	cf := s.state.Histories["SEEDCF"][0]
	if cf.Changes[0].Field != "table_ids" {
		t.Fatalf("pair field = %+v", cf.Changes[0])
	}
	ids, ok := cf.Changes[0].To.([]string)
	if !ok || !reflect.DeepEqual(ids, []string{"t_1", "t_2"}) {
		t.Fatalf("pair value = %#v", cf.Changes[0].To)
	}
	// Created-at instant preserved: entry at equals stored created_at.
	if cf.At != s.state.Reservations["SEEDCF"].CreatedAt {
		t.Fatalf("at=%q created=%q", cf.At, s.state.Reservations["SEEDCF"].CreatedAt)
	}
	// Terms frozen: entry terms equal current terms now; mutation isolation
	// covered in TestVersionSnapshotIsolation.
	var m map[string]any
	raw, _ := json.Marshal(cf.AcceptedTerms)
	if err := json.Unmarshal(raw, &m); err != nil || m["policy_version"] != float64(0) {
		t.Fatalf("entry terms = %s", raw)
	}
}

// seedVersionIsolationExtras adds one published policy and one series with
// populated members through the test-owned lock so isolation exercises every
// new nested type. No routes or behavior are involved.
func seedVersionIsolationExtras(t *testing.T, s *Service) {
	t.Helper()
	s.withLock(func(st *State) {
		st.Policies["r1"] = []policy.Policy{{EffectiveFrom: "2026-09-28", Terms: fixtureTerms(st.Restaurants[0])}}
		st.Policies["r1"][0].PolicyVersion = 1
		st.Series["sz1"] = Series{ID: "sz1", UserID: "u1", RestaurantID: "r1", Revision: 1, IntervalWeeks: 1,
			Members: []SeriesMember{
				{Index: 0, Reference: "SEEDCF", ScheduledDate: "2027-05-06", Exception: false},
				{Index: 1, Reference: "SEED02", ScheduledDate: "2027-05-13", Exception: false},
			}}
	})
}

// versionManagersFixture extends the version fixture with populated
// managers, a second combinable pair, one published policy and one series so
// isolation tests exercise every new nested type.
func versionManagersFixture() string {
	return verManagersFixture
}

const verManagersFixture = `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
	"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
	"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
	"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
	"combinable":[["t_1","t_2"]],
	"manager_user_ids":["u1"]}],
	"reservations":[
		{"id":"s1","reference":"SEEDCF","user_id":"u1","restaurant_id":"r1",
		"table_ids":["t_2","t_1"],"starts_at_local":"2027-05-06T19:00","party_size":4},
		{"id":"s2","reference":"SEED02","user_id":"u1","restaurant_id":"r1",
		"table_id":"t_2","starts_at_local":"2027-05-13T20:30","party_size":1}]}`

func TestVersionBeyondMaximaProducer(t *testing.T) {
	// Genuine above-publication-maxima fixture config stays usable: reset
	// accepts it and terms/Rules/Capacity retain it (no Parse re-restriction).
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":1441,
		"reservation_duration_minutes":1441,"cancellation_cutoff_minutes":10081,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":101}]}],
		"reservations":[]}`
	s := resetVersion(t, fix)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.state.Restaurants[0]
	tm := fixtureTerms(r)
	if tm.SlotMinutes != 1441 || tm.ReservationDurationMinutes != 1441 ||
		tm.CancellationCutoffMinutes != 10081 || tm.Capacities["t_1"] != 101 {
		t.Fatalf("fixture terms = %+v", tm)
	}
}

func TestVersionPartialMetadataRejected(t *testing.T) {
	// F1: positive revision with missing/empty/unusable terms or empty
	// history is rejected atomically; revision zero must not erase modern
	// terms/history. Genuine absent legacy shapes still normalize.
	base := resetVersion(t, versionFixture)
	mutate := func(t *testing.T, change func(env map[string]any)) {
		t.Helper()
		env := exportEnvelope(t, base)
		change(env["state"].(map[string]any))
		raw, _ := json.Marshal(env)
		dst := New()
		if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 422 {
			t.Fatalf("partial metadata import = %d, want 422", rec.Code)
		}
		if got := string(exportBytes(t, dst)); got == string(raw) {
			t.Fatal("rejected import must not publish the invalid state")
		}
	}
	t.Run("missing terms", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			res := st["reservations"].(map[string]any)["SEEDCF"].(map[string]any)
			delete(res, "accepted_terms")
		})
	})
	t.Run("zero duration terms", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			res := st["reservations"].(map[string]any)["SEEDCF"].(map[string]any)
			res["accepted_terms"].(map[string]any)["reservation_duration_minutes"] = float64(0)
		})
	})
	t.Run("missing capacities", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			res := st["reservations"].(map[string]any)["SEEDCF"].(map[string]any)
			delete(res["accepted_terms"].(map[string]any), "capacities")
		})
	})
	t.Run("empty history", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			st["histories"].(map[string]any)["SEEDCF"] = []any{}
		})
	})
	t.Run("revision zero keeps modern terms", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			st["reservations"].(map[string]any)["SEEDCF"].(map[string]any)["revision"] = float64(0)
		})
	})
	t.Run("revision zero keeps modern history", func(t *testing.T) {
		mutate(t, func(st map[string]any) {
			res := st["reservations"].(map[string]any)["SEEDCF"].(map[string]any)
			res["revision"] = float64(0)
			res["accepted_terms"] = map[string]any{}
		})
	})
	// Genuine publish+write selected v1 duration: publish a 60-minute policy
	// and create under it, so the stored v1 terms are producer-real rather
	// than hand-rewritten v0 bytes.
	t.Run("published v1 duration roundtrip", func(t *testing.T) {
		s := resetVersion(t, versionFixture)
		tok := versionLoginToken(t, s)
		s.mu.Lock()
		for i := range s.state.Restaurants {
			if s.state.Restaurants[i].ID == "r1" {
				s.state.Restaurants[i].ManagerUserIDs = []string{"u1"}
			}
		}
		s.mu.Unlock()
		pub := `{"effective_from":"2027-05-06","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`
		if res := s.PublishPolicy(tok, "r1", "v1dur", []byte(pub)); res.Status != 201 {
			t.Fatalf("publish: %d %v", res.Status, res.Body)
		}
		body := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T20:30","party_size":1}`
		if res := s.CreateReservation(tok, "v1c", []byte(body)); res.Status != 201 {
			t.Fatalf("create: %d %v", res.Status, res.Body)
		}
		ref := ""
		s.mu.Lock()
		for r, res := range s.state.Reservations {
			if r != "SEEDCF" && r != "SEEDCN" && r != "SEEDOFF" && r != "SEEDOVER" && res.StartsAtLocal == "2027-05-06T20:30" {
				ref = r
			}
		}
		s.mu.Unlock()
		if ref == "" {
			t.Fatal("v1 record not found")
		}
		s.mu.Lock()
		kept := s.state.Reservations[ref]
		s.mu.Unlock()
		if kept.AcceptedTerms.PolicyVersion != 1 || kept.AcceptedTerms.ReservationDurationMinutes != 60 {
			t.Fatalf("v1 terms = %+v", kept.AcceptedTerms)
		}
		before := string(exportBytes(t, s))
		dst := New()
		if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
			t.Fatalf("v1 import = %d", rec.Code)
		}
		if got := string(exportBytes(t, dst)); got != before {
			t.Fatal("v1 round trip changed state")
		}
	})
}

func TestVersionModernRoundtrip(t *testing.T) {
	s := resetVersion(t, versionFixture)
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d %q", rec.Code, rec.Body.String())
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern export/reimport not byte-identical")
	}
	// Receipts untouched: create + replay through the ordinary path.
	tok := versionLoginToken(t, dst)
	body := `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T20:30","party_size":1}`
	res := dst.CreateReservation(tok, "mkey", []byte(body))
	if res.Status != 201 {
		t.Fatalf("create = %d %v", res.Status, res.Body)
	}
	rep := dst.CreateReservation(tok, "mkey", []byte(body))
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	rb, _ := json.Marshal(res.Body)
	pb, _ := json.Marshal(rep.Body)
	if string(rb) != string(pb) {
		t.Fatal("replay differs from original")
	}
}

func versionLoginToken(t *testing.T, s *Service) string {
	t.Helper()
	login, _ := json.Marshal(map[string]any{"email": "a@b", "password": "password1"})
	rec := serveRequest(s, http.MethodPost, "/auth/login", login, nil)
	if rec.Code != 200 {
		t.Fatalf("login: %d %q", rec.Code, rec.Body.String())
	}
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	tok, _ := b["token"].(string)
	if tok == "" {
		t.Fatal("no token")
	}
	return tok
}

func TestVersionBirthCounter(t *testing.T) {
	s := resetVersion(t, versionFixture)
	tok := versionLoginToken(t, s)
	body := `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T20:30","party_size":1}`
	res := s.CreateReservation(tok, "birth1", []byte(body))
	if res.Status != 201 {
		t.Fatalf("birth = %d %v", res.Status, res.Body)
	}
	m, _ := res.Body.(map[string]any)
	ref, _ := m["reference"].(string)
	s.mu.Lock()
	rec := s.state.Reservations[ref]
	rev := s.state.RestaurantRevisions["r1"]
	hist := len(s.state.Histories[ref])
	s.mu.Unlock()
	if rec.Revision != 1 || hist != 1 {
		t.Fatalf("born metadata: %+v hist=%d", rec, hist)
	}
	if rev != 1 {
		t.Fatalf("counter = %d, want 1", rev)
	}
	if m["revision"] != 1 {
		t.Fatalf("public revision = %v", m["revision"])
	}
	// Immutable retry: replay changes nothing.
	rep := s.CreateReservation(tok, "birth1", []byte(body))
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	s.mu.Lock()
	rev2 := s.state.RestaurantRevisions["r1"]
	hist2 := len(s.state.Histories[ref])
	s.mu.Unlock()
	if rev2 != 1 || hist2 != 1 {
		t.Fatalf("replay mutated: rev=%d hist=%d", rev2, hist2)
	}
	// Failure claims nothing: conflict on the same table/slot.
	bad := s.CreateReservation(tok, "birth2", []byte(body))
	if bad.Status != 409 {
		t.Fatalf("conflict = %d", bad.Status)
	}
	s.mu.Lock()
	rev3 := s.state.RestaurantRevisions["r1"]
	nr := len(s.state.Receipts)
	nres := len(s.state.Reservations)
	var histEntries int
	for _, h := range s.state.Histories {
		histEntries += len(h)
	}
	s.mu.Unlock()
	if rev3 != 1 {
		t.Fatalf("failed write moved counter: %d", rev3)
	}
	if nr != 1 || nres != 5 || histEntries != 5 {
		t.Fatalf("failed write claimed state: receipts=%d reservations=%d histEntries=%d", nr, nres, histEntries)
	}

}

func TestVersionCloneNilEmptyShapes(t *testing.T) {
	// F2: cloneState/snapshot preserve exact nil-vs-empty shapes for the new
	// maps and nested slices; normal emptyState producer defaults stay
	// allocated.
	empty := emptyState()
	cl := cloneState(&empty)
	for _, tc := range []struct {
		name string
		got  any
	}{
		{"policies", cl.Policies}, {"histories", cl.Histories},
		{"series", cl.Series}, {"counters", cl.RestaurantRevisions},
	} {
		raw, _ := json.Marshal(map[string]any{"v": tc.got})
		_ = raw
	}
	if cl.Policies == nil || cl.Histories == nil || cl.Series == nil || cl.RestaurantRevisions == nil {
		t.Fatal("allocated emptyState maps must stay allocated after clone")
	}
	var nilSt State
	nilSt.Users = map[string]User{}
	nilSt.Tokens = map[string]string{}
	nilSt.Restaurants = []Restaurant{}
	nilSt.Reservations = map[string]Reservation{}
	nilSt.Receipts = map[string]Receipt{}
	// New maps left nil must stay nil through clone and JSON.
	nilCl := cloneState(&nilSt)
	if nilCl.Policies != nil || nilCl.Histories != nil || nilCl.Series != nil || nilCl.RestaurantRevisions != nil {
		t.Fatal("nil new maps must stay nil after clone")
	}
	before, _ := json.Marshal(nilSt)
	after, _ := json.Marshal(nilCl)
	if string(before) != string(after) {
		t.Fatalf("nil-state clone JSON differs:\n%s\n%s", before, after)
	}
	// Empty (non-nil) variants keep []/{} shapes, not null.
	emptySt := emptyState()
	emptySt.Policies["r1"] = []policy.Policy{}
	emptySt.Histories["k"] = []history.Entry{}
	emptySt.Series["s"] = Series{ID: "s", Members: []SeriesMember{}}
	emptyCl := cloneState(&emptySt)
	be, _ := json.Marshal(emptySt)
	ae, _ := json.Marshal(emptyCl)
	if string(be) != string(ae) {
		t.Fatalf("empty-state clone JSON differs:\n%s\n%s", be, ae)
	}
	if !strings.Contains(string(ae), `"members":[]`) {
		t.Fatalf("empty members must encode [], got %s", ae)
	}
}

func TestVersionSnapshotIsolation(t *testing.T) {
	s := resetVersion(t, verManagersFixture)
	seedVersionIsolationExtras(t, s)
	// Export path isolation: mutating an export envelope must not alias
	// stored nested state, and the stored export stays byte-stable.
	before := string(exportBytes(t, s))
	env := exportEnvelope(t, s)
	envState := env["state"].(map[string]any)
	resMap := envState["reservations"].(map[string]any)
	cfEnv := resMap["SEEDCF"].(map[string]any)
	cfTerms := cfEnv["accepted_terms"].(map[string]any)
	cfTerms["capacities"].(map[string]any)["t_1"] = float64(999)
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("export envelope aliases stored capacities")
	}
	// Clone path isolation across every new nested type: populate makes
	// aliasing observable, then mutate the clone and require the original
	// complete JSON to stay equal to pristine.
	snap := s.snapshot()
	snap.Reservations["SEEDCF"].AcceptedTerms.Capacities["t_1"] = 111
	snap.Reservations["SEEDCF"].AcceptedTerms.OpeningHours[0].Opens = "00:00"
	snap.Reservations["SEEDCF"].TableIDs[0] = "t_2"
	snap.Restaurants[0].ManagerUserIDs[0] = "uX"
	snap.Restaurants[0].Combinable[0][0] = "t_2"
	snap.Restaurants[0].Tables[0].Capacity = 999
	snap.Policies["r1"][0].Capacities["t_1"] = 999
	snap.Policies["r1"][0].OpeningHours[0].Opens = "00:00"
	snap.Histories["SEEDCF"][0].AcceptedTerms[10] = 'X'
	snap.Histories["SEEDCF"][0].Changes[0].To = []string{"t_2", "t_1"}
	snap.Series["sz1"].Members[0].Exception = true
	snap.Series["sz1"].Members[0].Reference = "MUT"
	snap.RestaurantRevisions["r1"] = 999
	pristine, _ := json.Marshal(s.state)
	mutated, _ := json.Marshal(snap)
	if string(pristine) == string(mutated) {
		t.Fatal("clone mutation had no effect; test is vacuous")
	}
	s.mu.Lock()
	againTerms := s.state.Reservations["SEEDCF"].AcceptedTerms.Capacities["t_1"]
	againMgr := s.state.Restaurants[0].ManagerUserIDs[0]
	againPair := s.state.Restaurants[0].Combinable[0][0]
	againHist := s.state.Histories["SEEDCF"][0].Changes[0].To
	againRev := s.state.RestaurantRevisions["r1"]
	s.mu.Unlock()
	if againTerms == 111 {
		t.Fatal("snapshot aliases stored capacities")
	}
	if againMgr != "u1" {
		t.Fatalf("snapshot aliases managers: %q", againMgr)
	}
	if againPair != "t_1" {
		t.Fatalf("snapshot aliases combinable inner pair: %q", againPair)
	}
	if ids, ok := againHist.([]string); !ok || !reflect.DeepEqual(ids, []string{"t_1", "t_2"}) {
		t.Fatalf("snapshot aliases history pair array: %#v", againHist)
	}
	if againRev == 999 {
		t.Fatal("snapshot aliases counters")
	}
	if got := string(exportBytes(t, s)); got != before {
		t.Fatal("clone mutation leaked into stored state")
	}
	// Export/import stability on the pristine state.
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("export/import not stable")
	}
}

// TestLegacyShapedVersionImport is explicitly SYNTHETIC legacy-shaped unit
// input (no table_ids, no managers, no version metadata anywhere): it proves
// old-shape normalization only. Real cross-version donor migration is proved
// by the independent donor lane, not claimed here.
func TestLegacyShapedVersionImport(t *testing.T) {
	legacy := map[string]any{
		"track": "tablekeeper", "format_version": float64(1),
		"state": map[string]any{
			"users": map[string]any{"u1": map[string]any{
				"id": "u1", "email": "a@b", "display_name": "A",
				"password_hash": mustTestHash(t, "password1"),
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
			"response": `{"reference":"LEGACY1"}`,
			"status":   float64(201),
		},
	}
	raw, _ := json.Marshal(legacy)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 204 {
		t.Fatalf("legacy import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	res := dst.state.Reservations["LEGACY1"]
	rc := dst.state.Receipts[ReceiptKey("u1", "POST", "/reservations", "k-legacy")]
	hist := dst.state.Histories["LEGACY1"]
	dst.mu.Unlock()
	if res.TableID != "t1" || len(res.TableIDs) != 1 || res.TableIDs[0] != "t1" {
		t.Fatalf("singleton not normalized: %+v", res)
	}
	if res.Revision != 1 || res.AcceptedTerms.SlotMinutes != 30 {
		t.Fatalf("legacy metadata = %+v", res)
	}
	if len(hist) != 1 || hist[0].Event != "created" {
		t.Fatalf("legacy history = %+v", hist)
	}
	if rc.Body != origBody || rc.Response != `{"reference":"LEGACY1"}` {
		t.Fatalf("receipt rewritten: %+v", rc)
	}
	// Tokens + hash login work; same-body/key replay returns original.
	if rec := serveRequest(dst, http.MethodGet, "/reservations/LEGACY1", nil,
		map[string]string{"Authorization": "Bearer tok-legacy"}); rec.Code != 200 {
		t.Fatalf("old token lookup: %d", rec.Code)
	}
	login, _ := json.Marshal(map[string]any{"email": "a@b", "password": "password1"})
	if rec := serveRequest(dst, http.MethodPost, "/auth/login", login, nil); rec.Code != 200 {
		t.Fatalf("hash login: %d", rec.Code)
	}
	rep := dst.CreateReservation("tok-legacy", "k-legacy", []byte(origBody))
	if rep.Status != 200 {
		t.Fatalf("replay = %d", rep.Status)
	}
	rb, _ := json.Marshal(rep.Body)
	if string(rb) != `{"reference":"LEGACY1"}` {
		t.Fatalf("replay body = %s", rb)
	}
	// Malformed import is atomic: destination unchanged.
	before := string(exportBytes(t, dst))
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte("{bad"), nil); rec.Code != 400 {
		t.Fatalf("malformed = %d", rec.Code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("malformed import mutated state")
	}
}

func mustTestHash(t *testing.T, password string) string {
	t.Helper()
	h, err := hashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
