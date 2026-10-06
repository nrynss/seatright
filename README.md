# Seatright: Tablekeeper

**Team:** Seatright (Narayan S S) · **Track:** Tablekeeper · **Reached:** stage 4

This repository holds the output of one run of the Seatright factory. Seatright is a
five-seat Band Desktop factory, set up like a rock band:

- Codex sings lead as the coordinator.
- OpenCode on bass and OMP on drums build the backend.
- Grok plays lead guitar as the frontend and design implementer.
- ZCode plays rhythm guitar as the reviewer.
- Antigravity, running Gemini 3.8 Flash, is the reviewer's private monitor engineer and
  design critic.

All four stages came from **one dispatch**. [FACTORY.md](FACTORY.md) explains the factory,
how we ran it, and every operator event.

## How to read this repository

**Start here**

| Path | What it is |
|---|---|
| [FACTORY.md](FACTORY.md) | The factory: seats, design choices, results, operator events, what review changed, learnings, and how to stand it up. The owner wrote it after the run |
| [FINAL-OUTCOME.md](FINAL-OUTCOME.md) | The coordinator's final report. It lists each stage's accepted revision, checks and review rounds, what review changed, and the final stage 4 verification |

**The factory**

| Path | What it is |
|---|---|
| [mandates/](mandates/) | One mandate per seat, named after the seat, each naming its harness and model. They describe how each seat works, not this problem |
| [factory/](factory/) | The factory itself, as we tagged it for this run (`run-tablekeeper2`): setup documents, mandates, scripts and learnings |
| [factory/AGENT-ROLES.md](factory/AGENT-ROLES.md) | Every seat's role, access and boundaries, with the reason for each |
| [factory/CODEX-SETUP.md](factory/CODEX-SETUP.md), [OPENCODE](factory/OPENCODE-SETUP.md), [OMP](factory/OMP-SETUP.md), [GROK](factory/GROK-SETUP.md), [ZCODE](factory/ZCODE-SETUP.md) `-SETUP.md` | How we set up each agent: harness, model, sandbox, network, credentials, and what we verified. ZCODE-SETUP.md also covers Antigravity |
| [factory/DOCKER-SETUP.md](factory/DOCKER-SETUP.md) | The Docker Sandboxes: VMs, mounts and network rules |
| [factory/LEARNINGS.md](factory/LEARNINGS.md) | What the rehearsals and run 1 taught us, and the change each lesson led to |
| [operator/](operator/) | The operator's own tooling for this run: the health watch that raised every seat drop-out, the room progress log and the transcript generator. Not part of the factory |

**The run's record**

| Path | What it is |
|---|---|
| [TASK.md](TASK.md) | The coordinator's copy of the top of the owner's dispatch, made when it planned stage 1. It has the task, paths, stack, design brief and check command, but not the four specifications. The coordinator never updated it. The seats worked from the dispatch in the room, the first message in the transcript |
| `room.json` | The room ("Seatright Redux"), downloaded unchanged from Band Desktop. It holds all 6,263 messages |
| [evidence/room-transcript.md](evidence/room-transcript.md) | The room's 371 text messages, oldest first, generated from Band's CLI. It covers the dispatch, every handoff, report and review, the operator message and the final report |
| [RUNLOG.md](RUNLOG.md) | The coordinator's run log, with one timestamped entry per handoff, report, verdict, retry and blocker |
| [plan.md](plan.md) | The coordinator's living room plan from stage 3 on. It holds the current status, architecture, active handoffs, work-item queue, interface contracts and requirements ledger (R1–R382). The coordinator rewrote it 70 times, and Git history keeps the earlier versions. Codex started it because it could not use Band's task tool (see RUNLOG's first entry) |
| [architecture.json](architecture.json) | The layered architecture diagram the coordinator published with the room plan |

**The product**

| Path | What it is |
|---|---|
| [stage-1/](stage-1/) … [stage-4/](stage-4/) | One complete service per stage, each extending the last. Each has its source (`cmd/`, `internal/`, `web/`), a Dockerfile and a `RUN.md` on how to build, run and use it |
| `stage-N/PLAN.md` | That stage's plan, with its requirements ledger and work items, as the coordinator mandate requires |
| `stage-N/probes/` | The API, HTML, upgrade and export probes the band wrote for that stage, beyond the supplied checks |

**Evidence**

| Path | What it is |
|---|---|
| [evidence/](evidence/) | The accepted review of each stage, in `evidence/stage-1/` … `stage-4/`. Each holds the reviewer's `REVIEW.md`, the isolated harness report, the reviewer's own probes and the design critique. Stage 1 also keeps the reviewer's 55 design screenshots. [evidence/README.md](evidence/README.md) describes every folder |
| [evidence/stage-4-blocked/](evidence/stage-4-blocked/) | A superseded record, kept as part of the run's history. It holds the audits and the "stage 4 blocked" outcome the coordinator wrote before the operator's clarification reopened stage 4. See FACTORY.md, Operator events |

## The factory's scripts

[factory/scripts/](factory/scripts/) is the machinery that stands the factory up and runs it.
None of it is specific to Tablekeeper. A run on another problem uses the same scripts, and
only the dispatch changes.

**Running a factory run** ([factory/scripts/factory/](factory/scripts/factory/))

| Script | What it does |
|---|---|
| `prepare-run.sh <run>` | Starts a run. It refuses a factory repository with uncommitted changes and tags the commit it uses, so each run uses one exact factory version. It creates the result repository, the worktree root and one evidence folder per seat. It mounts them into the sandboxes, read-only for the reviewer, and installs each mandate where its harness reads it |
| `new-room.sh <ids-file> --room <id>` | Brings the seats into the Band room the owner created. It starts every seat and drops sessions and queued messages from earlier rooms. It adds any missing seat, checks the coordinator's runtime settings, and writes the room and participant ids to a file |
| `end-run.sh <run>` | Ends a run. It stops every seat, detaches it from the room and revokes the sandbox mounts. It keeps the result repository and the worktrees |
| `snapshot-factory.sh <run>` | Copies the tagged factory version, not the working copy, into the result repository as `factory/`. That is how this folder got here |
| `gen-implementer-mandates.py` | Writes the three implementer mandates from one shared text. The backend and frontend seats therefore follow identical rules and differ only in their role section |

**Launching each seat** ([factory/scripts/sandboxes/](factory/scripts/sandboxes/))

Band starts each seat by running one of these commands. It then talks to the seat over the
Agent Client Protocol (ACP) on the seat's standard input and output.

| Script | What it does |
|---|---|
| `codex-host.sh` | Starts the coordinator on the host as a Codex app-server, inside Codex's own `workspace-write` sandbox and with no container runtime. It turns on network for this seat alone, to fetch modules and run tests. The owner's global Codex settings stay unchanged |
| `opencode.sh`, `omp.sh`, `grok.sh`, `zcode.sh` | Start each seat inside its own Docker Sandbox VM with `sbx exec`, in the seat's fixed working directory. `grok.sh` also removes the placeholder API key that Docker Sandboxes injects, which would otherwise override Grok's own login |
| `acp-strip-mcp.py` | Sits between Band and the OMP and Grok seats. It removes the host tool server Band offers, because a seat inside a VM cannot start it |

**A fix to a harness** ([factory/scripts/patches/](factory/scripts/patches/))

| File | What it does |
|---|---|
| `zcode-acp-server-0.60.0-no-success-footer.patch` | Stops the ZCode bridge from posting a "✓ completed" line after each turn. In a Band room every posted line wakes the seat it addresses, so those lines started reply loops between seats |

The scripts use this machine's absolute paths and Band session ids, so adapt them on another
machine. They contain no credentials. Each seat's login lives only inside its own sandbox.

## The service

Each stage folder builds one Docker image. The image holds an HTTP API in Go and a Svelte 5
web UI built on Chaaya's design tokens and themes, with an SVG floor plan of the restaurant.
The same process serves both, with every asset baked in. It needs no network at run time.

```sh
cd stage-4
docker build -t tablekeeper-stage4 . && docker run --rm -p 8080:8080 -e PORT=8080 tablekeeper-stage4
```

Then open <http://localhost:8080/>.

## Checks

On the supplied checks in isolated mode, every stage passes and the repository claims
stage 4. Stage 1 passes 120/120, stage 2 25/25, stage 3 7/7 and stage 4 6/6. We claim
nothing about the hidden checks. FACTORY.md lists the known limitations.
