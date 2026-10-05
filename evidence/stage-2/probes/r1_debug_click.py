import asyncio, json, sys
import httpx
from playwright.async_api import async_playwright

BASE = "http://127.0.0.1:9401"
FIX = json.load(open("/probes/fixture.json")) if False else None
FIX = {
    "users": [{"id": "u_diner", "email": "diner@example.com", "password": "correct horse", "display_name": "Dana Diner"}],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
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
        page = await (await browser.new_context(viewport={"width": 1280, "height": 900})).new_page()
        await page.goto(BASE)
        await page.click('[data-testid="theme-light"]')
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
        await page.click('[data-testid="booking-submit"]')
        await page.wait_for_selector('[data-testid="confirmation"]')
        print("after pair confirmation, summary:", await page.locator('[data-testid="booking-summary"]').inner_text())
        cell = page.locator('[data-testid="slot-t_3-19:30"]')
        print("t_3@19:30 data-available:", await cell.get_attribute("data-available"))
        box = await cell.bounding_box()
        print("cell box:", box)
        probe = await page.evaluate("""() => {
          const el = document.querySelector('[data-testid="slot-t_3-19:30"]');
          const r = el.getBoundingClientRect();
          const top = document.elementFromPoint(r.x + r.width/2, r.y + r.height/2);
          return {hit: top && (top.dataset.testid || top.className || top.tagName),
                  covered: top && !el.contains(top) && top !== el};
        }""")
        print("elementFromPoint at cell center:", probe)
        await cell.click()
        await page.wait_for_timeout(1500)
        print("after click, summary:", await page.locator('[data-testid="booking-summary"]').inner_text())
        print("selected cells:", await page.eval_on_selector_all(
            '[data-selected="true"]', "els => els.map(e => e.dataset.testid)"))
        await page.screenshot(path="/shots/debug-after-second-click.png")
        await browser.close()

asyncio.run(main())
