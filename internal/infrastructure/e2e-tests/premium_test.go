//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func requireLock(t *testing.T, path string, userID string) {
	t.Helper()
	// given
	page := newPage(t)
	loginAs(t, page, userID)

	// when
	open(t, page, path)

	// then
	requireHas(t, page, "[data-premium-lock]")
	assert.Equal(t, lockCopy, text(page, "[data-premium-lock] p"), "lock copy on %s", path)
	requireHas(t, page, "[data-premium-lock] "+supportLink())
}

func TestPremiumLock_MemberSeesTheLockOnEveryPage(t *testing.T) {
	for _, suffix := range []string{"", "/reservations", "/spots", "/stats", "/stats/players"} {
		t.Run(suffix, func(t *testing.T) {
			requireLock(t, serverPath(lockedGuildID, suffix), memberID)
		})
	}
}

func TestPremiumLock_OwnerSeesTheLockOnSettingsAndChannels(t *testing.T) {
	for _, suffix := range []string{"/settings", "/channels"} {
		t.Run(suffix, func(t *testing.T) {
			requireLock(t, serverPath(lockedGuildID, suffix), ownerID)
		})
	}
}

func TestPremiumLock_SiteAdminPasses(t *testing.T) {
	// given
	page := newPage(t)
	loginAs(t, page, siteAdminID)

	// when
	open(t, page, serverPath(lockedGuildID, "/reservations"))

	// then
	requireNotHas(t, page, "[data-premium-lock]")
}

func TestPremiumLock_PremiumServerOpens(t *testing.T) {
	// given
	page := newPage(t)
	loginAs(t, page, memberID)

	// when
	open(t, page, serverPath(guildID, "/spots"))

	// then
	requireNotHas(t, page, "[data-premium-lock]")
	assert.Contains(t, text(page, "main"), "Hero Cave", "the seeded respawns are listed")
}

func TestPremiumLock_DashboardCardOffersTheUnlock(t *testing.T) {
	// given
	page := newPage(t)

	// when
	loginAs(t, page, memberID)

	// then
	assert.Contains(t, text(page, `[data-guild-card="`+lockedGuildID+`"]`), "Unlock with Premium")
}

func TestPremiumLock_InPolish(t *testing.T) {
	// given a member on a locked page
	page := newPage(t)
	loginAs(t, page, memberID)
	open(t, page, serverPath(lockedGuildID, "/reservations"))

	// when the language picker switches to Polish
	page.MustElement("aside [data-language-picker] summary").MustClick()
	clickAndWaitReload(t, page, page.MustElement(`aside [data-language-option="pl"]`))

	// then
	assert.Equal(t, "Odblokuj tę funkcję w Premium. Dołącz do Discorda i wskakuj na pokład!", text(page, "[data-premium-lock] p"))
}
