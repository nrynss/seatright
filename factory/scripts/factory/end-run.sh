#!/bin/sh
# Host controller: stop a factory run's seats and revoke its sandbox mounts.
#   end-run.sh <run-name>
# Keeps the result repository, worktrees and Band identities. Room sessions are detached
# so a later run starts without this run's history.
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD
ROOT=/home/nryn/work/seatright
SBX=/home/nryn/.local/bin/sbx
[ "$#" -eq 1 ] || { echo "Usage: $0 <run-name>" >&2; exit 2; }
run="$ROOT/runs/$1"
for s in seatright-codex seatright-opencode seatright-omp seatright-grok seatright-zcode; do
  band --session "$s" sessions 2>/dev/null | awk '$2 ~ /^room=/ && $2 != "room=(parked)" {print $1}' |
    while read -r hs; do band --session "$s" detach --host-session "$hs" >/dev/null 2>&1 || true; done
  band --session "$s" stop >/dev/null 2>&1 || true
done
"$SBX" umount seatright-zcode "$run/evidence/seatright-zcode" >/dev/null 2>&1 || true
for vm in opencode omp grok zcode; do "$SBX" umount "seatright-$vm" "$run" >/dev/null 2>&1 || true; done
band list
