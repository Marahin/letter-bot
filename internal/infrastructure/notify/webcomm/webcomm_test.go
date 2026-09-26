package webcomm

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func TestNotifyOverbookedMember_SendsThePayload(t *testing.T) {
	// given
	notifier := mocks.NewMockBotNotifier(t)
	a := New(notifier, nil)
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	request := book.BookRequest{
		Guild:   &guild.Guild{ID: "g1"},
		Member:  &member.Member{ID: "m1", Nick: "Boss"},
		Spot:    "Hero Cave",
		StartAt: start,
		EndAt:   start.Add(time.Hour),
	}
	res := &reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{ID: 3, AuthorDiscordID: "u2"}}
	notifier.EXPECT().Overbooked(mock.Anything, request, res).Return(nil).Once()

	// when
	a.NotifyOverbookedMember(request, res)

	// then: expectations are asserted on cleanup
}

func TestNotifyOverbookedMember_NotifyErrorIsOnlyLogged(t *testing.T) {
	// given
	notifier := mocks.NewMockBotNotifier(t)
	a := New(notifier, nil)
	notifier.EXPECT().Overbooked(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("down"))

	// when / then
	assert.NotPanics(t, func() {
		a.NotifyOverbookedMember(book.BookRequest{Guild: &guild.Guild{ID: "g1"}}, &reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{}})
	})
}

func TestSendPrivateSummary_IsNotSupported(t *testing.T) {
	// given
	a := New(mocks.NewMockBotNotifier(t), nil)

	// when
	privateErr := a.SendPrivateSummary(summary.PrivateSummaryRequest{}, &summary.Summary{})

	// then
	assert.ErrorIs(t, privateErr, ErrNotSupported)
}
