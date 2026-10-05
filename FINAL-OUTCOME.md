# Tablekeeper final outcome: all four stages accepted

The complete four-stage delivery is accepted and merged to main. ZCode accepted stage 4 in **formal round 1** at frozen revision **a37befe400bd0e5686af7b47dbe417ad4a433c4e**. The coordinator verified the reconciled evidence and merged the unchanged accepted product at **c8b19b40d6fb9c05df26b4b03695427566f70fb3**. The subsequent archive commit changes only root records and evidence, preserving all accepted stage folders.

| Stage | Accepted revision | Requirements met / not met | Strict isolated checks | Formal rounds |
|---|---|---|---|---|
| 1 | b298700f790c166cf7ce8d98d731c80093ecb9af | R1–R160 accepted; none known unmet | 120/120; claimed/highest 1; expected next-stage failure | 2 |
| 2 | 8812cdeaaa993cd944493c654e51d355cdd6b676 | R1–R199 accepted; none known unmet | 120/120 + 25/25; claimed/highest 2; expected next-stage failure | 2 |
| 3 | e13272d90213be0914211ae6f6f0bfd5888900ff | R1–R303 accepted; none known unmet | 120/120 + 25/25 + 7/7; claimed/highest 3; expected next-stage failure | 1 |
| 4 | a37befe400bd0e5686af7b47dbe417ad4a433c4e | R1–R382 accepted; none known unmet | 120/120 + 25/25 + 7/7 + 6/6 = **178/178**; claimed/highest 4 | 1 |

Every accepted report names its exact revision, mode isolated, and completed result. Permanent review archives: evidence/stage-1/ through evidence/stage-4/. The final [isolated report](evidence/stage-4/checks/report.json) and [authoritative reconciled verdict](evidence/stage-4/REVIEW.md) are committed.

## What review changed

Stage 1 round 1 found credential refusals displaying bearer-token jargon. The frontend fix uses “Email or password is incorrect.” for the login refusal and preserves other errors; round 2 accepted it.

Stage 2 round 1 required four design corrections: blank signed-in names retain authenticated navigation; viewport measurements follow settled reveals; singleton summaries show human table names; pair rendering follows declared canonical order. Round 2 accepted those fixes, with browser closure 121/0, states 132/0, measurements 59/0 and no critical design finding.

Stage 3 passed formal round 1. Independent API 217/0, inherited stage-2 checks 346/0, migration 51/0, both true older-document upgrades 19/0 each, UI 90/90 and the full race rerun passed. The initial default 10-minute race timeout was followed by a successful 40-minute-timeout run (575.394 seconds); the initial timeout output was not separately retained and is qualified in its archive. No critical design finding remained.

Stage 4 passed formal round 1 with no CHANGES verdict or product repair after freeze. Before formal review, one bounded native fix resolved B1 (valid evolved zero-move-apply snapshot rejected) and B2 (preview receipt lacking plan_id accepted). Formal review independently confirmed valid import 204 and malformed import 422 with byte-atomic retention.

The initial stage-4 ACCEPT report contained unsupported evidence labels. The coordinator withheld adoption and requested reconciliation at the same frozen revision. Fresh standalone UI, strict receipt binding, original-byte migration and offline checks closed the evidence gaps. Reconciliation completes formal round 1; it is not another formal review round. The inaccurate initial report and failed attempts remain preserved and labeled superseded.

## Final stage-4 verification

- Unmodified supplied isolated harness: **178/178**, exit 0, exact frozen revision, highest/claimed 4. The first parallel-load attempt had five stage-2 browser timeouts; the serial clean-directory rerun passed. Both reports are retained.
- Source: gofmt clean, vet/full Go suite/build exit 0; full CGO race exit 0 in the Go container, service 1078.529 seconds. UI ci/check/build and both driver syntax checks passed. Original UI 98/2 under parallel load is preserved; fresh standalone UI proves **100/100**, exit 0.
- Independent API: stage 4 **69/0**, inherited stage 2 **346/0**, inherited stage 3 **217/0**.
- Browser/design: closure/reveal **121/0**, states **132/0**, measurements **59/0**, three genuine same-document upgrades **19/0 each**. Phone/desktop and light/dark/system states pass; held-pair contrast 7.611/9.915; design critic: no critical finding.
- Reconciled migration **134/0**: genuine stage 1→4 **23/0**, stage 2→4 **19/0**, stage 3→4 **32/0**, evolved stage 4→4 **60/0**. Original exports are imported unchanged, original first-201 bytes replay exactly after real mutation, and export bytes remain stable around replay/reimport. Native state includes an applied repair, stale unapplied preview, real clock amend of the repaired member with dated terms, exception-then-cancel and mixed batch.
- Fresh offline internal-network gate **31/0**, exit 0: authenticated policy/pair/series/repair/clock function, preview binder, six raw-equal replays, frozen metadata, typed HTML/JS/CSS, five distinct typed fonts, actual refused egress and cleanup. The earlier network-none functional gate stands separately.

These are separate measured checks, not a count of requirements or all factory tests. E2 combines one ingestible forgery rejected by the strict evidence binder with six atomically rejected imports; it is not seven direct binder invocations. E4's forced-wrong proof is a direct failing binder control, not a separately observed forced-failure subprocess. The adoption audit retains these limits.

## Blockers and gaps

**No acceptance blocker remains.** ZCode found every formal known issue nonblocking. The dedicated stage4-import.sh convenience probe is absent; independent actual multi-process migration discharges the older-source and native portability requirements. No manager or series browser UI is claimed or required.

Implementer Docker ENOSPC and missing-GCC attempts are qualified infrastructure failures; the reviewer built/tested the actual delivery image and full race suite. The earlier blocked disposition was superseded by the operator's clarification that the three-round limit counts formal CHANGES verdicts, not integration audits. Its preserved report is evidence/stage-4-blocked/FINAL-OUTCOME-PRE-ACCEPTANCE.md; it is historical.

## Time and usage per seat

Aggregate foreground/compute totals were unavailable. RUNLOG records UTC handoffs, reports, audits, retries, blockers and integrations. Durations below are reported items, not stage totals or billed compute. Concurrent items must not be summed into wall time.

| Seat | Stages 1–3 | Stage 4 recorded time | Tokens / cost |
|---|---|---|---|
| Codex coordinator | Per-item UTC entries; aggregate unavailable | Final audit, merge/archive in RUNLOG; aggregate unavailable | Unavailable throughout |
| OpenCode backend/probes | Per-item reports; aggregate unavailable | P1C ≈10 min + correction ≈25 min; guide ≈15 min + correction <1 min + final guide <2 min; other items in reports | Unavailable throughout |
| OMP backend/state | Per-item reports; aggregate unavailable | Native I1 attempts ≈23/20/35 min; bounded I1F fix ≈35 min; other items in reports | Unavailable throughout |
| Grok frontend | Per-item reports; aggregate unavailable | G2A 14:10:46–14:29:00Z; correction 14:42:11–14:53:31Z; G2B1 15:23:58–15:39:16Z | Unavailable throughout |
| ZCode reviewer | Stage-1 final round ≈45 min; stage-2 final round ≈31 min; stage-3 ≈37 min | Formal round 1 ≈41 min + reconciliation ≈27 min; delivery probes ≈12/14/15 min separately | Unavailable throughout |

Formal stage-4 dispatch reached ZCode about 31 minutes after operator clarification, within the requested three-hour aim. No estimated time is presented as a usage counter.

## Delivery

Use [stage-4/RUN.md](stage-4/RUN.md) for the complete current service. All four folders remain immutable at accepted tree IDs:

- stage-1: 8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c
- stage-2: 9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f
- stage-3: c783f9e08522a04a62bb11f9a0e3c485d077684f
- stage-4: 0e3d946a2b62ed4ea8dbe0c954472c8796c42094

No stage-5 folder was created. Committed review logs are sanitized and bound by original/archive hashes and redaction counts. Private credentials, tokens, snapshots, binaries and browser media remain outside the committed archive. The four-stage task is complete.
