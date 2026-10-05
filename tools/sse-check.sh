#!/usr/bin/env bash
# Checks that a live update reaches the other player at once, i.e. that no
# proxy between the browser and the server buffers the SSE stream.
# Usage: tools/sse-check.sh <base URL>
# Expected: "black got event: state" within about a second of "white sends".
set -euo pipefail
URL=${1:?usage: tools/sse-check.sh <base URL>}
now() { python3 -c 'import time; print(f"{time.time():.2f}")'; }
W=$(mktemp); B=$(mktemp); OUT=$(mktemp)
CODE=$(curl -s -c "$W" -b "$W" -XPOST "$URL/api/games" | python3 -c 'import json,sys; print(json.load(sys.stdin)["code"])')
curl -sN -c "$W" -b "$W" "$URL/api/games/$CODE/stream" > "$OUT" &      # White joins
sleep 1
curl -sN -c "$B" -b "$B" "$URL/api/games/$CODE/stream" |                 # Black joins and watches
  while IFS= read -r line; do [[ $line == event:* ]] && echo "$(now) black got $line"; done &
sleep 2
SEQ=$(grep '^data:' "$OUT" | tail -1 | sed 's/^data: //' | python3 -c 'import json,sys; print(json.load(sys.stdin)["seq"])')
BODY=$(printf '{"from":"e2","to":"e4","seq":%s}' "$SEQ")
echo "$(now) white sends e2-e4 (seq $SEQ)"
curl -s -b "$W" -XPOST "$URL/api/games/$CODE/move" -H 'content-type: application/json' -d "$BODY" -w ' -> HTTP %{http_code}\n'
sleep 2
kill $(jobs -p) 2>/dev/null || true
rm -f "$W" "$B" "$OUT"
