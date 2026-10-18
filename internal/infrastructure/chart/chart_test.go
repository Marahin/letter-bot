package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChart(t *testing.T) {
	// Given
	is := assert.New(t)
	values := []float64{1, 2, 3}
	legends := []string{"one", "two", "three"}
	adapter := NewAdapter()

	// When
	res, err := adapter.NewChart(values, legends)

	// Assert
	is.Nil(err)
	is.Greater(len(res), 0)
}

// I have not yet managed to make the library fail.
// If this happens, this test is to be properly fixed.
// func TestNewChartWithImageFailure(t *testing.T) {
// 	// Given
// 	is := assert.New(t)
// 	// values := []float64{1, 2, 3}
// 	// legends := []string{"one", "two", "three"}
// 	adapter := NewAdapter()

// 	// When
// 	_, err := adapter.NewChart(nil, nil)

// 	// Assert
// 	is.NotNil(err)
// }
