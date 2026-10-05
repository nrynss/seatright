# S3-R1 — Formal review round 1 (of 3), stage 3

**Verdict: ACCEPT**

Every requirement I checked holds at the exact frozen SHA, all supplied checks pass, all
four stage-3 suites pass isolated, and the full independent battery (stage-3 additions,
inherited regression, three-lane migration, browser/design, both true upgrades) is green.

- Candidate: `/home/nryn/work/seatright/runs/tablekeeper2/wt/review/stage-3`
- Frozen SHA: `e13272d90213be0914211ae6f6f0bfd5888900ff` — HEAD + clean `git status`
  verified at start (10:34:57Z), re-verified mid-review and at the end (11:12:12Z).
- Accepted stage-1 tree `8b8b28da…` and stage-2 tree `9fee3dc7…` confirmed unchanged.
- My images (all cache-hit, byte-identical to the harness/owner builds of the same frozen
  bytes): `s3r1/stage3:cand` = `6d07453dbf2f` (144MB), `s3r1/stage1:donor` = `e2d8b54aed4b`,
  `s3r1/stage2:donor` = `25baa81f42b8`. Reviewer containers/images/network removed at the end.
- Review window: 2026-10-05T10:34:57Z → 11:12:12Z (~37 min foreground).

## 1. Supplied check (verbatim, isolated)

Exit 0. `checks/report.json`: revision = frozen SHA, mode `isolated`, state `completed`,
stage 1 **120/120**, stage 2 **25/25**, stage 3 **7/7**, stage-4 probe **fail (expected)**,
`highest_contiguous: 3`, `claimed_stage: 3`, share 1.0. All logs retained.

## 2. Source gates (staging copies; candidate untouched)

- `gofmt -l internal` clean, `go vet ./...` 0, `go test -count=1 ./...` ok (service 116s),
  `go build ./cmd/tablekeeper` 0 — `notes/go-gates.log`.
- `go test -race -count=1 ./...` in `golang:1.27-bookworm` (host lacks gcc): first attempt
  hit Go's default 10-minute test timeout (600.041s, FAIL — infrastructure/timeout, output
  summarized here and not retained as a file; disclosed); rerun with `-timeout 40m` →
  **all packages pass incl. service 575.394s, race_exit=0** — `notes/go-race.log`.
- UI: npm ci 0, svelte-check 0 errors, **90/90 tests** (Chaaya contrast+a11y gates), build 0
  — `notes/ui-gates.log`.

## 3. Delivery and offline

- Default 8080 healthy in 0.27s with exact `{"status":"ok"}` and
  `application/json; charset=utf-8`; `PORT=9555` works; 0.0.0.0 verified via container-IP
  request from another container; 2CPU/2GiB caps inspected (review.log 10:40).
- Offline `--internal` network: reset/signup/login/booking/index/CSS/JS (stage-3 bundle
  `index-CCGk2hmV.js`)/5 woff2 fonts all 200; **policy publication 201, explain (hourly
  grid, fixture-order tables, policy_version), pair booking under selected capacity,
  owner history (table_ids-created) and decision (6-key terms), series adoption (weekly
  calendar) and owner GET — all offline**; egress ConnectError
  (`probes/r1_offline_probe.py` + `s3_offline_extra.py`, exit 0).

## 4. Independent stage-3 API probe — 217/217, exit 0

`probes/s3_api_probe.py` (`notes/s3-api-probe.log`). Coverage highlights (my own
expectations — several initial FAILs were my probe's arithmetic/2026-DST-date errors, each
corrected after manual curl verification showed the service correct; the failed-attempt
history is in review.log):
- Policies: 401/403/404 precedence; 18-case invalid-policy matrix all 422 with no
  version/state; versions 1→2→3 per restaurant; failed publications and replays allocate
  nothing; replay 200 identical; same-key-different-body 409; listing publication order;
  original detail unchanged; selection by greatest effective_from ≤ date with
  same-date-greatest-version supersession (15-min grid live), policy 0 before first
  effective date, past dates accepted and governing, selected capacities authorize party 4
  on t_1 (cap 4) while original cap is 2.
- Explain: `false`/`1`/empty → 422; no explain fields without the param; every table once
  in fixture order; rules capacity→no_overlap both reported; available == conjunction ==
  available_table_ids order; occupied pair blocks both members with capacity still true;
  zero-available slots keep full explain; closed day `[]`; policy_version per slot.
- Dated writes: revision 1 + 6-key accepted_terms (no effective_from); ends_at absolute
  under the selected 60-minute policy; off-grid 422 under the new grid; real amendment →
  rev 2 → resulting-date terms; no-op retains bytes/revision/counter (also after newer
  publications); expected_revision bool/string/0/fraction → 422, huge positive → 409
  before invalid fields; matching applies; 8-way same-expected race → exactly one 200 real
  winner + seven 409; cancel rev+1; repeat cancel nothing; decision current after cancel;
  history/decision 404 (foreign/no-token/bad-token) while ordinary lookup stays 401.
- History: created entry scalar/table_ids selector per shape, from=null, ordered; changed
  only-changed ordered fields (incl. table_id+starts_at_local+party_size move); frozen old
  terms; RFC3339 numeric offsets; no-op/replay add nothing; cancelled `changes: []` and
  nothing after; cancelled history readable.
- Series: 404 unknown/foreign anchor, 401 no token; count/interval boundary and type
  matrix; adoption 201 with calendar dates (+7d), anchor identity/terms/receipt preserved,
  occurrences occupy tables and appear in lists; owner-only GET 404; real PATCH → permanent
  exception + series rev+1 (no-op changes neither); cancel → series rev+1 without exception,
  repeat nothing, anchor cancel independent; replay byte-original after edits/cancels;
  already_in_series 409; DST 2027 Berlin skip → 422 invalid_local_time with byte-identical
  rollback and reusable key, Berlin fold → first-instant booking (2027-10-31 +02:00),
  Auckland 2027 skip → 422; generated occupancy enforced; accepted cutoff blocks adoption.
- Collective: swap with per-move expected_revision; batch counter once; conflicting batch →
  409 with byte-identical export and reusable key (a first probe attempt used an off-grid
  time — the service correctly 422'd `not_on_slot_grid`; corrected to a genuine overlap);
  all-no-op batch 201 with zero metadata (only the replay receipt added); stale-in-input-
  order 409; series propagation: two members of one series + one of another moved in a
  single batch → restaurant counter exactly once, each affected series exactly once,
  permanent exceptions on changed members only, one changed history each; replay
  byte-original; member cancel → series rev+1 with flag retained.
- Races: 50 same-key creates → 1×201 + 49×200; no 5xx anywhere (statuses seen: 200/201/204/
  401/403/404/409/422).

## 5. Inherited regression on the stage-3 build — 346/346, exit 0

`probes/r1_api_probe.py` (full stage-2 matrix: envelope/types/booking/cutoff/DST/pairs/
moves/export-import/limits) passes unchanged against the stage-3 image
(`notes/s2-regression-probe.log`) — R1–R199 preserved.

## 6. Migration — 51/51, exit 0

`probes/s3_migration_probe.py` (`notes/s3-migration-probe.log`), four distinct processes:
- Genuine stage-1 donor (fresh build from the accepted tree, staged state incl. past and
  cancelled seeds, create/move receipts, failed key, two sessions) → stage-3 destination
  with replacement semantics: tenant wiped, hash login + both tokens live, records
  normalized revision 1 / fixture-policy-0 terms / one reconstructed created entry at the
  retained created_at with donor-shaped selector, current mutation (rev 2) does not disturb
  the **original receipts replaying byte-exact**, failed key reusable, unused-then-valid
  first-201-then-200, **imported anchor adoption 201** with identity preserved.
- Genuine stage-2 donor (pair lane) → same checks with table_ids shapes.
- Modern stage-3 producer (policy + supersession, pair→single→pair history, adopted series,
  occurrence PATCH exception + cancel, mixed collective batch) → second stage-3: full
  export equality on reimport, **5 genuine captured HTTP originals replay byte-exact
  (including the manager-only policy publication)**, stored policies/series flags/histories/
  counters verified, corrupt import 422 atomic, malformed 400, reset clears.

## 7. Context probes (implementer work, corroborating only)

`stage3-api.sh` **245/0** exit 0; `stage1-html.sh` **16/0** exit 0.

## 8. Browser and design — all green

- F1–F4 closure driver: **121/121, exit 0** (auth links incl. blank-name session; reveal
  measurements with recorded scroll — caption+actionable cells after search, keyboard
  selection reveals form fully, confirmation fully visible with form retained, 409 no
  upward re-reveal, uncertain/empty visible on phone 900/812, party-typing no hijack, stale
  search guard, reduced motion immediate; one wrapped human name per plate + 16px seat
  metadata; connector paint order with elementFromPoint ink sampling).
- States driver: **132/132, exit 0** (all named states, light/dark/system, 375/1280).
- Measurements: **59/59, exit 0** — every state word ≥4.5, selected pair **7.611 light /
  9.915 dark**, density 15/75 and 43/215, page width fixed, reduced motion immediate.
- Policy-driven grid in the browser: **7/7, exit 0** — party 8 shows all singles
  unavailable, pair cells available, hourly 18:00–22:00 slots from the selected policy,
  live party-8 pair booking succeeds, original detail unchanged
  (`shots/s3-policy-grid-d.png` — visually verified: "10 seats together", held-pair
  contrast, human labels).
- True upgrades: stage-1 UI → stage-3 **19/19** and stage-2 UI → stage-3 **19/19**, both
  exit 0: same-Document commit-dropped 201 → export/import between requests → no-reload
  retry on the strict stage-3 importer returns the exact original legacy receipt (stage-1
  ten-field; stage-2 singleton `table_ids`+`table_id`), original reference, session and
  old-token lookup live, exactly one booking.
- Design critic (fresh curated set): **VERDICT-CRITICAL: NONE**, with independent
  confirmation of the header fix, settled reveals, single plate names, connector paint
  order and contrast (`notes/design-critic.log`).

## 9. Audits

No rate limiting (grep clean); no fixture-value branches in Go non-test code
(`search.ts DEFAULT_SEARCH_DATE` is the same sanctioned UX default as stage 2); bcrypt with
SHA-256 prehash; keel/id for ids/tokens; every API call via `@nrynss/chaaya/keel`; RUN.md
consistent with observed behavior (default PORT, offline probe client).

## 10. Honest limits

- The first race attempt's timeout-FAIL output was summarized rather than retained as a
  file; the passing rerun log is complete (`notes/go-race.log`).
- Stage-4 probe fails as required; no stage-4 behavior exists.
- My probe scripts' initial expectation errors (2026 DST dates, policy-hour grid starts,
  no-op-vs-real-change cases) are preserved in `review.log` with the manual curl
  verifications that ruled out product defects; final probe files contain the corrected
  assertions and pass end-to-end.
- Reviewer containers/images/network removed; Docker store 1.8G free; candidate clean at
  the frozen SHA after cleanup.

## 11. Counts

Supplied: 120+25+7 (claimed 3, highest 3, stage-4 fail). Independent: API 217 + inherited
346 + migration 51 + closure 121 + states 132 + measure 59 + policy-browser 7 + upgrades
19+19 = **971 assertions, 0 failures**. Context: 245 + 16. Gates: go fmt/vet/test/build 0,
race 0, UI 90/90. Elapsed ~37 minutes foreground; token/cost usage not visible in this
runtime.
