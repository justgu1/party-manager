// Package web embeds the built Svelte SPA and serves it with SPA-style
// fallback (unknown paths return index.html so client-side routing works).
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded SPA. Requests for
// existing static files are served directly; everything else falls back to
// index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// Not a real file: serve index.html for client-side routing.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		_ = path.Ext(p)
		fileServer.ServeHTTP(w, r)
	})
}
