package summary

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"
	"spot-assistant/util"
)

func TestBaseSummary(t *testing.T) {
	// given
	assert := assert.New(t)
	mockChartAdapter := new(MockChartAdapter)
	adapter := NewAdapter(mockChartAdapter)

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
	mockChartAdapter := new(MockChartAdapter)
	adapter := NewAdapter(mockChartAdapter)
	input := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				Author:  "test author",
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
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
			},
			Spot: reservation.Spot{
				Name: "test-2",
			},
		},
	}
	spotsToReservations := adapter.mapToSpotsToReservations(input)
	spotsToCounts := adapter.mapToSpotsToCounts(spotsToReservations)
	lvs := adapter.mapToLegendValues(spotsToCounts)
	legend := util.PoorMansMap(lvs, func(lv dto.LegendValue) string {
		return lv.Legend
	})
	values := util.PoorMansMap(lvs, func(lv dto.LegendValue) float64 {
		return lv.Value
	})

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
	}

	assert.Equal(secondEntry.Spot, "test-2")
	assert.Len(secondEntry.Bookings, 1)
	for _, entry := range secondEntry.Bookings {
		assert.NotNil(entry)
		assert.NotEmpty(entry.Author)
		assert.NotEmpty(entry.StartAt)
		assert.NotEmpty(entry.EndAt)
	}
}

func TestPrepareSummaryTruncated(t *testing.T) {
	// given
	assert := assert.New(t)
	mockChartAdapter := new(MockChartAdapter)
	adapter := NewAdapter(mockChartAdapter)

	input := []*reservation.ReservationWithSpot{}
	for ind := 0; ind < 2*MAX_CHART_RESPAWNS; ind++ {
		input = append(input, &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{
				Author:  "test author",
				StartAt: time.Now(),
				EndAt:   time.Now().Add(2 * time.Hour),
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
		},
		Spot: reservation.Spot{
			Name: "This Should Become Others",
		},
	}
	input = append(input, expectedOthers)

	// when
	mockChartAdapter.On("NewChart", mock.AnythingOfType("[]float64"), mock.AnythingOfType("[]string")).Return([]byte{123}, nil)
	summary, err := adapter.PrepareSummary(input)

	// assert
	assert.Nil(err)
	assert.NotNil(summary)
	legendValuePtrs := util.PoorMansMap(summary.LegendValues, func(lv dto.LegendValue) *dto.LegendValue {
		return &lv
	})
	otherEntry, index := util.PoorMansFind(legendValuePtrs, func(lv *dto.LegendValue) bool {
		return lv.Legend == "Other"
	})
	assert.NotNil(otherEntry)
	assert.NotEqual(-1, index)
	assert.NotZero(otherEntry.Value)
}
