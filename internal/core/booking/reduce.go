package booking

import (
	"strings"

	"spot-assistant/internal/core/dto/reservation"
)

/*
This function takes a slice of reservations and merges some of them when they overlap and have the same spot (with different side or floor)
*/
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
				// Reservation start and end time of processed reservation do not equal with the one that already exist in reduced slice
				if !(re.StartAt.Equal(r.StartAt) && re.EndAt.Equal(r.EndAt)) {
					var firstOne *reservation.ReservationWithSpot
					var lastOne *reservation.ReservationWithSpot

					// When start time matches between compared reservations
					if re.StartAt.Equal(r.StartAt) {
						// When reduced reservation ends before processed reservation
						if re.EndAt.Before(r.EndAt) {
							re.EndAt = r.EndAt
						}

						break
					}

					// When end time matches between compared reservations
					if re.EndAt.Equal(r.EndAt) {
						// When reduced reservation start after processed reservation
						if re.StartAt.After(r.StartAt) {
							re.StartAt = r.StartAt
						}

						break
					}

					// This piece of code checks which reservation starts first and sets variables accordingly
					if re.StartAt.Before(r.StartAt) {
						firstOne = re
						lastOne = r
					} else {
						firstOne = r
						lastOne = re
					}

					// When sooner reservation ends after the later one starts
					if firstOne.EndAt.After(lastOne.StartAt) {
						// When sooner reservation ends after the later one ends
						if firstOne.EndAt.After(lastOne.EndAt) {
							re.StartAt = firstOne.StartAt
							re.EndAt = firstOne.EndAt

							break
							// When sooner reservation ends before the later one ends
						} else {
							firstOne.EndAt = lastOne.EndAt
							re.StartAt = firstOne.StartAt
							re.EndAt = firstOne.EndAt

							break
						}
						// When sooner reservation ends before the later one starts
					} else {
						// When it is the last item from reduced slice
						if i == len(reducedReservations)-1 {
							reducedReservations = append(reducedReservations, r)
						}
					}
					// When both start and end time is equal in compared reservations
				} else {
					break
				}
				// When spot in compared reservations is not the same
			} else {
				// When it is the last item from reduced slice
				if i == len(reducedReservations)-1 {
					reducedReservations = append(reducedReservations, r)
				}
			}
		}
	}

	return reducedReservations
}
