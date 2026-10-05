# Tablekeeper stage 2

Status: stage1 accepted delivery38d5fd6deb3a4f4e4474a02ebb153edb24a6d69c, reviewedb298700f790c166cf7ce8d98d731c80093ecb9af, immutable tree8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c. Exact copy7593c881c798ef9d24765dcb3cfa7066cd739f96 committed before extensions. S2-M completed0cc9ea85bb4089617e06333ba9a408a013ddd34f mergedfad65755b6a8342353ac4935a6c9d3a66e88e7de; S2-I in progress; S2-H completed1e99a2338cde22f2ce958006ad0c4a5d2cbc2a0c mergedbb584a0b44f5a9405385ca4c03478b389b590280; S2-P2 completede524b838dd425bdbad6bde9828cfd4dafe8787bc merged356c5564b84fbb8ad648239c04daef66f29e4693; S2-P1 completed872c899204f2d5c04f961410852807406cc974cc merged775d4a9755aa2775798ef7a5c01954a600cac29a; S2-G5 completeda38403ad05881da10388d6f61b1fa85b5b0fdc81 merged95a0b1ffdb988d04ffcc8f6fa0d9a473821a5dd1; S2-B completed63abeeab69bd66e505454e9eb6b8f9d07e969101 mergedb874cea90fb02d13baceaa62b3a60575e61c3c56; S2-G4 completed8a04e7619b18aa224b0e8f087e8866477bc51a1c merged14fefa382e4c917ea99f60a925ad2558952d1b4e; S2-G completed d0f11fe0572ae01a33b06c26832dcf7036f20f42 merged42c80bcb0e5356133ffd8a52f28fb6bc9cc120d7; S2-G2 completed d2c2f013789a635756e9bffe702dc69fd1824f37 merged0fe3bd125ec4e0441868be37042aa3c6d3894a9c; S2-G3 completed de8c265678a61cbd5c38ce5f2c474091e4c8235e merged72eebf43aaf557f5a5050fedb7e38b0437ecc4b0; S2-D completed at92d0f53de6221ff8dc15e44ab2dc1b1a0e55e08c merged13763212db959b296047686d4718921a8a91644c. Stage3/4 pending acceptance/copy.

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


## Stage-2 added ledger

References below name stage-2.md sections. Inherited R1–R121 retain their stage1/task references. S = supplied sample exercises at least part, never exhaustive; U = independent probes required. No polling/cross-tab/reload recovery/in-flight migration or batch UI required.

| ID | Section | Check | Single testable statement |
|---|---|---|---|
| R122 | Scope | U | Stage-2 preserves every inherited requirement except the explicitly extended response table-set schema. |
| R123 | Routes | S | /, /signup, /login and /lookup return directly reachable HTML screens. |
| R124 | Routes | U | Other required screens remain reachable through consistent UI navigation. |
| R125 | Competing clients | U | Late search A never replaces search B grid, labels or booking form. |
| R126 | Competing clients | S | 409 table_unavailable displays booking-error. |
| R127 | Competing clients | U | A booking conflict refreshes authoritative availability. |
| R128 | Competing clients | U | Conflict refresh preserves selected form and edited inputs. |
| R129 | Competing clients | S | A refused booking attempt never displays its confirmation. |
| R130 | Uncertainty | U | Lost booking responses including after commit display nonempty booking-uncertain. |
| R131 | Uncertainty | U | An uncertain attempt displays neither booking-error nor a new confirmation. |
| R132 | Uncertainty | U | Unchanged uncertain forms retry the same key and exact body. |
| R133 | Uncertainty | U | Successful retry clears uncertainty/error and shows the original server reference. |
| R134 | Uncertainty | S | Confirmed booking rejection uses booking-error. |
| R135 | Uncertainty | U | Ordering, conflict and uncertain-outcome rules apply to combinations too. |
| R136 | Uncertainty | U | The browser never manufactures successful bookings from cached data. |
| R137 | Auth UI | S | Every named signup/login input and submit hook is exact. |
| R138 | Auth UI | S | auth-error exists only when an authentication error exists. |
| R139 | Auth UI | S | Every signed-in screen shows current-user containing display name. |
| R140 | Auth UI | S | logout-button signs out and clears signed-in presentation. |
| R141 | Search UI | S | restaurant-select option values are restaurant ids. |
| R142 | Search UI | S | date-input is the local YYYY-MM-DD date. |
| R143 | Search UI | S | party-size-input is numeric and search-button submits that search. |
| R144 | Search UI | S | availability-grid presents slots and no-slots replaces it on closed/no-slot days. |
| R145 | Search UI | S | There is one exact slot-{table_id}-{HH:MM} cell per table per slot. |
| R146 | Search UI | S | Single-cell data-available exactly matches searched-party available_table_ids membership. |
| R147 | Search UI | S | Available-cell clicks open the selected table/time booking form. |
| R148 | Search UI | S | Unavailable-cell clicks change nothing. |
| R149 | Search UI | S | Signed-out available-cell clicks show auth-error or navigate to login. |
| R150 | Booking UI | S | booking-form, booking-summary, booking-party-size, booking-submit and booking-error hooks are exact. |
| R151 | Booking UI | S | booking-party-size is prefilled from the searched party. |
| R152 | Booking UI | S | The booking form remains after success. |
| R153 | Booking UI | S | Unchanged resubmission returns the same reference without error or another booking. |
| R154 | Booking UI | S | A changed form field makes the next submission a new request identity. |
| R155 | Confirmation UI | S | Only success displays confirmation with confirmation-reference exactly the reference. |
| R156 | Confirmation UI | S | confirmation-details contains restaurant name, table label and local start. |
| R157 | Lookup UI | S | Lookup exposes lookup-reference-input and lookup-submit. |
| R158 | Lookup UI | S | Found reservations show reservation-detail and literal confirmed/cancelled status. |
| R159 | Lookup UI | S | reservation-cancel-button is absent once cancelled. |
| R160 | Lookup UI | S | reservation-error displays not-found or refused-cancel failures. |
| R161 | Upgrade | S | Stage-2 accepts unchanged exports from this team's accepted stage-1 service. |
| R162 | Upgrade | S | A pre-export bearer browser session remains valid after stage1-to2 import. |
| R163 | Upgrade | U | A retained stage-1 reference works in stage-2 lookup. |
| R164 | Upgrade | U | A lost stage-1 response remains retryable with original body/key and complete original receipt. |
| R165 | Upgrade | U | Between-request upgrade needs no reload/new screen and preserves form/pending identity. |
| R166 | Combined tables | S | Declared pairs occupy both members for the full reservation duration. |
| R167 | Combined tables | S | Legacy single-table request formats remain accepted. |
| R168 | Model | U | combinable consists of unordered pairs of ids in that restaurant. |
| R169 | Model | U | Three or more tables are never combinable. |
| R170 | Model | U | Undeclared pairs are forbidden and combinations are not transitive. |
| R171 | Model | U | Pair capacity is the sum of the two table capacities. |
| R172 | Model | U | Seeds default to confirmed and honor explicit cancelled status. |
| R173 | Model | U | Seeded bookings accept either table_id or table_ids. |
| R174 | Availability API | U | Every slot gains available_options alongside existing stage-1 fields. |
| R175 | Availability API | S | available_table_ids remains singles-only with original meaning. |
| R176 | Availability API | U | Options include exactly eligible singles/pairs with sufficient capacity and no member overlap. |
| R177 | Availability API | U | Options order is fixture singles followed by declared pairs. |
| R178 | Availability API | U | Pair table_ids use declared combinable order. |
| R179 | Create API | S | POST accepts table_ids, legacy table_id, and rejects both fields with422 validation_failed. |
| R180 | Response schema | S | New responses always have table_ids and have table_id only iff singleton. |
| R181 | Create API | U | Duplicate table ids yield422 validation_failed. |
| R182 | Create API | U | Undeclared pairs or more than two tables yield422 combination_not_allowed. |
| R183 | Occupancy API | S | Any occupied member yields409 table_unavailable. |
| R184 | Capacity API | U | Party above summed selected capacity yields422 party_exceeds_capacity. |
| R185 | PATCH API | U | PATCH accepts table_ids under the same selection/validation rules. |
| R186 | Cancellation API | U | Cancelling releases every table in the set immediately. |
| R187 | Combination grid | U | Pair cells use slot-{t_a}+{t_b}-{HH:MM} in declared order and authoritative data-available. |
| R188 | Combination booking UI | U | booking-summary names every selected table label and local start. |
| R189 | Combination confirmation UI | U | confirmation-tables contains every reserved table label. |
| R190 | Combination lookup UI | U | reservation-tables contains every reserved table label. |
| R191 | Compatibility UI | S | Single-table cell testids, confirmation and lookup retain preceding behavior. |
| R192 | Atomic moves | U | Each move accepts table_ids under ordinary stage-2 amendment rules. |
| R193 | Atomic moves | U | No table belongs to overlapping resulting bookings and every failed batch rolls back. |
| R194 | Receipts/recovery | U | Combination original receipts, browser recovery and export/import remain immutable/portable. |
| R195 | Concurrency | U | Bookings, amendments and every read are equivalent to some serial execution. |
| R196 | Product direction | U | Combined options read as intentional seating with table labels rather than joined technical ids. |
| R197 | Product direction | U | Warm hierarchy, consistent visual system, obvious primary actions and distinct states form one coherent product. |
| R198 | Product direction | U | 375px and desktop avoid horizontal page scroll with labels, focus and sufficient contrast. |
| R199 | Product direction | U | Empty/loading/error states and navigation are considered and consistent. |

## Work items and queue

Only stage-2 edits; stage-1/root records/mandates/later folders and PLAN.md are forbidden to implementers. Coordinator owns Git and all resets. Before each new item fixed worktree is reset to verified integrated base. Evidence under own seat only. No existing reservation-product sources/docs/schemas.

Backend OMP:
- S2-M (completed foundation; full pair behavior remains B): foundation/seed/import/table helpers. Own stage-2/internal/service/model.go, model_test.go, control.go, control_reset.go, control_seed.go as needed, control_test.go, control_validation_test.go, new seating.go/seating_test.go/migration_test.go. No reservations/availability/moves/http/state/idempotency/auth/web/package changes. Covers R122,R161–R174,R178–R182,R194 and inherited portable state/receipt guarantees. Add exact helpers below, pair fixture+seed parsing, new Public schema and legacy import normalization. Old response receipts/session hashes/ids/refs/timestamps never rewritten. Full core pair occupancy lands in B; foundation tests report this limitation honestly. Acceptance full Go race/build/image plus own helper/reset/detail/seed/export/import/legacyreceipt proofs.
- S2-I (in progress, shared #23, after accepted corrected B): migration/corrupt import/receipts/concurrent combinations+batch probes. Own migration*_test.go, control_validation_test.go, seating_test.go and probes/stage2-import.sh. Product fixes confined to M ownership unless reassigned exact additional files. Covers R161–R165,R172–R173,R194–R195 plus inherited cutoff/retry/move/portability invariants. Actual distinct stage1 donor + stage2 destination processes, original receipt JSON identical, credentials/sessions/refs/statuses/failedkey reuse preserved, replacement/repeat/invalidatomic/reset.

Backend OpenCode:
- S2-D (completed): accepted stage1 donor probe/evidence. Own stage-2/probes/stage2-donor.sh ONLY. Covers setup for R161–R165,R194. Source donor41/41 and independent stage1 import/replay9/9; host donor41/41 with quoted/Unicode passwords, direct Python-assertion negative guard exit1, private full metadata audit. Original unchecked-KeyError false-green corrected before acceptance; historical logs retained. Canonical private artifacts: /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S2-D/donor-final/export.json and manifest.json. No stage2 destination claims before core. Artifacts never Git credentials.
- S2-B (completed, shared #20, corrected retention): pair CRUD/availability/atomic moves. Own stage-2/internal/service/reservations.go/reservations_test.go, availability.go/availability_test.go, moves.go/moves_test.go, writes_test.go TEST ONLY, new combinations_test.go. No model/control/seating/auth/idempotency/http/web/package changes. Covers R166–R167,R174–R186,R192–R195 and inherited booking/time/error/idempotency/moves. Consume M exact helpers, canonical sets/intersection occupancy, options order, releaseboth/capacitysum, batch input-order validation/final published occupancy+rollback. Acceptance full race/build/image/APIpair/legacy/strictness/concurrency/batch matrices; full isolatedstage2 smoke after UI integrates.
- S2-P (after B+I+H): final packaging/RUN/probes. Own stage-2/RUN.md/.dockerignore/Dockerfile only if real change needed; probes/stage2-api.sh/stage2-export.sh only (donor/import scripts retain owners). Build single image, update startup/docs, full1+2 API/browser/upgrade probes; no reviewer acceptance claim.

Frontend Grok:
- S2-G (completed source foundation; real pair acceptance pending S2-H): stage-2/web/** only. Covers R125–R160,R187–R191,R196–R199,R107–R118. Combination visual/data source foundation against explicit fixture Transport below; real accepted singles continue working. Parse options/combinable, canonical pair selection, pair-aware requests/receipt fallback, grid/oneSVG connected two-table selection, all-label summaries/confirmation/lookup. No fake production pair backend/fallback. npmci/check/test/build with Chaaya gates, controlled fixture interactions/replay/uncertain and componentcaptures375/1280lightdark; single live regression. Real paired image flow waits for B.
- S2-H (completed source/browser verification, shared #25, after accepted B and G5): stage-2/web/** only. Pair real booking/409/lostresponse/lookup/cancel and real stage1→2 browser upgrade between requests. Covers R125–R136,R161–R165,R187–R199. Pending body/key immutable incl arrays; original stage1 receipt lackingtable_ids parsed via table_id; old browser token/form/key preserved. Full actualimages/probes/screens allstates/theme/width+mainflow/reducedmotionrecordings.
- S2-R (ZCode, pending final candidate): read-only exactSHA isolated suites1+2, stage3 expectedfail, independent everyU/API/browser/upgrade/design+critic. Max3rounds. No acceptance without corroborated exactSHA/mode/verdict.

## Frozen interfaces

Existing module tablekeeper/Go1.27/Keel0.5.0, Service mutex and cloned Idempotent callbacks, clock API, passwords and receipt JSON stay unchanged. No rate limiting.

M adds Restaurant.Combinable [][]string json combinable (absent=no pairs), Reservation.TableIDs []string json table_ids, retaining legacy TableID statefield. Public always table_ids, table_id iff singleton. Legacy imported singleton normalizes ids=[oldTableID]; canonical pair stores empty legacyTableID. Receipt.Response strings NEVER rewritten: imported original pre-upgrade retries retain original stage1 JSON withouttable_ids. New GET/write schemas extended; identity/time/hash/session fields preserved.

M owns pure exact helpers, B consumes:
- type SeatingOption struct { TableIDs []string (json table_ids); Capacity int (json capacity) }
- func reservationTableIDs(r Reservation) []string : fresh storedids else legacy singleton; no mutation.
- func setReservationTables(r *Reservation, ids []string) : copiesids, TableID iff singleton else empty.
- func parseTableSelection(r *Restaurant, obj map[string]any, current []string) ([]string,*codedError) : bothformats422; wrongJSONtypes400; missingcreate/empty/duplicateids422; >2combination_not_allowed; unknown/foreigntable404; undeclaredpair422combination_not_allowed; canonical declared order unorderedcomparison. current=nil=create requires format; nonnilPATCH retains current if noselectionfields. Correcttypeinvalidformat/length422. No time/party/occupancy/state mutation.
- func seatingOptions(r *Restaurant) []SeatingOption : fresh allfixture singles then declaredpairs with summedfixturecapacities, no party/occupancy filter.
- func tableSetsIntersect(a,b []string) bool : pure set intersection.
No locks in helpers. Existing createReservationLocked/prepareAmendment/conflictingReservation signatures stay; B updates private internals. moves fieldfilter gains table_ids. No new HTTP routes required.
Producer import compatibility: positive over-capacity/offgrid/outsidehours seeds accepted, emptycollections/blanklabels/displaynames and numericUTC/legacyZulu timestamps preserved. Ordinarybookingbusiness rules belong B. Receipt canonical/body/scope/status validation retained. No invented import restrictions/dropped fixtures.

## Explicit working frontend test fixture

For G independent source tests via existing Transport seam ONLY, never productionfallback:
r_anker Zum Anker Europe/Berlin slot30 duration90 cutoff120 thu18:00–23:00; tables t_1/label1/cap2, t_2/label2/cap4, t_3/label3/cap4; combinable[[t_1,t_2],[t_2,t_3]].
Catalogue ordinary; restaurant detail exactly fields above. Availability date2027-06-17 party6 slots18:00 onward: available_table_ids=[]; available_options=[{table_ids:[t_1,t_2],capacity:6},{table_ids:[t_2,t_3],capacity:8}]. Pair1+2 occupancy excludes bothpair options in intersecting slots (sharedt_2); singlesretain original semantics.
Pair POSTresponse: reservation_id res_pair,reference PAIR01,restaurant_id r_anker,table_ids[t_1,t_2],party_size6,statusconfirmed,starts_at_local2027-06-17T18:00,starts_at2027-06-17T18:00:00+02:00,ends_at2027-06-17T19:30:00+02:00,created_at2026-10-04T22:00:00+00:00; NOtable_id. Identicalkey/body replay200sameJSON; conflict409errorcode table_unavailable; networkthrowuncertain; lookupordinarypairrecord/cancelsamerecordstatuscancelled.
Existing real accepted single backend remains working until B. G may implement only this supplied explicit fixture transport in TESTS; never unpublished functions or productionfake receipts.
Frontend request uses legacytable_id for singleton to preserve old pending request identity; pairuses table_ids. Parser accepts old table_id-only receipts plus new arrays; canonicalorder from detail, copiedarraysimmutable. localStorage session key tablekeeper-session unchanged. All productcalls through ChaayaKeeladapter with stablecodes; sameGoimage web/dist + allwoff2/licenses, tokens/themes/gates/motion/reducedmotion.

## Exact final review command

cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 2 --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S2-R1/checks

Expected1+2pass/claimedstage2/highest2/stage3fail. Source gates aren't acceptance.

## Private progress fallback

Native private task creation failed under approval_policy=never earlier. PLAN/RUNLOG remain lossy private fallback (no native ids/dependency/history); shared cards coordinate actual delegated work only. Stage1 folder never changes.


## Coordinator integration updates

- S2-M continuation: same e25f41e base/dirty worktree retained. Narrow authorized consequential files: reservations_test.go complete structural no-op comparison; state.go deep-copy Combinable inner arrays and Reservation.TableIDs; http.go GetRestaurant combinable rendering only, narrow http_test regression if needed. Restore all inherited control.go duplicate-restaurant/receipt/timestamp/restaurant-membership validation calls accidentally removed; preserve legacy producer-valid snapshots and original receipt JSON. Full race suite must compile and pass before foundation acceptance. Handoff a420d48a-0bb1-4b0d-b4fb-77efa5e5dc6d.
- S2-G2 (Grok, completed): measure complete Free/Taken/Held state-word bounds within grid cells at375/1280 both themes, including long table labels; 40/40 desktop words were measured outside borders; full-width grid fixes all22 captured grid states, host64tests/check/build pass. If actual overflow exists, refine layout while retaining all authoritative cells/testids, no horizontal page scroll, state-before-motion and existing retry/confirmation behavior. Own stage-2/web/src/app.css, AvailabilityGrid.svelte and narrowly related tests/capture script only; no transport/API changes. npm ci/check/test/build plus fixture capture and numeric word/cell bounds proof. Real paired browser/upgrade acceptance remains S2-H after S2-B.
- S2-G3 (Grok, completed source verification): ordinary-input density and long-label verification using source-only pair Transport. Valid 15-minute grid gives15slots18:00–21:30 (90-minute duration,18:00–23:00 hours);5-minute grid gives43slots. Verify all cells/options/hooks/labels against returned slots and word bounds at375/1280 both themes, retaining8-slot regression. If layout fails, adapt generally to available width/slot count, never fixture ids/dates/test names. Own stage-2/web app.css, AvailabilityGrid/strictly related layout components and narrow tests/capture only; no transport/auth/booking/API changes. Actual pair/harness acceptance still B/H/R.
- S2-M final repair (same dirty e25f41e base, no reset/commit): host targeted gate independently reproduces negative-cutoff import204, synthetic producer snapshot mismatch and real single-table swap returning its old table set. Additional narrow authorized ownership: reservations.go canonical singleton setter in createReservationLocked and prepareAmendment only; control_validation_test.go synthetic seededSource canonical record remains original M ownership, no weakened assertions. Restore cutoff<0 and empty/oversized stored-table-set checks; verify canonical stored pair/mixed fields. Fresh imported failed-key first valid201/replay200 must replace unsubstantiated409 reuse claim. Full green source gates required before acceptance; no race-still-running report as finished proof. Handoff c1e3f535-0823-4e66-96f0-4dc5fb9402e5.

- S2-G3 accepted source item: host check/test/build all exit0, 66 tests; raw dense proof28shots/8uncertainposts and8-slot26shots/12posts, zero word/head collisions or page overflow. Dense desktop columns scroll inside their panel; phone stack unchanged. Live paired booking and upgrade remain B/H/R. Floor pair badge caption uses full joined labels in a fixed92x28 SVG chip; H must measure long-label SVG ink and repair actual overflow, retaining accessible full names and immediate selection.

- S2-M accepted: host targeted3tests/fullnonrace/vet/build/fullrace all exit0 (host race service139.240s/clock1.042s). Scope includes narrow idempotency_test synthetic canonical setter, preserving original receipt assertions. Peer image donor proves5original receipts unchanged/session/hash/pending retry/fresh failed-key201 then200. Peer race rawlog lacks command/exit wrapper, so acceptance relies on completed host race log with exact command/exit. Legacy-named unit test currently exports modern producer rather than genuine old state; actual old image donor establishes migration, and I must add a truthful legacy-shaped unit fixture without weakening receipt checks. Stage1 unchanged.
- S2-G4 (completed source item, shared #21, Grok): bounded SVG pair badge long-label ink measurement and repair only if real overflow, with full human names retained in accessible selection and ordinary labels/summary. Own PairMark.svelte/FloorPlan.svelte, caption/badge-only floor.ts/pairs.ts if needed, pair-only app.css if needed, narrow pair-badge test and capture. No auth/transport/booking/search/grid/backend/package/doc changes. Existing complete fixture Transport is the explicit working stub; no real API/harness claim. Run UI check/alltests/build and measured375/1280themes/reducedmotion proof. Independent of B.

- S2-B continuation required (same dirty a588c4c base, no commit/reset): all112top/63subtests pass but host HTTP reproduction finds pair-retention defect. reservations.go amendmentTables synthesizes table_ids as []string for omitted selection; parseTableSelection consumes decoded []any. Pair PATCH{},party-only,time-only and no-op move each return400 malformed_request instead of success. Repair internal merged-body representation in owned reservations.go, retain strict external type errors, add regression tests in owned combinations_test.go for omitted pair selections and mixed batch retention/replay/rollback. Original case evidence seatright-codex/S2-B/host-pair-retention-before.txt. B not accepted and I/H still depend on corrected integrated B. G4 recent source changes confirm active progress.

- S2-G4 accepted source: host check/all70tests/build exit0 and peer14before/after SVG captures numeric collision checks verified; before boundsFail14, after0 with no page overflow. Raw build afterJSBa13qXa5 differs from beforeBl77_tTU; report same-asset-name statement inaccurate, CSS retained legitimately. Pair captions stay13userunits, full names and grown clear placement. Independently viewed after desktop floor confirms separate real table plate overflow: Window alcove runs beyond its circle into adjacent tabletop.
- S2-G5 (completed source item, shared #22, Grok): only PlanTable.svelte and label/caption geometry/obstacle portions of floor.ts, narrowly FloorPlan if required, plate/name-only app.css if required, narrow label tests/capture. Fix verified table plate/name ink overflow generally; retain capacity-shaped tables, seats, full human names in accessible controls/place legend/grid/summary, accepted pair badge clearances and dense grid states. Numeric actual DOM SVG label bounds and collision measurements375/1280themes including selected/reducedmotion, UIcheck/alltests/build. No backend/transport/auth/booking/grid/package/docs edits. Source verification only, no real pair/harness claim.

- S2-B accepted after retention fix: four owned files only, decoded-array synthesis repaired without changing external strict parser; peer fullrace114top/63subtests0fail294.251s/newimage9retention+13standardchecksPASS. Host full114+63/vet/build pass and original fourHTTPrepros now PATCH200/200/200,no-opbatch201,replay200 identical. Author63abeeab69bd66e505454e9eb6b8f9d07e969101 mergedb874cea90fb02d13baceaa62b3a60575e61c3c56. No final UI/harness acceptance claim.
- S2-P1 (completed, shared #24, OpenCode): independently package reusable actual-image API probe in stage-2/probes/stage2-api.sh ONLY. Host and actual-image231checks PASS, both negative guards fail as expected, exact offline commands/provenance verified. Covers named inheritedAPI and combined option/occupancy/amend/move/error/idempotency/concurrency cases; source Go/UI unchanged. Final P image+liveUI verification still follows I/H.
- S2-P2 (completed, shared #26, OpenCode): stage-2/RUN.md ONLY. Corrected copied stage1 paths/title/schema/browser wording for stage2 singles and declared pairs, source/Docker commands, legacy immutable receipt/session compatibility and probe interfaces/dependencies. Doc-only read-only contract verification and git diff --check clean, no product tests required or claimed. Final P refreshes pending import-probe wording after I/H; P/R still await them.

- S2-G5 accepted source: host check/all75tests/build EXIT0, raw14plate/caption cases before14own-top failures→after0 and14badge regressions0 verified. Viewed desktop afterfloor confirms full wrapped names within own capacity top, room viewBoxes unchanged; names/focus/hook/member selection and fonts13/19 retained. Authora38403ad05881da10388d6f61b1fa85b5b0fdc81 merged95a0b1ffdb988d04ffcc8f6fa0d9a473821a5dd1. Fixture transport proof only.
- S2-H real upgrade proof must load accepted stage1 UI in a browser, commit-and-drop its booking response, retain actual pending form/key/body and signed-in session, export source and import into a distinct stage2 image, then forward subsequent original-browser API requests to stage2 without reload/new screen. Destination must authoritatively reply200originalreceipt; no fabricated/cached success. New stage2 UI separately exercises real combined bookings. Record actual twoimage/container/PORT provenance, original source commit201/lostresponse, export200/import204/retry200 and exact samekey/body/reference plus token-backed lookup. Source and destination processes remain distinct; routing/bridge may switch target between browser requests but must fetch actual serverresponses.
- S2-P1 continuation required, shared #24 remains in progress on dirty04befc9base: host exact existing script167PASS/EXIT0, sabotageEXIT1 and unreachableEXIT7 corroborated. Report/header overclaim omitted pair-selection error matrix/nontransitivity/capacity/grid/hours, pair time-only/time+party retention, pair/mixed batch occupancy rollback and valid successful cross-path key reuse. Add explicit checked cases and replace inert reversed-PATCH cmp with equality to immediate pre-reversal record. Offline log currently uses ellipsis rather than complete executed bundled-probe arguments/image+container provenance; require corrected exact command evidence. Full self-contained continuation dfd41089-64cc-4c30-ace7-159dfba1fc29; only stage2-api.sh owned, no product/source/doc edits. Existing checks retained; host logs evidence/seatright-codex/S2-P1. No commit or acceptance before complete verifiable probe report. I/H remain active independently.
- S2-P1 continuation resolved: only341-line probe file, host231PASS/EXIT0 plus sabotage1/unreachable7, peerimage231PASS and full offline none-network exec commands/container identity verified. Added strict pair matrix, capacity/grid/hours/nontransitivity, time-only/time+party retention with occupancy, whole-response reversed no-op equality, shared-member conflict rollback/reusable key, pair/mixed batch retention and successful cross-path first201/replay200. Same-image export/import remains bounded smoke; true upgrade remains I/H/R. Author872c899204f2d5c04f961410852807406cc974cc merged775d4a9755aa2775798ef7a5c01954a600cac29a. Shared#24completed; no finalstage2 acceptance claim.
- S2-P2 continuation required, shared#26inprogress: doc-only diff one ownedRUN.md, no source gates needed. Incorrect GET /restaurants/{reference} must read GET /reservations/{reference}; new ordinary response table_ids rule must preserve old immutable receipt replay exceptions; offline client example must not depend on unset host PORT; wording must explicitly allow singleton arrays and nontransitive pairs. Full continuation00062874-d6a2-406a-9f2d-21ce39733bfb on same dirtyc8fcbb9base, no reset/commit. Historical review note retained, corrected final note required; I/H remain independentactive.
- S2-P2 resolved and accepted: route/current response versus old receipt/singleton arrays/nontransitivity corrections verified against router/seating/idempotency, explicit9027 client URL removes host interpolation. One owned RUN.md diff87+/50-, doc-only review notes plus coordinator diff --check; no source gates claimed. Authore524b838dd425bdbad6bde9828cfd4dafe8787bc merged356c5564b84fbb8ad648239c04daef66f29e4693; shared#26completed. Final P/R still await integrated I/H and final guide refresh for new import script. Earlier required-continuation entry remains historical.
- S2-I continuation required, shared#23inprogress: host completed vet/fullnormal/build/fullrace117top+63subtestsPASS service164.806s/clock1.039s; host currentprobe30PASS against distinct actual stage1/stage2 binaries. Proof gaps remain: lookup only200 and manifest compares itself rather than destination currentrecords; only pending originalreceipt replayed; pairreceipt replay precedes import and no subsequent mutations despite claims; syntheticlegacyresponse onlyreference rather than fullordinary response; reset/fullrepeat/source snapshot/failed-assertion secrecy/provenance not fully evidenced. Full bounded continuation30e0bdbe-0b9e-4a53-8270-ce909ca52a2a on same dirty04befc9base requires truthful tests/checks and fresh+canonical actual-image transfer logs, fullcommands/IDs and sanitized failure stdout. BrowserR165 deferredH. No product bug found or coordinator product edits; only migration_test/probe fixes authorized.
- S2-H continuation required, shared#25inprogress: host UIcheck/all75tests/build/node--checkPASS; actual twoimage sameDocument commit/drop/export/import/destinationfetch/retry proof verified,64shots/fourvideos and layout inspected. Real selectedpair contrast defect: .cell.together[data-available=true] specificity0,3,0 retainsaccent-soft over selectedrule0,2,0 while stateword useson-accent. Light source-role contrast1.333:1, dark1.500:1; observed confirmeddesktop screenshot paleHeld. Existing25rolepairs miss erroneouscascade. Full continuation18e9ec5e-5dab-407f-bbb1-e8fe755e80dc asks general CSSfix and actual rendered foreground/background4.5+ measurement at375/1280themes for single/pair states plus full real-image/browser regression. Same dirty15ee59fbase retained, no commit/reset. Evidence seatright-codex/S2-H/host-contrast-observation.txt. No stage2 acceptance until correctedI/H and finalP/R.

- S2-H accepted after rendered contrast repair: only app.css, new selection-contrast.test.ts and real s2-h-live.mjs; host node--check/check/77tests/build EXIT0. Actual before computed pair text contrast1.333 light/1.500 dark; fixed available+selected pair selector outranks soft wash, after28samples weakest4.971 with selectedpair7.611/9.915 at375/1280. New image dc49484a/container5fd78c66:9029 and reused immutable stage1image983bf3cb/container639d22a3:9028 prove sameDocument lostsource201→export200→import204→destination200originallegacyreceipt/noarrays, samekey/body/receipt/reference, tokenlookup200/onebooking/noReload. All36checksPASS,64shots/fourvideos, zeroofforigin/pageerrors/overflow; observed desktop confirmed image correct. Author1e99a2338cde22f2ce958006ad0c4a5d2cbc2a0c mergedbb584a0b44f5a9405385ca4c03478b389b590280, shared#25completed. No supplied harness or stage acceptance claim.
- S2-I assertion repair still required: report closes most proof gaps, but stable.py line251 includes always-true or True and only receipts checked later; unit currentparty compared against original reservations ARRAY cannot prove mutation; reset proves only token401 while claiming allstate removed. Bounded handoff f44c9b6d-79b4-477d-88a6-718b26103e50 requires complete fresh export equality, meaningful originalparty5/current6/full-record difference plus unchanged original replay, and actual reset export replacement checks. Only migration_test.go/importprobe owned, same dirty04befc9base retained. Full actual command paths and fresh/canonical pass-versus-skip separation required. Shared#23remainsinprogress; finalP/R still awaits I.
