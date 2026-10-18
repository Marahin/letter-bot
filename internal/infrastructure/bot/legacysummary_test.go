package bot

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

// snowflakeAt is a Discord id created at t.
func snowflakeAt(t time.Time) string {
	return strconv.FormatInt((t.UnixMilli()-1420070400000)<<22, 10)
}

func upcomingReservations(start time.Time) []*reservation.ReservationWithSpot {
	return []*reservation.ReservationWithSpot{{
		Reservation: reservation.Reservation{Author: "Knight", StartAt: start, EndAt: start.Add(time.Hour), GuildID: "g1"},
		Spot:        reservation.Spot{Name: "Hero Cave"},
	}}
}

func ledgerOf(author string) summary.Ledger {
	return summary.Ledger{{Spot: "Hero Cave", Bookings: []*summary.Booking{{Author: author}}}}
}

// expectPost expects a full summary post in channel c1 with nothing to clean.
func (f *discordFixture) expectPost(ledger summary.Ledger) {
	f.summaries.On("PrepareSummary", mock.Anything).Return(&summary.Summary{Ledger: ledger, Chart: []byte{1}}, nil).Once()
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelFileSend", "c1", "spots.png", mock.Anything).Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c1", mock.Anything).Return(&discordgo.Message{}, nil).Once()
}

func TestPostLegacySummary_PostsOnlyWhenTheContentChanged(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.GuildID = "g1"
	f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
	start := time.Now().Add(time.Hour)
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return(upcomingReservations(start), nil)
	f.summaries.On("Ledger", mock.Anything).Return(ledgerOf("Knight")).Twice()
	f.expectPost(ledgerOf("Knight"))

	// when
	firstNext, firstErr := f.b.PostLegacySummary(context.Background(), &cfg)
	secondNext, secondErr := f.b.PostLegacySummary(context.Background(), &cfg)

	// then
	require.NoError(t, errors.Join(firstErr, secondErr))
	assert.Equal(t, start, firstNext)
	assert.Equal(t, start, secondNext)

	// given: the content changes
	f.summaries.On("Ledger", mock.Anything).Return(ledgerOf("Druid")).Once()
	f.expectPost(ledgerOf("Druid"))

	// when
	_, err := f.b.PostLegacySummary(context.Background(), &cfg)

	// then
	require.NoError(t, err)
}

func TestPostLegacySummary_EmptyGuildGetsTheEmptySummaryOnce(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.GuildID, cfg.SummaryChannelID = "g1", "c1"
	f.channels(&discordgo.Channel{ID: "c1", Name: "hunts"})
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c1", mock.Anything).Return(&discordgo.Message{}, nil).Once()

	// when
	next, err := f.b.PostLegacySummary(context.Background(), &cfg)
	_, again := f.b.PostLegacySummary(context.Background(), &cfg)

	// then
	require.NoError(t, errors.Join(err, again))
	assert.True(t, next.IsZero())
}

func TestPostLegacySummary_AFailedPostIsRetried(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.GuildID = "g1"
	f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return(upcomingReservations(time.Now()), nil)
	f.summaries.On("Ledger", mock.Anything).Return(ledgerOf("Knight"))
	f.summaries.On("PrepareSummary", mock.Anything).Return(&summary.Summary{Ledger: ledgerOf("Knight")}, nil)
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return(nil, errors.New("discord down")).Twice()

	// when
	_, first := f.b.PostLegacySummary(context.Background(), &cfg)
	_, second := f.b.PostLegacySummary(context.Background(), &cfg)

	// then
	assert.Error(t, first)
	assert.Error(t, second, "nothing was stored, so the next pass posts again")
}

func TestReconcileLegacy_SkipsWithoutDiscordCalls(t *testing.T) {
	cases := map[string]guildconfig.Config{
		"not premium": {BotPresent: true},
		"bot left":    {Premium: true},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newDiscordFixture(t)
			f.config(cfg)

			// when
			res := f.b.reconcileLegacy(context.Background(), "g1")

			// then
			assert.Equal(t, reconcileResult{}, res)
		})
	}
}

func TestReconcileLegacy_RunsThePassHooksOfAPremiumGuild(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.config(premiumConfig())
	f.channels()
	f.writer.On("GuildChannels", "g1").Return(nil, errors.New("discord down"))
	var calls []string
	f.b.WithSummaryPassHook(func(_ context.Context, guildID string) error {
		calls = append(calls, guildID)
		return nil
	})

	// when
	f.b.reconcileLegacy(context.Background(), "g1")

	// then
	assert.Equal(t, []string{"g1"}, calls)
}

func TestReconcileLegacy_Errors(t *testing.T) {
	cases := map[string]func(f *discordFixture){
		"config": func(f *discordFixture) {
			f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down"))
		},
		"no summary channel": func(f *discordFixture) {
			f.config(premiumConfig())
			f.channels()
			f.writer.On("GuildChannels", "g1").Return(nil, errors.New("discord down"))
		},
		"reservations": func(f *discordFixture) {
			f.config(premiumConfig())
			f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
			f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return(nil, errors.New("db down"))
		},
		"prepare": func(f *discordFixture) {
			f.config(premiumConfig())
			f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
			f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return(upcomingReservations(time.Now()), nil)
			f.summaries.On("Ledger", mock.Anything).Return(ledgerOf("Knight"))
			f.summaries.On("PrepareSummary", mock.Anything).Return(nil, errors.New("chart down"))
		},
	}
	for name, given := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newDiscordFixture(t)
			given(f)

			// when
			res := f.b.reconcileLegacy(context.Background(), "g1")

			// then
			assert.Error(t, res.Err)
		})
	}
}

func TestForgetLegacyChannel(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.b.legacyHashes.Set("g1/c1", "hash")

	// when
	known := f.b.forgetLegacyChannel("g1", "c1")
	unknown := f.b.forgetLegacyChannel("g1", "c1")

	// then
	assert.True(t, known)
	assert.False(t, unknown)
}

func TestSplitByAge(t *testing.T) {
	// given
	now := time.Now()
	young := snowflakeAt(now.Add(-time.Hour))
	edge := snowflakeAt(now.Add(-bulkDeleteMaxAge + time.Minute))
	old := snowflakeAt(now.Add(-15 * 24 * time.Hour))

	// when
	gotYoung, gotOld := splitByAge([]string{young, old, edge, "not-a-snowflake"}, now)

	// then
	assert.Equal(t, []string{young, edge}, gotYoung)
	assert.Equal(t, []string{old, "not-a-snowflake"}, gotOld)
}

func TestCleanChannel_BulkDeletesTheYoungAndDeletesTheOldOneByOne(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	now := time.Now()
	young1, young2 := snowflakeAt(now.Add(-time.Hour)), snowflakeAt(now.Add(-2*time.Hour))
	old1, old2 := snowflakeAt(now.Add(-20*24*time.Hour)), snowflakeAt(now.Add(-30*24*time.Hour))
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{{ID: young1}, {ID: old1}, {ID: young2}, {ID: old2}}, nil)
	f.writer.On("ChannelMessagesBulkDelete", "c1", []string{young1, young2}).Return(nil).Once()
	f.writer.On("ChannelMessageDelete", "c1", old1).Return(nil).Once()
	f.writer.On("ChannelMessageDelete", "c1", old2).Return(&discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusNotFound},
		Message:  &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownMessage},
	}).Once()
	var waits int
	f.b.pacer.sleep = func(context.Context, time.Duration) error { waits++; return nil }
	f.b.pacer.now = func() time.Time { return now }

	// when
	err := f.b.CleanChannel(context.Background(), &discord.Channel{ID: "c1", Name: "letter-summary"})

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, waits, "every write after the first waits for the gap")
	f.metrics.AssertCalled(t, "AddMessagesDeleted", "c1", "letter-summary", 4)
}

func TestCleanChannel_StopsOnADeleteError(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	old := snowflakeAt(time.Now().Add(-20 * 24 * time.Hour))
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{{ID: old}}, nil)
	f.writer.On("ChannelMessageDelete", "c1", old).Return(errors.New("missing access")).Once()

	// when
	err := f.b.CleanChannel(context.Background(), &discord.Channel{ID: "c1"})

	// then
	assert.ErrorContains(t, err, "missing access")
}

func TestFindChannel_AsksRESTOnlyWhenTheCacheMisses(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	g := &guildconfig.Config{GuildID: "g1"}
	f.gateway.On("StateChannels", "g1").Return([]*discordgo.Channel{{ID: "c1", Name: "letter"}}, nil)
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{{ID: "c1", Name: "letter"}, {ID: "c2", Name: "new"}}, nil).Once()

	// when
	cached, cachedErr := f.b.findChannel(g.GuildID, func(c *discordgo.Channel) bool { return c.ID == "c1" })
	fetched, fetchedErr := f.b.findChannel(g.GuildID, func(c *discordgo.Channel) bool { return c.ID == "c2" })

	// then
	require.NoError(t, errors.Join(cachedErr, fetchedErr))
	assert.Equal(t, "c1", cached.ID)
	assert.Equal(t, "c2", fetched.ID)
}

func stateWithGuilds(t *testing.T, ids ...string) *shards.Manager {
	t.Helper()
	session, err := discordgo.New("Bot x")
	require.NoError(t, err)
	for _, id := range ids {
		require.NoError(t, session.State.GuildAdd(&discordgo.Guild{ID: id}))
	}
	return &shards.Manager{Shards: []*shards.Shard{{Session: session}, {}}}
}

func TestRefreshAllGuildLetters_QueuesThePremiumGuilds(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.b.mgr = stateWithGuilds(t, "g1", "g2")
	f.b.started.Store(true)
	f.configs.On("ListPresentByIDs", mock.Anything, []string{"g1", "g2"}).Return([]*guildconfig.Config{
		{GuildID: "g1", Premium: true}, {GuildID: "g2"},
	}, nil)

	// when
	f.b.RefreshAllGuildLetters(context.Background())

	// then
	require.Eventually(t, func() bool {
		f.b.summaries.mu.Lock()
		defer f.b.summaries.mu.Unlock()
		return len(f.b.summaries.queue) == 1
	}, 5*time.Second, time.Millisecond)
	assert.Equal(t, []string{"g1"}, f.b.summaries.queue)
}

func TestRefreshAllGuildLetters_DoesNothingBeforeTheStart(t *testing.T) {
	// given
	f := newDiscordFixture(t)

	// when
	f.b.RefreshAllGuildLetters(context.Background())

	// then: the config mock fails on any call
}

func TestStartTicking_StartsOneTicker(t *testing.T) {
	// given
	core, logs := observer.New(zapcore.InfoLevel)
	f := newDiscordFixture(t)
	f.b.log = zap.New(core).Sugar()

	// when
	f.b.StartTicking()
	f.b.StartTicking()
	close(f.b.quit)

	// then
	require.Eventually(t, func() bool {
		return logs.FilterMessage("shutting down bot ticker").Len() == 1
	}, 5*time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, logs.FilterMessage("ticker started").Len())
	assert.Equal(t, 1, logs.FilterMessage("shutting down bot ticker").Len())
}

func TestShardGateway_StateChannelsIsACopy(t *testing.T) {
	// given
	session, err := discordgo.New("Bot x")
	require.NoError(t, err)
	require.NoError(t, session.State.GuildAdd(&discordgo.Guild{ID: "1", Channels: []*discordgo.Channel{{ID: "c1"}}}))
	gw := shardGateway{mgr: &shards.Manager{Shards: []*shards.Shard{{Session: session}}, ShardCount: 1}}

	// when
	channels, err := gw.StateChannels("1")
	channels[0] = &discordgo.Channel{ID: "changed"}
	_, missing := gw.StateChannels("2")
	_, invalid := gw.StateChannels("x")

	// then
	require.NoError(t, err)
	cached, _ := session.State.Guild("1")
	assert.Equal(t, "c1", cached.Channels[0].ID)
	assert.ErrorIs(t, missing, discordgo.ErrStateNotFound)
	assert.Error(t, invalid)
}

func TestPostLegacySummary_AFailedEmbedFailsThePass(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.GuildID = "g1"
	f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return(upcomingReservations(time.Now()), nil)
	big := make(summary.Ledger, 14)
	for i := range big {
		big[i] = summary.LedgerEntry{Spot: strconv.Itoa(i), Bookings: []*summary.Booking{{Author: "Knight"}}}
	}
	f.summaries.On("Ledger", mock.Anything).Return(big)
	f.summaries.On("PrepareSummary", mock.Anything).Return(&summary.Summary{Ledger: big}, nil)
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{}, nil)
	f.writer.On("ChannelFileSend", "c1", "spots.png", mock.Anything).Return(&discordgo.Message{}, nil)
	f.writer.On("ChannelMessageSendEmbed", "c1", mock.Anything).Return(nil, errors.New("embed too large")).Once()
	f.writer.On("ChannelMessageSendComplex", "c1", mock.Anything).Return(&discordgo.Message{}, nil).Once()

	// when
	_, err := f.b.PostLegacySummary(context.Background(), &cfg)

	// then
	assert.ErrorContains(t, err, "embed too large")
	assert.False(t, f.b.legacyHashes.Has("g1/c1"), "the next pass posts again")
}
