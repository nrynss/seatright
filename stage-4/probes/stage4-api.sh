#!/bin/sh
# stage4-api.sh - independent stage-4 manager-repair HTTP API probe (P1A).
#
# Usage: sh stage-4/probes/stage4-api.sh BASE_URL WORK_DIR
#   (run with CWD set to stage-4; e.g. sh probes/stage4-api.sh URL DIR)
#
# Covers R307-350 repair preview/apply/closure proofs plus inherited
# auth/idempotency/private-state foundations: manager gates, strict
# explicit-offset intervals, hand-computed optimization tiers 1-3 with an
# independent Python odometer oracle and a global-vs-greedy trap,
# preview/apply exact deltas and receipts, stale/already-applied replays,
# series propagation, and closure availability/explain/enforcement.
# Recurring clock amendments belong to P1B, not this script. No
# native-import transfer, packaging, browser or harness claims.
#
# POSIX sh + curl + python3 stdlib only (no jq). Interface:
#   sh probes/stage4-api.sh BASE_URL WORK_DIR
# BASE is validated as an http(s) URL; WORK_DIR is created 0700 and all
# token-bearing artifacts are stored 0600 inside it. Stdout carries only
# check names, codes and counts, never tokens, passwords, bodies or export
# JSON. Python diagnostics go to the private tree. Every curl carries
# connect-timeout 5 and max-time 25. Every genuine assertion counts through
# expect() or pycheck(); any nonzero FAIL count exits nonzero.
# STAGE4_API_SABOTAGE=1 corrupts one expectation to prove guards count
# failures (probe self-test only).
set -u

BASE=${1:?usage: sh probes/stage4-api.sh BASE_URL WORK_DIR}
OUT=${2:?usage: sh probes/stage4-api.sh BASE_URL WORK_DIR}
case "$OUT" in
  ""|/) echo "FAIL bad-workdir"; exit 1 ;;
esac

umask 077
mkdir -p "$OUT" || { echo "FAIL: workdir not writable"; exit 1; }
chmod 0700 "$OUT"
[ -z "$(ls -A "$OUT" 2>/dev/null)" ] || { echo "FAIL: workdir not fresh, refusing to overwrite evidence"; exit 1; }
: > "$OUT/.wprobe" 2>/dev/null || { echo "FAIL: workdir not writable"; exit 1; }
rm -f "$OUT/.wprobe"
DIAG="$OUT/diag.log"
: > "$DIAG"
chmod 600 "$DIAG"
exec 2>>"$DIAG"
if ! python3 - "$BASE" <<'PYEOF2'
import sys, urllib.parse
u = urllib.parse.urlparse(sys.argv[1])
assert u.scheme in ("http", "https") and u.hostname, "need http(s) scheme+host"
PYEOF2
then
  echo "FAIL bad-base (BASE must be an http(s) URL with a host)"
  exit 1
fi
ART="$OUT/art"
mkdir -p "$ART"
chmod 0700 "$ART"
SCR="$OUT/scratch"
mkdir -p "$SCR"
chmod 0700 "$SCR"
TMP="$OUT/tmp"
mkdir -p "$TMP"
chmod 0700 "$TMP"
trap 'rm -rf "$SCR"' EXIT INT TERM

PASS=0
FAIL=0

expect() {
  if [ "$2" = "$3" ]; then
    echo "PASS: $1 ($2)"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (got $2, want $3)"
    FAIL=$((FAIL + 1))
  fi
}

CURL="curl -s --connect-timeout 5 --max-time 25"

py() {
  python3 "$@" 2>>"$DIAG"
}

pycheck() {
  name=$1; shift
  if py "$@"; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
}

# export_ok fetches the export envelope into $1. It requires HTTP 200 and a
# valid track/format_version/state envelope; anything else counts FAIL and
# aborts nonzero so later checks never run on a false valid baseline.
export_ok() {
  $CURL -o "$SCR/exp-tmp.json" -w '%{http_code}' "$BASE/_test/export" > "$SCR/exp-code" 2>>"$DIAG"
  code=$(cat "$SCR/exp-code")
  if [ "$code" = "200" ] && py -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)' "$SCR/exp-tmp.json"; then
    cp "$SCR/exp-tmp.json" "$1"
    chmod 600 "$1"
    echo 200
  else
    echo "FAIL: export $1 (code $code)"
    FAIL=$((FAIL + 1))
    exit 1
  fi
}

login() {
  $CURL -o "$ART/login.out" -w '%{http_code}' -X POST "$BASE/auth/login" \
    -H 'Content-Type: application/json' -d '{"email":"'"$1"'","password":"'"$2"'"}' > "$SCR/login.code" 2>>"$DIAG"
  chmod 600 "$ART/login.out"
  echo "$(cat "$SCR/login.code"):$(py -c 'import json; print(json.load(open("'"$ART"'/login.out")).get("token",""))' 2>>"$DIAG")"
}

apicode() {
  py -c 'import json,sys; print(json.load(open(sys.argv[1]))["error"]["code"])' "$1" 2>>"$DIAG"
}

# reset_world writes stdin fixture to a private file and resets through HTTP.
reset_world() {
  cat > "$ART/fixture.json"
  chmod 600 "$ART/fixture.json"
  $CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/_test/reset" \
    -H 'Content-Type: application/json' --data-binary @"$ART/fixture.json"
}

echo "== A foundation: reset/login/fixture order/pairs =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"Mgr"},
 {"id":"a","email":"a@x","password":"password12","display_name":"A"},
 {"id":"b","email":"b@x","password":"password12","display_name":"B"},
 {"id":"d","email":"d@x","password":"password12","display_name":"D"}],
 "restaurants": [
  {"id":"r_anker","name":"Zum Anker","timezone":"Europe/Berlin","slot_minutes":30,
   "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
   "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
   "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
   "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]},
  {"id":"r_other","name":"Other","timezone":"Europe/Berlin","slot_minutes":30,
   "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
   "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
   "tables":[{"id":"o_1","label":"1","capacity":4}],"manager_user_ids":["m"]}],
 "reservations": []}
EOF
)
expect reset-foundation "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: reset-foundation-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_A=$(login "a@x" "password12")
TOK_A_CODE=${TOK_A%%:*}; TOK_A=${TOK_A#*:}
if [ "$TOK_A_CODE" = "200" ] && [ -n "$TOK_A" ]; then echo "PASS: login-a (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-a"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_B=$(login "b@x" "password12")
TOK_B_CODE=${TOK_B%%:*}; TOK_B=${TOK_B#*:}
if [ "$TOK_B_CODE" = "200" ] && [ -n "$TOK_B" ]; then echo "PASS: login-b (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-b"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_D=$(login "d@x" "password12")
TOK_D_CODE=${TOK_D%%:*}; TOK_D=${TOK_D#*:}
if [ "$TOK_D_CODE" = "200" ] && [ -n "$TOK_D" ]; then echo "PASS: login-d (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-d"; FAIL=$((FAIL + 1)); exit 1; fi
# per-login 200+nonempty checks above already fail-fast on any bad login.
$CURL -o "$ART/rest.out" "$BASE/restaurants/r_anker" 2>>"$DIAG"
chmod 600 "$ART/rest.out"
pycheck rest-order-pairs - "$ART/rest.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert [t["id"] for t in d["tables"]] == ["t_1","t_2","t_3"], d["tables"]
assert d["combinable"] == [["t_1","t_2"],["t_2","t_3"]], d["combinable"]
print("REST-OK")
PYEOF
$CURL -o "$ART/av0.out" -G "$BASE/availability" --data-urlencode restaurant_id=r_anker --data-urlencode date=2027-06-17 --data-urlencode party_size=6 2>>"$DIAG"
chmod 600 "$ART/av0.out"
pycheck no-transitive-pair - "$ART/av0.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
for sl in d["slots"]:
    for o in sl["available_options"]:
        assert sorted(o["table_ids"]) != ["t_1","t_3"], sl
print("NOTRANS-OK")
PYEOF

echo "== B manager gates, paths, intervals, offsets =="
CLO='{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}'
post_replan() {
  # $1 restaurant, $2 token (empty = none), $3 key, $4 body.
  if [ -n "$2" ]; then
    $CURL -o "$ART/o.out" -w '%{http_code}' -X POST "$BASE/restaurants/$1/replans" \
      -H 'Content-Type: application/json' -H "Authorization: Bearer $2" -H "Idempotency-Key: $3" -d "$4" 2>>"$DIAG"
  else
    $CURL -o "$ART/o.out" -w '%{http_code}' -X POST "$BASE/restaurants/$1/replans" \
      -H 'Content-Type: application/json' -H "Idempotency-Key: $3" -d "$4" 2>>"$DIAG"
  fi
  chmod 600 "$ART/o.out"
}
expect preview-no-token "$(post_replan r_anker "" b-nt1 "$CLO")" 401
expect preview-bad-token "$(post_replan r_anker bogus b-nt2 "$CLO")" 401
expect preview-nonmanager "$(post_replan r_anker "$TOK_D" b-nm1 "$CLO")" 403
expect preview-missing-key "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d "$CLO")" 400
expect preview-unknown-restaurant "$(post_replan nosuch "$TOK_M" b-ur1 "$CLO")" 404
expect preview-unknown-table "$(post_replan r_anker "$TOK_M" b-ut1 '{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 404
expect preview-malformed "$(post_replan r_anker "$TOK_M" b-mm1 '{oops')" 400
expect preview-from-after-to "$(post_replan r_anker "$TOK_M" b-iv1 '{"table_id":"t_2","from":"2027-06-17T23:00:00+02:00","to":"2027-06-17T18:00:00+02:00"}')" 422
expect preview-from-equal-to "$(post_replan r_anker "$TOK_M" b-iv2 '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:00:00+02:00"}')" 422
expect preview-offset-hour24 "$(post_replan r_anker "$TOK_M" b-of1 '{"table_id":"t_2","from":"2027-06-17T18:00:00+24:00","to":"2027-06-17T19:00:00+02:00"}')" 422
expect preview-offset-min60 "$(post_replan r_anker "$TOK_M" b-of2 '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:60","to":"2027-06-17T19:00:00+02:00"}')" 422
expect preview-single-digit-hour "$(post_replan r_anker "$TOK_M" b-of3 '{"table_id":"t_2","from":"2027-06-17T8:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 422
expect preview-comma-fraction "$(post_replan r_anker "$TOK_M" b-of4 '{"table_id":"t_2","from":"2027-06-17T18:00:00,5+02:00","to":"2027-06-17T19:00:00+02:00"}')" 422
expect preview-dot-fraction "$(post_replan r_anker "$TOK_M" b-of5 '{"table_id":"t_2","from":"2027-06-17T18:00:00.5+02:00","to":"2027-06-17T19:00:00+02:00"}')" 201
expect preview-zulu "$(post_replan r_anker "$TOK_M" b-of6 '{"table_id":"t_2","from":"2027-06-17T16:00:00Z","to":"2027-06-17T19:00:00+02:00"}')" 201
expect preview-cross-offset-order "$(post_replan r_anker "$TOK_M" b-of7 '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T17:00:00+00:00"}')" 201
# Unknown plan, foreign plan, wrong apply method/path.
expect apply-unknown-plan "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans/nosuch/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: b-ap1' -d '{}')" 404
$CURL -o "$ART/fpv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r_other/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: b-fpv' -d '{"table_id":"o_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}' > "$SCR/fpv.code" 2>>"$DIAG"
chmod 600 "$ART/fpv.out"
expect foreign-preview "$(cat "$SCR/fpv.code")" 201
FPID=$(py -c 'import json; print(json.load(open("'"$ART"'/fpv.out"))["plan_id"])' 2>>"$DIAG")
expect apply-foreign-plan "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans/$FPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: b-ap2' -d '{}')" 404
expect apply-no-token "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans/$FPID/apply" -H 'Content-Type: application/json' -H 'Idempotency-Key: b-ap3' -d '{}')" 401
expect apply-nonmanager "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans/$FPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_D" -H 'Idempotency-Key: b-ap4' -d '{}')" 403
# Failed preview is atomic (whole-export equal, no receipt) and its key is
# genuinely reusable for a valid closure.
export_ok "$ART/b-prefail.json" && { echo "PASS: export-prefail (200)"; PASS=$((PASS + 1)); }
expect preview-infeasible-first "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: b-reuse1' -d '{"table_id":"t_9","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 404
export_ok "$ART/b-postfail.json" && { echo "PASS: export-postfail (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/b-prefail.json" "$ART/b-postfail.json"; then echo "PASS: failed-preview-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: failed-preview-atomic"; FAIL=$((FAIL + 1)); fi
expect failed-key-reuse "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r_anker/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: b-reuse1' -d "$CLO")" 201

echo "== C optimization tiers, oracle, trap, limits =="
# Tier 1: overlapping the interval but not the closed table stays (moved 0).
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"CB0001","user_id":"m","restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect c1-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c1-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
WANT_PV=201
[ "${STAGE4_API_SABOTAGE:-0}" = "1" ] && WANT_PV=404
expect c1-tier1-stay "$($CURL -o "$ART/c1.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c1' -d '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}')" "$WANT_PV"
chmod 600 "$ART/c1.out"
pycheck c1-tier1-shape - "$ART/c1.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert sorted(d.keys()) == ["assignments","closure","moved_count","plan_id","restaurant_revision","unused_seats"], d.keys()
assert d["closure"] == {"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}, d["closure"]
assert d["assignments"] == [{"reference":"CB0001","table_ids":["t_3"],"changed":False}], d["assignments"]
assert (d["moved_count"],d["unused_seats"],d["restaurant_revision"]) == (0,2,0), d
print("C1-OK")
PYEOF
# Tier 2: two free singles, least waste wins (t_1 waste 0 over t_3 waste 2).
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_a","label":"a","capacity":4},{"id":"t_b","label":"b","capacity":2},{"id":"t_x","label":"x","capacity":4}],
  "manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"CB0002","user_id":"m","restaurant_id":"r","table_id":"t_x","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect c2-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c2-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c2-tier2-waste "$($CURL -o "$ART/c2.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c2' -d '{"table_id":"t_x","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}')" 201
chmod 600 "$ART/c2.out"
pycheck c2-tier2-shape - "$ART/c2.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"CB0002","table_ids":["t_b"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,0), d
print("C2-OK")
PYEOF
# Tier 3: waste tie broken by option rank (singles first in fixture order).
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"u_1","label":"1","capacity":4},{"id":"u_2","label":"2","capacity":4},{"id":"u_3","label":"3","capacity":4}],
  "manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"AT0003","user_id":"m","restaurant_id":"r","table_id":"u_2","starts_at_local":"2027-06-17T19:00","party_size":4}]}
EOF
)
expect c3-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c3-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c3-tier3-rank "$($CURL -o "$ART/c3.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c3' -d '{"table_id":"u_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}')" 201
chmod 600 "$ART/c3.out"
pycheck c3-tier3-shape - "$ART/c3.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"AT0003","table_ids":["u_1"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,0), d
print("C3-OK")
PYEOF
# Exact 6 tables / 4 declared pairs / 6 considered bookings succeed.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"a1","label":"1","capacity":2},{"id":"a2","label":"2","capacity":2},{"id":"a3","label":"3","capacity":2},{"id":"a4","label":"4","capacity":2},{"id":"a5","label":"5","capacity":2},{"id":"a6","label":"6","capacity":2}],
  "combinable":[["a1","a2"],["a2","a3"],["a4","a5"],["a5","a6"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"SX0001","user_id":"m","restaurant_id":"r","table_id":"a1","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"SX0002","user_id":"m","restaurant_id":"r","table_id":"a2","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s3","reference":"SX0003","user_id":"m","restaurant_id":"r","table_id":"a3","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s4","reference":"SX0004","user_id":"m","restaurant_id":"r","table_id":"a4","starts_at_local":"2027-06-17T21:30","party_size":2},
  {"id":"s5","reference":"SX0005","user_id":"m","restaurant_id":"r","table_id":"a5","starts_at_local":"2027-06-17T21:30","party_size":2},
  {"id":"s6","reference":"SX0006","user_id":"m","restaurant_id":"r","table_id":"a6","starts_at_local":"2027-06-17T21:30","party_size":2}]}
EOF
)
expect c11-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c11-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c11-six-preview "$($CURL -o "$ART/c11.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c11' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}')" 201
chmod 600 "$ART/c11.out"
pycheck c11-six-shape - "$ART/c11.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert [a["reference"] for a in d["assignments"]] == ["SX0001","SX0002","SX0003","SX0004","SX0005","SX0006"], d["assignments"]
assert d["assignments"][0] == {"reference":"SX0001","table_ids":["a4"],"changed":True}, d["assignments"][0]
assert all(a["changed"] is False for a in d["assignments"][1:]), d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,0), d
print("SIX-OK")
PYEOF
# More than 4 declared pairs is rejected atomically and reusably.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"a1","label":"1","capacity":2},{"id":"a2","label":"2","capacity":2},{"id":"a3","label":"3","capacity":2},{"id":"a4","label":"4","capacity":2},{"id":"a5","label":"5","capacity":2},{"id":"a6","label":"6","capacity":2}],
  "combinable":[["a1","a2"],["a2","a3"],["a3","a4"],["a4","a5"],["a5","a6"]],"manager_user_ids":["m"]}],
 "reservations": []}
EOF
)
expect c12-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c12-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/c12pre.json" && { echo "PASS: c12-export-pre (200)"; PASS=$((PASS + 1)); }
expect c12-five-pairs "$($CURL -o "$ART/c12.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c12' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 422
chmod 600 "$ART/c12.out"
pycheck c12-limit-code - "$ART/c12.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "planning_limit"
print("LIMIT5-OK")
PYEOF
export_ok "$ART/c12post.json" && { echo "PASS: c12-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/c12pre.json" "$ART/c12post.json"; then echo "PASS: c12-limit-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c12-limit-atomic"; FAIL=$((FAIL + 1)); fi
# Table/pair limits cannot become feasible by shifting the closure, so
# the same unclaimed key fails again: repeat failure pins planning_limit
# with no state change (genuine successful reuse is proven by the c5,
# booking, move and adopt scenarios instead).
expect c12-repeat-failure "$($CURL -o "$ART/c12b.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c12' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:30:00+02:00"}')" 422
chmod 600 "$ART/c12b.out"
pycheck c12-repeat-code - "$ART/c12b.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "planning_limit"
print("REPEAT-OK")
PYEOF
export_ok "$ART/c12post2.json" && { echo "PASS: c12-export-post2 (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/c12post.json" "$ART/c12post2.json"; then echo "PASS: c12-repeat-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c12-repeat-atomic"; FAIL=$((FAIL + 1)); fi
# More than 6 considered bookings is rejected atomically.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"a1","label":"1","capacity":2},{"id":"a2","label":"2","capacity":2},{"id":"a3","label":"3","capacity":2},{"id":"a4","label":"4","capacity":2},{"id":"a5","label":"5","capacity":2},{"id":"a6","label":"6","capacity":2}],
  "combinable":[["a1","a2"],["a2","a3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"SV0001","user_id":"m","restaurant_id":"r","table_id":"a1","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s2","reference":"SV0002","user_id":"m","restaurant_id":"r","table_id":"a2","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s3","reference":"SV0003","user_id":"m","restaurant_id":"r","table_id":"a3","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s4","reference":"SV0004","user_id":"m","restaurant_id":"r","table_id":"a4","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s5","reference":"SV0005","user_id":"m","restaurant_id":"r","table_id":"a5","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s6","reference":"SV0006","user_id":"m","restaurant_id":"r","table_id":"a6","starts_at_local":"2027-06-17T19:00","party_size":1},
  {"id":"s7","reference":"SV0007","user_id":"m","restaurant_id":"r","table_id":"a1","starts_at_local":"2027-06-17T21:30","party_size":1}]}
EOF
)
expect c13-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c13-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/c13pre.json" && { echo "PASS: c13-export-pre (200)"; PASS=$((PASS + 1)); }
expect c13-seven-considered "$($CURL -o "$ART/c13.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c13' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}')" 422
chmod 600 "$ART/c13.out"
pycheck c13-limit-code - "$ART/c13.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "planning_limit"
print("LIMIT7-OK")
PYEOF
export_ok "$ART/c13post.json" && { echo "PASS: c13-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/c13pre.json" "$ART/c13post.json"; then echo "PASS: c13-limit-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c13-limit-atomic"; FAIL=$((FAIL + 1)); fi
expect c13-bounded-reuse "$($CURL -o "$ART/c13r.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c13' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:30:00+02:00"}')" 201
chmod 600 "$ART/c13r.out"
expect c13-reuse-replay "$($CURL -o "$ART/c13rr.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c13' -d '{"table_id":"a1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:30:00+02:00"}')" 200
chmod 600 "$ART/c13rr.out"
if cmp -s "$ART/c13r.out" "$ART/c13rr.out"; then echo "PASS: c13-reuse-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c13-reuse-bytes"; FAIL=$((FAIL + 1)); fi
# Fixed tails, a prior applied closure and member intersections constrain a
# hand-computed plan; the prior closure ends exactly when B starts.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"FT0001","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"FT0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect c14-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c14-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c14-prior-preview "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c14p' -d '{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 201
$CURL -o "$ART/c14pv.out" -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c14p' -d '{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}' 2>>"$DIAG"
chmod 600 "$ART/c14pv.out"
C14PID=$(py -c 'import json; print(json.load(open("'"$ART"'/c14pv.out"))["plan_id"])' 2>>"$DIAG")
expect c14-prior-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$C14PID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c14a' -d '{}')" 201
expect c14-preview "$($CURL -o "$ART/c14.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c14' -d '{"table_id":"t_1","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T21:00:00+02:00"}')" 201
chmod 600 "$ART/c14.out"
pycheck c14-fixed-prior-shape - "$ART/c14.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"FT0001","table_ids":["t_2"],"changed":False},
                            {"reference":"FT0002","table_ids":["t_3"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,4), d
print("C14-OK")
PYEOF
# Independent odometer oracle: brute-force optimum from fixture tables/pairs
# plus export bookings/closures/accepted terms. No production code is called:
# options, ranks, capacities, overlap, member intersection, set-equality and
# the (moved, waste, rank-vector) tuple comparison are reimplemented here.
cat > "$ART/oracle.py" <<'PYEOF'
import json, sys
from datetime import datetime

def parse_inst(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00"))

def overlap(a0, a1, b0, b1):
    return a0 < b1 and b0 < a1

def main():
    world = json.load(open(sys.argv[1]))  # tables, pairs, restaurant, closure
    export = json.load(open(sys.argv[2]))["state"]
    plan = json.load(open(sys.argv[3]))
    tables = world["tables"]          # fixture order ids
    pairs = world["pairs"]            # declared order
    rest = world["restaurant"]
    clo = world["closure"]
    c0, c1 = parse_inst(clo["from"]), parse_inst(clo["to"])
    options = [[t] for t in tables] + [list(p) for p in pairs]
    res = export["reservations"]
    bookings = [r for r in res.values()
                if r["restaurant_id"] == rest and r["status"] == "confirmed"
                and overlap(parse_inst(r["starts_at"]), parse_inst(r["ends_at"]), c0, c1)]
    bookings.sort(key=lambda r: r["reference"])
    fixed = [r for r in res.values()
             if r["restaurant_id"] == rest and r["status"] == "confirmed"
             and not overlap(parse_inst(r["starts_at"]), parse_inst(r["ends_at"]), c0, c1)]
    applied = export.get("closures", {}).get(rest, [])
    blocks = [(parse_inst(c["from"]), parse_inst(c["to"]), [c["table_id"]]) for c in applied]
    blocks.append((c0, c1, [clo["table_id"]]))
    def fits(b, opt):
        caps = b["accepted_terms"]["capacities"]
        if sum(caps[t] for t in opt) < b["party_size"]:
            return False
        s0, s1 = parse_inst(b["starts_at"]), parse_inst(b["ends_at"])
        for f in fixed:
            if set(opt) & set(f.get("table_ids", [f.get("table_id")])) and \
               overlap(s0, s1, parse_inst(f["starts_at"]), parse_inst(f["ends_at"])):
                return False
        for (w0, w1, wt) in blocks:
            if set(opt) & set(wt) and overlap(s0, s1, w0, w1):
                return False
        return True
    feas = {}
    for b in bookings:
        feas[b["reference"]] = [o for o in options if fits(b, o)]
    best = None
    for combo in __import__("itertools").product(*[feas[b["reference"]] for b in bookings]):
        ok = True
        for i in range(len(bookings)):
            for j in range(i + 1, len(bookings)):
                bi, bj = bookings[i], bookings[j]
                if set(combo[i]) & set(combo[j]) and overlap(
                        parse_inst(bi["starts_at"]), parse_inst(bi["ends_at"]),
                        parse_inst(bj["starts_at"]), parse_inst(bj["ends_at"])):
                    ok = False
        if not ok:
            continue
        moved, waste, ranks = 0, 0, []
        for b, opt in zip(bookings, combo):
            cur = b.get("table_ids", [b.get("table_id")])
            if set(opt) != set(cur):
                moved += 1
            caps = b["accepted_terms"]["capacities"]
            waste += sum(caps[t] for t in opt) - b["party_size"]
            ranks.append(options.index(opt))
        key = (moved, waste, ranks)
        if best is None or key < best[0]:
            best = (key, combo)
    # Ref-ordered first-fit greedy: each booking takes min (waste, rank)
    # among options free against already placed ones.
    greedy = []
    placed = []
    for b in bookings:
        cands = []
        for opt in feas[b["reference"]]:
            clash = False
            for (p, popt) in placed:
                if set(opt) & set(popt) and overlap(
                        parse_inst(b["starts_at"]), parse_inst(b["ends_at"]),
                        parse_inst(p["starts_at"]), parse_inst(p["ends_at"])):
                    clash = True
            if clash:
                continue
            caps = b["accepted_terms"]["capacities"]
            cur = b.get("table_ids", [b.get("table_id")])
            cands.append((sum(caps[t] for t in opt) - b["party_size"],
                          options.index(opt), 0 if set(opt) == set(cur) else 1, opt))
        cands.sort()
        greedy.append((b["reference"], cands[0][3] if cands else None))
        if cands:
            placed.append((b, cands[0][3]))
    out = {"considered": [b["reference"] for b in bookings],
           "best": None, "greedy": [[r, o] for r, o in greedy]}
    if best is not None:
        (bm, bw, br), combo = best
        out["best"] = {"assignments": [
            {"reference": b["reference"], "table_ids": list(opt),
             "changed": set(opt) != set(b.get("table_ids", [b.get("table_id")]))}
            for b, opt in zip(bookings, combo)],
            "moved_count": bm, "unused_seats": bw}
    json.dump(out, open(sys.argv[4], "w"))
    print("ORACLE-OK considered=%d feasible=%s" % (len(bookings), best is not None))

main()
PYEOF
# Global-vs-greedy trap: deterministic seeded references AA0001 < AA0002.
# AA0001 (p2, closed t_3) would greedily take t_1 (waste 0), forcing AA0002
# onto t_2 (moved 2); the global optimum moves only AA0001 onto t_2.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"AA0001","user_id":"m","restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"AA0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect c4-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c4-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c4-trap-preview "$($CURL -o "$ART/c4.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c4' -d '{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T21:00:00+02:00"}')" 201
chmod 600 "$ART/c4.out"
cat > "$ART/c4world.json" <<'EOF'
{"restaurant":"r","tables":["t_1","t_2","t_3"],"pairs":[["t_1","t_2"],["t_2","t_3"]],
 "closure":{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T21:00:00+02:00"}}
EOF
chmod 600 "$ART/c4world.json"
export_ok "$ART/c4exp.json" && { echo "PASS: c4-export (200)"; PASS=$((PASS + 1)); }
if py "$ART/oracle.py" "$ART/c4world.json" "$ART/c4exp.json" "$ART/c4.out" "$ART/c4oracle.json" > "$SCR/ora.out" 2>>"$DIAG"; then echo "PASS: c4-oracle ($(cat "$SCR/ora.out"))"; PASS=$((PASS + 1)); else echo "FAIL: c4-oracle"; FAIL=$((FAIL + 1)); fi
pycheck c4-service-equals-oracle - "$ART/c4.out" "$ART/c4oracle.json" <<'PYEOF'
import json,sys
plan = json.load(open(sys.argv[1]))
oracle = json.load(open(sys.argv[2]))
assert oracle["considered"] == ["AA0001","AA0002"], oracle["considered"]
best = oracle["best"]
assert best is not None, "oracle found no plan"
assert plan["assignments"] == best["assignments"], (plan["assignments"], best["assignments"])
assert (plan["moved_count"],plan["unused_seats"]) == (best["moved_count"],best["unused_seats"]) == (1,2)
assert plan["assignments"] == [{"reference":"AA0001","table_ids":["t_2"],"changed":True},
                               {"reference":"AA0002","table_ids":["t_1"],"changed":False}]
g = oracle["greedy"]
assert [o for _,o in g] != [a["table_ids"] for a in best["assignments"]], g
print("TRAP-OK service==oracle!=greedy")
PYEOF
# Real fixed-tail witness: B is a confirmed pair overlapping A during
# 20:00-20:30 yet wholly outside the short proposed closure [19:00,19:30),
# so B is fixed (excluded from assignments) while its occupancy forces A
# onto t_3. Ignoring the fixed tail would pick t_1 (waste 0); the correct
# plan moves once with waste 2.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4},{"id":"t_4","label":"4","capacity":4}],
  "combinable":[["t_1","t_4"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"FX0001","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"FX0002","user_id":"m","restaurant_id":"r","table_ids":["t_1","t_4"],"starts_at_local":"2027-06-17T20:00","party_size":2}]}
EOF
)
expect c15-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c15-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/c15pre.json" && { echo "PASS: c15-export-pre (200)"; PASS=$((PASS + 1)); }
expect c15-preview "$($CURL -o "$ART/c15.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c15' -d '{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T19:30:00+02:00"}')" 201
chmod 600 "$ART/c15.out"
pycheck c15-fixed-tail - "$ART/c15.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert [a["reference"] for a in d["assignments"]] == ["FX0001"], d["assignments"]
assert d["assignments"] == [{"reference":"FX0001","table_ids":["t_3"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,2), d
print("FIXED-OK")
PYEOF
export_ok "$ART/c15post.json" && { echo "PASS: c15-export-post (200)"; PASS=$((PASS + 1)); }
pycheck c15-fixed-untouched - "$ART/c15pre.json" "$ART/c15post.json" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
assert pre["reservations"]["FX0002"] == post["reservations"]["FX0002"], "fixed record drift"
assert pre["histories"]["FX0002"] == post["histories"]["FX0002"], "fixed history drift"
assert len(post["plans"]) == len(pre["plans"]) + 1
assert len(post["receipts"]) == len(pre["receipts"]) + 1
for k in ("reservations","histories","series","closures","restaurant_revisions","users","tokens","restaurants","policies"):
    assert pre[k] == post[k], "namespace %s changed" % k
print("FIXEDDELTA-OK")
PYEOF
# The same world without the fixed booking prefers t_1 (waste 0): the
# fixed tail is what forces t_3. Proved through the independent oracle on
# a copied export with B removed.
cat > "$ART/c15world.json" <<'EOF'
{"restaurant":"r","tables":["t_1","t_2","t_3","t_4"],"pairs":[["t_1","t_4"],["t_2","t_3"]],
 "closure":{"table_id":"t_2","from":"2027-06-17T19:00:00+02:00","to":"2027-06-17T19:30:00+02:00"}}
EOF
chmod 600 "$ART/c15world.json"
pycheck c15-oracle-nofixed - "$ART/oracle.py" "$ART/c15world.json" "$ART/c15post.json" "$ART/c15.out" <<'PYEOF'
import json,sys,subprocess,os
oraprog, world, expf, planf = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
blank = expf + ".nofixed.json"
e = json.load(open(expf))
e["state"]["reservations"] = {k: v for k, v in e["state"]["reservations"].items() if k != "FX0002"}
e["state"]["histories"] = {k: v for k, v in e["state"]["histories"].items() if k != "FX0002"}
json.dump(e, open(blank, "w"))
r = subprocess.run([sys.executable, oraprog, world, blank, planf, blank + ".out.json"],
                   capture_output=True, text=True)
assert r.returncode == 0, r.stderr[-500:]
o = json.load(open(blank + ".out.json"))
assert o["best"]["assignments"] == [{"reference":"FX0001","table_ids":["t_1"],"changed":True}], o["best"]
print("NOFIXED-OK")
PYEOF
# Known infeasible world: every table taken at the slot, pairs blocked.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"IN0001","user_id":"m","restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T19:00","party_size":4},
  {"id":"s2","reference":"IN0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s3","reference":"IN0003","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect c5-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c5-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/c5pre.json" && { echo "PASS: c5-export-pre (200)"; PASS=$((PASS + 1)); }
expect c5-no-feasible "$($CURL -o "$ART/c5.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c5' -d '{"table_id":"t_3","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T21:00:00+02:00"}')" 409
chmod 600 "$ART/c5.out"
pycheck c5-no-feasible-code - "$ART/c5.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "no_feasible_plan"
print("INFEAS-OK")
PYEOF
export_ok "$ART/c5post.json" && { echo "PASS: c5-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/c5pre.json" "$ART/c5post.json"; then echo "PASS: c5-infeasible-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c5-infeasible-atomic"; FAIL=$((FAIL + 1)); fi
expect c5-failed-key-reuse "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c5' -d '{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T18:30:00+02:00"}')" 201
# Reversed pair input names the same set: accepted, a canonical no-op.
expect c6-pair-create "$($CURL -o "$ART/c6.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c6a' -d '{"restaurant_id":"r","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T21:30","party_size":5}')" 201
chmod 600 "$ART/c6.out"
C6REF=$(py -c 'import json; print(json.load(open("'"$ART"'/c6.out"))["reference"])' 2>>"$DIAG")
$CURL -o "$ART/c6h1.out" -G "$BASE/reservations/$C6REF/history" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/c6h1.out"
C6H1=$(py -c 'import json; print(len(json.load(open("'"$ART"'/c6h1.out"))["entries"]))' 2>>"$DIAG")
C6R1=$(py -c 'import json; print(json.load(open("'"$ART"'/c6.out"))["revision"])' 2>>"$DIAG")
expect c6-reversed-patch "$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/$C6REF" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d '{"table_ids":["t_2","t_1"]}')" 200
$CURL -o "$ART/c6h2.out" -G "$BASE/reservations/$C6REF/history" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/c6h2.out"
C6H2=$(py -c 'import json; print(len(json.load(open("'"$ART"'/c6h2.out"))["entries"]))' 2>>"$DIAG")
if [ -n "$C6H1" ] && [ "$C6H2" = "$C6H1" ]; then echo "PASS: c6-reversed-noop-history ($C6H2 entries)"; PASS=$((PASS + 1)); else echo "FAIL: c6-reversed-noop-history (got $C6H2, want $C6H1)"; FAIL=$((FAIL + 1)); fi
# Old accepted v1 capacity governs after a same-date v2 publication.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": []}
EOF
)
expect c7-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c7-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
V1='{"effective_from":"2027-01-05","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}'
expect c7-publish-v1 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c7p1' -d "$V1")" 201
expect c7-create "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c7a' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2}')" 201
V2='{"effective_from":"2027-01-05","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":1,"t_2":4,"t_3":4}}'
expect c7-publish-v2 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c7p2' -d "$V2")" 201
export_ok "$ART/c7pre.json" && { echo "PASS: c7-export-pre (200)"; PASS=$((PASS + 1)); }
expect c7-preview "$($CURL -o "$ART/c7.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c7' -d '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}')" 201
chmod 600 "$ART/c7.out"
pycheck c7-accepted-v1 - "$ART/c7.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert len(d["assignments"]) == 1, d["assignments"]
assert d["assignments"][0]["table_ids"] == ["t_1"], d["assignments"]
print("V1-OK")
PYEOF
$CURL -o "$ART/c7cr.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c7a' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2}' 2>>"$DIAG"
chmod 600 "$ART/c7cr.out"
C7REF=$(py -c 'import json; print(json.load(open("'"$ART"'/c7cr.out"))["reference"])' 2>>"$DIAG")
$CURL -o "$ART/c7dec.out" -G "$BASE/reservations/$C7REF/decision" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/c7dec.out"
pycheck c7-stored-v1 - "$ART/c7dec.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["accepted_terms"]["policy_version"] == 1, d["accepted_terms"]
assert d["accepted_terms"]["capacities"] == {"t_1":2,"t_2":4,"t_3":4}, d["accepted_terms"]
assert d["revision"] == 1, d
print("STOREDV1-OK")
PYEOF
$CURL -o "$ART/c7pol.out" -G "$BASE/restaurants/r/policies" 2>>"$DIAG"
chmod 600 "$ART/c7pol.out"
pycheck c7-current-v2 - "$ART/c7pol.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert [p["policy_version"] for p in d["policies"]] == [1,2], d["policies"]
assert d["policies"][1]["capacities"]["t_1"] == 1, d["policies"]
print("CURV2-OK")
PYEOF
export_ok "$ART/c7post.json" && { echo "PASS: c7-export-post (200)"; PASS=$((PASS + 1)); }
pycheck c7-preview-delta - "$ART/c7pre.json" "$ART/c7post.json" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
assert len(post["plans"]) == len(pre["plans"]) + 1
assert len(post["receipts"]) == len(pre["receipts"]) + 1
for k in ("reservations","histories","series","closures","restaurant_revisions","users","tokens","restaurants","policies"):
    assert pre[k] == post[k], "namespace %s changed" % k
print("C7DELTA-OK")
PYEOF
# Above-max fixture-0 capacity flows into accepted terms and stays frozen.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_big","label":"B","capacity":200},{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"BG0001","user_id":"m","restaurant_id":"r","table_id":"t_big","starts_at_local":"2027-06-17T19:00","party_size":150}]}
EOF
)
expect c8-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c8-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c8-preview "$($CURL -o "$ART/c8.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c8' -d '{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}')" 201
chmod 600 "$ART/c8.out"
pycheck c8-above-max-frozen - "$ART/c8.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"BG0001","table_ids":["t_big"],"changed":False}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (0,50), d
print("BIG-OK")
PYEOF
$CURL -o "$ART/c8dec.out" -G "$BASE/reservations/BG0001/decision" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/c8dec.out"
pycheck c8-accepted-terms - "$ART/c8dec.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["accepted_terms"]["policy_version"] == 0, d
assert d["accepted_terms"]["capacities"] == {"t_big":200,"t_1":2}, d["accepted_terms"]
print("TERMS-OK")
PYEOF
# Planning limit: 7 tables exceeds the supported input; atomic + reusable.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"big","name":"B","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t1","label":"1","capacity":2},{"id":"t2","label":"2","capacity":2},{"id":"t3","label":"3","capacity":2},{"id":"t4","label":"4","capacity":2},{"id":"t5","label":"5","capacity":2},{"id":"t6","label":"6","capacity":2},{"id":"t7","label":"7","capacity":2}],
  "manager_user_ids":["m"]}],
 "reservations": []}
EOF
)
expect c9-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c9-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/c9pre.json" && { echo "PASS: c9-export-pre (200)"; PASS=$((PASS + 1)); }
expect c9-planning-limit "$($CURL -o "$ART/c9.out" -w '%{http_code}' -X POST "$BASE/restaurants/big/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c9' -d '{"table_id":"t1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 422
chmod 600 "$ART/c9.out"
pycheck c9-limit-code - "$ART/c9.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "planning_limit"
print("LIMIT-OK")
PYEOF
export_ok "$ART/c9post.json" && { echo "PASS: c9-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/c9pre.json" "$ART/c9post.json"; then echo "PASS: c9-limit-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: c9-limit-atomic"; FAIL=$((FAIL + 1)); fi
# Past operator repair: cutoff does not stop the planner (2020-01-02 Thursday).
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"AT0005","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2020-01-02T19:00","party_size":2}]}
EOF
)
expect c10-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: c10-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect c10-past-repair "$($CURL -o "$ART/c10.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: c10' -d '{"table_id":"t_2","from":"2020-01-02T18:00:00+01:00","to":"2020-01-02T23:00:00+01:00"}')" 201
chmod 600 "$ART/c10.out"
pycheck c10-past-shape - "$ART/c10.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"AT0005","table_ids":["t_1"],"changed":True}], d["assignments"]
print("PAST-OK")
PYEOF

echo "== D preview delta, receipts, replays =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"DD0001","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"DD0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect d-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: d-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
export_ok "$ART/dpre.json" && { echo "PASS: d-export-pre (200)"; PASS=$((PASS + 1)); }
DCLO='{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}'
expect d-preview "$($CURL -o "$ART/dpv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: d-pv' -d "$DCLO")" 201
chmod 600 "$ART/dpv.out"
pycheck d-preview-shape - "$ART/dpv.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert sorted(d.keys()) == ["assignments","closure","moved_count","plan_id","restaurant_revision","unused_seats"], d.keys()
assert d["closure"] == {"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}, d["closure"]
assert d["assignments"] == [{"reference":"DD0001","table_ids":["t_3"],"changed":True},
                            {"reference":"DD0002","table_ids":["t_1"],"changed":False}], d["assignments"]
assert (d["moved_count"],d["unused_seats"],d["restaurant_revision"]) == (1,2,0), d
assert isinstance(d["plan_id"], str) and d["plan_id"], d["plan_id"]
print("DPV-OK")
PYEOF
DPID=$(py -c 'import json; print(json.load(open("'"$ART"'/dpv.out"))["plan_id"])' 2>>"$DIAG")
export_ok "$ART/dpost.json" && { echo "PASS: d-export-post (200)"; PASS=$((PASS + 1)); }
pycheck d-preview-delta - "$ART/dpre.json" "$ART/dpost.json" "$ART/dpv.out" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
pv = json.load(open(sys.argv[3]))
assert len(post["plans"]) == len(pre["plans"]) + 1, (len(pre["plans"]), len(post["plans"]))
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "receipt count"
for k in ("reservations","histories","series","closures","restaurant_revisions","users","tokens","restaurants","policies"):
    assert pre[k] == post[k], "namespace %s changed" % k
for k, v in pre["plans"].items():
    assert post["plans"][k] == v, "prior plan changed"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_plans = [k for k in post["plans"] if k not in pre["plans"]]
assert len(new_plans) == 1 and post["plans"][new_plans[0]]["applied"] is False
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]]
assert len(new_rc) == 1, "one receipt"
rc = new_rc[0]
assert rc["user_id"] == "m" and rc["method"] == "POST", rc
assert rc["path"] == "/restaurants/r/replans", rc["path"]
assert rc["key"] == "d-pv" and rc["status"] == 201, rc
assert json.loads(rc["body"]) == json.loads('{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}'), rc["body"]
assert rc["response"] == open(sys.argv[3]).read(), "receipt response != raw first201"
print("DELTA-OK")
PYEOF
pycheck d-stored-plan-binds - "$ART/dpost.json" "$ART/dpv.out" <<'PYEOF'
import json,sys
post = json.load(open(sys.argv[1]))["state"]
pv = json.load(open(sys.argv[2]))
stored = [v for v in post["plans"].values() if v["applied"] is False]
assert len(stored) == 1, "exactly one new unapplied plan"
st = stored[0]
assert st["plan_id"] == pv["plan_id"], "stored id != public id"
assert st["restaurant_id"] == "r", st["restaurant_id"]
assert st["restaurant_revision"] == pv["restaurant_revision"] == 0, "captured revision"
assert st["closure"] == pv["closure"], "stored closure != public closure"
assert st["assignments"] == pv["assignments"], "stored assignments != public"
assert st["moved_count"] == pv["moved_count"] and st["unused_seats"] == pv["unused_seats"], "totals"
print("STORED-OK")
PYEOF
# Replay after a REAL same-restaurant write: 200 byte-original, export same.
expect d-write "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: d-w1' -d '{"restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}')" 201
export_ok "$ART/drepre.json" && { echo "PASS: d-export-repre (200)"; PASS=$((PASS + 1)); }
expect d-replay "$($CURL -o "$ART/drepr.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: d-pv' -d "$DCLO")" 200
chmod 600 "$ART/drepr.out"
if cmp -s "$ART/dpv.out" "$ART/drepr.out"; then echo "PASS: d-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: d-replay-bytes"; FAIL=$((FAIL + 1)); fi
export_ok "$ART/drepost.json" && { echo "PASS: d-export-repost (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/drepre.json" "$ART/drepost.json"; then echo "PASS: d-replay-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: d-replay-atomic"; FAIL=$((FAIL + 1)); fi
# Committed key with a changed body is a reuse conflict.
expect d-changed-body "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: d-pv' -d '{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T19:00:00+02:00"}')" 409

cat > "$ART/apply_delta.py" <<'PYEOF'
import json, sys
from datetime import datetime
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
body = json.load(open(sys.argv[3]))
spec = json.load(open(sys.argv[4]))
raw = open(sys.argv[3]).read()
pid, rest = spec["plan_id"], spec["restaurant"]
# 1. response shape, order, Public==stored projection, scalar iff singleton.
assert sorted(body.keys()) == ["plan_id", "reservations", "restaurant_revision"], body.keys()
assert body["plan_id"] == pid
assert [r["reference"] for r in body["reservations"]] == spec["order"], body["reservations"]
for rm in body["reservations"]:
    ref = rm["reference"]
    st = post["reservations"][ref]
    want = {k: v for k, v in st.items() if k != "user_id"}
    if len(st["table_ids"]) != 1:
        del want["table_id"]
    assert rm == want, (ref, rm, want)
assert body["restaurant_revision"] == spec["revision"], body["restaurant_revision"]
# 2. stored plan: only Applied flipped.
assert set(post["plans"]) == set(pre["plans"]), "plan keys"
for k, v in pre["plans"].items():
    if k == pid:
        assert v["applied"] is False, "pre plan not unapplied"
        cur = dict(post["plans"][k])
        assert cur.pop("applied") is True
        old = dict(v); old.pop("applied", None)
        assert cur == old, "plan drifted beyond Applied"
    else:
        assert post["plans"][k] == v, "prior plan %s changed" % k
# 3. closures: exactly one appended on target; other maps retained.
pre_cl = pre.get("closures", {}) or {}
post_cl = post.get("closures", {}) or {}
assert set(post_cl) == set(pre_cl) | {rest}, "closure key set"
for k, v in pre_cl.items():
    if k == rest:
        continue
    assert post_cl.get(k) == v, "other closures %s changed" % k
for k, v in post_cl.items():
    if k != rest:
        assert pre_cl.get(k, []) == v, "closure map %s changed" % k
assert post_cl[rest][:-1] == (pre_cl.get(rest) or []), "prior closures changed"
assert post_cl[rest][-1] == post["plans"][pid]["closure"], "appended closure != plan closure"
# 4. FULL counter map: only target +1.
exp = dict(pre["restaurant_revisions"])
exp[rest] = exp.get(rest, 0) + 1
assert post["restaurant_revisions"] == exp, (post["restaurant_revisions"], exp)
# 5. moved records: selectors+revision only; identity/owner/terms/times kept.
assert set(post["reservations"]) == set(pre["reservations"]), "record set changed"
for coll in (pre["reservations"], post["reservations"]):
    for ref, rec in coll.items():
        if len(rec["table_ids"]) == 1:
            assert rec["table_id"] == rec["table_ids"][0], "stored scalar %s" % ref
        else:
            assert rec["table_id"] == "", "pair stored scalar %s" % ref
assert set(pre["histories"]) == set(pre["reservations"]), "pre history keyset"
assert set(post["histories"]) == set(post["reservations"]), "post history keyset"
assert set(spec["moved"]) | set(spec["unmoved"]) == set(pre["reservations"]), "moved/unmoved partition"
assert not (set(spec["moved"]) & set(spec["unmoved"])), "partition overlap"
for ref, ch in spec["moved"].items():
    b, a = pre["reservations"][ref], post["reservations"][ref]
    assert set(a) == set(b), "key set %s" % ref
    assert a["revision"] == b["revision"] + 1, ref
    assert a["table_ids"] == ch["to"] and b["table_ids"] == ch["from"], ref
    for k in b:
        if k not in ("table_id", "table_ids", "revision"):
            assert a[k] == b[k], "moved %s field %s" % (ref, k)
# 6. unmoved records + histories byte-equal.
for ref in spec["unmoved"]:
    assert pre["reservations"][ref] == post["reservations"][ref], "unmoved record %s" % ref
    assert pre["histories"][ref] == post["histories"][ref], "unmoved history %s" % ref
# 7. moved histories: prefix + one Reassigned with full bindings.
for ref, ch in spec["moved"].items():
    bh, ah = pre["histories"][ref], post["histories"][ref]
    assert len(ah) == len(bh) + 1 and ah[:len(bh)] == bh, "prefix %s" % ref
    ne = ah[-1]
    assert ne["event"] == "reassigned" and ne["plan_id"] == pid, ne
    assert ne["revision"] == post["reservations"][ref]["revision"], ne
    assert ne["seq"] == len(ah), ne
    assert ne["changes"] == [{"field": "table_ids", "from": ch["from"], "to": ch["to"]}], ne["changes"]
    assert ne["accepted_terms"] == post["reservations"][ref]["accepted_terms"], "terms %s" % ref
    at = datetime.fromisoformat(ne["at"].replace("Z", "+00:00"))
    assert at.tzinfo is not None, ne["at"]
# 8. exactly one new scoped receipt binding the original raw bytes.
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "receipt count"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]]
assert len(new_rc) == 1
rc = new_rc[0]
assert rc["user_id"] == spec["owner"] and rc["method"] == "POST", rc
assert rc["path"] == spec["path"] and rc["key"] == spec["key"], rc
assert rc["status"] == 201 and rc["body"] == spec["body"], rc
assert rc["response"] == raw, "receipt response != raw apply bytes"
# 9. every other namespace byte-equal (series handled in 10).
for k, v in pre.items():
    if k in ("plans", "receipts", "reservations", "histories", "closures", "restaurant_revisions", "series"):
        continue
    assert post[k] == v, "namespace %s changed" % k
# 10. explicit allowed series-revision delta; all other series byte-equal.
assert set(post.get("series", {}) or {}) == set(pre.get("series", {}) or {}), "series keys"
for sid, bounds in (spec.get("series") or {}).items():
    b, a = pre["series"][sid], post["series"][sid]
    assert b["revision"] == bounds["before"] and a["revision"] == bounds["after"], sid
    for k in ("series_id", "user_id", "restaurant_id", "interval_weeks"):
        assert a[k] == b[k], k
    assert [(m["index"], m["reference"], m["scheduled_date"], m["exception"]) for m in a["members"]] == \
           [(m["index"], m["reference"], m["scheduled_date"], m["exception"]) for m in b["members"]], sid
for sid, v in (pre.get("series", {}) or {}).items():
    if sid not in (spec.get("series") or {}):
        assert post["series"][sid] == v, "unrelated series %s changed" % sid
print("APPLY-DELTA-OK %s" % pid)
PYEOF
chmod 600 "$ART/apply_delta.py"
cat > "$ART/amend_delta.py" <<'PYEOF'
import json, sys, copy
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
body = json.load(open(sys.argv[3]))
spec = json.load(open(sys.argv[4]))
raw = open(sys.argv[3], "rb").read()
sid = spec["series"]
rb, ra = spec["rev_before"], spec["rev_before"] + 1
rest_id = pre["series"][sid]["restaurant_id"]
tzname = next(r["timezone"] for r in pre["restaurants"] if r["id"] == rest_id)
tz = ZoneInfo(tzname)

# 1. response shape, revision, member order.
assert sorted(body.keys()) == ["interval_weeks", "occurrences", "revision", "series_id"], body.keys()
assert body["series_id"] == sid
assert body["revision"] == ra, body["revision"]
pre_members = pre["series"][sid]["members"]
assert [o["reference"] for o in body["occurrences"]] == [m["reference"] for m in pre_members], "occurrence order"

# 2. eligible set from PRE (confirmed + non-exception + index>=from_index).
fi = spec["from_index"]
eligible = set()
for m in pre_members:
    rec = pre["reservations"].get(m["reference"])
    if m["index"] >= fi and rec is not None and rec["status"] == "confirmed" and not m["exception"]:
        eligible.add(m["reference"])
assert set(spec["changed"]) == eligible, (sorted(spec["changed"]), sorted(eligible))

# 3. per-occurrence pins: member fields, Public projection, scheduled clock.
for o in body["occurrences"]:
    ref = o["reference"]
    m = next(x for x in pre_members if x["reference"] == ref)
    assert o["index"] == m["index"] and o["exception"] == m["exception"], o
    st = post["reservations"][ref]
    want = {k: v for k, v in st.items() if k != "user_id"}
    if len(st["table_ids"]) != 1:
        del want["table_id"]
    assert o["reservation"] == want, (ref, o["reservation"], want)
    if ref in spec["changed"]:
        ch = spec["changed"][ref]
        assert st["starts_at_local"] == ch["date"] + "T" + spec["clock"], (ref, st["starts_at_local"])
        assert st["starts_at_local"] != pre["reservations"][ref]["starts_at_local"], "not a real change"

# 4. changed records: identity frozen, rev+1, clock/ends/terms move only.
for ref, ch in spec["changed"].items():
    b, a = pre["reservations"][ref], post["reservations"][ref]
    assert set(a) == set(b), "key set %s" % ref
    for k in ("user_id", "reference", "reservation_id", "created_at", "status",
              "party_size", "table_id", "table_ids", "restaurant_id"):
        assert a[k] == b[k], "identity drift %s.%s" % (ref, k)
    assert a["table_ids"] == ch["tables"], "assigned selectors %s" % ref
    assert a["revision"] == b["revision"] + 1, ref
    terms = ch["terms"] if ch["terms"] is not None else b["accepted_terms"]
    assert a["accepted_terms"] == terms, (ref, a["accepted_terms"])
    s0 = datetime.fromisoformat(a["starts_at"])
    s1 = datetime.fromisoformat(a["ends_at"])
    assert (s1 - s0).total_seconds() == terms["reservation_duration_minutes"] * 60, (ref, a)
    assert s0.tzinfo is not None and s1.tzinfo is not None, ref
    # Absolute coherence via the stored IANA zone (first-fold rule), not post output.
    y, mo, d = int(ch["date"][0:4]), int(ch["date"][5:7]), int(ch["date"][8:10])
    hh, mm = int(spec["clock"][0:2]), int(spec["clock"][3:5])
    exp_start = datetime(y, mo, d, hh, mm, tzinfo=tz)
    assert s0 == exp_start, (ref, a["starts_at"], exp_start.isoformat())
    assert s1 == exp_start + timedelta(minutes=terms["reservation_duration_minutes"]), (ref, a["ends_at"])

# 5. stored scalar convention on every stored record, pre and post.
for coll in (pre["reservations"], post["reservations"]):
    for ref, rec in coll.items():
        if len(rec["table_ids"]) == 1:
            assert rec["table_id"] == rec["table_ids"][0], "stored scalar %s" % ref
        else:
            assert rec["table_id"] == "", "pair stored scalar %s" % ref

# 6. history keysets; unmoved byte-equal; moved prefix + exact Changed.
assert set(pre["histories"]) == set(pre["reservations"]), "pre history keyset"
assert set(post["histories"]) == set(post["reservations"]), "post history keyset"
assert set(post["reservations"]) == set(pre["reservations"]), "post record keyset"
assert set(post["histories"]) == set(pre["histories"]), "post history drift"
assert set(spec["changed"]) | set(spec["unmoved"]) == set(pre["reservations"]), "partition"
assert not (set(spec["changed"]) & set(spec["unmoved"])), "partition overlap"
for ref in spec["unmoved"]:
    assert pre["reservations"][ref] == post["reservations"][ref], "unmoved record %s" % ref
    assert pre["histories"][ref] == post["histories"][ref], "unmoved history %s" % ref
for ref, ch in spec["changed"].items():
    bh, ah = pre["histories"][ref], post["histories"][ref]
    assert len(ah) == len(bh) + 1 and ah[:len(bh)] == bh, "prefix %s" % ref
    ne = ah[-1]
    assert ne["event"] == "changed", ne
    assert ne["revision"] == post["reservations"][ref]["revision"], ne
    assert ne["seq"] == len(ah), ne
    assert ne["changes"] == [{"field": "starts_at_local",
                              "from": pre["reservations"][ref]["starts_at_local"],
                              "to": post["reservations"][ref]["starts_at_local"]}], ne["changes"]
    assert ne["accepted_terms"] == post["reservations"][ref]["accepted_terms"], "terms %s" % ref
    assert "plan_id" not in ne, ne
    at = datetime.fromisoformat(ne["at"].replace("Z", "+00:00"))
    assert at.tzinfo is not None, ne["at"]

# 7. full series object: deep-copy PRE with only revision+1.
assert set(post.get("series", {}) or {}) == set(pre.get("series", {}) or {}), "series keys"
bsz = pre["series"][sid]
assert bsz["revision"] == rb, (sid, bsz["revision"])
exp_sz = copy.deepcopy(bsz)
exp_sz["revision"] = ra
assert post["series"][sid] == exp_sz, "affected series drift beyond revision"
for qid, v in (pre.get("series", {}) or {}).items():
    if qid != sid:
        assert post["series"][qid] == v, "unrelated series %s changed" % qid

# 8. full counter map: only target +1.
exp = dict(pre["restaurant_revisions"])
exp[spec["counter_rest"]] = exp.get(spec["counter_rest"], 0) + 1
assert post["restaurant_revisions"] == exp, (post["restaurant_revisions"], exp)

# 9. plans/closures/other namespaces fully byte-equal (amend stores none).
assert set(post.get("plans", {}) or {}) == set(pre.get("plans", {}) or {}), "plan keys"
for k, v in (pre.get("plans", {}) or {}).items():
    assert post["plans"][k] == v, "plan %s changed" % k
assert (post.get("closures", {}) or {}) == (pre.get("closures", {}) or {}), "closures changed"
for k, v in pre.items():
    if k in ("reservations", "histories", "series", "restaurant_revisions", "receipts"):
        continue
    assert post[k] == v, "namespace %s changed" % k

# 10. exactly one new scoped receipt, fully bound; priors kept; loser keys absent.
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "receipt count"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]]
assert len(new_rc) == 1
rc = new_rc[0]
assert rc["user_id"] == spec["owner"] and rc["method"] == "POST", rc
assert rc["path"] == spec["path"] and rc["key"] == spec["key"], rc
assert rc["status"] == 201 and rc["body"] == spec["body"], rc
assert rc["response"].encode() == raw, "receipt response != raw bytes"
for lk in spec.get("absent_keys", []):
    assert all(not (v["key"] == lk and v["path"] == spec["path"]) for v in post["receipts"].values()), \
        "loser receipt %s" % lk
print("AMEND-DELTA-OK %s" % sid)
PYEOF
chmod 600 "$ART/amend_delta.py"
echo "== E apply: records, history, counters, pairs, zero-move =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]},
  {"id":"ro","name":"O","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"q_1","label":"1","capacity":4}],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"EE0001","user_id":"m","restaurant_id":"r","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T19:00","party_size":6},
  {"id":"s2","reference":"EE0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T21:30","party_size":1},
  {"id":"s3","reference":"RO0001","user_id":"m","restaurant_id":"ro","table_id":"q_1","starts_at_local":"2027-06-17T19:00","party_size":1}]}
EOF
)
expect e-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: e-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
ECLO='{"table_id":"t_1","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}'
expect e-preview "$($CURL -o "$ART/epv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e-pv' -d "$ECLO")" 201
chmod 600 "$ART/epv.out"
EPID=$(py -c 'import json; print(json.load(open("'"$ART"'/epv.out"))["plan_id"])' 2>>"$DIAG")
pycheck e-pair-preview - "$ART/epv.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"EE0001","table_ids":["t_2","t_3"],"changed":True},
                            {"reference":"EE0002","table_ids":["t_2"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (2,5), d
print("EPV-OK")
PYEOF
export_ok "$ART/epre.json" && { echo "PASS: e-export-pre (200)"; PASS=$((PASS + 1)); }
expect e-apply "$($CURL -o "$ART/eap.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$EPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e-ap' -d '{}')" 201
chmod 600 "$ART/eap.out"
export_ok "$ART/epost.json" && { echo "PASS: e-export-post (200)"; PASS=$((PASS + 1)); }
cat > "$ART/espec.json" <<EOF
{"plan_id":"$EPID","restaurant":"r","path":"/restaurants/r/replans/$EPID/apply","key":"e-ap","owner":"m","body":"{}",
 "order":["EE0001","EE0002"],"revision":1,
 "moved":{"EE0001":{"from":["t_1","t_2"],"to":["t_2","t_3"]},"EE0002":{"from":["t_1"],"to":["t_2"]}},
 "unmoved":["RO0001"]}
EOF
chmod 600 "$ART/espec.json"
if py "$ART/apply_delta.py" "$ART/epre.json" "$ART/epost.json" "$ART/eap.out" "$ART/espec.json" > "$ART/edel.out" 2>>"$DIAG"; then echo "PASS: e-apply-delta ($(cat "$ART/edel.out"))"; PASS=$((PASS + 1)); else echo "FAIL: e-apply-delta"; FAIL=$((FAIL + 1)); fi
# Empty and nonempty zero-move applies still record closure + bump once.
expect e-empty-preview "$($CURL -o "$ART/ee.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e-epv' -d '{"table_id":"t_3","from":"2027-06-18T18:00:00+02:00","to":"2027-06-18T19:00:00+02:00"}')" 201
chmod 600 "$ART/ee.out"
EPID0=$(py -c 'import json; print(json.load(open("'"$ART"'/ee.out"))["plan_id"])' 2>>"$DIAG")
export_ok "$ART/eepre.json" && { echo "PASS: e-empty-export-pre (200)"; PASS=$((PASS + 1)); }
expect e-empty-apply "$($CURL -o "$ART/eeap.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$EPID0/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e-eap' -d '{}')" 201
chmod 600 "$ART/eeap.out"
export_ok "$ART/eepost.json" && { echo "PASS: e-empty-export-post (200)"; PASS=$((PASS + 1)); }
cat > "$ART/eespec.json" <<EOF
{"plan_id":"$EPID0","restaurant":"r","path":"/restaurants/r/replans/$EPID0/apply","key":"e-eap","owner":"m","body":"{}",
 "order":[],"revision":2,"moved":{},"unmoved":["EE0001","EE0002","RO0001"]}
EOF
chmod 600 "$ART/eespec.json"
if py "$ART/apply_delta.py" "$ART/eepre.json" "$ART/eepost.json" "$ART/eeap.out" "$ART/eespec.json" > "$ART/eedel.out" 2>>"$DIAG"; then echo "PASS: e-empty-delta ($(cat "$ART/eedel.out"))"; PASS=$((PASS + 1)); else echo "FAIL: e-empty-delta"; FAIL=$((FAIL + 1)); fi
pycheck e-empty-shape - "$ART/eeap.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["reservations"] == [] and d["restaurant_revision"] == 2, d
print("EMPTY-OK")
PYEOF

# Nonempty zero-move apply: considered but unchanged, full delta pins
# closure+counter+plan+receipt only, with records/histories frozen.
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "manager_user_ids":["m"]}],
 "reservations": [{"id":"s1","reference":"NZ0001","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect e2-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: e2-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect e2-preview "$($CURL -o "$ART/e2pv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e2-pv' -d '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T20:30:00+02:00"}')" 201
chmod 600 "$ART/e2pv.out"
E2PID=$(py -c 'import json; print(json.load(open("'"$ART"'/e2pv.out"))["plan_id"])' 2>>"$DIAG")
export_ok "$ART/e2pre.json" && { echo "PASS: e2-export-pre (200)"; PASS=$((PASS + 1)); }
expect e2-apply "$($CURL -o "$ART/e2ap.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$E2PID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: e2-ap' -d '{}')" 201
chmod 600 "$ART/e2ap.out"
export_ok "$ART/e2post.json" && { echo "PASS: e2-export-post (200)"; PASS=$((PASS + 1)); }
cat > "$ART/e2spec.json" <<EOF
{"plan_id":"$E2PID","restaurant":"r","path":"/restaurants/r/replans/$E2PID/apply","key":"e2-ap","owner":"m","body":"{}",
 "order":["NZ0001"],"revision":1,"moved":{},"unmoved":["NZ0001"]}
EOF
chmod 600 "$ART/e2spec.json"
if py "$ART/apply_delta.py" "$ART/e2pre.json" "$ART/e2post.json" "$ART/e2ap.out" "$ART/e2spec.json" > "$ART/e2del.out" 2>>"$DIAG"; then echo "PASS: e2-nonempty-zero-delta ($(cat "$ART/e2del.out"))"; PASS=$((PASS + 1)); else echo "FAIL: e2-nonempty-zero-delta"; FAIL=$((FAIL + 1)); fi
echo "== F stale, already-applied, replays, series =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]},
  {"id":"ro","name":"O","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"q_1","label":"1","capacity":4}],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"FF0001","user_id":"m","restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":2},
  {"id":"s2","reference":"FF0002","user_id":"m","restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect f-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: f-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
FCLO='{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-06-17T23:00:00+02:00"}'
fresh_preview() {
  $CURL -o "$ART/fpvx.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H "Idempotency-Key: $1" -d "$FCLO" 2>>"$DIAG"
  chmod 600 "$ART/fpvx.out"
}
stale_case() {
  # $1 = name. Fresh preview (required 201) captures the current revision;
  # the named staling write must itself succeed with counter advance; the
  # stale apply is then 409 stale_plan with whole-export equality and no
  # receipt for the losing key.
  if [ "$(fresh_preview "f-pv-$1")" != "201" ]; then echo "FAIL: f-$1-preview"; FAIL=$((FAIL + 1)); return; fi
  echo "PASS: f-$1-preview (201)"; PASS=$((PASS + 1))
  FPIDX=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvx.out"))["plan_id"])' 2>>"$DIAG")
  PREVREV=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvx.out"))["restaurant_revision"])' 2>>"$DIAG")
  case $1 in
    create)
      code=$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-create' -d '{"restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}') ;;
    real-patch)
      code=$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/FF0002" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d '{"party_size":1}') ;;
    cancel)
      code=$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/FF0002/cancel" -H "Authorization: Bearer $TOK_M") ;;
    publish)
      code=$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-pol' -d '{"effective_from":"2027-08-01","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}') ;;
    other-apply)
      $CURL -o "$ART/fpvb.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-pv2' -d "$FCLO" > /dev/null 2>>"$DIAG"
      chmod 600 "$ART/fpvb.out"
      FPID2=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvb.out"))["plan_id"])' 2>>"$DIAG")
      code=$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$FPID2/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-ap2' -d '{}') ;;
  esac
  case $1 in
    create|publish|other-apply) want=201 ;;
    *) want=200 ;;
  esac
  expect "f-mutate-$1" "$code" "$want"
  export_ok "$ART/fcnt-$1.json" && { echo "PASS: f-$1-counter-export (200)"; PASS=$((PASS + 1)); }
  pycheck "f-mutate-$1-counter" - "$ART/fcnt-$1.json" "$PREVREV" <<'PYEOF'
import json,sys
st = json.load(open(sys.argv[1]))["state"]
assert st["restaurant_revisions"]["r"] == int(sys.argv[2]) + 1, st["restaurant_revisions"]
print("CNT-OK")
PYEOF
  export_ok "$ART/fpre.json" && { echo "PASS: f-$1-export-pre (200)"; PASS=$((PASS + 1)); }
  expect "f-stale-$1" "$($CURL -o "$ART/fstale.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$FPIDX/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H "Idempotency-Key: f-ap-$1" -d '{}')" 409
  chmod 600 "$ART/fstale.out"
  pycheck "f-stale-$1-code" - "$ART/fstale.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "stale_plan"
print("STALE-OK")
PYEOF
  export_ok "$ART/fpost.json" && { echo "PASS: f-$1-export-post (200)"; PASS=$((PASS + 1)); }
  if cmp -s "$ART/fpre.json" "$ART/fpost.json"; then echo "PASS: f-stale-$1-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: f-stale-$1-atomic"; FAIL=$((FAIL + 1)); fi
  pycheck "f-stale-$1-noreceipt" - "$ART/fpost.json" "f-ap-$1" <<'PYEOF'
import json,sys
rcs = json.load(open(sys.argv[1]))["state"]["receipts"]
assert all(not (v["key"] == sys.argv[2] and v["path"].startswith("/restaurants/r/replans/")) for v in rcs.values()), "loser receipt stored"
print("NORECEIPT-OK")
PYEOF
  # The failed key is reusable on a fresh plan. Note the fresh plan means a
  # different idempotency path, so this proves new-scope reuse after failure,
  # not same-path replay: genuine same-path failed-key reuse is proven by
  # the preview/create/move/adopt scenarios elsewhere.
  fresh_preview "f-pv-$1b" > /dev/null
  FPIDB=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvx.out"))["plan_id"])' 2>>"$DIAG")
  expect "f-newscope-$1" "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$FPIDB/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H "Idempotency-Key: f-ap-$1" -d '{}')" 201
}
stale_case create
stale_case real-patch
stale_case cancel
stale_case publish
stale_case other-apply
# Non-staling writes: replay, no-op, failed write, extra preview and an
# other-restaurant write leave a fresh plan applicable.
fresh_preview "f-pv-keep" > /dev/null
KEEP=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvx.out"))["plan_id"])' 2>>"$DIAG")
$CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-create' -d '{"restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}' > "$SCR/keep-replay.code" 2>>"$DIAG"
expect f-keep-replay "$(cat "$SCR/keep-replay.code")" 200
expect f-keep-noop "$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/FF0001" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d '{}')" 200
expect f-keep-failed "$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/FF0001" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d '{"party_size":0}')" 422
fresh_preview "f-pv-keep2" > /dev/null
expect f-keep-other-rest "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-keep-ro' -d '{"restaurant_id":"ro","table_id":"q_1","starts_at_local":"2027-06-17T19:00","party_size":1}')" 201
expect f-keep-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$KEEP/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-keep-ap' -d '{}')" 201
# Already-applied under a new key wins over staleness; the original key
# replays byte-exact even after later writes.
fresh_preview "f-pv-aa" > /dev/null
AAPID=$(py -c 'import json; print(json.load(open("'"$ART"'/fpvx.out"))["plan_id"])' 2>>"$DIAG")
expect f-aa-first "$($CURL -o "$ART/faa.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$AAPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-aa1' -d '{}')" 201
chmod 600 "$ART/faa.out"
expect f-aa-second "$($CURL -o "$ART/faa2.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$AAPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-aa2' -d '{}')" 409
chmod 600 "$ART/faa2.out"
pycheck f-aa-code - "$ART/faa2.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "plan_already_applied"
print("ALREADY-OK")
PYEOF
$CURL -o "$ART/faa-look-pre.out" -G "$BASE/reservations/FF0001" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/faa-look-pre.out"
export_ok "$ART/faa-cnt-pre.json" && { echo "PASS: f-aa-counter-pre (200)"; PASS=$((PASS + 1)); }
expect f-aa-patch "$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/FF0001" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -d '{"party_size":1}')" 200
$CURL -o "$ART/faa-look-post.out" -G "$BASE/reservations/FF0001" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/faa-look-post.out"
pycheck f-aa-patch-proof - "$ART/faa-look-pre.out" "$ART/faa-look-post.out" "$ART/faa-cnt-pre.json" "$ART/faa.out" <<'PYEOF'
import json,sys
b = json.load(open(sys.argv[1]))
a = json.load(open(sys.argv[2]))
pre = json.load(open(sys.argv[3]))["state"]
orig = json.load(open(sys.argv[4]))
assert a["revision"] == b["revision"] + 1 and a["party_size"] == 1 and b["party_size"] == 2, (b, a)
origrec = [r for r in orig["reservations"] if r["reference"] == "FF0001"][0]
assert a != origrec, "current must differ from original apply receipt"
print("AAPATCH-OK")
PYEOF
export_ok "$ART/faapre.json" && { echo "PASS: f-aa-export-pre (200)"; PASS=$((PASS + 1)); }
pycheck f-aa-counter-proof - "$ART/faa-cnt-pre.json" "$ART/faapre.json" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
assert post["restaurant_revisions"]["r"] == pre["restaurant_revisions"]["r"] + 1, (pre["restaurant_revisions"], post["restaurant_revisions"])
print("AACNT-OK")
PYEOF
expect f-aa-replay "$($CURL -o "$ART/faar.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$AAPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-aa1' -d '{}')" 200
chmod 600 "$ART/faar.out"
if cmp -s "$ART/faa.out" "$ART/faar.out"; then echo "PASS: f-aa-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: f-aa-replay-bytes"; FAIL=$((FAIL + 1)); fi
export_ok "$ART/faapost.json" && { echo "PASS: f-aa-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/faapre.json" "$ART/faapost.json"; then echo "PASS: f-aa-replay-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: f-aa-replay-atomic"; FAIL=$((FAIL + 1)); fi
# Series propagation: two adopted series, one with two members in the
# closure window; each affected series bumps exactly once, flags and
# schedules retained, unrelated series byte-equal, reassigned never changed.
expect f-anchor "$($CURL -o "$ART/fanc.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-a' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-24T19:00","party_size":2}')" 201
chmod 600 "$ART/fanc.out"
AREF1=$(py -c 'import json; print(json.load(open("'"$ART"'/fanc.out"))["reference"])' 2>>"$DIAG")
$CURL -o "$ART/fbook.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-m-create' -d '{"restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/fbook.out"
FBOOK=$(py -c 'import json; print(json.load(open("'"$ART"'/fbook.out"))["reference"])' 2>>"$DIAG")
expect f-adopt1 "$($CURL -o "$ART/fad1.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-1' -d '{"anchor_reference":"'"$AREF1"'","count":3,"interval_weeks":1}')" 201
chmod 600 "$ART/fad1.out"
SID1=$(py -c 'import json; print(json.load(open("'"$ART"'/fad1.out"))["series_id"])' 2>>"$DIAG")
S1G1=$(py -c 'import json; print(json.load(open("'"$ART"'/fad1.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
S1G2=$(py -c 'import json; print(json.load(open("'"$ART"'/fad1.out"))["occurrences"][2]["reservation"]["reference"])' 2>>"$DIAG")
expect f-anchor2 "$($CURL -o "$ART/fanc2.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-b' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-07-01T21:30","party_size":2}')" 201
chmod 600 "$ART/fanc2.out"
AREF2=$(py -c 'import json; print(json.load(open("'"$ART"'/fanc2.out"))["reference"])' 2>>"$DIAG")
expect f-adopt2 "$($CURL -o "$ART/fad2.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-2' -d '{"anchor_reference":"'"$AREF2"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/fad2.out"
SID2=$(py -c 'import json; print(json.load(open("'"$ART"'/fad2.out"))["series_id"])' 2>>"$DIAG")
S2G1=$(py -c 'import json; print(json.load(open("'"$ART"'/fad2.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
expect f-anchor3 "$($CURL -o "$ART/fanc3.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-c' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-07-15T19:00","party_size":1}')" 201
chmod 600 "$ART/fanc3.out"
AREF3=$(py -c 'import json; print(json.load(open("'"$ART"'/fanc3.out"))["reference"])' 2>>"$DIAG")
expect f-adopt3 "$($CURL -o "$ART/fad3.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-3' -d '{"anchor_reference":"'"$AREF3"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/fad3.out"
SID3=$(py -c 'import json; print(json.load(open("'"$ART"'/fad3.out"))["series_id"])' 2>>"$DIAG")
S3G1=$(py -c 'import json; print(json.load(open("'"$ART"'/fad3.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
export_ok "$ART/fsexp-pre.json" && { echo "PASS: f-series-export-pre (200)"; PASS=$((PASS + 1)); }
expect f-series-preview "$($CURL -o "$ART/fspv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-pv' -d '{"table_id":"t_2","from":"2027-06-17T18:00:00+02:00","to":"2027-07-01T23:00:00+02:00"}')" 201
chmod 600 "$ART/fspv.out"
SPID=$(py -c 'import json; print(json.load(open("'"$ART"'/fspv.out"))["plan_id"])' 2>>"$DIAG")
export_ok "$ART/fsexp-mid.json" && { echo "PASS: f-series-export-mid (200)"; PASS=$((PASS + 1)); }
pycheck f-series-plan-binds - "$ART/fsexp-pre.json" "$ART/fsexp-mid.json" "$ART/fspv.out" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
mid = json.load(open(sys.argv[2]))["state"]
pv = json.load(open(sys.argv[3]))
assert set(mid["plans"]) == set(pre["plans"]) | {pv["plan_id"]}, "plan keys"
for k, v in pre["plans"].items():
    assert mid["plans"][k] == v, "prior plan changed"
st = mid["plans"][pv["plan_id"]]
assert st["applied"] is False
assert st["restaurant_id"] == "r", st["restaurant_id"]
for k in ("plan_id","restaurant_revision","closure","assignments","moved_count","unused_seats"):
    assert st[k] == pv[k], "stored plan field %s" % k
assert len(mid["receipts"]) == len(pre["receipts"]) + 1, "preview receipt count"
for k, v in pre["receipts"].items():
    assert mid["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in mid["receipts"].items() if k not in pre["receipts"]][0]
assert (new_rc["user_id"], new_rc["method"], new_rc["path"], new_rc["key"], new_rc["status"]) == \
    ("m", "POST", "/restaurants/r/replans", "f-ser-pv", 201), new_rc
assert new_rc["body"] == '{"from":"2027-06-17T18:00:00+02:00","table_id":"t_2","to":"2027-07-01T23:00:00+02:00"}', new_rc["body"]
assert new_rc["response"] == open(sys.argv[3]).read(), "receipt != raw preview bytes"
for k, v in pre.items():
    if k in ("plans", "receipts"):
        continue
    assert mid[k] == v, "preview changed %s" % k
print("PLANBIND-OK")
PYEOF
expect f-series-apply "$($CURL -o "$ART/fsap.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$SPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: f-ser-ap' -d '{}')" 201
chmod 600 "$ART/fsap.out"
export_ok "$ART/fsexp-post.json" && { echo "PASS: f-series-export-post (200)"; PASS=$((PASS + 1)); }
# Series allowed delta, driven by the shared checker: the spec is built
# from live captured inputs only (pre-export table sets, preview
# assignments, apply response order/revision), never from service output
# beyond those bindings. Moved set is exactly the two S1 members plus S2's
# anchor; every other record is unmoved; both affected series advance once.
pycheck f-series-spec - "$ART/fsexp-pre.json" "$ART/fspv.out" "$ART/fsap.out" "$SID1" "$SID2" "$AREF1" "$S1G1" "$AREF2" "$SPID" "$ART/fspec.json" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
pv = json.load(open(sys.argv[2]))
ap = json.load(open(sys.argv[3]))
sid1, sid2, a1, g1, a2, pid = sys.argv[4], sys.argv[5], sys.argv[6], sys.argv[7], sys.argv[8], sys.argv[9]
out = sys.argv[10]
assign = {a["reference"]: a["table_ids"] for a in pv["assignments"]}
moved_refs = [a1, g1, a2]
for ref in moved_refs:
    assert ref in assign, "moved ref missing from preview"
    assert assign[ref] != pre["reservations"][ref]["table_ids"], "not actually moved"
moved = {ref: {"from": pre["reservations"][ref]["table_ids"], "to": assign[ref]} for ref in moved_refs}
unmoved = sorted(r for r in pre["reservations"] if r not in moved)
series = {sid1: {"before": pre["series"][sid1]["revision"], "after": pre["series"][sid1]["revision"] + 1},
          sid2: {"before": pre["series"][sid2]["revision"], "after": pre["series"][sid2]["revision"] + 1}}
spec = {"plan_id": pid, "restaurant": "r",
        "path": "/restaurants/r/replans/%s/apply" % pid, "key": "f-ser-ap",
        "owner": "m", "body": "{}",
        "order": [r["reference"] for r in ap["reservations"]],
        "revision": ap["restaurant_revision"],
        "moved": moved, "unmoved": unmoved, "series": series}
json.dump(spec, open(out, "w"))
print("SPEC-OK moved=%s" % sorted(moved))
PYEOF
chmod 600 "$ART/fspec.json"
if py "$ART/apply_delta.py" "$ART/fsexp-mid.json" "$ART/fsexp-post.json" "$ART/fsap.out" "$ART/fspec.json" > "$ART/fsdel.out" 2>>"$DIAG"; then echo "PASS: f-series-delta ($(cat "$ART/fsdel.out"))"; PASS=$((PASS + 1)); else echo "FAIL: f-series-delta"; FAIL=$((FAIL + 1)); fi
echo "== G closure availability, explain, write enforcement =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"M"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],
  "combinable":[["t_1","t_2"],["t_2","t_3"]],"manager_user_ids":["m"]},
  {"id":"ro","name":"O","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"q_1","label":"1","capacity":4}],"manager_user_ids":["m"]}],
 "reservations": [
  {"id":"s1","reference":"GG0001","user_id":"m","restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T19:00","party_size":2}]}
EOF
)
expect g-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: g-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
expect g-preview "$($CURL -o "$ART/gpv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-pv' -d '{"table_id":"t_2","from":"2027-06-17T19:30:00+02:00","to":"2027-06-17T20:30:00+02:00"}')" 201
chmod 600 "$ART/gpv.out"
pycheck g-preview-shape - "$ART/gpv.out" <<'PYEOF2'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":"GG0001","table_ids":["t_3"],"changed":False}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (0,2), d
print("GPV-OK")
PYEOF2
GPID=$(py -c 'import json; print(json.load(open("'"$ART"'/gpv.out"))["plan_id"])' 2>>"$DIAG")
export_ok "$ART/gapp-pre.json" && { echo "PASS: g-apply-export-pre (200)"; PASS=$((PASS + 1)); }
expect g-apply "$($CURL -o "$ART/gap.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$GPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-ap' -d '{}')" 201
chmod 600 "$ART/gap.out"
export_ok "$ART/gapp-post.json" && { echo "PASS: g-apply-export-post (200)"; PASS=$((PASS + 1)); }
cat > "$ART/gspec.json" <<EOF
{"plan_id":"$GPID","restaurant":"r","path":"/restaurants/r/replans/$GPID/apply","key":"g-ap","owner":"m","body":"{}",
 "order":["GG0001"],"revision":1,"moved":{},"unmoved":["GG0001"]}
EOF
chmod 600 "$ART/gspec.json"
if py "$ART/apply_delta.py" "$ART/gapp-pre.json" "$ART/gapp-post.json" "$ART/gap.out" "$ART/gspec.json" > "$ART/gdel.out" 2>>"$DIAG"; then echo "PASS: g-apply-delta ($(cat "$ART/gdel.out"))"; PASS=$((PASS + 1)); else echo "FAIL: g-apply-delta"; FAIL=$((FAIL + 1)); fi
# Availability + explain at the 19:00 slot, party 4: all four rule combos.
$CURL -o "$ART/gav.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/gav.out"
pycheck g-explain-matrix - "$ART/gav.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T19:00"][0]
assert [e["table_id"] for e in sl["explain"]] == ["t_1","t_2","t_3"], "fixture order"
for e in sl["explain"]:
    assert [r["rule"] for r in e["rules"]] == ["capacity","no_overlap"], e
    assert e["policy_version"] == 0, e
    holds = {r["rule"]: r["holds"] for r in e["rules"]}
    assert e["available"] == (holds["capacity"] and holds["no_overlap"]), e
got = {e["table_id"]: ({r["rule"]: r["holds"] for r in e["rules"]}, e["available"]) for e in sl["explain"]}
assert got["t_1"] == ({"capacity":False,"no_overlap":True}, False), got["t_1"]
assert got["t_2"] == ({"capacity":True,"no_overlap":False}, False), got["t_2"]
assert got["t_3"] == ({"capacity":True,"no_overlap":False}, False), got["t_3"]
assert sl["available_table_ids"] == [], sl["available_table_ids"]
assert sl["available_options"] == [], sl["available_options"]
print("EXPLAIN-OK")
PYEOF
# Free slot true/true with exact ids/options; no-explain keeps legacy shape.
$CURL -o "$ART/gav2.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/gav2.out"
pycheck g-free-slot - "$ART/gav2.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T21:30"][0]
assert sl["available_table_ids"] == ["t_2","t_3"], sl["available_table_ids"]
assert sl["available_options"] == [{"table_ids":["t_2"],"capacity":4},
    {"table_ids":["t_3"],"capacity":4},{"table_ids":["t_1","t_2"],"capacity":6},
    {"table_ids":["t_2","t_3"],"capacity":8}], sl["available_options"]
got = {e["table_id"]: ({r["rule"]: r["holds"] for r in e["rules"]}, e["available"]) for e in sl["explain"]}
assert got["t_3"] == ({"capacity":True,"no_overlap":True}, True), got["t_3"]
assert got["t_2"] == ({"capacity":True,"no_overlap":True}, True), got["t_2"]
print("FREE-OK")
PYEOF
$CURL -o "$ART/gav3.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=2 2>>"$DIAG"
chmod 600 "$ART/gav3.out"
pycheck g-no-explain-shape - "$ART/gav3.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert sorted(d.keys()) == ["date","restaurant_id","slots","timezone"], d.keys()
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T19:00"][0]
assert sorted(sl.keys()) == ["available_options","available_table_ids","starts_at","starts_at_local"], sl.keys()
assert "t_2" not in sl["available_table_ids"], sl["available_table_ids"]
assert all("t_2" not in o["table_ids"] for o in sl["available_options"]), sl["available_options"]
print("NOEXPLAIN-OK")
PYEOF
# Adjacent-before: slot 18:00 ends exactly when the closure starts.
$CURL -o "$ART/gav4.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/gav4.out"
pycheck g-adjacent-before - "$ART/gav4.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T18:00"][0]
assert sl["available_table_ids"] == ["t_2"], sl["available_table_ids"]
got = {e["table_id"]: ({r["rule"]: r["holds"] for r in e["rules"]}, e["available"]) for e in sl["explain"]}
assert got["t_2"] == ({"capacity":True,"no_overlap":True}, True), got["t_2"]
assert got["t_3"] == ({"capacity":True,"no_overlap":False}, False), got["t_3"]
print("BEFORE-OK")
PYEOF
# Adjacent-after: slot 20:30 starts exactly when the closure ends.
$CURL -o "$ART/gav5.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 2>>"$DIAG"
chmod 600 "$ART/gav5.out"
pycheck g-adjacent-after - "$ART/gav5.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T20:30"][0]
assert sl["available_table_ids"] == ["t_2","t_3"], sl["available_table_ids"]
print("AFTER-OK")
PYEOF
# Short-inner closure strictly inside a slot, expressed in +00:00 to prove
# absolute-offset equivalence; the other restaurant stays isolated.
$CURL -o "$ART/riso-pre.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/riso-pre.out"
$CURL -o "$ART/gropv.out" -w '%{http_code}' -X POST "$BASE/restaurants/ro/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-ropv' -d '{"table_id":"q_1","from":"2027-06-17T17:10:00+00:00","to":"2027-06-17T17:20:00+00:00"}' > "$SCR/gropv.code" 2>>"$DIAG"
chmod 600 "$ART/gropv.out"
expect g-ro-preview "$(cat "$SCR/gropv.code")" 201
ROPID=$(py -c 'import json; print(json.load(open("'"$ART"'/gropv.out"))["plan_id"])' 2>>"$DIAG")
expect g-ro-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/ro/replans/$ROPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-roap' -d '{}')" 201
$CURL -o "$ART/gav6.out" -G "$BASE/availability" --data-urlencode restaurant_id=ro --data-urlencode date=2027-06-17 --data-urlencode party_size=1 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/gav6.out"
pycheck g-short-inner - "$ART/gav6.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T19:00"][0]
assert sl["available_table_ids"] == [], sl["available_table_ids"]
e = sl["explain"][0]
assert e["table_id"] == "q_1" and e["available"] is False, e
assert {r["rule"]: r["holds"] for r in e["rules"]} == {"capacity":True,"no_overlap":False}, e
print("INNER-OK")
PYEOF
$CURL -o "$ART/riso-post.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=4 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/riso-post.out"
if cmp -s "$ART/riso-pre.out" "$ART/riso-post.out"; then echo "PASS: g-other-rest-isolation (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-other-rest-isolation"; FAIL=$((FAIL + 1)); fi
# Closure-only F/F: party 8 at t_2 with no booking on t_2 at 19:00.
$CURL -o "$ART/gav7.out" -G "$BASE/availability" --data-urlencode restaurant_id=r --data-urlencode date=2027-06-17 --data-urlencode party_size=8 --data-urlencode explain=true 2>>"$DIAG"
chmod 600 "$ART/gav7.out"
pycheck g-explain-ff - "$ART/gav7.out" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
sl = [s for s in d["slots"] if s["starts_at_local"] == "2027-06-17T19:00"][0]
assert [e["table_id"] for e in sl["explain"]] == ["t_1","t_2","t_3"], "fixture order"
for e in sl["explain"]:
    assert [r["rule"] for r in e["rules"]] == ["capacity","no_overlap"], e
    assert e["policy_version"] == 0, e
    holds = {r["rule"]: r["holds"] for r in e["rules"]}
    assert e["available"] == (holds["capacity"] and holds["no_overlap"]), e
got = {e["table_id"]: ({r["rule"]: r["holds"] for r in e["rules"]}, e["available"]) for e in sl["explain"]}
assert got["t_2"] == ({"capacity":False,"no_overlap":False}, False), got["t_2"]
assert sl["available_table_ids"] == [], sl["available_table_ids"]
assert sl["available_options"] == [], sl["available_options"]
print("FF-OK")
PYEOF
# Write enforcement through the common closure seam + per-path reuse.
closed_reject() {
  # $1=name $2=method $3=path $4=key-or-empty $5=body-or-empty.
  # Captures the error JSON, pins table_unavailable, and requires whole-
  # export equality across the single rejected operation.
  export_ok "$ART/gw-$1-pre.json" && { echo "PASS: g-$1-export-pre (200)"; PASS=$((PASS + 1)); }
  if [ -n "$4" ]; then
    code=$($CURL -o "$ART/gw-$1.out" -w '%{http_code}' -X "$2" "$BASE$3" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H "Idempotency-Key: $4" -d "$5")
  else
    code=$($CURL -o "$ART/gw-$1.out" -w '%{http_code}' -X "$2" "$BASE$3" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" ${5:+-d "$5"})
  fi
  chmod 600 "$ART/gw-$1.out"
  expect "g-$1-status" "$code" 409
  pycheck "g-$1-code" - "$ART/gw-$1.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "table_unavailable"
print("REJ-OK")
PYEOF
  export_ok "$ART/gw-$1-post.json" && { echo "PASS: g-$1-export-post (200)"; PASS=$((PASS + 1)); }
  if cmp -s "$ART/gw-$1-pre.json" "$ART/gw-$1-post.json"; then echo "PASS: g-$1-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-$1-atomic"; FAIL=$((FAIL + 1)); fi
}
closed_reject create POST /reservations g-cr '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-17T19:00","party_size":1}'
closed_reject pair POST /reservations g-pr '{"restaurant_id":"r","table_ids":["t_2","t_3"],"starts_at_local":"2027-06-17T19:00","party_size":2}'
closed_reject patch PATCH /reservations/GG0001 "" '{"table_id":"t_2"}'
closed_reject move POST /reservation-moves g-mv '{"moves":[{"reference":"GG0001","table_id":"t_2"}]}' 
expect g-anchor "$($CURL -o "$ART/ganch.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-anch' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-10T19:00","party_size":1}')" 201
chmod 600 "$ART/ganch.out"
expect g-anchor-replay "$($CURL -o "$ART/ganchr.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-anch' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-10T19:00","party_size":1}')" 200
chmod 600 "$ART/ganchr.out"
if cmp -s "$ART/ganch.out" "$ART/ganchr.out"; then echo "PASS: g-anchor-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-anchor-replay-bytes"; FAIL=$((FAIL + 1)); fi
GAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/ganch.out"))["reference"])' 2>>"$DIAG")
export_ok "$ART/gexp-pre2.json" && { echo "PASS: g-export-pre2 (200)"; PASS=$((PASS + 1)); }
expect g-adopt-closed "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-ad' -d '{"anchor_reference":"'"$GAREF"'","count":2,"interval_weeks":1}')" 409
export_ok "$ART/gexp-post2.json" && { echo "PASS: g-export-post2 (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/gexp-pre2.json" "$ART/gexp-post2.json"; then echo "PASS: g-adopt-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-adopt-atomic"; FAIL=$((FAIL + 1)); fi
# Per-path failed keys succeed genuinely on adjacent/nonconflicting slots.
expect g-create-reuse "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-cr' -d '{"restaurant_id":"r","table_id":"t_3","starts_at_local":"2027-06-17T21:30","party_size":1}')" 201
expect g-pair-reuse "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-pr' -d '{"restaurant_id":"r","table_ids":["t_1","t_2"],"starts_at_local":"2027-06-17T18:00","party_size":2}')" 201
export_ok "$ART/gmv-pre.json" && { echo "PASS: g-move-export-pre (200)"; PASS=$((PASS + 1)); }
expect g-move-reuse "$($CURL -o "$ART/gmv.out" -w '%{http_code}' -X POST "$BASE/reservation-moves" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-mv' -d '{"moves":[{"reference":"GG0001","table_id":"t_1","starts_at_local":"2027-06-17T21:30"}]}')" 201
chmod 600 "$ART/gmv.out"
$CURL -o "$ART/gmv-rec.out" -G "$BASE/reservations/GG0001" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/gmv-rec.out"
pycheck g-move-changed - "$ART/gmv.out" "$ART/gmv-rec.out" "$ART/gmv-pre.json" <<'PYEOF'
import json,sys
m = json.load(open(sys.argv[1]))
r = json.load(open(sys.argv[2]))
pre = json.load(open(sys.argv[3]))["state"]
assert m["reservations"][0]["reference"] == "GG0001", m
assert r["revision"] == 2 and r["table_ids"] == ["t_1"] and r["table_id"] == "t_1", r
assert r["starts_at_local"] == "2027-06-17T21:30" and r["ends_at"] == "2027-06-17T23:00:00+02:00", r
assert pre["reservations"]["GG0001"]["revision"] == 1, "precondition drift"
print("MV-OK")
PYEOF
$CURL -o "$ART/gmv-hist.out" -G "$BASE/reservations/GG0001/history" -H "Authorization: Bearer $TOK_M" 2>>"$DIAG"
chmod 600 "$ART/gmv-hist.out"
pycheck g-move-history - "$ART/gmv-hist.out" <<'PYEOF'
import json,sys
es = json.load(open(sys.argv[1]))["entries"]
assert [e["event"] for e in es] == ["created","changed"], es
assert es[1]["changes"] == [{"field":"table_id","from":"t_3","to":"t_1"},
                            {"field":"starts_at_local","from":"2027-06-17T19:00","to":"2027-06-17T21:30"}], es[1]
print("MVH-OK")
PYEOF
export_ok "$ART/gmv-post.json" && { echo "PASS: g-move-export-post (200)"; PASS=$((PASS + 1)); }
pycheck g-move-counter - "$ART/gmv-pre.json" "$ART/gmv-post.json" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
assert post["restaurant_revisions"]["r"] == pre["restaurant_revisions"]["r"] + 1, (pre["restaurant_revisions"], post["restaurant_revisions"])
print("MVC-OK")
PYEOF
expect g-move-replay "$($CURL -o "$ART/gmvr.out" -w '%{http_code}' -X POST "$BASE/reservation-moves" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-mv' -d '{"moves":[{"reference":"GG0001","table_id":"t_1","starts_at_local":"2027-06-17T21:30"}]}')" 200
chmod 600 "$ART/gmvr.out"
if cmp -s "$ART/gmv.out" "$ART/gmvr.out"; then echo "PASS: g-move-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-move-replay-bytes"; FAIL=$((FAIL + 1)); fi
export_ok "$ART/gmv-repost.json" && { echo "PASS: g-move-export-repost (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/gmv-post.json" "$ART/gmv-repost.json"; then echo "PASS: g-move-replay-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: g-move-replay-atomic"; FAIL=$((FAIL + 1)); fi
expect g-anchor2 "$($CURL -o "$ART/ganch2.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-anch2' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-06-10T20:30","party_size":1}')" 201
chmod 600 "$ART/ganch2.out"
GAREF2=$(py -c 'import json; print(json.load(open("'"$ART"'/ganch2.out"))["reference"])' 2>>"$DIAG")
expect g-adopt-reuse "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_M" -H 'Idempotency-Key: g-ad' -d '{"anchor_reference":"'"$GAREF2"'","count":2,"interval_weeks":1}')" 201


echo "== H amend auth, routes, input matrix =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"m","email":"m@x","password":"password12","display_name":"Mgr"},
 {"id":"o","email":"o@x","password":"password12","display_name":"Own"},
 {"id":"p","email":"p@x","password":"password12","display_name":"Pln"},
 {"id":"q","email":"q@x","password":"password12","display_name":"QMrg"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["m","q"]}],
 "reservations": []}
EOF
)
expect h-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: h-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_M=$(login "m@x" "password12")
TOK_M_CODE=${TOK_M%%:*}; TOK_M=${TOK_M#*:}
if [ "$TOK_M_CODE" = "200" ] && [ -n "$TOK_M" ]; then echo "PASS: login-m (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-m"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_O=$(login "o@x" "password12")
TOK_O_CODE=${TOK_O%%:*}; TOK_O=${TOK_O#*:}
if [ "$TOK_O_CODE" = "200" ] && [ -n "$TOK_O" ]; then echo "PASS: login-o (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-o"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_P=$(login "p@x" "password12")
TOK_P_CODE=${TOK_P%%:*}; TOK_P=${TOK_P#*:}
if [ "$TOK_P_CODE" = "200" ] && [ -n "$TOK_P" ]; then echo "PASS: login-p (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-p"; FAIL=$((FAIL + 1)); exit 1; fi
TOK_Q=$(login "q@x" "password12")
TOK_Q_CODE=${TOK_Q%%:*}; TOK_Q=${TOK_Q#*:}
if [ "$TOK_Q_CODE" = "200" ] && [ -n "$TOK_Q" ]; then echo "PASS: login-q (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-q"; FAIL=$((FAIL + 1)); exit 1; fi
expect h-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}')" 201
$CURL -o "$ART/hanch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/hanch.out"
HAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/hanch.out"))["reference"])' 2>>"$DIAG")
expect h-adopt "$($CURL -o "$ART/had.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-ad' -d '{"anchor_reference":"'"$HAREF"'","count":3,"interval_weeks":1}')" 201
chmod 600 "$ART/had.out"
HSID=$(py -c 'import json; print(json.load(open("'"$ART"'/had.out"))["series_id"])' 2>>"$DIAG")
HVALID='{"expected_revision":1,"from_index":0,"local_time":"20:00"}'
amend_post() {
  # $1 token (empty = none), $2 key (empty = none), $3 series id, $4 body.
  if [ -n "$1" ] && [ -n "$2" ]; then
    $CURL -o "$ART/hresp.out" -w '%{http_code}' -X POST "$BASE/series/$3/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $1" -H "Idempotency-Key: $2" -d "$4" 2>>"$DIAG"
  elif [ -n "$1" ]; then
    $CURL -o "$ART/hresp.out" -w '%{http_code}' -X POST "$BASE/series/$3/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $1" -d "$4" 2>>"$DIAG"
  else
    $CURL -o "$ART/hresp.out" -w '%{http_code}' -X POST "$BASE/series/$3/amend" -H 'Content-Type: application/json' -H "Idempotency-Key: $2" -d "$4" 2>>"$DIAG"
  fi
  chmod 600 "$ART/hresp.out"
}
expect h-no-token "$(amend_post "" h-k1 "$HSID" "$HVALID")" 401
expect h-bad-token "$(amend_post bogus h-k2 "$HSID" "$HVALID")" 401
expect h-other-owner "$(amend_post "$TOK_P" h-k3 "$HSID" "$HVALID")" 404
expect h-manager-nonowner "$(amend_post "$TOK_Q" h-k4 "$HSID" "$HVALID")" 404
expect h-unknown-series "$(amend_post "$TOK_O" h-k5 nosuchseries "$HVALID")" 404
for m in GET PUT PATCH DELETE; do
  code=$($CURL -o /dev/null -w '%{http_code}' -X "$m" "$BASE/series/$HSID/amend" -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-wm' -d "$HVALID" 2>>"$DIAG")
  expect "h-wrong-method-$m" "$code" 404
done
expect h-trailing-slash "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$HSID/amend/" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-t1' -d "$HVALID")" 404
expect h-deeper "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$HSID/amend/x" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-t2' -d "$HVALID")" 404
expect h-empty-id "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series//amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -H 'Idempotency-Key: h-t3' -d "$HVALID")" 404
expect h-malformed "$(amend_post "$TOK_O" h-k6 "$HSID" '{oops')" 400
expect h-missing-key "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$HSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_O" -d "$HVALID")" 400
export_ok "$ART/hpre.json" && { echo "PASS: h-export-pre (200)"; PASS=$((PASS + 1)); }
h422() {
  expect "h-$1" "$(amend_post "$TOK_O" "h-m$2" "$HSID" "$3")" 422
}
h422 rev-absent 01 '{"from_index":0,"local_time":"20:00"}'
h422 rev-bool 02 '{"expected_revision":true,"from_index":0,"local_time":"20:00"}'
h422 rev-string 03 '{"expected_revision":"1","from_index":0,"local_time":"20:00"}'
h422 rev-null 04 '{"expected_revision":null,"from_index":0,"local_time":"20:00"}'
h422 rev-fraction 05 '{"expected_revision":1.5,"from_index":0,"local_time":"20:00"}'
h422 rev-zero 06 '{"expected_revision":0,"from_index":0,"local_time":"20:00"}'
h422 rev-negative 07 '{"expected_revision":-3,"from_index":0,"local_time":"20:00"}'
h422 from-absent 08 '{"expected_revision":1,"local_time":"20:00"}'
h422 from-bool 09 '{"expected_revision":1,"from_index":true,"local_time":"20:00"}'
h422 from-string 10 '{"expected_revision":1,"from_index":"0","local_time":"20:00"}'
h422 from-fraction 11 '{"expected_revision":1,"from_index":0.5,"local_time":"20:00"}'
h422 from-negative 12 '{"expected_revision":1,"from_index":-1,"local_time":"20:00"}'
h422 from-count 13 '{"expected_revision":1,"from_index":3,"local_time":"20:00"}'
h422 clock-absent 14 '{"expected_revision":1,"from_index":0}'
h422 clock-number 15 '{"expected_revision":1,"from_index":0,"local_time":2000}'
h422 clock-short 16 '{"expected_revision":1,"from_index":0,"local_time":"9:00"}'
h422 clock-hour 17 '{"expected_revision":1,"from_index":0,"local_time":"24:00"}'
h422 clock-min 18 '{"expected_revision":1,"from_index":0,"local_time":"20:60"}'
h422 clock-bare 19 '{"expected_revision":1,"from_index":0,"local_time":"2000"}'
h422 clock-empty 20 '{"expected_revision":1,"from_index":0,"local_time":""}'
export_ok "$ART/hpost.json" && { echo "PASS: h-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/hpre.json" "$ART/hpost.json"; then echo "PASS: h-matrix-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: h-matrix-atomic"; FAIL=$((FAIL + 1)); fi
pycheck h-matrix-noreceipt - "$ART/hpost.json" <<'PYEOF'
import json,sys
rcs = json.load(open(sys.argv[1]))["state"]["receipts"]
assert not [v for v in rcs.values() if v["path"].startswith("/series/") and "amend" in v["path"]], "amend receipt stored"
print("HNORECEIPT-OK")
PYEOF
expect h-exact-route "$(amend_post "$TOK_O" h-ok1 "$HSID" "$HVALID")" 201
chmod 600 "$ART/hresp.out"
cp "$ART/hresp.out" "$ART/hfirst.out"
chmod 600 "$ART/hfirst.out"
# A huge integral revision with an otherwise VALID body goes stale before
# any occurrence validation or cutoff; with an INVALID clock the top-level
# field error wins first. Both orders are pinned honestly below.
expect h-huge-stale "$(amend_post "$TOK_O" h-huge1 "$HSID" '{"expected_revision":1000000000000000000,"from_index":0,"local_time":"21:00"}')" 409
expect h-huge-valid-stale "$(amend_post "$TOK_O" h-huge2 "$HSID" '{"expected_revision":1000000000000000000,"from_index":0,"local_time":"xx"}')" 422
expect h-stale-rev "$(amend_post "$TOK_O" h-stale1 "$HSID" '{"expected_revision":999,"from_index":0,"local_time":"21:00"}')" 409
expect h-unknown-ignored "$(amend_post "$TOK_O" h-unk1 "$HSID" '{"expected_revision":2,"from_index":0,"local_time":"21:00","zzz":9}')" 201
expect h-unknown-identity "$(amend_post "$TOK_O" h-ok1 "$HSID" '{"expected_revision":1,"from_index":0,"local_time":"20:00","zzz":1}')" 409
expect h-failed-reuse "$(amend_post "$TOK_O" h-stale1 "$HSID" '{"expected_revision":3,"from_index":0,"local_time":"21:30"}')" 201
cp "$ART/hresp.out" "$ART/hreuse201.out"
chmod 600 "$ART/hreuse201.out"
expect h-reuse-replay "$(amend_post "$TOK_O" h-stale1 "$HSID" '{"expected_revision":3,"from_index":0,"local_time":"21:30"}')" 200
if cmp -s "$ART/hreuse201.out" "$ART/hresp.out"; then echo "PASS: h-reuse-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: h-reuse-bytes"; FAIL=$((FAIL + 1)); fi

echo "== I repair-then-amend workflow =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect i-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: i-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect i-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-anch' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}')" 201
$CURL -o "$ART/ianch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-anch' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T19:00","party_size":2}' 2>>"$DIAG"
chmod 600 "$ART/ianch.out"
IAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/ianch.out"))["reference"])' 2>>"$DIAG")
expect i-adopt "$($CURL -o "$ART/iad.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-ad' -d '{"anchor_reference":"'"$IAREF"'","count":3,"interval_weeks":1}')" 201
chmod 600 "$ART/iad.out"
ISID=$(py -c 'import json; print(json.load(open("'"$ART"'/iad.out"))["series_id"])' 2>>"$DIAG")
IR0=$(py -c 'import json; print(json.load(open("'"$ART"'/iad.out"))["occurrences"][0]["reservation"]["reference"])' 2>>"$DIAG")
IR1=$(py -c 'import json; print(json.load(open("'"$ART"'/iad.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
IR2=$(py -c 'import json; print(json.load(open("'"$ART"'/iad.out"))["occurrences"][2]["reservation"]["reference"])' 2>>"$DIAG")
expect i-preview "$($CURL -o "$ART/ipv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-pv' -d '{"table_id":"t_2","from":"2027-05-13T18:00:00+02:00","to":"2027-05-13T23:00:00+02:00"}')" 201
chmod 600 "$ART/ipv.out"
IPID=$(py -c 'import json; print(json.load(open("'"$ART"'/ipv.out"))["plan_id"])' 2>>"$DIAG")
pycheck i-preview-shape - "$ART/ipv.out" "$IR1" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["assignments"] == [{"reference":sys.argv[2],"table_ids":["t_1"],"changed":True}], d["assignments"]
assert (d["moved_count"],d["unused_seats"]) == (1,0), d
print("IPV-OK")
PYEOF
expect i-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$IPID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-ap' -d '{}')" 201
$CURL -o "$ART/irec.out" -G "$BASE/reservations/$IR1" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/irec.out"
pycheck i-repaired-record - "$ART/irec.out" <<'PYEOF'
import json,sys
r = json.load(open(sys.argv[1]))
assert (r["table_ids"],r["revision"],r["party_size"],r["starts_at_local"]) == (["t_1"],2,2,"2027-05-13T19:00"), r
assert r["accepted_terms"]["policy_version"] == 0, r["accepted_terms"]
print("REP-OK")
PYEOF
$CURL -o "$ART/ihist.out" -G "$BASE/reservations/$IR1/history" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/ihist.out"
pycheck i-repaired-history - "$ART/ihist.out" "$IPID" <<'PYEOF'
import json,sys
es = json.load(open(sys.argv[1]))["entries"]
assert [e["event"] for e in es] == ["created","reassigned"], es
assert es[1]["plan_id"] == sys.argv[2], es[1]
assert es[1]["changes"] == [{"field":"table_ids","from":["t_2"],"to":["t_1"]}], es[1]
print("REPH-OK")
PYEOF
VP1='{"effective_from":"2027-05-13","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}'
expect i-publish "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-pol' -d "$VP1")" 201
export_ok "$ART/ipre.json" && { echo "PASS: i-export-pre (200)"; PASS=$((PASS + 1)); }
expect i-amend "$($CURL -o "$ART/iam.out" -w '%{http_code}' -X POST "$BASE/series/$ISID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-am1' -d '{"expected_revision":2,"from_index":0,"local_time":"19:30"}')" 201
chmod 600 "$ART/iam.out"
export_ok "$ART/ipost.json" && { echo "PASS: i-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i-amend-shape - "$ART/iam.out" "$IR0" "$IR1" "$IR2" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
r0, r1, r2 = sys.argv[2], sys.argv[3], sys.argv[4]
assert sorted(d.keys()) == ["interval_weeks","occurrences","revision","series_id"], d.keys()
assert d["revision"] == 3, d["revision"]
occ = {o["reference"]: o for o in d["occurrences"]}
assert len(occ) == 3 and all(o["exception"] is False for o in occ.values()), occ
V0 = {"policy_version":0,"slot_minutes":30,"reservation_duration_minutes":90,
      "cancellation_cutoff_minutes":120,
      "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
      "capacities":{"t_1":2,"t_2":4}}
V1 = {"policy_version":1,"slot_minutes":30,"reservation_duration_minutes":60,
      "cancellation_cutoff_minutes":60,
      "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
      "capacities":{"t_1":2,"t_2":4}}
assert "effective_from" not in V0 and "effective_from" not in V1
a0 = occ[r0]["reservation"]
assert (a0["starts_at_local"],a0["table_ids"],a0["ends_at"],a0["revision"]) == \
    ("2027-05-06T19:30",["t_2"],"2027-05-06T21:00:00+02:00",2), a0
assert a0["accepted_terms"] == V0, a0["accepted_terms"]
a1 = occ[r1]["reservation"]
assert (a1["starts_at_local"],a1["table_ids"],a1["ends_at"],a1["revision"]) == \
    ("2027-05-13T19:30",["t_1"],"2027-05-13T20:30:00+02:00",3), a1
assert a1["accepted_terms"] == V1, a1["accepted_terms"]
a2 = occ[r2]["reservation"]
assert (a2["starts_at_local"],a2["table_ids"],a2["ends_at"],a2["revision"]) == \
    ("2027-05-20T19:30",["t_2"],"2027-05-20T20:30:00+02:00",2), a2
assert a2["accepted_terms"] == V1, a2["accepted_terms"]
print("IAM-OK")
PYEOF
pycheck i-amend-delta - "$ART/ipre.json" "$ART/ipost.json" "$ART/iam.out" "$IR0" "$IR1" "$IR2" "$ISID" <<'PYEOF'
import json, sys, os, subprocess
pref, postf, resp_f = sys.argv[1], sys.argv[2], sys.argv[3]
r0, r1, r2, sid = sys.argv[4], sys.argv[5], sys.argv[6], sys.argv[7]
pre = json.load(open(pref))["state"]
V0 = {"policy_version": 0, "slot_minutes": 30, "reservation_duration_minutes": 90,
      "cancellation_cutoff_minutes": 120,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "capacities": {"t_1": 2, "t_2": 4}}
V1 = {"policy_version": 1, "slot_minutes": 30, "reservation_duration_minutes": 60,
      "cancellation_cutoff_minutes": 60,
      "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
      "capacities": {"t_1": 2, "t_2": 4}}
assert pre["reservations"][r0]["accepted_terms"] == V0
assert pre["reservations"][r1]["accepted_terms"] == V0
assert pre["reservations"][r2]["accepted_terms"] == V0
rev0 = pre["series"][sid]["revision"]
assert rev0 == 2, rev0
spec = {"series": sid, "rev_before": rev0, "owner": "u1",
        "path": "/series/%s/amend" % sid, "key": "i-am1", "clock": "19:30",
        "body": '{"expected_revision":2,"from_index":0,"local_time":"19:30"}',
        "from_index": 0,
        "changed": {r0: {"date": "2027-05-06", "tables": ["t_2"], "terms": V0},
                    r1: {"date": "2027-05-13", "tables": ["t_1"], "terms": V1},
                    r2: {"date": "2027-05-20", "tables": ["t_2"], "terms": V1}},
        "unmoved": sorted(r for r in pre["reservations"] if r not in (r0, r1, r2)),
        "counter_rest": "r", "absent_keys": []}
spec_f = os.path.join(os.path.dirname(os.path.abspath(pref)), "ispec.json")
json.dump(spec, open(spec_f, "w"))
checker = os.path.join(os.path.dirname(os.path.abspath(pref)), "amend_delta.py")
r = subprocess.run([sys.executable, checker, pref, postf, resp_f, spec_f],
                   capture_output=True, text=True)
sys.stdout.write(r.stdout)
sys.stderr.write(r.stderr)
assert r.returncode == 0, "shared amend checker failed"
body = json.load(open(resp_f))
assert [m["scheduled_date"] for m in pre["series"][sid]["members"]] == ["2027-05-06", "2027-05-13", "2027-05-20"]
print("IDELTA-OK")
PYEOF
$CURL -o "$ART/iget.out" -G "$BASE/series/$ISID" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/iget.out"
if cmp -s "$ART/iam.out" "$ART/iget.out"; then echo "PASS: i-get-equals-amend (identical)"; PASS=$((PASS + 1)); else echo "FAIL: i-get-equals-amend"; FAIL=$((FAIL + 1)); fi
expect i-exception-patch "$($CURL -o /dev/null -w '%{http_code}' -X PATCH "$BASE/reservations/$IR2" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -d '{"party_size":1}')" 200
expect i-cancel-same "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/$IR2/cancel" -H "Authorization: Bearer $TOK_1")" 200
expect i-cancel-plain "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/$IR1/cancel" -H "Authorization: Bearer $TOK_1")" 200
export_ok "$ART/ifin-pre.json" && { echo "PASS: i-final-export-pre (200)"; PASS=$((PASS + 1)); }
expect i-final-amend "$($CURL -o "$ART/ifin.out" -w '%{http_code}' -X POST "$BASE/series/$ISID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-am2' -d '{"expected_revision":6,"from_index":0,"local_time":"18:00"}')" 201
chmod 600 "$ART/ifin.out"
pycheck i-exclusions - "$ART/ifin.out" "$IR0" "$IR1" "$IR2" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
r0, r1, r2 = sys.argv[2], sys.argv[3], sys.argv[4]
assert d["revision"] == 7, d["revision"]
occ = {o["reference"]: o for o in d["occurrences"]}
assert occ[r0]["reservation"]["starts_at_local"] == "2027-05-06T18:00", occ[r0]
assert occ[r0]["exception"] is False
assert occ[r1]["reservation"]["starts_at_local"] == "2027-05-13T19:30", occ[r1]
assert occ[r1]["reservation"]["status"] == "cancelled", occ[r1]
assert occ[r2]["exception"] is True, occ[r2]
assert occ[r2]["reservation"]["status"] == "cancelled", occ[r2]
print("EXCL-OK")
PYEOF
$CURL -o "$ART/iexc.out" -G "$BASE/reservations/$IR1/history" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/iexc.out"
pycheck i-excluded-frozen - "$ART/iexc.out" <<'PYEOF'
import json,sys
es = json.load(open(sys.argv[1]))["entries"]
assert [e["event"] for e in es] == ["created","reassigned","changed","cancelled"], es
assert es[-1]["changes"] == [] and es[2]["changes"][0]["to"] == "2027-05-13T19:30", es
print("EXCFROZEN-OK")
PYEOF
export_ok "$ART/ifin-post.json" && { echo "PASS: i-final-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i-final-delta - "$ART/ifin-pre.json" "$ART/ifin-post.json" "$ART/ifin.out" "$IR0" "$IR1" "$IR2" "$ISID" <<'PYEOF'
import json, sys, os, subprocess
pref, postf, resp_f = sys.argv[1], sys.argv[2], sys.argv[3]
r0, r1, r2, sid = sys.argv[4], sys.argv[5], sys.argv[6], sys.argv[7]
pre = json.load(open(pref))["state"]
assert pre["series"][sid]["revision"] == 6, pre["series"][sid]["revision"]
spec = {"series": sid, "rev_before": 6, "owner": "u1",
        "path": "/series/%s/amend" % sid, "key": "i-am2", "clock": "18:00",
        "body": '{"expected_revision":6,"from_index":0,"local_time":"18:00"}',
        "from_index": 0,
        "changed": {r0: {"date": "2027-05-06", "tables": ["t_2"], "terms": None}},
        "unmoved": sorted([r1, r2]),
        "counter_rest": "r", "absent_keys": []}
spec_f = os.path.join(os.path.dirname(os.path.abspath(pref)), "ifinspec.json")
json.dump(spec, open(spec_f, "w"))
checker = os.path.join(os.path.dirname(os.path.abspath(pref)), "amend_delta.py")
r = subprocess.run([sys.executable, checker, pref, postf, resp_f, spec_f],
                   capture_output=True, text=True)
sys.stdout.write(r.stdout)
sys.stderr.write(r.stderr)
assert r.returncode == 0, "shared amend checker failed"
print("IFINDELTA-OK")
PYEOF
pycheck i-corrupt-guards - "$ART/ipre.json" "$ART/ipost.json" "$ART/iam.out" "$ART/ispec.json" <<'PYEOF'
import json, sys, copy, subprocess, os
pref, postf, respf, specf = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
pre = json.load(open(pref))["state"]
post = json.load(open(postf))["state"]
spec = json.load(open(specf))
prog = os.path.join(os.path.dirname(os.path.abspath(pref)), "amend_delta.py")
changed0 = sorted(spec["changed"])[0]
def run(obj, tag):
    pj = pref + ".mut-%s.json" % tag
    json.dump({"track": "tablekeeper", "format_version": 1, "state": obj}, open(pj, "w"))
    r = subprocess.run([sys.executable, prog, pref, pj, respf, specf],
                       capture_output=True, text=True)
    print("%s exit=%d" % (tag, r.returncode))
    return r.returncode
results = {"genuine": run(post, "genuine")}
cases = []
o = copy.deepcopy(post)
o["reservations"][changed0]["table_ids"] = ["t_9"]
o["reservations"][changed0]["table_id"] = "t_9"
cases.append(("forged-selector", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["table_id"] = "t_9"
cases.append(("forged-scalar", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["user_id"] = "unknown-owner"
cases.append(("forged-owner", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["created_at"] = "1999-01-01T00:00:00Z"
cases.append(("forged-created", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["changes"][0]["from"] = "1999-01-01T00:00"
cases.append(("forged-from", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["seq"] = 99
cases.append(("forged-seq", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["at"] = "not-a-real-instant"
cases.append(("forged-at", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["accepted_terms"]["slot_minutes"] = 45
cases.append(("forged-terms", o))
o = copy.deepcopy(post)
o["plans"]["unexpected-plan"] = copy.deepcopy(next(iter(o["plans"].values())))
cases.append(("extra-plan", o))
o = copy.deepcopy(post)
o["series"]["unexpected-series"] = copy.deepcopy(o["series"][spec["series"]])
cases.append(("extra-series", o))
o = copy.deepcopy(post)
o["histories"]["unexpected-ref"] = copy.deepcopy(o["histories"][changed0])
cases.append(("extra-orphan-history", o))
o = copy.deepcopy(post)
for k, v in o["receipts"].items():
    if k not in pre["receipts"]:
        v["user_id"] = "wrong-owner"
cases.append(("forged-receipt-owner", o))
o = copy.deepcopy(post)
for k, v in o["receipts"].items():
    if k not in pre["receipts"]:
        v["body"] = '{"expected_revision":2,"from_index":0,"local_time":"00:00"}'
cases.append(("forged-receipt-body", o))
o = copy.deepcopy(post)
o["restaurant_revisions"]["r"] = pre["restaurant_revisions"]["r"] + 9
cases.append(("forged-counter", o))
for name, obj in cases:
    assert json.dumps(obj, sort_keys=True) != json.dumps(post, sort_keys=True), "mutation did not apply: " + name
    results[name] = run(obj, name)
    assert results[name] != 0, name
assert results["genuine"] == 0, results
print("ISELFTEST-OK")
PYEOF
VP2='{"effective_from":"2027-06-01","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}'
expect i-publish2 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-pol2' -d "$VP2")" 201
export_ok "$ART/irepre.json" && { echo "PASS: i-export-repre (200)"; PASS=$((PASS + 1)); }
expect i-amend-replay "$($CURL -o "$ART/irepr.out" -w '%{http_code}' -X POST "$BASE/series/$ISID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-am1' -d '{"expected_revision":2,"from_index":0,"local_time":"19:30"}')" 200
chmod 600 "$ART/irepr.out"
if cmp -s "$ART/iam.out" "$ART/irepr.out"; then echo "PASS: i-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: i-replay-bytes"; FAIL=$((FAIL + 1)); fi
pycheck i-replay-no-newline - "$ART/iam.out" <<'PYEOF'
import sys
raw = open(sys.argv[1], "rb").read()
assert not raw.endswith(b"\n"), "trailing newline"
print("NONEWL-OK")
PYEOF
export_ok "$ART/irepost.json" && { echo "PASS: i-export-repost (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/irepre.json" "$ART/irepost.json"; then echo "PASS: i-replay-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: i-replay-atomic"; FAIL=$((FAIL + 1)); fi
expect i-replay-changed "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$ISID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i-am1' -d '{"expected_revision":2,"from_index":0,"local_time":"19:30","z":0}')" 409
export_ok "$ART/irepost2.json" && { echo "PASS: i-export-repost2 (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/irepost.json" "$ART/irepost2.json"; then echo "PASS: i-reuse-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: i-reuse-atomic"; FAIL=$((FAIL + 1)); fi
pycheck i-adopt-receipt - "$ART/irepost2.json" "$ART/iad.out" <<'PYEOF'
import json,sys
st = json.load(open(sys.argv[1]))["state"]
orig = open(sys.argv[2]).read()
hits = [v for v in st["receipts"].values() if v["method"] == "POST" and v["path"] == "/series" and v["key"] == "i-ad"]
assert len(hits) == 1 and hits[0]["response"] == orig and hits[0]["status"] == 201, hits
print("ADOPTREC-OK")
PYEOF
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect i2-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: i2-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect i2-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T19:30","party_size":1}')" 201
$CURL -o "$ART/i2a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T19:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/i2a.out"
I2A=$(py -c 'import json; print(json.load(open("'"$ART"'/i2a.out"))["reference"])' 2>>"$DIAG")
expect i2-adopt "$($CURL -o "$ART/i2s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-s' -d '{"anchor_reference":"'"$I2A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/i2s.out"
I2SID=$(py -c 'import json; print(json.load(open("'"$ART"'/i2s.out"))["series_id"])' 2>>"$DIAG")
export_ok "$ART/i2pre.json" && { echo "PASS: i2-export-pre (200)"; PASS=$((PASS + 1)); }
expect i2-noop "$($CURL -o "$ART/i2n.out" -w '%{http_code}' -X POST "$BASE/series/$I2SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-n' -d '{"expected_revision":1,"from_index":0,"local_time":"19:30"}')" 201
chmod 600 "$ART/i2n.out"
export_ok "$ART/i2post.json" && { echo "PASS: i2-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i2-noop-delta - "$ART/i2pre.json" "$ART/i2post.json" "$ART/i2n.out" "$I2SID" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
body = json.load(open(sys.argv[3]))
sid = sys.argv[4]
assert isinstance(body["occurrences"], list) and len(body["occurrences"]) == 2, body
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "one noop receipt"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]][0]
assert (new_rc["user_id"],new_rc["method"],new_rc["status"]) == ("u1","POST",201), new_rc
assert new_rc["path"] == "/series/%s/amend" % sid and new_rc["key"] == "i2-n", new_rc
assert new_rc["body"] == '{"expected_revision":1,"from_index":0,"local_time":"19:30"}', new_rc["body"]
assert new_rc["response"] == open(sys.argv[3]).read(), "receipt != raw noop bytes"
for k, v in pre.items():
    if k == "receipts":
        continue
    assert post[k] == v, "noop changed %s" % k
print("NOOP-OK")
PYEOF
V60='{"effective_from":"2027-01-01","slot_minutes":60,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2}}'
expect i2-gridpub "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-p' -d "$V60")" 201
export_ok "$ART/i2gpre.json" && { echo "PASS: i2g-export-pre (200)"; PASS=$((PASS + 1)); }
expect i2-offgrid-noop "$($CURL -o "$ART/i2g.out" -w '%{http_code}' -X POST "$BASE/series/$I2SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-g' -d '{"expected_revision":1,"from_index":0,"local_time":"19:30"}')" 201
chmod 600 "$ART/i2g.out"
export_ok "$ART/i2gpost.json" && { echo "PASS: i2g-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i2-offgrid-delta - "$ART/i2gpre.json" "$ART/i2gpost.json" "$ART/i2g.out" "$I2SID" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
sid = sys.argv[4]
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "one offgrid receipt"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]][0]
assert (new_rc["user_id"],new_rc["method"],new_rc["status"]) == ("u1","POST",201), new_rc
assert new_rc["path"] == "/series/%s/amend" % sid and new_rc["key"] == "i2-g", new_rc
assert new_rc["body"] == '{"expected_revision":1,"from_index":0,"local_time":"19:30"}', new_rc["body"]
assert new_rc["response"] == open(sys.argv[3]).read(), "receipt != raw offgrid bytes"
for k, v in pre.items():
    if k == "receipts":
        continue
    assert post[k] == v, "offgrid changed %s" % k
print("OFFGRID-OK")
PYEOF
expect i2-empty-cancel1 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/$I2A/cancel" -H "Authorization: Bearer $TOK_1")" 200
I2G=$(py -c 'import json; print(json.load(open("'"$ART"'/i2s.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
expect i2-empty-cancel2 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/$I2G/cancel" -H "Authorization: Bearer $TOK_1")" 200
export_ok "$ART/i2epre.json" && { echo "PASS: i2e-export-pre (200)"; PASS=$((PASS + 1)); }
expect i2-empty "$($CURL -o "$ART/i2e.out" -w '%{http_code}' -X POST "$BASE/series/$I2SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i2-e' -d '{"expected_revision":3,"from_index":0,"local_time":"20:00"}')" 201
chmod 600 "$ART/i2e.out"
export_ok "$ART/i2epost.json" && { echo "PASS: i2e-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i2-empty-delta - "$ART/i2epre.json" "$ART/i2epost.json" "$ART/i2e.out" "$I2SID" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
sid = sys.argv[4]
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "one empty receipt"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]][0]
assert (new_rc["user_id"],new_rc["method"],new_rc["status"]) == ("u1","POST",201), new_rc
assert new_rc["path"] == "/series/%s/amend" % sid and new_rc["key"] == "i2-e", new_rc
assert new_rc["body"] == '{"expected_revision":3,"from_index":0,"local_time":"20:00"}', new_rc["body"]
assert new_rc["response"] == open(sys.argv[3]).read(), "receipt != raw empty bytes"
for k, v in pre.items():
    if k == "receipts":
        continue
    assert post[k] == v, "empty changed %s" % k
print("EMPTY-OK")
PYEOF

TOMORROW=$(py -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=24)).strftime("%Y-%m-%d"))' 2>>"$DIAG")
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect i3-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: i3-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect i3-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i3-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$TOMORROW"'T19:00","party_size":1}')" 201
$CURL -o "$ART/i3a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i3-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$TOMORROW"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/i3a.out"
I3A=$(py -c 'import json; print(json.load(open("'"$ART"'/i3a.out"))["reference"])' 2>>"$DIAG")
expect i3-adopt "$($CURL -o "$ART/i3s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i3-s' -d '{"anchor_reference":"'"$I3A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/i3s.out"
I3SID=$(py -c 'import json; print(json.load(open("'"$ART"'/i3s.out"))["series_id"])' 2>>"$DIAG")
VPB='{"effective_from":"2026-10-04","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":10080,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],"capacities":{"t_1":2}}'
expect i3-publish "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i3-p' -d "$VPB")" 201
export_ok "$ART/i3pre.json" && { echo "PASS: i3-export-pre (200)"; PASS=$((PASS + 1)); }
expect i3-cutoff-bypass "$($CURL -o "$ART/i3n.out" -w '%{http_code}' -X POST "$BASE/series/$I3SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: i3-n' -d '{"expected_revision":1,"from_index":0,"local_time":"19:00"}')" 201
chmod 600 "$ART/i3n.out"
export_ok "$ART/i3post.json" && { echo "PASS: i3-export-post (200)"; PASS=$((PASS + 1)); }
pycheck i3-bypass-delta - "$ART/i3pre.json" "$ART/i3post.json" "$ART/i3n.out" "$I3SID" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
sid = sys.argv[4]
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "one bypass receipt"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]][0]
assert (new_rc["user_id"],new_rc["method"],new_rc["status"]) == ("u1","POST",201), new_rc
assert new_rc["path"] == "/series/%s/amend" % sid and new_rc["key"] == "i3-n", new_rc
assert new_rc["body"] == '{"expected_revision":1,"from_index":0,"local_time":"19:00"}', new_rc["body"]
assert new_rc["response"] == open(sys.argv[3]).read(), "receipt != raw bypass bytes"
for k, v in pre.items():
    if k == "receipts":
        continue
    assert post[k] == v, "bypass changed %s" % k
print("BYPASS-OK")
PYEOF

echo "== J old-cutoff governance with dynamic UTC dates =="
JNOW=$(py -c 'import datetime; print(datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M"))' 2>>"$DIAG")
JTOMORROW=$(py -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=24)).strftime("%Y-%m-%d"))' 2>>"$DIAG")
JTODAY=$(py -c 'import datetime; print(datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d"))' 2>>"$DIAG")
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect j-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: j-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect j-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$JTOMORROW"'T19:00","party_size":1}')" 201
$CURL -o "$ART/janch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$JTOMORROW"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/janch.out"
JAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/janch.out"))["reference"])' 2>>"$DIAG")
expect j-adopt "$($CURL -o "$ART/jad.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-ad' -d '{"anchor_reference":"'"$JAREF"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/jad.out"
JSID=$(py -c 'import json; print(json.load(open("'"$ART"'/jad.out"))["series_id"])' 2>>"$DIAG")
JGEN=$(py -c 'import json; print(json.load(open("'"$ART"'/jad.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
JVP1='{"effective_from":"'"$JTODAY"'","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":10080,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],"capacities":{"t_1":2}}'
expect j-publish "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-pol' -d "$JVP1")" 201
expect j-amend "$($CURL -o "$ART/jam.out" -w '%{http_code}' -X POST "$BASE/series/$JSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-am1' -d '{"expected_revision":1,"from_index":0,"local_time":"19:30"}')" 201
chmod 600 "$ART/jam.out"
pycheck j-adopted-cutoff - "$ART/jam.out" "$JAREF" "$JNOW" <<'PYEOF'
import json,sys,datetime
d = json.load(open(sys.argv[1]))
anchor = sys.argv[2]
now = datetime.datetime.strptime(sys.argv[3], "%Y-%m-%dT%H:%M").replace(tzinfo=datetime.timezone.utc)
assert d["revision"] == 2, d["revision"]
occ = {o["reference"]: o for o in d["occurrences"]}
a = occ[anchor]["reservation"]
assert a["starts_at_local"].endswith("T19:30") and a["revision"] == 2, a
assert a["accepted_terms"]["cancellation_cutoff_minutes"] == 10080, a["accepted_terms"]
start = datetime.datetime.fromisoformat(a["starts_at"])
assert (start - now).total_seconds() / 60 <= 10080, "anchor not inside adopted cutoff window"
print("JCUTOFF-OK")
PYEOF
JVP2='{"effective_from":"'"$JTODAY"'","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],"capacities":{"t_1":2}}'
expect j-publish2 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-pol2' -d "$JVP2")" 201
export_ok "$ART/jpre.json" && { echo "PASS: j-export-pre (200)"; PASS=$((PASS + 1)); }
expect j-cutoff-refused "$($CURL -o "$ART/jrej.out" -w '%{http_code}' -X POST "$BASE/series/$JSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-am2' -d '{"expected_revision":2,"from_index":0,"local_time":"20:00"}')" 409
chmod 600 "$ART/jrej.out"
pycheck j-cutoff-code - "$ART/jrej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "cutoff_passed"
print("JREJ-OK")
PYEOF
export_ok "$ART/jpost.json" && { echo "PASS: j-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/jpre.json" "$ART/jpost.json"; then echo "PASS: j-cutoff-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: j-cutoff-atomic"; FAIL=$((FAIL + 1)); fi
expect j-huge-stale "$($CURL -o "$ART/jhuge.out" -w '%{http_code}' -X POST "$BASE/series/$JSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-huge' -d '{"expected_revision":1000000000000000000,"from_index":0,"local_time":"20:00"}')" 409
chmod 600 "$ART/jhuge.out"
pycheck j-huge-code - "$ART/jhuge.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "stale_revision"
print("JHUGE-OK")
PYEOF
export_ok "$ART/jsame-pre.json" && { echo "PASS: j-same-export-pre (200)"; PASS=$((PASS + 1)); }
expect j-same-clock "$($CURL -o "$ART/jsame.out" -w '%{http_code}' -X POST "$BASE/series/$JSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: j-same' -d '{"expected_revision":2,"from_index":0,"local_time":"19:30"}')" 201
chmod 600 "$ART/jsame.out"
export_ok "$ART/jsame-post.json" && { echo "PASS: j-same-export-post (200)"; PASS=$((PASS + 1)); }
pycheck j-same-delta - "$ART/jsame-pre.json" "$ART/jsame-post.json" "$ART/jsame.out" "$JSID" <<'PYEOF'
import json,sys
pre = json.load(open(sys.argv[1]))["state"]
post = json.load(open(sys.argv[2]))["state"]
sid = sys.argv[4]
assert len(post["receipts"]) == len(pre["receipts"]) + 1, "one bypass receipt"
for k, v in pre["receipts"].items():
    assert post["receipts"][k] == v, "prior receipt changed"
new_rc = [v for k, v in post["receipts"].items() if k not in pre["receipts"]]
assert len(new_rc) == 1
rc = new_rc[0]
assert (rc["user_id"],rc["method"],rc["status"]) == ("u1","POST",201), rc
assert rc["path"] == "/series/%s/amend" % sid and rc["key"] == "j-same", rc
assert rc["body"] == '{"expected_revision":2,"from_index":0,"local_time":"19:30"}', rc["body"]
assert rc["response"] == open(sys.argv[3]).read(), "receipt != raw bypass bytes"
for k, v in pre.items():
    if k == "receipts":
        continue
    assert post[k] == v, "bypass changed %s" % k
print("JSAME-OK")
PYEOF

echo "== K atomicity, precedence, calendar =="
KDATE=$(py -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=24)).strftime("%Y-%m-%d"))' 2>>"$DIAG")
KGENDATE=$(py -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=8)).strftime("%Y-%m-%d"))' 2>>"$DIAG")
KGENWD=$(py -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=8)).strftime("%a").lower())' 2>>"$DIAG")
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":0,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}')" 201
$CURL -o "$ART/kanch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/kanch.out"
KAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/kanch.out"))["reference"])' 2>>"$DIAG")
expect k-adopt "$($CURL -o "$ART/kad.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-ad' -d '{"anchor_reference":"'"$KAREF"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/kad.out"
KSID=$(py -c 'import json; print(json.load(open("'"$ART"'/kad.out"))["series_id"])' 2>>"$DIAG")
KPOL=$(py - "$KGENWD" 2>>"$DIAG" <<'PYEOF'
import json, sys
days = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
days.remove(sys.argv[1])
print(",".join(json.dumps({"weekday": d, "opens": "00:00", "closes": "23:00"}) for d in days))
PYEOF
)
KVP1='{"effective_from":"'"$KDATE"'","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":10080,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}'
expect k-adopt-cutoff "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-pol0' -d "$KVP1")" 201
expect k-adopt10080 "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$KSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-am0' -d '{"expected_revision":1,"from_index":0,"local_time":"19:30"}')" 201
expect k-closeday-pub "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-pol' -d '{"effective_from":"'"$KGENDATE"'","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":10080,"opening_hours":['"$KPOL"'],"capacities":{"t_1":2,"t_2":4}}')" 201
export_ok "$ART/kpre.json" && { echo "PASS: k-export-pre (200)"; PASS=$((PASS + 1)); }
expect k-index0-cutoff "$($CURL -o "$ART/krej.out" -w '%{http_code}' -X POST "$BASE/series/$KSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-r1' -d '{"expected_revision":2,"from_index":0,"local_time":"20:00"}')" 409
chmod 600 "$ART/krej.out"
pycheck k-index0-code - "$ART/krej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "cutoff_passed"
print("K0-OK")
PYEOF
expect k-index1-closed "$($CURL -o "$ART/krej2.out" -w '%{http_code}' -X POST "$BASE/series/$KSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k-r2' -d '{"expected_revision":2,"from_index":1,"local_time":"20:00"}')" 422
chmod 600 "$ART/krej2.out"
pycheck k-index1-code - "$ART/krej2.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "outside_opening_hours"
print("K1-OK")
PYEOF
export_ok "$ART/kpost.json" && { echo "PASS: k-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/kpre.json" "$ART/kpost.json"; then echo "PASS: k-precedence-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k-precedence-atomic"; FAIL=$((FAIL + 1)); fi
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k2-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k2-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k2-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}')" 201
$CURL -o "$ART/k2a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k2a.out"
K2A=$(py -c 'import json; print(json.load(open("'"$ART"'/k2a.out"))["reference"])' 2>>"$DIAG")
expect k2-adopt "$($CURL -o "$ART/k2s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-s' -d '{"anchor_reference":"'"$K2A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/k2s.out"
K2SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k2s.out"))["series_id"])' 2>>"$DIAG")
expect k2-closeday-pub "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/policies" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-p' -d '{"effective_from":"'"$KGENDATE"'","slot_minutes":30,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":['"$KPOL"'],"capacities":{"t_1":2,"t_2":4}}')" 201
expect k2-zero-preview "$($CURL -o "$ART/k2pv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-pv' -d '{"table_id":"t_1","from":"'"$KDATE"'T20:00:00+00:00","to":"'"$KDATE"'T20:30:00+00:00"}')" 201
chmod 600 "$ART/k2pv.out"
K2PID=$(py -c 'import json; print(json.load(open("'"$ART"'/k2pv.out"))["plan_id"])' 2>>"$DIAG")
expect k2-zero-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$K2PID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-ap' -d '{}')" 201
export_ok "$ART/k2pre.json" && { echo "PASS: k2-export-pre (200)"; PASS=$((PASS + 1)); }
expect k2-occ-after-nonocc "$($CURL -o "$ART/k2rej.out" -w '%{http_code}' -X POST "$BASE/series/$K2SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k2-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"19:30"}')" 422
chmod 600 "$ART/k2rej.out"
pycheck k2-occ-code - "$ART/k2rej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "outside_opening_hours"
print("K2-OK")
PYEOF
export_ok "$ART/k2post.json" && { echo "PASS: k2-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/k2pre.json" "$ART/k2post.json"; then echo "PASS: k2-precedence-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k2-precedence-atomic"; FAIL=$((FAIL + 1)); fi
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k3-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k3-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k3-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}')" 201
$CURL -o "$ART/k3a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k3a.out"
K3A=$(py -c 'import json; print(json.load(open("'"$ART"'/k3a.out"))["reference"])' 2>>"$DIAG")
expect k3-adopt "$($CURL -o "$ART/k3s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-s' -d '{"anchor_reference":"'"$K3A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/k3s.out"
K3SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k3s.out"))["series_id"])' 2>>"$DIAG")
expect k3-blocker "$($CURL -o "$ART/k3b.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-b' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T21:30","party_size":1}')" 201
chmod 600 "$ART/k3b.out"
K3B=$(py -c 'import json; print(json.load(open("'"$ART"'/k3b.out"))["reference"])' 2>>"$DIAG")
export_ok "$ART/k3pre.json" && { echo "PASS: k3-export-pre (200)"; PASS=$((PASS + 1)); }
expect k3-occupancy "$($CURL -o "$ART/k3rej.out" -w '%{http_code}' -X POST "$BASE/series/$K3SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"21:30"}')" 409
chmod 600 "$ART/k3rej.out"
pycheck k3-occ-code - "$ART/k3rej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "table_unavailable"
print("K3OCC-OK")
PYEOF
export_ok "$ART/k3post.json" && { echo "PASS: k3-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/k3pre.json" "$ART/k3post.json"; then echo "PASS: k3-rollback-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k3-rollback-atomic"; FAIL=$((FAIL + 1)); fi
expect k3-cancel-blocker "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations/$K3B/cancel" -H "Authorization: Bearer $TOK_1")" 200
expect k3-reuse "$($CURL -o "$ART/k3ok.out" -w '%{http_code}' -X POST "$BASE/series/$K3SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"21:30"}')" 201
chmod 600 "$ART/k3ok.out"
expect k3-reuse-replay "$($CURL -o "$ART/k3ok2.out" -w '%{http_code}' -X POST "$BASE/series/$K3SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k3-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"21:30"}')" 200
chmod 600 "$ART/k3ok2.out"
if cmp -s "$ART/k3ok.out" "$ART/k3ok2.out"; then echo "PASS: k3-replay-bytes (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k3-replay-bytes"; FAIL=$((FAIL + 1)); fi
FOUNDCODE=$(reset_world <<EOF
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"UTC","slot_minutes":30,
  "reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,
  "opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:00"},{"weekday":"tue","opens":"00:00","closes":"23:00"},{"weekday":"wed","opens":"00:00","closes":"23:00"},{"weekday":"thu","opens":"00:00","closes":"23:00"},{"weekday":"fri","opens":"00:00","closes":"23:00"},{"weekday":"sat","opens":"00:00","closes":"23:00"},{"weekday":"sun","opens":"00:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k4-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k4-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k4-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}')" 201
$CURL -o "$ART/k4a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"'"$KDATE"'T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k4a.out"
K4A=$(py -c 'import json; print(json.load(open("'"$ART"'/k4a.out"))["reference"])' 2>>"$DIAG")
expect k4-adopt "$($CURL -o "$ART/k4s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-s' -d '{"anchor_reference":"'"$K4A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/k4s.out"
K4SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k4s.out"))["series_id"])' 2>>"$DIAG")
expect k4-zero-preview "$($CURL -o "$ART/k4pv.out" -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-pv' -d '{"table_id":"t_1","from":"'"$KDATE"'T20:00:00+00:00","to":"'"$KDATE"'T20:30:00+00:00"}')" 201
chmod 600 "$ART/k4pv.out"
K4PID=$(py -c 'import json; print(json.load(open("'"$ART"'/k4pv.out"))["plan_id"])' 2>>"$DIAG")
expect k4-zero-apply "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans/$K4PID/apply" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-ap' -d '{}')" 201
export_ok "$ART/k4pre.json" && { echo "PASS: k4-export-pre (200)"; PASS=$((PASS + 1)); }
expect k4-closure-conflict "$($CURL -o "$ART/k4rej.out" -w '%{http_code}' -X POST "$BASE/series/$K4SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"20:00"}')" 409
chmod 600 "$ART/k4rej.out"
pycheck k4-conflict-code - "$ART/k4rej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "table_unavailable"
print("K4C-OK")
PYEOF
export_ok "$ART/k4post.json" && { echo "PASS: k4-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/k4pre.json" "$ART/k4post.json"; then echo "PASS: k4-conflict-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k4-conflict-atomic"; FAIL=$((FAIL + 1)); fi
expect k4-adjacent-retry "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/series/$K4SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k4-r1' -d '{"expected_revision":1,"from_index":0,"local_time":"20:30"}')" 201
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"sun","opens":"00:00","closes":"04:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k5-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k5-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k5-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k5-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-03-21T01:30","party_size":1}')" 201
$CURL -o "$ART/k5a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k5-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-03-21T01:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k5a.out"
K5A=$(py -c 'import json; print(json.load(open("'"$ART"'/k5a.out"))["reference"])' 2>>"$DIAG")
expect k5-adopt "$($CURL -o "$ART/k5s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k5-s' -d '{"anchor_reference":"'"$K5A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/k5s.out"
K5SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k5s.out"))["series_id"])' 2>>"$DIAG")
export_ok "$ART/k5pre.json" && { echo "PASS: k5-export-pre (200)"; PASS=$((PASS + 1)); }
expect k5-gap-invalid "$($CURL -o "$ART/k5rej.out" -w '%{http_code}' -X POST "$BASE/series/$K5SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k5-r1' -d '{"expected_revision":1,"from_index":1,"local_time":"02:30"}')" 422
chmod 600 "$ART/k5rej.out"
pycheck k5-gap-code - "$ART/k5rej.out" <<'PYEOF'
import json,sys
assert json.load(open(sys.argv[1]))["error"]["code"] == "invalid_local_time"
print("KGAP-OK")
PYEOF
export_ok "$ART/k5post.json" && { echo "PASS: k5-export-post (200)"; PASS=$((PASS + 1)); }
if cmp -s "$ART/k5pre.json" "$ART/k5post.json"; then echo "PASS: k5-gap-atomic (identical)"; PASS=$((PASS + 1)); else echo "FAIL: k5-gap-atomic"; FAIL=$((FAIL + 1)); fi
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"America/New_York","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"sun","opens":"00:00","closes":"05:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2}],
  "manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k6-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k6-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k6-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k6-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-10-31T03:30","party_size":1}')" 201
$CURL -o "$ART/k6a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k6-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-10-31T03:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k6a.out"
K6A=$(py -c 'import json; print(json.load(open("'"$ART"'/k6a.out"))["reference"])' 2>>"$DIAG")
expect k6-adopt "$($CURL -o "$ART/k6s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k6-s' -d '{"anchor_reference":"'"$K6A"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/k6s.out"
K6SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k6s.out"))["series_id"])' 2>>"$DIAG")
K6G=$(py -c 'import json; print(json.load(open("'"$ART"'/k6s.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
expect k6-fold-amend "$($CURL -o "$ART/k6am.out" -w '%{http_code}' -X POST "$BASE/series/$K6SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k6-r1' -d '{"expected_revision":1,"from_index":1,"local_time":"01:30"}')" 201
chmod 600 "$ART/k6am.out"
pycheck k6-fold-shape - "$ART/k6am.out" "$K6G" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
gen = sys.argv[2]
assert d["revision"] == 2, d["revision"]
occ = {o["reference"]: o for o in d["occurrences"]}
assert occ[gen]["exception"] is False
r = occ[gen]["reservation"]
assert r["starts_at_local"] == "2027-11-07T01:30", r["starts_at_local"]
assert r["starts_at"] == "2027-11-07T01:30:00-04:00", r["starts_at"]
assert r["ends_at"] == "2027-11-07T02:00:00-05:00", r["ends_at"]
assert r["revision"] == 2, r["revision"]
print("KFOLD-OK")
PYEOF
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect k7-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: k7-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect k7-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k7-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":1}')" 201
$CURL -o "$ART/k7a.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k7-a' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-06-17T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/k7a.out"
K7A=$(py -c 'import json; print(json.load(open("'"$ART"'/k7a.out"))["reference"])' 2>>"$DIAG")
expect k7-adopt12 "$($CURL -o "$ART/k7s.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k7-s' -d '{"anchor_reference":"'"$K7A"'","count":12,"interval_weeks":1}')" 201
chmod 600 "$ART/k7s.out"
K7SID=$(py -c 'import json; print(json.load(open("'"$ART"'/k7s.out"))["series_id"])' 2>>"$DIAG")
K7LAST=$(py -c 'import json; print(json.load(open("'"$ART"'/k7s.out"))["occurrences"][11]["reservation"]["reference"])' 2>>"$DIAG")
export_ok "$ART/k7apre.json" && { echo "PASS: k7a-export-pre (200)"; PASS=$((PASS + 1)); }
expect k7-last-only "$($CURL -o "$ART/k7am.out" -w '%{http_code}' -X POST "$BASE/series/$K7SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k7-r1' -d '{"expected_revision":1,"from_index":11,"local_time":"20:30"}')" 201
chmod 600 "$ART/k7am.out"
export_ok "$ART/k7apost.json" && { echo "PASS: k7a-export-post (200)"; PASS=$((PASS + 1)); }
pycheck k7-last-shape - "$ART/k7am.out" "$ART/k7s.out" "$K7LAST" "$ART/k7apre.json" "$ART/k7apost.json" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
orig = json.load(open(sys.argv[2]))
last = sys.argv[3]
pre = json.load(open(sys.argv[4]))["state"]
post = json.load(open(sys.argv[5]))["state"]
assert d["revision"] == 2, d["revision"]
assert len(d["occurrences"]) == 12, len(d["occurrences"])
for o in d["occurrences"]:
    if o["reference"] == last:
        assert o["reservation"]["starts_at_local"].endswith("T20:30"), o
        assert o["reservation"]["revision"] == 2, o
    else:
        oo = [x for x in orig["occurrences"] if x["reference"] == o["reference"]][0]
        assert o["reservation"] == oo["reservation"], o["reference"]
        assert pre["histories"][o["reference"]] == post["histories"][o["reference"]], o["reference"]
print("K7LAST-OK")
PYEOF
export_ok "$ART/k7pre.json" && { echo "PASS: k7-export-pre (200)"; PASS=$((PASS + 1)); }
expect k7-anchor-too "$($CURL -o "$ART/k7am2.out" -w '%{http_code}' -X POST "$BASE/series/$K7SID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: k7-r2' -d '{"expected_revision":2,"from_index":0,"local_time":"21:00"}')" 201
chmod 600 "$ART/k7am2.out"
export_ok "$ART/k7post.json" && { echo "PASS: k7-export-post (200)"; PASS=$((PASS + 1)); }
pycheck k7-anchor-changed - "$ART/k7am2.out" "$K7A" <<'PYEOF'
import json,sys
d = json.load(open(sys.argv[1]))
assert d["revision"] == 3, d["revision"]
occ = {o["reference"]: o for o in d["occurrences"]}
assert occ[sys.argv[2]]["reservation"]["starts_at_local"] == "2027-06-17T21:00", occ[sys.argv[2]]
print("K7ANCHOR-OK")
PYEOF

echo "== M two-key same-revision race =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect m-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: m-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect m-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}')" 201
$CURL -o "$ART/manch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/manch.out"
MAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/manch.out"))["reference"])' 2>>"$DIAG")
expect m-adopt "$($CURL -o "$ART/mad.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ad' -d '{"anchor_reference":"'"$MAREF"'","count":3,"interval_weeks":1}')" 201
chmod 600 "$ART/mad.out"
MSID=$(py -c 'import json; print(json.load(open("'"$ART"'/mad.out"))["series_id"])' 2>>"$DIAG")
expect m-unrelated-booking "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ub' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T21:30","party_size":1}')" 201
$CURL -o "$ART/mub.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ub' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T21:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/mub.out"
MUB=$(py -c 'import json; print(json.load(open("'"$ART"'/mub.out"))["reference"])' 2>>"$DIAG")
expect m-unrelated-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ua' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-20T19:00","party_size":1}')" 201
$CURL -o "$ART/mua.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ua' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-20T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/mua.out"
MUA=$(py -c 'import json; print(json.load(open("'"$ART"'/mua.out"))["reference"])' 2>>"$DIAG")
expect m-unrelated-adopt "$($CURL -o "$ART/mus.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-us' -d '{"anchor_reference":"'"$MUA"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/mus.out"
MUSID=$(py -c 'import json; print(json.load(open("'"$ART"'/mus.out"))["series_id"])' 2>>"$DIAG")
expect m-unrelated-preview "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-upv' -d '{"table_id":"t_2","from":"2027-05-06T18:00:00+02:00","to":"2027-05-06T23:00:00+02:00"}')" 201
export_ok "$ART/mpre.json" && { echo "PASS: m-export-pre (200)"; PASS=$((PASS + 1)); }
MRA='{"expected_revision":1,"from_index":0,"local_time":"20:00"}'
MRB='{"expected_revision":1,"from_index":0,"local_time":"21:00"}'
$CURL -o "$ART/mra.out" -w '%{http_code}' -X POST "$BASE/series/$MSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-ra' -d "$MRA" > "$SCR/mra.code" 2>>"$DIAG" &
$CURL -o "$ART/mrb.out" -w '%{http_code}' -X POST "$BASE/series/$MSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: m-rb' -d "$MRB" > "$SCR/mrb.code" 2>>"$DIAG" &
wait
chmod 600 "$ART/mra.out" "$ART/mrb.out"
CRA=$(cat "$SCR/mra.code"); CRB=$(cat "$SCR/mrb.code")
export_ok "$ART/mpost.json" && { echo "PASS: m-export-post (200)"; PASS=$((PASS + 1)); }
pycheck m-race-outcome - "$ART/mra.out" "$ART/mrb.out" "$CRA" "$CRB" "$ART/mpre.json" "$ART/mpost.json" "$MSID" "$MUSID" "$MUB" <<'PYEOF'
import json, sys, os, subprocess
ra, rb = sys.argv[1], sys.argv[2]
ca, cb = sys.argv[3], sys.argv[4]
pre_f, post_f = sys.argv[5], sys.argv[6]
sid, usid, ub = sys.argv[7], sys.argv[8], sys.argv[9]
pre = json.load(open(pre_f))["state"]
codes = sorted([(ca, "m-ra", "20:00", ra), (cb, "m-rb", "21:00", rb)])
assert [c[0] for c in codes] == ["201", "409"], codes
win = [c for c in codes if c[0] == "201"][0]
lose = [c for c in codes if c[0] == "409"][0]
assert json.load(open(lose[3]))["error"]["code"] == "stale_revision", lose
wbody = json.load(open(win[3]))
clocks = set(o["reservation"]["starts_at_local"].rsplit("T", 1)[1] for o in wbody["occurrences"])
assert len(clocks) == 1, clocks
wclock = clocks.pop()
assert wclock in ("20:00", "21:00"), wclock
rev0 = pre["series"][sid]["revision"]
members = {m["reference"]: m for m in pre["series"][sid]["members"]}
changed = {}
for ref, m in sorted(members.items()):
    rec = pre["reservations"][ref]
    assert m["scheduled_date"] in rec["starts_at_local"], (ref, m, rec)
    changed[ref] = {"date": m["scheduled_date"], "tables": list(rec["table_ids"]), "terms": None}
spec = {"series": sid, "rev_before": rev0, "owner": "u1",
        "path": "/series/%s/amend" % sid, "key": win[1], "clock": wclock,
        "body": '{"expected_revision":%d,"from_index":0,"local_time":"%s"}' % (rev0, wclock),
        "from_index": 0, "changed": changed,
        "unmoved": sorted(r for r in pre["reservations"] if r not in changed),
        "counter_rest": "r", "absent_keys": [lose[1]]}
spec_f = os.path.join(os.path.dirname(os.path.abspath(pre_f)), "mspec.json")
json.dump(spec, open(spec_f, "w"))
checker = os.path.join(os.path.dirname(os.path.abspath(pre_f)), "amend_delta.py")
r = subprocess.run([sys.executable, checker, pre_f, post_f, win[3], spec_f],
                   capture_output=True, text=True)
sys.stdout.write(r.stdout)
sys.stderr.write(r.stderr)
assert r.returncode == 0, "shared amend checker failed"
print("MRACE-OK winner=%s clock=%s" % (win[1], wclock))
PYEOF
if [ "$CRA" = "201" ]; then MWIN="$ART/mra.out"; else MWIN="$ART/mrb.out"; fi
$CURL -o "$ART/mget.out" -G "$BASE/series/$MSID" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/mget.out"
if cmp -s "$MWIN" "$ART/mget.out"; then echo "PASS: m-winner-get (identical)"; PASS=$((PASS + 1)); else echo "FAIL: m-winner-get"; FAIL=$((FAIL + 1)); fi
pycheck m-corrupt-guards - "$ART/mpre.json" "$ART/mpost.json" "$MWIN" "$ART/mspec.json" <<'PYEOF'
import json, sys, copy, subprocess, os
pref, postf, respf, specf = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
pre = json.load(open(pref))["state"]
post = json.load(open(postf))["state"]
spec = json.load(open(specf))
prog = os.path.join(os.path.dirname(os.path.abspath(pref)), "amend_delta.py")
changed0 = sorted(spec["changed"])[0]
def run(obj, tag):
    pj = pref + ".mut-%s.json" % tag
    json.dump({"track": "tablekeeper", "format_version": 1, "state": obj}, open(pj, "w"))
    r = subprocess.run([sys.executable, prog, pref, pj, respf, specf],
                       capture_output=True, text=True)
    print("%s exit=%d" % (tag, r.returncode))
    return r.returncode
results = {"genuine": run(post, "genuine")}
cases = []
o = copy.deepcopy(post)
o["reservations"][changed0]["user_id"] = "unknown-owner"
cases.append(("moved-private-owner", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["created_at"] = "1999-01-01T00:00:00Z"
cases.append(("moved-created-time", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["changes"][0]["from"] = "1999-01-01T00:00"
cases.append(("changed-from", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["seq"] = 99
cases.append(("changed-seq", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["at"] = "not-a-real-instant"
cases.append(("changed-at", o))
o = copy.deepcopy(post)
o["plans"]["unexpected-plan"] = copy.deepcopy(next(iter(o["plans"].values())))
cases.append(("extra-plan", o))
o = copy.deepcopy(post)
othersid = next(s for s in o["series"] if s != spec["series"])
o["series"]["unexpected-series"] = copy.deepcopy(o["series"][othersid])
cases.append(("extra-series", o))
o = copy.deepcopy(post)
o["histories"]["unexpected-ref"] = copy.deepcopy(o["histories"][changed0])
cases.append(("extra-orphan-history", o))
o = copy.deepcopy(post)
for k, v in o["receipts"].items():
    if k not in pre["receipts"]:
        v["body"] = '{"expected_revision":1,"from_index":0,"local_time":"00:00"}'
cases.append(("forged-receipt-body", o))
for name, obj in cases:
    assert json.dumps(obj, sort_keys=True) != json.dumps(post, sort_keys=True), "mutation did not apply: " + name
    results[name] = run(obj, name)
    assert results[name] != 0, name
assert results["genuine"] == 0, results
print("MSELFTEST-OK")
PYEOF

echo "== N fifty-way same-key race =="
FOUNDCODE=$(reset_world <<'EOF'
{"users": [{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],
 "restaurants": [{"id":"r","name":"N","timezone":"Europe/Berlin","slot_minutes":30,
  "reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,
  "opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],
  "tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4}],
  "combinable":[["t_1","t_2"]],"manager_user_ids":["u1"]}],
 "reservations": []}
EOF
)
expect n-reset "$FOUNDCODE" 204
[ "$FOUNDCODE" = "204" ] || { echo "FAIL: n-reset-abort"; FAIL=$((FAIL + 1)); exit 1; }
TOK_1=$(login "a@b" "password1")
TOK_1_CODE=${TOK_1%%:*}; TOK_1=${TOK_1#*:}
if [ "$TOK_1_CODE" = "200" ] && [ -n "$TOK_1" ]; then echo "PASS: login-u1 (200-nonempty)"; PASS=$((PASS + 1)); else echo "FAIL: login-u1"; FAIL=$((FAIL + 1)); exit 1; fi
expect n-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}')" 201
$CURL -o "$ART/nanch.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-anch' -d '{"restaurant_id":"r","table_id":"t_1","starts_at_local":"2027-05-06T18:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/nanch.out"
NAREF=$(py -c 'import json; print(json.load(open("'"$ART"'/nanch.out"))["reference"])' 2>>"$DIAG")
expect n-adopt "$($CURL -o "$ART/nad.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-ad' -d '{"anchor_reference":"'"$NAREF"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/nad.out"
NSID=$(py -c 'import json; print(json.load(open("'"$ART"'/nad.out"))["series_id"])' 2>>"$DIAG")
NGEN=$(py -c 'import json; print(json.load(open("'"$ART"'/nad.out"))["occurrences"][1]["reservation"]["reference"])' 2>>"$DIAG")
expect n-unrelated-booking "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-ub' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T21:30","party_size":1}')" 201
$CURL -o "$ART/nub.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-ub' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-06T21:30","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/nub.out"
NUB=$(py -c 'import json; print(json.load(open("'"$ART"'/nub.out"))["reference"])' 2>>"$DIAG")
expect n-unrelated-anchor "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-ua' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-20T19:00","party_size":1}')" 201
$CURL -o "$ART/nua.out" -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-ua' -d '{"restaurant_id":"r","table_id":"t_2","starts_at_local":"2027-05-20T19:00","party_size":1}' 2>>"$DIAG"
chmod 600 "$ART/nua.out"
NUA=$(py -c 'import json; print(json.load(open("'"$ART"'/nua.out"))["reference"])' 2>>"$DIAG")
expect n-unrelated-adopt "$($CURL -o "$ART/nus.out" -w '%{http_code}' -X POST "$BASE/series" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-us' -d '{"anchor_reference":"'"$NUA"'","count":2,"interval_weeks":1}')" 201
chmod 600 "$ART/nus.out"
NUSID=$(py -c 'import json; print(json.load(open("'"$ART"'/nus.out"))["series_id"])' 2>>"$DIAG")
expect n-unrelated-preview "$($CURL -o /dev/null -w '%{http_code}' -X POST "$BASE/restaurants/r/replans" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-upv' -d '{"table_id":"t_2","from":"2027-05-06T18:00:00+02:00","to":"2027-05-06T23:00:00+02:00"}')" 201
export_ok "$ART/npre.json" && { echo "PASS: n-export-pre (200)"; PASS=$((PASS + 1)); }
i=1
while [ "$i" -le 50 ]; do
  $CURL -o "$ART/n50-$i.out" -w '%{http_code}\n' -X POST "$BASE/series/$NSID/amend" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOK_1" -H 'Idempotency-Key: n-r50' -d '{"expected_revision":1,"from_index":0,"local_time":"20:00"}' > "$SCR/n50-$i.code" 2>>"$DIAG" &
  i=$((i + 1))
done
wait
chmod 600 "$ART"/n50-*.out
export_ok "$ART/npost.json" && { echo "PASS: n-export-post (200)"; PASS=$((PASS + 1)); }
pycheck n-race-outcome - "$ART" "$SCR" "$NSID" "$NUSID" "$NUB" "$NAREF" "$NGEN" "$ART/npre.json" "$ART/npost.json" <<'PYEOF'
import json, sys, glob, os, subprocess
art, scr, sid, usid, ub, aref, gen, pref, postf = sys.argv[1:10]
pre = json.load(open(pref))["state"]
codes, bodies = [], set()
for i in range(1, 51):
    codes.append(open(os.path.join(scr, "n50-%d.code" % i)).read().strip())
    bodies.add(open(os.path.join(art, "n50-%d.out" % i), "rb").read())
assert sorted(codes) == ["200"] * 49 + ["201"], sorted(codes)
assert len(bodies) == 1, "all 50 raw bodies byte-equal"
raw = bodies.pop()
wbody = json.loads(raw)
assert wbody["revision"] == pre["series"][sid]["revision"] + 1, wbody["revision"]
assert wbody["series_id"] == sid
members = {m["reference"]: m for m in pre["series"][sid]["members"]}
assert set(members) == set([aref, gen]), sorted(members)
changed = {}
for ref, m in sorted(members.items()):
    rec = pre["reservations"][ref]
    assert m["scheduled_date"] in rec["starts_at_local"], (ref, m, rec)
    changed[ref] = {"date": m["scheduled_date"], "tables": list(rec["table_ids"]), "terms": None}
rev0 = pre["series"][sid]["revision"]
spec = {"series": sid, "rev_before": rev0, "owner": "u1",
        "path": "/series/%s/amend" % sid, "key": "n-r50", "clock": "20:00",
        "body": '{"expected_revision":%d,"from_index":0,"local_time":"20:00"}' % rev0,
        "from_index": 0, "changed": changed,
        "unmoved": sorted(r for r in pre["reservations"] if r not in changed),
        "counter_rest": "r", "absent_keys": []}
spec_f = os.path.join(art, "nspec.json")
json.dump(spec, open(spec_f, "w"))
resp_f = os.path.join(art, "n50-winner.out")
open(resp_f, "wb").write(raw)
checker = os.path.join(art, "amend_delta.py")
r = subprocess.run([sys.executable, checker, pref, postf, resp_f, spec_f],
                   capture_output=True, text=True)
sys.stdout.write(r.stdout)
sys.stderr.write(r.stderr)
assert r.returncode == 0, "shared amend checker failed"
print("NRACE-OK")
PYEOF
$CURL -o "$ART/nget.out" -G "$BASE/series/$NSID" -H "Authorization: Bearer $TOK_1" 2>>"$DIAG"
chmod 600 "$ART/nget.out"
pycheck n-winner-get - "$ART/n50-1.out" "$ART/nget.out" <<'PYEOF'
import sys
outs = set()
import glob
for f in glob.glob(sys.argv[1].rsplit("/",1)[0] + "/n50-*.out"):
    outs.add(open(f,"rb").read())
assert len(outs) == 1
assert outs.pop() == open(sys.argv[2],"rb").read(), "GET != winner bytes"
print("NGET-OK")
PYEOF
pycheck n-corrupt-guards - "$ART/npre.json" "$ART/npost.json" "$ART/n50-winner.out" "$ART/nspec.json" <<'PYEOF'
import json, sys, copy, subprocess, os
pref, postf, respf, specf = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
pre = json.load(open(pref))["state"]
post = json.load(open(postf))["state"]
spec = json.load(open(specf))
prog = os.path.join(os.path.dirname(os.path.abspath(pref)), "amend_delta.py")
changed0 = sorted(spec["changed"])[0]
unmoved0 = sorted(spec["unmoved"])[0]
def run(obj, tag):
    pj = pref + ".mut-%s.json" % tag
    json.dump({"track": "tablekeeper", "format_version": 1, "state": obj}, open(pj, "w"))
    r = subprocess.run([sys.executable, prog, pref, pj, respf, specf],
                       capture_output=True, text=True)
    print("%s exit=%d" % (tag, r.returncode))
    return r.returncode
results = {"genuine": run(post, "genuine")}
cases = []
o = copy.deepcopy(post)
o["reservations"][changed0]["table_ids"] = ["t_2"]
cases.append(("forged-selector", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["user_id"] = "unknown-owner"
cases.append(("forged-owner", o))
o = copy.deepcopy(post)
o["reservations"][changed0]["created_at"] = "1999-01-01T00:00:00Z"
cases.append(("forged-created", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["changes"][0]["from"] = "1999-01-01T00:00"
cases.append(("forged-from", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["seq"] = 99
cases.append(("forged-seq", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["at"] = "not-a-real-instant"
cases.append(("forged-at", o))
o = copy.deepcopy(post)
o["histories"][changed0][-1]["accepted_terms"]["slot_minutes"] = 45
cases.append(("forged-terms", o))
o = copy.deepcopy(post)
o["plans"]["unexpected-plan"] = copy.deepcopy(next(iter(o["plans"].values())) if o["plans"] else {"applied": False})
cases.append(("extra-plan", o))
o = copy.deepcopy(post)
o["histories"]["unexpected-ref"] = copy.deepcopy(o["histories"][changed0])
cases.append(("extra-orphan-history", o))
o = copy.deepcopy(post)
for k, v in o["receipts"].items():
    if k not in pre["receipts"]:
        v["user_id"] = "wrong-owner"
cases.append(("forged-receipt-owner", o))
o = copy.deepcopy(post)
for k, v in o["receipts"].items():
    if k not in pre["receipts"]:
        v["body"] = '{"expected_revision":1,"from_index":0,"local_time":"00:00"}'
cases.append(("forged-receipt-body", o))
o = copy.deepcopy(post)
o["restaurant_revisions"]["r"] = pre["restaurant_revisions"]["r"] + 9
cases.append(("forged-counter", o))
for name, obj in cases:
    before = json.dumps(obj, sort_keys=True)
    results[name] = run(obj, name)
    assert results[name] != 0, name
assert results["genuine"] == 0, results
print("NSELFTEST-OK")
PYEOF

echo "checks passed: $PASS failed: $FAIL"
[ "$FAIL" = "0" ]