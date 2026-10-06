#!/bin/sh
# usage: msgs.sh SINCE_ISO_UTC [types] [maxchars] -> room messages after SINCE, oldest first
. /home/nryn/work/seatright/scratch/tablekeeper2-run/ids.sh
export LD_LIBRARY_PATH= LD_PRELOAD=
for p in 1 2 3 4 5; do band room messages $R --json --page $p 2>/dev/null; echo; done | python3 -c '
import json,sys
since=sys.argv[1]; types=sys.argv[2].split(",") if len(sys.argv)>2 and sys.argv[2] else None
lim=int(sys.argv[3]) if len(sys.argv)>3 else 400
seen={}
for line in sys.stdin.read().split("\n"):
    line=line.strip()
    if not line: continue
    try: d=json.loads(line)
    except Exception: continue
    items=d.get("messages") if isinstance(d,dict) else d
    for m in items or []: seen[m["id"]]=m
for m in sorted(seen.values(), key=lambda m:m["inserted_at"]):
    if m["inserted_at"]<=since: continue
    if types and m["message_type"] not in types: continue
    print(m["inserted_at"][11:19], m["message_type"], m["sender_name"], m["id"][:8], "|", m["content"].replace("\n"," / ")[:lim])
' "$@"
