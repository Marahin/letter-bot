package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStampManifestIcons(t *testing.T) {
	// given
	raw := []byte(`{"name": "x", "icons": [{"src": "/assets/a.png"}], "z": 1}`)

	// when
	got, err := stampManifestIcons(raw)

	// then
	assert.NoError(t, err)
	assert.Equal(t, `{"name": "x", "icons": [{"src":"`+AssetURL("/assets/a.png")+`"}], "z": 1}`, string(got))
}
