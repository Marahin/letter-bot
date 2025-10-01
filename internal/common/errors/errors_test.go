package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
)

func TestLogError(t *testing.T) {
	// given
	assert := assert.New(t)
	// new(mocks.MockLogAdapter)
	mockLogEntry := mocks.NewMockLogEntry(t)
	inputErr := errors.New("test error")
	mockLogEntry.On("Error", []interface{}{inputErr}).Return()

	// when
	LogError(mockLogEntry, inputErr)

	// assert
	assert.True(mockLogEntry.AssertExpectations(t))
}
