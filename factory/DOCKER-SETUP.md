# Docker Sandboxes setup

Current Seatright configuration, verified 2026-10-02.

## Installation

Docker Sandboxes v0.46.0 is installed in `~/.docker/sbx`, with the launcher
at `~/.local/bin/sbx`. The launcher clears `LD_LIBRARY_PATH` and `LD_PRELOAD`.
Docker sign-in is configured; credentials stay outside the repository.
The local daemon uses the global `balanced` network policy.

Release: [v0.46.0](https://github.com/docker/sbx-releases/releases/tag/v0.46.0).
The amd64 archive's verified SHA-256 is
`edd86e2f21559e190723fd884c3a1dced161a555afdff85c5921ed45e7d6d56e`.

Local diagnostics and VM execution passed on CachyOS. Docker's documented
Linux support targets Ubuntu 24.04 or later.

## Agent placement

| Agent | Placement | Runtime/model |
| --- | --- | --- |
| Codex coordinator | Host | Codex CLI 0.159.3; `gpt-6.1-sol`, high reasoning |
| OpenCode implementer | `seatright-opencode` | OpenCode 2.0.21; `opencode-go/muse-spark-1.3-contributor` |
| OMP implementer | `seatright-omp` | OMP 18.4.8; `opencode-go/muse-spark-1.3-contributor` |
| Grok frontend implementer | `seatright-grok` | Grok CLI 1.0.46; `grok-4.7` |
| ZCode reviewer | `seatright-zcode` | ZCode CLI 0.16.9; ZAI Coding Plan `GLM-5.3-Flash`; Antigravity CLI for the Gemini design critic |

Each worker VM uses the official shell template, with 2 CPUs and 2 GiB of RAM. Each has Docker,
Go 1.27.1 (checksum-verified, at `/usr/local/go`) and Node 26.10.0 / npm 11.19.1
(checksum-verified, at `/usr/local/node`). `seatright-grok` was created from a template of
the OMP VM (`sbx template save`), so it has the same toolchain; see GROK-SETUP.md. The unused `seatright-codex` VM is retained,
stopped; Codex runs on the host through `scripts/sandboxes/codex-host.sh`. The retired
`seatright-pi` VM is also retained, stopped; OMP replaced Pi on 2026-10-02 (see OMP-SETUP.md).

The worker launchers in `scripts/sandboxes/` pass ACP stdio through `sbx exec -i`. If jamd
spawns a launcher from its AppImage mount, the launcher falls back to the Seatright workspace
as its working directory. Manage these VMs with `sbx`, not Band's managed sandbox
reset/recovery commands. A VM that idles to a stop is restarted by the next `sbx exec`, so a
seat's wake restarts its VM.

## Access and credentials

The full access matrix, with the reason for each boundary, is in
[AGENT-ROLES.md](AGENT-ROLES.md#access-and-why). In short:

- **Filesystem.** The Seatright workspace and the challenge kit are read-only in every VM. Per
  run, `runs/<run>/` is mounted read-write for the implementers and read-only for the
  reviewer, plus the reviewer's own `evidence/seatright-zcode/` read-write.
- **Network.** The global `balanced` policy covers the package registries. Per-VM rules add
  each seat's model provider, `deb.debian.org:80`, `security.debian.org:80`,
  `dl-cdn.alpinelinux.org:80` and `playwright.download.prss.microsoft.com:443`. Slim Debian
  images ship without CA certificates, so apt needs port 80. Blocking it fails silently:
  `apt-get update` exits 0 with warnings. `scratch/net-matrix.sh` prints the current matrix.
- **Credentials.** Provider credentials live in private VM homes. Each VM holds only its own
  seat's key. Host account refreshes must be synchronized deliberately.

## Worktree workflow

`scripts/factory/prepare-run.sh <run>` creates `runs/<run>/{result,wt,evidence/<seat>}`, a seed
commit and the mounts above, and installs the mandates. The coordinator creates fixed
worktrees under `runs/<run>/wt/` and keeps them for the whole run, because seats cannot change
directory mid-run. `scripts/factory/end-run.sh <run>` revokes the mounts. `prepare-run.sh`
tags the factory commit `run-<run>`; after the run, `scripts/factory/snapshot-factory.sh <run>`
stages that tagged tree as `factory/` in the result repository.

## Verified capabilities

- **First rehearsal (host-driven, 2026-10-01).** OpenCode and Pi (since retired) completed
  independent worktree tasks. ZCode reproduced a seeded bug and passed the corrected candidate;
  eight acceptance assertions passed.
- **Toy rehearsal toy4 (2026-10-01), coordinated end to end by Codex.**
  - OpenCode and OMP implemented in parallel and are credited as commit authors.
  - ZCode reviewed twice at the exact SHA.
  - The isolated harness, re-run independently on the host, claimed stage 1.
- **Port 80 fix (2026-10-02).** After it, the reviewer VM builds the harness runner image
  directly; see the toy4 warm-up check.

Inspect the installation with `sbx version`, `sbx diagnose --json`,
`sbx policy ls`, and `sbx ls`.

See the root agent setup documents and [AGENT-ROLES.md](AGENT-ROLES.md) for
runtime details. Local setup progress and remaining work are in
`scratch/setup.md`. Sandbox infrastructure is separate from product hosting.
