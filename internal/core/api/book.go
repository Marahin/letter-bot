package api

import (
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/ports"
)

func (a *Application) OnBook(bot ports.BotPort, request book.BookRequest) (book.BookResponse, error) {
	response := book.BookResponse{
		Spot:    request.Spot,
		StartAt: request.StartAt,
		EndAt:   request.EndAt,
	}

	conflicting, err := a.bookingSrv.Book(
		request.Member,
		request.Guild,
		request.Spot, request.StartAt,
		request.EndAt, request.Overbook, bot.MemberHasRole(request.Guild, request.Member, "Postman"),
	)
	response.ConflictingReservations = conflicting

	if err != nil {
		return response, err
	}
	go a.UpdateGuildSummaryAndLogError(bot, request.Guild)

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
