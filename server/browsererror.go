package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"mamdani-chess/store"
)

// pages are the routes an error report can name; anything else is "other".
var pages = map[string]bool{"/": true, "/game": true, "/play": true, "/practice": true, "/how-to-play": true, "/rules": true, "/about": true}

// pageOf reduces a path to its route, so a game code is never stored.
func pageOf(path string) string {
	path, _, _ = strings.Cut(path, "?")
	if path == "" {
		return "other"
	}
	if path == "/" {
		return "/"
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if route := "/" + first; pages[route] {
		return route
	}
	return "other"
}

// isRobot reports a request that isn't a person: headless Chrome (the
// playtest, agent-browser), a crawler that runs scripts (Googlebot and the
// like name themselves crawler, spider, or bot followed by a version or a
// name: Googlebot/2.1, Slackbot-LinkExpanding) or any request the
// playtest marks. A phone model with bot in its name (CUBOT KINGKONG 7)
// is a person. The stats count none of what a robot does: its games
// (marked playtest), quick-match searches, events and errors.
func isRobot(r *http.Request) bool {
	ua := r.UserAgent()
	lower := strings.ToLower(ua)
	return strings.Contains(ua, "HeadlessChrome") || r.Header.Get("X-Playtest") != "" ||
		botName.MatchString(lower) || strings.Contains(lower, "crawler") || strings.Contains(lower, "spider")
}

var botName = regexp.MustCompile(`bot[/.-]`)

type errorBody struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// errorLen caps a reported message.
const errorLen = 300

// gameCode finds a game code in a page's or an API's URL, so an error
// message (a URL in a stack, say) is stored without it.
var gameCode = regexp.MustCompile(`(/games?)/[A-Z0-9]{6}`)

// browserError records an uncaught error a page reported: the route and
// the message, nothing about who saw it. Robots get a 204 and no row.
func (s *Server) browserError(w http.ResponseWriter, r *http.Request) {
	if isRobot(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body errorBody
	if !decodeBody(w, r, &body) {
		return
	}
	msg := gameCode.ReplaceAllString(body.Message, "$1")
	if len(msg) > errorLen {
		msg = msg[:errorLen]
	}
	e := store.BrowserError{At: time.Now(), Path: pageOf(body.Path), Message: msg}
	if err := s.store.AddBrowserError(r.Context(), e); err != nil {
		s.internalError(w, "save browser error", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
