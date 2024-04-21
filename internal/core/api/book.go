package api

import (
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Application) OnBook(request book.BookRequest) (book.BookResponse, error) {
	hasPermissions := a.botSrv.MemberHasRole(request.Guild, request.Member, "Postman")
	response := book.BookResponse{
		Spot:           request.Spot,
		StartAt:        request.StartAt,
		EndAt:          request.EndAt,
		Member:         request.Member,
		Overbook:       request.Overbook,
		HasPermissions: hasPermissions,
	}

	conflicting, err := a.bookingSrv.Book(
		request.Member,
		request.Guild,
		request.Spot, request.StartAt,
		request.EndAt, request.Overbook, hasPermissions)

	response.ConflictingReservations = conflicting

	if err != nil {
		return response, err
	}
	go a.UpdateGuildSummaryAndLogError(request.Guild)

	// Notify users about overbooking
	for _, res := range response.ConflictingReservations {
		go func(res *reservation.ClippedOrRemovedReservation) {
			member, err := a.botSrv.GetMember(request.Guild, res.Original.AuthorDiscordID)
			if err != nil {
				a.log.Errorf("error getting member: %s", err)
				return
			}

			a.commSrv.NotifyOverbookedMember(member, request, res)

		}(res)
	}

	return response, nil
}

func (a *Application) OnBookAutocomplete(request book.BookAutocompleteRequest) (book.BookAutocompleteResponse, error) {
	switch request.Field {
	case book.BookAutocompleteOverbook:
		// @TODO: make it based on user permissions
		return []string{"true", "false"}, nil
	case book.BookAutocompleteStartAt:
		return a.bookingSrv.GetSuggestedHours(time.Now(), request.Value), nil
	case book.BookAutocompleteEndAt:
		return a.bookingSrv.GetSuggestedHours(time.Now().Add(2*time.Hour), request.Value), nil
	case book.BookAutocompleteSpot:
		return a.bookingSrv.FindAvailableSpots(request.Value)
	default:
		return []string{}, fmt.Errorf("autocomplete not implemented for %v", request.Field)
	}
}
