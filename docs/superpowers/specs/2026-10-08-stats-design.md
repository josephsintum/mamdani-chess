# Stats: first-party visit and game metrics

**Date:** 2026-10-08
**Status:** Agreed. Plan 1 (visits and the page): [2026-10-08-stats-1-visits.md](../plans/2026-10-08-stats-1-visits.md), built (PR #22). Plan 2 (game stats, quick-match log): [2026-10-09-stats-2-games.md](../plans/2026-10-09-stats-2-games.md).
**Design:** the canvas https://claude.ai/artifact/Rzwy4PSQuJoKNZ629PYogL (Full site tab): the stats page at desktop and phone width, and the board "Where each number comes from".

## Why

The site is going to friends (milestone 07) and nothing shows who visits or whether they play. We want to know how many people visit, roughly where they are, what device they use, and, above all, whether the game works: do visitors play, where do they drop off, do quick matches find opponents, and are the pothole rules (3 rounds, cap of 5, saving rolls) doing what we want.

## Decisions

- **First party, local.** The Go server counts visits in its own SQLite database and shows them on a private page. No analytics company, no outside script. Ad blockers can't hide visits.
- **Page views come from the browser.** The app is a single-page app, so the server only sees the first load. The page sends one small `POST /api/visit` on every page change.
- **No new cookie.** The `guest` cookie everyone already gets is the visitor ID; the row keeps the first 12 hex characters of `guestID()` (the SHA-256 of the cookie), so a visit joins to the games that guest played, and nothing more.
- **Location from a local file.** DB-IP's free City Lite database (CC BY 4.0, no account) is downloaded into the Docker image and read with `github.com/oschwald/maxminddb-golang/v2` (pure Go, ISC licence; agreed exception to the no-new-dependencies rule). The IP address is used for the lookup and never stored.
- **Device from the screen**, not the user agent: under 640 px on the short side is a phone (the app's own phone breakpoint); otherwise a touch screen is a tablet and anything else a desktop. iPads say they are Macs, so the user agent can't tell, and a laptop's short side is often under 1024, so the size alone can't either (decided during execution, 2026-10-08).
- **OS and browser** by plain string matching on the user agent, no library. Instagram's and Facebook's in-app browsers are named, since that's where shared links open. WhatsApp's can't be told apart from Safari or Chrome, so it isn't.
- **Only numbers that lead to a decision.** No trivia (longest game, where pieces fell, White vs Black). Kept: the visit-to-game funnel with the drop at each step, how games end, game length, when people play, quick-match waits and the "searched when nobody else was looking" count, how full the road gets, whether the road decided the game.
- **Playtests aren't counted.** Headless Chrome (`HeadlessChrome` in the user agent) is dropped, and the playtest script sends an `X-Playtest: 1` header on every request so WebKit and phone runs are dropped too, including runs against production.
- **Private page** at `/stats`, HTTP Basic Auth with the password from `STATS_PASSWORD`. Without that variable the path serves the app shell as any unknown path does. Server-rendered with `html/template`, no JavaScript, Graphite style with one blue family for the charts (the canvas).
- **Days are grouped in `STATS_TZ`** (default `America/New_York`).
- **Kept for 400 days**, pruned at startup. Game codes are never stored (`/game/K7F3QZ` is recorded as `/game`).
- **The About page** says what is counted, that the IP isn't kept and nothing goes to anyone else, and credits DB-IP (CC BY requires it).

## What is recorded

**`visits`**: one row per page view: time, visitor (12 hex), page (`/`, `/game`, `/play`, `/practice`, `/how-to-play`, `/rules`, `/about`, else `other`), referrer host (empty for none or our own site; `ref:<tag>` for a `?ref=` link), country, city, device, OS, browser.

**`browser_errors`**: time, visitor, page, message (300 characters at most), browser. From `window.onerror` and unhandled rejections.

## The page (plan 1 builds the parts marked *)

Sidebar: Overview, Games, Quick match, The road, Visitors, Health. Range: 7, 30, 90 days, all.

- **Overview\***: Visitors, Played a game (made a move), Games, Came back (visited on 2+ days), each against the range before; the funnel Visited → Opened a game → Made a move → Finished a game → Played again, with the drop at each step.
- **Games**: games per day by kind, how games end, game length (median, decided on the clock, winner's clock left), when people play (weekday × 2-hour grid), friend links (joined, finished, time to join, rematches).
- **Quick match**: wait histogram, found an opponent, median wait, gave up after, searched when nobody else was.
- **The road**: potholes a game, falls a game, saving rolls saved, ended by a roll; how full the road gets (open holes after each turn; closed by rounds / cap / repair; reset by a roll); did the road decide it (winner lost less / more / the same to potholes).
- **Visitors\***: visitors per day (new and returning), countries, cities, arrived via, devices, systems, browsers, pages.
- **Health\***: browser errors (plan 1); reconnects and server restarts (plan 2).

## Where each number comes from

| Stat | Source | Plan |
| --- | --- | --- |
| How games end, game length, rematches, when people play | `games`, `turns` as saved today | 2 (read only) |
| Games per day by kind; practice count | a `kind` column on `games`; a daily practice counter | 2 |
| Potholes, falls, saving rolls, repairs, resets, mate by a roll, how full the road gets, did the road decide it | each game's dice replayed through `rules` when it ends; one stats row per player per game, shaped so milestone 08's player history reuses it; old games backfilled once | 2 |
| Quick match | a row per search: start, end, matched or gave up, queue size at the start | 2 |
| Friend links: joined, time to join | `joined_at` on `games` | 2 |
| Visitors, came back, funnel, country, city, device, system, browser, arrived via, pages | `visits`, joined to `games` by the 12-hex visitor key | 1 |
| Browser errors | `browser_errors` | 1 |
| Reconnects, restarts | counters on the server | 2 |

## Never kept

IP addresses, exact locations, game codes in page paths, anything about other sites. Headless browsers and playtests aren't counted.
