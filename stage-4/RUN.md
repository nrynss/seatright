# Tablekeeper — stage 4 service

Single-container HTTP reservation service with combined tables, dated booking
policies, reservation history and recurring reservations, plus seating repairs
after table closures and recurring clock amendments, plus a browser UI: search
availability (singles and declared pairs), book/amend/cancel tables, and change
several bookings together atomically. Recurring series adoption and series clock
amendment are API capabilities (`POST /series`, `POST /series/{id}/amend`);
manager seating repair is an API capability
(`POST /restaurants/{id}/replans`, `POST /restaurants/{id}/replans/{plan_id}/apply`).
The browser exposes diner search, booking, confirmation, lookup and
cancellation only — there is no manager or series UI. The same
Go service serves the API and the static web build (`web/dist`, built with
Node 26).

## Run with Docker (no manual setup)

```sh
cd stage-4
docker build -t tablekeeper:stage-4 .
docker run --rm -e PORT=8080 -p 8080:8080 tablekeeper:stage-4
```

The service listens on `0.0.0.0` at `$PORT` (default `8080`; see
`cmd/tablekeeper/main.go`, which binds `0.0.0.0:$PORT`). A non-default port
works the same way, with the mapping matched to the value of `PORT`:

```sh
docker run --rm -e PORT=9027 -p 9027:9027 tablekeeper:stage-4
curl localhost:9027/health
```

No volumes, no environment beyond `PORT` (and optional `WEB_DIR`, default
`web/dist`, `/app/web/dist` in the image), no outbound network at run time.
State is ephemeral: restarts lose state, and `POST /_test/reset` or
`POST /_test/import` replace everything. The image is built in stages —
Node 26 (`npm ci`, then the `web/package.json` build script, with the locked
`package-lock.json`) for `web/dist`, Go 1.27 for the static service binary —
and the runtime carries only the binary, zoneinfo (IANA time rules),
`web/dist` (scripts, styles, five local `woff2` fonts under `web/public/fonts`
— nothing loads from the network) and a tiny `/app/probe` HTTP client used for
offline checks. The container's server port is configured with `-e PORT`;
probe it from inside its own namespace, which works with `--network none`:

```sh
docker run -d --name tk-off --network none -e PORT=9027 tablekeeper:stage-4
docker exec tk-off /app/probe -method GET -url http://localhost:9027/health
docker stop tk-off && docker rm tk-off
```
`/app/probe` is built from `probes/pclient/main.go` (flags `-method`, `-url`,
`-body` string, `-token`, `-key`, `-timeout` Go duration).

## Run from source

Requires Go 1.27+ and Node >=26 <27. Build the web bundle first so the served
UI works as well as the API, then start Go:

```sh
cd stage-4/web
npm ci && npm run check && npm test && npm run build
cd ..
go test ./...
go build -o tablekeeper ./cmd/tablekeeper
PORT=8080 ./tablekeeper
```

Without `WEB_DIR` the service serves `web/dist` relative to the working
directory, so start from `stage-4` after building the web bundle; page routes
fall back to a JSON 404 (`web build is not available`) when the build is
absent. Backend: Go 1.27 with Keel v0.5.0 (`keel/id` for opaque ids and
tokens; no rate limiting). Frontend: Svelte with @nrynss/chaaya 0.3.0 — API
calls go through the Keel adapter (`api`, `ApiError`, `keelErrorParser` from
`@nrynss/chaaya/keel`), styling through the token system with light, dark and
system themes. An empty startup needs no fixture: the service starts with
empty state and `POST /_test/reset` loads restaurants, tables, combinable
pairs, `manager_user_ids`, users and seeded bookings (see `## Test control`).
There are no external runtime dependencies.

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
  (SHA-256 prehash, hex-encoded, fed to bcrypt); plaintext is never persisted.
  Bearer tokens grant their owner's bookings only; other owners' references
  read as 404.
- Cutoff: diner cancel, PATCH and collective moves are refused with 409
  `cutoff_passed` once now reaches the booking's start minus its **accepted**
  `cancellation_cutoff_minutes` (the cutoff from the booking's own accepted
  terms, checked against its current start — not the newest published policy).
  Operator seating repair never checks cutoffs.
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
  The two rules are independent: `capacity` compares party size against the
  selected policy's capacity, `no_overlap` reports confirmed-booking and
  applied-closure conflicts alike. `explain` accepts only the literal value
  `true`; `false`, `1`, the empty string and anything else are 422. Without
  `explain` the response keeps the earlier shape with no explanation fields.
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
  `POST /_test/import` (204 replacement, 422 without mutation). Reset is
  destructive by design: it discards every booking, receipt, session and
  imported state. Exports are opaque and private: they contain password hashes
  and bearer tokens, so keep them under `evidence/`, never committed.

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

- A real diner amendment checks the old accepted cutoff first, then validates
  **all** merged fields against the resulting start date's policy, atomically
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
  publication, plan application, series adoption, real series amendment and
  real move batch increments the per-restaurant revision counter exactly once
  for the whole operation. A successful plan application always increments,
  including closures that move zero bookings — an unchanged-table application
  is a write, not a no-op. Diner no-op amendments, all-no-op move batches,
  all-no-op or empty series amendments, failed writes, previews and replays
  never increment.

## Seating changes after a table closure

A manager reviews a proposed seating arrangement before applying it. Diners
keep their booking times, party sizes and accepted terms; no booking disappears
or is cancelled, and no new screens are required. An applied closure takes
effect in availability, and the repaired record is visible in current lookup
and new booking responses; a retained original confirmation or receipt stays
byte-original and is never rewritten by a repair.

`POST /restaurants/{id}/replans` requires a manager and an
`Idempotency-Key`. Body:

```json
{"table_id": "t_2", "from": "2026-09-28T18:00:00+02:00",
 "to": "2026-09-28T23:00:00+02:00"}
```

`from` and `to` are instants with explicit offsets and `from < to`; anything
else is 422 `validation_failed`, and an unknown table is 404. The proposed
closure is the half-open interval `[from, to)`. Every confirmed booking at
this restaurant overlapping that interval is considered; all other bookings
keep their assignments and remain fixed occupancy. Each considered booking
keeps its reference, owner, party size, start, end and accepted terms, and is
assigned a singleton or a declared pair with enough capacity under **its own
accepted terms** — never the newest policy — avoiding fixed bookings, other
assignments, previously applied closures and the proposed closure. Planning
supports up to 6 tables, 4 declared pairs and 6 considered bookings; larger
inputs may return 422 `planning_limit`. Among feasible plans the service
minimizes, in order: the number of changed table sets, then total unused seats
(capacity minus party size), then the option-rank vector in ascending
reservation-reference order (singles in fixture order, then pairs in declared
order, starting at 0). Success is 201 with exactly the six keys `plan_id`,
`restaurant_revision` (captured counter), `closure`, `assignments`
(`reference`, `table_ids`, `changed` — every considered booking in reference
order, possibly empty), `moved_count` and `unused_seats`. Preview stores only
a plan plus its receipt: no closure, occupancy, revision or history changes.
No feasible plan is 409 `no_feasible_plan`, changing nothing.

`POST /restaurants/{id}/replans/{plan_id}/apply` with body `{}` requires a
manager and an `Idempotency-Key`. Success is 201 with `plan_id`, the new
`restaurant_revision` and every considered reservation in reference order.
Unknown plans and plans from another restaurant are 404. Any intervening
restaurant revision invalidates the plan: 409 `stale_plan`, changing nothing.
A plan already applied under a different key is 409 `plan_already_applied` —
checked before staleness — while a replay of the successful key returns the
original response with 200 even after later changes. Application is atomic:
the closure and all assignments land together. Each moved booking increments
its revision once and gains one `reassigned` history entry carrying the
complete `table_ids` change and the `plan_id`; clocks, dates and accepted
terms stay frozen. Unmoved bookings gain nothing. The restaurant counter
increments once for the whole plan, even when zero bookings move, and each
affected series revision increases once without changing exception flags.
Closures thereafter exclude containing singles and pairs from availability and
reject conflicting creates, amendments, collective moves and series adoptions
with 409 `table_unavailable`. Concurrent applications never leave partially
moved bookings, and a closure at another restaurant never invalidates a plan.

## Recurring reservations and clock amendments

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

`POST /series/{series_id}/amend` is an owner-only idempotent write:
`{"expected_revision": 3, "from_index": 2, "local_time": "20:00"}`.
`expected_revision` must be a positive integer (booleans invalid; a mismatch —
including a huge integral revision — is 409 `stale_revision` before any
occurrence cutoff or booking validation); `from_index` must be an integer in
`0..count-1`; `local_time` must be exactly `HH:MM` in `00:00..23:59`. Anything
else is 422, unknown series or a foreign owner's series is 404, and a missing
token is 401; unknown body fields are ignored. Only indices at or after
`from_index` are eligible, excluding cancelled occurrences and diner
exceptions; each keeps its original scheduled local date and only its clock
time changes, retaining reference, owner, party size and current table
selection. A change with identical resulting fields is a no-op: it keeps its
terms and skips cutoff checks and revalidation. Each real change checks its
old accepted cutoff, then adopts the resulting start date's policy like an
individual PATCH. Results must not conflict with unchanged occurrences, other
bookings or applied closures; on failure every history, receipt and revision
stays unchanged. Non-occupancy errors take precedence in occurrence-index
order, otherwise occupancy conflicts are `table_unavailable`. Success is 201
with the current series response: each changed occurrence gains one ordinary
`changed` history entry and one revision, while the series and restaurant
revisions each increase once for the whole operation — never per occurrence —
and no new exception flags are marked. All-no-op and empty eligible sets
succeed without revision changes. A replay returns the original response with
200 even after further edits or cancellations, and the key, body and path
scope it exactly like any idempotent write. Seating repairs may move series
members while preserving their exception flags, scheduled dates, identities
and accepted terms.

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

The same service serves the diner UI: `/` (search, availability grid and an
inline SVG floor plan that mirrors it), `/signup`, `/login`, and `/lookup`
(find a reservation by reference, cancel from its detail view). The UI covers
diner search, booking, confirmation, lookup and cancellation only. The flow
is: sign up or log in, search by restaurant/date/party size, pick an available
single or pair cell, submit the booking form, keep the returned confirmation
reference; resubmitting the unchanged form replays the same reference, and
lookup retrieves or cancels the booking. An original booking receipt can keep
a pair or a clock time that a later seating repair or series amendment has
since changed on the current record: the confirmation shows what was booked,
lookup shows what is current. State is shown first and animations never
delay it (reduced motion respected); a late search never overwrites a newer
one; a taken table keeps the form and refreshes availability; a lost response
shows uncertainty and retries the same key and body to recover the original
reference. Explanations, history, seating repair and series management need
no new screens. Final browser and upgrade verification belongs to later items
and the reviewer; this guide makes no acceptance claim for live browser
flows.

## Idempotency

`POST /reservations`, `POST /reservation-moves`, `POST /series`,
`POST /series/{series_id}/amend`, `POST /restaurants/{id}/policies`,
`POST /restaurants/{id}/replans` and
`POST /restaurants/{id}/replans/{plan_id}/apply` require a 1–255 character
`Idempotency-Key`, scoped per user and per method+path. First use returns 201;
an identical replay returns 200 with the original JSON; a different body on a
used key is 409 `idempotency_key_reuse` even when otherwise invalid; keys from
failed (4xx) writes stay reusable; 50 concurrent identical writes commit once
(one 201, rest 200). Replays return the frozen original even after later
amendments, cancellations, republication, repairs or series changes. No batch,
series, policy or manager UI is required.

## Probes

Source-controlled scripts under `probes/` exercise the packaged image (needs:
POSIX `sh`, `curl`, `python3`; `jq` only where a script's header says so).
They write no state outside their workdir; exports/tokens stay under
`evidence/`. From `stage-4`:

```sh
sh probes/stage4-api.sh http://localhost:9027 /tmp/probe-api
```

`stage4-api.sh <base-url> <private-work-dir>` is the current stage-4 API
probe: inherited foundations (delivery, auth, public routes, errors,
idempotency, booking, pairs, PATCH, cancel, moves, DST, owner privacy) plus
dated policies and availability explanations, accepted terms, revisions,
`expected_revision` races, history/decision, series adoption and
calendar/rollback/exception behavior, collective transactions and counters,
manager repair preview/apply races with exact deltas and receipts,
stale/already-applied replays, series propagation, closure availability and
enforcement, and recurring clock amendments with same-revision and
same-key races. It does not claim browser behavior, cross-version transfer,
packaging or harness acceptance.

```sh
sh probes/stage3-api.sh http://localhost:9027 /tmp/probe-inherited
```

`stage3-api.sh <base-url> <private-work-dir>` run against the stage-4 binary
is the inheritance check: it exercises earlier-stage behavior on the current
image, including a same-image export/import smoke only. It is not a genuine
old-source transfer proof.

```sh
sh probes/stage4-donor.sh <src1-url> <src2-url> <src3-url> <private-output-directory>
```

`stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR` builds genuine
three-source migration donors: it drives independently running accepted
stage-1, stage-2 and stage-3 images through fixture lifecycles (reset, logins,
bookings, batches, policies, series, replays, mutations — the sources are
disposable donor fixtures, not production data), then writes
`stage1/export.json` + `manifest.json`, `stage2/export.json` +
`manifest.json` and `stage3/export.json` + `manifest.json` under a `0700`
directory (`0600` files). Stdout carries names, codes, counts and provenance
only. Only `OUT_DIR` is written; reset on these disposable services is
destructive by design.

There is no `stage4-import.sh`: the transfer probe is not implemented or
integrated. Native stage-4 export/import portability is unavailable in this
delivery, including current-stage-4 roundtrips — a functional native importer
blocker, not just a missing script, so adding the script alone would not
resolve readiness. Do not treat the same-image roundtrip inside
`stage3-api.sh` as a cross-version upgrade proof; only a multi-process
transfer run proves migration.
`stage1-api.sh`, `stage1-html.sh` and `stage1-export.sh` remain for inherited
single-table, page and export scope; they are not full stage-4 coverage.
This guide states probe interfaces only and claims no results, hashes or
acceptance.
