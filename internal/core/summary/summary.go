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
	sum.Ledger = a.Ledger(reservations)

	spotsToCounts := make(map[string]float64, len(sum.Ledger))
	for _, entry := range sum.Ledger {
		spotsToCounts[entry.Spot] = float64(len(entry.Bookings))
	}
	sum.LegendValues = a.mapToLegendValues(spotsToCounts)
	img, err := a.newChart(sum.LegendValues)
	if err != nil {
		return nil, err
	}
	sum.Chart = img

	return sum, nil
}

// Ledger groups the reservations by spot, in spot name order, each spot's by start time.
func (a *Adapter) Ledger(reservations []*reservation.ReservationWithSpot) dto.Ledger {
	spotsToReservations := a.mapToSpotsToReservations(reservations)
	spotNames := make([]string, 0, len(spotsToReservations))
	for spotName := range spotsToReservations {
		spotNames = append(spotNames, spotName)
	}
	slices.Sort(spotNames)

	images := make(map[string]string, len(spotNames))
	for _, r := range reservations {
		images[r.Spot.Name] = r.Spot.ImageURL
	}

	ledger := make(dto.Ledger, len(spotNames))
	for i, spotName := range spotNames {
		ledger[i] = dto.LedgerEntry{
			Spot:     spotName,
			ImageURL: images[spotName],
			Bookings: a.MapReservations(spotsToReservations[spotName]),
		}
	}
	return ledger
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
