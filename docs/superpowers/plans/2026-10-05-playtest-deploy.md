# Playtest Deploy on Railway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put the game on a Railway URL that friends can open for a playtest, deployed automatically from GitHub `main` once CI passes.

**Architecture:** One Railway service built from the existing `Dockerfile`: the Go server with the SvelteKit build embedded, serving pages, API and live streams itself (no nginx or Caddy; `server/static.go` already does SPA fallback, precompressed brotli/gzip and cache headers). Railway terminates TLS, so browsers get HTTP/2. SQLite sits on a volume at `/data` for milestone 05's saved games. Railway deploys every push to `main`, but only after GitHub Actions pass ("Wait for CI"), so CI has to be green first.

**Tech Stack:** Railway (CLI 5.62, GitHub autodeploy, volumes), Docker, Go, GitHub Actions, Playwright (the existing `web/scripts/playtest.js`).

**Spec:** `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md` §Stack ("Hosting: One Railway service"). Also `docs/superpowers/plans/2026-10-03-01-walking-skeleton.md` Task 7 (the original deploy steps, written for the honk demo; this plan replaces its checks with game checks) and the roadmap's "Launch check: HTTP/2".

## Decisions taken with the user (2026-10-05)

- **A playtest deploy, not the milestone 07 launch.** No per-IP limits, rules page, phone layouts or Mamdani art in this plan. Share the URL with friends only until milestone 07's limits land.
- **Auto-deploy from GitHub `main`**, gated on CI ("Wait for CI").
- **A Railway subdomain** (`https://<name>.up.railway.app`). A custom domain can be added later without redoing anything.

## Global Constraints

- **One Railway service** (CLAUDE.md, spec §Stack). The Go server serves the static files; add no web server in front of it.
- **Share the URL with friends only.** Anyone with it can open unlimited live streams (about 49 KB each) until milestone 07 adds per-IP limits.
- **Steps marked (you)** create paid Railway resources or need the GitHub/Railway dashboards. The executor proposes the exact command; the project owner runs it or approves it.
- **Git:** stage explicit paths and never use `git add -A`. No Claude attribution in commits.
- **CI checks only what ships:** the `go`, `web` and `docker` jobs. The Rust experiment in `server_rs/` stays in the repo but is not checked, so it can't block deploys (decided with the user, 2026-10-05).

## Review Focus

1. **The live stream is buffered by Railway's edge.** The board would never update for the other player. Pinned by `tools/sse-check.sh` against the public URL (Task 4): the other player's event must arrive within about a second of the move.
2. **A push to `main` during a playtest.** Every deploy restarts the server, and games live in memory, so every game in progress ends. Wait for CI doesn't prevent this. Documented as a working rule in CLAUDE.md (Task 5); there is no code fix until milestone 05 saves games.
3. **A push with failing CI.** Wait for CI must skip that deploy and keep the running one. Pinned by Task 1 (CI green) and the Wait for CI toggle checked in Task 3.
4. **HTTP/1.1 reaching the browser.** With 6+ game tabs, requests queue forever (roadmap "Launch check: HTTP/2"). Pinned by the `curl --http2` check in Task 4.
5. **The volume isn't writable or the database path is wrong.** The server would fail to start, so the healthcheck fails and the deploy never goes live. Pinned by the deploy reaching `SUCCESS` and the `listening … db=/data/mamdani.db` log line (Task 3).

## File Structure

| File | Responsibility |
| --- | --- |
| `.github/workflows/ci.yml` | Drop the `rust` job |
| `web/scripts/playtest.js` | `--turn-ms`, so it can play against a production build (full dice animation) |
| `tools/sse-check.sh` | Checks that a live update reaches the other player at once |
| `CLAUDE.md`, `docs/superpowers/plans/2026-10-03-00-roadmap.md` | Live URL, the deploy rules, the side-plan row |

---

### Task 1: Green CI

CI has failed on `main` since its Rust job moved to toolchain 1.99, whose new Clippy lints fail on `server_rs/` (the Rust server experiment). The Go, web and Docker jobs pass. The user chose to drop the Rust job: nothing in `server_rs/` ships, and with Wait for CI a red Rust job would block every deploy.

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: a green CI run on `main`, which Task 3's Wait for CI needs.

- [ ] **Step 1: Drop the job**

In `.github/workflows/ci.yml`, delete the `rust:` job and the comment above it (`# The Rust server experiment in server_rs/. …`), everything from that comment down to the line before `  docker:`.

- [ ] **Step 2: Check the workflow still parses**

Run: `ruby -ryaml -e 'puts YAML.load_file(".github/workflows/ci.yml")["jobs"].keys.inspect'`
Expected: `["go", "web", "docker"]`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "CI: drop the Rust job; server_rs is an experiment and doesn't ship"
```

CI itself runs on the push in Task 3, Step 1.

---

### Task 2: Checks that work against production

The playtest script waits 3 s for each move to land, which suits `?instant` dev builds. A production build ignores `?instant` and plays the dice in full. A plain turn takes about 2 s, so it still fits (checked against a local production build: 20 plies, clean). But a turn with re-rolls and a saving roll plays more dice steps, and Railway adds network latency, so the wait must be adjustable or the script reports false failures. The live-update check needs no browser, so it is a small shell script.

**Files:**
- Modify: `web/scripts/playtest.js`
- Create: `tools/sse-check.sh`

**Interfaces:**
- Consumes: the API (`POST /api/games` → `{"code"}`, `GET /api/games/{code}/stream` joins and streams `event: state` with `data.seq`, `POST /api/games/{code}/move` with `{"from","to","seq"}` → 204).
- Produces: `pnpm --dir web playtest --base <url> --turn-ms <ms>` and `tools/sse-check.sh <url>`, used in Task 4.

- [ ] **Step 1: Start a production build locally**

```bash
pnpm --dir web build
PORT=8090 DB_PATH=/tmp/prodcheck.db go run -tags embedweb ./cmd/server &
curl -s http://localhost:8090/healthz; echo
```

Expected: `{"status":"ok"}`. This is what the `Dockerfile` builds: the site embedded in the Go binary, no `?instant`. Leave it running for Step 4.

- [ ] **Step 2: Add `--turn-ms`**

In `web/scripts/playtest.js`:

After the usage line `//   pnpm --dir web playtest --games 12 --drag 0.5 # half the moves by dragging`, add:

```js
//
// Against a production build (no ?instant: the dice play out in full), allow
// each turn longer:
//
//   pnpm --dir web playtest --base https://<domain> --games 2 --max-plies 30 --turn-ms 15000
```

In the `parseArgs` options, after `'max-plies'`:

```js
		'turn-ms': { type: 'string', default: '3000' }, // how long a move may take to land
```

After `const MAX_PLIES = Number(opts['max-plies']);`:

```js
const TURN_MS = Number(opts['turn-ms']);
```

And change the landing loop's header from `for (let i = 0; i < 100 && !landed; i++) {` to:

```js
		for (let i = 0; i < TURN_MS / 30 && !landed; i++) {
```

- [ ] **Step 3: Create `tools/sse-check.sh`**

```bash
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
```

Run: `chmod +x tools/sse-check.sh`

- [ ] **Step 4: Run both against the production build**

```bash
pnpm --dir web playtest --base http://localhost:8090 --games 2 --max-plies 20 --turn-ms 15000
tools/sse-check.sh http://localhost:8090
kill %1   # the :8090 server
```

Expected: `chromium: 2/2 games finished cleanly` (each resigns at the ply limit, about 30 s); and `black got event: state` within 0.1 s of `white sends`, after `-> HTTP 204`.

- [ ] **Step 5: Commit**

```bash
git add web/scripts/playtest.js tools/sse-check.sh
git commit -m "Checks for production: playtest --turn-ms, and tools/sse-check.sh for live updates"
```

---

### Task 3: Push, then create the Railway service

**Files:** none.

**Interfaces:**
- Consumes: Task 1's green CI; `Dockerfile` and `railway.json` (healthcheck `/healthz`).
- Produces: `URL`, the service's `https://….up.railway.app` domain, used in Tasks 4 and 5.

- [ ] **Step 1 (you): Push `main`**

This also publishes `main`'s other unpushed work (the `go-encode-once` merge and the 04b follow-ups).

```bash
git push origin main
gh run watch $(gh run list --branch main --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
```

Expected: every job ✓, exit 0.

- [ ] **Step 2 (you): Create the project and the service from GitHub**

```bash
railway init --name mamdani-chess
railway add --service web --repo josephsintum/mamdani-chess --branch main --json
railway link --project mamdani-chess --service web
```

Expected: `railway status --json` shows project `mamdani-chess`, service `web`. If `railway add` reports no access to the repo, install or configure the Railway GitHub app for `josephsintum/mamdani-chess` (Railway dashboard → Account → GitHub), then rerun it.

- [ ] **Step 3 (you): Volume, domain and Wait for CI**

```bash
railway volume add --mount-path /data
railway domain --service web --json
```

Expected: `railway volume list` shows a volume at `/data`; the domain command prints `https://<name>.up.railway.app`. Set `URL` to it. The `Dockerfile` already sets `DB_PATH=/data/mamdani.db` and `PORT=8080`, so no variables are needed.

Then in the Railway dashboard: service `web` → Settings → Source → turn on **Wait for CI** (accept the GitHub permission prompt if shown).

- [ ] **Step 4: Wait for the deploy**

```bash
railway deployment list --service web --limit 1 --json
railway logs --service web --lines 50
```

Expected: the newest deployment reaches `SUCCESS` (poll every 30 s; a first build takes a few minutes), and the logs include `listening` with `addr=:8080 db=/data/mamdani.db`. If it is `FAILED` or `CRASHED`, read `railway logs --service web --build --lines 200` before changing anything.

---

### Task 4: Deployed checks

**Files:** none.

**Interfaces:**
- Consumes: `URL` (Task 3), `tools/sse-check.sh` and `playtest --turn-ms` (Task 2).
- Produces: evidence that the deploy is playable.

- [ ] **Step 1: Health, routes and HTTP/2**

```bash
curl -s $URL/healthz; echo
curl -s -o /dev/null -w '%{http_code}\n' $URL/game/TEST01
curl -sI --http2 $URL/ | head -1
```

Expected: `{"status":"ok"}`; `200` (the SPA fallback); `HTTP/2 200`.

- [ ] **Step 2: Live updates aren't buffered**

Run: `tools/sse-check.sh $URL`
Expected: `-> HTTP 204`, then `black got event: state` within about 1 s of `white sends`. If it only arrives at the 2 s cut-off, Railway's edge is buffering: stop and report (Review Focus 1).

- [ ] **Step 3: Whole games through the UI, both engines**

```bash
pnpm --dir web playtest --base $URL --games 2 --max-plies 30 --turn-ms 15000
pnpm --dir web playtest --base $URL --games 2 --max-plies 30 --turn-ms 15000 --browser webkit
```

Expected: `chromium: 2/2 games finished cleanly` and `webkit: 2/2 games finished cleanly`.

- [ ] **Step 4 (you): By hand, on a phone**

Open `$URL` on a laptop, press "Play a friend", and open the game link on a phone (a different guest). Play a few moves each way, including one drag on the phone.
Expected: moves show on the other screen within a second; pieces glide; the dice tray plays out.

- [ ] **Step 5: A redeploy comes back**

```bash
railway redeploy --service web --yes
railway deployment list --service web --limit 1 --json
curl -s $URL/healthz; echo
railway volume list
```

Expected: the new deployment reaches `SUCCESS`; `{"status":"ok"}`; the volume is still mounted at `/data`. (Games in progress end on a redeploy: they live in memory until milestone 05.)

---

### Task 5: Record it

**Files:**
- Modify: `CLAUDE.md`, `docs/superpowers/plans/2026-10-03-00-roadmap.md`

- [ ] **Step 1: CLAUDE.md**

In the "Sources of truth" table, add a row:

```markdown
| Live app (playtest) | `$URL` (Railway project `mamdani-chess`, service `web`, SQLite on volume `/data`). Deploys from `main` after CI passes. |
```

(with the real domain in place of `$URL`), and add to "Working notes":

```markdown
- **Deploys:** every push to `main` deploys to Railway once CI is green, and a deploy ends every game in progress (games live in memory until milestone 05). Don't push to `main` during a playtest. Share the URL with friends only until milestone 07's per-IP limits.
- **Checking a deploy:** `tools/sse-check.sh <url>` (live updates arrive at once) and `pnpm --dir web playtest --base <url> --turn-ms 15000`.
```

- [ ] **Step 2: Roadmap**

In the "Side plans" table, add:

```markdown
| [Playtest deploy on Railway](2026-10-05-playtest-deploy.md) | Green CI (Rust job dropped), Railway service from GitHub with Wait for CI, volume at `/data`, subdomain, production checks (`tools/sse-check.sh`, `playtest --turn-ms`) | Done |
```

and in milestone 07's row, change `then the Railway deploy (Plan 01 Task 7: …)` to `then the public launch on the existing Railway service (playtest deploy done; add the domain and rerun its checks)`.

- [ ] **Step 3: Commit and push**

```bash
git add CLAUDE.md docs/superpowers/plans/2026-10-03-00-roadmap.md
git commit -m "Docs: the playtest deploy on Railway"
git push origin main
```

Expected: CI passes and Railway deploys this commit (docs only, so the app is unchanged).

---

## Done when

- CI is green on `main`, and Railway deploys `main` only after it passes.
- `$URL/healthz` answers, `$URL` serves HTTP/2, `tools/sse-check.sh $URL` shows live updates within a second, and the playtest script finishes games in Chromium and WebKit against `$URL`.
- A friend on a phone and you on a laptop can play a game through the link.
