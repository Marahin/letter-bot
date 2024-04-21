package formatter

import (
	"fmt"
	"spot-assistant/internal/core/dto/discord"
	"strings"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

type DiscordFormatter struct{}

func NewFormatter() *DiscordFormatter {
	return &DiscordFormatter{}
}

func (f *DiscordFormatter) FormatGenericError(err error) string {
	return fmt.Sprintf("Sorry, but something went wrong. If you require support, join TibiaLoot.com Discord: https://discord.gg/F4YKgsnzmc \nError message:\n```\n%s\n```", err.Error())
}

func (f *DiscordFormatter) FormatUnbookResponse(res *reservation.ReservationWithSpot) string {
	return fmt.Sprintf("%s (%s - %s) reservation has been cancelled.", res.Spot.Name, res.StartAt.Format(stringsHelper.DcLongTimeFormat), res.EndAt.Format(stringsHelper.DcLongTimeFormat))
}

// FormatBookError formats book error to Discord format
func (f *DiscordFormatter) FormatBookError(response book.BookResponse, err error) string {
	var message strings.Builder
	message.WriteString(f.FormatGenericError(err))

	if len(response.ConflictingReservations) > 0 {
		message.WriteString("Following reservations are conflicting:\n\n")

		for _, res := range response.ConflictingReservations {
			var author string
			author = response.Member.Nick
			if len(author) == 0 {
				author = response.Member.Username
			}
			author = fmt.Sprintf("**%s**", author)

			message.WriteString(fmt.Sprintf(
				"* %s %s - %s\n",
				author,
				res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat),
				res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat)),
			)
		}
	}

	return message.String()
}

// FormatBookResponse formats book response to Discord format
func (f *DiscordFormatter) FormatBookResponse(response book.BookResponse) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf(
		"<@!%s> booked **%s** between %s and %s.\n\n",
		response.Member.ID,
		response.Spot,
		response.StartAt.Format("2006-01-02 15:04"),
		response.EndAt.Format("2006-01-02 15:04"),
	))

	if len(response.ConflictingReservations) > 0 { // We have overbooked
		message.WriteString("Following reservations are conflicting **and have been shortened or removed**:\n\n")

		for _, res := range response.ConflictingReservations {
			message.WriteString(fmt.Sprintf(
				"* %s ", fmt.Sprintf("<@!%s>", res.Original.AuthorDiscordID),
			))

			if len(res.New) > 0 {
				message.WriteString("had their reservation clipped to: ")
				newClippedRanges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
					return fmt.Sprintf("**%s - %s**", r.StartAt.Format(stringsHelper.DcLongTimeFormat), r.EndAt.Format(stringsHelper.DcLongTimeFormat))
				})
				message.WriteString(strings.Join(newClippedRanges, ", "))
			} else {
				message.WriteString("had their reservation removed ")
			}

			message.WriteString(fmt.Sprintf(
				" (originally: %s - %s)\n",
				res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat),
				res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat),
			))
			continue // Stop here
		}
	}

	return message.String()
}

func (f *DiscordFormatter) FormatOverbookedMemberNotification(member *discord.Member,
	request book.BookRequest,
	res *reservation.ClippedOrRemovedReservation) string {
	var msgBody strings.Builder
	msgBody.WriteString(fmt.Sprintf(
		"Your reservation was overbooked by %s\n",
		fmt.Sprintf("<@!%s>", request.Member.ID),
	))
	msgBody.WriteString(fmt.Sprintf("* %s %s ", fmt.Sprintf("<@!%s>", member.ID), request.Spot))
	if len(res.New) > 0 { // The reservation has been modified, but not entirely removed - lets notify the user!
		msgBody.WriteString("has been clipped to: ")
		newClippedRanges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
			return fmt.Sprintf("%s - %s", r.StartAt.Format(stringsHelper.DcLongTimeFormat), r.EndAt.Format(stringsHelper.DcLongTimeFormat))
		})
		msgBody.WriteString(strings.Join(newClippedRanges, ", "))
	} else {
		msgBody.WriteString(fmt.Sprintf("has been entirely removed (originally: **%s - %s**)", res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat), res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat)))
	}

	return msgBody.String()
}
