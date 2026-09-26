// Package branding holds Letter's site identity and the attribution text the web
// shell renders. The build version itself stays in common/version so -ldflags can
// inject it.
package branding

import "spot-assistant/internal/common/version"

const (
	Name   = "Letter"
	Domain = "tibialoot.com"
)

// Notice is the public attribution text shown in the web footer.
func Notice() string {
	return "Version: " + version.Version + " · " + Name + " by " + Domain
}
