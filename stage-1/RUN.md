# Tablekeeper — stage 1 service

Single-container HTTP reservation service: search availability, book/amend/cancel
tables, and change several bookings together atomically. The same Go service
serves the API and the static web build (`web/dist`, built with Node 26).

## Run with Docker (no manual setup)

```sh
cd stage-1
docker build -t tablekeeper:stage-1 .
docker run --rm -e PORT=8080 -p 8080:8080 tablekeeper:stage-1
```

The service listens on `0.0.0.0` at `$PORT` (default `8080`). A non-default port
works the same way:

```sh
docker run --rm -e PORT=9011 -p 9011:9011 tablekeeper:stage-1
curl localhost:9011/health
```

No volumes, no environment beyond `PORT` (and optional `WEB_DIR`, default
`web/dist`, `/app/web/dist` in the image), no outbound network at run time.
State is ephemeral: restarts and `POST /_test/reset` replace everything. The
image is built in stages — Node 26 (`npm ci` + `npm run build` with the locked
`package-lock.json`) for `web/dist`, Go 1.27 for the static service binary —
and the runtime carries only the binary, zoneinfo, `web/dist` and a tiny
`/app/probe` HTTP client used for offline checks.

## Run from source

Requires Go 1.27+ and Node >=26 <27. Build the web bundle first so the served
UI works as well as the API, then start Go:

```sh
cd stage-1/web
npm ci && npm run check && npm test && npm run build
cd ..
go test ./...
go build -o tablekeeper ./cmd/tablekeeper
PORT=8080 ./tablekeeper
```

An empty startup needs no fixture: the service starts with empty state and
`POST /_test/reset` loads restaurants, tables, users and seeded bookings (see
`## Test control`). There are no external runtime dependencies.

## API overview

The `application/json; charset=utf-8` convention applies to the API. Page
routes and bundled assets below are HTML/assets, not JSON.

- Errors share one envelope: `{"error": {"code": "...", "message": "..."}}`
  with the specified HTTP status and stable `code` (no string matching needed).
  Unknown body fields and query parameters are ignored, never errors.
- Public endpoints needing no bearer token: `GET /health`,
  `POST /_test/reset`, `GET /_test/export`, `POST /_test/import`,
  `POST /auth/signup`, `POST /auth/login`, `GET /restaurants`,
  `GET /restaurants/{id}`, `GET /availability`. The page routes `/`, `/signup`,
  `/login`, `/lookup` and local bundled assets are likewise public (HTML/JS/CSS).
  Everything else needs `Authorization: Bearer <token>`.
- Auth: `POST /auth/signup` (409 `email_taken`, short passwords and bad emails
  are 422) and `POST /auth/login` (401 on wrong credentials). Tokens never
  expire; accounts hold many concurrent tokens. Passwords are stored hashed
  (SHA-256 prehash + bcrypt); plaintext is never persisted.
- Availability: `GET /availability?restaurant_id=&date=YYYY-MM-DD&party_size=`
  lists every grid slot from opening whose absolute duration finishes by closing,
  with eligible `available_table_ids` in fixture order; closed days return
  `"slots": []`.
- Booking: `POST /reservations` (idempotent, see below) returns a `reference`
  of 6–12 `A-Z0-9` characters plus RFC 3339 `starts_at`/`ends_at` with explicit
  offsets. `GET /reservations`, `GET /reservations/{reference}`,
  `POST /reservations/{reference}/cancel` and `PATCH` amend time/table/party.
- Local time: `starts_at_local` is bare `YYYY-MM-DDTHH:MM` in the restaurant's
  IANA zone. Skipped spring-forward times are 422 `invalid_local_time`;
  repeated fall-back times resolve to the first occurrence; durations are
  absolute elapsed time across DST.
- Test control: `POST /_test/reset` (204, replaces all state),
  `GET /_test/export` (200 with `track`/`format_version: 1`/`state`) and
  `POST /_test/import` (204 replacement, 422 without mutation). Exports are
  opaque and private: they contain password hashes and bearer tokens, so keep
  them under `evidence/`, never committed.
- Stage 1 schema has no pairs/policies/series/replans: single `table_id`
  bookings only; combined tables, booking policies, recurring series and
  seating replan endpoints arrive in later stages.

## Browser routes

The same service serves the UI: `/` (search and availability grid), `/signup`,
`/login`, and `/lookup` (find a reservation by reference, cancel from its
detail view). The single-table contract is: sign up or log in, search by
restaurant/date/party size, pick an available grid cell, submit the booking
form, keep the returned confirmation reference; resubmitting the unchanged
form replays the same reference, and lookup retrieves or cancels the booking.
Final browser verification belongs to H1/H2 and the reviewer until those items
land; this guide makes no acceptance claim for live browser flows.

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
restaurant, each accepting PATCH fields. Either every move commits or nothing
does (occupancy, records, receipts). Non-occupancy errors win in input order
with cutoff before field validation; resulting overlaps are 409.

## Probes

Source-controlled scripts under `probes/` exercise the packaged image (not
implementation mirrors): `stage1-api.sh` (full API incl. 50-key concurrency,
DST, scopes, snapshots), `stage1-html.sh` (page routes, asset types,
off-origin scan) and `stage1-export.sh` (two-process replacement from a source
URL to a distinct destination URL, receipt and token preservation, atomicity).
They write no state outside their workdir; exports/tokens stay under
`evidence/`. Example:

```sh
docker build -t tablekeeper:s1-p .
docker run -d --name tk-src -e PORT=9011 -p 9011:9011 tablekeeper:s1-p
docker run -d --name tk-dst -e PORT=9012 -p 9012:9012 tablekeeper:s1-p
sh probes/stage1-api.sh http://localhost:9011 /tmp/probe-api
sh probes/stage1-html.sh http://localhost:9011 /tmp/probe-html
sh probes/stage1-export.sh http://localhost:9011 http://localhost:9012 /tmp/probe-exp
docker rm -f tk-src tk-dst
```
