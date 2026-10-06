# Seatright factory

Seatright is a five-seat dark factory for Band Desktop, set up like a rock band:

| Seat | In the band | Role |
|---|---|---|
| Seatright-Codex | Lead vocals | Coordinator: plans, hands out work, verifies, commits |
| Seatright-OpenCode | Bass | Backend implementer |
| Seatright-OMP | Drums | Backend implementer |
| Seatright-Grok | Lead guitar | Frontend and design implementer |
| Seatright-ZCode | Rhythm guitar | Independent reviewer |
| Antigravity (Gemini 3.8 Flash) | Monitor engineer | Design critic inside the reviewer's sandbox, heard only by the reviewer |

The whole run, all four stages, was started by **one dispatch message**. The seats planned,
built, reviewed and delivered it among themselves. The operator restarted seats whose
runtime dropped, and sent one clarification of the coordinator's mandate (below).

The complete factory, exactly as tagged for this run, is in [factory/](factory/)
(tag `run-tablekeeper2`, see [factory/SNAPSHOT.md](factory/SNAPSHOT.md)). This document
explains it; the files in `factory/` are what you need to stand it up.

## Seats

| Seat | Harness | Model | Runs in |
|---|---|---|---|
| Seatright-Codex | Codex (app-server) | `gpt-6.1-sol`, high reasoning | Host, Codex `workspace-write` sandbox, no container runtime |
| Seatright-OpenCode | OpenCode (ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker Sandbox `seatright-opencode` |
| Seatright-OMP | OMP (native ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker Sandbox `seatright-omp` |
| Seatright-Grok | Grok CLI (native ACP) | `grok-4.7` | Docker Sandbox `seatright-grok` |
| Seatright-ZCode | ZCode (ACP bridge) | ZAI Coding Plan `GLM-5.3-Flash` | Docker Sandbox `seatright-zcode` |
| Antigravity (critic) | Antigravity CLI (`agy`), headless | `gemini-3.8-flash-high` | Inside `seatright-zcode`; not a Band seat |

The mandates are in [mandates/](mandates/), one per seat, named after it. They are generic:
they describe how each seat works, not this problem, and were checked for task vocabulary
before the run. The problem, the stack and the paths arrive in the dispatch, which is the first
message in [evidence/room-transcript.md](evidence/room-transcript.md).

**Who does what**

- **Coordinator.** Turns the four specifications into a numbered requirements ledger
  (R1–R382 by stage 4) and small work items with owned files, interface contracts and
  acceptance checks. Sends each implementer a complete handoff in one room message, verifies
  each report against its raw evidence and its own re-run of the tests, commits each item
  with the implementer as Git author, integrates, sends a frozen candidate for review, and
  reports. It writes no product code.
- **Backend implementers.** OpenCode and OMP take whichever backend item is next: domain
  logic, the HTTP API, migration, packaging and API probes.
- **Frontend implementer.** Grok owns the interface and its design bar: one visual system on
  the stack's design tokens, 375 and 1280 px, every named state designed, motion that
  respects reduced-motion settings, an SVG floor plan, accessibility, and screenshot and
  recording evidence of every state.
- **Reviewer.** Checks a frozen candidate at an exact SHA, read-only. Runs the isolated
  harness and its own probes against the written requirements, then a design pass:
  screenshots and recordings at both widths, a critique from Antigravity, and its own browser
  measurements of every point it adopts. Returns ACCEPT or CHANGES.

## Any seat can be its own team

Antigravity is not in the room. The reviewer runs it as a command-line tool inside its own
sandbox, gives it the screenshots, and decides which of its points are real. Band coordinates
five seats; what happens inside a seat is up to that seat. Adding a second model to the
reviewer changed nothing in the room, the other mandates or the message flow, and Antigravity's
rejected points never reached the room.

| Review | Antigravity | Reviewer | Outcome |
|---|---|---|---|
| Stage 1, round 1 | 13 points, "not presentation-ready" | Measured each; disproved 12 (a contrast "failure" measured 5.71:1; one fix the specification forbids) | 1 blocker, login error copy; fixed by Grok in 10 minutes |
| Stage 2, round 1 | Points included sign-in links shown while signed in, results landing off-screen, table names drawn twice, a combined-table line through a label | Confirmed all four with browser measurements | 4 blockers; fixed by Grok in 32 minutes |
| Stages 3 and 4 | No critical findings | Own design pass agreed | Accepted in round 1 |

Antigravity's critiques for the accepted reviews are committed with them, in
`evidence/stage-1/design/`, `evidence/stage-3/notes/` and `evidence/stage-4/notes/`.

## Design choices, and why

- **The coordinator owns all Git.** History is the teamwork record. Each item is authored by
  the seat that wrote it: 25 commits by OpenCode, 17 by Grok and 16 by OMP, each naming its
  work item and handoff. The coordinator's 429 commits are integration merges and the room
  plan.
- **A different model reviews.** The implementers share a model; the reviewer runs a
  different model family on a different provider, and its critic a third.
- **Evidence, not claims.** Every seat writes the raw output of what it ran to
  `evidence/<seat>/`. A result that was never written down counts as not run. Accepted
  reviews are committed under [evidence/](evidence/).
- **The reviewer cannot change what it reviews.** The candidate is mounted read-only in the
  reviewer's sandbox; only its own evidence folder is writable.
- **Sandboxes for every seat that runs code.** Four VMs (2 CPU, 2 GiB each) with Docker, Go
  and Node. Each holds only its own seat's credentials, with a per-seat network allow-list on
  top of a shared baseline. The coordinator has no Docker access.
- **One message per handoff.** Band delivers one queued message per turn, between turns.
  Every handoff, however large, is one message that carries the full requirements.
- **Everything in the foreground.** A seat that ends its turn while work runs in the
  background can never report it, so long checks run in the foreground.
- **Frontend and backend as two streams.** Grok started from interface contracts and fixture
  data while the backend was built, then proved the screens against the real service.
- **The factory is pinned per run.** `prepare-run.sh` refuses a dirty factory repository and
  tags the commit it used; that tag is what [factory/](factory/) contains.

The access matrix, with the reason for every boundary, is in
[factory/AGENT-ROLES.md](factory/AGENT-ROLES.md).

## Results

Each stage folder is a complete service. On the supplied checks in isolated mode:

| Stage | Supplied checks | Review | Dispatch or previous acceptance → acceptance |
|---|---|---|---|
| 1 | 120/120 | CHANGES (1 blocker), then ACCEPT | 4 h 47 m |
| 2 | + 25/25 | CHANGES (4 blockers), then ACCEPT | 6 h 43 m |
| 3 | + 7/7 | ACCEPT, round 1 | 6 h 35 m |
| 4 | + 6/6 (178/178 in all) | ACCEPT, round 1 | 9 h 53 m, including the stop described below |

About 28 hours from dispatch to the final report. `python -m harness run --track tablekeeper
--stage 4 --mode isolated` claims stage 4; the operator re-ran it on a fresh clone of the
final commit with the same result. The supplied checks are a subset of the judged ones;
hidden assertions are not claimed.

The coordinator's [FINAL-OUTCOME.md](FINAL-OUTCOME.md) lists each accepted revision, what
review changed in each stage, and the reviewer's own probe counts (for stage 4: 69
independent API checks, 346 and 217 inherited, 134 migration checks across genuine stage 1,
2 and 3 exports, and an offline gate).

**Known limitations**

- **Grid labels clip.** In the availability grid at 1280 px, the state words "Held" and
  "Taken" are wider than their cell and spill past its edge. The operator measured this in
  stage 1 (Held 31.2 px and Taken 40.0 px in a 24.3 px text area); the cell styling is
  unchanged through stage 4. Antigravity flagged it in stage 1; the reviewer rejected it after
  measuring only the "Free" cells, the one state that fits.
- **No manager or series screens.** The specification requires none; repairs and recurring
  amendments are API features, and the existing screens show their results.

## Costs and time

- **Time.** 27 h 58 m from dispatch (2026-10-04 17:13 UTC) to the final report (2026-10-05
  21:11 UTC).
- **Money.** No seat used a metered API key. Codex runs on a ChatGPT subscription,
  OpenCode and OMP on OpenCode Go, Grok on a grok.com account, ZCode on the ZAI Coding Plan
  and Antigravity on its own login. None of the harnesses exposed per-seat token or cost counters through Band, so no per-seat
  token figures are claimed.

## Operator events in this run

**One room message.** At 19:03 UTC on 2026-10-05 the coordinator stopped the run with
stages 1–3 accepted and stage 4 blocked. Its mandate allows at most three review rounds per
work item; it had counted its own integration audits against that limit, so one stage 4 item
was declared blocked before the reviewer had seen stage 4 at all. Its stage 4 candidate
passed every supplied check. The owner sent this to the coordinator:

> Operator clarification of your mandate: the limit of 3 review rounds per work item counts
> the reviewer's formal review rounds (ZCode CHANGES verdicts), not your own integration
> audits. No stage 4 item has had a formal review round. Reopen stage 4: carry the remaining
> S4-I1 findings into a new work item and give it one fix attempt. Then freeze stage 4 and
> send it to ZCode's formal review, listing any finding still open as a known issue. ZCode
> decides whether a known issue blocks acceptance. Aim to reach the formal review within 3
> hours.

It clarifies the factory's own rule and gives no product or implementation hint. The
coordinator reopened stage 4, OMP fixed both open findings in one attempt, and the reviewer
accepted stage 4 in its first formal round.

**Six restarts, no room messages.** A seat's runtime stopped six times. Each time the
operator restarted that seat's room session and Band re-delivered the message it had been
handling:

| UTC (2026-10-05) | Seat | Cause |
|---|---|---|
| 01:56 | ZCode | Model connection cancelled mid-turn |
| 04:55 | OpenCode | Provider error: "The request contains invalid parameters" |
| 07:07 | Grok | Empty model reply on a message that needed no answer |
| 09:21 | OpenCode | Same provider error |
| 13:35 | OpenCode | Same provider error |
| 17:30 | OpenCode | Same provider error |

**The band recovered without the operator twice.** When OMP's provider rejected a request
and the error text was posted as its reply, the coordinator recognised it was not a report and
re-sent the assignment; OMP finished the work. When OpenCode's VM ran out of disk for the
harness runner image, the coordinator moved that check to the reviewer's sandbox.

## What review changed

- **Stage 1.** The login form showed the API's raw message ("missing or invalid bearer
  token"). It now says "Email or password is incorrect."
- **Stage 2.** The signed-in header no longer offers "Sign in" and "Create account"; results
  are scrolled into view after each action; the floor plan names each table once; the
  combined-table line no longer crosses a table's label.
- **Stages 3 and 4.** Most defects were caught earlier, in the coordinator's correction
  rounds before integration: for example, a recurring booking in Santiago landing on the
  previous day, an anchor booking missing from occupancy, forged values passing import, and
  the two stage 4 import cases above.

## The room record

`room.json` is Band Desktop's "Download full session" export of the room ("Seatright
Redux"), saved unchanged: all 6,263 messages, from the dispatch to the final report. The
complete set of text messages (the dispatch, every handoff, report, review, the operator message and
the final report) is in [evidence/room-transcript.md](evidence/room-transcript.md),
generated after the run from Band's CLI (`band room messages`, all pages): 371 text messages
of the room's 6,263. Band's room activity feed was off, so tool activity was not mirrored
into the room.

## What changed since run 1

Run 1 built the same four stages with four seats, one dispatch per stage, and took about 49
hours, mostly lost to stalls. For run 2:

- One dispatch for all four stages.
- A fifth seat for the interface, with a design bar, and a design pass in review with Antigravity
  as critic.
- Every handoff in one message; everything in the foreground; no acknowledgements.
- Room activity feed off, so the room stays within Band's message limit.
- Narrower write access for the coordinator, and every SHA read from Git.

## What we learned in this run

- **The coordinator absorbed the review.** From stage 3 the coordinator audited every item
  deeply before integrating it, so the reviewer confirmed rather than corrected. Quality went
  up, but stages got slower, and the round limit then backfired as described above.
- **Measure every state.** The clipped grid words survived because overflow was measured on
  one state only.
- **Some seats crash on an empty reply,** and one harness hit a provider error about every
  four hours. Both are infrastructure; a restart cleared each one.

The full list, with each fix, is in [factory/LEARNINGS.md](factory/LEARNINGS.md).

## Standing it up

Prerequisites: Band Desktop 0.4.12 or newer, Docker Sandboxes (`sbx`) 0.46.0 or newer with
KVM, a logged-in Codex CLI, and OpenCode, OMP, Grok CLI, ZCode and the Antigravity CLI with
their provider logins.

1. Create the five seats and their sandboxes as described in
   [factory/CODEX-SETUP.md](factory/CODEX-SETUP.md),
   [factory/OPENCODE-SETUP.md](factory/OPENCODE-SETUP.md),
   [factory/OMP-SETUP.md](factory/OMP-SETUP.md),
   [factory/GROK-SETUP.md](factory/GROK-SETUP.md),
   [factory/ZCODE-SETUP.md](factory/ZCODE-SETUP.md) and
   [factory/DOCKER-SETUP.md](factory/DOCKER-SETUP.md). The ZCode bridge needs the patch in
   `factory/scripts/patches/`.
2. In Band Desktop settings, set the room activity feed to off.
3. Run `factory/scripts/factory/prepare-run.sh <run>`. It creates the result repository,
   worktree root and evidence folders, mounts them into the sandboxes and installs each
   mandate where its harness reads it.
4. Create a room in Band Desktop and add the five seats, then run
   `factory/scripts/factory/new-room.sh <ids-file> --room <room-id>`.
5. Send the coordinator one dispatch: the task, the full specifications, the stack, the
   design brief, the result repository, worktree root and evidence root, and the check
   command.
6. When the run is done, run `end-run.sh <run>` and `snapshot-factory.sh <run>`.

Scripts use this machine's absolute paths and Band session ids; adapt them on another
machine. Credentials stay out of every repository.
