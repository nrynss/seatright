package service

// S4-W integration: genuine repair→amend workflows through the real HTTP
// router (reset/login/create/adopt/preview/apply/amend/history/export).
// No stored-state edits and no synthetic table-assignment seams: every move
// comes from an actual Preview+Apply, every clock change from AmendSeries.

import (
	"encoding/json"
	"net/http"
	"testing"
)

const repairAmendFixture = `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
	"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
	"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
	"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
	"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
	"combinable":[["t_1","t_2"]],
	"manager_user_ids":["u1"]}],
	"reservations":[]}`

func repairHTTPLogin(t *testing.T, s *Service) string {
	t.Helper()
	lr := serveRequest(s, http.MethodPost, "/auth/login",
		[]byte(`{"email":"a@b","password":"password1"}`), nil)
	if lr.Code != 200 {
		t.Fatalf("login: %d %q", lr.Code, lr.Body.String())
	}
	var lm map[string]any
	if err := json.Unmarshal(lr.Body.Bytes(), &lm); err != nil {
		t.Fatal(err)
	}
	return lm["token"].(string)
}

// repairAmendWorld builds the shared world through HTTP only: an adopted
// 3-member series plus one actually applied closure repair moving the middle
// member off t_2 onto t_1. It returns the service, owner token, series id,
// member references in index order, the applied plan id and the raw original
// adoption response bytes.
func repairAmendWorld(t *testing.T) (*Service, string, string, []string, string, string) {
	t.Helper()
	s := New()
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(repairAmendFixture), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := repairHTTPLogin(t, s)
	auth := func(key string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": key}
	}
	ar := serveRequest(s, http.MethodPost, "/reservations",
		[]byte(`{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}`),
		auth("rw-anchor"))
	if ar.Code != 201 {
		t.Fatalf("anchor: %d %q", ar.Code, ar.Body.String())
	}
	var ab map[string]any
	if err := json.Unmarshal(ar.Body.Bytes(), &ab); err != nil {
		t.Fatal(err)
	}
	adopt, _ := json.Marshal(map[string]any{"anchor_reference": ab["reference"], "count": 3, "interval_weeks": 1})
	sr := serveRequest(s, http.MethodPost, "/series", adopt, auth("rw-adopt"))
	if sr.Code != 201 {
		t.Fatalf("adopt: %d %q", sr.Code, sr.Body.String())
	}
	adoptRaw := sr.Body.String()
	var sb map[string]any
	if err := json.Unmarshal([]byte(adoptRaw), &sb); err != nil {
		t.Fatal(err)
	}
	sid, _ := sb["series_id"].(string)
	var refs []string
	for _, o := range sb["occurrences"].([]any) {
		refs = append(refs, o.(map[string]any)["reference"].(string))
	}
	// Real closure repair over the middle member's evening on t_2.
	pv := serveRequest(s, http.MethodPost, "/restaurants/r1/replans",
		[]byte(`{"table_id":"t_2","from":"2027-05-13T18:00:00+02:00","to":"2027-05-13T23:00:00+02:00"}`),
		auth("rw-pv"))
	if pv.Code != 201 {
		t.Fatalf("preview: %d %q", pv.Code, pv.Body.String())
	}
	var pm map[string]any
	if err := json.Unmarshal(pv.Body.Bytes(), &pm); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(pm["assignments"])
	want, _ := json.Marshal([]any{map[string]any{"reference": refs[1], "table_ids": []any{"t_1"}, "changed": true}})
	if string(got) != string(want) {
		t.Fatalf("preview assignments: %s", got)
	}
	pid, _ := pm["plan_id"].(string)
	ap := serveRequest(s, http.MethodPost, "/restaurants/r1/replans/"+pid+"/apply", []byte(`{}`), auth("rw-ap"))
	if ap.Code != 201 {
		t.Fatalf("apply: %d %q", ap.Code, ap.Body.String())
	}
	var am map[string]any
	if err := json.Unmarshal(ap.Body.Bytes(), &am); err != nil {
		t.Fatal(err)
	}
	recs, _ := am["reservations"].([]any)
	if len(recs) != 1 {
		t.Fatalf("apply reservations: %v", am)
	}
	rm, _ := recs[0].(map[string]any)
	if rm["reference"] != refs[1] || rm["revision"] != 2.0 {
		t.Fatalf("moved record: %v", rm)
	}
	if tids, _ := json.Marshal(rm["table_ids"]); string(tids) != `["t_1"]` {
		t.Fatalf("moved tables: %s", tids)
	}
	if rm["party_size"] != 2.0 || rm["starts_at_local"] != "2027-05-13T19:00" {
		t.Fatalf("moved identity drift: %v", rm)
	}
	// Exactly rev+1 plus one reassigned entry with the full From/To and the
	// plan id; the old created prefix is preserved with frozen terms.
	hr := serveRequest(s, http.MethodGet, "/reservations/"+refs[1]+"/history", nil,
		map[string]string{"Authorization": "Bearer " + tok})
	if hr.Code != 200 {
		t.Fatalf("history: %d", hr.Code)
	}
	var hm map[string]any
	if err := json.Unmarshal(hr.Body.Bytes(), &hm); err != nil {
		t.Fatal(err)
	}
	es, _ := hm["entries"].([]any)
	if len(es) != 2 {
		t.Fatalf("moved history entries: %v", hm)
	}
	last, _ := es[1].(map[string]any)
	if last["event"] != "reassigned" || last["plan_id"] != pid || last["revision"] != 2.0 {
		t.Fatalf("reassigned entry: %v", last)
	}
	ch, _ := json.Marshal(last["changes"])
	if string(ch) != `[{"field":"table_ids","from":["t_2"],"to":["t_1"]}]` {
		t.Fatalf("reassigned change: %s", ch)
	}
	// Series bumped exactly once; every member keeps its scheduled date and
	// no repair invents an exception.
	st := httpExportState(t, s)
	sz := st["series"].(map[string]any)[sid].(map[string]any)
	if sz["revision"] != 2.0 {
		t.Fatalf("series revision: %v", sz["revision"])
	}
	for i, mm := range sz["members"].([]any) {
		m, _ := mm.(map[string]any)
		if m["index"] != float64(i) || m["exception"] != false {
			t.Fatalf("member drift: %v", m)
		}
	}
	dates := []string{"2027-05-06", "2027-05-13", "2027-05-20"}
	for i, mm := range sz["members"].([]any) {
		if mm.(map[string]any)["scheduled_date"] != dates[i] {
			t.Fatalf("scheduled drift: %v", mm)
		}
	}
	if st["restaurant_revisions"].(map[string]any)["r1"] != 3.0 {
		t.Fatalf("restaurant revision: %v", st["restaurant_revisions"])
	}
	return s, tok, sid, refs, pid, adoptRaw
}

func repairOccurrence(t *testing.T, body map[string]any, ref string) map[string]any {
	t.Helper()
	for _, o := range body["occurrences"].([]any) {
		om, _ := o.(map[string]any)
		if om["reference"] == ref {
			return om
		}
	}
	t.Fatalf("occurrence %s missing", ref)
	return nil
}

func TestRepairSeriesAmendWorkflow(t *testing.T) {
	s, tok, sid, refs, _, adoptRaw := repairAmendWorld(t)
	auth := func(key string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": key}
	}
	// A dated policy for the middle member's date: duration 60 there, so a
	// later amend must adopt per-date terms while older entries stay frozen.
	pol := `{"effective_from":"2027-05-13","slot_minutes":30,"reservation_duration_minutes":60,` +
		`"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],` +
		`"capacities":{"t_1":2,"t_2":4}}`
	if rec := serveRequest(s, http.MethodPost, "/restaurants/r1/policies", []byte(pol), auth("rw-pol")); rec.Code != 201 {
		t.Fatalf("publish: %d %q", rec.Code, rec.Body.String())
	}
	// Real clock amend over the suffix starting at the anchor, through the
	// NEW HTTP route. The repaired member keeps its current t_1 tables.
	ar := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":2,"from_index":0,"local_time":"19:30"}`), auth("rw-am1"))
	if ar.Code != 201 {
		t.Fatalf("amend: %d %q", ar.Code, ar.Body.String())
	}
	var am map[string]any
	if err := json.Unmarshal(ar.Body.Bytes(), &am); err != nil {
		t.Fatal(err)
	}
	if am["series_id"] != sid || am["revision"] != 3.0 {
		t.Fatalf("amend shape: %q", ar.Body.String())
	}
	wantStarts := map[string]string{refs[0]: "2027-05-06T19:30", refs[1]: "2027-05-13T19:30", refs[2]: "2027-05-20T19:30"}
	wantTables := map[string]string{refs[0]: `["t_2"]`, refs[1]: `["t_1"]`, refs[2]: `["t_2"]`}
	wantEnds := map[string]string{
		refs[0]: "2027-05-06T21:00:00+02:00", refs[1]: "2027-05-13T20:30:00+02:00", refs[2]: "2027-05-20T20:30:00+02:00",
	}
	wantPV := map[string]float64{refs[0]: 0, refs[1]: 1, refs[2]: 1}
	for ref, want := range wantStarts {
		occ := repairOccurrence(t, am, ref)
		if occ["exception"] != false {
			t.Fatalf("new exception on %s", ref)
		}
		rec, _ := occ["reservation"].(map[string]any)
		if rec["starts_at_local"] != want {
			t.Fatalf("%s clock: %v", ref, rec["starts_at_local"])
		}
		if tids, _ := json.Marshal(rec["table_ids"]); string(tids) != wantTables[ref] {
			t.Fatalf("%s tables: %s", ref, tids)
		}
		if rec["ends_at"] != wantEnds[ref] {
			t.Fatalf("%s ends: %v", ref, rec["ends_at"])
		}
		if rec["accepted_terms"].(map[string]any)["policy_version"] != wantPV[ref] {
			t.Fatalf("%s terms: %v", ref, rec["accepted_terms"])
		}
	}
	// Booking revisions: every member +1 (the repaired member 2→3).
	// Series and restaurant revisions: exactly once each for the operation.
	st := httpExportState(t, s)
	if st["series"].(map[string]any)[sid].(map[string]any)["revision"] != 3.0 {
		t.Fatalf("series revision: %v", st["series"])
	}
	if st["restaurant_revisions"].(map[string]any)["r1"] != 5.0 {
		t.Fatalf("restaurant revision: %v", st["restaurant_revisions"])
	}
	for ref, want := range map[string]float64{refs[0]: 2, refs[1]: 3, refs[2]: 2} {
		if st["reservations"].(map[string]any)[ref].(map[string]any)["revision"] != want {
			t.Fatalf("%s revision: %v", ref, st["reservations"])
		}
	}
	// The repaired member's history is created→reassigned→changed, with the
	// new Changed entry following the Reassigned one, clock-only From/To,
	// and every old entry keeping its frozen policy-0 terms.
	hr := serveRequest(s, http.MethodGet, "/reservations/"+refs[1]+"/history", nil,
		map[string]string{"Authorization": "Bearer " + tok})
	var hm map[string]any
	if err := json.Unmarshal(hr.Body.Bytes(), &hm); err != nil {
		t.Fatal(err)
	}
	es, _ := hm["entries"].([]any)
	if len(es) != 3 {
		t.Fatalf("repaired history: %v", hm)
	}
	evs := []string{es[0].(map[string]any)["event"].(string), es[1].(map[string]any)["event"].(string), es[2].(map[string]any)["event"].(string)}
	if evs[0] != "created" || evs[1] != "reassigned" || evs[2] != "changed" {
		t.Fatalf("history events: %v", evs)
	}
	ch, _ := json.Marshal(es[2].(map[string]any)["changes"])
	if string(ch) != `[{"field":"starts_at_local","from":"2027-05-13T19:00","to":"2027-05-13T19:30"}]` {
		t.Fatalf("changed entry: %s", ch)
	}
	for i, e := range es {
		em, _ := e.(map[string]any)
		if em["accepted_terms"].(map[string]any)["policy_version"] != []float64{0, 0, 1}[i] {
			t.Fatalf("entry %d terms drift: %v", i, em["accepted_terms"])
		}
	}
	// Members keep scheduled dates and exception flags; the original
	// adoption receipt still returns its old bytes.
	sz := st["series"].(map[string]any)[sid].(map[string]any)
	for i, mm := range sz["members"].([]any) {
		m, _ := mm.(map[string]any)
		if m["scheduled_date"] != []string{"2027-05-06", "2027-05-13", "2027-05-20"}[i] || m["exception"] != false {
			t.Fatalf("member drift: %v", m)
		}
	}
	rc := amendReceipt(t, st, "/series", "rw-adopt")
	if rc["response"] != adoptRaw {
		t.Fatal("adoption receipt changed")
	}
	// Same-key replay of the amend returns the original bytes even though
	// the world has moved on (publication happened before the amend here,
	// so mutate once more first to prove post-change replay).
	pr := serveRequest(s, http.MethodPatch, "/reservations/"+refs[0], []byte(`{"party_size":1}`),
		map[string]string{"Authorization": "Bearer " + tok})
	if pr.Code != 200 {
		t.Fatalf("later patch: %d", pr.Code)
	}
	origRaw := ar.Body.String()
	base, _ := json.Marshal(httpExportState(t, s))
	rp := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":2,"from_index":0,"local_time":"19:30"}`), auth("rw-am1"))
	if rp.Code != 200 || rp.Body.String() != origRaw {
		t.Fatalf("amend replay = %d", rp.Code)
	}
	after, _ := json.Marshal(httpExportState(t, s))
	if string(base) != string(after) {
		t.Fatal("amend replay changed state")
	}
}

func TestRepairSeriesAmendClosureConflict(t *testing.T) {
	s, tok, sid, refs, _, _ := repairAmendWorld(t)
	auth := func(key string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": key}
	}
	// Succeed one suffix amend first: everything to 19:30 on current tables.
	if rec := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":2,"from_index":0,"local_time":"19:30"}`), auth("rw-am1")); rec.Code != 201 {
		t.Fatalf("amend: %d %q", rec.Code, rec.Body.String())
	}
	// A zero-move closure on the repaired member's CURRENT table, outside
	// its currently occupied half-open interval [19:30,21:00).
	pv := serveRequest(s, http.MethodPost, "/restaurants/r1/replans",
		[]byte(`{"table_id":"t_1","from":"2027-05-13T21:00:00+02:00","to":"2027-05-13T21:30:00+02:00"}`),
		auth("rw-pv2"))
	if pv.Code != 201 {
		t.Fatalf("preview2: %d %q", pv.Code, pv.Body.String())
	}
	var pm map[string]any
	if err := json.Unmarshal(pv.Body.Bytes(), &pm); err != nil {
		t.Fatal(err)
	}
	if as, _ := json.Marshal(pm["assignments"]); string(as) != `[]` {
		t.Fatalf("zero-move assignments: %s", as)
	}
	if pm["moved_count"] != 0.0 {
		t.Fatalf("moved_count: %v", pm)
	}
	preRec, _ := json.Marshal(httpExportState(t, s))
	ap := serveRequest(s, http.MethodPost, "/restaurants/r1/replans/"+pm["plan_id"].(string)+"/apply", []byte(`{}`), auth("rw-ap2"))
	if ap.Code != 201 {
		t.Fatalf("apply2: %d %q", ap.Code, ap.Body.String())
	}
	var am2 map[string]any
	if err := json.Unmarshal(ap.Body.Bytes(), &am2); err != nil {
		t.Fatal(err)
	}
	if rs, _ := json.Marshal(am2["reservations"]); string(rs) != `[]` {
		t.Fatalf("zero-move apply reservations: %s", rs)
	}
	st := httpExportState(t, s)
	if st["restaurant_revisions"].(map[string]any)["r1"] != 5.0 {
		t.Fatalf("restaurant revision after zero-move: %v", st["restaurant_revisions"])
	}
	if st["series"].(map[string]any)[sid].(map[string]any)["revision"] != 3.0 {
		t.Fatalf("series drifted on zero-move: %v", st["series"])
	}
	postRec, _ := json.Marshal(st)
	var preE, postE map[string]any
	_ = json.Unmarshal(preRec, &preE)
	_ = json.Unmarshal(postRec, &postE)
	for _, k := range []string{"reservations", "histories", "series"} {
		xv, _ := json.Marshal(preE[k])
		yv, _ := json.Marshal(postE[k])
		if string(xv) != string(yv) {
			t.Fatalf("zero-move changed %s", k)
		}
	}
	// Requesting 20:30 puts the repaired member [20:30,22:00) across the
	// applied closure: 409 through the common seam, full export identity,
	// and no receipt for the failed key.
	base, _ := json.Marshal(httpExportState(t, s))
	fail := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":3,"from_index":0,"local_time":"20:30"}`), auth("rw-fail"))
	if st, code := errorCode(t, fail); st != 409 || code != "table_unavailable" {
		t.Fatalf("closure conflict = %d %s", st, code)
	}
	after, _ := json.Marshal(httpExportState(t, s))
	if string(base) != string(after) {
		t.Fatal("failed amend changed state")
	}
	for _, v := range httpExportState(t, s)["receipts"].(map[string]any) {
		if rm, _ := v.(map[string]any); rm["key"] == "rw-fail" {
			t.Fatal("failed key claimed a receipt")
		}
	}
	// The same failed key reused at adjacent, nonconflicting 21:30 is a
	// genuine 201 with an exact 200 replay.
	retry := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":3,"from_index":0,"local_time":"21:30"}`), auth("rw-fail"))
	if retry.Code != 201 {
		t.Fatalf("retry = %d %q", retry.Code, retry.Body.String())
	}
	retryRaw := retry.Body.String()
	var rm map[string]any
	if err := json.Unmarshal([]byte(retryRaw), &rm); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if repairOccurrence(t, rm, ref)["reservation"].(map[string]any)["starts_at_local"] != map[string]string{
			refs[0]: "2027-05-06T21:30", refs[1]: "2027-05-13T21:30", refs[2]: "2027-05-20T21:30",
		}[ref] {
			t.Fatalf("retry clock for %s", ref)
		}
	}
	if rp := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":3,"from_index":0,"local_time":"21:30"}`), auth("rw-fail")); rp.Code != 200 || rp.Body.String() != retryRaw {
		t.Fatalf("retry replay = %d", rp.Code)
	}
	// Exception and cancelled members stay excluded while the remaining
	// member still amends for real.
	if rec := serveRequest(s, http.MethodPatch, "/reservations/"+refs[2], []byte(`{"party_size":1}`),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("exception patch: %d", rec.Code)
	}
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+refs[2]+"/cancel", nil,
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel exception member: %d", rec.Code)
	}
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+refs[0]+"/cancel", nil,
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel plain member: %d", rec.Code)
	}
	mid := httpExportState(t, s)
	sz := mid["series"].(map[string]any)[sid].(map[string]any)
	if sz["revision"] != 7.0 {
		t.Fatalf("series revision before final: %v", sz["revision"])
	}
	flags := map[string]bool{}
	for _, mm := range sz["members"].([]any) {
		m, _ := mm.(map[string]any)
		flags[m["reference"].(string)] = m["exception"].(bool)
	}
	if !flags[refs[2]] || flags[refs[0]] || flags[refs[1]] {
		t.Fatalf("exception flags: %v", flags)
	}
	midB, _ := json.Marshal(mid)
	last := serveRequest(s, http.MethodPost, "/series/"+sid+"/amend",
		[]byte(`{"expected_revision":7,"from_index":0,"local_time":"18:00"}`), auth("rw-final"))
	if last.Code != 201 {
		t.Fatalf("final amend: %d %q", last.Code, last.Body.String())
	}
	end := httpExportState(t, s)
	if end["series"].(map[string]any)[sid].(map[string]any)["revision"] != 8.0 {
		t.Fatalf("final series revision: %v", end["series"])
	}
	var midR, endR map[string]any
	_ = json.Unmarshal(midB, &midR)
	endB, _ := json.Marshal(end)
	_ = json.Unmarshal(endB, &endR)
	for _, ref := range []string{refs[0], refs[2]} {
		xv, _ := json.Marshal(midR["reservations"].(map[string]any)[ref])
		yv, _ := json.Marshal(endR["reservations"].(map[string]any)[ref])
		if string(xv) != string(yv) {
			t.Fatalf("excluded record %s changed", ref)
		}
		xh, _ := json.Marshal(midR["histories"].(map[string]any)[ref])
		yh, _ := json.Marshal(endR["histories"].(map[string]any)[ref])
		if string(xh) != string(yh) {
			t.Fatalf("excluded history %s changed", ref)
		}
	}
	if endR["reservations"].(map[string]any)[refs[1]].(map[string]any)["starts_at_local"] != "2027-05-13T18:00" {
		t.Fatalf("remaining member did not amend: %v", endR["reservations"].(map[string]any)[refs[1]])
	}
}
