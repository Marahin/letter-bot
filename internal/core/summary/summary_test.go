package summary

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"
)

func TestBaseSummary(t *testing.T) {
	// given
	assert := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)

	// when
	summary := adapter.BaseSummary()

	// assert
	assert.NotNil(summary)
	assert.Equal(summary.URL, "https://tibialoot.com")
	assert.Equal(summary.Title, "TibiaLoot.com - Spot Assistant")
	assert.Equal(summary.Description, "Current and upcoming hunts. Times are in **Europe/Berlin**.")
	assert.Contains(summary.Footer, "powered by TibiaLoot.com")
}

func TestPrepareSummary(t *testing.T) {
	// given
	assert := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)
	input := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				Author:  "test author",
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
				GuildID: "guild1",
			},
			Spot: reservation.Spot{
				Name: "test-1",
			},
		},
		{
			Reservation: reservation.Reservation{
				Author:  "test author",
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
				GuildID: "guild1",
			},
			Spot: reservation.Spot{
				Name: "test-1",
			},
		},
		{
			Reservation: reservation.Reservation{
				Author:  "test author 2",
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
				GuildID: "guild1",
			},
			Spot: reservation.Spot{
				Name: "test-2",
			},
		},
	}
	spotsToReservations := adapter.mapToSpotsToReservations(input)
	spotsToCounts := adapter.mapToSpotsToCounts(spotsToReservations)
	lvs := adapter.mapToLegendValues(spotsToCounts)
	legend := collections.PoorMansMap(lvs, func(lv dto.LegendValue) string {
		return lv.Legend
	})
	values := collections.PoorMansMap(lvs, func(lv dto.LegendValue) float64 {
		return lv.Value
	})

	// mock PlayerStatus for authors
	mockOnlineCheckService.On("PlayerStatus", "guild1", "test author").Return(dto.Online)
	mockOnlineCheckService.On("PlayerStatus", "guild1", "test author 2").Return(dto.Offline)

	// when
	mockChartAdapter.On("NewChart", values, legend).Return([]byte{123}, nil)
	summary, err := adapter.PrepareSummary(input)

	// assert
	assert.Nil(err)
	assert.NotNil(summary)
	assert.Equal(summary.URL, "https://tibialoot.com")
	assert.Equal(summary.Title, "TibiaLoot.com - Spot Assistant")
	assert.Equal(summary.Description, "Current and upcoming hunts. Times are in **Europe/Berlin**.")
	assert.Contains(summary.Footer, "powered by TibiaLoot.com")
	assert.Len(summary.Ledger, 2)

	firstEntry := summary.Ledger[0]
	secondEntry := summary.Ledger[1]

	assert.Equal(firstEntry.Spot, "test-1")
	assert.Len(firstEntry.Bookings, 2)
	for _, entry := range firstEntry.Bookings {
		assert.NotNil(entry)
		assert.NotEmpty(entry.Author)
		assert.NotEmpty(entry.StartAt)
		assert.NotEmpty(entry.EndAt)
		assert.Equal(dto.Online, entry.Status)
	}

	assert.Equal(secondEntry.Spot, "test-2")
	assert.Len(secondEntry.Bookings, 1)
	for _, entry := range secondEntry.Bookings {
		assert.NotNil(entry)
		assert.NotEmpty(entry.Author)
		assert.NotEmpty(entry.StartAt)
		assert.NotEmpty(entry.EndAt)
		assert.Equal(dto.Offline, entry.Status)
	}
}

func TestPrepareSummaryTruncated(t *testing.T) {
	// given
	assert := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)

	input := []*reservation.ReservationWithSpot{}
	for ind := 0; ind < 2*MAX_CHART_RESPAWNS; ind++ {
		input = append(input, &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{
				Author:  fmt.Sprintf("test author %d", ind),
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
				GuildID: "guild1",
			},
			Spot: reservation.Spot{
				Name: fmt.Sprintf("%d", ind%(MAX_CHART_RESPAWNS)),
			},
		})
	}
	expectedOthers := &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			Author:  "test author",
			StartAt: time.Now(),
			EndAt:   time.Now().Add(2 * time.Hour),
			GuildID: "guild1",
		},
		Spot: reservation.Spot{
			Name: "This Should Become Others",
		},
	}
	input = append(input, expectedOthers)

	// mock PlayerStatus for all authors
	for _, r := range input {
		mockOnlineCheckService.On("PlayerStatus", "guild1", r.Author).Return(dto.Offline)
	}

	mockChartAdapter.On("NewChart", mock.AnythingOfType("[]float64"), mock.AnythingOfType("[]string")).Return([]byte{123}, nil)
	summary, err := adapter.PrepareSummary(input)

	// assert
	assert.Nil(err)
	assert.NotNil(summary)
	legendValuePtrs := collections.PoorMansMap(summary.LegendValues, func(lv dto.LegendValue) *dto.LegendValue {
		return &lv
	})
	otherEntry, index := collections.PoorMansFind(legendValuePtrs, func(lv *dto.LegendValue) bool {
		return lv.Legend == "Other"
	})
	assert.NotNil(otherEntry)
	assert.NotEqual(-1, index)
	assert.NotZero(otherEntry.Value)

	// check that all bookings have the correct status
	for _, ledgerEntry := range summary.Ledger {
		for _, booking := range ledgerEntry.Bookings {
			assert.Equal(dto.Offline, booking.Status)
		}
	}
}

func TestPrepareSpotSummarySuccess(t *testing.T) {
	assert := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)

	input := []*reservation.ReservationWithSpot{
		{Reservation: reservation.Reservation{Author: "a", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), GuildID: "g"}, Spot: reservation.Spot{Name: "Cobra Bastion"}},
		{Reservation: reservation.Reservation{Author: "b", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), GuildID: "g"}, Spot: reservation.Spot{Name: "Flimsy"}},
	}
	// mock player status
	mockOnlineCheckService.On("PlayerStatus", "g", "a").Return(dto.Offline)
	mockOnlineCheckService.On("PlayerStatus", "g", "b").Return(dto.Offline)

	// need to assert that the chart is created
	mockChartAdapter.On("NewChart", mock.AnythingOfType("[]float64"), mock.AnythingOfType("[]string")).Return([]byte{1}, nil)

	summ, err := adapter.PrepareSpotSummary(input, "Flimsy")
	assert.NoError(err)
	assert.Len(summ.Ledger, 1)
	assert.Equal("Flimsy", summ.Ledger[0].Spot)
}

func TestPrepareSpotSummaryNoReservations(t *testing.T) {
	assert := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)
	input := []*reservation.ReservationWithSpot{}
	_, err := adapter.PrepareSpotSummary(input, "gamma")
	assert.Error(err)
}
