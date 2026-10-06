#!/bin/sh
# Stream new room messages (one line each) to stdout as they arrive, deduplicated by id.
# Exits when Codex addresses the human; bound it with an outer timeout.
D=/home/nryn/work/seatright/scratch/tablekeeper2-run
. $D/ids.sh
seen=$D/watch-seen.txt; [ -n "${KEEP_SEEN:-}" ] || : > $seen; rm -f $D/watch-done
since=${1:-$(cat $D/dispatch-time.txt)}
while :; do
  $D/msgs.sh "$since" text,error 220 | while IFS= read -r line; do
    id=$(printf '%s' "$line" | awk '{print $4}')
    grep -qx "$id" $seen && continue
    echo "$id" >> $seen; printf '%s\n' "$line"
    case "$line" in *"Seatright-Codex "*"$HUMAN"*) printf "%s" "$line" | grep -qiE "both stages|all four stages|stage 4 is (complete|delivered)|run is (complete|done|blocked)|final report" && { echo "== Codex final report to the human; watcher done"; touch $D/watch-done; };; esac
  done
  [ -f $D/watch-done ] && exit 0
  sleep 30
done
