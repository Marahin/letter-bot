package summary

import (
	"cmp"
	"slices"
	"strings"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	dto "spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/regions"
)

func (a *Adapter) PrepareRegionSummaries(spots []*spot.Spot, reservations []*reservation.ReservationWithSpot) []dto.RegionSummary {
	byRegion := map[string]*dto.RegionSummary{}
	group := func(key string) *dto.RegionSummary {
		r := regions.Of(key)
		rs, ok := byRegion[r.Key]
		if !ok {
			rs = &dto.RegionSummary{Region: r}
			byRegion[r.Key] = rs
		}
		return rs
	}

	placed := false
	for _, sp := range spots {
		if sp.IsArchived() {
			continue
		}
		rs := group(sp.Region)
		rs.Spots = append(rs.Spots, sp.Name)
		placed = placed || rs.Region.Key != regions.Other.Key
	}
	if !placed {
		return nil
	}

	// A reservation of an archived spot still shows, in the region of its spot.
	booked := map[string][]*reservation.ReservationWithSpot{}
	for _, r := range reservations {
		key := group(r.Spot.Region).Region.Key
		booked[key] = append(booked[key], r)
	}

	out := make([]dto.RegionSummary, 0, len(byRegion))
	for key, rs := range byRegion {
		slices.SortFunc(rs.Spots, func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
		rs.Ledger = a.Ledger(booked[key])
		out = append(out, *rs)
	}
	slices.SortFunc(out, func(x, y dto.RegionSummary) int {
		return cmp.Compare(regions.Index(x.Region.Key), regions.Index(y.Region.Key))
	})
	return out
}
