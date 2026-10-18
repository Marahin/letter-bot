package bot

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
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

func TestReconcileSummaries_WithoutTheStoreKeepsTheSingleSummary(t *testing.T) {
	// given
	f := newRegionFixture(t)
	f.b.layouts = nil
	f.config(guildconfig.Config{Premium: true})

	// when
	res := f.b.reconcileSummaries(context.Background(), "g1")

	// then: the legacy pass stops at the bot's absence
	assert.Equal(t, reconcileResult{}, res)
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
	f.b.mgr.Gateway = &discordgo.Session{State: discordgo.NewState()}
	f.b.mgr.Gateway.State.User = &discordgo.User{ID: "bot"}
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
	offered := f.now
	q.EXPECT().DueOffers(mock.Anything, "g1").Return([]queue.Entry{{ID: 9, GuildID: "g1", SpotName: "Lizard City", UserID: "u1", Length: time.Hour, OfferedAt: &offered}}, nil)
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
	// given
	f := newRegionFixture(t)

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
