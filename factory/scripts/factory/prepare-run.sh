#!/bin/sh
# Host controller: prepare one factory run before its dispatch.
#   prepare-run.sh <run-name> [--scaffold <dir>]
# Creates runs/<run-name>/{result,wt,evidence/<seat>}, initializes the result repository with the
# mandates, mounts the run folder into the worker sandboxes (implementers rw, reviewer ro)
# and the challenge kit read-only everywhere, and installs each seat's mandate where its
# harness reads it. Room creation and the dispatch are separate steps.
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD

ROOT=/home/nryn/work/seatright
KIT=/home/nryn/work/dark-factory-wearedevs
SBX=/home/nryn/.local/bin/sbx
CODEX_SESSION=seatright-codex

[ "$#" -ge 1 ] || { echo "Usage: $0 <run-name> [--scaffold <dir>]" >&2; exit 2; }
name=$1; shift
scaffold=
if [ "${1:-}" = --scaffold ]; then scaffold=$2; fi

run="$ROOT/runs/$name"
[ ! -e "$run" ] || { echo "Refusing: $run already exists" >&2; exit 1; }

# Pin the factory version this run uses: the tree must be committed, then tag it. After the
# run, snapshot-factory.sh copies exactly this tag into the result repository.
[ -z "$(git -C "$ROOT" status --porcelain)" ] ||
  { echo "Refusing: commit the factory repository first (git -C $ROOT status)" >&2; exit 1; }
tag="run-$name"
git -C "$ROOT" rev-parse -q --verify "refs/tags/$tag" >/dev/null ||
  git -C "$ROOT" tag -a "$tag" -m "Factory version used by run $name"
git -C "$ROOT" push -q origin "$tag" 2>/dev/null || echo "NOTE: tag $tag not pushed (no remote access)" >&2

mkdir -p "$run/result/mandates" "$run/wt"
echo "$tag" > "$run/factory-tag"
# Evidence: each seat writes raw command output (test logs, harness reports, probes) to its
# own subfolder; the coordinator reads them on the host before accepting work.
for seat in seatright-opencode seatright-omp seatright-grok seatright-zcode; do mkdir -p "$run/evidence/$seat"; done
cp "$ROOT"/mandates/*.md "$run/result/mandates/"
if [ -n "$scaffold" ]; then
  mkdir -p "$run/result/stage-1"
  cp -R "$scaffold"/. "$run/result/stage-1/"
fi
git -C "$run/result" init -q -b main
git -C "$run/result" config user.name "Seatright-Codex"
git -C "$run/result" config user.email "seatright-codex@seatright.invalid"
git -C "$run/result" add -A
git -C "$run/result" -c user.name=rocknarayan -c user.email=rocknarayan@users.noreply.github.com \
  commit -q -m "Seed $name: factory mandates${scaffold:+ and starting scaffold}"

for vm in opencode omp grok zcode; do "$SBX" exec "seatright-$vm" true; done
for vm in opencode omp grok; do
  "$SBX" mount "seatright-$vm" "$run:$run:rw"
  "$SBX" mount "seatright-$vm" "$KIT:$KIT:ro"
done
"$SBX" mount seatright-zcode "$run:$run:ro"
"$SBX" mount seatright-zcode "$run/evidence/seatright-zcode:$run/evidence/seatright-zcode:rw"
"$SBX" mount seatright-zcode "$KIT:$KIT:ro"

# Mandates: Codex reads Band owner instructions; the sandboxed harnesses read a global
# AGENTS.md in their private home.
for pair in opencode:.config/opencode omp:.omp/agent grok:.grok zcode:.zcode; do
  vm=${pair%%:*}; dir=${pair#*:}
  "$SBX" exec "seatright-$vm" sh -c "mkdir -p ~/$dir && cp $ROOT/mandates/seatright-$vm.md ~/$dir/AGENTS.md"
done

band --session "$CODEX_SESSION" agent instructions set --instructions-file "$ROOT/mandates/seatright-codex.md" \
  || echo "NOTE: Codex has no attached runtime; re-run the instructions set after attaching it to the room" >&2

echo "Prepared $run (factory $tag at $(git -C "$ROOT" rev-parse --short "$tag^{commit}"))"
echo "  result:   $run/result ($(git -C "$run/result" rev-parse --short HEAD))"
echo "  worktree: $run/wt (rw: opencode, omp, grok; ro: zcode)"
echo "  evidence: $run/evidence/<seat> (each seat writes its own; coordinator reads all)"
