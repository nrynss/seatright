# Seatright factory

Seatright is a five-seat dark factory for Band Desktop. We set it up like a rock band:

| Seat | In the band | Role |
|---|---|---|
| Seatright-Codex | Lead vocals | Coordinator: plans, hands out work, verifies, commits |
| Seatright-OpenCode | Bass | Backend implementer |
| Seatright-OMP | Drums | Backend implementer |
| Seatright-Grok | Lead guitar | Frontend and design implementer |
| Seatright-ZCode | Rhythm guitar | Independent reviewer |
| Antigravity (Gemini 3.8 Flash) | Monitor engineer | Design critic inside the reviewer's sandbox, heard only by the reviewer |

**One dispatch message** started the whole run, all four stages. The seats planned, built,
reviewed and delivered it among themselves. The operator restarted seats whose runtime
dropped. The operator also sent one clarification of the coordinator's mandate, described
below.

[factory/](factory/) holds the complete factory, exactly as we tagged it for this run (tag
`run-tablekeeper2`, see [factory/SNAPSHOT.md](factory/SNAPSHOT.md)). This document explains
it. The files in `factory/` are what you need to stand it up.

## Seats

| Seat | Harness | Model | Runs in |
|---|---|---|---|
| Seatright-Codex | Codex (app-server) | `gpt-6.1-sol`, high reasoning | Host, Codex `workspace-write` sandbox, no container runtime |
| Seatright-OpenCode | OpenCode (ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker Sandbox `seatright-opencode` |
| Seatright-OMP | OMP (native ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker Sandbox `seatright-omp` |
| Seatright-Grok | Grok CLI (native ACP) | `grok-4.7` | Docker Sandbox `seatright-grok` |
| Seatright-ZCode | ZCode (ACP bridge) | ZAI Coding Plan `GLM-5.3-Flash` | Docker Sandbox `seatright-zcode` |
| Antigravity (critic) | Antigravity CLI (`agy`), headless | `gemini-3.8-flash-high` | Inside `seatright-zcode`, not a Band seat |

[mandates/](mandates/) holds one mandate per seat, named after it. The mandates are generic.
They describe how each seat works, not this problem. We checked them for task vocabulary
before the run. The problem, the stack and the paths arrive in the dispatch. The dispatch is
the first message in [evidence/room-transcript.md](evidence/room-transcript.md).

**Who does what**

- **Coordinator.** Turns the four specifications into a numbered requirements ledger
  (R1–R382 by stage 4). Splits the work into small items with owned files, interface
  contracts and acceptance checks. Sends each implementer a complete handoff in one room
  message. Checks each report against its raw evidence and its own re-run of the tests.
  Commits each item with the implementer as Git author, integrates, and sends a frozen
  candidate for review. It writes no product code.
- **Backend implementers.** OpenCode and OMP take whichever backend item comes next. The
  items cover domain logic, the HTTP API, migration, packaging and API probes.
- **Frontend implementer.** Grok owns the interface and its design bar. That means one visual
  system on the stack's design tokens, 375 and 1280 px layouts, and every named state
  designed. It also means motion that respects reduced-motion settings, an SVG floor plan,
  accessibility, and screenshots and recordings of every state.
- **Reviewer.** Checks a frozen candidate at an exact SHA, read-only. Runs the isolated
  harness and its own probes against the written requirements. Then runs a design pass:
  screenshots and recordings at both widths, a critique from Antigravity, and its own
  browser measurements of every point it adopts. Returns ACCEPT or CHANGES.

## Any seat can be its own team

Antigravity is not in the room. The reviewer runs it as a command-line tool inside its own
sandbox, gives it the screenshots, and decides which of its points are real. Band coordinates
five seats. What happens inside a seat is up to that seat. Adding a second model to the
reviewer changed nothing in the room, the other mandates or the message flow. Antigravity's
rejected points never reached the room.

| Review | Antigravity | Reviewer | Outcome |
|---|---|---|---|
| Stage 1, round 1 | 13 points, "not presentation-ready" | Measured each and disproved 12. One "contrast failure" measured 5.71:1, and one suggested fix broke the specification | 1 blocker, the login error copy. Grok fixed it in 10 minutes |
| Stage 2, round 1 | Sign-in links shown while signed in, results landing off-screen, table names drawn twice, a combined-table line through a label | Confirmed all four with browser measurements | 4 blockers. Grok fixed them in 32 minutes |
| Stages 3 and 4 | No critical findings | Its own design pass agreed | Accepted in round 1 |

The repository keeps Antigravity's critique with each accepted review. Stage 1's is in
`evidence/stage-1/design/`, and stages 2 to 4 keep theirs in `evidence/stage-N/notes/`.

## Design choices, and why

- **The coordinator owns all Git.** History is the teamwork record. Each item carries the
  seat that wrote it as its author: 25 commits by OpenCode, 17 by Grok and 16 by OMP. Each
  names its work item and handoff. The coordinator's 429 commits are integration merges and
  the room plan.
- **A different model reviews.** The implementers share a model. The reviewer runs a
  different model family from a different provider, and its critic runs a third.
- **Evidence, not claims.** Every seat writes the raw output of what it ran to
  `evidence/<seat>/`. A result nobody wrote down counts as not run. The coordinator commits
  accepted reviews under [evidence/](evidence/).
- **The reviewer cannot change what it reviews.** Its sandbox mounts the candidate
  read-only. It can write only to its own evidence folder.
- **Sandboxes for every seat that runs code.** Four VMs, each with 2 CPUs, 2 GiB, Docker, Go
  and Node. Each VM holds only its own seat's credentials. Each has a per-seat network
  allow-list on top of a shared baseline. The coordinator has no Docker access.
- **One message per handoff.** Band delivers one queued message per turn, between turns.
  Every handoff, however large, travels as one message with the full requirements.
- **Everything in the foreground.** A seat that ends its turn while work runs in the
  background can never report it. Long checks therefore run in the foreground.
- **Frontend and backend as two streams.** Grok started from interface contracts and
  fixture data while the backend seats built the service. It then proved the screens
  against the real service.
- **One factory version per run.** `prepare-run.sh` refuses a factory repository with
  uncommitted changes and tags the commit it uses. [factory/](factory/) contains that tag.

[factory/AGENT-ROLES.md](factory/AGENT-ROLES.md) gives the access matrix, with the reason for
every boundary.

## What we built to run these seats on Band

Band runs each seat as an agent that speaks the Agent Client Protocol (ACP). Five different
harnesses inside isolated VMs needed integration work of our own to run as Band seats. Each
item below lives in [factory/scripts/](factory/scripts/) or the setup documents.

- **Seats in our own VMs.** Every seat that runs code lives in a Docker Sandboxes VM. We
  created these VMs and manage them with `sbx`. Band's own sandbox is off for every seat.
  Band starts each seat by running one of our launchers as its spawn command. The launcher
  then runs the harness inside its VM through `sbx exec -i`.
- **A bridge and a patch for ZCode.** ZCode does not speak ACP, so it runs through the
  `zcode-acp-server` bridge. The bridge posted a "✓ completed" line after each successful
  turn. In a Band room every posted line wakes the seat it addresses, so these lines started
  reply loops between seats. Our patch in [factory/scripts/patches/](factory/scripts/patches/)
  removes that line.
- **Filtering Band's tool server out of sandboxed sessions.** In each new session, Band
  offers its own host tool server (MCP). A seat inside a VM cannot start that host command.
  OMP then fails the whole wake. `acp-strip-mcp.py` sits between Band and the OMP and Grok
  seats and removes the offer. Sandboxed seats reply to the room through their turn output
  instead.
- **Clearing what Band Desktop passes down.** Band Desktop runs as an AppImage. It passes its
  library and Python overrides (`LD_LIBRARY_PATH`, `LD_PRELOAD`, `PYTHONHOME`,
  `PYTHONPATH`) to every seat it spawns, and they break Python and other host tools. Every
  launcher clears them. Band sometimes spawns a launcher from inside its AppImage mount, a
  path that does not exist in the VM. The launcher then falls back to the factory directory.
- **The coordinator as a Band-managed agent.** CLI onboarding first made the coordinator a
  "local terminal agent". Band would not add that identity to a room the owner created. We
  recreated it with `band agent create` and a parked runtime template, so it joins rooms like
  any other seat. Band owner instructions live-link its mandate. Its launcher turns on
  network for this seat alone, inside Codex's own sandbox. The owner's global Codex settings
  stay unchanged.
- **Work tracking without Band's task tools.** Band's task tools need an approval that the
  coordinator's sandbox policy never grants. The coordinator therefore tracks work in
  `PLAN.md`, `plan.md` and `RUNLOG.md` (see RUNLOG's first entry).
- **Each mandate where its harness reads it.** `prepare-run.sh` installs the mandates inside
  the VMs as `~/.config/opencode/AGENTS.md`, `~/.omp/agent/AGENTS.md`, `~/.grok/AGENTS.md`
  and `~/.zcode/AGENTS.md`.
- **Run folders mounted per run.** `prepare-run.sh` mounts the run folder read-write for the
  implementers and read-only for the reviewer. A seat cannot change directory mid-run, so each
  seat keeps one fixed worktree for the whole run.
- **Grok's login inside a VM.** Docker Sandboxes injects a placeholder `XAI_API_KEY` into
  every VM, and Grok prefers it over its own login. `grok.sh` removes it.
- **One rule per network need.** Each VM has an allow-list for its own provider. It also
  allows the Debian and Alpine mirrors on port 80. Slim images have no CA certificates, so
  `apt` uses HTTP and fails silently when port 80 is closed.
- **Room settings.** We turned off Band's room activity feed. Tool activity then stays out of
  the room, and the room stays within its message limit. Run 1 hit that limit, and Band
  raised it to 30,000 for the event.
- **Noticing a dropped seat.** When a seat's runtime stops, Band shows the seat as idle and
  wakes no one. The operator's health watch ([operator/](operator/)) reads Band's log, the
  seats' states and the room's activity, and raises the drop. To recover, the operator runs
  `band --session <seat> restart --host-session <room session>`. Band then re-delivers the
  message the seat had not acknowledged.

## Results

Each stage folder is a complete service. On the supplied checks in isolated mode:

| Stage | Supplied checks | Review | Time from the previous milestone to acceptance |
|---|---|---|---|
| 1 | 120/120 | CHANGES (1 blocker), then ACCEPT | 4 h 47 m from the dispatch |
| 2 | + 25/25 | CHANGES (4 blockers), then ACCEPT | 6 h 43 m |
| 3 | + 7/7 | ACCEPT, round 1 | 6 h 35 m |
| 4 | + 6/6 (178/178 in all) | ACCEPT, round 1 | 9 h 53 m, including the stop described below |

The run took about 28 hours from dispatch to the final report. `python -m harness run --track
tablekeeper --stage 4 --mode isolated` claims stage 4. The operator re-ran it on a fresh clone
of the final commit and got the same result. The supplied checks are a subset of the judged
ones, and we claim nothing about the hidden ones.

The coordinator's [FINAL-OUTCOME.md](FINAL-OUTCOME.md) lists each accepted revision and what
review changed in each stage. It also gives the reviewer's own probe counts. For stage 4,
these are 69 independent API checks and 346 and 217 inherited checks. They also include 134
migration checks across genuine stage 1, 2 and 3 exports, and an offline gate.

**Known limitations**

- **Grid labels clip.** In the availability grid at 1280 px, the state words "Held" and
  "Taken" are wider than their cell and spill past its edge. The operator measured this in
  stage 1: Held is 31.2 px and Taken 40.0 px, in a 24.3 px text area. Stages 2 to 4 kept the
  same cell styling. Antigravity flagged the problem in stage 1. The reviewer rejected it
  after measuring only the "Free" cells, the one state that fits.
- **No manager or series screens.** The specification requires none. Repairs and recurring
  amendments are API features, and the existing screens show their results.

## Costs and time

- **Time.** 27 h 58 m from the dispatch (2026-10-04 17:13 UTC) to the final report
  (2026-10-05 21:11 UTC).
- **Money.** No seat used a metered API key. Codex runs on a ChatGPT subscription and
  OpenCode and OMP on OpenCode Go. Grok runs on a grok.com account, ZCode on the ZAI Coding
  Plan and Antigravity on its own login. No harness exposed per-seat token or cost counters
  through Band, so we claim no per-seat token figures.

## Operator events in this run

**One room message.** At 19:03 UTC on 2026-10-05 the coordinator stopped the run. It reported
stages 1–3 accepted and stage 4 blocked. Its mandate allows at most three review rounds per
work item. The coordinator had counted its own integration audits against that limit. It
declared one stage 4 item blocked before the reviewer had seen stage 4 at all. Its stage 4
candidate passed every supplied check. At 19:07 UTC the owner sent this to the coordinator:

> Operator clarification of your mandate: the limit of 3 review rounds per work item counts
> the reviewer's formal review rounds (ZCode CHANGES verdicts), not your own integration
> audits. No stage 4 item has had a formal review round. Reopen stage 4: carry the remaining
> S4-I1 findings into a new work item and give it one fix attempt. Then freeze stage 4 and
> send it to ZCode's formal review, listing any finding still open as a known issue. ZCode
> decides whether a known issue blocks acceptance. Aim to reach the formal review within 3
> hours.

The message clarifies the factory's own rule and gives no product or implementation hint. The
coordinator reopened stage 4, and OMP fixed both open findings in one attempt. The reviewer
then accepted stage 4 in its first formal round.

**Six restarts, no room messages.** A seat's runtime stopped six times. The operator's health
watch on Band's log, the seats' states and the room's activity raised each drop-out. The
operator restarted the seat's room session within about two minutes each time. Band then
re-delivered the message the seat had been handling. The watch scripts are in
[operator/](operator/).

| UTC (2026-10-05) | Seat | Cause |
|---|---|---|
| 01:56 | ZCode | Model connection cancelled mid-turn |
| 04:55 | OpenCode | Provider error: "The request contains invalid parameters" |
| 07:07 | Grok | Empty model reply on a message that needed no answer |
| 09:21 | OpenCode | Same provider error |
| 13:35 | OpenCode | Same provider error |
| 17:30 | OpenCode | Same provider error |

**The band recovered twice without the operator.** OMP's provider once rejected a request,
and OMP posted the error text as its reply. The coordinator saw it was not a report and sent
the assignment again, and OMP finished the work. OpenCode's VM once ran out of disk for the
harness runner image. The coordinator moved that check to the reviewer's sandbox.

## What review changed

- **Stage 1.** The login form showed the API's raw message ("missing or invalid bearer
  token"). It now says "Email or password is incorrect."
- **Stage 2.** The signed-in header no longer offers "Sign in" and "Create account". The page
  scrolls results into view after each action. The floor plan names each table once. The
  combined-table line no longer crosses a table's label.
- **Stages 3 and 4.** The coordinator caught most defects earlier, in its correction rounds
  before integration. Examples include a recurring booking in Santiago landing on the
  previous day and an anchor booking missing from occupancy. Others were forged values
  passing import and the two stage 4 import cases above.

## The room record

`room.json` is Band Desktop's "Download full session" export of the room ("Seatright Redux"),
saved unchanged. It holds all 6,263 messages, from the dispatch to the final report.
[evidence/room-transcript.md](evidence/room-transcript.md) holds the room's 371 text messages:
the dispatch, every handoff, report and review, the operator message and the final report.
We generated it after the run from Band's CLI (`band room messages`, all pages). With the
room activity feed off, Band kept tool activity out of the room.

## What changed since run 1

Run 1 built the same four stages with four seats and one dispatch per stage. It took about
49 hours, mostly lost to stalls. For run 2:

- One dispatch for all four stages.
- A fifth seat for the interface, with a design bar, and a design pass in review with
  Antigravity as critic.
- Every handoff in one message, everything in the foreground, and no acknowledgements.
- The room activity feed off, so the room stays within Band's message limit.
- Narrower write access for the coordinator, and every SHA read from Git.

## What we learned in this run

- **The coordinator absorbed the review.** From stage 3 on, the coordinator audited every
  item in depth before integrating it. The reviewer then confirmed rather than corrected.
  Quality went up, but stages took longer, and the round limit backfired as described above.
- **Measure every state.** The clipped grid words survived because the reviewer measured
  overflow on one state only.
- **Some seats crash on an empty reply.** One harness also hit a provider error about every
  four hours. Both are infrastructure problems, and a restart cleared each one.

[factory/LEARNINGS.md](factory/LEARNINGS.md) has the full list, with each fix.

## Standing it up

Prerequisites:

- Band Desktop 0.4.12 or newer.
- Docker Sandboxes (`sbx`) 0.46.0 or newer, with KVM.
- A logged-in Codex CLI.
- OpenCode, OMP, Grok CLI, ZCode and the Antigravity CLI, each with its provider login.

Steps:

1. Create the five seats and their sandboxes. Follow
   [factory/CODEX-SETUP.md](factory/CODEX-SETUP.md),
   [factory/OPENCODE-SETUP.md](factory/OPENCODE-SETUP.md),
   [factory/OMP-SETUP.md](factory/OMP-SETUP.md),
   [factory/GROK-SETUP.md](factory/GROK-SETUP.md),
   [factory/ZCODE-SETUP.md](factory/ZCODE-SETUP.md) and
   [factory/DOCKER-SETUP.md](factory/DOCKER-SETUP.md). The ZCode bridge needs the patch in
   `factory/scripts/patches/`.
2. In Band Desktop settings, turn the room activity feed off.
3. Run `factory/scripts/factory/prepare-run.sh <run>`. It creates the result repository, the
   worktree root and the evidence folders. It mounts them into the sandboxes and installs
   each mandate where its harness reads it.
4. Create a room in Band Desktop and add the five seats. Then run
   `factory/scripts/factory/new-room.sh <ids-file> --room <room-id>`.
5. Send the coordinator one dispatch. Include the task, the full specifications, the stack,
   the design brief, the result repository, the worktree root, the evidence root and the
   check command.
6. When the run ends, run `end-run.sh <run>` and `snapshot-factory.sh <run>`.

The scripts use this machine's absolute paths and Band session ids, so adapt them on another
machine. Credentials stay out of every repository.
