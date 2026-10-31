package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
)

func TestLogError(t *testing.T) {
	// given
	is := assert.New(t)
	mockLogEntry := mocks.NewMockLogEntry(t)
	inputErr := errors.New("test error")
	mockLogEntry.On("Error", []any{inputErr}).Return()

	// when
	LogError(mockLogEntry, inputErr)

	// assert
	is.True(mockLogEntry.AssertExpectations(t))
}
