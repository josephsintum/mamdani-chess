//go:build !embedweb

package web

import (
	"io/fs"
	"os"
	"testing/fstest"
)

// Assets serves web/build from disk when it exists, so `go run ./cmd/server`
// works after `pnpm build` without the embedweb tag. Without a build it
// serves a page saying so.
func Assets() fs.FS {
	if info, err := os.Stat("web/build/index.html"); err == nil && !info.IsDir() {
		return os.DirFS("web/build")
	}
	return fstest.MapFS{"index.html": {Data: []byte(
		"<!doctype html><title>Not built</title><p>Frontend not built. " +
			"Run <code>pnpm --dir web build</code>, or use the Vite dev server on :5173.</p>")}}
}
