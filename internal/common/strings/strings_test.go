package strings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrToInt64(t *testing.T) {
	// given
	is := assert.New(t)
	input := "2137"

	// when
	res, err := StrToInt64(input)

	// assert
	is.Nil(err)
	is.Equal(res, int64(2137))
}

func TestStrToInt64WithErrorneousInput(t *testing.T) {
	// given
	is := assert.New(t)
	input := "asdf"

	// when
	_, err := StrToInt64(input)

	// assert
	is.NotNil(err)
}
