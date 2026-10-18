package worldapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ReadsTheBaseURL(t *testing.T) {
	// given
	t.Setenv("TIBIA_WORLD_API_BASE_URL", "https://api.example.test/v4")

	// when
	cfg, err := LoadConfig()

	// then
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.test/v4", cfg.BaseURL)
}
