//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestDevLogin_ListsEveryMockUser(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/dev/login")

	// then
	if n := len(page.MustElements("a[data-dev-user]")); n != 5 {
		t.Fatalf("expected 5 mock users, got %d", n)
	}
}

func TestDevLogin_LandsOnTheDashboard(t *testing.T) {
	// given
	page := newPage(t)

	// when
	loginAs(t, page, memberID)

	// then
	if currentPath(page) != "/dashboard" {
		t.Fatalf("expected /dashboard after dev login, got %s", currentPath(page))
	}
	if !strings.Contains(text(page, "main"), "Letter E2E") {
		t.Fatalf("expected the seeded server on the dashboard")
	}
}

func TestDevLogin_ReturnsToThePageItWasGiven(t *testing.T) {
	// given
	page := newPage(t)
	target := serverPath(guildID, "/reservations")

	// when
	open(t, page, "/dev/login/"+memberID+"?to="+target)

	// then
	if currentPath(page) != target {
		t.Fatalf("expected %s, got %s", target, currentPath(page))
	}
}

func TestAnonymous_ReservationsBounceToDiscord(t *testing.T) {
	// given
	page := newPage(t)

	// when
	page.MustNavigate(url(serverPath(guildID, "/reservations")))

	// then
	waitURLContains(t, page, "discord.com/oauth2/authorize")
}
