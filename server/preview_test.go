package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// page fetches path as a browser or a link preview would, and returns the
// HTML.
func page(t *testing.T, c *http.Client, url string, header ...string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("%s: status %d, type %q", url, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// meta returns the content of the tag with the given property or name.
func meta(html, key string) string {
	for _, attr := range []string{`property="` + key + `"`, `name="` + key + `"`} {
		if i := strings.Index(html, attr); i >= 0 {
			rest := html[i:]
			start := strings.Index(rest, `content="`) + len(`content="`)
			end := strings.Index(rest[start:], `"`)
			return rest[start : start+end]
		}
	}
	return ""
}

// A shared link shows a title, a line of text and the card image in
// iMessage, WhatsApp, Slack and the like, which read the page's tags
// without running its script.
func TestSharedLinksShowAPreview(t *testing.T) {
	_, ts := newTestServer(t)
	c := ts.Client()

	home := page(t, c, ts.URL+"/")
	if got := meta(home, "og:title"); got != "Mamdani Chess" {
		t.Errorf("home og:title %q", got)
	}
	if !strings.Contains(home, "<title>Mamdani Chess</title>") || !strings.Contains(home, "app shell") {
		t.Errorf("home page: %s", home)
	}
	if got := meta(home, "og:image"); got != ts.URL+"/og.png" {
		t.Errorf("og:image %q, want %s/og.png", got, ts.URL)
	}
	if meta(home, "twitter:card") != "summary_large_image" || meta(home, "og:image:width") != "1200" {
		t.Error("missing the large card tags")
	}
	for path, want := range map[string]string{
		"/play":        "Quick match · Mamdani Chess",
		"/how-to-play": "How to play · Mamdani Chess",
		"/rules":       "Rules · Mamdani Chess",
		"/practice":    "Practice · Mamdani Chess",
		"/about":       "About · Mamdani Chess",
	} {
		if got := meta(page(t, c, ts.URL+path), "og:title"); got != want {
			t.Errorf("%s og:title %q, want %q", path, got, want)
		}
	}
	if got := meta(page(t, c, ts.URL+"/game/ZZZZZZ"), "og:title"); got != "Mamdani Chess" {
		t.Errorf("unknown game og:title %q, want the home page's", got)
	}

	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	link := ts.URL + "/game/" + code
	a := alice.name()
	waiting := page(t, c, link)
	if got := meta(waiting, "og:title"); got != a+" invites you to Mamdani Chess" {
		t.Errorf("waiting og:title %q", got)
	}
	if got := meta(waiting, "og:description"); !strings.Contains(got, "take Black") {
		t.Errorf("waiting og:description %q", got)
	}
	if got := meta(waiting, "og:url"); got != link {
		t.Errorf("og:url %q, want %s", got, link)
	}

	bob.stream(code).state() // takes Black
	b := bob.name()
	playing := page(t, c, link)
	if got := meta(playing, "og:title"); got != a+" vs "+b+" · Mamdani Chess" {
		t.Errorf("playing og:title %q", got)
	}
	if got := meta(playing, "og:description"); got != "Watch live, move 1." {
		t.Errorf("playing og:description %q", got)
	}

	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	if got := meta(page(t, c, link), "og:description"); got != b+" won by resignation." {
		t.Errorf("finished og:description %q", got)
	}
}

func TestPreviewLinksUseTheProxysScheme(t *testing.T) {
	_, ts := newTestServer(t)
	html := page(t, ts.Client(), ts.URL+"/", "X-Forwarded-Proto", "https")
	host := strings.TrimPrefix(ts.URL, "http://")
	if got := meta(html, "og:image"); got != "https://"+host+"/og.png" {
		t.Errorf("og:image %q behind an https proxy", got)
	}
}

func TestPreviewTagsAreEscaped(t *testing.T) {
	tags := previewTags(preview{Title: `<b>"x"</b>`, Description: "a & b"}, "https://example.com", "/")
	if strings.Contains(tags, "<b>") || !strings.Contains(tags, "&lt;b&gt;&#34;x&#34;&lt;/b&gt;") || !strings.Contains(tags, "a &amp; b") {
		t.Errorf("tags not escaped: %s", tags)
	}
}
