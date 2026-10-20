package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"mamdani-chess/store"
)

// pageOf reduces a path to its route (a game, or one of pages), so a game
// code is never stored; anything else is "other".
func pageOf(path string) string {
	path, _, _ = strings.Cut(path, "?")
	if path == "" {
		return "other"
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	route := "/" + first
	if _, ok := pages[route]; ok || route == "/game" {
		return route
	}
	return "other"
}

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
