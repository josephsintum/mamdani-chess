package server

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"

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

// index serves the app shell with the preview tags for r's page. It sets
// the guest cookie on a fresh browser, so the page's first requests (the
// visit beacon and a game's stream, which race) carry the same one instead
// of each minting its own.
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	cookieValue(w, r)
	b, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		http.Error(w, "app not built", http.StatusInternalServerError)
		return
	}
	tags := []byte(previewTags(s.preview(r.URL.Path), baseURL(r), r.URL.Path))
	if i := bytes.Index(b, []byte("</head>")); i >= 0 {
		b = slices.Concat(b[:i], tags, b[i:])
	} else {
		b = append(tags, b...)
	}
	w.Header().Set("Cache-Control", "private, no-cache") // it can set the guest cookie: never in a shared cache
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
}

// preview describes the page at path. A game link reads the game's last
// published state, so it never waits on the game.
func (s *Server) preview(path string) preview {
	switch path {
	case "/play":
		return preview{Title: "Quick match · Mamdani Chess", Description: "Get paired with the next player looking for a game."}
	case "/how-to-play":
		return preview{Title: "How to play · Mamdani Chess", Description: "Potholes, dice and the Mamdani, played out on the board."}
	case "/rules":
		return preview{Title: "Rules · Mamdani Chess", Description: "Every rule of Mamdani Chess: potholes, the Mamdani and saving rolls."}
	case "/practice":
		return preview{Title: "Practice · Mamdani Chess", Description: "Play both sides and try the potholes out on your own."}
	case "/about":
		return preview{Title: "About · Mamdani Chess", Description: "An unofficial, ad-free fan project: credits and what the site keeps."}
	}
	code, ok := strings.CutPrefix(path, "/game/")
	if !ok || strings.Contains(code, "/") {
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

// previewTags is the <title> and Open Graph tags for p, for the page at
// path on base.
func previewTags(p preview, base, path string) string {
	esc := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, "<title>%s</title>\n", esc(p.Title))
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
