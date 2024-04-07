package api

import (
	"fmt"
	"time"

	"spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
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

	// Notify users about overbooking
	for _, res := range response.ConflictingReservations {
		go func(res *reservation.Reservation) {
			member, err := bot.GetMember(request.Guild, res.AuthorDiscordID)
			if err != nil {
				a.log.Errorf("error getting member: %s", err)
				return
			}

			author := fmt.Sprintf("<@!%s>", request.Member.ID)
			message := fmt.Sprintf(
				"Your reservation was overbooked by %s \n * %s %s %s - %s\n",
				fmt.Sprintf("<@!%s>", member.ID),
				author,
				request.Spot,
				res.StartAt.Format(strings.DC_LONG_TIME_FORMAT),
				res.EndAt.Format(strings.DC_LONG_TIME_FORMAT),
			)

			err = bot.SendDM(member, message)
			if err != nil {
				a.log.Errorf("error sending DM: %s", err)
			}
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
