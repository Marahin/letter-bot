package booking

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
)

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
		return hour.Format(stringsHelper.DcTimeFormat)
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

func (a *Adapter) Book(request book.BookRequest) ([]*reservation.ClippedOrRemovedReservation, error) {
	spotName := request.Spot
	member := request.Member
	startAt := request.StartAt
	endAt := request.EndAt
	overbook := request.Overbook
	hasPermissions := request.HasPermissions
	guild := request.Guild
	a.log.With(
		"spot", spotName,
		"member.id", member.ID,
		"member.name", member.Nick,
		"member.username", member.Username,
		"hasPermissions", hasPermissions,
		"overbook", overbook,
		"startAt", startAt,
		"endAt", endAt,
	).Info("booking request")

	spots, err := a.spotRepo.SelectAllSpots(context.Background())
	if err != nil {
		return nil, fmt.Errorf("could not fetch spots: %w", err)
	}

	spot, _ := collections.PoorMansFind(spots, func(s *spot.Spot) bool {
		return s.Name == spotName
	})
	if spot == nil {
		return nil, fmt.Errorf("could not find spot called %s", spotName)
	}

	if err = validateHuntLength(endAt.Sub(startAt)); err != nil {
		return nil, err
	}

	upcomingAuthorReservations, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(context.Background(), guild, member)
	if err != nil {
		return nil, fmt.Errorf("could not select upcoming member reservations: %w", err)
	}

	if err = validateHuntLengthForMultiFloorRespawns(spotName, upcomingAuthorReservations, startAt, endAt); err != nil {
		return nil, err
	}

	conflictingReservations, err := a.reservationRepo.SelectOverlappingReservations(context.Background(), spotName, startAt, endAt, guild.ID)
	if err != nil {
		return nil, fmt.Errorf("could not select overlapping reservations: %w", err)
	}

	if len(conflictingReservations) > 0 {
		if overbook {
			err = validateNoSelfOverbook(member, conflictingReservations)
			if err != nil {
				return nil, err
			}
		
			// Only block overbooking if the user does NOT have permissions
			if !hasPermissions {
				for _, conflict := range conflictingReservations {
					if time.Now().Before(conflict.StartAt.Add(10 * time.Minute)) {
						return nil, fmt.Errorf(
							"Overbooking is only allowed 10 minutes after the reservation starts. Reservation starts at %s.",
							conflict.StartAt.Format("15:04"),
						)
					}
				}
			}
		}
	
		if !canOverbook(overbook, hasPermissions, conflictingReservations) {
			// fallback: still enforce permission-based overbooking rules
			return collections.PoorMansMap(conflictingReservations, func(r *reservation.Reservation) *reservation.ClippedOrRemovedReservation {
				return &reservation.ClippedOrRemovedReservation{
					Original: r,
					New:      []*reservation.Reservation{r},
				}
			}), InsufficientPermissionsError
		}
	}

	res, err := a.reservationRepo.CreateAndDeleteConflicting(context.Background(), member, guild, conflictingReservations, spot.ID, startAt, endAt)
	if err != nil {
		return nil, fmt.Errorf("could not create the reservation: %w", err)
	}

	for _, res := range res {
		res := res
		go a.commSrv.NotifyOverbookedMember(request, res)
	}

	return res, nil
}

func (a *Adapter) UnbookAutocomplete(g *guild.Guild, m *member.Member, filter string) ([]*reservation.ReservationWithSpot, error) {
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
				r.StartAt.Format(stringsHelper.DcLongTimeFormat),
				r.StartAt.Format(stringsHelper.DcLongTimeFormat),
				r.Spot.Name}, "")
			containsFilterWord := strings.Contains(strings.ToLower(searchableString), strings.ToLower(filter))
			return containsFilterWord
		})
	}

	return reservations, nil
}

func (a *Adapter) Unbook(g *guild.Guild, m *member.Member, reservationId int64) (*reservation.ReservationWithSpot, error) {

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
