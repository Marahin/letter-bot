package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"spot-assistant/internal/infrastructure/asseturl"
)

// The whole directory is embedded so a checkout without the generated dist/app.css
// still compiles (go vet, lint); `make css` produces it for every real build.
//
//go:embed dist
var embedded embed.FS

const (
	assetsPrefix   = "/assets/"
	fontsCachePath = assetsPrefix + "fonts/"
	fontSubsetExt  = ".woff2"
)

// FS returns the asset tree with the "dist/" prefix stripped. WEB_ASSETS_DIR serves it
// live from disk instead (compose dev), so CSS edits show without a Go rebuild.
func FS() fs.FS {
	if dir := os.Getenv("WEB_ASSETS_DIR"); dir != "" {
		return os.DirFS(dir)
	}
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

func AssetURL(path string) string { return asseturl.Stamp(path) }

func AssetQuery() string { return asseturl.Query() }

// AssetHandler serves FS() under /assets/.
func AssetHandler() http.Handler {
	return immutableFonts(assetHandler())
}

// immutableFonts marks existing font subsets cacheable forever: embed.FS has a zero
// ModTime, so without it net/http sends no validators and every page refetches them.
// Replace a subset under a new file name, never in place.
func immutableFonts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := strings.CutPrefix(r.URL.Path, assetsPrefix)
		if ok && strings.HasPrefix(r.URL.Path, fontsCachePath) && strings.HasSuffix(name, fontSubsetExt) {
			if _, err := fs.Stat(FS(), name); err == nil {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		next.ServeHTTP(w, r)
	})
}
