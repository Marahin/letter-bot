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
	assert.Equal(t, DiscordConfig{ClientID: "client", ClientSecret: "secret"}, cfg.Discord)
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
