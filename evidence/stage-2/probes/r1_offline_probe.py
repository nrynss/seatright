import httpx
import re

b = "http://s2r2-off:8080"
c = httpx.Client(base_url=b, timeout=5)
h = c.get("/health")
print("health", h.status_code, h.text)
r = c.post("/_test/reset", json={
    "users": [{"id": "u1", "email": "o@x.com", "password": "password8", "display_name": "O"}],
    "restaurants": [{"id": "r1", "name": "Off", "timezone": "UTC", "slot_minutes": 30,
                     "reservation_duration_minutes": 60, "cancellation_cutoff_minutes": 0,
                     "opening_hours": [{"weekday": "mon", "opens": "00:00", "closes": "23:59"}],
                     "tables": [{"id": "t1", "label": "1", "capacity": 2}]}],
    "reservations": []})
print("reset", r.status_code)
s = c.post("/auth/signup", json={"email": "fresh@x.com", "password": "password8", "display_name": "F"})
print("signup(fresh)", s.status_code)
l = c.post("/auth/login", json={"email": "o@x.com", "password": "password8"})
print("login(seeded)", l.status_code)
assert s.status_code == 201 and l.status_code == 200, "offline auth failed"
tok = l.json()["token"]
H = {"Authorization": "Bearer " + tok}
a = c.get("/availability?restaurant_id=r1&date=2027-01-04&party_size=2")
print("availability", a.status_code, len(a.json()["slots"]), "slots")
bo = c.post("/reservations", json={"restaurant_id": "r1", "table_id": "t1",
                                   "starts_at_local": "2027-01-04T19:00", "party_size": 2},
            headers={"Idempotency-Key": "off1", **H})
print("booking", bo.status_code)
home = c.get("/")
print("index", home.status_code, home.headers.get("content-type"), len(home.content), "bytes")
assets = sorted(set(re.findall(r'(?:href|src)="(/[^"]+)"', home.text)))
print("asset links:", assets)
fonts = set()
for p in assets:
    ar = c.get(p)
    ct = ar.headers.get("content-type") or ""
    print("asset", p, ar.status_code, ct, len(ar.content), "bytes")
    if "css" in ct:
        for m in re.finditer(r'url\((["\']?)/?([^)"\']+\.woff2?)["\']?\)', ar.text):
            fonts.add("/" + m.group(2).lstrip("./"))
        for m in re.finditer(r'href="(/[^"]+\.css)"', home.text):
            pass
for f in sorted(fonts)[:6]:
    fr = c.get(f)
    print("font", f, fr.status_code, len(fr.content), "bytes (binary not dumped)")
print("routes", [c.get(p).status_code for p in ("/signup", "/login", "/lookup")])
try:
    c.get("https://example.com", timeout=3)
    print("OUTBOUND UNEXPECTEDLY OK")
except Exception as e:
    print("outbound blocked:", type(e).__name__)
