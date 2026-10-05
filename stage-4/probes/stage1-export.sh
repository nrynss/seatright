#!/bin/sh
# Two-process export/import probe. Transfers an unchanged export snapshot
# from an independently started SOURCE container to an independently started
# DESTINATION container (distinct ports, distinct processes, same built
# image) and proves replacement atomicity there: source token, hashed-password
# login, the complete original receipt JSON and the retained booking work on
# the destination; repeat import is stable; the destination's prior
# credentials/tokens are gone; a corrupt import leaves the destination
# byte-identical; destination reset clears only the destination. A later
# source write after export does not move the already-imported destination.
# Private exports/tokens stay in <workdir>, never committed.
#
# Usage: sh probes/stage1-export.sh <source-base-url> <destination-base-url> <workdir>
set -eu
SRC="$1"
DST="$2"
WORK="$3"
mkdir -p "$WORK"
PASS=0
FAIL=0
ok() { PASS=$((PASS+1)); echo "PASS $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

code_of() { curl -s -o "$WORK/out" -w '%{http_code}' "$@"; }
need() { [ "$1" = "$2" ] || { bad "$3" "want $2 got $1: $(cat "$WORK/out" 2>/dev/null)"; exit 1; }; ok "$3"; }

FIXTURE='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse","display_name":"Ada"}],"restaurants":[{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"},{"weekday":"fri","opens":"18:00","closes":"23:30"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}]}],"reservations":[]}'

# Source and destination start from different states: the destination holds
# unrelated credentials, tokens and an empty restaurant list.
[ "$(code_of -X POST "$SRC/_test/reset" -H 'Content-Type: application/json' -d "$FIXTURE")" = "204" ] && ok "src-reset-204" || { bad "src-reset-204" "$(cat "$WORK/out")"; exit 1; }
[ "$(code_of -X POST "$DST/_test/reset" -H 'Content-Type: application/json' -d '{"users":[{"id":"u_other","email":"other@example.com","password":"correct horse","display_name":"Other"}],"restaurants":[],"reservations":[]}')" = "204" ] && ok "dst-reset-204" || { bad "dst-reset-204" "$(cat "$WORK/out")"; exit 1; }
DST_TOK=$(curl -s -X POST "$DST/auth/signup" -H 'Content-Type: application/json' -d '{"email":"dst@example.com","password":"correct horse","display_name":"Dst"}' | jq -r .token)
[ -n "$DST_TOK" ] && [ "$DST_TOK" != "null" ] && ok "dst-prior-token" || { bad "dst-prior-token" "no token"; exit 1; }

# Source creates account + token + booking + successful receipt, then exports.
TOK=$(curl -s -X POST "$SRC/auth/signup" -H 'Content-Type: application/json' -d '{"email":"e@example.com","password":"correct horse","display_name":"E"}' | jq -r .token)
[ -n "$TOK" ] && [ "$TOK" != "null" ] && ok "src-signup-token" || { bad "src-signup-token" "no token"; exit 1; }
BODY='{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":4}'
C1=$(curl -s -o "$WORK/first" -w '%{http_code}' -X POST "$SRC/reservations" -H "Authorization: Bearer $TOK" -H 'Idempotency-Key: k-exp-1' -H 'Content-Type: application/json' -d "$BODY")
need "$C1" 201 "src-write-201"
REF=$(jq -r .reference "$WORK/first")

curl -s "$SRC/_test/export" > "$WORK/export.json"
jq -e '.track == "tablekeeper" and .format_version == 1 and (.state | type == "object")' "$WORK/export.json" >/dev/null && ok "export-shape" || { bad "export-shape" "$(head -c 200 "$WORK/export.json")"; exit 1; }

# Import the unchanged source snapshot into the destination (replacement).
[ "$(code_of -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$WORK/export.json")" = "204" ] && ok "dst-import-204" || { bad "dst-import-204" "$(cat "$WORK/out")"; exit 1; }
# Repeat import is stable.
[ "$(code_of -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$WORK/export.json")" = "204" ] && ok "dst-reimport-204" || exit 1

# Source token, hashed-password login, booking and the COMPLETE original
# receipt JSON (not just the reference) work on the destination.
C2=$(curl -s -o "$WORK/got" -w '%{http_code}' "$DST/reservations/$REF" -H "Authorization: Bearer $TOK")
need "$C2" 200 "dst-token-survives"
jq -e --arg r "$REF" '.reference == $r and .party_size == 4 and .status == "confirmed"' "$WORK/got" >/dev/null && ok "dst-booking-intact" || { bad "dst-booking-intact" "$(cat "$WORK/got")"; exit 1; }
C3=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d '{"email":"e@example.com","password":"correct horse"}')
need "$C3" 200 "dst-hash-login-survives"
C4=$(curl -s -o "$WORK/replay" -w '%{http_code}' -X POST "$DST/reservations" -H "Authorization: Bearer $TOK" -H 'Idempotency-Key: k-exp-1' -H 'Content-Type: application/json' -d "$BODY")
need "$C4" 200 "dst-receipt-replay-200"
jq -e --slurpfile orig "$WORK/first" '. == $orig[0]' "$WORK/replay" >/dev/null && ok "dst-receipt-complete-json" || { bad "dst-receipt-complete-json" "replay=$(cat "$WORK/replay") orig=$(cat "$WORK/first")"; exit 1; }

# Destination prior credentials/tokens are removed by replacement import.
C5=$(curl -s -o /dev/null -w '%{http_code}' "$DST/reservations/$REF" -H "Authorization: Bearer $DST_TOK")
need "$C5" 401 "dst-prior-token-removed"
C6=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d '{"email":"other@example.com","password":"correct horse"}')
need "$C6" 401 "dst-prior-login-removed"

# Corrupt import leaves the destination byte-identical.
BEFORE=$(curl -s "$DST/_test/export")
BAD=$(jq '.state.tokens["tok-ghost"] = "u_ghost"' "$WORK/export.json")
C7=$(curl -s -o "$WORK/bad" -w '%{http_code}' -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d "$BAD")
need "$C7" 422 "dst-corrupt-422"
AFTER=$(curl -s "$DST/_test/export")
[ "$AFTER" = "$BEFORE" ] && ok "dst-atomic-unchanged" || { bad "dst-atomic-unchanged" "destination moved"; exit 1; }

# The export is an immutable snapshot: a later source write does not move
# the already-imported destination.
C8=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$SRC/reservations" -H "Authorization: Bearer $TOK" -H 'Idempotency-Key: k-exp-late' -H 'Content-Type: application/json' -d '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":2}')
need "$C8" 201 "src-late-write-201"
STILL=$(curl -s "$DST/_test/export")
[ "$STILL" = "$BEFORE" ] && ok "dst-snapshot-immutable" || { bad "dst-snapshot-immutable" "destination moved after source write"; exit 1; }

# Destination reset clears only the destination; the source still serves.
[ "$(code_of -X POST "$DST/_test/reset" -H 'Content-Type: application/json' -d "$FIXTURE")" = "204" ] && ok "dst-reset-clears-204" || exit 1
C9=$(curl -s -o /dev/null -w '%{http_code}' "$DST/reservations/$REF" -H "Authorization: Bearer $TOK")
need "$C9" 401 "dst-reset-clears-token"
C10=$(curl -s -o /dev/null -w '%{http_code}' "$SRC/reservations/$REF" -H "Authorization: Bearer $TOK")
need "$C10" 200 "src-unaffected-by-dst-reset"

echo
echo "RESULT pass=$PASS fail=$FAIL"
[ "$FAIL" = "0" ]
