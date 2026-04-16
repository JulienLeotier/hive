package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var assets embed.FS

// Handler returns an HTTP handler that serves the embedded dashboard files.
func Handler() http.Handler {
	sub, _ := fs.Sub(assets, "dist")
	return http.FileServer(http.FS(sub))
}
