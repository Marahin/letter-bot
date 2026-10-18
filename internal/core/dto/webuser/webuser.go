package webuser

import "time"

// User is a Discord user who signed in to the web panel.
type User struct {
	DiscordUserID string
	Username      string
	GlobalName    string
	Avatar        string
	// DefaultGuildID is the last server visited, kept selected on pages without one.
	DefaultGuildID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DisplayName returns the global name, or the username when no global name is set.
func (u User) DisplayName() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// Token is the stored Discord OAuth token of a web user.
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       *time.Time
}
