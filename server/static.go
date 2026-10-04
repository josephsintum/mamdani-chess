package server

import (
	"io/fs"
	"net/http"
	"path"
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
		name = "index.html"
	}
	if strings.HasPrefix(name, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, s.assets, name)
}
