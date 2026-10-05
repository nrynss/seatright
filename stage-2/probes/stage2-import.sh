#!/bin/sh
# stage2-import.sh - prove REAL stage-1 to stage-2 migration across two
# independently started processes/images, plus modern pair behavior.
#
# Usage: sh stage-2/probes/stage2-import.sh <src-url> <dst-url> <donor-dir> <work-dir>
#
# <donor-dir> holds the stage-1 donor export.json and manifest.json
# (read-only inputs, never modified or committed). <work-dir> receives
# private working copies and assertion outputs only.
# Secret-safety contract: stdout carries ONLY sanitized check names,
# HTTP status codes and counts. Tokens, passwords, export bodies and raw
# response payloads stay inside WORK (mode 0700) and never reach stdout,
# even on failure. Python tracebacks and response bodies are captured to
# files under WORK; failing checks report the private file name only.
# Every check counts through ok()/bad(); any failure exits nonzero.
# R165 browser no-reload is DEFERRED to H proof; this probe claims only
# between-request server migration, never browser behavior.
set -eu
SRC=${1:?usage: sh stage-2/probes/stage2-import.sh SRC_URL DST_URL DONOR_DIR WORK_DIR}
DST=${2:?usage: sh stage-2/probes/stage2-import.sh SRC_URL DST_URL DONOR_DIR WORK_DIR}
DONOR=${3:?usage: sh stage-2/probes/stage2-import.sh SRC_URL DST_URL DONOR_DIR WORK_DIR}
WORK=${4:?usage: sh stage-2/probes/stage2-import.sh SRC_URL DST_URL DONOR_DIR WORK_DIR}
rm -rf "$WORK"
mkdir -p "$WORK"
chmod 0700 "$WORK" 2>/dev/null || true
PASS=0
FAIL=0
SKIP=0
ok() { PASS=$((PASS+1)); echo "PASS $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

# code_only: HTTP status to stdout-caller, body to a private file. NEVER
# prints the body. $1 = private outfile, rest = curl args.
code_only() {
  out=$1; shift
  curl -s -o "$out" -w '%{http_code}' "$@" > "$out.code" 2>"$out.err" || echo "000" > "$out.code"
  cat "$out.code"
}
# need_code <got> <want> <name> <private-file>: sanitized comparison only.
need_code() {
  if [ "$1" = "$2" ]; then ok "$3"; else bad "$3" "want $2 got $1 (see $4)"; return 1; fi
}
# run_py <check-name> <scriptfile>: executes python, logs traceback privately.
run_py() {
  name=$1; script=$2
  if python3 "$script" 2>"$WORK/$name.perr"; then ok "$name"; else bad "$name" "see $WORK/$name.perr"; return 1; fi
}
# write_py <filename>: heredoc body follows on stdin.
write_py() {
  cat > "$WORK/$1"
}

# 0. Both processes are distinct live services.
need_code "$(code_only "$WORK/health-src" "$SRC/health")" 200 "src-health-200" "$WORK/health-src"
need_code "$(code_only "$WORK/health-dst" "$DST/health")" 200 "dst-health-200" "$WORK/health-dst"

# 1. Donor directory holds the donor artifacts (read-only inputs).
[ -f "$DONOR/export.json" ] && [ -f "$DONOR/manifest.json" ] && ok "donor-present" || { bad "donor-present" "missing export.json/manifest.json"; exit 1; }

# 2. Fresh destination starts from unrelated state with its own credential.
FIX_OTHER='{"users":[{"id":"u_other","email":"other@example.com","password":"correct horse other","display_name":"Other"}],"restaurants":[],"reservations":[]}'
need_code "$(code_only "$WORK/reset-other" -X POST "$DST/_test/reset" -H 'Content-Type: application/json' -d "$FIX_OTHER")" 204 "dst-unrelated-reset-204" "$WORK/reset-other"
code_only "$WORK/signup-other" -X POST "$DST/auth/signup" -H 'Content-Type: application/json' -d '{"email":"dst@example.com","password":"correct horse dst","display_name":"Dst"}' >/dev/null
python3 -c 'import json; d=json.load(open("'"$WORK"'/signup-other")); open("'"$WORK"'/other-tok.txt","w").write(d.get("token",""))' 2>"$WORK/other-tok.perr" || { bad "dst-prior-token" "see $WORK/other-tok.perr"; exit 1; }
[ -s "$WORK/other-tok.txt" ] && ok "dst-prior-token" || { bad "dst-prior-token" "no token"; exit 1; }

# 3. Import the UNCHANGED donor export into the destination.
need_code "$(code_only "$WORK/import1" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$DONOR/export.json")" 204 "donor-import-204" "$WORK/import1"
need_code "$(code_only "$WORK/import2" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$DONOR/export.json")" 204 "donor-reimport-204" "$WORK/import2"

# 4. Donor receipt count preserved (5 completed; stuck failed key absent).
write_py count5.py <<'PYEOF'
import json
donor = json.load(open("DONORPH/export.json"))["state"]["receipts"]
live = json.load(open("WORKPH/dst-export.json"))["state"]["receipts"]
assert len(donor) == 5, len(donor)
assert len(live) == 5, len(live)
PYEOF
code_only "$WORK/dst-export.json" "$DST/_test/export" >/dev/null
sed -i "s|DONORPH|$DONOR|g;s|WORKPH|$WORK|g" "$WORK/count5.py"
run_py "receipt-count-5" "$WORK/count5.py" || exit 1

# 5. All 5 original receipts byte-identical (body/response/status/key).
write_py receipts5.py <<'PYEOF'
import json
donor = json.load(open("DONORPH/export.json"))["state"]["receipts"]
live = json.load(open("WORKPH/dst-export.json"))["state"]["receipts"]
assert set(donor) == set(live), (sorted(donor), sorted(live))
for k in donor:
    a, b = donor[k], live[k]
    assert a["body"] == b["body"] and a["response"] == b["response"] and a["status"] == b["status"] and a["key"] == b["key"], k
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|WORKPH|$WORK|g" "$WORK/receipts5.py"
run_py "receipts-byte-identical-5" "$WORK/receipts5.py" || exit 1

# 6. Old sessions authenticate; password-hash logins work for both users.
#    Tokens pass via files only; tracebacks never print tokens.
write_py sessions.py <<'PYEOF'
import json, urllib.request
man = json.load(open("DONORPH/manifest.json"))
base = "DSTPH"
toks = [t for u in man["users"] for t in u["tokens"]]
assert len(toks) == 4, len(toks)
for i, tok in enumerate(toks):
    req = urllib.request.Request(base + "/reservations", headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req) as r:
        assert r.status == 200, i
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|DSTPH|$DST|g" "$WORK/sessions.py"
run_py "old-sessions-4-valid" "$WORK/sessions.py" || exit 1
# 11a. Repeat-import stability on unchanged donor state (before new logins mint tokens):
code_only "$WORK/before.json" "$DST/_test/export" >/dev/null
need_code "$(code_only "$WORK/reimport3" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$WORK/before.json")" 204 "repeat-import-stable-204" "$WORK/reimport3"
code_only "$WORK/after.json" "$DST/_test/export" >/dev/null
write_py stable.py <<'PYEOF'
import json
before = json.load(open("WORKPH/before.json"))
after = json.load(open("WORKPH/after.json"))
assert before == after, "export state changed across repeat import"
assert open("WORKPH/before.json").read() == open("WORKPH/after.json").read(), "export bytes changed across repeat import"
PYEOF
sed -i "s|WORKPH|$WORK|g" "$WORK/stable.py"
run_py "export-stable-repeat" "$WORK/stable.py" || exit 1

python3 -c 'import json; m=json.load(open("'"$DONOR"'/manifest.json")); open("'"$WORK"'/ada-pw.txt","w").write([u["password"] for u in m["users"] if u["email"]=="ada@example.com"][0])' 2>"$WORK/ada-pw.perr" || { bad "ada-pw-extract" "see $WORK/ada-pw.perr"; exit 1; }
python3 -c 'import json; m=json.load(open("'"$DONOR"'/manifest.json")); open("'"$WORK"'/bea-pw.txt","w").write([u["password"] for u in m["users"] if u["email"]=="bea@example.com"][0])' 2>"$WORK/bea-pw.perr" || { bad "bea-pw-extract" "see $WORK/bea-pw.perr"; exit 1; }
chmod 0600 "$WORK/ada-pw.txt" "$WORK/bea-pw.txt" 2>/dev/null || true
python3 -c 'import json; print(json.dumps({"email":"ada@example.com","password":open("'"$WORK"'/ada-pw.txt").read().strip()}))' > "$WORK/login-ada.json" 2>"$WORK/login-ada.perr" || { bad "ada-login-body" "see $WORK/login-ada.perr"; exit 1; }
python3 -c 'import json; print(json.dumps({"email":"bea@example.com","password":open("'"$WORK"'/bea-pw.txt").read().strip()}))' > "$WORK/login-bea.json" 2>"$WORK/login-bea.perr" || { bad "bea-login-body" "see $WORK/login-bea.perr"; exit 1; }
chmod 0600 "$WORK/login-ada.json" "$WORK/login-bea.json" 2>/dev/null || true
need_code "$(code_only "$WORK/login-ada-resp" -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d @"$WORK/login-ada.json")" 200 "ada-hash-login-200" "$WORK/login-ada-resp"
need_code "$(code_only "$WORK/login-bea-resp" -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d @"$WORK/login-bea.json")" 200 "bea-hash-login-200" "$WORK/login-bea-resp"

# 7. ALL current records survive: compare every owner lookup AND both list
#    endpoints against the donor manifest current values field-by-field
#    (identity/ref/restaurant/table/current party/local start/start/end/
#    created/status), including amended A, cancelled B, pending confirmed
#    AND the reused failed-key record. No skips. Caller privacy: each owner
#    sees only their own records (cross-owner lookup must 404).
write_py records.py <<'PYEOF'
import json, urllib.request
man = json.load(open("DONORPH/manifest.json"))
base = "DSTPH"
FIELDS = ["reservation_id", "reference", "restaurant_id", "table_id",
          "party_size", "status", "starts_at_local", "starts_at", "ends_at", "created_at"]
def get(path, tok):
    req = urllib.request.Request(base + path, headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req) as r:
        return r.status, json.load(r)
atok = man["users"][0]["tokens"][0]
btok = man["users"][1]["tokens"][0]
cur = man["current_records"]
checks = [("lost", atok), ("swapped_a", atok), ("swapped_b", atok), ("reused", btok)]
assert cur["swapped_a"]["party_size"] == 1, cur["swapped_a"]
assert cur["swapped_b"]["status"] == "cancelled", cur["swapped_b"]
assert cur["lost"]["status"] == "confirmed", cur["lost"]
for name, tok in checks:
    want = cur[name]
    st, got = get("/reservations/" + want["reference"], tok)
    assert st == 200, (name, st)
    for f in FIELDS:
        assert got[f] == want[f], (name, f, got[f], want[f])
post = man["post_snapshot_write_excluded_from_export"]["reference"]
# lists agree with manifest current lists (order-insensitive by reference),
# EXCLUDING the post-snapshot write: it exists on the live source only and
# must be absent from the imported destination snapshot.
for lname, tok in (("ada_list", atok), ("bea_list", btok)):
    st, got = get("/reservations", tok)
    assert st == 200, (lname, st)
    gl = {r["reference"]: r for r in got["reservations"]}
    wl = {r["reference"]: r for r in cur[lname] if r["reference"] != post}
    assert set(gl) == set(wl), (lname, sorted(gl), sorted(wl))
    for ref, want in wl.items():
        for f in FIELDS:
            assert gl[ref][f] == want[f], (lname, ref, f)
# caller privacy: cross-owner lookups 404
for ref, tok in ((cur["lost"]["reference"], btok), (cur["reused"]["reference"], atok)):
    req = urllib.request.Request(base + "/reservations/" + ref, headers={"Authorization": "Bearer " + tok})
    try:
        urllib.request.urlopen(req)
    except urllib.error.HTTPError as e:
        assert e.code == 404, (ref, e.code)
    else:
        raise AssertionError(("leak", ref))
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|DSTPH|$DST|g" "$WORK/records.py"
run_py "current-records-survive" "$WORK/records.py" || exit 1

# 8. Replay EVERY successful donor create/batch receipt with original
#    method/path/body/key/owner token: 200 with complete original JSON,
#    even where current state differs (amended A, cancelled B).
write_py replays.py <<'PYEOF'
import json, subprocess, os
man = json.load(open("DONORPH/manifest.json"))
base = "DSTPH"
work = "WORKPH"
toks = {"u_ada": man["users"][0]["tokens"][0], "u_bea": man["users"][1]["tokens"][0]}
for r in man["receipts"]:
    key, uid, method, path = r["key"], r["user_id"], r["method"], r["path"]
    body = json.dumps(r["body"])
    bf = os.path.join(work, "replay-body-" + key + ".json")
    of = os.path.join(work, "replay-got-" + key + ".json")
    open(bf, "w").write(body)
    tokfile = os.path.join(work, "replay-tok-" + key + ".txt")
    open(tokfile, "w").write(toks[uid])
    code = subprocess.run(["curl", "-s", "-o", of, "-w", "%{http_code}", "-X", method,
                           base + path, "-H", "Authorization: Bearer " + toks[uid],
                           "-H", "Idempotency-Key: " + key, "-H", "Content-Type: application/json",
                           "-d", "@" + bf], capture_output=True, text=True).stdout.strip()
    assert code == "200", (key, code)
    got = json.load(open(of))
    assert got == r["response"], (key, got, r["response"])
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|DSTPH|$DST|g;s|WORKPH|$WORK|g" "$WORK/replays.py"
run_py "all-receipts-replay-200-identical" "$WORK/replays.py" || exit 1

# 9. Fresh failed key: absent before, first VALID use 201, replay 200 with
#    full-JSON identity (not a same-key-different-body 409).
code_only "$WORK/dst-export.json" "$DST/_test/export" >/dev/null
write_py absent.py <<'PYEOF'
import json
man = json.load(open("DONORPH/manifest.json"))
live = json.load(open("WORKPH/dst-export.json"))["state"]["receipts"]
skey = man["failed_keys_absent_from_receipts"][0]["key"]
assert all(r["key"] != skey for r in live.values()), "key already claimed"
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|WORKPH|$WORK|g" "$WORK/absent.py"
run_py "failed-key-absent" "$WORK/absent.py" || exit 1
SKEY=$(python3 -c 'import json; print(json.load(open("'"$DONOR"'/manifest.json"))["failed_keys_absent_from_receipts"][0]["key"])' 2>"$WORK/skey.perr") || { bad "failed-key-extract" "see $WORK/skey.perr"; exit 1; }
BTOK=$(python3 -c 'import json; m=json.load(open("'"$DONOR"'/manifest.json")); print(m["users"][1]["tokens"][0])' 2>"$WORK/btok.perr") || { bad "bea-tok-extract" "see $WORK/btok.perr"; exit 1; }
FBODY='{"restaurant_id":"r_anker","table_id":"t_2","starts_at_local":"2027-06-17T18:00","party_size":2}'
printf '%s' "$BTOK" > "$WORK/btok.txt"; chmod 0600 "$WORK/btok.txt" 2>/dev/null || true
curl -s -X POST "$DST/reservations" -H "Authorization: Bearer $BTOK" -H "Idempotency-Key: $SKEY" -H 'Content-Type: application/json' -d "$FBODY" -o "$WORK/first.json" -w '%{http_code}' > "$WORK/first-code.txt" 2>"$WORK/first.perr"
need_code "$(cat "$WORK/first-code.txt")" 201 "failed-key-first-201" "$WORK/first.json"
curl -s -X POST "$DST/reservations" -H "Authorization: Bearer $BTOK" -H "Idempotency-Key: $SKEY" -H 'Content-Type: application/json' -d "$FBODY" -o "$WORK/replay.json" -w '%{http_code}' > "$WORK/replay-code.txt" 2>"$WORK/replay.perr"
need_code "$(cat "$WORK/replay-code.txt")" 200 "failed-key-replay-200" "$WORK/replay.json"
write_py failedident.py <<'PYEOF'
import json
assert json.load(open("WORKPH/first.json")) == json.load(open("WORKPH/replay.json")), "first/replay differ"
PYEOF
sed -i "s|WORKPH|$WORK|g" "$WORK/failedident.py"
run_py "failed-key-identical" "$WORK/failedident.py" || exit 1

# 10. Destination prior credential removed by replacement import.
need_code "$(code_only "$WORK/prior-login" -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d '{"email":"other@example.com","password":"correct horse other"}')" 401 "dst-prior-removed-401" "$WORK/prior-login"
write_py priordead.py <<'PYEOF'
import json, urllib.request
tok = open("WORKPH/other-tok.txt").read().strip()
req = urllib.request.Request("DSTPH/reservations", headers={"Authorization": "Bearer " + tok})
try:
    urllib.request.urlopen(req)
except urllib.error.HTTPError as e:
    assert e.code == 401, e.code
else:
    raise AssertionError("prior token still valid")
PYEOF
sed -i "s|WORKPH|$WORK|g;s|DSTPH|$DST|g" "$WORK/priordead.py"
run_py "dst-prior-token-dead" "$WORK/priordead.py" || exit 1

need_code "$(code_only "$WORK/reset-clear" -X POST "$DST/_test/reset" -H 'Content-Type: application/json' -d "$FIX_OTHER")" 204 "reset-clears-204" "$WORK/reset-clear"
code_only "$WORK/cleared-export.json" "$DST/_test/export" >/dev/null
write_py cleared.py <<'PYEOF'
import json, urllib.request
man = json.load(open("DONORPH/manifest.json"))
base = "DSTPH"
for tok in [t for u in man["users"] for t in u["tokens"]]:
    req = urllib.request.Request(base + "/reservations", headers={"Authorization": "Bearer " + tok})
    try:
        urllib.request.urlopen(req)
    except urllib.error.HTTPError as e:
        assert e.code == 401, e.code
    else:
        raise AssertionError("old token alive after reset")
live = json.load(open("WORKPH/cleared-export.json"))["state"]
donor = json.load(open("DONORPH/export.json"))["state"]
assert len(live["reservations"]) == 0, live["reservations"]
assert len(live["receipts"]) == 0, live["receipts"]
assert len(live["restaurants"]) == 0, live["restaurants"]
assert len(live["tokens"]) == 0, live["tokens"]
for duid in donor["users"]:
    assert duid not in live["users"], duid
for dtok in donor["tokens"]:
    assert dtok not in live["tokens"], dtok
assert "u_other" in live["users"], live["users"]
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|DSTPH|$DST|g;s|WORKPH|$WORK|g" "$WORK/cleared.py"
run_py "reset-clears-state" "$WORK/cleared.py" || exit 1
# restore donor state for the remaining sections
need_code "$(code_only "$WORK/restore" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$DONOR/export.json")" 204 "donor-restore-204" "$WORK/restore"

# 12. Source snapshot exclusion + live source/dest divergence (fresh donor
#     only runs against its own live source; canonical input skips live-src
#     checks via DONOR_ORIGIN note). Invalid imports are atomic.
write_py snapshot.py <<'PYEOF'
import json
man = json.load(open("DONORPH/manifest.json"))
live = json.load(open("WORKPH/dst-export.json"))["state"]["reservations"]
post = man["post_snapshot_write_excluded_from_export"]["reference"]
assert post not in live, "post-snapshot write leaked"
PYEOF
sed -i "s|DONORPH|$DONOR|g;s|WORKPH|$WORK|g" "$WORK/snapshot.py"
code_only "$WORK/dst-export.json" "$DST/_test/export" >/dev/null
run_py "snapshot-immutable" "$WORK/snapshot.py" || exit 1
if [ "${DONOR_ORIGIN:-canonical}" = "fresh" ]; then
  write_py livediv.py <<'PYEOF'
import json, urllib.request
man = json.load(open("DONORPH/manifest.json"))
post = man["post_snapshot_write_excluded_from_export"]["reference"]
atok = man["users"][0]["tokens"][0]
btok = man["users"][1]["tokens"][0]
def get(base, path, tok):
    req = urllib.request.Request(base + path, headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req) as r:
        return json.load(r)
src = get("SRCPH", "/reservations/" + post, btok)
assert src["reference"] == post, src
try:
    get("DSTPH", "/reservations/" + post, btok)
except urllib.error.HTTPError as e:
    assert e.code == 404, e.code
else:
    raise AssertionError("post write leaked to dest")
PYEOF
  sed -i "s|DONORPH|$DONOR|g;s|SRCPH|$SRC|g;s|DSTPH|$DST|g" "$WORK/livediv.py"
  run_py "live-source-divergence" "$WORK/livediv.py" || exit 1
else
  SKIP=$((SKIP+1)); echo "SKIP live-source-divergence (canonical input: prior-process artifact, no live source)"
fi
code_only "$WORK/before.json" "$DST/_test/export" >/dev/null
python3 -c 'import json; v=json.load(open("'"$WORK"'/before.json")); k=next(iter(v["state"]["receipts"])); v["state"]["receipts"][k]["response"]="{oops"; json.dump(v, open("'"$WORK"'/bad.json","w"))' 2>"$WORK/bad.perr" || { bad "bad-build" "see $WORK/bad.perr"; exit 1; }
need_code "$(code_only "$WORK/bad-resp" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$WORK/bad.json")" 422 "corrupt-receipt-422" "$WORK/bad-resp"
need_code "$(code_only "$WORK/malformed-resp" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d '{')" 400 "malformed-400" "$WORK/malformed-resp"
write_py atomic.py <<'PYEOF'
import json, urllib.request
before = open("WORKPH/before.json").read()
live = urllib.request.urlopen("DSTPH/_test/export").read().decode()
assert before == live, "destination moved on invalid import"
PYEOF
sed -i "s|WORKPH|$WORK|g;s|DSTPH|$DST|g" "$WORK/atomic.py"
run_py "invalid-atomic" "$WORK/atomic.py" || exit 1

# 13. Modern pair behavior: seed confirmed/cancelled canonical pairs, run a
#     successful batch, MUTATE after, export/import into the destination
#     state, replay the ORIGINAL batch 200 with full original JSON while the
#     current record differs; verify canonical sets/status/identity/times
#     and mixed pair+single batch records after reimport.
PAIRFIX='{"users":[{"id":"u1","email":"a@b","password":"password1","display_name":"A"}],"restaurants":[{"id":"r1","name":"N","timezone":"Europe/Berlin","slot_minutes":30,"reservation_duration_minutes":90,"cancellation_cutoff_minutes":120,"opening_hours":[{"weekday":"thu","opens":"18:00","closes":"23:00"}],"tables":[{"id":"t_1","label":"1","capacity":2},{"id":"t_2","label":"2","capacity":4},{"id":"t_3","label":"3","capacity":4}],"combinable":[["t_1","t_2"],["t_2","t_3"]]}],"reservations":[{"id":"s1","reference":"PAIRCF","user_id":"u1","restaurant_id":"r1","table_ids":["t_2","t_1"],"starts_at_local":"2027-05-06T19:00","party_size":4},{"id":"s2","reference":"PAIRCN","user_id":"u1","restaurant_id":"r1","table_ids":["t_1","t_2"],"starts_at_local":"2027-05-06T20:30","party_size":2,"status":"cancelled"},{"id":"s3","reference":"SINGLE1","user_id":"u1","restaurant_id":"r1","table_id":"t_3","starts_at_local":"2027-05-06T19:00","party_size":2}]}'
need_code "$(code_only "$WORK/pair-seed" -X POST "$DST/_test/reset" -H 'Content-Type: application/json' -d "$PAIRFIX")" 204 "pair-seed-204" "$WORK/pair-seed"
code_only "$WORK/pair-login" -X POST "$DST/auth/login" -H 'Content-Type: application/json' -d '{"email":"a@b","password":"password1"}' >/dev/null
python3 -c 'import json; d=json.load(open("'"$WORK"'/pair-login")); open("'"$WORK"'/ptok.txt","w").write(d.get("token",""))' 2>"$WORK/ptok.perr" || { bad "pair-login" "see $WORK/ptok.perr"; exit 1; }
PTOK2=$(cat "$WORK/ptok.txt")
curl -s -X POST "$DST/reservation-moves" -H "Authorization: Bearer $PTOK2" -H 'Idempotency-Key: probe-pair-1' -H 'Content-Type: application/json' -d '{"moves":[{"reference":"PAIRCF","party_size":5},{"reference":"SINGLE1"}]}' -o "$WORK/pair-batch.json" -w '%{http_code}' > "$WORK/pair-batch-code.txt" 2>"$WORK/pair-batch.perr"
need_code "$(cat "$WORK/pair-batch-code.txt")" 201 "pair-batch-201" "$WORK/pair-batch.json"
code_only "$WORK/pair-mutate" -X PATCH "$DST/reservations/PAIRCF" -H "Authorization: Bearer $PTOK2" -H 'Content-Type: application/json' -d '{"party_size":6}' >/dev/null
code_only "$WORK/pair-exp.json" "$DST/_test/export" >/dev/null
need_code "$(code_only "$WORK/pair-reimport" -X POST "$DST/_test/import" -H 'Content-Type: application/json' -d @"$WORK/pair-exp.json")" 204 "pair-reimport-204" "$WORK/pair-reimport"
curl -s -X POST "$DST/reservation-moves" -H "Authorization: Bearer $PTOK2" -H 'Idempotency-Key: probe-pair-1' -H 'Content-Type: application/json' -d '{"moves":[{"reference":"PAIRCF","party_size":5},{"reference":"SINGLE1"}]}' -o "$WORK/pair-replay.json" -w '%{http_code}' > "$WORK/pair-replay-code.txt" 2>"$WORK/pair-replay.perr"
need_code "$(cat "$WORK/pair-replay-code.txt")" 200 "pair-batch-replay-200" "$WORK/pair-replay.json"
write_py paircheck.py <<'PYEOF'
import json, urllib.request
orig = json.load(open("WORKPH/pair-batch.json"))
got = json.load(open("WORKPH/pair-replay.json"))
assert got == orig, (got, orig)
base = "DSTPH"
tok = open("WORKPH/ptok.txt").read().strip()
def get(ref):
    req = urllib.request.Request(base + "/reservations/" + ref, headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req) as r:
        return json.load(r)
cf = get("PAIRCF")
assert cf["table_ids"] == ["t_1", "t_2"] and "table_id" not in cf, cf
assert cf["party_size"] == 6, cf
assert cf["status"] == "confirmed" and cf["reservation_id"] == "s1", cf
assert cf["starts_at_local"] == "2027-05-06T19:00", cf
cn = get("PAIRCN")
assert cn["table_ids"] == ["t_1", "t_2"] and cn["status"] == "cancelled", cn
assert cn["reservation_id"] == "s2" and cn["starts_at_local"] == "2027-05-06T20:30", cn
sg = get("SINGLE1")
assert sg["table_id"] == "t_3" and sg["table_ids"] == ["t_3"], sg
PYEOF
sed -i "s|WORKPH|$WORK|g;s|DSTPH|$DST|g" "$WORK/paircheck.py"
run_py "pair-mutated-replay-and-records" "$WORK/paircheck.py" || exit 1

echo
echo "RESULT pass=$PASS fail=$FAIL skip=$SKIP"
[ "$FAIL" = "0" ]
