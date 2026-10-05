//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func requireLock(t *testing.T, path string, userID string) {
	t.Helper()
	page := newPage(t)
	loginAs(t, page, userID)

	open(t, page, path)

	requireHas(t, page, "[data-premium-lock]")
	if got := text(page, "[data-premium-lock] p"); got != lockCopy {
		t.Fatalf("lock copy on %s: got %q", path, got)
	}
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
	if !strings.Contains(text(page, "main"), "Hero Cave") {
		t.Fatalf("expected the seeded respawns")
	}
}

func TestPremiumLock_DashboardCardOffersTheUnlock(t *testing.T) {
	// given
	page := newPage(t)

	// when
	loginAs(t, page, memberID)

	// then
	card := text(page, `[data-guild-card="`+lockedGuildID+`"]`)
	if !strings.Contains(card, "Unlock with Premium") {
		t.Fatalf("expected the locked server's card to offer the unlock, got %q", card)
	}
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
	if got := text(page, "[data-premium-lock] p"); got != "Odblokuj tę funkcję w Premium. Dołącz do Discorda i wskakuj na pokład!" {
		t.Fatalf("Polish lock copy: got %q", got)
	}
}
