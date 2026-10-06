#!/bin/sh
# Host controller: mount one worktree and set the parked agent's next cwd.
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD
if [ "$#" -ne 2 ]; then
  echo "Usage: $0 omp|opencode|zcode /absolute/worktree" >&2
  exit 2
fi
agent=$1
worktree=$(realpath "$2")
case "$agent" in
  omp|opencode) scope="seatright-$agent"; mode=rw ;;
  zcode) scope=seatright-zcode; mode=ro ;;
  *) echo 'Unknown agent' >&2; exit 2 ;;
esac
git -C "$worktree" rev-parse --is-inside-work-tree >/dev/null
sbx exec "seatright-$agent" true
sbx mount "seatright-$agent" "$worktree:$worktree:$mode"
band --session "$scope" runtime template set --spawn-cwd "$worktree"
echo "Assigned $worktree ($mode) for new sessions. Existing room sessions retain their cwd; finish or explicitly rebind them before switching tasks."
