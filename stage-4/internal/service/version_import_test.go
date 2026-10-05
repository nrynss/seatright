package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

// Requirement map (S3-I1 modern import validation + genuine donor imports;
// collective/exception portability deferred to I2, browser already G2A):
// R286/R288 genuine prior-process donors -> TestVersionImportGenuineDonors
// R287 imported anchor adoption -> TestVersionImportAdoptedAnchor
// R237/R244/R245 modern roundtrip + receipts -> TestVersionImportModernRoundtrip
// R257 histories + R242/R243 terms -> TestVersionImportHistories
// invalid-atomic matrices + sabotage -> TestVersionImportInvalidAtomic
// unknown managers -> TestVersionImportUnknownManagers
// legacy honest synthetic -> TestVersionImportLegacySynthetic
// replay-after-mutation byte identity -> TestVersionImportReplayAfterMutation
//
// Modern snapshots are built through the real reset/publish/create/patch/
// cancel/adopt APIs. Corruptions mutate a copied valid export one property
// at a time and require 422 with the destination byte-identical.

// versionImportModern builds a rich modern snapshot: policies (incl.
// supersession), singleton + pair creates, amendment, cancellation,
// cancelled seed, beyond-maxima fixture values on a second restaurant, and
// a series adoption. Returns the service, owner token and anchor reference.
func versionImportModern(t *testing.T) (*Service, string, string) {
	t.Helper()
	s := New()
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
		"reservations":[
			{"id":"s0","reference":"SEEDCN","user_id":"u1","restaurant_id":"r1",
			"table_id":"t_2","starts_at_local":"2027-05-06T20:30","party_size":1,"status":"cancelled"}]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := versionLoginToken(t, s)
	// Policies: v1 then same-date superseding v2.
	pub := func(from, key string, dur int) {
		t.Helper()
		body := `{"effective_from":"` + from + `","slot_minutes":30,"reservation_duration_minutes":` + itoa(dur) + `,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}`
		if res := s.PublishPolicy(tok, "r1", key, []byte(body)); res.Status != 201 {
			t.Fatalf("publish %s: %d %v", key, res.Status, res.Body)
		}
	}
	pub("2027-05-13", "pol1", 120)
	pub("2027-05-13", "pol2", 60)
	// Singleton create + amendment + pair create + cancel one.
	single := `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T19:00","party_size":1}`
	if res := s.CreateReservation(tok, "c-single", []byte(single)); res.Status != 201 {
		t.Fatalf("single: %d %v", res.Status, res.Body)
	}
	singleRef := refOf(t, s, "c-single", single, tok)
	if rec := serveRequest(s, http.MethodPatch, "/reservations/"+singleRef, []byte(`{"party_size":2}`),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("amend: %d %s", rec.Code, rec.Body.String())
	}
	pair := `{"restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T20:30","party_size":5}`
	pairRef := ""
	if res := s.CreateReservation(tok, "c-pair", []byte(pair)); res.Status != 201 {
		t.Fatalf("pair: %d %v", res.Status, res.Body)
	} else {
		pairRef = res.Body.(map[string]any)["reference"].(string)
	}
	_ = pairRef
	// Really cancel the amended singleton via API (revision+1, cancelled entry).
	if rec := serveRequest(s, http.MethodPost, "/reservations/"+singleRef+"/cancel", []byte("{}"),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	// Above-maxima booking on rbig (fixture cutoff 10081, capacity 101):
	// proves above-maxima booking/history proof on a real record. (1441
	// grid/duration fixtures remain reset/import-valid but unbookable by
	// grid arithmetic, covered by version_state tests.)
	bigBody := `{"restaurant_id":"rbig","table_id":"b1","starts_at_local":"2027-05-06T19:00","party_size":101}`
	if res := s.CreateReservation(tok, "c-big", []byte(bigBody)); res.Status != 201 {
		t.Fatalf("big: %d %v", res.Status, res.Body)
	}
	// Series anchor for adoption below.
	anchorBody := `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T18:00","party_size":1}`
	anchorRef := ""
	if res := s.CreateReservation(tok, "c-anchor", []byte(anchorBody)); res.Status != 201 {
		t.Fatalf("anchor: %d %v", res.Status, res.Body)
	} else {
		anchorRef = res.Body.(map[string]any)["reference"].(string)
	}
	adopt := `{"anchor_reference":` + quoted(anchorRef) + `,"count":2,"interval_weeks":1}`
	if res := s.AdoptSeries(tok, "sAdopt1", []byte(adopt)); res.Status != 201 {
		t.Fatalf("adopt: %d %v", res.Status, res.Body)
	}
	return s, tok, anchorRef
}

// versionImportShapes builds a dedicated fixture for the history-shape matrix:
//   - "shape-time" singleton with created + changed(time) history
//   - "shape-tab" singleton with created + changed(table scalar) history
//   - "shape-pair" pair with created + changed(table pair->singleton) history
//
// All on 2027-05-06 (fixture policy0, duration 90).
func versionImportShapes(t *testing.T) *Service {
	t.Helper()
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[
			{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
			"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
			"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
			"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
			"combinable":[["t_1","t_2"]],
			"manager_user_ids":["u1"]}],
		"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d %q", rec.Code, rec.Body.String())
	}
	tok := versionLoginToken(t, s)
	create := func(key, body string) string {
		t.Helper()
		res := s.CreateReservation(tok, key, []byte(body))
		if res.Status != 201 {
			t.Fatalf("create %s: %d %v", key, res.Status, res.Body)
		}
		return res.Body.(map[string]any)["reference"].(string)
	}
	patch := func(ref, body string) {
		t.Helper()
		if rec := serveRequest(s, http.MethodPatch, "/reservations/"+ref, []byte(body),
			map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
			t.Fatalf("patch %s: %d %s", ref, rec.Code, rec.Body.String())
		}
	}
	// Grid thu 18:00..23:00 dur90: slots ..21:30. Non-overlapping plan:
	// shape-time t_1 18:00 -> 19:30 (was 18:00, created 18:00 then time->19:30).
	rt := create("sh-time", `{"restaurant_id":"r1","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	patch(rt, `{"starts_at_local":"2027-05-06T19:30"}`)
	// shape-tab t_2 18:00 (t_1 busy 18:00-19:30? t_1 18:00 moved to 19:30, so t_1 free at 18:00;
	// use t_2 18:00 -> table t_1? t_1 19:30 occupies 19:30-21:00; t_2 18:00 occupies 18:00-19:30.
	// PATCH table t_2->t_1 at 18:00: t_1 18:00 free (moved away). OK.)
	rs := create("sh-tab", `{"restaurant_id":"r1","table_id":"t_2","starts_at_local":"2027-05-06T18:00","party_size":1}`)
	patch(rs, `{"table_id":"t_1"}`)
	// shape-pair [t_1,t_2] 21:00 (t_1 busy till 21:00? rt 19:30+90=21:00 edge-adjacent OK;
	// t_2 free after 19:30) -> PATCH to singleton t_2? t_2 21:00 free. party 2 fits t_2 cap4.
	rp := create("sh-pair", `{"restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T21:00","party_size":2}`)
	patch(rp, `{"table_id":"t_2"}`)
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

func quoted(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func refOf(t *testing.T, s *Service, key, body, tok string) string {
	t.Helper()
	rep := s.CreateReservation(tok, key, []byte(body))
	if rep.Status != 200 {
		t.Fatalf("replay for ref: %d", rep.Status)
	}
	m, _ := rep.Body.(map[string]any)
	return m["reference"].(string)
}

func importRaw(t *testing.T, dst *Service, raw []byte) (int, string) {
	t.Helper()
	rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil)
	if rec.Code != 204 {
		return rec.Code, rec.Body.String()
	}
	return rec.Code, ""
}

func TestVersionImportModernRoundtrip(t *testing.T) {
	s, tok, _ := versionImportModern(t)
	_ = tok
	before := string(exportBytes(t, s))
	dst := New()
	if code, body := importRaw(t, dst, []byte(before)); code != 204 {
		t.Fatalf("import: %d %s", code, body)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern export/import not byte-identical")
	}
	// Repeat import stable.
	if code, _ := importRaw(t, dst, []byte(before)); code != 204 {
		t.Fatalf("reimport: %d", code)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("modern reimport not stable")
	}
}

func TestVersionImportHistories(t *testing.T) {
	s, _, _ := versionImportModern(t)
	before := string(exportBytes(t, s))
	dst := New()
	if code, body := importRaw(t, dst, []byte(before)); code != 204 {
		t.Fatalf("import: %d %s", code, body)
	}
	s.mu.Lock()
	wantHist := len(s.state.Histories)
	dst.mu.Lock()
	gotHist := len(dst.state.Histories)
	dst.mu.Unlock()
	s.mu.Unlock()
	if wantHist == 0 || gotHist != wantHist {
		t.Fatalf("histories %d vs %d", wantHist, gotHist)
	}
	// Cancelled seed keeps exactly one reconstructed CREATED entry.
	dst.mu.Lock()
	seedHist := dst.state.Histories["SEEDCN"]
	dst.mu.Unlock()
	if len(seedHist) != 1 || seedHist[0].Event != "created" {
		t.Fatalf("cancelled seed history = %+v", seedHist)
	}
	// Sabotage: drop a history -> 422 and destination unchanged.
	env := exportEnvelope(t, s)
	hists := env["state"].(map[string]any)["histories"].(map[string]any)
	var victim string
	for k := range hists {
		victim = k
		break
	}
	delete(hists, victim)
	raw, _ := json.Marshal(env)
	probe := New()
	preProbe := string(exportBytes(t, probe))
	if rec := serveRequest(probe, http.MethodPost, "/_test/import", raw, nil); rec.Code != 422 {
		t.Fatalf("dropped history import = %d", rec.Code)
	}
	if got := string(exportBytes(t, probe)); got != preProbe {
		t.Fatal("rejected import mutated destination")
	}
}

func TestVersionImportInvalidAtomic(t *testing.T) {
	s, _, _ := versionImportModern(t)
	sh := versionImportShapes(t)
	before := string(exportBytes(t, s))
	cases := []struct {
		name   string
		shapes bool
		mutate func(env map[string]any) bool
	}{
		{"duplicate policy version", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			pols := st["policies"].(map[string]any)["r1"].([]any)
			dup, _ := json.Marshal(pols[0])
			var m map[string]any
			json.Unmarshal(dup, &m)
			st["policies"].(map[string]any)["r1"] = []any{pols[0], m}
			return true
		}},
		{"policy unknown restaurant", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			st["policies"].(map[string]any)["nope"] = []any{}
			return true
		}},
		{"published zero version", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			p := st["policies"].(map[string]any)["r1"].([]any)[0].(map[string]any)
			p["policy_version"] = float64(0)
			return true
		}},
		{"terms bind to no policy", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, r := range st["reservations"].(map[string]any) {
				rm := r.(map[string]any)
				if rm["revision"] != float64(0) {
					rm["accepted_terms"].(map[string]any)["policy_version"] = float64(99)
					break
				}
			}
			return true
		}},
		{"history orphan", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			st["histories"].(map[string]any)["NOPE01"] = []any{}
			return true
		}},
		{"history seq gap", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				if arr := h.([]any); len(arr) > 0 {
					arr[0].(map[string]any)["seq"] = float64(9)
					break
				}
			}
			return true
		}},
		{"series bad interval", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, sz := range st["series"].(map[string]any) {
				sz.(map[string]any)["interval_weeks"] = float64(9)
				break
			}
			return true
		}},
		{"series member unknown ref", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, sz := range st["series"].(map[string]any) {
				m := sz.(map[string]any)["members"].([]any)[0].(map[string]any)
				m["reference"] = "NOPE01"
				break
			}
			return true
		}},
		{"confirmed record after cancelled final history", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for ref, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				if len(arr) == 0 {
					continue
				}
				last := arr[len(arr)-1].(map[string]any)
				if last["event"] == "cancelled" {
					st["reservations"].(map[string]any)[ref].(map[string]any)["status"] = "confirmed"
					return true
				}
			}
			return true
		}},
		{"created party above own terms capacity", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				if len(arr) == 0 {
					continue
				}
				first := arr[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				chs[2].(map[string]any)["to"] = float64(99)
				return true
			}
			return true
		}},
		{"changed from diverges from running state", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				for _, e := range arr {
					em := e.(map[string]any)
					if em["event"] != "changed" {
						continue
					}
					chs := em["changes"].([]any)
					for _, c := range chs {
						cm := c.(map[string]any)
						if cm["field"] == "party_size" {
							cm["from"] = float64(99)
							return true
						}
					}
				}
			}
			return true
		}},
		{"undeclared pair in created history", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				if chs[0].(map[string]any)["field"] == "table_id" {
					chs[0].(map[string]any)["field"] = "table_ids"
					chs[0].(map[string]any)["from"] = nil
					chs[0].(map[string]any)["to"] = []any{"t_1", "b1"}
					return true
				}
			}
			return true
		}},
		{"reversed pair order in created history", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				if chs[0].(map[string]any)["field"] == "table_ids" {
					chs[0].(map[string]any)["to"] = []any{"t_2", "t_1"}
					return true
				}
			}
			return true
		}},
		{"unknown table in created history", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				if chs[0].(map[string]any)["field"] == "table_id" {
					chs[0].(map[string]any)["to"] = "nope"
					return true
				}
			}
			return true
		}},
		{"malformed local time in created history", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				chs[1].(map[string]any)["to"] = "2027-13-99T99:99"
				return true
			}
			return true
		}},
		{"cancelled record without cancelled history", true, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for ref, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				if len(arr) == 2 && arr[0].(map[string]any)["event"] == "created" &&
					arr[1].(map[string]any)["event"] == "changed" {
					st["reservations"].(map[string]any)[ref].(map[string]any)["status"] = "cancelled"
					return true
				}
			}
			return false
		}},
		{"history terms carry effective_from", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				first["accepted_terms"].(map[string]any)["effective_from"] = "2027-05-13"
				return true
			}
			return true
		}},
		{"unexceptioned member date diverges from schedule", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			for _, sz := range st["series"].(map[string]any) {
				szm := sz.(map[string]any)
				mems := szm["members"].([]any)
				if len(mems) < 2 {
					continue
				}
				// Shift every scheduled date by one day together: the weekly
				// calendar sequence stays valid, so only the record mapping
				// check (false member date == scheduled date) can reject.
				for _, m := range mems {
					mm := m.(map[string]any)
					if mm["exception"] == true {
						return false
					}
					d := mm["scheduled_date"].(string)
					day := int(d[8]-'0')*10 + int(d[9]-'0') + 1
					mm["scheduled_date"] = d[:8] + string(rune('0'+day/10)) + string(rune('0'+day%10))
				}
				return true
			}
			return false
		}},
		{"created singleton with table_ids length1", true, func(env map[string]any) bool {
			// Host shape case A: singleton CREATED must use scalar table_id.
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				first := h.([]any)[0].(map[string]any)
				if first["event"] != "created" {
					continue
				}
				chs := first["changes"].([]any)
				if chs[0].(map[string]any)["field"] == "table_id" {
					to := chs[0].(map[string]any)["to"]
					chs[0].(map[string]any)["field"] = "table_ids"
					chs[0].(map[string]any)["from"] = nil
					chs[0].(map[string]any)["to"] = []any{to}
					return true
				}
			}
			return false
		}},
		{"created and changed time with slash separators", true, func(env map[string]any) bool {
			// Host shape case B: separators must be literal '-' (positions
			// 4/7), not merely digits elsewhere.
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				if len(arr) == 2 && arr[0].(map[string]any)["event"] == "created" &&
					arr[1].(map[string]any)["event"] == "changed" {
					c0 := arr[0].(map[string]any)["changes"].([]any)
					c1 := arr[1].(map[string]any)["changes"].([]any)
					var t0, f1 string
					for _, c := range c1 {
						if c.(map[string]any)["field"] == "starts_at_local" {
							t0, _ = c0[1].(map[string]any)["to"].(string)
							f1, _ = c.(map[string]any)["from"].(string)
							if t0 != f1 {
								return false
							}
							bad := "2027/05/06T19:00"
							c0[1].(map[string]any)["to"] = bad
							c.(map[string]any)["from"] = bad
							return true
						}
					}
				}
			}
			return false
		}},
		{"singleton to singleton change with table_ids arrays", true, func(env map[string]any) bool {
			// Host shape case C: both sides singleton must use scalar.
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				for _, e := range arr {
					em := e.(map[string]any)
					if em["event"] != "changed" {
						continue
					}
					for _, c := range em["changes"].([]any) {
						cm := c.(map[string]any)
						if cm["field"] == "table_id" {
							from, _ := cm["from"].(string)
							to, _ := cm["to"].(string)
							cm["field"] = "table_ids"
							cm["from"] = []any{from}
							cm["to"] = []any{to}
							return true
						}
					}
				}
			}
			return false
		}},
		{"changed pair from reversed illegal order", true, func(env map[string]any) bool {
			// From arrays must be canonical legal sets, not merely equal.
			st := env["state"].(map[string]any)
			for _, h := range st["histories"].(map[string]any) {
				arr := h.([]any)
				for _, e := range arr {
					em := e.(map[string]any)
					if em["event"] != "changed" {
						continue
					}
					for _, c := range em["changes"].([]any) {
						cm := c.(map[string]any)
						if cm["field"] == "table_ids" {
							from, _ := cm["from"].([]any)
							if len(from) != 2 {
								continue
							}
							cm["from"] = []any{from[1], from[0]}
							return true
						}
					}
				}
			}
			return false
		}},
		{"negative counter", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			st["restaurant_revisions"].(map[string]any)["r1"] = float64(-1)
			return true
		}},
		{"counter unknown restaurant", false, func(env map[string]any) bool {
			st := env["state"].(map[string]any)
			st["restaurant_revisions"].(map[string]any)["nope"] = float64(3)
			return true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := s
			if tc.shapes {
				src = sh
			}
			env := exportEnvelope(t, src)
			if !tc.mutate(env) {
				t.Fatalf("mutation %q did not apply to fixture", tc.name)
			}
			raw, _ := json.Marshal(env)
			dst := New()
			pre := string(exportBytes(t, dst))
			if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 422 {
				t.Fatalf("import = %d, want 422", rec.Code)
			}
			if got := string(exportBytes(t, dst)); got != pre {
				t.Fatal("rejected import mutated destination")
			}
		})
	}
	_ = before
}

func TestVersionImportUnknownManagers(t *testing.T) {
	s := New()
	fix := `{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
		"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
		"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
		"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
		"tables":[{"id":"t_1","label":"1","capacity":2}],
		"manager_user_ids":["ghost-9"]}],"reservations":[]}`
	if rec := serveRequest(s, http.MethodPost, "/_test/reset", []byte(fix), nil); rec.Code != 204 {
		t.Fatalf("reset: %d", rec.Code)
	}
	before := string(exportBytes(t, s))
	dst := New()
	if code, body := importRaw(t, dst, []byte(before)); code != 204 {
		t.Fatalf("import: %d %s", code, body)
	}
	dst.mu.Lock()
	kept := dst.state.Restaurants[0].ManagerUserIDs
	dst.mu.Unlock()
	if !reflect.DeepEqual(kept, []string{"ghost-9"}) {
		t.Fatalf("managers = %#v", kept)
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("manager round trip changed state")
	}
}

// TestVersionImportLegacySynthetic is explicitly SYNTHETIC legacy-shaped unit
// input: it proves old-shape normalization only, never a real donor.
func TestVersionImportLegacySynthetic(t *testing.T) {
	legacy := map[string]any{
		"track": "tablekeeper", "format_version": float64(1),
		"state": map[string]any{
			"users": map[string]any{"u1": map[string]any{
				"id": "u1", "email": "a@b", "display_name": "A",
				"password_hash": mustTestHash(t, "password1"),
			}},
			"tokens": map[string]any{"tok-old": "u1"},
			"restaurants": []any{map[string]any{
				"id": "r1", "name": "N", "timezone": "Europe/Berlin",
				"slot_minutes": float64(30), "reservation_duration_minutes": float64(90),
				"cancellation_cutoff_minutes": float64(120),
				"opening_hours":               []any{map[string]any{"weekday": "thu", "opens": "18:00", "closes": "23:00"}},
				"tables":                      []any{map[string]any{"id": "t1", "label": "1", "capacity": float64(2)}},
			}},
			"reservations": map[string]any{"OLD01A": map[string]any{
				"reservation_id": "s1", "reference": "OLD01A", "user_id": "u1",
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
	raw, _ := json.Marshal(legacy)
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", raw, nil); rec.Code != 204 {
		t.Fatalf("legacy import: %d %q", rec.Code, rec.Body.String())
	}
	dst.mu.Lock()
	res := dst.state.Reservations["OLD01A"]
	hist := dst.state.Histories["OLD01A"]
	dst.mu.Unlock()
	if res.Revision != 1 || len(hist) != 1 || hist[0].Event != "created" {
		t.Fatalf("legacy normalized = %+v %+v", res, hist)
	}
}

func TestVersionImportReplayAfterMutation(t *testing.T) {
	s, tok, anchorRef := versionImportModern(t)
	// Capture the stored original receipt strings BEFORE further mutation.
	type rc struct {
		key, path, body, resp string
	}
	var receipts []rc
	s.mu.Lock()
	for _, r := range s.state.Receipts {
		receipts = append(receipts, rc{r.Key, r.Path, r.Body, r.Response})
	}
	s.mu.Unlock()
	if len(receipts) < 6 {
		t.Fatalf("want rich receipt set, got %d", len(receipts))
	}
	// Deterministic further mutation: amend the anchor party and cancel a
	// generated occurrence, so current fields provably differ afterwards.
	anchorGet := s.GetReservation(tok, anchorRef)
	if anchorGet.Status != 200 {
		t.Fatalf("anchor get: %d", anchorGet.Status)
	}
	anchorBefore, _ := json.Marshal(anchorGet.Body)
	if rec := serveRequest(s, http.MethodPatch, "/reservations/"+anchorRef, []byte(`{"party_size":2}`),
		map[string]string{"Authorization": "Bearer " + tok}); rec.Code != 200 {
		t.Fatalf("anchor amend: %d %s", rec.Code, rec.Body.String())
	}
	// Import round trip on the mutated state, then replay every stored
	// original receipt: each returns 200 with its exact stored bytes while
	// current fields stay independently asserted.
	before := string(exportBytes(t, s))
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", []byte(before), nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	cur := dst.GetReservation(tok, anchorRef)
	if cur.Status != 200 {
		t.Fatalf("current anchor: %d", cur.Status)
	}
	curB, _ := json.Marshal(cur.Body)
	if string(curB) == string(anchorBefore) {
		t.Fatal("current anchor identical to pre-mutation bytes")
	}
	for _, r := range receipts {
		var rep Result
		switch r.path {
		case "/reservations":
			rep = dst.CreateReservation(tok, r.key, []byte(r.body))
		case "/restaurants/r1/policies":
			rep = dst.PublishPolicy(tok, "r1", r.key, []byte(r.body))
		case "/series":
			rep = dst.AdoptSeries(tok, r.key, []byte(r.body))
		default:
			t.Fatalf("unknown receipt path %s", r.path)
		}
		if rep.Status != 200 {
			t.Fatalf("replay %s: %d", r.key, rep.Status)
		}
		rb, _ := json.Marshal(rep.Body)
		if string(rb) != r.resp {
			t.Fatalf("replay %s differs:\n%s\n%s", r.key, rb, r.resp)
		}
	}
	if got := string(exportBytes(t, dst)); got != before {
		t.Fatal("replays mutated the import")
	}
}

// TestVersionImportGenuineDonors imports PRIOR-PROCESS GENUINE donor artifacts
// (accepted stage1/stage2 images, not current fresh source): old tokens, hash
// logins, all scoped receipt replays, normalization, owner privacy and an
// eligible imported anchor adoption.
func TestVersionImportGenuineDonors(t *testing.T) {
	for _, stage := range []string{"stage1", "stage2"} {
		t.Run(stage, func(t *testing.T) {
			dir := os.Getenv("S3_DONORS_DIR")
			if dir == "" {
				t.Skip("SKIP artifact gate: S3_DONORS_DIR unset (pure normal tests need no private artifacts)")
			}
			expRaw, err := os.ReadFile(dir + "/" + stage + "/export.json")
			if err != nil {
				t.Fatalf("configured donor export missing: %s/%s/export.json", dir, stage)
			}
			manRaw, err := os.ReadFile(dir + "/" + stage + "/manifest.json")
			if err != nil {
				t.Fatalf("configured donor manifest missing: %s/%s/manifest.json", dir, stage)
			}
			var man map[string]any
			if err := json.Unmarshal(manRaw, &man); err != nil {
				t.Fatalf("configured donor manifest malformed: %s/%s (parse error)", dir, stage)
			}
			dst := New()
			if rec := serveRequest(dst, http.MethodPost, "/_test/import", expRaw, nil); rec.Code != 204 {
				t.Fatalf("donor import: %d", rec.Code)
			}
			// Repeat import stable.
			if rec := serveRequest(dst, http.MethodPost, "/_test/import", expRaw, nil); rec.Code != 204 {
				t.Fatalf("reimport: %d", rec.Code)
			}
			// Old tokens authenticate; hash logins work.
			users, _ := man["users"].([]any)
			if len(users) == 0 {
				t.Fatal("no donor users")
			}
			for _, u := range users {
				um := u.(map[string]any)
				for _, tok := range um["tokens"].([]any) {
					req := serveRequest(dst, http.MethodGet, "/reservations", nil,
						map[string]string{"Authorization": "Bearer " + tok.(string)})
					if req.Code != 200 {
						t.Fatalf("old token rejected: %d", req.Code)
					}
				}
				pw, _ := um["password"].(string)
				email, _ := um["email"].(string)
				if pw != "" {
					lb, _ := json.Marshal(map[string]any{"email": email, "password": pw})
					if rec := serveRequest(dst, http.MethodPost, "/auth/login", lb, nil); rec.Code != 200 {
						t.Fatalf("hash login failed: %d", rec.Code)
					}
				}
			}
			// All scoped receipt replays return EXACT original raw bytes:
			// the router writes json.Marshal(body) with no trailing newline,
			// so replay bytes must equal the preserved response_raw verbatim.
			// Body input uses the preserved body_raw (verbatim request bytes).
			nreplay := 0
			for _, r := range man["receipts"].([]any) {
				rm := r.(map[string]any)
				key, _ := rm["key"].(string)
				uid, _ := rm["user_id"].(string)
				method, _ := rm["method"].(string)
				path, _ := rm["path"].(string)
				bodyRaw, _ := rm["body_raw"].(string)
				wantResp, _ := rm["response_raw"].(string)
				if bodyRaw == "" || wantResp == "" {
					t.Fatalf("donor receipt %s missing preserved raw strings", key)
				}
				var tok string
				for _, u := range users {
					if u.(map[string]any)["id"] == uid {
						toks := u.(map[string]any)["tokens"].([]any)
						tok = toks[0].(string)
					}
				}
				hdr := map[string]string{"Authorization": "Bearer " + tok}
				var rec *httptest.ResponseRecorder
				switch {
				case method == "POST" && path == "/reservations":
					hdr["Idempotency-Key"] = key
					rec = serveRequest(dst, method, path, []byte(bodyRaw), hdr)
				case method == "POST" && path == "/reservation-moves":
					hdr["Idempotency-Key"] = key
					rec = serveRequest(dst, method, path, []byte(bodyRaw), hdr)
				default:
					t.Fatalf("unknown receipt path %s", path)
				}
				if rec.Code != 200 {
					t.Fatalf("receipt replay failed: %d", rec.Code)
				}
				if got := rec.Body.String(); got != wantResp {
					t.Fatalf("receipt bytes differ (got %d, want %d)", len(got), len(wantResp))
				}
				nreplay++
			}
			if nreplay == 0 {
				t.Fatal("no donor receipts replayed")
			}
			t.Logf("replayed %d receipts byte-exact", nreplay)
			// Owner privacy: another owner's reference is 404 to this caller.
			byRef := donorRefsByOwner(t, man)
			var adaTok, beaTok, adaRef, beaRef string
			for uid, refs := range byRef {
				toks := donorTokens(t, man, uid)
				if uid == "u_ada" {
					adaTok, adaRef = toks, refs[0]
				} else {
					beaTok, beaRef = toks, refs[0]
				}
			}
			if adaTok != "" && beaRef != "" {
				if req := serveRequest(dst, http.MethodGet, "/reservations/"+beaRef, nil,
					map[string]string{"Authorization": "Bearer " + adaTok}); req.Code != 404 {
					t.Fatalf("cross-owner lookup = %d, want 404", req.Code)
				}
			}
			if beaTok != "" && adaRef != "" {
				if req := serveRequest(dst, http.MethodGet, "/reservations/"+adaRef, nil,
					map[string]string{"Authorization": "Bearer " + beaTok}); req.Code != 404 {
					t.Fatalf("cross-owner lookup = %d, want 404", req.Code)
				}
			}
		})
	}
}

// TestVersionImportAdoptedAnchor adopts an eligible FUTURE confirmed imported
// donor anchor and requires 201: identity, original history, accepted terms and
// current series GET retained. The anchor is chosen from manifest records with a
// future local start (not past seeds), so the accepted cutoff precondition holds
// independently of first-list iteration.
func TestVersionImportAdoptedAnchor(t *testing.T) {
	dir := os.Getenv("S3_DONORS_DIR")
	if dir == "" {
		t.Skip("SKIP artifact gate: S3_DONORS_DIR unset (pure normal tests need no private artifacts)")
	}
	expRaw, err := os.ReadFile(dir + "/stage2/export.json")
	if err != nil {
		t.Fatalf("configured donor export missing: %s/stage2/export.json", dir)
	}
	manRaw, err := os.ReadFile(dir + "/stage2/manifest.json")
	if err != nil {
		t.Fatalf("configured donor manifest missing: %s/stage2/manifest.json", dir)
	}
	var man map[string]any
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatalf("configured donor manifest malformed: %s/stage2 (parse error)", dir)
	}
	dst := New()
	if rec := serveRequest(dst, http.MethodPost, "/_test/import", expRaw, nil); rec.Code != 204 {
		t.Fatalf("import: %d", rec.Code)
	}
	// Eligible anchor: manifest future confirmed record of u_ada.
	// Stage2 donor fixture date is 2026-11-12; seeds are 2020-01 dates.
	byRef, _ := man["records"].(map[string]any)["by_reference"].(map[string]any)
	// Receipt responses name the owner per reference (records carry no user).
	ownerOf := map[string]string{}
	for _, r := range man["receipts"].([]any) {
		rm := r.(map[string]any)
		resp, _ := rm["response"].(map[string]any)
		if resp == nil {
			continue
		}
		if ref, _ := resp["reference"].(string); ref != "" {
			ownerOf[ref], _ = rm["user_id"].(string)
		}
	}
	var anchor, owner string
	for ref, r := range byRef {
		rm := r.(map[string]any)
		status, _ := rm["status"].(string)
		loc, _ := rm["starts_at_local"].(string)
		if ownerOf[ref] == "u_ada" && status == "confirmed" && loc >= "2026-11-12" {
			anchor, owner = ref, "u_ada"
			break
		}
	}
	if anchor == "" {
		t.Fatal("no eligible future confirmed ada anchor in donor manifest")
	}
	tok := donorTokens(t, man, owner)
	// Independent preconditions: current GET confirmed, history present.
	cur := dst.GetReservation(tok, anchor)
	if cur.Status != 200 {
		t.Fatalf("anchor lookup: %d", cur.Status)
	}
	curBody, _ := cur.Body.(map[string]any)
	if curBody["status"] != "confirmed" {
		t.Fatalf("anchor status = %v", curBody["status"])
	}
	anchorTerms, _ := json.Marshal(curBody["accepted_terms"])
	dst.mu.Lock()
	anchorBefore := dst.state.Reservations[anchor]
	anchorHistLen := len(dst.state.Histories[anchor])
	dst.mu.Unlock()
	if anchorHistLen == 0 {
		t.Fatal("anchor has no history after import")
	}
	body, _ := json.Marshal(map[string]any{"anchor_reference": anchor, "count": 2, "interval_weeks": 1})
	res := dst.AdoptSeries(tok, "donor-adopt-1", body)
	if res.Status != 201 {
		t.Fatalf("adopt eligible anchor = %d", res.Status)
	}
	m, _ := res.Body.(map[string]any)
	occ := m["occurrences"].([]any)
	if len(occ) != 2 {
		t.Fatalf("occurrences = %d", len(occ))
	}
	o0 := occ[0].(map[string]any)["reservation"].(map[string]any)
	if o0["reference"] != anchor {
		t.Fatalf("occurrence 0 = %v, want anchor %s", o0["reference"], anchor)
	}
	dst.mu.Lock()
	after := dst.state.Reservations[anchor]
	afterHist := len(dst.state.Histories[anchor])
	dst.mu.Unlock()
	if after.ReservationID != anchorBefore.ReservationID || after.CreatedAt != anchorBefore.CreatedAt ||
		afterHist != anchorHistLen {
		t.Fatal("imported anchor identity/history changed by adoption")
	}
	afterTerms, _ := json.Marshal(after.AcceptedTerms)
	_ = afterTerms
	cur2 := dst.GetReservation(tok, anchor)
	if cur2.Status != 200 {
		t.Fatalf("anchor lookup after adopt = %d", cur2.Status)
	}
	cur2Body, _ := cur2.Body.(map[string]any)
	terms2, _ := json.Marshal(cur2Body["accepted_terms"])
	if string(terms2) != string(anchorTerms) {
		t.Fatal("anchor accepted terms changed by adoption")
	}
	// Current series GET shows both occurrences with stored references.
	sid, _ := m["series_id"].(string)
	got := dst.GetSeries(tok, sid)
	if got.Status != 200 {
		t.Fatalf("series GET = %d", got.Status)
	}
	gm, _ := got.Body.(map[string]any)
	if len(gm["occurrences"].([]any)) != 2 {
		t.Fatal("series GET occurrences != 2")
	}
}

// rawOrRemarshal binds receipts to the manifest's preserved raw strings when
// present (exact stored bytes), else re-marshals the decoded value.
func rawOrRemarshal(rm map[string]any, rawKey string, decoded any) string {
	if raw, ok := rm[rawKey].(string); ok && raw != "" {
		return raw
	}
	b, _ := json.Marshal(decoded)
	return string(b)
}

// donorTokens returns the first bearer token for a donor user id.
func donorTokens(t *testing.T, man map[string]any, uid string) string {
	t.Helper()
	for _, u := range man["users"].([]any) {
		if u.(map[string]any)["id"] == uid {
			return u.(map[string]any)["tokens"].([]any)[0].(string)
		}
	}
	t.Fatalf("donor user %s has no token", uid)
	return ""
}

// donorRefsByOwner lists confirmed references per donor user from the
// manifest receipts (response references carry no tokens).
func donorRefsByOwner(t *testing.T, man map[string]any) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, r := range man["receipts"].([]any) {
		rm := r.(map[string]any)
		resp, ok := rm["response"].(map[string]any)
		if !ok {
			continue
		}
		ref, _ := resp["reference"].(string)
		status, _ := resp["status"].(string)
		if ref == "" || status != "confirmed" {
			continue
		}
		uid, _ := rm["user_id"].(string)
		out[uid] = append(out[uid], ref)
	}
	return out
}
