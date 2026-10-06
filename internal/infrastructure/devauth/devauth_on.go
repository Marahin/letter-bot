//go:build devauth

// Package devauth holds the dev-only mock users behind /dev/login, the OAuth
// port that resolves them without Discord, and the seed data they act on. It
// compiles only under the devauth build tag; a production build has none of it.
package devauth

import (
	"net/url"
	"slices"
	"strings"
)

const (
	// DevCodePrefix marks an OAuth code that /dev/login mints for a mock user.
	DevCodePrefix = "dev-login:"
	// PremiumGuildID is the seeded premium server, with respawns and reservations.
	PremiumGuildID = "700000000000000900"
	// LockedGuildID is the seeded server without premium.
	LockedGuildID = "700000000000000901"
	// ManageRoleID is the seeded manage rank of both servers.
	ManageRoleID = "800000000000000001"
)

// MockUser is one selectable dev identity. Admin makes them a Discord admin
// (owner or Administrator) of every server in Guilds.
type MockUser struct {
	ID       string
	Username string
	Label    string
	Admin    bool
	Guilds   []string
	RoleIDs  []string
}

// Users are the selectable identities. The site admin is a member of no server:
// its access comes from WEB_ADMIN_DISCORD_IDS, which the dev stacks point at it.
var Users = []MockUser{
	{ID: "700000000000000001", Username: "dev-manager", Label: "Manager (manage rank)", Guilds: []string{PremiumGuildID, LockedGuildID}, RoleIDs: []string{ManageRoleID}},
	{ID: "700000000000000002", Username: "dev-member", Label: "Member (view and reserve)", Guilds: []string{PremiumGuildID, LockedGuildID}},
	{ID: "700000000000000003", Username: "dev-outsider", Label: "Outsider (no server)"},
	{ID: "700000000000000004", Username: "dev-siteadmin", Label: "Site admin"},
	{ID: "700000000000000005", Username: "dev-owner", Label: "Owner (Discord admin)", Admin: true, Guilds: []string{PremiumGuildID, LockedGuildID}},
}

// ByID returns the mock user with the given id.
func ByID(id string) (MockUser, bool) {
	for _, u := range Users {
		if u.ID == id {
			return u, true
		}
	}
	return MockUser{}, false
}

func (u MockUser) memberOf(guildID string) bool {
	return slices.Contains(u.Guilds, guildID)
}

// IsDevBaseURL reports whether the origin is a local development host.
func IsDevBaseURL(baseURL string) bool {
	u, err := url.Parse(strings.ToLower(baseURL))
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
