# Tablekeeper — stage 2 service

Single-container HTTP reservation service with combined tables and a browser
UI: search availability (singles and declared pairs), book/amend/cancel
tables, and change several bookings together atomically. The same Go service
serves the API and the static web build (`web/dist`, built with Node 26).

## Run with Docker (no manual setup)

```sh
cd stage-2
docker build -t tablekeeper:stage-2 .
docker run --rm -e PORT=8080 -p 8080:8080 tablekeeper:stage-2
```

The service listens on `0.0.0.0` at `$PORT` (default `8080`). A non-default port
works the same way:

```sh
docker run --rm -e PORT=9027 -p 9027:9027 tablekeeper:stage-2
curl localhost:9027/health
```

No volumes, no environment beyond `PORT` (and optional `WEB_DIR`, default
`web/dist`, `/app/web/dist` in the image), no outbound network at run time.
State is ephemeral: restarts and `POST /_test/reset` replace everything. The
image is built in stages — Node 26 (`npm ci` + `npm run build` with the locked
`package-lock.json`) for `web/dist`, Go 1.27 for the static service binary —
and the runtime carries only the binary, zoneinfo (IANA time rules),
`web/dist` (scripts, styles, fonts — nothing loads from the network) and a
tiny `/app/probe` HTTP client used for offline checks (`PORT=9027 docker exec
<container> /app/probe -method GET -url http://localhost:9027/health` answers
200 with `--network none`).

## Run from source

Requires Go 1.27+ and Node >=26 <27. Build the web bundle first so the served
UI works as well as the API, then start Go:

```sh
cd stage-2/web
npm ci && npm run check && npm test && npm run build
cd ..
go test ./...
go build -o tablekeeper ./cmd/tablekeeper
PORT=8080 ./tablekeeper
```

Backend: Go 1.27 with Keel v0.5.0 (`keel/id` for opaque ids and tokens; no
rate limiting). Frontend: Svelte with @nrynss/chaaya 0.3.0 — API calls go
through the Keel adapter, styling through the token system with light, dark
and system themes. An empty startup needs no fixture: the service starts with
empty state and `POST /_test/reset` loads restaurants, tables, combinable
pairs, users and seeded bookings (see `## Test control`). There are no
external runtime dependencies.

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
  `GET /restaurants/{id}`, `GET /availability`. Everything else needs
  `Authorization: Bearer <token>`.
- Auth: `POST /auth/signup` (409 `email_taken`, short passwords and bad emails
  are 422) and `POST /auth/login` (401 on wrong credentials). Tokens never
  expire; accounts hold many concurrent tokens. Passwords are stored hashed
  (SHA-256 prehash + bcrypt); plaintext is never persisted. Bearer tokens
  grant their owner's bookings only; other owners' references read as 404.
- Cutoff: cancel and PATCH are refused with 409 `cutoff_passed` once now
  reaches the booking's start minus its `cancellation_cutoff_minutes`.
- Availability: `GET /availability?restaurant_id=&date=YYYY-MM-DD&party_size=`
  lists every grid slot from opening whose absolute duration finishes by
  closing, with eligible `available_table_ids` (singles only, fixture order)
  plus `available_options`: every eligible singleton in fixture order, then
  every eligible declared pair in `combinable` order (`table_ids` in declared
  order, capacity summed). Closed days return `"slots": []`.
- Booking: `POST /reservations` (idempotent, see below) returns a `reference`
  of 6–12 `A-Z0-9` characters plus RFC 3339 `starts_at`/`ends_at` with explicit
  offsets. `GET /reservations`, `GET /reservations/{reference}`,
  `POST /reservations/{reference}/cancel` and `PATCH` amend time/tables/party.
  Every new reservation and current-state response carries `table_ids`, and
  carries `table_id` only for a singleton set; a replayed idempotency receipt
  keeps its stored original JSON verbatim, so an old stage-1 receipt may lack
  `table_ids` while new responses always have it.
- Table sets: write `table_id` for one table, or `table_ids` with one id for
  a singleton or two ids for a declared pair — never both fields at once
  (422). A pair must be exactly one declared unordered `[t_a, t_b]` entry of
  that restaurant; declaring `[t_1,t_2]` and `[t_2,t_3]` does not authorize
  `[t_1,t_3]`, and three-or-more tables are never combinable. Reversed input
  names the same set and reads back in declared order. Capacity is the
  sum of the members. `GET /reservations/{reference}` and cancel work the same
  for singles and pairs; cancelling frees every member at once.
- Local time: `starts_at_local` is bare `YYYY-MM-DDTHH:MM` in the restaurant's
  IANA zone. Skipped spring-forward times are 422 `invalid_local_time`;
  repeated fall-back times resolve to the first occurrence; durations are
  absolute elapsed time across DST.
- Test control: `POST /_test/reset` (204, replaces all state),
  `GET /_test/export` (200 with `track`/`format_version: 1`/`state`) and
  `POST /_test/import` (204 replacement, 422 without mutation). Exports are
  opaque and private: they contain password hashes and bearer tokens, so keep
  them under `evidence/`, never committed.
- Stage-1 compatibility: this service accepts unchanged exports from the
  accepted stage-1 service. Accounts, hashes, sessions, fixture configuration,
  reservation identities, references, statuses, timestamps and completed
  idempotent receipts (including original stage-1 responses without
  `table_ids`) are preserved; failed keys stay reusable. Import is
  replacement, not merge; reset clears imported state. No policies, series or
  replans exist in stage 2.

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
retries the same key and body to recover the original reference. Final browser
and upgrade verification belongs to later items and the reviewer; this guide
makes no acceptance claim for live browser flows.

## Idempotency

`POST /reservations` and `POST /reservation-moves` require a 1–255 character
`Idempotency-Key`, scoped per user and per method+path. First use returns 201;
an identical replay returns 200 with the original JSON; a different body on a
used key is 409 `idempotency_key_reuse` even when otherwise invalid; keys from
failed (4xx) writes stay reusable; 50 concurrent identical writes commit once
(one 201, rest 200). Replays return the frozen original even after later
amendments or cancellations.

## Atomic moves

`POST /reservation-moves` takes 1–8 `moves` with distinct references in one
restaurant, each accepting `table_id`/`table_ids`, `starts_at_local`,
`party_size`. Either every move commits or nothing does (occupancy, records,
receipts). Non-occupancy errors win in input order with cutoff before field
validation; resulting overlaps — against listed or unlisted bookings — are
409. No table may belong to overlapping resulting bookings.

## Probes

Source-controlled scripts under `probes/` exercise the packaged image (needs:
POSIX `sh`, `curl`, `jq`, `python3`). They write no state outside their
workdir; exports/tokens stay under `evidence/`. From `stage-2`:

```sh
sh probes/stage2-api.sh http://localhost:9027 /tmp/probe-api
```

`stage2-api.sh <base-url> <private-work-dir>` is the current stage-2 API
probe: inherited stage-1 behavior plus combined-table options, occupancy,
amendments, moves, idempotency, concurrency and same-image export/import
roundtrips. `probes/stage2-donor.sh <stage-1-base-url> <private-work-dir>`
builds an upgrade donor, but only against an accepted **stage-1** image — it
produces a stage-1 export plus manifest for migration work, not a stage-2
claim. Genuine stage-1→stage-2 transfer is proven by
`probes/stage2-import.sh <src-url> <dst-url> <donor-dir> <private-work-dir>`
run against two independently started processes. Make the donor first with
`stage2-donor.sh`, then run the import with `DONOR_ORIGIN=fresh` in its
environment (the flag configures the import command, not the donor command)
to additionally check live source/destination divergence; without it, a saved
canonical donor records one explicit `live-source-divergence` skip, since a
prior-process artifact has no live source:

```sh
sh probes/stage2-donor.sh http://127.0.0.1:9041 /tmp/priv-donor
DONOR_ORIGIN=fresh sh probes/stage2-import.sh http://127.0.0.1:9041 http://127.0.0.1:9042 /tmp/priv-donor /tmp/priv-import
```

It verifies original receipt JSON identical, credentials/sessions,
references/statuses, failed-key reuse, replacement/repeat/invalid atomicity,
reset clearing and pair behavior on the destination. `stage1-api.sh`,
`stage1-html.sh` and `stage1-export.sh` remain for inherited single-table and
page compatibility scope; they are not full stage-2 coverage. Do not treat
the same-image API roundtrip inside `stage2-api.sh` as a cross-version
upgrade proof; only a two-process `stage2-import.sh` run proves migration
(donor and destination must be distinct processes, and receipts must keep
their original stage-1 JSON without `table_ids`).
