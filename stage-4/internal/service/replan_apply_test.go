package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

const applyFixture = `{
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
      "id": "r_other", "name": "Other", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
      "manager_user_ids": ["u_ada"]
    }
  ],
  "reservations": [
    {"id": "seed-a", "reference": "APAAAA", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_2",
     "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "seed-b", "reference": "APBBBB", "user_id": "u_bea",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": "2027-06-17T19:00", "party_size": 2}
  ]
}`

const applyClosure = `{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`

func newApplyService(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
	if res := s.Reset([]byte(applyFixture)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	mgr := policyLogin(t, s, "ada@example.com", "correct horse")
	diner := policyLogin(t, s, "bea@example.com", "correct horse bea")
	return s, mgr, diner
}

func previewOK(t *testing.T, s *Service, mgr, key string) Result {
	t.Helper()
	res := s.PreviewReplan(mgr, "r_anker", key, []byte(applyClosure))
	if res.Status != 201 {
		t.Fatalf("preview: %d %v", res.Status, res.Body)
	}
	return res
}

func applyExportBytes(t *testing.T, s *Service) []byte {
	t.Helper()
	raw, err := json.Marshal(s.Export().Body)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	return raw
}

func applyReplan(t *testing.T, s *Service, token, restaurant, plan, key string) Result {
	t.Helper()
	return s.ApplyReplan(token, restaurant, plan, key, []byte(`{}`))
}

func planIDOf(t *testing.T, res Result) string {
	t.Helper()
	m, _ := res.Body.(map[string]any)
	id, _ := m["plan_id"].(string)
	if id == "" {
		t.Fatalf("no plan_id in %v", res.Body)
	}
	return id
}

func TestReplanApplyAuth(t *testing.T) {
	s, mgr, diner := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "auth-pv"))
	if res := s.ApplyReplan("", "r_anker", pid, "k1", []byte(`{}`)); res.Status != 401 {
		t.Fatalf("missing token: %d", res.Status)
	}
	if res := s.ApplyReplan("nope", "r_anker", pid, "k1", []byte(`{}`)); res.Status != 401 {
		t.Fatalf("bad token: %d", res.Status)
	}
	if st, code := resultCode(applyReplan(t, s, diner, "r_anker", pid, "k1")); st != 403 || code != "forbidden" {
		t.Fatalf("non-manager: %d %s", st, code)
	}
	if st, code := resultCode(applyReplan(t, s, mgr, "r_nope", pid, "k1")); st != 404 || code != "not_found" {
		t.Fatalf("unknown restaurant: %d %s", st, code)
	}
	if st, code := resultCode(applyReplan(t, s, mgr, "r_anker", "nosuch", "k1")); st != 404 || code != "not_found" {
		t.Fatalf("unknown plan: %d %s", st, code)
	}
	// A plan from another restaurant is foreign here: preview on r_other,
	// then apply it under r_anker.
	other := s.PreviewReplan(mgr, "r_other", "auth-pv2",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`))
	if other.Status != 201 {
		t.Fatalf("other preview: %d", other.Status)
	}
	oid := planIDOf(t, other)
	if st, code := resultCode(applyReplan(t, s, mgr, "r_anker", oid, "k2")); st != 404 || code != "not_found" {
		t.Fatalf("foreign plan: %d %s", st, code)
	}
	if res := s.ApplyReplan(mgr, "r_anker", pid, "k3", []byte(`{oops`)); res.Status != 400 {
		t.Fatalf("malformed body: %d", res.Status)
	}
}

func TestReplanApplySuccess(t *testing.T) {
	s, mgr, diner := newApplyService(t)
	pv := previewOK(t, s, mgr, "ap-pv")
	pid := planIDOf(t, pv)
	preRev := exportStateMap(t, s)["restaurant_revisions"].(map[string]any)["r_anker"]
	preExp := applyExportBytes(t, s)
	res := applyReplan(t, s, mgr, "r_anker", pid, "ap-1")
	if res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	if len(body) != 3 {
		t.Fatalf("apply shape keys: %v", body)
	}
	for _, k := range []string{"plan_id", "restaurant_revision", "reservations"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing key %q", k)
		}
	}
	if body["plan_id"] != pid {
		t.Fatalf("plan id: %v", body["plan_id"])
	}
	if fmt.Sprintf("%v", body["restaurant_revision"]) == fmt.Sprintf("%v", preRev) {
		t.Fatalf("counter did not increase: %v", body)
	}
	recs, _ := body["reservations"].([]any)
	if len(recs) != 2 {
		t.Fatalf("two considered reservations: %v", body)
	}
	got := map[string]map[string]any{}
	order := []string{}
	for _, r := range recs {
		rm, _ := r.(map[string]any)
		got[rm["reference"].(string)] = rm
		order = append(order, rm["reference"].(string))
	}
	if len(order) != 2 || order[0] != "APAAAA" || order[1] != "APBBBB" {
		t.Fatalf("reference order: %v", order)
	}
	// Full public projection equals owner lookup for each record.
	for ref, rm := range got {
		lk := s.GetReservation(diner, ref)
		if lk.Status != 200 {
			t.Fatalf("lookup %s: %d", ref, lk.Status)
		}
		lj, _ := json.Marshal(lk.Body)
		rj, _ := json.Marshal(rm)
		if string(lj) != string(rj) {
			t.Fatalf("response != lookup for %s:\n%s\n%s", ref, rj, lj)
		}
	}
	am := got["APAAAA"]
	if am["revision"] != 2 {
		t.Fatalf("moved revision: %v", am)
	}
	ids, _ := json.Marshal(am["table_ids"])
	if string(ids) != `["t_3"]` || am["table_id"] != "t_3" {
		t.Fatalf("moved tables: %v", am)
	}
	bm := got["APBBBB"]
	if bm["revision"] != 1 {
		t.Fatalf("unmoved revision: %v", bm)
	}
	// Accepted terms and times survive the repair byte-identically (owner
	// identity lives in state/export, not in the public projection).
	var preEnv2 map[string]any
	_ = json.Unmarshal(preExp, &preEnv2)
	preRec := preEnv2["state"].(map[string]any)["reservations"].(map[string]any)["APAAAA"].(map[string]any)
	if preRec["user_id"] != "u_bea" {
		t.Fatalf("stored owner: %v", preRec["user_id"])
	}
	for _, k := range []string{"accepted_terms", "starts_at_local", "starts_at", "ends_at", "party_size", "restaurant_id", "reservation_id", "reference", "created_at"} {
		xv, _ := json.Marshal(preRec[k])
		yv, _ := json.Marshal(am[k])
		if string(xv) != string(yv) {
			t.Fatalf("moved record field %q changed: %s -> %s", k, xv, yv)
		}
	}
	// Moved history: exactly one reassigned entry with complete table_ids
	// change and the plan id; terms frozen.
	hist := s.ReservationHistory(diner, "APAAAA")
	hm, _ := hist.Body.(map[string]any)
	he, _ := json.Marshal(hm["entries"])
	var entries []any
	_ = json.Unmarshal(he, &entries)
	if len(entries) != 2 {
		t.Fatalf("moved history entries: %v", hm)
	}
	last, _ := entries[1].(map[string]any)
	if last["event"] != "reassigned" || last["revision"] != float64(2) || last["plan_id"] != pid {
		t.Fatalf("reassigned entry: %v", last)
	}
	chs, _ := last["changes"].([]any)
	if len(chs) != 1 {
		t.Fatalf("reassigned changes: %v", last)
	}
	ch, _ := chs[0].(map[string]any)
	if ch["field"] != "table_ids" {
		t.Fatalf("change field: %v", ch)
	}
	cf, _ := json.Marshal(ch["from"])
	ct, _ := json.Marshal(ch["to"])
	if string(cf) != `["t_2"]` || string(ct) != `["t_3"]` {
		t.Fatalf("change from/to: %s -> %s", cf, ct)
	}
	// Unmoved history untouched.
	bh := s.ReservationHistory(diner, "APBBBB")
	bhm, _ := bh.Body.(map[string]any)
	be, _ := json.Marshal(bhm["entries"])
	var bentries []any
	_ = json.Unmarshal(be, &bentries)
	if len(bentries) != 1 {
		t.Fatalf("unmoved history: %v", bhm)
	}
	// Whole-export delta, pinned completely: moved records differ only
	// in table selectors + revision; unchanged records byte-equal; old
	// history prefixes byte-equal with exactly one appended reassigned
	// entry; saved plan fields unchanged except Applied; exact closure
	// appended with priors untouched; FULL counter map with only the
	// target +1; one new receipt with exact binding; all prior
	// plans/receipts and unrelated namespaces byte-equal.
	var preEnv, postEnv map[string]any
	_ = json.Unmarshal(preExp, &preEnv)
	postExp := applyExportBytes(t, s)
	_ = json.Unmarshal(postExp, &postEnv)
	ps := preEnv["state"].(map[string]any)
	qs := postEnv["state"].(map[string]any)
	preRecs := ps["reservations"].(map[string]any)
	postRecs := qs["reservations"].(map[string]any)
	if len(preRecs) != len(postRecs) {
		t.Fatalf("reservation count changed: %d -> %d", len(preRecs), len(postRecs))
	}
	for ref, bv := range preRecs {
		bm, _ := bv.(map[string]any)
		am2, _ := postRecs[ref].(map[string]any)
		if am2 == nil {
			t.Fatalf("record %s vanished", ref)
		}
		if ref == "APAAAA" {
			bj, _ := json.Marshal(bm)
			var bfull map[string]any
			_ = json.Unmarshal(bj, &bfull)
			aj, _ := json.Marshal(am2)
			var afull map[string]any
			_ = json.Unmarshal(aj, &afull)
			for k, v := range bfull {
				if k == "table_id" || k == "table_ids" || k == "revision" {
					continue
				}
				xv, _ := json.Marshal(v)
				yv, _ := json.Marshal(afull[k])
				if string(xv) != string(yv) {
					t.Fatalf("moved record field %q changed: %s -> %s", k, xv, yv)
				}
			}
			if afull["revision"] != float64(2) {
				t.Fatalf("stored moved revision: %v", afull["revision"])
			}
			mt, _ := json.Marshal(afull["table_ids"])
			if string(mt) != `["t_3"]` {
				t.Fatalf("stored moved tables: %s", mt)
			}
			if afull["user_id"] != "u_bea" {
				t.Fatalf("stored moved owner: %v", afull["user_id"])
			}
		} else {
			xv, _ := json.Marshal(bv)
			yv, _ := json.Marshal(am2)
			if string(xv) != string(yv) {
				t.Fatalf("unchanged record %s changed", ref)
			}
		}
	}
	preHist := ps["histories"].(map[string]any)
	postHist := qs["histories"].(map[string]any)
	if len(preHist) != len(postHist) {
		t.Fatalf("history key count: %d -> %d", len(preHist), len(postHist))
	}
	for ref, bv := range preHist {
		av := postHist[ref]
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(av)
		var bl, al []any
		_ = json.Unmarshal(xv, &bl)
		_ = json.Unmarshal(yv, &al)
		if ref == "APAAAA" {
			if len(al) != len(bl)+1 {
				t.Fatalf("moved history length: %d -> %d", len(bl), len(al))
			}
			px, _ := json.Marshal(bl)
			qx, _ := json.Marshal(al[:len(bl)])
			if string(px) != string(qx) {
				t.Fatal("moved history prefix changed")
			}
			ne, _ := al[len(al)-1].(map[string]any)
			if ne["event"] != "reassigned" || ne["revision"] != float64(2) || ne["plan_id"] != pid {
				t.Fatalf("appended entry: %v", ne)
			}
			nc, _ := ne["changes"].([]any)
			if len(nc) != 1 {
				t.Fatalf("appended changes: %v", ne)
			}
			nch, _ := nc[0].(map[string]any)
			nf, _ := json.Marshal(nch["from"])
			nt, _ := json.Marshal(nch["to"])
			if nch["field"] != "table_ids" || string(nf) != `["t_2"]` || string(nt) != `["t_3"]` {
				t.Fatalf("appended change: %v", nch)
			}
			et, _ := json.Marshal(ne["accepted_terms"])
			rt, _ := json.Marshal(postRecs[ref].(map[string]any)["accepted_terms"])
			if string(et) != string(rt) {
				t.Fatal("appended terms differ from record terms")
			}
			if ne["seq"] != float64(2) {
				t.Fatalf("appended seq: %v", ne["seq"])
			}
		} else if string(xv) != string(yv) {
			t.Fatalf("history %s changed", ref)
		}
	}
	bPlans := ps["plans"].(map[string]any)
	aPlans := qs["plans"].(map[string]any)
	if len(aPlans) != len(bPlans) {
		t.Fatalf("plan count: %d -> %d", len(bPlans), len(aPlans))
	}
	for k, bv := range bPlans {
		av := aPlans[k]
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(av)
		if k == pid {
			var bpl, apl map[string]any
			_ = json.Unmarshal(xv, &bpl)
			_ = json.Unmarshal(yv, &apl)
			for fk, fv := range bpl {
				if fk == "applied" {
					continue
				}
				fxj, _ := json.Marshal(fv)
				fyj, _ := json.Marshal(apl[fk])
				if string(fxj) != string(fyj) {
					t.Fatalf("plan field %q changed", fk)
				}
			}
			if bpl["applied"] != false || apl["applied"] != true {
				t.Fatalf("applied flag: %v -> %v", bpl["applied"], apl["applied"])
			}
		} else if string(xv) != string(yv) {
			t.Fatalf("prior plan %q changed", k)
		}
	}
	bClos := ps["closures"].(map[string]any)
	aClos := qs["closures"].(map[string]any)
	for k, bv := range bClos {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(aClos[k])
		if string(xv) != string(yv) {
			t.Fatalf("prior closures %q changed", k)
		}
	}
	bcj, _ := json.Marshal(bClos["r_anker"])
	acj, _ := json.Marshal(aClos["r_anker"])
	if string(bcj) != "null" && string(bcj) != "[]" {
		t.Fatalf("pre closures: %s", bcj)
	}
	var aClosList []any
	_ = json.Unmarshal(acj, &aClosList)
	if len(aClosList) != 1 {
		t.Fatalf("one closure recorded: %s", acj)
	}
	cl, _ := aClosList[0].(map[string]any)
	if cl["table_id"] != "t_2" || cl["from"] != "2027-06-17T18:00:00+02:00" || cl["to"] != "2027-06-17T23:00:00+02:00" {
		t.Fatalf("stored closure: %v", cl)
	}
	bRevMap, _ := ps["restaurant_revisions"].(map[string]any)
	aRevMap, _ := qs["restaurant_revisions"].(map[string]any)
	expRev := map[string]any{}
	for k, v := range bRevMap {
		expRev[k] = v
	}
	expRev["r_anker"] = bRevMap["r_anker"].(float64) + 1
	xr, _ := json.Marshal(expRev)
	yr, _ := json.Marshal(aRevMap)
	if string(xr) != string(yr) {
		t.Fatalf("counter map: %s vs %s", xr, yr)
	}
	if fmt.Sprintf("%v", body["restaurant_revision"]) != fmt.Sprintf("%v", bRevMap["r_anker"].(float64)+1) {
		t.Fatalf("response revision %v", body["restaurant_revision"])
	}
	bRec := ps["receipts"].(map[string]any)
	aRec := qs["receipts"].(map[string]any)
	if len(aRec) != len(bRec)+1 {
		t.Fatalf("apply stores one receipt: %d -> %d", len(bRec), len(aRec))
	}
	for k, v := range bRec {
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(aRec[k])
		if string(xv) != string(yv) {
			t.Fatalf("prior receipt %q changed", k)
		}
	}
	var newRec map[string]any
	for k, v := range aRec {
		if _, ok := bRec[k]; !ok {
			rm, _ := v.(map[string]any)
			newRec = rm
		}
	}
	if newRec == nil {
		t.Fatal("no new receipt")
	}
	if newRec["user_id"] != "u_ada" || newRec["method"] != "POST" ||
		newRec["path"] != "/restaurants/r_anker/replans/"+pid+"/apply" ||
		newRec["key"] != "ap-1" || newRec["status"] != float64(201) {
		t.Fatalf("receipt binding: %v", newRec)
	}
	if newRec["body"] != "{}" {
		t.Fatalf("receipt body: %v", newRec["body"])
	}
	respRaw, _ := json.Marshal(res.Body)
	if newRec["response"] != string(respRaw) {
		t.Fatal("receipt response != original raw response")
	}
	for k, bv := range ps {
		switch k {
		case "plans", "receipts", "reservations", "histories", "closures", "restaurant_revisions":
		default:
			xv, _ := json.Marshal(bv)
			yv, _ := json.Marshal(qs[k])
			if string(xv) != string(yv) {
				t.Fatalf("namespace %q changed", k)
			}
		}
	}
	ap, _ := aPlans[pid].(map[string]any)
	if ap["applied"] != true {
		t.Fatalf("stored plan not applied: %v", ap)
	}
	// Full Public projections equal the sorted response with scalar
	// table_id iff singleton.
	for ref, rm := range got {
		stored, _ := postRecs[ref].(map[string]any)
		var want map[string]any
		wantRaw, _ := json.Marshal(stored)
		_ = json.Unmarshal(wantRaw, &want)
		delete(want, "user_id")
		if len(stored["table_ids"].([]any)) != 1 {
			delete(want, "table_id")
		}
		rj, _ := json.Marshal(rm)
		wj, _ := json.Marshal(want)
		if string(rj) != string(wj) {
			t.Fatalf("response != stored public for %s:\n%s\n%s", ref, rj, wj)
		}
	}
}

func TestReplanApplyEmpty(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pv := s.PreviewReplan(mgr, "r_anker", "e-pv",
		[]byte(`{"table_id":"t_1","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}`))
	if pv.Status != 201 {
		t.Fatalf("preview: %d", pv.Status)
	}
	pid := planIDOf(t, pv)
	preBytes := applyExportBytes(t, s)
	res := applyReplan(t, s, mgr, "r_anker", pid, "e-ap")
	if res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	recs, _ := body["reservations"].([]any)
	if recs == nil || len(recs) != 0 {
		t.Fatalf("empty reservations must be allocated []: %v", body)
	}
	var preEnv, postEnv map[string]any
	_ = json.Unmarshal(preBytes, &preEnv)
	postBytes := applyExportBytes(t, s)
	_ = json.Unmarshal(postBytes, &postEnv)
	ps := preEnv["state"].(map[string]any)
	qs := postEnv["state"].(map[string]any)
	pr := ps["restaurant_revisions"].(map[string]any)["r_anker"].(float64)
	qr := qs["restaurant_revisions"].(map[string]any)["r_anker"].(float64)
	if qr != pr+1 {
		t.Fatalf("counter must bump on empty apply: %v -> %v", pr, qr)
	}
	if fmt.Sprintf("%v", body["restaurant_revision"]) != fmt.Sprintf("%v", qr) {
		t.Fatalf("response revision: %v", body)
	}
	for _, k := range []string{"reservations", "histories", "series", "users", "tokens", "restaurants", "policies"} {
		xv, _ := json.Marshal(ps[k])
		yv, _ := json.Marshal(qs[k])
		if string(xv) != string(yv) {
			t.Fatalf("namespace %q drifted on empty apply", k)
		}
	}
	bp, _ := json.Marshal(ps["plans"])
	ap, _ := json.Marshal(qs["plans"])
	var bpl, apl map[string]any
	_ = json.Unmarshal(bp, &bpl)
	_ = json.Unmarshal(ap, &apl)
	if len(apl) != len(bpl) {
		t.Fatalf("apply stores no new plan: %d -> %d", len(bpl), len(apl))
	}
	ep, _ := apl[pid].(map[string]any)
	if ep["applied"] != true {
		t.Fatalf("empty plan not applied: %v", ep)
	}
	br, _ := json.Marshal(ps["receipts"])
	arq, _ := json.Marshal(qs["receipts"])
	var brm, arm map[string]any
	_ = json.Unmarshal(br, &brm)
	_ = json.Unmarshal(arq, &arm)
	if len(arm) != len(brm)+1 {
		t.Fatalf("one stored receipt: %d -> %d", len(brm), len(arm))
	}
	bh, _ := json.Marshal(ps["histories"])
	ah, _ := json.Marshal(qs["histories"])
	if string(bh) != string(ah) {
		t.Fatal("histories drifted on empty apply")
	}
	br, _ = json.Marshal(ps["reservations"])
	arq, _ = json.Marshal(qs["reservations"])
	if string(br) != string(arq) {
		t.Fatal("records drifted on empty apply")
	}
	ac, _ := json.Marshal(qs["closures"].(map[string]any)["r_anker"])
	var acl []any
	_ = json.Unmarshal(ac, &acl)
	if len(acl) != 1 {
		t.Fatalf("closure recorded: %s", ac)
	}
}

func TestReplanApplyZeroMoveNonEmpty(t *testing.T) {
	// Considered bookings that all keep their tables: the closure is still
	// recorded and the counter still bumps once, with no booking, history
	// or series drift.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1","t_2"],["t_2","t_3"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "a", "reference": "ZMAAAA", "user_id": "u_ada", "restaurant_id": "r_anker", "table_id": "t_1", "starts_at_local": "2027-06-17T19:00", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	pv := s.PreviewReplan(tok, "r_anker", "zm-pv",
		[]byte(`{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if pv.Status != 201 {
		t.Fatalf("preview: %d %v", pv.Status, pv.Body)
	}
	pid := planIDOf(t, pv)
	pre, _ := json.Marshal(s.Export().Body)
	res := applyReplan(t, s, tok, "r_anker", pid, "zm-ap")
	if res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	recs, _ := body["reservations"].([]any)
	if len(recs) != 1 {
		t.Fatalf("one considered: %v", body)
	}
	rm, _ := recs[0].(map[string]any)
	if rm["revision"] != 1 {
		t.Fatalf("unmoved revision: %v", rm)
	}
	post, _ := json.Marshal(s.Export().Body)
	var pe, qe map[string]any
	_ = json.Unmarshal(pre, &pe)
	_ = json.Unmarshal(post, &qe)
	ps := pe["state"].(map[string]any)
	qs := qe["state"].(map[string]any)
	for _, k := range []string{"reservations", "histories", "series", "users", "tokens"} {
		xv, _ := json.Marshal(ps[k])
		yv, _ := json.Marshal(qs[k])
		if string(xv) != string(yv) {
			t.Fatalf("namespace %q drifted on zero-move apply", k)
		}
	}
	pr := ps["restaurant_revisions"].(map[string]any)["r_anker"].(float64)
	qr := qs["restaurant_revisions"].(map[string]any)["r_anker"].(float64)
	if qr != pr+1 {
		t.Fatalf("counter: %v -> %v", pr, qr)
	}
	ac, _ := json.Marshal(qs["closures"].(map[string]any)["r_anker"])
	var acl []any
	_ = json.Unmarshal(ac, &acl)
	if len(acl) != 1 {
		t.Fatalf("closure recorded: %s", ac)
	}
}

func TestReplanApplyPair(t *testing.T) {
	// A pair booking moves to a different declared pair with a complete
	// canonical table_ids change; terms frozen.
	fx := `{"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1","t_2"],["t_2","t_3"]], "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "a", "reference": "PWAAAA", "user_id": "u_ada", "restaurant_id": "r_anker", "table_ids": ["t_1","t_2"], "starts_at_local": "2027-06-17T19:00", "party_size": 6},
    {"id": "b", "reference": "PWBBBB", "user_id": "u_ada", "restaurant_id": "r_anker", "table_id": "t_1", "starts_at_local": "2027-06-17T21:30", "party_size": 1}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "ada@example.com", "correct horse")
	pv := s.PreviewReplan(tok, "r_anker", "pw-pv",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}`))
	if pv.Status != 201 {
		t.Fatalf("preview: %d %v", pv.Status, pv.Body)
	}
	pid := planIDOf(t, pv)
	res := applyReplan(t, s, tok, "r_anker", pid, "pw-ap")
	if res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	body, _ := res.Body.(map[string]any)
	recs, _ := body["reservations"].([]any)
	got := map[string]map[string]any{}
	for _, r := range recs {
		rm, _ := r.(map[string]any)
		got[rm["reference"].(string)] = rm
	}
	ids := func(m map[string]any) string {
		raw, _ := json.Marshal(m["table_ids"])
		return string(raw)
	}
	if ids(got["PWAAAA"]) != `["t_2","t_3"]` || got["PWAAAA"]["revision"] != 2 {
		t.Fatalf("pair must move canonically with revision+1: %v", got["PWAAAA"])
	}
	if _, ok := got["PWAAAA"]["table_id"]; ok {
		t.Fatalf("pair response must omit scalar table_id: %v", got["PWAAAA"])
	}
	if got["PWAAAA"]["revision"] != 2 {
		t.Fatalf("pair revision: %v", got["PWAAAA"])
	}
	hist := s.ReservationHistory(tok, "PWAAAA")
	hm, _ := hist.Body.(map[string]any)
	he, _ := json.Marshal(hm["entries"])
	var entries []any
	_ = json.Unmarshal(he, &entries)
	last, _ := entries[len(entries)-1].(map[string]any)
	if last["event"] != "reassigned" {
		t.Fatalf("pair history event: %v", last)
	}
	chs, _ := last["changes"].([]any)
	ch, _ := chs[0].(map[string]any)
	cf, _ := json.Marshal(ch["from"])
	ct, _ := json.Marshal(ch["to"])
	if ch["field"] != "table_ids" || string(cf) != `["t_1","t_2"]` || string(ct) != `["t_2","t_3"]` {
		t.Fatalf("pair change arrays: %v", ch)
	}
}

func TestReplanApplyStale(t *testing.T) {
	mutators := map[string]func(t *testing.T, s *Service, mgr, diner string){
		"create": func(t *testing.T, s *Service, mgr, diner string) {
			res := s.CreateReservation(diner, "st-cr",
				[]byte(`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}`))
			if res.Status != 201 {
				t.Fatalf("create: %d", res.Status)
			}
		},
		"real-patch": func(t *testing.T, s *Service, mgr, diner string) {
			res := s.PatchReservation(diner, "APBBBB", []byte(`{"party_size":1}`))
			if res.Status != 200 {
				t.Fatalf("patch: %d %v", res.Status, res.Body)
			}
		},
		"cancel": func(t *testing.T, s *Service, mgr, diner string) {
			res := s.CancelReservation(diner, "APBBBB")
			if res.Status != 200 {
				t.Fatalf("cancel: %d", res.Status)
			}
		},
		"publish": func(t *testing.T, s *Service, mgr, diner string) {
			if res := s.PublishPolicy(mgr, "r_anker", "st-pub", validPolicyBody("2027-08-01")); res.Status != 201 {
				t.Fatalf("publish: %d", res.Status)
			}
		},
		"other-apply": func(t *testing.T, s *Service, mgr, diner string) {
			p2 := previewOK(t, s, mgr, "st-other-pv")
			p2id := planIDOf(t, p2)
			if res := applyReplan(t, s, mgr, "r_anker", p2id, "st-other-ap"); res.Status != 201 {
				t.Fatalf("other apply: %d", res.Status)
			}
		},
	}
	for name, mutate := range mutators {
		s, mgr, diner := newApplyService(t)
		pid := planIDOf(t, previewOK(t, s, mgr, "st-pv"))
		mutate(t, s, mgr, diner)
		pre, _ := json.Marshal(s.Export().Body)
		res := applyReplan(t, s, mgr, "r_anker", pid, "st-ap")
		if st, code := resultCode(res); st != 409 || code != "stale_plan" {
			t.Fatalf("%s: got %d %s", name, st, code)
		}
		post, _ := json.Marshal(s.Export().Body)
		if string(pre) != string(post) {
			t.Fatalf("%s: stale apply changed state", name)
		}
		// Failed key stays genuinely reusable with a fresh valid plan.
		p3 := previewOK(t, s, mgr, "st-pv3")
		p3id := planIDOf(t, p3)
		if res := applyReplan(t, s, mgr, "r_anker", p3id, "st-ap"); res.Status != 201 {
			t.Fatalf("%s reuse: %d %v", name, res.Status, res.Body)
		}
	}
}

func TestReplanApplyNonStaling(t *testing.T) {
	s, mgr, diner := newApplyService(t)
	// A committed create BEFORE preview (its replay later changes nothing).
	cr := s.CreateReservation(diner, "ns-cr",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}`))
	if cr.Status != 201 {
		t.Fatalf("create: %d", cr.Status)
	}
	pid := planIDOf(t, previewOK(t, s, mgr, "ns-pv"))
	// Replay of the earlier create: no state change.
	cr2 := s.CreateReservation(diner, "ns-cr",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}`))
	if cr2.Status != 200 {
		t.Fatalf("replay: %d", cr2.Status)
	}
	// No-op patch: same values, still succeeds.
	if res := s.PatchReservation(diner, "APBBBB", []byte(`{}`)); res.Status != 200 {
		t.Fatalf("noop patch: %d", res.Status)
	}
	// Failed patch: invalid party.
	if res := s.PatchReservation(diner, "APBBBB", []byte(`{"party_size":0}`)); res.Status != 422 {
		t.Fatalf("failed patch: %d", res.Status)
	}
	// Another preview: no counter change.
	previewOK(t, s, mgr, "ns-pv2")
	// Other-restaurant booking: different counter.
	or := s.CreateReservation(diner, "ns-or",
		[]byte(`{"restaurant_id":"r_other","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":1}`))
	if or.Status != 201 {
		t.Fatalf("other restaurant create: %d", or.Status)
	}
	if res := applyReplan(t, s, mgr, "r_anker", pid, "ns-ap"); res.Status != 201 {
		t.Fatalf("apply after non-staling writes: %d %v", res.Status, res.Body)
	}
}

func TestReplanApplyAlreadyApplied(t *testing.T) {
	s, mgr, diner := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "aa-pv"))
	first := applyReplan(t, s, mgr, "r_anker", pid, "aa-1")
	if first.Status != 201 {
		t.Fatalf("first apply: %d", first.Status)
	}
	firstJSON, _ := json.Marshal(first.Body)
	// Baseline before the failed fresh-key request.
	baseBad, _ := json.Marshal(s.Export().Body)
	res := applyReplan(t, s, mgr, "r_anker", pid, "aa-2")
	if st, code := resultCode(res); st != 409 || code != "plan_already_applied" {
		t.Fatalf("second key: %d %s", st, code)
	}
	afterBad, _ := json.Marshal(s.Export().Body)
	if string(baseBad) != string(afterBad) {
		t.Fatal("already-applied failure changed state")
	}
	// Prove a real later edit occurred: different current record and an
	// advanced counter versus the original apply response.
	pe := s.PatchReservation(diner, "APBBBB", []byte(`{"party_size":1}`))
	if pe.Status != 200 {
		t.Fatalf("later edit: %d", pe.Status)
	}
	cur, _ := json.Marshal(s.GetReservation(diner, "APBBBB").Body)
	var firstEnv map[string]any
	_ = json.Unmarshal(firstJSON, &firstEnv)
	found := false
	for _, r := range firstEnv["reservations"].([]any) {
		rm, _ := r.(map[string]any)
		if rm["reference"] == "APBBBB" {
			found = true
			orig, _ := json.Marshal(rm)
			if string(orig) == string(cur) {
				t.Fatal("later edit did not change the record")
			}
		}
	}
	if !found {
		t.Fatal("APBBBB missing from original response")
	}
	baseReplay, _ := json.Marshal(s.Export().Body)
	replay := applyReplan(t, s, mgr, "r_anker", pid, "aa-1")
	if replay.Status != 200 {
		t.Fatalf("replay: %d", replay.Status)
	}
	replayJSON, _ := json.Marshal(replay.Body)
	if string(firstJSON) != string(replayJSON) {
		t.Fatal("replay bytes differ")
	}
	afterReplay, _ := json.Marshal(s.Export().Body)
	if string(baseReplay) != string(afterReplay) {
		t.Fatal("replay changed state")
	}
	// Changed body on the original key is a key-reuse conflict; baseline
	// the export around it too.
	baseReuse, _ := json.Marshal(s.Export().Body)
	if st, code := resultCode(s.ApplyReplan(mgr, "r_anker", pid, "aa-1", []byte(`{"other":1}`))); st != 409 || code != "idempotency_key_reuse" {
		t.Fatalf("changed body: %d %s", st, code)
	}
	afterReuse, _ := json.Marshal(s.Export().Body)
	if string(baseReuse) != string(afterReuse) {
		t.Fatal("reuse conflict changed state")
	}
}

func TestReplanApplyDemotion(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "dm-pv"))
	first := applyReplan(t, s, mgr, "r_anker", pid, "dm-1")
	if first.Status != 201 {
		t.Fatalf("apply: %d", first.Status)
	}
	firstJSON, _ := json.Marshal(first.Body)
	// Complete export baseline around the demotion itself: the strip must
	// change only the restaurant configuration, nothing else.
	preDemote, _ := json.Marshal(s.Export().Body)
	s.mu.Lock()
	for i := range s.state.Restaurants {
		if s.state.Restaurants[i].ID == "r_anker" {
			s.state.Restaurants[i].ManagerUserIDs = []string{"u_nobody"}
		}
	}
	s.mu.Unlock()
	postDemote, _ := json.Marshal(s.Export().Body)
	var preDE, postDE map[string]any
	_ = json.Unmarshal(preDemote, &preDE)
	_ = json.Unmarshal(postDemote, &postDE)
	preDS := preDE["state"].(map[string]any)
	postDS := postDE["state"].(map[string]any)
	for k, bv := range preDS {
		if k == "restaurants" {
			continue
		}
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postDS[k])
		if string(xv) != string(yv) {
			t.Fatalf("demotion changed namespace %q", k)
		}
	}
	var preRs, postRs []any
	_ = json.Unmarshal(mustJSON(t, preDS["restaurants"]), &preRs)
	_ = json.Unmarshal(mustJSON(t, postDS["restaurants"]), &postRs)
	if len(preRs) != len(postRs) {
		t.Fatalf("restaurant count: %d -> %d", len(preRs), len(postRs))
	}
	for i := range preRs {
		pm, _ := preRs[i].(map[string]any)
		qm, _ := postRs[i].(map[string]any)
		if pm["id"] != qm["id"] {
			t.Fatalf("restaurant order changed: %v vs %v", pm["id"], qm["id"])
		}
		if pm["id"] == "r_anker" {
			pmj, _ := json.Marshal(pm["manager_user_ids"])
			qmj, _ := json.Marshal(qm["manager_user_ids"])
			if string(pmj) != `["u_ada"]` || string(qmj) != `["u_nobody"]` {
				t.Fatalf("manager strip: %s -> %s", pmj, qmj)
			}
			delete(pm, "manager_user_ids")
			delete(qm, "manager_user_ids")
		}
		px, _ := json.Marshal(pm)
		qx, _ := json.Marshal(qm)
		if string(px) != string(qx) {
			t.Fatalf("restaurant %v drifted beyond managers", pm["id"])
		}
	}
	baseReplay, _ := json.Marshal(s.Export().Body)
	replay := applyReplan(t, s, mgr, "r_anker", pid, "dm-1")
	if replay.Status != 200 {
		t.Fatalf("replay after demotion: %d", replay.Status)
	}
	replayJSON, _ := json.Marshal(replay.Body)
	if string(firstJSON) != string(replayJSON) {
		t.Fatal("replay bytes differ after demotion")
	}
	afterReplay, _ := json.Marshal(s.Export().Body)
	if string(baseReplay) != string(afterReplay) {
		t.Fatal("replay after demotion changed state")
	}
	baseFresh, _ := json.Marshal(s.Export().Body)
	if st, _ := resultCode(applyReplan(t, s, mgr, "r_anker", pid, "dm-2")); st != 403 {
		t.Fatalf("fresh key after demotion: %d", st)
	}
	afterFresh, _ := json.Marshal(s.Export().Body)
	if string(baseFresh) != string(afterFresh) {
		t.Fatal("fresh-key refusal changed state")
	}
}

func TestReplanApplySeries(t *testing.T) {
	fx := `{"users": [{"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}],
  "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
      "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "tables": [{"id": "t_1", "label": "1", "capacity": 2},{"id": "t_2", "label": "2", "capacity": 4},{"id": "t_3", "label": "3", "capacity": 4}],
      "combinable": [["t_1","t_2"],["t_2","t_3"]], "manager_user_ids": ["u_bea"]}],
  "reservations": [
    {"id": "a1", "reference": "SEAAAA", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 2},
    {"id": "a2", "reference": "SEBBBB", "user_id": "u_bea", "restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-24T21:30", "party_size": 2}]}`
	s := New()
	if res := s.Reset([]byte(fx)); res.Status != 204 {
		t.Fatalf("reset: %d %v", res.Status, res.Body)
	}
	tok := policyLogin(t, s, "bea@example.com", "correct horse bea")
	// S1: anchor SEAAAA (06-17) + generated member (06-24).
	a1 := s.AdoptSeries(tok, "se-1", []byte(`{"anchor_reference":"SEAAAA","count":2,"interval_weeks":1}`))
	if a1.Status != 201 {
		t.Fatalf("adopt S1: %d %v", a1.Status, a1.Body)
	}
	a1m, _ := a1.Body.(map[string]any)
	sid1, _ := a1m["series_id"].(string)
	// S2: anchor SEBBBB (06-24) + generated member (07-01).
	a2 := s.AdoptSeries(tok, "se-2", []byte(`{"anchor_reference":"SEBBBB","count":2,"interval_weeks":1}`))
	if a2.Status != 201 {
		t.Fatalf("adopt S2: %d %v", a2.Status, a2.Body)
	}
	a2m, _ := a2.Body.(map[string]any)
	sid2, _ := a2m["series_id"].(string)
	// Mark the far-future S2 occurrence an exception first (flags retained).
	occs, _ := a2m["occurrences"].([]any)
	o1, _ := occs[1].(map[string]any)
	or1, _ := o1["reservation"].(map[string]any)
	o1ref, _ := or1["reference"].(string)
	if res := s.PatchReservation(tok, o1ref, []byte(`{"party_size":1}`)); res.Status != 200 {
		t.Fatalf("exception patch: %d %v", res.Status, res.Body)
	}
	// S3: fully outside the closure window; untouched by the apply.
	cr3 := s.CreateReservation(tok, "se-3a",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-07-01T19:00","party_size":1}`))
	if cr3.Status != 201 {
		t.Fatalf("S3 anchor: %d %v", cr3.Status, cr3.Body)
	}
	cr3m, _ := cr3.Body.(map[string]any)
	a3 := s.AdoptSeries(tok, "se-3", []byte(`{"anchor_reference":"`+cr3m["reference"].(string)+`","count":2,"interval_weeks":1}`))
	if a3.Status != 201 {
		t.Fatalf("adopt S3: %d %v", a3.Status, a3.Body)
	}
	a3m, _ := a3.Body.(map[string]any)
	sid3, _ := a3m["series_id"].(string)
	preSeries, _ := json.Marshal(s.Export().Body)
	// Wide closure covers both anchors' evenings; three members move.
	pv := s.PreviewReplan(tok, "r_anker", "se-pv",
		[]byte(`{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-24T23:00:00+02:00"}`))
	if pv.Status != 201 {
		t.Fatalf("preview: %d %v", pv.Status, pv.Body)
	}
	pid := planIDOf(t, pv)
	res := applyReplan(t, s, tok, "r_anker", pid, "se-ap")
	if res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	// S1 touched once (two moved members), S2 touched once.
	for _, tc := range []struct {
		sid      string
		revision int
	}{
		{sid1, 2},
		{sid2, 3},
	} {
		cur := s.GetSeries(tok, tc.sid)
		cm, _ := cur.Body.(map[string]any)
		if cur.Status != 200 {
			t.Fatalf("series %s: %d", tc.sid, cur.Status)
		}
		if fmt.Sprintf("%v", cm["revision"]) != fmt.Sprintf("%d", tc.revision) {
			t.Fatalf("series %s revision %v, want %d", tc.sid, cm["revision"], tc.revision)
		}
	}
	// Exception flags: pre-existing far member stays true, rest false.
	cur := s.GetSeries(tok, sid2)
	cm, _ := cur.Body.(map[string]any)
	occs, _ = cm["occurrences"].([]any)
	flags := map[string]bool{}
	for _, o := range occs {
		om, _ := o.(map[string]any)
		rm, _ := om["reservation"].(map[string]any)
		ef, _ := om["exception"].(bool)
		flags[rm["reference"].(string)] = ef
	}
	if !flags[o1ref] || len(flags) != 2 {
		t.Fatalf("exception flags: %v", flags)
	}
	cur = s.GetSeries(tok, sid1)
	cm, _ = cur.Body.(map[string]any)
	occs, _ = cm["occurrences"].([]any)
	for _, o := range occs {
		om, _ := o.(map[string]any)
		if ef, _ := om["exception"].(bool); ef {
			t.Fatalf("unexpected exception: %v", om)
		}
	}
	// Unrelated S3 series byte-identical; full counter map has only r_anker+1.
	postSeries, _ := json.Marshal(s.Export().Body)
	var preSE, postSE map[string]any
	_ = json.Unmarshal(preSeries, &preSE)
	_ = json.Unmarshal(postSeries, &postSE)
	preSt := preSE["state"].(map[string]any)
	postSt := postSE["state"].(map[string]any)
	for k, v := range preSt["series"].(map[string]any) {
		if k == sid1 || k == sid2 {
			continue
		}
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(postSt["series"].(map[string]any)[k])
		if string(xv) != string(yv) {
			t.Fatalf("unrelated series %q changed", k)
		}
	}
	cur3 := s.GetSeries(tok, sid3)
	cm3, _ := cur3.Body.(map[string]any)
	if cur3.Status != 200 || fmt.Sprintf("%v", cm3["revision"]) != "1" {
		t.Fatalf("unrelated series revision: %d %v", cur3.Status, cm3)
	}
	// Unmoved far occurrence byte-equal in record and history.
	for _, coll := range []string{"reservations", "histories"} {
		xv, _ := json.Marshal(preSt[coll].(map[string]any)[o1ref])
		yv, _ := json.Marshal(postSt[coll].(map[string]any)[o1ref])
		if string(xv) != string(yv) {
			t.Fatalf("unmoved %s/%s changed", coll, o1ref)
		}
	}
	preC := preSt["restaurant_revisions"].(map[string]any)
	postC := postSt["restaurant_revisions"].(map[string]any)
	expC := map[string]any{}
	for k, v := range preC {
		expC[k] = v
	}
	expC["r_anker"] = preC["r_anker"].(float64) + 1
	xc, _ := json.Marshal(expC)
	yc, _ := json.Marshal(postC)
	if string(xc) != string(yc) {
		t.Fatalf("counter map: %s vs %s", xc, yc)
	}
	// Moved anchor gained revision + reassigned entry with frozen terms.
	lk := s.GetReservation(tok, "SEAAAA")
	lm, _ := lk.Body.(map[string]any)
	if lk.Status != 200 || lm["revision"] != 2 || lm["starts_at_local"] != "2027-06-17T19:00" {
		t.Fatalf("moved anchor: %d %v", lk.Status, lm)
	}
	ids, _ := json.Marshal(lm["table_ids"])
	if string(ids) != `["t_1"]` {
		t.Fatalf("anchor moved to: %s", ids)
	}
	hist := s.ReservationHistory(tok, "SEAAAA")
	hm, _ := hist.Body.(map[string]any)
	he, _ := json.Marshal(hm["entries"])
	var entries []any
	_ = json.Unmarshal(he, &entries)
	last, _ := entries[len(entries)-1].(map[string]any)
	if last["event"] != "reassigned" || last["plan_id"] != pid {
		t.Fatalf("moved history: %v", last)
	}
	// Affected series keep every field except revision: member indices,
	// references, scheduled dates and exception flags are full-equal pre
	// to post; only the revision advances once per affected series.
	genRefOf := func(body map[string]any) string {
		occs, _ := body["occurrences"].([]any)
		o, _ := occs[1].(map[string]any)
		r, _ := o["reservation"].(map[string]any)
		ref, _ := r["reference"].(string)
		if ref == "" {
			t.Fatalf("generated occurrence reference: %v", o)
		}
		return ref
	}
	s1gen := genRefOf(a1m)
	preSer := preSt["series"].(map[string]any)
	postSer := postSt["series"].(map[string]any)
	if len(preSer) != len(postSer) {
		t.Fatalf("series count: %d -> %d", len(preSer), len(postSer))
	}
	for sid, wantRev := range map[string]int{sid1: 2, sid2: 3} {
		var bsz, asz map[string]any
		_ = json.Unmarshal(mustJSON(t, preSer[sid]), &bsz)
		_ = json.Unmarshal(mustJSON(t, postSer[sid]), &asz)
		if fmt.Sprintf("%v", asz["revision"]) != fmt.Sprintf("%d", wantRev) {
			t.Fatalf("series %s revision: %v", sid, asz["revision"])
		}
		delete(bsz, "revision")
		delete(asz, "revision")
		bj, _ := json.Marshal(bsz)
		aj, _ := json.Marshal(asz)
		if string(bj) != string(aj) {
			t.Fatalf("series %s drifted beyond revision:\n%s\n%s", sid, bj, aj)
		}
	}
	// The preview considered exactly the three in-window t_2 bookings;
	// each moved off the closed table to its previewed assignment.
	pvm, _ := pv.Body.(map[string]any)
	wantTables := map[string]string{}
	for _, a := range pvm["assignments"].([]any) {
		am, _ := a.(map[string]any)
		ref, _ := am["reference"].(string)
		tids, _ := json.Marshal(am["table_ids"])
		wantTables[ref] = string(tids)
		if am["changed"] != true {
			t.Fatalf("considered member must move off closed t_2: %v", am)
		}
	}
	wantMoved := map[string]bool{"SEAAAA": true, s1gen: true, "SEBBBB": true}
	if len(wantTables) != 3 {
		t.Fatalf("considered set: %v", wantTables)
	}
	for ref := range wantMoved {
		if _, ok := wantTables[ref]; !ok {
			t.Fatalf("considered set missing %s: %v", ref, wantTables)
		}
	}
	// Every moved booking: revision +1, only selectors change, times and
	// terms frozen, history prefix preserved plus exactly one reassigned
	// (never Changed) with full From/To and the plan id. Unmoved records
	// and histories are byte-equal.
	preRes := preSt["reservations"].(map[string]any)
	postRes := postSt["reservations"].(map[string]any)
	preHst := preSt["histories"].(map[string]any)
	postHst := postSt["histories"].(map[string]any)
	moved := map[string]bool{}
	for ref, bv := range preRes {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postRes[ref])
		if string(xv) != string(yv) {
			moved[ref] = true
		}
	}
	if len(moved) != len(wantMoved) {
		t.Fatalf("moved set: %v", moved)
	}
	for ref := range wantMoved {
		if !moved[ref] {
			t.Fatalf("expected moved %s unchanged", ref)
		}
		var bm, am map[string]any
		_ = json.Unmarshal(mustJSON(t, preRes[ref]), &bm)
		_ = json.Unmarshal(mustJSON(t, postRes[ref]), &am)
		for k, v := range bm {
			if k == "table_id" || k == "table_ids" || k == "revision" {
				continue
			}
			xv, _ := json.Marshal(v)
			yv, _ := json.Marshal(am[k])
			if string(xv) != string(yv) {
				t.Fatalf("moved %s field %q changed", ref, k)
			}
		}
		if am["revision"] != bm["revision"].(float64)+1 {
			t.Fatalf("moved %s revision: %v -> %v", ref, bm["revision"], am["revision"])
		}
		at, _ := json.Marshal(am["table_ids"])
		if string(at) != wantTables[ref] || string(at) == `["t_2"]` {
			t.Fatalf("moved %s tables %s, want %s", ref, at, wantTables[ref])
		}
		var bl, al []any
		_ = json.Unmarshal(mustJSON(t, preHst[ref]), &bl)
		_ = json.Unmarshal(mustJSON(t, postHst[ref]), &al)
		if len(al) != len(bl)+1 {
			t.Fatalf("moved %s history length: %d -> %d", ref, len(bl), len(al))
		}
		px, _ := json.Marshal(bl)
		qx, _ := json.Marshal(al[:len(bl)])
		if string(px) != string(qx) {
			t.Fatalf("moved %s history prefix changed", ref)
		}
		ne, _ := al[len(al)-1].(map[string]any)
		if ne["event"] != "reassigned" || ne["plan_id"] != pid {
			t.Fatalf("moved %s appended entry: %v", ref, ne)
		}
		if fmt.Sprintf("%v", ne["revision"]) != fmt.Sprintf("%v", am["revision"]) {
			t.Fatalf("moved %s entry revision %v vs record %v", ref, ne["revision"], am["revision"])
		}
		nc, _ := ne["changes"].([]any)
		if len(nc) != 1 {
			t.Fatalf("moved %s appended changes: %v", ref, ne)
		}
		nch, _ := nc[0].(map[string]any)
		bt, _ := json.Marshal(bm["table_ids"])
		nf, _ := json.Marshal(nch["from"])
		nt, _ := json.Marshal(nch["to"])
		if nch["field"] != "table_ids" || string(nf) != string(bt) || string(nt) != string(at) {
			t.Fatalf("moved %s change From/To: %v", ref, nch)
		}
	}
	for ref, bv := range preRes {
		if moved[ref] {
			continue
		}
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postRes[ref])
		if string(xv) != string(yv) {
			t.Fatalf("unmoved record %s changed", ref)
		}
		xh, _ := json.Marshal(preHst[ref])
		yh, _ := json.Marshal(postHst[ref])
		if string(xh) != string(yh) {
			t.Fatalf("unmoved history %s changed", ref)
		}
	}
	// Series occurrences bind to stored members and current records: each
	// occurrence keeps its member index/reference/exception, and its
	// embedded reservation equals the owner lookup of the post record.
	for _, sid := range []string{sid1, sid2} {
		cur := s.GetSeries(tok, sid)
		if cur.Status != 200 {
			t.Fatalf("series %s: %d", sid, cur.Status)
		}
		cm, _ := cur.Body.(map[string]any)
		var members []any
		_ = json.Unmarshal(mustJSON(t, postSer[sid].(map[string]any)["members"]), &members)
		byRef := map[string]map[string]any{}
		for _, mm := range members {
			mm2, _ := mm.(map[string]any)
			byRef[mm2["reference"].(string)] = mm2
		}
		for _, o := range cm["occurrences"].([]any) {
			om, _ := o.(map[string]any)
			ref, _ := om["reference"].(string)
			mem := byRef[ref]
			if mem == nil {
				t.Fatalf("occurrence %s not a stored member", ref)
			}
			if fmt.Sprintf("%v", om["index"]) != fmt.Sprintf("%v", mem["index"]) || om["exception"] != mem["exception"] {
				t.Fatalf("occurrence %s index/exception drift: %v vs %v", ref, om, mem)
			}
			lk := s.GetReservation(tok, ref)
			if lk.Status != 200 {
				t.Fatalf("lookup %s: %d", ref, lk.Status)
			}
			oj, _ := json.Marshal(om["reservation"])
			gj, _ := json.Marshal(lk.Body)
			if string(oj) != string(gj) {
				t.Fatalf("occurrence %s != current record:\n%s\n%s", ref, oj, gj)
			}
		}
	}
}
func TestReplanApplyAvailabilityExplain(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "av-pv"))
	if res := applyReplan(t, s, mgr, "r_anker", pid, "av-ap"); res.Status != 201 {
		t.Fatalf("apply: %d %v", res.Status, res.Body)
	}
	// Table and rule order are part of the contract: every explained slot
	// lists each fixture table exactly once in fixture order, each entry
	// carries the effective policy_version with rules capacity then
	// no_overlap, and available == capacity && no_overlap.
	explainTables := func(t *testing.T, slot map[string]any) []map[string]any {
		t.Helper()
		ex, ok := slot["explain"].([]any)
		if !ok || len(ex) != 3 {
			t.Fatalf("explain must list all 3 tables once: %v", slot["explain"])
		}
		out := make([]map[string]any, 0, len(ex))
		for i, e := range ex {
			em, _ := e.(map[string]any)
			if em["table_id"] != []string{"t_1", "t_2", "t_3"}[i] {
				t.Fatalf("fixture table order: %v", ex)
			}
			rs, _ := em["rules"].([]any)
			if len(rs) != 2 || rs[0].(map[string]any)["rule"] != "capacity" ||
				rs[1].(map[string]any)["rule"] != "no_overlap" {
				t.Fatalf("rule order capacity,no_overlap: %v", em)
			}
			var pvnum float64
			switch pv := em["policy_version"].(type) {
			case int:
				pvnum = float64(pv)
			case float64:
				pvnum = pv
			default:
				t.Fatalf("effective policy_version 0: %v", em)
			}
			if pvnum != 0 {
				t.Fatalf("effective policy_version 0: %v", em)
			}
			capHolds, _ := rs[0].(map[string]any)["holds"].(bool)
			freeHolds, _ := rs[1].(map[string]any)["holds"].(bool)
			if em["available"] != (capHolds && freeHolds) {
				t.Fatalf("available != conjunction: %v", em)
			}
			out = append(out, em)
		}
		return out
	}
	holdsOf := func(em map[string]any) map[string]bool {
		rm := map[string]bool{}
		for _, r := range em["rules"].([]any) {
			rn, _ := r.(map[string]any)
			rm[rn["rule"].(string)] = rn["holds"].(bool)
		}
		return rm
	}
	// Party 4 at the 19:00 slot: t_1 capacity-holds-false/no_overlap-
	// holds-false (APBBBB sits there and seats only 2), t_2 capacity-
	// holds-true/no_overlap-holds-false (closure only), t_3 capacity-
	// holds-true/no_overlap-holds-false (APAAAA moved there). Available
	// is false in all three; ids and options are exactly [].
	m := s.Availability(map[string][]string{
		"restaurant_id": {"r_anker"}, "date": {"2027-06-17"}, "party_size": {"4"}, "explain": {"true"},
	})
	if m.Status != 200 {
		t.Fatalf("availability: %d", m.Status)
	}
	bm, _ := m.Body.(map[string]any)
	slots, _ := bm["slots"].([]any)
	if len(slots) == 0 {
		t.Fatal("no slots")
	}
	sl, _ := slots[0].(map[string]any)
	if sl["starts_at_local"] == "" {
		t.Fatal("first slot missing")
	}
	// Pin the 19:00 slot, exactly covered by both moved/staying bookings:
	// t_1 capacity-holds-false/no_overlap-holds-false, t_2 capacity-
	// holds-true/no_overlap-holds-false (closure only), t_3 capacity-
	// holds-true/no_overlap-holds-false. Available is false in all three
	// cases; ids and options are exactly allocated [].
	var slot19 map[string]any
	for _, s := range slots {
		sm, _ := s.(map[string]any)
		if sm["starts_at_local"] == "2027-06-17T19:00" {
			slot19 = sm
		}
	}
	if slot19 == nil {
		t.Fatal("19:00 slot missing")
	}
	ids, _ := json.Marshal(slot19["available_table_ids"])
	if string(ids) != `[]` {
		t.Fatalf("closed-out ids must be exactly []: %s", ids)
	}
	opts, _ := json.Marshal(slot19["available_options"])
	if string(opts) != `[]` {
		t.Fatalf("closed-out options must be exactly []: %s", opts)
	}
	ext := explainTables(t, slot19)
	rules := map[string]map[string]bool{}
	for _, em := range ext {
		rules[em["table_id"].(string)] = holdsOf(em)
		if em["available"].(bool) {
			t.Fatalf("table available despite closure/capacity: %v", em)
		}
	}
	if rules["t_1"]["capacity"] || rules["t_1"]["no_overlap"] {
		t.Fatalf("t_1 must be capacity-holds-false/no_overlap-holds-false: %v", rules["t_1"])
	}
	if !rules["t_2"]["capacity"] || rules["t_2"]["no_overlap"] {
		t.Fatalf("t_2 must be capacity-holds-true/no_overlap-holds-false: %v", rules["t_2"])
	}
	if !rules["t_3"]["capacity"] || rules["t_3"]["no_overlap"] {
		t.Fatalf("t_3 must be capacity-holds-true/no_overlap-holds-false: %v", rules["t_3"])
	}
	// True/true on a genuinely free slot: party 4 at 21:30 (the last grid
	// slot, 21:30-23:00). Both 19:00 bookings end at 20:30 and the closure
	// covers only t_2, so t_3 is capacity-holds-true/no_overlap-holds-true
	// and available. t_2 stays capacity-holds-true/no_overlap-holds-false.
	mf := s.Availability(map[string][]string{
		"restaurant_id": {"r_anker"}, "date": {"2027-06-17"}, "party_size": {"4"}, "explain": {"true"},
	})
	bmf, _ := mf.Body.(map[string]any)
	slotsf, _ := bmf["slots"].([]any)
	var slot2130 map[string]any
	for _, x := range slotsf {
		xm, _ := x.(map[string]any)
		if xm["starts_at_local"] == "2027-06-17T21:30" {
			slot2130 = xm
		}
	}
	if slot2130 == nil {
		t.Fatal("21:30 slot missing")
	}
	idsf, _ := json.Marshal(slot2130["available_table_ids"])
	if string(idsf) != `["t_3"]` {
		t.Fatalf("21:30 ids: %s", idsf)
	}
	optsf, _ := json.Marshal(slot2130["available_options"])
	if string(optsf) != `[{"capacity":4,"table_ids":["t_3"]}]` {
		t.Fatalf("21:30 options: %s", optsf)
	}
	exf := explainTables(t, slot2130)
	rf := map[string]map[string]bool{}
	for _, em := range exf {
		rf[em["table_id"].(string)] = holdsOf(em)
	}
	if rf["t_3"]["capacity"] != true || rf["t_3"]["no_overlap"] != true {
		t.Fatalf("t_3 must be holds-true/holds-true: %v", rf["t_3"])
	}
	if exf[2]["available"] != true {
		t.Fatalf("t_3 must be available: %v", exf[2])
	}
	if rf["t_2"]["capacity"] != true || rf["t_2"]["no_overlap"] != false {
		t.Fatalf("t_2 must be holds-true/holds-false: %v", rf["t_2"])
	}
	if rf["t_1"]["capacity"] != false || rf["t_1"]["no_overlap"] != true {
		t.Fatalf("t_1 must be holds-false/holds-true: %v", rf["t_1"])
	}
	// Closure-driven false/false independent of any booking: party 8 at
	// the same 21:30 slot. t_2 seats only 4 (capacity-holds-false) and is
	// closed (no_overlap-holds-false from the closure, not from a booking:
	// no booking sits on t_2 at 21:30 since APAAAA moved away).
	mc := s.Availability(map[string][]string{
		"restaurant_id": {"r_anker"}, "date": {"2027-06-17"}, "party_size": {"8"}, "explain": {"true"},
	})
	bmc, _ := mc.Body.(map[string]any)
	var slotC map[string]any
	for _, x := range bmc["slots"].([]any) {
		xm, _ := x.(map[string]any)
		if xm["starts_at_local"] == "2027-06-17T21:30" {
			slotC = xm
		}
	}
	if slotC == nil {
		t.Fatal("21:30 party-8 slot missing")
	}
	exc := explainTables(t, slotC)
	rc := map[string]map[string]bool{}
	for _, em := range exc {
		rc[em["table_id"].(string)] = holdsOf(em)
		if em["available"].(bool) {
			t.Fatalf("party 8 available: %v", em)
		}
	}
	if rc["t_2"]["capacity"] != false || rc["t_2"]["no_overlap"] != false {
		t.Fatalf("closed t_2 must be holds-false/holds-false: %v", rc["t_2"])
	}
	// Fourth combination elsewhere: capacity-holds-false with no overlap
	// and no closure (party 8 on an empty date).
	mfe := s.Availability(map[string][]string{
		"restaurant_id": {"r_anker"}, "date": {"2027-06-24"}, "party_size": {"8"}, "explain": {"true"},
	})
	bmfe, _ := mfe.Body.(map[string]any)
	slfe, _ := bmfe["slots"].([]any)[0].(map[string]any)
	exfe := explainTables(t, slfe)
	rfe := holdsOf(exfe[0])
	if rfe["capacity"] || !rfe["no_overlap"] || exfe[0]["available"].(bool) {
		t.Fatalf("t_1 must be holds-false/holds-true: %v", exfe[0])
	}
	// Party 2 on the closure date: t_2 excluded as single and from every
	// pair; adjacency outside the window stays free is covered by unit
	// planner tests; cross-restaurant unaffected below.
	m2 := s.Availability(map[string][]string{
		"restaurant_id": {"r_anker"}, "date": {"2027-06-17"}, "party_size": {"2"},
	})
	bm2, _ := m2.Body.(map[string]any)
	sl2, _ := bm2["slots"].([]any)[0].(map[string]any)
	ids2, _ := json.Marshal(sl2["available_table_ids"])
	var idList []string
	_ = json.Unmarshal(ids2, &idList)
	for _, id := range idList {
		if id == "t_2" {
			t.Fatalf("closed t_2 listed available: %s", ids2)
		}
	}
	opts2, _ := json.Marshal(sl2["available_options"])
	var optList []map[string]any
	_ = json.Unmarshal(opts2, &optList)
	for _, o := range optList {
		for _, id := range o["table_ids"].([]any) {
			if id == "t_2" {
				t.Fatalf("closed t_2 in pair option: %s", opts2)
			}
		}
	}
	// Other restaurant unaffected.
	mo := s.Availability(map[string][]string{
		"restaurant_id": {"r_other"}, "date": {"2027-06-17"}, "party_size": {"1"},
	})
	bmo, _ := mo.Body.(map[string]any)
	slo, _ := bmo["slots"].([]any)[0].(map[string]any)
	io, _ := json.Marshal(slo["available_table_ids"])
	if string(io) != `["t_1"]` {
		t.Fatalf("other restaurant: %s", io)
	}
	// No-explain shape unchanged.
	if _, ok := sl2["explain"]; ok {
		t.Fatal("explain leaked without flag")
	}
}

func TestReplanApplyClosureWrites(t *testing.T) {
	s, mgr, diner := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "cw-pv"))
	if res := applyReplan(t, s, mgr, "r_anker", pid, "cw-ap"); res.Status != 201 {
		t.Fatalf("apply: %d", res.Status)
	}
	// Adoption generating into the closure window: anchor on the closed
	// table the week before, so its occurrence lands inside the closure.
	// The anchor is legitimate setup, created before the pre snapshot.
	an := s.CreateReservation(diner, "cw-an",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-10T19:00","party_size":1}`))
	if an.Status != 201 {
		t.Fatalf("anchor create: %d %v", an.Status, an.Body)
	}
	anm, _ := an.Body.(map[string]any)
	aref, _ := anm["reference"].(string)
	snap := func() string {
		raw, _ := json.Marshal(s.Export().Body)
		return string(raw)
	}
	pre := snap()
	// Create on the closed table overlapping the closure.
	cr := s.CreateReservation(diner, "cw-cr",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":1}`))
	if st, code := resultCode(cr); st != 409 || code != "table_unavailable" {
		t.Fatalf("closed create: %d %s", st, code)
	}
	if post := snap(); post != pre {
		t.Fatal("rejected create changed state")
	}
	// Real PATCH onto the closed table.
	if st, code := resultCode(s.PatchReservation(diner, "APBBBB", []byte(`{"table_id":"t_2"}`))); st != 409 || code != "table_unavailable" {
		t.Fatalf("closed patch: %d %s", st, code)
	}
	if post := snap(); post != pre {
		t.Fatal("rejected patch changed state")
	}
	// Batch move onto the closed table.
	mr := s.MoveReservations(diner, "cw-mv", []byte(`{"moves":[{"reference":"APBBBB","table_id":"t_2"}]}`))
	if st, code := resultCode(mr); st != 409 || code != "table_unavailable" {
		t.Fatalf("closed move: %d %s", st, code)
	}
	if post := snap(); post != pre {
		t.Fatal("rejected move changed state")
	}
	// Pair selector including the closed member rejects too.
	pr := s.CreateReservation(diner, "cw-pr",
		[]byte(`{"restaurant_id":"r_anker","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T21:30","party_size":2}`))
	if st, code := resultCode(pr); st != 409 || code != "table_unavailable" {
		t.Fatalf("closed pair create: %d %s", st, code)
	}
	if post := snap(); post != pre {
		t.Fatal("rejected pair create changed state")
	}
	ar := s.AdoptSeries(diner, "cw-ad", []byte(`{"anchor_reference":"`+aref+`","count":2,"interval_weeks":1}`))
	if st, code := resultCode(ar); st != 409 || code != "table_unavailable" {
		t.Fatalf("closure adoption: %d %s", st, code)
	}
	if post := snap(); post != pre {
		t.Fatal("rejected adoption changed state")
	}
	// Failed keys genuinely reusable with valid bodies.
	cr2 := s.CreateReservation(diner, "cw-cr",
		[]byte(`{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T21:30","party_size":1}`))
	if cr2.Status != 201 {
		t.Fatalf("failed-key reuse: %d %v", cr2.Status, cr2.Body)
	}
	mr2 := s.MoveReservations(diner, "cw-mv", []byte(`{"moves":[{"reference":"APBBBB","party_size":1}]}`))
	if mr2.Status != 201 {
		t.Fatalf("failed move-key reuse: %d %v", mr2.Status, mr2.Body)
	}
	ar2 := s.AdoptSeries(diner, "cw-ad", []byte(`{"anchor_reference":"APBBBB","count":2,"interval_weeks":1}`))
	if ar2.Status != 201 {
		t.Fatalf("failed adopt-key reuse: %d %v", ar2.Status, ar2.Body)
	}
}

// checkApplyDelta asserts the complete exact state delta of one successful
// application against the winner response: full counter map with only the
// target +1; winner entries equal stored public projections (scalar table_id
// iff singleton); exactly one record changed (selectors + revision +1,
// rest byte-equal); exactly one appended reassigned history with preserved
// prefix, seq, revision, plan id, full From/To and frozen terms; exactly
// one appended closure; the applied plan flipped only Applied; exactly one
// new receipt with owner/method/path/key/canonical body/status/raw bytes;
// all prior plans/receipts and unrelated namespaces byte-equal. It reads
// state only and never mutates.
func checkApplyDelta(t *testing.T, preRaw, postRaw []byte, winner map[string]any, planID, key, owner, path, restaurant string) {
	t.Helper()
	var preEnv, postEnv map[string]any
	if err := json.Unmarshal(preRaw, &preEnv); err != nil {
		t.Fatalf("pre export parses: %v", err)
	}
	if err := json.Unmarshal(postRaw, &postEnv); err != nil {
		t.Fatalf("post export parses: %v", err)
	}
	ps := preEnv["state"].(map[string]any)
	qs := postEnv["state"].(map[string]any)
	// Full counter map: only the target restaurant +1.
	preC := ps["restaurant_revisions"].(map[string]any)
	postC := qs["restaurant_revisions"].(map[string]any)
	expC := map[string]any{}
	for k, v := range preC {
		expC[k] = v
	}
	expC[restaurant] = preC[restaurant].(float64) + 1
	xc, _ := json.Marshal(expC)
	yc, _ := json.Marshal(postC)
	if string(xc) != string(yc) {
		t.Fatalf("counter map: %s vs %s", xc, yc)
	}
	if fmt.Sprintf("%v", winner["restaurant_revision"]) != fmt.Sprintf("%v", preC[restaurant].(float64)+1) {
		t.Fatalf("winner revision %v", winner["restaurant_revision"])
	}
	// Exactly one changed record; rest byte-equal.
	preR := ps["reservations"].(map[string]any)
	postR := qs["reservations"].(map[string]any)
	if len(preR) != len(postR) {
		t.Fatalf("record count: %d -> %d", len(preR), len(postR))
	}
	changed := []string{}
	for ref, bv := range preR {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postR[ref])
		if string(xv) != string(yv) {
			changed = append(changed, ref)
		}
	}
	if len(changed) != 1 {
		t.Fatalf("changed records: %v", changed)
	}
	mref := changed[0]
	var bm, am map[string]any
	_ = json.Unmarshal(mustJSON(t, preR[mref]), &bm)
	_ = json.Unmarshal(mustJSON(t, postR[mref]), &am)
	for k, v := range bm {
		if k == "table_id" || k == "table_ids" || k == "revision" {
			continue
		}
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(am[k])
		if string(xv) != string(yv) {
			t.Fatalf("moved field %q changed", k)
		}
	}
	if am["revision"] != bm["revision"].(float64)+1 {
		t.Fatalf("moved revision: %v -> %v", bm["revision"], am["revision"])
	}
	// Winner entries equal stored public projections.
	wrecs, _ := winner["reservations"].([]any)
	if len(wrecs) == 0 {
		t.Fatal("winner has no reservations")
	}
	for _, r := range wrecs {
		rm, _ := r.(map[string]any)
		ref, _ := rm["reference"].(string)
		stored, _ := postR[ref].(map[string]any)
		if stored == nil {
			t.Fatalf("winner ref %s missing in state", ref)
		}
		want := map[string]any{}
		for k, v := range stored {
			want[k] = v
		}
		delete(want, "user_id")
		if tids, _ := stored["table_ids"].([]any); len(tids) != 1 {
			delete(want, "table_id")
		}
		rj, _ := json.Marshal(rm)
		wj, _ := json.Marshal(want)
		if string(rj) != string(wj) {
			t.Fatalf("winner != stored public for %s:\n%s\n%s", ref, rj, wj)
		}
	}
	// Histories: exactly one appended reassigned entry on the moved
	// record with preserved prefix; all other keys byte-equal.
	preH := ps["histories"].(map[string]any)
	postH := qs["histories"].(map[string]any)
	if len(preH) != len(postH) {
		t.Fatalf("history keys: %d -> %d", len(preH), len(postH))
	}
	for ref, bv := range preH {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postH[ref])
		var bl, al []any
		_ = json.Unmarshal(xv, &bl)
		_ = json.Unmarshal(yv, &al)
		if ref == mref {
			if len(al) != len(bl)+1 {
				t.Fatalf("moved history length: %d -> %d", len(bl), len(al))
			}
			px, _ := json.Marshal(bl)
			qx, _ := json.Marshal(al[:len(bl)])
			if string(px) != string(qx) {
				t.Fatal("moved history prefix changed")
			}
			ne, _ := al[len(al)-1].(map[string]any)
			if ne["event"] != "reassigned" || ne["plan_id"] != planID {
				t.Fatalf("appended entry: %v", ne)
			}
			if fmt.Sprintf("%v", ne["revision"]) != fmt.Sprintf("%v", am["revision"]) {
				t.Fatalf("appended revision %v vs record %v", ne["revision"], am["revision"])
			}
			nc, _ := ne["changes"].([]any)
			if len(nc) != 1 {
				t.Fatalf("appended changes: %v", ne)
			}
			nch, _ := nc[0].(map[string]any)
			if nch["field"] != "table_ids" {
				t.Fatalf("appended field: %v", nch)
			}
			bt, _ := json.Marshal(bm["table_ids"])
			at, _ := json.Marshal(am["table_ids"])
			nf, _ := json.Marshal(nch["from"])
			nt, _ := json.Marshal(nch["to"])
			if string(nf) != string(bt) || string(nt) != string(at) {
				t.Fatalf("appended From/To: %s -> %s (record %s -> %s)", nf, nt, bt, at)
			}
			et, _ := json.Marshal(ne["accepted_terms"])
			rt, _ := json.Marshal(am["accepted_terms"])
			if string(et) != string(rt) {
				t.Fatal("appended terms differ from record terms")
			}
		} else if string(xv) != string(yv) {
			t.Fatalf("history %s changed", ref)
		}
	}
	// Exactly one appended closure; saved plan flipped only Applied.
	prePlans := ps["plans"].(map[string]any)
	postPlans := qs["plans"].(map[string]any)
	preCl := ps["closures"].(map[string]any)
	postCl := qs["closures"].(map[string]any)
	for k, bv := range preCl {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postCl[k])
		if string(xv) != string(yv) {
			t.Fatalf("prior closures %q changed", k)
		}
	}
	var bcl, acl []any
	_ = json.Unmarshal(mustJSON(t, preCl[restaurant]), &bcl)
	_ = json.Unmarshal(mustJSON(t, postCl[restaurant]), &acl)
	if len(acl) != len(bcl)+1 {
		t.Fatalf("closures: %d -> %d", len(bcl), len(acl))
	}
	if len(bcl) > 0 {
		bcj, _ := json.Marshal(bcl)
		acj, _ := json.Marshal(acl[:len(bcl)])
		if string(bcj) != string(acj) {
			t.Fatalf("prior closures changed for %q", restaurant)
		}
	}
	newCl, _ := acl[len(acl)-1].(map[string]any)
	storedCl, _ := postPlans[planID].(map[string]any)["closure"].(map[string]any)
	nj, _ := json.Marshal(newCl)
	sj, _ := json.Marshal(storedCl)
	if string(nj) != string(sj) {
		t.Fatalf("appended closure != plan closure: %s vs %s", nj, sj)
	}
	if len(prePlans) != len(postPlans) {
		t.Fatalf("plan count: %d -> %d", len(prePlans), len(postPlans))
	}
	for k, bv := range prePlans {
		xv, _ := json.Marshal(bv)
		yv, _ := json.Marshal(postPlans[k])
		if k == planID {
			var bpl, apl map[string]any
			_ = json.Unmarshal(xv, &bpl)
			_ = json.Unmarshal(yv, &apl)
			for fk, fv := range bpl {
				if fk == "applied" {
					continue
				}
				fxj, _ := json.Marshal(fv)
				fyj, _ := json.Marshal(apl[fk])
				if string(fxj) != string(fyj) {
					t.Fatalf("plan field %q changed", fk)
				}
			}
			if bpl["applied"] != false || apl["applied"] != true {
				t.Fatalf("applied flag: %v -> %v", bpl["applied"], apl["applied"])
			}
		} else if string(xv) != string(yv) {
			t.Fatalf("prior plan %q changed", k)
		}
	}
	// Exactly one new receipt with exact binding; priors unchanged.
	preRc := ps["receipts"].(map[string]any)
	postRc := qs["receipts"].(map[string]any)
	if len(postRc) != len(preRc)+1 {
		t.Fatalf("receipt count: %d -> %d", len(preRc), len(postRc))
	}
	for k, v := range preRc {
		xv, _ := json.Marshal(v)
		yv, _ := json.Marshal(postRc[k])
		if string(xv) != string(yv) {
			t.Fatalf("prior receipt %q changed", k)
		}
	}
	var newRec map[string]any
	for k, v := range postRc {
		if _, ok := preRc[k]; !ok {
			rm, _ := v.(map[string]any)
			newRec = rm
		}
	}
	if newRec == nil {
		t.Fatal("no new receipt")
	}
	if newRec["user_id"] != owner || newRec["method"] != "POST" ||
		newRec["path"] != path || newRec["key"] != key || newRec["status"] != float64(201) {
		t.Fatalf("receipt binding: %v", newRec)
	}
	canon, err := CanonicalBody(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if newRec["body"] != canon {
		t.Fatalf("receipt body: %v", newRec["body"])
	}
	wj, _ := json.Marshal(winner)
	if newRec["response"] != string(wj) {
		t.Fatal("receipt response != winner bytes")
	}
	// Unrelated namespaces byte-equal.
	for k, bv := range ps {
		switch k {
		case "plans", "receipts", "reservations", "histories", "closures", "restaurant_revisions":
		default:
			xv, _ := json.Marshal(bv)
			yv, _ := json.Marshal(qs[k])
			if string(xv) != string(yv) {
				t.Fatalf("namespace %q changed", k)
			}
		}
	}
}

func TestReplanApplyRace50(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "race-pv"))
	// Seed an unrelated prior plan and receipt (other restaurant) so the
	// delta's prior-plan and prior-receipt preservation checks below are
	// not vacuous.
	other := s.PreviewReplan(mgr, "r_other", "race-unrelated",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`))
	if other.Status != 201 {
		t.Fatalf("unrelated preview: %d %v", other.Status, other.Body)
	}
	preRaw, _ := json.Marshal(s.Export().Body)
	const n = 50
	codes := make([]int, n)
	outs := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := s.ApplyReplan(mgr, "r_anker", pid, "race50", []byte(`{}`))
			codes[i] = res.Status
			raw, _ := json.Marshal(res.Body)
			outs[i] = string(raw)
		}(i)
	}
	wg.Wait()
	ones, zeros := 0, 0
	for _, c := range codes {
		switch c {
		case 201:
			ones++
		case 200:
			zeros++
		}
	}
	if ones != 1 || zeros != n-1 {
		t.Fatalf("race: 201=%d 200=%d", ones, zeros)
	}
	for i := 1; i < n; i++ {
		if outs[i] != outs[0] {
			t.Fatal("race bytes differ")
		}
	}
	var winner map[string]any
	if err := json.Unmarshal([]byte(outs[0]), &winner); err != nil {
		t.Fatalf("winner parses: %v", err)
	}
	postRaw, _ := json.Marshal(s.Export().Body)
	checkApplyDelta(t, preRaw, postRaw, winner, pid, "race50", "u_ada",
		"/restaurants/r_anker/replans/"+pid+"/apply", "r_anker")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestReplanApplyCompeting(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "ck-pv"))
	// Unrelated prior plan+receipt so preservation checks are not vacuous.
	other := s.PreviewReplan(mgr, "r_other", "ck-unrelated",
		[]byte(`{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}`))
	if other.Status != 201 {
		t.Fatalf("unrelated preview: %d %v", other.Status, other.Body)
	}
	preRaw, _ := json.Marshal(s.Export().Body)
	// Two actual simultaneous different keys on the same plan: exactly
	// one 201 and one 409 plan_already_applied.
	var wg sync.WaitGroup
	type kres struct {
		res Result
		key string
	}
	krs := make([]kres, 2)
	keys := []string{"ck-1", "ck-2"}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			krs[i] = kres{applyReplan(t, s, mgr, "r_anker", pid, keys[i]), keys[i]}
		}(i)
	}
	wg.Wait()
	var winner *kres
	for i := range krs {
		if krs[i].res.Status == 201 {
			if winner != nil {
				t.Fatal("two concurrent winners")
			}
			w := krs[i]
			winner = &w
		} else if st, code := resultCode(krs[i].res); st != 409 || code != "plan_already_applied" {
			t.Fatalf("loser: %d %s", st, code)
		}
	}
	if winner == nil {
		t.Fatal("no concurrent winner")
	}
	var wbody map[string]any
	wj, _ := json.Marshal(winner.res.Body)
	_ = json.Unmarshal(wj, &wbody)
	postRaw, _ := json.Marshal(s.Export().Body)
	checkApplyDelta(t, preRaw, postRaw, wbody, pid, winner.key, "u_ada",
		"/restaurants/r_anker/replans/"+pid+"/apply", "r_anker")
	// Competing plans on the same captured revision: one 201, one stale.
	// The loser code is pinned and the losing plan stays unapplied with no
	// receipt; the winning delta is once-only.
	s2, mgr2, _ := newApplyService(t)
	p1 := planIDOf(t, previewOK(t, s2, mgr2, "ck-a"))
	p2 := previewOK(t, s2, mgr2, "ck-b")
	p2id := planIDOf(t, p2)
	preRaw2, _ := json.Marshal(s2.Export().Body)
	var w1, w2 sync.WaitGroup
	type outcome struct {
		res Result
		key string
		pid string
	}
	o1, o2 := outcome{}, outcome{}
	w1.Add(1)
	go func() {
		defer w1.Done()
		o1 = outcome{applyReplan(t, s2, mgr2, "r_anker", p1, "ck-c1"), "ck-c1", p1}
	}()
	w2.Add(1)
	go func() {
		defer w2.Done()
		o2 = outcome{applyReplan(t, s2, mgr2, "r_anker", p2id, "ck-c2"), "ck-c2", p2id}
	}()
	w1.Wait()
	w2.Wait()
	var win, lose outcome
	switch {
	case o1.res.Status == 201 && o2.res.Status == 409:
		win, lose = o1, o2
	case o2.res.Status == 201 && o1.res.Status == 409:
		win, lose = o2, o1
	default:
		t.Fatalf("competing plans: %d/%d", o1.res.Status, o2.res.Status)
	}
	if st, code := resultCode(lose.res); st != 409 || code != "stale_plan" {
		t.Fatalf("loser: %d %s", st, code)
	}
	var wbody2 map[string]any
	wj2, _ := json.Marshal(win.res.Body)
	_ = json.Unmarshal(wj2, &wbody2)
	postRaw2, _ := json.Marshal(s2.Export().Body)
	checkApplyDelta(t, preRaw2, postRaw2, wbody2, win.pid, win.key, "u_ada",
		"/restaurants/r_anker/replans/"+win.pid+"/apply", "r_anker")
	var postEnv map[string]any
	_ = json.Unmarshal(postRaw2, &postEnv)
	st := postEnv["state"].(map[string]any)
	if st["plans"].(map[string]any)[lose.pid].(map[string]any)["applied"] != false {
		t.Fatalf("losing plan applied: %s", lose.pid)
	}
	for _, v := range st["receipts"].(map[string]any) {
		rm, _ := v.(map[string]any)
		if rm["key"] == lose.key {
			t.Fatalf("losing key has receipt: %s", lose.key)
		}
	}
}

func TestReplanApplyDetachment(t *testing.T) {
	s, mgr, _ := newApplyService(t)
	pid := planIDOf(t, previewOK(t, s, mgr, "dt-pv"))
	res := applyReplan(t, s, mgr, "r_anker", pid, "dt-ap")
	if res.Status != 201 {
		t.Fatalf("apply: %d", res.Status)
	}
	pristine, _ := json.Marshal(res.Body)
	storedBefore, _ := json.Marshal(s.Export().Body)
	m, _ := res.Body.(map[string]any)
	recs, _ := m["reservations"].([]any)
	first, _ := recs[0].(map[string]any)
	if tids, ok := first["table_ids"].([]string); ok && len(tids) > 0 {
		tids[0] = "MUT"
	}
	terms, _ := first["accepted_terms"].(map[string]any)
	caps, _ := terms["capacities"].(map[string]any)
	caps["t_1"] = "MUT"
	hours, _ := terms["opening_hours"].([]any)
	if len(hours) > 0 {
		hours[0].(map[string]any)["opens"] = "MUT"
	}
	// Guard: the nested mutations really landed in the caller copy.
	if first["table_ids"].([]string)[0] != "MUT" || caps["t_1"] != "MUT" {
		t.Fatal("test mutation did not apply")
	}
	storedAfter, _ := json.Marshal(s.Export().Body)
	if string(storedBefore) != string(storedAfter) {
		t.Fatal("caller mutation leaked into state")
	}
	again := applyReplan(t, s, mgr, "r_anker", pid, "dt-ap")
	if again.Status != 200 {
		t.Fatalf("replay: %d", again.Status)
	}
	ag, _ := json.Marshal(again.Body)
	if string(ag) != string(pristine) {
		t.Fatal("replay bytes changed")
	}
}
