#!/bin/sh
# Exit (and so notify) on the first sign that a seat has dropped out of the run:
# a usage/quota error or a failed runtime in jamd's log, a seat in state Failed, or no room
# message for 45 minutes. Bound with an outer timeout.
D=/home/nryn/work/seatright/scratch/tablekeeper2-run
since=$(date -u +%Y-%m-%dT%H:%M:%S)
while :; do
  hit=$(awk -v s="$since" '$1 > s' /home/nryn/.jam/logs/jamd.log 2>/dev/null |
    grep -E "WARN|ERROR" | grep -viE "managed-network|Codex cleanup|usage_archive|projection failed" |
    grep -iE "limit|quota|usage|1308|turn failed|runtime stopped|cannot connect|unresolved cleanup" | tail -3)
  [ -n "$hit" ] && { echo "SEAT PROBLEM in jamd log:"; echo "$hit" | cut -c1-400; exit 0; }
  for s in codex opencode omp grok zcode; do
    band --session "seatright-$s" status 2>/dev/null | head -1 | grep -q " Failed " &&
      { echo "SEAT FAILED: seatright-$s"; exit 0; }
  done
  last=$($D/msgs.sh "$since" "" 80 2>/dev/null | tail -1 | awk '{print $1}')
  [ -n "$last" ] && lastsec=$(date -u -d "$(date -u +%F) $last" +%s) && now=$(date -u +%s) &&
    [ $((now - lastsec)) -gt 2700 ] && [ $((now - lastsec)) -lt 80000 ] &&
    { echo "ROOM QUIET since $last UTC (45+ min)"; exit 0; }
  sleep 60
done
