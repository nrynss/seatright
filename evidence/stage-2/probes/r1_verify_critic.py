import asyncio, json, sys
import httpx
from playwright.async_api import async_playwright

BASE = "http://127.0.0.1:9401"
FIX = {
    "users": [{"id": "u_diner", "email": "diner@example.com", "password": "correct horse", "display_name": "Dana Diner"}],
    "restaurants": [{"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
                     "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
                     "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                                        {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
                     "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
                     "tables": [{"id": "t_1", "label": "Window alcove", "capacity": 2},
                                {"id": "t_2", "label": "Garden corner", "capacity": 4},
                                {"id": "t_3", "label": "Hearth booth", "capacity": 4}]}],
    "reservations": []}

async def main():
    httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])
        for w, tag in ((1280, "desktop-1280x900"), (375, "phone-375x900")):
            ctx = await browser.new_context(viewport={"width": w, "height": 900})
            page = await ctx.new_page()
            await page.goto(BASE)
            await page.goto(f"{BASE}/login")
            await page.fill('[data-testid="login-email"]', "diner@example.com")
            await page.fill('[data-testid="login-password"]', "correct horse")
            await page.click('[data-testid="login-submit"]')
            await page.wait_for_selector('[data-testid="current-user"]')
            await page.goto(BASE)
            nav = await page.eval_on_selector_all("header a, header button",
                "els => els.map(e => (e.textContent || '').trim()).filter(Boolean)")
            print(f"[{tag}] signed-in nav items:", nav)
            await page.fill('[data-testid="date-input"]', "2027-06-17")
            await page.fill('[data-testid="party-size-input"]', "6")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="availability-grid"]')
            geo = await page.evaluate("""() => {
              const g = document.querySelector('[data-testid="availability-grid"]').getBoundingClientRect();
              return {gridTop: Math.round(g.top), innerH: window.innerHeight,
                      gridVisibleWithoutScroll: g.top < window.innerHeight && g.bottom > 0};
            }""")
            print(f"[{tag}] availability grid: {geo}")
            await page.click('[data-testid="slot-t_1+t_2-18:00"]')
            await page.wait_for_selector('[data-testid="booking-form"]')
            await page.wait_for_timeout(400)
            fgeo = await page.evaluate("""() => {
              const f = document.querySelector('[data-testid="booking-form"]').getBoundingClientRect();
              return {top: Math.round(f.top), bottom: Math.round(f.bottom), innerH: window.innerHeight,
                      fullyVisible: f.top >= 0 && f.bottom <= window.innerHeight};
            }""")
            print(f"[{tag}] booking form after selection: {fgeo}")
            plan = await page.evaluate("""() => {
              const names = [...document.querySelectorAll('.plan-name')].map(e => e.textContent.trim());
              const plates = [...document.querySelectorAll('.plate-label')].map(e => e.textContent.trim());
              return {planNames: names, plateLabels: plates};
            }""")
            print(f"[{tag}] floor plan naming:", json.dumps(plan))
            pairline = await page.evaluate("""() => {
              const link = document.querySelector('.pair-link');
              const label = [...document.querySelectorAll('.plate-label')]
                .find(e => e.textContent.includes('Garden'));
              if (!link || !label) return null;
              const lr = link.getBoundingClientRect(), tr = label.getBoundingClientRect();
              const order = link.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING;
              return {linkBox: [Math.round(lr.x), Math.round(lr.y), Math.round(lr.width), Math.round(lr.height)],
                      labelBox: [Math.round(tr.x), Math.round(tr.y), Math.round(tr.width), Math.round(tr.height)],
                      intersect: !(lr.right < tr.left || tr.right < lr.left || lr.bottom < tr.top || tr.bottom < lr.top),
                      textRenderedAfterLine: order};
            }""")
            print(f"[{tag}] pair-link vs label:", json.dumps(pairline))
            # submit and check confirmation visibility
            await page.click('[data-testid="booking-submit"]')
            await page.wait_for_selector('[data-testid="confirmation"]')
            await page.wait_for_timeout(400)
            cgeo = await page.evaluate("""() => {
              const c = document.querySelector('[data-testid="confirmation"]').getBoundingClientRect();
              return {top: Math.round(c.top), bottom: Math.round(c.bottom), innerH: window.innerHeight,
                      visiblePortion: Math.max(0, Math.min(c.bottom, window.innerHeight) - Math.max(c.top, 0)),
                      height: Math.round(c.height)};
            }""")
            print(f"[{tag}] confirmation after booking: {cgeo}")
            # closed-day empty state visibility
            await page.fill('[data-testid="date-input"]', "2027-06-16")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="no-slots"]')
            ngeo = await page.evaluate("""() => {
              const n = document.querySelector('[data-testid="no-slots"]').getBoundingClientRect();
              return {top: Math.round(n.top), innerH: window.innerHeight,
                      fullyVisible: n.top >= 0 && n.bottom <= window.innerHeight};
            }""")
            print(f"[{tag}] no-slots empty state: {ngeo}")
            await ctx.close()
        await browser.close()

asyncio.run(main())
