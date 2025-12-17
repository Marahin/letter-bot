package communication

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
)

func TestAdapter_NotifyUpcomingReservation(t *testing.T) {
	// given
	assert := assert.New(t)
	m := &member.Member{
		ID:       "test-id",
		Username: "test-username",
	}
	g := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotName := "test-spot"
	startAt := time.Now()

	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("SendDMUpcomingReservationNotification", g, m, spotName, startAt).Return(nil).Once()

	memberOperations := mocks.NewMockMemberRepository(t)
	adapter := NewAdapter(botOperations, memberOperations)

	// when
	err := adapter.NotifyUpcomingReservation(g, m, spotName, startAt)

	// assert
	assert.NoError(err)
	botOperations.AssertExpectations(t)
}

func TestAdapter_NotifyUpcomingReservation_Error(t *testing.T) {
	// given
	assert := assert.New(t)
	m := &member.Member{
		ID:       "test-id",
		Username: "test-username",
	}
	g := &guild.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	spotName := "test-spot"
	startAt := time.Now()
	expectedErr := errors.New("failed to send DM")

	botOperations := mocks.NewMockBotPort(t)
	botOperations.On("SendDMUpcomingReservationNotification", g, m, spotName, startAt).Return(expectedErr).Once()

	memberOperations := mocks.NewMockMemberRepository(t)
	adapter := NewAdapter(botOperations, memberOperations)

	// when
	err := adapter.NotifyUpcomingReservation(g, m, spotName, startAt)

	// assert
	assert.Error(err)
	assert.Equal(expectedErr, err)
	botOperations.AssertExpectations(t)
}
