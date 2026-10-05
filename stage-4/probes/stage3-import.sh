#!/bin/sh
# stage3-import.sh - real old-source transfer and modern collective/exception
# portability probe for stage-3.
#
# Usage: sh stage-3/probes/stage3-import.sh SRC1 SRC2 DST PEER DONORS WORK
#   SRC1/SRC2: independently running accepted stage-1/stage-2 processes.
#   DST/PEER:  two independent current-stage-3 processes.
#   DONORS:    directory with stage1/stage2 export.json + manifest.json made by
#              the existing genuine stage3-donor.sh.
#   WORK:      private output directory (created 0700; files 0600).
#
# Only WORK is written (plus OUT/tmp removed on exit; diagnostics in OUT/diag
# retained). Stdout carries names/statuses/counts only, never tokens, passwords,
# reservation bodies or export JSON. Counted units are shell expect assertions +
# whole-phase Python processes (one PASS per green phase) + phase-gate markers;
# markers confirm phase completion, not additional coverage. Python-internal
# assertion names stay in private diagnostics. Every python tool's stderr goes
# to OUT/diag;
# failures print a safe name plus a private-diagnostics pointer, count FAIL, and
# phase gates abort nonzero. No xtrace near credential operations. curl calls
# carry connect/max-time bounds. Nonzero on any effective failure; PASS/FAIL/SKIP
# counted separately.
#
# Provenance (actual deployment, not from HTTP) is fed via:
#   SRC1_IMAGE / SRC1_CONTAINER / SRC1_PORT / SRC1_CID
#   SRC2_IMAGE / SRC2_CONTAINER / SRC2_PORT / SRC2_CID
#   DST_IMAGE / DST_CONTAINER / DST_PORT / DST_CID
#   PEER_IMAGE / PEER_CONTAINER / PEER_PORT / PEER_CID

umask 077
set -u
SRC1=${1:?SRC1 stage1 url required}
SRC2=${2:?SRC2 stage2 url required}
DST=${3:?DST stage3 url required}
PEER=${4:?PEER stage3 url required}
DONORS=${5:?DONORS dir required}
WORK=${6:?WORK dir required}

# Harmless distinct URL / private-dir input validation (dead-base guard).
bad=$(printf '%s' "$SRC1$SRC2$DST$PEER" | tr -d 'a-zA-Z0-9:/._-')
if [ -n "$bad" ]; then
  echo "FAIL: bad characters in base URL"; exit 1
fi
for u in "$SRC1" "$SRC2" "$DST" "$PEER"; do
  case "$u" in
    http://127.0.0.1:*|http://localhost:*|http://10.*|http://172.*|http://192.168.*) ;;
    *) echo "FAIL: refusing non-local base $u"; exit 1;;
  esac
done
[ "$SRC1" != "$SRC2" ] && [ "$SRC1" != "$DST" ] && [ "$SRC1" != "$PEER" ] \
  && [ "$SRC2" != "$DST" ] && [ "$SRC2" != "$PEER" ] && [ "$DST" != "$PEER" ] || {
  echo "FAIL: base URLs must be four distinct processes"; exit 1
}
case "$WORK" in
  ""|/|/tmp|/tmp/|.|..) echo "FAIL: refusing unsafe WORK dir"; exit 1;;
esac
[ -f "$DONORS/stage1/export.json" ] || { echo "FAIL: donors stage1 export missing"; exit 1; }
[ -f "$DONORS/stage2/export.json" ] || { echo "FAIL: donors stage2 export missing"; exit 1; }

mkdir -p "$WORK" || exit 1
chmod 700 "$WORK" || exit 1
DIAG="$WORK/diag"; mkdir -p "$DIAG"
TMPD="$WORK/tmp.$$"; mkdir -p "$TMPD"
trap 'rm -rf "$TMPD"' EXIT INT TERM
export PYTHONDONTWRITEBYTECODE=1

PASS=0; FAIL=0; SKIP=0
expect() {
  # expect <name> <got> <want>
  if [ "$2" = "$3" ]; then PASS=$((PASS+1)); echo "PASS: $1 ($2)"; return 0; fi
  FAIL=$((FAIL+1)); echo "FAIL: $1 got=$2 want=$3 (see $DIAG)"; return 1
}
pyassert() {
  # pyassert <name> <heredoc-file> — python exits 0 on pass; stderr to diag.
  name=$1; shift
  if python3 "$@" 2>"$DIAG/$name.err"; then PASS=$((PASS+1)); echo "PASS: $name"; return 0; fi
  FAIL=$((FAIL+1)); echo "FAIL: $name (see $DIAG/$name.err)"; return 1
}
gate() {
  # gate <phase-name> — abort nonzero if any FAIL so far.
  if [ "$FAIL" -ne 0 ]; then echo "GATE-ABORT: $1 with FAIL=$FAIL"; exit 1; fi
  echo "GATE-OK: $1 (pass=$PASS fail=$FAIL skip=$SKIP)"
}

CURL="curl -sS --connect-timeout 3 --max-time 8"

echo "PROVENANCE src1 image=${SRC1_IMAGE:-?} container=${SRC1_CONTAINER:-?} port=${SRC1_PORT:-?}"
echo "PROVENANCE src2 image=${SRC2_IMAGE:-?} container=${SRC2_CONTAINER:-?} port=${SRC2_PORT:-?}"
echo "PROVENANCE dst image=${DST_IMAGE:-?} container=${DST_CONTAINER:-?} port=${DST_PORT:-?}"
echo "PROVENANCE peer image=${PEER_IMAGE:-?} container=${PEER_CONTAINER:-?} port=${PEER_PORT:-?}"

# Phase 0: liveness of all four processes.
for pair in "src1 $SRC1" "src2 $SRC2" "dst $DST" "peer $PEER"; do
  set -- $pair
  code=$($CURL -o /dev/null -w '%{http_code}' "$2/health" 2>"$DIAG/health-$1.err" || echo 000)
  expect "$1-health-200" "$code" "200" || exit 1
done
gate "liveness"

# Phase 1: fresh-source divergence / snapshot isolation.
# Binds each saved donor artifact (sha + post_snapshot_write reference) to its
# live source: the live export contains the post-snapshot booking while the
# saved export does not, and the two live states are actually distinct.
cat > "$TMPD/diverge.py" <<'PYEOF'
import hashlib, json, sys, urllib.request, urllib.error
def get(base, path):
    req = urllib.request.Request(base + path, method='GET')
    return urllib.request.urlopen(req, timeout=8).read()
src1, src2, donors, tmpd = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
live = {}
for name, base in (('stage1', src1), ('stage2', src2)):
    raw = get(base, '/_test/export')
    live[name] = raw
    man = json.load(open('%s/%s/manifest.json' % (donors, name)))
    saved = open('%s/%s/export.json' % (donors, name), 'rb').read()
    open(tmpd + '/saved-%s.sha' % name, 'w').write(hashlib.sha256(saved).hexdigest())
    post = (man.get('post_snapshot_write') or {}).get('reference')
    assert post, '%s manifest lacks post_snapshot_write' % name
    live_state = json.loads(raw.decode())['state']['reservations']
    saved_state = json.loads(saved.decode())['state']['reservations']
    live_refs = [r.get('reference') for r in live_state.values()] if isinstance(live_state, dict) else [r.get('reference') for r in live_state]
    saved_refs = [r.get('reference') for r in saved_state.values()] if isinstance(saved_state, dict) else [r.get('reference') for r in saved_state]
    assert post in live_refs, '%s live lacks post-snapshot %s' % (name, post)
    assert post not in saved_refs, '%s saved contains post-snapshot %s' % (name, post)
    src = man.get('source') or {}
    assert src.get('stage') == name, '%s manifest stage' % name
    open(tmpd + '/live-%s.sha' % name, 'w').write(hashlib.sha256(raw).hexdigest())
assert live['stage1'] != live['stage2'], 'sources not distinct'
print('DIVERGE-OK')
PYEOF
if [ "${SKIP_LIVE_SOURCE:-0}" = "1" ]; then
  SKIP=$((SKIP+1)); echo "SKIP: live-source divergence (canonical prior-process artifacts only)"
else
  pyassert "fresh-source-snapshot-isolation" "$TMPD/diverge.py" "$SRC1" "$SRC2" "$DONORS" "$TMPD" \
    > "$TMPD/diverge.out" 2>"$DIAG/diverge.err" || exit 1
grep -q '^DIVERGE-OK$' "$TMPD/diverge.out" || { FAIL=$((FAIL+1)); echo "FAIL: diverge-marker missing"; exit 1; }
PASS=$((PASS+1)); echo "PASS: diverge-marker"
fi
gate "source-isolation"

# Phase 2: old-export transfer into pre-seeded DST with replacement semantics.
# Imports ORIGINAL export file bytes unchanged (never reserialized). BEFORE any
# login-minting, compares complete old raw-state records, normalized canonical
# sets/revision/terms/history, owner lists/GETs, users/tokens/receipts, and
# destination credential removal. Receipt sets must be EXACT (stage1: 5, stage2: 8).
cat > "$TMPD/transfer.py" <<'PYEOF'
import json, sys, urllib.request, urllib.error
dst, donors, tmpd = sys.argv[1], sys.argv[2], sys.argv[3]
def call(method, path, body=None, token=None):
    data = json.dumps(body).encode() if isinstance(body, (dict, list)) else body
    req = urllib.request.Request(dst + path, data=data, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    try:
        r = urllib.request.urlopen(req, timeout=8)
        return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
def export_raw():
    req = urllib.request.Request(dst + '/_test/export', method='GET')
    return urllib.request.urlopen(req, timeout=8).read()
seed = {"users": [{"id": "u_x", "email": "x@y.zz", "password": "password1", "display_name": "X"}],
        "restaurants": [], "reservations": []}
code, _ = call('POST', '/_test/reset', seed)
assert code == 204, 'preseed reset %s' % code
code, sb = call('POST', '/auth/login', {"email": "x@y.zz", "password": "password1"})
assert code == 200, 'preseed login %s' % code
seed_tok = json.loads(sb)['token']
expect_counts = {'stage1': 5, 'stage2': 8}
for stage in ('stage1', 'stage2'):
    raw = open('%s/%s/export.json' % (donors, stage), 'rb').read()
    man = json.load(open('%s/%s/manifest.json' % (donors, stage)))
    code, _ = call('POST', '/_test/import', raw)
    assert code == 204, '%s import %s' % (stage, code)
    cur_raw = export_raw()
    cur = json.loads(cur_raw.decode())
    st = cur['state']
    # Replacement: preseed credential and token really removed.
    assert 'u_x' not in json.dumps(cur), '%s merge leak' % stage
    code, _ = call('GET', '/reservations', None, seed_tok)
    assert code == 401, '%s preseed token survives %s' % (stage, code)
    # Complete old raw-state records: every stored field, not six.
    byref = man['records']['by_reference']
    assert len(st['reservations']) == len(byref), '%s record count' % stage
    # Record owner lives in receipt responses (records carry no user_id).
    owner_of = {}
    for rc in man['receipts']:
        rr = (rc.get('response') or {})
        if rr.get('reference'):
            owner_of[rr['reference']] = rc['user_id']
    for sd in (man.get('fixture') or {}).get('seeds', []):
        if isinstance(sd, dict) and sd.get('reference'):
            owner_of.setdefault(sd['reference'], sd.get('user_id'))
    # Full expected fixture0 terms from the donor's ORIGINAL restaurant fixture.
    fix_rest = (man.get('fixture') or {}).get('restaurant')
    if isinstance(fix_rest, list):
        fix_rest = fix_rest[0] if fix_rest else {}
    if not isinstance(fix_rest, dict):
        fix_rest = {}
    want_terms = {"policy_version": 0, "slot_minutes": fix_rest.get('slot_minutes'),
                  "reservation_duration_minutes": fix_rest.get('reservation_duration_minutes'),
                  "cancellation_cutoff_minutes": fix_rest.get('cancellation_cutoff_minutes'),
                  "opening_hours": fix_rest.get('opening_hours'), "capacities": None}
    want_caps = {t['id']: t['capacity'] for t in fix_rest.get('tables', [])}
    want_terms['capacities'] = want_caps
    for ref, rec in byref.items():
        got = st['reservations'].get(ref)
        assert got is not None, '%s missing %s' % (stage, ref)
        for k in ('reservation_id', 'reference', 'restaurant_id', 'status',
                  'starts_at_local', 'starts_at', 'ends_at', 'party_size', 'created_at'):
            assert got.get(k) == rec.get(k), '%s %s field %s' % (stage, ref, k)
        assert got.get('user_id') == owner_of.get(ref), '%s %s owner' % (stage, ref)
        # Selectors: singleton normalization only (table_id xor table_ids len1).
        if 'table_id' in rec:
            assert got.get('table_id') == rec.get('table_id'), '%s %s table_id' % (stage, ref)
            assert got.get('table_ids') == [rec.get('table_id')], '%s %s table_ids' % (stage, ref)
        else:
            assert got.get('table_ids') == rec.get('table_ids'), '%s %s table_ids' % (stage, ref)
            assert not got.get('table_id'), '%s %s scalar leak' % (stage, ref)
        assert got.get('revision') == 1, '%s %s revision' % (stage, ref)
        terms = got.get('accepted_terms', {})
        assert terms == want_terms, '%s %s terms values' % (stage, ref)
        assert 'effective_from' not in terms, '%s %s terms effective_from' % (stage, ref)
        h = st['histories'].get(ref, [])
        assert len(h) == 1 and h[0]['event'] == 'created', '%s %s history len' % (stage, ref)
        e0 = h[0]
        assert e0.get('seq') == 1 and e0.get('revision') == 1, '%s %s history seq/rev' % (stage, ref)
        assert e0.get('at') == got.get('created_at'), '%s %s history at' % (stage, ref)
        assert e0.get('accepted_terms') == want_terms, '%s %s history terms' % (stage, ref)
        chs = e0.get('changes', [])
        if 'table_id' in rec:
            assert [c.get('field') for c in chs] == ['table_id', 'starts_at_local', 'party_size'], '%s %s changes' % (stage, ref)
            assert chs[0].get('to') == rec.get('table_id'), '%s %s changes table to' % (stage, ref)
        else:
            assert [c.get('field') for c in chs] == ['table_ids', 'starts_at_local', 'party_size'], '%s %s changes' % (stage, ref)
            assert chs[0].get('to') == rec.get('table_ids'), '%s %s changes tables to' % (stage, ref)
        assert chs[1].get('to') == rec.get('starts_at_local'), '%s %s changes time to' % (stage, ref)
        assert chs[2].get('to') == rec.get('party_size'), '%s %s changes party to' % (stage, ref)
        assert all(c.get('from') is None for c in chs), '%s %s changes from' % (stage, ref)
    # Exact expected owner record SET from old raw owners (receipts + seeds),
    # FULL normalized public projections, and per-ref GET full equality.
    want_owned = {}
    for ref, uid in owner_of.items():
        want_owned.setdefault(uid, set()).add(ref)
    for u in man['users']:
        tok = u['tokens'][0]
        code, lb = call('GET', '/reservations', None, tok)
        assert code == 200, '%s list %s' % (stage, u['id'])
        got_list = json.loads(lb).get('reservations', [])
        assert set(r['reference'] for r in got_list) == want_owned.get(u['id'], set()), '%s %s record set' % (stage, u['id'])
        for rec in got_list:
            want = st['reservations'][rec['reference']]
            assert 'user_id' not in rec, '%s %s projection leaks owner' % (stage, rec['reference'])
            for k in ('reservation_id', 'reference', 'restaurant_id', 'status',
                      'starts_at_local', 'starts_at', 'ends_at', 'party_size', 'created_at',
                      'revision', 'table_ids'):
                assert rec.get(k) == want.get(k), '%s %s projection %s' % (stage, rec['reference'], k)
            assert rec.get('accepted_terms') == want.get('accepted_terms'), '%s %s projection terms' % (stage, rec['reference'])
            code, gb = call('GET', '/reservations/' + rec['reference'], None, tok)
            assert code == 200, '%s get %s' % (stage, rec['reference'])
            assert json.loads(gb) == rec, '%s get/list differ %s' % (stage, rec['reference'])
    # Whole users map (ids/emails/names, no plaintext), full token map, original
    # fixture (manager ids default []), all receipt scope fields; exact namespaces.
    assert set(st['users'].keys()) == set(u['id'] for u in man['users']), '%s users map' % stage
    for u in man['users']:
        su = st['users'][u['id']]
        assert su.get('email') == u['email'] and su.get('display_name') == u['display_name'], '%s user %s' % (stage, u['id'])
        assert 'password' not in su and su.get('password_hash'), '%s plaintext %s' % (stage, u['id'])
    assert set(st.get('tokens', {}).keys()) == set(t for u in man['users'] for t in u['tokens']), '%s token map' % stage
    for u in man['users']:
        for t in u['tokens']:
            assert st['tokens'][t] == u['id'], '%s token owner' % stage
    assert (st.get('policies') or {}) == {}, '%s policies %s' % (stage, st.get('policies'))
    assert (st.get('series') or {}) == {}, '%s series' % stage
    assert set((st.get('restaurant_revisions') or {}).keys()) == {'r_anker'}, '%s counter keys' % (stage)
    assert all(v == 0 for v in (st.get('restaurant_revisions') or {}).values()), '%s counters' % stage
    for r in st.get('restaurants', []):
        assert r.get('manager_user_ids', []) == [], '%s managers %s' % (stage, r.get('id'))
    for ns in ('users', 'tokens', 'restaurants', 'reservations', 'receipts', 'histories'):
        assert ns in st, '%s namespace %s' % (stage, ns)
    # ALL receipt scopes with body/response/raw bytes; EXACT counts.
    assert len(man['receipts']) == expect_counts[stage], '%s receipt count' % stage
    got_rc = st['receipts']
    got_list = list(got_rc.values()) if isinstance(got_rc, dict) else got_rc
    assert len(got_list) == expect_counts[stage], '%s stored receipt count' % stage
    for r in man['receipts']:
        match = [v for v in got_list if v.get('key') == r['key']]
        assert match, '%s receipt %s missing' % (stage, r['key'])
        assert len(match) == 1, '%s receipt %s dup' % (stage, r['key'])
        assert match[0].get('method') == r['method'] and match[0].get('path') == r['path'], '%s receipt %s scope' % (stage, r['key'])
        assert match[0].get('user_id') == r['user_id'], '%s receipt %s owner' % (stage, r['key'])
        assert match[0].get('key') == r['key'] and match[0].get('status') == r['status'], '%s receipt %s key/status' % (stage, r['key'])
        assert json.loads(r['body_raw']) == r['body'], '%s receipt %s raw/parsed body' % (stage, r['key'])
        assert json.loads(match[0].get('body')) == r['body'], '%s receipt %s stored body' % (stage, r['key'])
        assert match[0].get('response') == r['response_raw'].rstrip('\n') or match[0].get('response') == r['response_raw'], '%s receipt %s response' % (stage, r['key'])
    open(tmpd + '/%s-post.json' % stage, 'w').write(json.dumps(cur, sort_keys=True))
    code, _ = call('POST', '/_test/import', raw)
    assert code == 204, '%s reimport %s' % (stage, code)
    assert export_raw() == cur_raw, '%s reimport unstable' % stage
print('TRANSFER-OK')
PYEOF
pyassert "old-export-transfer-replacement" "$TMPD/transfer.py" "$DST" "$DONORS" "$TMPD" \
  > "$TMPD/transfer.out" 2>"$DIAG/transfer.err" || exit 1
grep -q '^TRANSFER-OK$' "$TMPD/transfer.out" || { FAIL=$((FAIL+1)); echo "FAIL: transfer-marker missing"; exit 1; }
PASS=$((PASS+1)); echo "PASS: transfer-marker"
gate "transfer"

# Phase 3: donor sessions — all four tokens per source, hash logins, owner
# privacy (both directions), a REQUIRED actual cancel200 with current-vs-receipt
# proof, ALL exact raw replays after mutation with export-unchanged-by-replay,
# REQUIRED pending retry, and a proved-absent unused key (valid body 201,
# identical retry 200, new receipt/counter exactly once).
cat > "$TMPD/sessions.py" <<'PYEOF'
import json, sys, urllib.request, urllib.error
dst, donors, tmpd = sys.argv[1], sys.argv[2], sys.argv[3]
PRIV = tmpd + '/priv'
import os
os.makedirs(PRIV, exist_ok=True)
def call(method, path, body=None, token=None, key=None):
    if isinstance(body, (dict, list)):
        data = json.dumps(body).encode()
    elif isinstance(body, str):
        data = body.encode()
    else:
        data = body
    req = urllib.request.Request(dst + path, data=data, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    if key:
        req.add_header('Idempotency-Key', key)
    try:
        r = urllib.request.urlopen(req, timeout=8)
        return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
def export_raw():
    req = urllib.request.Request(dst + '/_test/export', method='GET')
    return urllib.request.urlopen(req, timeout=8).read()
expect_replays = {'stage1': 5, 'stage2': 8}
for stage in ('stage1', 'stage2'):
    gen = json.load(open('%s/%s/export.json' % (donors, stage)))
    man = json.load(open('%s/%s/manifest.json' % (donors, stage)))
    code, _ = call('POST', '/_test/import', gen)
    assert code == 204, '%s import %s' % (stage, code)
    users = man['users']
    assert len(users) == 2, '%s users' % stage
    for u in users:
        assert len(u['tokens']) == 2, '%s %s tokens' % (stage, u['id'])
        for tok in u['tokens']:
            code, _ = call('GET', '/reservations', None, tok)
            assert code == 200, '%s token %s' % (stage, code)
        code, _ = call('POST', '/auth/login', {'email': u['email'], 'password': u['password']})
        assert code == 200, '%s hash login %s' % (stage, u['id'])
    toks = {u['id']: u['tokens'][0] for u in users}
    # Owner privacy in BOTH directions between the two owners.
    refs = {}
    for r in man['receipts']:
        resp = r.get('response') or {}
        if resp.get('reference') and resp.get('status') == 'confirmed':
            refs.setdefault(r['user_id'], []).append(resp['reference'])
    ids = list(refs.keys())
    assert len(ids) == 2, '%s two owners' % stage
    code, _ = call('GET', '/reservations/' + refs[ids[1]][0], None, toks[ids[0]])
    assert code == 404, '%s cross-owner a %s' % (stage, code)
    code, _ = call('GET', '/reservations/' + refs[ids[0]][0], None, toks[ids[1]])
    assert code == 404, '%s cross-owner b %s' % (stage, code)
    # REQUIRED actual mutation: cancel one eligible FUTURE confirmed record 200.
    # Seeds are 2020 past (cutoff blocks); fixture records are 2026-11-12 future.
    cancelled, orig_resp = None, None
    for uid in ids:
        code, lb = call('GET', '/reservations', None, toks[uid])
        assert code == 200
        for rec in json.loads(lb).get('reservations', []):
            if rec.get('status') == 'confirmed' and rec.get('starts_at_local', '') >= '2026-11-12':
                for r in man['receipts']:
                    if (r.get('response') or {}).get('reference') == rec['reference']:
                        orig_resp = r['response_raw']
                code, cb = call('POST', '/reservations/' + rec['reference'] + '/cancel', {}, toks[uid])
                assert code == 200, '%s cancel %s = %s' % (stage, rec['reference'], code)
                cancelled = (uid, rec['reference'])
                break
        if cancelled:
            break
    assert cancelled, '%s no cancellable future record' % stage
    uid, cref = cancelled
    code, cb = call('GET', '/reservations/' + cref, None, toks[uid])
    cur = json.loads(cb)
    assert cur['status'] == 'cancelled' and cur['revision'] == 2, '%s current not cancelled/rev2' % stage
    assert orig_resp, '%s no source original receipt for %s' % (stage, cref)
    assert json.loads(orig_resp)['status'] == 'confirmed', '%s receipt not original' % stage
    assert json.loads(orig_resp)['reference'] == cref, '%s receipt ref' % stage
    code, hb = call('GET', '/reservations/' + cref + '/history', None, toks[uid])
    assert code == 200 and len(json.loads(hb)['entries']) == 2, '%s history not created+cancelled' % stage
    # Baseline BEFORE all replays; every replay 200 + verbatim raw bytes with
    # EXACT counts; whole export unchanged AFTER ALL replays.
    pre_replay = export_raw()
    n = 0
    for r in man['receipts']:
        if r['method'] != 'POST' or r['path'] not in ('/reservations', '/reservation-moves'):
            continue
        code, body = call('POST', r['path'], r['body_raw'], toks[r['user_id']], r['key'])
        assert code == 200, '%s replay %s = %s' % (stage, r['key'], code)
        assert body == r['response_raw'], '%s replay %s bytes' % (stage, r['key'])
        n += 1
    assert n == expect_replays[stage], '%s replays %d' % (stage, n)
    assert export_raw() == pre_replay, '%s replay mutates export' % stage
    # Pending retry REQUIRED: raw-string equality with the original pending
    # receipt (ref/status/key/body), plus no extra record.
    pr = man.get('pending_retry') or {}
    assert pr.get('key') and pr.get('body_raw') and pr.get('reference'), '%s pending fields' % stage
    assert pr.get('response_raw'), '%s pending raw' % stage
    n_before = len(json.loads(export_raw().decode())['state']['reservations'])
    code, body = call(pr['method'], pr['path'], pr['body_raw'], toks[pr['owner']], pr['key'])
    assert code == 200, '%s pending retry %s' % (stage, code)
    assert body == pr['response_raw'], '%s pending raw bytes' % stage
    assert json.loads(body).get('reference') == pr['reference'], '%s pending ref' % stage
    assert len(json.loads(export_raw().decode())['state']['reservations']) == n_before, '%s pending extra record' % stage
    # Truly unused key: proved ABSENT from stored receipts, then valid free-slot
    # body 201, identical retry 200/raw bytes; pins exact new record + receipt +
    # counter + owner scope against pre-write baselines.
    fk = (man.get('failed_keys') or {}).get('absent')
    assert isinstance(fk, list) and fk, '%s absent keys' % stage
    f = fk[0]
    fk_uid = f.get('user_id') or f.get('owner')
    pre_state = json.loads(export_raw().decode())['state']
    stored_keys = [v.get('key') for v in pre_state['receipts'].values()] if isinstance(pre_state['receipts'], dict) else [v.get('key') for v in pre_state['receipts']]
    assert f['key'] not in stored_keys, '%s key already used' % stage
    n_rc_before = len(stored_keys)
    n_res_before = len(pre_state['reservations'])
    ctr_before = dict(pre_state.get('restaurant_revisions') or {})
    valid = {"restaurant_id": "r_anker", "table_id": "t_3",
             "starts_at_local": "2026-11-19T18:00", "party_size": 1}
    code, b1 = call('POST', '/reservations', valid, toks[fk_uid], f['key'])
    assert code == 201, '%s unused key first %s %s' % (stage, code, b1[:120])
    new_ref = json.loads(b1)['reference']
    code, b2 = call('POST', '/reservations', valid, toks[fk_uid], f['key'])
    assert code == 200 and b2 == b1, '%s unused key retry' % (stage, code)
    after = json.loads(export_raw().decode())['state']
    got_list = list(after['receipts'].values()) if isinstance(after['receipts'], dict) else after['receipts']
    assert len(got_list) == n_rc_before + 1, '%s receipt count' % stage
    assert len(after['reservations']) == n_res_before + 1, '%s record count' % stage
    assert after['reservations'][new_ref]['user_id'] == fk_uid, '%s new record owner' % stage
    for rid, c in ctr_before.items():
        assert (after.get('restaurant_revisions') or {}).get(rid, 0) >= c, '%s counter regressed' % stage
    assert sum((after.get('restaurant_revisions') or {}).values()) == sum(ctr_before.values()) + 1, '%s counter +1' % stage
    # Reimport ORIGINAL FILE BYTES in sessions too (not remarshal).
    raw_file = open('%s/%s/export.json' % (donors, stage), 'rb').read()
    code, _ = call('POST', '/_test/import', raw_file)
    assert code == 204, '%s file reimport %s' % (stage, code)
print('SESSIONS-OK')
PYEOF
pyassert "donor-sessions-replays" "$TMPD/sessions.py" "$DST" "$DONORS" "$TMPD" \
  > "$TMPD/sessions.out" 2>"$DIAG/sessions.err" || exit 1
grep -q '^SESSIONS-OK$' "$TMPD/sessions.out" || { FAIL=$((FAIL+1)); echo "FAIL: sessions-marker missing"; exit 1; }
PASS=$((PASS+1)); echo "PASS: sessions-marker"
gate "sessions"

# Phase 4: modern producer on DST — policy supersession, fixture0 above-max seed,
# pair history, adopted series, PATCH exception then cancel (flag survives),
# mixed collective batch; captures first-201 raw receipts, asserts full
# record/terms/revision/history/counter metadata, then a later mutation so
# receipt bytes provably differ from current. Saves RAW export bytes + public
# GET/history/decision/series snapshots.
cat > "$TMPD/modern.py" <<'PYEOF'
import json, sys, urllib.request, urllib.error, os
peer = sys.argv[1]
tmpd = sys.argv[2]
PRIV = tmpd + '/priv'
os.makedirs(PRIV, exist_ok=True)
def call(method, path, body=None, token=None, key=None):
    if isinstance(body, (dict, list)):
        data = json.dumps(body).encode()
    elif isinstance(body, str):
        data = body.encode()
    else:
        data = body
    req = urllib.request.Request(peer + path, data=data, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    if key:
        req.add_header('Idempotency-Key', key)
    try:
        r = urllib.request.urlopen(req, timeout=8)
        return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
def export_raw():
    req = urllib.request.Request(peer + '/_test/export', method='GET')
    return urllib.request.urlopen(req, timeout=8).read()
fix = {"users": [{"id": "u1", "email": "m@n.oo", "password": "password1", "display_name": "M"}],
       "restaurants": [{"id": "r1", "name": "N", "timezone": "Europe/Berlin", "slot_minutes": 30,
                        "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 120,
                        "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
                        "tables": [{"id": "t_1", "label": "1", "capacity": 2},
                                   {"id": "t_2", "label": "2", "capacity": 4}],
                        "combinable": [["t_1", "t_2"]], "manager_user_ids": ["u1"]},
                      {"id": "rbig", "name": "Big", "timezone": "Europe/Berlin", "slot_minutes": 30,
                       "reservation_duration_minutes": 90, "cancellation_cutoff_minutes": 10081,
                       "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
                       "tables": [{"id": "b1", "label": "1", "capacity": 101}]}],
       "reservations": []}
assert call('POST', '/_test/reset', fix)[0] == 204
_, lb = call('POST', '/auth/login', {"email": "m@n.oo", "password": "password1"})
tok = json.loads(lb)['token']
open(PRIV + '/mtok', 'w').write(tok)
receipts = []
def pub(key, frm, dur):
    body = {"effective_from": frm, "slot_minutes": 30, "reservation_duration_minutes": dur,
            "cancellation_cutoff_minutes": 60,
            "opening_hours": [{"weekday": "thu", "opens": "18:00", "closes": "23:00"}],
            "capacities": {"t_1": 2, "t_2": 4}}
    code, b = call('POST', '/restaurants/r1/policies', body, tok, key)
    assert code == 201, 'pub %s %s' % (key, code)
    receipts.append({"key": key, "path": "/restaurants/r1/policies", "body": body, "response_raw": b})
pub('mp1', '2027-05-13', 120)
pub('mp2', '2027-05-13', 60)
def create(key, body):
    code, b = call('POST', '/reservations', body, tok, key)
    assert code == 201, 'create %s %s' % (key, code)
    receipts.append({"key": key, "path": "/reservations", "body": body, "response_raw": b})
    return json.loads(b)['reference']
def patch(ref, body):
    code, b = call('PATCH', '/reservations/' + ref, body, tok)
    assert code == 200, 'patch %s %s %s' % (ref, code, b[:100])
big = create('mk-big', {"restaurant_id": "rbig", "table_id": "b1",
                         "starts_at_local": "2027-05-06T19:00", "party_size": 101})
pair = create('mk-pair', {"restaurant_id": "r1", "table_ids": ["t_1", "t_2"],
                          "starts_at_local": "2027-05-06T19:00", "party_size": 2})
patch(pair, {"table_id": "t_1"})
patch(pair, {"table_ids": ["t_1", "t_2"]})
a1 = create('mk-a1', {"restaurant_id": "r1", "table_id": "t_1",
                      "starts_at_local": "2027-05-06T20:30", "party_size": 1})
a2 = create('mk-a2', {"restaurant_id": "r1", "table_id": "t_2",
                      "starts_at_local": "2027-05-06T21:30", "party_size": 1})
def adopt(key, anchor):
    code, b = call('POST', '/series', {"anchor_reference": anchor, "count": 2, "interval_weeks": 1}, tok, key)
    assert code == 201, 'adopt %s %s' % (key, code)
    receipts.append({"key": key, "path": "/series", "body": {"anchor_reference": anchor, "count": 2, "interval_weeks": 1}, "response_raw": b})
    return json.loads(b)
s1 = adopt('mk-s1', a1)
s2 = adopt('mk-s2', a2)
g1 = s1['occurrences'][1]['reference']
g2 = s2['occurrences'][1]['reference']
patch(g1, {"party_size": 2})
code, _ = call('POST', '/reservations/' + g1 + '/cancel', {}, tok)
assert code == 200, 'cancel g1 %s' % code
code, _ = call('POST', '/reservations/' + g2 + '/cancel', {}, tok)
assert code == 200, 'cancel g2 %s' % code
pre_batch_raw = export_raw()
pre_batch = json.loads(pre_batch_raw.decode())['state']
pre_ctr = dict(pre_batch.get('restaurant_revisions') or {})
code, bb = call('POST', '/reservation-moves',
               {"moves": [{"reference": a1, "party_size": 2}, {"reference": a2, "party_size": 2}]}, tok, 'mk-batch')
assert code == 201, 'batch %s' % code
receipts.append({"key": 'mk-batch', "path": "/reservation-moves",
                 "body": {"moves": [{"reference": a1, "party_size": 2}, {"reference": a2, "party_size": 2}]}, "response_raw": bb})
post_batch = json.loads(export_raw().decode())['state']
post_ctr = post_batch.get('restaurant_revisions') or {}
assert set(post_ctr.keys()) == set(pre_ctr.keys()), 'counter keys changed'
for rid, c in pre_ctr.items():
    want = c + (1 if rid == 'r1' else 0)
    assert post_ctr.get(rid) == want, 'restaurant %s counter %s want %s' % (rid, post_ctr.get(rid), want)
assert post_batch['series'][s1['series_id']]['revision'] == pre_batch['series'][s1['series_id']]['revision'] + 1, 's1 exact +1'
assert post_batch['series'][s2['series_id']]['revision'] == pre_batch['series'][s2['series_id']]['revision'] + 1, 's2 exact +1'
assert len(post_batch['series']) == len(pre_batch['series']), 'unrelated series retained'
for ref, want_rev, want_n in ((a1, 2, 2), (a2, 2, 2)):
    rh = [e for e in post_batch['histories'][ref]]
    assert post_batch['reservations'][ref]['revision'] == want_rev, 'booking %s rev' % ref
    assert len(rh) == want_n, 'booking %s entries' % ref
    assert rh[0]['event'] == 'created' and rh[1]['event'] == 'changed', 'booking %s events' % ref
    assert rh[1]['revision'] == want_rev, 'booking %s changed rev' % ref
assert post_batch['series'][s1['series_id']]['members'][0]['exception'] is True, 'a1 flag'
assert post_batch['series'][s2['series_id']]['members'][0]['exception'] is True, 'a2 flag'
assert post_batch['series'][s1['series_id']]['members'][1]['exception'] is True, 'g1 flag retained'
assert post_batch['series'][s2['series_id']]['members'][1]['exception'] is False, 'g2 flag retained'
code, pb = call('GET', '/reservations/' + pair, None, tok)
prec = json.loads(pb)
assert prec['revision'] == 3, 'pair revision %s' % prec['revision']
assert prec['table_ids'] == ['t_1', 't_2'] and 'table_id' not in prec, 'pair shape'
code, hb = call('GET', '/reservations/' + pair + '/history', None, tok)
hents = json.loads(hb)['entries']
assert [e['event'] for e in hents] == ['created', 'changed', 'changed'], 'pair events'
assert hents[0]['changes'][0] == {'field': 'table_ids', 'from': None, 'to': ['t_1', 't_2']}, 'pair created selector'
assert hents[1]['changes'][0]['field'] == 'table_ids', 'pair changed selector'
assert hents[2]['changes'][0]['field'] == 'table_ids', 'pair changed back selector'
assert [e['revision'] for e in hents] == [1, 2, 3], 'pair revisions'
code, db = call('GET', '/reservations/' + pair + '/decision', None, tok)
assert json.loads(db)['revision'] == 3, 'pair decision'
code, a1g = call('GET', '/reservations/' + a1, None, tok)
a1r = json.loads(a1g)
assert a1r['accepted_terms']['policy_version'] == 0, 'anchor fixture0'
assert a1r['accepted_terms']['reservation_duration_minutes'] == 90, 'anchor duration'
assert a1r['ends_at'] == '2027-05-06T22:00:00+02:00', 'anchor absolute end'
code, g1g = call('GET', '/reservations/' + g1, None, tok)
g1r = json.loads(g1g)
assert g1r['accepted_terms']['policy_version'] == 2, 'generated superseded policy2'
assert g1r['accepted_terms']['reservation_duration_minutes'] == 60, 'generated duration 60'
assert g1r['ends_at'] == '2027-05-13T21:30:00+02:00', 'generated absolute end'
code, bg = call('GET', '/reservations/' + big, None, tok)
assert json.loads(bg)['ends_at'] == '2027-05-06T20:30:00+02:00', 'big absolute end'
code, gb = call('GET', '/reservations/' + big, None, tok)
grec = json.loads(gb)
assert grec['accepted_terms']['cancellation_cutoff_minutes'] == 10081, 'big cutoff'
assert grec['accepted_terms']['capacities'] == {'b1': 101}, 'big caps'
assert set(grec['accepted_terms'].keys()) == {'policy_version', 'slot_minutes', 'reservation_duration_minutes', 'cancellation_cutoff_minutes', 'opening_hours', 'capacities'}, 'big terms keys'
refs_by_flag = {}
code, s1b = call('GET', '/series/' + s1['series_id'], None, tok)
s1occ = json.loads(s1b)['occurrences']
assert len(s1occ) == 2, 's1 len'
assert s1occ[0]['reference'] == a1 and s1occ[0]['exception'] is True, 'a1 batch exception'
assert s1occ[1]['reference'] == g1 and s1occ[1]['exception'] is True, 'g1 permanent exception survives cancel'
assert s1occ[1]['reservation']['status'] == 'cancelled', 'g1 cancelled'
assert json.loads(s1b)['revision'] >= 3, 's1 revision'
code, s2b = call('GET', '/series/' + s2['series_id'], None, tok)
s2occ = json.loads(s2b)['occurrences']
assert len(s2occ) == 2, 's2 len'
assert s2occ[0]['reference'] == a2 and s2occ[0]['exception'] is True, 'a2 batch exception'
assert s2occ[1]['reference'] == g2 and s2occ[1]['exception'] is False, 'g2 cancel no exception'
assert s2occ[1]['reservation']['status'] == 'cancelled', 'g2 cancelled'
json.dump(receipts, open(tmpd + '/modern-receipts.json', 'w'))
# Later mutation AFTER receipt capture so receipt bytes provably differ from current.
patch(a1, {"party_size": 1})
code, a1b = call('GET', '/reservations/' + a1, None, tok)
assert json.loads(a1b)['party_size'] == 1 and json.loads(a1b)['revision'] == 3, 'later mutation'
a1orig = [r for r in receipts if r['path'] == '/reservations' and r['body'].get('starts_at_local') == '2027-05-06T20:30']
assert len(a1orig) == 1, 'mk-a1 original bound'
a1o = json.loads(a1orig[0]['response_raw'])
assert a1o['reference'] == a1 and a1o['party_size'] == 1 and a1o['revision'] == 1, 'mk-a1 original 1/1'
a1c = json.loads(a1b)
assert a1c['reference'] == a1 and a1c['party_size'] == 1 and a1c['revision'] == 3, 'a1 current 1/3'
assert a1c['reservation_id'] == a1o['reservation_id'], 'a1 identity'
assert a1b != a1orig[0]['response_raw'], 'original/current full JSON differ'
batch_orig = [r for r in receipts if r['key'] == 'mk-batch']
assert len(batch_orig) == 1, 'mk-batch bound'
bresp = json.loads(batch_orig[0]['response_raw'])
b_a1 = [x for x in bresp['reservations'] if x['reference'] == a1]
assert len(b_a1) == 1 and b_a1[0]['party_size'] == 2 and b_a1[0]['revision'] == 2, 'batch a1 2/2'
# Save RAW DST export bytes + public snapshots (used by peer phase).
raw = export_raw()
open(tmpd + '/modern-snap.raw', 'wb').write(raw)
snap = json.loads(raw.decode())
for ref in (pair, a1, a2, g1, g2, big):
    code, gb = call('GET', '/reservations/' + ref, None, tok)
    assert code == 200, 'snap get %s' % ref
    open(tmpd + '/snap-get-%s.json' % ref, 'w').write(gb)
    code, hb = call('GET', '/reservations/' + ref + '/history', None, tok)
    assert code == 200, 'snap history %s' % ref
    open(tmpd + '/snap-history-%s.json' % ref, 'w').write(hb)
    code, db = call('GET', '/reservations/' + ref + '/decision', None, tok)
    assert code == 200, 'snap decision %s' % ref
    open(tmpd + '/snap-decision-%s.json' % ref, 'w').write(db)
for sid in (s1['series_id'], s2['series_id']):
    code, sb = call('GET', '/series/' + sid, None, tok)
    assert code == 200, 'snap series %s' % sid
    open(tmpd + '/snap-series-%s.json' % sid, 'w').write(sb)
print('MODERN-OK')
PYEOF
pyassert "modern-producer-dst" "$TMPD/modern.py" "$DST" "$TMPD" \
  > "$TMPD/modern.out" 2>"$DIAG/modern.err" || exit 1
grep -q '^MODERN-OK$' "$TMPD/modern.out" || { FAIL=$((FAIL+1)); echo "FAIL: modern-marker missing"; exit 1; }
PASS=$((PASS+1)); echo "PASS: modern-marker"
gate "modern-producer"

# Phase 5: raw DST bytes into independent PEER 204; immediate raw GET-export ==
# original raw bytes; reimport raw stable. Complete owner GET/history/decision/
# series compared to saved DST surfaces for every member (incl. cancelled and
# permanent exceptions and pair transitions). Every captured scoped original
# replayed via actual HTTP 200/raw equality while current provably differs and
# the whole export is unchanged by replay. Invalid 422 + malformed 400 each
# byte-atomic. Reset empties everything and revokes the old token (GET 401).
cat > "$TMPD/peer.py" <<'PYEOF'
import glob, json, os, sys, urllib.request, urllib.error
dst, peer, tmpd = sys.argv[1], sys.argv[2], sys.argv[3]
PRIV = tmpd + '/priv'
def call(base, method, path, body=None, token=None, key=None):
    if isinstance(body, (dict, list)):
        data = json.dumps(body).encode()
    elif isinstance(body, str):
        data = body.encode()
    else:
        data = body
    req = urllib.request.Request(base + path, data=data, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    if key:
        req.add_header('Idempotency-Key', key)
    try:
        r = urllib.request.urlopen(req, timeout=8)
        return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
def getexp_raw(base):
    req = urllib.request.Request(base + '/_test/export', method='GET')
    return urllib.request.urlopen(req, timeout=8).read()
snap_raw = open(tmpd + '/modern-snap.raw', 'rb').read()
code, _ = call(peer, 'POST', '/_test/import', snap_raw)
assert code == 204, 'peer import %s' % code
assert getexp_raw(peer) == snap_raw, 'peer raw bytes differ'
code, _ = call(peer, 'POST', '/_test/import', snap_raw)
assert code == 204, 'peer reimport %s' % code
assert getexp_raw(peer) == snap_raw, 'peer reimport unstable'
# Baselines were captured BEFORE this minting login, so equality is meaningful.
code, lb = call(peer, 'POST', '/auth/login', {"email": "m@n.oo", "password": "password1"})
assert code == 200, 'peer login %s' % code
tok = json.loads(lb)['token']
open(PRIV + '/peer-tok', 'w').write(tok)
for f in sorted(glob.glob(tmpd + '/snap-get-*.json')):
    want_raw = open(f).read()
    want = json.loads(want_raw)
    code, gb = call(peer, 'GET', '/reservations/' + want['reference'], None, tok)
    assert code == 200, 'peer get %s' % want['reference']
    assert gb == want_raw, 'peer GET bytes differ %s' % want['reference']
for f in sorted(glob.glob(tmpd + '/snap-history-*.json')):
    want_raw = open(f).read()
    ref = f.rsplit('-', 1)[1].rsplit('.', 1)[0]
    code, hb = call(peer, 'GET', '/reservations/' + ref + '/history', None, tok)
    assert code == 200, 'peer history %s' % ref
    assert hb == want_raw, 'peer history bytes differ %s' % ref
for f in sorted(glob.glob(tmpd + '/snap-decision-*.json')):
    want_raw = open(f).read()
    ref = f.rsplit('-', 1)[1].rsplit('.', 1)[0]
    code, db = call(peer, 'GET', '/reservations/' + ref + '/decision', None, tok)
    assert code == 200, 'peer decision %s' % ref
    assert db == want_raw, 'peer decision bytes differ %s' % ref
for f in sorted(glob.glob(tmpd + '/snap-series-*.json')):
    want_raw = open(f).read()
    sid = f.rsplit('-', 1)[1].rsplit('.', 1)[0]
    code, sb = call(peer, 'GET', '/series/' + sid, None, tok)
    assert code == 200, 'peer series %s' % sid
    assert sb == want_raw, 'peer series bytes differ %s' % sid
# Every captured scoped original replayed 200/raw-equal; export unchanged.
receipts = json.load(open(tmpd + '/modern-receipts.json'))
assert len(receipts) >= 9, 'receipts %d' % len(receipts)
pre_replay = getexp_raw(peer)
for r in receipts:
    code, b = call(peer, 'POST', r['path'], r['body'], tok, r['key'])
    assert code == 200, 'replay %s = %s' % (r['key'], code)
    assert b == r['response_raw'], 'replay %s bytes' % r['key']
assert getexp_raw(peer) == pre_replay, 'peer mutated by replays'
# SAME bound mk-a1/mk-batch original/current differences on PEER, before/after replay.
code, lb = call(peer, 'GET', '/reservations', None, tok)
assert code == 200
refs = {r['reference']: r for r in json.loads(lb).get('reservations', [])}
a1o = [r for r in receipts if r['key'] == 'mk-a1']
assert len(a1o) == 1, 'peer mk-a1 bound'
a1o_body = json.loads(a1o[0]['response_raw'])
pa1 = refs[a1o_body['reference']]
assert a1o_body['party_size'] == 1 and a1o_body['revision'] == 1, 'peer mk-a1 original 1/1'
assert pa1['party_size'] == 1 and pa1['revision'] == 3, 'peer a1 current 1/3'
assert pa1['reservation_id'] == a1o_body['reservation_id'], 'peer a1 identity'
bo = [r for r in receipts if r['key'] == 'mk-batch']
assert len(bo) == 1, 'peer mk-batch bound'
b_a1 = [x for x in json.loads(bo[0]['response_raw'])['reservations'] if x['reference'] == pa1['reference']]
assert len(b_a1) == 1 and b_a1[0]['party_size'] == 2 and b_a1[0]['revision'] == 2, 'peer batch a1 2/2'
# Invalid 422 + malformed 400 each individually byte-atomic.
bad = json.loads(snap_raw.decode())
bad['track'] = 'wrong'
code, _ = call(peer, 'POST', '/_test/import', bad)
assert code == 422, 'wrong track %s' % code
assert getexp_raw(peer) == pre_replay, 'peer mutated by 422'
code, _ = call(peer, 'POST', '/_test/import', '{invalid json')
assert code == 400, 'malformed %s' % code
assert getexp_raw(peer) == pre_replay, 'peer mutated by 400'
# Reset empties everything and revokes the old token (GET 401, not just login).
code, _ = call(peer, 'POST', '/_test/reset', {"users": [{"id": "u9", "email": "n@o.oo", "password": "password1", "display_name": "N"}], "restaurants": [], "reservations": []})
assert code == 204, 'peer reset %s' % code
st = json.loads(getexp_raw(peer).decode())['state']
assert st.get('reservations') in (None, {}, []), 'reset records'
assert st.get('receipts') in (None, {}, []), 'reset receipts'
assert st.get('restaurants') in (None, []), 'reset restaurants'
assert st.get('users') == {'u9': st.get('users', {}).get('u9')}, 'reset users replaced'
assert st.get('policies') in (None, {}, []), 'reset policies'
assert st.get('histories') in (None, {}, []), 'reset histories'
assert st.get('series') in (None, {}, []), 'reset series'
assert st.get('restaurant_revisions') in (None, {}, []), 'reset counters'
code, _ = call(peer, 'GET', '/reservations', None, tok)
assert code == 401, 'old token survives reset %s' % code
print('PEER-OK')
PYEOF
pyassert "peer-transfer-atomicity" "$TMPD/peer.py" "$DST" "$PEER" "$TMPD" \
  > "$TMPD/peer.out" 2>"$DIAG/peer.err" || exit 1
grep -q '^PEER-OK$' "$TMPD/peer.out" || { FAIL=$((FAIL+1)); echo "FAIL: peer-marker missing"; exit 1; }
PASS=$((PASS+1)); echo "PASS: peer-marker"
gate "peer"

# Phase 6: effective guard self-checks. With STAGE3_IMPORT_SABOTAGE=1 the probe
# injects a wrong expectation through the existing expect helper (counts FAIL)
# and reaches the normal nonzero phase gate — proving the guard path executes
# the real assertions. Clean runs (flag unset) require zero failures.
if [ "${STAGE3_IMPORT_SABOTAGE:-0}" = "1" ]; then
  expect "sabotage-injected-expectation" "200" "422"
fi
gate "guards"
chmod -R go-rwx "$WORK" 2>/dev/null || true
echo "RESULT pass=$PASS fail=$FAIL skip=$SKIP"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
