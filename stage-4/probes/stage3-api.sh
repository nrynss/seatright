#!/bin/sh
# stage3-api.sh - independent comprehensive stage-3 HTTP API probe.
#
# Usage: sh stage-3/probes/stage3-api.sh BASE PRIVATE_WORK
#   (run with CWD set to stage-3; e.g. sh probes/stage3-api.sh URL DIR)
#
# Covers inherited delivery/API/auth/public/errors/idempotency/booking/
# options/selection/PATCH/cancel/moves/DST/owner behavior plus stage-3
# R200-218 explain/history, R220-258 policies/terms/cutoffs/no-ops/
# expected-revision/decision, R259-285 series adoption/calendar/flags/
# replays and R289-303 pair capacity/collective counters. It also runs a
# same-image current export/import smoke, explicitly labeled as such: it is
# NOT a genuine stage-1/2 to 3 transfer and NOT exhaustive modern corruption
# validation (that belongs to the I lanes). No browser/R219/R288 claims.
#
# POSIX sh + curl + python3 stdlib only (no jq). Interface:
#   sh probes/stage3-api.sh BASE PRIVATE_WORK
# BASE is validated as an http(s) URL; PRIVATE_WORK is created 0700 and all
# token-bearing artifacts are stored 0600 inside it. Stdout/stderr carry
# only test names, codes and counts, never tokens, passwords, bodies or
# export JSON. Python diagnostics go to the private tree. curl calls carry
# connect/max-time bounds. Every genuine assertion counts through expect()
# or pycheck(); any nonzero FAIL count exits nonzero. SKIP is reported
# separately and never fails the run. STAGE3_API_SABOTAGE=1 corrupts one
# expectation to prove guards count failures (probe self-test only).
set -u

BASE=${1:?usage: sh probes/stage3-api.sh BASE PRIVATE_WORK}
OUT=${2:?usage: sh probes/stage3-api.sh BASE PRIVATE_WORK}
case "$BASE" in
  http://*|https://*) ;;
  *) echo "FAIL bad-base (BASE must be an http(s) URL)"; exit 1 ;;
esac
case "$OUT" in
  ""|/) echo "FAIL bad-workdir"; exit 1 ;;
esac

umask 077
mkdir -p "$OUT"
chmod 0700 "$OUT"
DIAG="$OUT/diag.log"
: > "$DIAG"
chmod 600 "$DIAG"
TMP=$(mktemp -d "$OUT/tmp.XXXXXX")
trap 'rm -rf "$TMP"' EXIT INT TERM

PASS=0
FAIL=0
SKIP=0

expect() {
  if [ "$2" = "$3" ]; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (got $2, want $3)"
    FAIL=$((FAIL + 1))
  fi
}

pycheck() {
  if python3 -c "$2" 2>>"$DIAG"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
}

skip() {
  echo "SKIP: $1 ($2)"
  SKIP=$((SKIP + 1))
}

require_clean() {
  if [ "$FAIL" != 0 ]; then
    echo "ABORT: $1 already failed ($FAIL failures)"
    exit 1
  fi
}

CURL="curl -s --connect-timeout 5 --max-time 25"

api() {
  method=$1; path=$2; bodyfile=$3; token=$4; key=$5
  set -- -s --connect-timeout 5 --max-time 25 -o "$TMP/out" -w '%{http_code}' -X "$method"
  if [ -n "$bodyfile" ]; then
    set -- "$@" -H 'Content-Type: application/json' --data-binary "@$bodyfile"
  fi
  if [ -n "$token" ]; then
    set -- "$@" -H "Authorization: Bearer $token"
  fi
  if [ -n "$key" ]; then
    set -- "$@" -H "Idempotency-Key: $key"
  fi
  curl "$@" "$BASE$path" > "$TMP/status"
}

jget() {
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get(sys.argv[2], ""))' "$1" "$2" 2>>"$DIAG"
}

write() {
  printf '%s' "$2" > "$TMP/$1"
}

export_state() {
  $CURL -s --connect-timeout 5 --max-time 25 -o "$OUT/$1" -w '%{http_code}' "$BASE/_test/export" > "$TMP/exp-status" 2>>"$DIAG"
  if [ "$(cat "$TMP/exp-status")" != "200" ]; then
    echo "FAIL: export-$1 status $(cat "$TMP/exp-status")"
    FAIL=$((FAIL + 1))
    return 1
  fi
  if ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)' "$OUT/$1" 2>>"$DIAG"; then
    echo "FAIL: export-$1 envelope"
    FAIL=$((FAIL + 1))
    return 1
  fi
  return 0
}

SABOTAGE=${STAGE3_API_SABOTAGE:-0}

echo "== input validation =="
HOST_PART=${BASE#*://}
HOST_PART=${HOST_PART%%/*}
SCHEME_PART=${BASE%%://*}
if [ -n "$SCHEME_PART" ] && [ -n "$HOST_PART" ] && [ "$HOST_PART" != *"$"* ] && [ "$HOST_PART" != *" "* ]; then
  echo "PASS: base-url-shape ($SCHEME_PART host present)"
  PASS=$((PASS + 1))
else
  echo "FAIL: base-url-shape"
  FAIL=$((FAIL + 1))
fi
if [ -d "$OUT" ] && [ -w "$OUT" ]; then
  echo "PASS: workdir-owned-writable"
  PASS=$((PASS + 1))
else
  echo "FAIL: workdir-owned-writable"
  FAIL=$((FAIL + 1))
fi

echo "== health =="
api GET /health "" "" ""
if [ "$SABOTAGE" = "1" ]; then
  expect "health-200" "$(cat "$TMP/status")" "404"
else
  expect "health-200" "$(cat "$TMP/status")" "200"
fi
pycheck "health-body" 'import json; assert json.load(open("'"$TMP"'/out"))=={"status":"ok"}'
$CURL -s -D "$TMP/hdrs" -o /dev/null "$BASE/health" 2>>"$DIAG"
pycheck "health-charset" 'import sys; h=open("'"$TMP"'/hdrs").read(); assert "application/json; charset=utf-8" in h, h'

echo "== reset fixture =="
python3 <<'PYEOF' > "$TMP/fixture.json" 2>>"$DIAG"
import json
print(json.dumps({
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": "correct horse bea", "display_name": "Bea"}],
  "restaurants": [
    {"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
     "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
     "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                       {"weekday": "fri", "opens": "18:00", "closes": "23:30"},
                       {"weekday": "sun", "opens": "00:00", "closes": "05:00"}],
     "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                {"id": "t_2", "label": "2", "capacity": 4},
                {"id": "t_3", "label": "3", "capacity": 4}],
     "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
     "manager_user_ids": ["u_ada"]},
    {"id": "r_ny", "name": "New York", "timezone": "America/New_York",
     "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
     "opening_hours": [{"weekday": "sun", "opens": "00:00", "closes": "05:00"}],
     "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
     "manager_user_ids": ["u_ada"]},
    {"id": "r_san", "name": "Santiago", "timezone": "America/Santiago",
     "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
     "opening_hours": [{"weekday": "sun", "opens": "18:00", "closes": "23:00"}],
     "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
     "manager_user_ids": ["u_ada"]}],
  "reservations": []}))
PYEOF
cp "$TMP/fixture.json" "$OUT/fixture.json"
api POST /_test/reset "$TMP/fixture.json" "" ""
expect "reset-204" "$(cat "$TMP/status")" "204"
require_clean "setup"

echo "== dates =="
DATE=$(python3 -c '
import datetime
d = datetime.date.today() + datetime.timedelta(days=35)
while d.strftime("%a") != "Thu":
    d += datetime.timedelta(days=1)
print(d)' 2>>"$DIAG")
export DATE
pycheck "future-thursday" 'import datetime,os; d=datetime.date.fromisoformat(os.environ["DATE"]); assert d.strftime("%a")=="Thu" and (d-datetime.date.today()).days>=30'
pycheck "fixed-sundays" 'import datetime; assert datetime.date(2026,3,29).strftime("%a")=="Sun"; assert datetime.date(2026,11,1).strftime("%a")=="Sun"; assert datetime.date(2027,8,29).strftime("%a")=="Sun"'

echo "== auth =="
write signup '{"email":"n@example.com","password":"correct horse","display_name":"N"}'
api POST /auth/signup "$TMP/signup" "" ""
expect "signup-201" "$(cat "$TMP/status")" "201"
api POST /auth/signup "$TMP/signup" "" ""
expect "dup-email-409" "$(cat "$TMP/status")" "409"
write short '{"email":"s@example.com","password":"short","display_name":"S"}'
api POST /auth/signup "$TMP/short" "" ""
expect "shortpw-422" "$(cat "$TMP/status")" "422"
write badmail '{"email":"nope","password":"correct horse","display_name":"X"}'
api POST /auth/signup "$TMP/badmail" "" ""
expect "bademail-422" "$(cat "$TMP/status")" "422"
write login-ada '{"email":"ada@example.com","password":"correct horse"}'
api POST /auth/login "$TMP/login-ada" "" ""
expect "login-200" "$(cat "$TMP/status")" "200"
cp "$TMP/out" "$OUT/tok-ada.json"
write login-bea '{"email":"bea@example.com","password":"correct horse bea"}'
api POST /auth/login "$TMP/login-bea" "" ""
expect "login-bea-200" "$(cat "$TMP/status")" "200"
cp "$TMP/out" "$OUT/tok-bea.json"
export OUT
pycheck "tokens-distinct" 'import json,os; a=json.load(open(os.environ["OUT"]+"/tok-ada.json"))["token"]; b=json.load(open(os.environ["OUT"]+"/tok-bea.json"))["token"]; assert a and b and a!=b'
TOKA=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-ada.json"))["token"])' 2>>"$DIAG")
TOKB=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-bea.json"))["token"])' 2>>"$DIAG")
write badpw '{"email":"ada@example.com","password":"nope nope nope"}'
api POST /auth/login "$TMP/badpw" "" ""
expect "badpw-401" "$(cat "$TMP/status")" "401"
require_clean "auth"
export TOKA TOKB

echo "== public catalogue =="
api GET /restaurants "" "" ""
expect "restaurants-200" "$(cat "$TMP/status")" "200"
pycheck "restaurants-shape" 'import json; d=json.load(open("'"$TMP"'/out")); assert [(r["id"],r["name"],r["timezone"]) for r in d["restaurants"]]==[("r_anker","Zum Anker","Europe/Berlin"),("r_ny","New York","America/New_York"),("r_san","Santiago","America/Santiago")]'
api GET /restaurants/r_nope "" "" ""
expect "detail-unknown-404" "$(cat "$TMP/status")" "404"
api GET /restaurants/r_anker "" "" ""
expect "detail-200" "$(cat "$TMP/status")" "200"
pycheck "detail-original" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["slot_minutes"]==30 and d["reservation_duration_minutes"]==90 and [t["capacity"] for t in d["tables"]]==[2,4,4] and "manager_user_ids" not in d'
api GET /restaurants/r_anker/policies "" "" ""
expect "policies-empty-200" "$(cat "$TMP/status")" "200"
pycheck "policies-empty" 'import json; assert json.load(open("'"$TMP"'/out"))=={"policies":[]}'

echo "== policy publication matrix =="
policy() {
  python3 -c 'import json,sys; print(json.dumps({"effective_from":sys.argv[1],"slot_minutes":60,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}))' "$1" > "$TMP/policy.json" 2>>"$DIAG"
}
api POST /restaurants/r_anker/policies "$TMP/login-ada" "" "nokey-body"
expect "policy-no-token-401" "$(cat "$TMP/status")" "401"
policy "$DATE"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "badtoken" "k-bad"
expect "policy-bad-token-401" "$(cat "$TMP/status")" "401"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKB" "k-bea"
expect "policy-nonmanager-403" "$(cat "$TMP/status")" "403"
pycheck "policy-forbidden-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="forbidden"'
api POST /restaurants/r_nope/policies "$TMP/policy.json" "$TOKA" "k-nope"
expect "policy-unknown-404" "$(cat "$TMP/status")" "404"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKA" ""
expect "policy-missing-key-400" "$(cat "$TMP/status")" "400"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKA" "k-01"
expect "policy-publish-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/pol-v1.json"
pycheck "policy-version-1" 'import json,os; d=json.load(open(os.environ["OUT"]+"/pol-v1.json")); assert d["policy_version"]==1 and d["effective_from"]==os.environ["DATE"] and "effective_from" in d'
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKA" "k-01"
expect "policy-replay-200" "$(cat "$TMP/status")" "200"
pycheck "policy-replay-bytes" 'import os; assert open(os.environ["OUT"]+"/pol-v1.json","rb").read()==open("'"$TMP"'/out","rb").read()'
write policy-extra '{"effective_from":"2027-06-01","slot_minutes":60,"x":1}'
api POST /restaurants/r_anker/policies "$TMP/policy-extra" "$TOKA" "k-01"
expect "policy-diffbody-409" "$(cat "$TMP/status")" "409"
write badpol1 '{"slot_minutes":30}'
api POST /restaurants/r_anker/policies "$TMP/badpol1" "$TOKA" "k-bad1"
expect "policy-partial-422" "$(cat "$TMP/status")" "422"
write badpol2 '{"effective_from":"2027-02-29","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/badpol2" "$TOKA" "k-bad2"
expect "policy-baddate-422" "$(cat "$TMP/status")" "422"
write badpol3 '{"effective_from":"2027-06-01","slot_minutes":true,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/badpol3" "$TOKA" "k-bad3"
expect "policy-bool-422" "$(cat "$TMP/status")" "422"
write badpol4 '{"effective_from":"2027-06-01","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4}}'
api POST /restaurants/r_anker/policies "$TMP/badpol4" "$TOKA" "k-bad4"
expect "policy-caps-422" "$(cat "$TMP/status")" "422"
api POST /restaurants/r_anker/policies "$TMP/badpol1" "$TOKA" "k-bad1"
expect "policy-failed-reusable" "$(cat "$TMP/status")" "422"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKA" "k-bad1"
expect "policy-failed-key-reused-201" "$(cat "$TMP/status")" "201"
api GET /restaurants/r_anker/policies "" "" ""
expect "policies-list-200" "$(cat "$TMP/status")" "200"
pycheck "policies-versions" 'import json; d=json.load(open("'"$TMP"'/out")); assert [p["policy_version"] for p in d["policies"]]==[1,2]'
EXPECT_V=$(python3 -c 'import json; print(max(p["policy_version"] for p in json.load(open("'"$TMP"'/out"))["policies"]))' 2>>"$DIAG")
export EXPECT_V
require_clean "policies"

echo "== dated availability =="
avail() {
  $CURL -s --connect-timeout 5 --max-time 25 -o "$TMP/avail.json" -w '%{http_code}' "$BASE/availability?restaurant_id=$1&date=$2&party_size=$3$4" > "$TMP/status" 2>>"$DIAG"
  cat "$TMP/status"
}
expect "avail-grid" "$(avail r_anker "$DATE" 8 "")" "200"
pycheck "avail-hourly" 'import json,os; d=json.load(open("'"$TMP"'/avail.json")); want=[os.environ["DATE"]+"T"+h for h in ["18:00","19:00","20:00","21:00","22:00"]]; assert [s["starts_at_local"] for s in d["slots"]]==want'

pycheck "avail-pair10" 'import json; d=json.load(open("'"$TMP"'/avail.json")); o=d["slots"][0]["available_options"]; assert len(o)==2 and o[0]["table_ids"]==["t_1","t_2"] and o[0]["capacity"]==10 and o[1]["table_ids"]==["t_2","t_3"] and o[1]["capacity"]==10, o'
pycheck "avail-no-transitive" 'import json; d=json.load(open("'"$TMP"'/avail.json")); pairs=[o["table_ids"] for s in d["slots"] for o in s["available_options"] if len(o["table_ids"])==2]; assert ["t_1","t_3"] not in pairs, pairs'
pycheck "avail-singles-empty" 'import json; d=json.load(open("'"$TMP"'/avail.json")); assert all(s["available_table_ids"]==[] for s in d["slots"])'
expect "avail-party4" "$(avail r_anker "$DATE" 4 "")" "200"
pycheck "avail-party4-caps" 'import json; d=json.load(open("'"$TMP"'/avail.json")); o=d["slots"][0]["available_options"]; assert [x["capacity"] for x in o[:3]]==[4,6,4] and len(o)==5'
expect "avail-explain" "$(avail r_anker "$DATE" 4 "&explain=true")" "200"
pycheck "explain-matrix" 'import json,os; d=json.load(open("'"$TMP"'/avail.json")); e=d["slots"][0]["explain"]; assert [x["table_id"] for x in e]==["t_1","t_2","t_3"]; assert all([r["rule"] for r in x["rules"]]==["capacity","no_overlap"] for x in e); assert all(x["available"]==(x["rules"][0]["holds"] and x["rules"][1]["holds"]) for x in e); assert [x["table_id"] for x in e if x["available"]]==d["slots"][0]["available_table_ids"]; assert all(x["policy_version"]==int(os.environ["EXPECT_V"]) for x in e)'
for bad in "false" "1" ""; do
  if [ -z "$bad" ]; then
    code=$(avail r_anker "$DATE" 4 "&explain=")
  else
    code=$(avail r_anker "$DATE" 4 "&explain=$bad")
  fi
  expect "explain-reject-$bad" "$code" "422"
done
expect "avail-noexplain" "$(avail r_anker "$DATE" 4 "")" "200"
pycheck "noexplain-absent" 'import json; d=json.load(open("'"$TMP"'/avail.json")); assert all("explain" not in s for s in d["slots"])'
expect "avail-closed" "$(avail r_anker 2027-06-16 2 "")" "200"
pycheck "closed-empty" 'import json; assert json.load(open("'"$TMP"'/avail.json"))["slots"]==[]'
require_clean "availability"

echo "== DST =="
expect "berlin-gap" "$(avail r_anker 2026-03-29 2 "")" "200"
pycheck "gap-skipped" 'import json; d=json.load(open("'"$TMP"'/avail.json")); locals=[s["starts_at_local"] for s in d["slots"]]; assert "2026-03-29T02:00" not in locals and "2026-03-29T02:30" not in locals, locals'
expect "ny-fold" "$(avail r_ny 2026-11-01 2 "")" "200"
pycheck "fold-first" 'import json; d=json.load(open("'"$TMP"'/avail.json")); ones=[s for s in d["slots"] if s["starts_at_local"]=="2026-11-01T01:00"]; assert len(ones)==1 and ones[0]["starts_at"]=="2026-11-01T01:00:00-04:00", d["slots"]'

echo "== dated writes =="
create() {
  printf '%s' "$3" > "$TMP/create-body.json"
  api POST /reservations "$TMP/create-body.json" "$1" "$2"
}
create "$TOKB" "w-single" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$DATE"'T19:00","party_size":4}'
expect "create-single-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/create-single.json"
CB_A=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/create-single.json"))["reference"])' 2>>"$DIAG")
export CB_A
pycheck "create-terms" 'import json,os; d=json.load(open(os.environ["OUT"]+"/create-single.json")); assert d["revision"]==1; t=d["accepted_terms"]; assert t["policy_version"]==int(os.environ["EXPECT_V"]) and set(t)=={"policy_version","slot_minutes","reservation_duration_minutes","cancellation_cutoff_minutes","opening_hours","capacities"}'
pycheck "create-end-60" 'import json,os; d=json.load(open(os.environ["OUT"]+"/create-single.json")); assert d["ends_at"][:16]==os.environ["DATE"]+"T20:00", d["ends_at"]'
create "$TOKB" "w-pair" '{"restaurant_id":"r_anker","table_ids":["t_2","t_1"],"starts_at_local":"'"$DATE"'T21:00","party_size":8}'
expect "create-pair-201" "$(cat "$TMP/status")" "201"
pycheck "pair-canonical" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["table_ids"]==["t_1","t_2"] and "table_id" not in d'
cp "$TMP/out" "$OUT/create-pair.json"
CB_P=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/create-pair.json"))["reference"])' 2>>"$DIAG")
export CB_P
api GET "/reservations/$CB_A/history" "" "$TOKB" ""
expect "history-200" "$(cat "$TMP/status")" "200"
pycheck "history-created" 'import json; d=json.load(open("'"$TMP"'/out")); e=d["entries"]; assert len(e)==1 and e[0]["seq"]==1 and e[0]["event"]=="created" and e[0]["revision"]==1; c=e[0]["changes"]; assert [x["field"] for x in c]==["table_id","starts_at_local","party_size"] and all(x["from"] is None for x in c)'
api GET "/reservations/$CB_P/history" "" "$TOKB" ""
pycheck "history-pair-created" 'import json; d=json.load(open("'"$TMP"'/out")); c=d["entries"][0]["changes"]; assert c[0]["field"]=="table_ids" and c[0]["from"] is None and c[0]["to"]==["t_1","t_2"]'
require_clean "writes"

echo "== PATCH matrix =="
patch() {
  printf '%s' "$3" > "$TMP/patch-body.json"
  api PATCH "/reservations/$1" "$TMP/patch-body.json" "$2" ""
}
patch "$CB_A" "$TOKB" '{"party_size":5}'
expect "patch-200" "$(cat "$TMP/status")" "200"
pycheck "patch-rev2" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==2 and d["party_size"]==5'
for frag in '"expected_revision":true' '"expected_revision":"2"' '"expected_revision":null' '"expected_revision":2.5' '"expected_revision":0'; do
  patch "$CB_A" "$TOKB" '{"party_size":5,'"$frag"'}'
  expect "expected-bad-$frag" "$(cat "$TMP/status")" "422"
done
patch "$CB_A" "$TOKB" '{"expected_revision":9000000000000001,"party_size":5}'
expect "expected-huge-409" "$(cat "$TMP/status")" "409"
patch "$CB_A" "$TOKB" '{"expected_revision":1000000000000000000000,"table_id":"t_nope"}'
expect "expected-1e21-409" "$(cat "$TMP/status")" "409"
patch "$CB_A" "$TOKB" '{"expected_revision":2,"party_size":6}'
expect "expected-match-200" "$(cat "$TMP/status")" "200"
api GET "/reservations/$CB_A/history" "" "$TOKB" ""
pycheck "history-changed" 'import json,os; d=json.load(open("'"$TMP"'/out")); e=d["entries"]; assert len(e)==3 and e[1]["event"]=="changed" and e[1]["revision"]==2 and e[2]["revision"]==3; assert [x["field"] for x in e[1]["changes"]]==["party_size"] and [x["field"] for x in e[2]["changes"]]==["party_size"]; assert e[0]["accepted_terms"]["policy_version"]==int(os.environ["EXPECT_V"]) and e[1]["accepted_terms"]["policy_version"]==int(os.environ["EXPECT_V"])'
require_clean "patch"

echo "== no-op retention =="
api GET "/reservations/$CB_P" "" "$TOKB" ""
cp "$TMP/out" "$OUT/pair-before.json"
for body in '{}' '{"zzz":9}' '{"table_ids":["t_1","t_2"],"starts_at_local":"'"$DATE"'T21:00","party_size":8}' '{"table_ids":["t_2","t_1"]}'; do
  patch "$CB_P" "$TOKB" "$body"
  expect "noop-200" "$(cat "$TMP/status")" "200"
done
pycheck "noop-bytes" 'import os; assert open(os.environ["OUT"]+"/pair-before.json","rb").read()==open("'"$TMP"'/out","rb").read()'
export_state "export-noop.json"
cp "$TMP/status" "$OUT/export-noop.status"
pycheck "noop-state-frozen" 'import json,os; a=json.load(open(os.environ["OUT"]+"/export-noop.json")); h=a["state"]["histories"]; assert len(h[os.environ["CB_P"]])==1; assert a["state"]["reservations"][os.environ["CB_P"]]["revision"]==1'

echo "== off-grid seed no-op retention =="
python3 <<'PYEOF' > "$TMP/offgrid.json" 2>>"$DIAG"
import json, os
print(json.dumps({
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
    "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
    "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
    "tables": [{"id": "t_1", "label": "1", "capacity": 2},
               {"id": "t_2", "label": "2", "capacity": 4},
               {"id": "t_3", "label": "3", "capacity": 4}],
    "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
    "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "seed-off", "reference": "SEDOFF", "user_id": "u_ada",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": os.environ["DATE"] + "T18:07", "party_size": 2},
    {"id": "seed-pair", "reference": "SEDPAIR", "user_id": "u_ada",
     "restaurant_id": "r_anker", "table_ids": ["t_1", "t_2"],
     "starts_at_local": os.environ["DATE"] + "T21:07", "party_size": 6}]}))
PYEOF
api POST /_test/reset "$TMP/offgrid.json" "" ""
expect "offgrid-reset-204" "$(cat "$TMP/status")" "204"
api POST /auth/login "$TMP/login-ada" "" ""
cp "$TMP/out" "$OUT/tok-off.json"
TOKOFF=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-off.json"))["token"])' 2>>"$DIAG")
export TOKOFF
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKOFF" "off-pol"
expect "offgrid-publish-201" "$(cat "$TMP/status")" "201"
api GET /reservations/SEDOFF "" "$TOKOFF" ""
cp "$TMP/out" "$OUT/off-single-before.json"
api GET /reservations/SEDPAIR "" "$TOKOFF" ""
cp "$TMP/out" "$OUT/off-pair-before.json"
export_state "off-pre.json"
for body in '{}' '{"zzz":9}' '{"table_id":"t_1","starts_at_local":"'"$DATE"'T18:07","party_size":2}'; do
  printf '%s' "$body" > "$TMP/off-body.json"
  api PATCH /reservations/SEDOFF "$TMP/off-body.json" "$TOKOFF" ""
  expect "offgrid-single-noop-200" "$(cat "$TMP/status")" "200"
  pycheck "offgrid-single-bytes" 'import os; assert open(os.environ["OUT"]+"/off-single-before.json","rb").read()==open("'"$TMP"'/out","rb").read()'
  export_state "off-mid.json"
  pycheck "offgrid-single-export" 'import os; assert open(os.environ["OUT"]+"/off-pre.json","rb").read()==open(os.environ["OUT"]+"/off-mid.json","rb").read()'
done
printf '{"table_ids":["t_2","t_1"]}' > "$TMP/off-rev.json"
api PATCH /reservations/SEDPAIR "$TMP/off-rev.json" "$TOKOFF" ""
expect "offgrid-reversed-200" "$(cat "$TMP/status")" "200"
pycheck "offgrid-pair-bytes" 'import os; assert open(os.environ["OUT"]+"/off-pair-before.json","rb").read()==open("'"$TMP"'/out","rb").read()'
export_state "off-post.json"
pycheck "offgrid-pair-export" 'import os; assert open(os.environ["OUT"]+"/off-pre.json","rb").read()==open(os.environ["OUT"]+"/off-post.json","rb").read()'
require_clean "offgrid-noop"

echo "== cutoff both directions =="
python3 <<'PYEOF' > "$TMP/cutfix.json" 2>>"$DIAG"
import datetime, json
now = datetime.datetime.now(datetime.timezone.utc).replace(second=0, microsecond=0)
start = (now + datetime.timedelta(minutes=75)).strftime("%Y-%m-%dT%H:%M")
print(json.dumps({
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_utc", "name": "UTC", "timezone": "UTC", "slot_minutes": 30,
    "reservation_duration_minutes": 30, "cancellation_cutoff_minutes": 120,
    "opening_hours": [{"weekday": d, "opens": "00:00", "closes": "23:59"}
                      for d in ["mon","tue","wed","thu","fri","sat","sun"]],
    "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
    "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "seed-near", "reference": "NEARCUT", "user_id": "u_ada",
     "restaurant_id": "r_utc", "table_id": "t_1",
     "starts_at_local": start, "party_size": 2}]}))
PYEOF
api POST /_test/reset "$TMP/cutfix.json" "" ""
expect "cutoff-reset-204" "$(cat "$TMP/status")" "204"
api POST /auth/login "$TMP/login-ada" "" ""
cp "$TMP/out" "$OUT/tok-cut.json"
TOKC=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-cut.json"))["token"])' 2>>"$DIAG")
export TOKC
write cutpol '{"effective_from":"2020-01-01","slot_minutes":30,"reservation_duration_minutes":30,"cancellation_cutoff_minutes":30,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:59"},{"weekday":"tue","opens":"00:00","closes":"23:59"},{"weekday":"wed","opens":"00:00","closes":"23:59"},{"weekday":"thu","opens":"00:00","closes":"23:59"},{"weekday":"fri","opens":"00:00","closes":"23:59"},{"weekday":"sat","opens":"00:00","closes":"23:59"},{"weekday":"sun","opens":"00:00","closes":"23:59"}],"capacities":{"t_1":4}}'
api POST /restaurants/r_utc/policies "$TMP/cutpol" "$TOKC" "cut-1"
expect "cutoff-publish-201" "$(cat "$TMP/status")" "201"
patchcut() {
  printf '%s' "$2" > "$TMP/cut-body.json"
  api PATCH "/reservations/$1" "$TMP/cut-body.json" "$TOKC" ""
}
patchcut NEARCUT '{"party_size":3}'
expect "old-cutoff-blocks-409" "$(cat "$TMP/status")" "409"
pycheck "old-cutoff-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="cutoff_passed"'
export_state "cut-block-pre.json"
api POST /reservations/NEARCUT/cancel "" "$TOKC" ""
expect "old-cutoff-cancel-409" "$(cat "$TMP/status")" "409"
pycheck "old-cutoff-cancel-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="cutoff_passed"'
export_state "cut-block-post.json"
pycheck "old-cutoff-export-same" 'import os; assert open(os.environ["OUT"]+"/cut-block-pre.json","rb").read()==open(os.environ["OUT"]+"/cut-block-post.json","rb").read()'

echo "== cutoff old-zero allows =="
python3 <<'PYEOF' > "$TMP/cutfix0.json" 2>>"$DIAG"
import datetime, json
now = datetime.datetime.now(datetime.timezone.utc).replace(second=0, microsecond=0)
floor = now.replace(minute=(now.minute // 30) * 30)
start = (floor + datetime.timedelta(minutes=90)).strftime("%Y-%m-%dT%H:%M")
start2 = (floor + datetime.timedelta(minutes=150)).strftime("%Y-%m-%dT%H:%M")
print(json.dumps({
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_utc", "name": "UTC", "timezone": "UTC", "slot_minutes": 30,
    "reservation_duration_minutes": 30, "cancellation_cutoff_minutes": 0,
    "opening_hours": [{"weekday": d, "opens": "00:00", "closes": "23:59"}
                      for d in ["mon","tue","wed","thu","fri","sat","sun"]],
    "tables": [{"id": "t_1", "label": "1", "capacity": 4}],
    "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "seed-near", "reference": "NEARCUT", "user_id": "u_ada",
     "restaurant_id": "r_utc", "table_id": "t_1",
     "starts_at_local": start, "party_size": 2},
    {"id": "seed-near2", "reference": "NEARC2", "user_id": "u_ada",
     "restaurant_id": "r_utc", "table_id": "t_1",
     "starts_at_local": start2, "party_size": 2}]}))
PYEOF
api POST /_test/reset "$TMP/cutfix0.json" "" ""
expect "cutoff0-reset-204" "$(cat "$TMP/status")" "204"
api POST /auth/login "$TMP/login-ada" "" ""
cp "$TMP/out" "$OUT/tok-cut0.json"
TOKC0=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-cut0.json"))["token"])' 2>>"$DIAG")
export TOKC0
write cutpol0 '{"effective_from":"2020-01-01","slot_minutes":30,"reservation_duration_minutes":30,"cancellation_cutoff_minutes":10080,"opening_hours":[{"weekday":"mon","opens":"00:00","closes":"23:59"},{"weekday":"tue","opens":"00:00","closes":"23:59"},{"weekday":"wed","opens":"00:00","closes":"23:59"},{"weekday":"thu","opens":"00:00","closes":"23:59"},{"weekday":"fri","opens":"00:00","closes":"23:59"},{"weekday":"sat","opens":"00:00","closes":"23:59"},{"weekday":"sun","opens":"00:00","closes":"23:59"}],"capacities":{"t_1":4}}'
api POST /restaurants/r_utc/policies "$TMP/cutpol0" "$TOKC0" "cut0-1"
expect "cutoff0-publish-201" "$(cat "$TMP/status")" "201"
api PATCH /reservations/NEARC2 "$TMP/cut-body.json" "$TOKC0" ""
expect "old-zero-patch-200" "$(cat "$TMP/status")" "200"
api POST /reservations/NEARCUT/cancel "" "$TOKC0" ""
expect "old-zero-cancel-200" "$(cat "$TMP/status")" "200"
require_clean "cutoff"

echo "== repeat cancel preserves state =="
api POST /_test/reset "$TMP/fixture.json" "" ""
expect "refix-204" "$(cat "$TMP/status")" "204"
api POST /auth/login "$TMP/login-bea" "" ""
cp "$TMP/out" "$OUT/tok-bea2.json"
TOKB2=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-bea2.json"))["token"])' 2>>"$DIAG")
export TOKB2
api POST /auth/login "$TMP/login-ada" "" ""
cp "$TMP/out" "$OUT/tok-ada2.json"
TOKA=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-ada2.json"))["token"])')
export TOKA
create "$TOKB2" "rc-1" '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"'"$DATE"'T19:00","party_size":2}'
expect "rc-create-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/rc.json"
REF_RC=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/rc.json"))["reference"])' 2>>"$DIAG")
export REF_RC
api POST "/reservations/$REF_RC/cancel" "" "$TOKB2" ""
expect "rc-cancel-200" "$(cat "$TMP/status")" "200"
cp "$TMP/out" "$OUT/rc-cancelled.json"
export_state "export-cancelled.json"
api POST "/reservations/$REF_RC/cancel" "" "$TOKB2" ""
expect "rc-recancel-200" "$(cat "$TMP/status")" "200"
pycheck "recancel-bytes" 'import os; assert open(os.environ["OUT"]+"/rc-cancelled.json","rb").read()==open("'"$TMP"'/out","rb").read()'
export_state "export-recancelled.json"
pycheck "recancel-export-bytes" 'import os; assert open(os.environ["OUT"]+"/export-cancelled.json","rb").read()==open(os.environ["OUT"]+"/export-recancelled.json","rb").read()'
require_clean "cancel"

echo "== history and decision privacy =="
api GET "/reservations/$REF_RC/history" "" "$TOKB2" ""
expect "history-cancelled-200" "$(cat "$TMP/status")" "200"
pycheck "history-cancelled-entry" 'import json; d=json.load(open("'"$TMP"'/out")); e=d["entries"]; assert e[-1]["event"]=="cancelled" and e[-1]["changes"]==[] and e[-1]["revision"]==2'
api GET "/reservations/$REF_RC/decision" "" "$TOKB2" ""
expect "decision-cancelled-200" "$(cat "$TMP/status")" "200"
pycheck "decision-shape" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==2 and set(d["accepted_terms"])=={"policy_version","slot_minutes","reservation_duration_minutes","cancellation_cutoff_minutes","opening_hours","capacities"}'
api GET "/reservations/$REF_RC/history" "" "$TOKA" ""
expect "history-foreign-404" "$(cat "$TMP/status")" "404"
api GET "/reservations/$REF_RC/decision" "" "" ""
expect "decision-anon-404" "$(cat "$TMP/status")" "404"
api GET "/reservations/NOPE01/history" "" "$TOKB2" ""
expect "history-unknown-404" "$(cat "$TMP/status")" "404"
api GET "/reservations/NOPE01/decision" "" "$TOKB2" ""
expect "decision-unknown-404" "$(cat "$TMP/status")" "404"
api GET "/reservations/$REF_RC" "" "badtoken" ""
expect "lookup-badtoken-401" "$(cat "$TMP/status")" "401"
require_clean "views"

echo "== series =="
create "$TOKB2" "sx-anchor" '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"'"$DATE"'T19:00","party_size":2}'
expect "anchor-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/anchor.json"
REF_ANCHOR=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/anchor.json"))["reference"])' 2>>"$DIAG")
export REF_ANCHOR
write adopt-bad '{"anchor_reference":"'"$REF_ANCHOR"'","count":99,"interval_weeks":1}'
api POST /series "$TMP/adopt-bad" "$TOKB2" "sx-bad"
expect "adopt-count-422" "$(cat "$TMP/status")" "422"
write adopt '{"anchor_reference":"'"$REF_ANCHOR"'","count":3,"interval_weeks":1}'
api POST /series "$TMP/adopt" "$TOKB2" "sx-01"
expect "adopt-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/adopt.json"
SID=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/adopt.json"))["series_id"])' 2>>"$DIAG")
export SID
pycheck "adopt-calendar" 'import json,os,datetime; d=json.load(open(os.environ["OUT"]+"/adopt.json")); a=datetime.date.fromisoformat(os.environ["DATE"]); occ=d["occurrences"]; assert len(occ)==3 and [o["index"] for o in occ]==[0,1,2] and all(o["exception"] is False for o in occ); assert occ[0]["reference"]==os.environ["REF_ANCHOR"]; assert occ[1]["reservation"]["starts_at_local"]==(a+datetime.timedelta(days=7)).isoformat()+"T19:00"; assert occ[2]["reservation"]["starts_at_local"]==(a+datetime.timedelta(days=14)).isoformat()+"T19:00"; assert d["revision"]==1'
pycheck "anchor-retained" 'import json,os; d=json.load(open(os.environ["OUT"]+"/adopt.json")); before=json.load(open(os.environ["OUT"]+"/anchor.json")); assert d["occurrences"][0]["reservation"]==before'
G1=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/adopt.json"))["occurrences"][1]["reference"])' 2>>"$DIAG")
G2=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/adopt.json"))["occurrences"][2]["reference"])' 2>>"$DIAG")
export G1 G2
api GET "/series/$SID" "" "$TOKB2" ""
expect "series-get-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID" "" "" ""
expect "series-anon-404" "$(cat "$TMP/status")" "404"
api GET "/series/$SID" "" "badtoken" ""
expect "series-badtoken-404" "$(cat "$TMP/status")" "404"
write patch-g1 '{"party_size":1}'
api PATCH "/reservations/$G1" "$TMP/patch-g1" "$TOKB2" ""
expect "occurrence-patch-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID" "" "$TOKB2" ""
pycheck "patch-flag-once" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==2; f=[o["exception"] for o in d["occurrences"]]; assert f==[False,True,False], f'
api POST "/reservations/$G2/cancel" "" "$TOKB2" ""
expect "occurrence-cancel-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID" "" "$TOKB2" ""
pycheck "cancel-keeps-flag" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==3; f=[o["exception"] for o in d["occurrences"]]; assert f==[False,True,False], f'
api POST "/reservations/$G2/cancel" "" "$TOKB2" ""
expect "occurrence-recancel-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID" "" "$TOKB2" ""
pycheck "recancel-stable" 'import json; assert json.load(open("'"$TMP"'/out"))["revision"]==3'
api POST /series "$TMP/adopt" "$TOKB2" "sx-01"
expect "adopt-replay-200" "$(cat "$TMP/status")" "200"
pycheck "adopt-replay-bytes" 'import os; assert open(os.environ["OUT"]+"/adopt.json","rb").read()==open("'"$TMP"'/out","rb").read()'
require_clean "series"


echo "== series failure precedence and rollback =="
D7=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=7)).isoformat())' 2>>"$DIAG")
export D7
create "$TOKB2" "sx-blocker" '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$D7"'T21:00","party_size":2}'
expect "blocker-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/blocker.json"
BLOCKREF=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/blocker.json"))["reference"])' 2>>"$DIAG")
export BLOCKREF
create "$TOKB2" "sx-anchor2" '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$DATE"'T21:00","party_size":2}'
expect "anchor2-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/anchor2.json"
REF_A2=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/anchor2.json"))["reference"])' 2>>"$DIAG")
export REF_A2
write adopt-fail2 '{"anchor_reference":"'"$REF_A2"'","count":2,"interval_weeks":1}'
export_state "sx-fail-pre.json"
api POST /series "$TMP/adopt-fail2" "$TOKB2" "sx-fail"
expect "adopt-index1-409" "$(cat "$TMP/status")" "409"
pycheck "adopt-index1-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="table_unavailable"'
export_state "sx-fail-post.json"
pycheck "adopt-fail-rollback" 'import os; assert open(os.environ["OUT"]+"/sx-fail-pre.json","rb").read()==open(os.environ["OUT"]+"/sx-fail-post.json","rb").read()'
api POST "/reservations/$BLOCKREF/cancel" "" "$TOKB2" ""
expect "blocker-cancel-200" "$(cat "$TMP/status")" "200"
api POST /series "$TMP/adopt-fail2" "$TOKB2" "sx-fail"
expect "adopt-reuse-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/adopt-reused.json"
api POST /series "$TMP/adopt-fail2" "$TOKB2" "sx-fail"
expect "adopt-reuse-replay-200" "$(cat "$TMP/status")" "200"
pycheck "adopt-reuse-bytes" 'import os; assert open(os.environ["OUT"]+"/adopt-reused.json","rb").read()==open("'"$TMP"'/out","rb").read()'
require_clean "series-fail"



echo "== long anchor occupancy =="
write pol120 '{"effective_from":"'"$DATE"'","slot_minutes":30,"reservation_duration_minutes":120,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol120" "$TOKA" "long-120"
expect "long120-publish-201" "$(cat "$TMP/status")" "201"
create "$TOKB2" "sx-long" '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$DATE"'T19:00","party_size":2}'
expect "long-anchor-201" "$(cat "$TMP/status")" "201"
policy "$DATE"
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKA" "long-60"
expect "long60-publish-201" "$(cat "$TMP/status")" "201"
create "$TOKB2" "sx-short2" '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$DATE"'T20:00","party_size":2}'
expect "short-accepted-end-409" "$(cat "$TMP/status")" "409"
pycheck "short-accepted-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="table_unavailable"'

echo "== santiago calendar =="
create "$TOKB2" "sx-san" '{"restaurant_id":"r_san","table_id":"t_1","starts_at_local":"2027-08-29T19:00","party_size":2}'
expect "san-anchor-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/san-anchor.json"
REF_SAN=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/san-anchor.json"))["reference"])' 2>>"$DIAG")
export REF_SAN
write adopt-san '{"anchor_reference":"'"$REF_SAN"'","count":2,"interval_weeks":1}'
api POST /series "$TMP/adopt-san" "$TOKB2" "sx-san"
expect "san-adopt-201" "$(cat "$TMP/status")" "201"
pycheck "san-sep5" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["occurrences"][1]["reservation"]["starts_at_local"]=="2027-09-05T19:00", d["occurrences"]'
require_clean "series-extra"
echo "== series index precedence =="
D21=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=21)).isoformat())' 2>>"$DIAG")
D28=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=28)).isoformat())' 2>>"$DIAG")
D35=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=35)).isoformat())' 2>>"$DIAG")
export D21 D28 D35
pycheck "precedence-thursdays" 'import datetime,os; assert datetime.date.fromisoformat(os.environ["D21"]).strftime("%a")=="Thu"; assert datetime.date.fromisoformat(os.environ["D28"]).strftime("%a")=="Thu"; assert datetime.date.fromisoformat(os.environ["D35"]).strftime("%a")=="Thu"'
write pol30b '{"effective_from":"'"$D21"'","slot_minutes":30,"reservation_duration_minutes":30,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol30b" "$TOKA" "prec-30"
expect "prec-pol30-201" "$(cat "$TMP/status")" "201"
create "$TOKB2" "prec-anchor" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$D21"'T18:30","party_size":2}'
expect "prec-anchor-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/prec-anchor.json"
REF_PREC=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/prec-anchor.json"))["reference"])' 2>>"$DIAG")
export REF_PREC
create "$TOKB2" "prec-block" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$D28"'T18:30","party_size":2}'
expect "prec-block-201" "$(cat "$TMP/status")" "201"
write pol60b '{"effective_from":"'"$D35"'","slot_minutes":60,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol60b" "$TOKA" "prec-60"
expect "prec-pol60-201" "$(cat "$TMP/status")" "201"
export_state "prec-pre.json"
write adopt-prec '{"anchor_reference":"'"$REF_PREC"'","count":3,"interval_weeks":1}'
api POST /series "$TMP/adopt-prec" "$TOKB2" "prec-01"
expect "prec-occupancy-wins-409" "$(cat "$TMP/status")" "409"
pycheck "prec-occupancy-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="table_unavailable"'
export_state "prec-post.json"
pycheck "prec-rollback-bytes" 'import os; assert open(os.environ["OUT"]+"/prec-pre.json","rb").read()==open(os.environ["OUT"]+"/prec-post.json","rb").read()'
require_clean "series-precedence"

echo "== policy versions explicit =="
D42=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=42)).isoformat())' 2>>"$DIAG")
D35B=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=35)).isoformat())' 2>>"$DIAG")
DM7=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])-datetime.timedelta(days=7)).isoformat())' 2>>"$DIAG")
export D42 D35B DM7
pycheck "version-dates-thursday" 'import datetime,os; assert datetime.date.fromisoformat(os.environ["D42"]).strftime("%a")=="Thu"; assert datetime.date.fromisoformat(os.environ["D35B"]).strftime("%a")=="Thu"; assert datetime.date.fromisoformat(os.environ["DM7"]).strftime("%a")=="Thu"'
write pol-ny '{"effective_from":"'"$DATE"'","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4}}'
api POST /restaurants/r_ny/policies "$TMP/pol-ny" "$TOKA" "ny-01"
expect "ny-per-restaurant-201" "$(cat "$TMP/status")" "201"
pycheck "ny-version-1" 'import json; assert json.load(open("'"$TMP"'/out"))["policy_version"]==1'
write pol-42 '{"effective_from":"'"$D42"'","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol-42" "$TOKA" "v42"
expect "v42-publish-201" "$(cat "$TMP/status")" "201"
VA=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/out"))["policy_version"])' 2>>"$DIAG")
export VA
write pol-35 '{"effective_from":"'"$D35B"'","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":2,"t_2":4,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol-35" "$TOKA" "v35"
expect "v35-publish-201" "$(cat "$TMP/status")" "201"
VB=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/out"))["policy_version"])' 2>>"$DIAG")
export VB
create "$TOKB2" "vb-book" '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$D35B"'T18:00","party_size":2}'
expect "vb-create-201" "$(cat "$TMP/status")" "201"
pycheck "vb-selects-earlier-date" 'import json,os; d=json.load(open("'"$TMP"'/out")); assert d["accepted_terms"]["policy_version"]==int(os.environ["VB"]), d["accepted_terms"]'
REF_VB=$(python3 -c 'import json,os; print(json.load(open("'"$TMP"'/out"))["reference"])' 2>>"$DIAG")
export REF_VB
printf '{"party_size":3}' > "$TMP/vb-amend.json"
api PATCH "/reservations/$REF_VB" "$TMP/vb-amend.json" "$TOKB2" ""
expect "vb-amend1-200" "$(cat "$TMP/status")" "200"
write pol-35b '{"effective_from":"'"$D35B"'","slot_minutes":60,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/pol-35b" "$TOKA" "v35b"
expect "v35b-publish-201" "$(cat "$TMP/status")" "201"
VC=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/out"))["policy_version"])' 2>>"$DIAG")
export VC
printf '{"starts_at_local":"'"$D35B"'T19:00"}' > "$TMP/vb-amend2.json"
api PATCH "/reservations/$REF_VB" "$TMP/vb-amend2.json" "$TOKB2" ""
expect "vb-amend2-200" "$(cat "$TMP/status")" "200"
api GET "/reservations/$REF_VB/history" "" "$TOKB2" ""
pycheck "vb-history-frozen" 'import json,os; e=json.load(open("'"$TMP"'/out"))["entries"]; assert len(e)==3; assert e[1]["accepted_terms"]["policy_version"]==int(os.environ["VB"]) and e[1]["accepted_terms"]["reservation_duration_minutes"]==90; assert e[2]["accepted_terms"]["policy_version"]==int(os.environ["VC"]) and e[2]["accepted_terms"]["reservation_duration_minutes"]==60'
expect "before-first-grid" "$(avail r_anker "$DM7" 2 "")" "200"
pycheck "before-first-fixture" 'import json; d=json.load(open("'"$TMP"'/avail.json")); assert len(d["slots"])==8 and d["slots"][0]["starts_at_local"].endswith("T18:00")'
require_clean "versions"

echo "== closed day independence =="
write wedpol '{"effective_from":"2027-06-16","slot_minutes":60,"reservation_duration_minutes":60,"cancellation_cutoff_minutes":60,"opening_hours":[{"weekday":"wed","opens":"18:00","closes":"23:00"},{"weekday":"sun","opens":"18:00","closes":"23:00"}],"capacities":{"t_1":4,"t_2":6,"t_3":4}}'
api POST /restaurants/r_anker/policies "$TMP/wedpol" "$TOKA" "wed-01"
expect "wed-publish-201" "$(cat "$TMP/status")" "201"
expect "wed-now-open" "$(avail r_anker 2027-06-16 2 "")" "200"
pycheck "wed-nonempty" 'import json; d=json.load(open("'"$TMP"'/avail.json")); assert len(d["slots"])==5 and len(d["slots"][0]["available_table_ids"])==3, d["slots"]'
expect "tue-still-closed" "$(avail r_anker 2027-06-15 2 "")" "200"
pycheck "tue-empty" 'import json; assert json.load(open("'"$TMP"'/avail.json"))["slots"]==[]'
require_clean "closed-day"

echo "== collective moves =="
api GET /restaurants/r_anker/policies "" "" ""
EXPECT_V=$(python3 -c 'import json; print(max(p["policy_version"] for p in json.load(open("'"$TMP"'/out"))["policies"]))' 2>>"$DIAG")
export EXPECT_V
create "$TOKB2" "cb-a" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$DATE"'T19:00","party_size":4}'
expect "cb-create-a-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/cb-a.json"
CB_A=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/cb-a.json"))["reference"])' 2>>"$DIAG")
export CB_A
create "$TOKB2" "cb-p" '{"restaurant_id":"r_anker","table_ids":["t_1","t_2"],"starts_at_local":"'"$DATE"'T21:00","party_size":8}'
expect "cb-create-p-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/cb-p.json"
CB_P=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/cb-p.json"))["reference"])' 2>>"$DIAG")
export CB_P
batch() {
  printf '%s' "$3" > "$TMP/batch-body.json"
  api POST /reservation-moves "$TMP/batch-body.json" "$1" "$2"
}
export_state "export-pre-batch.json"
batch "$TOKB2" "mb-01" '{"moves":[{"reference":"'"$CB_A"'","party_size":5},{"reference":"'"$CB_P"'"}]}'
expect "batch-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/batch.json"
pycheck "batch-order-rev" 'import json,os; d=json.load(open("'"$TMP"'/out")); r=d["reservations"]; assert [x["reference"] for x in r]==[os.environ["CB_A"],os.environ["CB_P"]]; assert r[0]["revision"]==2 and r[1]["revision"]==1'
export_state "export-post-batch.json"
pycheck "batch-counter-once" 'import json,os; a=json.load(open(os.environ["OUT"]+"/export-pre-batch.json")); b=json.load(open(os.environ["OUT"]+"/export-post-batch.json")); assert b["state"]["restaurant_revisions"]["r_anker"]==a["state"]["restaurant_revisions"]["r_anker"]+1'
pycheck "batch-histories-once" 'import json,os; b=json.load(open(os.environ["OUT"]+"/export-post-batch.json")); h=b["state"]["histories"]; assert len(h[os.environ["CB_A"]])==2 and len(h[os.environ["CB_P"]])==1, {k: len(v) for k, v in h.items()}'
batch "$TOKB2" "mb-01" '{"moves":[{"reference":"'"$CB_A"'","party_size":5},{"reference":"'"$CB_P"'"}]}'
expect "batch-replay-200" "$(cat "$TMP/status")" "200"
pycheck "batch-replay-bytes" 'import os; assert open(os.environ["OUT"]+"/batch.json","rb").read()==open("'"$TMP"'/out","rb").read()'
batch "$TOKB2" "mb-bad" '{"moves":[{"reference":"'"$CB_A"'","expected_revision":1,"table_id":"t_nope"}]}'
expect "batch-stale-first-409" "$(cat "$TMP/status")" "409"
batch "$TOKB2" "mb-big" '{"moves":[{"reference":"'"$CB_A"'","expected_revision":9000000000000001}]}'
expect "batch-huge-stale-409" "$(cat "$TMP/status")" "409"

echo "== mixed-series collective batch =="
create "$TOKB2" "mx-fa" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$D28"'T18:00","party_size":2}'
expect "mx-anchorF-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/mx-fa.json"
REF_MF=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/mx-fa.json"))["reference"])' 2>>"$DIAG")
export REF_MF
write adopt-mf '{"anchor_reference":"'"$REF_MF"'","count":2,"interval_weeks":1}'
api POST /series "$TMP/adopt-mf" "$TOKB2" "mx-sf"
expect "mx-adoptF-201" "$(cat "$TMP/status")" "201"
SID_F=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/out"))["series_id"])' 2>>"$DIAG")
export SID_F
create "$TOKB2" "mx-ha" '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"'"$D28"'T18:00","party_size":2}'
expect "mx-anchorH-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/mx-ha.json"
REF_MH=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/mx-ha.json"))["reference"])' 2>>"$DIAG")
export REF_MH
write adopt-mh '{"anchor_reference":"'"$REF_MH"'","count":2,"interval_weeks":1}'
api POST /series "$TMP/adopt-mh" "$TOKB2" "mx-sh"
expect "mx-adoptH-201" "$(cat "$TMP/status")" "201"
SID_H=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/out"))["series_id"])' 2>>"$DIAG")
export SID_H
OCCA2=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/adopt-reused.json"))["occurrences"][1]["reference"])' 2>>"$DIAG")
export OCCA2
SID_A2=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/adopt-reused.json"))["series_id"])' 2>>"$DIAG")
export SID_A2
api GET "/series/$SID" "" "$TOKB2" ""
cp "$TMP/out" "$OUT/mx-s1-before.json"
export_state "mx-pre.json"
batch "$TOKB2" "mx-01" '{"moves":[{"reference":"'"$G1"'","party_size":2},{"reference":"'"$OCCA2"'","party_size":1}]}'
expect "mx-batch-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/mx-batch.json"
cp "$TMP/batch-body.json" "$OUT/mx-01-body.json"
export_state "mx-post.json"
pycheck "mx-counter-once" 'import json,os; a=json.load(open(os.environ["OUT"]+"/mx-pre.json")); b=json.load(open(os.environ["OUT"]+"/mx-post.json")); assert b["state"]["restaurant_revisions"]["r_anker"]==a["state"]["restaurant_revisions"]["r_anker"]+1'
pycheck "mx-series-once" 'import json,os; b=json.load(open(os.environ["OUT"]+"/mx-post.json"))["state"]["series"]; before=json.load(open(os.environ["OUT"]+"/mx-s1-before.json")); s1=os.environ["SID"]; sa=os.environ["SID_A2"]; assert b[s1]["revision"]==before["revision"]+1 and b[sa]["revision"]==2'
pycheck "mx-flags" 'import json,os; b=json.load(open(os.environ["OUT"]+"/mx-post.json"))["state"]["series"]; s1=os.environ["SID"]; sa=os.environ["SID_A2"]; g1=os.environ["G1"]; occ=os.environ["OCCA2"]; f1={m["reference"]: m["exception"] for m in b[s1]["members"]}; fa={m["reference"]: m["exception"] for m in b[sa]["members"]}; assert f1[g1] is True and fa[occ] is True'
pycheck "mx-member-state" 'import json,os; b=json.load(open(os.environ["OUT"]+"/mx-post.json"))["state"]; g1=os.environ["G1"]; occ=os.environ["OCCA2"]; assert b["reservations"][g1]["revision"]==3 and b["reservations"][occ]["revision"]==2; assert len(b["histories"][g1])==3 and len(b["histories"][occ])==2'
pycheck "mx-third-untouched" 'import json,os; b=json.load(open(os.environ["OUT"]+"/mx-post.json"))["state"]["series"]; h=os.environ["SID_H"]; assert b[h]["revision"]==1 and all(m["exception"] is False for m in b[h]["members"])'
api PATCH "/reservations/$REF_MF" "$TMP/patch-g1" "$TOKB2" ""
expect "mx-flag-patch-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID_F" "" "$TOKB2" ""
pycheck "mx-flag-true" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==2 and d["occurrences"][0]["exception"] is True'
api POST "/reservations/$REF_MF/cancel" "" "$TOKB2" ""
expect "mx-flag-cancel-200" "$(cat "$TMP/status")" "200"
api GET "/series/$SID_F" "" "$TOKB2" ""
pycheck "mx-flag-retained" 'import json; d=json.load(open("'"$TMP"'/out")); assert d["revision"]==3 and d["occurrences"][0]["exception"] is True and d["occurrences"][0]["reservation"]["status"]=="cancelled"'
export_state "mx-fail-pre.json"
batch "$TOKB2" "mx-bad" '{"moves":[{"reference":"'"$REF_MF"'","party_size":2},{"reference":"'"$OCCA2"'","table_id":"t_1"}]}'
expect "mx-fail-409" "$(cat "$TMP/status")" "409"
pycheck "mx-fail-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="reservation_cancelled"'
export_state "mx-fail-post.json"
pycheck "mx-fail-rollback" 'import os; assert open(os.environ["OUT"]+"/mx-fail-pre.json","rb").read()==open(os.environ["OUT"]+"/mx-fail-post.json","rb").read()'
batch "$TOKB2" "mx-bad" '{"moves":[{"reference":"'"$OCCA2"'","party_size":2}]}'
expect "mx-reuse-201" "$(cat "$TMP/status")" "201"
export_state "mx-reuse-post.json"
batch "$TOKB2" "mx-noop" '{"moves":[{"reference":"'"$REF_MH"'"}]}'
expect "mx-noop-201" "$(cat "$TMP/status")" "201"
export_state "mx-noop-post.json"
pycheck "mx-noop-zero" 'import json,os; a=json.load(open(os.environ["OUT"]+"/mx-reuse-post.json")); b=json.load(open(os.environ["OUT"]+"/mx-noop-post.json")); sa={k: v for k, v in a["state"].items() if k != "receipts"}; sb={k: v for k, v in b["state"].items() if k != "receipts"}; assert sa==sb'
api POST /reservation-moves "$OUT/mx-01-body.json" "$TOKB2" "mx-01"
expect "mx-replay-200" "$(cat "$TMP/status")" "200"
pycheck "mx-replay-bytes" 'import os; assert open(os.environ["OUT"]+"/mx-batch.json","rb").read()==open("'"$TMP"'/out","rb").read()'
export_state "mx-replay-post.json"
pycheck "mx-replay-export-same" 'import os; assert open(os.environ["OUT"]+"/mx-noop-post.json","rb").read()==open(os.environ["OUT"]+"/mx-replay-post.json","rb").read()'
require_clean "collective"

echo "== races and scopes =="
create "$TOKB2" "comp-1" '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"'"$DATE"'T22:00","party_size":4}'
expect "comp-create-201" "$(cat "$TMP/status")" "201"
cp "$TMP/out" "$OUT/comp.json"
REF_COMP=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/comp.json"))["reference"])' 2>>"$DIAG")
export REF_COMP
export_state "comp-pre.json"
printf '{"party_size":1,"expected_revision":1}' > "$TMP/comp-a.json"
printf '{"party_size":2,"expected_revision":1}' > "$TMP/comp-b.json"
( $CURL -s --connect-timeout 5 --max-time 25 -o "$TMP/comp-a.out" -w '%{http_code}' -X PATCH "$BASE/reservations/$REF_COMP" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKB2" --data-binary @"$TMP/comp-a.json" > "$TMP/comp-a.code" 2>>"$DIAG" ) &
( $CURL -s --connect-timeout 5 --max-time 25 -o "$TMP/comp-b.out" -w '%{http_code}' -X PATCH "$BASE/reservations/$REF_COMP" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKB2" --data-binary @"$TMP/comp-b.json" > "$TMP/comp-b.code" 2>>"$DIAG" ) &
wait
ca=$(cat "$TMP/comp-a.code")
cb=$(cat "$TMP/comp-b.code")
if { [ "$ca" = "200" ] && [ "$cb" = "409" ]; } || { [ "$ca" = "409" ] && [ "$cb" = "200" ]; }; then
  echo "PASS: competing-expected-one-each"
  PASS=$((PASS + 1))
else
  echo "FAIL: competing-expected ($ca/$cb)"
  FAIL=$((FAIL + 1))
fi
if [ "$ca" = "409" ]; then
  loser="$TMP/comp-a.out"
else
  loser="$TMP/comp-b.out"
fi
pycheck "competing-stale-code" 'import json; assert json.load(open("'"$loser"'"))["error"]["code"]=="stale_revision"'
api GET "/reservations/$REF_COMP/history" "" "$TOKB2" ""
pycheck "comp-history-once" 'import json; assert len(json.load(open("'"$TMP"'/out"))["entries"])==2'
if [ "$ca" = "200" ]; then cp "$TMP/comp-a.out" "$OUT/comp-winner.json"; else cp "$TMP/comp-b.out" "$OUT/comp-winner.json"; fi
export_state "comp-post.json"
pycheck "comp-race-counter-once" 'import json,os; a=json.load(open(os.environ["OUT"]+"/comp-pre.json")); b=json.load(open(os.environ["OUT"]+"/comp-post.json")); assert b["state"]["restaurant_revisions"]["r_anker"]==a["state"]["restaurant_revisions"]["r_anker"]+1, (a["state"]["restaurant_revisions"], b["state"]["restaurant_revisions"])'
pycheck "comp-race-target-once" 'import json,os; OUT=os.environ["OUT"]; r=os.environ["REF_COMP"]; a=json.load(open(OUT+"/comp-pre.json"))["state"]; b=json.load(open(OUT+"/comp-post.json"))["state"]; w=json.load(open(OUT+"/comp-winner.json")); pre=a["reservations"][r]; post=b["reservations"][r]; assert post["revision"]==pre["revision"]+1; assert post["reservation_id"]==pre["reservation_id"] and post["user_id"]==pre["user_id"] and post["reference"]==pre["reference"]; assert b["histories"][r][:-1]==a["histories"][r] and len(b["histories"][r])==len(a["histories"][r])+1; e=b["histories"][r][-1]; assert e["event"]=="changed" and e["revision"]==post["revision"] and len(e["changes"])==1; c=e["changes"][0]; assert c["field"]=="party_size" and c["from"]==pre["party_size"] and c["to"]==w["party_size"]; assert e["accepted_terms"]==post["accepted_terms"]; single=(len(post.get("table_ids",[]))==1); assert ("table_id" in w)==single; proj={k: v for k, v in post.items() if k!="user_id" and (k!="table_id" or single)}; assert proj==w'
pycheck "comp-race-others-unchanged" 'import json,os; a=json.load(open(os.environ["OUT"]+"/comp-pre.json"))["state"]; b=json.load(open(os.environ["OUT"]+"/comp-post.json"))["state"]; r=os.environ["REF_COMP"]; assert set(b["reservations"])==set(a["reservations"]); assert all(b["reservations"][k]==a["reservations"][k] for k in a["reservations"] if k!=r); assert set(b["histories"])==set(a["histories"]); assert all(b["histories"][k]==a["histories"][k] for k in a["histories"] if k!=r); assert b["series"]==a["series"] and b["policies"]==a["policies"] and b["users"]==a["users"] and b["tokens"]==a["tokens"] and b["receipts"]==a["receipts"] and b["restaurants"]==a["restaurants"]; exp=dict(a["restaurant_revisions"]); exp["r_anker"]=exp["r_anker"]+1; assert b["restaurant_revisions"]==exp'
printf '{"restaurant_id":"r_anker","table_id":"t_3","starts_at_local":"'"$DATE"'T18:00","party_size":2}' > "$TMP/race-body.json"
export_state "race50-pre.json"
i=0
while [ "$i" -lt 50 ]; do
  ( $CURL -s --connect-timeout 5 --max-time 25 -o "$TMP/race-$i.out" -w '%{http_code}' -X POST "$BASE/reservations" -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKB2" -H 'Idempotency-Key: race-50' --data-binary @"$TMP/race-body.json" > "$TMP/race-$i.code" 2>>"$DIAG" ) &
  i=$((i + 1))
done
wait
ones=0
zeros=0
i=0
while [ "$i" -lt 50 ]; do
  case "$(cat "$TMP/race-$i.code")" in
    201) ones=$((ones + 1)) ;;
    200) zeros=$((zeros + 1)) ;;
  esac
  i=$((i + 1))
done
if [ "$ones" = "1" ] && [ "$zeros" = "49" ]; then
  echo "PASS: race-50-1x201-49x200"
  PASS=$((PASS + 1))
else
  echo "FAIL: race-50 (201=$ones 200=$zeros)"
  FAIL=$((FAIL + 1))
fi
same=1
i=0
while [ "$i" -lt 50 ]; do
  cmp -s "$TMP/race-$i.out" "$TMP/race-0.out" || same=0
  i=$((i + 1))
done
if [ "$same" = "1" ]; then
  echo "PASS: race-50-bytes"
  PASS=$((PASS + 1))
else
  echo "FAIL: race-50-bytes differ"
  FAIL=$((FAIL + 1))
fi
export_state "race50-post.json"
pycheck "race50-counter-once" 'import json,os; a=json.load(open(os.environ["OUT"]+"/race50-pre.json")); b=json.load(open(os.environ["OUT"]+"/race50-post.json")); assert b["state"]["restaurant_revisions"]["r_anker"]==a["state"]["restaurant_revisions"]["r_anker"]+1, (a["state"]["restaurant_revisions"], b["state"]["restaurant_revisions"])'
pycheck "race50-one-record" 'import json,os; OUT=os.environ["OUT"]; a=json.load(open(OUT+"/race50-pre.json"))["state"]; b=json.load(open(OUT+"/race50-post.json"))["state"]; new=[k for k in b["reservations"] if k not in a["reservations"]]; assert len(new)==1, new; r=new[0]; rec=b["reservations"][r]; w=json.load(open("'"$TMP"'/race-0.out")); req=json.load(open("'"$TMP"'/race-body.json")); single=(len(rec.get("table_ids",[]))==1); assert ("table_id" in w)==single; proj={k: v for k, v in rec.items() if k!="user_id" and (k!="table_id" or single)}; assert proj==w; assert rec["revision"]==1 and rec["status"]=="confirmed"; assert rec["restaurant_id"]==req["restaurant_id"] and rec["starts_at_local"]==req["starts_at_local"] and rec["party_size"]==req["party_size"]; h=b["histories"][r]; assert len(h)==1 and h[0]["event"]=="created" and h[0]["revision"]==1 and [c["field"] for c in h[0]["changes"]]==["table_id","starts_at_local","party_size"] and all(c["from"] is None for c in h[0]["changes"]); tos={c["field"]: c["to"] for c in h[0]["changes"]}; assert tos["table_id"]==req["table_id"] and tos["starts_at_local"]==req["starts_at_local"] and tos["party_size"]==req["party_size"]; assert h[0]["accepted_terms"]==rec["accepted_terms"]; nr=[v for v in b["receipts"].values() if v["key"]=="race-50" and v["method"]=="POST" and v["path"]=="/reservations"]; assert len(nr)==1, len(nr); assert nr[0]["user_id"]==rec["user_id"] and nr[0]["status"]==201; assert json.loads(nr[0]["body"])==req and json.loads(nr[0]["response"])==w; rb=nr[0]["response"]; ob=open("'"$TMP"'/race-0.out","rb").read().decode(); assert rb==ob or rb+"\n"==ob or rb==ob+"\n"'
pycheck "race50-others-unchanged" 'import json,os; a=json.load(open(os.environ["OUT"]+"/race50-pre.json"))["state"]; b=json.load(open(os.environ["OUT"]+"/race50-post.json"))["state"]; new=[k for k in b["reservations"] if k not in a["reservations"]]; assert len(new)==1; r=new[0]; assert all(b["reservations"][k]==a["reservations"][k] for k in a["reservations"]); assert set(b["histories"])==set(a["histories"])|{r}; assert all(b["histories"][k]==a["histories"][k] for k in a["histories"]); assert b["series"]==a["series"] and b["policies"]==a["policies"] and b["users"]==a["users"] and b["tokens"]==a["tokens"] and b["restaurants"]==a["restaurants"]; assert len([k for k in b["receipts"] if k not in a["receipts"]])==1 and all(b["receipts"][k]==a["receipts"][k] for k in a["receipts"]); exp=dict(a["restaurant_revisions"]); exp["r_anker"]=exp["r_anker"]+1; assert b["restaurant_revisions"]==exp'
create "$TOKB2" "scope-a" '{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"'"$DATE"'T20:00","party_size":2}'
expect "scope-create-201" "$(cat "$TMP/status")" "201"
create "$TOKA" "scope-a" '{"restaurant_id":"r_anker","table_id":"t_1","starts_at_local":"'"$DATE"'T18:00","party_size":2}'
expect "scope-user-201" "$(cat "$TMP/status")" "201"
batch "$TOKB2" "scope-a" '{"moves":[{"reference":"'"$CB_A"'"}]}'
expect "scope-path-201" "$(cat "$TMP/status")" "201"
api POST /reservations "$TMP/race-body.json" "$TOKB2" "race-50"
expect "scope-replay-200" "$(cat "$TMP/status")" "200"
require_clean "races"



echo "== same-image export/import smoke =="
export_state "export-final.json"
pycheck "export-envelope" 'import json,os; d=json.load(open(os.environ["OUT"]+"/export-final.json")); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)'
api POST /_test/import "$OUT/export-final.json" "" ""
expect "same-image-import-204" "$(cat "$TMP/status")" "204"
api GET "/reservations/$CB_A" "" "$TOKB2" ""
expect "session-survives-200" "$(cat "$TMP/status")" "200"
api POST /reservations "$TMP/race-body.json" "$TOKB2" "race-50"
expect "receipt-survives-200" "$(cat "$TMP/status")" "200"
write badimport '{"track":"nope"}'
api POST /_test/import "$TMP/badimport" "" ""
expect "bad-import-422" "$(cat "$TMP/status")" "422"
require_clean "import-smoke"

echo "== week-long accepted duration blocks adoption =="
python3 <<'PYEOF' > "$TMP/longseed.json" 2>>"$DIAG"
import json, os
print(json.dumps({
  "users": [{"id": "u_ada", "email": "ada@example.com", "password": "correct horse", "display_name": "Ada"}],
  "restaurants": [{
    "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
    "slot_minutes": 30, "reservation_duration_minutes": 20160,
    "cancellation_cutoff_minutes": 120,
    "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
    "tables": [{"id": "t_1", "label": "1", "capacity": 2},
               {"id": "t_2", "label": "2", "capacity": 4},
               {"id": "t_3", "label": "3", "capacity": 4}],
    "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
    "manager_user_ids": ["u_ada"]}],
  "reservations": [
    {"id": "seed-long", "reference": "SEEDLONG", "user_id": "u_ada",
     "restaurant_id": "r_anker", "table_id": "t_1",
     "starts_at_local": os.environ["DATE"] + "T19:00", "party_size": 2}]}))
PYEOF
api POST /_test/reset "$TMP/longseed.json" "" ""
expect "longseed-reset-204" "$(cat "$TMP/status")" "204"
api POST /auth/login "$TMP/login-ada" "" ""
cp "$TMP/out" "$OUT/tok-long.json"
TOKLONG=$(python3 -c 'import json,os; print(json.load(open(os.environ["OUT"]+"/tok-long.json"))["token"])' 2>>"$DIAG")
export TOKLONG
api POST /restaurants/r_anker/policies "$TMP/policy.json" "$TOKLONG" "long-short"
expect "long-short-201" "$(cat "$TMP/status")" "201"
export_state "long-pre.json"
write adopt-long '{"anchor_reference":"SEEDLONG","count":2,"interval_weeks":1}'
api POST /series "$TMP/adopt-long" "$TOKLONG" "long-adopt"
expect "long-adopt-409" "$(cat "$TMP/status")" "409"
pycheck "long-adopt-code" 'import json; assert json.load(open("'"$TMP"'/out"))["error"]["code"]=="table_unavailable"'
export_state "long-post.json"
pycheck "long-adopt-rollback" 'import os; assert open(os.environ["OUT"]+"/long-pre.json","rb").read()==open(os.environ["OUT"]+"/long-post.json","rb").read()'
require_clean "long-duration"

echo ""
echo "probe checks passed: $PASS failed: $FAIL skipped: $SKIP"
if [ "$FAIL" != 0 ]; then
  exit 1
fi
