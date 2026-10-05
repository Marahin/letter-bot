//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func loginCTA(path string) string {
	return `[data-login-cta] a[href^="/login?to=` + strings.ReplaceAll(path, "/", "%2F") + `"]`
}

func TestPublicStats_AnonymousOverview(t *testing.T) {
	// given
	page := newPage(t)
	path := serverPath(guildID, "/stats")

	// when
	open(t, page, path)

	// then
	requireHas(t, page, "[data-public-stats]")
	requireHas(t, page, "header")
	requireNotHas(t, page, "aside")
	requireHas(t, page, loginCTA(path))
	requireHas(t, page, `[data-public-stats] a[href="`+path+`"][aria-current="page"]`)
}

func TestPublicStats_AnonymousRespawnAndPlayer(t *testing.T) {
	// given
	page := newPage(t)
	open(t, page, serverPath(guildID, "/stats/spots"))

	// when a visitor opens the first respawn
	clickAndWaitReload(t, page, page.MustElement("table tbody a"))

	// then
	if !strings.Contains(currentPath(page), "/stats/spots/") {
		t.Fatalf("expected a respawn page, got %s", currentPath(page))
	}
	requireHas(t, page, "[data-public-stats]")
	requireNotHas(t, page, "aside")
	requireNotHas(t, page, `main a[href^="`+serverPath(guildID, "/spots/")+`"]`)
	requireHas(t, page, `[data-login-cta] a[href^="/login?to=%2Fservers%2F`+guildID+`%2Fstats%2Fspots%2F"]`)

	// when the visitor opens the first player
	open(t, page, serverPath(guildID, "/stats/players"))
	clickAndWaitReload(t, page, page.MustElement("table tbody a"))

	// then
	if !strings.Contains(currentPath(page), "/stats/players/") {
		t.Fatalf("expected a player page, got %s", currentPath(page))
	}
	requireHas(t, page, "[data-public-stats]")
}

func TestPublicStats_LockedServerShowsTheLock(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, serverPath(lockedGuildID, "/stats"))

	// then
	requireHas(t, page, "[data-premium-lock]")
	requireNotHas(t, page, "[data-public-stats]")
}

func TestPublicStats_OutsiderKeepsTheirOwnSidebar(t *testing.T) {
	// given
	page := newPage(t)
	loginAs(t, page, outsiderID)
	path := serverPath(guildID, "/stats")

	// when
	open(t, page, path)

	// then
	requireHas(t, page, "[data-public-stats]")
	requireHas(t, page, "aside")
	requireNotHas(t, page, "[data-login-cta]")
	requireNotHas(t, page, `aside a[href^="/servers/`+guildID+`"]`)
}

func TestPublicStats_MemberGetsTheSidebar(t *testing.T) {
	// given
	page := newPage(t)
	loginAs(t, page, memberID)
	path := serverPath(guildID, "/stats")

	// when
	open(t, page, path)

	// then
	requireNotHas(t, page, "[data-public-stats]")
	requireHas(t, page, `aside a[href="`+path+`"][aria-current="page"]`)
}

func TestPublicStats_UnknownServerIsNotFound(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, serverPath("123", "/stats"))

	// then
	if !strings.Contains(text(page, "main"), "404") {
		t.Fatalf("expected a 404 page")
	}
}
