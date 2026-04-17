package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var assets embed.FS

// Handler returns an HTTP handler that serves the embedded dashboard files.
// For SPA routing, any path that doesn't match a real file serves index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		panic("dashboard: embedded assets missing: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip API routes
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
			http.NotFound(w, r)
			return
		}

		// Try serving the actual file first
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in embedded FS
		trimmed := strings.TrimPrefix(path, "/")
		f, err := sub.Open(trimmed)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Prerendered route: try "{path}.html" for clean URLs like /cluster
		// (SvelteKit's static adapter emits cluster.html, not cluster/index.html here).
		// Without this, reloading /cluster falls through to the SPA fallback, which
		// has a different base/manifest and throws "Not found: /cluster".
		if !strings.Contains(trimmed, ".") {
			htmlPath := trimmed + ".html"
			if f, err := sub.Open(htmlPath); err == nil {
				f.Close()
				r.URL.Path = "/" + htmlPath
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for all other routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
