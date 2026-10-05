# S4-R1 — Formal review round 1 (of 3), stage 4

**Verdict: ACCEPT**

All four stage suites pass isolated at the exact frozen SHA; all independent stage-4
probes (planner/optimizer, preview/apply/closures, series amendments, B1/B2 portability
controls, three genuine older-source transfers, modern 4→4 portability) pass; the full
browser/design battery passes; the design critic finds no critical issues. No blocking
findings.

- Candidate: `/home/nryn/work/seatright/runs/tablekeeper2/wt/review/stage-4`
- Frozen SHA: `a37befe400bd0e5686af7b47dbe417ad4a433c4e` — HEAD + clean `git status`
  verified at start (19:38:21Z) and end (20:19:58Z). Accepted trees unchanged:
  stage-1 `8b8b28da…`, stage-2 `9fee3dc7…`, stage-3 `c783f9e0…`.
- My images (cache-hit from staging copies of the frozen bytes):
  `s4r1/stage4:cand` = `db22c5c65f59` (145MB), `s4r1/stage3:src` = `a210fa43017c`,
  `s4r1/stage2:src` = `2d56b93ba7ee`, `s4r1/stage1:src` = `e7e1f3f26d33`.
- Review window: 2026-10-05T19:38:21Z → 20:19:58Z (~41 min foreground).

## 1. Supplied check (verbatim, isolated)

First attempt at 19:38 contaminated by parallel Go/UI gate load: stage-2 had 5 Playwright
timeout failures with `highest_contiguous: 1` (infrastructure, preserved in
`checks/` and `notes/harness-stdout.log`). Serial rerun on a clean out dir
(`checks-serial/`, `notes/harness-serial.log`): exit 0, revision = frozen SHA, mode
`isolated`, state `completed`, **stage 1: 120/120, stage 2: 25/25, stage 3: 7/7,
stage 4: 6/6, highest_contiguous: 4, claimed_stage: 4**, stage-5 probe correctly absent
(no stage 5 exists).

## 2. Source gates (staging copies)

- `gofmt -l internal cmd probes/pclient` clean; `go vet ./...` 0;
  `go test -count=1 -timeout 40m ./...` all packages ok; `go build` 0 — `notes/go-gates.log`.
- `CGO_ENABLED=1 go test -race -count=1 -timeout 40m ./...` (golang:1.27-bookworm
  container; host lacks gcc): **all packages ok, race_exit=0** (service 1078.5s) —
  `notes/go-race.log`.
- UI: npm ci 0, svelte-check 0 errors, **100 tests / 15 files pass** (test_exit=0),
  build 0, `node --check s4-g2-live.mjs` ok, `node --check s4-g2-clock.mjs` ok —
  `notes/ui-gates.log`.

## 3. Delivery and offline

- Default 8080 healthy 0.34s, 15-byte exact body + charset; PORT=9571 healthy 0.38s;
  container-to-container binding proven; caps 2CPU/2GiB, privileged=false; full inspect
  records in review.log.
- Offline network-none gate (`probes/s4_offline_gate.sh`, exit 0, **pass 23 fail 0**):
  reset/logins/policy publish/explain (selected caps 10/10)/pair booking/history/decision/
  series adopt/preview/apply/series amend — full authenticated stage-4 function; pages,
  five fonts typed nonempty distinct hashes; egress nonzero with `dial tcp: lookup …
  network unreachable` reason captured.

## 4. Independent stage-4 API probe — 69/0, exit 0

`probes/s4_api_probe.py` (`notes/s4-api-probe.log`):
- Replan permissions (401/403/404), invalid intervals (from≥to, missing offset, unknown
  table), empty preview → zero-move apply increments counter once and stores closure;
  closure blocks creates; reset clears closures.
- Optimizer: hand-computed world (A on closed t_1 → unique min-unused target [t_2] at
  rank order, unused 2); preview-export delta = exactly plans(+1)+receipts(+1) with
  bookings/histories/counters unchanged; stored plans bind `applied=False` + captured
  revision; apply freezes times/terms and sets rev+1; stale_plan after intervening
  revision; plan_already_applied + same-key 200 replay; reassigned history with plan_id;
  closure excludes singles/pairs and explains no_overlap false; infeasible 409
  no_feasible_plan with state unchanged; past-cutoff operator repair works.
- Series amend: full validation matrix (bool/string expected, from_index bounds,
  local_time shapes), stale 409, unknown-404/no-token-401; all-occurrence clock move to
  scheduled dates +20:00; per-occurrence rev+1 + changed history (starts_at_local only);
  series+1/counter+1 once; no new exceptions; exception occurrence skipped; cancel
  occurrence → series+1 no exception, repeat no bump, suffix survivor confirmed via owner
  GET; replay 200 raw after edits.
- B1/B2 controls: evolved zero-move-apply snapshot imports 204; plan_id-less preview
  receipt rejected 422 with destination retention. (Coordinator's two known issues: both
  independently confirmed fixed on this frozen build.)

## 5. Inherited regression on the stage-4 build

- Stage-2 probe: **346/0** exit 0. Stage-3 probe: **217/0** exit 0
  (policies/history/series/collective all green under stage-4 service).
- Portability migration: **45/0** exit 0 — genuine stage-1/2/3 source processes (fresh
  builds from accepted trees) → stage-4 destinations with replacement semantics,
  hash login, token survival, revision/term normalization (legacy rev1/fixture0 for
  stage-1/2; modern metadata preserved verbatim for stage-3), original receipt replays
  byte-exact after destination mutation, imported anchor adoption, modern 4→4 with
  stale/unapplied plans, applied plan, adopted series with cancel/exception, collective
  batch receipt replay, corrupt-import atomicity, reset clearing.

## 6. Browser and design — all green

- Closure/reveal driver: **121/0, exit 0**.
- States driver: **132/0, exit 0** — all named states, light/dark/system, 375/1280.
- Measurements: **59/0, exit 0** — all state words ≥4.5, held pair **7.611/9.915**,
  density 15/75 and 43/215, no horizontal page scroll, reduced motion immediate.
- True same-Document upgrades × 3: stage-1→4 **19/0**, stage-2→4 **19/0**,
  stage-3→4 **19/0** — each: commit-dropped 201, same-Document retry on strict stage-4
  destination, byte-identical authentic legacy receipt (stage-1 ten-field no table_ids;
  stage-2 scalar+array no modern fields; stage-3 modern revision/terms preserved),
  old-token lookup live, one booking.
- Design critic on 15 curated settled screenshots: **VERDICT-CRITICAL: NONE**.

## 7. Audits

No rate limiting; no fixture-value branches in Go non-test code; bcrypt(SHA-256 prehash);
keel/id; Chaaya keel adapter; RUN.md consistent. Portability corruption-rejection matrix
already exercised in r2/r3 evidence and re-confirmed here (corrupt 422 atomic).

## 8. Known issues decision

- K1 (B1/B2): **both fixed on this frozen build** — independently reproduced the exact
  coordinator control sequences (valid evolved zero-move apply snapshot imports 204;
  plan_id-less preview receipt rejected 422 atomically). Not blocking.
- K2 (P2D binder deficiency): an evidence-driver deficiency, not a product defect — my
  formal probes bind preview/apply/receipt fields with strict equality and mutation
  rejection; no inheritance of the P2D driver. Not blocking.
- K3 (no integrated stage4-import.sh): coverage was my obligation and is discharged by
  my 45/0 three-lane migration probe + modern 4→4. The absent dedicated script is a
  packaging/report gap, not a requirements gap — not blocking acceptance.
- K4/K5: race completed with CGO container; ENOSPC risks did not materialize (1.1–1.6G
  free throughout); superseded FINAL-OUTCOME.md prose not counted.

## 9. Counts and cleanup

Supplied: 120+25+7+6 = **178**, claimed 4, highest 4. Independent: S4 probe 69 + S2
regression 346 + S3 regression 217 + migration 45 + closure 121 + states 132 + measure 59
+ upgrades 19×3 = **1017 assertions, 0 failures**. Context: 632/245/16 on earlier producer
(not this gate). Reviewer containers/images/network removed; images
`s4r1/stage4:cand (db22c5c65f59)`, `stage3:src`, `stage2:src`, `stage1:src` retained; no
prune. Candidate clean at frozen SHA after cleanup. Elapsed 19:38–20:20Z ≈ **41 minutes**
foreground. Usage figures not visible in this runtime.
