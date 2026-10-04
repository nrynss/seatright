# REVIEW S1-R2 — Tablekeeper stage 1 (re-review, round 2 of 3)

- Reviewer: Seatright-ZCode (independent)
- Date: 2026-10-04 (UTC evening)
- Candidate: `/home/nryn/work/seatright/runs/tablekeeper2/wt/review/stage-1`
- Frozen SHA: `b298700f790c166cf7ce8d98d731c80093ecb9af` (verified with `git rev-parse HEAD` before any review step; `git status --porcelain` clean). History since round 1: `5fd9979` (B1 fix, author Grok) merged via `c1e5ba0`; diff scope is exactly `web/src/lib/auth.ts`, `web/src/lib/components/LoginScreen.svelte`, `web/src/test/{live,login-copy}.test.ts`, `web/scripts/s1-h2-b1-login.mjs`, `PLAN.md` — no Go, packaging or routing changes.
- Mode: isolated (supplied harness); candidate read-only; no code edited by me.
- Round-1 verdict was CHANGES with one blocking finding (B1). This round re-ran everything, not only the fix.

## Verdict

**ACCEPT** — B1 is resolved and verified end to end; all supplied checks pass (120/120); all round-1 independent probes reproduce on the new SHA (186 API + 32 export + 139 candidate + 76 browser assertions, 0 failures); no regressions; no new blocking finding. Prior non-blocking notes remain non-blocking (see closure notes below).

## B1 verification (the round-1 blocker)

- **Code review**: `loginFailureMessage(error)` maps only `ApiError` with code `unauthenticated` to the fixed sentence `CREDENTIAL_REFUSAL = 'Email or password is incorrect.'`; every other code (including lost connections) keeps `authMessage`. The error object, code, status and server message are untouched; signup and general errors unchanged. The mapping is precise: on `/auth/login`, code `unauthenticated` means exactly a failed credential check.
- **Tests strengthened, none weakened**: live.test.ts now mocks the real server jargon ("missing or invalid bearer token") and asserts the UI shows exactly the diner sentence with no "bearer"/"token" substring; new login-copy.test.ts (4 tests) covers error-object integrity, wrong password + unknown email through the Keel adapter, lost-connection-not-wrong-password, and successful login clearing the refusal. Suite: **7 files, 56/56** (was 52).
- **Live browser (my driver, both themes)**: wrong password → exactly `Email or password is incorrect.` (no jargon); unknown email → same sentence; aborted request → different message (not the credential sentence); valid login → `current-user` appears and `auth-error` is gone. Screenshots: `design/375-light-login-error.png`, `design/w375-dark-login-error.png`, `design/verify-light-login-error.png`, `design/verify-dark-login-error.png`. Rendered as a soft red/refusal banner, on-token, legible.

## Supplied checks (exact command, fresh output dir)

```
cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run \
  --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 1 \
  --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S1-R2/checks
```

- stage 1: **pass 120/120**; stage 2: fail 1 (combinations — expected); highest contiguous 1; **claimed stage: 1**; report.json revision = `b298700f790c166cf7ce8d98d731c80093ecb9af`, mode isolated. Logs: `checks/stage-1.log`, `checks/stage-2.log`, `checks-harness-stdout.log`, `checks-harness-stderr.log`.

## Independent verification — commands and counts

Staging provenance: UI gates ran in a fresh copy of `stage-1/web` taken from the frozen candidate at SHA b298700f (`S1-R2/staging/web/`, copy recorded in `staging/npm-ci.log` context); nothing was installed or built inside the read-only candidate.

| Command | Result | Evidence |
|---|---|---|
| `git rev-parse HEAD` / `git status --porcelain` | b298700f…, clean | this file, logs |
| `docker build -t tablekeeper:s1-r2 .` | exit 0, 143 MB | `probes/docker-build.log` |
| `go test -mod=readonly -count=1 ./...` (fresh, no cache) | ok (race variant impossible: sandbox has no C compiler — same limitation as round 1) | `probes/go-test.log` |
| containers `tk-r2-a` PORT=9051, `tk-r2-b` PORT=9052, `tk-r2-off --network none` PORT=9054, `tk-r2-lim --cpus=2 --memory=2g` PORT=9053 | all healthy ≈2 s after start | `probes/resource.log` |
| offline: `docker exec tk-r2-off /app/probe -url …/health`, `/`, `/lookup` | 200 ×3, correct content types | `probes/resource.log` |
| resource run (exact commands + raw timings) | 50 mixed (25 bcrypt signups + 25 reads) all 2xx in 0.82 s; 50 concurrent reads all 200, max latency 19.5 ms; booking 201 in 1 ms; health 200 after | `probes/resource.log` |
| `sh probes/stage1-api.sh http://localhost:9051 …` | pass=101 fail=0 | `probes/cand-api.log` |
| `sh probes/stage1-html.sh …` | pass=16 fail=0 | `probes/cand-html.log` |
| `sh probes/stage1-export.sh http://localhost:9051 http://localhost:9052 …` | pass=22 fail=0 | `probes/cand-exp.log` |
| `python3 probe_api.py http://localhost:9051` (my suite, unchanged from round 1) | **pass=186 fail=0** | `probes/probe_api.log` |
| `python3 probe_export.py http://localhost:9051 http://localhost:9052` | **pass=32 fail=0** | `probes/probe_export.log` |
| `npm run check` / `npm test` / `npm run build` in staging | 0 errors 0 warnings / **56/56** / dist built | `staging/ui-check.log`, `staging/ui-test.log`, `staging/ui-build.log` |
| skipped/weakened test scan (`it.skip/describe.skip/todo`) | none | inline output |
| browser driver `design-pass.mjs` (72 round-1 assertions + 4 new B1 assertions) | **pass=76 fail=0**, 27 primary screenshots, 3 recordings | `design/design-pass.log`, `design/design-assertions.json` |
| dark-theme capture pass | done; **55 screenshots total, 0 duplicate hashes** | `design/dark-pass.log` |
| design critic (`/opt/seatright/agy --model gemini-3.8-flash-high --dangerously-skip-permissions -p "…"`) in the round-2 screenshot folder | raw output saved | `design/design-critic-output.txt` |

Evidence-quality note: the first `probe_export.py` invocation in this round accidentally ran against stale round-1 containers (script had hardcoded 9031/9032). I caught it, parameterized the ports, removed the round-1 containers, and re-ran — the recorded 32/32 is against the frozen round-2 image pair. Round-1 evidence itself was unaffected (its containers matched its own candidate).

## Regression coverage (full re-run, not just B1)

Everything from round 1 was re-executed on the new SHA: reset semantics, error envelope and input strictness, auth (unicode/long passwords, email rules, multi-tokens), availability grid math and closed days, create/list/lookup/cancel/PATCH with occupancy, past-date booking, cutoff inclusive boundary and current-start amendment, no-op PATCH, DST (Berlin spring/fall incl. absolute end and first-occurrence, New York spring/fall, Lord Howe 30-min shift, UTC +00:00), idempotency (order, scopes, replay identity after cancel, invalid-different-body 409, failed-key reuse, unauthenticated non-consumption, 50-concurrent exactly-one-201, 20 distinct keys one-201), moves (swap, self-exclusion, rollback, shape violations, ordering precedence, cancelled/cutoff, replay), cross-process export/import (receipts, tokens, hashes, replacement, atomic failures, reset-clears-imported), no-5xx fuzz, offline function, resource limits. **All pass; no regressions.**

## Design pass (my judgment, round 2)

Same visual system as round 1 (only the login error copy changed): warm coherent palette, human-readable dates/times/labels, distinct loading/empty/error/refused/uncertain/confirmed presentations, inline SVG floor plan with capacity-sized tables and seats mirroring the grid, spring/flip/staggered motion with reduced-motion honored (recording `design/videos/v375-reduced-motion.webm`), keyboard-operable plan tables, light+dark coherent. The login refusal is now a designed, diner-facing state in both themes. The booking form remains on screen after confirmation (explicit stage-2 requirement; re-verified). No horizontal page scroll at 375 (asserted), no off-origin requests (asserted over a full flow), no page errors.

## Design critic (second opinion, round 2)

Raw invocation + output in `design/design-critic-output.txt` (run inside the round-2 screenshot folder). It repeated the round-1 claims and added one new testable claim. Adoption decision, measured:

- **NEW — "dark banner text ~1.8–2.5:1 contrast"**: DISPROVEN by computed-style measurement on every dark error surface: text `rgb(243,168,162)` on `rgb(74,34,30)` = **7.11:1** (login/signup auth-error, signed-out refusal, booking-error, reservation-error); uncertain banner `rgb(240,194,122)` on `rgb(61,42,18)` = **8.26:1**; refused-banner link 5.71:1. All pass WCAG AA by wide margins. The critic's quoted colors do not exist in the app.
- Repeats already disproven in round 1 with numeric evidence (unchanged layout code since): colhead "10:0/0/PM" splitting (three stacked lines by design, 19.7 px each, constant 32 px columns), "Taken"/"Held" clipping (scrollWidth checks false), plan-pill "collision" (mid-flip capture artifact; settled DOM clean), "browser-default blue link" (measured 5.71:1 themed periwinkle).
- Spec-mandated behaviours it flags again: booking form kept after confirmation (stage-2 requires it); lowercase exact status text (stage-2: "Text is exactly `confirmed` or `cancelled`").
- Non-blocking observations re-confirmed as taste/polish notes (unchanged from round 1): auth links in nav while signed in, tall wrapped 375 header, "Taken" wording for capacity-excluded tables, server-message casing/punctuation, cancel button active after cutoff refusal, minimal empty-state illustration, error-banner placement variance.

No critic point rises to blocking; the one round-1 blocker (B1) is fixed.

## Prior findings — status

- **B1 (blocking, round 1)**: RESOLVED and verified (code, 4 strengthened tests, live DOM both themes/widths, screenshots). No regression elsewhere.
- Round-1 non-blocking notes 1–2 and 4–7: remain open as taste/polish (coordinator deferred unrelated redesign — accepted; none is a spec violation).
- Round-1 note 3 (duplicate table label in `confirmation-details`): **withdrawn** — stage-2 requires `confirmation-details` to contain the restaurant name, table label and local start time, so the label's presence there is mandated; the dedicated `confirmation-tables` element is also present. Coordinator's reading is correct.
- Round-1 note 8 (race-compiler limitation): unchanged environment; recorded again.

## Requirements not fully verifiable (and why)

- R10/R13 boundary values: verified functionally with large headroom under `--cpus=2 --memory=2g`; exact saturation/10 s boundary not measurable in this sandbox.
- R43 token non-expiry: implementation stores no expiry (code-inspected); many concurrent tokens and post-import validity verified.
- R121 provenance: audited — self-authored code only; not absolutely provable.
- `go test -race`: not executable in this sandbox (no gcc); plain fresh `go test -count=1` passes and HTTP-level concurrency was probed end to end.

## Elapsed / usage

Elapsed ≈ 45 minutes foreground. Usage figures not visible in this harness. No human consulted. Evidence root: `/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S1-R2/` — `checks/`, `probes/` (incl. `resource.log`), `design/` (55 screenshots, `videos/` ×3, critic output, assertion JSON, logs), `staging/` (gates + drivers), `notes/`, this REVIEW.md. Round-1 evidence untouched under `S1-R1/`. No private exports or tokens committed.
