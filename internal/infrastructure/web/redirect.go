package web

import (
	"net/url"
	"strings"
	"unicode"
)

// isLocalURL reports whether to is a path on our own origin, safe as a redirect
// target. Browsers treat "\" as "/" and drop tabs and newlines when resolving a
// Location, so "/\evil.com" or "/\t/evil.com" would otherwise leave the origin.
func isLocalURL(to string) bool {
	if to == "" || to[0] != '/' {
		return false
	}
	if len(to) > 1 && (to[1] == '/' || to[1] == '\\') {
		return false
	}
	if strings.ContainsRune(to, '\\') {
		return false
	}
	if strings.IndexFunc(to, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return false
	}
	u, err := url.Parse(to)
	return err == nil && u.Scheme == "" && u.Host == "" && u.User == nil && u.Opaque == ""
}
