# Stats 1: Visits and the Stats Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The browser reports every page view to our own server, which keeps it in SQLite with the visitor's rough location, device and browser, and a private `/stats` page shows visitors, the visit-to-game funnel, where people come from, what they use, and browser errors, as the stats spec describes.

**Architecture:**
- **`store`:** migration 7 adds `visits` and `browser_errors`; `store/visits.go` writes and prunes them; `store/stats.go` reads the numbers the page shows (aggregated in Go, so days follow a time zone).
- **`server`:** `POST /api/visit` and `POST /api/error` take the browser's reports; pure helpers turn a path, screen and user agent into the stored values; `server/geo.go` wraps the MMDB reader behind an interface; `GET /stats` renders an embedded `html/template` behind Basic Auth.
- **`cmd/server`:** reads `GEOIP_PATH`, `STATS_PASSWORD`, `STATS_TZ`, prunes old rows at startup.
- **Frontend:** `web/src/lib/visit.ts` builds and sends the reports; the layout sends one on every navigation and on every uncaught error; the About page says so; the playtest marks its requests so they aren't counted.
- **Deploy:** a Docker stage downloads DB-IP's City Lite file into the image.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` through `database/sql`, `html/template`, `github.com/oschwald/maxminddb-golang/v2` v2.7.0 (new; agreed), SvelteKit 3 / Svelte 5.57, Vitest, Playwright (playtest).

**Spec:** [docs/superpowers/specs/2026-10-08-stats-design.md](../specs/2026-10-08-stats-design.md). Read it first; this plan argues from it. The page's look: the canvas https://claude.ai/artifact/Rzwy4PSQuJoKNZ629PYogL (Full site tab).

**How this plan was made:** from a close reading of the code (the signatures and line numbers below were checked on `main` at 7466653), not from a scratch build. Every task ends by running its tests, so a step that doesn't compile shows up at once; fix it there and carry on.

## Global Constraints

- **Migrations are append only:** add migration 7; never edit 1–6.
- **One new Go dependency and no other:** `github.com/oschwald/maxminddb-golang/v2 v2.7.0`, added with `go get` in Task 3. Don't edit `go.mod` by hand. No new npm dependencies. Don't edit `names/`.
- **Nothing identifying is stored:** no IP address, no full guest ID (12 hex characters of `guestID()`), no game code (`pageOf` maps `/game/K7F3QZ` to `/game`), no referrer path (host only).
- **Robots and playtests are never counted:** a user agent containing `HeadlessChrome`, or a request with an `X-Playtest` header, gets 204 and no row.
- **JSON:** `POST /api/visit` body `{"path":"/game/ABCDEF","w":390,"h":844,"touch":true,"referrer":"https://l.instagram.com/"}`, reply 204 with no body. `POST /api/error` body `{"path":"/game/ABCDEF","message":"TypeError: x is undefined"}`, reply 204. Bad bodies get the existing 400 `{"error":"bad request body"}`. Both set the `guest` cookie when missing, like every API call.
- **`/stats`:** when `STATS_PASSWORD` is empty the path serves the app shell (so nothing new is reachable); otherwise it answers 401 with `WWW-Authenticate: Basic realm="stats"` until the right password arrives (any user name), compared in constant time.
- **Logging:** the request log line stays as it is. Nothing new logs the guest ID or the IP.
- **Frontend:** `#lib/...` imports with the file extension (no `$lib`), `$app/env` not `$app/environment`, no `$effect`, run `npx @sveltejs/mcp svelte-autofixer <file>` on every component you change, toasts only through `notify.*`.
- **Commits:** stage only the files you changed (`git add <paths>`, never `git add -A`), and add no Claude attribution (no `Co-Authored-By`, no "Generated with Claude Code").
- **Branch:** work on `stats-visits`, not `main`. Every push to `main` deploys and pauses games in progress, so it must not go out during a playtest.

## Review Focus

1. **A visit row that could identify someone.** The visitor key is 12 hex characters of a SHA-256; the path is normalised; the referrer is a host. Pinned by `TestVisitStoresNothingIdentifying` (Task 4), which posts a game path and a referrer with a query string and checks the row.
2. **A headless or playtest browser counted as a visitor.** Pinned by `TestVisitIgnoresRobots` (Task 4) for both the user agent and the header, and by the `X-Playtest` header on every playtest context (Task 6).
3. **`/stats` reachable without a password.** Pinned by `TestStatsNeedsPassword` (Task 5): no password configured → the app shell; configured → 401 without credentials, 401 with the wrong ones, 200 with the right ones.
4. **A visit from the day's last hour landing on the wrong day.** Days are grouped in `STATS_TZ`, not UTC. Pinned by `TestVisitStatsGroupsDaysInZone` (Task 1): 03:00 UTC on Oct 9 counts for Oct 8 in New York.
5. **The beacon failing on a page where the server is down (a deploy).** `sendBeacon` never throws and the fallback `fetch` swallows its rejection, so a deploy never surfaces an error toast or a console error (the playtest fails on console errors). Pinned by the `reportVisit` tests in Task 6.

## File map

| File | Task | Responsibility |
| --- | --- | --- |
| `store/migrate.go` | 1 | Migration 7: `visits`, `browser_errors` |
| `store/visits.go`, `store/visits_test.go` | 1 | `Visit`, `AddVisit`, `BrowserError`, `AddBrowserError`, `PruneVisits` |
| `store/stats.go`, `store/stats_test.go` | 1 | `VisitStats`, `FunnelStats` |
| `server/visit.go`, `server/visit_test.go` | 2, 4 | Pure helpers (`pageOf`, `deviceOf`, `parseUA`, `referrerOf`, `clientIP`, `isRobot`), then the two handlers |
| `server/geo.go`, `server/geo_test.go`, `go.mod`, `go.sum` | 3 | `GeoLookup`, `OpenGeo` over maxminddb |
| `server/stats.go`, `server/stats.html`, `server/stats_test.go` | 5 | `GET /stats`: Basic Auth, the range, the template |
| `server/server.go` | 4, 5 | Routes and the `Geo`, `StatsPassword`, `StatsZone` fields |
| `web/src/lib/visit.ts`, `web/src/lib/visit.test.ts` | 6 | `visitPayload`, `reportVisit`, `reportError` |
| `web/src/routes/+layout.svelte`, `web/src/routes/about/+page.svelte`, `web/scripts/playtest.js` | 6 | Send on navigation and on error; the privacy note; `X-Playtest` |
| `cmd/server/main.go`, `Dockerfile` | 7 | Env vars, prune at startup, the DB-IP file in the image |
| `contract/api_test.go`, `contract/main_test.go` | 7 | `POST /api/visit`, `/stats` with and without the password |
| `CLAUDE.md`, `docs/superpowers/plans/2026-10-03-00-roadmap.md` | 7 | Working notes and the milestone row |

---

### Task 1: The tables and the queries

**Files:**
- Modify: `store/migrate.go` (append migration 7 after the rules-2 retirement, line 66)
- Create: `store/visits.go`, `store/stats.go`
- Test: `store/visits_test.go`, `store/stats_test.go`

**Interfaces:**
- Consumes: `openTemp(t)` and `must(t, err)` from the existing store tests; `t0 = time.UnixMilli(1_791_000_000_000)` from `store/games_test.go`; `CreateGame`, `SeatBlack`, `AddTurn`, `EndGame` from `store/games.go`.
- Produces:
  - `type Visit struct { At time.Time; Visitor, Path, Referrer, Country, City, Device, OS, Browser string }`
  - `func (s *Store) AddVisit(ctx context.Context, v Visit) error`
  - `type BrowserError struct { At time.Time; Visitor, Path, Message, Browser string }`
  - `func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error`
  - `func (s *Store) PruneVisits(ctx context.Context, before time.Time) (int64, error)` (both tables; returns the rows removed)
  - `type Count struct { Label string; N int }`
  - `type DayCount struct { Day string; New, Returning int }` (`Day` is `2006-01-02` in the zone)
  - `type VisitStats struct { Visitors, Returning, Views int; Days []DayCount; Countries, Cities, Sources, Devices, Systems, Browsers, Pages []Count; Errors int; LatestError string }`
  - `func (s *Store) VisitStats(ctx context.Context, from, to time.Time, loc *time.Location) (VisitStats, error)`
  - `type Funnel struct { Visited, Opened, Moved, Finished, Again, Games int }`
  - `func (s *Store) FunnelStats(ctx context.Context, from, to time.Time) (Funnel, error)`

**How it works.** A visit is one row. `VisitStats` reads every visit in the range once (small data: a few friends), plus each visitor's first visit ever, and aggregates in Go so that "today" follows `loc` and "new" means "first visit ever is in this day". Top lists count distinct visitors (pages count views) and fold everything past the top six into "Other". `FunnelStats` joins visits to games through the 12-hex visitor key: `substr(games.white, 1, 12)`.

- [ ] **Step 1: Write the failing tests**

`store/visits_test.go`:

```go
package store

import (
	"testing"
	"time"
)

func TestVisitRoundTripAndPrune(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddVisit(ctx, Visit{At: t0, Visitor: "0123456789ab", Path: "/game", Referrer: "l.instagram.com",
		Country: "United States", City: "New York", Device: "phone", OS: "iOS", Browser: "Safari"}))
	must(t, s.AddVisit(ctx, Visit{At: t0.Add(24 * time.Hour), Visitor: "0123456789ab", Path: "/"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Visitor: "0123456789ab", Path: "/game", Message: "TypeError: x", Browser: "Safari"}))

	var n int
	var city string
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(city) FROM visits`).Scan(&n, &city))
	if n != 2 || city != "New York" {
		t.Fatalf("visits: %d rows, city %q", n, city)
	}
	removed, err := s.PruneVisits(ctx, t0.Add(time.Hour))
	must(t, err)
	if removed != 2 { // the first visit and the error
		t.Fatalf("pruned %d rows, want 2", removed)
	}
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits`).Scan(&n))
	if n != 1 {
		t.Fatalf("%d visits left, want 1", n)
	}
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM browser_errors`).Scan(&n))
	if n != 0 {
		t.Fatalf("%d errors left, want 0", n)
	}
}
```

`store/stats_test.go`:

```go
package store

import (
	"reflect"
	"testing"
	"time"
)

var newYork = must1(time.LoadLocation("America/New_York"))

func must1[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// day returns the instant at hour o'clock in New York on day d of October 2026.
func day(d, hour int) time.Time { return time.Date(2026, 10, d, hour, 0, 0, 0, newYork) }

func seedVisits(t *testing.T, s *Store) {
	t.Helper()
	ctx := t.Context()
	v := func(at time.Time, who, path string, more Visit) {
		more.At, more.Visitor, more.Path = at, who, path
		must(t, s.AddVisit(ctx, more))
	}
	// ann: three days, phone from New York, came from Instagram
	v(day(1, 9), "ann000000000", "/", Visit{Referrer: "l.instagram.com", Country: "United States", City: "New York", Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(1, 9), "ann000000000", "/play", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(2, 20), "ann000000000", "/game", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(3, 20), "ann000000000", "/game", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	// bob: one day, desktop from Toronto, an invite link
	v(day(2, 12), "bob000000000", "/game", Visit{Country: "Canada", City: "Toronto", Device: "desktop", OS: "macOS", Browser: "Chrome"})
	// cat: first visit before the range, so she is returning on day 3
	v(day(1, 9).Add(-30*24*time.Hour), "cat000000000", "/", Visit{Device: "desktop", OS: "Windows", Browser: "Firefox"})
	v(day(3, 23).Add(59*time.Minute), "cat000000000", "/rules", Visit{Device: "desktop", OS: "Windows", Browser: "Firefox"})
	must(t, s.AddBrowserError(ctx, BrowserError{At: day(3, 10), Visitor: "ann000000000", Path: "/game", Message: "TypeError: x is undefined", Browser: "Safari"}))
}

func TestVisitStats(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedVisits(t, s)
	st, err := s.VisitStats(ctx, day(1, 0), day(4, 0), newYork)
	must(t, err)
	if st.Visitors != 3 || st.Returning != 1 || st.Views != 6 {
		t.Errorf("visitors %d returning %d views %d, want 3 1 6", st.Visitors, st.Returning, st.Views)
	}
	wantDays := []DayCount{{"2026-10-01", 1, 0}, {"2026-10-02", 1, 1}, {"2026-10-03", 0, 2}}
	if !reflect.DeepEqual(st.Days, wantDays) {
		t.Errorf("days = %v, want %v", st.Days, wantDays)
	}
	check := func(what string, got, want []Count) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	// Ties sort by label, ignoring case.
	check("countries", st.Countries, []Count{{"Canada", 1}, {"United States", 1}, {"Unknown", 1}})
	check("cities", st.Cities, []Count{{"New York", 1}, {"Toronto", 1}, {"Unknown", 1}})
	check("sources", st.Sources, []Count{{"A game invite link", 1}, {"Direct or a chat app", 1}, {"Instagram", 1}})
	check("devices", st.Devices, []Count{{"desktop", 2}, {"phone", 1}})
	check("systems", st.Systems, []Count{{"iOS", 1}, {"macOS", 1}, {"Windows", 1}})
	check("browsers", st.Browsers, []Count{{"Chrome", 1}, {"Firefox", 1}, {"Safari", 1}})
	check("pages", st.Pages, []Count{{"/game", 3}, {"/", 1}, {"/play", 1}, {"/rules", 1}})
	if st.Errors != 1 || st.LatestError != "TypeError: x is undefined" {
		t.Errorf("errors %d %q", st.Errors, st.LatestError)
	}
}

// A visit at 03:00 UTC on Oct 4 is still Oct 3 in New York.
func TestVisitStatsGroupsDaysInZone(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddVisit(ctx, Visit{At: time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC), Visitor: "ann000000000", Path: "/"}))
	st, err := s.VisitStats(ctx, day(3, 0), day(4, 0), newYork)
	must(t, err)
	if len(st.Days) != 1 || st.Days[0].Day != "2026-10-03" || st.Days[0].New != 1 {
		t.Fatalf("days = %v", st.Days)
	}
}

// Top lists keep six entries and fold the rest into "Other".
func TestVisitStatsFoldsTheLongTail(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	for i, c := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		must(t, s.AddVisit(ctx, Visit{At: day(1, 10), Visitor: string(rune('a'+i)) + "00000000000", Path: "/", Country: c}))
	}
	must(t, s.AddVisit(ctx, Visit{At: day(1, 11), Visitor: "a00000000000", Path: "/", Country: "A"}))
	st, err := s.VisitStats(ctx, day(1, 0), day(2, 0), newYork)
	must(t, err)
	if len(st.Countries) != 7 || st.Countries[0] != (Count{"A", 1}) || st.Countries[6] != (Count{"Other", 2}) {
		t.Fatalf("countries = %v", st.Countries)
	}
}

func TestFunnelStats(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedVisits(t, s)
	// ann (white) and bob (black) played a game to checkmate; ann played a second one
	// that was aborted; dan moved in a game that is still on.
	ann, bob, dan := "ann000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff", "bob000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff", "dan000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff"
	must(t, s.CreateGame(ctx, Game{Code: "G00001", White: ann, WhiteName: "a", CreatedAt: day(2, 12)}))
	must(t, s.SeatBlack(ctx, "G00001", bob, "b"))
	must(t, s.AddTurn(ctx, "G00001", Turn{Ply: 0, Move: "e2e4", At: day(2, 12)}))
	must(t, s.EndGame(ctx, "G00001", Result{EndedAt: day(2, 13), Reason: "checkmate", Winner: "white"}, &Turn{Ply: 1, Move: "e7e5", At: day(2, 13)}))
	must(t, s.CreateGame(ctx, Game{Code: "G00002", White: ann, WhiteName: "a", Black: dan, BlackName: "d", CreatedAt: day(3, 12)}))
	must(t, s.EndGame(ctx, "G00002", Result{EndedAt: day(3, 13), Reason: "aborted"}, nil))
	must(t, s.CreateGame(ctx, Game{Code: "G00003", White: dan, WhiteName: "d", CreatedAt: day(3, 14)}))
	must(t, s.AddTurn(ctx, "G00003", Turn{Ply: 0, Move: "d2d4", At: day(3, 14)}))
	f, err := s.FunnelStats(ctx, day(1, 0), day(4, 0))
	must(t, err)
	want := Funnel{Visited: 3, Opened: 2, Moved: 3, Finished: 2, Again: 0, Games: 3}
	if f != want {
		t.Fatalf("funnel = %+v, want %+v", f, want)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./store -run 'TestVisit|TestFunnel' -count=1`
Expected: FAIL to compile (`undefined: Visit`).

- [ ] **Step 3: Implement**

Append to `migrations` in `store/migrate.go`, after the migration 6 string (keep the comma style):

```go
	// 7: visits and browser errors (stats plan 1). visitor is the first 12
	// hex characters of the guest ID; path is a route, never a game code;
	// referrer is a host. Both tables are pruned after 400 days.
	`CREATE TABLE visits (
		at       INTEGER NOT NULL,
		visitor  TEXT NOT NULL,
		path     TEXT NOT NULL,
		referrer TEXT NOT NULL DEFAULT '',
		country  TEXT NOT NULL DEFAULT '',
		city     TEXT NOT NULL DEFAULT '',
		device   TEXT NOT NULL DEFAULT '',
		os       TEXT NOT NULL DEFAULT '',
		browser  TEXT NOT NULL DEFAULT ''
	 );
	 CREATE INDEX visits_at ON visits(at);
	 CREATE INDEX visits_visitor ON visits(visitor, at);
	 CREATE TABLE browser_errors (
		at      INTEGER NOT NULL,
		visitor TEXT NOT NULL,
		path    TEXT NOT NULL,
		message TEXT NOT NULL,
		browser TEXT NOT NULL DEFAULT ''
	 );
	 CREATE INDEX browser_errors_at ON browser_errors(at);`,
```

`store/visits.go`:

```go
package store

import (
	"context"
	"time"
)

// Visit is one page view. Visitor is the first 12 hex characters of the
// guest ID, Path a route (never a game code), Referrer a host, or
// "ref:<tag>" for a ?ref= link. Country, City, Device, OS and Browser are
// "" when unknown.
type Visit struct {
	At                                                     time.Time
	Visitor, Path, Referrer, Country, City, Device, OS, Browser string
}

// BrowserError is an uncaught error a page reported.
type BrowserError struct {
	At                              time.Time
	Visitor, Path, Message, Browser string
}

func (s *Store) AddVisit(ctx context.Context, v Visit) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO visits (at, visitor, path, referrer, country, city, device, os, browser) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.At.UnixMilli(), v.Visitor, v.Path, v.Referrer, v.Country, v.City, v.Device, v.OS, v.Browser)
	return err
}

func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO browser_errors (at, visitor, path, message, browser) VALUES (?, ?, ?, ?, ?)`,
		e.At.UnixMilli(), e.Visitor, e.Path, e.Message, e.Browser)
	return err
}

// PruneVisits deletes visits and browser errors from before the cutoff and
// returns how many rows went.
func (s *Store) PruneVisits(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for _, table := range []string{"visits", "browser_errors"} {
		res, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE at < ?`, before.UnixMilli())
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
```

`store/stats.go`:

```go
package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
)

// Count is one row of a top list.
type Count struct {
	Label string
	N     int
}

// DayCount is one day's visitors: New made their first visit ever that
// day, Returning had visited before. Day is 2006-01-02 in the stats zone.
type DayCount struct {
	Day            string
	New, Returning int
}

// VisitStats is what the Visitors and Health sections show for a range.
type VisitStats struct {
	Visitors  int // distinct visitors
	Returning int // visitors seen on two or more days in the range
	Views     int
	Days      []DayCount
	// Top lists of distinct visitors (Pages: of views), at most six entries
	// plus "Other". Labels for unknowns are "Unknown".
	Countries, Cities, Sources, Devices, Systems, Browsers, Pages []Count
	Errors      int
	LatestError string
}

// topN is how many entries a top list keeps before "Other".
const topN = 6

// VisitStats aggregates the visits with from <= at < to. Days are grouped in loc.
func (s *Store) VisitStats(ctx context.Context, from, to time.Time, loc *time.Location) (VisitStats, error) {
	var st VisitStats
	// Each visitor's first visit ever, so "new" means new to the site.
	first := map[string]int64{}
	rows, err := s.db.QueryContext(ctx, `SELECT visitor, MIN(at) FROM visits GROUP BY visitor`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var who string
		var at int64
		if err := rows.Scan(&who, &at); err != nil {
			rows.Close()
			return st, err
		}
		first[who] = at
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	rows, err = s.db.QueryContext(ctx,
		`SELECT at, visitor, path, referrer, country, city, device, os, browser FROM visits WHERE at >= ? AND at < ? ORDER BY at`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return st, err
	}
	defer rows.Close()
	type visitorDays struct {
		days map[string]bool
		firstPath, firstRef, country, city, device, os, browser string
	}
	visitors := map[string]*visitorDays{}
	dayVisitors := map[string]map[string]bool{} // day → visitors seen
	pages := map[string]int{}
	for rows.Next() {
		var at int64
		var v Visit
		if err := rows.Scan(&at, &v.Visitor, &v.Path, &v.Referrer, &v.Country, &v.City, &v.Device, &v.OS, &v.Browser); err != nil {
			return st, err
		}
		st.Views++
		pages[v.Path]++
		day := time.UnixMilli(at).In(loc).Format("2006-01-02")
		vd := visitors[v.Visitor]
		if vd == nil {
			vd = &visitorDays{days: map[string]bool{}, firstPath: v.Path, firstRef: v.Referrer}
			visitors[v.Visitor] = vd
		}
		vd.days[day] = true
		// The first non-empty value wins: a later view rarely knows more.
		if vd.country == "" {
			vd.country, vd.city = v.Country, v.City
		}
		if vd.device == "" {
			vd.device = v.Device
		}
		if vd.os == "" {
			vd.os = v.OS
		}
		if vd.browser == "" {
			vd.browser = v.Browser
		}
		if dayVisitors[day] == nil {
			dayVisitors[day] = map[string]bool{}
		}
		dayVisitors[day][v.Visitor] = true
	}
	if err := rows.Err(); err != nil {
		return st, err
	}

	st.Visitors = len(visitors)
	countries, cities, sources, devices, systems, browsers := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, vd := range visitors {
		if len(vd.days) >= 2 {
			st.Returning++
		}
		countries[orUnknown(vd.country)]++
		cities[orUnknown(vd.city)]++
		sources[sourceOf(vd.firstPath, vd.firstRef)]++
		devices[orUnknown(vd.device)]++
		systems[orUnknown(vd.os)]++
		browsers[orUnknown(vd.browser)]++
	}
	for day, who := range dayVisitors {
		dc := DayCount{Day: day}
		dayStart := dayStartOf(day, loc).UnixMilli()
		for w := range who {
			if first[w] >= dayStart {
				dc.New++
			} else {
				dc.Returning++
			}
		}
		st.Days = append(st.Days, dc)
	}
	sort.Slice(st.Days, func(i, j int) bool { return st.Days[i].Day < st.Days[j].Day })
	st.Countries, st.Cities, st.Sources = top(countries), top(cities), top(sources)
	st.Devices, st.Systems, st.Browsers, st.Pages = top(devices), top(systems), top(browsers), top(pages)

	var latest sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), (SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1) FROM browser_errors WHERE at >= ? AND at < ?`,
		from.UnixMilli(), to.UnixMilli(), from.UnixMilli(), to.UnixMilli()).Scan(&st.Errors, &latest); err != nil {
		return st, err
	}
	st.LatestError = latest.String
	return st, nil
}

func dayStartOf(day string, loc *time.Location) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", day, loc)
	return t
}

func orUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

// sourceOf names where a visitor came from, by their first page view in
// the range: a game link, a tagged link, a known site, or nothing (a typed
// URL or a chat app, which pass no referrer).
func sourceOf(path, ref string) string {
	switch {
	case strings.HasPrefix(ref, "ref:"):
		return "Link tagged " + strings.TrimPrefix(ref, "ref:")
	case strings.Contains(ref, "instagram.com"):
		return "Instagram"
	case strings.Contains(ref, "facebook.com"):
		return "Facebook"
	case strings.Contains(ref, "google.") || strings.Contains(ref, "bing.com") || strings.Contains(ref, "duckduckgo.com"):
		return "Search"
	case ref != "":
		return ref
	case path == "/game":
		return "A game invite link"
	}
	return "Direct or a chat app"
}

// top sorts counts by size (ties by label, ignoring case) and folds
// everything past topN into "Other".
func top(m map[string]int) []Count {
	var out []Count
	for label, n := range m {
		out = append(out, Count{label, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	if len(out) > topN {
		other := 0
		for _, c := range out[topN:] {
			other += c.N
		}
		out = append(out[:topN], Count{"Other", other})
	}
	return out
}

// Funnel counts visitors who got each step further, for games created in
// the range. Moved: white made ply 0 or black ply 1. Finished: the game
// ended with a result (not aborted, expired or retired). Again: finished
// two or more games. Games: games created between players.
type Funnel struct {
	Visited, Opened, Moved, Finished, Again, Games int
}

const finishedReasons = `('checkmate','stalemate','fifty_moves','repetition','insufficient_material','timeout','timeout_vs_insufficient','resignation')`

func (s *Store) FunnelStats(ctx context.Context, from, to time.Time) (Funnel, error) {
	var f Funnel
	a, b := from.UnixMilli(), to.UnixMilli()
	for _, q := range []struct {
		dst  *int
		sql  string
		args []any
	}{
		{&f.Visited, `SELECT COUNT(DISTINCT visitor) FROM visits WHERE at >= ? AND at < ?`, []any{a, b}},
		{&f.Opened, `SELECT COUNT(DISTINCT visitor) FROM visits WHERE at >= ? AND at < ? AND path IN ('/game', '/play', '/practice')`, []any{a, b}},
		{&f.Moved, `SELECT COUNT(*) FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND EXISTS (SELECT 1 FROM turns WHERE game = code AND ply = 0)
			UNION
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND EXISTS (SELECT 1 FROM turns WHERE game = code AND ply = 1))`, []any{a, b, a, b}},
		{&f.Finished, `SELECT COUNT(*) FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND result IN ` + finishedReasons + `
			UNION
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND result IN ` + finishedReasons + `)`, []any{a, b, a, b}},
		{&f.Again, `SELECT COUNT(*) FROM (SELECT g FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND result IN ` + finishedReasons + `
			UNION ALL
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND result IN ` + finishedReasons + `) GROUP BY g HAVING COUNT(*) >= 2)`, []any{a, b, a, b}},
		{&f.Games, `SELECT COUNT(*) FROM games WHERE created_at >= ? AND created_at < ?`, []any{a, b}},
	} {
		if err := s.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dst); err != nil {
			return f, err
		}
	}
	return f, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./store -count=1`
Expected: `ok` (the new tests and `TestMigrationsAreIdempotent`, which now expects version 7). If `TestVisitStats` reports `pages`, remember pages count views, the others visitors.

- [ ] **Step 5: Commit**

```bash
git add store/migrate.go store/visits.go store/visits_test.go store/stats.go store/stats_test.go
git commit -m "store: visits and browser errors, with the stats queries"
```

---

### Task 2: Turning a request into a visit row

**Files:**
- Create: `server/visit.go` (helpers only; the handlers come in Task 4)
- Test: `server/visit_test.go`

**Interfaces:**
- Produces, all in package `server`:
  - `const visitorLen = 12`; `func visitorOf(guest string) string`
  - `func pageOf(path string) string`: `/`, `/game`, `/play`, `/practice`, `/how-to-play`, `/rules`, `/about` by their first segment; anything else `other`.
  - `func deviceOf(w, h int, touch bool) string`: `""` when either is 0; short side `< 640` → `phone`; else `tablet` when the screen is a touch screen, else `desktop`. (A laptop's short side is often under 1024, so the screen alone can't separate laptops from tablets; iPads say they are Macs, so the user agent can't either. Touch does.)
  - `func parseUA(ua string) (os, browser string)`
  - `func referrerOf(raw, ourHost string) string`: the referrer's host without `www.`, `""` for none, an unparsable value or our own host; `raw` of the form `ref:<tag>` (what the page sends for a `?ref=` link) passes through as `ref:<tag>` with the tag cut to 20 characters.
  - `func clientIP(r *http.Request) (netip.Addr, bool)`: the last `X-Forwarded-For` entry, else `X-Real-IP`, else `RemoteAddr`.
  - `func isRobot(r *http.Request) bool`: `HeadlessChrome` in the user agent, or an `X-Playtest` header.

- [ ] **Step 1: Write the failing tests**

`server/visit_test.go`:

```go
package server

import (
	"net/http/httptest"
	"testing"
)

func TestPageOf(t *testing.T) {
	for in, want := range map[string]string{
		"/": "/", "/game/K7F3QZ": "/game", "/game/K7F3QZ?instant": "/game", "/play": "/play", "/practice": "/practice",
		"/how-to-play": "/how-to-play", "/rules": "/rules", "/about": "/about", "/dev/board": "other", "": "other", "/stats": "other",
	} {
		if got := pageOf(in); got != want {
			t.Errorf("pageOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceOf(t *testing.T) {
	for _, c := range []struct {
		w, h  int
		touch bool
		want  string
	}{{390, 844, true, "phone"}, {844, 390, true, "phone"}, {639, 1200, true, "phone"}, {768, 1024, true, "tablet"}, {1024, 1366, true, "tablet"},
		{1440, 900, false, "desktop"}, {1440, 900, true, "tablet"}, {2560, 1440, false, "desktop"}, {0, 0, false, ""}} {
		if got := deviceOf(c.w, c.h, c.touch); got != c.want {
			t.Errorf("deviceOf(%d, %d, %v) = %q, want %q", c.w, c.h, c.touch, got, c.want)
		}
	}
}

func TestParseUA(t *testing.T) {
	for ua, want := range map[string][2]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1":              {"iOS", "Safari"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 334.0.4.32.98":                 {"iOS", "Instagram (in app)"},
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36":                              {"Android", "Chrome"},
		"Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36":            {"Android", "Samsung"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15":                              {"macOS", "Safari"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36":                              {"macOS", "Chrome"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0":                       {"Windows", "Edge"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0":                                                                   {"Windows", "Firefox"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/126.0 Mobile/15E148 Safari/605.1.15":           {"iOS", "Firefox"},
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36":                                              {"Linux", "Chrome"},
		"curl/8.6.0": {"", ""},
	} {
		os, browser := parseUA(ua)
		if os != want[0] || browser != want[1] {
			t.Errorf("parseUA(%q) = %q, %q; want %q, %q", ua, os, browser, want[0], want[1])
		}
	}
}

func TestReferrerOf(t *testing.T) {
	for in, want := range map[string]string{
		"":                                       "",
		"https://l.instagram.com/?u=x&e=y":       "l.instagram.com",
		"https://www.google.com/search?q=mamdani": "google.com",
		"https://mamdanichess.com/game/K7F3QZ":   "",
		"ref:ig":                                 "ref:ig",
		"ref:" + "a-very-long-tag-that-goes-on-and-on": "ref:a-very-long-tag-that",
		"not a url":                              "",
	} {
		if got := referrerOf(in, "mamdanichess.com"); got != want {
			t.Errorf("referrerOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/visit", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if ip, ok := clientIP(r); !ok || ip.String() != "10.0.0.5" {
		t.Errorf("from RemoteAddr: %v %v", ip, ok)
	}
	r.Header.Set("X-Real-IP", "203.0.113.9")
	if ip, _ := clientIP(r); ip.String() != "203.0.113.9" {
		t.Errorf("from X-Real-IP: %v", ip)
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.77")
	if ip, _ := clientIP(r); ip.String() != "203.0.113.77" {
		t.Errorf("from X-Forwarded-For: %v", ip)
	}
	r.Header.Set("X-Forwarded-For", "garbage")
	if _, ok := clientIP(r); ok {
		t.Error("garbage parsed as an address")
	}
}

func TestIsRobot(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/visit", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/125.0.0.0 Safari/537.36")
	if !isRobot(r) {
		t.Error("headless Chrome not a robot")
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Version/17.5 Mobile/15E148 Safari/604.1")
	if isRobot(r) {
		t.Error("a phone is a robot")
	}
	r.Header.Set("X-Playtest", "1")
	if !isRobot(r) {
		t.Error("the playtest header not a robot")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./server -run 'TestPageOf|TestDeviceOf|TestParseUA|TestReferrerOf|TestClientIP|TestIsRobot' -count=1`
Expected: FAIL to compile (`undefined: pageOf`).

- [ ] **Step 3: Implement**

`server/visit.go`:

```go
package server

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// visitorLen is how much of the guest ID a visit keeps: enough to tell
// visitors apart and join them to their games, not enough to be the ID.
const visitorLen = 12

func visitorOf(guest string) string {
	if len(guest) < visitorLen {
		return guest
	}
	return guest[:visitorLen]
}

// pages are the routes a visit can name; anything else is "other".
var pages = map[string]bool{"/": true, "/game": true, "/play": true, "/practice": true, "/how-to-play": true, "/rules": true, "/about": true}

// pageOf reduces a path to its route, so a game code is never stored.
func pageOf(path string) string {
	path, _, _ = strings.Cut(path, "?")
	if path == "/" {
		return "/"
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if route := "/" + first; pages[route] {
		return route
	}
	return "other"
}

// deviceOf reads the device from the screen: a short side under 640 CSS px
// is a phone (the app's own phone breakpoint); otherwise a touch screen is
// a tablet and anything else a desktop. iPads say they are Macs, so the
// user agent can't tell, and a laptop's short side is often under 1024, so
// the size alone can't either.
func deviceOf(w, h int, touch bool) string {
	switch {
	case w <= 0 || h <= 0:
		return ""
	case min(w, h) < 640:
		return "phone"
	case touch:
		return "tablet"
	}
	return "desktop"
}

// parseUA names the operating system and browser, by plain string
// matching; "" when it can't. In-app browsers come first because they
// also carry the system browser's tokens.
func parseUA(ua string) (os, browser string) {
	has := func(s string) bool { return strings.Contains(ua, s) }
	switch {
	case has("iPhone"), has("iPad"), has("iPod"):
		os = "iOS"
	case has("Android"):
		os = "Android"
	case has("Windows"):
		os = "Windows"
	case has("Mac OS X"), has("Macintosh"):
		os = "macOS"
	case has("CrOS"):
		os = "ChromeOS"
	case has("Linux"):
		os = "Linux"
	}
	switch {
	case has("Instagram"):
		browser = "Instagram (in app)"
	case has("FBAN"), has("FBAV"):
		browser = "Facebook (in app)"
	case has("Edg/"), has("EdgiOS/"):
		browser = "Edge"
	case has("SamsungBrowser/"):
		browser = "Samsung"
	case has("OPR/"), has("Opera"):
		browser = "Opera"
	case has("Firefox/"), has("FxiOS/"):
		browser = "Firefox"
	case has("CriOS/"), has("Chrome/"):
		browser = "Chrome"
	case has("Safari/") && has("Version/"):
		browser = "Safari"
	}
	return os, browser
}

// referrerOf keeps a referrer's host, or the page's own ref:<tag>; never a
// path or query, and nothing for our own site.
func referrerOf(raw, ourHost string) string {
	if tag, ok := strings.CutPrefix(raw, "ref:"); ok {
		if len(tag) > 20 {
			tag = tag[:20]
		}
		return "ref:" + tag
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == strings.TrimPrefix(strings.ToLower(ourHost), "www.") {
		return ""
	}
	return host
}

// clientIP is the caller's address: behind Railway's proxy the last
// X-Forwarded-For entry, else X-Real-IP, else the connection's. It is used
// for the city lookup and never stored.
func clientIP(r *http.Request) (netip.Addr, bool) {
	var raw string
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		raw = parts[len(parts)-1]
	} else if real := r.Header.Get("X-Real-IP"); real != "" {
		raw = real
	} else if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		raw = host
	} else {
		raw = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(raw))
	return ip, err == nil
}

// isRobot reports a visit that isn't a person: headless Chrome (the
// playtest, agent-browser) or any request the playtest marks.
func isRobot(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "HeadlessChrome") || r.Header.Get("X-Playtest") != ""
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./server -run 'TestPageOf|TestDeviceOf|TestParseUA|TestReferrerOf|TestClientIP|TestIsRobot' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/visit.go server/visit_test.go
git commit -m "server: read a page, device, browser and referrer from a visit"
```

---

### Task 3: The city lookup

**Files:**
- Create: `server/geo.go`
- Modify: `go.mod`, `go.sum` (by `go get` only)
- Test: `server/geo_test.go`

**Interfaces:**
- Produces:
  - `type GeoLookup interface { Lookup(ip netip.Addr) (country, city string) }`
  - `func OpenGeo(path string) (GeoLookup, error)`: `nil, nil` for an empty path; the file is memory-mapped.
  - `type geoFunc func(netip.Addr) (string, string)` with a `Lookup` method, so tests pass a function.

**Why an interface.** The handler (Task 4) does `if s.Geo != nil { country, city = s.Geo.Lookup(ip) }`, so `go run` without the file, the tests and the contract run all work with no database, and a test can fake the answer.

- [ ] **Step 1: Add the dependency**

Run, from the repo root:

```bash
go get github.com/oschwald/maxminddb-golang/v2@v2.7.0
```

Expected: `go.mod` gains `require github.com/oschwald/maxminddb-golang/v2 v2.7.0` (plus any indirect lines it needs) and `go.sum` its hashes. Run `go mod tidy` afterwards only if `go build ./...` asks for it.

- [ ] **Step 2: Write the failing tests**

`server/geo_test.go`:

```go
package server

import (
	"net/netip"
	"testing"
)

func TestOpenGeoWithoutAFile(t *testing.T) {
	g, err := OpenGeo("")
	if g != nil || err != nil {
		t.Fatalf("OpenGeo(\"\") = %v, %v; want nil, nil", g, err)
	}
	if _, err := OpenGeo(t.TempDir() + "/missing.mmdb"); err == nil {
		t.Fatal("a missing file opened")
	}
}

func TestGeoFunc(t *testing.T) {
	var g GeoLookup = geoFunc(func(ip netip.Addr) (string, string) { return "Canada", "Toronto" })
	if c, city := g.Lookup(netip.MustParseAddr("203.0.113.9")); c != "Canada" || city != "Toronto" {
		t.Fatalf("got %q, %q", c, city)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./server -run 'TestOpenGeo|TestGeoFunc' -count=1`
Expected: FAIL to compile (`undefined: OpenGeo`).

- [ ] **Step 4: Implement**

`server/geo.go`:

```go
package server

import (
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoLookup names the country and city an address is in, "" when unknown.
// The server's Geo is nil without a database: then every visit is
// "Unknown", and nothing else changes.
type GeoLookup interface {
	Lookup(ip netip.Addr) (country, city string)
}

// geoFunc lets a plain function serve as a GeoLookup (tests).
type geoFunc func(netip.Addr) (string, string)

func (f geoFunc) Lookup(ip netip.Addr) (string, string) { return f(ip) }

type mmdb struct{ r *maxminddb.Reader }

// OpenGeo opens an MMDB file such as DB-IP's City Lite (dbip-city-lite.mmdb);
// an empty path means no lookups. The file is memory-mapped, so a 130 MB
// database costs no RAM until it is read.
func OpenGeo(path string) (GeoLookup, error) {
	if path == "" {
		return nil, nil
	}
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	return mmdb{r}, nil
}

// Lookup reads the English names; DB-IP and MaxMind both store them under
// country.names.en and city.names.en.
func (m mmdb) Lookup(ip netip.Addr) (country, city string) {
	res := m.r.Lookup(ip)
	if !res.Found() {
		return "", ""
	}
	res.DecodePath(&country, "country", "names", "en")
	res.DecodePath(&city, "city", "names", "en")
	return country, city
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./server -run 'TestOpenGeo|TestGeoFunc' -count=1 && go vet ./...`
Expected: PASS, no vet findings.

- [ ] **Step 6: Try it against the real file, once, by hand**

Run, in the scratch directory (not the repo):

```bash
curl -fsSL -o /tmp/dbip.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-$(date +%Y-%m).mmdb.gz" && gunzip -f /tmp/dbip.mmdb.gz
```

Then, from the repo root, a throwaway test (don't commit it):

```go
// server/geo_real_test.go
package server

import (
	"net/netip"
	"testing"
)

func TestRealLookup(t *testing.T) {
	g, err := OpenGeo("/tmp/dbip.mmdb")
	if err != nil {
		t.Skip(err)
	}
	t.Log(g.Lookup(netip.MustParseAddr("8.8.8.8")))
	t.Log(g.Lookup(netip.MustParseAddr("142.250.80.46")))
}
```

Run: `go test ./server -run TestRealLookup -v -count=1`
Expected: two log lines naming a country (and usually a city); then `rm server/geo_real_test.go`. If the names come back empty, print `res.Decode(&any)` once to see DB-IP's record layout and fix the `DecodePath` keys.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum server/geo.go server/geo_test.go
git commit -m "server: city and country lookups from a local MMDB file"
```

---

### Task 4: `POST /api/visit` and `POST /api/error`

**Files:**
- Modify: `server/visit.go` (add the handlers), `server/server.go` (the `Geo` field and two routes)
- Test: `server/visit_test.go`

**Interfaces:**
- Consumes: `decodeBody`, `guestID`, `s.internalError` (server.go), `store.AddVisit`, `store.AddBrowserError` (Task 1), the helpers of Task 2, `GeoLookup` (Task 3).
- Produces:
  - On `Server`: `Geo GeoLookup` (exported, set by `main`; nil = unknown).
  - Routes `POST /api/visit` and `POST /api/error`, both answering 204.
  - `visitBody{Path string; W, H int; Touch bool; Referrer string}` and `errorBody{Path, Message string}` (unexported).

- [ ] **Step 1: Write the failing tests**

Append to `server/visit_test.go` (its imports become `net/http`, `net/http/httptest`, `net/netip`, `reflect`, `strings`, `testing`, `time` and `mamdani-chess/store`). The store has no raw-row accessor and shouldn't grow one for a test, so rows are read back through the same query the page uses:

```go
// lastVisit returns the stats for everything stored, so a test can check a
// row through the same path the page uses.
func lastVisit(t *testing.T, s *Server) store.VisitStats {
	t.Helper()
	st, err := s.store.VisitStats(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestVisitStoresNothingIdentifying(t *testing.T) {
	s, ts := newTestServer(t)
	s.Geo = geoFunc(func(ip netip.Addr) (string, string) {
		if ip.String() != "203.0.113.9" {
			t.Errorf("looked up %v", ip)
		}
		return "Canada", "Toronto"
	})
	alice := newPlayer(t, ts)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/visit",
		strings.NewReader(`{"path":"/game/K7F3QZ?instant","w":390,"h":844,"touch":true,"referrer":"https://l.instagram.com/?u=secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1")
	resp, err := alice.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /api/visit: %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 1 || resp.Cookies()[0].Name != "guest" {
		t.Errorf("a first visit set cookies %v, want the guest cookie", resp.Cookies())
	}
	st := lastVisit(t, s)
	got := []any{st.Views, st.Pages, st.Sources, st.Countries, st.Cities, st.Devices, st.Systems, st.Browsers}
	want := []any{1, []store.Count{{"/game", 1}}, []store.Count{{"Instagram", 1}}, []store.Count{{"Canada", 1}}, []store.Count{{"Toronto", 1}},
		[]store.Count{{"phone", 1}}, []store.Count{{"iOS", 1}}, []store.Count{{"Safari", 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
}

func TestVisitIgnoresRobots(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	for _, h := range []http.Header{
		{"User-Agent": {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/125.0.0.0 Safari/537.36"}},
		{"X-Playtest": {"1"}},
	} {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/visit", strings.NewReader(`{"path":"/","w":1440,"h":900}`))
		req.Header = h
		req.Header.Set("Content-Type", "application/json")
		resp, err := alice.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("%v: %d", h, resp.StatusCode)
		}
	}
	if st := lastVisit(t, s); st.Views != 0 {
		t.Errorf("robots stored %d views", st.Views)
	}
}

func TestVisitAndErrorBodies(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, body := alice.post("/api/visit", `nope`); status != http.StatusBadRequest || body != `{"error":"bad request body"}` {
		t.Errorf("bad visit: %d %s", status, body)
	}
	long := strings.Repeat("x", 500)
	if status, _ := alice.post("/api/error", `{"path":"/game/K7F3QZ","message":"`+long+`"}`); status != http.StatusNoContent {
		t.Errorf("error report: %d", status)
	}
	st := lastVisit(t, s)
	if st.Errors != 1 || len(st.LatestError) != 300 {
		t.Errorf("errors %d, latest %d chars; want 1 and 300", st.Errors, len(st.LatestError))
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./server -run 'TestVisit' -count=1`
Expected: FAIL to compile (`s.Geo undefined`).

- [ ] **Step 3: Implement**

In `server/server.go`, add the field after `Version` in the `Server` struct:

```go
	// Geo names a visitor's country and city for the stats page; nil
	// without a database (then they're "Unknown").
	Geo GeoLookup
```

and two routes after `POST /api/practice`:

```go
	s.mux.HandleFunc("POST /api/visit", s.visit)
	s.mux.HandleFunc("POST /api/error", s.browserError)
```

Append to `server/visit.go` (add `"time"` and `"mamdani-chess/store"` to its imports):

```go
// visitBody is what the page sends on every navigation: the path (the
// server keeps only the route), the screen in CSS px and whether it is a
// touch screen, and the referrer or "ref:<tag>" on the first page of a visit.
type visitBody struct {
	Path     string `json:"path"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	Touch    bool   `json:"touch"`
	Referrer string `json:"referrer"`
}

type errorBody struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// errorLen caps a reported message.
const errorLen = 300

// visit records a page view. Robots get a 204 and no row.
func (s *Server) visit(w http.ResponseWriter, r *http.Request) {
	if isRobot(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body visitBody
	if !decodeBody(w, r, &body) {
		return
	}
	v := store.Visit{
		At:       time.Now(),
		Visitor:  visitorOf(guestID(w, r)),
		Path:     pageOf(body.Path),
		Referrer: referrerOf(body.Referrer, r.Host),
		Device:   deviceOf(body.W, body.H, body.Touch),
	}
	v.OS, v.Browser = parseUA(r.UserAgent())
	if s.Geo != nil {
		if ip, ok := clientIP(r); ok {
			v.Country, v.City = s.Geo.Lookup(ip)
		}
	}
	if err := s.store.AddVisit(r.Context(), v); err != nil {
		s.internalError(w, "save visit", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// browserError records an uncaught error a page reported.
func (s *Server) browserError(w http.ResponseWriter, r *http.Request) {
	if isRobot(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body errorBody
	if !decodeBody(w, r, &body) {
		return
	}
	msg := body.Message
	if len(msg) > errorLen {
		msg = msg[:errorLen]
	}
	e := store.BrowserError{At: time.Now(), Visitor: visitorOf(guestID(w, r)), Path: pageOf(body.Path), Message: msg}
	_, e.Browser = parseUA(r.UserAgent())
	if err := s.store.AddBrowserError(r.Context(), e); err != nil {
		s.internalError(w, "save browser error", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./server -count=1`
Expected: `ok`. `TestUnknownAPIPath`-style tests in this package still pass: the new routes are POST only, so `GET /api/visit` still falls to `apiNotFound`.

- [ ] **Step 5: Commit**

```bash
git add server/server.go server/visit.go server/visit_test.go
git commit -m "server: POST /api/visit and /api/error record page views and uncaught errors"
```

---

### Task 5: The `/stats` page

**Files:**
- Create: `server/stats.go`, `server/stats.html`
- Modify: `server/server.go` (the `StatsPassword` and `StatsZone` fields, the route)
- Test: `server/stats_test.go`

**Interfaces:**
- Consumes: `store.VisitStats`, `store.FunnelStats` (Task 1), `s.static` (static.go) for the no-password case.
- Produces:
  - On `Server`: `StatsPassword string`, `StatsZone *time.Location` (nil → UTC).
  - Route `GET /stats`; `?days=7|30|90|all`, default 30.
  - `func statsRange(q string, now time.Time, loc *time.Location) (from, to time.Time, label string)`: `to` is the start of tomorrow in `loc`; `from` is `to` minus the days, or `time.Time{}` for `all`; `label` is e.g. `Sep 9 – Oct 8, 2026`.
  - `type statsPage struct` holding everything the template reads (below).

**How the page is built.** No JavaScript: every bar's width is a percentage computed in Go. The template is Graphite with one blue family, as on the canvas (sidebar, number cards, the funnel, visitors per day as stacked columns, top lists as bars). Only the sections this plan has data for are rendered: Overview, Visitors, Health. The sidebar still lists Games, Quick match and The road as plain text with "soon", so the layout matches the design.

- [ ] **Step 1: Write the failing tests**

`server/stats_test.go`:

```go
package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestStatsRange(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 10, 8, 23, 30, 0, 0, ny)
	from, to, label := statsRange("30", now, ny)
	if to != time.Date(2026, 10, 9, 0, 0, 0, 0, ny) || from != time.Date(2026, 9, 9, 0, 0, 0, 0, ny) || label != "Sep 9 – Oct 8, 2026" {
		t.Errorf("30: %v %v %q", from, to, label)
	}
	if from, _, label := statsRange("all", now, ny); !from.IsZero() || label != "All time" {
		t.Errorf("all: %v %q", from, label)
	}
	if from, _, _ := statsRange("nope", now, ny); from != time.Date(2026, 9, 9, 0, 0, 0, 0, ny) {
		t.Errorf("an unknown range isn't 30 days: %v", from)
	}
}

func TestStatsNeedsPassword(t *testing.T) {
	s, ts := newTestServer(t)
	get := func(user, pass string) (int, http.Header, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats?days=7", nil)
		if user != "" || pass != "" {
			req.SetBasicAuth(user, pass)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var sb strings.Builder
		_, _ = sb.ReadFrom(resp.Body)
		return resp.StatusCode, resp.Header, sb.String()
	}
	// No password configured: the app shell, as for any unknown path.
	if status, _, body := get("", ""); status != 200 || !strings.Contains(body, "app shell") {
		t.Errorf("without STATS_PASSWORD: %d %q", status, body)
	}
	s.StatsPassword = "hunter2"
	if status, h, _ := get("", ""); status != 401 || h.Get("WWW-Authenticate") != `Basic realm="stats"` {
		t.Errorf("no credentials: %d %q", status, h.Get("WWW-Authenticate"))
	}
	if status, _, _ := get("me", "wrong"); status != 401 {
		t.Errorf("wrong password: %d", status)
	}
	status, h, body := get("anyone", "hunter2")
	if status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") || !strings.Contains(body, "<h1>Stats</h1>") {
		t.Errorf("right password: %d %q", status, h.Get("Content-Type"))
	}
}

func TestStatsShowsTheNumbers(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	for _, v := range []store.Visit{
		{At: now.Add(-time.Hour), Visitor: "ann000000000", Path: "/", Country: "Canada", City: "Toronto", Device: "phone", OS: "iOS", Browser: "Safari"},
		{At: now.Add(-time.Minute), Visitor: "ann000000000", Path: "/game", Device: "phone", OS: "iOS", Browser: "Safari"},
	} {
		if err := s.store.AddVisit(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.AddBrowserError(ctx, store.BrowserError{At: now, Visitor: "ann000000000", Path: "/game", Message: "TypeError: <x>", Browser: "Safari"}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats", nil)
	req.SetBasicAuth("", "hunter2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sb strings.Builder
	_, _ = sb.ReadFrom(resp.Body)
	body := sb.String()
	for _, want := range []string{"Toronto", "Canada", "phone", "Safari", "TypeError: &lt;x&gt;", `class="num">1<`, "Direct or a chat app"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(body, "<x>") {
		t.Error("an error message was not escaped")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./server -run 'TestStats' -count=1`
Expected: FAIL to compile (`undefined: statsRange`).

- [ ] **Step 3: Implement**

In `server/server.go`, add after `Geo`:

```go
	// StatsPassword guards GET /stats (Basic Auth, any user name). Empty
	// means the page is off: the path serves the app shell like any other.
	StatsPassword string
	// StatsZone groups the stats page's days; nil means UTC.
	StatsZone *time.Location
```

and the route, before `s.mux.HandleFunc("/", s.static)`:

```go
	s.mux.HandleFunc("GET /stats", s.stats)
```

`server/stats.go`:

```go
package server

import (
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"mamdani-chess/store"
)

//go:embed stats.html
var statsHTML string

var statsTemplate = template.Must(template.New("stats").Funcs(template.FuncMap{
	"pct": func(n, of int) string {
		if of == 0 {
			return "0"
		}
		return fmt.Sprintf("%.1f", 100*float64(n)/float64(of))
	},
	"share": func(n, of int) int {
		if of == 0 {
			return 0
		}
		return int(100*float64(n)/float64(of) + 0.5)
	},
}).Parse(statsHTML))

// statsRange reads ?days= (7, 30, 90 or all; anything else is 30): to is
// the start of tomorrow in loc, so today counts in full.
func statsRange(q string, now time.Time, loc *time.Location) (from, to time.Time, label string) {
	y, m, d := now.In(loc).Date()
	to = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	days := 30
	switch q {
	case "7":
		days = 7
	case "90":
		days = 90
	case "all":
		return time.Time{}, to, "All time"
	}
	from = to.AddDate(0, 0, -days)
	last := to.AddDate(0, 0, -1)
	if from.Year() == last.Year() {
		label = from.Format("Jan 2") + " – " + last.Format("Jan 2, 2006")
	} else {
		label = from.Format("Jan 2, 2006") + " – " + last.Format("Jan 2, 2006")
	}
	return from, to, label
}

// statsPage is what the template reads.
type statsPage struct {
	Range, Days string
	Ranges      []statsRangeOption
	Visits      store.VisitStats
	Funnel      store.Funnel
	Steps       []funnelStep
	DayMax      int // the busiest day's visitors, for the column heights
}

type statsRangeOption struct {
	Days, Label string
	On          bool
}

type funnelStep struct {
	Label, Note string
	N           int
	Pct         int // of visited
	Drop        int // percent lost since the step before; 0 on the first
	Shade       int // 1 (darkest) to 5
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	if s.StatsPassword == "" {
		s.static(w, r)
		return
	}
	if _, pass, ok := r.BasicAuth(); !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.StatsPassword)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="stats"`)
		http.Error(w, "stats need the password", http.StatusUnauthorized)
		return
	}
	loc := s.StatsZone
	if loc == nil {
		loc = time.UTC
	}
	days := r.URL.Query().Get("days")
	from, to, label := statsRange(days, time.Now(), loc)
	visits, err := s.store.VisitStats(r.Context(), from, to, loc)
	if err != nil {
		s.internalError(w, "visit stats", err)
		return
	}
	funnel, err := s.store.FunnelStats(r.Context(), from, to)
	if err != nil {
		s.internalError(w, "funnel stats", err)
		return
	}
	page := statsPage{Range: label, Days: days, Visits: visits, Funnel: funnel}
	for _, o := range []statsRangeOption{{"7", "7 days", false}, {"30", "30 days", false}, {"90", "90 days", false}, {"all", "All", false}} {
		o.On = o.Days == days || (days != "7" && days != "90" && days != "all" && o.Days == "30")
		page.Ranges = append(page.Ranges, o)
	}
	for _, d := range visits.Days {
		page.DayMax = max(page.DayMax, d.New+d.Returning)
	}
	prev := 0
	for i, st := range []struct {
		label, note string
		n           int
	}{
		{"Visited", "", funnel.Visited},
		{"Opened a game", "quick match, a friend link, practice or watching", funnel.Opened},
		{"Made a move", "", funnel.Moved},
		{"Finished a game", "", funnel.Finished},
		{"Played again", "finished a second game, any day in the range", funnel.Again},
	} {
		step := funnelStep{Label: st.label, Note: st.note, N: st.n, Shade: i + 1}
		if funnel.Visited > 0 {
			step.Pct = int(100*float64(st.n)/float64(funnel.Visited) + 0.5)
		}
		if i > 0 && prev > 0 {
			step.Drop = int(100*float64(prev-st.n)/float64(prev) + 0.5)
		}
		prev = st.n
		page.Steps = append(page.Steps, step)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := statsTemplate.Execute(w, page); err != nil {
		s.log.Error("render stats", "err", err)
	}
}
```

`server/stats.html` (the whole file; the styles are the canvas's Graphite tokens, trimmed to what this page uses):

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Stats · Mamdani Chess</title>
<style>
:root{--bg:#f4f4f5;--card:#fff;--muted:#fafafa;--line:#e4e4e7;--text:#09090b;--text-2:#3f3f46;--text-3:#71717a;--a:#27272a;--a4:#ededf0;--c1:#033d8b;--c2:#0969da;--c3:#54aeff;--o1:#0a3069;--o2:#0550ae;--o3:#0969da;--o4:#218bff;--o5:#54aeff;--r:6px}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text-2);font:14px/1.45 "IBM Plex Sans",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
.site{display:flex;min-height:100vh}
.side{flex:0 0 232px;padding:24px 14px;border-right:1px solid var(--line);background:var(--card);display:flex;flex-direction:column;gap:28px}
.brand b{display:block;font-size:15px;color:var(--text)}.brand span{font-size:12px;color:var(--text-3)}
.nav{display:flex;flex-direction:column;gap:2px}
.nav a,.nav span{display:flex;min-height:40px;align-items:center;padding:0 10px;border-radius:4px;color:var(--text-2);text-decoration:none;font-weight:500}
.nav a:hover{background:var(--muted);color:var(--text)}.nav span{color:var(--text-3)}.nav span i{font-style:normal;font-size:11px;margin-left:auto}
.main{flex:1;min-width:0;padding:32px 40px 64px}.wrap{max-width:1120px;margin:0 auto;display:flex;flex-direction:column;gap:44px}
.top{display:flex;flex-wrap:wrap;align-items:flex-end;justify-content:space-between;gap:16px}
h1{margin:0;font-size:28px;line-height:1.2;letter-spacing:-.02em;color:var(--text)}h2{margin:0;font-size:20px;color:var(--text)}
.sub{margin:0;font-size:13px;color:var(--text-3)}
.seg{display:flex;gap:2px;padding:3px;background:var(--muted);border:1px solid var(--line);border-radius:var(--r)}
.seg a{height:38px;padding:0 14px;display:flex;align-items:center;border-radius:3px;color:var(--text-3);font-size:13px;font-weight:500;text-decoration:none}
.seg a.on{background:var(--card);color:var(--text);box-shadow:0 1px 2px rgb(0 0 0/.06),0 0 0 1px var(--line)}
section{display:flex;flex-direction:column;gap:16px}
.grid{display:grid;gap:16px;align-items:start}.g4{grid-template-columns:repeat(4,minmax(0,1fr))}.g3{grid-template-columns:repeat(3,minmax(0,1fr))}
.card{background:var(--card);border:1px solid var(--line);border-radius:var(--r);padding:20px;display:flex;flex-direction:column;gap:16px;min-width:0}
.card h3{margin:0;font-size:15px;color:var(--text)}
.kpi{padding:0;gap:0;overflow:hidden}.kpi .in{padding:18px 20px 20px;display:flex;flex-direction:column;gap:10px}.kpi .foot{padding:10px 20px;background:var(--muted);border-top:1px solid var(--line);font-size:13px;color:var(--text-3)}
.num{font:600 34px/1 "IBM Plex Mono",ui-monospace,monospace;letter-spacing:-.02em;color:var(--text)}
.rows{display:flex;flex-direction:column;gap:14px}.row{display:flex;flex-direction:column;gap:6px}
.row-top{display:flex;justify-content:space-between;gap:12px;font-size:13px}.val{font:500 13px "IBM Plex Mono",monospace;color:var(--text);white-space:nowrap}.val small{font-size:13px;color:var(--text-3)}
.track{height:6px;border-radius:999px;background:var(--a4);overflow:hidden}.track.thick{height:10px}.track i{display:block;height:100%;border-radius:999px;background:var(--a)}
.note{font-size:12px;color:var(--text-3)}
.bars{height:160px;display:flex;align-items:flex-end;gap:4px;border-bottom:1px solid var(--line)}.col{flex:1;min-width:0;height:100%;display:flex;flex-direction:column;justify-content:flex-end;gap:2px}.col i{display:block}.col i:first-child{border-radius:4px 4px 0 0}
.legend{display:flex;gap:14px;font-size:12px;color:var(--text-3)}.legend span{display:inline-flex;align-items:center;gap:6px}.dot{width:8px;height:8px;border-radius:50%}
.axis{display:flex;justify-content:space-between;font:500 11px "IBM Plex Mono",monospace;color:var(--text-3)}
.tile{background:var(--card);border:1px solid var(--line);border-radius:var(--r);padding:16px 18px;display:flex;flex-direction:column;gap:6px}.tile .num{font-size:28px}
.foot-note{border-top:1px solid var(--line);padding-top:20px;font-size:13px;color:var(--text-3)}
@media (max-width:760px){.site{flex-direction:column}.side{flex:none;padding:14px 16px 0;gap:12px;border-right:0;border-bottom:1px solid var(--line)}.nav{flex-direction:row;overflow-x:auto;gap:4px;margin:0 -16px;padding:0 16px 10px}.nav a,.nav span{flex:none;white-space:nowrap}.main{padding:20px 16px 48px}.g4{grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.g3{grid-template-columns:minmax(0,1fr)}.num{font-size:28px}.seg{width:100%}.seg a{flex:1;justify-content:center;padding:0 6px}}
</style>
</head>
<body>
<div class="site">
<aside class="side">
<div class="brand"><b>Mamdani Chess</b><span>Stats</span></div>
<nav class="nav" aria-label="Sections">
<a href="#overview">Overview</a><span>Games <i>soon</i></span><span>Quick match <i>soon</i></span><span>The road <i>soon</i></span><a href="#visitors">Visitors</a><a href="#health">Health</a>
</nav>
</aside>
<main class="main"><div class="wrap">

<div class="top">
<div><h1>Stats</h1><p class="sub">{{.Range}}</p></div>
<div class="seg" role="group" aria-label="Range">{{range .Ranges}}<a href="?days={{.Days}}"{{if .On}} class="on"{{end}}>{{.Label}}</a>{{end}}</div>
</div>

<section id="overview">
<h2>Overview</h2>
<div class="grid g4">
<div class="card kpi"><div class="in"><span>Visitors</span><span class="num">{{.Visits.Visitors}}</span></div><div class="foot">{{.Visits.Views}} page views</div></div>
<div class="card kpi"><div class="in"><span>Played a game</span><span class="num">{{.Funnel.Moved}}</span></div><div class="foot">{{share .Funnel.Moved .Funnel.Visited}}% of visitors made a move</div></div>
<div class="card kpi"><div class="in"><span>Games</span><span class="num">{{.Funnel.Games}}</span></div><div class="foot">between players</div></div>
<div class="card kpi"><div class="in"><span>Came back</span><span class="num">{{share .Visits.Returning .Visits.Visitors}}%</span></div><div class="foot">{{.Visits.Returning}} visited on 2 or more days</div></div>
</div>
<div class="card">
<div><h3>From visit to game</h3><p class="sub">Of the {{.Funnel.Visited}} visitors, how many got each step further. On the right, the share lost since the step before.</p></div>
<div class="rows">{{range .Steps}}
<div class="row"><div class="row-top"><span>{{.Label}}</span><span class="val">{{.N}} <small>{{.Pct}}%</small></span></div>
<div class="track thick"><i style="width:{{.Pct}}%;background:var(--o{{.Shade}})"></i></div>
{{if or .Note .Drop}}<div class="row-top"><span class="note">{{.Note}}</span>{{if .Drop}}<span class="note">−{{.Drop}}%</span>{{end}}</div>{{end}}</div>{{end}}
</div>
</div>
</section>

<section id="visitors">
<div><h2>Visitors</h2><p class="sub">A person counts once a day, however many pages they open</p></div>
<div class="card">
<div class="row-top"><div><h3>Visitors per day</h3><p class="sub">New and returning</p></div><div class="legend"><span><i class="dot" style="background:var(--c1)"></i>New</span><span><i class="dot" style="background:var(--c3)"></i>Returning</span></div></div>
<div class="bars">{{$max := .DayMax}}{{range .Visits.Days}}<div class="col" title="{{.Day}}: {{.New}} new, {{.Returning}} returning">{{if .Returning}}<i style="height:{{pct .Returning $max}}%;background:var(--c3)"></i>{{end}}{{if .New}}<i style="height:{{pct .New $max}}%;background:var(--c1)"></i>{{end}}</div>{{end}}</div>
<div class="axis">{{with .Visits.Days}}<span>{{(index . 0).Day}}</span><span>{{(index . (len . | sub1)).Day}}</span>{{end}}</div>
</div>
<div class="grid g3">
{{template "list" dict "Title" "Countries" "Sub" "From a local IP lookup; IPs aren't kept" "Rows" .Visits.Countries "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Cities" "Sub" "Rough on mobile data" "Rows" .Visits.Cities "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Arrived via" "Sub" "The first page of each visit" "Rows" .Visits.Sources "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Devices" "Sub" "Under 640 px on the short side is a phone" "Rows" .Visits.Devices "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Systems" "Sub" "From the browser's user agent" "Rows" .Visits.Systems "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Browsers" "Sub" "In-app browsers are where shared links open" "Rows" .Visits.Browsers "Of" .Visits.Visitors}}
{{template "list" dict "Title" "Pages" "Sub" "Page views; game codes are never stored" "Rows" .Visits.Pages "Of" .Visits.Views}}
</div>
</section>

<section id="health">
<h2>Health</h2>
<div class="grid g3">
<div class="tile"><span>Browser errors</span><span class="num">{{.Visits.Errors}}</span><span class="note">{{if .Visits.LatestError}}latest: {{.Visits.LatestError}}{{else}}none in this range{{end}}</span></div>
</div>
</section>

<p class="foot-note">Counted by this server and kept in its own database. No IP addresses are stored; cities come from a local lookup file. Headless browsers and playtests aren't counted.</p>
</div></main>
</div>
</body>
</html>

{{define "list"}}<div class="card"><div><h3>{{.Title}}</h3><p class="sub">{{.Sub}}</p></div>
<div class="rows">{{$of := .Of}}{{$top := 0}}{{range $i, $r := .Rows}}{{if eq $i 0}}{{$top = $r.N}}{{end}}
<div class="row"><div class="row-top"><span>{{$r.Label}}</span><span class="val">{{$r.N}} <small>{{share $r.N $of}}%</small></span></div><div class="track"><i style="width:{{pct $r.N $top}}%"></i></div></div>{{end}}
{{if not .Rows}}<p class="note">Nothing yet</p>{{end}}</div></div>{{end}}
```

The template uses two helpers the `FuncMap` above doesn't have. Add them to the map in `stats.go`:

```go
	"sub1": func(n int) int { return n - 1 },
	"dict": func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	},
```

`html/template` escapes `{{.Visits.LatestError}}` and every label, which is what `TestStatsShowsTheNumbers` checks. Style attributes with `{{...}}` inside are allowed for numbers; if the template parser rejects `style="width:{{.Pct}}%..."`, move the percentage into a `data-w` attribute and the width into a class per shade.

- [ ] **Step 4: Run the tests**

Run: `go test ./server -count=1 && go vet ./...`
Expected: `ok`. A template parse error panics at init and fails every test in the package: read its line number and fix the template.

- [ ] **Step 5: Look at it**

Run: `STATS_PASSWORD=x go run ./cmd/server` (Task 7 wires the variable; until then, start it from a throwaway test or set `s.StatsPassword` in a test server and print its URL). Easiest: temporarily add `handler.StatsPassword = os.Getenv("STATS_PASSWORD")` to `cmd/server/main.go` now (Task 7 keeps it), run the server with a few visits posted by `curl`, and open `http://localhost:8080/stats` at 1280 px and at 390 px wide. Check: the range buttons switch the range and keep the page; nothing overflows on the phone width; a visit from `curl -H 'X-Forwarded-For: 8.8.8.8'` shows "Unknown" (no database locally).

- [ ] **Step 6: Commit**

```bash
git add server/server.go server/stats.go server/stats.html server/stats_test.go
git commit -m "server: the private /stats page"
```

---

### Task 6: The page reports visits and errors

**Files:**
- Create: `web/src/lib/visit.ts`, `web/src/lib/visit.test.ts`
- Modify: `web/src/routes/+layout.svelte`, `web/src/routes/about/+page.svelte` (lines 74–77), `web/scripts/playtest.js` (line 160)

**Interfaces:**
- Consumes: `POST /api/visit` and `POST /api/error` (Task 4).
- Produces:
  - `export function visitPayload(url: URL, first: boolean, referrer: string, screen: { width: number; height: number }, touch: boolean): VisitPayload` with `type VisitPayload = { path: string; w: number; h: number; touch: boolean; referrer: string }`. `path` is `url.pathname`. `touch` is `navigator.maxTouchPoints > 0`. `referrer` is `ref:<tag>` when `url.searchParams.get('ref')` is set, else `referrer` on the first page of the visit, else `''`.
  - `export function reportVisit(payload: VisitPayload): void`: `navigator.sendBeacon('/api/visit', blob)` when available, else `fetch(..., { method: 'POST', keepalive: true })` with its rejection swallowed. Never throws.
  - `export function reportError(path: string, message: string): void`: same transport to `/api/error`; at most one report per 10 s so a loop can't flood.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/visit.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reportError, reportVisit, visitPayload } from './visit.ts';

const screen = { width: 390, height: 844 };

describe('visitPayload', () => {
	it('sends the path, the screen and the referrer of a first page', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/game/K7F3QZ'), true, 'https://l.instagram.com/', screen, true)).toEqual({
			path: '/game/K7F3QZ',
			w: 390,
			h: 844,
			touch: true,
			referrer: 'https://l.instagram.com/'
		});
	});

	it('sends no referrer after the first page', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/rules'), false, 'https://mamdanichess.com/', screen, false).referrer).toBe('');
	});

	it('turns a ?ref= tag into ref:<tag>', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/?ref=ig'), true, '', screen, false).referrer).toBe('ref:ig');
	});
});

describe('reportVisit', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('uses sendBeacon when there is one', () => {
		const sendBeacon = vi.fn(() => true);
		vi.stubGlobal('navigator', { sendBeacon });
		vi.stubGlobal('fetch', vi.fn());
		reportVisit({ path: '/', w: 1, h: 1, touch: false, referrer: '' });
		expect(sendBeacon).toHaveBeenCalledTimes(1);
		const [url, body] = sendBeacon.mock.calls[0] as unknown as [string, Blob];
		expect(url).toBe('/api/visit');
		expect(body.type).toBe('application/json');
		expect(fetch).not.toHaveBeenCalled();
	});

	it('falls back to fetch and swallows a failure', async () => {
		vi.stubGlobal('navigator', {});
		const fetch = vi.fn(() => Promise.reject(new Error('down')));
		vi.stubGlobal('fetch', fetch);
		expect(() => reportVisit({ path: '/', w: 1, h: 1, touch: false, referrer: '' })).not.toThrow();
		await Promise.resolve();
		expect(fetch).toHaveBeenCalledWith('/api/visit', expect.objectContaining({ method: 'POST', keepalive: true }));
	});
});

describe('reportError', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('sends at most one report every 10 seconds', () => {
		const sendBeacon = vi.fn(() => true);
		vi.stubGlobal('navigator', { sendBeacon });
		reportError('/game/K7F3QZ', 'TypeError: x');
		reportError('/game/K7F3QZ', 'TypeError: y');
		expect(sendBeacon).toHaveBeenCalledTimes(1);
		vi.advanceTimersByTime(10_001);
		reportError('/game/K7F3QZ', 'TypeError: z');
		expect(sendBeacon).toHaveBeenCalledTimes(2);
	});
});
```

- [ ] **Step 2: Run them to see them fail**

Run: `pnpm --dir web test -- visit`
Expected: FAIL (`Failed to resolve import "./visit.ts"`).

- [ ] **Step 3: Implement**

`web/src/lib/visit.ts`:

```ts
// What the page tells the server about a visit: the route it's on, the
// screen and whether it's a touch screen (for phone/tablet/desktop) and, on
// the first page, where the visitor came from. The server keeps the route, never a game code, and the
// referrer's host, never its path.

export type VisitPayload = { path: string; w: number; h: number; touch: boolean; referrer: string };

export function visitPayload(
	url: URL,
	first: boolean,
	referrer: string,
	screen: { width: number; height: number },
	touch: boolean
): VisitPayload {
	const tag = url.searchParams.get('ref');
	return {
		path: url.pathname,
		w: screen.width,
		h: screen.height,
		touch,
		referrer: tag ? `ref:${tag}` : first ? referrer : ''
	};
}

/** Sends a page view. Never throws and never surfaces a failure: a visit is not worth a toast. */
export function reportVisit(payload: VisitPayload): void {
	send('/api/visit', payload);
}

const ERROR_GAP_MS = 10_000;
let lastError = -Infinity;

/** Reports an uncaught error, at most once every 10 s, so a loop can't flood the server. */
export function reportError(path: string, message: string): void {
	const now = Date.now();
	if (now - lastError < ERROR_GAP_MS) return;
	lastError = now;
	send('/api/error', { path, message: message.slice(0, 300) });
}

function send(path: string, body: unknown): void {
	const json = JSON.stringify(body);
	try {
		if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
			if (navigator.sendBeacon(path, new Blob([json], { type: 'application/json' }))) return;
		}
		fetch(path, { method: 'POST', headers: { 'content-type': 'application/json' }, body: json, keepalive: true }).catch(
			() => {}
		);
	} catch {
		// A blocked beacon or a page that is going away: nothing to do.
	}
}
```

`web/src/routes/+layout.svelte` becomes:

```svelte
<script lang="ts">
	import '#lib/theme/tokens.css';
	import Toaster from '#lib/Toaster.svelte';
	import { reportError, reportVisit, visitPayload } from '#lib/visit.ts';
	import { afterNavigate } from '$app/navigation';
	import { onMount } from 'svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	// One visit per page the person lands on; the first carries the referrer.
	let first = true;
	afterNavigate(({ to }) => {
		if (!to) return;
		reportVisit(visitPayload(to.url, first, document.referrer, window.screen, navigator.maxTouchPoints > 0));
		first = false;
	});

	onMount(() => {
		const onError = (e: ErrorEvent) => reportError(location.pathname, String(e.message ?? e.error ?? 'error'));
		const onRejection = (e: PromiseRejectionEvent) => reportError(location.pathname, String(e.reason ?? 'rejection'));
		window.addEventListener('error', onError);
		window.addEventListener('unhandledrejection', onRejection);
		return () => {
			window.removeEventListener('error', onError);
			window.removeEventListener('unhandledrejection', onRejection);
		};
	});
</script>

{@render children()}
<Toaster />
```

Then run `npx @sveltejs/mcp svelte-autofixer web/src/routes/+layout.svelte` and apply what it says.

In `web/src/routes/about/+page.svelte`, replace lines 74–77

```svelte
				<li>
					<strong>No ads, no analytics.</strong> Our logs record games and requests with a shortened guest ID, not your IP
					address.
				</li>
```

with

```svelte
				<li>
					<strong>No ads, no tracking companies.</strong> We count visits ourselves, on this server: which page, your
					rough city, and your device and browser, so we know how many people play and whether the game works. Your IP
					address is used once to find the city and isn't kept, game links are counted without their code, and nothing
					goes to anyone else. City lookups use a local copy of
					<a href="https://db-ip.com" target="_blank" rel="noopener">IP geolocation by DB-IP</a>.
				</li>
```

In `web/scripts/playtest.js`, line 160, mark every playtest request so the server never counts it:

```js
	const device = opts.phone ? { ...devices['iPhone 15'], defaultBrowserType: undefined } : {};
	const mark = { ...device, extraHTTPHeaders: { 'X-Playtest': '1' } };
	const contexts = [await browser.newContext(mark), await browser.newContext(mark)];
```

That is the only `browser.newContext(` in the file (the `--match` path reuses these two contexts), so every playtest request carries the header.

- [ ] **Step 4: Run the tests**

Run: `pnpm --dir web test && pnpm --dir web check && pnpm --dir web build`
Expected: Vitest passes with the new tests; `0 errors`; the build succeeds. Restart `pnpm --dir web dev` afterwards (`check` can hang it).

- [ ] **Step 5: See a visit arrive**

With `go run ./cmd/server` and `pnpm --dir web dev` running, open http://localhost:5173/?ref=test, then /rules. In the Go server's log, two `POST /api/visit` lines at DEBUG (`LOG_FORMAT` text shows them). In `sqlite3 data/mamdani.db 'select * from visits order by at desc limit 2'`: paths `/rules` and `/`, referrer `ref:test` on the first, device from your screen.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/visit.ts web/src/lib/visit.test.ts web/src/routes/+layout.svelte web/src/routes/about/+page.svelte web/scripts/playtest.js
git commit -m "web: report page views and uncaught errors; say so on About"
```

---

### Task 7: Wiring, the image, contract checks and the docs

**Files:**
- Modify: `cmd/server/main.go`, `Dockerfile`, `contract/main_test.go` (line 111), `contract/api_test.go`, `CLAUDE.md`, `docs/superpowers/plans/2026-10-03-00-roadmap.md`

**Interfaces:**
- Consumes: `server.OpenGeo` (Task 3), `Server.Geo`, `Server.StatsPassword`, `Server.StatsZone` (Tasks 4–5), `store.PruneVisits` (Task 1).
- Produces: env vars `GEOIP_PATH`, `STATS_PASSWORD`, `STATS_TZ` (default `America/New_York`); `visitsKeep = 400 * 24 * time.Hour`.

- [ ] **Step 1: Write the failing contract tests**

In `contract/main_test.go` line 111, add the password so the page can be checked:

```go
	s.cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "DB_PATH="+filepath.Join(dir, "contract.db"), "STATS_PASSWORD=contract")
```

Append to `contract/api_test.go`:

```go
// A page view is a 204 that gives a new visitor the guest cookie; robots
// get the 204 and nothing else.
func TestVisit(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	res := c.do(http.MethodPost, "/api/visit", http.Header{"Content-Type": {"application/json"}})
	// c.do sends no body: a bad body is a 400 in the API's usual shape.
	expectJSON(t, []any{res.status, res.body}, `[400,"{\"error\":\"bad request body\"}\n"]`)
	p := c.players(1)[0]
	status, body := p.post("/api/visit", `{"path":"/game/ABCDEF","w":390,"h":844,"touch":true,"referrer":""}`)
	if status != 204 || body != "" {
		t.Errorf("POST /api/visit: %d %q", status, body)
	}
	if status, _ := p.post("/api/error", `{"path":"/","message":"TypeError: x"}`); status != 204 {
		t.Errorf("POST /api/error: %d", status)
	}
	if status := c.do(http.MethodGet, "/api/visit", nil).status; status != 404 {
		t.Errorf("GET /api/visit: %d, want 404", status)
	}
}

// /stats is Basic Auth with STATS_PASSWORD, and reads as HTML.
func TestStatsPage(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	if os.Getenv("BASE_URL") != "" && os.Getenv("STATS_PASSWORD") == "" {
		t.Skip("BASE_URL without STATS_PASSWORD: can't check the page on a running server")
	}
	res := c.do(http.MethodGet, "/stats", nil)
	expectJSON(t, []any{res.status, header(res, "WWW-Authenticate")}, `[401,"Basic realm=\"stats\""]`)
	pass := os.Getenv("STATS_PASSWORD")
	if pass == "" {
		pass = "contract"
	}
	res = c.do(http.MethodGet, "/stats?days=7", http.Header{"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pass))}})
	if res.status != 200 || !strings.Contains(res.body, "<h1>Stats</h1>") || !strings.Contains(res.body, "7 days") {
		t.Errorf("GET /stats: %d, HTML %v", res.status, strings.Contains(res.body, "<h1>Stats</h1>"))
	}
}
```

Add `"encoding/base64"` and `"os"` to the imports. (`base` in `contract/main_test.go` is always set, to the started server's URL or `BASE_URL`, so the skip reads the variable itself.)

- [ ] **Step 2: Run them to see them fail**

Run: `pnpm --dir web build && CONTRACT=1 go test ./contract -run 'TestVisit|TestStatsPage' -count=1`
Expected: `TestStatsPage` FAILS (`[200,null]`: the password isn't wired, so the path serves the shell). `TestVisit` passes already.

- [ ] **Step 3: Implement**

`cmd/server/main.go`: add `_ "time/tzdata"` to the imports (the distroless image has no zone files), and after `handler.Version = ...`:

```go
	// The stats page (plan: docs/superpowers/plans/2026-10-08-stats-1-visits.md).
	handler.StatsPassword = os.Getenv("STATS_PASSWORD")
	zone := envOr("STATS_TZ", "America/New_York")
	if handler.StatsZone, err = time.LoadLocation(zone); err != nil {
		return fmt.Errorf("STATS_TZ %q: %w", zone, err)
	}
	if handler.Geo, err = server.OpenGeo(os.Getenv("GEOIP_PATH")); err != nil {
		return fmt.Errorf("GEOIP_PATH: %w", err)
	}
	if n, err := st.PruneVisits(context.Background(), time.Now().Add(-visitsKeep)); err != nil {
		return fmt.Errorf("prune visits: %w", err)
	} else if n > 0 {
		slog.Info("old visits pruned", "rows", n)
	}
```

and the constant near the top of the file:

```go
// visitsKeep is how long page views are kept: about thirteen months, so a
// year can be compared with the year before.
const visitsKeep = 400 * 24 * time.Hour
```

`Dockerfile`: add a stage before the final one, and copy its file in:

```dockerfile
# 3. DB-IP's free City Lite database (CC BY 4.0; credited on /about): this
#    month's file, or last month's early in a month. Railway's build cache
#    keeps the layer, so the file refreshes when the cache does.
FROM alpine:3.21 AS geo
RUN apk add --no-cache curl
RUN set -e; for m in $(date +%Y-%m) $(date -d @$(( $(date +%s) - 20*86400 )) +%Y-%m); do \
      curl -fsSL -o /dbip.mmdb.gz "https://download.db-ip.com/free/dbip-city-lite-$m.mmdb.gz" && break; done; \
    gunzip /dbip.mmdb.gz && ls -l /dbip.mmdb
```

Renumber the run stage to 4, and in it:

```dockerfile
COPY --from=geo /dbip.mmdb /geo/dbip-city-lite.mmdb
ENV PORT=8080 DB_PATH=/data/mamdani.db LOG_FORMAT=json GEOIP_PATH=/geo/dbip-city-lite.mmdb
```

- [ ] **Step 4: Run every check**

```bash
go vet ./... && go test -race -short ./...
pnpm --dir web check && pnpm --dir web test && pnpm --dir web build
CONTRACT=1 go test ./contract -count=1
docker build -t mamdani-chess .
```

Expected: every Go package `ok`; `0 errors`; the contract run passes including `TestVisit` and `TestStatsPage`; the image builds and `ls -l /dbip.mmdb` in the build log shows a file of roughly 130 MB. Then the playtest, with the Go server and the dev server running: `pnpm --dir web playtest --games 2` and `pnpm --dir web playtest --phone`, both clean, and afterwards `sqlite3 data/mamdani.db 'select count(*) from visits'` has not grown (the header kept the playtest out).

- [ ] **Step 5: Docs**

`CLAUDE.md`:
- Under **Where things stand**, add: "**Done: stats, plan 1** ([spec](docs/superpowers/specs/2026-10-08-stats-design.md), [plan](docs/superpowers/plans/2026-10-08-stats-1-visits.md)): the page reports every page view and uncaught error to `POST /api/visit` and `/api/error`; `visits` and `browser_errors` in SQLite (12 hex of the guest ID, the route, referrer host, country and city from a local DB-IP file, device from the screen, OS and browser from the user agent), pruned after 400 days; `/stats` (Basic Auth, `STATS_PASSWORD`) shows Overview, Visitors and Health. Plan 2 (game stats, quick-match log) is next."
- Under **Working notes**, add: "**Stats:** `GET /stats` with the password from `STATS_PASSWORD` (any user name); off when the variable is unset. `GEOIP_PATH` names the MMDB file (the image has it at `/geo/dbip-city-lite.mmdb`; `go run` has none, so cities are Unknown); `STATS_TZ` groups the days (default `America/New_York`). Playtests and headless browsers aren't counted (`X-Playtest` header, `HeadlessChrome`)."
- In **Decisions that are settled**, add the bullet: "Metrics are first party: our own `visits` table and `/stats` page, no analytics service, no IP addresses stored."
- The about-page line in **Decisions** and anywhere else that says "No ads, no analytics" is reworded to "no tracking companies".

`docs/superpowers/plans/2026-10-03-00-roadmap.md`: in row 07's "What" column, after "launch logging (see below)", add "first-party stats ([spec](../specs/2026-10-08-stats-design.md); plan 1 done, plan 2 next)".

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go Dockerfile contract/main_test.go contract/api_test.go CLAUDE.md docs/superpowers/plans/2026-10-03-00-roadmap.md
git commit -m "server, deploy: wire the stats page, prune old visits, ship the DB-IP file"
```

- [ ] **Step 7: Whole-branch review, then hand over**

Run the whole-branch review (superpowers:requesting-code-review), fix what it finds, then superpowers:finishing-a-development-branch. **Don't push to `main` without asking**: a deploy pauses every game in progress. Before the merge, the user sets `STATS_PASSWORD` on the Railway `web` service (Railway dashboard → service → Variables), or asks for it to be set through the CLI. After the deploy: `curl https://mamdanichess.com/healthz` shows the new commit; a visit from a phone and a laptop; `https://mamdanichess.com/stats` shows two visitors with a city (that confirms which header Railway passes the client IP in: if cities stay Unknown, log `clientIP` once at DEBUG and look).

## Notes for the executor

- Ports: the Go server on :8080, Vite on :5173, a second test server with `PORT=8091 DB_PATH=/tmp/stats-check.db go run ./cmd/server`. Stop a test server by its PID or port, never with a broad `pkill`.
- `pnpm --dir web check` can hang a running `pnpm --dir web dev`; restart it after.
- The store opens one connection (`SetMaxOpenConns(1)`): close every `rows` before the next query, as `VisitStats` does.
- The `ERROR_GAP_MS` throttle in `visit.ts` is module state: Vitest runs each file in its own module instance, so the tests don't bleed.
- `window.screen` has `width` and `height` in CSS px, orientation-independent on phones; the server uses the short side, so landscape and portrait agree.
