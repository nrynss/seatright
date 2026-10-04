# Tablekeeper stage 1

Status: S1-A, S1-B, S1-C and S1-D1 verified and integrated; S1-D2 complete API binding/moves and S1-E import validation in progress; S1-G visual foundation in progress. Stages 2–4 pending; no later-stage service will be implemented before acceptance and copying.

## Requirements ledger

Each statement cites stage-1.md unless marked Task. S = supplied check (including partial coverage); U = no supplied check identified, independent probe required. Spec text is authoritative and coverage must include unsampled edge cases.

| ID | Source | Check | Testable requirement |
|---|---|---|---|
| R1 | §1 | S | Confirmed reservations never share a table during overlapping half-open occupancy intervals. |
| R2 | §1 | S | Adjacent intervals whose end equals the next start do not overlap. |
| R3 | §1 | S | Concurrent reservations serialize without duplicates or partial state. |
| R4 | §1 | U | Rejected writes leave every record and retry claim unchanged. |
| R5 | §1 | S | Restaurants independently determine table capacities and booking rules. |
| R6 | §2 | S | Each delivered stage builds from its Dockerfile into a standalone HTTP image. |
| R7 | §2 | U | RUN.md provides a build/start command requiring no manual setup. |
| R8 | §2 | S | The image operates with PORT and a port mapping without Compose. |
| R9 | §2 | U | All runtime dependencies and assets work without outbound networking. |
| R10 | §2 limits | U | Service operates with 2 vCPU and 2 GiB memory. |
| R11 | §2 limits | U | Service handles 50 requests in flight without 5xx. |
| R12 | §2 limits | U | Ordinary requests finish within 5 seconds. |
| R13 | §2 limits | U | Test control calls finish within 10 seconds. |
| R14 | §3.1 | S | Service binds 0.0.0.0 at PORT, defaulting to 8080. |
| R15 | §3.2 | S | Health returns 200 with exactly the status ok JSON once ready within 60 seconds. |
| R16 | §3.3 | S | Unauthenticated reset atomically replaces all state and returns empty 204. |
| R17 | §3.3 | S | Repeated resets leave only the last fixture. |
| R18 | §3.4 | U | API requests and responses use application/json; charset=utf-8. |
| R19 | §3.4 | S | Response timestamps are RFC3339 with explicit offsets. |
| R20 | §3.4 | S | Unknown request fields and query parameters are ignored. |
| R21 | §3.4 | S | Generated and fixture IDs are opaque strings of at most 64 characters. |
| R22 | §4 | S | Reset supplies users, restaurants, tables and reservations in the documented shape. |
| R23 | §4 | S | Weekday opening hours, local timezone, grid, duration, cutoff and capacity follow the fixture. |
| R24 | §4 | S | Seeded users immediately log in with their fixture passwords. |
| R25 | §4 | S | Seeded confirmed bookings retain their id, reference and owner and occupy their tables. |
| R26 | §4 | S | Past start dates alone never cause booking rejection. |
| R27 | §5 | U | Every 4xx/5xx uses the documented error envelope and specified status/code. |
| R28 | §5 | S | Unparseable JSON or ordinary wrong field types produce 400 malformed_request. |
| R29 | §5 | S | Missing required fields or invalid formats/ranges produce 422 validation_failed unless a specific code applies. |
| R30 | §5 | S | All invalid party_size values including strings and booleans produce 422 validation_failed. |
| R31 | §5 | S | starts_at_local strings must be bare YYYY-MM-DDTHH:MM or produce 422 validation_failed. |
| R32 | §5 | S | Integer query parameters contain only plain decimal digits. |
| R33 | §5 | U | Idempotency-Key length is 1–255 characters on every required path. |
| R34 | §5 | U | Requests including malformed and concurrent requests never produce 5xx. |
| R35 | §6 | S | Signup returns 201 with opaque user_id, display_name and valid token. |
| R36 | §6 | S | Duplicate signup email returns 409 email_taken. |
| R37 | §6 | S | Signup passwords shorter than 8 characters produce 422 validation_failed. |
| R38 | §6 | S | Signup email must have local@domain form. |
| R39 | §6 | S | Login returns 200 with user_id, display_name and token for valid credentials. |
| R40 | §6 | S | Unknown email or incorrect login password returns 401 unauthenticated. |
| R41 | §6 | S | Protected paths reject missing, malformed and unknown bearer tokens with 401 unauthenticated. |
| R42 | §6 | S | Health, reset, auth and the three restaurant/availability GET paths remain public. |
| R43 | §6 | U | Account tokens never expire and multiple concurrent tokens remain valid. |
| R44 | §6 | U | Passwords are stored using a password hash and never plaintext. |
| R45 | §7 | S | Reservation creation and move batches require a nonempty Idempotency-Key or return 400 missing_idempotency_key. |
| R46 | §7 | S | Idempotency is scoped by user and method/path so another user/path can use the same key. |
| R47 | §7 | U | After JSON-object parsing and authentication, receipt resolution precedes endpoint validation and current-resource checks. |
| R48 | §7 | S | The first successful key use returns 201 and a replay returns 200 with the original JSON value. |
| R49 | §7 | S | A different parsed JSON body on the same user/method/path/key returns 409 idempotency_key_reuse. |
| R50 | §7 | U | JSON key order and whitespace do not affect replay identity. |
| R51 | §7 | U | A key from a failed 4xx remains reusable. |
| R52 | §7 | U | Concurrent identical unused-key requests create exactly once with one 201 and all other responses 200. |
| R53 | §7 | S | Successful replays return the original response after cancellation or amendment without mutations. |
| R54 | §8 restaurants | S | Restaurant list returns fixture ids, names and timezones in an array. |
| R55 | §8 detail | S | Detail returns fixture configuration and tables; unknown restaurant is 404 not_found. |
| R56 | §8 availability | S | restaurant_id, date and party_size are required and local calendar date is validated. |
| R57 | §8 availability | S | Availability includes every opening-grid slot whose absolute duration finishes by closing. |
| R58 | §8 availability | S | Each slot has full starts_at_local, resolved starts_at and eligible table IDs in fixture order. |
| R59 | §8 availability | S | Tables are eligible exactly when capacity suffices and no confirmed booking overlaps. |
| R60 | §8 availability | S | Slots with no free tables remain present and closed days return an empty slots array. |
| R61 | §8 create | S | A valid creation returns the documented confirmed reservation and timestamps. |
| R62 | §8 create | S | References are unique 6–12 character uppercase alphanumeric strings that never change. |
| R63 | §8 create | S | Overlap returns 409 table_unavailable. |
| R64 | §8 create | S | A valid local start off the opening-grid returns 422 not_on_slot_grid. |
| R65 | §8 create | S | Outside hours or end after closing returns 422 outside_opening_hours. |
| R66 | §8 create | S | Party above capacity returns 422 party_exceeds_capacity. |
| R67 | §8 create | S | Party below 1 or noninteger returns 422 validation_failed. |
| R68 | §8 create | S | Unknown restaurant/table or foreign-restaurant table returns 404 not_found. |
| R69 | §8 list | S | Reservation list contains only caller-owned confirmed and cancelled bookings, descending by starts_at. |
| R70 | §8 lookup | S | Reference lookup returns the caller's booking and hides other owners/unknown references with 404. |
| R71 | §8 cancel | S | Cancellation returns current cancelled reservation and immediately releases occupancy. |
| R72 | §8 cancel | S | Repeated cancellation succeeds with current state without further changes. |
| R73 | §8 cancel | S | At or after start minus cancellation cutoff, cancellation is refused with 409 cutoff_passed. |
| R74 | §8 cancel | S | Cancellation of another owner's reference returns 404 not_found. |
| R75 | §8 PATCH | S | Any subset of table_id, starts_at_local and party_size can be amended without an idempotency key. |
| R76 | §8 PATCH | S | Amendments use create validation and check cutoff against the existing start. |
| R77 | §8 PATCH | S | Amending a cancelled booking returns 409 reservation_cancelled. |
| R78 | §8 PATCH | S | Successful amendment atomically releases old occupancy and claims new occupancy while keeping id and reference. |
| R79 | §8 PATCH | S | Failed amendments retain all original record values and occupancy. |
| R80 | §9 | S | Europe/Berlin and America/New_York offsets follow IANA transitions for arbitrary dates. |
| R81 | §9 | S | Skipped local times never appear in availability and booking them returns 422 invalid_local_time. |
| R82 | §9 | S | Repeated local times resolve to the first occurrence, appear once and never book the second. |
| R83 | §9 | S | Reservation duration is absolute elapsed time even across DST. |
| R84 | §10 | S | Public export returns track tablekeeper, format_version 1 and an opaque state object with 200. |
| R85 | §10 | U | Export is an atomic read-only snapshot independent of subsequent source writes. |
| R86 | §10 | S | Import accepts unchanged service exports and atomically replaces state with empty 204. |
| R87 | §10 | U | Import works across processes with no source file, volume, port or network dependency. |
| R88 | §10 | U | Repeated import restores state without duplicating resources. |
| R89 | §10 | U | Invalid JSON returns 400 and missing fields/wrong track/version/invalid state return 422 without mutations. |
| R90 | §10 | U | Import preserves accounts and password hashes and all existing session tokens. |
| R91 | §10 | U | Import preserves fixture configuration, reservation identities, references, statuses and timestamps. |
| R92 | §10 | U | Import preserves successful receipt bodies and responses while failed keys stay reusable. |
| R93 | §10 | U | Import removes previous destination credentials/data and reset clears imported state. |
| R94 | §11 | S | Atomic moves require authentication and idempotency and return 201 reservations in input order. |
| R95 | §11 | U | moves is 1–8 objects with distinct string references or returns 422 validation_failed. |
| R96 | §11 | U | Unknown/other-owner move references return 404 and mixed restaurants return 422. |
| R97 | §11 | U | Each move accepts PATCH fields retaining omitted values and ignoring unknown fields. |
| R98 | §11 | U | Moves preserve identity, ownership and creation time. |
| R99 | §11 | U | Cancelled and cutoff failures use ordinary amendment codes. |
| R100 | §11 | U | Nonoccupancy errors take precedence in input order with cutoff before changed-field validation. |
| R101 | §11 | U | Resulting overlap against listed or unlisted bookings returns 409 table_unavailable. |
| R102 | §11 | U | Listed unchanged bookings retain occupancy and values. |
| R103 | §11 | U | Move batches commit all records/occupancy/receipts together or change nothing. |
| R104 | §11 | U | Batch replay returns original success after subsequent changes without mutations. |
| R105 | §11 | U | Export/import preserves successful batch receipts and resulting bookings. |
| R106 | Task stack | U | Backend uses Go 1.27 and Keel v0.5.0, with keel/id for IDs and tokens and no rate limiting. |
| R107 | Task stack | U | UI uses Svelte and Chaaya 0.3.0 on Node >=26 <27 served by the same Go image. |
| R108 | Task stack | U | All UI API calls use Chaaya's Keel api, ApiError and keelErrorParser adapter. |
| R109 | Task stack | U | UI uses Chaaya tokens/reference.css and theme modes light, dark and system. |
| R110 | Task stack | U | UI tests run Chaaya testing contrast/accessibility gates. |
| R111 | Task design | U | Inline SVG floor plan draws seats and table shapes scaled by capacity and mirrors the grid. |
| R112 | Task design | U | Available and selected floor-plan states animate using token styling. |
| R113 | Task design | U | Svelte transition, animate and motion provide state transitions, entrances, spring selection and confirmation. |
| R114 | Task design | U | Reduced motion is respected and state truth never waits for an animation. |
| R115 | Task design | U | Loading, empty, error, uncertain and confirmed states each have clear designed presentations. |
| R116 | Task design | U | Dates and times are human readable with restaurant/table labels. |
| R117 | Task design | U | UI is usable at 375 and 1280 pixels without horizontal page scrolling. |
| R118 | Task design | U | Contrast, focus and labels are accessible in both light and dark modes. |
| R119 | Task delivery | U | Accepted stage folders remain unchanged and each next stage starts from their committed copy without nested .git. |
| R120 | Task checks | S | Each stage passes isolated suites 1..N, prints claimed stage N and leaves the next stage unimplemented. |
| R121 | §1 provenance | U | Domain source/API documentation/schemas from existing reservation products are not used. |

## Work items and queue

All paths below are relative to the owner's fixed worktree. Owned stage-1/PLAN.md is coordinator-only. All other stage folders, mandates and repository records are forbidden to implementers.

- S1-A — OpenCode — completed and integrated: stage-1/go.mod, go.sum, cmd/tablekeeper/**, internal/service/{model,state,auth,control,http}*.go, Dockerfile, RUN.md, .dockerignore, .gitignore. Covers R6–R53,R54–R55,R84–R93,R106,R121. Build the small foundation: state/auth/control/public restaurant routes and shared data contract. Seed reservation normalization may use a local validation helper until the clock package is integrated; no reservation write routes yet. Exact state contract is below. Acceptance: `cd stage-1 && go test -race ./...`; `go build ./cmd/tablekeeper`; `docker build -t tablekeeper:s1-a .`; run mapped PORT smoke tests for health/reset/auth/detail/export/import. Report container and source-only evidence separately.
- S1-B — OMP — completed and integrated: stage-1/internal/clock/** only. Covers R2,R19,R23,R26,R29–R32,R56–R58,R64–R65,R67,R73,R80–R83. Pure stdlib independent time/grid/overlap helpers with table-driven tests; no module or state dependency. Acceptance before foundation: `cd stage-1/internal/clock && GO111MODULE=off go test -race -v`; after integration: `cd stage-1 && go test -race ./internal/clock`.
- S1-G — Grok — completed and integrated: stage-1/web/** only. Covers R9,R107–R118. Build the independent visual system, floor plan, routed screen shells and state components with an explicit local preview fixture. It has no service dependency and must not implement combinations/policies/series/replans yet. Acceptance: `cd stage-1/web && npm ci && npm run check && npm test && npm run build`; screenshot every implemented state and theme at both widths and run Chaaya gates. Browser flows are queued for S1-H after the integrated API is available. Preview fixture: restaurant r_anker, Zum Anker, Europe/Berlin, tables t_1 label 1 capacity 2 and t_2 label 2 capacity 4; date 2026-09-24, 18:00 and 18:30 slots available for both, 19:00 unavailable for t_2, party 2. This is visual preview data, never a fallback for live API success.
- S1-C — OpenCode — completed and integrated: stage-1/internal/service/{reservations,availability}*.go, http.go, http_test.go, control_seed.go, control_reset.go and control_test.go. No model/state/auth edits. Covers R1–R5,R25–R26,R54–R83. Implements CRUD/public availability under common lock. Create core is a locked callback; creation HTTP binding lands with S1-D2 after the wrapper. Frozen additional signatures follow below.
- S1-D1 — OMP — completed and integrated: stage-1/internal/service/idempotency*.go only. Covers R33,R45–R53,R92. Generic atomic callback wrapper and independent receipt tests against the integrated State/Receipt; no reservation-core dependency or routing changes. Acceptance: `cd stage-1 && go test -race -v ./...`; `go build ./cmd/tablekeeper`; `docker build -t tablekeeper:s1-d1 .`; documented health/auth/export/import smoke.
- S1-E — OMP — completed and integrated: stage-1/internal/service/control.go, control_validation_test.go, idempotency_test.go, auth.go and auth_test.go. Covers R21,R34,R37,R43–R44,R85–R93. Validate opaque imported-state invariants and portable original snapshots, including malformed receipts, identities/configuration/timestamps/occupancy; preserve original valid state without regeneration. Fix password minimum to count Unicode characters. Update the receipt export test's deliberately incomplete synthetic reservation to a complete valid stage-1 record without changing its assertions. No control_reset/control_seed/http/reservation/availability/model/state changes, so no overlap with C. Acceptance: full race/build/image/control+auth smoke with command evidence; D2 follows actual C integration.
- S1-D2 — OpenCode — completed and integrated: stage-1/internal/service/moves*.go, writes*.go, http.go and http_test.go; no reservations/availability/auth/model edits. Covers R45–R53,R94–R105. Wire POST creation to generic receipt callback and implement atomic move batch using integrated preparation/conflict helpers. Exact API endpoints and full stage-1 harness API probes; complete single-table API before UI wiring.

S1-D2 was reassigned to OpenCode after C integration so API completion can run in parallel with OMP's independent S1-E validation. The existing TestAuthHeaderForms valid-token assertion was updated from404 to200 in C; OMP owns only its new Unicode-password tests in S1-E and must preserve that updated expectation when integrating.
- S1-H1 — Grok — in progress from integrated API/visual foundation: stage-1/web/** only; live auth/session/navigation, restaurant discovery/search/latest-response protection and selected form data, preserving SVG/grid/theme/accessibility. Covers R35–R43,R54–R60,R107–R118 and the stage-2 single-table auth/search hooks adopted early. Bundle primary serif and numeric fonts with licenses as local build assets. Acceptance: npm ci/check/test/build and browser auth/search/out-of-order probes against real Go service, screenshots at375/1280 in light/dark, Chaaya gates. No booking submission/lookup behavior until H2; preview data is never live fallback.
- S1-H2 — Grok — pending H1 integration: stage-1/web/** only; real booking/confirmation, unchanged-body retry identity, lost-response uncertainty,409 refresh preserving form, private lookup/cancel and upgrade-between-requests recovery. Full browser source/network failure probes and screenshots/recordings. No combinations until stage2.
- S1-P — OMP — completed and integrated: stage-1/Dockerfile, RUN.md, .dockerignore and probes/** only. One Node26+Go1.27 multistage image, local assets/zoneinfo, no runtime network, public HTML routes, final API/control probes. UI source can evolve through H1/H2 without changing packaging contract; full final reviewer build follows their integration. Acceptance: full Go race, npm gates on unchanged foundation, docker build/run nondefaultPORT with offline networking, isolated stage1 supplied checks and documented packaging/API probe scripts.
- S1-P2 — OMP — completed and integrated: stage-1/RUN.md only; finalize source/UI startup instructions and correct public API/test-control exceptions. Document single-table browser route contract without claiming reviewer acceptance. No product/test changes; final image rebuild is the reviewer gate after H2.
- S1-R — ZCode — pending candidate: read-only full stage-1 exact-SHA isolated harness and independent probes, plus design review. At most three review rounds. Evidence is required for every claim.

## Frozen foundation contract

Module path `tablekeeper`; Go files under internal/service use package service. `New() *Service` and `(*Service).Handler() http.Handler`; cmd calls these and listens on the contract address. Service has `mu sync.Mutex` and `state State`; every operation including reads holds mu for a consistent transaction, expensive password work may occur before locking only with final validation under the lock. State uses exported serializable fields: `Users map[string]User`, `Tokens map[string]string`, `Restaurants []Restaurant`, `Reservations map[string]Reservation` keyed by reference, `Receipts map[string]Receipt`. No plaintext passwords in User: ID, Email, DisplayName, PasswordHash. Restaurant retains fixture field names/order including tables; stage-1 stores only stage-1 configuration. Reservation stores ReservationID, Reference, UserID (excluded from ordinary JSON), RestaurantID, TableID, PartySize, Status, StartsAtLocal, StartsAt, EndsAt, CreatedAt. Receipt stores caller/method/path/key, canonical parsed JSON body and immutable original response JSON. Fields may use explicit JSON DTOs, but future import must preserve this state layout. Reset builds replacement off to the side then publishes under mu; import validates before replacement.

Clock package API: `type Hours struct { Weekday string; Opens string; Closes string }`; `type Rules struct { Timezone string; SlotMinutes int; DurationMinutes int; OpeningHours []Hours }`; `type Slot struct { Local string; Start time.Time; End time.Time }`; `type Error struct { Code string }` implementing error; `ParseDate(string) (time.Time,error)` strict valid YYYY-MM-DD; `ResolveLocal(local,timezone string) (time.Time,error)` strict bare local and earliest occurrence; `ValidateSlot(local string, rules Rules) (Slot,error)` checks local existence, hours/grid/end; `Slots(date string,rules Rules) ([]Slot,error)` produces wall-grid once, skips nonexistent slots, absolute end; `Overlap(aStart,aEnd,bStart,bEnd time.Time) bool`; `CutoffPassed(now,start time.Time,cutoffMinutes int) bool`. Codes validation_failed, invalid_local_time, not_on_slot_grid, outside_opening_hours. Opening and closing instants use the same first-occurrence resolver. Do not impose stage-3 limits on stage-1 fixture cutoffs.

Static serving contract: web build emits `web/dist`; Docker Node 26 build copies it into Go image at `/app/web/dist`. Go uses `WEB_DIR` default `web/dist`, returns index.html on `/`, `/signup`, `/login`, `/lookup`, and serves local bundled assets without treating API misses as HTML. Initial foundation must remain buildable before web exists; final packaging builds both.

## Review command

## Reservation/receipt seams after foundation integration

S1-C exports within package service: `func (s *Service) createReservationLocked(st *State, userID string, obj map[string]any) Result`; `func prepareAmendment(st *State, current Reservation, changes map[string]any) (Reservation, *codedError)` performs confirmed/cutoff/field/time/capacity validation only, no occupancy and no mutation; `func conflictingReservation(st *State, candidate Reservation, exclude map[string]bool) bool` checks confirmed overlap on the same restaurant/table, ignoring listed references. D2 will call these only after C is integrated. Create's callback performs its own full occupancy check before mutation.

S1-C public methods are `Availability(q url.Values) Result`, `ListReservations(token string) Result`, `GetReservation(token,reference string) Result`, `CancelReservation(token,reference string) Result`, `PatchReservation(token,reference string,raw []byte) Result`. Each protected operation authenticates and mutates/reads under the same state lock. C updates routes for these; POST creation and moves remain marked stubs until D2. C may consolidate fixture time normalization into clock and fix seeded party-size code/explicit zero offsets under its owned control files.

S1-D1 exports `func (s *Service) Idempotent(token,method,path,key string,raw []byte,fn func(*State,string,map[string]any) Result) Result`. Parse object; authenticate under mu; validate header; resolve completed receipt before endpoint fields/resource checks. Execute fn on cloned working state under mu; commit only successful 201 state and immutable original receipt together. A failure discards tentative state and claim. On replay decode the original response as a JSON value and return 200 with no callback. Callback MUST NOT lock the Service. The wrapper operates independently of C and has no HTTP routing changes. D2 later wires the creation callback and batch callback from the integrated revision; no stand-in is needed.

Password contract: accepted stage-1 hashes use bcrypt on the lowercase hex SHA-256 digest of the complete password; signup, fixture reset and login all use the same helper. Export/import keeps the hash untouched; empty display names are valid and portable. Foundation host test gate: 31/31 top-level tests. Clock gate: 17 top-level + 8 subtests. Foundation corrected raw smoke log contains 27 PASS lines (reported 28 is not corroborated); the two distinct source/destination URLs are localhost:9001 and localhost:9002. Named-container attribution is not in that raw log and is not claimed as verified.


`cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 1 --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S1-R1/checks`

## Progress fallback

Runtime private TaskCreate returned `MCP tool call requires approval, but approval policy is never`. This file and RUNLOG.md are the lossy progress fallback; private task IDs, dependencies, metadata/history and reliable deletions are unavailable. Shared cards represent delegated work only.
