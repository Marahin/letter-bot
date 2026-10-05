package web

import (
	"errors"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Addr                  string        `envconfig:"ADDR" default:":8080"`
	BaseURL               string        `envconfig:"BASE_URL" required:"true"`
	MetricsAddr           string        `envconfig:"METRICS_ADDR" default:":3005"`
	AdminDiscordIDs       []string      `envconfig:"ADMIN_DISCORD_IDS"`
	ExperienceJobEnabled  bool          `envconfig:"EXPERIENCE_JOB_ENABLED" default:"true"`
	ExperienceJobInterval time.Duration `envconfig:"EXPERIENCE_JOB_INTERVAL" default:"15m"`
	// DevAuth turns on the /dev/login mock users. It works only in a binary built
	// with the devauth tag and a localhost base URL.
	DevAuth bool `envconfig:"DEV_AUTH" default:"false"`

	Discord DiscordConfig `ignored:"true"`
}

type DiscordConfig struct {
	ClientID     string `envconfig:"CLIENT_ID"`
	ClientSecret string `envconfig:"CLIENT_SECRET"`
	// InviteLink is the public invite to the community Discord server. An empty
	// value hides every support link.
	InviteLink string `envconfig:"INVITE_LINK" default:"https://discord.gg/b7Qq8V2XFR"`
}

// SupportURL returns the community invite only when it is an http(s) URL, so a
// malformed value degrades to no link rather than a broken (or script-bearing) href.
func (c DiscordConfig) SupportURL() string {
	link := strings.TrimSpace(c.InviteLink)
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		return ""
	}
	return link
}

// CallbackURL is the OAuth redirect URL to register in the Discord developer portal.
func (c Config) CallbackURL() string {
	return strings.TrimRight(c.BaseURL, "/") + "/auth/callback"
}

// LoadConfig reads WEB_* and DISCORD_* from the environment.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := envconfig.Process("web", &cfg); err != nil {
		return Config{}, err
	}
	// envconfig's required accepts a variable that is set but empty, as a blank .env line leaves it.
	if cfg.BaseURL == "" {
		return Config{}, errors.New("WEB_BASE_URL must not be empty")
	}
	if cfg.ExperienceJobEnabled && cfg.ExperienceJobInterval <= 0 {
		return Config{}, errors.New("WEB_EXPERIENCE_JOB_INTERVAL must be positive")
	}
	if err := envconfig.Process("discord", &cfg.Discord); err != nil {
		return Config{}, err
	}
	// Sign-in cannot work without the Discord application credentials.
	if cfg.Discord.ClientID == "" || cfg.Discord.ClientSecret == "" {
		return Config{}, errors.New("DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET must be set")
	}
	return cfg, nil
}
