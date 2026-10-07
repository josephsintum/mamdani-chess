# Mamdani Chess

A browser chess variant to play with friends: the original Pot-Hole Chess (Spicer and Chamberlain, 2001) plus one neutral piece, the Mamdani.

## Sources of truth

| What | Where |
| --- | --- |
| Rules | Live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB. Its "Decisions to lock down" table records every rule choice and why. `RULES.md` is a copy; if they disagree, the doc wins and `RULES.md` should be re-synced. |
| Design spec | `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md`: stack, API, SSE events, clocks, rules engine, testing, visual tokens. |
| Visual design | Canvas: https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih. Every artboard is the reference ("road works" look). |
| Live app (playtest) | https://mamdanichess.com (Railway project `mamdani-chess`, service `web`, SQLite on volume `/data`; it also answers on www.mamdanichess.com and the original mamdani-chess.up.railway.app). Deploys from `main` after CI passes. |

## Decisions that are settled (don't reopen without asking)

- Rules are the original Pot-Hole Chess plus the Mamdani, with longer potholes (milestone 06c). Roll a d8 after every move; even opens a new pothole; it closes after 3 of the roller's moves; at most 5 are open (the oldest closes). A roll can checkmate, but kings never fall (re-roll). All dice are d8.
- Stack: Go server (SSE out, JSON POST in, SQLite), SvelteKit static frontend with a custom board (no chessground). One Railway service.
- Guests only, all games public, one clock (10+5), quick match only, emoji reactions instead of chat.
- Visual design: "road works", dark only. Potholes are a dark hole with an orange ring; small traffic cones on its front edge count the rounds left. The Mamdani's repair is celebrated (cone, sparks, 👍).

## Working rules

- Do not add Claude attribution to commits or PRs: no `Co-Authored-By: Claude` trailer, no "Generated with Claude Code" line.
- Stage only the files you changed (`git add <paths>`), never `git add -A`. `go.mod` and `names/` (guest-name generator) were written separately; don't move or rewrite them without asking.
- The Mamdani photo on the canvas is a placeholder; the shipped game needs art we have rights to.

## Where things stand (2026-10-07)

- **Done:** milestones 01–05 (skeleton, rules engine, playable friend game, game screen with dice tray, resign and result card; 10+5 clocks, games saved to SQLite and restored after a restart, rematch), milestone 06a (guest names, re-roll only, from `names/` ([how the words were chosen](docs/guest-names.md)); the home page with join by code and the live games list; quick match on `/play`), the `/dev/board` sandbox, and [Plan 04b](docs/superpowers/plans/2026-10-05-04b-omcb-feel.md): the board feels like One Million Chessboards (gliding pieces, instant moves, cross-fade captures, pothole effects, tumbling dice, cburnett pieces, reduced motion). A pre-05 pass added instant mode, the playtest script, and keeps an instant move through same-turn updates (`settlesGuess`), which milestone 05's clocks rely on.
- **Done: milestone 06c** ([plan](docs/superpowers/plans/2026-10-05-06c-longer-potholes.md)): potholes last 3 rounds with at most 5 open (the oldest closes), a roll can checkmate, traffic cones count each hole down, the Mamdani's repairs are celebrated (cone, sparks, 👍), and the Mamdani art is on the boards. Games saved under the old rules were retired by migration 5. Checked with whole games: Chromium 8/8, WebKit 8/8, phone 4/4 in instant mode, and 3/3 in each browser with full animations; the playtest now also fails if a closed pothole stays drawn.
- **Also in the repo, from another session:** two side plans (Go server improvements, Go rules speed).
- **Checked:** whole games through the UI with the playtest script, 24 in Chromium and 18 in WebKit (Safari's engine), all clean; WebKit at phone size also glides and takes touch taps.
- **Done: board feel, plans 1–3** ([spec](docs/superpowers/specs/2026-10-06-board-feel-design.md), [mockups](https://claude.ai/artifact/E6oCYQhp8xaDTZCVssczXy), PRs #3–#5 and #7):
  - Coordinates outside the board.
  - mpchess pieces at 90% of the square, with lines 1.2× heavier (the SVGs are edited) and a 2 px cream outline.
  - Ease-out glide with a trail and a lean; pick-up lean, drag tilt and ripple.
  - Board shake, capture knock-back, falls and saves.
  - Speech bubbles: every line, the pairs and the rare lines live in `web/src/lib/catchphrases.ts`, chosen from the [catchphrase shortlist](https://claude.ai/artifact/JVUbAGvZ5NKAyt5bvdTNGp).
  - Checkmate burst, the win screen's tally and confetti, and "+5" on the clock.
  - At game over the board darkens under a shade below the bubbles.
  - The dice (plan 3, PR #7):
    - Dice are thrown into the tray and into the phone's dice card, which stays below the board. Their colours say what each die decides:
      - the pothole roll's die is grey when odd and yellow when even, shown once it lands;
      - the file and rank dice are orange;
      - a saving die is cream.
    - A pill on the board's corner says what the roll means.
    - The file and rank dice tumble out of sync while an orange scan on the board follows their faces and lights the outside coordinates, then lands on the target. The faces are seeded, so the tray, the phone card and the board agree.
    - A re-roll makes the target blink twice.
- **Known, left for later:** a turn shown at once can still replay some effects:
  - a reload or reconnect re-throws the last turn's dice (and plays a save's hop and a bubble again);
  - coming back to a hidden tab mid-roll can replay the scan;
  - opening the phone's moves sheet re-throws the dice.
- **Done: the home page reel and the remembered name** (live since 2026-10-06):
  - Home page: "How it works" is the second section, beside the [Mamdani Patch reel](https://www.instagram.com/reel/Dd8tV_MxEQL/) with a credit to its creator, @bardelo_bardalini (canvas rows "Home" and "Home (phone)"). Step 01 covers three-round potholes.
  - Instagram won't play this reel inside the embed (no video loads; a click opens Instagram), even with its own script. Playing it on the page means hosting the clip, which needs the creator's OK.
  - The embed is a bare iframe without Instagram's script, so its height is measured (a 4:5 video plus 210 px). Recheck it if Instagram changes its layout.
  - The header shows a returning guest's name at once: `lobby.ts` keeps the last name `/api/me` gave in localStorage (`rememberedName`), and the server's answer replaces it. The `guest` cookie is HttpOnly and holds only a random ID; the name lives in SQLite.
- **Done: quality passes** over every Go package and every ts/svelte file (PR #16 and the go-cleanup merge): shared helpers instead of copies, no behaviour change.
- **Done: learning pages and help** (PR #17, live since 2026-10-07):
  - `/how-to-play`: "Get a game", then seven scenes on the real board (the dice, potholes blocking lines, rounds and the cap, the Mamdani blocking and repairing, saving rolls, mate by roll). A scene is data in `web/src/lib/scenes.ts`, played through `sandbox.ts`'s turns by `ScriptedBoard.svelte` (which `/dev/board` uses too); it plays while on screen and loops. `scenes.test.ts` replays each one. The sandbox ignores check, so a scene never shows a piece's moves while its king is in check.
  - `/rules`: the canvas's Rules layout, worded from the live rules doc, with still boards (`MiniBoard` dots, target, cones) and a collapsible "Every rule, in full".
  - `/practice`: `POST /api/practice` gives one guest both seats; they move for whichever side is to move, through the real engine and dice. No clock, not saved, never listed, not "in a game" for quick match or Rejoin, gone after 10 minutes unwatched, and a new one ends the last. The View says `practice: true`.
  - In a game: "What's on the board" (`GameHelp.svelte`), a legend of the dice, potholes, marks and the Mamdani; first-game tips (`tips.ts`): one toast the first time each thing happens, once per browser (localStorage), at most one per turn, switchable in the legend.
  - Home: a "Questions" FAQ, and "Live now" only while games are being played. `/about`: unofficial and ad-free (not affiliated with Zohran Mamdani), credits, what the site keeps. Header and `SiteFooter` link How to play · Rules · Practice · About (on phones the header's links take a second row).
  - Checked: two code reviews and a phone check (Chromium and WebKit, portrait 320×568 to 412×915, landscape 568×320 to 915×412), all findings fixed.
- **Left from the rename:** the canvas's Rules and Header artboards (still "Pothole Chess", a d6, one-round potholes) and the live rules doc's title.
- **Pending:** a playtest with friends, on their phones.

## Next step

Milestone 07 (launch). Its brainstorm has started ([notes](docs/superpowers/specs/2026-10-06-07-launch-brainstorm.md)): the app is renamed Mamdani Chess, audience a few friends. Done so far: the rename, the rules page (and How to play, Practice, About), the Mamdani art. Left: per-IP limits on streams and requests (`POST /api/practice` is a second, cheaper way to start games), the HTTP/2 check, launch logging, and the canvas and rules doc updates above. Player history waits for milestone 08 ([brainstorm](docs/superpowers/specs/2026-10-06-08-leaderboards-brainstorm.md)). Follow `docs/superpowers/plans/2026-10-03-00-roadmap.md`. Repo: https://github.com/josephsintum/mamdani-chess (public).

## Working notes

- **Run it:** `go run ./cmd/server` (:8080) and `pnpm --dir web dev` (:5173, forwards `/api`). `/dev/board` is a dev-only sandbox: one browser plays both sides, dice are scripted, and nothing goes to the server. For the real rules alone, use `/practice` (server-backed).
- **Stopping a test server:** stop it by its PID or port (`lsof -iTCP:<port>`), never with a broad `pkill -f server`: that also kills other sessions' servers.
- **Two players in one browser:** `localhost` and `[::1]` (or `127.0.0.1` against the Go server) keep separate cookies, so each origin is a different guest.
- **Tests:** `go test -race -short ./...` (the rules engine's full random-game suite takes about 2 min under `-race`), `pnpm --dir web check`, `pnpm --dir web test` (Vitest), `pnpm --dir web build`.
- **Contract checks:** `contract/` checks what clients see through the public API (status codes, error strings, JSON key order, cookies, SSE framing, static files, link previews) against the real server binary on a free port with a new database. Run `pnpm --dir web build` first, then `CONTRACT=1 go test ./contract -count=1` (about 16 s; `-count=1` because a cached pass doesn't see server changes); `CONTRACT_SLOW=1` adds the minute-long first-move abort, and `BASE_URL=<url>` checks a running server instead. Without `CONTRACT=1` the package skips, so `go test ./...` doesn't need the web build.
- **Wire types:** `web/src/lib/wire.gen.ts` (the JSON types the browser gets, and the pothole limits) is generated from the Go types by `go run ./cmd/wiregen`; `go test ./cmd/wiregen` fails while it's stale. A field that is narrower in TypeScript than in Go (a color is a Go string) carries a tag, e.g. `ts:"Color"`. Never edit the file by hand.
- **Playtest:** with both servers running, `pnpm --dir web playtest` plays whole games through the real UI (two guests per game, random legal moves, some by mouse drag) and exits 1 on any page error, stuck board or lost move. Flags: `--browser webkit`, `--games 12`, `--drag 0.5`, `--headed`, and `--match` (the two guests find each other through quick match instead of a link). Run it after any change to the board, game page or server protocol.
- **Phones:** under 640 px wide the game page renders its phone layout (canvas row "Phone game: playtest build"). Check phone changes with `pnpm --dir web playtest --phone`, which fails if the page ever scrolls.
- **Deploys:** every push to `main` deploys to Railway once CI is green, and a deploy pauses every game in progress: games are saved, open tabs reconnect on their own, and the side to move's clock restarts 10 s after the server is back, from what it had at the start of that turn. Don't push to `main` during a playtest. Share the URL with friends only until milestone 07's per-IP limits. The production image logs JSON (`LOG_FORMAT=json`); `go run` logs text.
- **Checking a deploy:** `curl <url>/healthz` shows the deployed commit (`version`), so a deploy is live without creating a game; then `tools/sse-check.sh <url>` (live updates arrive at once) and `pnpm --dir web playtest --base <url> --turn-ms 15000`. Both of those create games, which are saved on production.
- **Production logs:** `railway logs -s web -n 5000 --json` returns the current deployment's logs. `--since` alone stops at 500 lines without saying so, and `--since` with `--until` returned nothing (railway 5.62). Game events (created, ended, aborted, rematch) log at INFO, and a restart logs one `games restored` summary (with its duration); each restored game logs at DEBUG. Successful requests log at DEBUG, refused or failed ones at INFO, and requests over 1 s (streams excepted) at WARN.
- **Agents in a browser:** agent-browser 0.8.4 doesn't find its own Chromium here; point it at Playwright's with `--executable-path` (or `AGENT_BROWSER_EXECUTABLE_PATH`), e.g. `~/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell`. Give each player its own `--session`, since a guest is a cookie.
- **Instant mode:** dev builds only. `/game/CODE?instant`, or the sandbox's Instant checkbox, turns off every animation and plays the dice in 0 ms (`setInstant` in `motion.ts`), so a whole game takes seconds.
- **Dice timing:** `web/src/lib/dice-timing.json` is the one table of how long each dice step plays. The server's clock pause mirrors it (`pauseFor` and `playTime` in `game/clock.go`), and `TestDiceTimingMatchesTheBrowser` fails if they drift apart, so change both together.
- **SvelteKit 3 and Svelte 5 traps hit so far:**
  - use `$app/env`, not `$app/environment`;
  - `#lib/...` imports need the file extension (`#lib/game.ts`), and there is no `$lib`;
  - transition functions take `(node, params)`;
  - `$state.snapshot` only works in `.svelte`/`.svelte.ts` files;
  - run `npx @sveltejs/mcp svelte-autofixer` on every component and avoid `$effect`.
- **Dev server and `pnpm check`:** `pnpm --dir web check` regenerates SvelteKit's files and can leave a running `pnpm --dir web dev` hung (pages stop loading). Restart the dev server after it. Opening a page while the dev server is still starting can make it log `failed to load virtual css module` and serve that component's raw `.svelte` file as its CSS, so the page loses rules (e.g. the home page runs edge to edge); restart it and wait for "ready".
- **PR screenshots:** a PR that changes the UI embeds screenshots, before and after where it fits. They live on the unmerged `pr-screenshots` branch under `pr-<number>/` and are linked from `raw.githubusercontent.com`, so `main` stays free of images.
- **Notices:** show toasts with `notify.*` from `#lib/toast.ts` (svelte-sonner behind it, themed in `Toaster.svelte`, mounted once in the layout). Never import svelte-sonner in a page.
- **Board pointer rule:** capture the pointer only once a drag has moved more than 6px from the press (not from the last event, or slow drags never start). Capturing on pointerdown sends the click to the board, and taps stop working.
- **How plans have been written:** build the code in a scratch copy, check it in tests and a browser, write the plan from those files, dry-run the plan task by task from `main`, then execute it inline on a branch with one final whole-branch review.
