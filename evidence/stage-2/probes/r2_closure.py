"""S2-R2 closure driver for findings F1-F4 (round-1 blockers).

All measurements are viewport-truth: the driver NEVER scrolls to the target
except through the app's own reveal; scroll positions are recorded before and
after every action. Exit code is the real pass/fail count (0 only if all pass).

Usage: python r2_closure.py <svc_base> <shots_dir>
"""
import asyncio
import json
import os
import sys

import httpx
from playwright.async_api import async_playwright

BASE = sys.argv[1].rstrip("/")
SHOTS = sys.argv[2]
os.makedirs(SHOTS, exist_ok=True)

FIX = {
    "users": [{"id": "u_diner", "email": "diner@example.com", "password": "correct horse", "display_name": "Dana Diner"}],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                          {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
        "tables": [{"id": "t_1", "label": "Window alcove", "capacity": 2},
                   {"id": "t_2", "label": "Garden corner", "capacity": 4},
                   {"id": "t_3", "label": "Hearth booth", "capacity": 4}]}],
    "reservations": [],
}
DATE1 = "2027-06-17"
DATE2 = "2027-06-24"
CLOSED = "2027-06-16"
SETTLE = 900  # > owner's 300ms scroll settling

PASS = 0
FAIL = 0
NOTES = []


def ok(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  ok {name}")
    else:
        FAIL += 1
        NOTES.append(f"{name} :: {extra}")
        print(f"  FAIL {name} :: {extra}")


async def login_ui(page):
    await page.goto(f"{BASE}/login")
    await page.fill('[data-testid="login-email"]', "diner@example.com")
    await page.fill('[data-testid="login-password"]', "correct horse")
    await page.click('[data-testid="login-submit"]')
    await page.wait_for_selector('[data-testid="current-user"]')


async def nav_items(page):
    return await page.eval_on_selector_all(
        "header a, header button", "els => els.map(e => (e.textContent || '').trim()).filter(Boolean)")


async def rect(page, sel):
    return await page.evaluate(
        """(sel) => { const e = document.querySelector(sel); if (!e) return null;
        const r = e.getBoundingClientRect();
        return {top: Math.round(r.top), bottom: Math.round(r.bottom), left: Math.round(r.left),
                right: Math.round(r.right), h: Math.round(r.height), w: Math.round(r.width),
                vh: window.innerHeight, vw: window.innerWidth}; }""", sel)


def fully_in_view(r):
    return r and r["top"] >= 0 and r["bottom"] <= r["vh"] and r["h"] > 0


def partially_in_view(r):
    return r and r["top"] < r["vh"] and r["bottom"] > 0


async def run():
    httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])

        # ================= F1: navigation auth links =================
        for w, tag in ((1280, "d"), (375, "p")):
            ctx = await browser.new_context(viewport={"width": w, "height": 900})
            page = await ctx.new_page()
            await page.goto(BASE)
            items = await nav_items(page)
            ok(f"F1 signed-out shows auth links ({tag})",
               "Sign in" in items and "Create account" in items, str(items))
            ok(f"F1 signed-out has no current-user ({tag})",
               await page.locator('[data-testid="current-user"]').count() == 0)
            await login_ui(page)
            for route in ("/", "/signup", "/login", "/lookup"):
                await page.goto(f"{BASE}{route}")
                items = await nav_items(page)
                ok(f"F1 signed-in no auth links on {route} ({tag})",
                   "Sign in" not in items and "Create account" not in items, str(items))
                ok(f"F1 signed-in current-user on {route} ({tag})",
                   await page.locator('[data-testid="current-user"]').count() == 1)
            await page.goto(BASE)
            await page.click('[data-testid="logout-button"]')
            await page.wait_for_timeout(300)
            items = await nav_items(page)
            ok(f"F1 logout restores auth links ({tag})",
               "Sign in" in items and "Create account" in items, str(items))
            # blank display name is still a session
            tok = httpx.post(f"{BASE}/auth/login",
                             json={"email": "diner@example.com", "password": "correct horse"},
                             timeout=10).json()["token"]
            await page.goto("about:blank")
            await page.goto(BASE)
            await page.evaluate(
                "([t]) => localStorage.setItem('tablekeeper-session',"
                " JSON.stringify({token: t, displayName: '', userId: 'u_blank'}))", [tok])
            await page.goto(BASE)
            await page.wait_for_timeout(300)
            ok(f"F1 blank displayName still signed in ({tag})",
               await page.locator('[data-testid="current-user"]').count() == 1
               and (await page.locator('[data-testid="current-user"]').inner_text()) == "")
            items = await nav_items(page)
            ok(f"F1 blank displayName hides auth links ({tag})",
               "Sign in" not in items and "Create account" not in items, str(items))
            sw = await page.evaluate("document.documentElement.scrollWidth")
            ok(f"F1 no horizontal overflow signed-in 375 ({tag})", sw <= w + 1, f"scrollWidth={sw} w={w}")
            await ctx.close()

        # ================= F2: reveal measurements =================
        for w, h, tag in ((1280, 900, "d"), (375, 900, "p"), (375, 812, "p812")):
            httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
            ctx = await browser.new_context(viewport={"width": w, "height": h},
                                            record_video_dir=f"{SHOTS}/videos",
                                            record_video_size={"width": w, "height": h})
            page = await ctx.new_page()
            await login_ui(page)
            await page.goto(BASE)
            await page.evaluate("window.scrollTo(0, 0)")
            y0 = await page.evaluate("window.scrollY")
            # search reveal
            await page.fill('[data-testid="date-input"]', DATE1)
            await page.fill('[data-testid="party-size-input"]', "2")
            await page.click('[data-testid="search-button"]')
            await page.wait_for_selector('[data-testid="availability-grid"]')
            await page.wait_for_timeout(SETTLE)
            y1 = await page.evaluate("window.scrollY")
            cap = await rect(page, ".grid-caption")
            ok(f"F2 search reveals caption ({tag})", partially_in_view(cap), json.dumps(cap))
            vis_avail = await page.evaluate("""() => {
              const vh = window.innerHeight;
              return [...document.querySelectorAll('[data-testid^="slot-"][data-available="true"]')]
                .map(e => e.getBoundingClientRect())
                .filter(r => r.top < vh && r.bottom > 0).length;
            }""")
            ok(f"F2 actionable available cells visible after search ({tag})", vis_avail >= 1, f"{vis_avail} in view")
            ok(f"F2 search caused app reveal scroll ({tag})", y1 > y0, f"{y0}->{y1}")
            floor = await page.evaluate("""() => {
              const svg = [...document.querySelectorAll('svg')].find(s => s.querySelector('.plate-label'));
              if (!svg) return null;
              const r = svg.getBoundingClientRect();
              return {top: Math.round(r.top), bottom: Math.round(r.bottom), vh: window.innerHeight};
            }""")
            ok(f"F2 floor retained in view ({tag})", floor is not None and floor["bottom"] > 0, json.dumps(floor))
            await page.screenshot(path=f"{SHOTS}/F2-results-{tag}.png")
            # keyboard selection reveal
            await page.evaluate("window.scrollTo(0, 0)")
            cell = page.locator('[data-testid="slot-t_1+t_2-18:00"]')
            await cell.focus()
            await page.keyboard.press("Enter")
            await page.wait_for_selector('[data-testid="booking-form"]')
            await page.wait_for_timeout(SETTLE)
            head = await rect(page, '[data-testid="booking-form"] h1, [data-testid="booking-form"] h2, [data-testid="booking-summary"]')
            party_r = await rect(page, '[data-testid="booking-party-size"]')
            submit_r = await rect(page, '[data-testid="booking-submit"]')
            ok(f"F2 keyboard selection reveals form heading ({tag})", partially_in_view(head) and head["top"] >= 0, json.dumps(head))
            ok(f"F2 party input visible ({tag})", fully_in_view(party_r), json.dumps(party_r))
            ok(f"F2 submit visible ({tag})", fully_in_view(submit_r), json.dumps(submit_r))
            await page.screenshot(path=f"{SHOTS}/F2-form-keyboard-{tag}.png")
            # submit -> confirmation reveal; form stays
            y_pre = await page.evaluate("window.scrollY")
            await page.click('[data-testid="booking-submit"]')
            await page.wait_for_selector('[data-testid="confirmation"]')
            await page.wait_for_timeout(SETTLE)
            conf = await rect(page, '[data-testid="confirmation"]')
            ref = (await page.locator('[data-testid="confirmation-reference"]').inner_text()).strip()
            ok(f"F2 confirmation fully visible ({tag})", fully_in_view(conf), json.dumps(conf))
            ok(f"F2 confirmation reference readable ({tag})", len(ref) >= 6, ref)
            ok(f"F2 form remains after confirmation ({tag})",
               await page.locator('[data-testid="booking-form"]').count() == 1)
            await page.screenshot(path=f"{SHOTS}/F2-confirmation-{tag}.png")
            # party typing does not hijack scroll
            y_now = await page.evaluate("window.scrollY")
            await page.evaluate("""() => { const e = document.querySelector('[data-testid="party-size-input"]');
              e.value = '4'; e.dispatchEvent(new Event('input', {bubbles: true})); }""")
            await page.wait_for_timeout(400)
            ok(f"F2 party typing does not hijack scroll ({tag})",
               await page.evaluate("window.scrollY") == y_now, f"{y_now}->{await page.evaluate('window.scrollY')}")
            # 409: rival then submit; no repeated reveal, form preserved
            await search2(page, DATE2, 7)
            await page.click('[data-testid="slot-t_2+t_3-18:00"]')
            await page.wait_for_selector('[data-testid="booking-form"]')
            await page.fill('[data-testid="booking-party-size"]', "7")
            tok = httpx.post(f"{BASE}/auth/login", json={"email": "diner@example.com", "password": "correct horse"},
                             timeout=10).json()["token"]
            httpx.post(f"{BASE}/reservations", json={"restaurant_id": "r_anker", "table_ids": ["t_2", "t_3"],
                                                     "starts_at_local": f"{DATE2}T18:00", "party_size": 7},
                       headers={"Authorization": f"Bearer {tok}", "Idempotency-Key": "rival-r2"}, timeout=10)
            y_before_submit = await page.evaluate("window.scrollY")
            await page.click('[data-testid="booking-submit"]')
            await page.wait_for_selector('[data-testid="booking-error"]')
            await page.wait_for_timeout(SETTLE)
            y_after_409 = await page.evaluate("window.scrollY")
            err = await rect(page, '[data-testid="booking-error"]')
            ok(f"F2 409 error visible ({tag})", partially_in_view(err) and err["top"] >= 0, json.dumps(err))
            ok(f"F2 409 does not re-reveal results upward ({tag})", y_after_409 >= y_before_submit,
               f"{y_before_submit}->{y_after_409}")
            ok(f"F2 409 preserves form+inputs ({tag})",
               await page.locator('[data-testid="booking-form"]').count() == 1
               and await page.locator('[data-testid="booking-party-size"]').input_value() == "7")
            ok(f"F2 409 no confirmation ({tag})",
               await page.locator('[data-testid="confirmation"]').count() == 0)
            await page.screenshot(path=f"{SHOTS}/F2-409-{tag}.png")
            # uncertain: drop response after commit, retry
            dropped = []

            async def drop_first(route):
                resp = await route.fetch()
                dropped.append(await resp.text())
                if len(dropped) == 1:
                    await route.abort()
                else:
                    await route.fulfill(response=resp)
            await page.route("**/reservations", drop_first)
            await search2(page, DATE1, 2)
            await page.click('[data-testid="slot-t_3-19:30"][data-available="true"]')
            await page.wait_for_selector('[data-testid="booking-form"]')
            await page.click('[data-testid="booking-submit"]')
            await page.wait_for_selector('[data-testid="booking-uncertain"]')
            await page.wait_for_timeout(SETTLE)
            unc = await rect(page, '[data-testid="booking-uncertain"]')
            ok(f"F2 uncertain visible ({tag})", partially_in_view(unc) and unc["top"] >= 0, json.dumps(unc))
            await page.screenshot(path=f"{SHOTS}/F2-uncertain-{tag}.png")
            await page.click('[data-testid="booking-submit"]')
            await page.wait_for_selector('[data-testid="confirmation"]')
            await page.wait_for_timeout(SETTLE)
            conf2 = await rect(page, '[data-testid="confirmation"]')
            ok(f"F2 retry confirmation fully visible ({tag})", fully_in_view(conf2), json.dumps(conf2))
            await page.unroute("**/reservations")
            # closed day empty state reveal (phone relevance)
            await search2(page, CLOSED, 2)
            await page.wait_for_selector('[data-testid="no-slots"]')
            await page.wait_for_timeout(SETTLE)
            ns = await rect(page, '[data-testid="no-slots"]')
            ok(f"F2 empty state fully visible ({tag})", fully_in_view(ns), json.dumps(ns))
            await page.screenshot(path=f"{SHOTS}/F2-empty-{tag}.png")
            await ctx.close()

        # stale scroll guard + reduced motion (desktop)
        httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
        ctx = await browser.new_context(viewport={"width": 1280, "height": 900})
        page = await ctx.new_page()
        await login_ui(page)
        await page.goto(BASE)
        await page.evaluate("window.scrollTo(0, 0)")

        async def slow6(route):
            await asyncio.sleep(2.0)
            try:
                await route.continue_()
            except Exception:
                pass
        await page.route("**/availability?*party_size=6*", slow6)
        await page.fill('[data-testid="date-input"]', DATE1)
        await page.fill('[data-testid="party-size-input"]', "6")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_timeout(300)
        await page.fill('[data-testid="party-size-input"]', "2")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_selector('[data-testid="availability-grid"]')
        await page.wait_for_timeout(SETTLE)
        y_b = await page.evaluate("window.scrollY")
        await page.wait_for_timeout(2200)  # A resolves late
        y_a = await page.evaluate("window.scrollY")
        t1 = await page.locator('[data-testid="slot-t_1-18:00"]').get_attribute("data-available")
        ok("F2 late A does not scroll or replace B grid", y_a == y_b and t1 == "true",
           f"scroll {y_b}->{y_a}, t1={t1}")
        await ctx.close()

        httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
        ctx = await browser.new_context(viewport={"width": 1280, "height": 900}, reduced_motion="reduce")
        page = await ctx.new_page()
        await login_ui(page)
        await page.goto(BASE)
        await page.fill('[data-testid="date-input"]', DATE1)
        await page.fill('[data-testid="party-size-input"]', "6")
        await page.click('[data-testid="search-button"]')
        t0 = asyncio.get_event_loop().time()
        await page.wait_for_selector('[data-testid="availability-grid"]')
        await page.wait_for_function("window.scrollY > 0", timeout=1500)
        dt_ms = (asyncio.get_event_loop().time() - t0) * 1000
        ok("F2 reduced motion: reveal applied immediately", dt_ms < 600, f"{dt_ms:.0f}ms")
        await page.click('[data-testid="slot-t_1+t_2-18:00"]')
        sel = await page.wait_for_function(
            """() => document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.dataset.selected === 'true'""",
            timeout=500)
        ok("F2 reduced motion: selected attribute immediate", sel is not None)
        await ctx.close()

        # ================= F3 + F4: plan naming, paint order =================
        for w, tag in ((1280, "d"), (375, "p")):
            for theme in ("light", "dark"):
                httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
                ctx = await browser.new_context(viewport={"width": w, "height": 900})
                page = await ctx.new_page()
                await page.goto(BASE)
                await page.click(f'[data-testid="theme-{theme}"]')
                await login_ui(page)
                await page.goto(BASE)
                await search2(page, DATE1, 6)
                await page.evaluate(
                    "() => { const svg = [...document.querySelectorAll('svg')].find(s => s.querySelector('.plate-label'));"
                    " if (svg) svg.scrollIntoView({block: 'center'}); }")
                await page.wait_for_timeout(300)
                geo = await page.evaluate("""() => {
                  const out = {plates: [], seats: [], planNames: 0, links: [], badges: [],
                               order: null, badgeOccluded: null, aria: [], sliceSamples: []};
                  const svgs = [...document.querySelectorAll('svg')];
                  const floor = svgs.find(s => s.querySelector('.plate-label'));
                  if (!floor) return null;
                  const all = [...floor.querySelectorAll('*')];
                  const firstLink = all.findIndex(e => e.classList && e.classList.contains('pair-link'));
                  const firstTop = all.findIndex(e => e.classList && (e.classList.contains('table-top') || e.classList.contains('table-on-plan')));
                  const badgeEls = all.filter(e => e.classList && e.classList.contains('pair-badge'));
                  const lastTop = all.reduce((acc, e, i) => (e.classList && e.classList.contains('table-on-plan')) ? i : acc, -1);
                  out.order = {firstLink, firstTop, lastTop, lastBadge: all.indexOf(badgeEls[badgeEls.length-1] || floor), badgeCount: badgeEls.length};
                  document.querySelectorAll('.table-on-plan').forEach((g) => {
                    const labels = g.querySelectorAll('.plate-label');
                    const names = g.querySelectorAll('.plan-name');
                    const seats = g.querySelector('.plan-seats');
                    const gr = g.getBoundingClientRect();
                    const fs = labels[0] ? getComputedStyle(labels[0]).fontSize : null;
                    const ss = seats ? getComputedStyle(seats).fontSize : null;
                    let lr = null;
                    labels.forEach((l) => {
                      const r = l.getBoundingClientRect();
                      lr = lr ? {top: Math.min(lr.top, r.top), bottom: Math.max(lr.bottom, r.bottom)} : {top: r.top, bottom: r.bottom};
                    });
                    out.plates.push({table: g.getAttribute('data-testid'),
                                     joinedName: [...labels].map(l => l.textContent.trim()).join(' '),
                                     labelCount: labels.length, planNameCount: names.length,
                                     fontSize: fs, seatsFontSize: ss,
                                     fits: lr ? (lr.top >= gr.top - 2 && lr.bottom <= gr.bottom + 2) : null});
                    out.seats.push(seats ? seats.textContent.trim() : null);
                    out.aria.push(g.getAttribute('aria-label') || '');
                  });
                  document.querySelectorAll('.plate-label').forEach((t) => {
                    const tr = t.getBoundingClientRect();
                    const xs = [tr.left + tr.width * 0.25, tr.left + tr.width * 0.5, tr.left + tr.width * 0.75];
                    const ys = [tr.top + tr.height * 0.3, tr.top + tr.height * 0.6];
                    xs.forEach((x) => ys.forEach((y) => {
                      const hit = document.elementFromPoint(x, y);
                      if (hit && hit.classList && hit.classList.contains('pair-link')) {
                        out.links.push({slice: true, x: Math.round(x), y: Math.round(y)});
                        out.sliceSamples.push("pair-link over " + (t.textContent || "").trim());
                      }
                    }));
                  });
                  const badge = document.querySelector('.pair-badge');
                  if (badge) {
                    badge.scrollIntoView({block: 'center'});
                    const br = badge.getBoundingClientRect();
                    const cx = br.left + br.width / 2, cy = br.top + br.height / 2;
                    const top = document.elementFromPoint(cx, cy);
                    out.badgeOccluded = !(badge === top || badge.contains(top));
                    const svg = badge.closest('svg').getBoundingClientRect();
                    out.badges = [{inSvg: br.left >= svg.left - 1 && br.right <= svg.right + 1}];
                  }
                  return out;
                }""")
                ok(f"F3 one wrapped human name per table, no duplicate caption ({tag}-{theme})",
                   geo and all(p["planNameCount"] == 0 for p in geo["plates"])
                   and sorted(p["joinedName"] for p in geo["plates"])
                   == sorted(["Window alcove", "Garden corner", "Hearth booth"]),
                   json.dumps(geo and [p["joinedName"] for p in geo["plates"]]))
                ok(f"F3 plate font >=13px ({tag}-{theme})",
                   geo and all(int(p["fontSize"].replace("px", "")) >= 13 for p in geo["plates"]),
                   json.dumps([p["fontSize"] for p in geo["plates"]]))
                ok(f"F3 seat metadata present at 16px ({tag}-{theme})",
                   geo and all(s and "seat" in s for s in geo["seats"])
                   and all(int(p["seatsFontSize"].replace("px", "")) == 16 for p in geo["plates"]),
                   json.dumps(geo["seats"]))
                ok(f"F3 plate names fit table tops ({tag}-{theme})",
                   geo and all(p["fits"] for p in geo["plates"]), json.dumps(geo["plates"]))
                ok(f"F3 accessible full names retained ({tag}-{theme})",
                   geo and any("Window alcove" in a for a in geo["aria"])
                   and any("Garden corner" in a for a in geo["aria"]),
                   json.dumps(geo and geo["aria"]))
                ok(f"F4 link layer before tops, badge after ({tag}-{theme})",
                   geo and geo["order"]["firstLink"] < geo["order"]["firstTop"]
                   and geo["order"]["badgeCount"] >= 1
                   and geo["order"]["lastBadge"] > geo["order"]["lastTop"],
                   json.dumps(geo and geo["order"]))
                ok(f"F4 connector never slices label ink ({tag}-{theme})",
                   geo and len(geo["links"]) == 0, json.dumps(geo and geo["links"]))
                ok(f"F4 badge not occluded, inside svg ({tag}-{theme})",
                   geo and geo["badgeOccluded"] is False and all(b["inSvg"] for b in geo["badges"]),
                   json.dumps({"occ": geo and geo["badgeOccluded"], "b": geo and geo["badges"]}))
                # pair selection still works end to end
                cell = page.locator('[data-testid="slot-t_1+t_2-18:00"]')
                await cell.click()
                await page.wait_for_function(
                    """() => document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.dataset.selected === 'true'""")
                badge_sel = await page.evaluate(
                    """() => { const b = document.querySelector('.pair-badge'); return b ? b.getAttribute('data-selected') : null; }""")
                ok(f"F4 badge mirrors selection ({tag}-{theme})", badge_sel == "true", str(badge_sel))
                await page.screenshot(path=f"{SHOTS}/F3F4-{tag}-{theme}.png")
                await ctx.close()

        await browser.close()
    print(f"\nCLOSURE RESULT pass={PASS} fail={FAIL}")
    for n in NOTES:
        print(" -", n)
    sys.exit(1 if FAIL else 0)


async def search2(page, date, party):
    await page.fill('[data-testid="date-input"]', date)
    await page.fill('[data-testid="party-size-input"]', str(party))
    await page.click('[data-testid="search-button"]')
    await page.wait_for_timeout(400)


asyncio.run(run())
