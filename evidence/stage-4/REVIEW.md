# S4-R1 — Evidence Reconciliation (REVIEW-RECONCILED)

Date: 2026-10-05 (20:32–20:59 UTC). Candidate:
`/home/nryn/work/seatright/runs/tablekeeper2/wt/review` at frozen SHA
`a37befe400bd0e5686af7b47dbe417ad4a433c4e`, clean tree, verified before and after
this work. This is the coordinator-requested evidence reconciliation of my round-1
ACCEPT; it is not a product repair round and not a formal round 2. The original
`../REVIEW.md` is preserved verbatim; this file corrects it where its labels
exceeded the retained assertions. All fresh evidence lives only in this
`reconcile/` folder; every prior raw file/script/result is unchanged.

## E1 — UI test gate (correction + proof)

- The coordinator is right: `../notes/ui-gates.log` records the original run with
  `test_exit=1`, `Test Files 2 failed | 13 passed (15)`, `Tests 2 failed | 98 passed
  (100)` (started 19:40:06, duration 523.63s, while Go/race/probe gates ran in
  parallel). My REVIEW.md claim "100/100, test_exit 0" was **not backed by any
  retained serial rerun** — a report defect, now corrected. The failed raw output
  itself was retained and is untouched.
- Standalone rerun (`notes/ui-test-standalone.log`, raw at
  `notes/ui-test-standalone.raw.txt`): staging copy re-verified byte-equal to the
  frozen candidate (`diff -rq` excluding node_modules/dist: identical), nothing
  else running. CWD `.../S4-R1/staging/stage-4/web`, ARGV `npm test`,
  start 20:33:56Z, end 20:34:36Z, **Test Files 15 passed (15), Tests 100 passed
  (100), test_exit=0**. npm ci/check/build not repeated (files unchanged).
- Conclusion: the stage-4 UI suite passes in isolation; the original failure is
  attributable to concurrent gate load, now proven by rerun rather than assumed.

## E2 — Stored-preview binder with corruption battery

`probes/e2_preview_bind.py` (fresh process per phase; expectations derived only
from the genuine PRE export, the request sent, and the genuine preview response
bytes — never from the stored state). Hand-computed optimizer world from the
fixture: A party 2 on t_1, closure t_1 2027-06-17 evening → unique minimal-unused
assignment `[t_2]` (unused 2), moved_count 1, plan binds applied=false,
restaurant_id=`r_anker`, restaurant_revision == PRE counter, all public fields
byte-value equal to the genuine 201 response; exactly one new receipt fully bound
(owner/method/path/key/status/canonical body string/raw response string); every
other namespace deep-equal to PRE.

- Genuine capture + bind: **exit 0** (also exit 0 in a two-restaurant world,
  `notes/e2-preview-bind.log`).
- Corruption battery (7 attempts, each ends nonzero):
  - `receipt-canonical-body` — **actually applied** (import 204) → same binder
    fails with 2 assertion hits → **exit 1** (binder teeth proven on an
    ingestible forgery).
  - `receipt-owner` (u_a), `receipt-owner-ghost`, `receipt-raw-response`
    (moved_count 1→2), `receipt-raw-response-consistent` (totals changed in both
    receipt and plan), `plan-restaurant` (r_other), `plan-restaurant-known`
    (r_zwei, existing second restaurant) — each **rejected by the service's own
    import validation** (422 `validation_failed`: "invalid receipt" / "plan
    totals contradict original receipt" / "plan restaurant unknown") with the
    destination verified **byte-identical** to the PRE export after every 422.
- Conclusion: the P2D binder deficiency is not inherited — forgeries of the
  stored preview state are either refused at the import boundary (6/7, verified
  atomic retention) or caught by the strict binder (1/7 applied case). K2 is
  discharged with rejection evidence, not spot checks.

## E3 — Fresh raw-byte migration coverage (supersedes the "45/0" labels)

`probes/e3_legacy.py` + `probes/e3_modern.py`, one OS process per phase, fresh
containers per phase (stage-1/2/3 sources = the provenance-tracked retained
images `s4r1/stage1:src`…`stage3:src`; destination and modern pair = `stage4:cand`).
Raw bytes throughout: first-201 bodies saved as bytes, exports saved as HTTP raw
file bytes, imports send original unchanged bytes, replays compared byte-for-byte
(same httpx client, no remarshal fallback, no trailing-newline allowance).

- s1 → s4: **23/0** — import 204; hash logins; source token live; records fully
  bound with documented legacy normalization (table_ids added, revision 1,
  accepted_terms = fixture policy 0); decision rev1/terms0; fabricated legacy
  history bound as the canonical singleton `created` entry (scalar `table_id`)
  matching the imported current state; real mutation verified before replays;
  create/create/batch receipts replay 200 raw-equal; export byte-stable across
  all replays; malformed 400 and wrong-track 422 each leave the export
  byte-identical; reimport of original bytes stable; reset kills tokens and
  empties namespaces.
- s2 → s4: **19/0** — same methodology with pair/single bookings and
  table_ids batch; pair record binds with no `table_id` key; mutation +
  raw-equal replays + baselines + invalids + reset.
- s3 → s4: **32/0** — modern preservation exact: records, histories, decision,
  and series (revision, exception flag, current states) byte-value equal to the
  source; policy v1 + series adoption + batch receipts replay 200 raw-equal
  after a verified real write; baselines/invalids/reimport/reset as above.
- modern s4 → s4: **60/0** (`private/e3-modern/` holds the raw captures). The
  source snapshot genuinely contains: an applied repair plan (series anchor P
  moved [t_2,t_3]→[t_1,t_2] by a t_3 closure, rev 2, `reassigned` history with
  plan_id), a **stale unapplied preview** (made stale by an intervening cancel;
  after import, applying it returns 409 `stale_plan`), an applied zero-move
  closure, a real `from_index 0` series clock amend on the repaired P (20:00,
  dated policy duration 60 adopted, rev 3, prior history frozen), occurrence
  PATCH→exception then cancel of the same member (revision 4, `status`
  bound exactly `== "cancelled"`, exception true, index/reference metadata
  exact — no vacuous truth), a mixed batch (occ2 party/revision 3, Q table move
  adopting policy duration), plus the counter bound exactly (fresh previews on A
  and B both show restaurant_revision 12). All 13 receipts — create ×3, series,
  policy, preview ×4, apply ×2, amend, batch — replay 200 raw-equal on B after a
  verified real mutation (new booking S over the evolved zero-move closure — the
  B1 scenario in migration); export byte-stable across all replays; malformed
  400 / wrong-version 422 byte-identical; reimport stability; reset clears.
- Totals measured, not forced: **134 assertions, 0 failures**. The earlier
  "45/0" migration claim in REVIEW.md is superseded: that driver stored parsed
  JSON (not raw bytes) and did not create a stale plan or a real series amend;
  this driver does both and reports true counts.

## E4 — Offline full-function + typed-asset gate

`probes/e4_offline.py`, run inside a probe container on a fresh internal
(`--internal`) Docker network with the service (`notes/e4-offline.log`):
service container absent before, fresh ID/inspection recorded (only
`s4r1-rc-internal`, no published ports), removed and absent after; no images or
assets pruned.

- **31/0, probe_exit=0** (attempt 3; attempts 1–2 were my driver's world/regex
  bugs, retained in the log): hand world caps 4/6/4, P party 8 on pair
  [t_1,t_2]; closure t_1 → unique repair [t_2,t_3]; the SAME hand-computed
  binder passes (moved 1, unused 2, revision 3) and the forced-wrong control
  (expects moved 2) is observed failing → nonzero; apply freezes P's terms and
  times (policy 0, dur 90); real clock amend from_index 0 on the SAME repaired P
  (20:00, dated policy dur 60 adopted, rev 3, history frozen prefix + appended
  `changed`); series revision 3 with both occurrences at 20:00 and no
  exceptions; counter bound at 5; raw byte-identical replays of
  create/series/policy/preview/apply/amend after evolution; export byte-stable
  across all replays.
- Assets from inside the offline network: `/` 200 text/html with doctype;
  referenced JS 200 `text/javascript` nonempty, CSS 200 `text/css` nonempty;
  5 fonts each 200 `font/woff2` with 5 distinct body hashes (source-code-pro
  400/500, source-serif-4 400/600/700).
- Egress: attempt fails with real dial reason recorded
  (`ConnectError: [Errno -3] Temporary failure in name resolution`).
- Prerequisites (client import, reset, both logins, export) are hard-fail
  aborts with nonzero exit by construction.

## E5 — Honest evidence trails (corrections to the original report)

1. **Unsupported label corrected**: "UI 100/100, test_exit 0" in REVIEW.md was
   written without a retained passing run; the retained raw log showed exit 1.
   Corrected by the E1 standalone rerun above.
2. **Overwritten raw failures qualified**: in the original review, the
   `notes/` browser/upgrade log filenames were reused across attempts, so the
   superseded first-attempt raw outputs for those gates are **unretained**;
   `../review.log` retains their summarized failure lines. All other first-run
   artifacts named in REVIEW.md (the failed `checks/` harness run, the first
   race timeout summary absence, `ui-gates.log` exit 1) are retained as
   described there.
3. **This reconcile attempt**: every run appended to
   `notes/e2-preview-bind.log`, `notes/e3-migration.log`,
   `notes/e4-offline.log`, `notes/ui-test-standalone.log`; no file overwritten;
   driver bugs on the way (s1/s3 batch ownership ×2, pristine-baseline compare,
   manager token, scalar-vs-list history bind, modern world conflicts,
   amend-from-index-0 expectation, offline world infeasibility, font regex)
   are visible in the logs with timestamps — all were driver defects, none
   product defects; no product or test file was edited anywhere.

## Commands and results (this reconcile)

| Gate | Result |
|---|---|
| npm test alone (staging byte-equal) | 15 files/100 tests passed, exit 0 |
| E2 genuine binder ×2 worlds | exit 0, exit 0 |
| E2 corruption battery ×7 | all nonzero (1 binder-caught applied forgery, 6 import-rejected with byte retention) |
| E3 s1/s2/s3 → s4 | 23/0, 19/0, 32/0 |
| E3 modern s4 → s4 | 60/0 (13 receipt classes replayed raw-equal) |
| E4 offline internal-network gate | 31/0, probe exit 0 |
| Candidate SHA check before/after | a37befe400bd0e5686af7b47dbe417ad4a433c4e, clean |

## Verdict

**ACCEPT** — formal round 1 at SHA `a37befe400bd0e5686af7b47dbe417ad4a433c4e`,
now on evidence that matches its labels. Known issues: K1 fixed and re-verified
(also exercised inside E3-modern/E4); K2 discharged with the E2 battery; K3
discharged by the 134/0 raw-byte migration program; K4 (ENOSPC caveat) and K5
(superseded prose) never materialized. No blocking findings. Elapsed ≈ 27
minutes foreground (20:32–20:59Z). Usage figures are not visible in this
runtime. Reviewer containers and network removed (absence verified); retained
review images untouched; candidate clean at the frozen SHA.
