#!/bin/sh
# Host controller: bring the four seats into a fresh Band room for one factory run.
#   new-room.sh <ids-file> --room <room-id>
# The owner creates the room in Band Desktop, so the owner joins first and owns it, and adds
# the seats there. This script starts every seat, drops sessions and queued messages from
# earlier rooms, adds any seat still missing (through a seat already in the room), checks the
# coordinator's runtime settings, and writes room and participant ids to <ids-file>
# (sourceable shell). Band 0.4.12 reports a decode error on participant adds even when they
# succeed, so membership is verified from the participant list.
set -eu
unset LD_LIBRARY_PATH LD_PRELOAD

OWNER=rocknarayan
SEATS="seatright-codex seatright-opencode seatright-omp seatright-grok seatright-zcode"

[ "$#" -eq 3 ] && [ "$2" = --room ] || { echo "Usage: $0 <ids-file> --room <room-id>" >&2; exit 2; }
ids=$1
room=$3

# Sandboxes first: a VM that idled to a stop takes its seat's bridge process with it.
for vm in opencode omp grok zcode; do /home/nryn/.local/bin/sbx exec "seatright-$vm" true >/dev/null 2>&1; done

# Start every seat, then drop room sessions for other rooms and acknowledge every queued
# message from them, so no seat wakes into an older room or replays its backlog. Each seat's
# parked `default` template is kept: it is what Band spawns this room's session from.
for s in $SEATS; do
  band --session "$s" restart >/dev/null 2>&1 || band --session "$s" attach >/dev/null 2>&1 || true
  band --session "$s" sessions 2>/dev/null |
    awk -v r="room=$room" '$2 ~ /^room=/ && $2 != "room=(parked)" && $2 != r {print $1}' |
    while read -r hs; do band --session "$s" detach --host-session "$hs" >/dev/null 2>&1 || true; done
  band --session "$s" inbox 2>/dev/null | grep -E '^\[' | grep -v "^\[$room\]" | awk '{print $2}' |
    while read -r id; do band --session "$s" ack "$id" >/dev/null 2>&1 || true; done
done

parts=$(band room participants "$room")
printf '%s\n' "$parts" | grep -q "^$OWNER \[owner\]" ||
  { echo "Room $room is not owned by $OWNER; create it in Band Desktop" >&2; exit 1; }
member=$(for s in $SEATS; do printf '%s\n' "$parts" | grep -q "^$OWNER/$s " && { echo "$s"; break; }; done)
[ -n "$member" ] || { echo "Add at least one seat to room $room in Band Desktop" >&2; exit 1; }
for s in $SEATS; do
  printf '%s\n' "$parts" | grep -q "^$OWNER/$s " ||
    band --session "$member" chat add "$room" "$OWNER/$s" >/dev/null 2>&1 || true
done

parts=$(band room participants "$room")
pid() { printf '%s\n' "$parts" | grep -E "^$1 " | grep -oE 'id=[0-9a-f-]+' | cut -d= -f2; }
{
  echo "R=$room"
  echo "CODEX=$(pid "$OWNER/seatright-codex")"
  echo "OPENCODE=$(pid "$OWNER/seatright-opencode")"
  echo "OMP=$(pid "$OWNER/seatright-omp")"
  echo "GROK=$(pid "$OWNER/seatright-grok")"
  echo "ZCODE=$(pid "$OWNER/seatright-zcode")"
  echo "HUMAN=$(pid "$OWNER")"
} > "$ids"
. "$ids"
for v in CODEX OPENCODE OMP GROK ZCODE HUMAN; do
  eval "x=\$$v"; [ -n "$x" ] || { echo "Missing participant $v in $room" >&2; cat "$ids" >&2; exit 1; }
done
extra=$(printf '%s\n' "$parts" | awk '{print $1}' | grep -vE "^$OWNER(/seatright-(codex|opencode|omp|grok|zcode))?$" || true)
[ -z "$extra" ] || { echo "Room $room has participants without a mandate: $extra" >&2; exit 1; }

band --session seatright-codex runtime settings | grep -E 'model|effort|approval|sandbox'
echo "Room $room ready; ids in $ids"
