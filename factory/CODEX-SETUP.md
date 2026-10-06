# Codex Band seat

Seat: `@rocknarayan/seatright-codex` (Seatright-Codex), scope `seatright-codex`.
Role: coordinator, on the host. Mandate: [mandates/seatright-codex.md](mandates/seatright-codex.md).

## Runtime and configuration

Band Desktop/CLI 0.4.12 manages the Codex app-server over stdio. Codex CLI 0.159.3
runs `gpt-6.1-sol` with `high` reasoning, ChatGPT subscription authentication, a
`workspace-write` sandbox and `never` approvals. Astra is excluded by the owner.

The launcher is [scripts/sandboxes/codex-host.sh](scripts/sandboxes/codex-host.sh). It:

- clears the AppImage overrides Band passes down (`LD_LIBRARY_PATH`, `LD_PRELOAD`,
  `PYTHONHOME`, `PYTHONPATH`), which otherwise break Python and other host tools;
- sets `GOCACHE`, `GOMODCACHE` and `npm_config_cache` under `/tmp/seatright-codex-cache`,
  inside the sandbox's writable roots;
- starts Codex with `-c sandbox_workspace_write.network_access=true`. This override applies
  to this seat only; the owner's global `~/.codex/config.toml` is unchanged.

The mandate is Band owner instructions, live-linked to the mandate file. The seat is a
Band-managed agent created with `band agent create`, with a parked runtime template holding
these settings. Band spawns each room's session from that template, so the owner can add the
seat to a room in Band Desktop like any other seat.

The first coordinator identity was registered through CLI onboarding, which made it a "local
terminal agent". Band would not add it to a room the owner created, and Band Desktop showed it
as unavailable. On 2026-10-02 it was deleted and recreated as a Band-managed agent under the
same name and handle (details in `scratch/history/codex-terminal-identity.txt`).

## Access, and why

| Resource | Access | Why |
| --- | --- | --- |
| `/home/nryn/work/seatright` and `/tmp` | write (sandbox writable roots) | Owns the run's Git: worktrees, attributed commits, merges, `PLAN.md`, `RUNLOG.md`, committed evidence |
| Rest of the host filesystem | read | Reads the challenge kit's specifications and every seat's evidence folder |
| Network | on, inside the sandbox | Fetches Go and npm modules and runs local services for unit tests while verifying a report |
| Go 1.27.1, Node 26.10.0, Python (host) | run | Re-runs a work item's unit tests itself instead of trusting the report |
| Docker / container runtime | **none** | The Docker socket is root-equivalent on the host. Image builds, service containers and the task's checks belong to the sandboxed seats |
| `sbx`, Band CLI daemon socket | **none** (D-Bus and socket are blocked by the sandbox) | Mounts and rooms are prepared by the owner's scripts before dispatch |
| Band private task tools | **none** (they need approval; policy is `never`) | Work is tracked in `PLAN.md` and `RUNLOG.md` instead |
| Credentials | its own Codex login only | No provider keys for other seats |

Verified from inside the seat: HTTPS to `proxy.golang.org` returns 200, a localhost socket
binds, and Python and Node 26 run.

## Reproducing the configuration

Authenticate Codex, then create the seat. Adapt absolute paths on another machine, and reuse
an existing identity if one exists.

```sh
band agent create --name Seatright-Codex \
  --cwd /home/nryn/work/seatright --session seatright-codex \
  --transport codex-app-server \
  --spawn-command /home/nryn/work/seatright/scripts/sandboxes/codex-host.sh \
  --runtime-auth subscription --codex-channel stdio \
  --runtime-model gpt-6.1-sol --runtime-effort high \
  --runtime-sandbox workspace-write --runtime-approval never --json
band --session seatright-codex agent instructions set \
  --instructions-file /home/nryn/work/seatright/mandates/seatright-codex.md
```

Inspect the seat with `band --session seatright-codex status`, `... runtime settings` and
`... runtime template show`.

## Verified

The toy rehearsal (toy4, 2026-10-01) was coordinated end to end by this seat:

- plan and parallel handoffs;
- attributed commits and integration;
- two exact-SHA reviews;
- merge to main and the final report.

The isolated harness, re-run independently, claimed stage 1. See [AGENT-ROLES.md](AGENT-ROLES.md).
