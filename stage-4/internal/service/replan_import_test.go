package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Native producer/import tests for seating-repair state. All plans,
// closures, series and histories come from REAL producer operations on a
// fresh service — never hand-invented. Corruption cases mutate COPIES of a
// valid export and require 422 with the destination byte-identical.

// replanImportFixture builds a native stage-4 world: manager Ada, diners,
// dated policies, anchors, two series, individual exception/cancel and a
// cross-series batch — mirroring the donor workflow at service level.
const replanImportFixture = `{
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}
  ],
  "restaurants": [
    {
      "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [
        {"id": "t_1", "label": "1", "capacity": 2},
        {"id": "t_2", "label": "2", "capacity": 4},
        {"id": "t_3", "label": "3", "capacity": 4}
      ],
      "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
      "manager_user_ids": ["u_ada"]
    },
    {
      "id": "r_big", "name": "Big Hall", "timezone": "Europe/Berlin",
      "slot_minutes": 1441, "reservation_duration_minutes": 1441,
      "cancellation_cutoff_minutes": 10081,
      "opening_hours": [{"weekday": "thu", "opens": "00:00", "closes": "23:59"}],
      "tables": [{"id": "b_1", "label": "B1", "capacity": 101}],
      "combinable": [],
      "manager_user_ids": []
    }
  ],
  "reservations": [
    {"id": "seed-past", "reference": "SEDPST", "user_id": "u_ada",
     "restaurant_id": "r_anker", "table_id": "t_3",
     "starts_at_local": "2020-01-02T18:00", "party_size": 2},
    {"id": "seed-off", "reference": "SEDOFF", "user_id": "u_ada",
     "restaurant_id": "r_anker", "table_id": "t_2",
     "starts_at_local": "2020-01-02T18:07", "party_size": 2},
    {"id": "seed-ovr", "reference": "SEDOVR", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": "2020-01-02T20:30", "party_size": 9},
    {"id": "seed-cxd", "reference": "SEDCXD", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": "2020-01-02T18:00", "party_size": 2,
     "status": "cancelled"},
    {"id": "seed-max", "reference": "SEDMAX", "user_id": "u_bea",
     "restaurant_id": "r_big", "table_id": "b_1",
     "starts_at_local": "2020-01-09T00:00", "party_size": 101}
  ]
}`

// replanImportWorld drives the full native workflow and returns the service,
// tokens, plan ids, series ids and member references for import tests.
type replanImportWorld struct {
	s            *Service
	mgr, diner   string
	planMoved    string
	planEmpty    string
	planDup      string
	planStale    string
	seriesPair   string
	seriesSingle string
	genPair      string
	genSingle    string
	afterRef     string
	// first201 records genuine first-201 RAW response bytes per keyed write:
	// each entry is method, path, body string and raw response string.
	first201 map[string]firstWrite
}

type firstWrite struct {
	method, path, body, raw string
	token                   string
}

// captureHTTP performs a keyed write through the real HTTP router and
// records the exact recorder bytes as the first-201 raw response.
func (w *replanImportWorld) captureHTTP(t *testing.T, token, key, method, path, body string) {
	t.Helper()
	rec := serveRequest(w.s, method, path, []byte(body), map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": key,
	})
	if rec.Code != 201 {
		t.Fatalf("capture %s: %d", key, rec.Code)
	}
	w.first201[key] = firstWrite{method: method, path: path, body: body, raw: rec.Body.String(), token: token}
}

// planIDFromRaw reads plan_id from a preview raw response.
func planIDFromRaw(t *testing.T, raw string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("preview raw: %v", err)
	}
	id, _ := body["plan_id"].(string)
	if id == "" {
		t.Fatal("preview raw without plan_id")
	}
	return id
}

func (w *replanImportWorld) capture(t *testing.T, key, method, path, body string, res Result) {
	t.Helper()
	if res.Status != 201 {
		t.Fatalf("capture %s: %d", key, res.Status)
	}
	raw, err := json.Marshal(res.Body)
	if err != nil {
		t.Fatalf("marshal %s: %v", key, err)
	}
	w.first201[key] = firstWrite{method: method, path: path, body: body, raw: string(raw)}
}

func newReplanImportWorld(t *testing.T) *replanImportWorld {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(replanImportFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	w := &replanImportWorld{s: s, mgr: mgr, diner: diner, first201: map[string]firstWrite{}}
	// Pair + singleton anchors with future Thursday dates, two series.
	date := "2027-06-17"
	mk := func(key, body string) map[string]any {
		res := s.CreateReservation(diner, key, []byte(body))
		if res.Status != 201 {
			t.Fatalf("create %s: %d %v", key, res.Status, res.Body)
		}
		return res.Body.(map[string]any)
	}
	anchorPair := mk("w-pair", `{"restaurant_id":"r_anker","table_ids":["t_2","t_1"],"starts_at_local":"`+date+`T19:00","party_size":6}`)
	anchorSingle := mk("w-single", `{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"`+date+`T18:00","party_size":2}`)
	adopt := func(key, ref string) map[string]any {
		return adoptWeeks(t, s, diner, key, ref, 1)
	}
	s1 := adopt("w-sx1", anchorPair["reference"].(string))
	s2 := adoptWeeks(t, s, diner, "w-sx2", anchorSingle["reference"].(string), 2)
	w.seriesPair = s1["series_id"].(string)
	w.seriesSingle = s2["series_id"].(string)
	occs := func(m map[string]any) []any { return m["occurrences"].([]any) }
	o1 := occs(s1)
	o2 := occs(s2)
	mid1 := o1[1].(map[string]any)
	w.genPair = (mid1["reservation"].(map[string]any))["reference"].(string)
	mid2 := o2[1].(map[string]any)
	w.genSingle = (mid2["reservation"].(map[string]any))["reference"].(string)
	// Repair a still-confirmed unexceptioned generated occurrence through
	// actual preview/apply: close t_1 over the +7 pair member so it moves to
	// [t_2,t_3] with clock/terms preserved and no exception marked.
	plus7 := "2027-06-24"
	pvMBody := `{"table_id":"t_1","from":"` + plus7 + `T19:00:00+02:00","to":"` + plus7 + `T20:00:00+02:00"}`
	w.captureHTTP(t, mgr, "w-pvM", "POST", "/restaurants/r_anker/replans", pvMBody)
	planM := planIDFromRaw(t, w.first201["w-pvM"].raw)
	w.captureHTTP(t, mgr, "w-apM", "POST", "/restaurants/r_anker/replans/"+planM+"/apply", `{}`)
	if got := repairedTables(t, s, diner, w.genPair); !sameSet(got, []string{"t_2", "t_3"}) {
		t.Fatalf("member not repaired: %v", got)
	}
	// Publish the later-date policy, then genuinely amend the repaired
	// member on its original scheduled date and repaired table.
	polBody := `{"effective_from":"` + plus7 + `","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}`
	w.captureHTTP(t, mgr, "w-pol1", "POST", "/restaurants/r_anker/policies", polBody)
	s1rev := seriesRevision(t, s, diner, w.seriesPair)
	amBody := `{"expected_revision":` + impItoa(s1rev) + `,"from_index":1,"local_time":"19:30"}`
	w.captureHTTP(t, diner, "w-am1", "POST", "/series/"+w.seriesPair+"/amend", amBody)
	pinAmendedMember(t, s, diner, w, plus7)
	// Ordinary PATCH exception then cancel of the same repaired member, and
	// a mixed collective batch across both series.
	if res := s.PatchReservation(diner, w.genPair, []byte(`{"party_size":5}`)); res.Status != 200 {
		t.Fatalf("patch member: %d %v", res.Status, res.Body)
	}
	if res := s.CancelReservation(diner, w.genPair); res.Status != 200 {
		t.Fatalf("cancel member: %d %v", res.Status, res.Body)
	}
	moves := `{"moves":[{"reference":"` + genRef(t, s1, 2) + `","party_size":4},{"reference":"` + genRef(t, s2, 1) + `","party_size":3}]}`
	w.captureHTTP(t, diner, "w-mx", "POST", "/reservation-moves", moves)
	// Two plain bookings sharing the anchor slot, mirroring the classic
	// swap pair: closing t_2 moves one of them to t_3.
	w.captureHTTP(t, diner, "w-res-a", "POST", "/reservations", `{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"`+date+`T20:30","party_size":2}`)
	mk("w-b", `{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"`+date+`T20:30","party_size":2}`)
	// Preview then a nonempty changed apply, then a zero-move apply on a
	// quiet closure (empty considered set still records one closure).
	pvBody1 := `{"table_id":"t_2","from":"` + date + `T20:30:00+02:00","to":"` + date + `T22:00:00+02:00"}`
	w.captureHTTP(t, mgr, "w-pv1", "POST", "/restaurants/r_anker/replans", pvBody1)
	w.planMoved = planIDFromRaw(t, w.first201["w-pv1"].raw)
	w.captureHTTP(t, mgr, "w-ap1", "POST", "/restaurants/r_anker/replans/"+w.planMoved+"/apply", `{}`)
	// Pair-preserving apply: close t_3 over the single anchor; the pair
	// anchor is considered and keeps t_1+t_2 (pair assignment, unchanged).
	pvP := s.PreviewReplan(mgr, "r_anker", "w-pvP", []byte(`{"table_id":"t_3","from":"`+date+`T19:30:00+02:00","to":"`+date+`T20:00:00+02:00"}`))
	if pvP.Status != 201 {
		t.Fatalf("previewP: %d %v", pvP.Status, pvP.Body)
	}
	if res := s.ApplyReplan(mgr, "r_anker", pvP.Body.(map[string]any)["plan_id"].(string), "w-apP", []byte(`{}`)); res.Status != 201 {
		t.Fatalf("applyP: %d %v", res.Status, res.Body)
	}
	// Zero-move apply: closure over an empty slot (past date, no bookings).
	pv2 := s.PreviewReplan(mgr, "r_anker", "w-pv2", []byte(`{"table_id":"t_1","from":"2020-01-02T18:00:00+01:00","to":"2020-01-02T19:00:00+01:00"}`))
	if pv2.Status != 201 {
		t.Fatalf("preview2: %d %v", pv2.Status, pv2.Body)
	}
	w.planEmpty = pv2.Body.(map[string]any)["plan_id"].(string)
	if res := s.ApplyReplan(mgr, "r_anker", w.planEmpty, "w-ap2", []byte(`{}`)); res.Status != 201 {
		t.Fatalf("apply2: %d %v", res.Status, res.Body)
	}
	// Stale preview: preview, then a booking bumps the counter; the preview
	// stays unapplied and must still import.
	stalePv := s.PreviewReplan(mgr, "r_anker", "w-pvS", []byte(`{"table_id":"t_3","from":"2028-06-15T18:00:00+02:00","to":"2028-06-15T19:00:00+02:00"}`))
	if stalePv.Status != 201 {
		t.Fatalf("stale preview: %d %v", stalePv.Status, stalePv.Body)
	}
	w.planStale = stalePv.Body.(map[string]any)["plan_id"].(string)
	w.afterRef = mk("w-after", `{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-24T21:00","party_size":2}`)["reference"].(string)
	// Legitimate duplicate closure: same past closure previewed+applied twice.
	dupBody := `{"table_id":"t_1","from":"2020-01-02T18:00:00+01:00","to":"2020-01-02T19:00:00+01:00"}`
	d1 := s.PreviewReplan(mgr, "r_anker", "w-pvD1", []byte(dupBody))
	if d1.Status != 201 {
		t.Fatalf("dup preview1: %d %v", d1.Status, d1.Body)
	}
	if res := s.ApplyReplan(mgr, "r_anker", d1.Body.(map[string]any)["plan_id"].(string), "w-apD1", []byte(`{}`)); res.Status != 201 {
		t.Fatalf("dup apply1: %d %v", res.Status, res.Body)
	}
	d2 := s.PreviewReplan(mgr, "r_anker", "w-pvD2", []byte(dupBody))
	if d2.Status != 201 {
		t.Fatalf("dup preview2: %d %v", d2.Status, d2.Body)
	}
	if res := s.ApplyReplan(mgr, "r_anker", d2.Body.(map[string]any)["plan_id"].(string), "w-apD2", []byte(`{}`)); res.Status != 201 {
		t.Fatalf("dup apply2: %d %v", res.Status, res.Body)
	}
	w.planDup = d2.Body.(map[string]any)["plan_id"].(string)
	// Small pair for pair-to-singleton repair on a later Thursday.
	plus21 := "2027-07-08"
	mk("w-smallpair", `{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"`+plus21+`T18:00","party_size":2}`)
	pvPS := s.PreviewReplan(mgr, "r_anker", "w-pvPS", []byte(`{"table_id":"t_1","from":"`+plus21+`T18:00:00+02:00","to":"`+plus21+`T19:30:00+02:00"}`))
	if pvPS.Status != 201 {
		t.Fatalf("previewPS: %d %v", pvPS.Status, pvPS.Body)
	}
	if res := s.ApplyReplan(mgr, "r_anker", pvPS.Body.(map[string]any)["plan_id"].(string), "w-apPS", []byte(`{}`)); res.Status != 201 {
		t.Fatalf("applyPS: %d %v", res.Status, res.Body)
	}
	// Post-apply evolution: edit, cancel — originals must differ.
	if res := s.PatchReservation(diner, w.genSingle, []byte(`{"party_size":1}`)); res.Status != 200 {
		t.Fatalf("evolve patch: %d %v", res.Status, res.Body)
	}
	if res := s.CancelReservation(diner, w.genSingle); res.Status != 200 {
		t.Fatalf("evolve cancel: %d %v", res.Status, res.Body)
	}
	return w
}

// repairedTables returns the current table set of a member.
func repairedTables(t *testing.T, s *Service, token, ref string) []string {
	t.Helper()
	res := s.GetReservation(token, ref)
	if res.Status != 200 {
		t.Fatalf("lookup %s: %d", ref, res.Status)
	}
	raw := res.Body.(map[string]any)["table_ids"]
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		var out []string
		for _, id := range v {
			out = append(out, id.(string))
		}
		return out
	}
	t.Fatalf("table_ids type %T", raw)
	return nil
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}

// pinAmendedMember asserts the repaired member actually moved clock,
// adopted per-date terms with absolute duration/end, gained revision and a
// Changed entry on a frozen prefix, with series/counter movement.
func pinAmendedMember(t *testing.T, s *Service, diner string, w *replanImportWorld, plus7 string) {
	t.Helper()
	res := s.GetReservation(diner, w.genPair)
	if res.Status != 200 {
		t.Fatalf("amended lookup: %d", res.Status)
	}
	rec := res.Body.(map[string]any)
	if rec["starts_at_local"] != plus7+"T19:30" {
		t.Fatalf("amended clock: %v", rec["starts_at_local"])
	}
	if !sameSet(strList(rec["table_ids"]), []string{"t_2", "t_3"}) {
		t.Fatalf("amended tables: %v", rec["table_ids"])
	}
	at := rec["accepted_terms"].(map[string]any)
	for k, want := range map[string]int{"policy_version": 1, "slot_minutes": 30, "reservation_duration_minutes": 60, "cancellation_cutoff_minutes": 120} {
		if num(at[k]) != want {
			t.Fatalf("amended terms %s: %v", k, at[k])
		}
	}
	start, end := rec["starts_at"].(string), rec["ends_at"].(string)
	if dur := absMinutes(start, end); dur != 60 {
		t.Fatalf("amended duration: %d", dur)
	}
	if num(rec["revision"]) < 2 {
		t.Fatalf("amended revision: %v", rec["revision"])
	}
	h := s.ReservationHistory(diner, w.genPair)
	if h.Status != 200 {
		t.Fatalf("amended history: %d", h.Status)
	}
	rawEnts, _ := json.Marshal(h.Body.(map[string]any)["entries"])
	var ents []map[string]any
	if err := json.Unmarshal(rawEnts, &ents); err != nil {
		t.Fatalf("amended history decode: %v", err)
	}
	last := ents[len(ents)-1]
	if last["event"] != "changed" {
		t.Fatalf("amended last event: %v", last["event"])
	}
}

func strList(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		var out []string
		for _, x := range t {
			out = append(out, x.(string))
		}
		return out
	}
	return nil
}

func num(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return -1
}

func absMinutes(a, b string) int {
	ta, err1 := parseStoredInstant(a)
	tb, err2 := parseStoredInstant(b)
	if err1 != nil || err2 != nil {
		return -1
	}
	return int(tb.Sub(ta).Minutes())
}

func seriesRevision(t *testing.T, s *Service, token, sid string) int {
	t.Helper()
	res := s.GetSeries(token, sid)
	if res.Status != 200 {
		t.Fatalf("get series: %d", res.Status)
	}
	rev, _ := res.Body.(map[string]any)["revision"].(float64)
	if rev == 0 {
		if ri, ok := res.Body.(map[string]any)["revision"].(int); ok {
			return ri
		}
		t.Fatalf("series revision type %T", res.Body.(map[string]any)["revision"])
	}
	return int(rev)
}

func impItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func genRef(t *testing.T, series map[string]any, idx int) string {
	t.Helper()
	for _, o := range series["occurrences"].([]any) {
		om := o.(map[string]any)
		var n int
		switch v := om["index"].(type) {
		case float64:
			n = int(v)
		case int:
			n = v
		default:
			t.Fatalf("index type %T", om["index"])
		}
		if n == idx {
			return om["reservation"].(map[string]any)["reference"].(string)
		}
	}
	t.Fatalf("no occurrence %d", idx)
	return ""
}

// exportRaw marshals the live export body to canonical bytes.
func exportRaw(t *testing.T, s *Service) []byte {
	t.Helper()
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	return raw
}

// importBytes imports raw export bytes into a fresh service.
func importBytes(t *testing.T, raw []byte) *Service {
	t.Helper()
	d := New()
	if res := d.Import(raw); res.Status != 204 {
		t.Fatalf("import: %d %v", res.Status, res.Body)
	}
	return d
}

// TestReplanImportRoundtrip builds the full native world, exports, imports
// into a fresh service and compares complete normalized state plus all
// owner views; reimport is byte-stable and source/destination isolated.
func TestReplanImportRoundtrip(t *testing.T) {
	w := newReplanImportWorld(t)
	pre := exportRaw(t, w.s)
	d := importBytes(t, pre)
	post := exportRaw(t, d)
	var a, b map[string]any
	if err := json.Unmarshal(pre, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(post, &b); err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a["state"])
	bj, _ := json.Marshal(b["state"])
	if string(aj) != string(bj) {
		t.Fatal("imported state diverges from source export")
	}
	// Reimport byte-stability.
	d2 := importBytes(t, post)
	re := exportRaw(t, d2)
	if string(re) != string(post) {
		t.Fatal("reimport not byte-stable")
	}
	// Owner views agree across source and destination. Destination login
	// happens once up front: each login mints session state, so the
	// isolation baseline is captured after all destination writes.
	dDiner := loginAs(t, d, "bea@example.com", "correct horse bea")
	post = exportRaw(t, d)
	for _, ref := range ownerRefs(t, w.s, w.diner) {
		src := w.s.GetReservation(w.diner, ref)
		dst := d.GetReservation(dDiner, ref)
		if src.Status != 200 || dst.Status != 200 {
			t.Fatalf("view %s: %d %d", ref, src.Status, dst.Status)
		}
		sj, _ := json.Marshal(src.Body)
		dj, _ := json.Marshal(dst.Body)
		if string(sj) != string(dj) {
			t.Fatalf("view diverges for %s", ref)
		}
		hs := w.s.ReservationHistory(w.diner, ref)
		hd := d.ReservationHistory(dDiner, ref)
		hj1, _ := json.Marshal(hs.Body)
		hj2, _ := json.Marshal(hd.Body)
		if string(hj1) != string(hj2) {
			t.Fatalf("history diverges for %s", ref)
		}
		ds := w.s.ReservationDecision(w.diner, ref)
		dd := d.ReservationDecision(dDiner, ref)
		dj1, _ := json.Marshal(ds.Body)
		dj2, _ := json.Marshal(dd.Body)
		if string(dj1) != string(dj2) {
			t.Fatalf("decision diverges for %s", ref)
		}
	}
	// Series views agree across source and destination.
	for _, sid := range []string{w.seriesPair, w.seriesSingle} {
		ss := w.s.GetSeries(w.diner, sid)
		sd := d.GetSeries(dDiner, sid)
		if ss.Status != 200 || sd.Status != 200 {
			t.Fatalf("series %s: %d %d", sid, ss.Status, sd.Status)
		}
		sj, _ := json.Marshal(ss.Body)
		dj, _ := json.Marshal(sd.Body)
		if string(sj) != string(dj) {
			t.Fatalf("series diverges for %s", sid)
		}
	}
	// Clone isolation: real source change leaves the destination export
	// unchanged, and the detached original differs from evolved current.
	srcBefore := exportRaw(t, w.s)
	if res := w.s.PatchReservation(w.diner, w.afterRef, []byte(`{"party_size":3}`)); res.Status != 200 {
		t.Fatalf("source mutate: %d", res.Status)
	}
	if string(exportRaw(t, w.s)) == string(srcBefore) {
		t.Fatal("source mutation applied nothing")
	}
	after := exportRaw(t, d)
	if string(after) != string(post) {
		t.Fatal("destination changed by source mutation")
	}
}

func ownerRefs(t *testing.T, s *Service, token string) []string {
	t.Helper()
	res := s.ListReservations(token)
	if res.Status != 200 {
		t.Fatalf("list: %d", res.Status)
	}
	var refs []string
	for _, r := range res.Body.(map[string]any)["reservations"].([]any) {
		refs = append(refs, r.(map[string]any)["reference"].(string))
	}
	return refs
}

func loginAs(t *testing.T, s *Service, email, password string) string {
	t.Helper()
	return policyLogin(t, s, email, password)
}

// TestReplanImportNativeReplay captures genuine HTTP first-201 bytes in the
// world, evolves current state, imports into a fresh service, then replays
// every captured native key by real HTTP and requires byte-exact original
// responses with the whole export unchanged across all replays. The
// repaired/amended reference must differ from its old receipt.
func TestReplanImportNativeReplay(t *testing.T) {
	w := newReplanImportWorld(t)
	if len(w.first201) == 0 {
		t.Fatal("no native first201 captures")
	}
	// Evolve current state after capture, then import the evolved export.
	if res := w.s.PatchReservation(w.diner, w.afterRef, []byte(`{"party_size":4}`)); res.Status != 200 {
		t.Fatalf("evolve: %d", res.Status)
	}
	pre := exportRaw(t, w.s)
	d := importBytes(t, pre)
	dDiner := loginAs(t, d, "bea@example.com", "correct horse bea")
	base := exportRaw(t, d)
	// The repaired/amended member differs from its old amend receipt: live
	// revision advanced past the frozen amend response.
	live := d.GetReservation(dDiner, w.genPair)
	if live.Status != 200 {
		t.Fatalf("live lookup: %d", live.Status)
	}
	liveRaw, _ := json.Marshal(live.Body)
	if string(liveRaw) == w.first201["w-am1"].raw {
		t.Fatal("amended current matches old receipt")
	}
	for key, fw := range w.first201 {
		tok := fw.token
		if tok == "" {
			t.Fatalf("capture %s without token", key)
		}
		rec := serveRequest(d, fw.method, fw.path, []byte(fw.body), map[string]string{
			"Authorization":   "Bearer " + tok,
			"Idempotency-Key": key,
		})
		if rec.Code != 200 {
			t.Fatalf("replay without key: got %d", rec.Code)
		}
		if rec.Body.String() != fw.raw {
			t.Fatalf("replay without key bytes differ")
		}
	}
	if got := exportRaw(t, d); string(got) != string(base) {
		t.Fatal("export changed across replays")
	}
}

// TestReplanImportCorruptions mutates COPIES of a valid native export; each
// must 422 with the destination export byte-identical. A valid control
// imports alongside each family.
func TestReplanImportCorruptions(t *testing.T) {
	w := newReplanImportWorld(t)
	pre := exportRaw(t, w.s)
	var env map[string]any
	if err := json.Unmarshal(pre, &env); err != nil {
		t.Fatal(err)
	}
	planKey := w.planMoved
	cases := []struct {
		name string
		mut  func(m map[string]any)
	}{
		{"apply-receipt-omits-all-considered-records", func(m map[string]any) {
			st := m["state"].(map[string]any)
			for _, rc := range st["receipts"].(map[string]any) {
				rm := rc.(map[string]any)
				if rm["path"] == "/restaurants/r_anker/replans/"+planKey+"/apply" {
					rb := rm["response"].(string)
					var b map[string]any
					if err := json.Unmarshal([]byte(rb), &b); err != nil {
						panic(err)
					}
					b["reservations"] = []any{}
					nb, _ := json.Marshal(b)
					rm["response"] = string(nb)
				}
			}
		}},
		{"apply-receipt-target-contradicts-saved-assignment", func(m map[string]any) {
			st := m["state"].(map[string]any)
			for _, rc := range st["receipts"].(map[string]any) {
				rm := rc.(map[string]any)
				if rm["path"] == "/restaurants/r_anker/replans/"+planKey+"/apply" {
					rb := rm["response"].(string)
					var b map[string]any
					if err := json.Unmarshal([]byte(rb), &b); err != nil {
						panic(err)
					}
					recs := b["reservations"].([]any)
					recs[0].(map[string]any)["table_ids"] = []any{"t_2"}
					recs[0].(map[string]any)["table_id"] = "t_2"
					nb, _ := json.Marshal(b)
					rm["response"] = string(nb)
				}
			}
		}},
		{"preview-receipt-unknown-plan-id-skips-binding", func(m map[string]any) {
			st := m["state"].(map[string]any)
			for _, rc := range st["receipts"].(map[string]any) {
				rm := rc.(map[string]any)
				if rm["path"] == "/restaurants/r_anker/replans" {
					rb := rm["response"].(string)
					var b map[string]any
					if err := json.Unmarshal([]byte(rb), &b); err != nil {
						panic(err)
					}
					b["plan_id"] = "UNKNOWN1"
					nb, _ := json.Marshal(b)
					rm["response"] = string(nb)
				}
			}
		}},
		{"preview-receipt-missing-plan-id-skips-binding", func(m map[string]any) {
			st := m["state"].(map[string]any)
			for _, rc := range st["receipts"].(map[string]any) {
				rm := rc.(map[string]any)
				if rm["path"] == "/restaurants/r_anker/replans" {
					rb := rm["response"].(string)
					var b map[string]any
					if err := json.Unmarshal([]byte(rb), &b); err != nil {
						panic(err)
					}
					delete(b, "plan_id")
					nb, _ := json.Marshal(b)
					rm["response"] = string(nb)
				}
			}
		}},
		{"preview-total-contradicts-original-receipt", func(m map[string]any) {
			st := m["state"].(map[string]any)
			st["plans"].(map[string]any)[planKey].(map[string]any)["unused_seats"] = float64(9999)
		}},
		{"captured-revision-contradicts-original-receipt", func(m map[string]any) {
			st := m["state"].(map[string]any)
			st["plans"].(map[string]any)[planKey].(map[string]any)["restaurant_revision"] = float64(0)
		}},
		{"closure-contradicts-original-preview-receipt", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			cl := p["closure"].(map[string]any)
			cl["from"] = "2027-06-24T19:00:00+02:00"
			cl["to"] = "2027-06-24T20:00:00+02:00"
			for _, c := range st["closures"].(map[string]any)["r_anker"].([]any) {
				cm := c.(map[string]any)
				if cm["from"] == "2027-06-17T20:30:00+02:00" {
					cm["from"] = "2027-06-24T19:00:00+02:00"
					cm["to"] = "2027-06-24T20:00:00+02:00"
				}
			}
		}},
		{"reassigned-adopts-different-valid-policy-terms", func(m map[string]any) {
			st := m["state"].(map[string]any)
			var target string
			for _, ref := range sortedKeys(st["histories"].(map[string]any)) {
				es := st["histories"].(map[string]any)[ref].([]any)
				last := es[len(es)-1].(map[string]any)
				if last["event"] != "reassigned" {
					continue
				}
				// Deterministic pre-publication target: the trailing
				// Reassigned entry must still carry v0 terms so the v1
				// adoption forgery actually changes bytes.
				if pv, _ := last["accepted_terms"].(map[string]any)["policy_version"].(float64); pv != 0 {
					continue
				}
				target = ref
				break
			}
			if target == "" {
				panic("no trailing v0 reassigned entry")
			}
			v1 := map[string]any{"policy_version": float64(1), "slot_minutes": float64(30), "reservation_duration_minutes": float64(60), "cancellation_cutoff_minutes": float64(120), "opening_hours": []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}}, "capacities": map[string]any{"t_1": float64(2), "t_2": float64(4), "t_3": float64(4)}}
			es := st["histories"].(map[string]any)[target].([]any)
			es[len(es)-1].(map[string]any)["accepted_terms"] = v1
			st["reservations"].(map[string]any)[target].(map[string]any)["accepted_terms"] = v1
		}},
		{"plan-id-mismatch", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			p["plan_id"] = "FORGED01"
		}},
		{"closure-interval", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			cl := p["closure"].(map[string]any)
			cl["from"], cl["to"] = cl["to"], cl["from"]
		}},
		{"closure-unknown-table", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			cl := p["closure"].(map[string]any)
			cl["table_id"] = "t_nope"
		}},
		{"assignment-unknown-ref", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			as := p["assignments"].([]any)
			as[0].(map[string]any)["reference"] = "NOPE01"
		}},
		{"assignment-duplicate", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			as := p["assignments"].([]any)
			as = append(as, as[0])
			p["assignments"] = as
		}},
		{"assignment-illegal-pair", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			as := p["assignments"].([]any)
			as[0].(map[string]any)["table_ids"] = []any{"t_1", "t_3"}
		}},
		{"assignment-reversed-pair", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			for _, v := range plans {
				as := v.(map[string]any)["assignments"].([]any)
				for _, x := range as {
					xm := x.(map[string]any)
					if ids, _ := xm["table_ids"].([]any); len(ids) == 2 {
						xm["table_ids"] = []any{ids[1], ids[0]}
					}
				}
			}
		}},
		{"moved-count", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			p["moved_count"] = float64(99)
		}},
		{"negative-totals", func(m map[string]any) {
			st := m["state"].(map[string]any)
			plans := st["plans"].(map[string]any)
			p := plans[planKey].(map[string]any)
			p["unused_seats"] = float64(-1)
		}},
		{"orphan-closure", func(m map[string]any) {
			st := m["state"].(map[string]any)
			cl := st["closures"].(map[string]any)
			cl["r_anker"] = append(cl["r_anker"].([]any), map[string]any{
				"table_id": "t_1", "from": "2020-01-02T18:00:00+01:00", "to": "2020-01-02T19:00:00+01:00"})
		}},
		{"reassigned-forged-from", func(m map[string]any) {
			st := m["state"].(map[string]any)
			h := st["histories"].(map[string]any)
			for ref, es := range h {
				_ = ref
				for i, e := range es.([]any) {
					em := e.(map[string]any)
					if em["event"] == "reassigned" {
						ch := em["changes"].([]any)
						ch[0].(map[string]any)["from"] = []any{"t_1"}
						es.([]any)[i] = em
						return
					}
				}
			}
		}},
		{"reassigned-term-drift", func(m map[string]any) {
			st := m["state"].(map[string]any)
			h := st["histories"].(map[string]any)
			for _, es := range h {
				for i, e := range es.([]any) {
					em := e.(map[string]any)
					if em["event"] == "reassigned" {
						at := em["accepted_terms"].(map[string]any)
						at["slot_minutes"] = float64(999)
						es.([]any)[i] = em
						return
					}
				}
			}
		}},
		{"reassigned-plan-drift", func(m map[string]any) {
			st := m["state"].(map[string]any)
			h := st["histories"].(map[string]any)
			for _, es := range h {
				for i, e := range es.([]any) {
					em := e.(map[string]any)
					if em["event"] == "reassigned" {
						em["plan_id"] = "FORGED02"
						es.([]any)[i] = em
						return
					}
				}
			}
		}},
		{"planid-on-changed", func(m map[string]any) {
			st := m["state"].(map[string]any)
			h := st["histories"].(map[string]any)
			for _, es := range h {
				for i, e := range es.([]any) {
					em := e.(map[string]any)
					if em["event"] == "changed" {
						em["plan_id"] = planKey
						es.([]any)[i] = em
						return
					}
				}
			}
		}},
	}
	for _, c := range cases {
		var cp map[string]any
		raw, _ := json.Marshal(env)
		if err := json.Unmarshal(raw, &cp); err != nil {
			t.Fatal(err)
		}
		beforeMut, _ := json.Marshal(cp)
		c.mut(cp)
		afterMut, _ := json.Marshal(cp)
		if string(beforeMut) == string(afterMut) {
			t.Fatalf("%s: mutation applied nothing", c.name)
		}
		bad, _ := json.Marshal(cp)
		seed := New()
		if res := seed.Reset([]byte(`{"users":[],"restaurants":[],"reservations":[]}`)); res.Status != 204 {
			t.Fatalf("seed: %d", res.Status)
		}
		base := exportRaw(t, seed)
		if res := seed.Import(bad); res.Status != 422 {
			t.Fatalf("%s: got %d %v", c.name, res.Status, res.Body)
		}
		if got := exportRaw(t, seed); string(got) != string(base) {
			t.Fatalf("%s: destination mutated", c.name)
		}
		// Valid control imports alongside each family.
		ctrl := New()
		if res := ctrl.Import(pre); res.Status != 204 {
			t.Fatalf("%s control: %d %v", c.name, res.Status, res.Body)
		}
	}
	// Malformed JSON is 400, wrong track 422, both with destination retained.
	seed := New()
	if res := seed.Reset([]byte(`{"users":[],"restaurants":[],"reservations":[]}`)); res.Status != 204 {
		t.Fatalf("seed: %d", res.Status)
	}
	seedBase := exportRaw(t, seed)
	if res := seed.Import([]byte(`{oops`)); res.Status != 400 {
		t.Fatalf("malformed: %d", res.Status)
	}
	if got := exportRaw(t, seed); string(got) != string(seedBase) {
		t.Fatal("malformed mutated destination")
	}
	var wt map[string]any
	raw, _ := json.Marshal(env)
	_ = json.Unmarshal(raw, &wt)
	wt["track"] = "other"
	bad, _ := json.Marshal(wt)
	if res := seed.Import(bad); res.Status != 422 {
		t.Fatalf("track: %d", res.Status)
	}
	if got := exportRaw(t, seed); string(got) != string(seedBase) {
		t.Fatal("wrong track mutated destination")
	}
}

// TestReplanImportGenuineDonors imports the prior-process artifacts for all
// three stages. Unset env skips; configured absent/malformed fails.
func TestReplanImportGenuineDonors(t *testing.T) {
	dir := os.Getenv("S4_DONORS_DIR")
	if dir == "" {
		t.Skip("S4_DONORS_DIR unset")
	}
	for _, stage := range []string{"stage1", "stage2", "stage3"} {
		raw, err := os.ReadFile(filepath.Join(dir, stage, "export.json"))
		if err != nil {
			t.Fatalf("donor %s unreadable: %v", stage, err)
		}
		var env map[string]any
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("donor %s malformed: %v", stage, err)
		}
		d := New()
		if res := d.Import(raw); res.Status != 204 {
			t.Fatalf("donor %s import: %d %v", stage, res.Status, res.Body)
		}
		st := exportState(t, d)
		if len(st["plans"].(map[string]any)) != 0 || len(st["closures"].(map[string]any)) != 0 {
			t.Fatalf("donor %s native maps nonempty", stage)
		}
		// Every manifest receipt replays to its exact original raw bytes.
		manRaw, err := os.ReadFile(filepath.Join(dir, stage, "manifest.json"))
		if err != nil {
			t.Fatalf("donor %s manifest: %v", stage, err)
		}
		var man map[string]any
		if err := json.Unmarshal(manRaw, &man); err != nil {
			t.Fatalf("donor %s manifest malformed: %v", stage, err)
		}
		users := map[string]map[string]any{}
		for _, u := range man["users"].([]any) {
			um := u.(map[string]any)
			users[um["id"].(string)] = um
		}
		wantCounts := map[string]int{"stage1": 5, "stage2": 8, "stage3": 9}
		if len(man["receipts"].([]any)) != wantCounts[stage] {
			t.Fatalf("donor %s receipts: got %d want %d", stage, len(man["receipts"].([]any)), wantCounts[stage])
		}
		// Eligible real mutation before replay where the donor supports it:
		// stage3 fail-reuse booking is live and confirmed; patch it so
		// current differs while replays must return original bytes.
		if stage == "stage3" {
			mutateDonorReuse(t, d, man)
		}
		for _, r := range man["receipts"].([]any) {
			rm := r.(map[string]any)
			uid := rm["user_id"].(string)
			toks := users[uid]["tokens"].([]any)
			// Scope binding: receipt owner/method/path/key must match the
			// stored canonical receipt for the same scoped key.
			stRec := storedReceipt(t, d, uid, rm["method"].(string), rm["path"].(string), rm["key"].(string))
			if stRec["status"] != rm["status"] {
				t.Fatalf("donor %s receipt status", stage)
			}
			replayExact(t, d, toks[0].(string), rm)
		}
	}
}

// mutateDonorReuse patches the stage3 fail-reuse booking (live, confirmed)
// so current differs from its original receipt before replays run.
func mutateDonorReuse(t *testing.T, d *Service, man map[string]any) {
	t.Helper()
	fb := man["failed_keys"].(map[string]any)["reused"].(map[string]any)
	body := fb["body"].(map[string]any)
	uid := fb["owner"].(string)
	var tok string
	for _, u := range man["users"].([]any) {
		um := u.(map[string]any)
		if um["id"] == uid {
			tok = um["tokens"].([]any)[0].(string)
		}
	}
	// Find the booking by matching current record to the reuse body.
	st := exportState(t, d)
	var ref string
	for r, v := range st["reservations"].(map[string]any) {
		rm := v.(map[string]any)
		if rm["user_id"] == uid && rm["starts_at_local"] == body["starts_at_local"] {
			ids, _ := rm["table_ids"].([]any)
			match := false
			if tb, ok := body["table_id"].(string); ok {
				match = len(ids) == 1 && ids[0] == tb
			}
			if match {
				ref = r
			}
		}
	}
	if ref == "" {
		t.Fatal("donor reuse booking not found")
	}
	if res := d.PatchReservation(tok, ref, []byte(`{"party_size":1}`)); res.Status != 200 {
		t.Fatalf("donor mutate: %d", res.Status)
	}
	cur := d.GetReservation(tok, ref)
	if cur.Status != 200 {
		t.Fatalf("donor lookup: %d", cur.Status)
	}
	if cur.Body.(map[string]any)["party_size"] == fb["body"].(map[string]any)["party_size"] {
		t.Fatal("donor current matches original receipt")
	}
}

// storedReceipt fetches the canonical stored receipt for a scoped key.
// sortedKeys returns map keys in ascending order for determinism.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

func storedReceipt(t *testing.T, s *Service, uid, method, path, key string) map[string]any {
	t.Helper()
	st := exportState(t, s)
	for _, v := range st["receipts"].(map[string]any) {
		rm := v.(map[string]any)
		if rm["user_id"] == uid && rm["method"] == method && rm["path"] == path && rm["key"] == key {
			return rm
		}
	}
	t.Fatalf("stored receipt missing for %s %s", method, path)
	return nil
}

func exportState(t *testing.T, s *Service) map[string]any {
	t.Helper()
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env["state"].(map[string]any)
}

// replayExact replays one stored receipt through the real HTTP router with
// the original method/path/token/body/key and requires status 200 with the
// recorder body string EXACTLY equal to the stored first-201 raw bytes. No
// remarshal fallback: any byte difference fails.
func replayExact(t *testing.T, s *Service, token string, rm map[string]any) {
	t.Helper()
	method := rm["method"].(string)
	path := rm["path"].(string)
	body := rm["body_raw"].(string)
	key := rm["key"].(string)
	rec := serveRequest(s, method, path, []byte(body), map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": key,
	})
	if rec.Code != 200 {
		t.Fatalf("replay %s %s: got %d", method, path, rec.Code)
	}
	if rec.Body.String() != rm["response_raw"].(string) {
		t.Fatalf("replay %s %s bytes differ", method, path)
	}
}

func adoptWeeks(t *testing.T, s *Service, diner, key, ref string, weeks int) map[string]any {
	t.Helper()
	res := s.AdoptSeries(diner, key, []byte(`{"anchor_reference":"`+ref+`","count":3,"interval_weeks":`+impItoa(weeks)+`}`))
	if res.Status != 201 {
		t.Fatalf("adopt %s: %d %v", key, res.Status, res.Body)
	}
	return res.Body.(map[string]any)
}

// TestReplanImportEvolvedZeroMoveApply reproduces the coordinator audit
// workflow: a zero-move apply (one unchanged considered booking) stays
// importable after a genuine later PATCH evolves the record. The frozen
// apply receipt retains the apply-time revision/party; validation binds
// it to the history replay at the receipt's own revision, never to the
// evolved final state.
func TestReplanImportEvolvedZeroMoveApply(t *testing.T) {
	s := New()
	if res := s.Reset([]byte(replanImportFixture)); res.Status != 204 {
		t.Fatalf("reset: %d", res.Status)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	mk := serveRequest(s, "POST", "/reservations", []byte(`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":1}`), map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "evo-create"})
	if mk.Code != 201 {
		t.Fatalf("create: %d", mk.Code)
	}
	var created map[string]any
	if err := json.Unmarshal(mk.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	ref := created["reference"].(string)
	pv := serveRequest(s, "POST", "/restaurants/r_anker/replans", []byte(`{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T20:00:00+02:00"}`), map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "evo-preview"})
	if pv.Code != 201 {
		t.Fatalf("preview: %d", pv.Code)
	}
	var pvBody map[string]any
	if err := json.Unmarshal(pv.Body.Bytes(), &pvBody); err != nil {
		t.Fatal(err)
	}
	if len(pvBody["assignments"].([]any)) != 1 || pvBody["moved_count"].(float64) != 0 {
		t.Fatalf("preview shape: %s", pv.Body.String())
	}
	pid := pvBody["plan_id"].(string)
	ap := serveRequest(s, "POST", "/restaurants/r_anker/replans/"+pid+"/apply", []byte(`{}`), map[string]string{"Authorization": "Bearer " + tok, "Idempotency-Key": "evo-apply"})
	if ap.Code != 201 {
		t.Fatalf("apply: %d", ap.Code)
	}
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatal(err)
	}
	if res := New(); res.Import(raw).Status != 204 {
		t.Fatal("valid pre-patch control must import")
	}
	pt := serveRequest(s, "PATCH", "/reservations/"+ref, []byte(`{"expected_revision":1,"party_size":2}`), map[string]string{"Authorization": "Bearer " + tok})
	if pt.Code != 200 {
		t.Fatalf("patch: %d", pt.Code)
	}
	var patched map[string]any
	if err := json.Unmarshal(pt.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched["revision"].(float64) != 2 || patched["party_size"].(float64) != 2 {
		t.Fatalf("patch outcome: %s", pt.Body.String())
	}
	evolved, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(evolved) == string(raw) {
		t.Fatal("evolution applied nothing")
	}
	d := New()
	if res := d.Import(evolved); res.Status != 204 {
		t.Fatalf("evolved import: %d", res.Status)
	}
}
