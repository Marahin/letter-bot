package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPoorMansContains(t *testing.T) {
	// given
	is := assert.New(t)
	input := []int{1, 2, 3}

	// when
	res := PoorMansContains(input, 2)

	// assert
	is.True(res)
}

func TestPoorMansContainsWithFalsyValues(t *testing.T) {
	// given
	is := assert.New(t)
	input := []int{1, 2, 3}

	// when
	res := PoorMansContains(input, 15)

	// assert
	is.False(res)
}

func TestPoorMansSum(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	is := assert.New(t)
	input := make([]*testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = &testStruct{V: x}
	}

	// when
	res := PoorMansSum(input, func(el *testStruct) int64 {
		return int64(el.V)
	})

	// assert
	is.Equal(int64(6), res)
}

func TestPoorMansFindWithNoMatch(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	is := assert.New(t)
	input := make([]*testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = &testStruct{V: x}
	}

	// when
	res, index := PoorMansFind[*testStruct](input, func(el *testStruct) bool {
		return el.V > 15
	})

	// assert
	is.Equal(index, -1)
	is.Nil(res)
}

func TestTruncate(t *testing.T) {
	// given
	is := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := Truncate(input, 3)

	// assert
	is.Len(res, 3)
	is.Contains(res, 0, 1, 2)
}

func TestPoorMansMap(t *testing.T) {
	// given
	is := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := PoorMansMap(input, func(el int) int {
		return el * 2
	})

	for i, el := range res {
		is.Equal(el, input[i]*2)
	}
}

func TestPoorMansFilter(t *testing.T) {
	// given
	is := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := PoorMansFilter(input, func(el int) bool {
		return el%2 == 0
	})

	for i, el := range res {
		is.Equal(el, input[i*2])
	}
}

func TestPoorMansPartition(t *testing.T) {
	// given
	is := assert.New(t)
	input := make([]int, 4)
	for x := 0; x < 4; x++ {
		input[x] = x
	}

	// when
	res := PoorMansPartition(input, 2)

	// assert
	is.Len(res, 2)
	for _, element := range res {
		is.Len(element, 2)
	}
}

func TestPoorMansPartitionWithSmallSlice(t *testing.T) {
	// given
	is := assert.New(t)
	input := make([]int, 4)
	for x := 0; x < 4; x++ {
		input[x] = x
	}

	// when
	res := PoorMansPartition(input, 5)

	// assert
	is.Len(res, 1)
	for _, element := range res {
		is.Len(element, 4)
	}
}

func TestPoorMansFind(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	is := assert.New(t)
	input := make([]testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = testStruct{V: x}
	}

	// when
	res, index := PoorMansFind[testStruct](input, func(el testStruct) bool {
		return el.V > 2
	})

	// assert
	is.Equal(index, len(input)-1)
	is.NotEmpty(res)
	is.Equal(res.V, 3)
}
