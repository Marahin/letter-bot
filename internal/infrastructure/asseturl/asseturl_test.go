package asseturl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/version"
	"spot-assistant/internal/infrastructure/asseturl"
)

func TestStamp_CarriesTheBuild(t *testing.T) {
	// given
	const path = "/assets/app.css"

	// when
	got := asseturl.Stamp(path)

	// then
	assert.Equal(t, path+"?v="+version.Version, got)
}

func TestQuery_IsTheSuffixAlone(t *testing.T) {
	// when
	got := asseturl.Query()

	// then
	assert.Equal(t, "?v="+version.Version, got)
}
