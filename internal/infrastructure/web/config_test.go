package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setDiscord(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "client")
	t.Setenv("DISCORD_CLIENT_SECRET", "secret")
}

func TestLoadConfig_Defaults(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "http://localhost:8080")
	setDiscord(t)

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.Addr)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Equal(t, ":3005", cfg.MetricsAddr)
	assert.Empty(t, cfg.AdminDiscordIDs)
	assert.True(t, cfg.ExperienceJobEnabled)
	assert.Equal(t, 15*time.Minute, cfg.ExperienceJobInterval)
	assert.Equal(t, "client", cfg.Discord.ClientID)
	assert.Equal(t, "https://discord.gg/b7Qq8V2XFR", cfg.Discord.InviteLink)
}

func TestLoadConfig_EmptyInviteLinkHidesTheSupportLinks(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "http://localhost:8080")
	t.Setenv("DISCORD_INVITE_LINK", "")
	setDiscord(t)

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Empty(t, cfg.Discord.SupportURL())
}

func TestDiscordConfig_SupportURL(t *testing.T) {
	for name, tc := range map[string]struct {
		link string
		want string
	}{
		"https invite":       {link: "https://discord.gg/b7Qq8V2XFR", want: "https://discord.gg/b7Qq8V2XFR"},
		"trimmed":            {link: "  https://discord.gg/x \n", want: "https://discord.gg/x"},
		"http":               {link: "http://discord.gg/x", want: "http://discord.gg/x"},
		"no scheme":          {link: "discord.gg/x", want: ""},
		"javascript is gone": {link: "javascript:alert(1)", want: ""},
		"empty":              {link: "", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			c := DiscordConfig{InviteLink: tc.link}

			// when
			got := c.SupportURL()

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLoadConfig_FromEnvironment(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "https://letter.example")
	t.Setenv("WEB_ADDR", ":9000")
	t.Setenv("WEB_METRICS_ADDR", ":9001")
	t.Setenv("WEB_ADMIN_DISCORD_IDS", "1,2")
	t.Setenv("WEB_EXPERIENCE_JOB_ENABLED", "false")
	t.Setenv("WEB_EXPERIENCE_JOB_INTERVAL", "5m")
	t.Setenv("DISCORD_CLIENT_ID", "client")
	t.Setenv("DISCORD_CLIENT_SECRET", "secret")

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, ":9000", cfg.Addr)
	assert.Equal(t, ":9001", cfg.MetricsAddr)
	assert.Equal(t, []string{"1", "2"}, cfg.AdminDiscordIDs)
	assert.False(t, cfg.ExperienceJobEnabled)
	assert.Equal(t, 5*time.Minute, cfg.ExperienceJobInterval)
	assert.Equal(t, DiscordConfig{ClientID: "client", ClientSecret: "secret", InviteLink: "https://discord.gg/b7Qq8V2XFR"}, cfg.Discord)
}

func TestLoadConfig_RequiresBaseURL(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "")

	// when
	_, err := LoadConfig()

	// then
	assert.Error(t, err)
}

func TestLoadConfig_RejectsInvalidInterval(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "http://localhost:8080")
	t.Setenv("WEB_EXPERIENCE_JOB_INTERVAL", "soon")
	setDiscord(t)

	// when
	_, err := LoadConfig()

	// then
	assert.Error(t, err)
}

func TestLoadConfig_RejectsNonPositiveIntervalWhenJobEnabled(t *testing.T) {
	// given
	t.Setenv("WEB_BASE_URL", "http://localhost:8080")
	t.Setenv("WEB_EXPERIENCE_JOB_INTERVAL", "0s")
	setDiscord(t)

	// when
	_, err := LoadConfig()
	t.Setenv("WEB_EXPERIENCE_JOB_ENABLED", "false")
	_, disabledErr := LoadConfig()

	// then
	assert.ErrorContains(t, err, "WEB_EXPERIENCE_JOB_INTERVAL")
	assert.NoError(t, disabledErr)
}

func TestLoadConfig_RequiresDiscordCredentials(t *testing.T) {
	for _, missing := range []string{"DISCORD_CLIENT_ID", "DISCORD_CLIENT_SECRET"} {
		t.Run(missing, func(t *testing.T) {
			// given
			t.Setenv("WEB_BASE_URL", "http://localhost:8080")
			setDiscord(t)
			t.Setenv(missing, "")

			// when
			_, err := LoadConfig()

			// then
			assert.ErrorContains(t, err, "DISCORD_CLIENT_ID")
		})
	}
}

func TestConfig_CallbackURL(t *testing.T) {
	assert.Equal(t, "https://letter.example/auth/callback", Config{BaseURL: "https://letter.example/"}.CallbackURL())
	assert.Equal(t, "http://localhost:8080/auth/callback", Config{BaseURL: "http://localhost:8080"}.CallbackURL())
}
