package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func TestLocalNotifier_ForwardsToTheBot(t *testing.T) {
	// given
	guilds := mocks.NewMockGuildActions(t)
	comm := mocks.NewMockCommunicationService(t)
	n := NewLocalNotifier(guilds, comm)
	ctx := context.Background()
	boom := errors.New("missing access")
	request := book.BookRequest{Spot: "Hero Cave"}
	res := &reservation.ClippedOrRemovedReservation{Original: &reservation.Reservation{ID: 7}}
	guilds.On("RefreshGuildLetter", "g1").Once()
	guilds.On("SyncGuild", ctx, "g2").Return(boom).Once()
	guilds.On("ApplyGuildConfig", ctx, "g3").Once()
	comm.On("NotifyOverbookedMember", request, res).Once()

	// when
	summaryErr := n.SummaryChanged(ctx, "g1")
	resyncErr := n.ResyncRequested(ctx, "g2")
	configErr := n.ConfigChanged(ctx, "g3")
	overbookedErr := n.Overbooked(ctx, request, res)

	// then
	assert.NoError(t, summaryErr)
	assert.ErrorIs(t, resyncErr, boom)
	assert.NoError(t, configErr)
	assert.NoError(t, overbookedErr)
}
