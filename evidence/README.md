# Evidence

The run's record, beyond the code. The coordinator committed each stage's accepted review
here when it accepted the stage; the room transcript was added after the run. The full raw
evidence of every seat (every work item and every review round) stayed outside this
repository; what is here is the accepted trail.

| Path | What it is |
|---|---|
| [room-transcript.md](room-transcript.md) | The room's 371 text messages, oldest first: the owner's dispatch, every handoff, report and review, the operator's one message and the final report. Generated after the run from Band's CLI (`band room messages`, all pages) |
| [stage-1/](stage-1/) | The accepted stage 1 review (round 2 of 2) |
| [stage-2/](stage-2/) | The accepted stage 2 review (round 2 of 2) |
| [stage-3/](stage-3/) | The accepted stage 3 review (round 1) |
| [stage-4/](stage-4/) | The accepted stage 4 review (round 1, with its evidence reconciliation) |
| [stage-4-blocked/](stage-4-blocked/) | A superseded record, kept as history. Before the operator's clarification, the coordinator stopped the run with stage 4 "blocked" and wrote that outcome; this folder holds it and the audits it rested on. Stage 4 was then reopened and accepted. See FACTORY.md, Operator events |

## What a stage folder holds

Every `stage-N/` folder has:

- **`REVIEW.md`**: the reviewer's (ZCode's) report for the accepted round. It has the verdict, the exact revision reviewed, every check it ran with its result, the design pass and what it could not verify.
- **`checks/`**: the supplied isolated harness run at that revision: `report.json` and one log and count file per stage suite.
- **The design critique** from Antigravity (Gemini 3.8 Flash), which the reviewer verified point by point: `stage-1/design/design-critic-output.txt` and `notes/design-critic.log` in stages 2 to 4.
- **The reviewer's own runs**: independent API, upgrade, migration, browser and race checks. These are in `probes/` (stages 1 and 2) or `notes/` (stages 2 to 4).

Some folders hold more:

| Path | What it is |
|---|---|
| `stage-1/design/` | The 55 screenshots of the stage 1 design pass, at 375 and 1280 px, in light and dark, plus the design pass logs |
| `stage-1/staging/` | The reviewer's UI check, test and build logs |
| `stage-1/ARCHIVE.json`, `stage-N/MANIFEST.json` | The list of files the coordinator archived, with their sources |
| `stage-N/README.md`, `review.log` | The coordinator's summary of the archive, and the reviewer's command log |
| `stage-4/REVIEW-INITIAL-SUPERSEDED.md` | The reviewer's first stage 4 report. The coordinator found that some of its evidence labels were not supported and asked for a reconciliation at the same revision; this first report is kept, marked superseded |
| `stage-4/checks-initial-failed/` | A first harness attempt under parallel load, which failed with browser timeouts in stage 2's suite. The serial rerun in `checks/` passed (see FINAL-OUTCOME.md) |
| `stage-4/reconcile/` | The reviewer's reconciliation runs |
| `stage-4/COORDINATOR-ADOPTION-AUDIT.md` | The coordinator's audit of the reconciled review before it accepted stage 4 |
| `stage-4/review-sanitized.log` | The reviewer's command log, archived with two redactions (recorded in `stage-4/MANIFEST.json`) |
