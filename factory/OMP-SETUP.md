# OMP Band seat

Seat: `@rocknarayan/seatright-omp` (Seatright-OMP), scope `seatright-omp`.
Role: implementer in the `seatright-omp` Docker VM over native ACP.

OMP (oh-my-pi) 18.4.8 is a single native binary. The host binary
`/home/nryn/.local/bin/omp` is mounted read-only in the VM at `/opt/seatright/omp`.
The tracked launcher `scripts/sandboxes/omp.sh` runs `omp acp` through `sbx exec -i`.
No third-party ACP bridge is involved.

The VM's `~/.omp/agent/config.yml` sets the default model role to
`opencode-go/muse-spark-1.3-contributor`, the same model as the OpenCode seat.
`~/.omp/agent/.env` holds only `OPENCODE_API_KEY`, mode 600, in the VM's private
home. The host's other OMP credentials are not copied.

Band uses `approve-all`. The launcher pipes ACP through
[scripts/sandboxes/acp-strip-mcp.py](scripts/sandboxes/acp-strip-mcp.py). It removes the host
MCP server Band offers in session requests: the sandbox cannot start that host command, and
OMP fails the whole wake when an offered server fails. The launcher also clears the AppImage
`PYTHONHOME`/`PYTHONPATH`, which otherwise break the filter's Python. The mandate is installed
as `~/.omp/agent/AGENTS.md`.

## Access, and why

| Resource | Access | Why |
| --- | --- | --- |
| `/home/nryn/work/seatright` | read-only | Reads mandates and setup; cannot alter the factory |
| `/opt/seatright/omp` | read-only (host binary) | One OMP version shared with the host; the VM cannot modify it |
| `runs/<run>/` (result repo, worktrees, evidence) | write, mounted per run | Writes code in its fixed worktree and its own evidence. File ownership comes from the mandate and is checked in review. It runs no state-changing Git; the coordinator commits and credits it |
| Challenge kit | read-only | Reads the specification and can run the supplied checks while developing |
| Network | shared `balanced` policy (Go proxy, npm, Docker Hub, PyPI, GitHub, Playwright CDN) plus `opencode.ai`, `models.opencode.ai`, Debian/Alpine mirrors on port 80, and Playwright's fallback host | Model provider, module and package downloads, and image builds. Slim Debian images need port 80 because they ship without CA certificates |
| Docker inside the VM | yes | Builds and smoke-tests the service image the way judges will |
| Go 1.27.1, Node 26.10.0 / npm 11.19.1 (`libatomic1` installed for Node), Python 3.14 | yes | Keel needs Go 1.27; Chaaya 0.2.4 and 0.3.0 require Node `>=26 <27` |
| `sudo` inside the VM | yes, VM only | Toolchain installs. Has no effect on the host |
| Credentials | `OPENCODE_API_KEY` only, in `~/.omp/agent/.env` (mode 600) | Its own model provider. The host's other OMP credentials are deliberately not copied |
| Band tools | room messaging through turn output | Band's host MCP server cannot run inside the sandbox |

The VM has 2 CPUs and 2 GiB of memory.

Verified: direct inference in the VM in 4 s, and ACP `initialize` plus `session/new`
(with Band's MCP offer filtered) through the launcher. In the toy rehearsal (toy4) it
implemented the packaging work item, credited as author, and diagnosed the blocked runner
build.

OMP replaced the Pi seat on 2026-10-02. Pi's Obit/Qwen model was slow and twice hung
on a model call without ending its turn; DeepSeek through Pi also stalled inside Band.
The retired Pi configuration is in `scratch/history/pi-retired/`.

See [AGENT-ROLES.md](AGENT-ROLES.md) and [DOCKER-SETUP.md](DOCKER-SETUP.md).
