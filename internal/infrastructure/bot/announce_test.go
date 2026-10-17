package bot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/ports"
)

type discordFixture struct {
	b            *Bot
	gateway      *mocks.MockInteractionGateway
	writer       *mocks.MockSummaryChannelWriter
	configs      *mocks.MockGuildConfigRepository
	reservations *mocks.MockReservationRepository
	summaries    *mocks.MockSummaryService
	spots        *mocks.MockSpotRepository
	metrics      *mocks.MockMetricsPort
}

func newDiscordFixture(t *testing.T) *discordFixture {
	f := &discordFixture{
		gateway:      mocks.NewMockInteractionGateway(t),
		writer:       mocks.NewMockSummaryChannelWriter(t),
		configs:      mocks.NewMockGuildConfigRepository(t),
		reservations: mocks.NewMockReservationRepository(t),
		summaries:    mocks.NewMockSummaryService(t),
		spots:        mocks.NewMockSpotRepository(t),
		metrics:      mocks.NewMockMetricsPort(t),
	}
	f.b = &Bot{
		mgr:             &shards.Manager{},
		gateway:         f.gateway,
		writer:          f.writer,
		guildConfigs:    f.configs,
		reservationRepo: f.reservations,
		summarySrv:      f.summaries,
		spots:           f.spots,
		metrics:         f.metrics,
		formatter:       formatter.NewFormatter(),
		log:             zap.NewNop().Sugar(),
		webBaseURL:      "https://tibialoot.test",
		quit:            make(chan struct{}),
		legacyHashes:    cmap.New[string](),
		channelLocks:    cmap.New[*sync.RWMutex](),
		pacer:           &pacer{now: time.Now, sleep: func(context.Context, time.Duration) error { return nil }},
	}
	f.b.summaries = newSummaryWorker(f.b.reconcileLegacy, time.Millisecond)
	f.metrics.On("IncMessagesSent", mock.Anything, mock.Anything).Maybe()
	f.metrics.On("AddMessagesDeleted", mock.Anything, mock.Anything, mock.Anything).Maybe()
	f.metrics.On("SetUpcomingReservations", mock.Anything, mock.Anything, mock.Anything).Maybe()
	return f
}

func (f *discordFixture) config(cfg guildconfig.Config) {
	cfg.GuildID = "g1"
	f.configs.On("Get", mock.Anything, "g1").Return(&cfg, nil)
}

func (f *discordFixture) channels(channels ...*discordgo.Channel) {
	f.gateway.On("StateChannels", "g1").Return(channels, nil)
}

func premiumConfig() guildconfig.Config {
	return guildconfig.Config{Name: "Celesta", Premium: true, BotPresent: true}
}

func announcement() book.Booked {
	start := time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC)
	return book.Booked{
		GuildID:         "g1",
		SpotID:          4,
		StartAt:         start,
		EndAt:           start.Add(2 * time.Hour),
		Author:          "Knight",
		AuthorDiscordID: "u1",
		ActorID:         "u1",
		Source:          book.SourceDiscord,
	}
}

// expectAnnouncement records the message sent to the channel.
func (f *discordFixture) expectAnnouncement(channelID string) *discordgo.MessageSend {
	sent := &discordgo.MessageSend{}
	f.writer.On("ChannelMessageSendComplex", channelID, mock.Anything).Run(func(args mock.Arguments) {
		*sent = *args.Get(1).(*discordgo.MessageSend)
	}).Return(&discordgo.Message{}, nil).Once()
	return sent
}

func TestBookingMade_AnnouncesInTheConfiguredCommandChannel(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.CommandChannelID = "c2"
	f.config(cfg)
	f.channels(&discordgo.Channel{ID: "c1", Name: "letter"}, &discordgo.Channel{ID: "c2", Name: "bookings"})
	f.spots.On("SelectGuildSpotByID", mock.Anything, "g1", int64(4)).Return(&spot.Spot{ID: 4, Name: "Hero Cave"}, nil)
	sent := f.expectAnnouncement("c2")

	// when
	f.b.BookingMade(context.Background(), announcement())

	// then
	assert.Contains(t, sent.Content, "**Hero Cave** booked by **Knight** (<@u1>)")
	require.NotNil(t, sent.AllowedMentions)
	assert.Empty(t, sent.AllowedMentions.Parse, "nobody is pinged")
	assert.NotNil(t, sent.AllowedMentions.Parse, "an empty list, not a missing one, stops the pings")
}

func TestBookingMade_FallsBackToTheLetterChannel(t *testing.T) {
	cases := map[string]string{
		"no command channel set":  "",
		"command channel is gone": "deleted",
	}
	for name, commandChannelID := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newDiscordFixture(t)
			cfg := premiumConfig()
			cfg.CommandChannelID = commandChannelID
			f.config(cfg)
			f.channels(&discordgo.Channel{ID: "c1", Name: "letter"})
			if commandChannelID != "" {
				f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{{ID: "c1", Name: "letter"}}, nil)
			}
			event := announcement()
			event.SpotName = "Asura Palace"
			sent := f.expectAnnouncement("c1")

			// when
			f.b.BookingMade(context.Background(), event)

			// then
			assert.Contains(t, sent.Content, "**Asura Palace**")
		})
	}
}

func TestBookingMade_SkipsWithoutAChannelOrPremium(t *testing.T) {
	cases := map[string]func(f *discordFixture){
		"not premium": func(f *discordFixture) {
			cfg := premiumConfig()
			cfg.Premium = false
			f.config(cfg)
		},
		"bot left": func(f *discordFixture) {
			cfg := premiumConfig()
			cfg.BotPresent = false
			f.config(cfg)
		},
		"config fails": func(f *discordFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down"))
		},
		"no channel": func(f *discordFixture) {
			f.config(premiumConfig())
			f.channels()
			f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{{ID: "c9", Name: "general"}}, nil)
		},
		"spot lookup fails": func(f *discordFixture) {
			f.config(premiumConfig())
			f.channels(&discordgo.Channel{ID: "c1", Name: "letter"})
			f.spots.On("SelectGuildSpotByID", mock.Anything, "g1", int64(4)).Return(nil, ports.ErrNotFound)
		},
	}
	for name, given := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newDiscordFixture(t)
			given(f)

			// when
			f.b.BookingMade(context.Background(), announcement())

			// then: the writer mock fails on any send
		})
	}
}

func TestBookingMade_SlashCommandOnlyTellsTheFulfiller(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	fulfiller := mocks.NewMockBookingFulfiller(t)
	f.b.WithBookingFulfiller(fulfiller)
	event := announcement()
	event.SpotID, event.SpotName, event.Source = 0, "Hero Cave", book.SourceSlash
	f.spots.On("SelectGuildSpotByName", mock.Anything, "g1", "Hero Cave").Return(&spot.Spot{ID: 4, Name: "Hero Cave"}, nil)
	fulfiller.On("Fulfil", mock.Anything, "g1", "u1", int64(4)).Return(nil).Once()

	// when
	f.b.BookingMade(context.Background(), event)

	// then: no config read and no message
}

func TestBookingMade_FulfilErrorsDoNotStopTheAnnouncement(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	fulfiller := mocks.NewMockBookingFulfiller(t)
	f.b.WithBookingFulfiller(fulfiller)
	fulfiller.On("Fulfil", mock.Anything, "g1", "u1", int64(4)).Return(errors.New("db down")).Once()
	f.config(premiumConfig())
	f.channels(&discordgo.Channel{ID: "c1", Name: "letter"})
	event := announcement()
	event.SpotName = "Hero Cave"
	f.expectAnnouncement("c1")

	// when
	f.b.BookingMade(context.Background(), event)

	// then: expectations are asserted on cleanup
}

func TestBookingMade_FreeTextAuthorSkipsTheFulfiller(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.b.WithBookingFulfiller(mocks.NewMockBookingFulfiller(t))
	event := announcement()
	event.AuthorDiscordID, event.Source = "", book.SourceSlash

	// when
	f.b.BookingMade(context.Background(), event)

	// then: the fulfiller mock fails on any call
}
