package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "asdf"
	value := []int{1, 2, 3}
	store := NewStore[[]int]()

	// when
	err := store.Store(input, value)

	// assert
	assert.Nil(err)
}

func TestGet(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "asdf"
	value := []int{1, 2, 3}
	store := NewStore[[]int]()

	// when
	_ = store.Store(input, value)
	res := store.Get(input)

	// assert
	assert.NotEmpty(res)
	assert.Equal(res, value)
}
