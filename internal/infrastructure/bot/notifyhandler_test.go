package bot

import (
	"context"
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
)

func TestNotifyHandler_ForwardsGuildSignals(t *testing.T) {
	// given
	guilds := mocks.NewMockGuildActions(t)
	h := NewNotifyHandler(guilds, mocks.NewMockCommunicationService(t))
	ctx := context.Background()
	guilds.On("RefreshGuildLetter", "g1").Once()
	guilds.On("SyncGuild", ctx, "g2").Return(nil).Once()
	guilds.On("SyncGuild", ctx, "g3").Return(errors.New("missing access")).Once()
	guilds.On("ApplyGuildConfig", ctx, "g4").Once()

	// when
	h.OnSummaryRefresh(ctx, "g1")
	h.OnGuildResync(ctx, "g2")
	h.OnGuildResync(ctx, "g3")
	h.OnGuildConfig(ctx, "g4")

	// then: expectations are asserted on cleanup
}

func TestNotifyHandler_OnOverbooked(t *testing.T) {
	// given
	comm := mocks.NewMockCommunicationService(t)
	h := NewNotifyHandler(mocks.NewMockGuildActions(t), comm)
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	request := book.BookRequest{
		Guild:   &guild.Guild{ID: "g1"},
		Member:  &member.Member{ID: "m1", Nick: "Knight", Username: "knight"},
		Spot:    "Hero Cave",
		StartAt: start,
		EndAt:   start.Add(time.Hour),
	}
	res := &reservation.ClippedOrRemovedReservation{
		Original: &reservation.Reservation{ID: 7, AuthorDiscordID: "m2", StartAt: start, EndAt: start.Add(2 * time.Hour)},
	}
	comm.On("NotifyOverbookedMember", request, res).Once()

	// when
	h.OnOverbooked(context.Background(), request, res)

	// then: expectations are asserted on cleanup
}

func TestNotifyHandler_OnBookedAnnouncesInTheBackground(t *testing.T) {
	// given
	guilds := mocks.NewMockGuildActions(t)
	h := NewNotifyHandler(guilds, mocks.NewMockCommunicationService(t))
	ctx, cancel := context.WithCancel(context.Background())
	event := book.Booked{GuildID: "g1", SpotID: 4, Source: book.SourceWeb}
	announced := make(chan error, 1)
	guilds.On("BookingMade", mock.Anything, event).Run(func(args mock.Arguments) {
		c, _ := args.Get(0).(context.Context)
		announced <- c.Err()
	}).Once()

	// when
	h.OnBooked(ctx, event)
	cancel()

	// then
	select {
	case err := <-announced:
		assert.NoError(t, err, "the listener stopping does not cancel the announcement")
	case <-time.After(5 * time.Second):
		t.Fatal("BookingMade was not called")
	}
}

func TestNotifyHandler_OnListenerConnectedRefreshesEverySummary(t *testing.T) {
	// given
	guilds := mocks.NewMockGuildActions(t)
	h := NewNotifyHandler(guilds, mocks.NewMockCommunicationService(t))
	ctx := context.Background()
	guilds.On("RefreshAllGuildLetters", ctx).Once()

	// when
	h.OnListenerConnected(ctx)

	// then: expectations are asserted on cleanup
}
