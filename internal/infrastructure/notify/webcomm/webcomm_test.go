package webcomm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
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
	var sent []byte
	notifier.EXPECT().Overbooked(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p []byte) error {
		sent = p
		return nil
	})

	// when
	a.NotifyOverbookedMember(request, res)

	// then
	decoded, err := notifypg.DecodeOverbookedPayload(sent)
	require.NoError(t, err)
	assert.Equal(t, "g1", decoded.GuildID)
	assert.Equal(t, "m1", decoded.MemberID)
	assert.Equal(t, "Hero Cave", decoded.Spot)
	assert.Equal(t, int64(3), decoded.Original.ID)
}

func TestNotifyOverbookedMember_NotifyErrorIsOnlyLogged(t *testing.T) {
	// given
	notifier := mocks.NewMockBotNotifier(t)
	a := New(notifier, nil)
	notifier.EXPECT().Overbooked(mock.Anything, mock.Anything).Return(errors.New("down"))

	// when / then
	assert.NotPanics(t, func() {
		a.NotifyOverbookedMember(book.BookRequest{Guild: &guild.Guild{ID: "g1"}}, &reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{}})
	})
}

func TestSummaries_AreNotSupported(t *testing.T) {
	// given
	a := New(mocks.NewMockBotNotifier(t), nil)

	// when
	guildErr := a.SendGuildSummary(&guild.Guild{}, &summary.Summary{})
	privateErr := a.SendPrivateSummary(summary.PrivateSummaryRequest{}, &summary.Summary{})

	// then
	assert.ErrorIs(t, guildErr, ErrNotSupported)
	assert.ErrorIs(t, privateErr, ErrNotSupported)
}
