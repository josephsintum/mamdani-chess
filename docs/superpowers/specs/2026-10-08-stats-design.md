# Stats: first-party game metrics

**Date:** 2026-10-08 (revised 2026-10-19: game stats only)
**Status:** Agreed. Plan 1 built (PR #22); plan 2 built ([2026-10-09-stats-2-games.md](../plans/2026-10-09-stats-2-games.md)). Plan 1's visit tracking was removed before it shipped (2026-10-19, below). The page reads daily counters ([counters spec](2026-10-19-stats-counters-design.md)).
**Design:** the canvas https://claude.ai/artifact/Rzwy4PSQuJoKNZ629PYogL (Full site tab). It still shows the Visitors section and the funnel, which the page no longer has.

## Why

The site is going to friends (milestone 07). We want to know whether the game works: do games get going and finish, do quick matches find opponents, and are the pothole rules (3 rounds, cap of 5, saving rolls) doing what we want.

## Decisions

- **Game stats only, nothing about people** (decided 2026-10-19). The first version also counted visits: every page view with a visitor key (12 hex of the guest ID), country and city from a local DB-IP file, device, system, browser and referrer, and a visit-to-game funnel built on them. It was built (plan 1) and removed before it was deployed: the stats we act on come from the games, and per-person tracking isn't worth keeping. Gone with it: the `visits` table, `POST /api/visit`, the DB-IP download in the Docker image, the `maxminddb` dependency, the Visitors section, the funnel, and the guest key on quick-match searches.
- **First party, local.** The Go server counts in its own SQLite database and shows the numbers on a private page. No analytics company, no outside script.
- **Only numbers that lead to a decision.** No trivia (longest game, where pieces fell, White vs Black). Kept: how games end, game length, when people play, friend links, quick-match waits and the "searched when nobody else was looking" count, how full the road gets, whether the road decided the game.
- **Browser errors stay, anonymous.** A page reports an uncaught error to `POST /api/error`; the row keeps the time, the route (never a game code; codes are also stripped from the message) and the message, cut to 300 characters. No visitor, no browser, and the endpoint mints no cookie.
- **Playtests aren't counted.** Headless Chrome (`HeadlessChrome` in the user agent) and requests with `X-Playtest: 1` (the playtest script sends it on every request) are left out: their games are marked `playtest` and skipped, their searches, events and errors aren't written. Runs against production are left out too.
- **Private page** at `/stats`, HTTP Basic Auth with the password from `STATS_PASSWORD` (any user name). Without that variable the path serves the app shell as any unknown path does. Server-rendered with `html/template`, no JavaScript, Graphite style with one blue family for the charts (the canvas).
- **Days are grouped in `STATS_TZ`** (default `America/New_York`).
- **Browser errors are kept for 400 days**, pruned at startup. Games, searches and events are kept (the counters' rebuild recounts them).
- **The About page** says there are no ads and no analytics: games are counted, nothing in those counts says who played, and an error report carries nothing about the person.
- **Daily counters written with the data** (decided 2026-10-19; replaces the 24-hour cache planned on 2026-10-09): every counted write also adds to a daily counter in the same transaction, and a page load only sums counters. See [the counters spec](2026-10-19-stats-counters-design.md).

## What is recorded

**`game_stats`**: one row per game that got going, written with its result: potholes opened, reset, closed by their rounds or the cap, repaired; pieces fallen, saving rolls and saves, pieces each side lost; mate by a roll; how many holes were open after each turn.

**`games`** gains `kind` (`quick` or `friend`; a rematch keeps its game's kind), `joined_at` (when Black sat down) and `playtest`.

**`searches`**: one row per quick-match search: start, end, matched or gave up, how many others were looking at the start. Nothing about who searched.

**`events`**: a `practice` row per practice game, a `reconnect` row when the page reopens a game stream after an error, a `restart` row at each start.

**`browser_errors`**: time, route, message (300 characters at most).

## The page

One column, no sidebar (dropped 2026-10-10). Range: 7, 30, 90 days, all.

- **Overview**: Games (quick match and friend link), Got going (ended with a result), Practice games, Found an opponent (share of quick-match searches); each against the range before.
- **Games**: games per day by kind, how games end, game length (histogram, decided on the clock, winner's clock left), when people play (weekday × 2-hour grid), friend links (joined, finished, time to join, rematches). Medians become histograms with a threshold line ([counters spec](2026-10-19-stats-counters-design.md)).
- **Quick match**: wait histogram, found an opponent, gave up after (histogram), searched when nobody else was.
- **The road**: potholes a game, falls a game, saving rolls saved, ended by a roll; how full the road gets (open holes after each turn; closed by rounds / cap / repair; reset by a roll); did the road decide it (winner lost less / more / the same to potholes).
- **Health**: browser errors, streams reopened, server restarts.

Cards in a row share one height; a histogram's value labels stay inside the chart; chart legends drop below their title when there isn't room.

## Where each number comes from

| Stat | Source |
| --- | --- |
| How games end, game length, rematches, when people play | `games`, `turns` as saved today |
| Games per day by kind; practice count | `kind` on `games`; `practice` rows in `events` |
| Potholes, falls, saving rolls, repairs, resets, mate by a roll, how full the road gets, did the road decide it | `game_stats` (old games backfilled at startup by replaying their saved dice) |
| Quick match | `searches` |
| Friend links: joined, time to join | `joined_at` on `games`; rematches aren't links |
| Browser errors | `browser_errors` |
| Streams reopened, restarts | `events` |
| Playtests left out | `playtest` on `games`; a playtest's searches, events and errors aren't written |

## Never kept

Visits, IP addresses, locations, devices, browsers, referrers, game codes in paths or error messages, and anything that ties a search or an error to a person.
