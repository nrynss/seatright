# Seatright agent roles

| Seat | Role | Harness | Model | Placement |
| --- | --- | --- | --- | --- |
| Seatright-Codex | Coordinator | Codex CLI 0.159.3 (app-server) | `gpt-6.1-sol`, high reasoning | Host |
| Seatright-OpenCode | Backend implementer | OpenCode 2.0.21 (ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker sandbox `seatright-opencode` |
| Seatright-OMP | Backend implementer | OMP 18.4.8 (native ACP) | `opencode-go/muse-spark-1.3-contributor` | Docker sandbox `seatright-omp` |
| Seatright-Grok | Frontend implementer | Grok Build 1.0.46 (native ACP) | `grok-4.7` | Docker sandbox `seatright-grok` |
| Seatright-ZCode | Reviewer (correctness and design) | ZCode CLI 0.16.9 (ACP bridge) | ZAI Coding Plan `GLM-5.3-Flash`, with Gemini 3.8 Flash as design critic | Docker sandbox `seatright-zcode` |

The mandates in [mandates/](mandates/) define how each seat works. Each seat's setup document
covers its runtime: [Codex](CODEX-SETUP.md), [OpenCode](OPENCODE-SETUP.md), [OMP](OMP-SETUP.md),
[Grok](GROK-SETUP.md), [ZCode](ZCODE-SETUP.md). The sandbox infrastructure is in [DOCKER-SETUP.md](DOCKER-SETUP.md).

## Who does what

- **The coordinator** plans each stage into a requirements ledger and parallel work items. It
  sends self-contained handoffs, verifies the implementers' evidence, and commits each item
  with the implementer as author. It then integrates the items, sends the candidate for
  review, routes findings, and reports. It writes no product code.
- **The backend implementers** build the service: domain logic, interfaces, data handling,
  packaging and API probes. **The frontend implementer** builds the user interface to a
  design bar: one visual system, responsive layouts, a distinct presentation for every named
  state, motion that explains change, and accessibility, with screenshots and recordings as
  evidence. All implementers build one assigned item at a time in their own worktree, work
  test-first, stay inside the files they own, run everything in the foreground and report
  once. They do not run state-changing Git.
- **The reviewer** independently checks a candidate at an exact SHA against the written
  requirements. It runs the task's checks and its own probes, and makes a design pass on any
  interface, with Gemini as a second opinion on screenshots. It returns ACCEPT or CHANGES
  with reproducible findings. It never edits code.

The model choices are the owner's. Do not substitute Astra or any other model.

## Access, and why

The design principle: each seat gets exactly what its role needs to produce or check
evidence, and nothing that would let it overstep that role. The owner asked for unattended
runs, so no seat waits for a permission prompt. The boundaries come from the filesystem,
network and credential scoping below, not from per-action approval.

| | Coordinator (host) | Implementers (VMs) | Reviewer (VM) |
| --- | --- | --- | --- |
| Approval policy | Codex `never` | Band `approve-all` | Band `approve-all` |
| Seatright workspace | write (sandbox writable root) | read-only | read-only |
| Run's result repo and worktrees | write; owns all Git | write | **read-only** |
| Own evidence folder | (reads all seats') | write | write |
| Challenge kit (specs, harness) | read | read-only | read-only |
| Network | on, inside Codex's sandbox | package registries plus own provider | package registries plus own provider |
| Container runtime | **none** | Docker inside its own VM | Docker inside its own VM |
| Toolchains | Go 1.27.1, Node 26, Python (host) | Go 1.27.1, Node 26.10.0, Python 3.14 | the same, plus the harness venv and Chromium |
| Credentials | its own Codex login | only its own model-provider key | only its own model-provider key |
| Band tools | room messaging | room messaging (turn output) | room messaging (turn output) |

Why each boundary exists:

- **Coordinator writes, reviewer reads.** Git history is the teamwork evidence. The coordinator
  makes every commit and credits the implementer as author, so history stays linear and
  attributable. The reviewer's candidate is mounted read-only so a review cannot quietly
  change what it is reviewing.
- **Implementers can write the whole run folder.** Worktrees are fixed per run because seats
  cannot change directory mid-run. Ownership of individual files is enforced by the mandates
  and checked in review, not by the filesystem.
- **Evidence folders.** Each seat writes the raw output of what it ran to
  `runs/<run>/evidence/<seat>/`. The coordinator reads those files before accepting anything,
  so a reported result that was never written down counts as not run. The accepted review's
  evidence is committed under `evidence/stage-N/`.
- **No container runtime for the coordinator.** The Docker socket is root-equivalent on the
  host, and building, running and checking images is the sandboxed seats' job. The coordinator
  still verifies independently: it reads raw evidence and runs unit tests itself, which is why
  it has network access and the toolchains.
- **Coordinator network is per-seat.** It is enabled by `-c sandbox_workspace_write.network_access=true`
  in `scripts/sandboxes/codex-host.sh`, not in the owner's global Codex config.
- **Sandbox network** is the shared `balanced` policy plus per-VM rules. These cover each
  seat's model provider, Debian and Alpine package mirrors on port 80 (slim images have no CA
  certificates, so apt cannot use HTTPS), and Playwright's fallback browser host. The image
  under test has no network at run time, as the specification requires.
- **Credentials stay with their seat.** Each VM's private home holds only the key its harness
  needs. Nothing is copied into a repository or the room.
- **Band tools.** Sandboxed seats reply through their turn output. Band's offered MCP server is
  a host command a sandbox cannot start, so the OMP launcher removes it, and the other bridges
  do not use it. The coordinator's private task tools need approvals that `never` forbids, so
  it tracks work in `PLAN.md` and `RUNLOG.md`.

A change outside these boundaries (a new mount, endpoint or credential) is a controller
configuration change made by the owner before a run, never something a seat does mid-run.

## Running the factory

`scripts/factory/prepare-run.sh <run>` creates the result repository, worktree root and evidence
folders. It mounts them, and installs the mandates where each harness reads them.
`scripts/factory/new-room.sh <ids-file>` opens a fresh room with all four seats and the owner.
`scripts/factory/end-run.sh <run>` stops the seats and revokes the mounts. The stage dispatch
is the only human input.

Report completed tests and failures accurately. A runtime handshake alone is not evidence of
model inference or collaboration.
