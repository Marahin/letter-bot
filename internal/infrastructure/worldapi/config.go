package worldapi

import "github.com/kelseyhightower/envconfig"

// Config is the world API connection. An empty BaseURL turns off the online check and the experience job.
type Config struct {
	BaseURL string `envconfig:"TIBIA_WORLD_API_BASE_URL"`
}

// LoadConfig reads TIBIA_WORLD_API_BASE_URL from the environment.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
