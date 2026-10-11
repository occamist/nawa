package handlers

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/occamist/nawa/config"
	"github.com/occamist/nawa/webdist"
)

// Match the Astro dev server port in web/astro.config.mjs.
const devFrontendURL = "http://localhost:4321"

func Frontend(cfg config.Config) (http.Handler, error) {
	if cfg.Dev {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "development frontend is at "+devFrontendURL, http.StatusNotFound)
		}), nil
	}

	static, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		return nil, fmt.Errorf("static files has no dist directory: %v", err)
	}
	return http.FileServer(http.FS(static)), nil
}
