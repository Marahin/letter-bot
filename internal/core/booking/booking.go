package booking

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"

	"github.com/sirupsen/logrus"
)

const MAXIMUM_RESERVATIONS_TIME = 3 * time.Hour

var MAXIMUM_RESERVATIONS_TIME_EXCEEDED_ERROR = errors.New("You can only book 3 hours of reservations within 24 hour window")
var HourRegex = regexp.MustCompile(`(\d{2}:\d{2})`)

// Returns spots filtered by filter, if non-zero length.
func (a *Adapter) FindAvailableSpots(filter string) ([]string, error) {
	spots, err := a.spotRepo.SelectAllSpots(context.Background())
	if err != nil {
		return []string{}, fmt.Errorf("could not fetch spots matching your query: %w", err)
	}

	if len(filter) > 0 {
		spots = collections.PoorMansFilter(spots, func(spot *spot.Spot) bool {
			return strings.Contains(strings.ToLower(spot.Name), strings.ToLower(filter))
		})
	}

	spots = collections.Truncate(spots, 15)

	return collections.PoorMansMap(spots, func(s *spot.Spot) string {
		return s.Name
	}), nil
}

// Returns suggested hours based on requested time. If filter is non-zero length,
// it will return filtered results.
func (a *Adapter) GetSuggestedHours(baseTime time.Time, filter string) []string {
	suggestedHours := make([]time.Time, 0)
	validatedFilter := HourRegex.FindString(filter)

	roundedMinutes := baseTime.Minute()
	roundedHour := baseTime.Hour()
	if roundedMinutes >= 30 {
		roundedMinutes = 0
		roundedHour += 1
	} else {
		roundedMinutes = 30
	}

	baseTimeRounded := time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), roundedHour, roundedMinutes, 0, 0, baseTime.Location())

	suggestedHours = append(suggestedHours, baseTimeRounded)
	for x := 1; x <= 7; x++ {
		suggestedHours = append(suggestedHours, suggestedHours[x-1].Add(30*time.Minute))
	}

	suggestedOptions := collections.PoorMansMap(suggestedHours, func(hour time.Time) string {
		return hour.Format(stringsHelper.DC_TIME_FORMAT)
	})

	if len(validatedFilter) > 0 {
		suggestedOptions = collections.PoorMansFilter(suggestedOptions, func(t string) bool {
			return strings.Contains(strings.ToLower(t), strings.ToLower(validatedFilter))
		})

		// Add user input, if it's valid
		if !collections.PoorMansContains(suggestedOptions, validatedFilter) {
			suggestedOptions = append(suggestedOptions, validatedFilter)
		}
	}

	return suggestedOptions
}

func (a *Adapter) Book(member *discord.Member, guild *discord.Guild, spotName string, startAt time.Time, endAt time.Time, overbook bool, hasPermissions bool) ([]*reservation.Reservation, error) {
	currTime := time.Now()

	a.log.WithFields(logrus.Fields{
		"member":         member,
		"hasPermissions": hasPermissions,
		"overbook":       overbook,
		"startAt":        startAt,
		"endAt":          endAt,
		"currTime":       currTime,
	}).Info("booking request")

	spots, err := a.spotRepo.SelectAllSpots(context.Background())
	if err != nil {
		return []*reservation.Reservation{}, fmt.Errorf("could not fetch spots: %w", err)
	}

	spot, _ := collections.PoorMansFind(spots, func(s *spot.Spot) bool {
		return s.Name == spotName
	})
	if spot == nil {
		return []*reservation.Reservation{}, fmt.Errorf("could not find spot called %s", spotName)
	}

	if endAt.Sub(startAt) > 3*time.Hour {
		return []*reservation.Reservation{}, errors.New("reservation cannot take more than 3 hours")
	}

	conflictingReservations, err := a.reservationRepo.SelectOverlappingReservations(context.Background(), spotName, startAt, endAt, guild.ID)
	if err != nil {
		return []*reservation.Reservation{}, fmt.Errorf("could not select overlapping reservations: %w", err)
	}

	authorsConflictingReservations, _ := collections.PoorMansFind(conflictingReservations, func(r *reservation.Reservation) bool {
		return r.AuthorDiscordID == member.ID
	})

	if authorsConflictingReservations != nil && overbook {
		return []*reservation.Reservation{}, errors.New("you cannot overbook yourself")
	}

	if len(conflictingReservations) > 0 {
		switch canDo := overbook && isPotentiallyAbandonedReservation(conflictingReservations) || overbook && hasPermissions; canDo {
		case true:
			break
		case false:
			return conflictingReservations, errors.New("There are conflicting reservation which prevented booking this reservation. If you would like to overbook them, ensure you have a @Postman role, then repeat the command and set 'overbook' parameter to 'true'.")
		}
	}

	// Check for potentially exceeding maximum hours, with an exception for multi-floor respawns
	upcomingAuthorReservations, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(context.Background(), guild, member)
	if err != nil {
		return []*reservation.Reservation{}, fmt.Errorf("could not select upcoming member reservations: %w", err)
	}

	if len(upcomingAuthorReservations) > 0 {
		tempReservation := reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{
				ID:      -1,
				Author:  member.ID,
				StartAt: startAt,
				EndAt:   endAt,
			},
			Spot: reservation.Spot{
				Name: spotName,
			},
		}
		upcomingAuthorReservations = append(upcomingAuthorReservations, &tempReservation)

		reducedReservations := reduceAllAuthorReservationsByLongestPerSpot(upcomingAuthorReservations)
		totalReservationsTime := collections.PoorMansSum(reducedReservations, func(reservation *reservation.ReservationWithSpot) time.Duration {
			return reservation.EndAt.Sub(reservation.StartAt)
		})

		if totalReservationsTime > MAXIMUM_RESERVATIONS_TIME {
			return []*reservation.Reservation{}, MAXIMUM_RESERVATIONS_TIME_EXCEEDED_ERROR
		}
	}

	res, err := a.reservationRepo.CreateAndDeleteConflicting(context.Background(), member, guild, conflictingReservations, spot.ID, startAt, endAt)
	if err != nil {
		return nil, fmt.Errorf("could not create the reservation: %w", err)
	}

	return res, nil
}

func (a *Adapter) UnbookAutocomplete(g *discord.Guild, m *discord.Member, filter string) ([]*reservation.ReservationWithSpot, error) {
	// Get reservations with end_date >= time.Now()
	// a.reservationRepo.SelectUpcomingReservationsWithSpot(context.Background(), g.ID)
	reservations, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(context.Background(), g, m)
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}

	// If any input value is passed, try to match it with startAt, endAt and spot name
	if len(filter) > 0 {
		reservations = collections.PoorMansFilter(reservations, func(r *reservation.ReservationWithSpot) bool {
			searchableString := strings.Join([]string{
				r.StartAt.Format(stringsHelper.DC_LONG_TIME_FORMAT),
				r.StartAt.Format(stringsHelper.DC_LONG_TIME_FORMAT),
				r.Spot.Name}, "")
			containsFilterWord := strings.Contains(strings.ToLower(searchableString), strings.ToLower(filter))
			return containsFilterWord
		})
	}

	return reservations, nil
}

func (a *Adapter) Unbook(g *discord.Guild, m *discord.Member, reservationId int64) (*reservation.ReservationWithSpot, error) {

	// Get non-expired reservation for guild + member + reservation
	// Remove it
	// Return removed reservation and an error
	res, err := a.reservationRepo.FindReservationWithSpot(context.Background(), reservationId, g.ID, m.ID)
	if err != nil {
		return nil, err
	}

	err = a.reservationRepo.DeletePresentMemberReservation(context.Background(), g, m, res.Reservation.ID)
	if err != nil {
		return res, err
	}

	return res, nil
}

// This is an edge case, where we check:
// if there is only one overlapping reservation,
// and if it started,
// and if it hasn't ended,
// and it contains our reservation request and time
func isPotentiallyAbandonedReservation(overlappingReservations []*reservation.Reservation) bool {
	return len(overlappingReservations) == 1 &&
		overlappingReservations[0].StartAt.Before(time.Now()) &&
		(overlappingReservations[0].EndAt.After(time.Now()) ||
			overlappingReservations[0].EndAt.Equal(time.Now()))
}
