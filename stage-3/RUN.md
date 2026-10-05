# Tablekeeper — stage 3 service

Single-container HTTP reservation service with combined tables, dated booking
policies, reservation history and recurring reservations, plus a browser UI:
search availability (singles and declared pairs), book/amend/cancel tables,
and change several bookings together atomically. Recurring series adoption is
an API capability (`POST /series`); the browser exposes search, booking,
lookup and cancellation, not series management. The same
Go service serves the API and the static web build (`web/dist`, built with
Node 26).

## Run with Docker (no manual setup)

```sh
cd stage-3
docker build -t tablekeeper:stage-3 .
docker run --rm -e PORT=8080 -p 8080:8080 tablekeeper:stage-3
```

The service listens on `0.0.0.0` at `$PORT` (default `8080`; see
`cmd/tablekeeper/main.go`, which binds `0.0.0.0:$PORT`). A non-default port
works the same way:

```sh
docker run --rm -e PORT=9027 -p 9027:9027 tablekeeper:stage-3
curl localhost:9027/health
```

No volumes, no environment beyond `PORT` (and optional `WEB_DIR`, default
`web/dist`, `/app/web/dist` in the image), no outbound network at run time.
State is ephemeral: restarts lose state, and `POST /_test/reset` or
`POST /_test/import` replace everything. The image is built in stages —
Node 26 (`npm ci` + `npm run build` with the locked `package-lock.json`) for
`web/dist`, Go 1.27 for the static service binary — and the runtime carries
only the binary, zoneinfo (IANA time rules), `web/dist` (scripts, styles,
five local `woff2` fonts under `web/public/fonts` — nothing loads from the
network) and a tiny `/app/probe` HTTP client used for offline checks. The
container's server port is configured with `-e PORT`; probe it from inside
its own namespace, which works with `--network none`:

```sh
docker run -d --name tk-off --network none -e PORT=9027 tablekeeper:stage-3
docker exec tk-off /app/probe -method GET -url http://localhost:9027/health
docker stop tk-off && docker rm tk-off
```
`/app/probe` is built from `probes/pclient/main.go` (flags `-method`, `-url`,
`-body` string, `-token`, `-key`, `-timeout` Go duration).

## Run from source

Requires Go 1.27+ and Node >=26 <27. Build the web bundle first so the served
UI works as well as the API, then start Go:

```sh
cd stage-3/web
npm ci && npm run check && npm test && npm run build
cd ..
go test ./...
go build -o tablekeeper ./cmd/tablekeeper
PORT=8080 ./tablekeeper
```

Without `WEB_DIR` the service serves `web/dist` relative to the working
directory, so start from `stage-3` after building the web bundle; page routes
fall back to a JSON 404 when the build is absent. Backend: Go 1.27 with Keel
v0.5.0 (`keel/id` for opaque ids and tokens; no rate limiting). Frontend:
Svelte with @nrynss/chaaya 0.3.0 — API calls go through the Keel adapter,
styling through the token system with light, dark and system themes. An empty
startup needs no fixture: the service starts with empty state and
`POST /_test/reset` loads restaurants, tables, combinable pairs,
`manager_user_ids`, users and seeded bookings (see `## Test control`). There
are no external runtime dependencies.

## API overview

The `application/json; charset=utf-8` convention applies to the API. Page
routes (`/`, `/signup`, `/login`, `/lookup`) and local bundled assets are
HTML/JS/CSS, not JSON.

- Errors share one envelope: `{"error": {"code": "...", "message": "..."}}`
  with the specified HTTP status and stable `code` (no string matching needed).
  Unknown body fields and query parameters are ignored, never errors.
- Public endpoints needing no bearer token: `GET /health`,
  `POST /_test/reset`, `GET /_test/export`, `POST /_test/import`,
  `POST /auth/signup`, `POST /auth/login`, `GET /restaurants`,
  `GET /restaurants/{id}`, `GET /restaurants/{id}/policies`,
  `GET /availability`. Everything else needs
  `Authorization: Bearer <token>`.
- Auth: `POST /auth/signup` (409 `email_taken`, short passwords and bad emails
  are 422) and `POST /auth/login` (401 on wrong credentials). Tokens never
  expire; accounts hold many concurrent tokens. Passwords are stored hashed
  (SHA-256 prehash, hex-encoded, fed to bcrypt); plaintext is never persisted. Bearer tokens grant their owner's
  bookings only; other owners' references read as 404.
- Cutoff: cancel, PATCH and collective moves are refused with 409
  `cutoff_passed` once now reaches the booking's start minus its **accepted**
  `cancellation_cutoff_minutes` (the cutoff from the booking's own accepted
  terms, checked against its current start — not the newest published policy).
- Availability: `GET /availability?restaurant_id=&date=YYYY-MM-DD&party_size=`
  lists every grid slot from opening whose absolute duration finishes by
  closing, with eligible `available_table_ids` (singles only, fixture order)
  plus `available_options`: every eligible singleton in fixture order, then
  every eligible declared pair in `combinable` order (`table_ids` in declared
  order, capacity summed). Closed days return `"slots": []`. Grid, duration,
  hours and capacities come from the **dated selected policy** for the queried
  local date, not from the restaurant detail.
- `GET /availability` with `explain=true`:
```http
GET /availability?restaurant_id=r_anker&date=2026-09-24&party_size=4&explain=true
```
adds per-slot `explain`: every table of the
  restaurant exactly once in fixture order, each with its effective
  `policy_version`, `available`, and both rules in `capacity`, `no_overlap`
  order (`available` is the conjunction, matching `available_table_ids`).
  `explain` accepts only the literal value `true`; `false`, `1`, the empty
  string and anything else are 422. Without `explain` the response keeps the
  earlier shape with no explanation fields.
- Booking: `POST /reservations` (idempotent, see below) returns a `reference`
  of 6–12 `A-Z0-9` characters plus RFC 3339 `starts_at`/`ends_at` with explicit
  offsets, `revision` (1 at creation) and complete `accepted_terms` (the full
  selected policy snapshot: `policy_version`, grid, duration, cutoff, opening
  hours, capacities — never `effective_from`). `GET /reservations`,
  `GET /reservations/{reference}`,
  `POST /reservations/{reference}/cancel` and `PATCH` amend time/tables/party.
  Every current response carries `table_ids`, and carries `table_id` only for
  a singleton set. A replayed idempotency receipt keeps its stored original
  JSON verbatim, so an old stage-1 receipt may lack `table_ids`,
  `revision` and `accepted_terms` while current records always carry them.
- Table sets: write `table_id` for one table, or `table_ids` with one id for
  a singleton or two ids for a declared pair — never both fields at once
  (422). A pair must be exactly one declared unordered `[t_a, t_b]` entry of
  that restaurant; declaring `[t_1,t_2]` and `[t_2,t_3]` does not authorize
  `[t_1,t_3]`, and three-or-more tables are never combinable. Reversed input
  names the same set and reads back in declared order. Capacity is the sum of
  the members **under the selected dated policy**. Cancelling frees every
  member at once.
- Local time: `starts_at_local` is bare `YYYY-MM-DDTHH:MM` in the restaurant's
  IANA zone. Skipped spring-forward times are 422 `invalid_local_time`;
  repeated fall-back times resolve to the first occurrence; durations are
  absolute elapsed time across DST.
- Test control: `POST /_test/reset` (204, replaces all state),
  `GET /_test/export` (200 with `track`/`format_version: 1`/`state`) and
  `POST /_test/import` (204 replacement, 422 without mutation). Exports are
  opaque and private: they contain password hashes and bearer tokens, so keep
  them under `evidence/`, never committed.
- Earlier-stage compatibility: this service accepts unchanged exports from
  this team's accepted stage-1 and stage-2 services. Accounts, hashes,
  sessions, fixture configuration, reservation identities, references,
  statuses, timestamps and completed idempotent receipts (including original
  stage-1 responses without `table_ids` and stage-2 `table_ids` shapes) are
  preserved; failed keys stay reusable. Import is replacement, not merge;
  reset clears imported state. Imported legacy bookings normalize to
  revision 1 under original fixture (policy 0) terms with one reconstructed
  `created` history entry; adoption works on imported anchors and original
  retries stay valid.

## Booking policies

Restaurants declare `manager_user_ids` in the reset fixture (default `[]`).
Only a manager may publish: `POST /restaurants/{id}/policies` with an
`Idempotency-Key` and a **complete** policy body (`effective_from`
`YYYY-MM-DD`, integer `slot_minutes` 1..1440,
`reservation_duration_minutes` 1..1440, `cancellation_cutoff_minutes`
0..10080, stage-1-shaped `opening_hours` without duplicate weekdays, and
`capacities` naming exactly the restaurant's tables with integers 1..100;
booleans are never valid integers; unknown fields are ignored). Success is
201 with the policy plus `policy_version` (1, 2, … per restaurant; failures
and replays allocate none). Unknown restaurant is 404, non-manager is 403,
missing token is 401, invalid policy is 422 with no state change. Policies
are immutable: table ids, labels, timezone and declared pairs can never
change through a policy.

`GET /restaurants/{id}/policies` is public and lists published policies in
publication order (policy 0 — the original fixture — is omitted). The
ordinary restaurant detail keeps its original fixture configuration and is
never mutated by publication. For a booking's local start date the service
selects the greatest `effective_from` not later than that date, breaking ties
by greatest `policy_version`; publication order and effective-date order may
disagree, same-date republication supersedes for future decisions only, and
past effective dates are accepted. Publication never edits existing bookings,
their end times, revisions, terms or histories.

## Revisions, accepted terms and history

- A real amendment checks the old accepted cutoff first, then validates **all**
  merged fields against the resulting start date's policy, atomically
  replacing accepted terms and end time and incrementing `revision` once.
- A no-op amendment (empty, unknown-only, equal values, reversed pair) keeps
  terms, end time, revision and history byte-identical — it still requires a
  confirmed, editable booking and a valid provided body. Failed amendments
  change nothing. Cancel uses the accepted cutoff, increments revision once
  with one `cancelled` history entry (`changes: []`); repeating cancel returns
  the current state unchanged.
- `PATCH` accepts optional `expected_revision` (positive integer; anything
  else is 422). A mismatch is 409 `stale_revision` before cutoff or field
  validation; concurrent amendments on one revision admit at most one real
  change.
- `GET /reservations/{reference}/history` returns `seq`-ordered entries
  (`created` names table selection, time and party from null; `changed` names
  only real changes in selection/time/party order, `table_id` for
  single-to-single and `table_ids` otherwise; replays and no-ops record
  nothing). `GET /reservations/{reference}/decision` returns the current
  reference, revision and terms, even after cancellation. Both are owner-only:
  unknown, foreign, manager-non-owner and missing/invalid-token callers all
  get 404.
- Every successful new booking, real amendment, cancellation, policy
  publication, series adoption and real move batch
  increments the per-restaurant revision counter exactly once for the whole
  operation; no-ops, failures and replays never do.

## Recurring reservations

`POST /series` (idempotent) adopts an owned, confirmed, within-cutoff anchor
as occurrence 0: `{"anchor_reference": "...", "count": 2..12,
"interval_weeks": 1..4}`. Unknown/foreign anchor is 404, cancelled is 409,
already-adopted is 409. Occurrence *i* keeps the anchor's local calendar date
plus *i × weeks × 7* days at the same clock time, with the anchor's party
size and table set; each occurrence independently selects its own date's
policy (grid, duration, capacity, DST). Skipped wall times reject the whole
adoption (`invalid_local_time`); folds resolve first. The first failing index
determines the error; failure leaves no partial series, records, histories,
counters or receipt claim. Success is 201 with `series_id`, revision 1 and
all occurrences in index order, each with a distinct ordinary reference and
`exception: false`. `GET /series/{id}` is owner-only (404 otherwise) and
renders current reservation states. A real individual PATCH permanently flags
its occurrence `exception: true` and bumps the series revision once;
cancellation bumps the series revision without flagging; no-ops, failures
and repeats change neither. Cancelling the anchor leaves siblings alone.
Original series receipts replay byte-identical after later edits.

## Collective moves

`POST /reservation-moves` takes 1–8 `moves` with distinct references in one
restaurant, each accepting `table_id`/`table_ids`, `starts_at_local`,
`party_size` plus optional per-move `expected_revision`. Every real change
uses individual PATCH semantics (old accepted cutoff, then resulting-date
policy); failure leaves every booking unchanged. Each changed booking gains
one revision and one history entry; the restaurant counter and each affected
series revision increase exactly once for the whole batch, and every changed
series occurrence becomes a permanent diner exception. All-no-op batches
return 201 with zero metadata change; replays return the original bytes.

## Browser routes

The same service serves the UI: `/` (search, availability grid and an inline
SVG floor plan that mirrors it), `/signup`, `/login`, and `/lookup` (find a
reservation by reference, cancel from its detail view). The flow is: sign up
or log in, search by restaurant/date/party size, pick an available single or
pair cell, submit the booking form, keep the returned confirmation reference;
resubmitting the unchanged form replays the same reference, and lookup
retrieves or cancels the booking. State is shown first and animations never
delay it (reduced motion respected); a late search never overwrites a newer
one; a taken table keeps the form and refreshes availability; a lost response
retries the same key and body to recover the original reference. Explanations
and history need no new screens. Final browser and upgrade verification
belongs to later items and the reviewer; this guide makes no acceptance claim
for live browser flows.

## Idempotency

`POST /reservations`, `POST /reservation-moves`, `POST /series` and
`POST /restaurants/{id}/policies` require a 1–255 character
`Idempotency-Key`, scoped per user and per method+path. First use returns 201;
an identical replay returns 200 with the original JSON; a different body on a
used key is 409 `idempotency_key_reuse` even when otherwise invalid; keys from
failed (4xx) writes stay reusable; 50 concurrent identical writes commit once
(one 201, rest 200). Replays return the frozen original even after later
amendments, cancellations, republication or series changes. No batch, series
or policy UI is required.

## Probes

Source-controlled scripts under `probes/` exercise the packaged image (needs:
POSIX `sh`, `curl`, `python3`; `jq` only where a script's header says so).
They write no state outside their workdir; exports/tokens stay under
`evidence/`. From `stage-3`:

```sh
sh probes/stage3-api.sh http://localhost:9027 /tmp/probe-api
```

`stage3-api.sh <base-url> <private-work-dir>` is the current stage-3 API
probe: inherited stage-1/2 behavior (delivery, auth, public routes, errors,
idempotency, booking, pairs, PATCH, cancel, moves, DST, owner privacy) plus
dated policies and availability explanations, accepted terms, revisions,
no-ops, `expected_revision` races, history/decision, series adoption and
calendar/rollback/exception behavior, collective transactions and counters,
and a same-image export/import smoke. It does not claim browser behavior or
cross-version transfer.

`probes/stage3-donor.sh <stage1-source-url> <stage2-source-url>
<private-output-directory>` builds genuine two-source migration donors: it
drives an independently running accepted stage-1 image and an accepted
stage-2 image (past/current/cancelled/moved bookings, stage-2 pairs,
original create/move receipts, a reusable failed key, multiple sessions),
then writes `stage1/export.json` + `manifest.json` and
`stage2/export.json` + `manifest.json` under a `0700` directory (`0600`
files). Stdout carries names, codes, counts and provenance only.

The six-argument transfer script
`stage3-import.sh SRC1 SRC2 DST PEER DONORS WORK` (old-source imports,
modern collective/exception portability, receipt replay, atomicity guards)
is **pending integration** under a separate repair item and is not present in
this tree. Do not treat the same-image roundtrip inside `stage3-api.sh` as a
cross-version upgrade proof; only a two-process import run proves migration.
`stage1-api.sh`, `stage1-html.sh` and `stage1-export.sh` remain for inherited
single-table, page and export scope; they are not full stage-3 coverage.
