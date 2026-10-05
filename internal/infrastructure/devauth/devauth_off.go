//go:build !devauth

// Package devauth is empty in a production build: the mock users exist only
// under the devauth build tag.
package devauth

// MockUser mirrors the tagged type, so a reference compiles in both builds.
type MockUser struct {
	ID       string
	Username string
	Label    string
	Admin    bool
	Guilds   []string
	RoleIDs  []string
}

// Users is empty in a production build.
var Users []MockUser

// ByID always misses in a production build.
func ByID(string) (MockUser, bool) { return MockUser{}, false }
