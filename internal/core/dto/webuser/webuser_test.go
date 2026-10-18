package webuser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_DisplayName(t *testing.T) {
	// given
	withGlobal := User{Username: "nyx", GlobalName: "Quiet Nyx"}
	withoutGlobal := User{Username: "nyx"}

	// when / then
	assert.Equal(t, "Quiet Nyx", withGlobal.DisplayName())
	assert.Equal(t, "nyx", withoutGlobal.DisplayName())
}
