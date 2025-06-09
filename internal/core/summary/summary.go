package summary

import (
	"fmt"
	"slices"
	"time"

	commonStrings "spot-assistant/internal/common/strings"
	"spot-assistant/internal/common/version"
	"spot-assistant/internal/core/dto/reservation"
	dto "spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) BaseSummary() *dto.Summary {
	return &dto.Summary{
		PreMessage:  commonStrings.PeriodicMessageContent,
		URL:         "https://tibialoot.com",
		Title:       "TibiaLoot.com - Spot Assistant",
		Description: "Current and upcoming hunts. Times are in **Europe/Berlin**.",
		Footer: fmt.Sprintf(
			"Version: %s powered by TibiaLoot.com (%s)", version.Version, time.Now().Format("15:04 01.02"),
		),
	}
}

func (a *Adapter) PrepareSummary(reservations []*reservation.ReservationWithSpot) (*dto.Summary, error) {
	sum := a.BaseSummary()

	spotsToReservations := a.mapToSpotsToReservations(reservations)

	// Chart generation
	spotsToCounts := a.mapToSpotsToCounts(spotsToReservations)
	sum.LegendValues = a.mapToLegendValues(spotsToCounts)
	img, err := a.newChart(sum.LegendValues)
	if err != nil {
		return nil, err
	}

	sum.Chart = img

	// Ledger preparation
	spotNamesAlphabetically := make([]string, 0, len(spotsToReservations))
	for k := range spotsToReservations {
		spotNamesAlphabetically = append(spotNamesAlphabetically, k)
	}
	slices.Sort(spotNamesAlphabetically)
	ledger := make(dto.Ledger, len(spotNamesAlphabetically))
	for i, spotName := range spotNamesAlphabetically {
		ledger[i] = dto.LedgerEntry{
			Spot:     spotName,
			Bookings: a.MapReservations(spotsToReservations[spotName]),
		}
	}
	sum.Ledger = ledger

	return sum, nil
}

func (a *Adapter) mapToSpotsToReservations(reservations []*reservation.ReservationWithSpot) map[string][]*reservation.Reservation {
	spotsToReservations := map[string][]*reservation.Reservation{}

	// Sort reservations by times
	slices.SortFunc(reservations, func(a *reservation.ReservationWithSpot, b *reservation.ReservationWithSpot) int {
		if b.StartAt.After(a.StartAt) {
			return -1
		}

		return 1
	})

	for _, reserv := range reservations {
		if _, ok := spotsToReservations[reserv.Spot.Name]; !ok {
			spotsToReservations[reserv.Spot.Name] = []*reservation.Reservation{&reserv.Reservation}
		} else {
			spotsToReservations[reserv.Spot.Name] = append(spotsToReservations[reserv.Spot.Name], &reserv.Reservation)
		}
	}

	return spotsToReservations
}

func (a *Adapter) mapToSpotsToCounts(spotsToReservations map[string][]*reservation.Reservation) map[string]float64 {
	spotsToCounts := map[string]float64{}
	for spot, val := range spotsToReservations {
		spotsToCounts[spot] = float64(len(val))
	}

	return spotsToCounts
}

func (a *Adapter) RefreshOnlinePlayers() error {
	if a.onlineCheck == nil {
		return nil
	}
	return a.onlineCheck.RefreshOnlinePlayers()
}
