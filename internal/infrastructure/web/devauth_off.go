//go:build !devauth

package web

// registerDevAuth is a no-op in a production build: /dev/login exists only under
// the devauth tag.
func registerDevAuth(_ *Server, _ *Router) {}
