package experience

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCharacterKey(t *testing.T) {
	// when / then
	assert.Equal(t, "quiet nyx", CharacterKey("  Quiet Nyx "))
	assert.Equal(t, "", CharacterKey("   "))
}

func TestCharacters(t *testing.T) {
	// when
	got := Characters("Quiet Nyx/ Storm Quiet //quiet nyx/Dark Quiet ")

	// then
	assert.Equal(t, []Character{
		{Key: "quiet nyx", Name: "Quiet Nyx"},
		{Key: "storm quiet", Name: "Storm Quiet"},
		{Key: "dark quiet", Name: "Dark Quiet"},
	}, got)
	assert.Empty(t, Characters(""))
}
