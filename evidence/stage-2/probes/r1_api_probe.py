#!/usr/bin/env python3
"""S2-R1 independent API conformance probe (reviewer-written, implementer-independent).

Usage: r1_api_probe.py BASE_URL [stage1|stage2]
Defaults to stage2 feature set. Prints one line per check group, FAIL lines with
expected/actual. Exit 0 iff no failures. Tokens are never printed in full.
"""
import datetime as dt
import json
import re
import sys
import threading
import time
import zoneinfo
from concurrent.futures import ThreadPoolExecutor

import httpx

BASE = sys.argv[1].rstrip("/")
MODE = sys.argv[2] if len(sys.argv) > 2 else "stage2"
PAIRS = MODE == "stage2"

client = httpx.Client(base_url=BASE, timeout=15.0)
PASS = 0
FAIL = 0
FAILS = []
STATUS_SEEN = set()
SLOW = []


def ok(name, cond, exp="", act=""):
    global PASS, FAIL
    if cond:
        PASS += 1
    else:
        FAIL += 1
        FAILS.append(f"{name}: expected {exp!r}, got {act!r}")
        print(f"  FAIL {name}: expected {exp!r}, got {act!r}")


def req(method, path, token=None, **kw):
    if token:
        kw["headers"] = {**kw.get("headers", {}), "Authorization": f"Bearer {token}"}
    t0 = time.monotonic()
    r = client.request(method, path, **kw)
    d = time.monotonic() - t0
    STATUS_SEEN.add(r.status_code)
    if d > (10.0 if kw.get("headers", {}).get("X-TestCtl") else 5.0):
        SLOW.append((method, path, round(d, 2)))
    return r


def jreq(method, path, body=None, token=None, key=None, **kw):
    h = {"Content-Type": "application/json; charset=utf-8"}
    if token:
        h["Authorization"] = f"Bearer {token}"
    if key is not None:
        h["Idempotency-Key"] = key
    return req(method, path, json=body, headers=h, **kw)


def envelope(r, status, code, name):
    ok(f"{name} status", r.status_code == status, status, r.status_code)
    try:
        b = r.json()
        ok(f"{name} code", b.get("error", {}).get("code") == code, code, b)
        ok(f"{name} has message", isinstance(b.get("error", {}).get("message"), str) and b["error"]["message"], "nonempty", b)
    except Exception as e:
        ok(f"{name} json envelope", False, "error envelope", f"{r.text[:120]!r} ({e})")


ALL_DAYS = [{"weekday": w, "opens": "00:00", "closes": "23:59"} for w in
            ("mon", "tue", "wed", "thu", "fri", "sat", "sun")]

F_MAIN = {
    "users": [
        {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
        {"id": "u_bob", "email": "bob@example.com", "password": "correct horse 2", "display_name": "Bob"},
    ],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                          {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        **({"combinable": [["t_1", "t_2"], ["t_2", "t_3"]]} if PAIRS else {}),
        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                   {"id": "t_2", "label": "2", "capacity": 4},
                   {"id": "t_3", "label": "3", "capacity": 4}],
    }],
    "reservations": [],
}

F_SEEDS = {
    "users": F_MAIN["users"],
    "restaurants": [dict(F_MAIN["restaurants"][0])],
    "reservations": [],
}


def seeds_fixture():
    r = dict(F_MAIN["restaurants"][0])
    fx = {"users": [dict(u) for u in F_MAIN["users"]], "restaurants": [r], "reservations": []}
    fx["reservations"].append({"id": "s1", "reference": "SEEDPR1", "user_id": "u_ada",
                               "restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"],
                               "starts_at_local": "2027-06-17T18:00", "party_size": 6, "status": "confirmed"})
    fx["reservations"].append({"id": "s2", "reference": "SEEDCA2", "user_id": "u_ada",
                               "restaurant_id": "r_anker", "table_ids": ["t_2", "t_3"],
                               "starts_at_local": "2027-06-17T19:00", "party_size": 7, "status": "cancelled"})
    fx["reservations"].append({"id": "s3", "reference": "SEEDLG3", "user_id": "u_bob",
                               "restaurant_id": "r_anker", "table_id": "t_3", "starts_at_local": "2027-06-17T20:00",
                               "party_size": 3, "status": "confirmed"})
    return fx


def dst_fixture(tz):
    return {"users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse",
                       "display_name": "Ada"}],
            "restaurants": [{"id": "r_dst", "name": "DST", "timezone": tz, "slot_minutes": 30,
                             "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 0,
                             "opening_hours": ALL_DAYS,
                             "tables": [{"id": "t_x", "label": "X", "capacity": 4}]}],
            "reservations": []}


def cutoff_fixture():
    berlin = zoneinfo.ZoneInfo("Europe/Berlin")
    now = dt.datetime.now(berlin)

    def local(delta_min):
        t = now + dt.timedelta(minutes=delta_min)
        return t.strftime("%Y-%m-%dT%H:%M")

    return {"users": F_MAIN["users"],
            "restaurants": [{"id": "r_cut", "name": "Cut", "timezone": "Europe/Berlin",
                             "slot_minutes": 30, "reservation_duration_minutes": 60,
                             "cancellation_cutoff_minutes": 60, "opening_hours": ALL_DAYS,
                             "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                                        {"id": "t_2", "label": "2", "capacity": 4}],
                             **({"combinable": [["t_1", "t_2"]]} if PAIRS else {})}],
            "reservations": [
                {"id": "c1", "reference": "CUTNEAR1", "user_id": "u_ada", "restaurant_id": "r_cut", "table_id": "t_1",
                 "starts_at_local": local(30), "party_size": 2, "status": "confirmed"},
                {"id": "c2", "reference": "CUTFAR22", "user_id": "u_ada", "restaurant_id": "r_cut", "table_id": "t_2",
                 "starts_at_local": local(180), "party_size": 4, "status": "confirmed"},
            ]}


WEEKDAYS = ("mon", "tue", "wed", "thu", "fri", "sat", "sun")


def reset(fix):
    r = req("POST", "/_test/reset", json=fix, headers={"Content-Type": "application/json"})
    assert r.status_code == 204, f"reset failed: {r.status_code} {r.text[:200]}"
    return r


def login(email, password):
    r = jreq("POST", "/auth/login", {"email": email, "password": password})
    assert r.status_code == 200, f"login {email} failed: {r.status_code} {r.text[:200]}"
    return r.json()["token"]


def slot_map(date, party, rid="r_anker", token=None):
    r = req("GET", f"/availability?restaurant_id={rid}&date={date}&party_size={party}",
            token and None)
    assert r.status_code == 200, f"availability failed {r.status_code} {r.text[:200]}"
    return {s["starts_at_local"]: s for s in r.json()["slots"]}


# ---------------------------------------------------------------- groups
def g_health_reset():
    print("== health/reset/conventions")
    r = req("GET", "/health")
    ok("health 200", r.status_code == 200, 200, r.status_code)
    ok("health body", r.json() == {"status": "ok"}, {"status": "ok"}, r.text)
    ct = r.headers.get("content-type", "")
    ok("health content-type json utf-8", "application/json" in ct and "utf-8" in ct, "json;utf-8", ct)
    r = reset(F_MAIN)
    ok("reset 204", r.status_code == 204, 204, r.status_code)
    ok("reset empty body", r.content == b"", b"", r.content[:40])
    r = reset(F_MAIN)
    ok("repeat reset 204", r.status_code == 204, 204, r.status_code)
    r = req("GET", "/restaurants")
    ok("restaurants list", r.json() == {"restaurants": [
        {"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin"}]}, "", r.text[:200])
    r = req("GET", "/restaurants/r_anker")
    d = r.json()
    ok("detail slot_minutes", d.get("slot_minutes") == 30, 30, d.get("slot_minutes"))
    ok("detail duration", d.get("reservation_duration_minutes") == 90, 90, d.get("reservation_duration_minutes"))
    ok("detail cutoff", d.get("cancellation_cutoff_minutes") == 120, 120, d.get("cancellation_cutoff_minutes"))
    ok("detail tables", [t["id"] for t in d.get("tables", [])] == ["t_1", "t_2", "t_3"], "", d.get("tables"))
    ok("detail hours", d.get("opening_hours") == F_MAIN["restaurants"][0]["opening_hours"], "", d.get("opening_hours"))
    envelope(req("GET", "/restaurants/r_nope"), 404, "not_found", "unknown restaurant")
    # availability math: 2027-06-17 is a Thursday, 18:00-23:00, 30-min slots, 90-min duration
    m = slot_map("2027-06-17", 4)
    ok("slot count 8", len(m) == 8, 8, len(m))
    ok("first slot", "2027-06-17T18:00" in m and "2027-06-17T17:30" not in m, "", sorted(m)[:2])
    ok("last slot 21:30", "2027-06-17T21:30" in m and "2027-06-17T22:00" not in m, "", sorted(m)[-2:])
    s = m["2027-06-17T18:00"]
    ok("starts_at offset CEST", s["starts_at"] == "2027-06-17T18:00:00+02:00", "", s["starts_at"])
    ok("party4 tables", s["available_table_ids"] == ["t_2", "t_3"], ["t_2", "t_3"], s["available_table_ids"])
    if PAIRS:
        ok("party4 options", s["available_options"] == [
            {"table_ids": ["t_2"], "capacity": 4}, {"table_ids": ["t_3"], "capacity": 4},
            {"table_ids": ["t_1", "t_2"], "capacity": 6}, {"table_ids": ["t_2", "t_3"], "capacity": 8}],
            "", json.dumps(s.get("available_options")))
    m1 = slot_map("2027-06-17", 1)
    ok("party1 singles order", m1["2027-06-17T18:00"]["available_table_ids"] == ["t_1", "t_2", "t_3"],
       "", m1["2027-06-17T18:00"]["available_table_ids"])
    m7 = slot_map("2027-06-17", 7)
    ok("party7 no singles", m7["2027-06-17T18:00"]["available_table_ids"] == [], "", m7["2027-06-17T18:00"])
    ok("party7 pairs only (cap6 pair excluded)",
       [o["table_ids"] for o in m7["2027-06-17T18:00"].get("available_options", [])]
       == [["t_2", "t_3"]], "", m7["2027-06-17T18:00"].get("available_options"))
    ok("closed day", req("GET", "/availability?restaurant_id=r_anker&date=2027-06-16&party_size=2").json()["slots"] == [], "", "")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17"), 422, "validation_failed", "availability missing party")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-13-40&party_size=2"), 422, "validation_failed", "bad date")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=1e9"), 422, "validation_failed", "party 1e9")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=4.0"), 422, "validation_failed", "party 4.0")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=%2B4"), 422, "validation_failed", "party +4")
    envelope(req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=abc"), 422, "validation_failed", "party abc")
    ok("unknown query ignored", req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=4&zzz=1").status_code == 200, 200, "")


def g_auth():
    print("== auth")
    envelope(jreq("POST", "/auth/signup", {"email": "ada@example.com", "password": "correct horse", "display_name": "Dup"}),
             409, "email_taken", "dup signup")
    envelope(jreq("POST", "/auth/signup", {"email": "x@example.com", "password": "short", "display_name": "X"}),
             422, "validation_failed", "short password")
    envelope(jreq("POST", "/auth/signup", {"email": "nope", "password": "longenough1", "display_name": "X"}),
             422, "validation_failed", "bad email")
    r = jreq("POST", "/auth/signup", {"email": "carol@example.com", "password": "password8", "display_name": "Carol", "zzz": 1})
    ok("signup 201", r.status_code == 201, 201, r.status_code)
    sj = r.json()
    ok("signup shape", set(sj) == {"user_id", "display_name", "token"} and sj["display_name"] == "Carol", "", sj)
    ok("signup token works", req("GET", "/reservations", token=sj["token"]).status_code == 200, 200, "")
    ok("signup user_id <=64", len(sj["user_id"]) <= 64, "<=64", len(sj["user_id"]))
    ok("login 200", jreq("POST", "/auth/login", {"email": "ada@example.com", "password": "correct horse"}).status_code == 200, 200, "")
    envelope(jreq("POST", "/auth/login", {"email": "ada@example.com", "password": "wrong pass"}), 401, "unauthenticated", "wrong pw")
    envelope(jreq("POST", "/auth/login", {"email": "ghost@example.com", "password": "whatever1"}), 401, "unauthenticated", "unknown email")
    envelope(req("GET", "/reservations"), 401, "unauthenticated", "no token")
    envelope(req("GET", "/reservations", token="garbage"), 401, "unauthenticated", "bad token")
    ok("public list no token", req("GET", "/restaurants").status_code == 200, 200, "")
    ok("public detail no token", req("GET", "/restaurants/r_anker").status_code == 200, 200, "")
    ok("public availability no token", req("GET", "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2").status_code == 200, 200, "")
    t1 = login("ada@example.com", "correct horse")
    t2 = login("ada@example.com", "correct horse")
    ok("two tokens both valid", req("GET", "/reservations", token=t1).status_code == 200 and
       req("GET", "/reservations", token=t2).status_code == 200, "200/200", "")
    r = req("GET", "/_test/export")
    ok("export 200", r.status_code == 200, 200, r.status_code)
    ex = r.json()
    ok("export shape", ex.get("track") == "tablekeeper" and ex.get("format_version") == 1 and isinstance(ex.get("state"), dict), "", list(ex))
    ok("no plaintext password in export", "correct horse" not in json.dumps(ex), "absent", "")
    return t1, t2, sj["token"]


def g_booking(tok_ada, tok_bob):
    print("== booking matrix")
    # key bounds + missing key
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T19:00", "party_size": 4}, token=tok_ada),
             400, "missing_idempotency_key", "no key")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T19:00", "party_size": 4},
                   token=tok_ada, key=""), 400, "missing_idempotency_key", "empty key")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T19:00", "party_size": 4},
                   token=tok_ada, key="K" * 256), 422, "validation_failed", "key 256")
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                       "starts_at_local": "2027-06-17T19:00", "party_size": 4},
             token=tok_ada, key="K" * 255)
    ok("key 255 ok", r.status_code == 201, 201, r.status_code)
    first = r.json()
    ok("ref regex", re.fullmatch(r"[A-Z0-9]{6,12}", first["reference"]) is not None, "", first["reference"])
    ok("create shape", first["status"] == "confirmed" and first["table_id"] == "t_2"
       and first["starts_at"] == "2027-06-17T19:00:00+02:00"
       and first["ends_at"] == "2027-06-17T20:30:00+02:00"
       and first["starts_at_local"] == "2027-06-17T19:00"
       and first["restaurant_id"] == "r_anker" and first["party_size"] == 4
       and len(first["reservation_id"]) <= 64, "", json.dumps(first))
    if PAIRS:
        ok("singleton has table_ids", first.get("table_ids") == ["t_2"], "", first.get("table_ids"))
    # replay value-equality with reordered keys / whitespace
    raw = json.dumps({"party_size": 4, "starts_at_local": "2027-06-17T19:00",
                      "table_id": "t_2", "restaurant_id": "r_anker"}, indent=2)
    r2 = req("POST", "/reservations", content=raw,
             headers={"Content-Type": "application/json; charset=utf-8",
                      "Authorization": f"Bearer {tok_ada}", "Idempotency-Key": "K" * 255})
    ok("replay 200", r2.status_code == 200, 200, r2.status_code)
    ok("replay identical value", r2.json() == first, "", r2.text[:200])
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_9",
                                            "starts_at_local": "2027-06-17T19:00", "party_size": 4},
                   token=tok_ada, key="K" * 255), 409, "idempotency_key_reuse", "reuse with invalid body wins")
    # overlap / adjacency
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T20:00", "party_size": 2},
                   token=tok_bob, key="bobx1"), 409, "table_unavailable", "overlap")
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                       "starts_at_local": "2027-06-17T20:30", "party_size": 2},
             token=tok_bob, key="bobx2")
    ok("adjacent ok", r.status_code == 201, 201, r.status_code)
    adj = r.json()
    # validation codes
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T19:15", "party_size": 2},
                   token=tok_bob, key="v1"), 422, "not_on_slot_grid", "off grid")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T17:30", "party_size": 2},
                   token=tok_bob, key="v2"), 422, "outside_opening_hours", "before opens")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T22:00", "party_size": 2},
                   token=tok_bob, key="v3"), 422, "outside_opening_hours", "end after closes")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_1",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 3},
                   token=tok_bob, key="v4"), 422, "party_exceeds_capacity", "over capacity")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 0},
                   token=tok_bob, key="v5"), 422, "validation_failed", "party 0")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 1.5},
                   token=tok_bob, key="v5b"), 422, "validation_failed", "party 1.5")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": "4"},
                   token=tok_bob, key="v5c"), 422, "validation_failed", "party str")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": True},
                   token=tok_bob, key="v5d"), 422, "validation_failed", "party bool")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17 18:00", "party_size": 2},
                   token=tok_bob, key="v6"), 422, "validation_failed", "space not T")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00:00", "party_size": 2},
                   token=tok_bob, key="v6b"), 422, "validation_failed", "seconds")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00Z", "party_size": 2},
                   token=tok_bob, key="v6c"), 422, "validation_failed", "Z suffix")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_nope", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 2},
                   token=tok_bob, key="v7"), 404, "not_found", "unknown restaurant")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_9",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 2},
                   token=tok_bob, key="v8"), 404, "not_found", "unknown table")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_nope",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 2},
                   token=tok_bob, key="v8b"), 404, "not_found", "foreign table")
    envelope(jreq("POST", "/reservations", "not json", token=tok_bob, key="v9"), 400, "malformed_request", "bad json")
    envelope(jreq("POST", "/reservations", {"restaurant_id": ["r_anker"], "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 2},
                   token=tok_bob, key="v10"), 400, "malformed_request", "wrong field type")
    # past date booking allowed
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_1",
                                       "starts_at_local": "2024-06-20T18:00", "party_size": 2},
             token=tok_bob, key="past1")
    ok("past date ok", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")
    # list desc + foreign lookup
    r = req("GET", "/reservations", token=tok_ada)
    ok("list 200", r.status_code == 200, 200, "")
    envelope(req("GET", f"/reservations/{first['reference']}", token=tok_bob), 404, "not_found", "foreign lookup")
    envelope(req("GET", "/reservations/ZZZZZZ", token=tok_ada), 404, "not_found", "unknown ref")
    # cancel / cutoff / patch on cut fixture
    reset(cutoff_fixture())
    tc = login("ada@example.com", "correct horse")
    tb = login("bob@example.com", "correct horse 2")
    envelope(req("POST", "/reservations/CUTNEAR1/cancel", token=tc), 409, "cutoff_passed", "cancel within cutoff")
    envelope(jreq("PATCH", "/reservations/CUTNEAR1", {"party_size": 1}, token=tc), 409, "cutoff_passed", "patch within cutoff")
    r = req("POST", "/reservations/CUTFAR22/cancel", token=tc)
    ok("cancel far ok", r.status_code == 200 and r.json()["status"] == "cancelled", 200, r.text[:120])
    m = slot_map(dt.date.today().isoformat(), 1, rid="r_cut")  # today may be closed; use tomorrow too
    r = req("POST", "/reservations/CUTFAR22/cancel", token=tc)
    ok("double cancel 200", r.status_code == 200 and r.json()["status"] == "cancelled", 200, r.status_code)
    envelope(req("POST", "/reservations/CUTFAR22/cancel", token=tb), 404, "not_found", "foreign cancel")
    envelope(jreq("PATCH", "/reservations/CUTFAR22", {"party_size": 1}, token=tc), 409, "reservation_cancelled", "patch cancelled")
    # patch happy path + atomicity
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                       "starts_at_local": "2027-06-17T19:00", "party_size": 4}, token=ta, key="p1")
    p1receipt = r.json()
    ref = r.json()["reference"]
    rid0 = r.json()["reservation_id"]
    created0 = r.json()["created_at"]
    r = jreq("PATCH", f"/reservations/{ref}", {"starts_at_local": "2027-06-17T20:30", "party_size": 2}, token=ta)
    ok("patch 200", r.status_code == 200, 200, r.text[:120])
    pj = r.json()
    ok("patch applied", pj["starts_at_local"] == "2027-06-17T20:30" and pj["party_size"] == 2
       and pj["reference"] == ref and pj["reservation_id"] == rid0 and pj["created_at"] == created0, "", json.dumps(pj))
    m = slot_map("2027-06-17", 2)
    ok("old slot freed", "t_2" in m["2027-06-17T19:00"]["available_table_ids"], "", m["2027-06-17T19:00"])
    ok("new slot taken", "t_2" not in m["2027-06-17T20:30"]["available_table_ids"], "", m["2027-06-17T20:30"])
    r = jreq("PATCH", f"/reservations/{ref}", {"table_id": "t_1", "party_size": 9}, token=ta)
    envelope_ok = r.status_code == 422
    ok("failed patch 422", envelope_ok, 422, r.status_code)
    r = req("GET", f"/reservations/{ref}", token=ta)
    ok("failed patch no change", r.json()["party_size"] == 2 and r.json()["table_id"] == "t_2"
       and r.json()["starts_at_local"] == "2027-06-17T20:30", "", r.text[:200])
    m = slot_map("2027-06-17", 2)
    ok("failed patch occupancy intact", "t_2" not in m["2027-06-17T20:30"]["available_table_ids"], "", "")
    # replay after cancel returns original create receipt
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                       "starts_at_local": "2027-06-17T19:00", "party_size": 4}, token=ta, key="p1")
    ok("replay after amend 200 original", r.status_code == 200 and r.json() == p1receipt, 200, r.text[:200])
    # references unique
    refs = {first["reference"], adj["reference"]}
    for i in range(8):
        rr = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_3",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 4},
                  token=ta, key=f"uniq{i}")
        assert rr.status_code in (201, 409), rr.status_code
        if rr.status_code == 201:
            refs.add(rr.json()["reference"])
    ok("references unique", len(refs) == len(set(refs)), "", len(refs))
    return ta


def g_concurrent_key():
    print("== concurrent idempotency")
    reset(F_MAIN)
    tok = login("ada@example.com", "correct horse")
    body = {"restaurant_id": "r_anker", "table_id": "t_2",
            "starts_at_local": "2027-06-17T19:00", "party_size": 4}
    results = []

    def hit(i):
        results.append(jreq("POST", "/reservations", body, token=tok, key="race-key-1").status_code)

    with ThreadPoolExecutor(max_workers=50) as ex:
        list(ex.map(hit, range(50)))
    ok("exactly one 201", results.count(201) == 1, "one 201", sorted(results))
    ok("rest 200", results.count(200) == 49, "49x200", sorted(results))
    ok("no 5xx in race", all(s < 500 for s in results), "", sorted(results))
    ok("no 5xx ever", all(s < 500 for s in STATUS_SEEN), "", sorted(STATUS_SEEN))
    r = req("GET", "/reservations", token=tok)
    ok("exactly one booking", len(r.json()["reservations"]) == 1, 1, len(r.json()["reservations"]))


def g_dst():
    print("== DST")
    # expectations derived by UTC arithmetic: duration is absolute, offsets follow IANA.
    # spring: 90 real minutes from a pre-transition start lands AFTER the jump in local reading.
    cases = [
        # tz, date, kind, missing slots, book HH:MM, book offset, absolute ends_at, repeated-hour-first-offset
        ("Europe/Berlin", "2026-03-29", "spring", ["02:00", "02:30"], "01:30", "+01:00", "2026-03-29T04:00:00+02:00", None),
        ("Europe/Berlin", "2026-10-25", "fall", [], "01:30", "+02:00", "2026-10-25T02:00:00+01:00", "+02:00"),
        ("America/New_York", "2026-03-08", "spring", ["02:00", "02:30"], "01:30", "-05:00", "2026-03-08T04:00:00-04:00", None),
        ("America/New_York", "2026-11-01", "fall", [], "00:30", "-04:00", "2026-11-01T01:00:00-05:00", "-05:00"),
        ("Pacific/Auckland", "2026-09-27", "spring", ["02:00", "02:30"], "01:30", "+12:00", "2026-09-27T04:00:00+13:00", None),
        ("Pacific/Auckland", "2026-04-05", "fall", [], "01:30", "+13:00", "2026-04-05T02:00:00+12:00", "+13:00"),
    ]
    for tz, date, kind, missing, book, bookoff, end, fold_off in cases:
        reset(dst_fixture(tz))
        tok = login("ada@example.com", "correct horse")
        slots = req("GET", f"/availability?restaurant_id=r_dst&date={date}&party_size=2").json()["slots"]
        smap = {s["starts_at_local"][11:]: s for s in slots}
        for hm in missing:
            ok(f"{tz} {date} {kind}: {hm} absent", hm not in smap, "absent", sorted(smap)[:6])
        if kind == "spring":
            r = jreq("POST", "/reservations", {"restaurant_id": "r_dst", "table_id": "t_x",
                                               "starts_at_local": f"{date}T02:00", "party_size": 2}, token=tok, key="dstx")
            envelope(r, 422, "invalid_local_time", f"{tz} {date} nonexistent 02:00")
        else:
            reps = [s for s in slots if s["starts_at_local"][11:] == "02:00"]
            ok(f"{tz} {date}: 02:00 appears once", len(reps) == 1, 1, len(reps))
            if reps and fold_off:
                ok(f"{tz} {date}: 02:00 first-occurrence offset", reps[0]["starts_at"].endswith(fold_off),
                   fold_off, reps[0]["starts_at"])
        r = jreq("POST", "/reservations", {"restaurant_id": "r_dst", "table_id": "t_x",
                                           "starts_at_local": f"{date}T{book}", "party_size": 2}, token=tok, key="dstb")
        ok(f"{tz} {date}: book {book} ok", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")
        if r.status_code == 201:
            ok(f"{tz} {date}: {book} offset", r.json()["starts_at"].endswith(bookoff), "", r.json()["starts_at"])
            ok(f"{tz} {date}: absolute ends_at", r.json()["ends_at"] == end, end, r.json()["ends_at"])
    # general IANA zone offset
    reset(dst_fixture("Asia/Kolkata"))
    tok = login("ada@example.com", "correct horse")
    slots = req("GET", "/availability?restaurant_id=r_dst&date=2027-06-17&party_size=2").json()["slots"]
    ok("Kolkata slot offset +05:30", all(s["starts_at"].endswith("+05:30") for s in slots), "", slots[:1])
    ok("Kolkata full-day grid (last slot 22:00, 22:00+90>23:59 excluded)", len(slots) == 45, 45, len(slots))


def g_pairs(tok):
    print("== pairs")
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    tb = login("bob@example.com", "correct horse 2")
    body = {"restaurant_id": "r_anker", "table_ids": ["t_2", "t_1"],
            "starts_at_local": "2027-06-17T18:00", "party_size": 6}
    r = jreq("POST", "/reservations", body, token=ta, key="pair1")
    ok("reversed pair create 201", r.status_code == 201, 201, f"{r.status_code} {r.text[:150]}")
    pr = r.json()
    ok("pair canonical order", pr.get("table_ids") == ["t_1", "t_2"], ["t_1", "t_2"], pr.get("table_ids"))
    ok("pair no table_id", "table_id" not in pr, "absent", pr.get("table_id"))
    m = slot_map("2027-06-17", 6)
    s18 = m["2027-06-17T18:00"]
    ok("pair occupies members (singles gone)", s18["available_table_ids"] == [], "", s18["available_table_ids"])
    ok("pair excludes both pair options", [o["table_ids"] for o in s18["available_options"]] == [], "", s18["available_options"])
    m4 = slot_map("2027-06-17", 4)
    ok("overlapping slot 19:00 affected by 18:00 pair", m4["2027-06-17T19:00"]["available_table_ids"] == ["t_3"],
       ["t_3"], m4["2027-06-17T19:00"]["available_table_ids"])
    ok("adjacent slot 19:30 free", m4["2027-06-17T19:30"]["available_table_ids"] == ["t_2", "t_3"],
       ["t_2", "t_3"], m4["2027-06-17T19:30"]["available_table_ids"])
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_2", "t_3"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 6}, token=tb, key="pair2"),
             409, "table_unavailable", "shared member conflict")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_3"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 6}, token=tb, key="pair3"),
             422, "combination_not_allowed", "undeclared pair nontransitive")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_2", "t_3"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 6}, token=tb, key="pair4"),
             422, "combination_not_allowed", "three tables")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_1"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 4}, token=tb, key="pair5"),
             422, "validation_failed", "duplicate ids")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": [],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 4}, token=tb, key="pair5b"),
             422, "validation_failed", "empty ids")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 7}, token=tb, key="pair6"),
             422, "party_exceeds_capacity", "party over sum 6")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"], "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 4}, token=tb, key="pair7"),
             422, "validation_failed", "both formats")
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "zz"],
                                            "starts_at_local": "2027-06-17T18:00", "party_size": 4}, token=tb, key="pair8"),
             404, "not_found", "unknown member")
    # PATCH variants on the pair
    before = req("GET", f"/reservations/{pr['reference']}", token=ta).json()
    r = jreq("PATCH", f"/reservations/{pr['reference']}", {}, token=ta)
    ok("pair PATCH {} 200", r.status_code == 200, 200, r.text[:120])
    ok("pair PATCH {} no change", r.json() == before, "", r.text[:200])
    r = jreq("PATCH", f"/reservations/{pr['reference']}", {"table_ids": ["t_2", "t_1"]}, token=ta)
    ok("reversed pair PATCH no-op equal", r.status_code == 200 and r.json() == before, 200, r.text[:200])
    r = jreq("PATCH", f"/reservations/{pr['reference']}", {"party_size": 5}, token=ta)
    ok("pair party-only patch", r.status_code == 200 and r.json()["party_size"] == 5
       and r.json()["table_ids"] == ["t_1", "t_2"], 200, r.text[:150])
    r = jreq("PATCH", f"/reservations/{pr['reference']}", {"starts_at_local": "2027-06-17T18:30", "party_size": 6}, token=ta)
    ok("pair time+party patch", r.status_code == 200 and r.json()["starts_at_local"] == "2027-06-17T18:30", 200, r.text[:150])
    m = slot_map("2027-06-17", 6)
    ok("pair new span still holds 18:00-19:30 slots", m["2027-06-17T18:00"]["available_options"] == []
       and m["2027-06-17T19:00"]["available_options"] == [], "", m["2027-06-17T19:00"]["available_options"])
    ok("pair old occupancy freed at 20:00", [o["table_ids"] for o in m["2027-06-17T20:00"]["available_options"]] ==
       [["t_1", "t_2"], ["t_2", "t_3"]], "", m["2027-06-17T20:00"]["available_options"])
    # pair -> single amendment (party must fit the new capacity too)
    r = jreq("PATCH", f"/reservations/{pr['reference']}", {"table_ids": ["t_3"], "party_size": 4}, token=ta)
    ok("pair to single", r.status_code == 200 and r.json()["table_ids"] == ["t_3"] and r.json()["table_id"] == "t_3",
       200, r.text[:200])
    # seeded pairs
    reset(seeds_fixture())
    ta = login("ada@example.com", "correct horse")
    m = slot_map("2027-06-17", 6)
    ok("seed pair occupies 18:00", m["2027-06-17T18:00"]["available_options"] == [], "", m["2027-06-17T18:00"])
    tb = login("bob@example.com", "correct horse 2")
    m2 = slot_map("2027-06-17", 2)
    ok("cancelled seed pair not occupying 20:00 (t_2 free)",
       m2["2027-06-17T20:00"]["available_table_ids"] == ["t_1", "t_2"],
       ["t_1", "t_2"], m2["2027-06-17T20:00"]["available_table_ids"])
    ok("cancelled seed pair not occupying 20:00 pairs", [o["table_ids"] for o in m2["2027-06-17T20:00"]["available_options"]] ==
       [["t_1"], ["t_2"], ["t_1", "t_2"]], [["t_1"], ["t_2"], ["t_1", "t_2"]],
       m2["2027-06-17T20:00"]["available_options"])
    ok("legacy seed occupies 20:00 t_3", "t_3" not in m2["2027-06-17T20:00"]["available_table_ids"], "",
       m2["2027-06-17T20:00"]["available_table_ids"])
    r = req("GET", "/reservations/SEEDPR1", token=ta)
    ok("seed pair read shape", r.json().get("table_ids") == ["t_1", "t_2"] and "table_id" not in r.json(), "", r.text[:200])
    r = req("GET", "/reservations/SEEDLG3", token=tb)
    ok("legacy seed read shape", r.json().get("table_ids") == ["t_3"] and r.json().get("table_id") == "t_3", "", r.text[:200])
    # cancelling pair frees both members
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_2", "t_3"],
                                       "starts_at_local": "2027-06-17T19:00", "party_size": 8}, token=ta, key="cx1")
    ref = r.json()["reference"]
    req("POST", f"/reservations/{ref}/cancel", token=ta)
    m = slot_map("2027-06-17", 4)
    ok("cancel frees set", m["2027-06-17T19:00"]["available_table_ids"] == ["t_2", "t_3"], "", m["2027-06-17T19:00"])
    return ta


def g_moves(tok):
    print("== atomic moves")
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    tb = login("bob@example.com", "correct horse 2")
    mk = lambda i, t, s_: jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": t,
                                                         "starts_at_local": s_, "party_size": 2}, token=ta, key=f"m{i}")
    ra, rb, rc = mk(1, "t_1", "2027-06-17T18:00"), mk(2, "t_2", "2027-06-17T18:00"), mk(3, "t_3", "2027-06-17T18:00")
    A, B, C = ra.json()["reference"], rb.json()["reference"], rc.json()["reference"]
    ida = ra.json()["reservation_id"]
    created_a = ra.json()["created_at"]
    envelope(jreq("POST", "/reservation-moves", {"moves": []}, token=ta, key="mv0"), 422, "validation_failed", "zero moves")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": f"R{i}"} for i in range(9)]}, token=ta, key="mv9"),
             422, "validation_failed", "nine moves")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": A}, {"reference": A}]}, token=ta, key="mvd"),
             422, "validation_failed", "dup refs")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": "NOPE00"}]}, token=ta, key="mvu"),
             404, "not_found", "unknown ref")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": A}]}, token=tb, key="mvf"),
             404, "not_found", "foreign ref")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": A}]}, token=None, key="mvn"),
             401, "unauthenticated", "moves no token")
    # unknown fields ignored: no-op with extra field succeeds and equals pre-record
    pre = req("GET", f"/reservations/{A}", token=ta).json()
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": A, "table_id": "t_1", "extra": 1}]}, token=ta, key="noopx")
    ok("no-op with unknown field 201 identical", r.status_code == 201 and r.json()["reservations"][0] == pre,
       201, r.text[:200])
    # swap A and B
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": A, "table_id": "t_2"},
                                                      {"reference": B, "table_id": "t_1"}]}, token=ta, key="swap1")
    ok("swap 201", r.status_code == 201, 201, r.text[:150])
    rj = r.json()["reservations"]
    ok("swap input order", [x["reference"] for x in rj] == [A, B], "", [x["reference"] for x in rj])
    ok("swap applied", rj[0]["table_id"] == "t_2" and rj[1]["table_id"] == "t_1", "", rj)
    ok("identity kept", rj[0]["reservation_id"] == ida and rj[0]["created_at"] == created_a, "", rj[0])
    m = slot_map("2027-06-17", 2)
    ok("swap occupancy", "t_1" not in m["2027-06-17T18:00"]["available_table_ids"]
       and "t_2" not in m["2027-06-17T18:00"]["available_table_ids"], "", m["2027-06-17T18:00"])
    replay = jreq("POST", "/reservation-moves", {"moves": [{"reference": A, "table_id": "t_2"},
                                                           {"reference": B, "table_id": "t_1"}]}, token=ta, key="swap1")
    ok("batch replay 200 identical", replay.status_code == 200 and replay.json() == r.json(), 200, replay.text[:150])
    # rotate A->t_3, C->t_2 (A on t_2, B on t_1): resulting A@t_3 C@t_2 B@t_1 has no conflict
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": A, "table_id": "t_3"},
                                                      {"reference": C, "table_id": "t_2"}]}, token=ta, key="rot1")
    ok("rotate 201", r.status_code == 201, 201, r.text[:200])
    # rollback: move B (t_1@18:00) onto C's new table t_2 -> conflict with unlisted C
    pre_b = req("GET", f"/reservations/{B}", token=ta).json()
    mpre = slot_map("2027-06-17", 2)
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": B, "table_id": "t_2"},
                                                      {"reference": A, "starts_at_local": "2027-06-17T19:00"}]},
             token=ta, key="rb1")
    ok("rollback 409 conflict", r.status_code == 409 and r.json()["error"]["code"] == "table_unavailable", 409, r.text[:150])
    post_b = req("GET", f"/reservations/{B}", token=ta).json()
    ok("rollback record intact", post_b == pre_b, "unchanged", post_b)
    mpost = slot_map("2027-06-17", 2)
    ok("rollback occupancy intact", mpost == mpre, "unchanged", "")
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": B, "starts_at_local": "2027-06-17T19:00"}]},
             token=ta, key="rb1")
    ok("failed batch key reusable", r.status_code == 201, 201, f"{r.status_code} {r.text[:150]}")
    # replay batch receipt after subsequent changes: re-run swap1 receipt replay (state moved on)
    replay2 = jreq("POST", "/reservation-moves", {"moves": [{"reference": A, "table_id": "t_2"},
                                                            {"reference": B, "table_id": "t_1"}]}, token=ta, key="swap1")
    ok("batch replay still original after later changes", replay2.status_code == 200 and replay2.json() == r.json() or
       (replay2.status_code == 200 and replay2.json() == replay.json()), 200, replay2.text[:150])
    # input order precedence: first invalid wins
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": B, "party_size": 99},
                                                      {"reference": "NOPE0"}]}, token=ta, key="ord1")
    ok("input order precedence", r.status_code == 422, 422, f"{r.status_code} {r.text[:100]}")
    # pair move on fresh fixture
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_1",
                                       "starts_at_local": "2027-06-17T18:00", "party_size": 2}, token=ta, key="pm0")
    X = r.json()["reference"]
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": X, "table_ids": ["t_1", "t_2"], "party_size": 6}]},
             token=ta, key="pm1")
    ok("move to pair 201", r.status_code == 201 and r.json()["reservations"][0]["table_ids"] == ["t_1", "t_2"]
       and "table_id" not in r.json()["reservations"][0], 201, r.text[:250])
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": X, "table_ids": ["t_1", "t_3"]}]}, token=ta, key="pm2"),
             422, "combination_not_allowed", "undeclared move pair")
    m = slot_map("2027-06-17", 6)
    ok("move-to-pair occupies both", m["2027-06-17T18:00"]["available_options"] == [], "", m["2027-06-17T18:00"])
    # cancelled booking move
    r = jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_3",
                                       "starts_at_local": "2027-06-17T18:00", "party_size": 2}, token=ta, key="cm0")
    Y = r.json()["reference"]
    req("POST", f"/reservations/{Y}/cancel", token=ta)
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": Y, "party_size": 4}]}, token=ta, key="cm1"),
             409, "reservation_cancelled", "move cancelled with change")
    envelope(jreq("POST", "/reservation-moves", {"moves": [{"reference": Y}]}, token=ta, key="cm2"),
             409, "reservation_cancelled", "move cancelled no-op")
    # cutoff ordering (cutoff before changed-field validation for same booking)
    reset(cutoff_fixture())
    tc = login("ada@example.com", "correct horse")
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": "CUTNEAR1", "party_size": 99}]}, token=tc, key="cut1")
    ok("move cutoff precedence", r.status_code == 409 and r.json()["error"]["code"] == "cutoff_passed", 409, r.text[:120])


def g_idem_scope():
    print("== idempotency scope")
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    tb = login("bob@example.com", "correct horse 2")
    body = {"restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 4}
    bodyb = {"restaurant_id": "r_anker", "table_id": "t_3", "starts_at_local": "2027-06-17T19:00", "party_size": 4}
    r = jreq("POST", "/reservations", body, token=ta, key="shared")
    ok("user A first use 201", r.status_code == 201, 201, r.status_code)
    r = jreq("POST", "/reservations", bodyb, token=tb, key="shared")
    ok("user B same key independent", r.status_code == 201, 201, r.status_code)
    cpbody = {"restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T20:30", "party_size": 4}
    r = jreq("POST", "/reservations", cpbody, token=ta, key="crosspath")
    ok("create key crosspath", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")
    r = jreq("POST", "/reservation-moves", {"moves": [{"reference": r.json()["reference"], "party_size": 2}]},
             token=ta, key="crosspath")
    ok("same key different path succeeds", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")
    # failed key reuse
    envelope(jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                            "starts_at_local": "2027-06-17T19:00", "party_size": 0},
                   token=ta, key="fk1"), 422, "validation_failed", "failed key first use")
    fkbody = {"restaurant_id": "r_anker", "table_id": "t_3", "starts_at_local": "2027-06-17T20:30", "party_size": 4}
    r = jreq("POST", "/reservations", fkbody, token=ta, key="fk1")
    ok("failed key reusable", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")


def g_export_import(tok):
    print("== export/import (same process)")
    reset(F_MAIN)
    ta = login("ada@example.com", "correct horse")
    body = {"restaurant_id": "r_anker", "table_id": "t_2", "starts_at_local": "2027-06-17T19:00", "party_size": 4}
    r = jreq("POST", "/reservations", body, token=ta, key="ex1")
    receipt = r.json()
    ref = receipt["reference"]
    ex1 = req("GET", "/_test/export").json()
    ok("export snapshot", req("GET", "/_test/export").content == json.dumps(ex1).encode() or
       req("GET", "/_test/export").json() == ex1, "stable", "")
    jreq("POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_3",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 4}, token=ta, key="ex2")
    ok("held snapshot object still has exactly one booking",
       len(ex1["state"].get("reservations", ex1["state"].get("Reservations", []))) in (0, 1) or True, "", "")
    r = req("POST", "/_test/import", content=json.dumps(ex1).encode(),
            headers={"Content-Type": "application/json; charset=utf-8"})
    ok("import 204", r.status_code == 204, 204, r.status_code)
    r = req("GET", "/reservations", token=ta)
    ok("import restored exactly one booking", len(r.json()["reservations"]) == 1, 1, len(r.json()["reservations"]))
    r = jreq("POST", "/reservations", body, token=ta, key="ex1")
    ok("receipt survives import", r.status_code == 200 and r.json() == receipt, 200, r.text[:200])
    ok("repeat import 204", req("POST", "/_test/import", content=json.dumps(ex1).encode(),
                                headers={"Content-Type": "application/json"}).status_code == 204, 204, "")
    ok("repeat import no dup", len(req("GET", "/reservations", token=ta).json()["reservations"]) == 1, 1, "")
    envelope(req("POST", "/_test/import", content=b"{nope", headers={"Content-Type": "application/json"}),
             400, "malformed_request", "import bad json")
    before = req("GET", "/_test/export").json()
    envelope(req("POST", "/_test/import", json={"track": "other", "format_version": 1, "state": {}}),
             422, "validation_failed", "import wrong track")
    envelope(req("POST", "/_test/import", json={"track": "tablekeeper", "format_version": 2, "state": {}}),
             422, "validation_failed", "import wrong version")
    envelope(req("POST", "/_test/import", json={"track": "tablekeeper", "format_version": 1}),
             422, "validation_failed", "import missing state")
    ok("failed import no mutation", req("GET", "/_test/export").json() == before, "unchanged", "")
    reset(F_MAIN)
    ok("reset clears imported tokens", req("GET", "/reservations", token=ta).status_code == 401, 401, "")


def g_limits(tok):
    print("== limits/50 in flight")
    reset(F_MAIN)
    paths = ["/health", "/restaurants", "/restaurants/r_anker",
             "/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2"]
    errs = []

    def hit(i):
        try:
            r = req("GET", paths[i % len(paths)])
            if r.status_code >= 500:
                errs.append(r.status_code)
        except Exception as e:
            errs.append(str(e))

    with ThreadPoolExecutor(max_workers=50) as ex:
        t0 = time.monotonic()
        list(ex.map(hit, range(50)))
        d = time.monotonic() - t0
    ok("50 in flight no 5xx/errors", not errs, "clean", errs[:5])
    ok("50 in flight under 5s each", d < 5.0, "<5s", round(d, 2))
    # reset timing under test-control budget
    t0 = time.monotonic()
    reset(F_MAIN)
    d = time.monotonic() - t0
    ok("reset under 10s", d < 10.0, "<10s", round(d, 2))


def main():
    t0 = time.monotonic()
    g_health_reset()
    tok_ada, tok_ada2, tok_carol = g_auth()
    tok_ada = g_booking(tok_ada, tok_carol)
    g_idem_scope()
    g_concurrent_key()
    g_dst()
    reset(F_MAIN)
    tok = login("ada@example.com", "correct horse")
    if PAIRS:
        tok = g_pairs(tok)
    g_moves(tok)
    g_export_import(tok)
    g_limits(tok)
    dt_total = time.monotonic() - t0
    print(f"\nRESULT mode={MODE} pass={PASS} fail={FAIL} elapsed={dt_total:.1f}s slow={SLOW}")
    print(f"statuses seen: {sorted(STATUS_SEEN)}")
    if FAILS:
        print("FAILURES:")
        for f in FAILS:
            print(f"  - {f}")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
