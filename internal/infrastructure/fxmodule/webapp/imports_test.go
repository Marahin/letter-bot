package webapp

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bot adapter dials the Discord gateway, so the web binary must not link it.
func TestWebBinary_DoesNotImportTheBotAdapter(t *testing.T) {
	// when
	out, err := exec.Command("go", "list", "-e", "-deps", "spot-assistant/cmd/web").Output()

	// then
	require.NoError(t, err)
	deps := strings.Fields(string(out))
	assert.NotContains(t, deps, "spot-assistant/internal/infrastructure/bot")
	assert.NotContains(t, deps, "spot-assistant/internal/infrastructure/bot/formatter")
	assert.NotContains(t, deps, "spot-assistant/internal/infrastructure/fxmodule/botapp")
}

// /dev/login signs in without Discord, so a build without the devauth tag must not link it.
func TestWebBinary_WithoutTagsDoesNotImportDevAuth(t *testing.T) {
	// when
	out, err := exec.Command("go", "list", "-e", "-deps", "spot-assistant/cmd/web").Output()

	// then
	require.NoError(t, err)
	assert.NotContains(t, strings.Fields(string(out)), "spot-assistant/internal/infrastructure/devauth")
}
