package booking

import (
	"strings"

	"spot-assistant/internal/core/dto/reservation"
)

func reduceAllAuthorReservationsByLongestPerSpot(reservations []*reservation.ReservationWithSpot) []*reservation.ReservationWithSpot {
	reducedReservations := []*reservation.ReservationWithSpot{reservations[0]}

	for _, r := range reservations[1:] {
		replacer := strings.NewReplacer(
			"(NORTH)", "",
			"(EAST)", "",
			"(SOUTH)", "",
			"(WEST)", "",
			"(RIGHT)", "",
			"(LEFT)", "",
			"-1", "",
			"-2", "",
			"-3", "",
			"-4", "",
			"-5", "",
			"-6", "",
			"-7", "",
			"-8", "",
		)
		spotNameWithoutLevelOrSide := strings.TrimSpace(replacer.Replace(r.Spot.Name))

		for i, re := range reducedReservations {
			// Spot is already present in one of reservations; add time to it
			if strings.Contains(re.Spot.Name, spotNameWithoutLevelOrSide) {
				if !re.StartAt.Equal(r.StartAt) || !re.EndAt.Equal(r.EndAt) {
					var firstOne *reservation.ReservationWithSpot
					var lastOne *reservation.ReservationWithSpot

					if re.StartAt.Equal(r.StartAt) {
						if re.EndAt.Before(r.EndAt) {
							re.EndAt = r.EndAt
						}

						break
					}

					if re.EndAt.Equal(r.EndAt) {
						if re.StartAt.After(r.StartAt) {
							re.StartAt = r.StartAt
						}

						break
					}

					if re.StartAt.Before(r.StartAt) {
						firstOne = re
						lastOne = r
					} else {
						firstOne = r
						lastOne = re
					}

					if firstOne.EndAt.After(lastOne.StartAt) {
						if firstOne.EndAt.After(lastOne.EndAt) {
							re.StartAt = firstOne.StartAt
							re.EndAt = firstOne.EndAt

							break
						} else {
							firstOne.EndAt = lastOne.EndAt
							re.StartAt = firstOne.StartAt
							re.EndAt = firstOne.EndAt

							break
						}
					} else {
						if i == len(reducedReservations)-1 {
							reducedReservations = append(reducedReservations, r)
						}
					}
				}
			} else {
				if i == len(reducedReservations)-1 {
					reducedReservations = append(reducedReservations, r)
				}
			}
		}
	}

	return reducedReservations
}
