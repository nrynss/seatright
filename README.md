# Seatright: Tablekeeper

**Team:** Seatright (Narayan S S) · **Track:** Tablekeeper · **Reached:** stage 4

This repository is the output of one run of the Seatright factory, a five-seat Band Desktop
factory set up like a rock band: Codex on lead vocals (coordinator), OpenCode on bass and OMP
on drums (backend), Grok on lead guitar (frontend and design), and ZCode on rhythm guitar
(reviewer), with Antigravity, running Gemini 3.8 Flash, as its private monitor engineer (design critic). All four stages
came from **one dispatch**. See [FACTORY.md](FACTORY.md) for the factory, how it was run,
and every operator event.

## How to read this repository

| Path | What it is |
|---|---|
| [FACTORY.md](FACTORY.md) | The factory: seats, design choices, results, operator events, learnings, how to stand it up |
| [FINAL-OUTCOME.md](FINAL-OUTCOME.md) | The coordinator's final report: accepted revisions, what review changed, verification |
| [mandates/](mandates/) | One mandate per seat, named after the seat, each naming its harness and model |
| `room.json` | The room ("Seatright Redux"), downloaded unchanged from Band Desktop: all 6,263 messages |
| [evidence/room-transcript.md](evidence/room-transcript.md) | All 371 text messages in the room, generated from Band's CLI: the dispatch, every handoff, report and review, the operator message and the final report |
| [stage-1/](stage-1/) … [stage-4/](stage-4/) | One complete service per stage, each extending the last. Build and run with its `RUN.md` |
| [evidence/](evidence/) | The accepted review trail of each stage, including the design critiques |
| [RUNLOG.md](RUNLOG.md), [plan.md](plan.md) | The coordinator's run log and its requirements ledger and work-item plan |
| [factory/](factory/) | The factory itself, exactly as tagged for this run |

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
