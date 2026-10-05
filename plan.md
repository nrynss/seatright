# Tablekeeper stage 4: parallel foundations and compatibility

Stage 3 is accepted at **e13272d90213be0914211ae6f6f0bfd5888900ff**, formal round 1. Exact isolated results: 120/120 + 25/25 + 7/7, highest/claimed stage 3, expected stage-4 failure. Full source race passed with a 40-minute timeout; UI 90/90; independent correctness, transfer, both true upgrades and design passed. The reviewer reported no critical design findings. Committed review archive: evidence/stage-3/; original private artifacts remain outside Git.

Stage 4 starts from the untouched accepted stage-3 tree, copied in **4f52394728aaf2f15dc9afcd7081e3c2a489eba1**. Accepted stages 1–3 remain immutable. Stage 4 is not accepted.

## Architecture

```arch
{
  "kind": "layered",
  "title": "Tablekeeper: atomic seating repair and recurring clock amendments",
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
          "label": "Recurring agreements, exceptions and atomic clock amendments"
        },
        {
          "id": "clock_engine",
          "label": "IANA wall times and half-open intervals"
        },
        {
          "id": "planner_engine",
          "label": "Pure global lexicographic seating optimizer",
          "detail": "Own accepted capacities; fixed occupancy; closures; deterministic reference-ranked options"
        },
        {
          "id": "replan_engine",
          "label": "Manager preview and atomic apply",
          "detail": "Revision-bound plans; immutable replay; one restaurant counter; moved member history"
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
        },
        {
          "id": "closure_store",
          "label": "Applied half-open table closures"
        },
        {
          "id": "replan_store",
          "label": "Detached preview plans and application status"
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
    },
    {
      "from": "replan_engine",
      "to": "planner_engine",
      "label": "Pure preview from immutable booking inputs"
    },
    {
      "from": "replan_engine",
      "to": "closure_store",
      "label": "Atomic closure plus assignments"
    },
    {
      "from": "replan_engine",
      "to": "replan_store",
      "label": "Revision-bound preview and applied marker"
    },
    {
      "from": "replan_engine",
      "to": "history_engine",
      "label": "One reassigned entry per moved booking"
    },
    {
      "from": "replan_engine",
      "to": "series_engine",
      "label": "Preserve exception flags; increment affected series once"
    },
    {
      "from": "reservation_engine",
      "to": "closure_store",
      "label": "Creates, amendments and availability exclude closed members"
    }
  ]
}
```

## Current work

S4-P/shared51 and S4-M/shared52 are accepted and integrated after the final live-object proof repairs. Solver author commit: 1e1a381e80ad0d4554184843aa1cb1115f234497. Foundation author commit: 019a27b0fe5280a6a6ee66acd06666044860a685. Host scoped races, formatting, vet, full normal tests and builds passed. The repairs changed tests only; no product defect was found.

OpenCode/shared54 owns manager seating previews and their exact HTTP route; OMP/shared55 owns recurring clock amendments in two new service files only. Both are in progress from verified base **727eae17559928196cc7f3f5c7dc8967b0d7088e**. Complete single-message handoffs were accepted: preview 5ba35ade-763f-47c8-9313-329c61c20796; amendments a006af16-14f4-4148-809b-145869cd5ce7. Router integration for amendments remains serial, so the owners do not overlap. Grok/shared53 continues fixture compatibility independently. Stage 4 remains unaccepted.

Historical foundation handoffs at bda7c43023123bd8c96809d2c2f8066f1f81bdae: S4-P 58aa1df7-5be2-42aa-93f4-077f0c133e23; S4-M 72120fff-a6c8-4fd6-b5a6-53a51307b228; S4-G 7e47c02b-4aea-447e-8dd7-555ba8f80ea4. P and M are completed; G is active. Final endpoint handoff bases are recorded after their metadata commits exist.


## Active endpoint contracts

## ACTIONABLE S4-R1 — manager seating-preview integration (OpenCode)
Role: backend implementer. Mandate: /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md.
Worktree: /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode; assigned full base/card follows below. Own ONLY stage-4/internal/service/replans.go (NEW), replans_test.go (NEW), http.go and http_test.go (append tests, retain all inherited tests). Do NOT edit planner/history engines, model/state/replan_state, validators/control/reset, reservations/availability/moves/series/version_writes, web/Docker/probes/docs/earlier stages/Git. Components: replan_engine, planner_engine, replan_store, receipt_store.
Dependencies now REAL: S4-P solver and S4-M foundation integrated. No invented stub. Implement preview ONLY; apply/closure write enforcement belongs fresh S4-R2 later, do not add an unimplemented apply route now.

Frozen service API: func (s *Service) PreviewReplan(token, restaurantID, key string, raw []byte) Result; callback func previewReplanLocked(st *State, userID, restaurantID string, obj map[string]any) Result. Wrapper delegates existing Idempotent on POST /restaurants/{id}/replans with actual path scope; callback no locks/reentry. Existing wrapper checks auth/key/receipt-before-validation and commits only 201 on detached WORK. Callback resolves restaurant (404), manager membership using real Restaurant.ManagerUserIDs (403), then validates body. Unknown fields ignored, canonical body remains all fields for reuse. Required table_id string referring to real table; unknown table 404; from/to RFC3339 instants with explicit offset (Z represents explicit UTC), valid absolute from<to otherwise 422 validation_failed. Absent/wrong types/zone-less/unparseable interval must be 422 validation_failed as stage4 interval contract; malformed JSON retains inherited 400. Preserve valid supplied from/to text for stored/public closure; compare absolute parsed instants. Frozen Closure fields TableID,From,To strings.

Construct planner.Request from fixture TableIDs order and declared Combinable order, proposed closure, every previously applied closure in this restaurant (absolute parsed), all confirmed bookings at this restaurant overlapping [from,to) as Considered (including bookings not on closed table), every other confirmed same-restaurant booking as Fixed. Cancelled/other restaurants excluded. Booking.Reference/table IDs via reservationTableIDs, PartySize, parsed stored StartsAt/EndsAt (accepted absolute ends; no current-duration recomputation), OWN AcceptedTerms.Capacities copied/read-only; do not use new selected policy or stale fixture caps. Preserve full original booking state, no cutoff checks for operator repair. Source is produced valid state; invalid native state is I lane, never synthesize a repair bypass.
Use planner.Solve exactly. planning_limit→422 planning_limit (>6tables/>4pairs/>6considered; exact supported maxima), no_feasible_plan→409 no_feasible_plan. Any failure must leave whole state/export/counters/receipts unchanged (failed key reusable).
Allocate opaque plan id with keel/id only AFTER feasible solve; store Replan{ID,RestaurantID,RestaurantRevision:st.RestaurantRevisions[id],Closure,Assignments,MovedCount,UnusedSeats,Applied:false}, fresh slices, ALL considered reference-sorted. Store only Plans; no closure/booking/history/terms/series/restaurant counter changes. Successful preview does store its normal receipt. Return 201 replanPublic(stored) EXACT six-key shape; replay original200 bytes even after later booking/policy changes and after another preview. Caller mutation of returned map cannot alias plan. Empty considered → assignments[] and0 totals, still store plan with captured revision.

HTTP: extend existing restaurantRoute for EXACT POST /restaurants/{nonempty}/replans; no trailing slash/deeper/wrong-method aliases. Existing policies/detail/auth semantics unchanged. No series router edits. Future R2 alone adds /replans/{plan_id}/apply. Keep above signatures/types frozen for next owner.

Effective tests named TestReplanPreview*: 401/403/404/body422/malformed400, per-restaurant managers, token/key/path/user scoping+receipt-before-validation+failed reuse+original full byte replay; exact public keys/array/order/captured rev/state-only plan+receipt delta; empty and all-supported maxima/planning_limit; pure global-tier/true greedy worlds exercised through service; considered includes unrelated-table overlapping bookings, fixed outside closure interval still conflicts at overlapping booking tail, own per-booking accepted-capacities (different/currentpolicy/above maxima), cancelledignored, declared/nontransitive pairs, reversed-set unchanged/canonical pair, previous/proposed closures including any-member+absolute-offset+half-open adjacency, operator can repair past/old-cutoff booking. Failure assertions compare COMPLETE pre/post export; success assert complete no-change namespaces plus exactly one plan/receipt. Real 50-same-key (1×201/49×200 identical; exactly one plan/receipt/no counter), detachment both directions. HTTP exact routing matrix and live first201/replay200/fullJSON shape. Do not derive service results by invoking production solver in the expected-value test oracle.

Gates CWD stage-4: gofmt -l internal/service; go vet ./...; go test -count=1 ./...; go test -race -count=1 -v ./internal/service -run '^TestReplanPreview'; go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-R1/tablekeeper ./cmd/tablekeeper. If sandbox gcc absent run/retain actual CGO_ENABLED=1 failure then targeted nonrace concurrency; host coordinator verifies scoped race.
Root Docker: docker build -t tablekeeper:s4-r1 stage-4; own container tk-s4-r1 --cpus 2 --memory 2g -e PORT=9180 -p 9180:9180. Verify actual full image/container IDs/PORT, health200 exact+charset; live manager fixture/booking/preview201+replay200 immutable/currentcounter snapshot, forbidden403/invalid422/no-feasible409 unchanged. Only own container cleanup, no shared cache/image deletion. No browser/harness/import/apply claims.
All work must finish in one foreground turn (~20–30 minutes), with one DONE or concrete BLOCKED report. Do not wait/poll/sleep for another seat. You do not run any state-changing Git command. Coordinator owns all commits/resets/merges, PLAN.md, RUNLOG.md. Earlier stages 1–3 and all unowned files are immutable. Never weaken inherited tests, add fixture branches, manufacture receipts/outputs, or claim stage acceptance. Every raw log records actual argv, absolute CWD, UTC start/end, effective process exit, and raw output; preserve failed attempts separately. Private token/body/export artifacts 0700/0600; public stdout names/status/counts only, no secrets/tracebacks/xtrace. Report item/card/full Git base/worktree/owned paths, requirement→named effective tests, exact commands/evidence paths/exits/counts, honest gaps/deviations/failures/cleanup/measured elapsed/visible usage (unavailable if absent).

## ACTIONABLE S4-A — atomic recurring clock amendments (OMP)
Role: backend implementer. Mandate: /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md.
Worktree: /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp; assigned full base/card follows below. Own ONLY NEW stage-4/internal/service/series_amend.go and series_amend_test.go. No router/http, existing series.go/series_test, reservations/moves/version_writes/history/policy engines/model/state/reset/control/validator/web/Docker/probes/docs/earlier stages/Git edits. Components: series_engine, reservation_engine, history_engine, closure_store, receipt_store.
Dependencies now REAL: integrated M closureBlocks and actual stage3 prepareAmendment/commitReservationAmendment + seriesMembership/touchSeriesForChanges/seriesPublic and Idempotent. Router added serially S4-W later; service-direct acceptance now, not fake route or mock backend.

Frozen API func (s *Service) AmendSeries(token, seriesID, key string, raw []byte) Result; callback func amendSeriesLocked(st *State,userID,seriesID string,obj map[string]any) Result. Wrapper Idempotent POST /series/{seriesID}/amend actual path. Callback no locks/reentry; Idempotent clones/atomic commits only201 and stores normal receipt. Unknown/foreign series404, missing/invalid token401 via wrapper; do not use owner GET which hides no-token as404. Required expected_revision positive integral JSON-number, bool/string/null/fraction/nonpositive invalid422 validation_failed; HUGE positive integral must compare without int cast/ceiling (math.Trunc + compare float64(series.Revision)), mismatch409 stale_revision before any occurrence cutoff/booking validation. Required from_index strict integer0..len(Members)-1 boolinvalid; local_time EXACT5bytes HH:MM valid00:00..23:59 (digits not arbitraryformat), invalid422 validation_failed. Unknown fields ignored while original canonical body remains all fields for idempotency conflict. Validate top-level required types/ranges then revision guard before member checks; no invented upperlimit on expected revision.

Eligible members original occurrence-index order, index>=from_index, current record confirmed AND Exception=false. Cancelled/permanentexception ignored entirely. Resulting local string ORIGINAL member.ScheduledDate + T + local_time, preserve reference/reservation identity/user/party/current table selection (including previously repaired assignment). Do not use current record date to reconstruct agreement, zone date arithmetic, or fixed168hours. If resulting StartsAtLocal already equal and selection/party retained, it is no-op: retain ALL stored fields/accepted terms/end/revision/history; bypass newly published grid/capacity AND old cutoff for this identical operation (existing individual prepareAmendment checks cutoff before no-op, so handle actual series no-op BEFORE invoking it; do not edit shared helper). For every REAL change call actual prepareAmendment(st,before,{starts_at_local:resultLocal}) without per-booking expected_revision; it checks OLD accepted cutoff then fully merged booking against resulting date's policy incl grid/DST first-fold/gap/capacity. Pure candidates unchanged rev/no mutation. Original policy and state/history untouched until all nonoccupancy validation succeeds.
Prepare ALL eligible indices first; return first nonoccupancy error in occurrence-index order, even if earlier candidate would have occupancy conflict. Complete resulting occupancy then includes unchanged/cancelled excluded as usual, all eligible candidates against each other, other confirmed bookings, applied closures. To permit legitimate collective swaps, use detached WORK candidates for final occupancy and exclude self only, as current collective moves; no partial commit/history. Actual conflictingReservation currently checks bookings; ALSO explicitly closureBlocks(st,restaurantID,ids,start,end) before success. Any failure409 table_unavailable or earlier returned validation/cutoff code discards whole callback work, whole-export byte-identical, no key claimed.
Only after all validation+occupancy passes, commit real changes against ORIGINAL befores with commitReservationAmendment(...,time.Now()) (one ordinary ordered Changed/rev+1 each); noops untouched. If changedrefs nonempty call touchSeriesForChanges(st,changedrefs,false) exactly once to bump this series once, preserve ALL exception flags/scheduled dates, no new exceptions; increment restaurant counter exactly once. Emptyeligible/allnoop201 current seriesPublic with zero metadata/counter revisions. Return complete current series map (normal existing shape, index order) ensuring original receipt replay200 remains byte-identical after later changes/cancel even when current differs. Idempotency/concurrent expected-series guard runs under existing mutex so at most one real amendment wins (other stale409); all-noop may bothsucceed appropriately.

Effective tests TestSeriesAmend*: bodymatrix+huge-stale-before-cutoff/invalid member data; auth owner/notoken/methodpath userscope key guards; partial suffix/anchor/fromindex boundaries/count12/exceptioncancelled skips; original scheduled dates versus real current repairedtable, scalar/pairs selectedownpolicy capacity; real adopted policy by per-date, latestsame-date, accepted oldcutoff bothdirections; no-op offgrid/past/changed-policy retains FULL record/terms/history despite cutoff; emptyeligible zero-metadata; DST Berlin springgap/NYfirstfold/Santiago valid local date absolute duration; independent earlier occupancy + later nonoccupancy failure demonstrates nonoccup precedence, two distinct nonoccup errors pin indexorder; unchanged-member/external/eligible-mutual/applied-anymemberclosure absoluteoffset/halfopen conflicts; whole-export rollback and SAMEfailedkey genuine reuse201; real commit full previoushistoryprefix + one entry/bookrev+1, exact fullrestaurantcounter map +1/seriesonce/flagsretain/unrelated unchanged; replay AFTER actual edit/cancel original bytes + export unchanged; two concurrent sameexpected revisions one201/onestale409 zero loser delta; 50same-key1×201/49×200 one rev/counter; inheritedall tests unchanged; fulldetachment. Direct service calls are acceptance here; router integration explicitly deferred, do not claim API route.
Gates CWDstage4: gofmt -l internal/service/series_amend.go internal/service/series_amend_test.go; go vet ./...; go test -count=1 ./...; go test -race -count=1 -v ./internal/service -run '^TestSeriesAmend'; go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-omp/S4-A/tablekeeper ./cmd/tablekeeper. Retain actual sandbox raceerror if gcc absent and coordinatorhostrace; never invent pass. Optional owned binary health smoke PORT9184 exactcharset+body, stop own pid. No Docker/UI/harness/currentHTTP-amend claim required since serialrouterdependency intentional. Working actual service APIs sufficient, no handcrafted stub.
All work must finish in one foreground turn (~20–30 minutes), with one DONE or concrete BLOCKED report. Do not wait/poll/sleep for another seat. You do not run any state-changing Git command. Coordinator owns all commits/resets/merges, PLAN.md, RUNLOG.md. Earlier stages 1–3 and all unowned files are immutable. Never weaken inherited tests, add fixture branches, manufacture receipts/outputs, or claim stage acceptance. Every raw log records actual argv, absolute CWD, UTC start/end, effective process exit, and raw output; preserve failed attempts separately. Private token/body/export artifacts 0700/0600; public stdout names/status/counts only, no secrets/tracebacks/xtrace. Report item/card/full Git base/worktree/owned paths, requirement→named effective tests, exact commands/evidence paths/exits/counts, honest gaps/deviations/failures/cleanup/measured elapsed/visible usage (unavailable if absent).

## Work items and dependency queue

All work is restricted to stage-4. Stages 1–3 are immutable. Coordinator owns Git, PLAN.md, RUNLOG.md, integration and exact-SHA acceptance. Items finish in one foreground turn, with accurate raw evidence, then the coordinator verifies ownership and host gates before committing with the implementer's author and merging without fast-forward. A future item becomes actionable only through a fresh full handoff from its integrated dependency base.

| Item | Owner | Scope and dependency | Coverage | Acceptance |
|---|---|---|---|---|
| S4-P / shared51 | OpenCode | New internal/planner/**; independent pure engine | R311–326, R333 | API contract and unit/race oracle gates below |
| S4-M / shared52 | OMP | Value/state/history paths listed below; small foundation, independent | R328–329, R342, R346–348, R380–382 foundation only | Clone, serialization, half-open membership, Reassigned and inherited gates below |
| S4-G / shared53 | Grok | Existing web/** with working fixture Transport; independent | R110–160, R196–199, R305–306, R360, R376, R382 frontend compatibility | Built preview fixture and viewport/contrast/motion gates below |
| S4-R1 | OpenCode | After P+M: new replans.go and replan preview tests, exact HTTP route arm/tests; freeze helper interface before R2 | R307–333 | Real manager/idempotency/optimization/atomic preview and full inherited tests |
| S4-A | OMP | After M: new series_amend.go/tests only; pure preparation + real atomic state helpers; no concurrent router edits | R351–379 | Service-direct full input/cutoff/DST/policy/rollback/metadata/concurrency matrices, races |
| S4-R2 | OpenCode | After R1: apply implementation/tests plus narrow reservations.go conflict and availability.go explanation seams; preserve existing write helpers | R334–350, R377–379 | Real apply/closure/state/counter/series/replay/concurrency matrices, regression/races |
| S4-W | Assigned after R2+A | Serial narrow http.go/http_test.go series-amend dispatch and integration checks; no router ownership overlap | R351, R370 | HTTP auth/privacy/body/path/idempotency semantics plus final all-source gates |
| S4-I1 | OMP | After R2+A+W: native import validation extension and corrupt-state/producer tests, owned paths frozen at handoff | R380–382, R339, R342, R376 | Strict legitimate reassigned history/closure/plan/series acceptance, atomic corruption rejection, all-source races |
| S4-D | OpenCode | Separate genuine stages 1–3 donor producer; new stage4-donor probe only | R380–382 donor preparation | Inspected independent old processes, raw receipt bindings, sabotage guards, private evidence |
| S4-I2 | OMP | After I1 and D: new transfer tests + stage4-import probe only | R380–382, R377, R376, R339 | Genuine old sources → current and modern current → peer, full state/receipt/metadata/session agreement |
| S4-P1 | OpenCode | After integrated endpoints: new stage4-api probe only | R304–382 API surface | Independent exhaustive optimization examples, race-state deltas, guards and truthful counts |
| S4-G2 | Grok | After real integrated endpoints/import: web live probe only unless a demonstrated product defect | R305–306, R382 plus inherited browser/design | Real repair + recurring amendment, three true old-document upgrades, current assignment/clock + immutable retry, screenshots/videos |
| S4-P2 | OpenCode | Final guide/packaging plus supplied implementer smoke; split bounded items if needed | R6–16, R304, R380–382 | RUN.md source facts, default/override/offline image/assets, API, HTML, real transfer and honest harness result |
| S4-R | ZCode | Final clean frozen candidate; no modification | All R1–382 | Strict isolated stage4 checks, all source/race/UI gates, independent API/optimizer/transfer/browser/design; exact-SHA ACCEPT |

S4-R1 and S4-A use actual foundation seams from M. S4-R2 is serial with R1. Only S4-W owns the final series router arm; do not schedule two router writers simultaneously. Any changed interface is settled by the coordinator before the next full handoff. Imports do not loosen consistency to excuse an uncommitted history mutation.

Review requires the unchanged supplied command:
```sh
cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 4 --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S4-R1/checks
```
All suites 1–4 must pass and claim stage 4; there is no stage-5 probe. Reviewer checks the exact frozen SHA, independent global optimum and state transitions, genuine transfer from all three older services, full offline behavior, current UI and original receipts, same-document upgrade, viewport truth, motion/contrast/accessibility and design critique. Maximum three formal rounds. No stage-4 acceptance before that report and evidence corroborate.

## Initial frozen interface contracts
## S4-P — pure seating optimizer, OpenCode/shared51
Own ONLY stage-4/internal/planner/** (new). Worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. No service/model/http/history/web/Docker/probes edits. Components: planner_engine, clock_engine

Implement pure package planner, no service import/locks/ids/clock read/state mutation. Frozen Go API: `type Booking struct {Reference string; TableIDs []string; PartySize int; StartsAt, EndsAt time.Time; Capacities map[string]int}`; `type Closure struct {TableID string; From, To time.Time}`; `type Request struct {TableIDs []string; Pairs [][]string; Considered, Fixed []Booking; Closures []Closure; Proposed Closure}`; `type Assignment struct {Reference string; TableIDs []string; Changed bool}`; `type Plan struct {Assignments []Assignment; MovedCount, UnusedSeats int}`; `type Error struct {Code string}` (Error() string); `func Solve(req Request) (Plan,*Error)`. Inputs are parsed caller-supplied instants/valid table ids; return planning_limit for >6tables/>4pairs/>6considered (never artificially lower), no_feasible_plan when no assignment. Preserve input arrays/maps and nil-vs-empty without aliasing; empty considered yields allocated assignments[] withzero totals if withinlimits. No service stubs needed.

Options rank singles in Request.TableIDs fixtureorder, then declared Request.Pairs order, rankstarts0. Capacity per considered booking is SUM of its OWN Capacities members (acceptfixtureterms above publication maxima, never use latestpolicy/fixturecapacity). Eachoption uses canonical declared pairorder; no transitivity. Fixed input containsconfirmed nonconsidered bookings atsame restaurant and blocks byhalf-open intervals + member intersection. Priorclosures/proposedclosure block members only for overlappingbookinginterval; closed single and every containingpair excluded. Considered assignments mutually conflict bysame rules, including portionsoutsideclosure window. Globally minimize tuple(number tableSETS changed, totalcapacity-party acrossALLconsidered, rankvector in ascendingreferenceorder). Rank tiesdeterministic, changedtable equalityset-based; sort cloned consideredreferences, preserve input and all values. Cutoffs irrelevant. Prune/search withinrequiredlimits efficiently; objective must be exhaustive/demonstrably correct, no greedy shortcut.

Named meaningful tests: empty/boundaries/maxlimits; genuine infeasible; half-open adjacency/fullbooking versus shortclosure; fixedpair/sharedmember/previousclosure/proposedmember; mixed acceptedcapacities/above-max/fixedcapacitynotlatest; each objective tier isolated incl uniquelex tie; single/pair changing/unmoving/setequality/nonglobalgreedy trap; declaredpairsnontransitive; shuffledreferencesdeterminism; deep snapshot isolation. Useful independent brute-force oracle on several smallgeneric worlds if implementationsearchprunes, do not merely mirrorimplementation. Full serviceexisting suite unmodified.
Gates CWDstage4: gofmt -l internal/planner; go vet ./internal/planner; go test -race -count=1 -v ./internal/planner; go test -count=1 ./...; go build ./cmd/tablekeeper. Actual missinggcc raceerror recorded, coordinatorhostrace. No Docker/API/UI/harness requiredpurepackage; no endpoint/closure/persistence claims. QueueafteracceptedP+M: S4-R servicepreview/apply+closureoccupancy integration fromnewbase, separate assignment.
All work is foreground, one DONE or concrete BLOCKED report in roughly20–30min. No Git mutations, earlier stages, PLAN/RUNLOG or unowned files. Mandate is result/mandates/seatright-<seat>.md; fixed worktrees below, coordinator alone resets/commits/merges. Never weaken inherited tests or use production fixture branches; no stage4 acceptance claim. Evidence per-seat/item: exact runnable argv/absoluteCWD/UTCstartend/effectiveexit/rawoutputs; preserve failed attempts separately, private0700/0600, no credential/token/body/export stdout. Unit gates scopeappropriate and full nonrace regression; host coordinator verifies scoped races if sandbox lacks gcc, record actual failure rather than invented pass. Final report item/card/fullGitbase/worktree/ownedpaths/requirements→namedtests/commands+rawpaths+exits/counts/gaps/failures/cleanup/measuredelapsed/visibleusage (unavailable if absent). Do not wait/poll/sleep for peers or send acknowledgments. Questions about code/interfaces go to coordinator in the completed report; continue independent scope.

## S4-M — small shared value/state/history foundation, OMP/shared52
Own ONLY stage-4/internal/service/{model.go,state.go,version_state.go,replan_state.go(new),replan_state_test.go(new)} and stage-4/internal/history/{history.go,reassigned_test.go(new)}. Worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md. No control/reset/validator/router/reservations/availability/moves/series/core/otherexistingtests/web/Docker/probe edits. If inherited test seam cannot pass withinownedfoundation, reportprecise seam; do not broaden/delete/weakentests. Components: snapshot_store, closure_store, replan_store, history_engine

Frozen stored values in service: `type Closure struct {TableID string json:"table_id"; From string json:"from"; To string json:"to"}`; `type ReplanAssignment struct {Reference string json:"reference"; TableIDs []string json:"table_ids"; Changed bool json:"changed"}`; `type Replan struct {ID string json:"plan_id"; RestaurantID string json:"restaurant_id"; RestaurantRevision int json:"restaurant_revision"; Closure Closure json:"closure"; Assignments []ReplanAssignment json:"assignments"; MovedCount int json:"moved_count"; UnusedSeats int json:"unused_seats"; Applied bool json:"applied"}`. State extends with `Plans map[string]Replan json:"plans"` (keyplanID), `Closures map[string][]Closure json:"closures"` (restaurant-keyed), no duplicatedreservation/seriesindex. New/empty/resetviaexistingemptyState allocate maps; imported earlier absentfields normalize empty via `normalizeReplanState(st *State)` invoked by existing normalizeVersionState (tinyseam only), modern allocated/nonallocated JSONshapes remain honest. Deep clone map/assignment.TableIDs/closures/maps with nil-vs-empty fidelity. No plans/closures fixture fields invented on RESET; unknown fields still ignored. No endpointoperations yet.

Pure frozen helper `closureBlocks(st *State,restaurantID string,ids []string,start,end time.Time) bool`: parsed storedRFC3339explicitoffset closureintervals, tableintersection andhalfopenoverlap; otherrestaurants harmless, adjacencyfree, anypairmemberblocks; read-only/no counter/id/clock mutation. Invalidstoredclosure validation islaterI scope, do not silentlyintroduce businessrules/nativevalidatorloosening. `cloneReplan(p Replan) Replan` detachedtable slices. `replanPublic(p Replan) map[string]any` emits EXACTpreviewshape plan_id,restaurant_revision,closure{table_id/from/to}, assignmentsallocated[] eachreference/table_ids/changed, moved_count,unused_seats; excludeRestaurantID/Applied frompublic. Use maps toalign201/replayJSONmarshalorder laterwrapper; detachedvalues.

History frozenaddition `Entry.PlanID string json:"plan_id,omitempty"`, `EventReassigned="reassigned"`, `func Reassigned(before,after Snapshot,seq int,at,planID string) (Entry,bool)`. SameunorderedtableSET returnsnoentryfalse. Realsetchange emitsoneeventreassigned withEXACTLY oneChanges fieldtable_ids, fullcanonical arraysFrom/To even singleton→singleton, plan_id; after.Revision/frozenacceptedTerms; no time/party changes, no modifyinginputs. Existing Created/Changed/Cancelled outputsunchanged, Next clamp remains. CloneEntriesalreadyfieldcopiesPlanID. No validatorchanges here; S4-I laterpermitsstrict legitimate reassignedchains.

Namedtests snapshot/map/arrayisolation, nil-vs-emptyserialization, fullpreviewpublicshape/privacy/detachment, emptyoldimport/newstate mapswithoutdroppingoldreceipts, closureshalfopen/offsetabsolute/crossrestaurant/member, Reassignedsingle/pair/unchanged/canonicalarrays/planid/termsfrozen/order/seq, inheritedhistoryoutputsunchanged. Gates CWDstage4: gofmt -l internal/service internal/history; go vet ./...; go test -count=1 ./...; go test -race -count=1 -v ./internal/service -run '^TestReplanState|^TestClosureBlocks'; go test -race -count=1 -v ./internal/history -run '^TestReassigned'; go build ./cmd/tablekeeper. No Docker/UI/harness neededvaluefoundation. QueueafteracceptedM: S4-A atomicseriesamend ONLY new series_amend.go/tests; router integration is serially assigned after endpoint owners finish, fromnewintegratedbase; existingpreparedAmendment/commithelpersreal now, closureBlocksavailablethen.
All work is foreground, one DONE or concrete BLOCKED report in roughly20–30min. No Git mutations, earlier stages, PLAN/RUNLOG or unowned files. Mandate is result/mandates/seatright-<seat>.md; fixed worktrees below, coordinator alone resets/commits/merges. Never weaken inherited tests or use production fixture branches; no stage4 acceptance claim. Evidence per-seat/item: exact runnable argv/absoluteCWD/UTCstartend/effectiveexit/rawoutputs; preserve failed attempts separately, private0700/0600, no credential/token/body/export stdout. Unit gates scopeappropriate and full nonrace regression; host coordinator verifies scoped races if sandbox lacks gcc, record actual failure rather than invented pass. Final report item/card/fullGitbase/worktree/ownedpaths/requirements→namedtests/commands+rawpaths+exits/counts/gaps/failures/cleanup/measuredelapsed/visibleusage (unavailable if absent). Do not wait/poll/sleep for peers or send acknowledgments. Questions about code/interfaces go to coordinator in the completed report; continue independent scope.

## S4-G — existing-screen repair/series compatibility, Grok/shared53
Own ONLY stage-4/web/**. Worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-grok.md. No Go/service/router/Docker/PLAN/earlierstages/Git. Components: restaurant_ui

Startnow independently with EXISTING WORKING Transport teststub/interface, no missingbackend dependency/no productionfixturefallback. No managerreplan orseriesamend screens required. Existing availability/confirmation/lookup must render returnedcurrentassignments/times/unchangedacceptedterms, humanlabels/floor/authoritativepairs, while old receipts/pendingkey/body immutable. Keep everytestid/labels/stateword/displayreferenceonly, immediateattributes/motion/stagger<=480/reduced/focus/contrast/appreveal/signedinblank exact. foundation banonreplan word ifpresent isobsoleteforstage4; narrow removal only, retainoffline/fetch/Keeladapter/auth assertions. Producteditonlyiffixture demonstratescompatibilitydefect; tests/probe maybesufficient.

Frozen concrete fixture usingexisting `(path:string,init?:TransportRequest)=>Promise<unknown>` fromsrc/lib/transport.ts: TransportRequest {method?,token?,body?,idempotencyKey?,signal?}; routes /restaurants, /restaurants/r_anker, /availability?restaurant_id=r_anker&date=2027-06-17&party_size=6 plusauth/login/signup and/reservations POST/lookup/cancel existing shapes. Detail originalslot30/duration90/caps t_1=2,t_2=4,t_3=6, combinable[[t_1,t_2],[t_2,t_3]], labels1/2/3 orlonghumanlabels. Selectedpolicyoptionalcapacityappropriate, allpublicGETno bearer.
Repair fixture currentrecordreferenceREPAIR01/userprivateomitted/rid r_anker/party6/statusconfirmed/startlocal2027-06-17T19:00/start2027-06-17T19:00:00+02:00/end20:30+02:00/creatednumericRFC3339/rev1/table_ids[t_1,t_2], no table_id/acceptedterms fixture0fullsixkeys slot30/duration90/cutoff120/openingthu18-23/caps2,4,6. BeforeoriginalHTTP201 receiptretained. AFTERAPPLIEDREPAIR GETsameidentity/reference/party/start/end/terms/status, rev2,table_ids[t_3],table_idt_3. Availabilityat19:00 closuret_2 blocks t_2/everydeclaredpaircontainingt_2, t_3takenbybooking, t_1freebutparty6unavailable; adjacentpostclosure slots exposecorrectauthoritativeoptions. UIdoesnotcalculateclosure. LookupnamesTable3 ratheroldpair; oldsamekey/body bookingreplayreturnsoriginalpairreceiptwith200, notcurrentlookup. Recoveruncertaincommittedbody similarlysamekey/body, neverreplaceit withappliedassignment. Fixturetestdrivercan mutatefixture worldbetweenGETs; do not makeproduct callnewoperatorroute.
Recurring fixture samebookingref withoriginalscheduled2027-06-24 date/currenttableunchanged; servercurrentGETrealamendedclock20:00/end21:00 (selectedtermsduration60/revisionnew), fullservertermsreceived; oldconfirmationreceipt remainsoldJSON/acceptedterms/time and unchangedretryidentity. Cancelled andpermanentexception records showserverreturnedclock unchanged. Tests don'tinventclientdatepolicy/seriesgeneration.

Tests passmeaningful before/afterlookup/summary/floorgrid hooks/availabilityrefresh/legacycurrent-versus-receipt separation andsamebodykey/unavailableinert/409form preservation/uncertainrecovery/latestsearchblankname/reducedmotion. Capturebuiltpreviewfixturetransport at375/1280 light/dark inselected/uncertain/confirmed/lookupbefore-after, no pageoverflow/wordplatebounds/paircontrastcomputed>=4.5, ownapprevealviewport, screenshotinspection+keyboard/reducedvideo. Label everyartifact FIXTURETRANSPORT, no liveoperator/harness/migration/acceptance claim. Gates CWDstage4/web npmci/check/test/build; node --check scripts/s4-g-fixture.mjs(new); actualfixtureprobe exit0. No fullGo/Docker gates. QueuelaterS4-G2: realimagesliveappliedrepair+seriesclock/browserupgrade allthreesources afterbackendlands.
All work is foreground, one DONE or concrete BLOCKED report in roughly20–30min. No Git mutations, earlier stages, PLAN/RUNLOG or unowned files. Mandate is result/mandates/seatright-<seat>.md; fixed worktrees below, coordinator alone resets/commits/merges. Never weaken inherited tests or use production fixture branches; no stage4 acceptance claim. Evidence per-seat/item: exact runnable argv/absoluteCWD/UTCstartend/effectiveexit/rawoutputs; preserve failed attempts separately, private0700/0600, no credential/token/body/export stdout. Unit gates scopeappropriate and full nonrace regression; host coordinator verifies scoped races if sandbox lacks gcc, record actual failure rather than invented pass. Final report item/card/fullGitbase/worktree/ownedpaths/requirements→namedtests/commands+rawpaths+exits/counts/gaps/failures/cleanup/measuredelapsed/visibleusage (unavailable if absent). Do not wait/poll/sleep for peers or send acknowledgments. Questions about code/interfaces go to coordinator in the completed report; continue independent scope.



## Complete requirements ledger

S means supplied partial smoke coverage; U requires independent checks. Earlier rows are carried forward verbatim. New rows do not imply stage acceptance.

| ID | Source | Supplied check | Single testable requirement |
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
| R304 | stage-4 §Inheritance | U | All earlier-stage requirements continue to apply. |
| R305 | stage-4 §Seating changes | U | No new screens are required for seating repair. |
| R306 | stage-4 §Seating changes | U | Existing availability, confirmation and lookup screens reflect an applied plan. |
| R307 | stage-4 §Seating changes | U | Manager-only POST /restaurants/{id}/replans uses real idempotency with the specified closure body. |
| R308 | stage-4 §Seating changes | U | Closure from and to are instants with explicit offsets. |
| R309 | stage-4 §Seating changes | U | Invalid from/to or from >= to returns 422 validation_failed. |
| R310 | stage-4 §Seating changes | U | An unknown closure table returns 404. |
| R311 | stage-4 §Seating changes | U | A proposed closure occupies the half-open interval [from,to). |
| R312 | stage-4 §Seating changes | U | Planning considers every confirmed booking at the restaurant overlapping the closure. |
| R313 | stage-4 §Seating changes | U | Other bookings keep their assignments and remain fixed occupancy. |
| R314 | stage-4 §Seating changes | U | Planning supports up to 6 tables, 4 declared pairs and 6 considered bookings; larger inputs may return 422 planning_limit. |
| R315 | stage-4 §Seating changes | U | Every considered booking retains its reference, owner, party size, start, end and accepted terms. |
| R316 | stage-4 §Seating changes | U | Each assignment is a singleton or declared pair with capacity under that booking's own accepted terms. |
| R317 | stage-4 §Seating changes | U | Assignments avoid overlapping fixed bookings on any member table. |
| R318 | stage-4 §Seating changes | U | Assignments avoid conflicts with other considered assignments. |
| R319 | stage-4 §Seating changes | U | Assignments avoid previously applied closures. |
| R320 | stage-4 §Seating changes | U | Assignments avoid the proposed closure. |
| R321 | stage-4 §Seating changes | U | Cancellation cutoffs do not prevent an operator repair. |
| R322 | stage-4 §Seating changes | U | Planning and application never remove or cancel a booking. |
| R323 | stage-4 §Seating changes | U | The first optimization objective minimizes the number of changed table sets. |
| R324 | stage-4 §Seating changes | U | The second optimization objective minimizes total unused capacity over all considered bookings. |
| R325 | stage-4 §Seating changes | U | The third optimization objective minimizes the option-rank vector in ascending reservation-reference order. |
| R326 | stage-4 §Seating changes | U | Option ranks start at zero, singles in fixture order followed by pairs in declared order. |
| R327 | stage-4 §Seating changes | S | Preview returns 201 with opaque plan_id, restaurant_revision, closure, assignments, moved_count and unused_seats. |
| R328 | stage-4 §Seating changes | S | Preview assignments contain all considered bookings in reference order with table_ids and changed, including an empty array. |
| R329 | stage-4 §Seating changes | U | Restaurant revisions start at zero after reset. |
| R330 | stage-4 §Seating changes | U | Each successful new booking, real amendment, cancellation, publication or whole plan application increments the restaurant revision once. |
| R331 | stage-4 §Seating changes | U | No-op writes, failed writes, previews and replays do not increment restaurant revisions. |
| R332 | stage-4 §Seating changes | U | Preview stores only a plan and leaves closures, occupancy, booking revisions and histories unchanged. |
| R333 | stage-4 §Seating changes | U | Infeasible planning returns 409 no_feasible_plan with state unchanged. |
| R334 | stage-4 §Seating changes | U | Manager-only POST /restaurants/{id}/replans/{plan_id}/apply is idempotent with body {}. |
| R335 | stage-4 §Seating changes | U | Successful apply returns 201 with plan_id, new restaurant_revision and every considered reservation sorted by reference. |
| R336 | stage-4 §Seating changes | U | Unknown plans and plans belonging to a different restaurant return 404. |
| R337 | stage-4 §Seating changes | U | Any intervening revision of the same restaurant makes a plan stale: 409 stale_plan, atomically unchanged. |
| R338 | stage-4 §Seating changes | U | An already applied plan submitted with a different key returns 409 plan_already_applied. |
| R339 | stage-4 §Seating changes | U | A successful apply-key replay returns the original bytes with 200 after later changes. |
| R340 | stage-4 §Seating changes | U | Application atomically records the closure and all assignments together. |
| R341 | stage-4 §Seating changes | U | Each moved booking increments its reservation revision once. |
| R342 | stage-4 §Seating changes | U | Each moved booking gains one reassigned history entry with a complete table_ids change and plan_id. |
| R343 | stage-4 §Seating changes | U | Repair preserves accepted terms and booking times exactly. |
| R344 | stage-4 §Seating changes | U | Unmoved bookings gain no new revision or history. |
| R345 | stage-4 §Seating changes | U | The restaurant revision increments once for the entire applied plan, including a closure that moves zero bookings. |
| R346 | stage-4 §Seating changes | U | Applied closures exclude containing singles and pairs from availability. |
| R347 | stage-4 §Seating changes | U | Applied closures reject conflicting creates and amendments with 409 table_unavailable. |
| R348 | stage-4 §Seating changes | U | Explanations report no_overlap false for closure conflicts. |
| R349 | stage-4 §Seating changes | U | Concurrent applications cannot leave partially moved bookings. |
| R350 | stage-4 §Seating changes | U | A closure at another restaurant does not invalidate the plan. |
| R351 | stage-4 §Recurring amendments | U | Owner-only POST /series/{series_id}/amend is idempotent; unknown/foreign series return 404 and missing token returns 401. |
| R352 | stage-4 §Recurring amendments | U | expected_revision must be a positive integer and Boolean values are invalid integers. |
| R353 | stage-4 §Recurring amendments | U | from_index must be an integer in 0..count-1; Boolean values are invalid. |
| R354 | stage-4 §Recurring amendments | U | local_time must be exactly HH:MM in 00:00..23:59. |
| R355 | stage-4 §Recurring amendments | U | Invalid amendment input returns 422 validation_failed. |
| R356 | stage-4 §Recurring amendments | U | Stale series revisions return 409 stale_revision before any occurrence cutoff or booking validation. |
| R357 | stage-4 §Recurring amendments | U | Unknown series-amend body fields are ignored. |
| R358 | stage-4 §Recurring amendments | U | Eligible occurrences are indices >= from_index excluding cancelled and exception members. |
| R359 | stage-4 §Recurring amendments | U | Clock amendments use each occurrence's original scheduled local date. |
| R360 | stage-4 §Recurring amendments | U | Clock amendments retain reference, owner, party size and current table selection. |
| R361 | stage-4 §Recurring amendments | U | Identical resulting fields are no-ops and retain their accepted terms. |
| R362 | stage-4 §Recurring amendments | U | Each real occurrence change checks its old accepted cancellation cutoff. |
| R363 | stage-4 §Recurring amendments | U | Each real occurrence change adopts policy terms for its resulting start date. |
| R364 | stage-4 §Recurring amendments | U | Final occupancy excludes conflicts with unchanged occurrences. |
| R365 | stage-4 §Recurring amendments | U | Final occupancy excludes conflicts with other bookings. |
| R366 | stage-4 §Recurring amendments | U | Final occupancy excludes conflicts with applied closures. |
| R367 | stage-4 §Recurring amendments | U | Failures preserve all histories, idempotency records and revisions atomically. |
| R368 | stage-4 §Recurring amendments | U | Non-occupancy errors take precedence in occurrence-index order. |
| R369 | stage-4 §Recurring amendments | U | If non-occupancy validation succeeds, an occupancy conflict returns table_unavailable. |
| R370 | stage-4 §Recurring amendments | S | A successful series amendment returns 201 with the current series response. |
| R371 | stage-4 §Recurring amendments | U | Each changed occurrence gains one ordinary changed history entry and one reservation revision. |
| R372 | stage-4 §Recurring amendments | U | The series revision increments once per amendment operation if anything changed. |
| R373 | stage-4 §Recurring amendments | U | The restaurant revision increments once per amendment operation if anything changed. |
| R374 | stage-4 §Recurring amendments | U | Series amendments do not mark new exceptions. |
| R375 | stage-4 §Recurring amendments | U | All-no-op and empty eligible sets succeed without revision changes. |
| R376 | stage-4 §Recurring amendments | U | A successful amendment replay returns its original response with 200 after edits or cancellations. |
| R377 | stage-4 §Recurring amendments | U | Repairs of series members preserve exception flags, scheduled dates, identities and accepted terms. |
| R378 | stage-4 §Recurring amendments | U | Each affected series increments its revision once per application if any member moved. |
| R379 | stage-4 §Recurring amendments | U | Concurrent amendments using the same expected revision cannot both make a real change. |
| R380 | stage-4 §Portability | U | Stage 4 accepts genuine exports produced by this team's stages 1, 2 and 3. |
| R381 | stage-4 §Portability | U | Stage-4 operations support imported series including moved and cancelled occurrences. |
| R382 | stage-4 §Portability | U | Earlier booking and series receipts, histories and retries remain valid. |
