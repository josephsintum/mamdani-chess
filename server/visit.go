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
// playtest, agent-browser) or any request the playtest marks.
func isRobot(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "HeadlessChrome") || r.Header.Get("X-Playtest") != ""
}
