# Stage 4 acceptance archive

ACCEPT, formal round 1, at frozen revision a37befe400bd0e5686af7b47dbe417ad4a433c4e.
The coordinator adopted the reconciled verdict and merged those exact stage bytes to main
at c8b19b40d6fb9c05df26b4b03695427566f70fb3. No product changed during reconciliation.

REVIEW.md is the authoritative reconciled report. REVIEW-INITIAL-SUPERSEDED.md records
the original report whose UI, receipt-binding and migration labels needed correction.
checks/report.json is the byte-exact successful serial isolated report: 120/120 + 25/25
+ 7/7 + 6/6, claimed/highest stage 4. checks-initial-failed preserves the earlier
parallel-load failure. notes/ui-gates.log contains the original 98/2 failed UI run;
reconcile/notes/ui-test-standalone.* proves the fresh 100/100 successful standalone run.

Reconciled migration measures 134/0 with original export and receipt bytes, covering
all three older source stages plus evolved native 4-to-4 state. The fresh offline
internal-network gate measures 31/0; the earlier network-none gate stands separately.
E2 combines one ingestible forgery rejected by the strict binder with six corruptions
rejected atomically by import validation. It is not seven direct binder invocations.
The E4 forced-wrong check is a failing direct binder control, not an independently
observed forced-failure subprocess. These qualifications do not change the reviewer's
nonblocking decision. COORDINATOR-ADOPTION-AUDIT.md explains adoption and limits.

MANIFEST.json binds original and archived hashes and records redactions. Public command
and result logs are included; inline secrets and printed idempotency keys are removed.
No private exports, tokens, credential fixtures, binaries, browser media or staging
dependencies are committed. Unredacted originals and media remain in the seat evidence
directory. Failed attempts and superseded or unretained raw outputs are qualified in the
reconciled report; summaries are not reconstructed raw logs.
