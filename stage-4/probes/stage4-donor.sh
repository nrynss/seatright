#!/bin/sh
# stage4-donor.sh - genuine stage-1/2/3 export donors for stage-4 portability.
#
# Usage: sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR
#
# Each source URL is an independently running genuine older-service process
# built from the immutable accepted stage folders (stage-1, stage-2,
# stage-3), never the new stage-4 binary. The script resets those disposable
# sources and mutates them through a fixture-only lifecycle (reset, logins,
# bookings, batches, policies, series, replays, mutations); the sources are
# disposable donor fixtures, not production data. Artifacts written:
#   OUT/stage1/export.json + manifest.json   (exactly 5 scoped receipts)
#   OUT/stage2/export.json + manifest.json   (exactly 8 scoped receipts)
#   OUT/stage3/export.json + manifest.json   (stage-3 workflow ledger, count
#     declared in the ledger comment below and asserted with equality)
# Only OUT_DIR is written (0700; exports/manifests 0600). Temporary run files
# live in OUT/tmp.XXXXXX (removed on exit); private python diagnostics live in
# OUT/diag (retained). Stdout carries names/statuses/counts/provenance only,
# never tokens, passwords, reservation bodies or export JSON. Every python
# tool's stderr is redirected into OUT/diag; assertion failures print a safe
# name plus a private-diagnostics pointer, count FAIL through expect()/
# pyassert(), and phase gates abort nonzero. No xtrace is used anywhere near
# credential operations. curl calls carry connect/max-time bounds. The script
# fails on unreachable sources, wrong-stage shapes or any missing assertion.
# Exports are fetched intact (exact HTTP file bytes) and never rewritten.
#
# Provenance (actual deployment, not from HTTP) is fed via:
#   STAGE1_IMAGE / STAGE1_CONTAINER / STAGE1_PORT / STAGE1_CID
#   STAGE2_IMAGE / STAGE2_CONTAINER / STAGE2_PORT / STAGE2_CID
#   STAGE3_IMAGE / STAGE3_CONTAINER / STAGE3_PORT / STAGE3_CID
# (container .Id values and deployed image IDs come from docker inspect
# before the run; IMAGE carries the deployed image sha, CONTAINER the actual
# container name, CID the actual container .Id).
# Fixture seed passwords default below and may be overridden with
# ADA_PASSWORD / BEA_PASSWORD (serialized safely as JSON, never interpolated
# into shell).
#
# Manifest schema (manifest_version 1, compatible with stage3-donor.sh for
# stage1/2; stage3 EXTENDS with metadata):
#   manifest_version: 1
#   source: {stage, reviewed_revision, stage_tree, image, container,
#     container_id, port}
#   fixture: {date, pair_seed_date|null, past_seed_date,
#     restaurant (COMPLETE original detail record incl. labels, hours,
#     combinable and managers), seeds[full seed request records in reset
#     order]}
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
#   metadata (stage3 only): {policies: FULL export policies map,
#     histories: FULL per-reference frozen entries, series: FULL map,
#     restaurant_revisions: FULL map}
#   post_snapshot_write: {reference, excluded_from_export true}
#   current_lists_before_post_snapshot_write: true
# The manifest is private supplemental metadata; the opaque export is stored
# byte-identical and never rewritten to inject it. A copied-artifact
# validator re-checks manifest/export agreement on the genuine files and
# proves sabotaged COPIES are rejected; sabotaged copies are always labeled
# as such and never described as producer output.
#
# Sabotage self-test: STAGE4_DONOR_SABOTAGE=1 injects a wrong expectation
# through the real expect() counter/exit path (counts FAIL, exits nonzero).
set -u

S1_URL=${1:?usage: sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR}
S2_URL=${2:?usage: sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR}
S3_URL=${3:?usage: sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR}
OUT=${4:?usage: sh stage-4/probes/stage4-donor.sh SRC1_URL SRC2_URL SRC3_URL OUT_DIR}
ADA_PASSWORD=${ADA_PASSWORD:-correct horse ada}
BEA_PASSWORD=${BEA_PASSWORD:-correct horse bea}
STAGE1_IMAGE=${STAGE1_IMAGE:-tablekeeper:s4-d-stage1}
STAGE1_CONTAINER=${STAGE1_CONTAINER:-tk-s4-d-src1}
STAGE1_PORT=${STAGE1_PORT:-9185}
STAGE1_CID=${STAGE1_CID:-unknown}
STAGE2_IMAGE=${STAGE2_IMAGE:-tablekeeper:s4-d-stage2}
STAGE2_CONTAINER=${STAGE2_CONTAINER:-tk-s4-d-src2}
STAGE2_PORT=${STAGE2_PORT:-9186}
STAGE2_CID=${STAGE2_CID:-unknown}
STAGE3_IMAGE=${STAGE3_IMAGE:-tablekeeper:s4-d-stage3}
STAGE3_CONTAINER=${STAGE3_CONTAINER:-tk-s4-d-src3}
STAGE3_PORT=${STAGE3_PORT:-9187}
STAGE3_CID=${STAGE3_CID:-unknown}

S1_REV=b298700f790c166cf7ce8d98d731c80093ecb9af
S1_TREE=8b8b28da1d7772bbc443ed4fccb57d8e5ed8530c
S2_REV=8812cdeaaa993cd944493c654e51d355cdd6b676
S2_TREE=9fee3dc7d0766091b3fb7cdbb521c6dfaf652b7f
S3_REV=e13272d90213be0914211ae6f6f0bfd5888900ff
S3_TREE=c783f9e08522a04a62bb11f9a0e3c485d077684f
PAST=2020-01-02
PAIRSEED=2020-01-09

PASS=0
FAIL=0
BASE=""
TMP=""
DIAG=""
S_OUT=""

cleanup() {
  # The script owns only its temp subdir OUT/tmp.XXXXXX; normal runs remove
  # it on exit. No other cleanup is performed: sources are disposable donor
  # fixtures left running for inspection, images are retained.
  if [ -n "$TMP" ] && [ -d "$TMP" ]; then
    rm -rf "$TMP"
  fi
}

expect() {
  name=$1; got=$2; want=$3
  if [ "${STAGE4_DONOR_SABOTAGE:-0}" = "1" ] && [ "$name" = "sabotage injected expectation" ]; then
    want="injected-wrong-value"
  fi
  if [ "$got" = "$want" ]; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name status (see private diagnostics)"
    echo "expect $name: got=$got want=$want" >>"$DIAG/stderr.log"
    FAIL=$((FAIL + 1))
  fi
}

pyassert() {
  name=$1; code=$2
  if python3 -c "$code" 2>>"$DIAG/stderr.log"; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
}

require_clean() {
  name=$1
  if [ "$FAIL" != 0 ]; then
    echo "ABORT: $name after $FAIL failures (see private diagnostics)"
    exit 1
  fi
}

CURL_OPTS="--connect-timeout 5 --max-time 25"

api_post() {
  path=$1; bodyfile=$2; outfile=$3; token=$4; key=$5
  if [ -n "$token" ]; then
    auth="Authorization: Bearer $token"
  else
    auth="X-No-Auth: 1"
  fi
  code=$(curl $CURL_OPTS -s -o "$outfile" -w "%{http_code}" -X POST "$BASE$path" -H "Content-Type: application/json" -H "$auth" -H "Idempotency-Key: $key" --data-binary "@$bodyfile" 2>>"$DIAG/stderr.log") || code="000"
  printf '%s' "$code"
}

api_post_nokey() {
  path=$1; bodyfile=$2; outfile=$3; token=$4
  if [ -n "$token" ]; then
    auth="Authorization: Bearer $token"
  else
    auth="X-No-Auth: 1"
  fi
  code=$(curl $CURL_OPTS -s -o "$outfile" -w "%{http_code}" -X POST "$BASE$path" -H "Content-Type: application/json" -H "$auth" --data-binary "@$bodyfile" 2>>"$DIAG/stderr.log") || code="000"
  printf '%s' "$code"
}

api_patch() {
  path=$1; bodyfile=$2; outfile=$3; token=$4
  code=$(curl $CURL_OPTS -s -o "$outfile" -w "%{http_code}" -X PATCH "$BASE$path" -H "Content-Type: application/json" -H "Authorization: Bearer $token" --data-binary "@$bodyfile" 2>>"$DIAG/stderr.log") || code="000"
  printf '%s' "$code"
}

api_get() {
  path=$1; outfile=$2; token=$3
  if [ -n "$token" ]; then
    auth="Authorization: Bearer $token"
  else
    auth="X-No-Auth: 1"
  fi
  code=$(curl $CURL_OPTS -s -o "$outfile" -w "%{http_code}" -X GET "$BASE$path" -H "$auth" 2>>"$DIAG/stderr.log") || code="000"
  printf '%s' "$code"
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
  elif [ "$STAGE" = "stage2" ]; then
    BASE=$S2_URL
  else
    BASE=$S3_URL
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
  cp "$TMP/$STAGE-fixture.json" "$TMP/$STAGE-fixture-orig.json"
  expect "$STAGE restaurant detail" "$(api_get /restaurants/r_anker "$TMP/$STAGE-restaurant.json" "")" 200
  require_clean "$STAGE restaurant detail"
  export STAGE
  pyassert "$STAGE public detail matches original projection" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; orig=[r for r in json.load(open(t+"/"+s+"-fixture-orig.json"))["restaurants"] if r["id"]=="r_anker"][0]; pub=json.load(open(t+"/"+s+"-restaurant.json")); assert pub["id"]==orig["id"] and pub["name"]==orig["name"] and pub["timezone"]==orig["timezone"] and pub["slot_minutes"]==orig["slot_minutes"] and pub["reservation_duration_minutes"]==orig["reservation_duration_minutes"] and pub["cancellation_cutoff_minutes"]==orig["cancellation_cutoff_minutes"] and pub["opening_hours"]==orig["opening_hours"] and pub["tables"]==orig["tables"] and [sorted(p) for p in pub.get("combinable",[])]==[sorted(p) for p in orig.get("combinable",[])]'

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
  expect "$STAGE export pre-failure" "$(api_get /_test/export "$TMP/$STAGE-prefail.json" "")" 200
  expect "$STAGE failed create 409" "$(api_post /reservations "$TMP/$STAGE-fail-body.json" "$TMP/$STAGE-fail-resp.json" "$TOK_B1" "$FAIL_KEY")" 409
  pyassert "$STAGE failed code table_unavailable" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-fail-resp.json")); assert d["error"]["code"]=="table_unavailable"'
  expect "$STAGE export post-failure" "$(api_get /_test/export "$TMP/$STAGE-postfail.json" "")" 200
  pyassert "$STAGE failure export unchanged" 'import os; assert open("'"$TMP"'/'"$STAGE"'-prefail.json","rb").read()==open("'"$TMP"'/'"$STAGE"'-postfail.json","rb").read()'
  export FAIL_KEY
  pyassert "$STAGE failed key unclaimed" 'import json,os; d=json.load(open("'"$TMP"'/'"$STAGE"'-postfail.json")); keys=[r["key"] for r in d["state"]["receipts"].values()]; assert os.environ["FAIL_KEY"] not in keys'
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
  expect "$STAGE export pre-replays" "$(api_get /_test/export "$TMP/$STAGE-prereplays.json" "")" 200
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
  expect "$STAGE export post-replays" "$(api_get /_test/export "$TMP/$STAGE-postreplays.json" "")" 200
  pyassert "$STAGE replays leave export unchanged" 'import os; assert open("'"$TMP"'/'"$STAGE"'-prereplays.json","rb").read()==open("'"$TMP"'/'"$STAGE"'-postreplays.json","rb").read()'

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
  python3 -c 'import hashlib; print(hashlib.sha256(open("'"$S_OUT"'/export.json","rb").read()).hexdigest())' > "$TMP/$STAGE-snapshot-sha.txt" 2>>"$DIAG/stderr.log"
  expect "$STAGE post-snapshot write" "$(api_post /reservations "$TMP/$STAGE-post-body.json" "$TMP/$STAGE-post-resp.json" "$TOK_B1" "$KEYP-post-01")" 201
  POST_REF=$(jget "$TMP/$STAGE-post-resp.json" "['reference']")
  export POST_REF
  pyassert "$STAGE saved file sha unchanged" 'import hashlib,os; assert hashlib.sha256(open(os.environ["S_OUT"]+"/export.json","rb").read()).hexdigest()==open(os.path.join(os.environ["TMP"],os.environ["STAGE"]+"-snapshot-sha.txt")).read().strip()'
  pyassert "$STAGE live post reference present" 'import json,os; d=json.load(open("'"$TMP"'/'"$STAGE"'-post-resp.json")); assert d["reference"]==os.environ["POST_REF"]'
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



# run_source3 STAGE OUT_SUBDIR IMAGE CONTAINER PORT CID REV TREE KEY_PREFIX
# Stage-3 donor: manager + two owners, dated policies, pair+singleton series,
# exception flags, cross-series moves batch, receipt capture with replays,
# failed-key reuse, pending-retry, snapshot isolation, extended manifest.
run_source3() {
  STAGE=$1
  S_OUT="$OUT/$2"
  IMAGE=$3
  CONTAINER=$4
  PORT=$5
  CID=$6
  REV=$7
  TREE=$8
  KEYP=$9
  BASE=$S3_URL
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
  WDATE=$(python3 -c 'import datetime,os; print((datetime.date.fromisoformat(os.environ["DATE"])+datetime.timedelta(days=21)).isoformat())' 2>>"$DIAG/stderr.log")
  export WDATE

  # 2. Fixture: manager Ada + owners; producer-valid historical seeds
  #    (past, off-grid, over-capacity, cancelled) with original policy-0
  #    terms; pair seed on a separate date.
  export ADA_PASSWORD BEA_PASSWORD PAST PAIRSEED
  python3 <<'PYEOF' > "$TMP/$STAGE-fixture.json" 2>>"$DIAG/stderr.log"
import json, os
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
  {"id": "seed-pair-1", "reference": "SEDPAIR", "user_id": "u_ada",
   "restaurant_id": "r_anker", "table_ids": ["t_2", "t_1"],
   "starts_at_local": os.environ["PAIRSEED"] + "T18:00", "party_size": 6},
  # Above publication maxima: separate historical restaurant r_big with terms no
  # publication could carry (slot 1441 > 1440, duration 1441, cutoff 10081,
  # capacity 101 > 100). Fixture-0 seed SEDMAX party 101 on a separate past
  # date; reset retains it without business grid/cap validation. r_anker keeps
  # ordinary 30/90 terms for all modern policy/series workflow.
  {"id": "seed-max-1", "reference": "SEDMAX", "user_id": "u_bea",
   "restaurant_id": "r_big", "table_id": "b_1",
   "starts_at_local": os.environ["PAIRSEED"] + "T00:00", "party_size": 101},
]
rest = {"id": "r_anker", "name": "Zum Anker", "timezone": "Europe/Berlin",
        "slot_minutes": 30, "reservation_duration_minutes": 90,
        "cancellation_cutoff_minutes": 120,
        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"},
                          {"weekday": "fri", "opens": "18:00", "closes": "23:30"}],
        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                   {"id": "t_2", "label": "2", "capacity": 4},
                   {"id": "t_3", "label": "3", "capacity": 4}],
        "combinable": [["t_1", "t_2"], ["t_2", "t_3"]],
        "manager_user_ids": ["u_ada"]}
rbig = {"id": "r_big", "name": "Big Hall", "timezone": "Europe/Berlin",
        "slot_minutes": 1441, "reservation_duration_minutes": 1441,
        "cancellation_cutoff_minutes": 10081,
        "opening_hours": [{"weekday": "thu", "opens": "00:00", "closes": "23:59"}],
        "tables": [{"id": "b_1", "label": "B1", "capacity": 101}],
        "combinable": [],
        "manager_user_ids": []}
print(json.dumps({
  "users": [
    {"id": "u_ada", "email": "ada@example.com", "password": os.environ["ADA_PASSWORD"], "display_name": "Ada"},
    {"id": "u_bea", "email": "bea@example.com", "password": os.environ["BEA_PASSWORD"], "display_name": "Bea"}],
  "restaurants": [rest, rbig],
  "reservations": seeds}))
PYEOF
  expect "$STAGE reset fixture" "$(api_post /_test/reset "$TMP/$STAGE-fixture.json" "$TMP/$STAGE-reset.out" "" "-")" 204
  cp "$TMP/$STAGE-fixture.json" "$TMP/$STAGE-fixture-orig.json"
  require_clean "$STAGE fixture reset"
  expect "$STAGE restaurant detail" "$(api_get /restaurants/r_anker "$TMP/$STAGE-restaurant.json" "")" 200
  require_clean "$STAGE restaurant detail"
  export STAGE
  pyassert "$STAGE public detail matches original projection" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; orig=[r for r in json.load(open(t+"/"+s+"-fixture-orig.json"))["restaurants"] if r["id"]=="r_anker"][0]; pub=json.load(open(t+"/"+s+"-restaurant.json")); assert pub["id"]==orig["id"] and pub["name"]==orig["name"] and pub["timezone"]==orig["timezone"] and pub["slot_minutes"]==orig["slot_minutes"] and pub["reservation_duration_minutes"]==orig["reservation_duration_minutes"] and pub["cancellation_cutoff_minutes"]==orig["cancellation_cutoff_minutes"] and pub["opening_hours"]==orig["opening_hours"] and pub["tables"]==orig["tables"] and [sorted(p) for p in pub.get("combinable",[])]==[sorted(p) for p in orig.get("combinable",[])]'

  # 3. Sessions: two tokens per user; hash login + multi-session proof.
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

  # 4. Pending-retry booking (Bea, stays confirmed; donor analogue of a lost
  #    response whose original receipt is retried after migration).
  LOST_KEY=$KEYP-lost-01
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-lost-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE lost booking create" "$(api_post /reservations "$TMP/$STAGE-lost-body.json" "$TMP/$STAGE-lost-resp.json" "$TOK_B1" "$LOST_KEY")" 201
  require_clean "$STAGE lost booking"
  LOST_REF=$(jget "$TMP/$STAGE-lost-resp.json" "['reference']")

  # 5. Two dated policies with supersession (manager Ada; Bea is 403).
  #    Policy 1: effective DATE, hourly grid, generous capacities.
  #    Policy 2: same effective DATE, supersedes for future decisions
  #    (greater policy_version wins the tie).
  python3 -c 'import os; print("{\"effective_from\":\""+os.environ["DATE"]+"\",\"slot_minutes\":60,\"reservation_duration_minutes\":60,\"cancellation_cutoff_minutes\":60,\"opening_hours\":[{\"weekday\":\"thu\",\"opens\":\"18:00\",\"closes\":\"23:00\"}],\"capacities\":{\"t_1\":4,\"t_2\":6,\"t_3\":4}}")' > "$TMP/$STAGE-pol1-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE nonmanager publish 403" "$(api_post /restaurants/r_anker/policies "$TMP/$STAGE-pol1-body.json" "$TMP/$STAGE-pol1-deny.json" "$TOK_B1" "$KEYP-poldeny")" 403
  POL1_KEY=$KEYP-pol-01
  expect "$STAGE publish policy 1" "$(api_post /restaurants/r_anker/policies "$TMP/$STAGE-pol1-body.json" "$TMP/$STAGE-pol1-resp.json" "$TOK_A1" "$POL1_KEY")" 201
  pyassert "$STAGE policy 1 version" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-pol1-resp.json")); assert d["policy_version"]==1'
  python3 -c 'import os; print("{\"effective_from\":\""+os.environ["DATE"]+"\",\"slot_minutes\":60,\"reservation_duration_minutes\":60,\"cancellation_cutoff_minutes\":30,\"opening_hours\":[{\"weekday\":\"thu\",\"opens\":\"18:00\",\"closes\":\"23:00\"}],\"capacities\":{\"t_1\":4,\"t_2\":6,\"t_3\":8}}")' > "$TMP/$STAGE-pol2-body.json" 2>>"$DIAG/stderr.log"
  POL2_KEY=$KEYP-pol-02
  expect "$STAGE publish policy 2" "$(api_post /restaurants/r_anker/policies "$TMP/$STAGE-pol2-body.json" "$TMP/$STAGE-pol2-resp.json" "$TOK_A1" "$POL2_KEY")" 201
  pyassert "$STAGE policy 2 version" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-pol2-resp.json")); assert d["policy_version"]==2'
  require_clean "$STAGE policies"

  # 6. Canonical pair anchor + singleton anchor under the selected policy
  #    (hourly grid 18:00/19:00; pair capacity 10; singleton 8 on t_3).
  PA_KEY=$KEYP-cr-pair
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_ids\":[\"t_2\",\"t_1\"],\"starts_at_local\":\""+os.environ["DATE"]+"T19:00\",\"party_size\":8}")' > "$TMP/$STAGE-pair-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE pair anchor create" "$(api_post /reservations "$TMP/$STAGE-pair-body.json" "$TMP/$STAGE-pair-resp.json" "$TOK_B1" "$PA_KEY")" 201
  require_clean "$STAGE pair anchor"
  pyassert "$STAGE pair canonical order" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-pair-resp.json")); assert d["table_ids"]==["t_1","t_2"]; assert "table_id" not in d'
  REF_PAIR=$(jget "$TMP/$STAGE-pair-resp.json" "['reference']")
  SA_KEY=$KEYP-cr-single
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\""+os.environ["DATE"]+"T20:00\",\"party_size\":4}")' > "$TMP/$STAGE-single-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE singleton anchor create" "$(api_post /reservations "$TMP/$STAGE-single-body.json" "$TMP/$STAGE-single-resp.json" "$TOK_B1" "$SA_KEY")" 201
  require_clean "$STAGE singleton anchor"
  REF_SINGLE=$(jget "$TMP/$STAGE-single-resp.json" "['reference']")

  # 7. Adopt TWO real series (count 3, interval 1 week each).
  SX1_KEY=$KEYP-sx-01
  python3 -c 'print("{\"anchor_reference\":\"'"$REF_PAIR"'\",\"count\":3,\"interval_weeks\":1}")' > "$TMP/$STAGE-sx1-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE adopt pair series" "$(api_post /series "$TMP/$STAGE-sx1-body.json" "$TMP/$STAGE-sx1-resp.json" "$TOK_B1" "$SX1_KEY")" 201
  require_clean "$STAGE adopt pair series"
  SID1=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx1-resp.json"))["series_id"])' 2>>"$DIAG/stderr.log")
  G1=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx1-resp.json"))["occurrences"][1]["reference"])' 2>>"$DIAG/stderr.log")
  G2=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx1-resp.json"))["occurrences"][2]["reference"])' 2>>"$DIAG/stderr.log")
  pyassert "$STAGE series1 anchor retained" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-sx1-resp.json")); before=json.load(open("'"$TMP"'/'"$STAGE"'-pair-resp.json")); assert d["occurrences"][0]["reservation"]==before and d["occurrences"][0]["reference"]==before["reference"]'
  pyassert "$STAGE pair anchor dated terms" 'import json,datetime; t=json.load(open("'"$TMP"'/'"$STAGE"'-pair-resp.json")); at=t["accepted_terms"]; assert at["policy_version"]==2 and at["slot_minutes"]==60 and at["reservation_duration_minutes"]==60 and at["cancellation_cutoff_minutes"]==30 and at["capacities"]=={"t_1":4,"t_2":6,"t_3":8} and len(at["opening_hours"])==1; st=datetime.datetime.fromisoformat(t["starts_at"]); en=datetime.datetime.fromisoformat(t["ends_at"]); assert (en-st).total_seconds()==3600, (st,en)'
  SX2_KEY=$KEYP-sx-02
  python3 -c 'print("{\"anchor_reference\":\"'"$REF_SINGLE"'\",\"count\":3,\"interval_weeks\":1}")' > "$TMP/$STAGE-sx2-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE adopt singleton series" "$(api_post /series "$TMP/$STAGE-sx2-body.json" "$TMP/$STAGE-sx2-resp.json" "$TOK_B1" "$SX2_KEY")" 201
  require_clean "$STAGE adopt singleton series"
  SID2=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx2-resp.json"))["series_id"])' 2>>"$DIAG/stderr.log")
  H1=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx2-resp.json"))["occurrences"][1]["reference"])' 2>>"$DIAG/stderr.log")
  H2=$(python3 -c 'import json; print(json.load(open("'"$TMP"'/'"$STAGE"'-sx2-resp.json"))["occurrences"][2]["reference"])' 2>>"$DIAG/stderr.log")
  pyassert "$STAGE series2 anchor retained" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-sx2-resp.json")); before=json.load(open("'"$TMP"'/'"$STAGE"'-single-resp.json")); assert d["occurrences"][0]["reservation"]==before and d["occurrences"][0]["reference"]==before["reference"]'
  pyassert "$STAGE single anchor dated terms" 'import json,datetime; t=json.load(open("'"$TMP"'/'"$STAGE"'-single-resp.json")); at=t["accepted_terms"]; assert at["policy_version"]==2 and at["capacities"]["t_3"]==8 and at["slot_minutes"]==60 and at["reservation_duration_minutes"]==60; st=datetime.datetime.fromisoformat(t["starts_at"]); en=datetime.datetime.fromisoformat(t["ends_at"]); assert (en-st).total_seconds()==3600'
  export SID1 SID2 G1 G2 H1 H2

  # 8. Exception flags: PATCH generated member G1 (exception=true, series+1);
  #    cancel G1 (flag retained); cancel unexceptioned H2 (no new exception).
  echo '{"party_size":7}' > "$TMP/$STAGE-patch-g1.json"
  expect "$STAGE patch G1" "$(api_patch /reservations/"$G1" "$TMP/$STAGE-patch-g1.json" "$TMP/$STAGE-patch-g1-resp.json" "$TOK_B1")" 200
  : > "$TMP/$STAGE-empty.json"
  expect "$STAGE cancel G1" "$(api_post_nokey /reservations/"$G1"/cancel "$TMP/$STAGE-empty.json" "$TMP/$STAGE-cancel-g1-resp.json" "$TOK_B1")" 200
  expect "$STAGE cancel H2" "$(api_post_nokey /reservations/"$H2"/cancel "$TMP/$STAGE-empty.json" "$TMP/$STAGE-cancel-h2-resp.json" "$TOK_B1")" 200
  expect "$STAGE series1 flags" "$(api_get /series/"$SID1" "$TMP/$STAGE-s1-flags.json" "$TOK_B1")" 200
  pyassert "$STAGE G1 exception retained" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-s1-flags.json")); f=[o["exception"] for o in d["occurrences"]]; assert f==[False,True,False], f'
  expect "$STAGE series2 flags" "$(api_get /series/"$SID2" "$TMP/$STAGE-s2-flags.json" "$TOK_B1")" 200
  pyassert "$STAGE H2 no exception" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-s2-flags.json")); f=[o["exception"] for o in d["occurrences"]]; assert f==[False,False,False], f'

  # 9. Cross-series idempotent moves batch: party change on G2 (series 1)
  #    and H1 (series 2). Each affected series +1 once, restaurant +1 once.
  MX_KEY=$KEYP-mx-01
  python3 -c 'print("{\"moves\":[{\"reference\":\"'"$G2"'\",\"party_size\":9},{\"reference\":\"'"$H1"'\",\"party_size\":3}]}")' > "$TMP/$STAGE-mx-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE export pre-batch" "$(api_get /_test/export "$TMP/$STAGE-prebatch.json" "")" 200
  expect "$STAGE cross-series batch" "$(api_post /reservation-moves "$TMP/$STAGE-mx-body.json" "$TMP/$STAGE-mx-resp.json" "$TOK_B1" "$MX_KEY")" 201
  require_clean "$STAGE cross-series batch"
  expect "$STAGE export post-batch" "$(api_get /_test/export "$TMP/$STAGE-postbatch.json" "")" 200
  pyassert "$STAGE batch restaurant counter once" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); assert b["state"]["restaurant_revisions"]["r_anker"]==a["state"]["restaurant_revisions"]["r_anker"]+1'
  pyassert "$STAGE batch series once each" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); assert b["state"]["series"]["'"$SID1"'"]["revision"]==a["state"]["series"]["'"$SID1"'"]["revision"]+1 and b["state"]["series"]["'"$SID2"'"]["revision"]==a["state"]["series"]["'"$SID2"'"]["revision"]+1'
  pyassert "$STAGE batch moved are exceptions" 'import json; b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); f1={m["reference"]: m["exception"] for m in b["state"]["series"]["'"$SID1"'"]["members"]}; f2={m["reference"]: m["exception"] for m in b["state"]["series"]["'"$SID2"'"]["members"]}; assert f1["'"$G2"'"] is True and f2["'"$H1"'"] is True'
  pyassert "$STAGE batch only two members changed" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); ar=a["state"]["reservations"]; br=b["state"]["reservations"]; assert set(ar)==set(br); changed=[r for r in ar if ar[r]!=br[r]]; assert sorted(changed)==sorted(["'"$G2"'","'"$H1"'"]), changed'
  pyassert "$STAGE batch full counter map" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json"))["state"]["restaurant_revisions"]; b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json"))["state"]["restaurant_revisions"]; assert set(a)==set(b)=={"r_anker","r_big"} and b["r_anker"]==a["r_anker"]+1 and b["r_big"]==a["r_big"]==0'
  pyassert "$STAGE batch both members full records" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); ar=a["state"]["reservations"]; br=b["state"]["reservations"]; assert set(ar)==set(br) and sorted([r for r in ar if ar[r]!=br[r]])==sorted(["'"$G2"'","'"$H1"'"]); assert sorted([k for k in set(ar["'"$G2"'"])|set(br["'"$G2"'"]) if ar["'"$G2"'"].get(k)!=br["'"$G2"'"].get(k)])==["party_size","revision"] and sorted([k for k in set(ar["'"$H1"'"])|set(br["'"$H1"'"]) if ar["'"$H1"'"].get(k)!=br["'"$H1"'"].get(k)])==["party_size","revision"]'
  pyassert "$STAGE batch both Changed entries" 'import json,datetime; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); ah=a["state"]["histories"]; bh=b["state"]["histories"]; ar=a["state"]["reservations"]; br=b["state"]["reservations"]; e1=bh["'"$G2"'"][-1]; e2=bh["'"$H1"'"][-1]; assert len(bh["'"$G2"'"])==len(ah["'"$G2"'"])+1 and bh["'"$G2"'"][:-1]==ah["'"$G2"'"] and len(bh["'"$H1"'"])==len(ah["'"$H1"'"])+1 and bh["'"$H1"'"][:-1]==ah["'"$H1"'"]; assert e1["event"]=="changed" and e1["seq"]==len(ah["'"$G2"'"])+1 and e1["revision"]==br["'"$G2"'"]["revision"] and e1["accepted_terms"]==br["'"$G2"'"]["accepted_terms"] and e2["event"]=="changed" and e2["seq"]==len(ah["'"$H1"'"])+1 and e2["revision"]==br["'"$H1"'"]["revision"] and e2["accepted_terms"]==br["'"$H1"'"]["accepted_terms"]; assert [c["field"] for c in e1["changes"]]==["party_size"] and e1["changes"][0]["from"]==ar["'"$G2"'"]["party_size"] and e1["changes"][0]["to"]==br["'"$G2"'"]["party_size"] and [c["field"] for c in e2["changes"]]==["party_size"] and e2["changes"][0]["from"]==ar["'"$H1"'"]["party_size"] and e2["changes"][0]["to"]==br["'"$H1"'"]["party_size"]; assert datetime.datetime.fromisoformat(e1["at"]).tzinfo is not None and datetime.datetime.fromisoformat(e2["at"]).tzinfo is not None'
  cat > "$TMP/$STAGE-batch-check.py" <<'PYEOF'
import copy, json, os
t = os.environ["TMP"]
s = os.environ["STAGE"]
a = json.load(open(t + "/" + s + "-prebatch.json"))
b = json.load(open(t + "/" + s + "-postbatch.json"))
sa = a["state"]["series"]
sb = b["state"]["series"]
sid1 = os.environ["SID1"]
sid2 = os.environ["SID2"]
g2 = os.environ["G2"]
h1 = os.environ["H1"]
# C1: independent expected POST map from a deep copy of PRE: exactly the two
# affected series gain one revision; G2/H1 flip false->true; every prior true
# flag and all other fields retained verbatim.
exp = copy.deepcopy(sa)
assert sid1 in exp and sid2 in exp, "target series missing"
exp[sid1]["revision"] += 1
exp[sid2]["revision"] += 1
hits = []
for sz in exp.values():
    for m in sz["members"]:
        if m["reference"] in (g2, h1):
            assert m["exception"] is False, m
            m["exception"] = True
            hits.append(m["reference"])
assert sorted(hits) == sorted([g2, h1]), hits
assert sb == exp, "whole POST series map differs from expected"
print("series whole-map compared: %d" % len(sb))
# C2: receipt keyset exactly PRE plus ONE; every prior receipt object equal.
ar = a["state"]["receipts"]
br = b["state"]["receipts"]
new = [k for k in br if k not in ar]
assert len(new) == 1, new
assert set(br) == set(ar) | set(new), "keyset drift"
for k in ar:
    assert br[k] == ar[k], k
# New receipt keeps the existing owner/method/path/key/status bindings plus
# canonical body and raw-response bindings to the captured first-201 files.
r = br[new[0]]
assert r["user_id"] == "u_bea" and r["method"] == "POST", r
assert r["path"] == "/reservation-moves", r
assert r["key"] == os.environ["MX_KEY"] and r["status"] == 201, r
with open(t + "/" + s + "-mx-body.json") as f:
    sent_body = f.read()
with open(t + "/" + s + "-mx-resp.json") as f:
    first_resp = f.read()
assert json.loads(r["body"]) == json.loads(sent_body), "body canonical"
assert json.loads(r["response"]) == json.loads(first_resp), "resp canonical"
assert r["response"] == first_resp, "resp raw bytes"
print("receipts compared: %d prior plus one" % len(ar))
PYEOF
  export SID1 SID2 G1 G2 H1 H2 MX_KEY
  if python3 "$TMP/$STAGE-batch-check.py" 2>>"$DIAG/stderr.log"; then
    echo "PASS: $STAGE batch whole series map and receipts"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE batch whole series map and receipts (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  pyassert "$STAGE batch unrelated untouched" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); skip={"'"$G2"'","'"$H1"'"}; assert a["state"]["users"]==b["state"]["users"] and a["state"]["tokens"]==b["state"]["tokens"] and a["state"]["restaurants"]==b["state"]["restaurants"] and a["state"]["policies"]==b["state"]["policies"]; ar=a["state"]["reservations"]; br=b["state"]["reservations"]; assert all(ar[r]==br[r] for r in ar if r not in skip); ah=a["state"]["histories"]; bh=b["state"]["histories"]; assert all(ah[r]==bh[r] for r in ah if r not in skip)'
  pyassert "$STAGE batch receipt scoped" 'import json; b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); recs=[r for r in b["state"]["receipts"].values() if r["method"]=="POST" and r["path"]=="/reservation-moves" and r["key"]=="'"$MX_KEY"'"]; assert len(recs)==1 and recs[0]["user_id"]=="u_bea" and recs[0]["status"]==201'
  pyassert "$STAGE batch schedules retained" 'import json; a=json.load(open("'"$TMP"'/'"$STAGE"'-prebatch.json")); b=json.load(open("'"$TMP"'/'"$STAGE"'-postbatch.json")); am={m["reference"]: m for sz in a["state"]["series"].values() for m in sz["members"]}; bm={m["reference"]: m for sz in b["state"]["series"].values() for m in sz["members"]}; assert set(am)==set(bm); assert all(am[r]["scheduled_date"]==bm[r]["scheduled_date"] and am[r]["index"]==bm[r]["index"] for r in am)'

  # 10. Failed create key (occupied slot 409, no receipt), then reuse 201 +
  #     replay 200 identical. One permanently failed key stays absent.
  FAIL_KEY=$KEYP-fail-01
  STUCK_KEY=$KEYP-fail-02
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_3\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-fail-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE export pre-failure" "$(api_get /_test/export "$TMP/$STAGE-prefail.json" "")" 200
  expect "$STAGE failed create 409" "$(api_post /reservations "$TMP/$STAGE-fail-body.json" "$TMP/$STAGE-fail-resp.json" "$TOK_A1" "$FAIL_KEY")" 409
  pyassert "$STAGE failed code table_unavailable" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-fail-resp.json")); assert d["error"]["code"]=="table_unavailable"'
  expect "$STAGE export post-failure" "$(api_get /_test/export "$TMP/$STAGE-postfail.json" "")" 200
  pyassert "$STAGE failure export unchanged" 'import os; assert open("'"$TMP"'/'"$STAGE"'-prefail.json","rb").read()==open("'"$TMP"'/'"$STAGE"'-postfail.json","rb").read()'
  export FAIL_KEY
  pyassert "$STAGE failed key unclaimed" 'import json,os; d=json.load(open("'"$TMP"'/'"$STAGE"'-postfail.json")); keys=[r["key"] for r in d["state"]["receipts"].values()]; assert os.environ["FAIL_KEY"] not in keys'
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-fail-reuse-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE failed key reused 201" "$(api_post /reservations "$TMP/$STAGE-fail-reuse-body.json" "$TMP/$STAGE-fail-reuse-resp.json" "$TOK_A1" "$FAIL_KEY")" 201
  REF_REUSE=$(jget "$TMP/$STAGE-fail-reuse-resp.json" "['reference']")
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["DATE"]+"T18:00\",\"party_size\":\"many\"}")' > "$TMP/$STAGE-stuck-body.json" 2>>"$DIAG/stderr.log"
  expect "$STAGE stuck key stays failed" "$(api_post /reservations "$TMP/$STAGE-stuck-body.json" "$TMP/$STAGE-stuck-resp.json" "$TOK_A1" "$STUCK_KEY")" 422

  # 11. Real mutation AFTER capture: patch the reuse booking so current
  #     differs from its original receipt (replay must still return old).
  echo '{"party_size":1}' > "$TMP/$STAGE-mutate-reuse.json"
  echo '{"party_size":3}' > "$TMP/$STAGE-mutate-lost.json"
  expect "$STAGE mutate lost" "$(api_patch /reservations/"$LOST_REF" "$TMP/$STAGE-mutate-lost.json" "$TMP/$STAGE-mutate-lost-resp.json" "$TOK_B1")" 200
  pyassert "$STAGE lost current differs from receipt" 'import json; cur=json.load(open("'"$TMP"'/'"$STAGE"'-mutate-lost-resp.json")); orig=json.load(open("'"$TMP"'/'"$STAGE"'-lost-resp.json")); assert cur["party_size"]==3 and cur!=orig and cur["reference"]==orig["reference"]'
  expect "$STAGE mutate reuse" "$(api_patch /reservations/"$REF_REUSE" "$TMP/$STAGE-mutate-reuse.json" "$TMP/$STAGE-mutate-reuse-resp.json" "$TOK_A1")" 200
  pyassert "$STAGE reuse current differs" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-mutate-reuse-resp.json")); assert d["party_size"]==1'

  # 12. Replay ALL stored receipts: 200 byte-identical originals, after all
  #     later mutations and cancellations.
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
  expect "$STAGE export pre-replays" "$(api_get /_test/export "$TMP/$STAGE-prereplays.json" "")" 200
  replay_check lost "$LOST_KEY" "$TMP/$STAGE-lost-body.json" "$TMP/$STAGE-lost-resp.json" /reservations "$TOK_B1"
  replay_check pol1 "$POL1_KEY" "$TMP/$STAGE-pol1-body.json" "$TMP/$STAGE-pol1-resp.json" /restaurants/r_anker/policies "$TOK_A1"
  replay_check pol2 "$POL2_KEY" "$TMP/$STAGE-pol2-body.json" "$TMP/$STAGE-pol2-resp.json" /restaurants/r_anker/policies "$TOK_A1"
  replay_check pair-anchor "$PA_KEY" "$TMP/$STAGE-pair-body.json" "$TMP/$STAGE-pair-resp.json" /reservations "$TOK_B1"
  replay_check single-anchor "$SA_KEY" "$TMP/$STAGE-single-body.json" "$TMP/$STAGE-single-resp.json" /reservations "$TOK_B1"
  replay_check series1 "$SX1_KEY" "$TMP/$STAGE-sx1-body.json" "$TMP/$STAGE-sx1-resp.json" /series "$TOK_B1"
  replay_check series2 "$SX2_KEY" "$TMP/$STAGE-sx2-body.json" "$TMP/$STAGE-sx2-resp.json" /series "$TOK_B1"
  export SID1 SID2 G1 G2 H1 H2
  replay_check fail-reuse "$FAIL_KEY" "$TMP/$STAGE-fail-reuse-body.json" "$TMP/$STAGE-fail-reuse-resp.json" /reservations "$TOK_A1"
  expect "$STAGE export post-replays" "$(api_get /_test/export "$TMP/$STAGE-postreplays.json" "")" 200
  pyassert "$STAGE replays leave export unchanged" 'import os; assert open("'"$TMP"'/'"$STAGE"'-prereplays.json","rb").read()==open("'"$TMP"'/'"$STAGE"'-postreplays.json","rb").read()'

  # 13. Current-state reads with the correct owner's token, plus isolation:
  #     cross-owner 404, missing token 401. History/decision/series views.
  expect "$STAGE lost lookup" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-lookup-lost.json" "$TOK_B1")" 200
  expect "$STAGE cross-owner lookup 404" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-xlookup.json" "$TOK_A1")" 404
  expect "$STAGE unauthenticated lookup 401" "$(api_get /reservations/"$LOST_REF" "$TMP/$STAGE-nolookup.json" "")" 401
  expect "$STAGE reuse lookup" "$(api_get /reservations/"$REF_REUSE" "$TMP/$STAGE-lookup-reuse.json" "$TOK_A1")" 200
  pyassert "$STAGE reuse mutated current" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-reuse.json")); assert d["party_size"]==1'
  expect "$STAGE history G2" "$(api_get /reservations/"$G2"/history "$TMP/$STAGE-hist-g2.json" "$TOK_B1")" 200
  pyassert "$STAGE history G2 entries" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-hist-g2.json")); assert d["reference"]=="'"$G2"'" and [e["seq"] for e in d["entries"]]==[1,2] and all("accepted_terms" in e and "revision" in e for e in d["entries"])'
  expect "$STAGE decision G2" "$(api_get /reservations/"$G2"/decision "$TMP/$STAGE-dec-g2.json" "$TOK_B1")" 200
  pyassert "$STAGE decision current terms" 'import json; h=json.load(open("'"$TMP"'/'"$STAGE"'-hist-g2.json")); d=json.load(open("'"$TMP"'/'"$STAGE"'-dec-g2.json")); assert d["revision"]==h["entries"][-1]["revision"] and d["accepted_terms"]==h["entries"][-1]["accepted_terms"]'
  expect "$STAGE history H1" "$(api_get /reservations/"$H1"/history "$TMP/$STAGE-hist-h1.json" "$TOK_B1")" 200
  expect "$STAGE decision H1" "$(api_get /reservations/"$H1"/decision "$TMP/$STAGE-dec-h1.json" "$TOK_B1")" 200
  expect "$STAGE history cross-owner 404" "$(api_get /reservations/"$G2"/history "$TMP/$STAGE-xhist.json" "$TOK_A1")" 404
  expect "$STAGE G1 lookup" "$(api_get /reservations/"$G1" "$TMP/$STAGE-lookup-g1.json" "$TOK_B1")" 200
  pyassert "$STAGE G1 cancelled" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-g1.json")); assert d["status"]=="cancelled"'
  expect "$STAGE H2 lookup" "$(api_get /reservations/"$H2" "$TMP/$STAGE-lookup-h2.json" "$TOK_B1")" 200
  # Every series member by reference (anchors + generated): full GET set so
  # the live agreement covers all occurrences, not just mutated ones.
  expect "$STAGE pair anchor lookup" "$(api_get /reservations/"$REF_PAIR" "$TMP/$STAGE-lookup-pairanchor.json" "$TOK_B1")" 200
  expect "$STAGE single anchor lookup" "$(api_get /reservations/"$REF_SINGLE" "$TMP/$STAGE-lookup-singleanchor.json" "$TOK_B1")" 200
  expect "$STAGE G2 lookup" "$(api_get /reservations/"$G2" "$TMP/$STAGE-lookup-g2.json" "$TOK_B1")" 200
  expect "$STAGE H1 lookup" "$(api_get /reservations/"$H1" "$TMP/$STAGE-lookup-h1.json" "$TOK_B1")" 200
  pyassert "$STAGE H2 cancelled" 'import json; d=json.load(open("'"$TMP"'/'"$STAGE"'-lookup-h2.json")); assert d["status"]=="cancelled"'
  expect "$STAGE all tokens valid" "$(api_get /reservations "$TMP/$STAGE-list-b1.json" "$TOK_B1")" 200
  expect "$STAGE second bea token valid" "$(api_get /reservations "$TMP/$STAGE-list-b2.json" "$TOK_B2")" 200
  expect "$STAGE ada token valid" "$(api_get /reservations "$TMP/$STAGE-list-a1.json" "$TOK_A1")" 200
  expect "$STAGE second ada token valid" "$(api_get /reservations "$TMP/$STAGE-list-a2.json" "$TOK_A2")" 200
  if cmp -s "$TMP/$STAGE-list-a1.json" "$TMP/$STAGE-list-a2.json"; then
    echo "PASS: $STAGE ada sessions identical lists"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE ada sessions identical lists (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  expect "$STAGE series1 current" "$(api_get /series/"$SID1" "$TMP/$STAGE-s1-cur.json" "$TOK_B1")" 200
  expect "$STAGE series2 current" "$(api_get /series/"$SID2" "$TMP/$STAGE-s2-cur.json" "$TOK_B1")" 200
  pyassert "$STAGE series current bound" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; s1=json.load(open(t+"/"+s+"-s1-cur.json")); s2=json.load(open(t+"/"+s+"-s2-cur.json")); assert s1["revision"]>=3 and s2["revision"]>=2; assert [o["index"] for o in s1["occurrences"]]==[0,1,2] and [o["index"] for o in s2["occurrences"]]==[0,1,2]; assert [o["exception"] for o in s1["occurrences"]]==[False,True,True] and [o["exception"] for o in s2["occurrences"]]==[False,True,False]; refs1=[o["reference"] for o in s1["occurrences"]]; refs2=[o["reference"] for o in s2["occurrences"]]; assert len(set(refs1+refs2))==6; assert all(set(o["reservation"]) >= {"reference","revision","accepted_terms","status","starts_at_local"} for o in s1["occurrences"]+s2["occurrences"])'
  if cmp -s "$TMP/$STAGE-list-b1.json" "$TMP/$STAGE-list-b2.json"; then
    echo "PASS: $STAGE bea sessions identical lists"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE bea sessions identical lists (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi
  # Seed lookups with owner tokens (refs from the submitted fixture).
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
  expect "$STAGE history SEDPST" "$(api_get /reservations/SEDPST/history "$TMP/$STAGE-hist-sedpst.json" "$TOK_A1")" 200
  expect "$STAGE history SEDMAX" "$(api_get /reservations/SEDMAX/history "$TMP/$STAGE-hist-sedmax.json" "$TOK_B1")" 200
  pyassert "$STAGE policy0 seeds frozen" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; h=json.load(open(t+"/"+s+"-hist-sedpst.json")); assert h["reference"]=="SEDPST" and len(h["entries"])==1 and h["entries"][0]["event"]=="created" and h["entries"][0]["revision"]==1 and [c["field"] for c in h["entries"][0]["changes"]]==["table_id","starts_at_local","party_size"] and all(c["from"] is None for c in h["entries"][0]["changes"]); past=json.load(open(t+"/"+s+"-get-SEDPST.json")); assert past["revision"]==1 and past["accepted_terms"]["policy_version"]==0; mx=json.load(open(t+"/"+s+"-get-SEDMAX.json")); assert mx["party_size"]==101 and mx["revision"]==1 and mx["restaurant_id"]=="r_big" and mx["table_id"]=="b_1" and mx["table_ids"]==["b_1"]; at=mx["accepted_terms"]; assert at["policy_version"]==0 and at["slot_minutes"]==1441 and at["reservation_duration_minutes"]==1441 and at["cancellation_cutoff_minutes"]==10081 and at["capacities"]=={"b_1":101} and at["opening_hours"]==[{"weekday":"thu","opens":"00:00","closes":"23:59"}]; import datetime as _dt; _st=_dt.datetime.fromisoformat(mx["starts_at"]); _en=_dt.datetime.fromisoformat(mx["ends_at"]); assert (_en-_st).total_seconds()==1441*60, (mx["starts_at"],mx["ends_at"]); hx=json.load(open(t+"/"+s+"-hist-sedmax.json")); assert hx["reference"]=="SEDMAX" and len(hx["entries"])==1 and hx["entries"][0]["event"]=="created" and hx["entries"][0]["revision"]==1 and [c["field"] for c in hx["entries"][0]["changes"]]==["table_id","starts_at_local","party_size"] and all(c["from"] is None for c in hx["entries"][0]["changes"]) and hx["entries"][0]["changes"][0]["to"]=="b_1" and hx["entries"][0]["changes"][2]["to"]==101;cxd=json.load(open(t+"/"+s+"-get-SEDCXD.json")); assert cxd["status"]=="cancelled" and cxd["revision"]>=1'
  # Modern shape guard: revision + six-key accepted_terms, singleton
  # table_id iff exactly one member.
  export STAGE
  pyassert "$STAGE modern response shape" 'import json,os; t=os.environ["TMP"]; s=os.environ["STAGE"]; recs=[json.load(open(t+"/"+s+"-lookup-lost.json")),json.load(open(t+"/"+s+"-lookup-reuse.json"))]; assert all(r["revision"]>=1 and set(r["accepted_terms"])=={"policy_version","slot_minutes","reservation_duration_minutes","cancellation_cutoff_minutes","opening_hours","capacities"} for r in recs); assert all(("table_id" in r)==(len(r["table_ids"])==1) for r in recs)'

  # 14. Final current lists BEFORE the deliberate post-snapshot write.
  expect "$STAGE final ada list" "$(api_get /reservations "$TMP/$STAGE-list-final-ada.json" "$TOK_A1")" 200
  expect "$STAGE final bea list" "$(api_get /reservations "$TMP/$STAGE-list-final-bea.json" "$TOK_B1")" 200

  # 15. Atomic export snapshot; exact scoped receipt ledger; stuck key
  #     absent. Write ledger: 2 policy + 3 reservation (lost/pair/single) +
  #     2 series + 1 moves + 1 fail-reuse = 9 scoped receipts.
  expect "$STAGE export" "$(api_get /_test/export "$S_OUT/export.json" "")" 200
  require_clean "$STAGE export"
  chmod 0600 "$S_OUT/export.json"
  export S_OUT
  pyassert "$STAGE export envelope" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); assert d["track"]=="tablekeeper" and d["format_version"]==1 and isinstance(d["state"],dict)'
  export STUCK_KEY
  pyassert "$STAGE stuck key absent from receipts" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); keys=[r["key"] for r in d["state"]["receipts"].values()]; assert os.environ["STUCK_KEY"] not in keys'
  export LOST_KEY POL1_KEY POL2_KEY PA_KEY SA_KEY SX1_KEY SX2_KEY MX_KEY FAIL_KEY TOK_A1 TOK_B1
  pyassert "$STAGE nine scoped receipts" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); got={(r["user_id"],r["method"],r["path"],r["key"]) for r in d["state"]["receipts"].values()}; want={("u_bea","POST","/reservations",os.environ["LOST_KEY"]),("u_ada","POST","/restaurants/r_anker/policies",os.environ["POL1_KEY"]),("u_ada","POST","/restaurants/r_anker/policies",os.environ["POL2_KEY"]),("u_bea","POST","/reservations",os.environ["PA_KEY"]),("u_bea","POST","/reservations",os.environ["SA_KEY"]),("u_bea","POST","/series",os.environ["SX1_KEY"]),("u_bea","POST","/series",os.environ["SX2_KEY"]),("u_bea","POST","/reservation-moves",os.environ["MX_KEY"]),("u_ada","POST","/reservations",os.environ["FAIL_KEY"])}; assert got==want'

  # 16. Snapshot isolation: a post-export booking is live but absent from
  #     the saved file.
  python3 -c 'import os; print("{\"restaurant_id\":\"r_anker\",\"table_id\":\"t_1\",\"starts_at_local\":\""+os.environ["WDATE"]+"T18:00\",\"party_size\":2}")' > "$TMP/$STAGE-post-body.json" 2>>"$DIAG/stderr.log"
  python3 -c 'import hashlib; print(hashlib.sha256(open("'"$S_OUT"'/export.json","rb").read()).hexdigest())' > "$TMP/$STAGE-snapshot-sha.txt" 2>>"$DIAG/stderr.log"
  expect "$STAGE post-snapshot write" "$(api_post /reservations "$TMP/$STAGE-post-body.json" "$TMP/$STAGE-post-resp.json" "$TOK_A1" "$KEYP-post-01")" 201
  POST_REF=$(jget "$TMP/$STAGE-post-resp.json" "['reference']")
  pyassert "$STAGE saved file sha unchanged" 'import hashlib,os; assert hashlib.sha256(open(os.environ["S_OUT"]+"/export.json","rb").read()).hexdigest()==open(os.path.join(os.environ["TMP"],os.environ["STAGE"]+"-snapshot-sha.txt")).read().strip()'
  pyassert "$STAGE live post reference present" 'import json,os; d=json.load(open("'"$TMP"'/'"$STAGE"'-post-resp.json")); assert d["reference"]==os.environ["POST_REF"]'
  export POST_REF
  pyassert "$STAGE saved snapshot unchanged by later write" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/export.json")); assert os.environ["POST_REF"] not in d["state"]["reservations"]'

  # 17. Manifest with metadata (FULL policies/histories/series/counters).
  export TOK_A2 TOK_B2 LOST_REF REF_PAIR REF_SINGLE REF_REUSE POST_REF ADA_PASSWORD BEA_PASSWORD IMAGE CONTAINER PORT CID REV TREE STAGE SID1 SID2 G1 G2 H1 H2
  export POL1_KEY POL2_KEY PA_KEY SA_KEY SX1_KEY SX2_KEY MX_KEY FAIL_KEY
  python3 <<'PYEOF' > "$S_OUT/manifest.json" 2>>"$DIAG/stderr.log"
import json, os
s_out = os.environ["S_OUT"]
stage = os.environ["STAGE"]
tmp = os.environ["TMP"]
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
    receipt("LOST_KEY", "u_bea", "POST", "/reservations", "lost-body.json", "lost-resp.json"),
    receipt("POL1_KEY", "u_ada", "POST", "/restaurants/r_anker/policies", "pol1-body.json", "pol1-resp.json"),
    receipt("POL2_KEY", "u_ada", "POST", "/restaurants/r_anker/policies", "pol2-body.json", "pol2-resp.json"),
    receipt("PA_KEY", "u_bea", "POST", "/reservations", "pair-body.json", "pair-resp.json"),
    receipt("SA_KEY", "u_bea", "POST", "/reservations", "single-body.json", "single-resp.json"),
    receipt("SX1_KEY", "u_bea", "POST", "/series", "sx1-body.json", "sx1-resp.json"),
    receipt("SX2_KEY", "u_bea", "POST", "/series", "sx2-body.json", "sx2-resp.json"),
    receipt("MX_KEY", "u_bea", "POST", "/reservation-moves", "mx-body.json", "mx-resp.json"),
    receipt("FAIL_KEY", "u_ada", "POST", "/reservations", "fail-reuse-body.json", "fail-reuse-resp.json",
            failed=("fail-body.json", "table_unavailable")),
]
exp = json.load(open(os.path.join(s_out, "export.json")))
st = exp["state"]
manifest = {
    "manifest_version": 1,
    "source": {"stage": "stage3",
               "reviewed_revision": os.environ["REV"],
               "stage_tree": os.environ["TREE"],
               "image": os.environ["IMAGE"],
               "container": os.environ["CONTAINER"],
               "container_id": os.environ["CID"],
               "port": int(os.environ["PORT"])},
    "fixture": {"date": os.environ["DATE"],
                "pair_seed_date": "2020-01-09",
                "past_seed_date": os.environ["PAST"],
                "restaurant": [r for r in json.load(open(os.path.join(tmp, stage + "-fixture-orig.json")))["restaurants"] if r["id"] == "r_anker"][0],
                "restaurant_big": [r for r in json.load(open(os.path.join(tmp, stage + "-fixture-orig.json")))["restaurants"] if r["id"] == "r_big"][0],
                "restaurants": json.load(open(os.path.join(tmp, stage + "-fixture-orig.json")))["restaurants"],
                "restaurant_public": load("restaurant.json"),
                "seeds": json.load(open(os.path.join(tmp, stage + "-fixture-orig.json")))["reservations"]},
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
                      "owner": "u_bea", "method": "POST", "path": "/reservations",
                      "body": json.loads(raw("lost-body.json")),
                      "body_raw": raw("lost-body.json"),
                      "response": load("lost-resp.json"),
                      "response_raw": raw("lost-resp.json"),
                      "current": by_ref[os.environ["LOST_REF"]]},
    "failed_keys": {
        "reused": {"key": os.environ["FAIL_KEY"], "owner": "u_ada", "method": "POST",
                   "path": "/reservations",
                   "failed_body": json.loads(raw("fail-body.json")),
                   "failed_body_raw": raw("fail-body.json"),
                   "failed_code": "table_unavailable",
                   "body": json.loads(raw("fail-reuse-body.json"))},
        "absent": [{"key": os.environ["STUCK_KEY"], "owner": "u_ada", "method": "POST",
                    "path": "/reservations",
                    "body": json.loads(raw("stuck-body.json")),
                    "body_raw": raw("stuck-body.json"), "expected_status": 422}],
    },
    "metadata": {"policies": st.get("policies", {}),
                 "histories": st.get("histories", {}),
                 "series": st.get("series", {}),
                 "restaurant_revisions": st.get("restaurant_revisions", {})},
    "post_snapshot_write": {"reference": os.environ["POST_REF"], "excluded_from_export": True},
    "current_lists_before_post_snapshot_write": True,
}
print(json.dumps(manifest, indent=2, sort_keys=True))
PYEOF
  chmod 0600 "$S_OUT/manifest.json"
  export S_OUT
  pyassert "$STAGE manifest envelope" 'import json,os; d=json.load(open(os.environ["S_OUT"]+"/manifest.json")); assert d["manifest_version"]==1 and d["current_lists_before_post_snapshot_write"] is True'

  # 18. Full live agreement: owner lists + by-reference GETs + history +
  #     decision + series views agree with the export metadata.
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
exp = json.load(open(t + "/" + s + "-postreplays.json"))
st = exp["state"]
hfiles = glob.glob(t + "/" + s + "-hist-*.json")
assert hfiles, "no history views fetched"
for f in hfiles:
    h = json.load(open(f))
    assert h["entries"] == st["histories"][h["reference"]], f
print("histories compared: %d" % len(hfiles))
dfiles = glob.glob(t + "/" + s + "-dec-*.json")
assert dfiles, "no decision views fetched"
for f in dfiles:
    d = json.load(open(f))
    cur = st["reservations"][d["reference"]]
    proj = {k: v for k, v in cur.items() if k != "user_id"}
    if proj.get("table_id") == "":
        del proj["table_id"]
    assert d["reference"] == proj["reference"] and d["revision"] == proj["revision"] and d["accepted_terms"] == proj["accepted_terms"], f
print("decisions compared: %d" % len(dfiles))
sfiles = glob.glob(t + "/" + s + "-s1-cur.json") + glob.glob(t + "/" + s + "-s2-cur.json")
assert len(sfiles) == 2, sfiles
for f in sfiles:
    sv = json.load(open(f))
    stored = st["series"][sv["series_id"]]
    assert sv["revision"] == stored["revision"], f
    assert [(o["index"], o["reference"], o["exception"]) for o in sv["occurrences"]] == [(m["index"], m["reference"], m["exception"]) for m in stored["members"]], f
    for o in sv["occurrences"]:
        cur = st["reservations"][o["reference"]]
        proj = {k: v for k, v in cur.items() if k != "user_id"}
        if proj.get("table_id") == "":
            del proj["table_id"]
        assert o["reservation"] == proj, (f, o["reference"])
print("series compared: %d" % len(sfiles))
PYEOF
  if python3 "$TMP/$STAGE-agree.py" 2>>"$DIAG/stderr.log"; then
    echo "PASS: $STAGE full GET/list agreement"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $STAGE full GET/list agreement (see private diagnostics)"
    FAIL=$((FAIL + 1))
  fi

  # 19. Copied-artifact validator: genuine files pass; labelled COPY
  #     corruptions are rejected while originals stay untouched:
  #     changed identity/local clock, raw-only request-body mismatch,
  #     whitespace-only response raw mismatch, history/series metadata
  #     mismatch. Exact raw/parsed/canonical binding, exact owner/ref/
  #     receipt sets, declared pair order.
cat > "$TMP/$STAGE-validate.py" <<'PYEOF'
import json, sys
exp_path, man_path = sys.argv[1], sys.argv[2]
e = json.load(open(exp_path))
m = json.load(open(man_path))
errs = []
if not (e.get("track") == "tablekeeper" and e.get("format_version") == 1 and isinstance(e.get("state"), dict)):
    errs.append("envelope")
st = e["state"]
# Owner lists: exact per-owner reference sets and full projections from the
# stored export. by_reference must equal their union; ada/bea lists must
# partition it with no overlap. Each named list binds to the stored export
# user_id: ada_list refs must all be u_ada-owned, bea_list u_bea-owned.
# Duplicate refs inside one list are rejected.
byref = m["records"]["by_reference"]
ada_list = m["records"]["ada_list"]
bea_list = m["records"]["bea_list"]
ada_refs = [r["reference"] for r in ada_list]
bea_refs = [r["reference"] for r in bea_list]
if len(set(ada_refs)) != len(ada_refs):
    errs.append("owner-dup:ada")
if len(set(bea_refs)) != len(bea_refs):
    errs.append("owner-dup:bea")
if set(ada_refs) & set(bea_refs):
    errs.append("owner-overlap")
if set(ada_refs) | set(bea_refs) != set(byref):
    errs.append("owner-union")
for r in ada_refs:
    if r in st["reservations"] and st["reservations"][r].get("user_id") != "u_ada":
        errs.append("owner-binding:ada:" + r)
        break
for r in bea_refs:
    if r in st["reservations"] and st["reservations"][r].get("user_id") != "u_bea":
        errs.append("owner-binding:bea:" + r)
        break
if sorted(byref) != sorted(set(byref)):
    errs.append("byref-dup")
union = {r["reference"]: r for r in ada_list + bea_list}
for r, rec in union.items():
    if byref.get(r) != rec:
        errs.append("owner-projection:" + r)
        break
# Owner IDs bound to stored export user_id; no user_id leak into public
# records; every stored reservation carries a known owner.
owners = {u["id"] for u in m["users"]}
for r, rec in byref.items():
    if "user_id" in rec:
        errs.append("owner-leak:" + r)
        break
for r, stored in st["reservations"].items():
    if stored.get("user_id") not in owners:
        errs.append("owner-unknown:" + r)
        break
if set(byref) != set(st["reservations"]):
    errs.append("ref-sets")
for r, rec in byref.items():
    if r not in st["reservations"]:
        continue
    proj = {k: v for k, v in st["reservations"][r].items() if k != "user_id"}
    if proj.get("table_id") == "":
        del proj["table_id"]
    ids = proj.get("table_ids", [proj.get("table_id")])
    if ("table_id" in proj) != (len(ids) == 1):
        errs.append("table-shape:" + r)
    # Declared pair order: pair table_ids must match the fixture combinable
    # entry order, never reversed or re-sorted.
    if len(ids) == 2:
        combs = [sorted(p) for p in m["fixture"]["restaurant"].get("combinable", [])]
        if sorted(ids) not in combs:
            errs.append("pair-undeclared:" + r)
        else:
            decl = [p for p in m["fixture"]["restaurant"]["combinable"] if sorted(p) == sorted(ids)]
            if decl and ids != decl[0]:
                errs.append("pair-order:" + r)
    if rec != proj:
        errs.append("record:" + r)
    if rec.get("revision", 0) < 1:
        errs.append("revision:" + r)
    if set(rec.get("accepted_terms", {})) != {"policy_version", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "capacities"}:
        errs.append("terms:" + r)
# Manifest users bind to stored users: id/email/display_name exact,
# stored password_hash nonempty (hash retained, never plaintext), exactly two
# distinct live tokens per known user bound to stored tokens (no invented
# plaintext fields in the export).
exp_users = st["users"] if isinstance(st.get("users"), dict) else {}
exp_tokens = st.get("tokens", {})
if set(exp_users) != {u["id"] for u in m["users"]}:
    errs.append("users-idset")
for u in m["users"]:
    eu = exp_users.get(u["id"], {})
    if u.get("email") != eu.get("email"):
        errs.append("user-email:" + u["id"])
        break
    if u.get("display_name") != eu.get("display_name"):
        errs.append("user-name:" + u["id"])
        break
    if not eu.get("password_hash"):
        errs.append("user-hash:" + u["id"])
        break
    toks = u.get("tokens", [])
    if len(toks) != 2 or toks[0] == toks[1]:
        errs.append("tokens-distinct:" + u["id"])
        continue
    for tk in toks:
        if exp_tokens.get(tk) != u["id"]:
            errs.append("token-binding:" + u["id"])
            break
# Fixture: complete original restaurant incl. managers; seeds match the
# original reset request records by reference/owner/identity.
fx = m["fixture"]["restaurant"]
for k in ["id", "name", "timezone", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "tables", "combinable", "manager_user_ids"]:
    if k not in fx:
        errs.append("fixture-missing:" + k)
        break
if fx.get("manager_user_ids") != ["u_ada"]:
    errs.append("fixture-managers")
fxb = m["fixture"].get("restaurant_big", {})
if not (fxb.get("slot_minutes") == 1441 and fxb.get("reservation_duration_minutes") == 1441 and fxb.get("cancellation_cutoff_minutes") == 10081):
    errs.append("fixture-big-terms")
if [t.get("capacity") for t in fxb.get("tables", [])] != [101]:
    errs.append("fixture-big-cap")
seed_refs = {s["reference"]: s for s in m["fixture"]["seeds"]}
if set(seed_refs) != {"SEDPST", "SEDOFF", "SEDOVR", "SEDCXD", "SEDPAIR", "SEDMAX"}:
    errs.append("fixture-seeds")
for ref, s in seed_refs.items():
    if ref not in st["reservations"]:
        errs.append("seed-absent:" + ref)
        break
    stored = st["reservations"][ref]
    if stored.get("user_id") != s["user_id"]:
        errs.append("seed-owner:" + ref)
        break
pub = m["fixture"].get("restaurant_public", {})
for k in ["id", "name", "timezone", "slot_minutes", "reservation_duration_minutes", "cancellation_cutoff_minutes", "opening_hours", "tables"]:
    if pub.get(k) != fx.get(k):
        errs.append("public-projection:" + k)
        break
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
    if json.loads(mr["body_raw"]) != mr["body"]:
        errs.append("receipt-body-raw:" + k[3])
    if json.loads(mr["body_raw"]) != json.loads(er["body"]):
        errs.append("receipt-body-raw-canonical:" + k[3])
    if json.loads(mr["response_raw"]) != mr["response"]:
        errs.append("receipt-response-raw:" + k[3])
    if json.loads(mr["response_raw"]) != json.loads(er["response"]):
        errs.append("receipt-response-raw-canonical:" + k[3])
    rr, stored = mr["response_raw"], er["response"]
    if rr != stored and rr != stored + "\n":
        errs.append("receipt-response-bytes:" + k[3])
if m.get("metadata", {}).get("histories", {}) != st.get("histories", {}):
    errs.append("metadata-histories")
if m.get("metadata", {}).get("series", {}) != st.get("series", {}):
    errs.append("metadata-series")
if m.get("metadata", {}).get("policies", {}) != st.get("policies", {}):
    errs.append("metadata-policies")
if m.get("metadata", {}).get("restaurant_revisions", {}) != st.get("restaurant_revisions", {}):
    errs.append("metadata-counters")
for sid, sz in st.get("series", {}).items():
    refs = [mm["reference"] for mm in sz.get("members", [])]
    if len(set(refs)) != len(refs):
        errs.append("series-dup:" + sid)
print("validator findings: %d" % len(errs))
if errs:
    print("findings: " + ",".join(sorted(errs)[:8]))
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
  python3 -c 'import json; p="'"$SCRATCH"'/export.json"; d=json.load(open(p)); r=sorted(d["state"]["reservations"])[0]; d["state"]["reservations"][r]["reservation_id"]="SABOTAGED01"; d["state"]["reservations"][r]["starts_at_local"]="2020-01-01T00:00"; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
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
  cp "$S_OUT/export.json" "$SCRATCH/export.json"
  chmod 0600 "$SCRATCH/export.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); k=sorted(d["metadata"]["histories"])[0]; d["metadata"]["histories"][k]=[]; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects history metadata sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects history metadata sabotage"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); k=sorted(d["metadata"]["series"])[0]; d["metadata"]["series"][k]["revision"]=999; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects series metadata sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects series metadata sabotage"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); d["records"]["ada_list"]=[]; d["records"]["bea_list"]=[]; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects owner-list sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects owner-list sabotage"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); d["records"]["ada_list"],d["records"]["bea_list"]=d["records"]["bea_list"],d["records"]["ada_list"]; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects swapped-list sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects swapped-list sabotage"
    PASS=$((PASS + 1))
  fi
  cp "$S_OUT/manifest.json" "$SCRATCH/manifest.json"
  chmod 0600 "$SCRATCH/manifest.json"
  python3 -c 'import json; p="'"$SCRATCH"'/manifest.json"; d=json.load(open(p)); d["users"][0]["email"]="tampered@example.com"; json.dump(d, open(p, "w"))' 2>>"$DIAG/stderr.log"
  if python3 "$TMP/$STAGE-validate.py" "$SCRATCH/export.json" "$SCRATCH/manifest.json" >>"$DIAG/stderr.log" 2>&1; then
    echo "FAIL: $STAGE validator rejects user-identity sabotage (see private diagnostics)"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $STAGE validator rejects user-identity sabotage"
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
if [ -n "${1:-}" ] && [ "${1:-}" = "--selftest-parse" ]; then
  echo "parse-ok"
  exit 0
fi
if ! mkdir -p "$OUT" 2>/dev/null; then
  echo "FAIL: output directory not writable (see private diagnostics)"
  FAIL=$((FAIL + 1))
fi
if [ -e "$OUT" ] && [ ! -d "$OUT" ]; then
  echo "FAIL: output path is not a directory (see private diagnostics)"
  FAIL=$((FAIL + 1))
fi
require_clean "output directory writable"
chmod 0700 "$OUT"
TMP=$(mktemp -d "$OUT/tmp.XXXXXX")
export TMP
DIAG="$OUT/diag"
mkdir -p "$DIAG"
chmod 0700 "$DIAG"
trap cleanup EXIT INT TERM

# URL scheme/host guard via stdlib: scheme must be http/https with a
# non-empty host; OUT_DIR must be an existing writable directory (created
# with 0700 if missing; a file at that path is rejected).
cat > "$TMP/url-guard.py" <<'PYEOF'
import os, sys
from urllib.parse import urlparse
ok = True
for u in sys.argv[1:4]:
    try:
        p = urlparse(u)
        if p.scheme not in ("http", "https") or not p.hostname:
            ok = False
    except Exception:
        ok = False
out = sys.argv[4]
if os.path.exists(out) and not os.path.isdir(out):
    ok = False
print("url-guard-ok" if ok else "url-guard-bad")
sys.exit(0 if ok else 1)
PYEOF
if python3 "$TMP/url-guard.py" "$S1_URL" "$S2_URL" "$S3_URL" "$OUT" 2>>"$DIAG/stderr.log"; then
  echo "PASS: source URL scheme/host valid"
  PASS=$((PASS + 1))
else
  echo "FAIL: source URL scheme/host invalid (see private diagnostics)"
  FAIL=$((FAIL + 1))
fi
require_clean "source URL validation"
require_clean "source URL validation"

run_source stage1 stage1 "$STAGE1_IMAGE" "$STAGE1_CONTAINER" "$STAGE1_PORT" "$STAGE1_CID" "$S1_REV" "$S1_TREE" s4d-s1 0
require_clean "stage-1 donor"
run_source stage2 stage2 "$STAGE2_IMAGE" "$STAGE2_CONTAINER" "$STAGE2_PORT" "$STAGE2_CID" "$S2_REV" "$S2_TREE" s4d-s2 1
require_clean "stage-2 donor"
run_source3 stage3 stage3 "$STAGE3_IMAGE" "$STAGE3_CONTAINER" "$STAGE3_PORT" "$STAGE3_CID" "$S3_REV" "$S3_TREE" s4d-s3
require_clean "stage-3 donor"

expect "sabotage injected expectation" "real-value" "real-value"

echo "donor checks passed: $PASS failed: $FAIL"
if [ "$FAIL" != 0 ]; then
  exit 1
fi
