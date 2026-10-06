# Seatright: Tablekeeper

**Team:** Seatright (Narayan S S) · **Track:** Tablekeeper · **Reached:** stage 4

This repository is the output of one run of the Seatright factory, a five-seat Band Desktop
factory set up like a rock band: Codex on lead vocals (coordinator), OpenCode on bass and OMP
on drums (backend), Grok on lead guitar (frontend and design), and ZCode on rhythm guitar
(reviewer), with Antigravity, running Gemini 3.8 Flash, as its private monitor engineer (design critic). All four stages
came from **one dispatch**. See [FACTORY.md](FACTORY.md) for the factory, how it was run,
and every operator event.

## How to read this repository

**Start here**

| Path | What it is |
|---|---|
| [FACTORY.md](FACTORY.md) | The factory: seats, design choices, results, operator events, what review changed, learnings, how to stand it up. Written by the owner after the run |
| [FINAL-OUTCOME.md](FINAL-OUTCOME.md) | The coordinator's final report: each stage's accepted revision, its checks and review rounds, what review changed, and the final stage 4 verification |

**The factory**

| Path | What it is |
|---|---|
| [mandates/](mandates/) | One mandate per seat, named after the seat, each naming its harness and model. Generic: they describe how each seat works, not this problem |
| [factory/](factory/) | The factory itself, as tagged for this run (`run-tablekeeper2`): setup documents, mandates, scripts and learnings |
| [factory/AGENT-ROLES.md](factory/AGENT-ROLES.md) | Every seat's role, access and boundaries, with the reason for each |
| [factory/CODEX-SETUP.md](factory/CODEX-SETUP.md), [OPENCODE](factory/OPENCODE-SETUP.md), [OMP](factory/OMP-SETUP.md), [GROK](factory/GROK-SETUP.md), [ZCODE](factory/ZCODE-SETUP.md) `-SETUP.md` | How each agent is set up: harness, model, sandbox, network, credentials, and what was verified. Antigravity is set up inside ZCODE-SETUP.md |
| [factory/DOCKER-SETUP.md](factory/DOCKER-SETUP.md) | The Docker Sandboxes: VMs, mounts and network rules |
| [factory/LEARNINGS.md](factory/LEARNINGS.md) | What the rehearsals and run 1 taught, and the change each lesson led to |
| [operator/](operator/) | The operator's own tooling for this run: the health watch that raised every seat drop-out, the room progress log, and the transcript generator. Not part of the factory |

**The run's record**

| Path | What it is |
|---|---|
| [TASK.md](TASK.md) | A copy of the top of the owner's dispatch (task, paths, stack, design brief and check command, without the four specifications), written by the coordinator when it planned stage 1. Never updated. The seats worked from the dispatch in the room, which is the first message in the transcript |
| `room.json` | The room ("Seatright Redux"), downloaded unchanged from Band Desktop: all 6,263 messages |
| [evidence/room-transcript.md](evidence/room-transcript.md) | The room's 371 text messages, oldest first, generated from Band's CLI: the dispatch, every handoff, report and review, the operator message and the final report |
| [RUNLOG.md](RUNLOG.md) | The coordinator's run log: one timestamped entry per handoff, report, verdict, retry and blocker |
| [plan.md](plan.md) | The coordinator's living room plan from stage 3 onward: current status, architecture, the full text of active handoffs, the work-item queue with owners and dependencies, interface contracts and the requirements ledger (R1–R382). The coordinator rewrote it 70 times. Earlier versions are in Git history. It began because Band's task tool was unavailable to the coordinator (RUNLOG, first entry) |
| [architecture.json](architecture.json) | The layered architecture diagram the coordinator published with the room plan |

**The product**

| Path | What it is |
|---|---|
| [stage-1/](stage-1/) … [stage-4/](stage-4/) | One complete service per stage, each extending the last. Each has its source (`cmd/`, `internal/`, `web/`), a Dockerfile and `RUN.md` (how to build, run and use it) |
| `stage-N/PLAN.md` | That stage's plan: its requirements ledger and work items, as the coordinator mandate requires |
| `stage-N/probes/` | The API, HTML, upgrade and export probes the band wrote for that stage, beyond the supplied checks |

**Evidence**

| Path | What it is |
|---|---|
| [evidence/](evidence/) | The accepted review of each stage (`evidence/stage-1/` … `stage-4/`): the reviewer's `REVIEW.md`, the isolated harness report, its own probes and the design critique. Stage 1 also keeps the reviewer's 55 design screenshots. [evidence/README.md](evidence/README.md) describes every folder |
| [evidence/stage-4-blocked/](evidence/stage-4-blocked/) | A superseded record: the audits and the "stage 4 blocked" outcome the coordinator wrote before the operator's clarification reopened stage 4 (see FACTORY.md, Operator events). Kept as part of the run's history |

## The factory's scripts

[factory/scripts/](factory/scripts/) is the machinery that stands the factory up and runs it.
Nothing in it is specific to Tablekeeper: a run on another problem uses the same scripts, and
only the dispatch changes.

**Running a factory run** ([factory/scripts/factory/](factory/scripts/factory/))

| Script | What it does |
|---|---|
| `prepare-run.sh <run>` | Starts a run. Refuses a factory repository with uncommitted changes and tags the commit it uses, so every run is pinned to an exact factory version. Creates the result repository, the worktree root and one evidence folder per seat. Mounts them into the sandboxes (read-only for the reviewer) and installs each mandate where its harness reads it. |
| `new-room.sh <ids-file> --room <id>` | Brings the seats into the Band room the owner created. Starts every seat, drops sessions and queued messages from earlier rooms, adds any seat still missing, checks the coordinator's runtime settings, and writes the room and participant ids to a file |
| `end-run.sh <run>` | Ends a run. Stops every seat, detaches it from the room and revokes the sandbox mounts. The result repository and worktrees are kept. |
| `snapshot-factory.sh <run>` | Copies the tagged factory version, not the working copy, into the result repository as `factory/` (how this folder got here) |
| `gen-implementer-mandates.py` | Writes the three implementer mandates from one shared text, so the backend and frontend seats follow identical rules and differ only in their role section |

**Launching each seat** ([factory/scripts/sandboxes/](factory/scripts/sandboxes/))

Band starts each seat by running one of these commands, and talks to the seat over the Agent
Client Protocol (ACP) on its standard input and output.

| Script | What it does |
|---|---|
| `codex-host.sh` | Starts the coordinator on the host as a Codex app-server inside Codex's own `workspace-write` sandbox, with no container runtime. It allows network for this seat only (to fetch modules and run tests) without changing the owner's global Codex settings. |
| `opencode.sh`, `omp.sh`, `grok.sh`, `zcode.sh` | Start each seat inside its own Docker Sandbox VM with `sbx exec`, in the seat's fixed working directory. `grok.sh` also removes the placeholder API key Docker Sandboxes injects, which would otherwise override Grok's own login. |
| `acp-strip-mcp.py` | Sits between Band and the OMP and Grok seats and removes the host tool server Band offers, which a seat inside a VM cannot start |

**A fix to a harness** ([factory/scripts/patches/](factory/scripts/patches/))

| File | What it does |
|---|---|
| `zcode-acp-server-0.60.0-no-success-footer.patch` | Stops the ZCode bridge posting a "✓ completed" line after each turn. In a Band room every posted line wakes the seat it is addressed to, so those lines started reply loops between seats. |

The scripts use this machine's absolute paths and Band session ids. Adapt them on another
machine. They contain no credentials: each seat's login lives only inside its own sandbox.

## The service

Each stage folder builds one Docker image: an HTTP API in Go, and a Svelte 5 web UI built on
Chaaya's design tokens and themes, with an SVG floor plan of the restaurant, served by the
same process with every asset baked in. It runs with no network at run time.

```sh
cd stage-4
docker build -t tablekeeper-stage4 . && docker run --rm -p 8080:8080 -e PORT=8080 tablekeeper-stage4
```

Then open <http://localhost:8080/>.

## Checks

On the supplied checks, in isolated mode, every stage passes and the repository claims
stage 4 (stage 1: 120/120, stage 2: 25/25, stage 3: 7/7, stage 4: 6/6). The hidden checks
are not claimed. See FACTORY.md for the known limitations.
