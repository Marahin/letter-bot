package booking

import (
	"context"
	"errors"
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
	"spot-assistant/internal/ports"
)

var HourRegex = regexp.MustCompile(`(\d{2}:\d{2})`)

// FindAvailableSpots returns the names of the active guild spots matching the given filter.
// If the filter is empty, it returns a default list of spots (e.g., top 15).
func (a *Adapter) FindAvailableSpots(guildID, filter string) ([]string, error) {
	spots, err := a.spotRepo.SelectGuildSpotsLike(context.Background(), guildID, strings.TrimSpace(filter))
	if err != nil {
		return []string{}, fmt.Errorf("could not fetch spots matching your query: %w", err)
	}

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
		roundedHour++
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

	if validatedFilter != "" {
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
	m := request.Member
	startAt := request.StartAt
	endAt := request.EndAt
	overbook := request.Overbook
	hasPermissions := request.HasPermissions
	g := request.Guild
	a.log.With(
		"spot", spotName,
		"spotID", request.SpotID,
		"member.id", m.ID,
		"member.name", m.Nick,
		"member.username", m.Username,
		"hasPermissions", hasPermissions,
		"overbook", overbook,
		"startAt", startAt,
		"endAt", endAt,
	).Info("booking request")

	sp, err := a.bookedSpot(context.Background(), g.ID, request)
	if err != nil {
		return nil, err
	}
	request.Spot = sp.Name

	if err := validateHuntLength(endAt.Sub(startAt)); err != nil {
		return nil, err
	}

	if err := a.validateAuthorQuota(context.Background(), g, m, sp.Name, startAt, endAt, 0); err != nil {
		return nil, err
	}

	conflictingReservations, err := a.reservationRepo.SelectOverlappingReservations(context.Background(), sp.ID, startAt, endAt, g.ID)
	if err != nil {
		return nil, fmt.Errorf("could not select overlapping reservations: %w", err)
	}

	if len(conflictingReservations) > 0 {
		if overbook {
			err = validateNoSelfOverbook(m, conflictingReservations)
			if err != nil {
				return nil, err
			}
		}

		if !canOverbook(overbook, hasPermissions, conflictingReservations) {
			return collections.PoorMansMap(conflictingReservations, func(r *reservation.Reservation) *reservation.ClippedOrRemovedReservation {
				return &reservation.ClippedOrRemovedReservation{
					Original: r,
					New:      []*reservation.Reservation{r},
				}
			}), ErrInsufficientPermissions

		}
	}

	res, err := a.reservationRepo.CreateAndDeleteConflicting(context.Background(), m, g, conflictingReservations, sp.ID, startAt, endAt)
	if err != nil {
		return nil, fmt.Errorf("could not create the reservation: %w", err)
	}

	for _, res := range res {
		// A free-text author booked in the web has nobody to DM.
		if res.Original.AuthorDiscordID == "" {
			continue
		}
		go a.commSrv.NotifyOverbookedMember(request, res)
	}

	return res, nil
}

// bookedSpot resolves the spot of a booking: by SpotID when set, else by name.
func (a *Adapter) bookedSpot(ctx context.Context, guildID string, request book.BookRequest) (*spot.Spot, error) {
	if request.SpotID != 0 {
		return a.activeSpotByID(ctx, guildID, request.SpotID)
	}
	sp, err := a.spotRepo.SelectGuildSpotByName(ctx, guildID, request.Spot)
	if err != nil {
		return nil, fmt.Errorf("could not find spot called %s: %w", request.Spot, err)
	}
	return sp, nil
}

func (a *Adapter) activeSpotByID(ctx context.Context, guildID string, id int64) (*spot.Spot, error) {
	sp, err := a.spotRepo.SelectGuildSpotByID(ctx, guildID, id)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, ErrSpotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select spot: %w", err)
	}
	if sp.IsArchived() {
		return nil, ErrSpotArchived
	}
	return sp, nil
}

func (a *Adapter) UnbookAutocomplete(g *guild.Guild, m *member.Member, filter string) ([]*reservation.ReservationWithSpot, error) {
	// Get reservations with end_date >= time.Now()
	// a.reservationRepo.SelectUpcomingReservationsWithSpot(context.Background(), g.ID)
	reservations, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(context.Background(), g, m, 0)
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}

	// If any input value is passed, try to match it with startAt, endAt and spot name
	if filter != "" {
		reservations = collections.PoorMansFilter(reservations, func(r *reservation.ReservationWithSpot) bool {
			return strings.Contains(strings.ToLower(r.Label()), strings.ToLower(filter))
		})
	}

	return reservations, nil
}

func (a *Adapter) Unbook(g *guild.Guild, m *member.Member, reservationID int64) (*reservation.ReservationWithSpot, error) {

	// Get non-expired reservation for guild + member + reservation
	// Remove it
	// Return removed reservation and an error
	res, err := a.reservationRepo.FindReservationWithSpot(context.Background(), reservationID, g.ID, m.ID)
	if err != nil {
		return nil, err
	}

	err = a.reservationRepo.DeletePresentMemberReservation(context.Background(), g, m, res.Reservation.ID)
	if err != nil {
		return res, err
	}

	return res, nil
}
