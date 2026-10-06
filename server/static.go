package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// static serves the built SvelteKit app. Paths that aren't files fall back
// to index.html so client-side routes like /game/K7F3QZ load the app.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if info, err := fs.Stat(s.assets, name); err != nil || info.IsDir() {
		if strings.HasPrefix(name, "_app/") {
			// A missing build asset (an old chunk after a deploy, say) is a
			// real 404: the app shell in its place can't run as a script.
			w.Header().Set("Cache-Control", "no-cache")
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	if name == "index.html" {
		s.index(w, r) // with the page's link preview tags
		return
	}
	if strings.HasPrefix(name, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Add("Vary", "Accept-Encoding")
	file, encoding := s.compressed(r, name)
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}
	http.ServeFileFS(w, r, s.assets, file)
}

// compressed picks the precompressed copy of name that the build has and
// the browser accepts, brotli first, or name itself with no encoding.
func (s *Server) compressed(r *http.Request, name string) (file, encoding string) {
	accept := r.Header.Get("Accept-Encoding")
	for _, c := range []struct{ encoding, ext string }{{"br", ".br"}, {"gzip", ".gz"}} {
		if !accepts(accept, c.encoding) {
			continue
		}
		if _, err := fs.Stat(s.assets, name+c.ext); err == nil {
			return name + c.ext, c.encoding
		}
	}
	return name, ""
}

// accepts reports whether an Accept-Encoding header allows encoding. A
// token with q=0 is a refusal.
func accepts(header, encoding string) bool {
	for _, part := range strings.Split(header, ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(token), encoding) {
			continue
		}
		if q, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if v, err := strconv.ParseFloat(q, 64); err == nil && v == 0 {
				return false
			}
		}
		return true
	}
	return false
}
