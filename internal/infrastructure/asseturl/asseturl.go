// Package asseturl stamps static asset URLs with the running build. It sits below the
// web shell so packages the shell imports (branding) can build asset URLs too.
package asseturl

import (
	"net/url"

	"spot-assistant/internal/common/version"
)

var query = "?v=" + url.QueryEscape(version.Version)

// Stamp appends the running build to a static asset path, so a release serves URLs no
// cache has seen. The bare path keeps serving, so a link from an older page never 404s.
func Stamp(path string) string { return path + query }

// Query is the suffix alone, for a script that builds asset URLs client-side.
func Query() string { return query }
