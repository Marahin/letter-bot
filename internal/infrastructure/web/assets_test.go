package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	AssetHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestAssetHandler_ServesEmbeddedStylesheet(t *testing.T) {
	// when
	rec := serve(t, "/assets/app.css")

	// then
	require.Equal(t, http.StatusOK, rec.Code, "run `make css` to generate dist/app.css")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
	assert.Contains(t, rec.Body.String(), "font-face")
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestAssetHandler_FontSubsetsAreImmutable(t *testing.T) {
	// when
	rec := serve(t, "/assets/fonts/hanken-grotesk-latin-ext.woff2")

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestAssetHandler_FontLicenseKeepsRevalidating(t *testing.T) {
	// when
	rec := serve(t, "/assets/fonts/OFL-space-grotesk.txt")

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestAssetHandler_MissingFontIsNotCachedForever(t *testing.T) {
	// when
	rec := serve(t, "/assets/fonts/missing.woff2")

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestFS_HonoursAssetsDir(t *testing.T) {
	// given
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.css"), []byte("body{color:red}"), 0o600))
	t.Setenv("WEB_ASSETS_DIR", dir)

	// when
	rec := serve(t, "/assets/app.css")

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{color:red}", rec.Body.String())
}

func TestAssetURL_StampsTheBuild(t *testing.T) {
	// when
	got := AssetURL("/assets/app.css")

	// then
	assert.Equal(t, "/assets/app.css"+AssetQuery(), got)
}
