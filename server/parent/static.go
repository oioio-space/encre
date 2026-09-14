package parent

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/htmx.min.js static/keyboard-layout.js static/style.css
var staticFS embed.FS

// staticHandler serves the panel's vendored static assets (htmx, above) from
// [staticFS] under staticPrefix, keeping the built server a single binary
// with no external JavaScript fetch.
func staticHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return http.StripPrefix(staticPrefix, http.FileServerFS(sub)), nil
}

// staticPrefix is the URL path this package's static assets are served
// under.
const staticPrefix = "/parent/static/"
