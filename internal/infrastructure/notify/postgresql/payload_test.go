package postgresql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

func TestOverbookedPayload_RoundTrip(t *testing.T) {
	// given
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	request := book.BookRequest{
		Guild:   &guild.Guild{ID: "g1", Name: "Celesta Community"},
		Member:  &member.Member{ID: "m1", Nick: "Knight", Username: "knight"},
		Spot:    "Hero Cave",
		StartAt: start,
		EndAt:   start.Add(2 * time.Hour),
	}
	res := &reservation.ClippedOrRemovedReservation{
		Original: &reservation.Reservation{ID: 7, Author: "Druid", AuthorDiscordID: "m2", StartAt: start.Add(-time.Hour), EndAt: start.Add(time.Hour), SpotID: 4, GuildID: "g1"},
		New:      []*reservation.Reservation{{ID: 7, Author: "Druid", AuthorDiscordID: "m2", StartAt: start.Add(-time.Hour), EndAt: start, SpotID: 4, GuildID: "g1"}},
	}

	// when
	data, err := NewOverbookedPayload(request, res).Encode()
	require.NoError(t, err)
	decoded, err := DecodeOverbookedPayload(data)
	require.NoError(t, err)

	// then
	assert.Equal(t, book.BookRequest{
		Guild:    &guild.Guild{ID: "g1"},
		Member:   &member.Member{ID: "m1", Nick: "Knight", Username: "knight"},
		Spot:     "Hero Cave",
		StartAt:  start,
		EndAt:    start.Add(2 * time.Hour),
		Overbook: true,
	}, decoded.BookRequest())
	assert.Equal(t, res, decoded.Reservation())
}

func TestNewOverbookedPayload_WithoutGuildOrMember(t *testing.T) {
	// given / when
	p := NewOverbookedPayload(book.BookRequest{Spot: "x"}, &reservation.ClippedOrRemovedReservation{})

	// then
	assert.Empty(t, p.GuildID)
	assert.Empty(t, p.MemberID)
}

func TestDecodeOverbookedPayload_Errors(t *testing.T) {
	tests := map[string]string{
		"not json":         "{",
		"missing guild":    `{"original":{"ID":1}}`,
		"missing original": `{"guild_id":"g1"}`,
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			// given / when
			_, err := DecodeOverbookedPayload([]byte(data))

			// then
			assert.Error(t, err)
		})
	}
}

func TestBookedPayload_RoundTrip(t *testing.T) {
	// given
	start := time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC)
	event := book.Booked{
		GuildID:         "g1",
		SpotID:          4,
		SpotName:        "not sent",
		StartAt:         start,
		EndAt:           start.Add(2 * time.Hour),
		Author:          "Knight",
		AuthorDiscordID: "u1",
		ActorID:         "m1",
		Overbooked:      2,
		Source:          book.SourceDiscord,
	}

	// when
	data, err := NewBookedPayload(event).Encode()
	require.NoError(t, err)
	decoded, err := DecodeBookedPayload(data)
	require.NoError(t, err)

	// then
	assert.JSONEq(t, `{"guild_id":"g1","spot_id":4,"start_at":"2026-10-19T18:30:00Z","end_at":"2026-10-19T20:30:00Z",
		"author":"Knight","author_discord_id":"u1","actor_id":"m1","overbooked":2}`, string(data))
	expected := event
	expected.SpotName, expected.Source = "", book.SourceWeb
	assert.Equal(t, expected, decoded.Event())
}

func TestDecodeBookedPayload_Errors(t *testing.T) {
	tests := map[string]string{
		"not json":      "{",
		"missing guild": `{"spot_id":1}`,
		"missing spot":  `{"guild_id":"g1"}`,
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			// given / when
			_, err := DecodeBookedPayload([]byte(data))

			// then
			assert.Error(t, err)
		})
	}
}
