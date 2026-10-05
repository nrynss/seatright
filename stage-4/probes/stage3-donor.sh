#!/bin/sh
# stage3-donor.sh - genuine stage-1/stage-2 export donors for stage-3 migration.
#
# Usage: sh stage-3/probes/stage3-donor.sh <stage1-source-url> <stage2-source-url> <private-output-directory>
#
# Each source URL is an independently running genuine older-service process
# (immutable stage-1 / stage-2 folder image), never the new stage-3 binary.
# The script drives EACH source through fixture reset, logins, bookings, an
# atomic swap batch, an amendment, a cancellation, a failed reusable key and
# receipt replays (plus a genuine pair flow with a real idempotent pair batch
# for the stage-2 source), then writes private artifacts:
#   OUT/stage1/export.json + manifest.json   (5 scoped receipts)
#   OUT/stage2/export.json + manifest.json   (8 scoped receipts)
# Only OUT_DIR is written (0700; exports/manifests 0600). Temporary run files
# live in OUT/tmp.XXXXXX (removed on exit); private python diagnostics live in
# OUT/diag (retained). Stdout carries names/statuses/counts/provenance only,
# never tokens, passwords, reservation bodies or export JSON. Every python
# tool's stderr is redirected into OUT/diag; assertion failures print a safe
# name plus a private-diagnostics pointer, count FAIL through expect()/
# pyassert(), and phase gates abort nonzero. No xtrace is used anywhere near
# credential operations. curl calls carry connect/max-time bounds. The script
# fails on unreachable sources, wrong-stage shapes or any missing assertion.
# Nothing is synthesized: exports are fetched intact and never edited.
#
# Provenance (actual deployment, not from HTTP) is fed via:
#   STAGE1_IMAGE / STAGE1_CONTAINER / STAGE1_PORT / STAGE1_CID
#   STAGE2_IMAGE / STAGE2_CONTAINER / STAGE2_PORT / STAGE2_CID
# (container .Id values and deployed image IDs come from docker inspect
# before the run; IMAGE carries the deployed image sha, CONTAINER the actual
# container name, CID the actual container .Id).
# Fixture seed passwords default below and may be overridden with
# ADA_PASSWORD / BEA_PASSWORD (serialized safely as JSON, never interpolated
# into shell).
#
# Manifest schema (manifest_version 1):
#   manifest_version: 1
#   source: {stage, reviewed_revision, stage_tree, image, container,
#     container_id, port}
#   fixture: {date, pair_seed_date|null, past_seed_date,
#     restaurant (COMPLETE original detail record incl. labels, hours and
#     combinable), seeds[full seed request records in reset order]}
#   users: [{id, email, password, display_name, tokens[2 live tokens]}]
#   records: {ada_list, bea_list (full current owner lists taken BEFORE the
#     deliberate post-snapshot write), by_reference{ref: full GET record}}
#   receipts: [{key, user_id, method, path, body(parsed), body_raw(sent
#     string), response(parsed), response_raw(received string), status}]
#     The validator binds json.loads(body_raw) to the parsed body and the
#     canonical stored Body, json.loads(response_raw) to the parsed response
#     and the full stored Response, and requires received response raw bytes
#     to equal the stored response bytes, tolerating only a single trailing
#     HTTP framing newline (genuine donors carry none).
#   pending_retry: {role, reference, key, owner, method, path, body,
#     response, current}
#   failed_keys: {reused{...}, absent[{...}]}
#   post_snapshot_write: {reference, excluded_from_export true}
#   current_lists_before_post_snapshot_write: true
# The manifest is private supplemental metadata; the opaque export is stored
# byte-identical and never rewritten to inject it. A copied-artifact
# validator (step 15) re-checks manifest/export agreement on the genuine
# files and proves sabotaged COPIES are rejected; sabotaged copies are
# always labeled as such and never described as producer output.
set -u

S1_URL=${1:?usage: sh stage-3/probes/stage3-donor.sh STAGE1_URL STAGE2_URL OUT_DIR}
S2_URL=${2:?usage: sh stage-3/probes/stage3-donor.sh STAGE1_URL STAGE2_URL OUT_DIR}
OUT=${3:?usage: sh stage-3/probes/stage3-donor.sh STAGE1_URL STAGE2_URL OUT_DIR}
ADA_PASSWORD=${ADA_PASSWORD:-correct horse ada}
BEA_PASSWORD=${BEA_PASSWORD:-correct horse bea}
STAGE1_IMAGE=${STAGE1_IMAGE:-tablekeeper:s3-d-stage1}
STAGE1_CONTAINER=${STAGE1_CONTAINER:-tk-s3-d-src1}
STAGE1_PORT=${STAGE1_PORT:-9143}
STAGE1_CID=${STAGE1_CID:-unknown}
STAGE2_IMAGE=${STAGE2_IMAGE:-tablekeeper:s3-d-stage2}
STAGE2_CONTAINER=${STAGE2_CONTAINER:-tk-s3-d-src2}
STAGE2_PORT=${STAGE2_PORT:-9144}
STAGE2_CID=${STAGE2_CID:-unknown}

S1_REV=b298700f790c166cf7ce8d98d731c80093ecb9af
S1_TREE=8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c
S2_REV=8812cdeaaa993cd944493c654e51d355cdd6b676
S2_TREE=9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f
PAST=2020-01-02
PAIRSEED=2020-01-09

PASS=0
FAIL=0
BASE=""
TMP=""
DIAG=""
S_OUT=""

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
    echo "FAIL: $1 (got $2, want $3; see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
}

pyassert() {
  if python3 -c "$2" 2>>"$DIAG/stderr.log"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1 (python check failed; see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
}

require_clean() {
  if [ "$FAIL" != 0 ]; then
    echo "ABORT: $1 already failed ($FAIL failures)"
    exit 1
  fi
}

CURL_OPTS="--connect-timeout 5 --max-time 25"

api_post() {
  curl -s $CURL_OPTS -o "$3" -w '%{http_code}' -X POST "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4" -H "Idempotency-Key: $5"
}

api_post_nokey() {
  curl -s $CURL_OPTS -o "$3" -w '%{http_code}' -X POST "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4"
}

api_patch() {
  curl -s $CURL_OPTS -o "$3" -w '%{http_code}' -X PATCH "$BASE$1" \
    -H 'Content-Type: application/json' --data-binary "@$2" \
    -H "Authorization: Bearer $4"
}

api_get() {
  if [ -n "$3" ]; then
    curl -s $CURL_OPTS -o "$2" -w '%{http_code}' "$BASE$1" -H "Authorization: Bearer $3"
  else
    curl -s $CURL_OPTS -o "$2" -w '%{http_code}' "$BASE$1"
  fi
}

jget() {
  python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d'"$2"')' "$1" 2>>"$DIAG/stderr.log"
}

# run_source STAGE OUT_SUBDIR IMAGE CONTAINER PORT CID REVIEWED_REV TREE KEY_PREFIX PAIR_MODE
run_source() {
  STAGE=$1
  S_OUT="$OUT/$2"
  IMAGE=$3
  CONTAINER=$4
  PORT=$5
  CID=$6
  REV=$7
  TREE=$8
  KEYP=$9
  PAIR=${10}
  BASE=""
  if [ "$STAGE" = "stage1" ]; then
    BASE=$S1_URL
  else
    BASE=$S2_URL
  fi
  mkdir -p "$S_OUT"
  chmod 0700 "$S_OUT"
  if [ -z "$CID" ] || [ "$CID" = "unknown" ]; then
    echo "FAIL: $STAGE proven container id missing (set STAGE CID env)"
    FAIL=$((FAIL + 1))
    return 1
  else
    echo "PASS: $STAGE proven container id supplied"
    PASS=$((PASS + 1))
  fi

  # 1. Computed future Thursday (>=30 days out).
  DATE=$(python3 -c '
import datetime
d = datetime.date.today() + datetime.timedelta(days=35)
while d.strftime("%a") != "Thu":
    d += datetime.timedelta(days=1)
print(d)' 2>>"$DIAG/stderr.log")
  export DATE
  pyassert "$STAGE future date is Thursday" 'import datetime,os; assert datetime.date.fromisoformat(os.environ["DATE"]).strftime("%a")=="Thu"'
  pyassert "$STAGE future date >=30 days out" 'import datetime,os; assert (datetime.date.fromisoformat(os.environ["DATE"])-datetime.date.today()).days>=30'

  # 2. Self-contained stage-correct fixture with producer-valid seeds
  #    (past, off-grid, over-capacity, cancelled; stage-2 adds a canonical
  #    pair seed on a separate date).
  export ADA_PASSWORD BEA_PASSWORD PAST PAIRSEED STAGE
  python3 <<'PYEOF' > "$TMP/$STAGE-fixture.json" 2>>"$DIAG/stderr.log"
import json, os
stage = os.environ["STAGE"]
seeds = [
  {"id": "seed-past-1", "reference": "SEDPST", "user_id": "u_ada",
   "restaurant_id": "r_anker", "table_id": "t_3",
   "starts_at_local": os.environ["PAST"] + "T18:00", "party_size": 2},
  {"id": "seed-off-1", "reference": "SEDOFF", "user_id": "u_ada",
   "restaurant_id": "r_anker", "table_id": "t_2",
   "starts_at_local": os.environ["PAST"] + "T18:07", "party_size": 2},
  {"id": "seed-ovr-1", "reference": "SEDOVR", "user_id": "u_bea",
   "restaurant_id": "r_anker", "table_id": "t_1",
   "starts_at_local": os.environ["PAST"] + "T20:30", "party_size": 9},
  {"id": "seed-cxd-1", "reference": "SEDCXD", "user_id": "u_bea",
   "restaurant_id": "r_anker", "table_id": "t_1",
   "starts_at_local": os.environ["PAST"] + "T18:00", "party_size": 2,
   "status": "cancelled"},
]
if stage == "stage2":
    seeds.append({"id": "seed-pair-1", "reference": "SEDPAIR", "user_id": "u_ada",
                  "restaurant_id": "r_anker", "table_ids": ["t_2", "t_1"],
                  "starts_at_local": os.environ["PAIRSEED"] + "T18:00", "party_size": 6})
rest = {"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                          {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                   {"id": "t_2", "label": "2", "capacity": 4},
                   {"id": "t_3", "label": "3", "capacity": 4}]}
if stage == "stage2":
    rest["combinable"] = [["t_1", "t_2"], ["t_2", "t_3"]]
print(json.dumps({
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": os.environ["ADA_PASSWORD"], "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": os.environ["BEA_PASSWORD"], "display_name": "Bea"}],
  "restaurants": [rest],
  "reservations": seeds}))
PYEOF
  expect "$STAGE reset fixture" "$(api_post /_test/reset "$TMP/$STAGE-fixture.json" "$TMP/$STAGE-reset.out" "" "-")" 204
  require_clean "$STAGE fixture reset"
  expect "$STAGE restaurant detail" "$(api_get /restaurants/r_anker "$TMP/$STAGE-restaurant.json" "")" 200
  require_clean "$STAGE restaurant detail"

  # 3. Four sessions: two tokens per user; hash login + multi-session proof.
  login_body() {
    python3 -c 'import json,sys; print(json.dumps({"email":sys.argv[1],"password":sys.argv[2]}))' "$1" "$2" > "$3" 2>>"$DIAG/stderr.log"
  }
  login_body ada@example.com "$ADA_PASSWORD" "$TMP/$STAGE-login-ada.json"
  login_body bea@example.com "$BEA_PASSWORD" "$TMP/$STAGE-login-bea.json"
  expect "$STAGE ada login 1" "$(api_post /auth/login "$TMP/$STAGE-login-ada.json" "$TMP/$STAGE-tok-a1.json" "" "-")" 200
  expect "$STAGE ada login 2" "$(api_post /auth/login "$TMP/$STAGE-login-ada.json" "$TMP/$STAGE-tok-a2.json" "" "-")" 200
  expect "$STAGE bea login 1" "$(api_post /auth/login "$TMP/$STAGE-login-bea.json" "$TMP/$STAGE-tok-b1.json" "" "-")" 200
  expect "$STAGE bea login 2" "$(api_post /auth/login "$TMP/$STAGE-login-bea.json" "$TMP/$STAGE-tok-b2.json" "" "-")" 200
  require_clean "$STAGE logins"
  TOK_A1=$(jget "$TMP/$STAGE-tok-a1.json" "['token']")
  TOK_A2=$(jget "$TMP/$STAGE-tok-a2.json" "['token']")
  TOK_B1=$(jget "$TMP/$STAGE-tok-b1.json" "['token']")
  TOK_B2=$(jget "$TMP/$STAGE-tok-b2.json" "['token']")
  if [ "$TOK_A1" = "$TOK_A2" ] || [ "$TOK_B1" = "$TOK_B2" ]; then
    echo "FAIL: $STAGE concurrent sessions share a token"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE distinct session tokens"
    PASS=$((PASS + 1))
  fi

  # 4. Pending-retry booking (Ada, stays confirmed; donor analogue of a lost
  #    response whose original receipt is retried after migration).
  LOST_KEY=$KEYP-lost-01
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-lost-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE lost booking create" "$(api_post /reservations "$TMP/$STAGE-lost-body.json" "$TMP/$STAGE-lost-resp.json" "$TOK_A1" "$LOST_KEY")" 201
  require_clean "$STAGE lost booking"
  LOST_REF=$(jget "$TMP/$STAGE-lost-resp.json" "['reference']")

  # 5. Swap pair A (t_1) and B (t_2) at the same future slot, atomic batch,
  #    then amend A and cancel B. Originals are stored before those edits.
  A_KEY=$KEYP-create-a
  B_KEY=$KEYP-create-b
  BATCH_KEY=$KEYP-batch-01
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T19:30\",\"party_size\":2}")' > "$TMP/$STAGE-a-body.json" 2>>"$DIAG/stderr.log"
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_2\",\"starts_at_local\":\""+os.environ["DATE"]+"T19:30\",\"party_size\":2}")' > "$TMP/$STAGE-b-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE create A" "$(api_post /reservations "$TMP/$STAGE-a-body.json" "$TMP/$STAGE-a-resp.json" "$TOK_A1" "$A_KEY")" 201
  expect "$STAGE create B" "$(api_post /reservations "$TMP/$STAGE-b-body.json" "$TMP/$STAGE-b-resp.json" "$TOK_A1" "$B_KEY")" 201
  require_clean "$STAGE swap pair creation"
  REF_A=$(jget "$TMP/$STAGE-a-resp.json" "['reference']")
  REF_B=$(jget "$TMP/$STAGE-b-resp.json" "['reference']")
  python3 -c 'import json,sys; print(json.dumps({"moves":[{"reference":sys.argv[1],"table_id":"t_2"},{"reference":sys.argv[2],"table_id":"t_1"}]}))' "$REF_A" "$REF_B" > "$TMP/$STAGE-batch-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE swap batch" "$(api_post /reservation-moves "$TMP/$STAGE-batch-body.json" "$TMP/$STAGE-batch-resp.json" "$TOK_A1" "$BATCH_KEY")" 201
  require_clean "$STAGE swap batch"
  echo '{"party_size":1}' > "$TMP/$STAGE-amend-a.json"
  expect "$STAGE amend A" "$(api_patch /reservations/"$REF_A" "$TMP/$STAGE-amend-a.json" "$TMP/$STAGE-amend-a-resp.json" "$TOK_A1")" 200
  : > "$TMP/$STAGE-empty.json"
  expect "$STAGE cancel B" "$(api_post_nokey /reservations/"$REF_B"/cancel "$TMP/$STAGE-empty.json" "$TMP/$STAGE-cancel-b-resp.json" "$TOK_A1")" 200

  # 6. Failed create key on the occupied slot (409, no receipt), then the same
  #    key with a legal body (201 + replay 200 identical). One permanently
  #    failed key stays absent from the export for later destination use.
  FAIL_KEY=$KEYP-fail-01
  STUCK_KEY=$KEYP-fail-02
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-fail-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE failed create 409" "$(api_post /reservations "$TMP/$STAGE-fail-body.json" "$TMP/$STAGE-fail-resp.json" "$TOK_B1" "$FAIL_KEY")" 409
  pyassert "$STAGE failed code table_unavailable" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-fail-resp.json")); assert d["error"]["code"]=="table_unavailable"'
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\""+os.environ["DATE"]+"T20:30\",\"party_size\":2}")' > "$TMP/$STAGE-fail-reuse-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE failed key reused 201" "$(api_post /reservations "$TMP/$STAGE-fail-reuse-body.json" "$TMP/$STAGE-fail-reuse-resp.json" "$TOK_B1" "$FAIL_KEY")" 201
  REF_REUSE=$(jget "$TMP/$STAGE-fail-reuse-resp.json" "['reference']")
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":\"many\"}")' > "$TMP/$STAGE-stuck-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE stuck key stays failed" "$(api_post /reservations "$TMP/$STAGE-stuck-body.json" "$TMP/$STAGE-stuck-resp.json" "$TOK_B1" "$STUCK_KEY")" 422

  # 7. Stage-2 genuine pair flow: reversed table_ids canonicalize to declared
  #    order; a party-only PATCH keeps the set; a plain PATCH moves the set;
  #    then a REAL idempotent pair batch (POST /reservation-moves) moves the
  #    pair back, its receipt is captured, current party is mutated 5 to 6
  #    afterwards, and replay still returns the original bytes. A second pair
  #    is created then cancelled live.
  if [ "$PAIR" = 1 ]; then
    P_KEY=$KEYP-pair-01
    PB_KEY=$KEYP-pairbatch-01
    P2_KEY=$KEYP-pair-02
    python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_ids\":[\"t_2\",\"t_1\"],\"starts_at_local\":\""+os.environ["DATE"]+"T21:00\",\"party_size\":6}")' > "$TMP/$STAGE-p-body.json" 2>>"$DIAG/stderr.log"
    expect "$STAGE pair create" "$(api_post /reservations "$TMP/$STAGE-p-body.json" "$TMP/$STAGE-p-resp.json" "$TOK_A1" "$P_KEY")" 201
    require_clean "$STAGE pair create"
    REF_P=$(jget "$TMP/$STAGE-p-resp.json" "['reference']")
    pyassert "$STAGE pair canonical order" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-p-resp.json")); assert d["table_ids"]==["t_1","t_2"]; assert "table_id" not in d'
    echo '{"party_size":5}' > "$TMP/$STAGE-amend-p.json"
    expect "$STAGE pair party amend" "$(api_patch /reservations/"$REF_P" "$TMP/$STAGE-amend-p.json" "$TMP/$STAGE-amend-p-resp.json" "$TOK_A1")" 200
    pyassert "$STAGE pair amend keeps set" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-amend-p-resp.json")); assert d["table_ids"]==["t_1","t_2"] and d["party_size"]==5'
    python3 -c 'import os; print("{\"table_ids\":[\"t_2\",\"t_3\"],\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\"}")' > "$TMP/$STAGE-pmove-body.json" 2>>"$DIAG/stderr.log"
    expect "$STAGE pair patch move" "$(api_patch /reservations/"$REF_P" "$TMP/$STAGE-pmove-body.json" "$TMP/$STAGE-pmove-resp.json" "$TOK_A1")" 200
    pyassert "$STAGE pair move current set" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-pmove-resp.json")); assert d["table_ids"]==["t_2","t_3"]'
    python3 -c 'import json,os,sys; print(json.dumps({"moves":[{"reference":sys.argv[1],"table_ids":["t_1","t_2"],"starts_at_local":os.environ["DATE"]+"T21:00"}]}))' "$REF_P" > "$TMP/$STAGE-pbatch-body.json" 2>>"$DIAG/stderr.log"
    expect "$STAGE pair batch move" "$(api_post /reservation-moves "$TMP/$STAGE-pbatch-body.json" "$TMP/$STAGE-pbatch-resp.json" "$TOK_A1" "$PB_KEY")" 201
    require_clean "$STAGE pair batch move"
    pyassert "$STAGE pair batch result" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-pbatch-resp.json")); assert d["reservations"][0]["table_ids"]==["t_1","t_2"] and d["reservations"][0]["party_size"]==5'
    echo '{"party_size":6}' > "$TMP/$STAGE-mutate-p.json"
    expect "$STAGE pair mutate after batch" "$(api_patch /reservations/"$REF_P" "$TMP/$STAGE-mutate-p.json" "$TMP/$STAGE-mutate-p-resp.json" "$TOK_A1")" 200
    pyassert "$STAGE pair current differs from batch receipt" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-mutate-p-resp.json")); assert d["party_size"]==6 and d["table_ids"]==["t_1","t_2"]'
    python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_ids\":[\"t_2\",\"t_3\"],\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":4}")' > "$TMP/$STAGE-p2-body.json" 2>>"$DIAG/stderr.log"
    expect "$STAGE second pair create" "$(api_post /reservations "$TMP/$STAGE-p2-body.json" "$TMP/$STAGE-p2-resp.json" "$TOK_A1" "$P2_KEY")" 201
    REF_P2=$(jget "$TMP/$STAGE-p2-resp.json" "['reference']")
    expect "$STAGE second pair cancel" "$(api_post_nokey /reservations/"$REF_P2"/cancel "$TMP/$STAGE-empty.json" "$TMP/$STAGE-cancel-p2-resp.json" "$TOK_A1")" 200
  fi

  # 8. Replay every completed receipt: 200 with byte-identical original JSON,
  #    after all later amendments and cancellations.
  replay_check() {
    name=$1; key=$2; bodyfile=$3; origfile=$4; path=$5; token=$6
    st=$(api_post "$path" "$bodyfile" "$TMP/$STAGE-replay-$name.json" "$token" "$key")
    if [ "$st" != 200 ]; then
      echo "FAIL: $STAGE replay $name status (got $st, want 200; see private diagnostics)"
      FAIL=$((FAIL + 1))
      return
    fi
    if cmp -s "$origfile" "$TMP/$STAGE-replay-$name.json"; then
      echo "PASS: $STAGE replay $name identical"
      PASS=$((PASS + 1))
    else
      echo "FAIL: $STAGE replay $name body differs (see private diagnostics)"
      FAIL=$((FAIL + 1))
    fi
  }
  replay_check lost "$LOST_KEY" "$TMP/$STAGE-lost-body.json" "$TMP/$STAGE-lost-resp.json" /reservations "$TOK_A1"
  replay_check create-a "$A_KEY" "$TMP/$STAGE-a-body.json" "$TMP/$STAGE-a-resp.json" /reservations "$TOK_A1"
  replay_check create-b "$B_KEY" "$TMP/$STAGE-b-body.json" "$TMP/$STAGE-b-resp.json" /reservations "$TOK_A1"
  replay_check batch "$BATCH_KEY" "$TMP/$STAGE-batch-body.json" "$TMP/$STAGE-batch-resp.json" /reservation-moves "$TOK_A1"
  replay_check fail-reuse "$FAIL_KEY" "$TMP/$STAGE-fail-reuse-body.json" "$TMP/$STAGE-fail-reuse-resp.json" /reservations "$TOK_B1"
  if [ "$PAIR" = 1 ]; then
    replay_check pair-create "$P_KEY" "$TMP/$STAGE-p-body.json" "$TMP/$STAGE-p-resp.json" /reservations "$TOK_A1"
    replay_check pair-batch "$PB_KEY" "$TMP/$STAGE-pbatch-body.json" "$TMP/$STAGE-pbatch-resp.json" /reservation-moves "$TOK_A1"
    replay_check pair2-create "$P2_KEY" "$TMP/$STAGE-p2-body.json" "$TMP/$STAGE-p2-resp.json" /reservations "$TOK_A1"
  fi

  # 9. Current-state reads with the correct owner's token, plus isolation:
  #    cross-owner reads 404, missing token 401. Both tokens of each owner
  #    must see identical lists.
  expect "$STAGE lost lookup" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-lookup-lost.json" "$TOK_A1")" 200
  pyassert "$STAGE lost status confirmed" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-lost.json")); assert d["status"]=="confirmed"'
  expect "$STAGE cross-owner lookup 404" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-xlookup.json" "$TOK_B1")" 404
  expect "$STAGE unauthenticated lookup 401" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-nolookup.json" "")" 401
  expect "$STAGE A lookup" "$(api_get /reservations/"$REF_A" "$TMP/$STAGE-lookup-a.json" "$TOK_A1")" 200
  pyassert "$STAGE A current values" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-a.json")); assert d["party_size"]==1 and d["table_id"]=="t_2"'
  expect "$STAGE B lookup" "$(api_get /reservations/"$REF_B" "$TMP/$STAGE-lookup-b.json" "$TOK_A1")" 200
  pyassert "$STAGE B current cancelled" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-b.json")); assert d["status"]=="cancelled"'
  expect "$STAGE reuse lookup" "$(api_get /reservations/"$REF_REUSE" "$TMP/$STAGE-lookup-reuse.json" "$TOK_B1")" 200
  pyassert "$STAGE reuse current confirmed" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-reuse.json")); assert d["status"]=="confirmed"'
  expect "$STAGE all tokens valid" "$(api_get /reservations "$TMP/$STAGE-list-a1.json" "$TOK_A1")" 200
  expect "$STAGE second ada token valid" "$(api_get /reservations "$TMP/$STAGE-list-a2.json" "$TOK_A2")" 200
  expect "$STAGE bea token valid" "$(api_get /reservations "$TMP/$STAGE-list-b1.json" "$TOK_B1")" 200
  expect "$STAGE second bea token valid" "$(api_get /reservations "$TMP/$STAGE-list-b2.json" "$TOK_B2")" 200
  if cmp -s "$TMP/$STAGE-list-a1.json" "$TMP/$STAGE-list-a2.json"; then
    echo "PASS: $STAGE ada sessions identical lists"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE ada sessions identical lists (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  if cmp -s "$TMP/$STAGE-list-b1.json" "$TMP/$STAGE-list-b2.json"; then
    echo "PASS: $STAGE bea sessions identical lists"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE bea sessions identical lists (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  if [ "$PAIR" = 1 ]; then
    expect "$STAGE pair lookup" "$(api_get /reservations/"$REF_P" "$TMP/$STAGE-lookup-p.json" "$TOK_A1")" 200
    pyassert "$STAGE pair current canonical confirmed" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-p.json")); assert d["status"]=="confirmed" and d["table_ids"]==["t_1","t_2"] and d["party_size"]==6'
    expect "$STAGE pair2 lookup" "$(api_get /reservations/"$REF_P2" "$TMP/$STAGE-lookup-p2.json" "$TOK_A1")" 200
    pyassert "$STAGE pair2 current cancelled" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-p2.json")); assert d["status"]=="cancelled" and d["table_ids"]==["t_2","t_3"]'
  fi

  # 9b. GET every fixture seed with its owner token (ref/owner map from the
  #     submitted fixture, never from server output).
  python3 <<'PYEOF' > "$TMP/$STAGE-seedrefs.txt" 2>>"$DIAG/stderr.log"
import json, os
stage = os.environ["STAGE"]
tmp = os.environ["TMP"]
fix = json.load(open(os.path.join(tmp, stage + "-fixture.json")))
for r in fix["reservations"]:
    print(r["reference"], r["user_id"])
PYEOF
  while read -r ref owner; do
    if [ "$owner" = "u_ada" ]; then
      stok=$TOK_A1
    else
      stok=$TOK_B1
    fi
    expect "$STAGE seed lookup" "$(api_get /reservations/"$ref" "$TMP/$STAGE-get-$ref.json" "$stok")" 200
  done < "$TMP/$STAGE-seedrefs.txt"
  require_clean "$STAGE seed lookups"

  # 9c. Wrong-stage shape guard on live records: stage-1 stays scalar,
  #     stage-2 stays canonical, and neither carries stage-3 fields.
  export STAGE
  if [ "$PAIR" = 1 ]; then
    pyassert "$STAGE pair response shapes" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; recs=[json.load(open(t+"/"+s+"-lookup-lost.json")),json.load(open(t+"/"+s+"-lookup-a.json")),json.load(open(t+"/"+s+"-lookup-b.json"))]; assert all("revision" not in r and "accepted_terms" not in r for r in recs); assert all(r["table_ids"]==[r["table_id"]] for r in recs)'
    pyassert "$STAGE singleton canonical" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; r=json.load(open(t+"/"+s+"-lookup-lost.json")); assert r["table_ids"]==["t_1"] and r["table_id"]=="t_1"'
  else
    pyassert "$STAGE scalar response shapes" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; recs=[json.load(open(t+"/"+s+"-lookup-lost.json")),json.load(open(t+"/"+s+"-lookup-a.json")),json.load(open(t+"/"+s+"-lookup-b.json"))]; assert all("revision" not in r and "accepted_terms" not in r for r in recs); assert all("table_id" in r for r in recs); assert all("table_ids" not in r for r in recs)'
  fi

  # 9d. Sentinel proof: a sentinel-bearing failed assertion counts FAIL and
  #     exits nonzero, with the sentinel present only in the private
  #     diagnostic and absent from the room-visible transcript.
  SENTINEL="S3D-SENTINEL-$STAGE-PRIVATE-MARKER"
  export SENTINEL
  (
    FAIL=0
    pyassert "$STAGE sentinel probe" 'import os; assert False, "PRIVATE-DIAG:"+os.environ["SENTINEL"]'
    exit $FAIL
  ) >"$DIAG/sentinel-room.log" 2>>"$DIAG/stderr.log"
  if [ "$?" != 0 ]; then
    echo "PASS: $STAGE sentinel failure exits nonzero"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE sentinel failure exits nonzero (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  if grep -q "$SENTINEL" "$DIAG/sentinel-room.log" 2>/dev/null; then
    echo "FAIL: $STAGE sentinel leaked to room transcript (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE sentinel absent from room transcript"
    PASS=$((PASS + 1))
  fi
  if grep -q "$SENTINEL" "$DIAG/stderr.log" 2>/dev/null; then
    echo "PASS: $STAGE sentinel retained in private diagnostic"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE sentinel retained in private diagnostic (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi

  # 10. Final current lists BEFORE the deliberate post-snapshot write, so the
  #     exported-state expected records match exactly.
  expect "$STAGE final ada list" "$(api_get /reservations "$TMP/$STAGE-list-final-ada.json" "$TOK_A1")" 200
  expect "$STAGE final bea list" "$(api_get /reservations "$TMP/$STAGE-list-final-bea.json" "$TOK_B1")" 200

  # 11. Atomic export snapshot; exact scoped receipts; stuck key absent.
  expect "$STAGE export" "$(api_get /_test/export "$S_OUT/export.json" "")" 200
  require_clean "$STAGE export"
  chmod 0600 "$S_OUT/export.json"
  export S_OUT
  pyassert "$STAGE export envelope" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)'
  export STUCK_KEY
  pyassert "$STAGE stuck key absent from receipts" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); keys=[r["key"] for r in d["state"]["receipts"].values()]; assert os.environ["STUCK_KEY"] not in keys'
  export LOST_KEY A_KEY B_KEY BATCH_KEY FAIL_KEY TOK_A1 TOK_B1
  if [ "$PAIR" = 1 ]; then
    export P_KEY PB_KEY P2_KEY
    pyassert "$STAGE eight scoped receipts" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); got={(r["user_id"],r["method"],r["path"],r["key"]) for r in d["state"]["receipts"].values()}; want={("u_ada","POST","/reservations",os.environ["LOST_KEY"]),("u_ada","POST","/reservations",os.environ["A_KEY"]),("u_ada","POST","/reservations",os.environ["B_KEY"]),("u_ada","POST","/reservation-moves",os.environ["BATCH_KEY"]),("u_bea","POST","/reservations",os.environ["FAIL_KEY"]),("u_ada","POST","/reservations",os.environ["P_KEY"]),("u_ada","POST","/reservation-moves",os.environ["PB_KEY"]),("u_ada","POST","/reservations",os.environ["P2_KEY"])}; assert got==want'
  else
    pyassert "$STAGE five scoped receipts" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); got={(r["user_id"],r["method"],r["path"],r["key"]) for r in d["state"]["receipts"].values()}; want={("u_ada","POST","/reservations",os.environ["LOST_KEY"]),("u_ada","POST","/reservations",os.environ["A_KEY"]),("u_ada","POST","/reservations",os.environ["B_KEY"]),("u_ada","POST","/reservation-moves",os.environ["BATCH_KEY"]),("u_bea","POST","/reservations",os.environ["FAIL_KEY"])}; assert got==want'
  fi

  # 12. Snapshot isolation: a DIFFERENT post-export booking must not appear in
  #     the saved file. The slot is stage-aware: the stage-2 moved pair
  #     occupies t_2+t_3 at 18:00, so its probe uses free t_1 at 19:30.
  if [ "$PAIR" = 1 ]; then
    python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T19:30\",\"party_size\":2}")' > "$TMP/$STAGE-post-body.json" 2>>"$DIAG/stderr.log"
  else
    python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-post-body.json" 2>>"$DIAG/stderr.log"
  fi
  expect "$STAGE post-snapshot write" "$(api_post /reservations "$TMP/$STAGE-post-body.json" "$TMP/$STAGE-post-resp.json" "$TOK_B1" "$KEYP-post-01")" 201
  POST_REF=$(jget "$TMP/$STAGE-post-resp.json" "['reference']")
  export POST_REF
  pyassert "$STAGE saved snapshot unchanged by later write" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); assert os.environ["POST_REF"] not in d["state"]["reservations"]'

  # 13. Manifest (private by design: credentials, tokens, full records).
  #     Original request strings and received response strings are preserved
  #     alongside parsed JSON; nothing is reconstructed from later state.
  export TOK_A2 TOK_B2 LOST_REF REF_A REF_B REF_REUSE POST_REF ADA_PASSWORD BEA_PASSWORD IMAGE CONTAINER PORT CID REV TREE STAGE
  if [ "$PAIR" = 1 ]; then
    export REF_P REF_P2 P_KEY PB_KEY P2_KEY
  fi
  python3 <<'PYEOF' > "$S_OUT/manifest.json" 2>>"$DIAG/stderr.log"
import json, os
s_out = os.environ["S_OUT"]
stage = os.environ["STAGE"]
tmp = os.environ["TMP"]
pair = os.environ.get("REF_P") is not None
def load(name):
    with open(os.path.join(tmp, stage + "-" + name)) as f:
        return json.load(f)
def raw(name):
    with open(os.path.join(tmp, stage + "-" + name)) as f:
        return f.read()
ada_list = load("list-final-ada.json")["reservations"]
bea_list = load("list-final-bea.json")["reservations"]
by_ref = {r["reference"]: r for r in ada_list + bea_list}
records = {
    "ada_list": ada_list,
    "bea_list": bea_list,
    "by_reference": {r: by_ref[r] for r in sorted(by_ref)},
}
def receipt(key, user, method, path, bodyfile, respfile, status=201, failed=None):
    entry = {"key": os.environ[key], "user_id": user, "method": method,
             "path": path, "body": json.loads(raw(bodyfile)),
             "body_raw": raw(bodyfile), "response": load(respfile),
             "response_raw": raw(respfile), "status": status}
    if failed is not None:
        entry["failed_body"] = json.loads(raw(failed[0]))
        entry["failed_body_raw"] = raw(failed[0])
        entry["failed_code"] = failed[1]
    return entry
receipts = [
    receipt("LOST_KEY", "u_ada", "POST", "/reservations", "lost-body.json", "lost-resp.json"),
    receipt("A_KEY", "u_ada", "POST", "/reservations", "a-body.json", "a-resp.json"),
    receipt("B_KEY", "u_ada", "POST", "/reservations", "b-body.json", "b-resp.json"),
    receipt("BATCH_KEY", "u_ada", "POST", "/reservation-moves", "batch-body.json", "batch-resp.json"),
    receipt("FAIL_KEY", "u_bea", "POST", "/reservations", "fail-reuse-body.json", "fail-reuse-resp.json",
            failed=("fail-body.json", "table_unavailable")),
]
if pair:
    receipts.append(
        receipt("P_KEY", "u_ada", "POST", "/reservations", "p-body.json", "p-resp.json"))
    receipts.append(
        receipt("PB_KEY", "u_ada", "POST", "/reservation-moves", "pbatch-body.json", "pbatch-resp.json"))
    receipts.append(
        receipt("P2_KEY", "u_ada", "POST", "/reservations", "p2-body.json", "p2-resp.json"))
manifest = {
    "manifest_version": 1,
    "source": {"stage": stage,
               "reviewed_revision": os.environ["REV"],
               "stage_tree": os.environ["TREE"],
               "image": os.environ["IMAGE"],
               "container": os.environ["CONTAINER"],
               "container_id": os.environ["CID"],
               "port": int(os.environ["PORT"])},
    "fixture": {"date": os.environ["DATE"],
                "pair_seed_date": "2020-01-09" if pair else None,
                "past_seed_date": os.environ["PAST"],
                "restaurant": load("restaurant.json"),
                "seeds": json.load(open(os.path.join(tmp, stage + "-fixture.json")))["reservations"]},
    "users": [
        {"id": "u_ada", "email": "ada@example.com", "password": os.environ["ADA_PASSWORD"],
         "display_name": "Ada", "tokens": [os.environ["TOK_A1"], os.environ["TOK_A2"]]},
        {"id": "u_bea", "email": "bea@example.com", "password": os.environ["BEA_PASSWORD"],
         "display_name": "Bea", "tokens": [os.environ["TOK_B1"], os.environ["TOK_B2"]]},
    ],
    "records": records,
    "receipts": receipts,
    "pending_retry": {"role": "lost browser response analogue, stays confirmed",
                      "reference": os.environ["LOST_REF"], "key": os.environ["LOST_KEY"],
                      "owner": "u_ada", "method": "POST", "path": "/reservations",
                      "body": json.loads(raw("lost-body.json")),
                      "body_raw": raw("lost-body.json"),
                      "response": load("lost-resp.json"),
                      "response_raw": raw("lost-resp.json"),
                      "current": by_ref[os.environ["LOST_REF"]]},
    "failed_keys": {
        "reused": {"key": os.environ["FAIL_KEY"], "owner": "u_bea", "method": "POST",
                   "path": "/reservations",
                   "failed_body": json.loads(raw("fail-body.json")),
                   "failed_body_raw": raw("fail-body.json"),
                   "failed_code": "table_unavailable",
                   "body": json.loads(raw("fail-reuse-body.json"))},
        "absent": [{"key": os.environ["STUCK_KEY"], "owner": "u_bea", "method": "POST",
                    "path": "/reservations",
                    "body": json.loads(raw("stuck-body.json")),
                    "body_raw": raw("stuck-body.json"), "expected_status": 422}],
    },
    "post_snapshot_write": {"reference": os.environ["POST_REF"], "excluded_from_export": True},
    "current_lists_before_post_snapshot_write": True,
}
print(json.dumps(manifest, indent=2, sort_keys=True))
PYEOF
  chmod 0600 "$S_OUT/manifest.json"
  export S_OUT
  pyassert "$STAGE manifest envelope" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/manifest.json")); assert d["manifest_version"]==1 and d["current_lists_before_post_snapshot_write"] is True'

  # 14. Full live agreement: every by-reference GET equals its owner-list
  #     entry with all current fields (identity, party, timestamps intact).
  export TMP
  cat > "$TMP/$STAGE-agree.py" <<'PYEOF'
import glob, json, os
t = os.environ["TMP"]
s = os.environ["STAGE"]
ada = json.load(open(t + "/" + s + "-list-final-ada.json"))["reservations"]
bea = json.load(open(t + "/" + s + "-list-final-bea.json"))["reservations"]
want = {r["reference"]: r for r in ada + bea}
files = glob.glob(t + "/" + s + "-lookup-*.json") + glob.glob(t + "/" + s + "-get-*.json")
assert files, "no by-reference records fetched"
got = {}
for f in files:
    rec = json.load(open(f))
    got[rec["reference"]] = rec
assert set(got) == set(want), (sorted(got), sorted(want))
for r, rec in got.items():
    assert rec == want[r], r
print("full records compared: %d" % len(got))
PYEOF
  if python3 "$TMP/$STAGE-agree.py" 2>>"$DIAG/stderr.log"; then
    echo "PASS: $STAGE full GET/list agreement"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE full GET/list agreement (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi

  # 15. Copied-artifact validator: genuine files pass; sabotaged COPIES
  #     (changed identity/timestamp with reference/status intact, changed
  #     receipt field) are rejected; restored copies pass. Sabotaged copies
  #     are labeled as such and are never producer output.
  cat > "$TMP/$STAGE-validate.py" <<'PYEOF'
import json, sys
exp_path, man_path = sys.argv[1], sys.argv[2]
e = json.load(open(exp_path))
m = json.load(open(man_path))
errs = []
if not (e.get("track") == "tablekeeper" and e.get("format_version") == 1 and isinstance(e.get("state"), dict)):
    errs.append("envelope")
st = e["state"]
byref = m["records"]["by_reference"]
if set(byref) != set(st["reservations"]):
    errs.append("ref-sets")
for r, rec in byref.items():
    if r not in st["reservations"]:
        continue
    # Exported public projection: drop the private owner, and drop table_id
    # exactly when the stored set is not a singleton (public responses carry
    # table_id if and only if the set has one member).
    proj = {k: v for k, v in st["reservations"][r].items() if k != "user_id"}
    ids = proj.get("table_ids", [proj.get("table_id")])
    if proj.get("table_id") == "":
        del proj["table_id"]
    if ("table_id" in proj) != (len(ids) == 1):
        errs.append("table-shape:" + r)
    if rec != proj:
        errs.append("record:" + r)
mrec = {(r["user_id"], r["method"], r["path"], r["key"]): r for r in m["receipts"]}
erec = {(r["user_id"], r["method"], r["path"], r["key"]): r for r in st["receipts"].values()}
if set(mrec) != set(erec):
    errs.append("receipt-scopes")
for k in mrec:
    if k not in erec:
        continue
    mr, er = mrec[k], erec[k]
    if mr["status"] != er["status"]:
        errs.append("receipt-status:" + k[3])
    if mr["body"] != json.loads(er["body"]):
        errs.append("receipt-body:" + k[3])
    if mr["response"] != json.loads(er["response"]):
        errs.append("receipt-response:" + k[3])
    # Preserved raw strings are bound at parsed level to both the manifest
    # parsed values and the canonical stored strings.
    if json.loads(mr["body_raw"]) != mr["body"]:
        errs.append("receipt-body-raw:" + k[3])
    if json.loads(mr["body_raw"]) != json.loads(er["body"]):
        errs.append("receipt-body-raw-canonical:" + k[3])
    if json.loads(mr["response_raw"]) != mr["response"]:
        errs.append("receipt-response-raw:" + k[3])
    if json.loads(mr["response_raw"]) != json.loads(er["response"]):
        errs.append("receipt-response-raw-canonical:" + k[3])
    # Received response raw bytes must equal the stored response bytes. The
    # only tolerated difference is one trailing HTTP framing newline; the
    # genuine donors carry no such newline, and the allowance is explicit.
    rr, stored = mr["response_raw"], er["response"]
    if rr != stored and rr != stored + "\n":
        errs.append("receipt-response-bytes:" + k[3])
print("validator findings: %d" % len(errs))
sys.exit(1 if errs else 0)
PYEOF
  if python3 "$TMP/$STAGE-validate.py" "$S_OUT/export.json" "$S_OUT/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "PASS: $STAGE copied-artifact validator accepts genuine files"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE copied-artifact validator accepts genuine files (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  SCRATCH="$OUT/scratch-$STAGE"
  rm -rf "$SCRATCH"
  mkdir -p "$SCRATCH"
  chmod 0700 "$SCRATCH"
  cp "$S_OUT/export.json" "$S_OUT/manifest.json" "$SCRATCH/"
  chmod 0600 "$SCRATCH/export.json" "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/export.json"; d=json.load(open(p)); r=sorted(d["state"]["reservations"])[0]; d["state"]["reservations"][r]["reservation_id"]="SABOTAGED01"; d["state"]["reservations"][r]["created_at"]="2020-01-01T00:00:00+00:00"; d["state"]["reservations"][r]["party_size"]+=1; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects sabotaged identity copy (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects sabotaged identity copy"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/export.json" "$SCRATCH/export.json"
  chmod 0600 "$SCRATCH/export.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); d["receipts"][0]["response"]["party_size"]=99; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects sabotaged receipt copy (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects sabotaged receipt copy"
    PASS=$((PASS + 1))
  fi
  # Raw-only sabotage: change the preserved body_raw value while the parsed
  # body object is untouched; the raw binding must reject the copy.
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); r=d["receipts"][0]; b=json.loads(r["body_raw"]); b["party_size"]=9; r["body_raw"]=json.dumps(b); json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects raw-only body sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects raw-only body sabotage"
    PASS=$((PASS + 1))
  fi
  # Whitespace-only response_raw sabotage: parsed values are unchanged, so
  # only the received-bytes rule can reject the copy.
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); d["receipts"][0]["response_raw"]+=" "; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects raw-only response sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects raw-only response sabotage"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/export.json" "$S_OUT/manifest.json" "$SCRATCH/"
  chmod 0600 "$SCRATCH/export.json" "$SCRATCH/manifest.json"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "PASS: $STAGE validator accepts restored copies"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE validator accepts restored copies (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
}

umask 077
mkdir -p "$OUT"
chmod 0700 "$OUT"
TMP=$(mktemp -d "$OUT/tmp.XXXXXX")
export TMP
DIAG="$OUT/diag"
mkdir -p "$DIAG"
chmod 0700 "$DIAG"
trap cleanup EXIT INT TERM

run_source stage1 stage1 "$STAGE1_IMAGE" "$STAGE1_CONTAINER" "$STAGE1_PORT" "$STAGE1_CID" "$S1_REV" "$S1_TREE" s3d-s1 0
require_clean "stage-1 donor"
run_source stage2 stage2 "$STAGE2_IMAGE" "$STAGE2_CONTAINER" "$STAGE2_PORT" "$STAGE2_CID" "$S2_REV" "$S2_TREE" s3d-s2 1
require_clean "stage-2 donor"

echo "donor checks passed: $PASS failed: $FAIL"
if [ "$FAIL" != 0 ]; then
  exit 1
fi
