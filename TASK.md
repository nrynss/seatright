@rocknarayan/seatright-codex Build the Tablekeeper reservation service, all four stages in order, using your band. You are the lead seat. This message is the only human input for the whole run. Do not ask me anything; report the outcome to me when all four stages are done or the work is blocked.

Implement each stage fully, and only then move to the next. Stage 1 goes in stage-1/. When stage 1 is accepted, copy stage-1/ to stage-2/ and extend the copy to the stage 2 spec; the same again for stage-3/ and stage-4/. Leave each earlier stage folder unchanged once it is accepted. Each stage folder must be a complete, buildable service on its own that meets every requirement of its own stage and all earlier stages, with source, a Dockerfile and RUN.md; if a copied folder contains a .git directory, delete it. After each stage is accepted, post its full committed revision in the room, then continue.

Track: tablekeeper
Result repository: /home/nryn/work/seatright/runs/tablekeeper2/result (branch main, seed commit 0770a6d). Mandates are in mandates/.
Worktree root: /home/nryn/work/seatright/runs/tablekeeper2/wt. The implementer sandboxes can write here and the reviewer's sandbox can only read it.
Evidence root: /home/nryn/work/seatright/runs/tablekeeper2/evidence. One subfolder per seat (seatright-opencode, seatright-omp, seatright-grok, seatright-zcode).
Challenge kit (specs and checks, read-only, visible in every sandbox): /home/nryn/work/dark-factory-wearedevs. The four specs are tablekeeper/spec/stage-1.md to stage-4.md, reproduced in full below.

Stack and how to use it:
- HTTP API in Go 1.27 with Keel v0.5.0 (module github.com/nrynss/keel, version v0.5.0). Use keel/id for opaque ids and tokens. Keel's other packages serve media and paid-API apps; use one only where it meets the specification exactly, including headers. Do not use keel/gate, keel/throttle or any other rate limiting: the specification does not ask for it.
- Web UI in Svelte with @nrynss/chaaya 0.3.0 (npm; Node >=26 <27), served by the same Go service. Make every API call through the Keel adapter, @nrynss/chaaya/keel (api, ApiError, keelErrorParser): the specification's error envelope has the same shape as Keel's, so refusals keep their stable error codes. Chaaya 0.3.0 has no @nrynss/chaaya/wire; error parsing lives in the Keel adapter. Build the visual system on @nrynss/chaaya/tokens and @nrynss/chaaya/tokens/reference.css, with @nrynss/chaaya/theme for light, dark and system modes. Run @nrynss/chaaya/testing's contrast and accessibility gates in the UI tests.
- One Dockerfile builds both into a single image. Every font, script, stylesheet and image ships inside the image; nothing loads from the network at run time.

Design brief for the UI:
- It should look gorgeous and feel alive: a warm, polished restaurant product someone would be proud to demo, not a form.
- The centrepiece is an inline SVG floor plan of the restaurant. Tables are drawn as shapes with their seats, sized by capacity. Availability, selection and (from stage 2) combined tables show on the plan and animate as they change. The plan mirrors the availability grid the specification requires; it never replaces it.
- Motion is slick and purposeful: smooth transitions between search, results, booking and confirmation, staggered entrances for results, spring-based selection, and a confirmation moment that feels like an achievement. Use Svelte's transition, animate and motion modules. Respect prefers-reduced-motion.
- Every state the specification names is designed, not just present: loading, empty, error, uncertain and confirmed each have their own clear presentation, with human-readable dates and times.
- Responsive at 375 px and 1280 px; light and dark themes from the tokens; accessible contrast, focus and labels.
- Two hard rules. Keep every data-testid, label and element the specification names, exactly. Never let an animation delay a state: the state is true first, and the animation shows it.

Checks:
- The reviewer runs the supplied checks inside its own sandbox, for the stage under review N:
cd /tmp && PYTHONPATH=/home/nryn/work/dark-factory-wearedevs /home/agent/harness-venv/bin/python -m harness run --track tablekeeper --repo <candidate worktree> --stage N --mode isolated --out /home/nryn/work/seatright/runs/tablekeeper2/evidence/seatright-zcode/<review id>/checks
stage-N/ is graded against every suite up to N. The supplied checks are a subset of the judged ones. A good stage N passes stages 1 to N, fails stage N+1 (expected; there is no stage 5), and prints "claimed stage: N".
- Implementers can build and run containers in their own sandboxes with docker.

Specifications (tablekeeper/spec/stage-1.md to stage-4.md, verbatim):
