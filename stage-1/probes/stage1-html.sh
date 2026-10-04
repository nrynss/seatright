#!/bin/sh
# Probes packaged HTML/assets: page routes return 200 HTML, bundled JS/CSS
# are served with correct content types, API misses stay JSON 404, and no
# fetched asset URL points off-origin (absolute http(s) loads outside
# data:/svelte-namespace strings fail this probe).
#
# Usage: sh probes/stage1-html.sh <base-url> <workdir>
set -eu
BASE="$1"
WORK="$2"
mkdir -p "$WORK"
PASS=0
FAIL=0
ok() { PASS=$((PASS+1)); echo "PASS $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

for p in / /signup /login /lookup; do
  C=$(curl -s -o "$WORK/page$(echo "$p" | tr '/' '_')" -w '%{http_code}' -H 'Accept: text/html' "$BASE$p")
  CT=$(curl -s -o /dev/null -w '%{content_type}' "$BASE$p")
  [ "$C" = "200" ] && ok "page-$p-200" || { bad "page-$p-200" "got $C"; exit 1; }
  case "$CT" in text/html*) ok "page-$p-html";; *) bad "page-$p-html" "$CT"; exit 1;; esac
done
grep -q '<div id="app">' "$WORK/page_" || { bad "page-app-div" "no app mount"; exit 1; }; ok "page-app-div"

INDEX="$WORK/page_"
JS=$(grep -oE 'src="/[^"]+"' "$INDEX" | head -1 | cut -d'"' -f2)
CSS=$(grep -oE 'href="/[^"]+\.css"' "$INDEX" | head -1 | cut -d'"' -f2)
[ -n "${JS:-}" ] && ok "asset-js-ref" || { bad "asset-js-ref" "no bundled script"; exit 1; }
[ -n "${CSS:-}" ] && ok "asset-css-ref" || { bad "asset-css-ref" "no bundled stylesheet"; exit 1; }
JCT=$(curl -s -o /dev/null -w '%{content_type}' "$BASE$JS")
CCT=$(curl -s -o /dev/null -w '%{content_type}' "$BASE$CSS")
case "$JCT" in *javascript*) ok "asset-js-type";; *) bad "asset-js-type" "$JCT"; exit 1;; esac
case "$CCT" in *css*) ok "asset-css-type";; *) bad "asset-css-type" "$CCT"; exit 1;; esac

C=$(curl -s -o "$WORK/apimiss" -w '%{http_code}' "$BASE/nope")
[ "$C" = "404" ] && ok "api-miss-404" || { bad "api-miss-404" "got $C"; exit 1; }
grep -q '"code":"not_found"' "$WORK/apimiss" && ok "api-miss-json" || { bad "api-miss-json" "$(cat "$WORK/apimiss")"; exit 1; }

# Off-origin loads: absolute http(s) URLs that are real fetches (not
# data: URIs, XML namespaces, or svelte.dev diagnostic strings).
OFF=$(grep -rhoE '(src|href)="https?://[^"]+"' "$WORK"/page* 2>/dev/null || true)
if [ -n "$OFF" ]; then bad "no-off-origin" "$OFF"; exit 1; fi
ok "no-off-origin"

echo
echo "RESULT pass=$PASS fail=$FAIL"
[ "$FAIL" = "0" ]
