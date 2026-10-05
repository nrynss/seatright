"""Refutation probe for round-2 critic claims: post-settle reveal measurements.
Every measurement is taken >=900ms after the action (app reveal = tick + 300ms + smooth)."""
import asyncio, json, sys
import httpx
from playwright.async_api import async_playwright

BASE = "http://127.0.0.1:9401"
FIX = {
    "users": [{"id": "u_diner", "email": "diner@example.com", "password": "correct horse", "display_name": "Dana Diner"}],
    "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
                     "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
                     "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
                     "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
                     "tables": [{"id": "t_1", "label": "Window alcove", "capacity": 2},
                                {"id": "t_2", "label": "Garden corner", "capacity": 4},
                                {"id": "t_3", "label": "Hearth booth", "capacity": 4}]}],
    "reservations": []}
P = 0
F = 0
def ok(n, c, e=""):
    global P, F
    if c: P += 1; print(f"  ok {n}")
    else: F += 1; print(f"  FAIL {n} :: {e}")

async def main():
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])
        # 1) dense grids: post-settle reveal shows caption + actionable cells (party 2: singles at top)
        for slot_min in (15, 5):
            fx = json.loads(json.dumps(FIX)); fx["restaurants"][0]["slot_minutes"] = slot_min
            httpx.post(f"{BASE}/_test/reset", json=fx, timeout=10)
            ctx = await browser.new_context(viewport={"width": 1280, "height": 900})
            page = await ctx.new_page()
            await page.goto(f"{BASE}/login")
            await page.fill('[data-testid="login-email"]', "diner@example.com")
            await page.fill('[data-testid="login-password"]', "correct horse")
            await page.click('[data-testid="login-submit"]')
            await page.wait_for_selector('[data-testid="current-user"]')
            await page.goto(BASE)
            await page.fill('[data-testid="date-input"]', "2027-06-17")
            await page.fill('[data-testid="party-size-input"]', "2")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="availability-grid"]')
            await page.wait_for_timeout(1200)  # settle: reveal complete
            m = await page.evaluate("""() => {
              const cap = document.querySelector('.grid-caption').getBoundingClientRect();
              const cells = [...document.querySelectorAll('[data-testid^="slot-"][data-available="true"]')]
                .map(e => e.getBoundingClientRect()).filter(r => r.top < window.innerHeight && r.bottom > 0);
              return {capTop: Math.round(cap.top), capIn: cap.top >= 0 && cap.top < window.innerHeight,
                      availVisible: cells.length};
            }""")
            ok(f"dense slot{slot_min}: caption in view after reveal", m["capIn"], json.dumps(m))
            ok(f"dense slot{slot_min}: actionable available cells visible after reveal", m["availVisible"] >= 1, json.dumps(m))
            await page.screenshot(path="/shots/refute-dense-%s.png" % slot_min)
            await ctx.close()
        # 2) F3F4 form after settle: fully visible incl. submit, desktop light/dark + phone light/dark
        for w, h, theme, tag in ((1280, 900, "light", "d-light"), (1280, 900, "dark", "d-dark"),
                                 (375, 900, "light", "p-light"), (375, 900, "dark", "p-dark")):
            httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
            ctx = await browser.new_context(viewport={"width": w, "height": h})
            page = await ctx.new_page()
            await page.goto(BASE)
            await page.click(f'[data-testid="theme-{theme}"]')
            await page.goto(f"{BASE}/login")
            await page.fill('[data-testid="login-email"]', "diner@example.com")
            await page.fill('[data-testid="login-password"]', "correct horse")
            await page.click('[data-testid="login-submit"]')
            await page.wait_for_selector('[data-testid="current-user"]')
            await page.goto(BASE)
            await page.fill('[data-testid="date-input"]', "2027-06-17")
            await page.fill('[data-testid="party-size-input"]', "6")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="availability-grid"]')
            await page.click('[data-testid="slot-t_1+t_2-18:00"]')
            await page.wait_for_selector('[data-testid="booking-form"]')
            await page.wait_for_timeout(1200)  # settle: reveal complete
            m = await page.evaluate("""() => {
              const f = document.querySelector('[data-testid="booking-form"]').getBoundingClientRect();
              const sub = document.querySelector('[data-testid="booking-submit"]').getBoundingClientRect();
              return {form: {top: Math.round(f.top), bottom: Math.round(f.bottom), vh: window.innerHeight},
                      submitFullyVisible: sub.top >= 0 && sub.bottom <= window.innerHeight};
            }""")
            ok(f"form fully visible after settle ({tag})",
               m["form"]["top"] >= 0 and m["form"]["bottom"] <= m["form"]["vh"], json.dumps(m))
            ok(f"submit fully visible after settle ({tag})", m["submitFullyVisible"], json.dumps(m))
            await page.screenshot(path=f"/shots/refute-form-{tag}.png")
            await ctx.close()
        await browser.close()
    print(f"REFUTE RESULT pass={P} fail={F}")
    sys.exit(1 if F else 0)

asyncio.run(main())
