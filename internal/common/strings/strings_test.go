package strings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToInt64(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "2137"

	// when
	res, err := StrToInt64(input)

	// assert
	assert.Nil(err)
	assert.Equal(res, int64(2137))
}

func TestStrToInt64WithErrorneousInput(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "asdf"

	// when
	_, err := StrToInt64(input)

	// assert
	assert.NotNil(err)
}
