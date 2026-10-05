# Pothole Chess: Mamdani Edition

A browser chess variant to play with friends: the original Pot-Hole Chess (Spicer and Chamberlain, 2001) plus one neutral piece, the Mamdani.

## Sources of truth

| What | Where |
| --- | --- |
| Rules | Live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB. Its "Decisions to lock down" table records every rule choice and why. `RULES.md` is a copy; if they disagree, the doc wins and `RULES.md` should be re-synced. |
| Design spec | `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md`: stack, API, SSE events, clocks, rules engine, testing, visual tokens. |
| Visual design | Canvas: https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih. Every artboard is the reference ("road works" look). |
| Live app (playtest) | https://mamdani-chess.up.railway.app (Railway project `mamdani-chess`, service `web`, SQLite on volume `/data`). Deploys from `main` after CI passes. |

## Decisions that are settled (don't reopen without asking)

- Rules are the original Pot-Hole Chess plus only the Mamdani. Roll a d8 after every move; even opens a new pothole; it closes after the roller's next move. Kings never fall (re-roll). All dice are d8.
- Stack: Go server (SSE out, JSON POST in, SQLite), SvelteKit static frontend with a custom board (no chessground). One Railway service.
- Guests only, all games public, one clock (10+5), quick match only, emoji reactions instead of chat.
- Visual design: "road works", dark only. Potholes are a dark hole with an orange ring.

## Working rules

- Do not add Claude attribution to commits or PRs: no `Co-Authored-By: Claude` trailer, no "Generated with Claude Code" line.
- Stage only the files you changed (`git add <paths>`), never `git add -A`. `go.mod` and `names/` (guest-name generator) were written separately; don't move or rewrite them without asking.
- The Mamdani photo on the canvas is a placeholder; the shipped game needs art we have rights to.

## Where things stand (2026-10-05)

- **Done:** milestones 01–05 (skeleton, rules engine, playable friend game, game screen with dice tray, resign and result card; 10+5 clocks, games saved to SQLite and restored after a restart, rematch), milestone 06a (guest names, re-roll only, from `names/` ([how the words were chosen](docs/guest-names.md)); the home page with join by code and the live games list; quick match on `/play`), the `/dev/board` sandbox, and [Plan 04b](docs/superpowers/plans/2026-10-05-04b-omcb-feel.md): the board feels like One Million Chessboards (gliding pieces, instant moves, cross-fade captures, pothole effects, tumbling dice, cburnett pieces, reduced motion). A pre-05 pass added instant mode, the playtest script, and keeps an instant move through same-turn updates (`settlesGuess`), which milestone 05's clocks rely on.
- **Also in the repo, from another session:** the Rust server experiment in `server_rs/`, and two side plans (Go server improvements, Go rules speed).
- **Checked:** whole games through the UI with the playtest script, 24 in Chromium and 18 in WebKit (Safari's engine), all clean; WebKit at phone size also glides and takes touch taps.
- **Pending:** a playtest with friends, on their phones.

## Next step

Milestone 07 (launch). Brainstorm it first; player history waits for milestone 08. 06b (watcher count and emoji reactions) moved to milestone 10, after 09 and before the security and UX reviews (11, 12). Follow `docs/superpowers/plans/2026-10-03-00-roadmap.md`. Repo: https://github.com/josephsintum/mamdani-chess (public).

## Working notes

- **Run it:** `go run ./cmd/server` (:8080) and `pnpm --dir web dev` (:5173, forwards `/api`). `/dev/board` is a dev-only sandbox: one browser plays both sides, dice are scripted, and nothing goes to the server.
- **Two players in one browser:** `localhost` and `[::1]` (or `127.0.0.1` against the Go server) keep separate cookies, so each origin is a different guest.
- **Tests:** `go test -race -short ./...` (the rules engine's full random-game suite takes about 2 min under `-race`), `pnpm --dir web check`, `pnpm --dir web test` (Vitest), `pnpm --dir web build`.
- **Playtest:** with both servers running, `pnpm --dir web playtest` plays whole games through the real UI (two guests per game, random legal moves, some by mouse drag) and exits 1 on any page error, stuck board or lost move. Flags: `--browser webkit`, `--games 12`, `--drag 0.5`, `--headed`, and `--match` (the two guests find each other through quick match instead of a link). Run it after any change to the board, game page or server protocol.
- **Phones:** under 640 px wide the game page renders its phone layout (canvas row "Phone game: playtest build"). Check phone changes with `pnpm --dir web playtest --phone`, which fails if the page ever scrolls.
- **Deploys:** every push to `main` deploys to Railway once CI is green, and a deploy pauses every game in progress: games are saved, open tabs reconnect on their own, and the side to move's clock restarts 10 s after the server is back, from what it had at the start of that turn. Don't push to `main` during a playtest. Share the URL with friends only until milestone 07's per-IP limits. The production image logs JSON (`LOG_FORMAT=json`); `go run` logs text.
- **Checking a deploy:** `curl <url>/healthz` shows the deployed commit (`version`), so a deploy is live without creating a game; then `tools/sse-check.sh <url>` (live updates arrive at once) and `pnpm --dir web playtest --base <url> --turn-ms 15000`. Both of those create games, which are saved on production.
- **Production logs:** `railway logs -s web -n 5000 --json` returns the current deployment's logs. `--since` alone stops at 500 lines without saying so, and `--since` with `--until` returned nothing (railway 5.62). Game events (created, ended, aborted, rematch, restored) log at INFO; move requests at DEBUG.
- **Agents in a browser:** agent-browser 0.8.4 doesn't find its own Chromium here; point it at Playwright's with `--executable-path` (or `AGENT_BROWSER_EXECUTABLE_PATH`), e.g. `~/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell`. Give each player its own `--session`, since a guest is a cookie.
- **Instant mode:** dev builds only. `/game/CODE?instant`, or the sandbox's Instant checkbox, turns off every animation and plays the dice in 0 ms (`setInstant` in `motion.ts`), so a whole game takes seconds.
- **SvelteKit 3 and Svelte 5 traps hit so far:**
  - use `$app/env`, not `$app/environment`;
  - `#lib/...` imports need the file extension (`#lib/game.ts`), and there is no `$lib`;
  - transition functions take `(node, params)`;
  - `$state.snapshot` only works in `.svelte`/`.svelte.ts` files;
  - run `npx @sveltejs/mcp svelte-autofixer` on every component and avoid `$effect`.
- **Notices:** show toasts with `notify.*` from `#lib/toast.ts` (svelte-sonner behind it, themed in `Toaster.svelte`, mounted once in the layout). Never import svelte-sonner in a page.
- **Board pointer rule:** capture the pointer only once a drag has moved more than 6px from the press (not from the last event, or slow drags never start). Capturing on pointerdown sends the click to the board, and taps stop working.
- **How plans have been written:** build the code in a scratch copy, check it in tests and a browser, write the plan from those files, dry-run the plan task by task from `main`, then execute it inline on a branch with one final whole-branch review.
