#!/usr/bin/env bash
# Checks that a live update reaches the other player at once, i.e. that no
# proxy between the browser and the server buffers the SSE stream.
# Usage: tools/sse-check.sh <base URL>
# Exits 0 with "ok: …" when Black gets White's move within 1 s; otherwise
# prints "FAIL: …" and exits 1.
set -euo pipefail
URL=${1:?usage: tools/sse-check.sh <base URL>}
now() { python3 -c 'import time; print(f"{time.time():.3f}")'; }
fail() { echo "FAIL: $*"; exit 1; }
W=$(mktemp); B=$(mktemp); OUT=$(mktemp); EVENTS=$(mktemp)
# Whatever happens, close both streams and remove the temp files.
trap 'kill $(jobs -p) 2>/dev/null || true; { wait; } 2>/dev/null; rm -f "$W" "$B" "$OUT" "$EVENTS"' EXIT

CODE=$(curl -s -c "$W" -b "$W" -XPOST "$URL/api/games" | python3 -c 'import json,sys; print(json.load(sys.stdin)["code"])' 2>/dev/null) ||
	fail "could not create a game at $URL"
curl -sN -c "$W" -b "$W" "$URL/api/games/$CODE/stream" > "$OUT" &      # White joins
sleep 1
curl -sN -c "$B" -b "$B" "$URL/api/games/$CODE/stream" |                 # Black joins; log when each event arrives
	while IFS= read -r line; do [[ $line == event:* ]] && now >> "$EVENTS"; done &
sleep 2

SEQ=$(grep '^data:' "$OUT" | tail -1 | sed 's/^data: //' | python3 -c 'import json,sys; print(json.load(sys.stdin)["seq"])' 2>/dev/null) ||
	fail "White's stream sent no state within 3 s (buffered or down?)"
SENT=$(now)
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b "$W" -XPOST "$URL/api/games/$CODE/move" \
	-H 'content-type: application/json' -d "$(printf '{"from":"e2","to":"e4","seq":%s}' "$SEQ")")
[[ $STATUS == 204 ]] || fail "the move got HTTP $STATUS, not 204"
sleep 2

# Black's first event after the move is the move itself (its join event came earlier).
GOT=$(awk -v sent="$SENT" '$1 > sent { print; exit }' "$EVENTS")
[[ -n $GOT ]] || fail "Black got no update within 2 s of the move (the stream is buffered)"
GAP=$(python3 -c "print(f'{$GOT - $SENT:.2f}')")
python3 -c "import sys; sys.exit(0 if $GOT - $SENT <= 1 else 1)" || fail "Black got the move ${GAP} s after it was sent (over 1 s)"
echo "ok: Black got White's move ${GAP} s after it was sent"
