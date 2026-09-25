package web

import (
	"errors"
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

	Discord DiscordConfig `ignored:"true"`
}

type DiscordConfig struct {
	ClientID     string `envconfig:"CLIENT_ID"`
	ClientSecret string `envconfig:"CLIENT_SECRET"`
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
	if err := envconfig.Process("discord", &cfg.Discord); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
