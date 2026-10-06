# Learnings

What the toy rehearsals taught us while building this factory, and the change each lesson
led to. Rehearsals ran on the challenge kit's toy track, 2026-10-01 and 2026-10-02.

## Results

| Run | Outcome |
| --- | --- |
| toy, toy2, toy3 | Did not finish. Each one surfaced a problem below; we fixed it and ran again |
| toy4 | Passed stage 1 in 1 h 0 m with no human input after dispatch. About 33 m of that was the blocked apt mirror |
| toy5 | Passed stage 1 in 1 h 10 m, in a room we created and joined first. Re-run independently, the isolated harness claimed stage 1 at 100% |
| toy6 | Five seats, stages 1 and 2 in one dispatch, in 1 h 55 m with no stalls. Grok designed and built the page; the reviewer's design pass used Gemini as critic. Fixed afterwards: the coordinator twice posted a placeholder SHA, and the reviewer accepted visible polish defects |
| tablekeeper (run 1) | All four stages delivered; the isolated harness claims stage 4. About 20 h of work over 49 h, the rest lost to the stalls below. Stage 4, with the fixes in place, ran 6 h with no stalls |

## Band messaging

- **A message reaches a seat only between its turns.** In the first run the coordinator sent
  handoffs and then waited inside the same turn. The replies queued unseen, and it declared
  the stage blocked. The coordinator now ends its turn after sending, and never waits or
  polls. Liveness is checked only when it wakes.
- **Band posts each turn's final text to the sender.** Two seats that both reply set off an
  endless ping-pong. Seats now send no acknowledgements, and a turn with nothing to do ends
  with no text. ZCode's ACP bridge appended a success footer to every turn, which counted as a
  reply, so we patch it out (`scripts/patches/`).
- **Nothing wakes the coordinator if a seat hangs silently.** A seat stuck on a model call
  stalls the run. The 20-minute retry-then-reassign rule only fires when the coordinator wakes
  for some other reason. This is still an open risk.
- **The coordinator's private task tools need approval.** With approvals set to `never` they
  fail, so work is tracked in `PLAN.md` and `RUNLOG.md`. In toy5 the coordinator still tried
  them once, recorded the refusal and carried on.

## Band identities and rooms

- **Create seats with `band agent create`, not CLI onboarding.** An onboarded seat becomes a
  "local terminal agent". It cannot be added to a room someone else created, and Band Desktop
  shows it as unavailable. We deleted the coordinator and recreated it under the same handle.
- **The human creates the room.** That makes the human its owner and first member, and the
  seats join after. A room created from the CLI is owned by the agent that created it.
- **`band attach` without runtime flags resets a seat's runtime settings.** Always pass the
  full flags, or use `restart`.
- **Old rooms come back to life.** Leftover sessions and queued inbox messages from earlier
  rooms woke seats in the wrong room. `new-room.sh` now detaches them and acknowledges their
  messages before a run.
- **Participant adds report a decode error even when they succeed.** We verify membership
  from the participant list instead.

## Harnesses and models

- **Pi hung on model calls twice, with Qwen and with DeepSeek.** We replaced it with OMP on
  the same model as OpenCode.
- **OMP introduced itself as OpenCode.** Nothing in its mandate said who it was, so every
  mandate now opens with "You are **Seatright-X**".
- **Coordinator network access is set for its seat only.** It comes from
  `-c sandbox_workspace_write.network_access=true` in the launcher, which leaves the global
  Codex config alone. The coordinator needs network access to re-run unit tests while it
  verifies a report.

## Sandboxes

- **Slim Debian images have no CA certificates, so apt has to use port 80.** With port 80
  blocked, `apt-get update` still exits 0 and only prints warnings. That cost toy4 about
  33 minutes. The Debian and Alpine mirrors are now allowed on port 80 per VM.
- **Band's AppImage leaks its environment.** `LD_LIBRARY_PATH`, `PYTHONHOME` and `PYTHONPATH`
  broke Python and other tools in launchers, so every launcher unsets them.
- **Band offers each seat an MCP server that is a host command.** It cannot start inside a
  sandbox, and OMP failed its wake on it. `acp-strip-mcp.py` removes it from session requests.
- **A launcher can start in Band's AppImage mount.** Launchers fall back to the workspace when
  their working directory is a `/tmp/.mount_*` path.
- **Setup details.**
  - A sandbox's primary workspace must be read-write, so we create it without one and mount
    the workspace read-only afterwards.
  - Node's `.tar.xz` needs `xz`, so we use `.tar.gz`. Node also needs `libatomic1`.
  - VMs idle-stop; the next `sbx exec` starts them again.

## Mandates and process

- **Don't make a seat invent its own stand-in.** An implementer whose item needed another
  item's output built against a guessed interface. The plan now either starts such an item
  after integration or puts a working stub in its handoff.
- **Mandate rules must be things the seat can actually do.** An early membership rule
  required a participant tool the coordinator could not call. It now says the room is
  prepared and tells the coordinator to retry a rejected mention once.
- **A report is not evidence.** Seats write the raw output of every command to their own
  evidence folder, and the coordinator reads it before accepting. In toy5 it twice sent an
  implementer back for an abbreviated command and placeholder probe scripts, and it checked
  the reviewer's evidence before merging.
- **Size work items to keep implementers busy.** In toy5, OMP finished its packaging item and
  then waited about 40 minutes for the API item. The coordinator mandate now sizes items at
  about 20–30 minutes each and keeps a queue for each implementer. How to split the work is
  still the coordinator's call. The dispatch carries only the task, the spec and the paths.
- **Every handoff carries the complete spec.** After toy5's review handoff came out malformed,
  the coordinator resent it and the reviewer byte-compared it with the kit and the plan before
  accepting again.
- **Mandates stay generic.** The stack and paths go in the stage dispatch. We scan each
  mandate change for track vocabulary with `harness.vocabulary`.
- **Seat timings are self-reported.** Band shows no per-seat token or cost figures.

## Tablekeeper run 1

- **A full room stops the run.** Band caps a room at a fixed number of messages (10,000, raised
  to 30,000 for the event). Mirrored tool activity was 97% of ours, and the reviewer's ACCEPT
  was lost when the room filled. Turn Band's room activity feed off before a run.
- **Band Desktop's room export is partial.** "Download full session" exported only the
  messages it had loaded (4,400 of 12,685). Keep a complete copy from `band room messages`,
  all pages, alongside it.
- **One message per handoff.** Band accepts a complete handoff of 120 KB or more in one
  message. Splitting into parts gave one implementer something to wait for, and it slept
  inside its turns. The coordinator mandate now sends one message per handoff.
- **Never wait inside a turn.** An implementer ran `sleep` while waiting for later parts; they
  cannot arrive until the turn ends. Every mandate now says to end the turn at once on a
  non-final part, with no tool calls.
- **Never leave work in the background.** An implementer started its final probe in the
  background and ended its turn with "report to follow". The probe passed, but nothing woke
  any seat for eight hours. Implementers now run everything in the foreground, and the
  coordinator treats "report to follow" as no report.
- **Seats die silently.** The reviewer stopped three times: a usage limit, a model connection
  failure, and the full room. Each time Band showed it as idle or not live, and nothing in
  the factory noticed. Check each provider's usage window before a dispatch.
- **A library mentioned is not a library used.** The dispatch named Keel and Chaaya; the band
  used two small Keel utilities and Chaaya's API client, and put Keel's rate limiter (meant for
  paid routes) in front of everything, a risk on larger test suites. Name what each library
  is for in the dispatch.
- **Nobody owned the design.** The interface met its checks but looked plain, and every review
  accepted in round one. Run 2 adds a frontend implementer with a design bar, and a design
  pass in review with a multimodal critic.
- **Narrow the coordinator's write access.** The coordinator wrote its room plan to the
  factory repository root. Its mandate now limits writes to the run's folders.
