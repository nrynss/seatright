# ZCode Band seat

Seat: `@rocknarayan/seatright-zcode` (Seatright-ZCode), scope `seatright-zcode`.
Role: reviewer in the `seatright-zcode` Docker VM over ACP.
Mandate: [mandates/seatright-zcode.md](mandates/seatright-zcode.md).

## Runtime and configuration

ZCode Desktop 3.14.4 supplies CLI 0.16.9. The local `zcode-acp-server` 0.60.0 bridge and the
extracted runtime live in `scratch/tools/zcode-acp/` (through the `.tools` compatibility link).
The tracked launcher is [scripts/sandboxes/zcode.sh](scripts/sandboxes/zcode.sh). It selects
the bridge's build mode and clears inherited AppImage library overrides.

The bridge carries one local patch: a successful turn posts no "✓ completed" status line. In a
Band room every posted chunk wakes its addressee, so a footer-only reply restarted reply loops
between seats. The patch is
[scripts/patches/zcode-acp-server-0.60.0-no-success-footer.patch](scripts/patches/zcode-acp-server-0.60.0-no-success-footer.patch).
Apply it from the bridge package directory after any install:

```sh
cd scratch/tools/zcode-acp/node_modules/zcode-acp-server
patch -p1 < /home/nryn/work/seatright/scripts/patches/zcode-acp-server-0.60.0-no-success-footer.patch
```

The model is pinned in the bridge's start script, `scratch/tools/zcode-acp/start-band.sh`:

```sh
export ZCODE_PROVIDER=account:zai-individual-coding-plan
export ZCODE_MODEL=GLM-5.3-Flash
```

GLM-5.3-Flash is on the ZAI Individual Coding Plan and reads images, which the design pass
needs. Before run 2 the seat used `GLM-5.3`; that start script is kept as
`start-band.sh.glm53`. The mandate is installed as `~/.zcode/AGENTS.md`. Band uses
`approve-all`.

**Design critic.** The Antigravity CLI (`agy` 1.2.3) is mounted read-only at
`/opt/seatright/agy`, with the owner's Antigravity login (`antigravity-oauth-token` and
`installation_id`) copied into `~/.gemini/antigravity-cli/`, mode 600. The reviewer runs it
headless during the design pass:

```sh
/opt/seatright/agy --model gemini-3.8-flash-high --dangerously-skip-permissions -p "<prompt>"
```

It is a tool the reviewer calls, not a seat: Gemini 3.8 Flash gives a multimodal second
opinion on screenshots, and the reviewer decides which points to adopt.

The harness venv is `/home/agent/harness-venv`, with Python 3.14, the kit's requirements and
Playwright Chromium. Run the checks from outside the kit with
`PYTHONPATH=<kit> /home/agent/harness-venv/bin/python -m harness run ...`.

## Access, and why

| Resource | Access | Why |
| --- | --- | --- |
| `/home/nryn/work/seatright` | read-only | Reads mandates and setup; cannot alter the factory |
| `runs/<run>/` (result repo, worktrees) | **read-only**, mounted per run | Reviews the candidate exactly as committed. Read-only Git (`rev-parse`, `status`, `diff`) confirms the SHA. It can never change what it is reviewing |
| `runs/<run>/evidence/seatright-zcode/` | write, mounted per run on top of the read-only view | The only place it writes: check reports, `REVIEW.md`, probe output. The coordinator verifies the verdict from these files |
| Challenge kit | read-only | Specification and the official harness |
| Network | shared `balanced` policy plus `zcode.z.ai`, `api.z.ai`, Debian/Alpine mirrors on port 80, Playwright's fallback host, and for the design critic `cloudcode-pa.googleapis.com`, `daily-cloudcode-pa.googleapis.com`, `oauth2.googleapis.com`, `www.googleapis.com`, `generativelanguage.googleapis.com` and `antigravity.google.com` | Model provider; the harness runner image build (`python:3.12-slim` plus `playwright install --with-deps`, which uses Debian apt over HTTP); Gemini through `agy` |
| Docker inside the VM | yes | Builds the candidate's image and runs the harness in isolated mode, as judges do |
| Go 1.27.1, Node 26.10.0 / npm 11.19.1, Python 3.14, harness venv, Chromium | yes | Probes requirements the supplied checks do not cover, including browser checks for stage 2 |
| `sudo` inside the VM | yes, VM only | Toolchain installs. Has no effect on the host |
| Credentials | ZAI Coding Plan configuration, and the Antigravity login for the design critic, in the VM's private home only | Its own model provider and its critic; no other keys |
| Band tools | room messaging through turn output | Band's host MCP server cannot run inside the sandbox |

The VM has 2 CPUs and 2 GiB of memory. Isolated checks cap the service at 2 vCPU and
2 GiB; actual use is well below that.

## Verified

- GLM-5.3-Flash through the bridge: a session on `account:zai-individual-coding-plan` /
  `GLM-5.3-Flash` (confirmed in ZCode's log) described a UI screenshot accurately.
- `agy` in the VM: logged in, and Gemini 3.8 Flash returned three concrete design problems
  with fixes for the same screenshot.
- Toy rehearsal (toy4): two exact-SHA reviews.
- Round 1 surfaced the blocked runner build (Debian apt on port 80), since fixed; round 2
  ran the isolated harness to `claimed stage: 1`.
- Earlier rehearsal: reproduced a seeded bug and passed the remediated candidate.

See [AGENT-ROLES.md](AGENT-ROLES.md) and [DOCKER-SETUP.md](DOCKER-SETUP.md).
