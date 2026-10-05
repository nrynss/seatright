package service

import (
	"encoding/json"
	"testing"
)

// adoptFixture returns a service with an adopted 3-member series plus the
// owner token, series id, member references in index order and the anchor
// Thursday date. Members are ordinary future bookings on t_1.
func adoptFixture(t *testing.T, anchorParty int) (*Service, string, string, []string, string) {
	t.Helper()
	s, ada, date := collectiveSetup(t)
	anchor, _ := createRef(t, s, ada, "sx-anchor", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_1",
		"starts_at_local": date + "T19:00", "party_size": anchorParty})
	body, _ := json.Marshal(map[string]any{
		"anchor_reference": anchor, "count": 3, "interval_weeks": 1})
	res := s.AdoptSeries(ada, "sx-adopt", body)
	if res.Status != 201 {
		t.Fatalf("adopt: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	sid, _ := v["series_id"].(string)
	var refs []string
	for _, o := range v["occurrences"].([]any) {
		refs = append(refs, o.(map[string]any)["reference"].(string))
	}
	if len(refs) != 3 || refs[0] != anchor {
		t.Fatalf("occurrences = %v", refs)
	}
	return s, ada, sid, refs, date
}

func seriesState(s *Service, sid string) (int, []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sz := s.state.Series[sid]
	flags := make([]bool, 0, len(sz.Members))
	for _, m := range sz.Members {
		flags = append(flags, m.Exception)
	}
	return sz.Revision, flags
}

func TestSeriesMutationIndividualPatch(t *testing.T) {
	s, ada, sid, refs, _ := adoptFixture(t, 2)
	// Real individual change: permanent exception, series +1, restaurant +1.
	res := s.PatchReservation(ada, refs[1], patchBody(map[string]any{"party_size": 1}))
	if res.Status != 200 {
		t.Fatalf("patch occurrence: %d %v", res.Status, res.Body)
	}
	rev, flags := seriesState(s, sid)
	if rev != 2 || len(flags) != 3 || !flags[1] || flags[0] || flags[2] {
		t.Fatalf("series rev=%d flags=%v", rev, flags)
	}
	if counterOf(s, "r_anker") != 3 {
		t.Fatalf("counter = %d, want 3 (create + adopt + patch)", counterOf(s, "r_anker"))
	}
	// No-op on another member changes nothing.
	before := marshalBody(t, s.GetReservation(ada, refs[2]))
	if res := s.PatchReservation(ada, refs[2], patchBody(map[string]any{})); res.Status != 200 {
		t.Fatalf("no-op patch: %d", res.Status)
	} else if marshalBody(t, res) != before {
		t.Fatalf("no-op changed record")
	}
	if rev, flags := seriesState(s, sid); rev != 2 || flags[2] {
		t.Fatalf("no-op touched series: rev=%d flags=%v", rev, flags)
	}
	// Failed change changes nothing.
	before = marshalBody(t, s.GetReservation(ada, refs[2]))
	if res := s.PatchReservation(ada, refs[2], patchBody(map[string]any{"table_id": "t_nope"})); res.Status != 404 {
		t.Fatalf("bad table patch: %d %v", res.Status, res.Body)
	}
	if marshalBody(t, s.GetReservation(ada, refs[2])) != before {
		t.Fatalf("failed patch mutated record")
	}
	if rev, flags := seriesState(s, sid); rev != 2 || flags[2] {
		t.Fatalf("failed patch touched series: rev=%d flags=%v", rev, flags)
	}
}

func TestSeriesMutationCancel(t *testing.T) {
	s, ada, sid, refs, _ := adoptFixture(t, 2)
	// Cancel one occurrence: series +1, no exception, retained cancelled.
	if res := s.CancelReservation(ada, refs[1]); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	rev, flags := seriesState(s, sid)
	if rev != 2 || flags[1] {
		t.Fatalf("cancel series rev=%d flags=%v", rev, flags)
	}
	if got := s.GetReservation(ada, refs[1]).Body.(map[string]any)["status"]; got != "cancelled" {
		t.Fatalf("occurrence status = %v", got)
	}
	// Repeat cancel changes nothing.
	n0, h0, c0 := stateInts(s, refs[1])
	if res := s.CancelReservation(ada, refs[1]); res.Status != 200 {
		t.Fatalf("repeat cancel: %d", res.Status)
	}
	if n1, h1, c1 := stateInts(s, refs[1]); n1[0] != n0[0] || h1[0] != h0[0] || c1 != c0 {
		t.Fatalf("repeat cancel mutated series member")
	}
	if rev, _ := seriesState(s, sid); rev != 2 {
		t.Fatalf("repeat cancel touched series")
	}
	// Cancelling the anchor leaves siblings confirmed and bumps once.
	if res := s.CancelReservation(ada, refs[0]); res.Status != 200 {
		t.Fatalf("anchor cancel: %d", res.Status)
	}
	for _, r := range refs[1:] {
		if got := s.GetReservation(ada, r).Body.(map[string]any)["status"]; r == refs[1] {
			if got != "cancelled" {
				t.Fatalf("sibling changed: %v", got)
			}
		} else if got != "confirmed" {
			t.Fatalf("sibling changed: %v", got)
		}
	}
	if rev, flags := seriesState(s, sid); rev != 3 || flags[0] {
		t.Fatalf("anchor cancel series rev=%d flags=%v", rev, flags)
	}
}

func TestSeriesMutationBatchOnce(t *testing.T) {
	s, ada, sid, refs, _ := adoptFixture(t, 2)
	counterBefore := counterOf(s, "r_anker")
	moves := `{"reference":"` + refs[1] + `","party_size":1},` +
		`{"reference":"` + refs[2] + `","party_size":1}`
	status, body := batchRaw(t, s, ada, "sm-01", moves)
	if status != 201 {
		t.Fatalf("batch: %d %s", status, body)
	}
	// One restaurant increment and one series increment for two members.
	if counterOf(s, "r_anker") != counterBefore+1 {
		t.Fatalf("counter = %d", counterOf(s, "r_anker"))
	}
	rev, flags := seriesState(s, sid)
	if rev != 2 || !flags[1] || !flags[2] || flags[0] {
		t.Fatalf("batch series rev=%d flags=%v", rev, flags)
	}
	revs, histLens, _ := stateInts(s, refs[1], refs[2])
	if revs[0] != 2 || revs[1] != 2 || histLens[0] != 2 || histLens[1] != 2 {
		t.Fatalf("member state rev=%v hist=%v", revs, histLens)
	}
	// Anchor untouched.
	if revs, _, _ := stateInts(s, refs[0]); revs[0] != 1 {
		t.Fatalf("anchor rev = %v", revs)
	}
}

func TestSeriesMutationMultipleSeries(t *testing.T) {
	s, ada, sid1, refs1, date := adoptFixture(t, 2)
	anchor2, _ := createRef(t, s, ada, "sy-anchor", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_3",
		"starts_at_local": date + "T21:00", "party_size": 2})
	adoptRaw, _ := json.Marshal(map[string]any{
		"anchor_reference": anchor2, "count": 2, "interval_weeks": 1})
	res := s.AdoptSeries(ada, "sy-adopt", adoptRaw)
	if res.Status != 201 {
		t.Fatalf("adopt 2: %d %v", res.Status, res.Body)
	}
	sid2 := res.Body.(map[string]any)["series_id"].(string)
	refs2 := []string{anchor2}
	for _, o := range res.Body.(map[string]any)["occurrences"].([]any) {
		if o.(map[string]any)["index"].(int) == 1 {
			refs2 = append(refs2, o.(map[string]any)["reference"].(string))
		}
	}
	moves := `{"reference":"` + refs1[1] + `","party_size":1},` +
		`{"reference":"` + refs2[1] + `","party_size":1}`
	status, body := batchRaw(t, s, ada, "sm-multi", moves)
	if status != 201 {
		t.Fatalf("multi-series batch: %d %s", status, body)
	}
	rev1, flags1 := seriesState(s, sid1)
	rev2, flags2 := seriesState(s, sid2)
	if rev1 != 2 || rev2 != 2 {
		t.Fatalf("series revs = %d/%d, want 2/2", rev1, rev2)
	}
	if !flags1[1] || flags1[0] || flags1[2] || !flags2[1] || flags2[0] {
		t.Fatalf("flags = %v / %v", flags1, flags2)
	}
	// Non-member bookings are untouched.
	if revs, _, _ := stateInts(s, refs1[0]); revs[0] != 1 {
		t.Fatalf("non-member rev = %v", revs)
	}
}

func TestSeriesMutationReplay(t *testing.T) {
	s, ada, sid, refs, _ := adoptFixture(t, 2)
	moves := `{"reference":"` + refs[1] + `","party_size":1}`
	status, body := batchRaw(t, s, ada, "sm-replay", moves)
	if status != 201 {
		t.Fatalf("batch: %d %s", status, body)
	}
	original := body
	beforeExport := exportStateBytes(t, s)
	// Later edits, a cancellation and a publication do not disturb the
	// original receipt; replaying it changes no state at all.
	if res := s.PatchReservation(ada, refs[2], patchBody(map[string]any{"party_size": 1})); res.Status != 200 {
		t.Fatalf("later patch: %d", res.Status)
	}
	if res := s.CancelReservation(ada, refs[2]); res.Status != 200 {
		t.Fatalf("later cancel: %d", res.Status)
	}
	if res := s.PublishPolicy(ada, "r_anker", "sm-pol", validPolicyBody("2027-06-17")); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	_ = sid
	status, body = batchRaw(t, s, ada, "sm-replay", moves)
	if status != 200 || body != original {
		t.Fatalf("replay = %d %s, want 200 original bytes", status, body)
	}
	if exportStateBytes(t, s) == beforeExport {
		t.Fatalf("export unexpectedly identical despite later edits")
	}
	midExport := exportStateBytes(t, s)
	status, body = batchRaw(t, s, ada, "sm-replay", moves)
	if status != 200 || body != original {
		t.Fatalf("second replay = %d", status)
	}
	if exportStateBytes(t, s) != midExport {
		t.Fatalf("replay changed exported state")
	}
}

func TestSeriesMutationCutoffAndTerms(t *testing.T) {
	s, ada, date := collectiveSetup(t)
	// Date-crossing batch move adopts the resulting date's terms and end.
	refA, _ := createRef(t, s, ada, "st-a", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": "2027-06-10T19:00", "party_size": 4})
	if res := s.PublishPolicy(ada, "r_anker", "st-pol", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	status, body := batchRaw(t, s, ada, "st-01",
		`{"reference":"`+refA+`","table_id":"t_1","starts_at_local":"`+date+`T19:00"}`)
	if status != 201 {
		t.Fatalf("date-crossing batch: %d %s", status, body)
	}
	item := batchItem(t, body, 0)
	if item["revision"] != float64(2) {
		t.Fatalf("revision = %v", item)
	}
	if item["accepted_terms"].(map[string]any)["policy_version"] != float64(1) {
		t.Fatalf("terms not adopted: %v", item)
	}
	// Old accepted cutoff still governs batches: a past booking fails even
	// though its fields would otherwise validate.
	past, _ := createRef(t, s, ada, "st-p", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_3",
		"starts_at_local": "2020-01-02T19:00", "party_size": 2})
	status, body = batchRaw(t, s, ada, "st-02", `{"reference":"`+past+`","party_size":1}`)
	if status != 409 {
		t.Fatalf("past batch = %d %s", status, body)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	if v["error"].(map[string]any)["code"] != "cutoff_passed" {
		t.Fatalf("past batch code = %v", v)
	}
}
