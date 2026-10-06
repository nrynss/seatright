# Operator tooling

The scripts the operator ran on the host during this run, copied unchanged. They are not part
of the factory: they watch the room and the seats from outside, and send nothing to the room.
They use this run's room id and paths, so they are a record of how the run was watched rather
than drop-in tools.

| File | What it does |
|---|---|
| `health.sh` | Ran in the background in six-hour stretches. Exits, and so alerts the operator, on the first sign that a seat has dropped out: a usage, quota, connection or "turn failed" error in Band's log, any seat in state Failed, or no room message for 45 minutes. It raised all six drop-outs in this run, and the quiet room after the final report. |
| `watch.sh` | Streams each new text message in the room as one line, and stops when the coordinator sends its final report to the owner |
| `msgs.sh` | Reads the room's messages through Band's CLI (`band room messages`). Called by `health.sh` and `watch.sh` |
| `ids.sh` | The room id and each participant's id in this run, written by `new-room.sh`. Read by `msgs.sh` and `watch.sh` |
| `gen-transcript.py` | Generated [../evidence/room-transcript.md](../evidence/room-transcript.md) from Band's CLI after the run |

Each restart in [../FACTORY.md](../FACTORY.md) followed a `health.sh` alert.
