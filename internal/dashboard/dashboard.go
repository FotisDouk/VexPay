// Package dashboard serves the embedded admin UI. The UI is a single
// self-contained HTML file compiled into the binary, so deployment stays
// "one container" with no separate frontend build.
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets
var assets embed.FS

// Handler returns an http.Handler that serves the dashboard assets. It is
// mounted under /dashboard/ by the API server.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		// The embed is compiled in; this can only fail on a broken build.
		panic("dashboard: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
