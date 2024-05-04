package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_sanitizeTimeFormatDots(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "19.00"

	// when
	output := sanitizeTimeFormat(input)

	// output
	assert.Equal(output, "19:00")
}

func Test_sanitizeTimeFormatSemicolons(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "19;00"

	// when
	output := sanitizeTimeFormat(input)

	// output
	assert.Equal(output, "19:00")
}

func Test_sanitizeTimeFormatMidnight(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "24:00"

	// when
	output := sanitizeTimeFormat(input)

	// output
	assert.Equal(output, "00:00")
}
