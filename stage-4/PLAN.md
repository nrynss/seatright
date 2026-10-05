# Tablekeeper Stage 4 implementation plan



## S4-R0 readiness verified — shared57 completed

Reviewer report b53cbcb4-231f-4f24-a696-fc872cad007c and retained PREFLIGHT.md/preflight.log have been read. The separate reviewer daemon has 1.8G free on its 9.8G Docker volume, a present 1.93GB runner with the recorded 1.24GB layer, healthy interpreter/import/CLI and build-time egress. Expected peak writes0.8–1.1GB and warm-cache/runner content-equivalence are estimates, not executed guarantees; the changed runner ID is recorded. A cold ~1.25GB base pull could require safe owned reclamation first. No deletion/prune/reconfigure/lifecycle or harness run occurred or is authorized by this readiness acceptance.

The preflight saw context0191aee2 with coordinator RUNLOG.md dirty, correctly disclosed; it was not frozen and metadata has since been committed. Accepted stages1–3 trees remain immutable. Shared57 is completed for read-only readiness only. S4-R1 requires the final clean frozen integrated SHA and complete original-task/spec/ledger/design/source/migration/browser/strict-isolated handoff. S4-A/shared55 and S4-R2/shared56 remain inprogress with final narrow corrections; no stage4 acceptance. Reviewer measured raw probes13:33:17–13:33:40 UTC and reported overall window13:33:17–13:34:10 plus writing (~4min); usage unavailable. Evidence absolute paths: /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S4-R0/PREFLIGHT.md and preflight.log.



## S4-A r3 residual audit and final narrow test repair

Report 5872b4db is audited against the current two owned new files. Real terms adoption, genuine fold/gap, later non-occupancy precedence and exact receipt binding now work; host 21 amendment races pass. No-op cutoff claim is still false: from_index1 selects tomorrow+7days outside10080-minute cutoff. Race full-state comments still exceed assertions. Final test-only correction c934bcf6-39b6-4b09-ac2d-f320e6957e9b stays on existing dirty base727eae17559928196cc7f3f5c7dc8967b0d7088e. No commit/reset/acceptance; W and migration interactions remain deferred.

S4-A/shared55 r3 report audit — narrow final test repair, same dirty base. Role: backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md. Worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp; existing full base 727eae17559928196cc7f3f5c7dc8967b0d7088e. Do not reset or perform Git operations. Retain only the two owned new files stage-4/internal/service/series_amend.go and series_amend_test.go; edit the test file only unless a real product defect is reproduced. No router/existing test/validator/web/Docker/probe/stages1–3/PLAN/RUNLOG edits. The original frozen contract below stands.

The real carry, genuine NY fold, later non-occupancy precedence, response receipt ownership/path/key/status/raw binding and fresh GetSeries detachment are now present. Two precise residual proofs remain; finish only these instead of expanding scenarios.

P1: TestSeriesAmendNoOp still carries cutoff10080 on from_index1, which is tomorrow+7 days at19:30. That member is OUTSIDE the seven-day cutoff. The comment claiming inside-cutoff and the report's cutoff-inside assertion are false; no actual now-vs-start assertion exists. Use the existing REAL 19:00→19:30 adoption on from_index0 so the near-term anchor is included and adopts cutoff10080 under its old0 permission. Pin the anchor's adopted policy/cutoff/revision/clock, then prove now >= old accepted start minus10080 using the actual parsed UTC start. Publish the slot60 policy, issue identical19:30 from_index0, require201 plus complete state retention except exactly one correctly bound new receipt. Both eligible members may be identical no-ops; do not pretend the next-week member is itself cutoff-blocked. Include plans/closures in the no-op namespace comparisons (the current loop still omits them). TestHugeStale's final matching-revision cutoff failure should also compare export against its pre-call baseline; current stale failure identity already works.

P2: TestSeriesAmendConcurrency comment says FULL counter/history/current delta but code still never compares restaurant_revisions beyond scalar r1, never compares the old target history prefix or the appended Changed contents, and compares winner/current only four selected fields. Strengthen this existing race exactly: full counter map equality with only r1+1; complete unchanged stored target fields except clock/absolute start/end/accepted terms/revision allowed by the dated amendment; complete public projection == winner record with singleton scalar shape; history prefix byte-equal plus one exact Changed entry at expected seq/revision with old-clock From and winner-clock To, complete terms, valid timestamp; affected series unchanged except revision+1 and all flags/schedules pinned; exact winner canonical body local_time value (currently only key existence is asserted). Existing owner/method/path/winner-key/201/raw-response/new-receipt and loser absence assertions stay.

TestSeriesAmendSameKey50 now has useful exact receipt and prefix checks, but its full-map claim still permits extra counter keys, its history check omits From/seq/at/terms, public projection omits selectors/status/created_at, and it does not assert unrelated histories. Bind the corresponding existing assertions to the real201, full expected counter map, full Public projection with scalar rule, exact Changed metadata/From/To/terms and unrelated histories. Preserve prior receipts. A small test-only independent comparison helper is fine; do not derive expected output through production commit functions. Keep this bounded to the existing no-op/two race tests; no new scenario or claimed count needed.

Gates CWD /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp/stage-4:
gofmt -l internal/service/series_amend_test.go
go test -race -count=1 -v ./internal/service -run '^TestSeriesAmend(NoOp|HugeStaleBeforeCutoff|Concurrency|SameKey50)$'
go test -count=1 ./internal/service -run '^TestSeriesAmend'
Product unchanged: prior full suite/vet/build/smoke stand separately; no Docker/UI/harness rerun. Fresh S4-A/r4 logs, literal argv/absoluteCWD/UTC/effectiveEXIT/raw; retain r3 logs unchanged. Report ONE foreground DONE/BLOCKED with actual assertions/counts/evidence paths and no unproved full-map/cutoff claims, no acknowledgment-only turn. Measured command timestamps rather than a future estimated end time. No stage acceptance. W/router and R377–378/I portability remain deferred. Components: series_engine, reservation_engine, history_engine, closure_store.

## S4-R2 r2 residual audit and final owned proof repair

Report33aeb08a is audited against actual files and retained evidence. Host19 apply/closure scoped races and vet/format pass; original37/fullnormal/build remain green. Successful-state/boundary/write reuse checks improved, but race tests remain unchanged, matrix lackstrue/true, a forbidden previewtest was edited, and livehistory prints a marker without asserting. Final full same-base correction79b21874-60e4-4e45-8171-47e1bf9eaca7 requires actual owned concurrent/matrix/live proof and restores forbiddenfile. No commit/reset/acceptance.

## ACTIONABLE S4-R2/shared56 final narrow residual proof repair

Seatright-OpenCode backend implementer, mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. Continue SAME dirty worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode at unchanged full base267f71e74767764d84dbbad121680c09fd8be972. No reset/Git/new item. Product core is unchanged and looks correct. TESTS/EVIDENCE ONLY; preserve all effective r2 improvements. Shared56 not accepted or committed yet. OMP's A work is independent, don't touch it.

Coordinator audited report33aeb08a: complete success singleton delta, pair→pair case, nonempty zero move, service-level short-inner/adjacency/offset/crossrestaurant availability, per-operation closure rollback and failed move/adopt key reuse, real later-edit replay baseline, nested response isolation, current Docker run/inspect/caps/health/cleanup retained, are actual improvements. Host fmt/vet/19 apply+closure scoped races pass (34.741s); prior fullnormal/build green on unchanged product. Retain these rather than rewriting them. Current fullnormal raw r2 passes; actual claims below DO NOT match checked source.

A — scope restoration and actual apply demotion.
git status additionally shows MODIFIED stage-4/internal/service/replans_test.go, forbidden originalpreview file, not mentioned in report. Its diff strengthens PREVIEW demotion tests, while APPLY TestReplanApplyDemotion still lacks export baselines. Restore replans_test.go BYTE-IDENTICAL to base (read-only git show allowed; no Git state mutation), and put equivalent before-each-request complete export comparisons in your owned replan_apply_test.go APPLY demotion test. Keep existing apply originalbytes200/fresh403. Final status only seven owned paths: NEWreplan_apply.go/tests + narrowreservations/availability/availability_test/http/http_test. Don't silently expand ownership.

B — implement the F2 race repair that is currently absent.
At current replan_apply_test.go:1263 TestReplanApplyRace50 is literally still:
preRev scalar; run50; after rev marshal; if pre==post fail. This accepts +2 or +50, and NO applyreceipt assertion. Your report says fullmap+F1style comparisons, but they are absent. Capture complete export BEFORE burst; after1×201+49×200 identical assert exact allowed delta bound to actual201 using effective Success comparisons: FULL counter map onlyr_anker+1; winner stored public response exact/scalar iff singleton; each moved revision+1 and EXACT one new reassigned with priorprefix preserved/seq/rev/planid/fullFromTo/terms; unmoved/unrelated records/histories bytes equal; exactly one closure appended and one plan Applied (all other fields/entries preserved); exactly one new receipt owner/method/exactpath/key/body/status201/raworiginal, ALL prior receipt values and unrelated namespaces unchanged. Seed existing unrelated entries before baseline so checks aren't vacuous.
TestReplanApplyCompeting:1330 still runs SAME-plan differentkeys SEQUENTIALLY. Change to two actual simultaneous requests, assert1×201+1×409 plan_already_applied, exact once delta and no losing receipt. Existing different-plan concurrency still stores only status integers and allows ANY409. Capture fullResults, pin loser stale_plan, one plan Applied/winningreceipt, losing plan untouched/no receipt, complete once-only delta. Keep hand-computed expected assignments. You may factor an owned TEST-only assertion helper to reuse the actual independently expected state delta, never call production mutation helpers to derive expected values.

C — actual fourth rule combination and exact rules/shape.
TestReplanApplyAvailabilityExplain:1039 is unchanged from first report: false/false and true/false at19:00 plus false/true onemptydate; NO true/true asserted. It maps rule names, losing order; comments still say overlap-true when no_overlap false, and accepts null. Report's all4/order claims don't stand.
Add true/true assertion on a genuinely free slot at21:30 for party4 (t_3 free), while closure t_2 remains true/false; use party8 at closed t_2 to prove capacityfalse/no_overlapfalse from the CLOSURE independent of a booking. Verify exact fixture table order, rule capacity then no_overlap, policy_version, available iff conjunction, complete matching IDs/options, exact allocated [] where empty. Existing new ClosureAvailabilityService baseline and boundary test is effective (injects stored closure, no actual apply in that test); keep this honest distinction from actual apply tests. Correct obsolete terminology/unused q variable, don't claim all4 from duplicated true/false tables.

D — fill remaining precise series/replay binding gaps, keep working improvements.
CurrentSeries now populates unrelatedsid3/counter map/unmoved far member equality: good. It still doesn't compare affected series' full fields excluding revision (schedules/memberindices/anchors/flags), or every moved book/history prefix. Add these comparisons on captured pre/post; main Success already proves one-book event but series propagation must not drift schedule/flags on multiple moves. Every affected series +1 once, otherfields full-equal; all moved histories append reassigned (not Changed), untouched keys/memberrecords prefix preserved. No R377–378 amend interaction until W.
AlreadyApplied now proves currentdiff/originalreceipt/export replay and reuse equality: good. Keep it. No new product scenario for optional past-cutoff/terms claims unless you actually prove it; source no-revalidation and prior preview tests stand separately. Final report must list precise proofs instead of declaring all prior F1–F6 globally closed when omitted.

E — replace fake history marker and guard the REAL live driver.
Current r2-driver.sh 'hist-marker' heredoc only imports json/sys and PRINTS HIST-CHECK-OK; it never GETs or validates history. Thus 23PASS includes a completion marker, NOT a reassigned history assertion. Replace with real owner GET /reservations/LVAAAA/history200 + exact oldprefix/new reassigned seq/rev/plan_id/complete FromTo/terms/absoluteclock retention. Check closure explain and stale code as current script does. After a real later mutation of LVAAAA (PATCH party1, assert recorddiff/revision advance), replay original applykey to200 EXACToriginalapbytes with full export baseline BEFORE replay unchanged AFTER. Record alreadyapplied errorcode (current only409).
r2-guard.sh is a SEPARATE 7-line health404 script. It proves THAT script exits1, not the actual r2-driver.sh failure wiring. Put an evidence-only forced-wrong flag inside the actual fresh driver, invoke the SAME final driver normally and with flag, retain counted FAIL/nonzero under new r3 paths. All checks explicitfailcount; exportHTTP200+envelope/login200 and privatePythonstderr alreadyimproved, retain. Preserve r2 logs23/0 and independentguard1 honestly, don't overwrite them. The first r2 helperfailure raw output was overwritten: keep that disclosure, don't reconstruct.
Reuse unchanged real e5e5271d1bb53c53df38aa485f7d1919211ed1b26071dd1bb15d7c7cd199948c image, fresh owncontainer with literal run/inspect fullId/Image/env/caps/health/stoprm/absence. Prior three-deleted-image-ID claim still not in retained catalogue; qualify as unretained report, don't repeat 'IDs recorded' without an actual existing file. No sharedcache cleanup. No Dockerbuild if product unchanged.

Exact gates CWD worktree/stage-4:
gofmt -l internal/service
go vet ./...
go test -count=1 ./...
go test -race -count=1 -v ./internal/service -run '^TestReplanApply|^TestClosure'
If gcc missing retain actual CGO_ENABLED=1 attempt; host verifies race. TEST-only no build/UI/harness repeat. Fresh r3 evidence command headers actual argv/absoluteCWD/UTC/effectiveexit/raw; actual counts from final runner, not forced16. No new Go service product changes, validator changes, source probes, RUN/PLAN/locks or earlier stages.
Complete foreground once and report DONE/BLOCKED with A–E effective namedassertions, exactownedstatus/counts/exits/rawpaths and honest qualifications. No acknowledgement-only turn/acceptanceclaim. Components: replan_engine, closure_store, reservation_engine, history_engine, series_engine.


## S4-R0 reviewer harness readiness

Shared57 is inprogress with readonly fullhandoff6a415606-afda-4f15-a1b0-3253650f94ff. Candidate0191aee2ba026c266d1d1231ca0e440b012a675c is context only, never an acceptance/frozen revision. No harness run or Docker lifecycle is assigned; readiness report precedes exact-SHA formal S4-R1 after full integration.

## ACTIONABLE S4-R0/shared57 — read-only reviewer sandbox readiness

You are reviewer Seatright-ZCode; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-zcode.md. Candidate readable local path /home/nryn/work/seatright/runs/tablekeeper2/wt/review. Current HEAD0191aee2ba026c266d1d1231ca0e440b012a675c is CONTEXT ONLY, NOT a frozen review revision; coordinator metadata/integration can advance while this preflight runs. Stage-4/ present, managerpreview integrated, R2apply/Aamendment proofs pending. Accepted earlier trees MUST remain stage1=8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c, stage2=9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f, stage3=c783f9e08522a04a62bb11f9a0e3c485d077684f.

This is readiness ONLY. Do NOT run harness, any Docker build/run/stop/rm/prune/tag/pull/network lifecycle, or source/kit/Git/state mutation. No correctness/design acceptance verdict or stage outcome. Write evidence only under /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S4-R0/ (private sensitive diagnostics when needed). Reviewer candidate read-only; no source edits.

Reason: implementer separate sandbox10GB Docker volume has repeatedly hit ENOSPC, last R2 build recovered by owner-image removal; don't assume its environment or previous S3-R0's2GBfree/warmrunner applies now. Your own daemon/caches are separate and only your measurements govern future strict isolatedstage4.

Bounded actual read-only checks:
- Candidate gitrevparseHEAD/status and accepted1–3 treeIDs; inspect stage4Dockerfile pattern/read source dirs needed for forecast. No frozenSHAclaim.
- assigned /home/agent/harness-venv/bin/python -V; import unmodified harness/httpx; exact --help supportedflags, currentkit/source paths. Record missing interpreter/nonzero honestly, don't invent fallback outcome or install.
- docker version/info/context/DOCKER_HOST presence (no secretvalues); DockerRootDir filesystem total/used/free, root/tmp separate filesystems; docker systemdf/imagecache counts and relevantwarmbasecache/runner identity. Do not purge cache even if accounting says reclaimable.
- df-harness-runner:latest availability/fullID/size/creation/layers/cache usability from genuine unmodified kit harness/Dockerfile/ensure_runner_image path. Prior Chromium layer can be warm but rebuild-cache forecast is an estimate untilrun. Identify whether no Chromium/apt download is expected; don't cold-build now.
- stage4candidatebases (node26-bookworm-slim/golang1.27-bookworm/debianbookworm-slim) warmstatus. Fullstage4isolated run may build ALL olderstage1/2/3 upgrade sources; verify actual kit implementation rather than assuming stage3source behavior. Quantify likely newwrites vsfree and cold-base invalidation residualrisk.
- build-time requiredregistries reachable via bounded network requests ifneeded, distinguish actualDNS/HTTP responses from product runtime; no runtimeisolationverdict.
- identify safe OWNED superseded reviewer image candidates byname/ID/provenance + reclaimestimate ONLY, no deletion. Never touch runner/sharedbases/otherseats/caches/volumes. Ifno safeownership established, sayso.

Future exact command to VERIFY/DOCUMENT, DO NOT EXECUTE:
cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo /home/nryn/work/seatright/runs/tablekeeper2/wt/review --stage 4 --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/S4-R1/checks
Formalreview later receives existingcommitted fullfrozenSHA and fullhandoff; acceptance requiresexact1–4 isolatedchecks/claimed4, nostage5, fullindependentAPI/planner/state/migration/all3trueDocumentupgrades/UI/designsourcegate gauntlet. Nothing ofthat is claimed bythispreflight.

Foreground finish ONE READY/BLOCKED report with actualcandidatecontext/checks/mode/interpreter/daemonwarmcache/disk/headroomforecast/ownedreclaimoptionsnotexecuted; exactrawlog/PREFLIGHT.md paths, commandsabsoluteCWDUTCeffectiveexit, failedprobes preserved and qualified, elapsed/usagevisibleorunavailable. No acknowledgement-only turn, no approvalquestion/sleep/wait/poll. Components: service_image, review_gate.


Status: S4-P, S4-M, fixture S4-G and manager preview S4-R1 are accepted and integrated as work items. S4-R2/shared56 atomic apply and S4-A/shared55 recurring amendment are uncommitted on their existing dirty bases for effective-proof repair. Stage 4 is not accepted; accepted stages 1–3 remain immutable.

## Requirements ledger

R1–R303 are carried forward verbatim from the accepted stage-3 ledger. R304–R382 enumerate stage-4 requirements, each with its source section. S means the supplied checks exercise at least a part of the requirement; U means independent verification is required. Supplied stage-4 checks only add shallow empty-preview and recurring-clock smoke coverage, so S never substitutes for the full requirement proof. Each requirement is proved against the integrated image and source, rather than inferred from a fixture transport.

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
| S4-W | OpenCode, shared59 | Narrow http.go + append http_test.go + NEW series_amend_integration_test.go; real integrated A/R2 dependencies, no router ownership overlap | R351, R370 | HTTP auth/privacy/body/path/idempotency semantics plus final all-source gates |
| S4-I1 | OMP | After R2+A+W: native import validation extension and corrupt-state/producer tests, owned paths frozen at handoff | R380–382, R339, R342, R376 | Strict legitimate reassigned history/closure/plan/series acceptance, atomic corruption rejection, all-source races |
| S4-D | OMP, shared58 | Separate genuine stages 1–3 donor producer; new stage4-donor probe only; reassigned while R2 remains active | R380–382 donor preparation | Inspected independent old processes, raw receipt bindings, sabotage guards, private evidence |
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


## Coordinator record

Final narrow proof bindings pending: S4-P tiers/oracle/true greedy trap are verified and host fmt/vet/race16/fullnormal/build all pass; the saved-output isolation assertion must mutate the actual Request that produced it. S4-M clone-shape matrix/full synthetic receipt retention/pair-to-pair are verified; converse public-map isolation must reserialize the live map and history clone mutation must compare complete original Entry JSON. Same dirty bases and only new test files; no product defect or commit. Final tiny handoffs sent, with no unnecessary broader sandbox gate repeats. G53 stays independent.

S4-M first report is under bounded test-only proof repair. Seven owned paths and raw fmt/vet/fullnormal/scoped races/build gates are verified; deep-clone/value/helper/Reassigned source matches the frozen contract with no product defect found. Tests must mutate the actual returned public map, exercise nil/empty/populated clone paths, pin complete synthetic old receipt retention and prove pair-to-pair plus cloned-entry isolation. Full same-dirty-base F1–F4 correction sent; shared52 remains in progress, no M commit or S4-A handoff yet. P51 test repair and G53 remain independent.

S4-P first report is under bounded test repair. Only new planner.go/planner_test.go are owned and changed; host fmt/vet/race16 tests pass. Source exhaustive solver matches the frozen API, but objective tiers lack real competing outcomes, both oracle worlds are infeasible and its helpers reuse production code, the documented greedy trap is not a trap, and snapshot assertions are incomplete. Full same-base F1–F4 test/evidence correction dispatched; shared51 remains in progress with no commit. M52/G53 continue independently. No product defect or stage verdict inferred.

Quiet lossy private-task fallback: native private creation was rejected under approval_policy=never; PLAN/RUNLOG preserve current steps and outcomes without private ids/dependencies/history. Acceptance audit, evidence archive and accepted folder copy are complete. Initial contracts/ledger/publication and dispatch are the current step. Shared cards are only actual team assignments.

Initial dispatch complete: clean fixed worktrees reset and verified at bda7c43023123bd8c96809d2c2f8066f1f81bdae; shared51/52/53 in progress. Full self-contained original-task/four-spec/382-ledger/scoped-contract handoffs accepted: S4-P 58aa1df7-5be2-42aa-93f4-077f0c133e23; S4-M 72120fff-a6c8-4fd6-b5a6-53a51307b228; S4-G 7e47c02b-4aea-447e-8dd7-555ba8f80ea4. Quiet fallback's audit/archive/copy/planning/dispatch steps complete; next verification starts on actual reports. No peer waiting/polling; progress resumes when reports arrive. Accepted stage folder hashes must remain unchanged at every integration.

## S4-P and S4-M accepted; endpoint streams start

S4-P/shared51 accepted after final binding b4bfac37: only new planner.go/tests, pure exhaustive source unchanged; genuine objective-tier competition, independent two-feasible/one-infeasible oracle, global greedy trap and live input/output isolation effective. Host fmt/vet/race16/fullnormal/build0, final isolation race0; peer missing-gcc attempt retained. Owner 1e1a381e80ad0d4554184843aa1cb1115f234497 integrated.
S4-M/shared52 accepted after final binding 55311d38: seven owned paths; full preview live-map isolation, complete cloned Entry preservation, 12 nil/empty/populated clone subtests and honest synthetic old storage proofs effective. Host fmt/vet/fullnormal (service39.458s)/service6+history5 scoped races/build all0. Owner 019a27b0fe5280a6a6ee66acd06666044860a685 integrated. Stage4 still not accepted; stages1–3 immutable. Grok53 remains independent active.
Lossy private audit fallback: source/evidence verification, host gates and integration completed; next self-contained contracts and dispatch in progress. Native private creation previously rejected by approval policy; no shared-board substitution for private steps.

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

## S4-A/shared55 proof correction — in progress, dispatch 5e9edb2b-bfab-4add-91ed-fee0549686eb
Only two owned new files; no product defect established in core. Host formatting/vet/20 scoped race/full normal/build pass. Several initial tests do not prove reported cutoff, no-op, precedence, metadata, replay or real fold behavior; new effective assertions required before commit. Router remains S4-W; real repair and imports remain later lanes. No worktree reset. Historical contract follows the bounded correction.
## ACTIONABLE S4-A/shared55 bounded effective-proof repair
Role backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md. SAME DIRTY fixed worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp; SAME fullbase727eae17559928196cc7f3f5c7dc8967b0d7088e. No reset/Git operations. Own ONLY NEW series_amend.go/tests; correction edits ONLY series_amend_test.go unless an actually reproduced core defect requires a bounded owned series_amend.go fix. No product defect established: source preparation/working occupancy/commit/noop/closure/Idempotent flow looks correct, host fmt/vet/scoped20race already EXIT0 and fullnormal/build running. Retain correct product. All inherited files/otherstages/router/pureengines/controls/import/frontend/Docker/probes/docs forbidden. W serial router later, R2 otherowner active. Do not rebase onto R2 or invent realrepair/HTTP-amend claims.

Report56487fc3 and requirement map overstate several proofs. Repair the specific assertions below in one foreground turn. Keep correct auth/globalbodymatrix/eligibility/rollbackfailedkey201/two-independent-nonoccupindex failures/absoluteclosure/DST structure. No unrelated scenarios or large helper framework; no brainstorming comments/emptyif blocks/unused snapshots.

F1 OLD accepted cutoff both directions + dated full terms:
CutoffAndPolicy uses May2027 future times (~7months away) and merely adopts duration60/v1; no cutoff is exercised. Stable genuine workflow: UTC restaurant/all-weekopening, anchor TOMORROW calculated ONCE from time.Now(), fixturecutoff0/slot30/duration60. Realcreate/adopt2 while old0 permits; publish selected cutoff10080 for sameanchor date. Amend index0 to another valid clock: old0 permits201 despite new10080 (anchor inside new7day cutoff), adopts newterms. Publish latestcutoff0; another REAL clock amendment must409 cancellation_cutoff using STORED10080, despite newest0. Full exportidentity/no claimedkey on failure. This proves bothdirections using realproducers, not forging v0terms/systemclock/sharedhelpers. Assert complete6key terms/version/cutoff/capacities/actualabsolute start-end duration and frozen oldhistory terms/identity/party/tables/noeffective_from. HugeStaleBeforeCutoff should use an actuallycutoff-blocked record (reuse genuine setup) with validGLOBALbody plus invalidCANDIDATEgrid; stale409 wins. InvalidGLOBAL99:99 stays422 beforeguard per originalcontract.

F2 NOOP + empty eligible zero-metadata:
NoOp claims off-grid but18:00 stillvalidslot30; onlydurationchanged. Make storedclock reallyoff-grid underselectedlaterpolicy (original19:30slot30 thenpublishslot60from18:00), assert selectedrules would reject retainedtime for REALvalidation, then identicalclockamend201 keeps COMPLETE records/histories/series/countermaps/users/tokens/policies/restaurants/closures/plans unchanged EXCEPT one normal newreceipt. Also genuinelybypass oldacceptedcutoff using F1 state carrying10080 and SAMEclock. Capture full exportbefore/after; compareallnamespaces/count+bindreceipt, not historylengthonly. EmptyEligible samewhole-state comparison exreceipt/exactcurrentresponse. Receipt is a legitimate statechange.

F3 NONOCCUPANCY before earlier occupancy:
Precedence onlyoccupancy409; comment promises exportrollback without assertion. Build actualindependent earlier memberoccupancy atrequestedclock and latereligiblemember invalidgrid/closedday via datedpublication. Independently earlier-alone409 table_unavailable and later-alone422 correctcode, eachfullrollback; fullsuffix mustlater422 beforeearlyoccupancy, full exportidentity/noreceipt. Keep PrecedenceIndexOrder's two distinctvalidationfailures; pinbothindependently pluscomplete rollback. Remove speculativecomments. RollbackAndReuse genuinefailedkey201 good; addexactcode/replay200 originalbytes/exportunchanged ifmissing.

F4 COMMIT/concurrency full-state once:
CommitMetadata currentlyhistorylengths/subsetcounterloop/falseflags; unrelatedseries assertion is unused sid2, no bookrevision/Changedvalues/historyprefix binding. CaptureFULL before export; affectedoccurrences identities/owner/party/currenttables/scheduleddates same, exactnewclock/end/terms/rev+1, oldhistoryprefixbyteequal+EXACTone Changed matchingseq/revision/nondecreasingat/fullterms/orderedstartsLocalFromTo; series+1ONCE/flagsretain; unrelatedseries EXACTbytes; FULLcounter maponlyr1+1; alluntouchednamespaces exceptnormalowner/method/path/key/status/body/rawresponse receipt. Skippedexceptions/cancelled have fullimmutable record/history/flag checks.
Concurrency/SameKey50 currentlyonlycodes/body/counter. PinFULLpre/postdelta: targetbookrev/historyexactlyonce/currentmatches REALwinner201response, seriesonce/fullcounteronce, unrelatedrecords/histories/series/policies/users/tokens/restaurants/plans/closures identical, EXACTone new scopedreceipt; differentkeyloser no receipt; allpriorreceiptspreserved. Delete empty200if-block. Never print tokens/responsebytes in failures.

F5 FIRST replay and live-object isolation:
Replay baseline is AFTERfirstreplay (mid), so firstreplaywrites canpass. Capture export AFTERactualrk2mutation+cancel BEFORE FIRSTrk replay; requirecurrentrecorddiffers/cancelled, first200originalseriesbytes/exportidentity, second200same, changedcanonicalbodycommittedkey409+stateidentity. RemoveunusedpreReplay.
Detachment onlyscalarparty and PRE-amend inequality. CaptureFULLexport AFTER201/pristineoriginalresponsebytes; mutate ACTUALreturnednested table_ids, accepted_terms capacities/hours and occurrencefields; FULLexportbyteidentical, samekeyreplay200pristinebytes and freshcurrentseries unchanged. Correctactual types/liveobjects, not decodedcopies/oldbytecomparison. Removedummy s2/unreachable auth checks.

F6 REAL fold amendment + exactgap:
NYFoldFirst adoptionclock01:30 thenamend01:30 is NOOP, provesadoptionnotamendment. Create/adopt03:30 then REALamendmember01:30 on2026-11-01 (futurethisrun). Assert start firstEDT01:30-04:00 UTC05:30, exact90min end02:00-05:00 UTC07:00, realbookrev/history/counter/seriesonce. Correctcomment falselysaying01:00end. Berlin2027gap exact422 invalid_local_time +wholeexportidentity, not any422.

F7 BOUNDARIES and claims:
Map claims count12/fromindexbounds/pairretention nottested. Smallgenuinecount12 case from_index=count-1 (onerealchange earlier11records/historyuntouched), index0includeanchorrealchange; or qualifyunproven count12/paircase anddefer W insteadofclaiming. Prefercompactexistingfixture. ScheduledDates directlychangesrecordwithoutrepair/history: keep honest syntheticcurrent-selectionseam (norealappliedrepair/scheduled-vs-currentdate distinction). Actual R377–378 plan/series interactions deferredR2+W; do NOT forgeunexceptioneddate divergencejusttofakeproof. PortabilityR380–382 deferredI, notnewinheritedacceptance.

F8 HONEST evidence:
Catalogue S4-A has sourcegate6txt/run.sh/requirements/binary; no smoke or failedattempt rawfiles. "Preserved" failedattempts are DISCLOSED ONLY unlessactualcontemporaneousfileexists; never reconstruct. Optionalearlierhealth/ownedpidstopunverifiedsummary: withdrawclaim or freshownPID9184smoke literalargv/CWD/UTC/status/bodycharset/killwait separate/private serverlogs0700. No HTTPamend/Dockerharnessrequired. Broader12:00–44approximate, rawgatefirst12:41:35; state actualgatewindowseparately.
Original6logs untouched. Fresh S4-A/r2/ actualquoted/listARGV (priorrun.sh $*losesboundaries), absoluteCWD/UTC/effectiveexit/raw. No secrets/exports/tracebacks stdout. Gates: gofmt -l internal/service/series_amend.go internal/service/series_amend_test.go; go vet ./...; go test -count=1 ./...; go test -race -count=1 -v ./internal/service -run '^TestSeriesAmend'; go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-omp/S4-A/tablekeeper-r2 ./cmd/tablekeeper. No UI/Dockerharness/existingfileedits.
ONE final foregroundDONE/BLOCKED report: F1–F8actualproofmap/scope/fullbase/evidence/exits/measuredcounts/elapsed/usage/failuresretainedordisclosed/expectedrouter-realrepair-import deferrals. Noacknowledgments/peerwait. Components: series_engine, reservation_engine, history_engine, closure_store, receipt_store.

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

Dispatch complete: shared54/55 from verified727eae17559928196cc7f3f5c7dc8967b0d7088e, receipts5ba35ade-763f-47c8-9313-329c61c20796/a006af16-14f4-4148-809b-145869cd5ce7. Independent real dependency endpoints active; Grok53 untouched. Lossy audit/integration/dispatch fallback steps complete.

## S4-G fixture consistency correction — completed
Final host check/100 tests/script syntax pass. Corrected browser proof: 91 checks, 54 screenshots, four recordings. Policy version1/duration60, exception18:30–19:30 and valid edited party5 are asserted on parsed records and exact requests. Author851ecb367a5a3c5912c8ca36e2d7fa8f4f34094e merged213835b7ee3659953b1d5bed1cba2fecde74ee29; shared53 completed. No product components changed; no live API/import or stage acceptance claim. Earlier correction below remains historical.
## ACTIONABLE S4-G/shared53 bounded fixture consistency correction
Role frontend implementer. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-grok.md. SAME DIRTY worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-grok; SAME FULL base bda7c43023123bd8c96809d2c2f8066f1f81bdae. No reset/Git operations, no new shared item. Prior three-owned-path work remains in place. Product components passed fixture behavior; no product defect found. Host npm test passed 100/100 and exact raw peer check/test/build/browser outputs were corroborated (90 checks,54 shots,4 videos). Only NEW src/test/s4-g.test.ts and NEW scripts/s4-g-fixture.mjs need the corrections below. Keep existing correct foundation.test.ts ban removal as is. No src/lib/component/router/backend/Docker/other-stage/docs/PLAN edits.

F1 — synthetic clock policy incorrectly claims version0. In BOTH files clockTerms uses policy_version:0 while changing original fixture duration90 to60 (script spreads policy0). A genuine current booking cannot have fixture0 terms differing from original detail. Set clockTerms.policy_version to a genuine published-version label1 (still fixture transport, no policy API claim), retain complete six keys/no effective_from, and keep originalClockReceipt/pairReceipt + seating-repair record at policy0/duration90. Assert current amended/cancelled clock terms version1/duration60, old receipt stillversion0/duration90, start/end elapsed matches accepted duration. No need to add a screen/policy route or claim live publication.

F2 — exception start/end is inconsistent in BOTH files. Exception changes starts_at_local/starts_at to 2027-06-24T18:30 but leaves inherited ends_at21:00 while clockTerms saysduration60. Change its ends_at to19:30+02:00; keep display18:30, table set and appropriate terms/revision. Add actual full parsed-record elapsed/terms assertion for exception as well as amended20:00→21:00 and cancelled record. Avoid merely asserting key names or changing renderer to conceal a malformed payload. This is fixture fidelity, not a backend-validation test.

F3 — the UNIT conflict request has party7 on original pair t_1+t_2 with capacity2+4=6; actual service rejects over-capacity422 before a seating conflict409. In 'keeps the form on a conflict and refreshes the returned closure', edit party to5 (valid and different from searched6), assert exact POSTparty5, preservedform5/summary, refresh searchedparty6 and all closure memberships as before. The browser's refused Table3 party6 is already valid; leave that scenario intact. Assert fixture selected-set capacity can hold this edited party so the future fixture cannot silently become an impossible409 case.

Keep meaningful immutable pending-body/key and current-repaired-versus-old-receipt tests, closure/floor/grid availability, no operator/explain/public-bearer requests, reduced/blank session/app-only viewport reveal/contrast gates. This correction is only fixture source/test values and targeted assertions; no invented real replan/amend/import/harness acceptance.

## S4-R1 accepted and integrated — shared54 completed
Final source proofs verified: date-selectedv2 versus stored acceptedv1 capacity, exact-once assignment, complete storedplanrestaurant/closure and prior maps/receipt bindings. Host final two-test race EXIT0 (5.343s); previous21previewrace/fullnormal/vet/build remaingreen on unchangedproduct. Fresh r3 executeddriver24actualPASS observations, fullcontaineridentity/run/inspect/stop/rm retained. Coordinator independently verified saved replay/export byte equality and actual validenvelopes; evidence-only driver conditional-cmp failure wiring is not claimed fail-closed and newR2 drivers require explicit failures/negativeguards. Author9068f2c98f46188faf1eeb54d536c1f1ee3341c0 merged ef41d20db3948355b02b442a0929c676ccc3dc0e; earlier1–3trees unchanged. R1previewonly, no apply/stageacceptance.

## ACTIONABLE S4-R2/shared56 — atomic manager replan application and closure enforcement
Role backend implementer. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. Fixed worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode; coordinator resets it to the verified integrated base read after this contract is committed (full SHA in dispatch). No Git mutations. Finish in one foreground turn, roughly20–30min; report DONE or concrete BLOCKED, not still-running. Do not wait/poll/sleep for another seat. All original task/four verbatim specs/382ledger attached below are authoritative.

Dependency acceptance: S4-P planner and S4-M values/history already integrated. S4-R1 author9068f2c98f46188faf1eeb54d536c1f1ee3341c0 merged ef41d20db3948355b02b442a0929c676ccc3dc0e. Actual PreviewReplan manager/real Idempotent/exact route/strict offset timestamp validation/planner assembly and immutable preview receipts are available. Host prior21 preview races/fullnormal/vet/build EXIT0; final own-capacity/race bindings2/2 race EXIT0. Fresh actual image de05a9bd... driver24 observations and saved-byte comparisons verified; run/inspect/stop/rm retained. No apply implementation exists yet. OMP independently writes ONLY new series_amend.go/tests from older727eae1; do NOT touch either. Serial router belongs to you in R2, later W adds amendment route after both integrations.

Own ONLY:
- NEW stage-4/internal/service/replan_apply.go + replan_apply_test.go.
- NARROW stage-4/internal/service/reservations.go: common conflictingReservation closureBlocks integration (plus precise doc); no ordinary-write/revision/terms logic rewrite.
- NARROW stage-4/internal/service/availability.go + appended availability_test.go: closures join no_overlap for singles and pairs while preserving existing detail/grid/capacity/explain rules.
- NARROW stage-4/internal/service/http.go + appended http_test.go: exact manager apply route arm/tests.
No other product/test files. Inherited tests retained unmodified; append your tests. Do NOT touch replans.go/replans_test.go, planner/**, history/**, model/state/version_* /controls/import/auth/idempotency/policies/series/moves, frontend/package/locks/Docker/probes/RUN/PLAN/RUNLOG/mandates/earlier stages1–3. Coordinator owns commits/integration. If a frozen seam genuinely cannot support spec, stop the conflicting edit and report precise required change; no invented shadow foundation.

Frozen actual seams:
Service.ApplyReplan(token, restaurantID, planID, key string, raw []byte) Result (NEW).
applyReplanLocked(st *State, userID, restaurantID, planID string, obj map[string]any) Result (NEW pure locked callback).
Wrap real s.Idempotent(token,http.MethodPost,"/restaurants/"+restaurantID+"/replans/"+planID+"/apply",key,raw,callback). Receipt BEFORE callback manager/state validation, actual path/user scope, canonical body, 201 commits work snapshot+receipt, non201 discards work, replay200 original bytes. No nested Service locks.
State.Plans map[string]Replan, Closures map[string][]Closure, RestaurantRevisions map[string]int; actual plan ID/restaurant/revision/Closure/Assignments/MovedCount/UnusedSeats/Applied. Existing replanPublic preview exact6fields unchanged. closureBlocks(st,restaurantID,ids,start,end) bool is half-open absolute intervals/shared member/cross-rest safe.
reservationSnapshot(r) history.Snapshot, history.Next(entries,time.Now()) (seq,at), history.Reassigned(before,after,seq,at,planID) (Entry,bool). This gives EXACT one full table_ids change even singleton→singleton with plan_id, clones accepted terms; unordered set noop returnsfalse. DO NOT use ordinary prepareAmendment/commitAmendment for an operator repair: these would revalidate/cutoff/adopt terms or mark changed instead of reassigned. Existing touchSeriesForChanges(st,changedRefs,false) coalesces each series once preserving ALL flags; restaurantcounter doneonce outside. reservationTableIDs helper and Public() preserve scalar iff singleton (TableID=t forsingleton, empty forpair; canonical TableIDs).
Common conflictingReservation is consumed by create/PATCH/moves/adoptSeries and will also be consumed by OMP amendment. Add closureBlocks after candidate absolute bounds parsed, before booking loop; keep exclusions affecting reservations only, closures neverexcluded. Cancel and immutable receipt replays keep inherited behavior.

Implementation required R334–350/R377–379:
1 Manager auth through Idempotent, real ManagerUserIDs; unknown restaurant/unknown or foreign plan404, valid nonmanager403, missing/invalid token401 as inherited. Body {} through normal JSON parsing (malformed400). Unknown body fields follow inherited ignored-field semantics; body remains canonicalreceipt-scoped. Exact POST /restaurants/{id}/replans/{plan_id}/apply only; wrong method/trailing/deeper/empty id404; preservepreview/publicpolicy/router arms.
2 Callback resolve same-restaurant saved plan. Already Applied under NEW key =>409 plan_already_applied BEFORE captured revision (its own application increments counter, so checking stale first would mask required result). Unapplied with currentRestaurantRevision!=captured =>409 stale_plan atomic unchanged. Same key replay resolves before either check and before later manager demotion. Intervening write at OTHER restaurant does not invalidate. No solver rerun/currentpolicy/cutoff use; apply captured assignments under unchanged revision.
3 Preflight all saved considered refs, preserve exact identity/UserID/Reference/Restaurant/party/startsLocal/starts/end/createdAt/status/AcceptedTerms. Build detached candidates by changing only canonical table selection; set-based truly moved increments Reservation.Revision once, history.Next plus one Reassigned with complete From/To (even singleton-singleton), frozen complete6keyterms/noeffective_from, correct planID and sequence/instant. Unmoved record/history bytes unchanged, including every considered record in ordered response. No cancellation/no booking disappear.
4 On work snapshot atomically record closure and all assignments, set Applied=true, restaurantRevision++ EXACTLY once even zero considered/zero moved, touchSeriesForChanges(changedRefs,false) once => each series with >=1moved member +1 once, preserve schedule/exception flags/anchor/member identities, unrelatedseries unchanged. Same old booking/history/terms remain immutable except required moved metadata.
5 Return201 EXACT {"plan_id":..., "restaurant_revision":newcounter, "reservations":[allconsidered Public records in referenceorder]} allocated[] whenempty. Store apply receipt immutable original response; later patch/cancel/publish/otherapply does not alter replay200 bytes or state.
6 Availability: applied closures independently block selected-date slot intervals on closed members, so singles and BOTH declared pairs includingmember excluded; half-open adjacency free; cross-date/offset differences treated absolute, otherrestaurant unaffected. Explain no_overlap false from closure even whencapacity false; capacity rule independent, ordercapacity thenno_overlap, conjunction==available ids. No-explain shape unchanged. Minimal approach append closure occupancy to local confirmed list for eligibleOptions/explainSlot; may use helper without changing frozen signatures. Closure does NOT remove grid slots/create new screen.
7 Creates/real PATCH/moves/adoptSeries/new recurring amendments must409 table_unavailable on closure overlap via common conflict seam, with full rollback/counter/history/receipt none. Old-receipt replay bypass still200. Preserve no-op semantics and existing time/capacity/precedence logic, no rate limiter/fixture branches.

Effective acceptance tests, hand-computed expected assignments (never planner-derived expected):
- manager/owner/path/authbody/privacy exactHTTP guards.
- Success singleton move, canonical pair transitions, unmoved bytes; whole-export delta pins closure+plan.Applied+each moved record/history+counter+single applyreceipt with owner/method/path/key/status/body/rawresponse, all unrelated namespaces/prefixes unchanged; response allrefs sorted/fullPublic projection.
- Zero moved and empty considered apply eachrecordclosure/counter+1 (but no booking/history/series drift).
- stale aftercreate/realPATCH/cancel/publish/successotherapply, but no-op/failed/replay/preview and otherrestaurant writes do not stale. Check wholeexport unchanged on409 and failed-key genuine201reuse with a newvalidplan.
- alreadyapplied newkey409 propercode despitecounterchanged; originalkey200EXACToriginalbytes after real lateredit/cancel/publication, exportunchanged; changedbody samekey409. Demoted-manager originalreceipt200 butfreshkey403.
- series multiplemovedmembers once/eachaffectedseriesonce, unmovederiesnone, existingtrue+falseexception flags retained/scheduleddates and accepted terms retained; not ordinary Changed event.
- closure avail/explain all4rulecombinations includingcapacityfalse/no_overlapfalse; singleton/pairs/sharedmembers/no transitivity; adjacency, short-innerclosure, absoluteoffset/crossrestaurant.
- actual create/realPATCH/batch/adoption closure409 + exportidentity + failedkeyreuse; inherited no-ops/replays unchanged. A engine not yet merged, leave its closure integration test for W through commonseam rather than inventingstub.
- samekey50 apply =>1×201+49×200 rawidentical/wholecounter+1/eachbookonce/oneclosure/onereceipt; competingdifferentkeys sameplan =>one201 + one plan_already_applied409, cleanloser; competingplans samecapturedrevision=>one201+one stale409; no partial moves. Fullmap comparisons rather than onlycounts.
- Detachment actualnested response arrays/terms/history maps vspristine export/replay; source-state noaliasing.

Gates CWD stage-4: gofmt -l internal/service; go vet ./...; go test -count=1 ./...; go test -race -count=1 -v ./internal/service -run '^TestReplanApply|^TestClosure|^TestReplanPreview'; go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-R2/tablekeeper ./cmd/tablekeeper. If gcc missing retain actual CGO_ENABLED=1 attempt+nonrace concurrency; coordinatorhostrace. Final merged A/W/import suites later; do not weaken exportvalidator to excuse unimplementedstage4historyvalidator (I1owns it).
Docker rootcwd build -t tablekeeper:s4-r2 stage-4; own tk-s4-r2 PORT9180 with2CPU/2GiB, realhealth200body/charset; actual HTTPpreview201/apply201/movedhistory/closureavail+explain/rejectedcreate409/stale409/already409/replay200 immutable; owninspectprovenance+literalstop/rm absence. Fresh artifactdir S4-R2, preserveattempts.

Evidence strength: r3 preview artifactdriver's "cmp && check" can skip FAIL on mismatch; current24observations independently verified by coordinator raw-byte compares, so no fail-closed guard claim is made. For ALL NEW R2 driver checks use explicit if/else incrementFAIL+nonzero, not conditional omissions; export helper requiresHTTP200+validenvelope, tokens/login require actual200/nonempty. Test drivernegativeguard by forcedwrong expectation (countedFAIL/nonzero), saveguardrawlog. Private Python diagnostics stderr under0700work not publictraceback; nevermanufacture counts/response/reconstructlosthistory. Capture actuallyexecuted driver+argv/CWD/UTC/effectiveexit/raw status/countlogs, full .Id/.Image/.Name.Env/caps; no token/password/body/export in room/publiclogs. Screens/browser notowned. Raw modes0700/0600, no secretsGit.

Report ONE final foreground DONE/BLOCKED: shared56/fullbase/worktree/exactownedpaths; requirements→named effective tests; source andDocker commands/CWD/UTC/exits/counts/evidencepaths/provenance/cleanup; known expectedlaterI/W/G work vsactualgaps, failedattempts retained or explicitlyunretained; measuredforegroundelapsed/visibleusageunavailable. No stage acceptance. Components: replan_engine, closure_store, reservation_engine, history_engine, series_engine.


## S4-R1 historical final proof repair — dispatched b811f5bd-99ae-45eb-8689-2b6af2febb7e
Strict timestamp product fix verified; host fmt/vet/scoped race/full normal/build EXIT0. No commit yet: dated capacity proof must supersede on the actual booking date, complete stored plan binding must include restaurant/closure, and fresh live functional plus cleanup evidence must be retained. Prior live-r2.log records run/inspect/health only, not its reported functional checks. Shared54 remains in progress; R2 remains dependent. Earlier correction below is historical.
## ACTIONABLE S4-R1/shared54 final bounded proof/evidence repair
Role backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. SAME DIRTY worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode, SAME full base 727eae17559928196cc7f3f5c7dc8967b0d7088e. No reset, no Git operation. Original work and strict interval fix remain; no apply implementation yet. All original task/spec/ledger/contracts below remain authoritative. Complete this repair in the foreground and send ONE final DONE or concrete BLOCKED report. Do not acknowledge or wait for another seat.

Verified: four owned paths only; strict regex/ranges/calendar plus time.Parse correct the four real host cases. Replay now really writes a later same-restaurant booking, exact original bytes and export unchanged; failed infeasible key genuinely succeeds201/replays200. Max6/4/6, real canonical pair/no-transitivity, nested caller-map mutation and prior-plan/receipt retention assertions are useful and pass. Host fmt/vet/scoped race already EXIT0; full normal/build running. No additional product defect established. Keep replans.go/http.go product unchanged for this repair. Edit ONLY replans_test.go for the narrow proof points below; fresh private evidence under evidence/seatright-opencode/S4-R1/r3/ permitted. Retain all r1/r2 files unchanged.

F1 effective own-accepted-vs-date-selected capacity proof: TestReplanPreviewOwnAcceptedCapacities creates bookings on2027-06-24, then publishes policy2 effective2027-07-01. That later policy is NOT selected on the bookings' date, so an incorrect preview using policy.Select(date) would still see policy1 and pass. Change policy2 effective_from to the SAME2027-06-01 (same-date supersession) or a date<=2027-06-24. Explicitly assert selectedTerms for booking date is version2/caps t_1=4,t_3=1, while actual stored booking accepted terms remain version1/caps t_1=1,t_3=4. Require the booking reference occurs EXACTLY once in the second preview (current loop can pass if absent), same t_3 assignment/changed true, complete record/history byte equality after preview. Keep the historical accepted capacity101 fixture. No new product fallback/policy modifications.

F2 narrow full-plan binding: Race50 claims stored restaurant and closure match public, but never checks pm["restaurant_id"] and serializes wc without equality. Add exact restaurant_id=="r_anker"; stored closure==public closure==expected complete JSON. Existing assignment/totals/applied/id/prior maps/owner/path/body/status/raw receipt assertions stay. This is a few-line repair, not new scenarios.

F3 lifecycle evidence still incomplete: actual /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-R1/live-r2.log is1417 bytes, contains only docker-run/inspect/health, ends health exit0 at12:31:58Z. Its directory catalogue contains no separate executable live driver/functional log/stop-rm output. Private reset/preview files do not prove which commands/status assertions ran. Report claims manager201/replay200/nonmanager403/eight-invalid422atomic/nofeasible409atomic/cleanup are therefore NOT corroborated by retained raw lifecycle evidence. Do not reconstruct past commands as history or label unretained attempts "retained". Qualify these r2 claims plainly.

Perform ONE fresh evidence-only live execution with unchanged product image tablekeeper:s4-r1-r2 (sha256:de05a9bd05cc65bea2a5cf09d4bae2d4a0dc60cc2d54c687d1314abf5c95a1d9), no rebuild required for test-only edits. New owned container tk-s4-r1-r3 PORT9180, 2CPU/2GiB. Save an ACTUALLY EXECUTED script and raw per-step logs under S4-R1/r3/ with literal argv, absolute CWD, UTC start/end, effective exits, full inspect.Id/.Image/.Name/Env/caps, exact health body+charset assertion. Script must fail nonzero on failed checks. Private request/response/export/token files under0700/0600; stdout names/codes/counts only.

Fresh functional evidence: real fixture reset204 and logins200, manager preview201 EXACT6-key shape/expected assignment/moved count, replay200 raw-byte equality, nonmanager403, all four malformed forms in BOTH from/to422 validation_failed with full export byte equality, and genuine409 no_feasible_plan with full export equality (use valid >=6char seed refs and a new uncommitted key). Show SAME failed invalid or infeasible key genuine feasible201 then identical200. Preserve any fresh failures separately (attempt dirs). Every command actually executes; no abbreviated curl summaries presented as runnable commands. Save actual stop/rm exits for only owned r3 container and final own-name absence. Check if r2 container remains and clean ONLY your own r2 if needed, recording current action honestly, not backdating.

Gates stage-4: gofmt -l internal/service/replans_test.go; go test -count=1 -v ./internal/service -run '^TestReplanPreviewOwnAcceptedCapacities$|^TestReplanPreviewRace50$' (single quoted -run argv); actual CGO_ENABLED=1 targeted race attempt recorded if still compiler absent. Previous full suite/build/strict host gates stand on unchanged product. No whole-Go/UI/Dockerbuild/harness rerun necessary; coordinator runs final narrow host race on these effective proof tests. No changes to inherited tests, product source, pure solver, model/state/history, router beyond retained original edits, web/Docker/RUN/PLAN/earlierstages. No manufactured responses, silent failure or credential stdout.

Final report: exact samebase/ownedpaths, F1/F2 effective asserted bindings, F3 genuinely executed fresh driver/log paths/exits/counts/provenance/cleanup, r2 unretained-functional-history disclosure, actual measured foreground interval/usageunavailable if absent, no stage acceptance. After acceptance coordinator commits and resets you to integrated base for actual R2. Components: replan_engine, planner_engine, receipt_store.

## S4-R1 earlier bounded correction — dispatched 871e15c5-c0a5-4296-b0f8-e5f0b5978a9c
## ACTIONABLE S4-R1/shared54 bounded correction — strict interval + effective proofs/evidence
Role backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. SAME DIRTY worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode; SAME FULL BASE727eae17559928196cc7f3f5c7dc8967b0d7088e. NO reset or Git operations, no new item/card. Own ONLY existing new replans.go/replans_test.go plus retained http.go/http_test.go edits. Keep frozen PreviewReplan/previewReplanLocked signatures. No planner/history/helper/state/model/othercore/validator/web/Docker/docs/earlierstage edits. R1 still preview-only; no apply route yet. Host fmt/vet/scoped race/fullnormal/build passed; this is not acceptance because F1 is a real validation defect and F2-F4 proof/evidence gaps remain.

F1 REAL interval validation defect, host live reproduction against your actual evidence binary, ownedPORT9185:
POST /restaurants/r_anker/replans with a manager/key/tablet_2 and
(a) from2027-06-18T18:00:00+24:00 to2027-06-18T19:00:00+24:00,
(b) from2027-06-18T18:00:00+02:60 to2027-06-18T19:00:00+02:60,
(c) from2027-06-18T8:00:00+02:00 to2027-06-18T9:00:00+02:00,
(d) from2027-06-18T18:00:00,1+02:00 to2027-06-18T19:00:00,1+02:00
ALL returned201 and created plans/receipts. Go time.Parse(RFC3339) is permissive here; accepting them does not fulfill the RFC3339 interval contract. Evidence: /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-codex/S4-R1/host-interval-probe.txt (status-only logs), private-first holds public response bytes, no token stdout; ownPID stopped. Implement LOCAL strict RFC3339 instant validation in replans.go BEFORE time.Parse: exactly four-digit date/two-digit hour/minute/second, literal T and separators, optional DOT digits fractional seconds only, explicit Z or signed two-digit hour:minute offset with hour0..23/minute0..59; real date/time bounds then time.Parse; require absolutefrom<to. Keep Z and valid fractional .1 and valid alternative offsets accepted, preserve original valid from/to text. BOTH endpoint fields invalidcases must422 validation_failed, COMPLETE export byte-identical (no plan/receipt/counter/other state), same failed key reusable on a genuine valid201. Do not alter shared clock engine or reject valid optionalfraction RFC3339. Add permanent tests for all4 forms, invalidfrom and invalidto, validfraction/Z/offset-equivalent ordering and valid fractions; no brittle format-roundtrip that rejects valid preserved offset spelling.

F2 REPLAY / reuse proof was not performed as reported:
TestReplanPreviewReplay comment says later legitimate booking but only '_ = diner'; no write occurs. Perform a real successful booking or policy publication at SAMErestaurant, assert its counter really increased and current export differs, then original key/body replay200 EXACT original JSON bytes; pre-replay versus post-replay COMPLETE export bytes identical, captured restaurant_revision remains old. Also reuse-before-revalidation by removing manager membership or corrupting callback-only context in an isolated test can prove receipt comes first, without product branches. TestNoFeasible's supposed failed reuse returns404 for unknown table; keep initial409+export equality but make SAME key later return genuine201 after removing/blocker cancellation or choosing valid feasible interval/body and assert replay200 bytes. No weakening of existing403/400/404/409 assertions.

F3 EFFECTIVE integration boundaries/capacity/pair/state evidence:
- Report says6-considered201 but TestLimits only checks7→422. Add a feasible max-world with6fixture tables,4declared pairs,6considered bookings and assert201/all6 reference-sorted assignments/fullunchanged identities/totals. Nonoverlapping30-min bookings on an unclosed table are a legitimate feasible boundary; 7books/table7/pair5 exceed each limit422 planning_limit + whole-state atomicity. Pureplannermax tests do not prove service assembly.
- OwnAcceptedCapacities current test uses policy mostrecent atcreation AND preview; it cannot detect switching to newest selected caps. After records created under oldselectedcaps, publish a DIFFERENT later policy (e.g oldt_1=1,t_3=4 versus latestt_1=4,t_3=1), then require preview uses OWN oldacceptedmap/correctassignment and storedrecord/terms/history byte-equal. Existing above-max case retained.
- Add at least one feasible actual service preview assigning/retaining a declared PAIR (canonical order and set-based changed flag), prior/proposed closure on anypairmember, no transitivepair; pin fullreturned assignments/totals independently, never call solver tocomputeexpected. Most currentfixtures close t_2, thereby excluding every declaredpair; this path is unproven.
- Existing preview/race state checks count plan/receipt maps but skip their whole values/prefix. Prepopulate a previous preview or create receipt; require ALL pre-existing plan/receipt entries unchanged, exactlyone NEW storedplan whose ID/rest/applied=false/revision/closure/assignments/totals fullymatch publicresponse and selectedtarget; exactlyone NEW receipt bound to manager/method/actualpath/key/canonicalbody/status201/original raw response. Othernamespace map equality including fullcounters remains. Caller-output alias assertion must MUTATE actual nested []string table_ids and actual closuremap returned, not replace the table_ids slot; capture FULL storedexport beforemutation andcompareafter, then receipt replay pristine. Foundation detachment tests remain unchanged, but R1 callback should prove its actual returned response.

F4 EVIDENCE gap:
Shared S4-R1 catalogue has commands.log with actual sourcegates and docker-build.log, but no actual run/inspect/live/cleanup log for reported image/container/health/201/replay403/422/409. Do not reconstruct historical logs; qualify unretained first lifecycle claim. For corrected source rebuild own tablekeeper:s4-r1-r2 and run tk-s4-r1-r2 PORT9180 --cpus2 --memory2g, fresh exact executable quoted argv/absoluteCWD/UTCstartend/effectiveexit/raw outputs. Full inspect .Id/.Image/.Name/Config.Env/caps; assert health200/exactbody/charset, actual managerpreview201+replay200 bytes, nonmanager403, invalidintervalmatrix422+exportatomic, valid infeasible409+atomic and ownedstop/rm. Token/body files under private0700/0600, public logs names/status/counts only. Preserve original all logs and failures; fresh r2 paths, no overwrites or unowned disk cleanup.

Gates CWDstage4: gofmt -l internal/service; go vet ./...; go test -count=1 ./...; CGO_ENABLED=1 go test -race -count=1 -v ./internal/service -run '^TestReplanPreview' (record real gcc error if unavailable, targeted nonrace includes concurrent tests, host race after); go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-R1/tablekeeper-r2 ./cmd/tablekeeper. Rootdocker build -t tablekeeper:s4-r1-r2 stage-4; live gatesabove. No UI/fullharness/import/apply/newstageacceptance. One foreground final DONE/BLOCKED report: F1 actual defect fixed and repro now422, F2-F3 effective tests with actualcounts, F4 exact raw lifecycle paths+fullIDs+exits, scope/base/failedattempts/gaps/cleanup/measured elapsed/visibleusage. No acknowledgment/peerwait.


Gates CWD .../stage-4/web: env -u FORCE_COLOR NO_COLOR=1 npm run check; env -u FORCE_COLOR NO_COLOR=1 npm test; env -u FORCE_COLOR NO_COLOR=1 npm run build; node --check scripts/s4-g-fixture.mjs; EVIDENCE=/home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-grok/S4-G/r2 node scripts/s4-g-fixture.mjs. npm ci alreadypassed and package/lock unchanged; do not repeat unless actual dependency issue. Use same available CHROME andPORT4184 or explicit documented overrides. Preserve original S4-G artifacts/logs untouched; fresh r2 command/CWD/UTC/effective-exit/raw logs. Preserve any failures rather than overwrite. Capture corrected lookup-clock/exception stills and proof plus same browser flow; private modes0700/0600, no tokens/bodies in stdout. Report measured actual unit/browser counts without forcing90/100 if assertion count legitimately differs; fixture-only caveat retained. Stop only own preview. No Go/Docker/fullreview/real upgrades yet. One final foreground DONE/BLOCKED report with three finding→effective-assertion map, exactpaths/scope/base/gates/raw evidence/failedattempts/gaps/measured elapsed/visible usage. No acknowledgments or peerwait. Components: restaurant_ui.

S4-A/shared55 provider400 retry: no completed report/test outcome; one full same-base retry requested foreground completion/concrete failure. Worktree/base unchanged, no reset or reassignment. HTTP reply cap16000 forces full retry through jam_send with separate no_reply settlement.


## S4-R2/shared56 effective-proof correction — in progress

Report38bf0720 source/scope audit and host fmt/vet/37 scoped races/full normal/build passed. No product defect established; complete state/concurrency/closure/series/replay proofs and retained lifecycle/negative-guard evidence remain required. Full correction123197e5-3592-4423-a106-6f330ceeb4bb continues same dirty base267f71e, no reset/commit. OMP55 remains independent active proof repair. Original contract is retained above.

## ACTIONABLE S4-R2/shared56 bounded effective-proof and lifecycle repair

You are backend implementer Seatright-OpenCode. Mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. Continue in SAME dirty worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode, full unchanged base 267f71e74767764d84dbbad121680c09fd8be972. No reset, Git mutations, or new work item. Shared56 stays in_progress; coordinator has NOT accepted/committed R2. Original full task/spec/382 ledger and frozen R2 contract are included below, unchanged. Preserve all existing effective assertions and inherited tests.

Coordinator audited report38bf0720: exact six owned paths, immutable stages1–3, core ApplyReplan/closure assembly looks correct; no product defect established. Host formatting/vet and 37 scoped apply/closure/preview race tests passed (service66.134s); full normal/build gates are being completed separately. This correction targets ineffective/missing proofs and inaccurate evidence claims. Source is unchanged unless a real defect is reproduced, explained and fixed within original ownership.

Owned only NEW replan_apply.go/replan_apply_test.go, narrow reservations.go/commonconflict, availability.go (append availability_test.go permitted if needed), http.go + appended http_test.go, all stage-4/internal/service. No planner/history/model/state/version validators/controls/policies/moves/series/OMP series_amend/**/web/Dockerfile/probes/RUN/PLAN/RUNLOG/earlier stages. Tests may live in your new file; current report's availability '+tests' are there, not an actual availability_test.go modification. Do not change product just to make assertions pass.

F1 — exact success and zero-move state delta (R335/R340–345).
TestReplanApplySuccess currently SKIPS plans/receipts/reservations/histories/closures/counters in its namespace equality loop, then checks only counts and APAAAA's public fields. It does not prove all skipped namespaces/prefixes/prior entries unchanged, full moved owner equality or stored receipt binding, despite the report's whole-export claim.
Pin COMPLETE pre→post allowed delta: each moved stored record differs only table selectors/revision; unchanged records byte-equal; old history prefix byte-equal, exactly one appended reassigned with seq/revision/planID/full canonical From/To/frozen complete terms; all saved plan fields unchanged except Applied; exact closure appended, prior closures untouched; FULL restaurant counter map with only target+1; new receipt exact owner/method/actual path/key/status201/canonical body/original raw response; all prior plans/receipts and unrelated namespaces unchanged. Compare full Public projections to sorted response, scalar table_id iff singleton. Existing Empty proof must also pin plan/receipt/other state and allocated [].
Add genuine NONEMPTY ZERO-MOVE apply, and a real pair-involving application (canonical pair→singleton or different pair) with complete reassigned arrays; hand-computed expectations. Pure planner pair and history tests do not replace service apply proof. Preserve old terms even after an effective superseding publication and repair past cutoff, proving no operator revalidation/adoption.

F2 — actual concurrency and exact-once deltas (R349/R52/R377–379).
TestReplanApplyRace50 currently compares pre/post counter !=, which accepts +2/+50; its 'exactly one apply receipt' comment has NO receipt assertion. Replace with full exact+1 counter map and F1-style record/history/closure/plan/receipt/prior-state comparisons bound to the actual single201 result (all 49 200 bytes identical). Populate pre-existing entries so prefix equality is meaningful.
TestReplanApplyCompeting's different keys on SAME plan are sequential, not concurrent. Run two actual concurrent different keys, assert exactly one201 and one409 plan_already_applied. Existing two concurrent plans must assert loser code stale_plan, exactly one winning plan applied, clean losing plan/no losing receipt, complete once-only state delta. Preserve input/ref order and no partial moves.

F3 — closure availability/explain truth, not inferred boundaries (R346/R348).
Current AvailabilityExplain samples three capacity/no_overlap values with false no_overlap and one false/true; NO true/true case, despite '4-rule' claim. Comments say 'overlap-true' when no_overlap is false; fix terminology. Use one applied closure and actual available baseline, assert all four capacity/no_overlap conjunctions, table/rule fixture order, policy version, available IDs/options exactly match conjunction, capacity independent even closed. Do not accept null where allocated [] is required.
Add SERVICE availability tests of half-open start/end adjacency, a short closure strictly inside a booking/slot occupancy interval, absolute-offset equivalent instants, pair shared-member exclusion and other-restaurant noninterference. Planner/helper adjacency alone does not prove closureOccupancy actually feeds availability. No-explain shape stays exact; existing grid/detail unchanged.

F4 — every closure write and failed-key reuse (R347/R4/R51).
Current ClosureWrites correctly checks create/PATCH/move/adopt 409 and a combined export snapshot, but only create key reused, singles only. Add per-operation pre/post complete export equality and exact code, genuine failed move/adopt key reuse to201 with feasible bodies; a pair selector including closed member must reject. Pin eligible adjacent success and a short-inner closure so checking just candidate start would fail. Keep cutoffs/policy validation precedence honest and avoid ordinary bookings being the real conflict. Closure check for series amendments is later W with real integrated A; do not invent a stub.

F5 — series/immutable replay/isolation bindings (R337–339/R377–378).
Current Series covers two affected series but checks only revisions/one anchor event/partial flags. Capture all members/series/history/counters before apply; assert each affected series exactly+1 regardless moved member count, full permanent flags/scheduled dates/anchor/index/interval/identity retained, every moved book exact reassigned, each unmoved byte-equal, one FULL restaurant counter delta, populated unrelated series unchanged. Existing true exception remains true and false remain false.
AlreadyApplied: capture baseline BEFORE the failed fresh-key request and BEFORE EACH replay/body-reuse request, assert complete export unchanged. Prove real later edit occurred (current != original apply response and counter advanced); replay original201 bytes after that mutation. Demotion baseline similarly before replay/fresh403. Detachment mutate actual nested table_ids and nested accepted_terms capacities/hours, guard mutation applied and compare COMPLETE pristine stored export/replay. No JSON-decoded stand-in mutation or post-first-replay baseline.

F6 — evidence/provenance and negative guard honesty.
Actual retained commands.log contains ONLY source gates (ends13:01:57); live-commands.log contains driver/stop/rm, NOT docker run/inspect/health/caps or deleted image IDs. Build logs prove initial ENOSPC and retry image creation, but claims of prior image deletions and current container identity/caps aren't verified by these files. Do NOT reconstruct historical outputs. Qualify unretained prior claims; fresh r2 own container can reuse unchanged final image (if product unchanged), record actually executed literal run/inspect with FULL .Id/.Image/.Name/Env/Ports/caps and health200 exact body/charset + stop/rm absence. Record actual current image ID read from Docker. Keep all current files/logs untouched, use new r2 paths.
Driver explicit if/else FAIL counting is an improvement, but login labels200 merely on nonempty token (HTTPstatus absent), and Python-failure branches PRINT private diagnostic content via cat. Require actual login200/nonempty token, private stderr/diagnostics (public only safe name/status/count), export helper status200+valid envelope for atomic comparisons. Extend live checks to reassigned history/terms, explain closure independence, proper already_applied vs true stale after another write, rejected write rollback, original apply receipt after later real mutation. Run a forced wrong expectation and prove counted FAIL + nonzero exit with separate retained log. Never manufacture pass counts or claim this guard without running it. Cleanup only YOUR owned disposable objects; no shared cache deletion. Existing ENOSPC first attempt stays preserved.

Exact source gates CWD /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode/stage-4:
gofmt -l internal/service
go vet ./...
go test -count=1 ./...
go test -race -count=1 -v ./internal/service -run '^TestReplanApply|^TestClosure|^TestReplanPreview'
go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-R2/tablekeeper-r2 ./cmd/tablekeeper
If sandbox gcc unavailable, retain actual CGO_ENABLED=1 targeted race failure; host repeats race. No UI/supplied harness/validator claim. Docker rebuild only if product bytes change; tests-only repair can reuse unchanged image. Fresh driver/lifecycle/guard logs with actual argv/absoluteCWD/UTC/effectiveexit/raw outputs, private0700/0600. Foreground complete this turn; ONE final DONE/BLOCKED report fullbase/ownedpaths/F1–F6 effective namedproofs/actual counts + evidencepaths + deviations retained or unretained + elapsed/usage. No acceptance claim or acknowledgment-only turn. Components: replan_engine, closure_store, reservation_engine, history_engine, series_engine.

## S4-A residual proof correction — shared55 in progress

R2 report529416de's two-file source scope, real cutoff workflow, real NY fold, exact gap, replay baseline and live-response isolation improvements are retained. Host21 amendment races/full normal/vet/build pass; no product defect established. Same-base test-only correction50a7fcd8-7ede-402f-b85c-c4532cfb2ec0 closes the remaining no-op cutoff setup, genuinely blocked stale case, nonoccupancy-over-occupancy precedence and race/receipt/history bindings. No acceptance/commit/reset; R2apply56 continues independently.

## ACTIONABLE S4-A/shared55 narrow residual proof repair

Seatright-OMP backend implementer, mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md. Continue SAME dirty worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp, unchanged full base727eae17559928196cc7f3f5c7dc8967b0d7088e. No reset/Git/new item. Shared55 stays in_progress; no acceptance/commit yet. Own ONLY NEW stage-4/internal/service/series_amend.go + series_amend_test.go. Product core is unchanged and looks correct; edit TESTS ONLY unless a real reproducible product defect is established. OpenCode owns R2 elsewhere; do not touch router/existing/validator/model/web/Docker/probes/PLAN/RUNLOG/earlier stages.

Coordinator verified report529416de: source scope and earlier trees unchanged, genuine CutoffAndPolicy old-0→adopt10080→latest0 cutoff409 is effective; real fold03:30→01:30 + absolute end, gap exactcode/atomicity, first replay baseline after mutation before replay and live response isolation/export stable are improvements. Actual r2 fullnormal43.059s/race21tests38.275s/smoke ownPID raw evidence is retained. Host21 scoped races pass; full regression/vet/build recorded separately. Keep ALL these effective assertions. A few claims still exceed the actual tests. Complete this narrow residual correction; don't rewrite unrelated effective tests or repeat Docker/UI.

P1: TestSeriesAmendNoOp lines~340 calls 'carry amend' at19:30 when every eligible record is ALREADY19:30. This is a no-op, so it NEVER adopts10080: stored cutoff remains0. Thus the test proves off-grid bypass only, not claimed cutoff bypass. Start at a DIFFERENT valid clock (e.g.19:00), publish10080, perform real amendment from_index0 to19:30 under old0; assert clock/revision changed and anchor terms10080, and anchor is genuinely inside stored7day cutoff. Then publishslot60 (same clock off-grid), identical19:30 from0 must201 with unchanged complete nonreceiptstate. Use exact BEFORE baseline, preserve prior receipts; do not fake metadata by direct writes.
TestSeriesAmendHugeStaleBeforeCutoff currently tests huge-stale BEFORE the real amendment adopting10080. Move the huge-stale assertion AFTER that real adoption and use matching-current revision same real targetclock to prove cutoff409 on exactly that candidate. Assert complete export unchanged for invalid/stale/cutoff failures. Correct any comments claiming the future record was already blocked.

P2: TestSeriesAmendPrecedence still has brainstorming comments ('member2 invalid is impossible', 'member2 succeeds on clone') and no actual later validation failure. Independent member1-only409 does NOT prove nonoccupancy beats earlier occupancy. Construct both failures in ONE real service: index1 ordinary-valid candidate conflicts at20:00; publish a policy effective index2 date closing that weekday so index2 same20:00 returns422 outside_opening_hours. Prove them separately using real PATCH of index1→20:00 (409 table_unavailable) and index2→20:00 (422 outside_opening_hours), each export-identical. Then AmendSeries from1→20:00 must return the later422, not earlier409, with complete export identity. No product stub/forged series. Existing distinct nonoccupancy index-order grid-before-closed test good, but add pre/post export equality and independently pin later closed error so its competition is effective. RollbackAndReuse claimed exactcode but only409; pin table_unavailable.

P3: Concurrency (lines904–1052) comment claims FULL counters/scopedreceipt/loser-no-receipt/prior-preservation, but actual code ends WITHOUT ANY receipt assertion and only scalar r1counter. SameKey50 has receipt count+priors but no newreceipt binding and no COMPLETE winner projection/history prefix/entry assertion. Close those precise omissions on BOTH real race baselines:
- compare entire counter map with only r1+1 and exact keys;
- FULL public projection of stored changed record==winning201 reservation, exact identity/owner/party/tables/terms/end preservation or new-clock expectations; target rev+1, old history prefix byte-equal and exactly one new Changed with seq/rev/at/terms/startsLocal From/To matching pre+winner;
- unchanged series fields/memberflags/schedules except seriesrev+1, all other series/records/histories and namespaces identical (no extra keys);
- exactly one new receipt with owner=userID, methodPOST, EXACT /series/{sid}/amend, winning key, canonical winning body, status201, raw stored response identical winning201 serialization; loser scoped key ABSENT; all prior receipt values unchanged.
Populate unrelated entries when needed to avoid vacuous assertions. Keep actual1×201+1×stale409 and1×201+49×200/bodyidentity checks. Do not implement comparisons by calling the production mutation helper or deleting expected namespaces.

P4: CommitMetadata similarly has only method/status/path-CONTAINS-sid for new receipt, no owner/key/canonical body/raw binding; it doesn't compare untouched records/histories (unrelatedseries alone is less than whole state). Strengthen exactreceiptbinding and preserve all unmodified stored records/histories/priorreceipts; capture affected series then require EXACT same fields except revision+1. Pin actual accepted duration using parsed absolute ends minus starts (CutoffAndPolicy currently only checks starts/end nonempty) and full old terms serialization, rather than claiming elapsed or full term snapshot checks that are not present. Keep separate per-record From/To/seq/revision and frozen-prefix tests.

P5: NoOp/EmptyEligible and Boundaries must not claim complete shape/prefix proof beyond assertions. NoOp's namespace loop omits plans/closures even though they are now real allocated maps; use all namespaces except receipt, or exact expected envelope. EmptyEligible preserves old receipt values as well as count. Count12 boundary test presently compares earlier records but not earlier histories: add unchanged history assertions or qualify that narrow claim. Detachment already proves full export/replay; its fresh GET 'unchanged' check onlyoccurrenceslen2, revNowunused: compare freshly serialized actual GetSeries to pristine201 FULL response (or qualify/remove redundant weak assertion). Fix stale NY end comment01:00→02:00, remove obsolete brainstorming and unused variables, preserving assertions.

Evidence: r2 run.sh executes arguments correctly, but ARGV '$*' loses quoting; retain a runnable exact -run command or document this display limitation (regex passed one argument). Smoke raw200/charset/15B and kill143 already verified; do not rerun it for test-only edits. Keep all existing r1/r2 logs untouched; fresh r3 scoped/normals logs with actual argv/absoluteCWD/UTC/effectiveexit/raw output. Report only assertions actually present; no 'full-map' or 'exact scope' based only count/substrings.

Exact gates CWD /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp/stage-4:
gofmt -l internal/service/series_amend_test.go
go vet ./...
go test -count=1 ./...
go test -race -count=1 -v ./internal/service -run '^TestSeriesAmend'
No product-build/Docker/UI/harness repeat if product unchanged; r2 binary/ownedPIDsmoke stand separately. ONE foreground final DONE/BLOCKED report shared55/fullbase/onlyownedpaths/P1–P5 named effective proofs/counts/exits/evidencepaths and actual remaining deferredW/I interactions. No acknowledgment-only turn or acceptance claim. Usage/elapsed when visible. Components: series_engine, reservation_engine, history_engine, closure_store.


## S4-A accepted core and independent S4-D queue

Shared55 finalreport305c6374 accepted: author ec3253596130e4542a9ab693483647b1526f5340 mergeddf703c2b4bc155259f9ddf503ff7db2cf006d8c4. Hostfinal4race4.567s0, prior21race22.080s0, integratedvet/fullnormal0. Twoownednewfilesonly/no conflict/productdefect/coordinatorproduct/testedit. W alone wiresHTTP+repair interactions; I laterimport. Stage4unaccepted.

Shared58/S4-D moves fromOpenCode toOMP whileR2active. OneNEWdonorprobe, three immutable acceptedoldsourceprocesses; no R2 dependency or stage4destinationclaim. FixedcleanOMPworktree resets at dispatch to actualfullintegratedmetadataSHA.

ACTIONABLE S4-D — genuine stage-1/2/3 donor preparation (OMP)
Role: backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-omp.md. Fixed worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-omp. This starts AFTER the coordinator commits/merges S4-A and resets your clean fixed worktree to the full base supplied in this message. No state-changing Git commands by you. Owned ONLY one NEW executable file stage-4/probes/stage4-donor.sh. Do not edit Go/tests/model/state/validator/router/web/Docker/RUN/PLAN/RUNLOG/locks/any existing probe/accepted stages1–3. No destination stage4 import or acceptance claim.

Why this independent queue item now: your S4-A two-file service core is accepted and integrated; router and actual repair interactions remain W/I. OpenCode is still correcting R2 and must retain its active worktree. S4-D moves from the planned OpenCode queue to OMP to keep the independent old-source lane moving; the shared plan records this reassignment. This donor needs no R2 or stage4 endpoint and no invented stub.

Freeze interface:
sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR
Exactly FOUR positional arguments, three genuine independently running immutable accepted older services plus private directory. The script resets and mutates those disposable sources; document this destructive fixture-only lifecycle. POSIX sh + curl + python3 stdlib, no jq/new dependency. curl connect-timeout5/max-time25, validate URL scheme/host and writable private output, umask077. Only OUT_DIR writes; exports/manifests0600, dirs0700, tmp beneathOUT own cleanup, diagnostics retained beneathOUT/diag. All Python stderr private; stdout only safe test names/statuses/counts, no credentials/tokens/bodies/exports/tracebacks/xtrace.

Provenance ENV schema: STAGE1_IMAGE/STAGE1_CONTAINER/STAGE1_PORT/STAGE1_CID and same STAGE2/STAGE3 names. IMAGE is ACTUAL deployed full sha256, CID actual full container.Id, name/port from inspect. reviewed_revision and stage_tree below are fixed accepted Git IDs, not current stage4HEAD:
stage1 revision b298700f790c166cf7ce8d98d731c80093ecb9af/tree8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c
stage2 revision8812cdeaaa993cd944493c654e51d355cdd6b676/tree9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f
stage3 revisione13272d90213be0914211ae6f6f0bfd5888900ff/treec783f9e08522a04a62bb11f9a0e3c485d077684f
Known accepted source folders at your worktree remain byte-immutable. Reuse only proven images from your own daemon whose build provenance+folder tree agree; otherwise build ONLY older folders. Never use stage4 as old source. No prune/sharedbase/runner/unowned image removal. If disk blocks build, report quantified infrastructure blocker and complete source/syntax/guard work possible.

Output OUT/stage1,stage2,stage3/{export.json,manifest.json}; opaque export is exact HTTP file bytes, never rewritten. Existing accepted stage-3/probes/stage3-donor.sh can be invoked read-only to produce stage1/2 donors (forward its documented provenance environment); alternatively reuse its logic inside the NEW script, without editing it. Retain its mature real5/8receipt assertions, owner-record/list/export/fullprojection/raw-binding/sabotage checks and genuine pair batch. Stage1/2 original manifest_version1 schema remains compatible.

Stage3 manifest_version1 EXTENDS the same schema:
source {stage:3,reviewed_revision,stage_tree,image,container,container_id,port}
fixture {date,past_seed_date,restaurant FULL original fixture incl hours/capacities/combinable/managers,seeds full reset request records}
users [{id,email,password,display_name,tokens:[two distinct live tokens]}] private only
records {ada_list,bea_list,by_reference} full CURRENT owner public list+GET matching stored export public projection, singleton table_id iff exactly1, modern revision/complete six-key terms
receipts [{key,user_id,method,path,body parsed,body_raw exact sent,response parsed,response_raw exact first201 received,status:201}], scoped record matches export canonical Body and raw Response; single framing newline allowance only if documented, genuine source normally none.
pending_retry {role,reference,key,owner,method,path,body,response,current} genuine committed receipt+same-ref retry after current mutation, no fabricated receipt.
failed_keys {reused:{...},absent:[{...}]} distinguish actual failed key reuse from a genuinely absent unclaimed key.
metadata {policies: FULL export policies map,histories:FULL per-reference frozen entries,series:FULL map,restaurant_revisions:FULL map}; captured before snapshot divergence, no stage4 plan/closure data invented.
post_snapshot_write {reference,excluded_from_export:true}, current_lists_before_post_snapshot_write:true.
Additional fields only if documented, future I2 consumes this schema. Bind all named reference/owner/map sets exactly; do not conflate current records with immutable old receipt bodies. Full token/user metadata in export versus private manifest, no plaintext keys invented into export.

Stage3 real workflow via HTTP:
- genuine fixture manager plus two owners, two live tokens each+hash-password logins, owner cross-access404/no token401.
- include past/offgrid/above-maxima/cancelled original-fixture0 seeds (full retained fields/terms/history; no invented business validation for historical producer seeds).
- publish two valid dated policies with supersession (real manager POST201 on actual /restaurants/id/policies, versions/order/frozen snapshots), unchanged public detail.
- create one canonical pair anchor and a second separate singleton anchor using selected policy capacities/date/duration; adopt TWO real series201, original anchor identity/history/accepted terms retained.
- individual PATCH one generated member → exception=true; cancel that same member → flag remains true; cancel another unexceptioned generated member → no new exception.
- real idempotent POST /reservation-moves affecting members from both series, current records rev/history each+1, restaurant fullmap+1 exactly once, each affected series+1 exactly once, schedules/flags retained; all inherited namespaces detached. Avoid own deliberate conflicts by appropriate dates/tables/clock/party under actual policies.
- capture raw FIRST201 receipt for each keyed successful write on FOUR actual paths: /restaurants/id/policies, /reservations, /series, /reservation-moves. Declare exact expected counts from the named workflow and require equality, never only >=. Stage1 exactly5, stage2 exactly8; stage3 count follows a documented deterministic write ledger.
- after capture perform REAL mutation/cancel on a booked record; require current differs from original receipt; replay ALL stored receipts200 raw-byte exact with export unchanged. Publication/series/moves original responses remain old after evolution.
- one genuine failed idempotent key, required expected failure/no record/export change, then reusable with successful body201 + same body replay200 bytes. Keep one genuinely absent failed key for future destination retry proof.
- create a pending retry reference with original receipt; source has only one corresponding booking. All current owner lists+GET/history/decision/series views agree with entire export metadata and manifests at capture.
- save original export then make one actual post-snapshot booking: present in live source, absent saved bytes; sha file remains identical; preserve live source and artifact distinction.

Copied-artifact validator required for all THREE donors, accepts genuine files, rejects labelled COPY corruptions while originals remain untouched: changed identity/local clock, raw-only request-body mismatch, whitespace-only response raw mismatch despite equal parsed JSON, and stage3 history/series metadata mismatch. Exact raw/parsed/canonical binding, exact owner/ref/receipt sets, expected revision/terms shapes, declared pair order. Never pass a sabotage copy off as producer data.
Guards: unreachable-source subprocess countsFAIL/ABORT nonzero; wrong shape/reset-status guard honestly labelled; final script sabotage flag STAGE4_DONOR_SABOTAGE=1 injects a wrong expectation through its real counter/exit path, must countedFAIL+nonzero. No hardcoded success markers or vacuous conditions.

Acceptance commands:
CWD worktree root:
sh -n stage-4/probes/stage4-donor.sh
python3 stdlib compile all embedded static heredocs (read source; no bad quote-splitting)
If needed ONLY docker build -t tablekeeper:s4-d-stage1 stage-1; corresponding stage2/stage3.
Own new containers tk-s4-d-src1 --cpus2 --memory2g -e PORT=9185 -p9185:9185, src2PORT9186,src3PORT9187 (normal properly separated flags, no invented APIs). Document literal argv/cwd; inspect full .Id/.Image/.Name/Env/ports/caps, health200 exact15Bbody+charset, inject actual provenanceenv.
sh stage-4/probes/stage4-donor.sh http://127.0.0.1:9185 http://127.0.0.1:9186 http://127.0.0.1:9187 /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-omp/S4-D/donors
Guard each failed attempt fresh evidence path; own stop/rm per container, absence proven; images may stay. No Go/UI/harness reruns for probe-only scope.

Evidence /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-omp/S4-D/: executable actual lifecycle/gate driver, raw per-attempt quoted argv/absoluteCWD/UTC-start/end/effectiveEXIT, command/provenance/donor/validator/guards/cleanup logs, private artifacts0700/0600. Every claim needs retained raw evidence; preserve failed attempts separately, never overwrite then claim preserved, no reconstruction. Counts describe actual assertions vs phase markers honestly.
Report ONE foreground DONE/BLOCKED item/card/fullbase/owned-file Gitstatus/requirement→effective named assertion/commandscounts/exits/evidencepaths/provenance/cleanup/gaps/elapsed/usage. No acknowledgment-only turn. No destination import/browser/currentstage4packaging/acceptance claims. Components: snapshot_store, receipt_store, history_engine, series_engine, migration_gate.


## S4-R2 accepted; S4-W serial HTTP integration assigned

S4-R2/shared56 finalreport7ad152c0 verified sevenownedpaths/restoredpreviewtest. Author 5efc4933c68133da514ffce694a33007f936e233 merged5585bbac2ff9b023adffda65bc5248397cab7f52; host19applyclosure races0/service40.818s, formatting/vet0; combinedcandidatefullnormal/vet0. Realdriver34/0 andsame-scriptforcedguard33/1EXIT1, fullactualownerhistory/planID/seq2/frozenv0/recordtimes/laterPATCHreplayexportidentity verified; runinspecthealth/stopauto-removal retained. Coreunchanged/no newproductdefect/coordinatorproducttestedits. Helperseqclaim qualified by independentSuccess/liveproof. Shared56completed for sourceworkscope, stage4stillunaccepted.

S4-W/shared59 assignedOpenCode after cleanreset to existingfullintegratedmetadata base atdispatch. ONLY narrowhttp.go/appendhttp_test.go/NEWseries_amend_integration_test.go. RealA/R2dependenciesnowavailable; no router writer overlap. OMPshared58Dactive untouched. Formalreview/nativeimport/G2/P remainlater.

ACTIONABLE S4-W — exact series-amend HTTP dispatch and real repair/amend integration (OpenCode)
Role backend implementer; mandate /home/nryn/work/seatright/runs/tablekeeper2/result/mandates/seatright-opencode.md. Fixed worktree /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode, clean new full integrated base supplied at dispatch. Host alone performs reset/Git; you no Git mutations. Owned ONLY stage-4/internal/service/http.go (narrow exact dispatch), http_test.go (append tests, never overwrite inherited), NEW series_amend_integration_test.go (real service+HTTP integration tests). Do NOT edit AmendSeries/replan_apply/replans/engines/shared helpers/model/state/control/validator/availability/reservations/moves/series existingtest/web/Dockerfile/RUN/PLAN/RUNLOG/probes/stages1–3. If a real engine defect appears, preserve reproduction and report it to coordinator instead of editing outside ownership.

Dependencies actually integrated:
S4-A author ec3253596130e4542a9ab693483647b1526f5340/mergeddf703c2b4bc155259f9ddf503ff7db2cf006d8c4: real Service.AmendSeries(token,seriesID,key string,raw []byte) Result; amendSeriesLocked pure callback; original scheduled dates/suffix eligibleconfirmed nonexception/no-op bypass before cutoff+newpolicy; all nonoccupancy errors before final working occupancy+closureBlocks; Changed history per realmember, once-only series/restaurant, immutable receipt/counters. Host21races+final4/integratedfullnormalvet0.
S4-R2 author 5efc4933c68133da514ffce694a33007f936e233/merge5585bbac2ff9b023adffda65bc5248397cab7f52: real PreviewReplan/ApplyReplan and exact manager routes, current common conflictingReservation closure rejection and availability closure/explain. Host19 apply/closure races0/service40.818s, peerfullnormal0, realimage34/0+actualdriverguardnonzero. NewReassigned histories correct; operatorrepair preserves ALL old terms/time/party/identity, flags false retain, affectedseries once, counteronce. No unimplemented stub or fake apply.
Existing service.Handler()/serveRequest test helper in http_test.go is real HTTP handler; authHeader/errorCode/decodeBody/reset/login helpers exist. Use actual Idempotency-Key header and real JSON request bytes; don't call AmendSeries directly as substitute for route proof.

Product scope:
Add only exact POST /series/{nonempty-series_id}/amend route (no trailing slash/deeper/empty-id/wrong-method aliases), forwarding actual bearer token, actual header key, actual readBody bytes to AmendSeries. Preserve existing /series POST and GET /series/{id}, all restaurant/reservation routes/pages/static/health. Unknown/foreign series404 via engine for valid body/token; missing/invalid token401 for valid exact route. No GET-series owner masking, no new screen/route beyond required endpoint. New body global validation 422/malformed400/missingkey existing wrapper semantics, positive integral expected_revision including huge stale409, integer from_index, exactHH:MM. Canonical unknown-field identity remains Idempotent behavior; full raw response replay200 byte exact. Function signatures/terms/hooks unchanged. Do not perform engine validation in router.

Acceptance tests (R351–379 route + actualR377/378/R366 interactions):
Name new top-level TestSeriesAmendHTTP* in appended http_test.go and TestRepairSeriesAmend* in new integration file so exact gate patterns are stable. Preserve all inherited tests unchanged; may use existing fixture helper READONLY, no hardcoded fixture branch in product.
- Exact-path/method matrix including validPOST201, GET/PUT/PATCH/DELETE wrongmethod404, trailing/deeper/emptyID404, ordinary validGET series retains200/foreign404. auth no/bogus401, realowner201, otherowner and manager-nonowner404, unknown404 with validbody; header idempotency scope exact actual path.
- Invalid JSON400, missing key wrapper code, invalidrevision bool/string/null/fraction/zero/nonpositive422, missing fields/fromindexmatrix/clockformat422, huge positive integer stale409 before occurrence cutoff/field validation; stale expected vs current with valid input409, failures full export unchanged/no failedkey claim; unknown fields ignored in behavior but changed canonicalbody on committedkey409.
- Actual first201/currentseries shape+HTTPbytes equals storedscopePOST/exactpath/key/owner/body/201receipt, GETcurrentmatches first response, recordmetadata/history/countermaps/flag-schedules retained; samekey replay200 raw exact after real laterPATCH/cancel with current differing and pre/post replay export identity; committedkeydifferentbody409 atomic; SAME unclaimed failurekey genuine201reuse then200 bytes.
- Route no-op/emptyeligible201 preserve all namespaces/counters/history/revisions except correctly scoped single new receipt; don't pretend success entails change.

Genuine real repair→amend workflow, not direct stored-state edits/synthetic assignedTable seam:
1 reset real manager/owner fixture, ordinary create anchor + adopt real series (>=3) via HTTP. Save original record/history/terms/public timestamps, original adoption first201 receipt. Use ordinary timestamps future2027 valid grid and acceptedcutoff.
2 manager Preview + Apply closure over one generated member's original table/time to actually MOVE that member. Require preview changedassignment true and newtables differ, actualApply201. Pin full member identity/owner/party/acceptedterms/start/end unchanged, exactly rev+1/one reassigned/full From-To/planID/historyprefix; series once and every member scheduledDate/exception flags unchanged. GET series/owner lookup agrees; originaladoption receipt still old.
3 Real AmendSeries over suffix including repaired member via NEW HTTP route. It must keep the repaired CURRENT tables and original scheduled dates; real newclock adopts each resultingdate policy while old CREATED/Reassigned terms stay frozen. NewChanged followsReassigned, ordinary clock-only From-To, bookrev+1, series/restaurant once, no new exceptions. Bind all unchanged eligible/skipped members/unrelated state, complete countermaps and full current response.
4 Applied-closure failure through same common seam: choose another actual zero-move closure on repaired member's CURRENT table at the requested NEW clock (outside its currently occupied half-open interval). RealPreview/Apply201 with no moved members -> restaurant+1, no series revision drift; then actualseriesamend targetclock must409 table_unavailable with FULL export byte identity/no receipt. Required same failed key reused to an ADJACENT nonconflicting clock -> genuine201 +200 exact replay. Do not confuse the old closed table (member moved away) with the current repaired table.
Example admissible grid-safe world, adjust actual fixture arithmetic coherently: original19:00–20:30 t2, firstclosuret2 at19 forces membert1; secondzero-moveclosuret1 [20:30,21:30), requested21:00 conflicts, retry21:30 is adjacent/open until23. If newpolicy duration60 used, pin actual end–start60; prioraccepted fixture90/Reassigned90 remains frozen. Any policy publication bumps restaurantcounter, so apply saved plan BEFORE publication or refreshpreview; no stale bypass.
5 Actual individual PATCH exception + cancel on same member remains excluded; another unexceptioned cancelled member excluded; ordinary repair doesn't invent exception=true. If needed separate scenario to keep suffix realchange nonempty. No import because native stage4validator is nextI1; do not weaken it.

Source gates CWD /home/nryn/work/seatright/runs/tablekeeper2/wt/seatright-opencode/stage-4:
gofmt -l internal/service/http.go internal/service/http_test.go internal/service/series_amend_integration_test.go
go vet ./...
go test -count=1 ./...
go test -race -count=1 -v ./internal/service -run '^TestSeriesAmendHTTP|^TestRepairSeriesAmend'
If sandboxgccmissing record actualCGO_ENABLED=1 attempt+nonrace same targets; hostrunsrace. go build -o /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-W/tablekeeper ./cmd/tablekeeper.

Real HTTP smoke with actual new build:
worktreeroot docker build -t tablekeeper:s4-w stage-4 (DO NOT reuse old R2image: it lacksA/route). Own tk-s4-w --cpus2 --memory2g -e PORT=9180 -p9180:9180, actual full .Id/.Image/.Name/Env/caps/PORT inspection; health200 exact15Bbody+charset. Driver real manager/owner fixturecreate/adopt/previewapply/seriesamend currenttable/history/closurefail/reuse/replay/privacy; all HTTPstatuses+payloads asserted, explicitFAIL counter+nonzero guard through SAME driver, private diagnostic stderr. Capture original first201 byte files before mutation and baseline BEFORE replay, not marker-only proofs.
If Dockerbuild ENOSPC, preservefailure/disk and no unowned/sharedbase/runner/cache deletion. Complete source gates plus actual own binary HTTP smoke where possible; packaging image check can be recorded concretely infrastructure-blocked for laterP/reviewer. No human question. Stoponlyownedcontainer/PID, recordliteralcleanup/absence.

Evidence /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-opencode/S4-W/: actual executable driver+rawperattempt logs quotedARGV/absoluteCWD/UTC-start/end/effectiveEXIT/statuscounts/provenance/runinspecthealthcleanup; preserve failedattempts separately. Token/password/body/exportfiles0600 under0700private dirs, publicstdoutnames/status/counts only; no xtrace/privateerrorcat/body dumps. ReportONE finalforegroundDONE/BLOCKED sharedcard/fullbase/exactownedpaths/gitstatus/requirement→named REALtests/commands/counts/exits/evidencepaths/deviations/gaps/measuredelapsed/visibleusage; no acceptance claim or acknowledgment-onlyturn. No Docker/browser/migration/packaging claim unless actuallyverifiedinassignedscope. Components: series_engine, replan_engine, closure_store, reservation_engine, history_engine.
