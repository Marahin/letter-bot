package bot

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
)

const (
	// listLimit leaves one of Discord's five component rows for the list actions.
	listLimit = 4
	// Discord's limits.
	maxRows              = 5
	maxButtonsPerRow     = 5
	maxLabelLength       = 80
	maxOptionLength      = 100
	maxPlaceholderLength = 150
	maxSelectOptions     = 25
	maxSearchQueryLength = 100
	minSearchQueryLength = 2
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
	Disabled bool                  `json:"disabled,omitempty"`
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
	Default     bool   `json:"default,omitempty"`
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

func disabledButton(label string, style discordgo.ButtonStyle, action formAction, disabled bool) plainButton {
	b := button(label, style, action)
	b.Disabled = disabled
	return b
}

func searchModal(submit formAction) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		CustomID: submit.customID(),
		Title:    "Search for a respawn",
		Components: []discordgo.MessageComponent{row(discordgo.TextInput{
			CustomID:    inputQuery,
			Label:       "Respawn",
			Style:       discordgo.TextInputShort,
			Placeholder: "Name or a part of it, e.g. Library",
			Required:    true,
			MinLength:   minSearchQueryLength,
			MaxLength:   maxSearchQueryLength,
		})},
	}
}

// listView is the member's reservation list, page nil when it could not be read.
// extra buttons go first in the last row.
func (b *Bot) listView(page *reservation.Page, status string, now time.Time, extra ...discordgo.MessageComponent) formView {
	components := make([]discordgo.MessageComponent, 0, listLimit+1)
	var items []*reservation.ReservationWithSpot
	if page != nil {
		items = page.Items
	}
	for i, r := range items {
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

func (b *Bot) bookSuccessView(outcome *reservation.FormOutcome) formView {
	return formView{
		content: b.formatter.FormatFormBooked(outcome),
		components: []discordgo.MessageComponent{row(
			button("My reservations", discordgo.SecondaryButton, formAction{Kind: actionMine}),
			button("Book another", discordgo.PrimaryButton, formAction{Kind: actionBookForm}),
		)},
	}
}

// bookOutcomeView is the reply to a booking. retry returns to step 2 with the
// choice kept.
func (b *Bot) bookOutcomeView(outcome *reservation.FormOutcome, err error, retry formAction) formView {
	if err == nil {
		return b.bookSuccessView(outcome)
	}
	retryButton := button("Try again", discordgo.SecondaryButton, retry.as(actionWindow))
	if errors.Is(err, booking.ErrInsufficientPermissions) && outcome != nil && len(outcome.Conflicts) > 0 {
		buttons := []discordgo.MessageComponent{}
		if outcome.CanOverbook {
			overbook := formAction{Kind: actionOverbook, SpotID: outcome.Draft.SpotID, StartAt: outcome.Draft.StartAt, EndAt: outcome.Draft.EndAt}
			buttons = append(buttons, button("Overbook them", discordgo.DangerButton, overbook))
		}
		buttons = append(buttons, retryButton)
		return formView{content: b.formatter.FormatFormConflicts(outcome), components: []discordgo.MessageComponent{row(buttons...)}}
	}
	return formView{content: b.formatter.FormatFormError(err), components: []discordgo.MessageComponent{row(retryButton)}}
}

// editOutcomeView is the list after an edit, with the result as its status.
// retry returns to step 2 with the choice kept.
func (b *Bot) editOutcomeView(outcome *reservation.FormOutcome, err error, retry formAction, page *reservation.Page, now time.Time) formView {
	if err == nil {
		return b.listView(page, b.formatter.FormatFormEdited(outcome), now)
	}
	status := b.formatter.FormatFormError(err)
	if errors.Is(err, booking.ErrConflict) && outcome != nil && len(outcome.Conflicts) > 0 {
		status = b.formatter.FormatFormConflicts(outcome)
	}
	return b.listView(page, "Not changed. "+status, now, button("Try again", discordgo.SecondaryButton, retry.as(actionWindow)))
}
