# Grok Band seat

Seat: `@rocknarayan/seatright-grok` (Seatright-Grok), scope `seatright-grok`.
Role: frontend implementer in the `seatright-grok` Docker VM over native ACP.
Mandate: [mandates/seatright-grok.md](mandates/seatright-grok.md).

Grok CLI 1.0.46 is a single static binary. The host binary
(`~/.grok/downloads/grok-1.0.46-linux-x86_64`) is mounted read-only in the VM at
`/opt/seatright/grok`. The tracked launcher
[scripts/sandboxes/grok.sh](scripts/sandboxes/grok.sh) runs
`grok agent --always-approve -m grok-4.7 stdio`, which speaks ACP, through `sbx exec -i`.
No third-party bridge is involved.

The launcher does two more things:

- It removes `XAI_API_KEY` and `SBX_CRED_XAI_MODE` from the environment. Docker Sandboxes
  sets a placeholder `XAI_API_KEY` in every VM for its credential proxy, and Grok prefers it
  over the grok.com login, so the seat would fail to authenticate.
- It pipes ACP through [scripts/sandboxes/acp-strip-mcp.py](scripts/sandboxes/acp-strip-mcp.py),
  which removes the host MCP server Band offers; the sandbox cannot start it.

The VM's `~/.grok/config.toml` sets `grok-4.7` as the default model and `always-approve`
permissions, with no MCP servers. `~/.grok/auth.json` (mode 600) holds the grok.com login,
copied from the host. Grok refreshes it itself; a login expires after seven days without use.
Grok reads the mandate from `~/.grok/AGENTS.md`, installed by `prepare-run.sh`. Grok caps each
rules file at 10,000 characters, and the frontend mandate is about 6,500.

## Access, and why

| Resource | Access | Why |
| --- | --- | --- |
| `/home/nryn/work/seatright` | read-only | Reads mandates and setup; cannot alter the factory |
| `/opt/seatright/grok` | read-only (host binary) | One Grok version shared with the host; the VM cannot modify it |
| `runs/<run>/` (result repo, worktrees, evidence) | write, mounted per run | Writes the interface in its fixed worktree and its own evidence. It runs no state-changing Git; the coordinator commits and credits it |
| Challenge kit | read-only | Reads the specification and runs the supplied checks, including the browser checks |
| Network | shared `balanced` policy plus `cli-chat-proxy.grok.com`, `code.grok.com`, `api.x.ai`, `auth.x.ai`, `accounts.x.ai`, Debian/Alpine mirrors on port 80, and Playwright's fallback host | Model provider and login refresh, packages, image builds and browser installs |
| Docker inside the VM | yes | Builds and runs the service image, and records browser screenshots and videos against it |
| Go 1.27.1, Node 26.10.0 / npm 11.19.1, Python 3.14 | yes | The same toolchain as the other implementers |
| `sudo` inside the VM | yes, VM only | Toolchain installs. Has no effect on the host |
| Credentials | the grok.com login only, in `~/.grok/auth.json` (mode 600) | Its own model provider |
| Band tools | room messaging through turn output | Band's host MCP server cannot run inside the sandbox |

## How the VM was made

The VM was created from a template of the OMP VM, so it has the same OS and toolchain:

```sh
sbx stop seatright-omp
sbx template save seatright-omp seatright-worker:v1
sbx create --name seatright-grok --cpus 2 --memory 2g --pull never \
  -t docker.io/library/seatright-worker:v1 shell
sbx exec seatright-grok sh -c 'rm -rf ~/.omp'     # drop the template's OMP credentials
sbx mount seatright-grok "/home/nryn/work/seatright:/home/nryn/work/seatright:ro"
sbx mount seatright-grok "$(readlink -f ~/.grok/bin/grok):/opt/seatright/grok:ro"
```

Then add the network rules in the table above with `sbx policy allow network --sandbox
seatright-grok <host>:<port>`, copy `auth.json` and a minimal `config.toml` into
`~/.grok/`, and create the seat:

```sh
band agent create --name Seatright-Grok --cwd /home/nryn/work/seatright \
  --session seatright-grok --transport acp \
  --spawn-command /home/nryn/work/seatright/scripts/sandboxes/grok.sh \
  --runtime-approval approve-all --no-spawn-sandbox --json
```

## Verified

- Inference inside the VM with the grok.com login, with the placeholder key removed.
- ACP `initialize` and `session/new` through the launcher, with Band's MCP offer filtered;
  the session reports `grok-4.7`.
- The Band agent is created, connected, with `approve-all`.

See [AGENT-ROLES.md](AGENT-ROLES.md) and [DOCKER-SETUP.md](DOCKER-SETUP.md).
