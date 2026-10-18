package bot

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/dto/summarylayout"
	"spot-assistant/internal/core/regions"
)

type regionFixture struct {
	*discordFixture

	layouts     *mocks.MockSummaryLayoutRepository
	now         time.Time
	savedRows   []summarylayout.RegionChannel
	savedLayout []summarylayout.Layout
	sleeps      []time.Duration
}

func newRegionFixture(t *testing.T) *regionFixture {
	f := &regionFixture{
		discordFixture: newDiscordFixture(t),
		layouts:        mocks.NewMockSummaryLayoutRepository(t),
		now:            time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}
	f.b.layouts = f.layouts
	f.b.now = func() time.Time { return f.now }
	f.b.pacer = &pacer{now: func() time.Time { return f.now }, sleep: func(_ context.Context, d time.Duration) error {
		f.sleeps = append(f.sleeps, d)
		return nil
	}}
	f.layouts.On("SaveRegionChannel", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			f.savedRows = append(f.savedRows, args.Get(1).(summarylayout.RegionChannel))
		}).
		Return(nil).Maybe()
	f.layouts.On("SaveLayout", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { f.savedLayout = append(f.savedLayout, args.Get(1).(summarylayout.Layout)) }).
		Return(nil).Maybe()
	return f
}

func regionConfig() guildconfig.Config {
	cfg := premiumConfig()
	cfg.RegionSummaries = true
	return cfg
}

func (f *regionFixture) guild(cfg guildconfig.Config, layout summarylayout.Layout, rows ...summarylayout.RegionChannel) {
	f.config(cfg)
	f.layouts.On("GetLayout", mock.Anything, "g1").Return(&layout, nil)
	if rows == nil {
		rows = []summarylayout.RegionChannel{}
	}
	f.layouts.On("ListRegionChannels", mock.Anything, "g1").Return(rows, nil)
}

func (f *regionFixture) desired(rs ...summary.RegionSummary) {
	f.spots.On("SelectGuildSpots", mock.Anything, "g1", false).Return([]*spot.Spot{}, nil)
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("PrepareRegionSummaries", mock.Anything, mock.Anything).Return(rs)
}

func (f *regionFixture) lastRow(t *testing.T) summarylayout.RegionChannel {
	t.Helper()
	require.NotEmpty(t, f.savedRows)
	return f.savedRows[len(f.savedRows)-1]
}

func (f *regionFixture) lastLayout(t *testing.T) summarylayout.Layout {
	t.Helper()
	require.NotEmpty(t, f.savedLayout)
	return f.savedLayout[len(f.savedLayout)-1]
}

func discordErr(code int) error {
	return &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusBadRequest},
		Message:  &discordgo.APIErrorMessage{Code: code},
	}
}

var (
	portHope = summary.RegionSummary{Region: regions.Of("port-hope"), Spots: []string{"Banuta"}}
	zao      = summary.RegionSummary{Region: regions.Of("zao"), Spots: []string{"Lizard City"}}
)

func category() *discordgo.Channel {
	return &discordgo.Channel{ID: "cat", Name: regions.CategoryName, Type: discordgo.ChannelTypeGuildCategory}
}

func regionChannel(id, key string) *discordgo.Channel {
	return &discordgo.Channel{ID: id, Name: "letter-" + key, Type: discordgo.ChannelTypeGuildText, ParentID: "cat"}
}

func summaryChannelOf() *discordgo.Channel {
	return &discordgo.Channel{ID: "c-sum", Name: "letter-summary", Type: discordgo.ChannelTypeGuildText}
}

// readyShards is the manager as in production: the REST session never
// connects, and only the shard sessions get Ready.
func readyShards(t *testing.T, botID string) *shards.Manager {
	t.Helper()
	gateway, err := discordgo.New("Bot x")
	require.NoError(t, err)
	shard, err := discordgo.New("Bot x")
	require.NoError(t, err)
	require.NoError(t, shard.State.OnInterface(shard, &discordgo.Ready{User: &discordgo.User{ID: botID}}))
	return &shards.Manager{Gateway: gateway, Shards: []*shards.Shard{{Session: shard}}, ShardCount: 1}
}

func (f *regionFixture) hashOf(rs summary.RegionSummary) string {
	return payloadHash(regionMessages(rs, f.b.webBaseURL))
}

func TestReconcileSummaries_LeavesAGuildWithoutPremiumOrTheBotAlone(t *testing.T) {
	for name, cfg := range map[string]guildconfig.Config{
		"no premium": {BotPresent: true, RegionSummaries: true},
		"bot absent": {Premium: true, RegionSummaries: true},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			f := newRegionFixture(t)
			f.config(cfg)

			// when
			res := f.b.reconcileSummaries(context.Background(), "g1")

			// then: the strict mocks fail on any other call
			assert.Equal(t, reconcileResult{}, res)
		})
	}
}

func TestReconcileSummaries_MakesTheCategoryFirst(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{})
	f.desired(portHope)
	f.channels()
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{}, nil).Once()
	f.writer.On("GuildChannelCreateComplex", "g1", discordgo.GuildChannelCreateData{Name: regions.CategoryName, Type: discordgo.ChannelTypeGuildCategory}, mock.Anything).
		Return(category(), nil).Once()
	f.b.pacer.last = f.now

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, channelOpGap, res.Again)
	assert.Equal(t, "cat", f.lastLayout(t).CategoryID)
	assert.Equal(t, []time.Duration{channelOpGap}, f.sleeps)
}

func TestReconcileSummaries_MakesOneChannelPerPass(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{})
	f.desired(portHope, zao)
	f.channels(category())
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{category()}, nil).Once()
	f.writer.On("GuildChannelCreateComplex", "g1", mock.MatchedBy(func(d discordgo.GuildChannelCreateData) bool {
		return d.Name == "letter-port-hope" && d.Type == discordgo.ChannelTypeGuildText && d.ParentID == "cat" &&
			d.Topic == "Letter: upcoming hunts in Port Hope & Tiquanda. Managed by the bot."
	}), mock.Anything).Return(regionChannel("c-ph", "port-hope"), nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, channelOpGap, res.Again)
	assert.Equal(t, "cat", f.lastLayout(t).CategoryID, "the category of the name is adopted")
	assert.Equal(t, summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph"}, f.lastRow(t))
}

func TestReconcileSummaries_AdoptsAChannelThenWritesTheRegionAndTheIndex(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat"})
	f.desired(portHope)
	f.channels(category(), regionChannel("c-ph", "port-hope"), summaryChannelOf())
	f.writer.On("ChannelMessageSendComplex", "c-ph", mock.Anything).Return(&discordgo.Message{ID: "m1"}, nil).Once()
	f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{ID: "idx"}, nil).Once()
	f.b.legacyHashes.Set("g1/c-sum", "old")

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Zero(t, res.Again)
	assert.Equal(t, summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)}, f.lastRow(t))
	layout := f.lastLayout(t)
	assert.Equal(t, "c-sum", layout.IndexChannelID)
	assert.Equal(t, "idx", layout.IndexMessageID)
	assert.NotEmpty(t, layout.IndexHash)
	assert.False(t, f.b.legacyHashes.Has("g1/c-sum"), "a switch back posts the single summary again")
}

// unchanged returns the stored state of two regions whose index is up to date.
func (f *regionFixture) unchanged(zaoHash string) {
	rows := []summarylayout.RegionChannel{
		{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)},
		{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2", "m3"}, ContentHash: zaoHash},
	}
	byKey := map[string]*summarylayout.RegionChannel{"port-hope": &rows[0], "zao": &rows[1]}
	index := payloadHash([]*discordgo.MessageSend{indexMessage([]summary.RegionSummary{portHope, zao}, byKey, f.b.webBaseURL)})
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: index}, rows...)
	f.desired(portHope, zao)
	f.channels(category(), regionChannel("c-ph", "port-hope"), regionChannel("c-zao", "zao"), summaryChannelOf())
}

func TestReconcileSummaries_NothingChangedWritesNothing(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged(f.hashOf(zao))

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: the strict writer mock fails on any write
	require.NoError(t, res.Err)
	assert.Empty(t, f.savedRows)
	assert.Empty(t, f.savedLayout)
}

func TestReconcileSummaries_EditsOnlyTheChangedRegion(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged("stale")
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool {
		return m.Channel == "c-zao" && m.ID == "m2" && len(m.Embeds) == 1 && m.Components != nil
	})).Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageDelete", "c-zao", "m3").Return(nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	require.Len(t, f.savedRows, 1)
	assert.Equal(t, []string{"m2"}, f.savedRows[0].MessageIDs)
	assert.Equal(t, f.hashOf(zao), f.savedRows[0].ContentHash)
}

func TestReconcileSummaries_AMessageDeletedByHandIsSentAgain(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged("stale")
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeUnknownMessage)).Once()
	f.writer.On("ChannelMessageSendComplex", "c-zao", mock.Anything).Return(&discordgo.Message{ID: "m9"}, nil).Once()
	f.writer.On("ChannelMessageDelete", "c-zao", "m3").Return(discordErr(discordgo.ErrCodeUnknownMessage)).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"m9"}, f.lastRow(t).MessageIDs)
}

func TestReconcileSummaries_AChannelDeletedByHandIsDropped(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged("stale")
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeUnknownChannel)).Once()
	f.layouts.On("DeleteRegionChannel", mock.Anything, "g1", "zao").Return(nil).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool {
		return m.ID == "idx" && !strings.Contains(m.Embeds[0].Description, "c-zao")
	})).Return(&discordgo.Message{}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: the next pass makes the channel again
	require.NoError(t, res.Err)
	assert.Equal(t, channelOpGap, res.Again)
}

func TestReconcileSummaries_AFailedWriteIsRetried(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged("stale")
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(nil, errors.New("discord down")).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	assert.Error(t, res.Err)
	assert.Equal(t, summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2", "m3"}}, f.lastRow(t))
}

func TestReconcileSummaries_RevivesARetiredRegion(t *testing.T) {
	// given
	f := newRegionFixture(t)
	retired := f.now.Add(-time.Hour)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, RetiredAt: &retired})
	f.desired(portHope)
	f.channels(category(), regionChannel("c-ph", "port-hope"), summaryChannelOf())
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{ID: "idx"}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Nil(t, f.savedRows[0].RetiredAt)
	assert.Equal(t, f.hashOf(portHope), f.lastRow(t).ContentHash)
}

func TestReconcileSummaries_RetiresThenDeletesAnUnneededRegion(t *testing.T) {
	// given
	f := newRegionFixture(t)
	index := payloadHash([]*discordgo.MessageSend{indexMessage([]summary.RegionSummary{portHope}, map[string]*summarylayout.RegionChannel{
		"port-hope": {ChannelID: "c-ph"},
	}, f.b.webBaseURL)})
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: index},
		summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "x"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)})
	f.desired(portHope)
	f.channels(category(), regionChannel("c-ph", "port-hope"), regionChannel("c-zao", "zao"), summaryChannelOf())
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool {
		return m.ID == "m2" && len(m.Components) == 0 && m.Embeds[0].Description == retiredMessage().Embeds[0].Description
	})).Return(&discordgo.Message{}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	retired := f.lastRow(t)
	require.NotNil(t, retired.RetiredAt)
	assert.Equal(t, f.now, *retired.RetiredAt)
	assert.Empty(t, retired.ContentHash)
	assert.Equal(t, retiredChannelTTL, res.Again, "the worker comes back for the delete")
}

func TestReconcileSummaries_DeletesARegionRetiredADayAgoThenTheCategory(t *testing.T) {
	// given
	f := newRegionFixture(t)
	dayAgo := f.now.Add(-retiredChannelTTL)
	cfg := regionConfig()
	cfg.RegionSummaries = false
	f.guild(cfg, summarylayout.Layout{CategoryID: "cat"}, summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", RetiredAt: &dayAgo})
	f.channels(summaryChannelOf())
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.b.legacyHashes.Set("g1/c-sum", summary.Ledger{}.Hash())
	f.writer.On("ChannelDelete", "c-zao", mock.Anything).Return(nil, nil).Once()
	f.layouts.On("DeleteRegionChannel", mock.Anything, "g1", "zao").Return(nil).Once()

	// when
	first := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, first.Err)
	assert.Equal(t, channelOpGap, first.Again)

	// given: the next pass finds no region channel left
	f.layouts.On("ListRegionChannels", mock.Anything, "g1").Unset()
	f.layouts.On("ListRegionChannels", mock.Anything, "g1").Return([]summarylayout.RegionChannel{}, nil)
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{category(), summaryChannelOf()}, nil).Once()
	f.writer.On("ChannelDelete", "cat", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeUnknownChannel)).Once()

	// when
	second := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, second.Err)
	assert.Empty(t, f.lastLayout(t).CategoryID)
}

func TestReconcileSummaries_LeavesACategoryInUseToTheGuild(t *testing.T) {
	// given
	f := newRegionFixture(t)
	cfg := regionConfig()
	cfg.RegionSummaries = false
	f.guild(cfg, summarylayout.Layout{CategoryID: "cat"})
	f.channels(summaryChannelOf())
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.b.legacyHashes.Set("g1/c-sum", summary.Ledger{}.Hash())
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{category(), {ID: "own", ParentID: "cat"}}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: no delete, and the bot forgets the category
	require.NoError(t, res.Err)
	assert.Empty(t, f.lastLayout(t).CategoryID)
}

func TestReconcileSummaries_RefusedChannelsKeepTheSingleSummaryForAnHour(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{})
	f.desired(portHope)
	f.channels(summaryChannelOf())
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{summaryChannelOf()}, nil).Once()
	f.b.mgr = readyShards(t, "bot")
	var overwrites [][]*discordgo.PermissionOverwrite
	f.writer.On("GuildChannelCreateComplex", "g1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			overwrites = append(overwrites, args.Get(1).(discordgo.GuildChannelCreateData).PermissionOverwrites)
		}).
		Return(nil, discordErr(discordgo.ErrCodeMissingPermissions)).Twice()
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.b.legacyHashes.Set("g1/c-sum", summary.Ledger{}.Hash())

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	require.Len(t, overwrites, 2)
	assert.Len(t, overwrites[0], 2, "first with the overwrites")
	assert.Equal(t, "g1", overwrites[0][0].ID)
	assert.Equal(t, "bot", overwrites[0][1].ID)
	assert.Nil(t, overwrites[1], "then without")
	blocked := f.lastLayout(t).BlockedUntil
	require.NotNil(t, blocked)
	assert.Equal(t, f.now.Add(blockedRetry), *blocked)
	assert.Equal(t, blockedRetry, res.Again)
}

func TestReconcileSummaries_AGuildWithoutRegionsGetsTheSingleSummary(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "h"})
	f.desired()
	f.channels(summaryChannelOf())
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, summarylayout.Layout{GuildID: "g1"}, f.lastLayout(t), "the single summary replaced the index")
}

func TestReconcileSummaries_RunsThePassHooks(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged(f.hashOf(zao))
	var calls []string
	f.b.WithSummaryPassHook(func(_ context.Context, guildID string) error {
		calls = append(calls, guildID)
		return errors.New("queue down")
	})

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"g1"}, calls)
}

func TestReconcileSummaries_SendsTheDueQueueOffersPaced(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.unchanged(f.hashOf(zao))
	q := mocks.NewMockSpotQueueService(t)
	f.b.WithSpotQueue(q)
	q.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{{ID: 9, GuildID: "g1", SpotName: "Lizard City", UserID: "u1", Length: time.Hour}}, nil)
	q.EXPECT().ClaimOffer(mock.Anything, int64(9)).Return(f.now, true, nil)
	f.writer.On("UserChannelCreate", "u1").Return(&discordgo.Channel{ID: "dm1"}, nil)
	f.writer.On("ChannelMessageSendComplex", "dm1", mock.Anything).Return(&discordgo.Message{}, nil)

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, []time.Duration{messageOpGap}, f.sleeps, "the DM waits a gap after opening the DM channel")
}

func TestReconcileSummaries_StoreErrors(t *testing.T) {
	t.Run("layout", func(t *testing.T) {
		// given
		f := newRegionFixture(t)
		f.config(regionConfig())
		f.layouts.On("GetLayout", mock.Anything, "g1").Return(nil, errors.New("db down"))

		// when
		res := f.b.reconcileSummaries(context.Background(), "g1")

		// then
		assert.ErrorContains(t, res.Err, "db down")
	})
	t.Run("channels", func(t *testing.T) {
		// given
		f := newRegionFixture(t)
		f.config(regionConfig())
		f.layouts.On("GetLayout", mock.Anything, "g1").Return(&summarylayout.Layout{}, nil)
		f.layouts.On("ListRegionChannels", mock.Anything, "g1").Return(nil, errors.New("db down"))

		// when
		res := f.b.reconcileSummaries(context.Background(), "g1")

		// then
		assert.ErrorContains(t, res.Err, "db down")
	})
	t.Run("config", func(t *testing.T) {
		// given
		f := newRegionFixture(t)
		f.configs.On("Get", mock.Anything, "g1").Return(nil, errors.New("db down"))

		// when
		res := f.b.reconcileSummaries(context.Background(), "g1")

		// then
		assert.ErrorContains(t, res.Err, "db down")
	})
}

func TestIsSummaryChannel(t *testing.T) {
	assert.True(t, isSummaryChannel(&discordgo.Channel{Name: "letter-zao"}))
	assert.True(t, isSummaryChannel(&discordgo.Channel{Name: regions.CategoryName}))
	assert.False(t, isSummaryChannel(&discordgo.Channel{Name: "general"}))
	assert.False(t, isSummaryChannel(nil))
}

func TestSummaryOverwrites_NeedTheBotUser(t *testing.T) {
	// given: only the REST session, which never gets Ready
	f := newRegionFixture(t)
	f.b.mgr = readyShards(t, "bot")
	f.b.mgr.Shards = nil

	// when
	without := f.b.summaryOverwrites("g1")

	// then
	assert.Nil(t, without)
	assert.Empty(t, f.b.botUserID())
}

func TestReconcileSummaries_EditsTheIndexInPlace(t *testing.T) {
	for name, editErr := range map[string]error{
		"edited":            nil,
		"deleted by hand":   discordErr(discordgo.ErrCodeUnknownMessage),
		"discord is broken": errors.New("discord down"),
	} {
		t.Run(name, func(t *testing.T) {
			// given
			f := newRegionFixture(t)
			rows := []summarylayout.RegionChannel{{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)}}
			f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "old"}, rows...)
			f.desired(portHope)
			f.channels(category(), regionChannel("c-ph", "port-hope"), summaryChannelOf())
			f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "idx" })).Return(nil, editErr).Once()
			if isDiscordCode(editErr, discordgo.ErrCodeUnknownMessage) {
				f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
				f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{ID: "idx2"}, nil).Once()
			}

			// when
			res := f.b.reconcileSummaries(context.Background(), "g1")

			// then
			switch name {
			case "discord is broken":
				assert.Error(t, res.Err)
			case "deleted by hand":
				require.NoError(t, res.Err)
				assert.Equal(t, "idx2", f.lastLayout(t).IndexMessageID)
			default:
				require.NoError(t, res.Err)
				assert.Equal(t, "idx", f.lastLayout(t).IndexMessageID)
				assert.NotEqual(t, "old", f.lastLayout(t).IndexHash)
			}
		})
	}
}

// legacyWithRows is a guild that switched the region summaries off and still
// has the given region channels.
func (f *regionFixture) legacyWithRows(rows ...summarylayout.RegionChannel) {
	cfg := regionConfig()
	cfg.RegionSummaries = false
	f.guild(cfg, summarylayout.Layout{CategoryID: "cat"}, rows...)
}

func TestReconcileSummaries_AFailedSingleSummaryStillRetiresTheRegions(t *testing.T) {
	// given: the summary channel is gone
	f := newRegionFixture(t)
	f.legacyWithRows(summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "x"})
	f.channels(category(), regionChannel("c-zao", "zao"))
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{category(), regionChannel("c-zao", "zao")}, nil).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" })).
		Return(&discordgo.Message{}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	assert.Error(t, res.Err)
	require.NotNil(t, f.lastRow(t).RetiredAt)
	assert.Equal(t, retiredChannelTTL, res.Again)
}

func TestReconcileSummaries_AFailedIndexStillRetiresTheRegions(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "old"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)},
		summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "x"})
	f.desired(portHope)
	f.channels(category(), regionChannel("c-ph", "port-hope"), regionChannel("c-zao", "zao"), summaryChannelOf())
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "idx" })).
		Return(nil, errors.New("discord down")).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" })).
		Return(&discordgo.Message{}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	assert.ErrorContains(t, res.Err, "discord down")
	assert.Equal(t, "zao", f.lastRow(t).Region)
	assert.NotNil(t, f.lastRow(t).RetiredAt)
}

func TestReconcileSummaries_ABlockedPassKeepsTheFirstRetirement(t *testing.T) {
	// given: a retired region is needed again, but the next channel is refused
	f := newRegionFixture(t)
	retired := f.now.Add(-time.Hour)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, RetiredAt: &retired})
	f.desired(portHope, zao)
	f.channels(category(), regionChannel("c-ph", "port-hope"), summaryChannelOf())
	f.writer.On("GuildChannels", "g1").Return([]*discordgo.Channel{category(), regionChannel("c-ph", "port-hope"), summaryChannelOf()}, nil).Once()
	f.writer.On("GuildChannelCreateComplex", "g1", mock.Anything, mock.Anything).
		Return(nil, discordErr(discordgo.ErrCodeMaximumNumberOfGuildChannelsReached)).Once()
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.b.legacyHashes.Set("g1/c-sum", summary.Ledger{}.Hash())

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: no new notice and no new RetiredAt
	require.NoError(t, res.Err)
	assert.Empty(t, f.savedRows)
	assert.Equal(t, blockedRetry, res.Again)
}

// blockedGuild is a guild on the single summary for another 30 minutes, with
// the given region channels.
func (f *regionFixture) blockedGuild(rows ...summarylayout.RegionChannel) {
	until := f.now.Add(30 * time.Minute)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", BlockedUntil: &until}, rows...)
	f.channels(category(), regionChannel("c-ph", "port-hope"), regionChannel("c-zao", "zao"), summaryChannelOf())
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.b.legacyHashes.Set("g1/c-sum", summary.Ledger{}.Hash())
}

func TestReconcileSummaries_ABlockedGuildDeletesNoChannel(t *testing.T) {
	// given: a region retired two days ago, and a live one
	f := newRegionFixture(t)
	twoDaysAgo := f.now.Add(-2 * retiredChannelTTL)
	f.blockedGuild(
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, RetiredAt: &twoDaysAgo},
		summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "x"})
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" })).
		Return(&discordgo.Message{}, nil).Once()

	// when
	blocked := f.b.reconcileSummaries(context.Background(), "g1")

	// then: the strict writer mock fails on a ChannelDelete
	require.NoError(t, blocked.Err)
	assert.Equal(t, 30*time.Minute, blocked.Again, "the block timer, not the TTL")
	retiredZao := f.lastRow(t)
	assert.Equal(t, "zao", retiredZao.Region)
	require.NotNil(t, retiredZao.RetiredAt)

	// given: the block ended and port-hope is needed again
	f.now = f.now.Add(time.Hour)
	f.savedRows = nil
	f.layouts.On("ListRegionChannels", mock.Anything, "g1").Unset()
	f.layouts.On("ListRegionChannels", mock.Anything, "g1").Return([]summarylayout.RegionChannel{
		{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, RetiredAt: &twoDaysAgo},
		retiredZao,
	}, nil)
	f.desired(portHope)
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m1" })).
		Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{ID: "idx"}, nil).Once()

	// when
	unblocked := f.b.reconcileSummaries(context.Background(), "g1")

	// then: port-hope is written again, zao waits for the rest of its 24 hours
	require.NoError(t, unblocked.Err)
	require.Len(t, f.savedRows, 1)
	assert.Equal(t, "port-hope", f.savedRows[0].Region)
	assert.Nil(t, f.savedRows[0].RetiredAt)
	assert.Equal(t, f.hashOf(portHope), f.savedRows[0].ContentHash)
	assert.Equal(t, retiredChannelTTL-time.Hour, unblocked.Again)
}

func TestReconcileSummaries_ABlockedGuildRetriesAFailedNoticeAfterTheBlock(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.blockedGuild(summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "x"})
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(nil, discordErr(discordgo.ErrCodeMissingPermissions)).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: no error, so the worker does not back off
	require.NoError(t, res.Err)
	assert.Equal(t, 30*time.Minute, res.Again)
	assert.Nil(t, f.lastRow(t).RetiredAt)
}

func TestReconcileSummaries_ABlockedGuildKeepsItsEmptyCategory(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.blockedGuild()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: the strict writer mock fails on GuildChannels and ChannelDelete
	require.NoError(t, res.Err)
	assert.Empty(t, f.savedLayout)
	assert.Equal(t, 30*time.Minute, res.Again)
}

func TestReconcileSummaries_AFailedRegionDoesNotStopTheOthers(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: "stale"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}, ContentHash: "stale"})
	f.desired(portHope, zao)
	f.channels(category(), regionChannel("c-ph", "port-hope"), regionChannel("c-zao", "zao"), summaryChannelOf())
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m1" })).
		Return(nil, errors.New("discord down")).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" || m.ID == "idx" })).
		Return(&discordgo.Message{}, nil).Twice()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	assert.ErrorContains(t, res.Err, "port-hope")
	require.Len(t, f.savedRows, 2)
	assert.Empty(t, f.savedRows[0].ContentHash, "the failed region is written again on the retry")
	assert.Equal(t, f.hashOf(zao), f.savedRows[1].ContentHash)
}

func TestReconcileSummaries_RetriesARefusedRegionWithoutImages(t *testing.T) {
	// given
	f := newRegionFixture(t)
	withImage := summary.RegionSummary{Region: regions.Of("zao"), Spots: []string{"Lizard City"}, Ledger: summary.Ledger{
		{Spot: "Lizard City", ImageURL: "https://bad.example/x.png", Bookings: []*summary.Booking{{Author: "Knight"}}},
	}}
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat"},
		summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2"}})
	f.desired(withImage)
	f.channels(category(), regionChannel("c-zao", "zao"))
	var thumbnails []bool
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" })).
		Run(func(args mock.Arguments) {
			thumbnails = append(thumbnails, args.Get(0).(*discordgo.MessageEdit).Embeds[1].Thumbnail != nil)
		}).
		Return(nil, discordErr(discordgo.ErrCodeInvalidFormBody)).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m2" })).
		Run(func(args mock.Arguments) {
			thumbnails = append(thumbnails, args.Get(0).(*discordgo.MessageEdit).Embeds[1].Thumbnail != nil)
		}).
		Return(&discordgo.Message{}, nil).Once()
	f.writer.On("GuildChannels", "g1").Return(nil, errors.New("no summary channel"))

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, []bool{true, false}, thumbnails)
	assert.Equal(t, f.hashOf(withImage), f.lastRow(t).ContentHash, "the next pass does not try the image again")
}

func TestWriteMessages_ResendsFromAMessageDeletedByHand(t *testing.T) {
	// given
	f := newRegionFixture(t)
	p := &regionPass{b: f.b, ctx: context.Background()}
	msgs := []*discordgo.MessageSend{{Content: "0"}, {Content: "1"}, {Content: "2"}}
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m0" })).Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m1" })).
		Return(nil, discordErr(discordgo.ErrCodeUnknownMessage)).Once()
	var order []string
	f.writer.On("ChannelMessageDelete", "c1", mock.Anything).Run(func(args mock.Arguments) { order = append(order, "delete "+args.String(1)) }).Return(nil).Twice()
	f.writer.On("ChannelMessageSendComplex", "c1", mock.Anything).Run(func(args mock.Arguments) {
		order = append(order, "send "+args.Get(1).(*discordgo.MessageSend).Content)
	}).Return(func(_ string, m *discordgo.MessageSend, _ ...discordgo.RequestOption) *discordgo.Message {
		return &discordgo.Message{ID: "n" + m.Content}
	}, nil).Twice()

	// when
	ids, err := p.writeMessages("c1", []string{"m0", "m1", "m2", "m3"}, msgs)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"m0", "n1", "n2"}, ids)
	assert.Equal(t, []string{"delete m2", "delete m3", "send 1", "send 2"}, order)
}

func TestRetireRow_KeepsTheNoticeOnAFailedWrite(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.layouts.On("SaveRegionChannel", mock.Anything, mock.Anything).Unset()
	f.layouts.On("SaveRegionChannel", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			f.savedRows = append(f.savedRows, args.Get(1).(summarylayout.RegionChannel))
		}).
		Return(nil)
	p := &regionPass{b: f.b, ctx: context.Background(), cfg: &guildconfig.Config{GuildID: "g1"}, now: f.now, rows: map[string]*summarylayout.RegionChannel{}}
	row := &summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2", "m3"}, ContentHash: "x"}
	f.writer.On("ChannelMessageEditComplex", mock.Anything).Return(&discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageDelete", "c-zao", "m3").Return(errors.New("discord down")).Once()

	// when
	err := p.retireRow(row)

	// then
	assert.Error(t, err)
	assert.Equal(t, summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{"m2", "m3"}}, f.lastRow(t))
}

func TestReconcileSummaries_DeletesTheIndexOfTheFormerSummaryChannel(t *testing.T) {
	// given
	f := newRegionFixture(t)
	rows := []summarylayout.RegionChannel{{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: f.hashOf(portHope)}}
	f.guild(regionConfig(), summarylayout.Layout{CategoryID: "cat", IndexChannelID: "c-old", IndexMessageID: "idx", IndexHash: "h"}, rows...)
	f.desired(portHope)
	f.channels(category(), regionChannel("c-ph", "port-hope"), summaryChannelOf())
	f.writer.On("ChannelMessageDelete", "c-old", "idx").Return(nil).Once()
	f.writer.On("ChannelMessages", "c-sum", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c-sum", mock.Anything).Return(&discordgo.Message{ID: "idx2"}, nil).Once()

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	layout := f.lastLayout(t)
	assert.Equal(t, "c-sum", layout.IndexChannelID)
	assert.Equal(t, "idx2", layout.IndexMessageID)
}

func TestMessageDelete_OfASummaryMessageWritesTheRegionAgain(t *testing.T) {
	// given: a pass tracked the stored messages
	f := newRegionFixture(t)
	f.unchanged(f.hashOf(zao))
	require.NoError(t, f.b.reconcileSummaries(context.Background(), "g1").Err)

	// when
	f.b.MessageDelete(nil, &discordgo.MessageDelete{Message: &discordgo.Message{ID: "m1", ChannelID: "c-ph"}})

	// then
	assert.Eventually(t, func() bool {
		f.b.summaries.mu.Lock()
		defer f.b.summaries.mu.Unlock()
		return f.b.summaries.queued["g1"]
	}, 5*time.Second, time.Millisecond)

	// given
	f.writer.On("ChannelMessageEditComplex", mock.MatchedBy(func(m *discordgo.MessageEdit) bool { return m.ID == "m1" })).
		Return(nil, discordErr(discordgo.ErrCodeUnknownMessage)).Once()
	f.writer.On("ChannelMessageSendComplex", "c-ph", mock.Anything).Return(&discordgo.Message{ID: "m9"}, nil).Once()

	// when: the same content, but the message is gone
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"m9"}, f.lastRow(t).MessageIDs)
}

func TestMessageDeleteBulk_OfTheSingleSummaryPostsItAgain(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	cfg := premiumConfig()
	cfg.GuildID = "g1"
	f.channels(&discordgo.Channel{ID: "c1", Name: discord.SummaryChannel})
	f.reservations.On("SelectUpcomingReservationsWithSpot", mock.Anything, "g1").Return([]*reservation.ReservationWithSpot{}, nil)
	f.summaries.On("Ledger", mock.Anything).Return(summary.Ledger{})
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{}, nil).Once()
	f.writer.On("ChannelMessageSendComplex", "c1", mock.Anything).Return(&discordgo.Message{ID: "s1"}, nil).Once()
	_, err := f.b.PostLegacySummary(context.Background(), &cfg)
	require.NoError(t, err)

	// when
	f.b.MessageDeleteBulk(nil, &discordgo.MessageDeleteBulk{ChannelID: "c1", GuildID: "g1", Messages: []string{"other", "s1"}})

	// then
	assert.False(t, f.b.legacyHashes.Has("g1/c1"), "the next pass posts it again")
}

func TestMessageDelete_IgnoresOtherMessages(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.b.tracked.track("g1", "c1", []string{"s1"})
	f.b.legacyHashes.Set("g1/c1", "h")

	// when
	f.b.MessageDelete(nil, &discordgo.MessageDelete{Message: &discordgo.Message{ID: "s2", ChannelID: "c1"}})
	f.b.MessageDelete(nil, &discordgo.MessageDelete{Message: &discordgo.Message{ID: "s1", ChannelID: "c2"}})
	f.b.MessageDelete(nil, &discordgo.MessageDelete{})

	// then
	assert.True(t, f.b.legacyHashes.Has("g1/c1"))
	assert.False(t, f.b.tracked.takeStale("c1"))
}

func TestCleanChannel_ItsOwnDeletesAreNotDeletesByHand(t *testing.T) {
	// given
	f := newDiscordFixture(t)
	f.b.tracked.track("g1", "c1", []string{"s1"})
	f.writer.On("ChannelMessages", "c1", 100, "", "", "").Return([]*discordgo.Message{}, nil)

	// when
	require.NoError(t, f.b.CleanChannel(context.Background(), &discord.Channel{ID: "c1"}))
	f.b.MessageDeleteBulk(nil, &discordgo.MessageDeleteBulk{ChannelID: "c1", Messages: []string{"s1"}})

	// then
	_, hit := f.b.tracked.deleted("c1", []string{"s1"})
	assert.False(t, hit)
	assert.False(t, f.b.tracked.takeStale("c1"))
}

func TestMinPositive(t *testing.T) {
	assert.Equal(t, time.Second, minPositive(0, time.Second))
	assert.Equal(t, time.Second, minPositive(time.Second, 0))
	assert.Equal(t, time.Second, minPositive(time.Minute, time.Second))
	assert.Zero(t, minPositive(0, 0))
}
