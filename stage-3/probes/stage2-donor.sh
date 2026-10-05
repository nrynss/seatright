#!/bin/sh
# stage2-donor.sh - generate an accepted-stage-1 upgrade donor for stage-2 work.
#
# Usage: sh stage-2/probes/stage2-donor.sh BASE_URL OUT_DIR
#
# Builds no images and changes no source: it drives an already-running
# accepted stage-1 container (see acceptance commands) through fixture reset,
# logins, bookings, an atomic swap batch, an amendment, a cancellation, a
# failed reusable key and receipt replays, then writes private artifacts:
#   OUT_DIR/export.json    atomic source export (snapshot)
#   OUT_DIR/manifest.json  donor manifest (fixture, users, tokens, receipts
#                          with user/method/path, full current records,
#                          failed keys, pending retry identity, provenance)
# Only OUT_DIR is written. Every check counts through expect()/pyassert(),
# which record FAIL and drive a nonzero exit; phase gates abort nonzero on
# any accumulated failure. Stdout carries PASS/FAIL lines and counts, never
# secrets or export bodies.
#
# Fixture seed passwords default below and may be overridden with
# ADA_PASSWORD / BEA_PASSWORD (serialized safely as JSON, never interpolated
# into shell). SRC_IMAGE / SRC_CONTAINER / SRC_PORT record the actual source
# deployment for the manifest (defaults match the acceptance commands).
set -u

BASE=${1:?usage: sh stage-2/probes/stage2-donor.sh BASE_URL OUT_DIR}
OUT=${2:?usage: sh stage-2/probes/stage2-donor.sh BASE_URL OUT_DIR}
ADA_PASSWORD=${ADA_PASSWORD:-correct horse ada}
BEA_PASSWORD=${BEA_PASSWORD:-correct horse bea}
SRC_IMAGE=${SRC_IMAGE:-tablekeeper:s2-donor-stage1}
SRC_CONTAINER=${SRC_CONTAINER:-tk-s2-donor}
SRC_PORT=${SRC_PORT:-9021}

DATE=2027-06-17
PASS=0
FAIL=0
TMP=""

cleanup() {
  if [ -n "$TMP" ]; then
    rm -rf "$TMP"
  fi
}

expect() {
  if [ "$2" = "$3" ]; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (got $2, want $3)"
    FAIL=$((FAIL + 1))
  fi
}

pyassert() {
  if python3 -c "$2"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (python check failed)"
    FAIL=$((FAIL + 1))
  fi
}

require_clean() {
  if [ "$FAIL" != 0 ]; then
    echo "ABORT: $1 already failed ($FAIL failures)"
    exit 1
  fi
}

api_post() {
  curl -s -o "$3" -w '%{http_code}' -X POST "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4" -H "Idempotency-Key: $5"
}

api_post_nokey() {
  curl -s -o "$3" -w '%{http_code}' -X POST "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4"
}

api_patch() {
  curl -s -o "$3" -w '%{http_code}' -X PATCH "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4"
}

api_get() {
  if [ -n "$3" ]; then
    curl -s -o "$2" -w '%{http_code}' "$BASE$1" -H "Authorization: Bearer $3"
  else
    curl -s -o "$2" -w '%{http_code}' "$BASE$1"
  fi
}

jget() {
  python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d'"$2"')' "$1"
}

mkdir -p "$OUT"
TMP=$(mktemp -d)
trap cleanup EXIT INT TERM

echo "donor date: $DATE"
pyassert "donor date is Thursday" 'import datetime; assert datetime.date(2027,6,17).strftime("%a")=="Thu"'

# 1. Fixture assembled as JSON (passwords cannot break the encoding).
export ADA_PASSWORD BEA_PASSWORD
python3 <<'PYEOF' > "$TMP/fixture.json"
import json, os
print(json.dumps({
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": os.environ["ADA_PASSWORD"], "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": os.environ["BEA_PASSWORD"], "display_name": "Bea"}],
  "restaurants": [{
    "id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
    "slot_minutes": 30, "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
    "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                      {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
    "tables": [{"id": "t_1", "label": "1", "capacity": 2},
               {"id": "t_2", "label": "2", "capacity": 4},
               {"id": "t_3", "label": "3", "capacity": 4}]}],
  "reservations": []}))
PYEOF
expect "reset fixture" "$(api_post /_test/reset "$TMP/fixture.json" "$TMP/reset.out" "" "-")" 204
require_clean "fixture reset"

# 2. Logins: two Ada tokens, two Bea tokens (hash login + multi-session).
login_body() {
  python3 -c 'import json,sys; print(json.dumps({"email":sys.argv[1],"password":sys.argv[2]}))' "$1" "$2" > "$3"
}
login_body ada@example.com "$ADA_PASSWORD" "$TMP/login-ada.json"
login_body bea@example.com "$BEA_PASSWORD" "$TMP/login-bea.json"
expect "ada login 1" "$(api_post /auth/login "$TMP/login-ada.json" "$TMP/tok-a1.json" "" "-")" 200
expect "ada login 2" "$(api_post /auth/login "$TMP/login-ada.json" "$TMP/tok-a2.json" "" "-")" 200
expect "bea login 1" "$(api_post /auth/login "$TMP/login-bea.json" "$TMP/tok-b1.json" "" "-")" 200
expect "bea login 2" "$(api_post /auth/login "$TMP/login-bea.json" "$TMP/tok-b2.json" "" "-")" 200
require_clean "logins"
TOK_A1=$(jget "$TMP/tok-a1.json" "['token']")
TOK_A2=$(jget "$TMP/tok-a2.json" "['token']")
TOK_B1=$(jget "$TMP/tok-b1.json" "['token']")
TOK_B2=$(jget "$TMP/tok-b2.json" "['token']")
if [ "$TOK_A1" = "$TOK_A2" ] || [ "$TOK_B1" = "$TOK_B2" ]; then
  echo "FAIL: concurrent sessions share a token"
  FAIL=$((FAIL + 1))
else
  echo "PASS: distinct session tokens"
  PASS=$((PASS + 1))
fi

# 3. Lost-response booking (Ada, stays confirmed for UI recovery).
LOST_KEY=donor-lost-01
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-06-17T18:00\",\"party_size\":2}")' > "$TMP/lost-body.json"
expect "lost booking create" "$(api_post /reservations "$TMP/lost-body.json" "$TMP/lost-resp.json" "$TOK_A1" "$LOST_KEY")" 201
require_clean "lost booking"
LOST_REF=$(jget "$TMP/lost-resp.json" "['reference']")

# 4. Swap pair A (t_1 19:30) and B (t_2 19:30), then atomic batch A<->B.
A_KEY=donor-create-a
B_KEY=donor-create-b
BATCH_KEY=donor-batch-01
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-06-17T19:30\",\"party_size\":2}")' > "$TMP/a-body.json"
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_2\",\"starts_at_local\":\"2027-06-17T19:30\",\"party_size\":2}")' > "$TMP/b-body.json"
expect "create A" "$(api_post /reservations "$TMP/a-body.json" "$TMP/a-resp.json" "$TOK_A1" "$A_KEY")" 201
expect "create B" "$(api_post /reservations "$TMP/b-body.json" "$TMP/b-resp.json" "$TOK_A1" "$B_KEY")" 201
require_clean "swap pair creation"
REF_A=$(jget "$TMP/a-resp.json" "['reference']")
REF_B=$(jget "$TMP/b-resp.json" "['reference']")
python3 -c 'import json,sys; print(json.dumps({"moves":[{"reference":sys.argv[1],"table_id":"t_2"},{"reference":sys.argv[2],"table_id":"t_1"}]}))' "$REF_A" "$REF_B" > "$TMP/batch-body.json"
expect "swap batch" "$(api_post /reservation-moves "$TMP/batch-body.json" "$TMP/batch-resp.json" "$TOK_A1" "$BATCH_KEY")" 201
require_clean "swap batch"

# 5. Amend A (party 1) and cancel B, so original receipts differ from state
#    while the lost booking stays confirmed.
echo '{"party_size":1}' > "$TMP/amend-a.json"
expect "amend A" "$(api_patch /reservations/"$REF_A" "$TMP/amend-a.json" "$TMP/amend-a-resp.json" "$TOK_A1")" 200
: > "$TMP/empty.json"
expect "cancel B" "$(api_post_nokey /reservations/"$REF_B"/cancel "$TMP/empty.json" "$TMP/cancel-b-resp.json" "$TOK_A1")" 200

# 6. Failed key (reused successfully) plus a permanently failed key.
FAIL_KEY=donor-fail-01
STUCK_KEY=donor-fail-02
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-06-17T18:00\",\"party_size\":99}")' > "$TMP/fail-body.json"
expect "failed create 422" "$(api_post /reservations "$TMP/fail-body.json" "$TMP/fail-resp.json" "$TOK_B1" "$FAIL_KEY")" 422
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\"2027-06-17T20:30\",\"party_size\":2}")' > "$TMP/fail-reuse-body.json"
expect "failed key reused 201" "$(api_post /reservations "$TMP/fail-reuse-body.json" "$TMP/fail-reuse-resp.json" "$TOK_B1" "$FAIL_KEY")" 201
REF_REUSE=$(jget "$TMP/fail-reuse-resp.json" "['reference']")
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\"2027-06-17T18:00\",\"party_size\":\"many\"}")' > "$TMP/stuck-body.json"
expect "stuck key stays failed" "$(api_post /reservations "$TMP/stuck-body.json" "$TMP/stuck-resp.json" "$TOK_B1" "$STUCK_KEY")" 422

# 7. Replay every completed receipt: 200 with byte-identical original JSON.
replay_check() {
  name=$1; key=$2; bodyfile=$3; origfile=$4; path=$5; token=$6
  st=$(api_post "$path" "$bodyfile" "$TMP/replay-$name.json" "$token" "$key")
  if [ "$st" != 200 ]; then
    echo "FAIL: replay $name status (got $st, want 200)"
    FAIL=$((FAIL + 1))
    return
  fi
  if cmp -s "$origfile" "$TMP/replay-$name.json"; then
    echo "PASS: replay $name identical"
    PASS=$((PASS + 1))
  else
    echo "FAIL: replay $name body differs"
    FAIL=$((FAIL + 1))
  fi
}
replay_check lost "$LOST_KEY" "$TMP/lost-body.json" "$TMP/lost-resp.json" /reservations "$TOK_A1"
replay_check create-a "$A_KEY" "$TMP/a-body.json" "$TMP/a-resp.json" /reservations "$TOK_A1"
replay_check create-b "$B_KEY" "$TMP/b-body.json" "$TMP/b-resp.json" /reservations "$TOK_A1"
replay_check batch "$BATCH_KEY" "$TMP/batch-body.json" "$TMP/batch-resp.json" /reservation-moves "$TOK_A1"
replay_check fail-reuse "$FAIL_KEY" "$TMP/fail-reuse-body.json" "$TMP/fail-reuse-resp.json" /reservations "$TOK_B1"

# 8. Current-state reads: lost confirmed; A amended; B cancelled.
expect "lost lookup" "$(api_get /reservations/"$LOST_REF" "$TMP/lookup-lost.json" "$TOK_A1")" 200
pyassert "lost status confirmed" 'import json; d=json.load(open("'"$TMP"'/lookup-lost.json")); assert d["status"]=="confirmed"'
expect "A lookup" "$(api_get /reservations/"$REF_A" "$TMP/lookup-a.json" "$TOK_A1")" 200
pyassert "A current values" 'import json; d=json.load(open("'"$TMP"'/lookup-a.json")); assert d["party_size"]==1 and d["table_id"]=="t_2", d'
expect "B lookup" "$(api_get /reservations/"$REF_B" "$TMP/lookup-b.json" "$TOK_A1")" 200
pyassert "B current cancelled" 'import json; d=json.load(open("'"$TMP"'/lookup-b.json")); assert d["status"]=="cancelled", d'
expect "all tokens valid" "$(api_get /reservations "$TMP/list-a1.json" "$TOK_A1")" 200
expect "second ada token valid" "$(api_get /reservations "$TMP/list-a2.json" "$TOK_A2")" 200
expect "bea token valid" "$(api_get /reservations "$TMP/list-b1.json" "$TOK_B1")" 200
expect "second bea token valid" "$(api_get /reservations "$TMP/list-b2.json" "$TOK_B2")" 200

# 9. Atomic export snapshot; later write must not appear in the saved file.
expect "export" "$(api_get /_test/export "$OUT/export.json" "")" 200
require_clean "export"
export OUT
pyassert "export envelope" 'import json,os; d=json.load(open(os.environ["OUT"]+"/export.json")); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)'
export STUCK_KEY
pyassert "stuck key absent from receipts" 'import json,os; d=json.load(open(os.environ["OUT"]+"/export.json")); ks=[r["key"] for r in d["state"]["receipts"].values()]; assert os.environ["STUCK_KEY"] not in ks, ks'
pyassert "five completed receipts" 'import json,os; d=json.load(open(os.environ["OUT"]+"/export.json")); assert len(d["state"]["receipts"])==5, len(d["state"]["receipts"])'
python3 -c 'print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\"2027-06-17T18:00\",\"party_size\":2}")' > "$TMP/post-body.json"
expect "post-snapshot write" "$(api_post /reservations "$TMP/post-body.json" "$TMP/post-resp.json" "$TOK_B1" donor-post-01)" 201
POST_REF=$(jget "$TMP/post-resp.json" "['reference']")
export POST_REF
pyassert "saved snapshot unchanged by later write" 'import json,os; d=json.load(open(os.environ["OUT"]+"/export.json")); assert os.environ["POST_REF"] not in d["state"]["reservations"], "snapshot moved"'

# 10. Final current lists for the manifest, then the manifest itself
#     (private; contains credentials and tokens by design).
expect "final ada list" "$(api_get /reservations "$TMP/list-final-ada.json" "$TOK_A1")" 200
expect "final bea list" "$(api_get /reservations "$TMP/list-final-bea.json" "$TOK_B1")" 200
export TOK_A1 TOK_A2 TOK_B1 TOK_B2 LOST_REF LOST_KEY REF_A REF_B A_KEY B_KEY BATCH_KEY REF_REUSE FAIL_KEY POST_REF ADA_PASSWORD BEA_PASSWORD SRC_IMAGE SRC_CONTAINER SRC_PORT DATE TMP
python3 <<'PYEOF' > "$OUT/manifest.json"
import json, os
tmp = os.environ["TMP"]
def load(name):
    with open(os.path.join(tmp, name)) as f:
        return json.load(f)
def raw(name):
    with open(os.path.join(tmp, name)) as f:
        return f.read()
ada_list = load("list-final-ada.json")["reservations"]
bea_list = load("list-final-bea.json")["reservations"]
by_ref = {r["reference"]: r for r in ada_list + bea_list}
manifest = {
    "origin": {
        "reviewed_revision": "b298700f790c166cf7ce8d98d731c80093ecb9af",
        "stage1_tree": "8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c",
        "image": os.environ["SRC_IMAGE"],
        "container": os.environ["SRC_CONTAINER"],
        "port": int(os.environ["SRC_PORT"]),
    },
    "fixture": {
        "date": os.environ["DATE"],
        "restaurant": {"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
                       "slot_minutes": 30, "reservation_duration_minutes": 90,
                       "cancellation_cutoff_minutes": 120,
                       "tables": [{"id": "t_1", "capacity": 2}, {"id": "t_2", "capacity": 4}, {"id": "t_3", "capacity": 4}]},
    },
    "users": [
        {"id": "u_ada", "email": "ada@example.com", "password": os.environ["ADA_PASSWORD"],
         "display_name": "Ada", "tokens": [os.environ["TOK_A1"], os.environ["TOK_A2"]]},
        {"id": "u_bea", "email": "bea@example.com", "password": os.environ["BEA_PASSWORD"],
         "display_name": "Bea", "tokens": [os.environ["TOK_B1"], os.environ["TOK_B2"]]},
    ],
    "pending_retry": {"role": "lost browser response, stays confirmed",
                      "reference": os.environ["LOST_REF"], "key": os.environ["LOST_KEY"],
                      "body": json.loads(raw("lost-body.json")),
                      "response": load("lost-resp.json"),
                      "current": by_ref[os.environ["LOST_REF"]]},
    "receipts": [
        {"key": os.environ["LOST_KEY"], "user_id": "u_ada", "method": "POST", "path": "/reservations",
         "body": json.loads(raw("lost-body.json")), "response": load("lost-resp.json"), "status": 201},
        {"key": os.environ["A_KEY"], "user_id": "u_ada", "method": "POST", "path": "/reservations",
         "body": json.loads(raw("a-body.json")), "response": load("a-resp.json"), "status": 201},
        {"key": os.environ["B_KEY"], "user_id": "u_ada", "method": "POST", "path": "/reservations",
         "body": json.loads(raw("b-body.json")), "response": load("b-resp.json"), "status": 201},
        {"key": os.environ["BATCH_KEY"], "user_id": "u_ada", "method": "POST", "path": "/reservation-moves",
         "body": json.loads(raw("batch-body.json")), "response": load("batch-resp.json"), "status": 201},
        {"key": os.environ["FAIL_KEY"], "user_id": "u_bea", "method": "POST", "path": "/reservations",
         "failed_body": json.loads(raw("fail-body.json")),
         "body": json.loads(raw("fail-reuse-body.json")), "response": load("fail-reuse-resp.json"), "status": 201},
    ],
    "failed_keys_absent_from_receipts": [
        {"key": os.environ["STUCK_KEY"], "body": json.loads(raw("stuck-body.json")), "expected_status": 422},
    ],
    "current_records": {
        "lost": by_ref[os.environ["LOST_REF"]],
        "swapped_a": by_ref[os.environ["REF_A"]],
        "swapped_b": by_ref[os.environ["REF_B"]],
        "reused": by_ref[os.environ["REF_REUSE"]],
        "ada_list": ada_list,
        "bea_list": bea_list,
    },
    "post_snapshot_write_excluded_from_export": {"reference": os.environ["POST_REF"]},
}
print(json.dumps(manifest, indent=2, sort_keys=True))
PYEOF
pyassert "manifest shape" 'import json,os; d=json.load(open(os.environ["OUT"]+"/manifest.json")); assert d["pending_retry"]["current"]["status"]=="confirmed"; assert len(d["receipts"])==5; assert d["current_records"]["swapped_b"]["status"]=="cancelled"'
pyassert "manifest/export agreement" 'import json,os; m=json.load(open(os.environ["OUT"]+"/manifest.json")); e=json.load(open(os.environ["OUT"]+"/export.json")); mrefs={r["reference"]: r["status"] for r in m["current_records"]["ada_list"]+m["current_records"]["bea_list"]}; erefs={r: v["status"] for r, v in e["state"]["reservations"].items()}; post=os.environ["POST_REF"]; assert set(erefs)==set(mrefs)-{post}, (mrefs, erefs); assert all(erefs[r]==mrefs[r] for r in erefs); assert post in mrefs and post not in erefs'

echo "donor checks passed: $PASS failed: $FAIL"
if [ "$FAIL" != 0 ]; then
  exit 1
fi
