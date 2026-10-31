//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDevLogin_ListsEveryMockUser(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/dev/login")

	// then
	assert.Len(t, page.MustElements("a[data-dev-user]"), 5)
}

func TestDevLogin_LandsOnTheDashboard(t *testing.T) {
	// given
	page := newPage(t)

	// when
	loginAs(t, page, memberID)

	// then
	assert.Equal(t, "/dashboard", currentPath(page))
	assert.Contains(t, text(page, "main"), "Letter E2E", "the seeded server is on the dashboard")
}

func TestDevLogin_ReturnsToThePageItWasGiven(t *testing.T) {
	// given
	page := newPage(t)
	target := serverPath(guildID, "/reservations")

	// when
	open(t, page, "/dev/login/"+memberID+"?to="+target)

	// then
	assert.Equal(t, target, currentPath(page))
}

func TestAnonymous_ReservationsBounceToDiscord(t *testing.T) {
	// given
	page := newPage(t)

	// when
	page.MustNavigate(abs(serverPath(guildID, "/reservations")))

	// then
	waitURLContains(t, page, "discord.com/oauth2/authorize")
}
