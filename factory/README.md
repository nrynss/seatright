# Seatright factory

A five-seat software factory for [Band Desktop](https://band.ai). A coordinator plans and
integrates, two backend implementers and a frontend implementer build in parallel, and an
independent reviewer checks every candidate for correctness and design. Every seat except the coordinator runs in its own Docker Sandbox, and the stage
dispatch is the only human input.

This repository is the factory: seat mandates, runtime setup, access boundaries and the
scripts that prepare and tear down a run. The code the factory produces lives in a separate
result repository for each run.

## Seats

| Seat | Role | Harness | Model |
| --- | --- | --- | --- |
| Seatright-Codex | coordinator (host) | Codex | `gpt-6.1-sol`, high reasoning |
| Seatright-OpenCode | backend implementer (sandbox) | OpenCode | `opencode-go/muse-spark-1.3-contributor` |
| Seatright-OMP | backend implementer (sandbox) | OMP | `opencode-go/muse-spark-1.3-contributor` |
| Seatright-Grok | frontend implementer (sandbox) | Grok | `grok-4.7` |
| Seatright-ZCode | reviewer (sandbox) | ZCode | ZAI Coding Plan `GLM-5.3-Flash`, Gemini 3.8 Flash as design critic |

- [mandates/](mandates/): how each seat works. The mandates are generic and say nothing about
  any particular problem.
- [AGENT-ROLES.md](AGENT-ROLES.md): who does what, and the access matrix with the reason for
  each boundary.
- Per-seat runtime and access: [CODEX-SETUP.md](CODEX-SETUP.md),
  [OPENCODE-SETUP.md](OPENCODE-SETUP.md), [OMP-SETUP.md](OMP-SETUP.md),
  [GROK-SETUP.md](GROK-SETUP.md), [ZCODE-SETUP.md](ZCODE-SETUP.md).
- Sandbox infrastructure, network policy and the run workflow:
  [DOCKER-SETUP.md](DOCKER-SETUP.md).
- What the rehearsals taught us, and what each lesson changed: [LEARNINGS.md](LEARNINGS.md).

## How a run works

1. **Prepare.** `scripts/factory/prepare-run.sh <run> [--scaffold <dir>]` creates
   `runs/<run>/{result,wt,evidence/<seat>}`. It seeds the result repository with the
   mandates, mounts the run into the sandboxes (implementers read-write, reviewer read-only
   plus its own evidence folder), and installs each mandate where its harness reads it.
2. **Open a room.** The owner creates a fresh room in Band Desktop, so the owner joins first
   and owns it, and adds the five seats. Then `scripts/factory/new-room.sh <ids-file> --room <room-id>`
   starts the seats, clears sessions and messages left over from earlier rooms, adds any seat
   still missing, and refuses a room that includes a participant without a mandate.
3. **Dispatch.** Send the coordinator one message: the complete task and specification, the
   result repository, worktree root and evidence root paths, and the checks to run. Nothing
   else is sent until the coordinator reports.
4. **End.** `scripts/factory/end-run.sh <run>` stops the seats and revokes the mounts.
5. **Package the submission.** `scripts/factory/snapshot-factory.sh <run>` copies the factory,
   exactly as tagged for this run, into the result repository's `factory/`. Commit it with
   the owner-written `README.md`, `FACTORY.md` and `room.json`.

`prepare-run.sh` refuses to start until the factory repository is committed. It then tags the
commit `run-<run>`, so every run is pinned to the exact factory version it used.

During the run:

- The coordinator writes a requirements ledger and splits the work into parallel items.
- The implementers build and record evidence.
- The coordinator verifies that evidence, commits each item with its implementer as author,
  and integrates.
- The reviewer checks the exact SHA. The coordinator verifies the reviewer's evidence before
  merging.

## Prerequisites

- Band Desktop 0.4.12 or newer.
- Docker Sandboxes (`sbx`) 0.46.0 or newer, with KVM.
- Codex CLI, logged in.
- OpenCode, OMP, Grok and ZCode with their provider credentials, installed into the VMs as
  each setup document describes. The reviewer's design critic needs the Antigravity CLI.
- Band Desktop's room activity feed set to off (Settings → Runtime), so rooms carry only real
  messages.
- The ZCode ACP bridge needs
  [scripts/patches/zcode-acp-server-0.60.0-no-success-footer.patch](scripts/patches/zcode-acp-server-0.60.0-no-success-footer.patch).

Scripts use this machine's absolute paths (`/home/nryn/...`) and Band session ids; adapt them
on another machine. Credentials, the private brief, bridge runtimes and run output stay out of
the repository (see [.gitignore](.gitignore)).
