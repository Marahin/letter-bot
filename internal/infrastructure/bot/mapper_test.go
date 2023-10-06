package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapAuthorWithUsernameToAuthorText(t *testing.T) {
	// given
	assert := assert.New(t)
	ioMap := map[string]string{
		" (username)":     "username",
		"nick (username)": "nick",
		"(username)":      "username",
		"Vio Lee/Akiongir/Meliodak/Takar (absol_tes)": "Vio Lee/Akiongir/Meliodak/Takar",
	}

	// When & Assert
	for i, o := range ioMap {
		assert.Equal(o, MapAuthorWithUsernameToAuthorText(i))
	}
}
