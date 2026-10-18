package bot

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/dto/summarylayout"
	"spot-assistant/internal/core/regions"
)

var viewStart = time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

func bookings(n int) []*summary.Booking {
	out := make([]*summary.Booking, 0, n)
	for i := range n {
		start := viewStart.Add(time.Duration(i) * time.Hour)
		out = append(out, &summary.Booking{Author: fmt.Sprintf("Player %02d with a long name", i), StartAt: start, EndAt: start.Add(time.Hour)})
	}
	return out
}

func regionSummary(spots int, perSpot int) summary.RegionSummary {
	rs := summary.RegionSummary{Region: regions.Of("port-hope")}
	for i := range spots {
		name := fmt.Sprintf("Respawn %02d", i)
		rs.Spots = append(rs.Spots, name)
		rs.Ledger = append(rs.Ledger, summary.LedgerEntry{Spot: name, Bookings: bookings(perSpot)})
	}
	return rs
}

func sentCustomIDs(t *testing.T, components []discordgo.MessageComponent) []string {
	t.Helper()
	raw, err := json.Marshal(components)
	require.NoError(t, err)
	var rows []sentRow
	require.NoError(t, json.Unmarshal(raw, &rows))
	var ids []string
	for _, r := range rows {
		for _, c := range r.Components {
			ids = append(ids, c.CustomID)
		}
	}
	return ids
}

func TestRegionMessages_PacksWithinDiscordsLimitsWithTheButtonsLast(t *testing.T) {
	// given
	rs := regionSummary(30, 20)

	// when
	msgs := regionMessages(rs, "https://tibialoot.test")

	// then
	require.Greater(t, len(msgs), 1)
	embeds := 0
	for i, m := range msgs {
		chars := 0
		for _, e := range m.Embeds {
			chars += embedChars(e)
		}
		assert.LessOrEqual(t, len(m.Embeds), maxEmbedsPerMessage)
		assert.LessOrEqual(t, chars, maxMessageChars)
		if i < len(msgs)-1 {
			assert.Empty(t, m.Components)
		}
		embeds += len(m.Embeds)
	}
	assert.Equal(t, 31, embeds, "the header and one embed per booked respawn")
	assert.Equal(t, []string{"lf1:bookr:port-hope", "lf1:mine", ""}, sentCustomIDs(t, msgs[len(msgs)-1].Components))
	assert.Equal(t, "Port Hope & Tiquanda", msgs[0].Embeds[0].Title)
}

func TestRegionMessages_EmptyRegionAndSpotEmbeds(t *testing.T) {
	// given
	empty := summary.RegionSummary{Region: regions.Of("zao"), Spots: []string{"Lizard City", "WOTE 8"}}
	busy := summary.RegionSummary{Region: regions.Of("zao"), Spots: []string{"Lizard City"}, Ledger: summary.Ledger{
		{Spot: "Lizard City", ImageURL: "https://img.test/lc.png", Bookings: append(bookings(21), &summary.Booking{Author: "Knight", Status: summary.Online, StartAt: viewStart, EndAt: viewStart})},
	}}

	// when
	emptyMsgs := regionMessages(empty, "")
	busyMsgs := regionMessages(busy, "")

	// then
	require.Len(t, emptyMsgs, 1)
	require.Len(t, emptyMsgs[0].Embeds, 1)
	header := emptyMsgs[0].Embeds[0].Description
	assert.Contains(t, header, "No upcoming hunts in Zao & Farmine.")
	assert.Contains(t, header, "Respawns: Lizard City, WOTE 8")
	assert.Equal(t, []string{"lf1:bookr:zao", "lf1:mine"}, sentCustomIDs(t, emptyMsgs[0].Components))

	spot := busyMsgs[0].Embeds[1]
	assert.Equal(t, "Lizard City", spot.Title)
	require.NotNil(t, spot.Thumbnail)
	assert.Equal(t, "https://img.test/lc.png", spot.Thumbnail.URL)
	assert.Equal(t, 20, strings.Count(spot.Description, "\n"), "20 lines, then the rest counted")
	assert.True(t, strings.HasSuffix(spot.Description, "+2 more"))
	assert.Contains(t, busyMsgs[0].Embeds[0].Description, "**Online**")
}

func TestCutList(t *testing.T) {
	assert.Equal(t, "a, b", cutList([]string{"a", "b"}, 10))
	assert.Equal(t, "alpha and 2 more", cutList([]string{"alpha", "beta", "gamma"}, 8))
}

func TestPayloadHash_IgnoresTheUpdateTime(t *testing.T) {
	// given
	rs := regionSummary(2, 1)
	msgs := regionMessages(rs, "")
	before := payloadHash(msgs)

	// when
	stampUpdated(msgs, viewStart)
	again := payloadHash(regionMessages(rs, ""))
	changed := regionSummary(2, 2)

	// then
	assert.NotEmpty(t, before)
	assert.Equal(t, before, again)
	assert.Equal(t, viewStart.Format(time.RFC3339), msgs[0].Embeds[0].Timestamp)
	assert.NotEqual(t, before, payloadHash(regionMessages(changed, "")))
}

func TestIndexMessage_LinksTheRegionChannels(t *testing.T) {
	// given
	desired := []summary.RegionSummary{regionSummary(2, 2), {Region: regions.Of("zao")}, {Region: regions.Of("edron")}}
	desired[1].Ledger = summary.Ledger{{Spot: "Lizard City", Bookings: bookings(1)}}
	rows := map[string]*summarylayout.RegionChannel{
		"port-hope": {ChannelID: "c-ph"},
		"zao":       {ChannelID: "c-zao"},
	}

	// when
	msg := indexMessage(desired, rows, "https://tibialoot.test")

	// then
	desc := msg.Embeds[0].Description
	assert.Contains(t, desc, "<#c-ph> 4 upcoming hunts")
	assert.Contains(t, desc, "<#c-zao> 1 upcoming hunt")
	assert.NotContains(t, desc, "edron")
	assert.Equal(t, []string{"lf1:book", "lf1:mine", ""}, sentCustomIDs(t, msg.Components))
	assert.Equal(t, "no upcoming hunts", upcomingHunts(0))
}

func TestRetiredMessage_HasNoButtons(t *testing.T) {
	msg := retiredMessage()
	assert.Empty(t, msg.Components)
	assert.Contains(t, msg.Embeds[0].Description, "no longer used")
}
