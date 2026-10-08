// Package web embeds the single-page app the server hands out.
package web

import (
	"embed"
	"io/fs"
)

// dist is filled by `make web`. On a fresh clone it holds only .gitkeep, and
// the server falls back to a built-in page, so `go build` and `go test` work
// without a frontend toolchain.
//
//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend rooted at its index.html.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	return sub
}
