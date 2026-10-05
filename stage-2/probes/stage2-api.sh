#!/bin/sh
# stage2-api.sh - reusable HTTP image probe for the stage-2 service.
#
# Usage: sh stage-2/probes/stage2-api.sh <base-url> <private-work-directory>
#
# Covers inherited stage-1 API behavior (envelope, content types, health,
# reset, auth incl. duplicate/short-password/bad-email/wrong-type logins,
# public/protected paths, unknown fields and params, strict body/query/local/
# date formats, pair and single seeds, half-open adjacency, DST gaps/folds/
# absolute duration, numeric UTC, past-cutoff cancel/PATCH) plus stage-2
# combined tables (pair selection matrix with both/empty/duplicate/>2/
# undeclared/unknown/foreign/wrong-type/missing, reversed canonical order,
# summed capacity, grid/hours errors, failed-key reuse, options order and
# capacity, nontransitivity, pair overlap incl. seeded pairs, singleton/pair
# schemas, PATCH empty/party/time/time+party retention and reversed no-op,
# cancel releases both members, atomic moves incl. pair batches, shared-member
# rollback with record and key checks, input-order errors, no-op/replay
# identity, idempotency missing/empty/length scopes, canonical replay, reuse
# before validation, cross-path key independence and valid reuse, failed-key
# reuse, replay after cancel, 50-way races, competing writes, same-image
# export/import roundtrip with replay preservation and invalid-import
# atomicity).
#
# Private artifacts (tokens, exports) stay in <work-directory>, never printed
# or committed. Stdout carries PASS/FAIL lines and codes only. Exit status is
# nonzero if any check fails. STAGE2_API_SABOTAGE=1 corrupts one expectation
# to prove the guard counts failures (probe self-test only, never product).
set -eu
BASE="$1"
WORK="$2"
mkdir -p "$WORK"
PASS=0
FAIL=0

ok() { PASS=$((PASS+1)); echo "PASS $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

# req METHOD PATH [BODY] [TOKEN] [KEY] -> $WORK/out (body), $WORK/status.
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
jget() { jq -r "$1" "$WORK/out" 2>/dev/null || true; }

SABOTAGE=${STAGE2_API_SABOTAGE:-0}

echo "== health + content type =="
req GET /health; code_is 200 "health-200"
grep -q '"status":"ok"' "$WORK/out" || { bad "health-body" "$(cat "$WORK/out")"; exit 1; }; ok "health-body"
curl -s -D "$WORK/hdrs" -o /dev/null "$BASE/health"
grep -qi 'application/json; charset=utf-8' "$WORK/hdrs" || { bad "health-ctype" "$(cat "$WORK/hdrs")"; exit 1; }; ok "health-ctype"
req GET /health
if [ "$SABOTAGE" = "1" ]; then code_is 404 "sabotage-health"; else code_is 200 "health-again"; fi

echo "== reset fixture (pairs) =="
FIXTURE='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse ada","display_name":"Ada"},{"id":"u_bob","email":"bob@example.com","password":"correct horse bob","display_name":"Bob"}],"restaurants":[{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"},{"weekday":"fri","opens":"18:00","closes":"23:30"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],"combinable":[["t_1","t_2"],["t_2","t_3"]]},{"id":"r_zwei","name":"Zwei","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t_9","label":"9","capacity":2},{"id":"t_8","label":"8","capacity":4}]}],"reservations":[]}'
req POST /_test/reset "$FIXTURE"; code_is 204 "reset-204"
[ -s "$WORK/out" ] && { bad "reset-empty" "reset has a body"; exit 1; }; ok "reset-empty"

echo "== auth =="
req POST /auth/signup '{"email":"n@example.com","password":"correct horse","display_name":"N","admin":true}'; code_is 201 "signup-201"
TOKEN_N=$(jget .token); [ -n "$TOKEN_N" ] || { bad "signup-token" "empty"; exit 1; }; ok "signup-token"
req POST /auth/signup '{"email":"n@example.com","password":"correct horse","display_name":"N2"}'; code_is 409 "dup-409"; code_has "dup-code" "email_taken"
req POST /auth/signup '{"email":"s@example.com","password":"short","display_name":"S"}'; code_is 422 "shortpw-422"
req POST /auth/signup '{"email":"nope","password":"correct horse","display_name":"X"}'; code_is 422 "bademail-422"
req POST /auth/signup '{"email":7,"password":"correct horse","display_name":"X"}'; code_is 400 "emailtype-400"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "login-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "login-token" "empty"; exit 1; }; ok "login-token"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "login2-200"
TOKEN_A2=$(jget .token)
[ "$TOKEN_A" != "$TOKEN_A2" ] && ok "tokens-distinct" || { bad "tokens-distinct" "same"; exit 1; }
req POST /auth/login '{"email":"ada@example.com","password":"nope nope nope"}'; code_is 401 "badpw-401"; code_has "badpw-code" "unauthenticated"
req POST /auth/login '{"email":"ghost@example.com","password":"correct horse ada"}'; code_is 401 "ghost-401"
req POST /auth/login '{"email":"ada@example.com"}'; code_is 422 "login-missing-422"
req GET /reservations; code_is 401 "noauth-401"; code_has "noauth-code" "unauthenticated"
req GET /reservations/nonexist "" "y"; code_is 401 "badauth-401"
AUTH_BAD="Bearer bogus"
curl -s -o "$WORK/out" -w '%{http_code}' -H "Authorization: $AUTH_BAD" "$BASE/reservations" > "$WORK/status"; code_is 401 "bogus-401"

echo "== public paths + unknown fields/params =="
req GET /restaurants; code_is 200 "list-200"
jq -e '.restaurants == [{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin"},{"id":"r_zwei","name":"Zwei","timezone":"Europe/Berlin"}]' "$WORK/out" >/dev/null && ok "list-shape" || { bad "list-shape" "$(cat "$WORK/out")"; exit 1; }
req GET '/restaurants?zzz=1'; code_is 200 "list-extraparam-200"
req GET /restaurants/r_anker; code_is 200 "detail-200"
jq -e '.combinable == [["t_1","t_2"],["t_2","t_3"]]' "$WORK/out" >/dev/null && ok "detail-combinable" || { bad "detail-combinable" "$(cat "$WORK/out")"; exit 1; }
req GET /restaurants/r_nope; code_is 404 "detail-404"; code_has "detail-404-code" "not_found"
req GET '/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2&zzz=9'; code_is 200 "avail-extraparam-200"

echo "== strict params =="
req GET '/availability?restaurant_id=r_anker&date=2027-06-17'; code_is 422 "avail-missing-422"
for q in 'party_size=1e9' 'party_size=4.0' 'party_size=+4' 'party_size=0' 'party_size=-2'; do
  req GET "/availability?restaurant_id=r_anker&date=2027-06-17&$q"; code_is 422 "avail-$q-422" || true
done
req GET '/availability?restaurant_id=r_anker&date=2027-13-40&party_size=2'; code_is 422 "avail-baddate-422"
req GET '/availability?restaurant_id=r_anker&date=2027-06-17T18:00&party_size=2'; code_is 422 "avail-datetime-422"
req GET '/availability?restaurant_id=r_nope&date=2027-06-17&party_size=2'; code_is 404 "avail-norest-404"
req GET '/availability?restaurant_id=r_anker&date=2027-06-19&party_size=2'; code_is 200 "closed-200"
jq -e '.slots == []' "$WORK/out" >/dev/null && ok "closed-empty" || { bad "closed-empty" "$(cat "$WORK/out")"; exit 1; }

echo "== options on fresh state =="
req GET '/availability?restaurant_id=r_anker&date=2027-06-17&party_size=6'; code_is 200 "opt6-200"
jq -e '.slots[0].available_table_ids == []' "$WORK/out" >/dev/null && ok "opt6-singles-empty" || { bad "opt6-singles-empty" "$(cat "$WORK/out")"; exit 1; }
jq -e '.slots[0].available_options == [{"table_ids":["t_1","t_2"],"capacity":6},{"table_ids":["t_2","t_3"],"capacity":8}]' "$WORK/out" >/dev/null && ok "opt6-order-cap" || { bad "opt6-order-cap" "$(cat "$WORK/out")"; exit 1; }

echo "== strict bodies =="
CREATE1='{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T20:30","party_size":2}'
req POST /reservations "$CREATE1" "$TOKEN_A" k-body-1; code_is 201 "single-create-201"
REF_S1=$(jget .reference)
jq -e '.table_ids == ["t_3"] and .table_id == "t_3"' "$WORK/out" >/dev/null && ok "single-schema" || { bad "single-schema" "$(cat "$WORK/out")"; exit 1; }
grep -q '"created_at":"[^"]*+00:00"' "$WORK/out" && ok "numeric-utc" || { bad "numeric-utc" "$(cat "$WORK/out")"; exit 1; }
for t in '"2"' 'true' '2.5' '0' '-1' 'null'; do
  req POST /reservations "{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-06-17T18:00\",\"party_size\":$t}" "$TOKEN_A" "k-party-$t"; code_is 422 "party-$t-422" || true
done
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17 18:00","party_size":2}' "$TOKEN_A" k-local-space; code_is 422 "local-space-422"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00:00","party_size":2}' "$TOKEN_A" k-local-sec; code_is 422 "local-sec-422"
req POST /reservations '{"restaurant_id":"r_anker","table_id":5,"starts_at_local":"2027-06-17T18:00","party_size":2}' "$TOKEN_A" k-tabletype; code_is 400 "tabletype-400"
req POST /reservations '{"restaurant_id":"r_nope","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}' "$TOKEN_A" k-norest; code_is 404 "create-norest-404"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_9","starts_at_local":"2027-06-17T18:00","party_size":2}' "$TOKEN_A" k-notable; code_is 404 "create-notable-404"
req POST /reservations '{oops' "$TOKEN_A" k-unparse; code_is 400 "unparse-400"
req POST /reservations '[]' "$TOKEN_A" k-array; code_is 400 "array-400"

echo "== pair selection matrix =="
pair_case() { req POST /reservations "$2" "$TOKEN_A" "k-pm-$1"; code_is "$3" "pair-$1-$3" && code_has "pair-$1-code" "$4"; }
pair_case both '{"restaurant_id":"r_anker","table_id":"t_1","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 422 validation_failed
pair_case empty '{"restaurant_id":"r_anker","table_ids":[],"starts_at_local":"2027-06-17T18:00","party_size":2}' 422 validation_failed
pair_case dup '{"restaurant_id":"r_anker","table_ids":["t_1","t_1"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 422 validation_failed
pair_case three '{"restaurant_id":"r_anker","table_ids":["t_1","t_2","t_3"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 422 combination_not_allowed
pair_case undeclared '{"restaurant_id":"r_anker","table_ids":["t_1","t_3"],"starts_at_local":"2027-06-17T18:00","party_size":6}' 422 combination_not_allowed
pair_case unknown '{"restaurant_id":"r_anker","table_ids":["t_1","t_x"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 404 not_found
pair_case foreign '{"restaurant_id":"r_anker","table_ids":["t_1","t_9"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 404 not_found
pair_case ids-string '{"restaurant_id":"r_anker","table_ids":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}' 400 malformed_request
pair_case member-number '{"restaurant_id":"r_anker","table_ids":["t_1",4],"starts_at_local":"2027-06-17T18:00","party_size":2}' 400 malformed_request
pair_case id-array '{"restaurant_id":"r_anker","table_id":["t_1"],"starts_at_local":"2027-06-17T18:00","party_size":2}' 400 malformed_request
pair_case missing '{"restaurant_id":"r_anker","starts_at_local":"2027-06-17T18:00","party_size":2}' 422 validation_failed
pair_case overcap '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":7}' 422 party_exceeds_capacity
pair_case offgrid '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:15","party_size":2}' 422 not_on_slot_grid
pair_case outside '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T17:30","party_size":2}' 422 outside_opening_hours
req POST /reservations '{"restaurant_id":"r_anker","table_ids":["t_2","t_1"],"starts_at_local":"2027-06-17T18:00","party_size":2}' "$TOKEN_A" k-pm-rev; code_is 201 "pair-reversed-201"
jq -e '.table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "pair-reversed-canonical" || { bad "pair-reversed-canonical" "$(cat "$WORK/out")"; exit 1; }
req POST /reservations '{"restaurant_id":"r_anker","table_ids":["t_1","t_3"],"starts_at_local":"2027-06-17T21:00","party_size":2}' "$TOKEN_A" k-pm-fail; code_is 422 "pair-fail-422"; code_has "pair-fail-code" "combination_not_allowed"
req POST /reservations '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T21:00","party_size":6}' "$TOKEN_A" k-pm-fail; code_is 201 "pair-fail-reuse-201"

echo "== seeds: pair confirmed + single cancelled =="
SEED='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse ada","display_name":"Ada"}],"restaurants":[{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],"combinable":[["t_1","t_2"],["t_2","t_3"]]}],"reservations":[{"id":"sP","reference":"SEEDP1","user_id":"u_ada","restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":6},{"id":"sC","reference":"SEEDC1","user_id":"u_ada","restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T18:00","party_size":2,"status":"cancelled"}]}'
req POST /_test/reset "$SEED"; code_is 204 "seed-reset-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "seed-login-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "seed-token" "empty"; exit 1; }; ok "seed-token"
req GET /reservations/SEEDP1 "" "$TOKEN_A"; code_is 200 "seedpair-lookup-200"
jq -e '.table_ids == ["t_1","t_2"] and (has("table_id") | not)' "$WORK/out" >/dev/null && ok "seedpair-schema" || { bad "seedpair-schema" "$(cat "$WORK/out")"; exit 1; }
req GET '/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2'; code_is 200 "seedavail-200"
jq -e '[.slots[] | select(.starts_at_local=="2027-06-17T18:00")][0].available_table_ids == ["t_3"]' "$WORK/out" >/dev/null && ok "seedpair-blocks" || { bad "seedpair-blocks" "$(cat "$WORK/out")"; exit 1; }
req POST /_test/reset "$FIXTURE"; code_is 204 "fixture-restore-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "relogin-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "relogin-token" "empty"; exit 1; }; ok "relogin-token"
req POST /auth/login '{"email":"bob@example.com","password":"correct horse bob"}'; code_is 200 "boblogin-200"
TOKEN_B=$(jget .token); [ -n "$TOKEN_B" ] || { bad "bob-token" "empty"; exit 1; }; ok "bob-token"

echo "== adjacency + options =="
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T19:00","party_size":2}' "$TOKEN_A" k-adj1; code_is 201 "adj-first-201"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T20:30","party_size":2}' "$TOKEN_A" k-adj2; code_is 201 "adjacent-201"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-17T20:00","party_size":2}' "$TOKEN_A" k-adj3; code_is 409 "overlap-409"; code_has "overlap-code" "table_unavailable"

echo "== DST gaps/folds + absolute duration =="
DSTFIX='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse ada","display_name":"Ada"}],"restaurants":[{"id":"r_ber","name":"B","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"04:00"}],"tables":[{"id":"t_1","label":"1","capacity":2}]},{"id":"r_ny","name":"N","timezone":"America/New_York","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":0,"opening_hours":[{"weekday":"sun","opens":"00:00","closes":"04:00"}],"tables":[{"id":"t_1","label":"1","capacity":2}]}],"reservations":[]}'
req POST /_test/reset "$DSTFIX"; code_is 204 "dst-reset-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "dst-login-200"
TOKEN_D=$(jget .token); [ -n "$TOKEN_D" ] || { bad "dst-token" "empty"; exit 1; }; ok "dst-token"
req GET '/availability?restaurant_id=r_ber&date=2026-03-29&party_size=2'; code_is 200 "spring-grid-200"
grep -q '2026-03-29T02:' "$WORK/out" && { bad "spring-skip" "skipped wall present"; exit 1; }; ok "spring-skip"
req POST /reservations '{"restaurant_id":"r_ber","table_id":"t_1","starts_at_local":"2026-03-29T02:30","party_size":2}' "$TOKEN_D" k-gap; code_is 422 "gap-422"; code_has "gap-code" "invalid_local_time"
req GET '/availability?restaurant_id=r_ber&date=2026-10-25&party_size=2'; code_is 200 "fall-grid-200"
[ "$(jq -r '[.slots[].starts_at_local] | map(select(.=="2026-10-25T02:30")) | length' "$WORK/out")" = "1" ] && ok "fall-once" || { bad "fall-once" "$(cat "$WORK/out")"; exit 1; }
jq -e '[.slots[] | select(.starts_at_local=="2026-10-25T02:30")][0].starts_at == "2026-10-25T02:30:00+02:00"' "$WORK/out" >/dev/null && ok "fall-first" || { bad "fall-first" "$(cat "$WORK/out")"; exit 1; }
req POST /reservations '{"restaurant_id":"r_ny","table_id":"t_1","starts_at_local":"2026-11-01T01:30","party_size":2}' "$TOKEN_D" k-fold; code_is 201 "fold-book-201"
jq -e '.starts_at == "2026-11-01T01:30:00-04:00" and .ends_at == "2026-11-01T02:00:00-05:00"' "$WORK/out" >/dev/null && ok "absolute-duration" || { bad "absolute-duration" "$(cat "$WORK/out")"; exit 1; }
req POST /_test/reset "$FIXTURE"; code_is 204 "fixture-restore2-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "relogin2-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "relogin2-token" "empty"; exit 1; }; ok "relogin2-token"
req POST /auth/login '{"email":"bob@example.com","password":"correct horse bob"}'; code_is 200 "boblogin2-200"
TOKEN_B=$(jget .token); [ -n "$TOKEN_B" ] || { bad "bob2-token" "empty"; exit 1; }; ok "bob2-token"

echo "== cutoff (past booking) =="
PASTFIX='{"users":[{"id":"u_ada","email":"ada@example.com","password":"correct horse ada","display_name":"Ada"}],"restaurants":[{"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],"combinable":[["t_1","t_2"],["t_2","t_3"]]}],"reservations":[{"id":"sP","reference":"PAST01","user_id":"u_ada","restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2020-01-02T19:00","party_size":2}]}'
req POST /_test/reset "$PASTFIX"; code_is 204 "past-reset-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "past-login-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "past-token" "empty"; exit 1; }; ok "past-token"
req POST /reservations/PAST01/cancel "" "$TOKEN_A"; code_is 409 "past-cancel-409"; code_has "past-cancel-code" "cutoff_passed"
req PATCH /reservations/PAST01 '{"party_size":1}' "$TOKEN_A"; code_is 409 "past-patch-409"
req POST /_test/reset "$FIXTURE"; code_is 204 "fixture-restore3-204"
req POST /auth/login '{"email":"ada@example.com","password":"correct horse ada"}'; code_is 200 "relogin3-200"
TOKEN_A=$(jget .token); [ -n "$TOKEN_A" ] || { bad "relogin3-token" "empty"; exit 1; }; ok "relogin3-token"
req POST /auth/login '{"email":"bob@example.com","password":"correct horse bob"}'; code_is 200 "boblogin3-200"
TOKEN_B=$(jget .token); [ -n "$TOKEN_B" ] || { bad "bob3-token" "empty"; exit 1; }; ok "bob3-token"

echo "== idempotency scopes =="
req POST /reservations '{"restaurant_id":"r_nope"}' "$TOKEN_A"; code_is 400 "nokey-400"; code_has "nokey-code" "missing_idempotency_key"
curl -s -o "$WORK/out" -w '%{http_code}' -H 'Content-Type: application/json' -d '{}' -H "Authorization: Bearer $TOKEN_A" -H 'Idempotency-Key:' -X POST "$BASE/reservations" > "$WORK/status"; code_is 400 "emptykey-400"
LONGKEY=$(python3 -c 'print("k"*256)')
req POST /reservations '{}' "$TOKEN_A" "$LONGKEY"; code_is 422 "longkey-422"
CR1='{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}'
req POST /reservations "$CR1" "$TOKEN_A" k-scope; code_is 201 "first-201"
cp "$WORK/out" "$WORK/first.json"
REF_C1=$(jget .reference)
req POST /reservations ' { "party_size" : 2 , "table_id" : "t_1" , "starts_at_local" : "2027-06-17T18:00" , "restaurant_id" : "r_anker" } ' "$TOKEN_A" k-scope; code_is 200 "replay-200"
cmp -s "$WORK/out" "$WORK/first.json" && ok "replay-identical" || { bad "replay-identical" "differs"; exit 1; }
req POST /reservations '{"party_size":"many"}' "$TOKEN_A" k-scope; code_is 409 "reuse-409"; code_has "reuse-code" "idempotency_key_reuse"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-18T18:00","party_size":2}' "$TOKEN_B" k-scope; code_is 201 "user-scope-201"
req POST /reservation-moves '{"moves":[]}' "$TOKEN_A" k-scope; code_is 422 "path-scope-422"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_C1'"}]}' "$TOKEN_A" k-scope; code_is 201 "path-scope-valid-201"
cp "$WORK/out" "$WORK/pathscope-orig.json"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_C1'"}]}' "$TOKEN_A" k-scope; code_is 200 "path-scope-replay-200"
cmp -s "$WORK/out" "$WORK/pathscope-orig.json" && ok "path-scope-identical" || { bad "path-scope-identical" "differs"; exit 1; }
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":99}' "$TOKEN_A" k-fail1; code_is 422 "fail-422"
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T18:00","party_size":2}' "$TOKEN_A" k-fail1; code_is 201 "fail-reuse-201"

echo "== replay after mutation =="
req POST /reservations/$REF_C1/cancel "" "$TOKEN_A"; code_is 200 "cancel-c1-200"
req POST /reservations "$CR1" "$TOKEN_A" k-scope; code_is 200 "replay-cancelled-200"
cmp -s "$WORK/out" "$WORK/first.json" && ok "replay-immutable" || { bad "replay-immutable" "changed"; exit 1; }

echo "== 50 same-key race =="
RACE='{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T21:00","party_size":6}'
i=0; while [ "$i" -lt 50 ]; do i=$((i+1)); ( curl -s -o "$WORK/race$i" -w '%{http_code}' -H 'Content-Type: application/json' -d "$RACE" -H "Authorization: Bearer $TOKEN_A" -H 'Idempotency-Key: k-race50' -X POST "$BASE/reservations" > "$WORK/race$i.code" ) & done; wait || true
n201=$(cat "$WORK"/race*.code 2>/dev/null | grep -o 201 | wc -l); n200=$(cat "$WORK"/race*.code 2>/dev/null | grep -o 200 | wc -l)
[ "$n201" = "1" ] && [ "$n200" = "49" ] && ok "race-1x201-49x200" || { bad "race-counts" "201=$n201 200=$n200"; exit 1; }
REF_RACE=$(jq -r .reference "$WORK/race1" 2>/dev/null || true)
[ -n "$REF_RACE" ] || REF_RACE=$(jq -r .reference "$WORK/race2" 2>/dev/null || true)
first200=""; same=1
for f in "$WORK"/race[0-9]*; do case "$f" in *.code) continue;; esac; [ -z "$first200" ] && first200="$f"; cmp -s "$first200" "$f" || same=0; done
[ "$same" = "1" ] && ok "race-identical" || { bad "race-identical" "bodies differ"; exit 1; }

echo "== competing singles on one table =="
CP_A='{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T18:00","party_size":2}'
( curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "$CP_A" -H "Authorization: Bearer $TOKEN_A" -H 'Idempotency-Key: k-comp-a' -X POST "$BASE/reservations" > "$WORK/comp-a" ) &
( curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "$CP_A" -H "Authorization: Bearer $TOKEN_B" -H 'Idempotency-Key: k-comp-b' -X POST "$BASE/reservations" > "$WORK/comp-b" ) &
wait || true
[ "$(sort "$WORK/comp-a" "$WORK/comp-b" | tr '
' ' ')" = "201 409 " ] && ok "competing-exactly-one" || { bad "competing-exactly-one" "got $(cat "$WORK/comp-a") $(cat "$WORK/comp-b")"; exit 1; }

echo "== pair PATCH retention =="
PAIR='{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T19:30","party_size":4}'
req POST /reservations "$PAIR" "$TOKEN_A" k-pair1; code_is 201 "pair-create-201"
REF_P=$(jget .reference)
jq -e '.table_ids == ["t_1","t_2"] and (has("table_id") | not)' "$WORK/out" >/dev/null && ok "pair-schema" || { bad "pair-schema" "$(cat "$WORK/out")"; exit 1; }
cp "$WORK/out" "$WORK/pair-orig.json"
req PATCH /reservations/$REF_P '{}' "$TOKEN_A"; code_is 200 "patch-empty-200"
cmp -s "$WORK/out" "$WORK/pair-orig.json" && ok "patch-empty-identical" || { bad "patch-empty-identical" "changed"; exit 1; }
req PATCH /reservations/$REF_P '{"party_size":3}' "$TOKEN_A"; code_is 200 "patch-party-200"
jq -e '.party_size == 3 and .table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "patch-party-retain" || { bad "patch-party-retain" "$(cat "$WORK/out")"; exit 1; }
req GET /reservations/$REF_P "" "$TOKEN_A"; code_is 200 "pre-reversal-200"
cp "$WORK/out" "$WORK/pre-reversal.json"
req PATCH /reservations/$REF_P '{"table_ids":["t_2","t_1"]}' "$TOKEN_A"; code_is 200 "patch-reversed-200"
cmp -s "$WORK/out" "$WORK/pre-reversal.json" && ok "patch-reversed-identical" || { bad "patch-reversed-identical" "changed"; exit 1; }
req PATCH /reservations/$REF_P '{"starts_at_local":"2027-06-18T19:30"}' "$TOKEN_A"; code_is 200 "patch-time-200"
jq -e '.starts_at_local == "2027-06-18T19:30" and .ends_at == "2027-06-18T21:00:00+02:00" and .table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "patch-time-retain" || { bad "patch-time-retain" "$(cat "$WORK/out")"; exit 1; }
req GET '/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2'; code_is 200 "avail-oldslot-200"
jq -e '[.slots[] | select(.starts_at_local=="2027-06-17T19:30")][0].available_table_ids == ["t_1","t_2","t_3"]' "$WORK/out" >/dev/null && ok "oldslot-freed" || { bad "oldslot-freed" "$(cat "$WORK/out")"; exit 1; }
req GET '/availability?restaurant_id=r_anker&date=2027-06-18&party_size=2'; code_is 200 "avail-newslot-200"
jq -e '[.slots[] | select(.starts_at_local=="2027-06-18T19:30")][0].available_table_ids == ["t_3"]' "$WORK/out" >/dev/null && ok "newslot-occupied" || { bad "newslot-occupied" "$(cat "$WORK/out")"; exit 1; }
req PATCH /reservations/$REF_P '{"starts_at_local":"2027-06-18T20:30","party_size":5}' "$TOKEN_A"; code_is 200 "patch-timeparty-200"
jq -e '.starts_at_local == "2027-06-18T20:30" and .party_size == 5 and .table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "patch-timeparty-retain" || { bad "patch-timeparty-retain" "$(cat "$WORK/out")"; exit 1; }
req PATCH /reservations/$REF_P '{"table_id":"t_1","table_ids":["t_1"]}' "$TOKEN_A"; code_is 422 "patch-both-422"

echo "== cancel releases both =="
req POST /reservations/$REF_P/cancel "" "$TOKEN_A"; code_is 200 "pair-cancel-200"
req GET '/availability?restaurant_id=r_anker&date=2027-06-18&party_size=4'; code_is 200 "avail-fri-200"
jq -e '[.slots[] | select(.starts_at_local=="2027-06-18T20:30")][0].available_table_ids == ["t_2","t_3"]' "$WORK/out" >/dev/null && ok "cancel-frees-members" || { bad "cancel-frees-members" "$(cat "$WORK/out")"; exit 1; }
jq -e '[.slots[] | select(.starts_at_local=="2027-06-18T20:30")][0].available_options | map(.table_ids) | contains([["t_1","t_2"]])' "$WORK/out" >/dev/null && ok "cancel-frees-pair" || { bad "cancel-frees-pair" "$(cat "$WORK/out")"; exit 1; }

echo "== atomic moves =="
req POST /reservation-moves '{"moves":[{"reference":"'$REF_C1'","table_id":"t_2"}]}' "$TOKEN_A" k-mv-cancelled; code_is 409 "moves-cancelled-409"; code_has "moves-cancelled-code" "reservation_cancelled"
CR2='{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"2027-06-17T19:30","party_size":2}'
req POST /reservations "$CR2" "$TOKEN_A" k-mv-a; code_is 201 "mv-setup-a-201"
REF_MA=$(jget .reference)
CR3='{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T19:30","party_size":2}'
req POST /reservations "$CR3" "$TOKEN_A" k-mv-b; code_is 201 "mv-setup-b-201"
REF_MB=$(jget .reference)
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'","table_id":"t_2"},{"reference":"'$REF_MB'","table_id":"t_1"}]}' "$TOKEN_A" k-mv-swap; code_is 201 "swap-201"
jq -e '.reservations | map(.reference) == ["'$REF_MA'","'$REF_MB'"]' "$WORK/out" >/dev/null && ok "swap-order" || { bad "swap-order" "$(cat "$WORK/out")"; exit 1; }
cp "$WORK/out" "$WORK/swap-orig.json"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'","table_id":"t_2"},{"reference":"'$REF_MB'","table_id":"t_1"}]}' "$TOKEN_A" k-mv-swap; code_is 200 "swap-replay-200"
cmp -s "$WORK/out" "$WORK/swap-orig.json" && ok "swap-replay-identical" || { bad "swap-replay-identical" "differs"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"NOPE01"}]}' "$TOKEN_A" k-mv-unknown; code_is 404 "moves-unknown-404"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'"},{"reference":"'$REF_MA'"}]}' "$TOKEN_A" k-mv-dup; code_is 422 "moves-dup-422"
req POST /reservation-moves '{"moves":[]}' "$TOKEN_A" k-mv-empty; code_is 422 "moves-empty-422"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'","table_id":"t_9"}]}' "$TOKEN_A" k-mv-badtable; code_is 404 "moves-badtable-404"
LOOKUP_BEFORE=$(curl -s -H "Authorization: Bearer $TOKEN_A" "$BASE/reservations/$REF_MA")
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'","table_id":"t_2"},{"reference":"NOPE02"}]}' "$TOKEN_A" k-mv-order; code_is 404 "moves-order-404"
LOOKUP_AFTER=$(curl -s -H "Authorization: Bearer $TOKEN_A" "$BASE/reservations/$REF_MA")
[ "$LOOKUP_BEFORE" = "$LOOKUP_AFTER" ] && ok "moves-rollback" || { bad "moves-rollback" "mutated"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'"}]}' "$TOKEN_A" k-mv-noop; code_is 201 "noop-201"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MA'"}]}' "$TOKEN_A" k-mv-noop; code_is 200 "noop-replay-200"
req POST /reservation-moves '{"moves":[{"reference":"NOPE03"}]}' "$TOKEN_A" k-mv-fail; code_is 404 "mvfail-404"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_MB'","party_size":1}]}' "$TOKEN_A" k-mv-fail; code_is 201 "mvfail-reuse-201"

echo "== pair batch retention (Friday) =="
req POST /reservations '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-18T19:30","party_size":4}' "$TOKEN_A" k-pb-a; code_is 201 "pb-pair-201"
REF_PB=$(jget .reference)
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-18T19:30","party_size":2}' "$TOKEN_A" k-pb-s; code_is 201 "pb-single-201"
REF_PS=$(jget .reference)
req POST /reservations '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"2027-06-18T21:00","party_size":2}' "$TOKEN_A" k-pb-u; code_is 201 "pb-blocker-201"
req GET /reservations/$REF_PB "" "$TOKEN_A"; code_is 200 "pb-pre-200"
cp "$WORK/out" "$WORK/pb-pre.json"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PB'","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-18T21:00"}]}' "$TOKEN_A" k-pb-conflict; code_is 409 "pb-shared-conflict-409"; code_has "pb-shared-conflict-code" "table_unavailable"
req GET /reservations/$REF_PB "" "$TOKEN_A"; code_is 200 "pb-post-200"
cmp -s "$WORK/out" "$WORK/pb-pre.json" && ok "pb-rollback-records" || { bad "pb-rollback-records" "mutated"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PB'"}]}' "$TOKEN_A" k-pb-conflict; code_is 201 "pb-key-reusable-201"
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PB'"}]}' "$TOKEN_A" k-pb-noop; code_is 201 "pb-noop-201"
req GET /reservations/$REF_PB "" "$TOKEN_A"; code_is 200 "pb-cur-200"
cmp -s "$WORK/out" "$WORK/pb-pre.json" && ok "pb-noop-identical" || { bad "pb-noop-identical" "changed"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PB'","party_size":5}]}' "$TOKEN_A" k-pb-party; code_is 201 "pb-party-201"
jq -e '.reservations[0].party_size == 5 and .reservations[0].table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "pb-party-retain" || { bad "pb-party-retain" "$(cat "$WORK/out")"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PB'","starts_at_local":"2027-06-18T20:30"}]}' "$TOKEN_A" k-pb-time; code_is 201 "pb-time-201"
jq -e '.reservations[0].starts_at_local == "2027-06-18T20:30" and .reservations[0].table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "pb-time-retain" || { bad "pb-time-retain" "$(cat "$WORK/out")"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PS'","party_size":1},{"reference":"'$REF_PB'"}]}' "$TOKEN_A" k-pb-mix; code_is 201 "pb-mix-201"
cp "$WORK/out" "$WORK/pb-mixed-orig.json"
jq -e '.reservations | map(.reference) == ["'$REF_PS'","'$REF_PB'"]' "$WORK/out" >/dev/null && ok "pb-mix-order" || { bad "pb-mix-order" "$(cat "$WORK/out")"; exit 1; }
jq -e '.reservations[0].table_id == "t_3" and .reservations[1].table_ids == ["t_1","t_2"]' "$WORK/out" >/dev/null && ok "pb-mix-schema" || { bad "pb-mix-schema" "$(cat "$WORK/out")"; exit 1; }
req POST /reservation-moves '{"moves":[{"reference":"'$REF_PS'","party_size":1},{"reference":"'$REF_PB'"}]}' "$TOKEN_A" k-pb-mix; code_is 200 "pb-mix-replay-200"
cmp -s "$WORK/out" "$WORK/pb-mixed-orig.json" && ok "pb-mix-identical" || { bad "pb-mix-identical" "differs"; exit 1; }

echo "== export/import roundtrip smoke =="
req GET /_test/export; code_is 200 "export-200"
cp "$WORK/out" "$WORK/export.json"
jq -e '.track == "tablekeeper" and .format_version == 1' "$WORK/out" >/dev/null && ok "export-envelope" || { bad "export-envelope" "shape"; exit 1; }
req POST /_test/reset '{"users":[],"restaurants":[],"reservations":[]}'; code_is 204 "wipe-204"
req POST /_test/import @"$WORK/export.json"; code_is 204 "import-204"
req POST /reservations "$CR1" "$TOKEN_A" k-scope; code_is 200 "postimport-replay-200"
cmp -s "$WORK/out" "$WORK/first.json" && ok "postimport-identical" || { bad "postimport-identical" "differs"; exit 1; }
req POST /_test/import '{"track":"nope","format_version":1,"state":{}}'; code_is 422 "badimport-422"
req POST /reservations "$CR1" "$TOKEN_A" k-scope; code_is 200 "afterbadimport-200"

echo
echo "RESULT pass=$PASS fail=$FAIL"
[ "$FAIL" = "0" ]
