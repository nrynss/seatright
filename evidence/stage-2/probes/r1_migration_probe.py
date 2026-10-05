#!/usr/bin/env python3
"""S2-R1 cross-process migration probe: fresh stage-1 donor -> stage-2 destinations.

Usage: r1_migration_probe.py DONOR_URL DEST1_URL DEST2_URL
Distinct container processes for source and both destinations. Tokens are never
printed in full; export bodies stay in memory (throwaway test credentials only).
"""
import json
import sys
from concurrent.futures import ThreadPoolExecutor

import httpx

DONOR = sys.argv[1].rstrip("/")
DEST1 = sys.argv[2].rstrip("/")
DEST2 = sys.argv[3].rstrip("/")

PASS = 0
FAIL = 0
FAILS = []


def ok(name, cond, exp="", act=""):
    global PASS, FAIL
    if cond:
        PASS += 1
    else:
        FAIL += 1
        FAILS.append(f"{name}: expected {exp!r} got {act!r}")
        print(f"  FAIL {name}: expected {exp!r} got {act!r}")


def cl(base):
    return httpx.Client(base_url=base, timeout=15)


def j(c, method, path, body=None, token=None, key=None):
    h = {"Content-Type": "application/json; charset=utf-8"}
    if token:
        h["Authorization"] = f"Bearer {token}"
    if key is not None:
        h["Idempotency-Key"] = key
    return c.request(method, path, json=body, headers=h)


FIX = {
    "users": [
        {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
        {"id": "u_bob", "email": "bob@example.com", "password": "correct horse 2", "display_name": "Bob"},
    ],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                   {"id": "t_2", "label": "2", "capacity": 4}],
    }],
    "reservations": [],
}
FIX2 = dict(FIX)
FIX2["restaurants"] = [dict(FIX["restaurants"][0])]
FIX2["restaurants"][0]["combinable"] = [["t_1", "t_2"]]


def mask(s):
    return (s[:6] + "...") if isinstance(s, str) else s


print("== phase 1: build state in fresh stage-1 donor")
d = cl(DONOR)
assert j(d, "POST", "/_test/reset", FIX).status_code == 204
tok1a = j(d, "POST", "/auth/signup", {"email": "up1@example.com", "password": "password11", "display_name": "Up One"}).json()["token"]
tok1b = j(d, "POST", "/auth/login", {"email": "up1@example.com", "password": "password11"}).json()["token"]
tok2 = j(d, "POST", "/auth/signup", {"email": "up2@example.com", "password": "password22", "display_name": "Up Two"}).json()["token"]
r = j(d, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 3}, token=tok1a, key="leg-K1")
ok("donor create 201", r.status_code == 201, 201, r.text[:120])
leg_create = r.json()
ref1 = leg_create["reference"]
r = j(d, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_1",
                                   "starts_at_local": "2027-06-17T18:00", "party_size": 2}, token=tok1a, key="leg-K2")
ok("donor create2 201", r.status_code == 201, 201, r.status_code)
ref2 = r.json()["reference"]
r = j(d, "POST", "/reservation-moves", {"moves": [{"reference": ref1, "starts_at_local": "2027-06-17T20:30"}]},
      token=tok1a, key="leg-K3")
ok("donor batch 201", r.status_code == 201, 201, r.text[:120])
leg_batch = r.json()
r = j(d, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 0}, token=tok1a, key="leg-K4")
ok("donor failed key 422", r.status_code == 422, 422, r.status_code)
snap = j(d, "GET", "/_test/export")
ok("donor export 200", snap.status_code == 200, 200, snap.status_code)
export1 = snap.json()
j(d, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_1",
                               "starts_at_local": "2027-06-17T21:00", "party_size": 2}, token=tok2, key="post-snap")
ok("post-snapshot write exists only in live state (3 total now)",
   len(j(d, "GET", "/reservations", token=tok1a).json()["reservations"]) == 2
   and len(j(d, "GET", "/reservations", token=tok2).json()["reservations"]) == 1, "2+1", "")
print(f"donor state: refs {ref1},{ref2}; receipt keys leg-K1..K4 (tokens masked {mask(tok1a)})")

print("== phase 2: import donor export into stage-2 destination 1 (replacement)")
t = cl(DEST1)
assert j(t, "POST", "/_test/reset", {
    "users": [{"id": "u_old", "email": "old@x.com", "password": "password99", "display_name": "Old"}],
    "restaurants": FIX["restaurants"], "reservations": []}).status_code == 204
tok_old = j(t, "POST", "/auth/login", {"email": "old@x.com", "password": "password99"}).json()["token"]
ok("dest prestate token works", j(t, "GET", "/reservations", token=tok_old).status_code == 200, 200, "")
r = j(t, "POST", "/_test/import", export1)
ok("import 204", r.status_code == 204, 204, f"{r.status_code} {r.text[:120]}")
ok("import replaced destination credentials", j(t, "GET", "/reservations", token=tok_old).status_code == 401, 401, "")
ok("old password works", j(t, "POST", "/auth/login", {"email": "up1@example.com", "password": "password11"}).status_code == 200, 200, "")
ok("token A survives", j(t, "GET", "/reservations", token=tok1a).status_code == 200, 200, "")
ok("token B survives", j(t, "GET", "/reservations", token=tok1b).status_code == 200, 200, "")
ok("two bookings after import", len(j(t, "GET", "/reservations", token=tok1a).json()["reservations"]) == 2, 2, "")
r = j(t, "GET", f"/reservations/{ref1}", token=tok1a)
ok("retained ref lookup 200", r.status_code == 200, 200, r.text[:120])
now = r.json()
ok("ordinary read carries table_ids", now.get("table_ids") == ["t_2"], ["t_2"], now.get("table_ids"))
ok("ordinary read singleton table_id", now.get("table_id") == "t_2", "t_2", now.get("table_id"))
ok("amended state migrated", now["starts_at_local"] == "2027-06-17T20:30", "", now["starts_at_local"])
r = j(t, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 3}, token=tok1a, key="leg-K1")
ok("legacy receipt replay 200", r.status_code == 200, 200, r.text[:200])
ok("legacy receipt EXACT original JSON (no table_ids)", r.json() == leg_create, "original", r.text[:200])
ok("original receipt has no table_ids", "table_ids" not in r.json(), "absent", list(r.json()))
r = j(t, "POST", "/reservation-moves", {"moves": [{"reference": ref1, "starts_at_local": "2027-06-17T20:30"}]},
      token=tok1a, key="leg-K3")
ok("legacy batch receipt replay 200", r.status_code == 200, 200, r.text[:150])
ok("legacy batch receipt EXACT original", r.json() == leg_batch, "original", r.text[:150])
r = j(t, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 2}, token=tok1a, key="leg-K4")
ok("failed key reusable after import", r.status_code == 201, 201, f"{r.status_code} {r.text[:120]}")
ref3 = r.json()["reference"] if r.status_code == 201 else ""
r = j(t, "PATCH", f"/reservations/{ref1}", {"party_size": 4}, token=tok1a)
ok("amend imported booking 200", r.status_code == 200, 200, r.text[:120])
r = j(t, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T19:00", "party_size": 3}, token=tok1a, key="leg-K1")
ok("legacy receipt STILL original after dest mutation", r.status_code == 200 and r.json() == leg_create, 200, r.text[:200])
snap2 = j(t, "GET", "/_test/export")
ok("dest export 200", snap2.status_code == 200, 200, snap2.status_code)
ok("repeat import 204", j(t, "POST", "/_test/import", snap2.json()).status_code == 204, 204, "")
ok("repeat import no dup", len(j(t, "GET", "/reservations", token=tok1a).json()["reservations"]) == 3, 3, "")
assert j(t, "POST", "/_test/reset", FIX).status_code == 204
ok("reset clears imported tokens", j(t, "GET", "/reservations", token=tok1a).status_code == 401, 401, "")

print("== phase 3: modern pair/mixed receipts stage-2 -> stage-2 (distinct processes)")
s = cl(DEST2)
assert j(s, "POST", "/_test/reset", FIX2).status_code == 204
pta = j(s, "POST", "/auth/signup", {"email": "mp@example.com", "password": "password33", "display_name": "MP"}).json()["token"]
r = j(s, "POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"],
                                   "starts_at_local": "2027-06-17T18:00", "party_size": 6}, token=pta, key="mod-K5")
ok("pair create 201", r.status_code == 201, 201, r.text[:150])
mod_pair = r.json()
pair_ref = mod_pair["reference"]
r = j(s, "POST", "/reservations", {"restaurant_id": "r_anker", "table_id": "t_2",
                                   "starts_at_local": "2027-06-17T20:30", "party_size": 2}, token=pta, key="mod-K6")
single_ref = r.json()["reference"]
r = j(s, "POST", "/reservation-moves", {"moves": [{"reference": pair_ref, "starts_at_local": "2027-06-17T19:00"},
                                                  {"reference": single_ref, "party_size": 4}]}, token=pta, key="mod-K7")
ok("mixed batch 201", r.status_code == 201, 201, r.text[:200])
mod_batch = r.json()
r = j(s, "POST", f"/reservations/{single_ref}/cancel", token=pta)
ok("cancel one 200", r.status_code == 200, 200, r.status_code)
snap3 = j(s, "GET", "/_test/export")
assert snap3.status_code == 200
r = j(t, "POST", "/_test/import", snap3.json())
ok("pair export into other process 204", r.status_code == 204, 204, r.text[:120])
ok("sessions survive", j(t, "GET", "/reservations", token=pta).status_code == 200, 200, "")
r = j(t, "POST", "/reservations", {"restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"],
                                   "starts_at_local": "2027-06-17T18:00", "party_size": 6}, token=pta, key="mod-K5")
ok("pair receipt replay EXACT", r.status_code == 200 and r.json() == mod_pair, 200, r.text[:250])
r = j(t, "POST", "/reservation-moves", {"moves": [{"reference": pair_ref, "starts_at_local": "2027-06-17T19:00"},
                                                  {"reference": single_ref, "party_size": 4}]}, token=pta, key="mod-K7")
ok("mixed batch receipt replay EXACT", r.status_code == 200 and r.json() == mod_batch, 200, r.text[:250])
recs = j(t, "GET", "/reservations", token=pta).json()["reservations"]
byst = {x["reference"]: x["status"] for x in recs}
ok("cancelled status migrated", byst.get(single_ref) == "cancelled", "cancelled", byst)
ok("pair record intact", any(x["reference"] == pair_ref and x["table_ids"] == ["t_1", "t_2"]
                             and "table_id" not in x for x in recs), "", recs)

print(f"\nRESULT pass={PASS} fail={FAIL}")
for f in FAILS:
    print(" -", f)
sys.exit(1 if FAIL else 0)
