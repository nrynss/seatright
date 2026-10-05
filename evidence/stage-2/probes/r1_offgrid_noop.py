"""Isolated check: no-op PATCH/move on an off-grid seeded reservation.
Spec: omitted fields retain current values; no-op retains; seeds may be off-grid
(producer import compatibility). Expect 200/201 retention, not 422 not_on_slot_grid."""
import httpx, datetime as dt, zoneinfo, sys
B = sys.argv[1].rstrip("/")
c = httpx.Client(base_url=B, timeout=10)
berlin = zoneinfo.ZoneInfo("Europe/Berlin")
off = (dt.datetime.now(berlin) + dt.timedelta(minutes=180)).strftime("%Y-%m-%dT%H:%M")
fx = {"users": [{"id": "u1", "email": "a@x.com", "password": "password8", "display_name": "A"}],
      "restaurants": [{"id": "r1", "name": "R", "timezone": "Europe/Berlin", "slot_minutes": 30,
                       "reservation_duration_minutes": 60, "cancellation_cutoff_minutes": 60,
                       "opening_hours": [{"weekday": w, "opens": "00:00", "closes": "23:59"} for w in
                                         ("mon", "tue", "wed", "thu", "fri", "sat", "sun")],
                       "tables": [{"id": "t1", "label": "1", "capacity": 4}]}],
      "reservations": [{"id": "s1", "reference": "OFFGRD1", "user_id": "u1", "restaurant_id": "r1",
                        "table_id": "t1", "starts_at_local": off, "party_size": 2, "status": "confirmed"}]}
print("seed start (off-grid):", off)
r = c.post("/_test/reset", json=fx); print("reset", r.status_code)
tok = c.post("/auth/login", json={"email": "a@x.com", "password": "password8"}).json()["token"]
H = {"Authorization": "Bearer " + tok}
r = c.patch(f"/reservations/OFFGRD1", json={}, headers=H)
print("PATCH {} ->", r.status_code, r.text[:160])
r = c.post("/reservation-moves", json={"moves": [{"reference": "OFFGRD1"}]},
           headers={**H, "Idempotency-Key": "noop1"})
print("MOVE {} ->", r.status_code, r.text[:160])
r = c.get("/availability?restaurant_id=r1&date=" + off[:10] + "&party_size=2")
slots = {s["starts_at_local"][11:]: s["available_table_ids"] for s in r.json()["slots"]}
print("occupancy held:", slots)

print("--- real amendment on off-grid seeded booking ---")
r = c.patch("/reservations/OFFGRD1", json={"party_size": 3}, headers=H)
print("PATCH party-only ->", r.status_code, r.text[:160])
r = c.patch("/reservations/OFFGRD1", json={"table_id": "t1"}, headers=H)
print("PATCH table-only(same) ->", r.status_code, r.text[:160])
r = c.post("/reservations/OFFGRD1/cancel", headers=H)
print("CANCEL ->", r.status_code, r.text[:160])
