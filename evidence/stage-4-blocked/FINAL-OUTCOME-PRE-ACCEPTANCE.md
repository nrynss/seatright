# Tablekeeper final outcome: stages 1–3 accepted; stage 4 blocked

> Historical outcome superseded by operator instruction46964814 on2026-10-05T19:07:05Z: stage4 reopened for one native fix attempt then first formal review. Three-round limit counts formal ZCode CHANGES verdicts, not integration audits. See active plan and RUNLOG; no stage4 acceptance yet.

The requested four-stage run stops at stage 4. Highest independently accepted stage: **3**. There is **no accepted stage-4 revision** and no formal isolated stage-4 acceptance run. Accepted earlier folders remain immutable. The coordinator did not merge stage-4 product changes to main; they remain in the integration candidate, with the failed native import item preserved separately.

## Accepted revisions and checks

| Stage | Exact accepted revision | Requirements outcome | Supplied isolated checks | Formal rounds |
|---|---|---|---|---|
| 1 | b298700f790c166cf7ce8d98d731c80093ecb9af | R1–R160 accepted; no known unmet stage-1 requirement at acceptance | 120/120; claimed/highest 1; stage-2 probe failed as expected | 2 |
| 2 | 8812cdeaaa993cd944493c654e51d355cdd6b676 | R1–R199 accepted, including legacy upgrade and pairs | 120/120 + 25/25; claimed/highest 2; stage-3 probe failed as expected | 2 |
| 3 | e13272d90213be0914211ae6f6f0bfd5888900ff | R1–R303 accepted, including policies, history, series and true older-source transfers | 120/120 + 25/25 + 7/7; claimed/highest 3; stage-4 probe failed as expected | 1 |
| 4 | None | Seating repair and recurring-clock components have substantial passing evidence; R380–R382 portability is incomplete and full R304 inheritance/whole-stage compliance is not accepted | No formal stage-4 isolated acceptance run; component probes listed below | No formal review; failed I1 and delivery items each reached three coordinator audits |

The accepted report JSONs were re-read at close: exact revisions above, mode isolated, state completed and highest_contiguous/claimed_stage 1, 2 and 3. Permanent review archives: evidence/stage-1/, evidence/stage-2/, evidence/stage-3/.

Stage trees verified unchanged:
- stage-1: 8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c
- stage-2: 9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f
- stage-3: c783f9e08522a04a62bb11f9a0e3c485d077684f

## What review changed

Stage 1 round 1 found credential refusals displaying bearer-token jargon. The frontend fix maps only unauthenticated login refusal to “Email or password is incorrect.” and preserves other errors. Round 2 accepted after all checks and independent probes were rerun: API186, export32, candidate139, browser76; UI56 tests; phone/desktop, themes, motion and design checked.

Stage 2 round 1 required four concrete design fixes: signed-in auth links, viewport reveal of results/form/confirmation and refusal/uncertainty states, one readable human table name, and pair-link/badge paint order. Round 2 verified closure121, browser states132 and measurements59; API346, migration38, true old-document upgrade19, UI82 and full source race passed. Selected-pair contrast7.611 light/9.915 dark. Critic concerns were measured against settled states; refuted claims and required repeated hooks were documented.

Stage 3 passed its first formal round. Independent new API217, inherited API346, three-lane migration51, both genuine older-document upgrades19 each, UI90, full race and design passed; no critical design finding. The first race attempt reached the default10-minute timeout; its output was summarized rather than retained. The40-minute-timeout rerun passed in575.394s; qualification remains in the accepted review.

## Stage 4 partial delivery

Integrated components implement deterministic global seating optimization, manager-only revision-bound preview/apply, half-open applied closures, reassigned history, atomic per-series clock changes on original calendar dates/current tables, once-only counters, immutable receipts, exact routing, and existing diner screens. Component source/full/race gates were run where appropriate; independent API/browser proofs cover R305–R379, subject to their recorded limits. These are component results, not formal whole-stage acceptance.

The final current probe passes **632/0** and the unchanged inherited stage-3 probe passes **245/0/0** on the same fresh host binary. O/P/Q/R run true50-way preview/apply and competing-key/plan races with full snapshot/receipt/history/counter comparisons and corruption controls. Host Bash readonly PPID failure was corrected; failed attempt retained. Guides accurately describe immutable original receipts versus evolved current records and the unavailable native portability.

Packaged producer: **45581bf3a2dd422b5700a33c3075e56471679971**, stage-4 tree **00a063ca63ecadee8f5f3ffa1df1512b2aa61571**. Reviewer-built image **sha256:c61a81282c70ae37c7d092f653aba2af3bbf4e8cf05520ea5bbdff5f65ab885b** has observed current632/0, inherited245/0/0 and HTML16/0. Frozen archived tracked bytes were independently compared to Git (163files). Default8080, overriddenPORT, bind,2CPU/2GiB caps and cleanup were inspected. R3 offline container startup measured0.256s from run command start, network none, authenticated policy/explain/book/adopt/repair/clock operations, raw receipt comparisons, typedJS/CSS/fonts and blocked egress were observed.

The reviewer’s whole **DELIVERY PASS is not adopted**. After the third evidence audit, its stored-plan/receipt checker still accepts actual mutations of receipt owner, canonical request, raw response and plan restaurant. Its mutation self-test merely checks that a changed copy differs from the original; it never invokes the checker to prove rejection. Shared67 is blocked for incomplete effective evidence. This is an evidence defect, not a demonstrated service packaging defect; passing observations remain individually recorded. No fourth correction or unnecessary rebuilding.

Live browser evidence: repair/upgrades31checks with144screens and six decoded recordings, actual accepted older source trees; recurring clock105checks with44screens and four decoded recordings. Phone/desktop, themes, reduced motion, original receipt versus evolved current lookup are verified. In-window cutoff refusal was not exercised by the recurring browser probe; backend probes cover it. Strict native roundtrip browser follow-up and native transfer remain blocked.

## Functional native-import blockers

S4-I1/shared63 reached its three-audit limit and is **not integrated**. Its host normal/full/race/build gates passed, and seven coherent corruptions were repaired. Two independently reproduced defects remain:

1. **Valid evolved state rejected:** create an unchanged considered booking, apply a genuine zero-move plan, then successfully PATCH party size. The original snapshot imports204; the actual evolved native snapshot imports **422**, expected204. Immutable apply receipts must survive later evolution. The unchanged-apply binder compares the historic receipt against the final evolved chain.
2. **Malformed native receipt accepted:** delete only plan_id from a genuine retained preview response. Import returns **204**, expected422, and replaces the destination. Missing-id lookup/orphan checks skip the malformed receipt.

Status-only repro results and audit are archived under evidence/stage-4-blocked/. Executable original repro and private genuine bytes remain outside Git in evidence/seatright-codex/S4-I1/. OMP’s five owned dirty paths remain preserved at base29d6e55a867e6d8dd0423bb3331b54cc5ffac62a; no author commit, merge, reset, fourth repair or weakening. I2 modern portability and G2B2 strict-native browser follow-ups could not proceed. A new transfer script alone would not resolve the importer.

## Infrastructure and limits

OpenCode’s separate Docker volume reached100%/zero available; some item image builds and the supplied stage-3 implementer smoke were infrastructure-blocked. Host binaries provided explicitly qualified HTTP proofs; no image acceptance was inferred from them. ZCode’s separate warm daemon built the actual stage-4 image successfully. Native importer defects, not this disk constraint, prevent stage-4 acceptance. No unowned cleanup, shared cache pruning or human input was used.

Reviewer delivery attempts include disclosed overwritten empty/intermediate named logs and failed expectation drivers. The sanitized archive distinguishes producer claims from coordinator dispositions. Private tokens, credentials, bodies, snapshots, videos and other secret artifacts were not committed.

## Measured time and usage

Per-seat total foreground/compute time was not available, so totals are not fabricated. RUNLOG.md records UTC handoffs/reports/audits/retries and each evidence report retains command windows. The following are reported **item durations, not stage totals**:

| Seat | Stage 1 | Stage 2 | Stage 3 | Stage 4 |
|---|---|---|---|---|
| Codex coordinator | UTC audit/integration entries in RUNLOG; aggregate unavailable | Same | Same | Same; final audit/closure entries recorded |
| OpenCode backend/probes | Per-item reports/logs; aggregate unavailable | Same | Same | P1C initial≈10min + correction≈25min; guide≈15min + bounded wording correction<1min; other item durations in reports |
| OMP backend/state | Per-item reports/logs; aggregate unavailable | Same | Same | Native I1 initial≈23min, correction1≈20min, correction2≈35min; not aggregate factory time |
| Grok frontend | Per-item reports/logs; aggregate unavailable | Same | Same | G2A recorded14:10:46–14:29:00Z, corrected provenance14:42:11–14:53:31Z; G2B1 recorded15:23:58–15:39:16Z; not summed foreground |
| ZCode reviewer | Final formal round≈45min | Final formal round≈31min | Formal round≈37min | Delivery initial≈12min, r2≈14min, r3≈15min; no formal S4 acceptance |

Token and cost counters were unavailable for **every seat and every stage**. Approximate reported item times are not billed compute time and concurrent seats must not be summed into elapsed wall time.

## Final disposition

Deliverable accepted service: stage-3/ at e13272d90213be0914211ae6f6f0bfd5888900ff, with immutable stage-1/ and stage-2/ alongside. Stage-4 work is retained in the candidate as partial, unaccepted work; its native correction remains in the owned worktree. Stage-4 root evidence is explicitly marked BLOCKED, never accepted. No stage-5 folder was created.
