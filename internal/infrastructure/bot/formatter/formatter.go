package formatter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/reservationforms"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/ports"
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
		message.WriteString("\nFollowing reservations are conflicting:\n\n")

		for _, res := range response.ConflictingReservations {
			fmt.Fprintf(&message,
				"* **%s** %s - %s\n",
				res.Original.Author,
				res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat),
				res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat),
			)
		}
	}

	return message.String()
}

// FormatBookResponse formats book response to Discord format
func (f *DiscordFormatter) FormatBookResponse(response book.BookResponse) string {
	var message strings.Builder

	fmt.Fprintf(&message,
		"<@!%s> booked **%s** between %s and %s.\n\n",
		response.Request.Member.ID,
		response.Request.Spot,
		response.Request.StartAt.Format("2006-01-02 15:04"),
		response.Request.EndAt.Format("2006-01-02 15:04"),
	)

	if len(response.ConflictingReservations) > 0 { // We have overbooked
		message.WriteString("Following reservations are conflicting **and have been shortened or removed**:\n\n")

		for _, res := range response.ConflictingReservations {
			fmt.Fprintf(&message,
				"* %s ", fmt.Sprintf("<@!%s>", res.Original.AuthorDiscordID),
			)

			if len(res.New) > 0 {
				message.WriteString("had their reservation clipped to: ")
				newClippedRanges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
					return fmt.Sprintf("**%s - %s**", r.StartAt.Format(stringsHelper.DcLongTimeFormat), r.EndAt.Format(stringsHelper.DcLongTimeFormat))
				})
				message.WriteString(strings.Join(newClippedRanges, ", "))
			} else {
				message.WriteString("had their reservation removed ")
			}

			fmt.Fprintf(&message,
				" (originally: %s - %s)\n",
				res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat),
				res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat),
			)
			continue // Stop here
		}
	}

	return message.String()
}

func (f *DiscordFormatter) FormatOverbookedMemberNotification(
	m *member.Member,
	request book.BookRequest,
	res *reservation.ClippedOrRemovedReservation,
) string {
	var msgBody strings.Builder

	fmt.Fprintf(&msgBody, "Your reservation was overbooked by %s (<@!%s>)\n", request.Member.Nick, request.Member.ID)
	fmt.Fprintf(&msgBody, "* %s %s ", fmt.Sprintf("<@!%s>", m.ID), request.Spot)
	if len(res.New) > 0 { // The reservation has been modified, but not entirely removed - lets notify the user!
		msgBody.WriteString("has been clipped to: ")
		newClippedRanges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
			return fmt.Sprintf("%s - %s", r.StartAt.Format(stringsHelper.DcLongTimeFormat), r.EndAt.Format(stringsHelper.DcLongTimeFormat))
		})
		msgBody.WriteString(strings.Join(newClippedRanges, ", "))
	} else {
		fmt.Fprintf(&msgBody, "has been entirely removed (originally: **%s - %s**)", res.Original.StartAt.Format(stringsHelper.DcLongTimeFormat), res.Original.EndAt.Format(stringsHelper.DcLongTimeFormat))
	}

	return msgBody.String()
}

const (
	formDayFormat  = "Mon 02 Jan 15:04"
	formTimeFormat = "15:04"
)

// FormatWindow is "Mon 05 Oct 18:30 – 20:30".
func (f *DiscordFormatter) FormatWindow(startAt, endAt time.Time) string {
	return startAt.Format(formDayFormat) + " – " + endAt.Format(formTimeFormat)
}

// FormatReservationLine is one numbered line of the member's reservation list.
func (f *DiscordFormatter) FormatReservationLine(n int, r *reservation.ReservationWithSpot, now time.Time) string {
	line := fmt.Sprintf("%d. **%s** · %s", n, r.Spot.Name, f.FormatWindow(r.StartAt, r.EndAt))
	if !r.StartAt.After(now) {
		line += " (ongoing)"
	}
	return line
}

// FormatMyReservations is the text of the member's reservation list. status,
// when set, says what the last action did.
func (f *DiscordFormatter) FormatMyReservations(page *reservation.Page, status string, now time.Time) string {
	var msg strings.Builder
	if status != "" {
		msg.WriteString(status + "\n\n")
	}
	if len(page.Items) == 0 {
		msg.WriteString("You have no upcoming reservations.")
		return msg.String()
	}
	fmt.Fprintf(&msg, "**Your upcoming reservations** (times in %s)\n", now.Format("MST"))
	for i, r := range page.Items {
		msg.WriteString(f.FormatReservationLine(i+1, r, now) + "\n")
	}
	if page.Total > int64(len(page.Items)) {
		fmt.Fprintf(&msg, "\nShowing %d of %d. Manage the others in the web panel or with /unbook.", len(page.Items), page.Total)
	}
	return msg.String()
}

// FormatFormBooked is the reply to a successful booking from a form.
func (f *DiscordFormatter) FormatFormBooked(outcome *reservation.FormOutcome) string {
	var msg strings.Builder
	fmt.Fprintf(&msg, "Booked **%s**, %s.", outcome.SpotName, f.FormatWindow(outcome.Draft.StartAt, outcome.Draft.EndAt))
	if len(outcome.Overbooked) > 0 {
		msg.WriteString("\n\nThese reservations were shortened or removed:\n")
		for _, res := range outcome.Overbooked {
			fmt.Fprintf(&msg, "* **%s** %s", res.Original.Author, f.FormatWindow(res.Original.StartAt, res.Original.EndAt))
			if len(res.New) == 0 {
				msg.WriteString(": removed\n")
				continue
			}
			ranges := collections.PoorMansMap(res.New, func(r *reservation.Reservation) string {
				return r.StartAt.Format(formTimeFormat) + " – " + r.EndAt.Format(formTimeFormat)
			})
			msg.WriteString(": now " + strings.Join(ranges, ", ") + "\n")
		}
	}
	return msg.String()
}

// FormatFormConflicts lists the reservations that stopped a booking or an edit.
func (f *DiscordFormatter) FormatFormConflicts(outcome *reservation.FormOutcome) string {
	var msg strings.Builder
	fmt.Fprintf(&msg, "**%s**, %s overlaps these reservations:\n", outcome.SpotName, f.FormatWindow(outcome.Draft.StartAt, outcome.Draft.EndAt))
	for _, r := range outcome.Conflicts {
		fmt.Fprintf(&msg, "* **%s** %s\n", r.Author, f.FormatWindow(r.StartAt, r.EndAt))
	}
	if outcome.CanOverbook {
		msg.WriteString("\nYou can overbook them. They will be shortened or removed, and their authors get a DM.")
	} else {
		msg.WriteString("\nChoose other times or another respawn.")
	}
	return msg.String()
}

// FormatFormEdited is the status line after a successful edit.
func (f *DiscordFormatter) FormatFormEdited(outcome *reservation.FormOutcome) string {
	return fmt.Sprintf("Updated **%s**, %s.", outcome.SpotName, f.FormatWindow(outcome.Draft.StartAt, outcome.Draft.EndAt))
}

// FormatCancelConfirm asks to confirm a cancellation.
func (f *DiscordFormatter) FormatCancelConfirm(r *reservation.ReservationWithSpot) string {
	return fmt.Sprintf("Cancel **%s**, %s? You cannot undo this.", r.Spot.Name, f.FormatWindow(r.StartAt, r.EndAt))
}

// FormatCancelled is the status line after a cancellation.
func (f *DiscordFormatter) FormatCancelled(r *reservation.ReservationWithSpot) string {
	return fmt.Sprintf("Cancelled **%s**, %s.", r.Spot.Name, f.FormatWindow(r.StartAt, r.EndAt))
}

// FormatSpotPick asks which of the matching respawns the member meant.
func (f *DiscordFormatter) FormatSpotPick(query string) string {
	return fmt.Sprintf("More than one respawn matches **%s**. Which one do you mean?", query)
}

// FormatFormOutdated is the reply to a button of an older bot version.
func (f *DiscordFormatter) FormatFormOutdated() string {
	return "This button is out of date. Use the buttons in the summary channel or /reservations."
}

// FormatFormNoGuild is the reply to a button or a form outside a server.
func (f *DiscordFormatter) FormatFormNoGuild() string {
	return "Use this in a server."
}

// FormatFormError explains why a form or a button failed. An unknown error gets
// the generic message.
func (f *DiscordFormatter) FormatFormError(err error) string {
	var ambiguous *reservationforms.AmbiguousSpotError
	switch {
	case errors.As(err, &ambiguous):
		return f.FormatSpotPick(ambiguous.Query)
	case errors.Is(err, reservationforms.ErrTimeFormat):
		return "Write the times as HH:MM, e.g. 18:30."
	case errors.Is(err, booking.ErrSpotNotFound):
		return "No respawn has this name. Write the name or a part of it, e.g. Library."
	case errors.Is(err, booking.ErrSpotArchived):
		return "This respawn is archived. Choose another respawn."
	case errors.Is(err, booking.ErrReserveNotAllowed):
		return "You do not have a rank that can book respawns on this server."
	case errors.Is(err, reservations.ErrForbidden):
		return "You can only change your own upcoming reservations."
	case errors.Is(err, ports.ErrNotFound):
		return "This reservation does not exist any more."
	case errors.Is(err, booking.ErrReservationEnded):
		return "This reservation has already ended."
	case errors.Is(err, booking.ErrReservationTooLong):
		return "A reservation can be at most 3 hours long."
	case errors.Is(err, booking.ErrQuotaExceeded):
		return "You can book at most 3 hours within 24 hours."
	case errors.Is(err, booking.ErrStartInPast):
		return "The reservation cannot start in the past."
	case errors.Is(err, booking.ErrInvalidRange):
		return "The reservation must end after it starts."
	case errors.Is(err, booking.ErrSpotLocked):
		return "A started reservation cannot move to another respawn."
	case errors.Is(err, booking.ErrSelfOverbook):
		return "You cannot overbook your own reservation."
	case errors.Is(err, booking.ErrConflict), errors.Is(err, booking.ErrInsufficientPermissions):
		return "Other reservations overlap these times. Choose other times or another respawn."
	}
	return f.FormatGenericError(err)
}
