package summary

import (
	"fmt"
	"strconv"
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
	is := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)

	// when
	summary := adapter.BaseSummary()

	// assert
	is.NotNil(summary)
	is.Equal(summary.URL, "https://tibialoot.com")
	is.Equal(summary.Title, "TibiaLoot.com - Spot Assistant")
	is.Equal(summary.Description, "Current and upcoming hunts. Times are in **Europe/Berlin**.")
	is.Contains(summary.Footer, "powered by TibiaLoot.com")
}

func TestPrepareSummary(t *testing.T) {
	// given
	is := assert.New(t)
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
	spotsToCounts := make(map[string]float64)
	for spot, val := range spotsToReservations {
		spotsToCounts[spot] = float64(len(val))
	}
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
	is.Nil(err)
	is.NotNil(summary)
	is.Equal(summary.URL, "https://tibialoot.com")
	is.Equal(summary.Title, "TibiaLoot.com - Spot Assistant")
	is.Equal(summary.Description, "Current and upcoming hunts. Times are in **Europe/Berlin**.")
	is.Contains(summary.Footer, "powered by TibiaLoot.com")
	is.Len(summary.Ledger, 2)

	firstEntry := summary.Ledger[0]
	secondEntry := summary.Ledger[1]

	is.Equal(firstEntry.Spot, "test-1")
	is.Len(firstEntry.Bookings, 2)
	for _, entry := range firstEntry.Bookings {
		is.NotNil(entry)
		is.NotEmpty(entry.Author)
		is.NotEmpty(entry.StartAt)
		is.NotEmpty(entry.EndAt)
		is.Equal(dto.Online, entry.Status)
	}

	is.Equal(secondEntry.Spot, "test-2")
	is.Len(secondEntry.Bookings, 1)
	for _, entry := range secondEntry.Bookings {
		is.NotNil(entry)
		is.NotEmpty(entry.Author)
		is.NotEmpty(entry.StartAt)
		is.NotEmpty(entry.EndAt)
		is.Equal(dto.Offline, entry.Status)
	}
}

func TestPrepareSummaryTruncated(t *testing.T) {
	// given
	is := assert.New(t)
	mockChartAdapter := new(mocks.MockChartAdapter)
	mockOnlineCheckService := new(mocks.MockOnlineCheckService)
	adapter := NewAdapter(mockChartAdapter, mockOnlineCheckService)

	input := []*reservation.ReservationWithSpot{}
	for ind := 0; ind < 2*maxChartRespawns; ind++ {
		input = append(input, &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{
				Author:  fmt.Sprintf("test author %d", ind),
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
				GuildID: "guild1",
			},
			Spot: reservation.Spot{
				Name: strconv.Itoa(ind % maxChartRespawns),
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
	is.Nil(err)
	is.NotNil(summary)
	legendValuePtrs := collections.PoorMansMap(summary.LegendValues, func(lv dto.LegendValue) *dto.LegendValue {
		return &lv
	})
	otherEntry, index := collections.PoorMansFind(legendValuePtrs, func(lv *dto.LegendValue) bool {
		return lv.Legend == "Other"
	})
	is.NotNil(otherEntry)
	is.NotEqual(-1, index)
	is.NotZero(otherEntry.Value)

	// check that all bookings have the correct status
	for _, ledgerEntry := range summary.Ledger {
		for _, booking := range ledgerEntry.Bookings {
			is.Equal(dto.Offline, booking.Status)
		}
	}
}
