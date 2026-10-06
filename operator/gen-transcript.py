#!/usr/bin/env python3
"""Write the room's complete text-message transcript (oldest first) from Band's CLI.

    gen-transcript.py <room-id> <room-title> <out.md> [<all-messages-backup.json>]
"""
import json
import subprocess
import sys

room, title, out = sys.argv[1:4]
backup = sys.argv[4] if len(sys.argv) > 4 else None


def pages(kind):
    msgs, page = [], 1
    while True:
        cmd = ["band", "room", "messages", room, "--json", "--page", str(page)]
        if kind:
            cmd += ["--type", kind]
        d = json.loads(subprocess.run(cmd, capture_output=True, text=True, check=True, timeout=120).stdout)
        msgs += d["messages"]
        if not d.get("has_more"):
            return msgs
        page += 1


text = sorted(pages("text"), key=lambda m: m["inserted_at"])

# Mentions are stored as @[[<id>]]; show them as @<name>, as the room displays them.
names = {}
for m in text:
    names[m["sender_id"]] = m["sender_name"]
    mn = m.get("mention_names")
    if isinstance(mn, str):
        try:
            mn = json.loads(mn.replace("'", '"'))
        except ValueError:
            mn = {}
    names.update(mn or {})
for m in text:
    for i, n in names.items():
        m["content"] = m["content"].replace(f"@[[{i}]]", f"@{n}")
if backup:
    allm = sorted(pages(None), key=lambda m: m["inserted_at"])
    with open(backup, "w") as f:
        json.dump(allm, f)
    total = len(allm)
else:
    total = None

with open(out, "w") as f:
    f.write("# Room transcript: text messages\n\n")
    f.write(f'Generated from Band\'s CLI (`band room messages`, all pages) for room `{room}` ("{title}") '
            "after the run ended.\n")
    f.write(f"It lists all {len(text)} text messages in the room, oldest first: the dispatch, every handoff, "
            "report, review, operator message and final report. Tool calls, tool results, thoughts and status "
            "events are omitted.")
    if total:
        f.write(f" The room held {total:,} messages in all.")
    f.write(" Times are UTC.\n")
    for m in text:
        ts = m["inserted_at"][:19].replace("T", " ")
        f.write(f"\n## {ts} · {m['sender_name']} · `{m['id'][:8]}`\n\n{m['content'].rstrip()}\n")
print(f"{len(text)} text messages -> {out}" + (f"; {total} total -> {backup}" if total else ""))
