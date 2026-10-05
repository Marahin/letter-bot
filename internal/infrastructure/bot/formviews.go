package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/reservationforms"
)

const (
	// listLimit leaves one of Discord's five component rows for the list actions.
	listLimit         = 4
	maxLabelLength    = 80
	maxSpotNameLength = 100
)

// formView is the content and the components of an ephemeral form message.
type formView struct {
	content    string
	components []discordgo.MessageComponent
}

func (v formView) edit() *discordgo.WebhookEdit {
	components := v.components
	if components == nil {
		components = []discordgo.MessageComponent{}
	}
	return &discordgo.WebhookEdit{
		Content:         &v.content,
		Components:      &components,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}
}

// discordgo v0.27.1 sends "emoji":{} on every button and select option. The
// plain types below leave it out, as Discord may refuse an empty emoji.
type plainButton struct {
	Label    string                `json:"label"`
	Style    discordgo.ButtonStyle `json:"style"`
	CustomID string                `json:"custom_id,omitempty"`
	URL      string                `json:"url,omitempty"`
}

func (plainButton) Type() discordgo.ComponentType {
	return discordgo.ButtonComponent
}

func (b plainButton) MarshalJSON() ([]byte, error) {
	type fields plainButton
	return json.Marshal(struct {
		fields

		Type discordgo.ComponentType `json:"type"`
	}{fields(b), b.Type()})
}

type plainOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type plainSelect struct {
	CustomID    string        `json:"custom_id"`
	Placeholder string        `json:"placeholder"`
	Options     []plainOption `json:"options"`
}

func (plainSelect) Type() discordgo.ComponentType {
	return discordgo.SelectMenuComponent
}

func (m plainSelect) MarshalJSON() ([]byte, error) {
	type fields plainSelect
	return json.Marshal(struct {
		fields

		Type discordgo.ComponentType `json:"type"`
	}{fields(m), m.Type()})
}

func button(label string, style discordgo.ButtonStyle, action formAction) plainButton {
	return plainButton{Label: cut(label, maxLabelLength), Style: style, CustomID: action.customID()}
}

func linkButton(label, url string) plainButton {
	return plainButton{Label: label, Style: discordgo.LinkButton, URL: url}
}

func row(components ...discordgo.MessageComponent) discordgo.ActionsRow {
	return discordgo.ActionsRow{Components: components}
}

func cut(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "…"
}

// summaryComponents are the buttons under the guild summary.
func summaryComponents(webBaseURL string) []discordgo.MessageComponent {
	buttons := []discordgo.MessageComponent{
		button("Book a respawn", discordgo.PrimaryButton, formAction{Kind: actionBookForm}),
		button("My reservations", discordgo.SecondaryButton, formAction{Kind: actionMine}),
	}
	if webBaseURL != "" {
		buttons = append(buttons, linkButton("Open panel", webBaseURL))
	}
	return []discordgo.MessageComponent{row(buttons...)}
}

func textInput(id, label, placeholder, value string, minLength, maxLength int) discordgo.ActionsRow {
	return row(discordgo.TextInput{
		CustomID:    id,
		Label:       label,
		Style:       discordgo.TextInputShort,
		Placeholder: placeholder,
		Value:       value,
		Required:    true,
		MinLength:   minLength,
		MaxLength:   maxLength,
	})
}

func reservationModal(title string, submit formAction, form reservation.Form) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		CustomID: submit.customID(),
		Title:    title,
		Components: []discordgo.MessageComponent{
			textInput(inputSpot, "Respawn", "Name or a part of it, e.g. Library -1", cut(form.Spot, maxSpotNameLength), 2, maxSpotNameLength),
			textInput(inputStart, "Start", "HH:MM, e.g. 18:30", form.StartAt, 1, 5),
			textInput(inputEnd, "End", "HH:MM, at most 3 hours after the start", form.EndAt, 1, 5),
		},
	}
}

func bookModal(form reservation.Form) *discordgo.InteractionResponseData {
	return reservationModal("Book a respawn", formAction{Kind: actionBookSubmit}, form)
}

func editModal(id int64, form reservation.Form) *discordgo.InteractionResponseData {
	return reservationModal("Edit reservation", formAction{Kind: actionEditSubmit, ReservationID: id}, form)
}

// existingForm fills the edit form with the reservation.
func existingForm(r *reservation.ReservationWithSpot) reservation.Form {
	return reservation.Form{Spot: r.Spot.Name, StartAt: r.StartAt.Format("15:04"), EndAt: r.EndAt.Format("15:04")}
}

// retryForm fills a form again from a retry button.
func retryForm(a formAction) reservation.Form {
	return reservation.Form{Spot: a.SpotText, StartAt: clockInput(a.StartText), EndAt: clockInput(a.EndText)}
}

// typedClock is the "HHMM" of a typed time, or "" when it does not parse.
func typedClock(text string) string {
	hour, minute, err := reservationforms.ParseClock(text)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%02d%02d", hour, minute)
}

func retryAction(kind formActionKind, id int64, form reservation.Form) formAction {
	return formAction{Kind: kind, ReservationID: id, StartText: typedClock(form.StartAt), EndText: typedClock(form.EndAt), SpotText: form.Spot}
}

// outcomeForm is the form behind an outcome, for a retry after a button.
func outcomeForm(outcome *reservation.FormOutcome) reservation.Form {
	return reservation.Form{Spot: outcome.SpotName, StartAt: outcome.Draft.StartAt.Format("15:04"), EndAt: outcome.Draft.EndAt.Format("15:04")}
}

// listView is the member's reservation list. extra buttons go first in the last row.
func (b *Bot) listView(page *reservation.Page, status string, now time.Time, extra ...discordgo.MessageComponent) formView {
	components := make([]discordgo.MessageComponent, 0, listLimit+1)
	for i, r := range page.Items {
		if i == listLimit {
			break
		}
		n := strconv.Itoa(i + 1)
		components = append(components, row(
			button("Edit "+n, discordgo.SecondaryButton, formAction{Kind: actionEditForm, ReservationID: r.Reservation.ID}),
			button("Cancel "+n, discordgo.DangerButton, formAction{Kind: actionCancel, ReservationID: r.Reservation.ID}),
		))
	}
	last := make([]discordgo.MessageComponent, 0, len(extra)+3)
	last = append(last, extra...)
	last = append(last,
		button("Book a respawn", discordgo.PrimaryButton, formAction{Kind: actionBookForm}),
		button("Refresh", discordgo.SecondaryButton, formAction{Kind: actionList}),
	)
	if b.webBaseURL != "" {
		last = append(last, linkButton("Open panel", b.webBaseURL))
	}
	components = append(components, row(last...))
	return formView{content: b.formatter.FormatMyReservations(page, status, now), components: components}
}

func (b *Bot) confirmCancelView(r *reservation.ReservationWithSpot) formView {
	return formView{
		content: b.formatter.FormatCancelConfirm(r),
		components: []discordgo.MessageComponent{row(
			button("Yes, cancel it", discordgo.DangerButton, formAction{Kind: actionCancelConfirm, ReservationID: r.Reservation.ID}),
			button("No, keep it", discordgo.SecondaryButton, formAction{Kind: actionList}),
		)},
	}
}

// spotPickView asks which respawn the member meant. pick carries the times, the
// select gives the spot id.
func (b *Bot) spotPickView(ambiguous *reservationforms.AmbiguousSpotError, pick formAction, back plainButton) formView {
	options := make([]plainOption, 0, len(ambiguous.Candidates))
	window := b.formatter.FormatWindow(pick.StartAt, pick.EndAt)
	for _, candidate := range ambiguous.Candidates {
		options = append(options, plainOption{
			Label:       cut(candidate.Name, maxLabelLength),
			Value:       strconv.FormatInt(candidate.ID, 10),
			Description: window,
		})
	}
	return formView{
		content: b.formatter.FormatSpotPick(ambiguous.Query),
		components: []discordgo.MessageComponent{
			row(plainSelect{CustomID: pick.customID(), Placeholder: "Choose a respawn", Options: options}),
			row(back),
		},
	}
}

func (b *Bot) bookSuccessView(outcome *reservation.FormOutcome) formView {
	return formView{
		content: b.formatter.FormatFormBooked(outcome),
		components: []discordgo.MessageComponent{row(
			button("My reservations", discordgo.SecondaryButton, formAction{Kind: actionMine}),
			button("Book another", discordgo.PrimaryButton, formAction{Kind: actionBookForm}),
		)},
	}
}

// bookOutcomeView is the reply to a booking. form is what the member typed; after
// a button it is the form of the outcome.
func (b *Bot) bookOutcomeView(outcome *reservation.FormOutcome, err error, form reservation.Form) formView {
	retry := button("Try again", discordgo.SecondaryButton, retryAction(actionRetry, 0, form))
	var ambiguous *reservationforms.AmbiguousSpotError
	switch {
	case err == nil:
		return b.bookSuccessView(outcome)
	case errors.As(err, &ambiguous) && outcome != nil:
		pick := formAction{Kind: actionPick, StartAt: outcome.Draft.StartAt, EndAt: outcome.Draft.EndAt}
		return b.spotPickView(ambiguous, pick, retry)
	case errors.Is(err, booking.ErrInsufficientPermissions) && outcome != nil && len(outcome.Conflicts) > 0:
		buttons := []discordgo.MessageComponent{}
		if outcome.CanOverbook {
			overbook := formAction{Kind: actionOverbook, SpotID: outcome.Draft.SpotID, StartAt: outcome.Draft.StartAt, EndAt: outcome.Draft.EndAt}
			buttons = append(buttons, button("Overbook them", discordgo.DangerButton, overbook))
		}
		buttons = append(buttons, retry)
		return formView{content: b.formatter.FormatFormConflicts(outcome), components: []discordgo.MessageComponent{row(buttons...)}}
	}
	return formView{content: b.formatter.FormatFormError(err), components: []discordgo.MessageComponent{row(retry)}}
}

// editOutcomeView is the list after an edit, with the result as its status.
func (b *Bot) editOutcomeView(id int64, outcome *reservation.FormOutcome, err error, form reservation.Form, page *reservation.Page, now time.Time) formView {
	if err == nil {
		return b.listView(page, b.formatter.FormatFormEdited(outcome), now)
	}
	retry := button("Try again", discordgo.SecondaryButton, retryAction(actionEditRetry, id, form))
	var ambiguous *reservationforms.AmbiguousSpotError
	if errors.As(err, &ambiguous) && outcome != nil {
		pick := formAction{Kind: actionEditPick, ReservationID: id, StartAt: outcome.Draft.StartAt, EndAt: outcome.Draft.EndAt}
		return b.spotPickView(ambiguous, pick, button("Back to the list", discordgo.SecondaryButton, formAction{Kind: actionList}))
	}
	status := b.formatter.FormatFormError(err)
	if errors.Is(err, booking.ErrConflict) && outcome != nil && len(outcome.Conflicts) > 0 {
		status = b.formatter.FormatFormConflicts(outcome)
	}
	return b.listView(page, "Not changed. "+status, now, retry)
}
