package api

import (
	"errors"
	"spot-assistant/internal/common/strings"
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"testing"
)

func TestSendPeriodicMessageUnlessRedundantWhenChannelExistsAndNoPreviousMessages(t *testing.T) {
	// given
	guild := &discord.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	ch := &discord.Channel{ID: "test-channel-id", Name: "letter"}
	bot := new(mocks.MockBot)
	bot.On("FindChannel", guild, "letter").Return(ch, nil)
	bot.On("ChannelMessages", guild, ch, 1).Return([]*discord.Message{}, nil)
	bot.On("SendChannelMessage", guild, ch, strings.PeriodicMessageContent).Return(nil)
	defer bot.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	defer reservationRepo.AssertExpectations(t)
	summarySrv := new(mocks.MockSummaryService)
	defer summarySrv.AssertExpectations(t)
	bookingSrv := new(mocks.MockBookingService)
	defer bookingSrv.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when && then
	adapter.SendPeriodicMessageUnlessRedundant(bot, guild)
}

func TestSendPeriodicMessageUnlessRedundantWhenChannelExistsAndMessageIsNotRedundant(t *testing.T) {
	// given
	guild := &discord.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	ch := &discord.Channel{ID: "test-channel-id", Name: "letter"}
	bot := new(mocks.MockBot)
	bot.On("FindChannel", guild, "letter").Return(ch, nil)
	bot.On("ChannelMessages", guild, ch, 1).Return([]*discord.Message{{Content: "asdf"}}, nil)
	bot.On("SendChannelMessage", guild, ch, strings.PeriodicMessageContent).Return(nil)
	defer bot.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	defer reservationRepo.AssertExpectations(t)
	summarySrv := new(mocks.MockSummaryService)
	defer summarySrv.AssertExpectations(t)
	bookingSrv := new(mocks.MockBookingService)
	defer bookingSrv.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when && then
	adapter.SendPeriodicMessageUnlessRedundant(bot, guild)
}

func TestSendPeriodicMessageUnlessRedundantWhenChannelExistsAndMessageIsRedundant(t *testing.T) {
	// given
	guild := &discord.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	ch := &discord.Channel{ID: "test-channel-id", Name: "letter"}
	bot := new(mocks.MockBot)
	bot.On("FindChannel", guild, "letter").Return(ch, nil)
	bot.On("ChannelMessages", guild, ch, 1).Return([]*discord.Message{{Content: strings.PeriodicMessageContent}}, nil)
	defer bot.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	defer reservationRepo.AssertExpectations(t)
	summarySrv := new(mocks.MockSummaryService)
	defer summarySrv.AssertExpectations(t)
	bookingSrv := new(mocks.MockBookingService)
	defer bookingSrv.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when && then
	adapter.SendPeriodicMessageUnlessRedundant(bot, guild)
}

func TestSendPeriodicMessageUnlessRedundantWhenChannelDoesNotExist(t *testing.T) {
	// given
	guild := &discord.Guild{
		ID:   "test-guild-id",
		Name: "test-guild-name",
	}
	bot := new(mocks.MockBot)
	bot.On("FindChannel", guild, "letter").Return(&discord.Channel{}, errors.New("channel does not exist"))
	defer bot.AssertExpectations(t)
	reservationRepo := new(mocks.MockReservationRepo)
	defer reservationRepo.AssertExpectations(t)
	summarySrv := new(mocks.MockSummaryService)
	defer summarySrv.AssertExpectations(t)
	bookingSrv := new(mocks.MockBookingService)
	defer bookingSrv.AssertExpectations(t)
	adapter := NewApplication(reservationRepo, summarySrv, bookingSrv)

	// when && then
	adapter.SendPeriodicMessageUnlessRedundant(bot, guild)
}
