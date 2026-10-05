# S4-I1 third coordinator audit: BLOCKED

Candidate work-item base: 29d6e55a867e6d8dd0423bb3331b54cc5ffac62a. Exactly five owned OMP dirty paths verified; diff-check clean. No author commit, merge, reset or coordinator product/test edit.

Host r3 gates: gofmt clean, vet0, full normal0 (service57.622s), configured-donor four top-level import races0 (service12.608s), build0. Raw command/CWD/UTC/exit logs under r3-host/. Peer r3 logs independently corroborate full/race4/build/native smoke; unset donors really SKIP, configured empty directory really fails1.

All seven earlier coherent forgeries reject422 with retained destination and valid controls204. Original four results in r3-host; remaining three in r3-residual-repro/repro-results.json.

B1: actual valid workflow creates t_1 party1 at2027-06-17T19:00, previews a t_2 closure19:00–20:00+02:00 with ONE unchanged considered booking/moved_count0, applies201, exports valid bytes and imports204. Owner PATCH party2 at expected_revision1 succeeds200/revision2. The actual evolved export imports422 (expected204). The original apply receipt legitimately retains revision1/party1. bindApplyRecord unchanged branch calls replayToFinal and compares immutable receipt to evolved history.

B2: from the valid pre-PATCH control snapshot, delete only plan_id in the stored genuine native preview response. Import returns204 (expected422) and replaces the destination. Missing id skips both receipt lookup and orphan check.

Evidence script: r3-evolution-repro-final2.py. Status-only results: r3-evolution-repro-final2/repro-results.json. Actual original/evolved/malformed exports and first201 apply/preview bytes remain private0700/0600 beneath its private/ directory. Every owned PID stopped/port closed. Tool stdout for residual/evolution invocations was observed directly; no complete raw wrapper transcript is claimed for those commands. Gate wrapper logs do retain exact argv/CWD/UTC/exits.

Host expectation mistakes retained: first evolution attempt used public moved instead of moved_count (KeyError, before import); second immediate bind to reused9294 failed address-in-use before PID start; corrected fresh9295 run reproduced both findings. These are driver failures, not product findings.

The three-round work-item limit (initial+r1+r2) is reached. Shared63 Blocked with reason; no fourth repair. OMP notified to preserve dirty scope and stop edits. Native I2/G2B2 and stage4 acceptance cannot proceed. Independent OpenCode65 manager races continue. No stage4 acceptance or image claim. Usage unavailable.
