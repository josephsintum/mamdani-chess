package server

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"mamdani-chess/store"
)

// isRobot reports a request that isn't a person: headless Chrome, the
// playtest's X-Playtest header, or a crawler (bot followed by / . or -,
// crawler, spider; a CUBOT phone is a person). The stats count nothing a
// robot does.
func isRobot(r *http.Request) bool {
	ua := r.UserAgent()
	lower := strings.ToLower(ua)
	return strings.Contains(ua, "HeadlessChrome") || r.Header.Get("X-Playtest") != "" ||
		botName.MatchString(lower) || strings.Contains(lower, "crawler") || strings.Contains(lower, "spider")
}

var botName = regexp.MustCompile(`bot[/.-]`)

// countTimeout bounds a counted write. The writes don't use the request's
// context: a search is logged once its stream has ended.
const countTimeout = 5 * time.Second

// logSearch writes one quick-match search. A failure is logged, never
// shown: the search itself has already happened.
func (s *Server) logSearch(started time.Time, matched bool, others int) {
	ctx, cancel := context.WithTimeout(context.Background(), countTimeout)
	defer cancel()
	if err := s.store.AddSearch(ctx, store.Search{StartedAt: started, EndedAt: time.Now(), Matched: matched, Others: others}); err != nil {
		s.log.Error("log search", "err", err)
	}
}

// logEvent counts one event (a practice game, a reopened game stream) that
// r caused, unless a playtest sent r. Failures are logged, never shown.
func (s *Server) logEvent(r *http.Request, kind string) {
	if isRobot(r) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), countTimeout)
	defer cancel()
	if err := s.store.AddEvent(ctx, kind, time.Now()); err != nil {
		s.log.Error("log event", "kind", kind, "err", err)
	}
}
