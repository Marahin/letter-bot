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
// when set, says what the last action did. page is nil when the list could not
// be read.
func (f *DiscordFormatter) FormatMyReservations(page *reservation.Page, status string, now time.Time) string {
	var msg strings.Builder
	if status != "" {
		msg.WriteString(status + "\n\n")
	}
	if page == nil {
		msg.WriteString("Could not load your reservations. Press Refresh to try again.")
		return msg.String()
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

// FormatRespawnStep is the text of step 1 of the booking wizard. status, when
// set, goes first.
func (f *DiscordFormatter) FormatRespawnStep(editing, empty bool, status string) string {
	var msg strings.Builder
	if status != "" {
		msg.WriteString(status + "\n\n")
	}
	if editing {
		msg.WriteString("**Change the respawn** — pick it from a list.")
	} else {
		msg.WriteString("**Book a respawn** — pick it from a list.")
	}
	if empty {
		msg.WriteString("\nThis server has no respawns yet. A manager can add them in the web panel.")
	}
	return msg.String()
}

// FormatRespawnMatches introduces the respawns that match a search. capped
// means only some of them are listed.
func (f *DiscordFormatter) FormatRespawnMatches(query string, capped bool) string {
	msg := fmt.Sprintf("Respawns that match **%s**:", query)
	if capped {
		msg += "\nNot all of them fit in the list. If yours is missing, search for more of its name."
	}
	return msg
}

// FormatUsualRespawn describes a respawn in the usual list.
func (f *DiscordFormatter) FormatUsualRespawn(r reservation.UsualRespawn) string {
	switch r.Bookings {
	case 0:
		return "Popular on this server"
	case 1:
		return "You booked it once"
	}
	return fmt.Sprintf("You booked it %d times", r.Bookings)
}

// FormatTimeStep is the text of step 2 of the booking wizard. status, when set,
// goes first.
func (f *DiscordFormatter) FormatTimeStep(p *reservation.TimePicker, status string, now time.Time) string {
	var msg strings.Builder
	if status != "" {
		msg.WriteString(status + "\n\n")
	}
	zone := now.Format("MST")
	switch {
	case p.Ongoing:
		fmt.Fprintf(&msg, "**%s** — started at %s. You can change the length. Times in %s.", p.Spot.Name, p.Editing.StartAt.In(now.Location()).Format(formTimeFormat), zone)
	case p.Editing != nil:
		fmt.Fprintf(&msg, "**%s** — change the start, the length or the respawn. Times in %s.", p.Spot.Name, zone)
	default:
		fmt.Fprintf(&msg, "**%s** — pick the start and the length. Times in %s.", p.Spot.Name, zone)
	}
	if p.Choice.Complete() {
		start, startText := p.Choice.StartAt, f.FormatClock(p.Choice.StartAt, now)
		if p.Choice.Now {
			start, startText = now, "now"
		}
		fmt.Fprintf(&msg, "\nChosen: %s – %s.", startText, start.Add(p.Choice.Length).In(now.Location()).Format(formTimeFormat))
	}
	if len(p.Booked) > 0 {
		msg.WriteString("\n\nAlready booked:")
		for i, r := range p.Booked {
			if i == maxBookedLines {
				fmt.Fprintf(&msg, "\n…and %d more.", len(p.Booked)-i)
				break
			}
			fmt.Fprintf(&msg, "\n* %s – %s · %s", f.FormatClock(r.StartAt, now), r.EndAt.In(now.Location()).Format(formTimeFormat), r.Author)
		}
	}
	return msg.String()
}

const maxBookedLines = 10

// FormatClock is "18:30", with "tomorrow" or the weekday when not today.
func (f *DiscordFormatter) FormatClock(t, now time.Time) string {
	t = t.In(now.Location())
	clock := t.Format(formTimeFormat)
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	switch {
	case ty == ny && tm == nm && td == nd:
		return clock
	case time.Date(ny, nm, nd+1, 0, 0, 0, 0, now.Location()).Equal(time.Date(ty, tm, td, 0, 0, 0, 0, now.Location())):
		return clock + " tomorrow"
	}
	return clock + " " + t.Format("Mon")
}

// FormatStartOption is the label and the description of a start in the list.
func (f *DiscordFormatter) FormatStartOption(o reservation.StartOption, now time.Time) (label, description string) {
	switch {
	case o.Keep:
		label = "Keep (started " + o.StartAt.In(now.Location()).Format(formTimeFormat) + ")"
	case o.Now:
		label = "Now"
	case o.Current:
		label = f.FormatClock(o.StartAt, now) + " (current)"
	default:
		label = f.FormatClock(o.StartAt, now)
	}
	description = "Free"
	if o.BookedBy != nil {
		description = fmt.Sprintf("Booked by %s until %s", o.BookedBy.Author, o.BookedBy.EndAt.In(now.Location()).Format(formTimeFormat))
	}
	if o.Keep {
		description = ""
	}
	return label, description
}

// FormatLength is "30 min", "2 h" or "1 h 30".
func (f *DiscordFormatter) FormatLength(d time.Duration) string {
	hours, minutes := int(d/time.Hour), int(d%time.Hour/time.Minute)
	switch {
	case hours == 0:
		return fmt.Sprintf("%d min", minutes)
	case minutes == 0:
		return fmt.Sprintf("%d h", hours)
	}
	return fmt.Sprintf("%d h %02d", hours, minutes)
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
		return fmt.Sprintf("More than one respawn matches **%s**. Pick one from the list.", ambiguous.Query)
	case errors.Is(err, reservationforms.ErrChoiceIncomplete):
		return "Choose a start and a length first."
	case errors.Is(err, reservationforms.ErrTimeFormat):
		return "Write the times as HH:MM, e.g. 18:30."
	case errors.Is(err, booking.ErrSpotNotFound):
		return "No respawn has this name. Search for a part of it, e.g. Library, or pick it from the list."
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
