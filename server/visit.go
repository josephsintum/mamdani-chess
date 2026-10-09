package server

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mamdani-chess/store"
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

// hostLen is the longest a host name can be.
const hostLen = 253

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
	if h, _, err := net.SplitHostPort(ourHost); err == nil {
		ourHost = h
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == strings.TrimPrefix(strings.ToLower(ourHost), "www.") {
		return ""
	}
	if len(host) > hostLen {
		host = host[:hostLen]
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
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		raw = realIP
	} else if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		raw = host
	} else {
		raw = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(raw))
	return ip, err == nil
}

// isRobot reports a visit that isn't a person: headless Chrome (the
// playtest, agent-browser), a crawler that runs scripts (Googlebot and the
// like name themselves bot, crawler or spider) or any request the
// playtest marks.
func isRobot(r *http.Request) bool {
	ua := r.UserAgent()
	lower := strings.ToLower(ua)
	return strings.Contains(ua, "HeadlessChrome") || r.Header.Get("X-Playtest") != "" ||
		strings.Contains(lower, "bot") || strings.Contains(lower, "crawler") || strings.Contains(lower, "spider")
}

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

// gameCode finds a game code in a page's or an API's URL, so an error
// message (a URL in a stack, say) is stored without it.
var gameCode = regexp.MustCompile(`(/games?)/[A-Z0-9]{6}`)

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
	msg := gameCode.ReplaceAllString(body.Message, "$1")
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
