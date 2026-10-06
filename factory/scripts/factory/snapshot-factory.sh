#!/bin/sh
# After a run: copy the factory version the run used into the result repository.
#   snapshot-factory.sh <run-name>
# Reads the tag prepare-run.sh recorded, exports that tagged tree (not the working copy)
# into runs/<run>/result/factory/, and stages it. Commit it together with README.md,
# FACTORY.md and room.json; those are written by the owner, not the band.
set -eu
ROOT=/home/nryn/work/seatright
[ "$#" -eq 1 ] || { echo "Usage: $0 <run-name>" >&2; exit 2; }
run="$ROOT/runs/$1"
tag=$(cat "$run/factory-tag")
dest="$run/result/factory"
[ ! -e "$dest" ] || { echo "Refusing: $dest already exists" >&2; exit 1; }
mkdir -p "$dest"
git -C "$ROOT" archive --format=tar "$tag" | tar -x -C "$dest"
commit=$(git -C "$ROOT" rev-parse "$tag^{commit}")
remote=$(git -C "$ROOT" remote get-url origin 2>/dev/null | sed 's/\.git$//')
printf 'Snapshot of the Seatright factory used by this run.\n\nTag: %s\nCommit: %s\nSource: %s/tree/%s\n' \
  "$tag" "$commit" "${remote:-local}" "$tag" > "$dest/SNAPSHOT.md"
[ ! -e "$dest/.git" ] || { echo "Refusing: nested .git in snapshot" >&2; exit 1; }
git -C "$run/result" add factory
echo "Staged factory/ from $tag ($commit) in $run/result"
