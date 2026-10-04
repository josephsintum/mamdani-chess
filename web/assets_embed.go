//go:build embedweb

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var build embed.FS

// Assets returns the SvelteKit build compiled into the binary.
func Assets() fs.FS {
	sub, err := fs.Sub(build, "build")
	if err != nil {
		panic(err)
	}
	return sub
}
