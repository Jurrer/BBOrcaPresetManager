// Package web exposes the embedded single-page UI assets via //go:embed.
//
// Plan §3 "Embedded web UI (embed)" + plan §10 "vanilla HTML/JS/CSS
// embedded via go:embed".
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html app.js style.css
var assets embed.FS

// FS returns an fs.FS rooted at the embedded asset directory so callers
// can mount it under any URL prefix.
func FS() fs.FS {
	return assets
}

// Handler returns an http.Handler that serves the embedded UI at "/".
func Handler() http.Handler {
	sub, err := fs.Sub(assets, ".")
	if err != nil {
		// Sub on the root fs always succeeds; this would only fire if
		// the embed var were repointed. Keep a defensive fallback.
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
