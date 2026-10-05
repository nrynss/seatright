# S2-R2 — Formal review round 2 (of 3), stage 2

**Verdict: ACCEPT**

Every requirement I checked holds, every supplied check passes at the exact frozen SHA, and
all four round-1 blocking findings (F1–F4) are closed with fresh viewport-truth evidence.

- Candidate: `/home/nryn/work/seatright/runs/tablekeeper2/wt/review/stage-2`
- Frozen SHA: `8812cdeaaa993cd944493c654e51d355cdd6b676` — `git rev-parse HEAD` + empty
  `git status --short` verified BEFORE any review step (04:07:43Z) and again at the end
  (04:37:5xZ, see review.log). Branch `tablekeeper2-candidate`.
- Change scope vs reviewed `bb604b82…`: exactly 10 declared web files + stage-2/PLAN.md
  coordinator metadata (`git diff --stat` in review.log). No Go/data/Docker/dependency/RUN/
  stage-1 changes.
- Mode: isolated (supplied harness) + my own containers from staging copies of the same
  bytes. My images: `s2r2/stage2:cand` = `898659df1aed` (144MB) and `s2r2/stage1:donor` =
  `87dec5741642` (143MB), both fully cache-hit → byte-identical to the owner's build of the
  same frozen bytes.
- Review window: 2026-10-05T04:07:43Z → 2026-10-05T04:38:30Z (~31 min foreground).
- Raw chronological log: `review.log`; driver outputs under `notes/` with effective exit
  codes (`*_EXIT=` captured from the container process, no tee-masking).

## 1. Supplied check (verbatim, isolated)

Exit 0; runtime ≈2 min. `checks/report.json`: revision `8812cde…`, mode `isolated`,
state `completed`, stage 1 **120/120**, stage 2 **25/25**, stage-3 probe **fail (expected,
policies 404)**, `highest_contiguous: 2`, `claimed_stage: 2`, share 1.0.

## 2. Full gauntlet re-run (not just the fix)

| Area | Result | Evidence |
|---|---|---|
| Delivery: default 8080 (healthy 1.08s, exact body + `application/json; charset=utf-8`), PORT=9555, 0.0.0.0 via container IP, 2CPU/2GiB caps inspected | pass | review.log 04:11:48–04:11:58 |
| Offline internal network: reset/signup/login/availability/booking/index/CSS/JS (new bundle `index-D-GAb4zz.js` proves the F1–F4 build ships)/5 woff2 fonts, all 200; egress ConnectError | pass | notes 04:12:23 (`offline_exit=0`) |
| Go gates (staging copy): vet 0, build 0, test ok | pass | notes/go-gates.log |
| Go race (golang container; host lacks gcc): exit 0 | pass | notes/go-race.log (363.6s) |
| UI gates (staging copy): npm ci 0, svelte-check 0 errors, **82/82 tests** incl. Chaaya contrast+a11y gates, build 0 | pass | notes/ui-gates.log |
| Independent API probe (my own 346 checks: envelope matrix, 400-vs-422 types, party/local/query-decimal exceptions, key bounds/scope/receipt-precedence/failed-key reuse, 50-way concurrency, overlap/adjacency/cutoff, DST Berlin/NY/Auckland + Kolkata, pairs full matrix, moves full matrix, export/import) | **346/346**, exit 0 | probes/r1_api_probe.py, notes/api-probe-stage2.log |
| Cross-process migration (fresh stage-1 donor → stage-2 dests): replacement, sessions/hashes, legacy + modern receipts byte-exact, failed-key reuse, repeat/reset/corrupt atomicity | **38/38**, exit 0 | probes/r1_migration_probe.py, notes/migration-probe.log |
| True browser upgrade (stage-1 UI, commit+dropped 201, same Document, export 200 → import 204 replacing a different tenant → no-reload retry on stage-2 → 200 exact legacy receipt, no table_ids, old-token lookup 200, exactly one booking) | **19/19**, exit 0 | probes/r1_upgrade.py, notes/upgrade-run.log |
| Browser states (all named hooks/states, light/dark/system, 375/1280, uncertain/409/lookup/auth flows) | **132/132**, exit 0 | probes/r1_states.py, notes/states-run.log |
| Measurements: rendered contrast all ≥4.5 (selected pair **7.611 light / 9.915 dark**), density 15min=15 slots/75 cells + 5min=43/215, page width fixed, rail internal scroll, reduced motion immediate, stagger ≤480ms, late-search-wins, party snapshot | **59/59**, exit 0 | probes/r1_measure.py, notes/measure-run.log |
| Context-only (implementer probes, not acceptance): stage2-api 231/0, donor 41/0, import 34+1skip/0, html 16/0 | pass | notes/ctx-*.log |

## 3. F1–F4 closure (dedicated driver, viewport-truth)

`probes/r2_closure.py` → **121/121 pass, CLOSURE_EXIT=0** (`notes/closure-run.log`).
Methodology: scroll positions recorded before/after every action; measurements taken after
the app's own reveal (≥900ms settle; no driver scrolling of the target); effective exit
codes propagated.

- **F1 (auth links)**: signed-out shows Sign in/Create account; signed in they are absent on
  `/`, `/signup`, `/login`, `/lookup` with current-user present (both widths); logout
  restores them; injected `{displayName: ""}` session keeps current-user (empty text) and
  hides auth links; no horizontal overflow at 375.
- **F2 (reveal)**: search reveal shows `.grid-caption` in viewport with ≥1 actionable
  available cell visible and floor retained; keyboard selection reveals form heading +
  party + submit fully in view (desktop 1280×900, phone 375×900 and 375×812); confirmation
  fully visible with readable reference and form retained; 409 shows error without upward
  re-reveal and preserves form/inputs; uncertain visible on phone after reveal and retry
  restores confirmation; closed-day empty state fully visible; party typing does not move
  scroll (DOM-event injection, no driver scroll); late stale search neither scrolls nor
  replaces the newer grid; reduced motion applies reveal immediately and sets
  `data-selected` at once. Viewport screenshots: `shots/F2-*.png`; videos in `shots/videos/`.
- **F3 (single human name)**: exactly one wrapped `.plate-label` name per table (joined =
  "Window alcove"/"Garden corner"/"Hearth booth"), `.plan-name` count 0, `.plan-seats`
  metadata "N seats" at 16px, plate font ≥13px, names fit table tops, accessible names
  retain full phrases; light+dark, both widths.
- **F4 (paint order)**: `.pair-link` layer before table tops, badges after; paint sampling
  (elementFromPoint over label ink grids) finds zero pair-link pixels over label text in
  all four theme/width combinations; badge not occluded and inside the SVG; badge mirrors
  selection and remains clickable.

## 4. Design critic and independent verification

Critic ran over 16 curated round-2 screenshots (`notes/design-critic.log`). Its three
VERDICT-CRITICAL claims were tested and **refuted by measurement**
(`probes/r2_refute.py`, **12/12, exit 0**, `notes/refute-run.log`, `shots/refute-*.png`):
- "Dense grid below fold after search" → after the app's reveal settles, caption is in view
  and actionable available cells are visible for both 15-min and 5-min grids (my density
  driver had screenshotted pre-reveal; artifacts, not product state).
- "Form off-screen after selection (mobile/desktop)" → with the reveal settled, form and
  submit are fully visible in all four theme/width combinations (the F3F4 screenshots were
  taken pre-reveal by design: state-truth-then-scroll).
- `confirmation-tables` repeating the table names below `confirmation-details` is
  **spec-required** (two separate mandatory testids; R156/R189) — the critic's removal fix
  is rejected, consistent with the round-1 rule against adopting critic contradictions.
- Lowercase server message in `booking-error` remains permitted ("message may use any
  wording") — taste note only.

## 5. Prior-findings closure

- R1 F1–F4: **closed** (section 3).
- S2-H contrast, G4/G5 ink: still closed (measure pass, 7.611/9.915).
- N1 (off-grid seeded amendments, stage-3 awareness): backend unchanged this round
  (diff scope: web-only); remains a non-blocking recorded observation. Re-verified live
  once during the gauntlet via the same repro path (`r1_offgrid_noop.py` retained in
  probes/; behavior unchanged).
- Round-1 evidence limitation (critic-verification phone timeout with wrapper exit 0):
  corrected methodology — every driver exit this round is the container process's real
  code (`CLOSURE_EXIT=0`, `STATES_EXIT=0`, `MEASURE_EXIT=0`, `UPGRADE_EXIT=0`,
  `REFUTE_EXIT=0`), and fresh phone+desktop reveal measurements are complete.

## 6. Limits and provenance

- Stage-3/4 intentionally absent; stage-3 probe fails as required.
- `-race` executed in `golang:1.27-bookworm` (host has no gcc), exit 0.
- System theme: explicit light/dark measured; system-mode verified via
  `prefers-color-scheme` emulation in the states pass (round-1 methodology retained).
- Private artifacts (tokens, exports) confined to `/tmp/s2r2-priv` (0700) and never
  printed; no secrets in this report.
- Reviewer containers/network removed at the end; candidate re-verified clean at the
  frozen SHA after cleanup.

## 7. Commands and counts

Harness 120+25/claimed2/highest2/next3fail · API 346 · migration 38 · upgrade 19 ·
states 132 · measure 59 · closure 121 · refute 12 · context 231+41+35+16 · Go
vet/build/test/race 0 · UI ci/check/82tests/build 0. Total independent assertions this
round: 767 (plus supplied 145). No blocking findings. Elapsed ~31 minutes foreground.
