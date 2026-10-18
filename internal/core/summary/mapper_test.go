package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"
)

var mapperNow = time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

func newMapperAdapter(t *testing.T) (*Adapter, *mocks.MockOnlineCheckService) {
	online := mocks.NewMockOnlineCheckService(t)
	adapter := NewAdapter(mocks.NewMockChartAdapter(t), online)
	adapter.now = func() time.Time { return mapperNow }
	return adapter, online
}

func TestMapReservation_ShowsTheStatusOfAnOngoingReservation(t *testing.T) {
	// given
	adapter, online := newMapperAdapter(t)
	input := &reservation.Reservation{
		Author:          "test author",
		AuthorDiscordID: "u1",
		StartAt:         mapperNow,
		EndAt:           mapperNow.Add(2 * time.Hour),
		GuildID:         "guild1",
	}
	online.EXPECT().PlayerStatus("guild1", "test author").Return(dto.Online)

	// when
	res := adapter.MapReservation(input)

	// then
	assert.Equal(t, &dto.Booking{
		Author:          "test author",
		AuthorDiscordID: "u1",
		Status:          dto.Online,
		StartAt:         input.StartAt,
		EndAt:           input.EndAt,
	}, res)
}

func TestMapReservation_HidesTheStatusOutsideTheReservation(t *testing.T) {
	cases := map[string]struct{ start, end time.Time }{
		"upcoming": {mapperNow.Add(time.Minute), mapperNow.Add(time.Hour)},
		"ended":    {mapperNow.Add(-time.Hour), mapperNow},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			adapter, _ := newMapperAdapter(t)

			// when
			res := adapter.MapReservation(&reservation.Reservation{Author: "a", StartAt: tc.start, EndAt: tc.end, GuildID: "guild1"})

			// then
			assert.Equal(t, dto.Unknown, res.Status)
		})
	}
}

func TestMapReservations(t *testing.T) {
	// given
	adapter, online := newMapperAdapter(t)
	input := []*reservation.Reservation{
		{Author: "test author", StartAt: mapperNow.Add(-time.Hour), EndAt: mapperNow.Add(time.Hour), GuildID: "guild1"},
		{Author: "test author 2", StartAt: mapperNow.Add(5 * time.Minute), EndAt: mapperNow.Add(3 * time.Hour), GuildID: "guild1"},
	}
	online.EXPECT().PlayerStatus("guild1", "test author").Return(dto.Offline)

	// when
	res := adapter.MapReservations(input)

	// then
	assert.Len(t, res, 2)
	for i, booking := range res {
		assert.Equal(t, input[i].Author, booking.Author)
		assert.Equal(t, input[i].StartAt, booking.StartAt)
		assert.Equal(t, input[i].EndAt, booking.EndAt)
	}
	assert.Equal(t, dto.Offline, res[0].Status)
	assert.Equal(t, dto.Unknown, res[1].Status)
}
