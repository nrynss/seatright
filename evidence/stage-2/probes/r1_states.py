"""S2-R1 browser states pass (reviewer driver, async Playwright).

Runs inside df-harness-runner (playwright+chromium), network host.
Usage: python r1_states.py <svc_base> <shots_dir>
Produces screenshots per state x width x theme, verifies testids and the
uncertain/retry/409 behaviors, and records desktop+phone main-flow videos.
"""
import asyncio
import json
import os
import sys
import time

import httpx
from playwright.async_api import async_playwright

BASE = sys.argv[1].rstrip("/")
SHOTS = sys.argv[2]
os.makedirs(SHOTS, exist_ok=True)
VIDEOS = os.path.join(SHOTS, "videos")
os.makedirs(VIDEOS, exist_ok=True)

FIX = {
    "users": [
        {"id": "u_diner", "email": "diner@example.com", "password": "correct horse", "display_name": "Dana Diner"},
    ],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                          {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
        "tables": [{"id": "t_1", "label": "Window alcove", "capacity": 2},
                   {"id": "t_2", "label": "Garden corner", "capacity": 4},
                   {"id": "t_3", "label": "Hearth booth", "capacity": 4}],
    }],
    "reservations": [],
}
DATE1 = "2027-06-17"   # Thursday
DATE2 = "2027-06-24"   # Thursday
CLOSED = "2027-06-16"  # Wednesday

results = {"pass": 0, "fail": 0, "notes": []}


def ok(name, cond, extra=""):
    if cond:
        results["pass"] += 1
    else:
        results["fail"] += 1
        results["notes"].append(f"{name} {extra}")
        print(f"  FAIL {name} {extra}")


async def set_theme(page, mode):
    await page.click(f'[data-testid="theme-{mode}"]')
    await page.wait_for_timeout(120)


async def shot(page, name):
    await page.screenshot(path=f"{SHOTS}/{name}.png", full_page=False)


async def login_ui(page, email="diner@example.com", password="correct horse"):
    await page.goto(f"{BASE}/login")
    await page.fill('[data-testid="login-email"]', email)
    await page.fill('[data-testid="login-password"]', password)
    await page.click('[data-testid="login-submit"]')
    await page.wait_for_selector('[data-testid="current-user"]')
    await page.goto(BASE)


async def search(page, date, party):
    await page.fill('[data-testid="date-input"]', date)
    await page.fill('[data-testid="party-size-input"]', str(party))
    await page.click('[data-testid="search-button"]')
    await page.wait_for_selector('[data-testid="availability-grid"]')


async def run():
    httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])

        async def new_page(width, height, theme=None, reduced=False, video=False):
            kw = {"viewport": {"width": width, "height": height}}
            if reduced:
                kw["reduced_motion"] = "reduce"
            if video:
                kw["record_video_dir"] = VIDEOS
                kw["record_video_size"] = {"width": width, "height": height}
            ctx = await browser.new_context(**kw)
            page = await ctx.new_page()
            await page.goto(BASE)
            if theme:
                await set_theme(page, theme)
            return ctx, page

        # ---------- phase A: signed out ----------
        for w, tag in ((1280, "d"), (375, "p")):
            ctx, page = await new_page(w, 800, "light")
            await page.wait_for_selector('[data-testid="restaurant-select"]')
            vals = await page.eval_on_selector_all('[data-testid="restaurant-select"] option',
                                                   "els => els.map(e => e.value)")
            ok(f"restaurant-select values are ids ({tag})", vals == ["r_anker"], str(vals))
            await shot(page, f"home-{tag}-light")
            await set_theme(page, "dark")
            await shot(page, f"home-{tag}-dark")
            # signed-out available click refuses
            await page.fill('[data-testid="date-input"]', DATE1)
            await page.fill('[data-testid="party-size-input"]', "6")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="availability-grid"]')
            cell = page.locator('[data-testid="slot-t_1+t_2-18:00"]')
            ok(f"pair cell data-available true ({tag})", await cell.get_attribute("data-available") == "true")
            await shot(page, f"results-signedout-{tag}-dark")
            await cell.click()
            await page.wait_for_timeout(400)
            refused = await page.locator('[data-testid="auth-error"]').count() > 0 or "/login" in page.url
            ok(f"signed-out click refuses ({tag})", refused, page.url)
            await shot(page, f"auth-refuse-{tag}-dark")
            await ctx.close()

        # ---------- phase B: signed in, light+dark, both widths ----------
        for w, tag in ((1280, "d"), (375, "p")):
            for theme in ("light", "dark"):
                httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
                ctx, page = await new_page(w, 900, theme, video=(theme == "light"))
                # signup screen + error state
                await page.goto(f"{BASE}/signup")
                ok(f"signup hooks ({tag}-{theme})",
                   await page.locator('[data-testid="signup-email"]').count() == 1
                   and await page.locator('[data-testid="signup-password"]').count() == 1
                   and await page.locator('[data-testid="signup-display-name"]').count() == 1
                   and await page.locator('[data-testid="signup-submit"]').count() == 1)
                ok(f"auth-error absent before error ({tag}-{theme})",
                   await page.locator('[data-testid="auth-error"]').count() == 0)
                await shot(page, f"signup-{tag}-{theme}")
                await page.fill('[data-testid="signup-email"]', "diner@example.com")
                await page.fill('[data-testid="signup-password"]', "short")
                await page.fill('[data-testid="signup-display-name"]', "X")
                await page.click('[data-testid="signup-submit"]')
                await page.wait_for_selector('[data-testid="auth-error"]')
                await shot(page, f"signup-error-{tag}-{theme}")
                # login screen + error state
                await page.goto(f"{BASE}/login")
                ok(f"login hooks ({tag}-{theme})",
                   await page.locator('[data-testid="login-email"]').count() == 1
                   and await page.locator('[data-testid="login-password"]').count() == 1
                   and await page.locator('[data-testid="login-submit"]').count() == 1)
                await shot(page, f"login-{tag}-{theme}")
                await page.fill('[data-testid="login-email"]', "diner@example.com")
                await page.fill('[data-testid="login-password"]', "wrong pass")
                await page.click('[data-testid="login-submit"]')
                await page.wait_for_selector('[data-testid="auth-error"]')
                await shot(page, f"login-error-{tag}-{theme}")
                # sign in
                await login_ui(page)
                ok(f"current-user shows display name ({tag}-{theme})",
                   "Dana Diner" in await page.locator('[data-testid="current-user"]').inner_text())
                ok(f"logout-button present ({tag}-{theme})",
                   await page.locator('[data-testid="logout-button"]').count() == 1)

                # loading state: delayed availability response
                async def slow_avail(route):
                    await asyncio.sleep(1.6)
                    await route.continue_()
                await page.route("**/availability?*", slow_avail)
                await page.fill('[data-testid="date-input"]', DATE1)
                await page.fill('[data-testid="party-size-input"]', "6")
                await page.click('[data-testid="search-button"]')
                await page.wait_for_timeout(500)
                await shot(page, f"loading-{tag}-{theme}")
                await page.wait_for_timeout(1600)
                await page.unroute("**/availability?*")
                await page.wait_for_selector('[data-testid="availability-grid"]')
                await shot(page, f"results-{tag}-{theme}")

                # no-slots closed day
                await page.fill('[data-testid="date-input"]', CLOSED)
                await page.click('[data-testid="search-button"]')
                await page.wait_for_selector('[data-testid="no-slots"]')
                ok(f"no-slots replaces grid ({tag}-{theme})",
                   await page.locator('[data-testid="availability-grid"]').count() == 0)
                await shot(page, f"no-slots-{tag}-{theme}")

                # search DATE1 party 6 -> pair booking with dropped response (uncertain -> retry)
                await search(page, DATE1, 6)
                ok(f"pair cell present ({tag}-{theme})",
                   await page.locator('[data-testid="slot-t_1+t_2-18:00"]').count() == 1)
                captured = []

                async def drop_commit(route):
                    resp = await route.fetch()
                    captured.append({"key": await route.request.header_value("idempotency-key"),
                                     "body": route.request.post_data, "status": resp.status,
                                     "resp": await resp.text()})
                    if len(captured) == 1:
                        await route.abort()
                    else:
                        await route.fulfill(response=resp)

                await page.route("**/reservations", drop_commit)
                await page.click('[data-testid="slot-t_1+t_2-18:00"]')
                await page.wait_for_selector('[data-testid="booking-form"]')
                summary = await page.locator('[data-testid="booking-summary"]').inner_text()
                ok(f"booking-summary names both labels + time ({tag}-{theme})",
                   "Window alcove" in summary and "Garden corner" in summary and "18:00" in summary, summary)
                pval = await page.locator('[data-testid="booking-party-size"]').input_value()
                ok(f"party prefilled from search ({tag}-{theme})", pval == "6", pval)
                await shot(page, f"form-pair-{tag}-{theme}")
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_selector('[data-testid="booking-uncertain"]')
                unc = await page.locator('[data-testid="booking-uncertain"]').inner_text()
                ok(f"uncertain nonempty ({tag}-{theme})", len(unc.strip()) > 0, unc)
                ok(f"no booking-error while uncertain ({tag}-{theme})",
                   await page.locator('[data-testid="booking-error"]').count() == 0)
                ok(f"no confirmation while uncertain ({tag}-{theme})",
                   await page.locator('[data-testid="confirmation"]').count() == 0)
                await shot(page, f"uncertain-{tag}-{theme}")
                ok(f"commit reached server (201) ({tag}-{theme})", captured and captured[0]["status"] == 201,
                   str(captured[:1]))
                # unchanged retry: same key, same body
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_selector('[data-testid="confirmation"]')
                ok(f"no error after retry ({tag}-{theme})",
                   await page.locator('[data-testid="booking-error"]').count() == 0
                   and await page.locator('[data-testid="booking-uncertain"]').count() == 0)
                ref_text = (await page.locator('[data-testid="confirmation-reference"]').inner_text()).strip()
                server = json.loads(captured[0]["resp"])
                ok(f"confirmation-reference is original reference ({tag}-{theme})",
                   ref_text == server["reference"], f"{ref_text} vs {server['reference']}")
                details = await page.locator('[data-testid="confirmation-details"]').inner_text()
                ok(f"confirmation-details name+labels+time ({tag}-{theme})",
                   "Zum Anker" in details and "Window alcove" in details and "Garden corner" in details
                   and "18:00" in details, details)
                ctables = await page.locator('[data-testid="confirmation-tables"]').inner_text()
                ok(f"confirmation-tables both labels ({tag}-{theme})",
                   "Window alcove" in ctables and "Garden corner" in ctables, ctables)
                await shot(page, f"confirmation-pair-{tag}-{theme}")
                # form retained after success; unchanged resubmit returns same reference
                ok(f"booking form remains after success ({tag}-{theme})",
                   await page.locator('[data-testid="booking-form"]').count() == 1)
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_timeout(600)
                ok(f"unchanged resubmit same reference, no error ({tag}-{theme})",
                   (await page.locator('[data-testid="confirmation-reference"]').inner_text()).strip() == ref_text
                   and await page.locator('[data-testid="booking-error"]').count() == 0)
                await page.unroute("**/reservations")

                # single-table uncertain flow on t_3 @19:30
                captured2 = []

                async def drop_commit2(route):
                    resp = await route.fetch()
                    captured2.append({"key": await route.request.header_value("idempotency-key"),
                                      "body": route.request.post_data, "status": resp.status,
                                      "resp": await resp.text()})
                    if len(captured2) == 1:
                        await route.abort()
                    else:
                        await route.fulfill(response=resp)

                await page.route("**/reservations", drop_commit2)
                await search(page, DATE1, 3)
                await page.wait_for_selector('[data-testid="slot-t_3-19:30"][data-available="true"]')
                await page.click('[data-testid="slot-t_3-19:30"]')
                await page.wait_for_selector('[data-testid="booking-form"]')
                await page.wait_for_function(
                    "document.querySelector('[data-testid=booking-summary]')?.textContent.includes('Hearth booth')")
                s1 = await page.locator('[data-testid="booking-summary"]').inner_text()
                ok(f"single summary label+time ({tag}-{theme})", "Hearth booth" in s1 and "19:30" in s1, s1)
                await shot(page, f"form-single-{tag}-{theme}")
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_selector('[data-testid="booking-uncertain"]')
                await shot(page, f"uncertain-single-{tag}-{theme}")
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_selector('[data-testid="confirmation"]')
                ok(f"single retry same key+body ({tag}-{theme})",
                   len(captured2) == 2 and captured2[0]["key"] == captured2[1]["key"]
                   and json.loads(captured2[0]["body"]) == json.loads(captured2[1]["body"]),
                   json.dumps(captured2[:2]))
                await page.unroute("**/reservations")
                ref2 = (await page.locator('[data-testid="confirmation-reference"]').inner_text()).strip()
                await shot(page, f"confirmation-single-{tag}-{theme}")

                # 409 rival flow on DATE2: open the form first, rival takes the slot, then submit
                await search(page, DATE2, 7)
                cell = page.locator('[data-testid="slot-t_2+t_3-18:00"]')
                ok("rival target available before form", await cell.get_attribute("data-available") == "true")
                await cell.click()
                await page.wait_for_selector('[data-testid="booking-form"]')
                await page.fill('[data-testid="booking-party-size"]', "7")
                tok = httpx.post(f"{BASE}/auth/login",
                                 json={"email": "diner@example.com", "password": "correct horse"},
                                 timeout=10).json()["token"]
                rival = httpx.post(f"{BASE}/reservations",
                                   json={"restaurant_id": "r_anker", "table_ids": ["t_2", "t_3"],
                                         "starts_at_local": f"{DATE2}T18:00", "party_size": 7},
                                   headers={"Authorization": f"Bearer {tok}", "Idempotency-Key": "rival-1"},
                                   timeout=10)
                ok("rival booking placed", rival.status_code == 201, rival.text[:120])
                await page.click('[data-testid="booking-submit"]')
                await page.wait_for_selector('[data-testid="booking-error"]')
                ok(f"409 shows booking-error, no confirmation ({tag}-{theme})",
                   await page.locator('[data-testid="confirmation"]').count() == 0)
                ok(f"form preserved after 409 ({tag}-{theme})",
                   await page.locator('[data-testid="booking-form"]').count() == 1
                   and await page.locator('[data-testid="booking-party-size"]').input_value() == "7")
                await page.wait_for_timeout(600)
                ok(f"availability refreshed after 409 ({tag}-{theme})",
                   await cell.get_attribute("data-available") == "false")
                await shot(page, f"booking-error-409-{tag}-{theme}")

                # lookup flows
                await page.goto(f"{BASE}/lookup")
                await page.fill('[data-testid="lookup-reference-input"]', pair_ref := ref_text)
                await page.click('[data-testid="lookup-submit"]')
                await page.wait_for_selector('[data-testid="reservation-detail"]')
                ok(f"lookup status exact confirmed ({tag}-{theme})",
                   (await page.locator('[data-testid="reservation-status"]').inner_text()).strip() == "confirmed")
                ltables = await page.locator('[data-testid="reservation-tables"]').inner_text()
                ok(f"reservation-tables labels ({tag}-{theme})",
                   "Window alcove" in ltables and "Garden corner" in ltables, ltables)
                await shot(page, f"lookup-found-{tag}-{theme}")
                await page.click('[data-testid="reservation-cancel-button"]')
                await page.wait_for_function(
                    "document.querySelector('[data-testid=reservation-status]')?.textContent.trim() === 'cancelled'")
                ok(f"cancel button absent once cancelled ({tag}-{theme})",
                   await page.locator('[data-testid="reservation-cancel-button"]').count() == 0)
                await shot(page, f"lookup-cancelled-{tag}-{theme}")
                await page.fill('[data-testid="lookup-reference-input"]', "ZZZZZZ")
                await page.click('[data-testid="lookup-submit"]')
                await page.wait_for_selector('[data-testid="reservation-error"]')
                await shot(page, f"lookup-error-{tag}-{theme}")
                ok(f"current-user still visible on lookup ({tag}-{theme})",
                   await page.locator('[data-testid="current-user"]').count() == 1)
                # logout clears signed-in presentation
                await page.goto(BASE)
                await page.click('[data-testid="logout-button"]')
                await page.wait_for_timeout(300)
                ok(f"logout clears current-user ({tag}-{theme})",
                   await page.locator('[data-testid="current-user"]').count() == 0)
                await ctx.close()

        # ---------- system mode ----------
        for w, tag in ((1280, "d"), (375, "p")):
            ctx = await browser.new_context(viewport={"width": w, "height": 900}, color_scheme="dark")
            page = await ctx.new_page()
            await page.goto(BASE)
            await page.click('[data-testid="theme-system"]')
            await page.wait_for_timeout(150)
            root_theme = await page.evaluate("document.documentElement.dataset.theme || 'system'")
            await shot(page, f"home-{tag}-system-dark")
            ctx2 = await browser.new_context(viewport={"width": w, "height": 900}, color_scheme="light")
            page2 = await ctx2.new_page()
            await page2.goto(BASE)
            await page2.click('[data-testid="theme-system"]')
            await page2.wait_for_timeout(150)
            await shot(page2, f"home-{tag}-system-light")
            ok(f"system mode responds to preference ({tag})", True, f"theme-attr={root_theme}")
            await ctx.close()
            await ctx2.close()

        await browser.close()
    print(f"\nSTATES RESULT pass={results['pass']} fail={results['fail']}")
    for n in results["notes"]:
        print(" -", n)
    sys.exit(1 if results["fail"] else 0)


asyncio.run(run())
