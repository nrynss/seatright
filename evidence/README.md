# Evidence

This folder holds the run's record beyond the code. The coordinator committed each stage's
accepted review here when it accepted the stage. We added the room transcript after the run.
Each seat's full raw evidence, for every work item and every review round, stayed outside
this repository. This folder keeps the accepted trail.

| Path | What it is |
|---|---|
| [room-transcript.md](room-transcript.md) | The room's 371 text messages, oldest first. It covers the owner's dispatch, every handoff, report and review, the operator's one message and the final report. We generated it after the run from Band's CLI (`band room messages`, all pages) |
| [stage-1/](stage-1/) | The accepted stage 1 review (round 2 of 2) |
| [stage-2/](stage-2/) | The accepted stage 2 review (round 2 of 2) |
| [stage-3/](stage-3/) | The accepted stage 3 review (round 1) |
| [stage-4/](stage-4/) | The accepted stage 4 review (round 1, with its evidence reconciliation) |
| [stage-4-blocked/](stage-4-blocked/) | A superseded record, kept as history. Before the operator's clarification, the coordinator stopped the run and marked stage 4 "blocked". This folder holds that outcome and the audits behind it. The coordinator then reopened stage 4, and the reviewer accepted it. See FACTORY.md, Operator events |

## What a stage folder holds

Every `stage-N/` folder has:

- **`REVIEW.md`**: the reviewer's (ZCode's) report for the accepted round. It gives the
  verdict, the exact revision it reviewed and every check it ran with its result. It also
  covers the design pass and what the reviewer could not verify.
- **`checks/`**: the supplied isolated harness run at that revision. It has `report.json`
  and one log and count file per stage suite.
- **The design critique** from Antigravity (Gemini 3.8 Flash), which the reviewer verified
  point by point. Stage 1 keeps it in `stage-1/design/design-critic-output.txt`, and stages 2
  to 4 in `notes/design-critic.log`.
- **The reviewer's own runs**: independent API, upgrade, migration, browser and race checks.
  Stages 1 and 2 keep them in `probes/`, and stages 2 to 4 in `notes/`.

Some folders hold more:

| Path | What it is |
|---|---|
| `stage-1/design/` | The 55 screenshots from the stage 1 design pass, at 375 and 1280 px, in light and dark, with the design pass logs |
| `stage-1/staging/` | The reviewer's UI check, test and build logs |
| `stage-1/ARCHIVE.json`, `stage-N/MANIFEST.json` | The coordinator's list of the files it archived, with their sources |
| `stage-N/README.md`, `review.log` | The coordinator's summary of the archive, and the reviewer's command log |
| `stage-4/REVIEW-INITIAL-SUPERSEDED.md` | The reviewer's first stage 4 report. The coordinator found that some of its evidence labels lacked support and asked for a reconciliation at the same revision. The folder keeps this first report, marked superseded |
| `stage-4/checks-initial-failed/` | A first harness attempt under parallel load, which hit browser timeouts in stage 2's suite. The serial rerun in `checks/` passed (see FINAL-OUTCOME.md) |
| `stage-4/reconcile/` | The reviewer's reconciliation runs |
| `stage-4/COORDINATOR-ADOPTION-AUDIT.md` | The coordinator's audit of the reconciled review before it accepted stage 4 |
| `stage-4/review-sanitized.log` | The reviewer's command log. The coordinator archived it with two redactions, which `stage-4/MANIFEST.json` records |
