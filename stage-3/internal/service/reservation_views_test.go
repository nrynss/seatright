package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"tablekeeper/internal/history"
)

func historyEntries(t *testing.T, res Result) []any {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("history: %d %v", res.Status, res.Body)
	}
	v := res.Body.(map[string]any)
	rawEntries, ok := v["entries"]
	if !ok {
		t.Fatalf("history missing entries: %v", res.Body)
	}
	var list []any
	switch entries := rawEntries.(type) {
	case []any:
		list = entries
	case []history.Entry:
		for _, e := range entries {
			list = append(list, entryToMap(t, e))
		}
	default:
		t.Fatalf("entries not an array: %T", rawEntries)
	}
	if list == nil {
		t.Fatalf("entries is null: %v", res.Body)
	}
	if _, ok := v["reference"].(string); !ok {
		t.Fatalf("history missing reference: %v", res.Body)
	}
	return list
}

// entryToMap renders a history entry with native service values preserved
// (seq/revision stay ints, changes keep their JSON values); accepted_terms
// decodes to the usual JSON object shape.
func entryToMap(t *testing.T, e history.Entry) map[string]any {
	t.Helper()
	changes := make([]any, 0, len(e.Changes))
	for _, c := range e.Changes {
		changes = append(changes, map[string]any{"field": c.Field, "from": c.From, "to": c.To})
	}
	var terms map[string]any
	if len(e.AcceptedTerms) != 0 {
		if err := json.Unmarshal(e.AcceptedTerms, &terms); err != nil {
			t.Fatalf("entry terms do not parse: %v", err)
		}
	}
	return map[string]any{
		"seq": e.Seq, "at": e.At, "event": e.Event,
		"changes": changes, "revision": e.Revision,
		"accepted_terms": terms,
	}
}

func TestReservationHistoryScalarPair(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "h-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	entries := historyEntries(t, s.ReservationHistory(diner, ref))
	if len(entries) != 1 {
		t.Fatalf("entries = %v", entries)
	}
	first := entries[0].(map[string]any)
	if first["seq"] != 1 || first["event"] != "created" || first["revision"] != 1 {
		t.Fatalf("created entry = %v", first)
	}
	changes := first["changes"].([]any)
	if len(changes) != 3 {
		t.Fatalf("created changes = %v", changes)
	}
	fields := []string{changes[0].(map[string]any)["field"].(string), changes[1].(map[string]any)["field"].(string), changes[2].(map[string]any)["field"].(string)}
	if fields[0] != "table_id" || fields[1] != "starts_at_local" || fields[2] != "party_size" {
		t.Fatalf("created order = %v", fields)
	}
	if changes[0].(map[string]any)["from"] != nil || changes[0].(map[string]any)["to"] != "t_2" {
		t.Fatalf("created table change = %v", changes[0])
	}
	if _, ok := first["accepted_terms"].(map[string]any); !ok {
		t.Fatalf("created entry missing terms: %v", first)
	}
	// Singleton-to-singleton change keeps scalar table_id with full values.
	if res := s.PatchReservation(diner, ref, patchBody(map[string]any{"table_id": "t_3", "party_size": 3})); res.Status != 200 {
		t.Fatalf("amend: %d %v", res.Status, res.Body)
	}
	entries = historyEntries(t, s.ReservationHistory(diner, ref))
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	second := entries[1].(map[string]any)
	if second["seq"] != 2 || second["event"] != "changed" || second["revision"] != 2 {
		t.Fatalf("changed entry = %v", second)
	}
	fields = nil
	for _, c := range second["changes"].([]any) {
		fields = append(fields, c.(map[string]any)["field"].(string))
	}
	if len(fields) != 2 || fields[0] != "table_id" || fields[1] != "party_size" {
		t.Fatalf("changed order = %v", fields)
	}
	tc := second["changes"].([]any)[0].(map[string]any)
	if tc["from"] != "t_2" || tc["to"] != "t_3" {
		t.Fatalf("scalar table change = %v", tc)
	}
	// Pair creation replaces table_id with table_ids from null.
	pref, _ := createRef(t, s, diner, "h-02", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 6,
	})
	pentries := historyEntries(t, s.ReservationHistory(diner, pref))
	pfirst := pentries[0].(map[string]any)
	pc := pfirst["changes"].([]any)[0].(map[string]any)
	if pc["field"] != "table_ids" || pc["from"] != nil {
		t.Fatalf("pair creation change = %v", pc)
	}
	// Pair-to-singleton change uses complete canonical lists (party narrows to
	// fit the singleton under the same rules).
	if res := s.PatchReservation(diner, pref, patchBody(map[string]any{"table_id": "t_3", "party_size": 4})); res.Status != 200 {
		t.Fatalf("pair amend: %d %v", res.Status, res.Body)
	}
	pentries = historyEntries(t, s.ReservationHistory(diner, pref))
	psecond := pentries[1].(map[string]any)["changes"].([]any)[0].(map[string]any)
	if psecond["field"] != "table_ids" {
		t.Fatalf("pair change field = %v", psecond)
	}
	raw, _ := json.Marshal(psecond)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	from := decoded["from"].([]any)
	to := decoded["to"].([]any)
	if len(from) != 2 || from[0].(string) != "t_1" || from[1].(string) != "t_2" || len(to) != 1 || to[0].(string) != "t_3" {
		t.Fatalf("pair change lists = %v", psecond)
	}
}

func TestReservationHistoryCancelled(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "hc-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	if res := s.CancelReservation(diner, ref); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	entries := historyEntries(t, s.ReservationHistory(diner, ref))
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	last := entries[1].(map[string]any)
	if last["event"] != "cancelled" || last["revision"] != 2 {
		t.Fatalf("cancelled entry = %v", last)
	}
	if changes, ok := last["changes"].([]any); !ok || changes == nil || len(changes) != 0 {
		t.Fatalf("cancelled changes = %v", last["changes"])
	}
	raw, _ := json.Marshal(last)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["changes"].([]any); !ok {
		t.Fatalf("cancelled changes must render []: %s", raw)
	}
	// Decision stays available after cancellation.
	dec := s.ReservationDecision(diner, ref)
	if dec.Status != 200 {
		t.Fatalf("cancelled decision: %d", dec.Status)
	}
	dv := dec.Body.(map[string]any)
	if dv["reference"] != ref || dv["revision"] != 2 {
		t.Fatalf("decision = %v", dv)
	}
	if _, ok := dv["accepted_terms"].(map[string]any); !ok {
		t.Fatalf("decision missing terms: %v", dv)
	}
}

func TestReservationHistoryDecisionPrivacy(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	_ = mgr
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "hp-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	other := policyLogin(t, s, "bea@example.com", "correct horse bea")
	if other == diner {
		t.Fatal("expected distinct login token")
	}
	// A second user (even the manager of the restaurant) sees 404.
	mgrToken := policyLogin(t, s, "ada@example.com", "correct horse")
	for name, res := range map[string]Result{
		"unknown history":    s.ReservationHistory(diner, "NOPE01"),
		"foreign history":    s.ReservationHistory(mgrToken, ref),
		"no-token history":   s.ReservationHistory("", ref),
		"bad-token history":  s.ReservationHistory("bad", ref),
		"unknown decision":   s.ReservationDecision(diner, "NOPE01"),
		"foreign decision":   s.ReservationDecision(mgrToken, ref),
		"no-token decision":  s.ReservationDecision("", ref),
		"bad-token decision": s.ReservationDecision("bad", ref),
	} {
		if status, code := resultCode(res); status != 404 || code != "not_found" {
			t.Errorf("%s = %d %s, want 404 not_found", name, status, code)
		}
	}
	// Exact owner views still work, and ordinary lookup keeps 401 rules.
	if res := s.ReservationHistory(diner, ref); res.Status != 200 {
		t.Fatalf("owner history: %d", res.Status)
	}
	if res := s.ReservationDecision(diner, ref); res.Status != 200 {
		t.Fatalf("owner decision: %d", res.Status)
	}
	if status, code := resultCode(s.GetReservation("bad", ref)); status != 401 || code != "unauthenticated" {
		t.Fatalf("ordinary lookup bad token = %d %s", status, code)
	}
	// Privacy holds after cancellation too.
	if res := s.CancelReservation(diner, ref); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	if status, _ := resultCode(s.ReservationHistory(mgrToken, ref)); status != 404 {
		t.Fatalf("foreign history after cancel = %d", status)
	}
	if status, _ := resultCode(s.ReservationDecision("", ref)); status != 404 {
		t.Fatalf("anonymous decision after cancel = %d", status)
	}
}

func TestAcceptedTermsShape(t *testing.T) {
	s, mgr, diner := newWriteService(t)
	date := "2027-06-17"
	if res := s.PublishPolicy(mgr, "r_anker", "sh-01", validPolicyBody(date)); res.Status != 201 {
		t.Fatalf("publish: %d", res.Status)
	}
	_, v := createRef(t, s, diner, "sh-create", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	raw, _ := json.Marshal(v)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["revision"] != float64(1) {
		t.Fatalf("create revision = %v", decoded["revision"])
	}
	terms, ok := decoded["accepted_terms"].(map[string]any)
	if !ok {
		t.Fatalf("no accepted_terms: %s", raw)
	}
	for _, k := range []string{"policy_version", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"} {
		if _, ok := terms[k]; !ok {
			t.Fatalf("terms missing %s: %s", k, raw)
		}
	}
	if _, hasEff := terms["effective_from"]; hasEff {
		t.Fatalf("terms carry effective_from: %s", raw)
	}
	if len(terms) != 6 {
		t.Fatalf("terms have extra keys: %s", raw)
	}
}

func TestHistoryDecisionRoutesHTTP(t *testing.T) {
	s2, _, diner2 := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s2, diner2, "http-h-01", map[string]any{
		"restaurant_id": "r_anker", "table_id": "t_2",
		"starts_at_local": date + "T19:00", "party_size": 4,
	})
	get := func(token, path string) (int, map[string]any) {
		headers := map[string]string{}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
		rec := serveRequest(s2, http.MethodGet, path, nil, headers)
		var v map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("GET %s: %v (%q)", path, err, rec.Body.String())
		}
		return rec.Code, v
	}
	if status, v := get(diner2, "/reservations/"+ref+"/history"); status != 200 {
		t.Fatalf("history: %d %v", status, v)
	} else if v["reference"] != ref || len(v["entries"].([]any)) != 1 {
		t.Fatalf("history body = %v", v)
	}
	if status, v := get(diner2, "/reservations/"+ref+"/decision"); status != 200 {
		t.Fatalf("decision: %d %v", status, v)
	} else if v["reference"] != ref || v["revision"] != float64(1) {
		t.Fatalf("decision body = %v", v)
	}
	for name, path := range map[string]string{
		"unknown history":   "/reservations/NOPE01/history",
		"unknown decision":  "/reservations/NOPE01/decision",
		"no-token history":  "/reservations/" + ref + "/history",
		"no-token decision": "/reservations/" + ref + "/decision",
	} {
		var status int
		if name == "unknown history" || name == "unknown decision" {
			status, _ = get(diner2, path)
		} else {
			status, _ = get("", path)
		}
		if status != 404 {
			t.Errorf("%s status = %d, want 404", name, status)
		}
	}
	// Wrong methods and deeper paths stay 404; ordinary lookup keeps 401.
	rec := serveRequest(s2, http.MethodPost, "/reservations/"+ref+"/history", nil, authHeader(diner2))
	if rec.Code != 404 {
		t.Fatalf("POST history = %d", rec.Code)
	}
	rec = serveRequest(s2, http.MethodGet, "/reservations/"+ref+"/history/extra", nil, authHeader(diner2))
	if rec.Code != 404 {
		t.Fatalf("deep history = %d", rec.Code)
	}
	rec = serveRequest(s2, http.MethodGet, "/reservations/"+ref, nil, nil)
	if rec.Code != 401 {
		t.Fatalf("anonymous lookup = %d", rec.Code)
	}
}

func TestReservationHistoryDetached(t *testing.T) {
	s, _, diner := newWriteService(t)
	date := "2027-06-17"
	ref, _ := createRef(t, s, diner, "det-01", map[string]any{
		"restaurant_id": "r_anker", "table_ids": []string{"t_1", "t_2"},
		"starts_at_local": date + "T21:00", "party_size": 6,
	})
	exportJSON := func() string {
		raw, err := json.Marshal(s.Export().Body)
		if err != nil {
			t.Fatalf("export marshal: %v", err)
		}
		return string(raw)
	}
	pristine := exportJSON()
	first := s.ReservationHistory(diner, ref)
	if first.Status != 200 {
		t.Fatalf("history: %d", first.Status)
	}
	// Mutate the returned typed result: the nested pair array and the raw
	// accepted-terms bytes must not alias stored state.
	entries := first.Body.(map[string]any)["entries"].([]history.Entry)
	entries[0].Changes[0].To.([]string)[0] = "t_9"
	entries[0].AcceptedTerms[5] = 'X'
	second := s.ReservationHistory(diner, ref)
	if second.Status != 200 {
		t.Fatalf("fresh history: %d", second.Status)
	}
	fresh := second.Body.(map[string]any)["entries"].([]history.Entry)
	if fresh[0].Changes[0].To.([]string)[0] != "t_1" {
		t.Fatalf("stored pair array mutated through view: %#v", fresh[0].Changes[0].To)
	}
	var terms map[string]any
	if err := json.Unmarshal(fresh[0].AcceptedTerms, &terms); err != nil {
		t.Fatal(err)
	}
	if terms["policy_version"] != float64(0) || terms["slot_minutes"] != float64(30) {
		t.Fatalf("stored terms mutated through view: %s", fresh[0].AcceptedTerms)
	}
	if exportJSON() != pristine {
		t.Fatalf("view mutation changed exported state")
	}
	// The cancelled entry's empty changes stay [] rather than null.
	if res := s.CancelReservation(diner, ref); res.Status != 200 {
		t.Fatalf("cancel: %d", res.Status)
	}
	after := s.ReservationHistory(diner, ref)
	raw, _ := json.Marshal(after.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	last := decoded["entries"].([]any)[1].(map[string]any)
	if changes, ok := last["changes"].([]any); !ok || changes == nil || len(changes) != 0 {
		t.Fatalf("cancelled changes must render []: %s", raw)
	}
}
