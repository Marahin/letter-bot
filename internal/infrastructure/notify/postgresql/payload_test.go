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
