package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	dto "spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/regions"
)

func inRegion(id int64, name, key string) *spot.Spot {
	return &spot.Spot{ID: id, Name: name, Region: key}
}

func reservedIn(name, key, image, author string, start time.Time) *reservation.ReservationWithSpot {
	r := reserved(name, author, start, start.Add(time.Hour))
	r.Spot.Region, r.Spot.ImageURL = key, image
	return r
}

func TestPrepareRegionSummaries_GroupsInCatalogOrderWithOtherLast(t *testing.T) {
	// given
	adapter, _ := newMapperAdapter(t)
	archivedAt := mapperNow
	gone := inRegion(5, "Old Cave", "zao")
	gone.ArchivedAt = &archivedAt
	spots := []*spot.Spot{
		inRegion(1, "issavi sewers", "issavi"),
		inRegion(2, "Mystery", ""),
		inRegion(3, "Hero Cave", "edron"),
		inRegion(4, "Girtablilu", "issavi"),
		inRegion(6, "Forgotten", "atlantis"),
		gone,
	}
	start := mapperNow.Add(time.Hour)
	res := []*reservation.ReservationWithSpot{
		reservedIn("Hero Cave", "edron", "https://img.test/hc.png", "Knight", start),
		reservedIn("Old Cave", "zao", "", "Druid", start),
	}

	// when
	summaries := adapter.PrepareRegionSummaries(spots, res)

	// then
	require.Len(t, summaries, 4)
	assert.Equal(t, []string{"edron", "zao", "issavi", "other"}, []string{
		summaries[0].Region.Key, summaries[1].Region.Key, summaries[2].Region.Key, summaries[3].Region.Key,
	})
	assert.Equal(t, dto.RegionSummary{
		Region: regions.Of("edron"),
		Spots:  []string{"Hero Cave"},
		Ledger: dto.Ledger{{Spot: "Hero Cave", ImageURL: "https://img.test/hc.png", Bookings: []*dto.Booking{{Author: "Knight", StartAt: start, EndAt: start.Add(time.Hour)}}}},
	}, summaries[0])
	assert.Empty(t, summaries[1].Spots, "the reservation of an archived spot still shows")
	assert.Len(t, summaries[1].Ledger, 1)
	assert.Equal(t, []string{"Girtablilu", "issavi sewers"}, summaries[2].Spots)
	assert.Empty(t, summaries[2].Ledger)
	assert.Equal(t, []string{"Forgotten", "Mystery"}, summaries[3].Spots, "an unknown key is Other")
}

func TestPrepareRegionSummaries_NoRegionKeepsTheSingleSummary(t *testing.T) {
	// given
	adapter, _ := newMapperAdapter(t)
	spots := []*spot.Spot{inRegion(1, "Mystery", ""), inRegion(2, "Forgotten", "atlantis")}

	// when
	summaries := adapter.PrepareRegionSummaries(spots, []*reservation.ReservationWithSpot{
		reservedIn("Mystery", "", "", "Knight", mapperNow.Add(time.Hour)),
	})

	// then
	assert.Empty(t, summaries)
}
