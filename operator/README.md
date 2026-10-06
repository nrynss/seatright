# Operator tooling

These are the scripts the operator ran on the host during this run, copied unchanged. They
are not part of the factory. They watch the room and the seats from outside and send nothing
to the room. They use this run's room id and paths, so they record how we watched the run.
They are not drop-in tools.

| File | What it does |
|---|---|
| `health.sh` | Ran in the background in six-hour stretches. It exits, and so alerts the operator, at the first sign of a dropped seat. The signs are a usage, quota, connection or "turn failed" error in Band's log, a seat in state Failed, or 45 minutes without a room message. It raised all six drop-outs in this run, and the quiet room after the final report |
| `watch.sh` | Prints each new text message in the room as one line. It stops when the coordinator sends its final report to the owner |
| `msgs.sh` | Reads the room's messages through Band's CLI (`band room messages`). `health.sh` and `watch.sh` call it |
| `ids.sh` | The room id and each participant's id in this run, written by `new-room.sh`. `msgs.sh` and `watch.sh` read it |
| `gen-transcript.py` | Generated [../evidence/room-transcript.md](../evidence/room-transcript.md) from Band's CLI after the run |

Each restart in [../FACTORY.md](../FACTORY.md) followed a `health.sh` alert.
