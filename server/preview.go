package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// preview is what a shared link shows in a chat app: iMessage, WhatsApp,
// Slack and the like read the page's tags without running its script, so
// the server writes them.
type preview struct {
	Title, Description string
}

var homePreview = preview{
	Title:       "Mamdani Chess",
	Description: "Chess where potholes open under your pieces. Play a friend or a stranger, no sign-up.",
}

// siteHost is the site's one address. Search engines list its pages there,
// whichever host served them, and its aliases redirect to it.
const siteHost = "mamdanichess.com"

// aliases are the other hosts the Railway service answers on. Railway has
// no redirect setting, so the server redirects their pages.
var aliases = map[string]bool{"www." + siteHost: true, "mamdani-chess.up.railway.app": true}

// shell is the SPA's fallback page, for every route that isn't prerendered.
const shell = "200.html"

// route is one of the app's routes other than a game's.
type route struct {
	preview        // what a link to it shows
	file    string // its prerendered HTML (its +page.ts), or "" for the shell
}

// pages are the app's routes other than a game's. Only the prerendered
// ones are listed by search engines (and in web/static/sitemap.xml).
var pages = map[string]route{
	"/":            {homePreview, "index.html"},
	"/play":        {preview{Title: "Quick match · Mamdani Chess", Description: "Get paired with the next player looking for a game."}, ""},
	"/how-to-play": {preview{Title: "How to play · Mamdani Chess", Description: "Potholes, dice and the Mamdani, played out on the board."}, "how-to-play.html"},
	"/rules":       {preview{Title: "Rules · Mamdani Chess", Description: "Every rule of Mamdani Chess: potholes, the Mamdani and saving rolls."}, "rules.html"},
	"/practice":    {preview{Title: "Practice · Mamdani Chess", Description: "Play both sides and try the potholes out on your own."}, ""},
	"/about":       {preview{Title: "About · Mamdani Chess", Description: "An unofficial, ad-free fan project: credits and what the site keeps."}, "about.html"},
}

// isPageFile reports whether name is one of the build's page files, which
// are only served through page, with their tags.
func isPageFile(name string) bool {
	if name == shell {
		return true
	}
	for _, pg := range pages {
		if pg.file != "" && pg.file == name {
			return true
		}
	}
	return false
}

// gameRoute is the code in a game's path, /game/CODE.
func gameRoute(p string) (string, bool) {
	code, ok := strings.CutPrefix(p, "/game/")
	return code, ok && code != "" && !strings.Contains(code, "/")
}

// page serves the HTML for r's route with the preview tags for it. The
// content pages are prerendered, so search engines read them without
// running a script, and only they are listed; every other route gets the
// shell. A path that is no route is a 404. It sets the guest cookie on a
// fresh browser, so the page's first requests (the name lookup and a
// game's stream, which race) carry the same one instead of each minting
// its own.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	if aliases[strings.ToLower(r.Host)] {
		// One address for search engines, bookmarks and the guest cookie
		// (which belongs to one host). Only pages move: a tab opened on the
		// alias still loads its scripts and the API there until it reloads.
		http.Redirect(w, r, "https://"+siteHost+r.URL.RequestURI(), http.StatusMovedPermanently)
		return
	}
	p := r.URL.Path
	pg, known := pages[path.Clean(p)]
	_, isGame := gameRoute(path.Clean(p))
	if c := path.Clean(p); c != p && (known || isGame) {
		// A prerendered page loads its files by relative paths, which a
		// trailing slash breaks. The target starts with a known route, so
		// it can't lead off the site.
		target := c
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}
	cookieValue(w, r)
	listed := pg.file != ""
	var b []byte
	var err error
	if listed {
		b, err = fs.ReadFile(s.assets, pg.file)
	}
	if !listed || err != nil {
		listed = false
		b, err = fs.ReadFile(s.assets, shell)
	}
	if err != nil {
		http.Error(w, "app not built", http.StatusInternalServerError)
		return
	}
	status := http.StatusOK
	if !known && !isGame {
		status = http.StatusNotFound
	}
	head := len(b)
	if i := bytes.Index(b, []byte("</head>")); i >= 0 {
		head = i
	}
	// A prerendered page has its own <title>, from its <svelte:head>.
	title := !bytes.Contains(b[:head], []byte("<title>"))
	tags := []byte(previewTags(s.preview(p), baseURL(r), p, title) + seoTags(p, listed))
	b = slices.Concat(b[:head], tags, b[head:])
	w.Header().Set("Cache-Control", "private, no-cache") // it can set the guest cookie: never in a shared cache
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Vary", "Accept-Encoding")
	if accepts(r.Header.Get("Accept-Encoding"), "gzip") {
		// The tags differ per request, so the build's precompressed copies
		// can't be used; a prerendered page is up to 160 KB.
		var z bytes.Buffer
		zw := gzip.NewWriter(&z)
		zw.Write(b)
		zw.Close()
		b = z.Bytes()
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		w.Write(b)
	}
}

// seoTags tells search engines what to do with the page at p: the home
// page names the site, and the shell's pages (games, quick match,
// practice, a 404) stay out of search results. A listed page's canonical
// link is in its <svelte:head>, so it goes away when the app navigates on.
func seoTags(p string, listed bool) string {
	if !listed {
		return "<meta name=\"robots\" content=\"noindex\" />\n"
	}
	if p == "/" {
		return `<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"Mamdani Chess","url":"https://` + siteHost + `/"}</script>` + "\n"
	}
	return ""
}

// preview describes the page at path. A game link reads the game's last
// published state, so it never waits on the game.
func (s *Server) preview(path string) preview {
	if pg, ok := pages[path]; ok {
		return pg.preview
	}
	code, ok := gameRoute(path)
	if !ok {
		return homePreview
	}
	g, ok := s.games.Get(code)
	if !ok {
		return homePreview
	}
	return gamePreview(g.Summary())
}

func gamePreview(sum game.Summary) preview {
	white, black := orColor(sum.White, "White"), orColor(sum.Black, "Black")
	switch sum.Status {
	case game.Waiting:
		title := "You're invited to Mamdani Chess"
		if sum.White != "" {
			title = sum.White + " invites you to Mamdani Chess"
		}
		return preview{Title: title, Description: "Tap to take Black. 10+5, and potholes open under the pieces."}
	case game.Playing:
		return preview{Title: white + " vs " + black + " · Mamdani Chess", Description: fmt.Sprintf("Watch live, move %d.", sum.Plies/2+1)}
	}
	return preview{Title: white + " vs " + black + " · Mamdani Chess", Description: resultLine(sum, white, black)}
}

// resultLine says how a finished game ended, e.g. "uws-bialy won by
// checkmate after 31 moves."
func resultLine(sum game.Summary, white, black string) string {
	r := sum.Result
	if r == nil {
		return "The game is over."
	}
	after := "."
	if moves := (sum.Plies + 1) / 2; moves > 0 {
		after = fmt.Sprintf(" after %d moves.", moves)
	}
	switch {
	case r.Reason == game.Aborted:
		return "Aborted: the first move never came."
	case r.Reason == game.Expired:
		return "Nobody took the Black seat."
	case r.Reason == game.TimeoutVsInsufficient:
		return "Drawn: time ran out, but no mate was possible" + after
	case r.Winner != "":
		winner := white
		if r.Winner == "black" {
			winner = black
		}
		return winner + " won by " + reasonWords(r.Reason) + after
	}
	return "Drawn by " + reasonWords(r.Reason) + after
}

// reasonWords is how a result line says reason, e.g. "the fifty-move rule".
func reasonWords(reason rules.Reason) string {
	if words, ok := reasons[reason]; ok {
		return words
	}
	return string(reason)
}

var reasons = map[rules.Reason]string{
	rules.Checkmate: "checkmate", game.Resignation: "resignation", game.Timeout: "time",
	rules.Stalemate: "stalemate", rules.Repetition: "repetition",
	rules.FiftyMoves: "the fifty-move rule", rules.InsufficientMaterial: "insufficient material",
}

func orColor(name, color string) string {
	if name == "" {
		return color
	}
	return name
}

// baseURL is the site's address as the caller reached it.
func baseURL(r *http.Request) string {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// previewTags is the <title> (if title) and Open Graph tags for p, for the
// page at path on base.
func previewTags(p preview, base, path string, title bool) string {
	esc := html.EscapeString
	var b strings.Builder
	if title {
		fmt.Fprintf(&b, "<title>%s</title>\n", esc(p.Title))
	}
	tag := func(attr, key, value string) {
		fmt.Fprintf(&b, "<meta %s=\"%s\" content=\"%s\" />\n", attr, key, esc(value))
	}
	tag("name", "description", p.Description)
	tag("property", "og:type", "website")
	tag("property", "og:site_name", "Mamdani Chess")
	tag("property", "og:title", p.Title)
	tag("property", "og:description", p.Description)
	tag("property", "og:url", base+path)
	tag("property", "og:image", base+"/og.png")
	tag("property", "og:image:width", "1200")
	tag("property", "og:image:height", "630")
	tag("property", "og:image:alt", "Mamdani Chess: a corner of the board with an open pothole and traffic cones")
	tag("name", "twitter:card", "summary_large_image")
	return b.String()
}
