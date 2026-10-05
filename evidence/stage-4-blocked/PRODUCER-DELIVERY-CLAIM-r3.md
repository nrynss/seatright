# S4-P2D r3 — final evidence correction (coordinator audit A–F)

**DONE — DELIVERY PASS (r3)** at frozen producer SHA `45581bf3a2dd422b5700a33c3075e56471679971`
(stage-4 tree `00a063ca…`). Stage 4 remains **unaccepted**; native I1/shared63 stays BLOCKED
(after 3 audits); no harness/browser/Go-full/transfer/acceptance claimed.

Evidence root (r3 only): `S4-P2D/r3/` — driver `logs/s4p2d_r3_offline.sh` (fresh, copied
structure, own r3 work/private), raw logs `logs/offline-r3-*.log` + `r3-delivery.log`,
private artifacts `private/` 0700/0600. **F/preservation:** r1 and r2 files (incl. the r2
final driver and all logs) are untouched on disk. Honest overwrite disclosure: during r2's
first "final clean run" I invoked `bash` with an EMPTY driver variable which truncated
`r2/logs/offline-r2-final.log` to zero, then reran the same named file with the real
driver — the zero-byte intermediate was overwritten and only the valid 61/0 content
survives there; r3 now uses entirely fresh named files, and r2's
`offline-r2-sabotage.log` similarly had its real content (fail=55 forced=1, exit
inverted=0) superseded by `offline-r2-sabotage2.log` (fail=55 forced=1, exit **1**). No
log reconstruction anywhere; every named file holds an actual run.

## Provenance / F6

- Producer SHA `45581bf3…`, stage-4 tree `00a063ca…`; accepted trees stage-1 `8b8b28da…`,
  stage-2 `9fee3dc7…`, stage-3 `c783f9e0…` all unchanged at every checkpoint.
- **E (path truth):** the claimed `r2/source-stage4` path did not exist (my r2 mkdir
  omitted it; the r2 build actually built from the original `S4-P2D/source/stage-4`,
  itself byte-verified against the frozen commit and host-corroborated). r3 re-verified
  the r1/r2 archives remain identical (`diff -r` at r2 start). No rebuild performed — the
  unchanged proven image `tablekeeper:s4-p2d-r2` (`c61a81282c70…`) was reused per the
  coordinator's allowance; the r1 image `tablekeeper:s4-p2d` (`9fb6c9a7c39e…`) also
  retained. No build/API-suite reruns claimed.
- **E (startup):** the r1/r2 ".016s" figures were readiness-poll durations measured AFTER
  `docker run` returned, and are relabeled as such. For r3, elapsed was measured from the
  actual run command start (before `docker run`) to the first successful in-container
  health: **0.256s** (docker run → 200 via /app/probe), recorded with the probe output.
- **E (container):** fresh `tk-s4-p2d-r3-offline` (`6253bab889f4…`), direct inspect before
  the driver: image `sha256:c61a81282c70…`, network `none`, PORT=9546, caps 2CPU/2GiB,
  privileged=false; name confirmed absent before start (no rm -f of unknown resources).

## A — tautology removed and full PRE-state binding

The r2 line `ok = ok and plan.get('applied') in (False, None, 0) or True` (always-true)
is gone; `grep -c "or True"` and the old pattern both return 0 in the r3 driver
(`bash -n` exit 0). The stored-plan binding now: sole new namespace delta is
`{plans, receipts}`; exactly one new plan bound by id — `applied` **is False exactly**,
`restaurant_revision == PRE counter (3)`, closure table/from/to exact, assignments
P→[t_2,t_3] changed — and the sole new receipt bound to owner `u_mgr`, method POST, path
`/restaurants/r_anker/replans`, key `r3-prev`, status 201, canonical request and raw
first-201 response substrings. **Mutation-applied guards** (each mutated copy of the true
post must differ): applied→True, plan revision−1, closure.from change, assignment target
change, receipt.response change — all 5 reject (evidence-code strictness, `r3` normal log).
The first r3-normal attempts exposed a REAL always-fail bug of mine (a dead
`('applied': ) if False` fallback with invalid syntax from the r2 carryover); the dead
lines were deleted from the **r3 evidence driver only** — no product/source edit — and
preserved in `offline-r3-normal.log`/`-normal2.log`/`-normal3.log` (disclosed; normal3
failed exactly as predicted before the delete, normal4 passed the stored-plan binding).

## B — raw-byte replays with export capture

Every replay now has PRE export immediately before and POST export immediately after:
apply replay, amendment replay, and (newly captured) the original pair-create replay
**after real repair+amend evolution** — `cmp` byte-identical first-201 vs 200 for all
three; `cmp` full-export equality across each replay; and the replayed original [t_1,t_2]
differs from the CURRENT record [t_2,t_3] (mutated forward before export comparison).
Apply/amendment replays retained from r2 still pass; history equality remains JSON-value
frozen-prefix proof (stated as such), raw cmp is the byte proof.

## C — typed assets and egress

Pages 200 + `text/html` asserted; root doctype + referenced assets; **both** referenced
assets (`index-*.js` → javascript type, `index-*.css` → text/css) 200/typed/nonempty;
five fonts 200/`font/woff2`/distinct sha256 body hashes (kept); egress nonzero exit **1**
with captured reason `dial tcp: lookup example.com … connect: network unreachable`
(asserted DNS/dial/network, not any client error). Dead `page/.html` line removed; python
stderr is redirected to a private log and any python failure counts FAIL/nonzero (pyok
outputs are compared to literal "1", so empty/traceback = FAIL).

## D — prerequisite and guard modes (all counted, real exits)

- Normal: `offline-r3-final.log` — **pass 60, fail 0, FINAL_NORMAL_EXIT=0**.
- Forced wrong: `offline-r3-forced.log` — **51 FORCED-FAILs, exit 1**.
- Injected wrong normal expectation (reset 999): `offline-r3-inject.log` — 1 counted
  prerequisite FAIL, **abort exit 1** before any cascade (no public tracebacks; python
  stderr private).
- Prerequisite/unreachable: container removed → probe exec fails → `HTTP ERR` counted
  FAIL, **abort exit 1** (`offline-r3-prereq.log`, guard=prereq). Reset/login/token/export
  envelope failures all route through the same counted abort path; export envelope
  (track/format_version) validated on every capture.

## Requirement→proof map (this gate)

R6/R8/R14/R15 (build/PORT/bind/health) — r1/r2 stand + r3 readiness 0.256s; R9/R10 —
offline function under network-none with caps inspected; R200–R208 explain; R220–R246
policy publication/explain/selection/no-retroactivity; R242–243/R249–252/R253–255 — terms,
revision, no-op/real/cutoff, expected_revision; R258–285 — history/decision/series
adoption/exceptions/replays; R305–R348 — preview/apply/closure/Reassigned/counters
(offline gate binds preview/apply world by hand-computed expectations with export deltas);
R349–350 concurrency/other-restaurant NOT exercised here (inherited from stage4-api
632/0 in r1/r2 against the same frozen bytes — context, not this gate). Explicit
non-claims: no native import (I1 blocker), no R380–382, no formal S4-R1.

## Cleanup and ledger

Own container `tk-s4-p2d-r3-offline` removed; absence confirmed (docker ps -a count 0);
images `tablekeeper:s4-p2d-r2` (c61a81282c70) and `tablekeeper:s4-p2d` (9fb6c9a7c39e)
retained; no prune/cache/shared deletion. Candidate repo clean at close (HEAD advanced by
coordinator metadata as permitted; archived objects authoritative). Elapsed 18:42–18:57Z
≈ **15 minutes** foreground. Usage figures not visible in this runtime.
