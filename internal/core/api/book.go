package api

import (
	"fmt"
	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"strings"
	"time"

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
		go func(res *reservation.ClippedOrRemovedReservation) {
			member, err := bot.GetMember(request.Guild, res.Original.AuthorDiscordID)
			if err != nil {
				a.log.Errorf("error getting member: %s", err)
				return
			}

			msgHeader := fmt.Sprintf(
				"Your reservation was overbooked by %s\n",
				fmt.Sprintf("<@!%s>", request.Member.ID),
			)

			var msgBody strings.Builder
			msgBody.WriteString(fmt.Sprintf("* %s %s ", fmt.Sprintf("<@!%s>", member.ID), request.Spot))
			if len(res.New) > 0 { // The reservation has been modified, but not entirely removed - lets notify the user!
				msgBody.WriteString("has been clipped to: ")
				newClippedRanges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
					return fmt.Sprintf("%s - %s", r.StartAt.Format(stringsHelper.DC_LONG_TIME_FORMAT), r.EndAt.Format(stringsHelper.DC_LONG_TIME_FORMAT))
				})
				msgBody.WriteString(strings.Join(newClippedRanges, ", "))
			} else {
				msgBody.WriteString(fmt.Sprintf("has been entirely removed (originally: **%s - %s**)", res.Original.StartAt.Format(stringsHelper.DC_LONG_TIME_FORMAT), res.Original.EndAt.Format(stringsHelper.DC_LONG_TIME_FORMAT)))
			}

			err = bot.SendDM(member, msgHeader+msgBody.String())
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
