# Tablekeeper delivery plan

Stages are implemented and independently accepted in order. Stage 1 and stage 2 are accepted and immutable. Stage 3 is active; stage 4 starts only after stage 3 acceptance. This snapshot replaces the missing active room plan and records the current contracts and work queue. The full stage ledger follows the architecture.

```arch
{
  "kind": "layered",
  "title": "Tablekeeper: dated policy decisions and atomic recurring reservations",
  "layers": [
    {
      "id": "browser",
      "title": "Diner experience",
      "items": [
        {
          "id": "restaurant_ui",
          "label": "Search, SVG floor, booking, lookup",
          "detail": "Svelte and Chaaya; authoritative availability; immutable pending retry; existing screens"
        }
      ]
    },
    {
      "id": "api",
      "title": "HTTP service",
      "items": [
        {
          "id": "policy_engine",
          "label": "Dated policies and accepted terms"
        },
        {
          "id": "reservation_engine",
          "label": "Atomic bookings, revisions and moves"
        },
        {
          "id": "history_engine",
          "label": "Immutable per-booking history and decision"
        },
        {
          "id": "series_engine",
          "label": "Recurring agreements and exceptions"
        },
        {
          "id": "clock_engine",
          "label": "IANA wall times and half-open intervals"
        }
      ]
    },
    {
      "id": "state",
      "title": "Atomic state and portability",
      "items": [
        {
          "id": "snapshot_store",
          "label": "Mutex transaction, deep snapshots, opaque exports"
        },
        {
          "id": "receipt_store",
          "label": "Immutable idempotency receipts and sessions"
        }
      ]
    },
    {
      "id": "delivery",
      "title": "Delivery and verification",
      "items": [
        {
          "id": "service_image",
          "label": "Single offline Go + Svelte image"
        },
        {
          "id": "review_gate",
          "label": "Exact-SHA isolated checks and independent review"
        }
      ]
    }
  ],
  "flows": [
    {
      "from": "restaurant_ui",
      "to": "api",
      "label": "Same-origin Keel adapter"
    },
    {
      "from": "reservation_engine",
      "to": "policy_engine",
      "label": "Resulting local date selects terms"
    },
    {
      "from": "reservation_engine",
      "to": "history_engine",
      "label": "One entry per real change"
    },
    {
      "from": "series_engine",
      "to": "reservation_engine",
      "label": "One atomic agreement operation"
    },
    {
      "from": "api",
      "to": "clock_engine",
      "label": "Generic DST resolution"
    },
    {
      "from": "api",
      "to": "state",
      "label": "One transaction and immutable replay"
    },
    {
      "from": "service_image",
      "to": "review_gate",
      "label": "Frozen revision acceptance"
    }
  ]
}
```

# Tablekeeper stage 3: policies, history and recurring reservations

Status: stage2 accepted and copied forward; S3-P (OMP/shared32), S3-H (OpenCode/shared33) and S3-G (Grok/shared34) are in progress from integrated base1dd5e0ec555216d3607374cbf251bf2c637335c5. Stage3 is not accepted. Earlier stage folders are immutable.

## Requirements ledger

R1–R199 are inherited unchanged from stage2, with stage3's explicit policy/history/privacy extensions below taking precedence where stated. S=at least partial supplied coverage; U=no supplied check identified, independent evidence required. Stage3 supplied sample has seven tests; it does not cover most policy, history, agreement, migration or collective edge cases. Full specs are authoritative.

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
| R200 | stage-3: Availability explanations | S | With explain=true each table's capacity and no_overlap rules are evaluated independently. |
| R201 | stage-3: Availability explanations | U | A supplied explain parameter accepts exactly true; false, 1, empty and every other value return422 validation_failed. |
| R202 | stage-3: Availability explanations | S | Without explain the availability response has no explanation fields and retains the preceding-stage shape. |
| R203 | stage-3: Availability explanations | S | Every explained slot includes every restaurant table exactly once in fixture order. |
| R204 | stage-3: Availability explanations | S | Each explanation lists capacity then no_overlap, including both true/false results without omission. |
| R205 | stage-3: Availability explanations | S | Explanation.available equals the conjunction of both rules and true ids exactly equal available_table_ids in order. |
| R206 | stage-3: Availability explanations | U | A confirmed pair makes no_overlap false for every occupied member independently of party capacity. |
| R207 | stage-3: Availability explanations | U | Closed days return slots=[] and zero-available slots retain full explanations. |
| R208 | stage-3: Availability explanations / policies | U | Each table explanation includes the effective policy_version for the slot's local start date. |
| R209 | stage-3: Reservation history | S | GET /reservations/{reference}/history returns reference and entries oldest-first for the owner. |
| R210 | stage-3: Reservation history / decision | U | History and decision return404 not_found for unknown, non-owner and unauthenticated requests, including managers who are not owners. |
| R211 | stage-3: Reservation history | U | Cancelled reservations retain readable owner history. |
| R212 | stage-3: Reservation history | U | History seq starts1, increments exactly1 and orders entries even for writes in the same second. |
| R213 | stage-3: Reservation history | U | History at values are RFC3339 with explicit offsets and nondecreasing in seq order. |
| R214 | stage-3: Reservation history | S | A singleton created entry names table_id, starts_at_local, party_size in order with from=null. |
| R215 | stage-3: Reservation history | S | A changed entry names only actually changed fields with complete old/new values in table-selection, starts_at_local, party_size order. |
| R216 | stage-3: Reservation history | U | A successful no-op PATCH adds no history entry. |
| R217 | stage-3: Reservation history | U | Cancelled entries have changes=[] and no later ordinary diner entry follows cancellation. |
| R218 | stage-3: Reservation history / retries | U | Replayed idempotent creates add no history and return the original stored response. |
| R219 | stage-3: Existing screens | U | Explanations and history require no new screens and the grid retains every stage-2 hook and availability rule. |
| R220 | stage-3: Policies / managers | U | Reset accepts manager_user_ids per restaurant, defaulting to an empty array. |
| R221 | stage-3: Policies / permissions | U | Policy publication rejects absent/invalid tokens401, unknown restaurants404, authenticated non-managers403 forbidden. |
| R222 | stage-3: Policies / permissions | U | Managers gain no access to other diners' private reservation, history or decision records. |
| R223 | stage-3: Policies / publication | S | POST /restaurants/{id}/policies is an idempotent write accepting a complete policy and returning201 with policy_version. |
| R224 | stage-3: Policies / publication | U | Versions start1 and increment once per successful new publication per restaurant. |
| R225 | stage-3: Policies / publication | U | Failed publications and successful replays allocate no policy version or state change. |
| R226 | stage-3: Policies / publication | U | Policy0 is the original fixture and applies before any eligible published policy. |
| R227 | stage-3: Policies / selection | U | Policy selection uses a booking's local start date and chooses greatest effective_from <= date. |
| R228 | stage-3: Policies / selection | U | Equal effective dates choose greatest policy_version even when publication and effective-date orders differ. |
| R229 | stage-3: Policies / selection | U | Effective dates in the past are accepted and a newer same-date policy affects only future decisions. |
| R230 | stage-3: Policies / validation | U | Every documented policy field is required; a partial policy is422 validation_failed. |
| R231 | stage-3: Policies / validation | U | effective_from is a real strict YYYY-MM-DD date. |
| R232 | stage-3: Policies / validation | U | Policy grid and duration are integers1..1440, rejecting booleans and other invalid types/ranges. |
| R233 | stage-3: Policies / validation | U | Policy cutoff is an integer0..10080, rejecting booleans and other invalid types/ranges. |
| R234 | stage-3: Policies / validation | U | Policy opening_hours follows stage-1 weekday/HH:MM/non-overnight rules and has no duplicate weekdays. |
| R235 | stage-3: Policies / validation | U | Policy capacities names exactly the fixture table ids with integer capacities1..100. |
| R236 | stage-3: Policies / validation | U | Every invalid policy yields422 validation_failed with no version, receipt or state mutation. |
| R237 | stage-3: Policies / immutability | U | Published policies are immutable and cannot change table ids, labels, timezone or declared pairs. |
| R238 | stage-3: Policies / unknown fields | U | Unknown policy fields are ignored for policy behavior while the full canonical request still determines replay identity. |
| R239 | stage-3: Policies / listing | U | Public GET /restaurants/{id}/policies returns policies in publication order, omitting policy0. |
| R240 | stage-3: Policies / original detail | U | Ordinary restaurant detail retains its original fixture configuration after publication. |
| R241 | stage-3: Policies / decisions | S | Availability slot grid, duration, hours and capacities use the date-selected policy rather than original detail. |
| R242 | stage-3: Policies / responses | S | Every current reservation response has revision1 at creation and complete accepted_terms. |
| R243 | stage-3: Policies / terms | U | accepted_terms snapshots all selected policy fields and policy_version but excludes effective_from. |
| R244 | stage-3: Policies / seeds | U | Seeded reservations start revision1 with original policy0 terms. |
| R245 | stage-3: Policies / retries | U | Earlier idempotency receipts retain their exact original JSON, revision and terms after future changes or imports. |
| R246 | stage-3: Policies / nonretroactivity | U | Publication changes no existing booking terms, end times, revision or history. |
| R247 | stage-3: Policies / cancellation | U | Cancellation checks the booking's accepted cutoff against its current start. |
| R248 | stage-3: Policies / amendments | U | A real amendment checks old accepted cutoff before all resulting field validation against the resulting date's policy. |
| R249 | stage-3: Policies / amendments | U | A real amendment atomically replaces terms and end time and increments reservation revision once. |
| R250 | stage-3: Policies / no-op | U | A no-op retains terms, end time and revision and records nothing, but still requires confirmed/editable status. |
| R251 | stage-3: Policies / failed writes | U | Failed amendments change no records, terms, histories, revisions or exception flags. |
| R252 | stage-3: Policies / cancellation | U | Successful cancellation increments revision once; repeated cancellation increments nothing. |
| R253 | stage-3: Policies / optimistic revision | U | Optional PATCH expected_revision must be a positive integer; invalid type/range including booleans is422. |
| R254 | stage-3: Policies / optimistic revision | U | Mismatched expected_revision returns409 stale_revision before cutoff or booking-field validation. |
| R255 | stage-3: Policies / optimistic revision | U | Omitted expected_revision retains preceding-stage behavior and unrelated unknown fields remain ignored. |
| R256 | stage-3: Policies / concurrency | U | Concurrent amendments with one expected revision cannot both make real changes. |
| R257 | stage-3: Policies / history snapshots | U | Each history entry carries its resulting revision and complete terms; old entries never acquire new terms. |
| R258 | stage-3: Policies / decision | S | Owner GET /reservations/{reference}/decision returns current reference, revision and terms even after cancellation. |
| R259 | stage-3: Recurring / adoption | S | POST /series adopts an existing owned confirmed editable reservation as occurrence0 and requires idempotency. |
| R260 | stage-3: Recurring / authentication | U | Series adoption without a valid token is401 unauthenticated. |
| R261 | stage-3: Recurring / anchor errors | U | Unknown/non-owned anchor404, cancelled409 reservation_cancelled, already adopted409 already_in_series. |
| R262 | stage-3: Recurring / anchor cutoff | U | Anchor adoption obeys its accepted cancellation cutoff. |
| R263 | stage-3: Recurring / validation | U | count is an integer2..12 including anchor; interval_weeks is an integer1..4; booleans and invalid values are422. |
| R264 | stage-3: Recurring / anchor identity | S | Adoption preserves occurrence0 reference, identity, revision, terms, history, timestamps and original receipt exactly. |
| R265 | stage-3: Recurring / calendar | U | Occurrence i uses anchor's original local calendar date plus i*interval_weeks*7 days and the same local clock time. |
| R266 | stage-3: Recurring / dated policy | U | Each generated occurrence independently selects its date's policy including duration and capacity. |
| R267 | stage-3: Recurring / validation | U | Generated occurrences obey ordinary opening, grid, occupancy and accepted table/party validation. |
| R268 | stage-3: Recurring / DST | U | A nonexistent generated wall time rejects all adoption with invalid_local_time; folds choose the first instant. |
| R269 | stage-3: Recurring / inherited fields | U | Generated occurrences retain anchor party size and canonical selected table set. |
| R270 | stage-3: Recurring / atomic failure | U | Failed adoption leaves no partial series, reservations, histories, counters or idempotency claim. |
| R271 | stage-3: Recurring / precedence | U | The first failing generated occurrence by index determines the ordinary booking error. |
| R272 | stage-3: Recurring / response | S | Adoption returns201 with opaque series_id, revision1, interval_weeks and every occurrence in index order. |
| R273 | stage-3: Recurring / stable identities | U | Every occurrence has a distinct ordinary reservation reference, and reference/index stay fixed after later edits. |
| R274 | stage-3: Recurring / ordinary behavior | U | Occurrences appear in ordinary lists, occupy tables and have ordinary histories. |
| R275 | stage-3: Recurring / current lookup | U | Owner GET /series/{id} returns the same shape with current reservation states. |
| R276 | stage-3: Recurring / privacy | U | Unknown, non-owner or unauthenticated series GET returns404 not_found. |
| R277 | stage-3: Recurring / exceptions | U | Real individual PATCH permanently marks its occurrence exception=true and increments series revision once. |
| R278 | stage-3: Recurring / no-op failure | U | No-op and failed individual amendments change neither series revision nor exception. |
| R279 | stage-3: Recurring / cancellation | U | Cancellation increments series revision once, retains cancelled occurrence and does not mark a new exception. |
| R280 | stage-3: Recurring / cancellation | U | Repeated occurrence cancellation changes no series revision. |
| R281 | stage-3: Recurring / independence | U | Cancelling an anchor leaves sibling bookings unchanged. |
| R282 | stage-3: Recurring / checks | U | Ordinary reservation cutoff and optimistic revision checks apply to all occurrences. |
| R283 | stage-3: Recurring / restaurant revision | U | Adoption increments restaurant revision once for the whole successful operation. |
| R284 | stage-3: Recurring / receipts | U | Series replay returns the exact original response after later edits/cancellations and changes no counter. |
| R285 | stage-3: Recurring / request semantics | U | Series is a distinct idempotency path and ignores unrelated unknown fields. |
| R286 | stage-3: Migration | U | Stage3 imports unchanged exports produced by this team's accepted stages1 and2. |
| R287 | stage-3: Migration / adoption | U | Adoption works on imported legacy reservations without losing their identities or links. |
| R288 | stage-3: Migration / sessions | U | Existing confirmation references, bearer sessions and original booking retries remain valid across stage1/2 to3 import. |
| R289 | stage-3: Combined history / terms | U | Pair capacity sums selected-policy capacities, never stale original-fixture capacities. |
| R290 | stage-3: Combined history | U | Singleton-to-singleton history retains table_id with scalar before/after values. |
| R291 | stage-3: Combined history | U | Pair creation replaces table_id change with table_ids from null to the full canonical pair. |
| R292 | stage-3: Combined history | U | Any table-selection change involving a pair records table_ids complete canonical before/after lists. |
| R293 | stage-3: Combined history | U | Canonical pair order follows declared order and reversed input alone is a no-op. |
| R294 | stage-3: Combined history / policies | U | Policy selection, revision and replay rules apply identically to singles and pairs. |
| R295 | stage-3: Collective moves / policies | U | Every real move uses old accepted cutoff then resulting-date policy for all resulting fields. |
| R296 | stage-3: Collective moves / revision | U | Per-move optional expected_revision follows PATCH type/range/stale precedence. |
| R297 | stage-3: Collective moves / no-op | U | No-op moves retain terms, revision and history. |
| R298 | stage-3: Collective moves / atomicity | U | Every resulting booking obeys amendment and final occupancy rules; failure preserves every booking. |
| R299 | stage-3: Collective moves / history | U | Each changed booking gains one revision and one ordinary changed entry. |
| R300 | stage-3: Collective moves / restaurant revision | U | A successful batch with real changes increments restaurant revision once for the whole batch; all-no-op increments nothing. |
| R301 | stage-3: Collective moves / series | U | Each affected series revision increases once per changed batch regardless of member count. |
| R302 | stage-3: Collective moves / exceptions | U | Every changed series occurrence in a move batch becomes a permanent diner exception. |
| R303 | stage-3: Collective moves / replay failure | U | Failed batches and replays change no revisions, histories, receipts or exception flags. |

## Work items and sequencing

Each item is bounded to one foreground turn (target20–30 minutes plus measured gates). No seat runs state-changing Git. The coordinator validates scope/raw evidence/host gates, commits with the seat author, and merges without rebasing, squashing or amending. Each new work item begins on a clean fixed worktree reset to the latest integrated revision. Do not start dependent items before their interfaces are integrated. Full handoffs include the original task, all four verbatim specs and this ledger.

First independent wave:
- **S3-P — OMP — in_progress when dispatched.** Own only stage-3/internal/policy/** (new pure package). R226–238/R241/R243–244/R266–268/R289/R294–295 policy-engine portions. No model/state/core/router/reset/web edits. Build strict complete-policy parsing, effective selection, snapshot cloning, clock rules and selected-set capacity using the frozen package contract below. Acceptance: gofmt, go vet ./internal/policy, go test -race -count=1 -v ./internal/policy, go build ./...; general non-fixture-date tests, malformed matrices, same-date supersession, publication/effective order disagreement, immutable clone and pair capacity. This does not claim endpoints or migration.
- **S3-H — OpenCode — in_progress when dispatched.** Own only stage-3/internal/history/** (new pure package). R209–218/R257, R290–293, replay/no-op helper portions. No model/state/core/router/reset/web edits. Build immutable history value construction and actual change detection with frozen contract below. Acceptance: gofmt, go vet ./internal/history, go test -race -count=1 -v ./internal/history, go build ./...; creation/single/pair transitions, reversed-pair no-op, ordered multi-field changes, sequence/time ties, cancellation empty changes and deep clone tests. This does not claim service history routing.
- **S3-G — Grok — in_progress when dispatched.** Own stage-3/web/** only. R110–118/R125–160/R187–199 regressions plus R219/R240–245/R288 response compatibility. Use the explicit existing Transport fixture below for tests only; no backend dependency and no production fallback. Exercise dated-policy results despite unchanged fixture detail, current extra revision/terms fields plus legacy receipts, selected pair contrast, viewport reveal and stable retry identity. Update the copied stage-2 foundation ban so stage-3 policy/history/series words are permitted; keep stage-4 replan behavior absent. No new screens or manager/adoption flows are required. Fix only demonstrated existing-screen compatibility defects, retain all hooks. Acceptance: npm ci; npm run check; npm test with Chaaya contrast/a11y gates; npm run build. Label fixture browser captures as fixture transport. Real policy/upgrade browser proof follows integrated backend.

Subsequent queued items (all pending; precise owned seams finalized in fresh handoff from the integrated base):
- **S3-M — OMP — after P+H.** Small shared model/state foundation and producer/migration defaults: policy/history types, revision/accepted terms, managers, series membership/exception/original schedule metadata, per-restaurant counters; deeply cloned nested maps/lists/JSON; seed revision1/policy0/created history; normalize true old exports without rewriting opaque receipts. Own model.go, state.go, control*.go, new version_state.go plus narrow explicitly delegated producer hooks/tests. Do not absorb full booking or series behavior. Maintain producer-valid off-grid/over-capacity/cancelled seeds. Source build+unit/race and old producer snapshot consistency; actual older donor lane below supplies genuine migration.
- **S3-D — OpenCode — parallel M after H.** Prepare genuine accepted-stage1 and stage2 export donors/receipt manifests in private0700 directories, no credentials/stdout/Git. Own new probes/stage3-donor.sh only. Cover past/current/cancelled/moved bookings, pairs for stage2, original create/move receipts, failed reusable key, multiple sessions. Two source images required; guard counts and nonzero failure. No destination claim.
- **S3-B — OpenCode — after M.** Policy publication/listing with manager permissions and exact idempotency ordering, dated availability/explanations, ordinary create/amend/cancel accepted terms and history/decision/expected revisions. Split B into B1 policy+availability and B2 ordinary writes/history if scope exceeds one turn; do not leave an unbuildable shared seam. Own policy_service.go, availability.go, reservations.go/writes.go and narrowly assigned http routes/tests. Test real whole-field validation, old-cutoff before resulting rules, no-op retention, same-revision race, independent explanations. Publication cannot retroactively mutate bookings.
- **S3-S — OMP — after B2.** Series adoption/current owner-only lookup, weekly local calendar generation, unchanged anchor, independent dated policies/DST and atomic rollback. Own series.go/series_test.go plus series router arm only. Reuse integrated ordinary prepared creation/commit helpers without incrementing restaurant counter once per generated member. Test pairs, DST skips/folds, per-index precedence, occupancy with sibling generated bookings, anchor preservation and immutable replay.
- **S3-C — OpenCode — after S.** Collective policy-aware moves and individual occurrence exception/cancellation propagation. Own moves.go/tests and narrowly named transaction-finalization seams; no changes to policy engine. Restaurant/each affected series counter once per real batch, no-op/replay/failure none. Test expected-revision precedence, multi-member series, swaps, mixed-policy changes, concurrent revisions, full rollback. Existing ordinary per-occurrence propagation may be implemented with B2 if integrated membership helpers already exist; C re-verifies whole semantics.
- **S3-I — OMP — after C.** Harden modern policy/history/series state validation and full real stage1/2→3 portability. Own migration tests, control validation deltas and new probes/stage3-import.sh; preserve producer contract and original receipts. Test adoption of old imported anchors, cancellation/moves/exceptions in modern series, repeated import, current-state history/decision and old frozen receipts/session/hash replacement. Full JSON comparisons, guards and no always-true assertions.
- **S3-G2 — Grok — after B/C.** Real image browser regressions with policy grid/capacity behavior and true stage1→3 plus stage2→3 same-Document uncertain retry/lookup/session recovery; no reload/fake cached reference. Current terms/revision extras and original legacy response accepted. Original detail stays unchanged. Capture every named state, widths375/1280 light/dark/system, reduced motion, viewport action visibility, floor/grid bounds and computed text contrast.
- **S3-P2 — OpenCode — after I/G2.** Final Docker/RUN.md/probe packaging accuracy and comprehensive stage3 API probes incl policies/history/series/revision/concurrency/portable errors. Reuse untouched completed source gates; build actual combined image, default/nondefault/offline assets/fonts/API and resource headroom. Optional supplied smoke is context only.
- **S3-R — ZCode — after all integration.** Exact-SHA full isolated stage1–3 and expectedstage4 failure, all unsampled ledger probes, general DST/concurrency/migration and design pass. At most3 rounds. ACCEPT only if evidence report names exact frozen candidate and modeisolated. Archive safe review evidence on main before copy to stage4.

Backend queues are shared comparably between OMP and OpenCode; Grok starts immediately with no missing backend. Reviewer stays idle until a concrete inspectable candidate exists. The coordinator never writes merged product code or tests; conflicts are aborted and sent to the owners.

## Frozen first-wave package interfaces

Existing module is tablekeeper, Go1.27; clock remains tablekeeper/internal/clock. All first-wave package code is pure and owns its own types, so neither backend item needs the other.

### internal/policy (S3-P)

Define:
- type Hours struct { Weekday string; Opens string; Closes string }, exact JSON weekday/opens/closes tags.
- type Terms struct { PolicyVersion int; SlotMinutes int; ReservationDurationMinutes int; CancellationCutoffMinutes int; OpeningHours []Hours; Capacities map[string]int }, exact JSON names from specification.
- type Policy struct { EffectiveFrom string; Terms }, anonymous Terms embedding so JSON is flat (not a nested terms property).
- type Error struct { Code string; Message string }; Error() string. All Parse errors Code=validation_failed; HTTP integration maps policy validation to422.
- Parse(obj map[string]any, tableIDs []string) (Policy,error): complete strict documented policy, caller passes decoded JSON object; unknown fields ignored, version is not caller-set and defaults0. Validate exact capacities membership, weekday duplicates/time bounds and all policy-specific integer ranges/types. Invalid JSON itself remains HTTP ParseBody responsibility.
- Select(base Terms, published []Policy, date string) (Terms,error): strict actual date; choose eligible greatest date then version, otherwise copied base. Does not allocate versions or mutate input.
- CloneTerms(Terms) Terms and ClonePolicy(Policy) Policy: deep copy hours/map.
- Rules(Terms, timezone string) clock.Rules and Capacity(Terms, tableIDs []string) int.
Fixture policy0 may have original stage1 values outside new-publication maxima; Select/Clone/Rules must not reapply publication restrictions to fixture terms. Capacity only sums validated selected ids; selection/canonical pair legality stays existing service/seating.go. Parse takes ordinary decoded []any/maps/float64 values and rejects bool/invalid types as422 policy validation. Package never locks, allocates opaque ids, publishes policy, reads system current time or reads service state.

### internal/history (S3-H)

Define:
- type Snapshot struct { TableIDs []string; StartsAtLocal string; PartySize int; Revision int; AcceptedTerms json.RawMessage }.
- type Change struct { Field string; From any; To any }, JSON field/from/to.
- type Entry struct { Seq int; At string; Event string; Changes []Change; Revision int; AcceptedTerms json.RawMessage }, JSON seq/at/event/changes/revision/accepted_terms. No stage4 fields yet.
- Next(entries []Entry, now time.Time) (seq int, at string): seq=len(entries)+1 for well-formed producer histories; produce numeric-offset RFC3339 UTC, clamp to the last entry instant if wall clock moves backward. Parse valid existing entry at, no named-date behavior.
- Created(after Snapshot, at string) Entry: seq1/eventcreated, all three fields in selection/time/party order, From=nil, resulting revision/complete raw terms copied. Single→table_id scalar; pair→table_ids copied full list.
- Changed(before, after Snapshot, seq int, at string) (Entry,bool): eventchanged, only real selection/time/party changes in that order, resulting revision/terms, boolfalse for no-op even merely reversed equivalent pair. Caller supplies canonical stored sets in declared order; equality is set-based and emitted complete values retain supplied canonical order. Table change uses table_id iff both before and after are singleton; otherwise table_ids full lists. Terms-only differences do not make an ordinary diner amendment.
- Cancelled(after Snapshot, seq int, at string) Entry: eventcancelled, Changes allocated empty[], resulting revision/terms.
- CloneEntries([]Entry) []Entry: deep-copy terms and any array/map values in changes.
No service locks, state mutation, references, ids, permission/cutoff checking or system-time reading. Service assigns revisions and calls Next/constructors only after a real successful transaction. Replay never re-enters callbacks; atomic failed clones publish no histories.

## Shared service transaction contracts for later integration

Keep the existing Service.mu/State/Idempotent transaction design, opaque ids via keel/id, immutable original receipt JSON and field names. Existing helpers do not take nested locks. Planned model extensions:
- Reservation.Revision int and AcceptedTerms policy.Terms; public current records always carry both, old receipt JSON is never regenerated.
- State.Policies map[restaurantID][]policy.Policy, Histories map[reference][]history.Entry, Series map[seriesID]Series and RestaurantRevisions map[restaurantID]int. Names/tags must remain consistent across clone/export/import.
- Restaurant.ManagerUserIDs []string default[], original detail configuration stays original including table capacities.
- Series stores ID, UserID, RestaurantID, Revision, IntervalWeeks and ordered members {Index,Reference,ScheduledDate,Exception}; scheduled dates survive diner edits and operator repairs. Public response omits owner/internal metadata and renders current ordinary records.
- Every successful new booking, real amendment, cancellation or policy publication updates restaurant revision once; a series adoption or real move batch updates it once for the whole operation. No-op/failure/preview/replay none. This counter is needed by the recurring requirement and later stage4; do not implement stage4 routes.
- Initialize old imported reservations revision1/fixture-policy0 and one created entry at the retained created_at, while preserving original fields/ends_at, tokens/hashes and all original receipts byte-for-byte. Legacy cancelled imports and cancelled reset seeds remain statuscancelled/revision1 with the same single reconstructed created entry: prior cancellation/edit timestamps are unavailable, so do not invent them. Modern histories and revisions are preserved verbatim. Subsequent observed cancellations of confirmed records create ordinary revision2 cancelled entries. Do not regenerate references or apply new booking business restrictions to reset/import producer-valid seeds.
- Real amendment order: own record; expected revision type/range and mismatch; confirmed status/old accepted cutoff; parse requested selection/time/party, determine canonical no-op; for real changes validate every resulting field using resulting date policy; final occupancy; atomically commit terms/end/revision/history/series exception and counters. No-op skips adoption of newer terms and revalidation of unchanged old business fields, but still enforces editable status/cutoff and valid provided input.
- The S2-R1 nonblocking off-grid-seed note is resolved for stage3 by the explicit no-op contract above: valid unchanged fields retain old terms/end; a real change validates all merged fields under the effective policy. Invalid provided fields still fail. Normal old accepted cutoff applies even to no-op.
- Series adoption generation uses original anchor local calendar dates, not fixed168-hour UTC increments. Construct all members/terms/history in a clone and consider other generated/fixed bookings before publication; no counter/receipt/ids visible on failure. Occurrence0 is exactly the anchor, no new history or revision. Adopted membership cannot alter ordinary response shape beyond revision/terms already specified.
- Individual real edits mark permanent exceptions; cancellation changes series counter but does not create exception. Per-series counter increments once per transaction even when several members move. No-op generation/adoption behavior is not invented: count2..12 always generates members and successful adoption is real.

## Explicit S3-G fixture transport (tests only)

Use the existing injectable Transport seam; never implement backend stand-ins or fixtures in production.
Original detail: r_anker / Zum Anker / Europe/Berlin, slot30 duration90 cutoff120, thu18:00–23:00, tables t_1(label1,cap2), t_2(label2,cap4), t_3(label3,cap4), combinable[[t_1,t_2],[t_2,t_3]]. Original detail remains this shape.
Selected published policy version1 effective2027-06-17: slot60 duration60 cutoff60, thu18:00–23:00, capacities{t_1:4,t_2:6,t_3:4}. Availability returned slots18:00,19:00,20:00,21:00,22:00. For party8: singles ids=[]; pair options capacities10 and10 in declared order. All cell availability comes from response; no client exclusion by original capacities. For party4: available_table_ids[t_1,t_2,t_3], singles option capacities4/6/4 then pairs10/10.
A pair response has reservation_id res_pair, reference POLICY01, restaurant_id r_anker, table_ids[t_1,t_2], party_size8, statusconfirmed, starts_at_local2027-06-17T18:00, starts_at2027-06-17T18:00:00+02:00, ends_at2027-06-17T19:00:00+02:00, created_at2026-10-05T05:00:00+00:00, revision1 and accepted_terms exactly the above policy without effective_from. Replay200 returns identical JSON, even if current fixture transport lookup later reports revision2 with different terms. Legacy response uses existing original ten fields/table_id only and has no revision/terms/arrays.
Ordinary browsing need not add explain=true; do not add new manager/history/adoption screens. Keep known pending body/key and localStorage session identity; current fields may be parsed as metadata without displaying implementation detail. Refusal409 and dropped-commit uncertainty keep their existing distinction and form. The original selected pair CSS must retain computed contrast >=4.5 and outcome reveal must make actionable states visible.

## Stage acceptance and immutable folders

Accepted stage1 delivery38d5fd6deb3a4f4e4474a02ebb153edb24a6d69c, tree8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c.
Accepted stage2 delivery0010e4c44eb9114dd42e59159f05ceeffca2ba44, reviewed8812cdeaaa993cd944493c654e51d355cdd6b676, tree9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f.
Exact stage3 copy committed0f26b0c110de132eea88f00a3f6502b9c4fc6596 before any extension. Both earlier trees stay byte-identical; no nested .git was present.

Exact reviewer command (stage3 candidate only after all items):
cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 3 --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S3-R1/checks
Expected stages1–3 pass, claimedstage3/highest3, stage4 fails. Own image/resource/offline, original legacy and modern series migration, unsampled errors/replays/concurrency/DST and full existing-screen design/viewport/contrast remain independently required. No reviewer verdict is inherited from stage2.

## Bookkeeping and private progress fallback

Coordinator accepts stage2/archive/copy complete; stage3 planning/handoffs in progress; stage3 implementation/review and stage4 pending. Native private task creation was blocked by approval_policy=never earlier; this PLAN and RUNLOG are a lossy private fallback without native task ids/dependency history. Shared cards are only actual seat assignments. Every handoff/report/verdict/retry/blocker records UTC, evidence paths and visible elapsed/usage; invisible tokens/costs are marked unavailable. No human input or approval is sought.
