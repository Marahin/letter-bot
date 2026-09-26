package branding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/version"
	"spot-assistant/internal/infrastructure/branding"
)

func TestNotice_CarriesVersionAndName(t *testing.T) {
	// when
	got := branding.Notice()

	// then
	assert.Equal(t, "Version: "+version.Version+" · Letter by tibialoot.com", got)
}
