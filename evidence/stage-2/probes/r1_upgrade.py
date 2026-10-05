"""S2-R1 true existing-browser upgrade proof.

Stage-1 donor UI in a real browser: sign in, submit a booking whose 201 response
is dropped AFTER the source commits. Same Document, pending form/key/body and
session preserved. Export source (200), import a distinct stage-2 destination
(204) between requests, then forward the original page's subsequent API requests
to the destination WITHOUT reload/new screen. Same-key retry must return the
original legacy receipt (no table_ids) with the original reference.

Usage: python r1_upgrade.py <donor_base> <dest_base> <shots_dir>
"""
import asyncio
import json
import os
import sys
from urllib.parse import urlsplit

import httpx
from playwright.async_api import async_playwright

DONOR = sys.argv[1].rstrip("/")
DEST = sys.argv[2].rstrip("/")
SHOTS = sys.argv[3]
DATE = "2027-06-18"  # Friday (hours 18:00-23:30)

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
        NOTES.append(f"{name} {extra}")
        print(f"  FAIL {name} {extra}")


FIX1 = {
    "users": [{"id": "u_legacy", "email": "legacy@example.com", "password": "legacy pass 1", "display_name": "Legacy Lena"}],
    "restaurants": [{
        "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                   {"id": "t_2", "label": "2", "capacity": 4},
                   {"id": "t_3", "label": "3", "capacity": 4}],
    }],
    "reservations": [],
}
DEST_PRE = {
    "users": [{"id": "u_other", "email": "other@example.com", "password": "other pass 1", "display_name": "Other Otto"}],
    "restaurants": FIX1["restaurants"],
    "reservations": [],
}


async def main():
    d = httpx.Client(base_url=DONOR, timeout=10)
    t = httpx.Client(base_url=DEST, timeout=10)
    assert d.post("/_test/reset", json=FIX1).status_code == 204
    assert t.post("/_test/reset", json=DEST_PRE).status_code == 204
    tok_other = t.post("/auth/login", json={"email": "other@example.com", "password": "other pass 1"}).json()["token"]
    ok("destination prestate ready (different tenant)", t.get("/reservations", headers={"Authorization": "Bearer " + tok_other}).status_code == 200)

    dropped = {}
    forwarded = []

    async with async_playwright() as pw:
        browser = await pw.chromium.launch(args=["--no-sandbox"])
        ctx = await browser.new_context(viewport={"width": 1280, "height": 900},
                                        record_video_dir=f"{SHOTS}/videos",
                                        record_video_size={"width": 1280, "height": 900})
        page = await ctx.new_page()
        await page.goto(f"{DONOR}/")
        await shot_wrap(page, SHOTS)
        # sign up in the stage-1 UI
        await page.goto(f"{DONOR}/login")
        await page.fill('[data-testid="login-email"]', "legacy@example.com")
        await page.fill('[data-testid="login-password"]', "legacy pass 1")
        await page.click('[data-testid="login-submit"]')
        await page.wait_for_selector('[data-testid="current-user"]')
        name = await page.inner_text('[data-testid="current-user"]')
        ok("stage-1 UI signed in", "Legacy Lena" in name, name)
        # search and open the booking form
        await page.goto(f"{DONOR}/")
        await page.fill('[data-testid="date-input"]', DATE)
        await page.fill('[data-testid="party-size-input"]', "4")
        await page.click('[data-testid="search-button"]')
        await page.wait_for_selector('[data-testid="availability-grid"]')
        await page.click('[data-testid="slot-t_2-18:00"]')
        await page.wait_for_selector('[data-testid="booking-form"]')
        await shot_wrap(page, SHOTS)
        # submit and drop the response after the source commits
        async def drop_after_commit(route):
            resp = await route.fetch()
            dropped["key"] = await route.request.header_value("idempotency-key")
            dropped["body"] = route.request.post_data
            dropped["status"] = resp.status
            dropped["resp"] = await resp.text()
            await route.abort()
        await page.route("**/reservations", drop_after_commit)
        await page.click('[data-testid="booking-submit"]')
        await page.wait_for_selector('[data-testid="booking-uncertain"]')
        ok("source committed 201 then response dropped", dropped.get("status") == 201,
           str(dropped.get("status")))
        orig = json.loads(dropped["resp"])
        orig_ref = orig["reference"]
        ok("stage-1 receipt has no table_ids", "table_ids" not in orig, list(orig))
        ok("uncertain shown, no confirmation/error",
           await page.locator('[data-testid="booking-error"]').count() == 0
           and await page.locator('[data-testid="confirmation"]').count() == 0)
        await shot_wrap(page, SHOTS)

        # export source, import into distinct stage-2 destination between requests
        snap = d.get("/_test/export")
        ok("source export 200", snap.status_code == 200, snap.status_code)
        pre = t.post("/_test/import", json={"track": "tablekeeper", "format_version": 1, "state": {"x": 1}})
        ok("destination rejects junk import first", pre.status_code == 422, pre.status_code)
        imp = t.post("/_test/import", content=snap.content,
                     headers={"Content-Type": "application/json; charset=utf-8"})
        ok("destination import 204", imp.status_code == 204, imp.status_code)
        ok("destination replaced prestate tenant", t.get("/reservations", headers={"Authorization": "Bearer " + tok_other}).status_code == 401)

        # forward all subsequent origin requests of the SAME page to the destination
        async def forward(route):
            u = urlsplit(route.request.url)
            new_url = DEST + u.path + ("?" + u.query if u.query else "")
            headers = {k: v for k, v in (await route.request.all_headers()).items()
                       if k.lower() not in ("host", "origin", "referer")}
            resp = await route.fetch(url=new_url, method=route.request.method, headers=headers,
                                     post_data=route.request.post_data)
            if u.path.startswith(("/api", "/reservations", "/auth", "/availability", "/restaurants",
                                  "/_test", "/signup", "/login", "/lookup")) or u.path == "/":
                pass
            if route.request.method == "POST" and u.path == "/reservations":
                forwarded.append({"key": await route.request.header_value("idempotency-key"),
                                  "body": route.request.post_data,
                                  "status": resp.status, "resp": await resp.text()})
            await route.fulfill(response=resp)
        await page.route(f"{DONOR}/**", forward)

        # same-page retry through the destination, no reload
        await page.click('[data-testid="booking-submit"]')
        await page.wait_for_selector('[data-testid="confirmation"]')
        ok("retry forwarded exactly once", len(forwarded) == 1, len(forwarded))
        f = forwarded[0]
        ok("retry same key across upgrade", f["key"] == dropped["key"], f"{f['key']} vs {dropped['key']}")
        ok("retry same body across upgrade", json.loads(f["body"]) == json.loads(dropped["body"]),
           f"{f['body']} vs {dropped['body']}")
        ok("destination returned 200 original receipt", f["status"] == 200, f["status"])
        ok("forwarded receipt EXACT original legacy JSON", json.loads(f["resp"]) == orig, f["resp"][:200])
        ref_text = (await page.inner_text('[data-testid="confirmation-reference"]')).strip()
        ok("original reference shown, uncertainty cleared", ref_text == orig_ref
           and await page.locator('[data-testid="booking-uncertain"]').count() == 0
           and await page.locator('[data-testid="booking-error"]').count() == 0, ref_text)
        ok("session survived upgrade (current-user present)",
           "Legacy Lena" in await page.inner_text('[data-testid="current-user"]'))
        # old token lookup through the same page via destination
        lookup = await page.evaluate(
            "async (ref) => { const s = JSON.parse(localStorage.getItem('tablekeeper-session'));"
            " const r = await fetch('/reservations/' + ref, {headers: {Authorization: 'Bearer ' + s.token}});"
            " return {status: r.status, body: await r.json()}; }", orig_ref)
        ok("old session token works on destination via page fetch", lookup["status"] == 200,
           str(lookup))
        ok("destination lookup carries table_ids + singleton table_id",
           lookup["body"].get("table_ids") == ["t_2"] and lookup["body"].get("table_id") == "t_2",
           json.dumps(lookup["body"])[:160])
        # destination really holds exactly one booking
        tl = t.post("/auth/login", json={"email": "legacy@example.com", "password": "legacy pass 1"}).json()["token"]
        recs = t.get("/reservations", headers={"Authorization": "Bearer " + tl}).json()["reservations"]
        ok("destination holds exactly one booking", len(recs) == 1 and recs[0]["reference"] == orig_ref,
           json.dumps([(r["reference"], r["status"]) for r in recs]))
        await shot_wrap(page, SHOTS)
        await ctx.close()
        await browser.close()
    print(f"\nUPGRADE RESULT pass={PASS} fail={FAIL}")
    for n in NOTES:
        print(" -", n)
    sys.exit(1 if FAIL else 0)


async def shot_wrap(page, shots):
    await page.screenshot(path=f"{shots}/upgrade-{upgrade_n[0]}.png")
    upgrade_n[0] += 1

upgrade_n = [0]
asyncio.run(main())
