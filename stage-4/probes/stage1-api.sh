#!/bin/sh
# Probes the packaged tablekeeper:s1-p image: health on a non-default PORT,
# reset/auth/tokens, public browsing, error envelopes, every write path
# (create/list/lookup/cancel/PATCH), atomic move batches with rollback and
# precedence, idempotency scopes/identity/reuse, DST + adjacency, and
# export/import preservation. Exits nonzero on the first failure.
#
# Usage: sh probes/stage1-api.sh <base-url> <workdir>
# Private artifacts (exports, tokens) stay in <workdir>, never committed.
set -eu
BASE="$1"
WORK="$2"
mkdir -p "$WORK"
PASS=0
FAIL=0

ok() { PASS=$((PASS+1)); echo "PASS $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

# request METHOD PATH [BODY] [TOKEN] [KEY] -> writes $WORK/out (body) and
# $WORK/status (code); prints nothing.
req() {
  curl -s -o "$WORK/out" -w '%{http_code}' \
    ${4:+-H "Authorization: Bearer $4"} \
    ${5:+-H "Idempotency-Key: $5"} \
    ${3:+-H 'Content-Type: application/json' -d "$3"} \
    -X "$1" "$BASE$2" > "$WORK/status"
}
code() { cat "$WORK/status"; }
code_is() { [ "$(code)" = "$1" ] || { bad "$2" "want HTTP $1 got $(code): $(cat "$WORK/out")"; return 1; }; ok "$2"; }
code_has() { grep -q "\"code\":\"$2\"" "$WORK/out" || { bad "$1" "want code $2: $(cat "$WORK/out")"; return 1; }; ok "$1"; }

echo "== health =="
req GET /health; code_is 200 "health-200"
grep -q '"status":"ok"' "$WORK/out" || { bad "health-body" "$(cat "$WORK/out")"; exit 1; }; ok "health-body"

echo "== repeated reset =="
FIXTURE='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse","display_name":"Ada"}],"restaurants":[{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"},{"weekday":"fri","opens":"18:00","closes":"23:30"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}]}],"reservations":[]}'
req POST /_test/reset "$FIXTURE"; code_is 204 "reset-204"
req POST /_test/reset '{"users":[],"restaurants":[],"reservations":[]}'; code_is 204 "reset-empty-204"
req POST /_test/reset "$FIXTURE"; code_is 204 "reset-restore-204"

echo "== auth + tokens =="
req POST /auth/signup '{"email":"a@example.com","password":"correct horse","display_name":"A"}'; code_is 201 "signup-201"
TOKEN_A=$(jq -r .token "$WORK/out")
req POST /auth/signup '{"email":"a@example.com","password":"correct horse","display_name":"A2"}'; code_is 409 "dup-email-409"; code_has "dup-email-code" "email_taken"
req POST /auth/signup '{"email":"short@example.com","password":"short","display_name":"S"}'; code_is 422 "short-pw-422"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse"}'; code_is 200 "seed-login-200"
req POST /auth/login '{"email":"ada@example.com","password":"wrong password"}'; code_is 401 "bad-login-401"; code_has "bad-login-code" "unauthenticated"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse"}'; code_is 200 "login-again-200"
TOKEN_B=$(jq -r .token "$WORK/out")
[ "$TOKEN_A" != "$TOKEN_B" ] && ok "tokens-distinct" || { bad "tokens-distinct" "same token twice"; exit 1; }

echo "== public browsing =="
req GET '/restaurants'; code_is 200 "list-200"
jq -e '.restaurants[0].id == "r_anker"' "$WORK/out" >/dev/null && ok "list-id" || { bad "list-id" "$(cat "$WORK/out")"; exit 1; }
req GET '/restaurants/r_anker'; code_is 200 "detail-200"
req GET '/restaurants/nope'; code_is 404 "detail-404"; code_has "detail-404-code" "not_found"
req GET '/reservations' '' "$TOKEN_A"; code_is 200 "authed-list-200"
req GET '/reservations'; code_is 401 "anon-list-401"; code_has "anon-list-code" "unauthenticated"

echo "== unknown fields/params ignored =="
req GET '/availability?restaurant_id=r_anker&date=2027-05-06&party_size=2&zzz=1'; code_is 200 "unknown-param-200"

echo "== malformed vs validation =="
req POST /auth/signup '{oops' ; code_is 400 "malformed-400"; code_has "malformed-code" "malformed_request"
req POST /auth/signup '{"email":42,"password":"correct horse","display_name":"X"}'; code_is 400 "wrong-type-400"

echo "== idempotency key required =="
req POST /reservations '{"restaurant_id":"r_anker"}' "$TOKEN_A"; code_is 400 "missing-key-400"; code_has "missing-key-code" "missing_idempotency_key"

echo "== create + availability + adjacent =="
CREATE_A="{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_2\",\"starts_at_local\":\"2027-05-06T19:00\",\"party_size\":4}"
req POST /reservations "$CREATE_A" "$TOKEN_A" k-create-a; code_is 201 "create-201"
REF_A=$(jq -r .reference "$WORK/out")
REF_A="${REF_A:?no reference}"
grep -q '"starts_at":"2027-05-06T19:00:00+02:00"' "$WORK/out" && ok "create-offset" || { bad "create-offset" "$(cat "$WORK/out")"; exit 1; }
# Overlap on the same table is refused.
req POST /reservations "$CREATE_A" "$TOKEN_A" k-create-b; code_is 409 "overlap-409"; code_has "overlap-code" "table_unavailable"
# Adjacent (end == next start) does not overlap.
CREATE_ADJ="{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_2\",\"starts_at_local\":\"2027-05-06T20:30\",\"party_size\":4}"
req POST /reservations "$CREATE_ADJ" "$TOKEN_A" k-create-adj; code_is 201 "adjacent-201"
REF_ADJ=$(jq -r .reference "$WORK/out")
# t_2 holds 19:00-20:30 and adjacent 20:30-22:00; party 2 still sees free
# t_1 at 19:00, while party 4 (t_1 too small) sees nothing there.
req GET '/availability?restaurant_id=r_anker&date=2027-05-06&party_size=2'; code_is 200 "avail-200"
jq -e '.slots[] | select(.starts_at_local=="2027-05-06T19:00") | .available_table_ids == ["t_1"]' "$WORK/out" >/dev/null && ok "avail-occupied" || { bad "avail-occupied" "$(cat "$WORK/out")"; exit 1; }
req GET '/availability?restaurant_id=r_anker&date=2027-05-06&party_size=4'; code_is 200 "avail4-200"
jq -e '.slots[] | select(.starts_at_local=="2027-05-06T19:00") | .available_table_ids == []' "$WORK/out" >/dev/null && ok "avail4-empty" || { bad "avail4-empty" "$(cat "$WORK/out")"; exit 1; }
# Closed day is an empty slots array.
req GET '/availability?restaurant_id=r_anker&date=2027-05-05&party_size=2'; code_is 200 "closed-200"
jq -e '.slots == []' "$WORK/out" >/dev/null && ok "closed-empty" || { bad "closed-empty" "$(cat "$WORK/out")"; exit 1; }
echo "== validation codes =="
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-06T19:15","party_size":2}' "$TOKEN_A" k-grid; code_is 422 "grid-422"; code_has "grid-code" "not_on_slot_grid"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-06T17:30","party_size":2}' "$TOKEN_A" k-hours; code_is 422 "hours-422"; code_has "hours-code" "outside_opening_hours"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-05-06T19:00","party_size":4}' "$TOKEN_A" k-cap; code_is 422 "capacity-422"; code_has "capacity-code" "party_exceeds_capacity"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":0}' "$TOKEN_A" k-party0; code_is 422 "party0-422"
req POST /reservations '{"restaurant_id":"r_nope","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}' "$TOKEN_A" k-norest; code_is 404 "unknown-rest-404"

echo "== DST spring/fall =="
SPRING='{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],"restaurants":[{"id":"r_ber","name":"B","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":30,"cancellation_cutoff_minutes":0,"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"05:00"}],"tables":[{"id":"t1","label":"1","capacity":2}]}],"reservations":[]}'
req POST /_test/reset "$SPRING"; code_is 204 "dst-reset-204"
req POST /auth/signup '{"email":"d@example.com","password":"correct horse","display_name":"D"}'; code_is 201 "dst-signup-201"
TOKEN_D=$(jq -r .token "$WORK/out")
req POST /reservations '{"restaurant_id":"r_ber","table_id":"t1","starts_at_local":"2026-03-29T02:30","party_size":1}' "$TOKEN_D" k-skip; code_is 422 "skipped-422"; code_has "skipped-code" "invalid_local_time"
req GET '/availability?restaurant_id=r_ber&date=2026-03-29&party_size=1'; code_is 200 "spring-avail-200"
grep -q '2026-03-29T02:00\|2026-03-29T02:30' "$WORK/out" && { bad "spring-skip-absent" "$(cat "$WORK/out")"; exit 1; }; ok "spring-skip-absent"
req GET '/availability?restaurant_id=r_ber&date=2026-10-25&party_size=1'; code_is 200 "fall-avail-200"
[ "$(jq -r '[.slots[].starts_at_local] | map(select(.=="2026-10-25T02:00" or .=="2026-10-25T02:30")) | length' "$WORK/out")" = "2" ] && ok "fall-once" || { bad "fall-once" "$(cat "$WORK/out")"; exit 1; }
req POST /reservations '{"restaurant_id":"r_ber","table_id":"t1","starts_at_local":"2026-10-25T01:30","party_size":1}' "$TOKEN_D" k-fall; code_is 201 "fall-book-201"
jq -e '.ends_at == "2026-10-25T02:00:00+02:00"' "$WORK/out" >/dev/null && ok "fall-absolute-end" || { bad "fall-absolute-end" "$(cat "$WORK/out")"; exit 1; }
req POST /_test/reset "$FIXTURE"; code_is 204 "fixture-restore-204"
# The DST reset wiped all tokens and bookings; recreate identity + both bookings fresh.
req POST /auth/signup '{"email":"a@example.com","password":"correct horse","display_name":"A"}'; code_is 201 "relogin-201"
TOKEN_A=$(jq -r .token "$WORK/out")
req POST /reservations "$CREATE_A" "$TOKEN_A" k-create-a2; code_is 201 "recreate-a-201"
REF_A=$(jq -r .reference "$WORK/out")
req POST /reservations "$CREATE_ADJ" "$TOKEN_A" k-create-adj2; code_is 201 "recreate-adj-201"
REF_ADJ=$(jq -r .reference "$WORK/out")

echo "== list/lookup/cancel/PATCH =="
req POST /reservations "$CREATE_A" "$TOKEN_A" k-create-a2; code_is 200 "replay-200"
echo "$REF_A" | grep -q "$(jq -r .reference "$WORK/out")" && ok "replay-same-ref" || { bad "replay-same-ref" "$(cat "$WORK/out")"; exit 1; }
req GET "/reservations/$REF_A" '' "$TOKEN_A"; code_is 200 "lookup-200"
req GET "/reservations/$REF_A"; code_is 401 "lookup-anon-401"
req POST "/reservations/$REF_ADJ/cancel" '' "$TOKEN_A"; code_is 200 "cancel-200"
req POST "/reservations/$REF_ADJ/cancel" '' "$TOKEN_A"; code_is 200 "cancel-twice-200"
req PATCH "/reservations/$REF_A" '{"party_size":2}' "$TOKEN_A"; code_is 200 "patch-200"
jq -e '.party_size == 2' "$WORK/out" >/dev/null && ok "patch-party" || { bad "patch-party" "$(cat "$WORK/out")"; exit 1; }
# Replay after PATCH still returns the original created response.
req POST /reservations "$CREATE_A" "$TOKEN_A" k-create-a2; code_is 200 "replay-after-patch-200"
jq -e '.party_size == 4' "$WORK/out" >/dev/null && ok "replay-immutable" || { bad "replay-immutable" "$(cat "$WORK/out")"; exit 1; }
MK_A="{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-05-06T18:00\",\"party_size\":2}"
MK_B="{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_2\",\"starts_at_local\":\"2027-05-06T21:30\",\"party_size\":2}"
req POST /reservations "$MK_A" "$TOKEN_A" k-mk-a; code_is 201 "mk-a-201"; REF_MA=$(jq -r .reference "$WORK/out")
req POST /reservations "$MK_B" "$TOKEN_A" k-mk-b; code_is 201 "mk-b-201"; REF_MB=$(jq -r .reference "$WORK/out")
MOVES_SWAP="{\"moves\":[{\"reference\":\"$REF_MA\",\"table_id\":\"t_2\",\"starts_at_local\":\"2027-05-06T21:30\"},{\"reference\":\"$REF_MB\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-05-06T18:00\"}]}"
req POST /reservation-moves "$MOVES_SWAP" "$TOKEN_A" k-swap; code_is 201 "swap-201"
jq -e '.reservations | length == 2' "$WORK/out" >/dev/null && ok "swap-two" || { bad "swap-two" "$(cat "$WORK/out")"; exit 1; }
# Failing batch rolls back everything.
MOVES_BAD="{\"moves\":[{\"reference\":\"$REF_MA\",\"table_id\":\"t_2\"},{\"reference\":\"$REF_MB\",\"table_id\":\"t_1\",\"party_size\":99}]}"
req POST /reservation-moves "$MOVES_BAD" "$TOKEN_A" k-swap-bad; code_is 422 "swap-bad-422"
req GET "/reservations/$REF_MA" '' "$TOKEN_A"; code_is 200 "rollback-check-200"
jq -e '.table_id == "t_2" and .starts_at_local == "2027-05-06T21:30"' "$WORK/out" >/dev/null && ok "rollback-kept" || { bad "rollback-kept" "$(cat "$WORK/out")"; exit 1; }
# Input-order precedence: first move's non-occupancy error wins.
MOVES_PREC="{\"moves\":[{\"reference\":\"$REF_MA\",\"party_size\":0},{\"reference\":\"$REF_MB\",\"table_id\":\"t_ghost\"}]}"
req POST /reservation-moves "$MOVES_PREC" "$TOKEN_A" k-prec; code_is 422 "precedence-422"
# Batch replay returns the original success.
req POST /reservation-moves "$MOVES_SWAP" "$TOKEN_A" k-swap; code_is 200 "batch-replay-200"
echo "== idempotency scopes/identity =="
req POST /auth/signup '{"email":"b@example.com","password":"correct horse","display_name":"B"}'; code_is 201 "signup-b-201"
TOKEN_C=$(jq -r .token "$WORK/out")
# Same key string under another user is independent (free Friday slot).
FREE_BODY='{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-07T19:00","party_size":4}'
req POST /reservations "$FREE_BODY" "$TOKEN_C" k-create-a; code_is 201 "cross-user-201"
# Same key+body on the other path is a different request.
req POST /reservation-moves "$CREATE_ADJ" "$TOKEN_C" k-create-a; code_is 422 "cross-path-422"
# Key order/whitespace-insensitive identity incl. unknown fields.
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-05-07T18:00","party_size":2,"z_extra":true}' "$TOKEN_C" k-canon; code_is 201 "canon-first-201"
req POST /reservations '  {"z_extra":true,"party_size":2,"starts_at_local":"2027-05-07T18:00","table_id":"t_1","restaurant_id":"r_anker" }' "$TOKEN_C" k-canon; code_is 200 "canon-replay-200"
# Different body same key is reuse even when otherwise invalid.
req POST /reservations '{}' "$TOKEN_C" k-canon; code_is 409 "reuse-409"; code_has "reuse-code" "idempotency_key_reuse"
# Failed key stays reusable (grid error first, then the still-free slot).
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-05-07T18:15","party_size":2}' "$TOKEN_C" k-fail1; code_is 422 "fail-first-422"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-05-07T20:30","party_size":4}' "$TOKEN_C" k-fail1; code_is 201 "fail-reuse-201"

echo "== 50 identical concurrent keys: exactly one 201 =="
CONC_BODY='{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-05-07T21:30","party_size":2}'
N201=0; N200=0
for i in $(seq 1 50); do
  (
    C=$(curl -s -o "$WORK/conc-$i" -w '%{http_code}' -H "Authorization: Bearer $TOKEN_C" -H 'Idempotency-Key: k-conc-50' -H 'Content-Type: application/json' -d "$CONC_BODY" -X POST "$BASE/reservations")
    echo "$C" > "$WORK/conc-$i.code"
  ) &
done
wait
for i in $(seq 1 50); do
  case "$(cat "$WORK/conc-$i.code")" in
    201) N201=$((N201+1));;
    200) N200=$((N200+1));;
    *) bad "conc-$i" "got $(cat "$WORK/conc-$i.code"): $(cat "$WORK/conc-$i")"; exit 1;;
  esac
done
[ "$N201" = "1" ] && [ "$N200" = "49" ] && ok "conc-once" || { bad "conc-once" "201=$N201 200=$N200"; exit 1; }
REFS=$(jq -r .reference "$WORK"/conc-* 2>/dev/null | sort -u | wc -l)
[ "$REFS" = "1" ] && ok "conc-one-ref" || { bad "conc-one-ref" "refs=$REFS"; exit 1; }

echo "== numeric UTC export/import + seeded snapshots =="
UTC_FIX='{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],"restaurants":[{"id":"r_utc","name":"U","timezone":"UTC","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t1","label":"1","capacity":2}]}],"reservations":[{"id":"s1","reference":"UTC001","user_id":"u1","restaurant_id":"r_utc","table_id":"t1","starts_at_local":"2027-05-06T19:00","party_size":2}]}'
req POST /_test/reset "$UTC_FIX"; code_is 204 "utc-reset-204"
curl -s "$BASE/_test/export" > "$WORK/export.json"
grep -q '+00:00' "$WORK/export.json" && ok "numeric-utc" || { bad "numeric-utc" "$(head -c 300 "$WORK/export.json")"; exit 1; }
CP2=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/_test/import" -H 'Content-Type: application/json' -d @"$WORK/export.json")
[ "$CP2" = "204" ] && ok "utc-reimport-204" || { bad "utc-reimport" "$CP2"; exit 1; }
NULL_FIX='{"users":[],"restaurants":[{"id":"r_closed","name":"C","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[],"tables":[{"id":"t1","label":"","capacity":2}]}],"reservations":[]}'
req POST /_test/reset "$NULL_FIX"; code_is 204 "null-reset-204"
curl -s "$BASE/_test/export" > "$WORK/export2.json"
CP3=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/_test/import" -H 'Content-Type: application/json' -d @"$WORK/export2.json")
[ "$CP3" = "204" ] && ok "null-reimport-204" || { bad "null-reimport" "$CP3"; exit 1; }
OVER_FIX='{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t1","label":"1","capacity":4}]}],"reservations":[{"id":"s1","reference":"OVERC1","user_id":"u1","restaurant_id":"r1","table_id":"t1","starts_at_local":"2027-05-06T19:00","party_size":5}]}'
req POST /_test/reset "$OVER_FIX"; code_is 204 "over-reset-204"
curl -s "$BASE/_test/export" > "$WORK/export3.json"
CP4=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/_test/import" -H 'Content-Type: application/json' -d @"$WORK/export3.json")
[ "$CP4" = "204" ] && ok "over-reimport-204" || { bad "over-reimport" "$CP4"; exit 1; }
req POST /_test/reset "$FIXTURE"; code_is 204 "fixture-final-204"

echo "== unknown API path is JSON 404 =="
req GET '/nope'; code_is 404 "unknown-404"; code_has "unknown-404-code" "not_found"

echo
echo "RESULT pass=$PASS fail=$FAIL"
[ "$FAIL" = "0" ]
