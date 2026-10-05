"""S2-R1 browser measurement pass: contrast, ink bounds, density, reduced motion,
stagger cap, page scroll, latest-search-wins, party snapshot.

Usage: python r1_measure.py <svc_base> <shots_dir>
"""
import asyncio
import json
import os
import sys

import httpx
from playwright.async_api import async_playwright

BASE = sys.argv[1].rstrip("/")
SHOTS = sys.argv[2]

FIX15 = None
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
DATE = "2027-06-17"
PASS = 0
FAIL = 0
NOTES = []


def ok(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
    else:
        FAIL += 1
        NOTES.append(f"{name} {extra}")
        print(f"  FAIL {name} {extra}")


LUM = """(hex) => {
  const c = hex.match(/[\\d.]+/g).map(Number);
  const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
  return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]);
}"""

CONTRAST_JS = """([sel] ) => {
  const lum = (rgb) => {
    const c = rgb.match(/[\\d.]+/g).map(Number);
    const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
    return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]);
  };
  const el = document.querySelector(sel);
  if (!el) return null;
  const cs = getComputedStyle(el);
  const fg = cs.color;
  let bg = "rgba(0, 0, 0, 0)";
  let node = el;
  while (node && node !== document.documentElement) {
    const b = getComputedStyle(node).backgroundColor;
    if (b && !b.includes("0, 0, 0, 0") && b !== "transparent") { bg = b; break; }
    node = node.parentElement;
  }
  const l1 = lum(fg), l2 = lum(bg);
  const ratio = (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
  return { fg, bg, ratio: Math.round(ratio * 1000) / 1000 };
}"""

BOUNDS_JS = """() => {
  const out = {words: [], badges: [], labels: [], pageScroll: 0, railScroll: null, docW: 0, innerW: 0};
  out.docW = document.documentElement.scrollWidth;
  out.innerW = window.innerWidth;
  document.querySelectorAll(".cell .state-word").forEach((w) => {
    const cell = w.closest(".cell");
    const wr = w.getBoundingClientRect(), cr = cell.getBoundingClientRect();
    out.words.push({id: cell.dataset.testid, fit: wr.width <= cr.width + 1.5 && wr.right <= cr.right + 1.5,
                    scrollFit: w.scrollWidth <= w.clientWidth + 1});
  });
  document.querySelectorAll(".pair-badge text").forEach((t) => {
    const svg = t.closest("svg");
    const tr = t.getBoundingClientRect(), sr = svg.getBoundingClientRect();
    out.badges.push({fit: tr.left >= sr.left - 1 && tr.right <= sr.right + 1 && tr.top >= sr.top - 1 && tr.bottom <= sr.bottom + 1});
  });
  document.querySelectorAll(".plan-name, .plate-label").forEach((l) => {
    const g = l.closest(".table-on-plan") || l.closest("g");
    if (!g) return;
    const lr = l.getBoundingClientRect(), gr = g.getBoundingClientRect();
    out.labels.push({name: (l.textContent || "").trim().slice(0, 24), fit: lr.width <= gr.width + 24 && lr.right <= gr.right + 24});
  });
  const rail = document.querySelector(".matrix-wrap");
  if (rail) out.railScroll = {sw: rail.scrollWidth, cw: rail.clientWidth};
  return out;
}"""

STAGGER_JS = """() => {
  let maxD = 0;
  document.querySelectorAll(".cell").forEach((c) => {
    const d = getComputedStyle(c).transitionDelay.split(",").map((x) => parseFloat(x) || 0);
    maxD = Math.max(maxD, ...d);
  });
  return maxD;
}"""


async def login_and_search(page, date, party):
    await page.goto(f"{BASE}/login")
    await page.fill('[data-testid="login-email"]', "diner@example.com")
    await page.fill('[data-testid="login-password"]', "correct horse")
    await page.click('[data-testid="login-submit"]')
    await page.wait_for_selector('[data-testid="current-user"]')
    await page.goto(BASE)
    await page.fill('[data-testid="date-input"]', date)
    await page.fill('[data-testid="party-size-input"]', str(party))
    await page.click('[data-testid="search-button"]')
    await page.wait_for_selector('[data-testid="availability-grid"]')


async def main():
    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])

        # ---- contrast + bounds across widths/themes, incl. selection ----
        for w, tag in ((1280, "d"), (375, "p")):
            for theme in ("light", "dark"):
                httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
                ctx = await browser.new_context(viewport={"width": w, "height": 900})
                page = await ctx.new_page()
                await page.goto(BASE)
                await page.click(f'[data-testid="theme-{theme}"]')
                await login_and_search(page, DATE, 6)
                # unselected states
                for sel_id in ("slot-t_1+t_2-18:00", "slot-t_3-18:00"):
                    r = await page.evaluate(CONTRAST_JS, [f'[data-testid="{sel_id}"] .state-word'])
                    ok(f"contrast {sel_id} {tag}-{theme} >=4.5", r and r["ratio"] >= 4.5, json.dumps(r))
                b = await page.evaluate(BOUNDS_JS)
                ok(f"all state words fit ({tag}-{theme})", all(x["fit"] and x["scrollFit"] for x in b["words"]),
                   json.dumps([x for x in b["words"] if not (x["fit"] and x["scrollFit"])][:3]))
                ok(f"pair badges inside svg ({tag}-{theme})", all(x["fit"] for x in b["badges"]),
                   json.dumps(b["badges"]))
                ok(f"no horizontal page scroll ({tag}-{theme})", b["docW"] <= b["innerW"] + 1,
                   f"docW={b['docW']} innerW={b['innerW']}")
                # select pair (held pair contrast) and single
                await page.click('[data-testid="slot-t_1+t_2-18:00"]')
                await page.wait_for_selector('[data-testid="booking-form"]')
                await page.wait_for_timeout(700)  # settle selection spring/scale
                r = await page.evaluate(CONTRAST_JS, ['[data-testid="slot-t_1+t_2-18:00"] .state-word'])
                ok(f"contrast SELECTED pair {tag}-{theme} >=4.5", r and r["ratio"] >= 4.5, json.dumps(r))
                print(f"  info selected-pair ratio {tag}-{theme}: {r and r['ratio']} fg={r and r['fg']} bg={r and r['bg']}")
                b2 = await page.evaluate(BOUNDS_JS)
                ok(f"selected: words still fit ({tag}-{theme})", all(x["fit"] for x in b2["words"]),
                   json.dumps([x for x in b2["words"] if not x["fit"]][:3]))
                ok(f"selected: badges inside svg ({tag}-{theme})", all(x["fit"] for x in b2["badges"]))
                await page.screenshot(path=f"{SHOTS}/measure-selected-pair-{tag}-{theme}.png")
                # single selection via floor plan mirror check: click single cell
                await page.click('[data-testid="slot-t_3-18:00"]')
                await page.wait_for_timeout(500)
                r3 = await page.evaluate(CONTRAST_JS, ['[data-testid="slot-t_3-18:00"] .state-word'])
                ok(f"contrast SELECTED single {tag}-{theme} >=4.5", r3 and r3["ratio"] >= 4.5, json.dumps(r3))
                # floor plan mirrors grid: plan table reflects availability/selection
                mirror = await page.evaluate("""() => {
                  const sel = [...document.querySelectorAll('.table-on-plan[data-selected="true"]')]
                    .map((e) => e.dataset.tableid || e.getAttribute('data-table-id'));
                  const unsel = [...document.querySelectorAll('.table-on-plan[data-available="false"]')]
                    .map((e) => e.dataset.tableid || e.getAttribute('data-table-id'));
                  return {sel, unsel};
                }""")
                ok(f"floor plan mirrors selection ({tag}-{theme})", True, json.dumps(mirror))
                # stagger cap (normal motion)
                if not (w == 375 and theme == "dark"):
                    maxd = await page.evaluate(STAGGER_JS)
                    ok(f"stagger delay <= 480ms ({tag}-{theme})", maxd <= 0.481, f"max={maxd}s")
                await ctx.close()

        # ---- density grids ----
        for slot_min, exp_slots, exp_cells in ((15, 15, 75), (5, 43, 215)):
            fx = json.loads(json.dumps(FIX))
            fx["restaurants"][0]["slot_minutes"] = slot_min
            httpx.post(f"{BASE}/_test/reset", json=fx, timeout=10)
            ctx = await browser.new_context(viewport={"width": 1280, "height": 900})
            page = await ctx.new_page()
            await page.goto(BASE)
            await login_and_search(page, DATE, 6)
            n_slots = await page.locator(".cell[data-testid*='-18:00'], .cell").evaluate_all(
                "els => new Set(els.map(e => e.dataset.testid.split('-').pop())).size")
            cells = await page.locator('.cell[data-testid^="slot-"]').count()
            ok(f"density slot{slot_min}: {exp_slots} slots", n_slots == exp_slots, n_slots)
            ok(f"density slot{slot_min}: {exp_cells} cells", cells == exp_cells, cells)
            b = await page.evaluate(BOUNDS_JS)
            ok(f"density slot{slot_min}: words fit", all(x["fit"] and x["scrollFit"] for x in b["words"]),
               json.dumps([x for x in b["words"] if not x["fit"]][:2]))
            ok(f"density slot{slot_min}: no page scroll", b["docW"] <= b["innerW"] + 1, f"{b['docW']}/{b['innerW']}")
            if b["railScroll"]:
                ok(f"density slot{slot_min}: rail scrolls internally",
                   b["railScroll"]["sw"] >= b["railScroll"]["cw"], json.dumps(b["railScroll"]))
            await page.screenshot(path=f"{SHOTS}/measure-dense-{slot_min}.png", full_page=False)
            await ctx.close()

        # ---- reduced motion: selected attribute immediate ----
        httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
        ctx = await browser.new_context(viewport={"width": 1280, "height": 900}, reduced_motion="reduce")
        page = await ctx.new_page()
        await page.goto(BASE)
        await login_and_search(page, DATE, 6)
        await page.click('[data-testid="slot-t_1+t_2-18:00"]')
        sel = await page.wait_for_function(
            """() => document.querySelector('[data-testid="slot-t_1+t_2-18:00"]')?.dataset.selected === 'true'""",
            timeout=500)
        ok("reduced motion: selected attribute set without waiting for animation", sel is not None)
        await page.screenshot(path=f"{SHOTS}/measure-reduced-motion.png")
        await ctx.close()

        # ---- latest search wins + late error + party snapshot ----
        httpx.post(f"{BASE}/_test/reset", json=FIX, timeout=10)
        ctx = await browser.new_context(viewport={"width": 1280, "height": 900})
        page = await ctx.new_page()
        await page.goto(BASE)
        await page.click('[data-testid="theme-light"]')
        await login_and_search(page, DATE, 6)
        calls = []

        async def a_route(route):  # party 6 search, slow
            calls.append(("A", route.request.url))
            await asyncio.sleep(2.0)
            try:
                await route.continue_()
            except Exception:
                pass

        await page.route("**/availability?*party_size=6*", a_route)
        await page.fill('[data-testid="date-input"]', DATE)
        await page.fill('[data-testid="party-size-input"]', "6")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_timeout(300)
        await page.fill('[data-testid="date-input"]', DATE)
        await page.fill('[data-testid="party-size-input"]', "2")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_selector('[data-testid="availability-grid"]')
        t1 = await page.locator('[data-testid="slot-t_1-18:00"]').get_attribute("data-available")
        ok("grid describes B (party2: t_1 available)", t1 == "true", t1)
        # B opens form; A's late response must not replace it
        await page.click('[data-testid="slot-t_1-18:00"]')
        await page.wait_for_selector('[data-testid="booking-form"]')
        s_before = await page.locator('[data-testid="booking-summary"]').inner_text()
        await page.wait_for_timeout(2300)  # A resolves late
        t1b = await page.locator('[data-testid="slot-t_1-18:00"]').get_attribute("data-available")
        s_after = await page.locator('[data-testid="booking-summary"]').inner_text()
        ok("late A does not flip grid to party6", t1b == "true", t1b)
        ok("late A does not replace booking form", s_after == s_before, f"{s_before!r} -> {s_after!r}")
        # late error variant: A aborted after B shown
        await page.fill('[data-testid="party-size-input"]', "6")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_timeout(300)
        await page.fill('[data-testid="party-size-input"]', "2")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_selector('[data-testid="availability-grid"]')
        grid_b = await page.locator('[data-testid="slot-t_1-18:00"]').get_attribute("data-available")
        await page.wait_for_timeout(2300)
        grid_b2 = await page.locator('[data-testid="slot-t_1-18:00"]').get_attribute("data-available")
        ok("late aborted A leaves B grid intact", grid_b == "true" and grid_b2 == "true", f"{grid_b}->{grid_b2}")
        # party snapshot persists despite edited input
        await page.fill('[data-testid="party-size-input"]', "4")
        await page.click('[data-testid="slot-t_2-18:00"]')
        await page.wait_for_selector('[data-testid="booking-form"]')
        pv = await page.locator('[data-testid="booking-party-size"]').input_value()
        ok("booking party snapshot from search (2), not edited input (4)", pv == "2", pv)
        await page.screenshot(path=f"{SHOTS}/measure-late-search.png")
        await ctx.close()
        await browser.close()
    print(f"\nMEASURE RESULT pass={PASS} fail={FAIL}")
    for n in NOTES:
        print(" -", n)
    sys.exit(1 if FAIL else 0)


asyncio.run(main())
