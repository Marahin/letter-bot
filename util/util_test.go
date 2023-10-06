package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestPoorMansMap(t *testing.T) {
	// given
	assert := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := PoorMansMap(input, func(el int) int {
		return el * 2
	})

	for i, el := range res {
		assert.Equal(el, input[i]*2)
	}
}

func TestPoorMansFilter(t *testing.T) {
	// given
	assert := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := PoorMansFilter(input, func(el int) bool {
		return el%2 == 0
	})

	for i, el := range res {
		assert.Equal(el, input[i*2])
	}
}

func TestPoorMansPartition(t *testing.T) {
	// given
	assert := assert.New(t)
	input := make([]int, 4)
	for x := 0; x < 4; x++ {
		input[x] = x
	}

	// when
	res := PoorMansPartition(input, 2)

	// assert
	assert.Len(res, 2)
	for _, element := range res {
		assert.Len(element, 2)
	}
}

func TestPoorMansPartitionWithSmallSlice(t *testing.T) {
	// given
	assert := assert.New(t)
	input := make([]int, 4)
	for x := 0; x < 4; x++ {
		input[x] = x
	}

	// when
	res := PoorMansPartition(input, 5)

	// assert
	assert.Len(res, 1)
	for _, element := range res {
		assert.Len(element, 4)
	}
}

func TestPoorMansFind(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	assert := assert.New(t)
	input := make([]testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = testStruct{V: x}
	}

	// when
	res, index := PoorMansFind[testStruct](input, func(el testStruct) bool {
		return el.V > 2
	})

	// assert
	assert.Equal(index, len(input)-1)
	assert.NotEmpty(res)
	assert.Equal(res.V, 3)
}

func TestPoorMansFindWithNoMatch(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	assert := assert.New(t)
	input := make([]*testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = &testStruct{V: x}
	}

	// when
	res, index := PoorMansFind[*testStruct](input, func(el *testStruct) bool {
		return el.V > 15
	})

	// assert
	assert.Equal(index, -1)
	assert.Nil(res)
}

func TestTruncate(t *testing.T) {
	// given
	assert := assert.New(t)
	input := make([]int, 10)
	for x := 0; x < 10; x++ {
		input[x] = x
	}

	// when
	res := Truncate(input, 3)

	// assert
	assert.Len(res, 3)
	assert.Contains(res, 0, 1, 2)
}

func TestPoorMansContains(t *testing.T) {
	// given
	assert := assert.New(t)
	input := []int{1, 2, 3}

	// when
	res := PoorMansContains(input, 2)

	// assert
	assert.True(res)
}

func TestPoorMansContainsWithFalsyValues(t *testing.T) {
	// given
	assert := assert.New(t)
	input := []int{1, 2, 3}

	// when
	res := PoorMansContains(input, 15)

	// assert
	assert.False(res)
}

func TestPoorMansSum(t *testing.T) {
	// given
	type testStruct struct {
		V int
	}
	assert := assert.New(t)
	input := make([]*testStruct, 4)
	for x := 0; x < 4; x++ {
		input[x] = &testStruct{V: x}
	}

	// when
	res := PoorMansSum(input, func(el *testStruct) int64 {
		return int64(el.V)
	})

	// assert
	assert.Equal(int64(6), res)
}

type MockLogAdapter struct {
	mock.Mock
}

func (a *MockLogAdapter) Error(inputArgs ...interface{}) {
	a.Called(inputArgs...)
}

func TestLogError(t *testing.T) {
	// given
	assert := assert.New(t)
	mockLogEntry := new(MockLogAdapter)
	inputErr := errors.New("test error")
	mockLogEntry.On("Error", inputErr).Return()

	// when
	LogError(mockLogEntry, inputErr)

	// assert
	assert.True(mockLogEntry.AssertExpectations(t))
}
