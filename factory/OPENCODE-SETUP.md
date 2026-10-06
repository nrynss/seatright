# OpenCode Band seat

Seat: `@rocknarayan/seatright-opencode` (Seatright-OpenCode), scope `seatright-opencode`.
Role: implementer in the `seatright-opencode` Docker VM over ACP.
Mandate: [mandates/seatright-opencode.md](mandates/seatright-opencode.md).

## Runtime and configuration

OpenCode 2.0.21 runs through [scripts/sandboxes/opencode.sh](scripts/sandboxes/opencode.sh)
with `acp` arguments. The host binary `/usr/bin/opencode` is mounted read-only in the VM at
`/opt/seatright/opencode`.

The VM's OpenCode configuration selects `opencode-go/muse-spark-1.3-contributor`. The Band ACP
model override is empty because model selection belongs to the VM configuration; OpenCode's
ACP catalog is empty.

The mandate is installed as `~/.config/opencode/AGENTS.md` in the VM. Band uses `approve-all`.
The launcher clears inherited AppImage library overrides. Manage the VM with `sbx`, not Band's
sandbox reset commands.

## Access, and why

| Resource | Access | Why |
| --- | --- | --- |
| `/home/nryn/work/seatright` | read-only | Reads mandates and setup; cannot alter the factory |
| `runs/<run>/` (result repo, worktrees, evidence) | write, mounted per run | Writes code in its fixed worktree and its own evidence. File ownership comes from the mandate and is checked in review. It runs no state-changing Git; the coordinator commits and credits it |
| Challenge kit | read-only | Reads the specification and can run the supplied checks while developing |
| Network | shared `balanced` policy (Go proxy, npm, Docker Hub, PyPI, GitHub, Playwright CDN) plus `opencode.ai`, `models.opencode.ai`, Debian/Alpine mirrors on port 80, and Playwright's fallback host | Model provider, module and package downloads, and image builds. Slim Debian images need port 80 because they ship without CA certificates |
| Docker inside the VM | yes | Builds and smoke-tests the service image the way judges will |
| Go 1.27.1, Node 26.10.0 / npm 11.19.1, Python 3.14 | yes | Keel needs Go 1.27; Chaaya 0.2.4 and 0.3.0 require Node `>=26 <27` |
| `sudo` inside the VM | yes, VM only | Toolchain installs. Has no effect on the host |
| Credentials | OpenCode Go key in the VM's private home only | Its own model provider; no other keys |
| Band tools | room messaging through turn output | Band's host MCP server cannot run inside the sandbox |

The VM has 2 CPUs and 2 GiB of memory.

## Verified

- Model inference and Band room replies.
- An independent host-worktree task.
- Toy rehearsal (toy4): implemented the API work item, credited as author.
- Built and ran the harness runner over HTTPS when port 80 was blocked.

See [AGENT-ROLES.md](AGENT-ROLES.md) and [DOCKER-SETUP.md](DOCKER-SETUP.md).
