package bot

import (
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/reservationforms"
)

// The selects of step 1 are told apart by their index.
const (
	usualSelectIndex   = 0
	firstGroupIndex    = 1
	matchesSelectIndex = 4
)

// respawnStepView is step 1 of the wizard. state carries the reservation of an
// edit and the start and the length to keep.
func (b *Bot) respawnStepView(p *reservation.RespawnPicker, state formAction, status string) formView {
	components := make([]discordgo.MessageComponent, 0, maxRows)
	if len(p.Usual) > 0 {
		options := make([]plainOption, 0, len(p.Usual))
		for _, u := range p.Usual {
			options = append(options, spotOption(u.Spot, b.formatter.FormatUsualRespawn(u)))
		}
		components = append(components, row(respawnSelect(state, usualSelectIndex, usualPlaceholder(p.Usual), options)))
	}
	for i, g := range p.Groups {
		options := make([]plainOption, 0, len(g.Spots))
		for _, sp := range g.Spots {
			options = append(options, spotOption(sp, ""))
		}
		components = append(components, row(respawnSelect(state, firstGroupIndex+i, groupPlaceholder(g), options)))
	}
	components = append(components, row(respawnNav(p, state)...))
	empty := len(p.Groups) == 0
	return formView{content: b.formatter.FormatRespawnStep(state.editing(), empty, status), components: components}
}

// matchesView replaces the respawn lists with the respawns that match a search.
func (b *Bot) matchesView(matches *reservationforms.AmbiguousSpotError, state formAction) formView {
	options := make([]plainOption, 0, len(matches.Candidates))
	for _, sp := range matches.Candidates {
		options = append(options, spotOption(reservation.Spot{ID: sp.ID, Name: sp.Name}, ""))
	}
	all := state.as(actionRespawnPage)
	all.Page = 0
	nav := []discordgo.MessageComponent{
		button("Search again", discordgo.PrimaryButton, state.as(actionSearch)),
		button("All respawns", discordgo.SecondaryButton, all),
	}
	if state.editing() {
		nav = append(nav, button("Back", discordgo.SecondaryButton, formAction{Kind: actionList}))
	}
	return formView{
		content: b.formatter.FormatRespawnMatches(matches.Query, matches.Capped),
		components: []discordgo.MessageComponent{
			row(respawnSelect(state, matchesSelectIndex, "Matching respawns", options)),
			row(nav...),
		},
	}
}

func respawnSelect(state formAction, index int, placeholder string, options []plainOption) plainSelect {
	pick := state.as(actionRespawnPick)
	pick.Index = index
	return plainSelect{CustomID: pick.customID(), Placeholder: cut(placeholder, maxPlaceholderLength), Options: options}
}

func usualPlaceholder(usual []reservation.UsualRespawn) string {
	if usual[0].Bookings == 0 {
		return "Popular respawns"
	}
	return "Your usual respawns"
}

func groupPlaceholder(g reservation.RespawnGroup) string {
	if g.From == g.To {
		return "Respawns " + g.From
	}
	return "Respawns " + g.From + "–" + g.To
}

func spotOption(sp reservation.Spot, description string) plainOption {
	return plainOption{Label: cut(sp.Name, maxOptionLength), Value: strconv.FormatInt(sp.ID, 10), Description: cut(description, maxOptionLength)}
}

// respawnNav is the button row of step 1. A disabled button keeps the current
// page, so its custom id stays unique.
func respawnNav(p *reservation.RespawnPicker, state formAction) []discordgo.MessageComponent {
	buttons := make([]discordgo.MessageComponent, 0, 4)
	if p.Pages > 1 {
		previous, next := state.as(actionRespawnPage), state.as(actionRespawnPage)
		previous.Page, next.Page = max(p.Page-1, 0), min(p.Page+1, p.Pages-1)
		buttons = append(buttons,
			disabledButton("‹ Previous", discordgo.SecondaryButton, previous, p.Page == 0),
			disabledButton("More respawns ›", discordgo.SecondaryButton, next, p.Page == p.Pages-1),
		)
	}
	buttons = append(buttons, button("Search by name", discordgo.PrimaryButton, state.as(actionSearch)))
	if state.editing() {
		buttons = append(buttons, button("Back", discordgo.SecondaryButton, formAction{Kind: actionList}))
	}
	return buttons
}

// timeStepView is step 2 of the wizard: the start, the length and the submit.
func (b *Bot) timeStepView(p *reservation.TimePicker, status string, now time.Time) formView {
	state := choiceAction(p.Choice)
	starts := make([]plainOption, 0, len(p.Starts))
	for _, o := range p.Starts {
		label, description := b.formatter.FormatStartOption(o, now)
		starts = append(starts, plainOption{
			Label:       cut(label, maxOptionLength),
			Value:       choiceValue(o.Now, o.StartAt),
			Description: cut(description, maxOptionLength),
			Default:     o.Selected,
		})
	}
	lengths := make([]plainOption, 0, len(p.Lengths))
	for _, o := range p.Lengths {
		label := b.formatter.FormatLength(o.Length)
		if o.Current {
			label += " (current)"
		}
		lengths = append(lengths, plainOption{Label: label, Value: strconv.Itoa(int(o.Length / time.Minute)), Default: o.Selected})
	}
	components := []discordgo.MessageComponent{
		row(plainSelect{CustomID: state.as(actionStartPick).customID(), Placeholder: "Start", Options: starts}),
		row(plainSelect{CustomID: state.as(actionLengthPick).customID(), Placeholder: "Length", Options: lengths}),
	}
	if !p.Ongoing {
		earlier, later := state.as(actionWindow), state.as(actionWindow)
		earlier.Window, later.Window = max(p.Choice.Window-1, 0), min(p.Choice.Window+1, p.Windows-1)
		components = append(components, row(
			disabledButton("‹ Earlier", discordgo.SecondaryButton, earlier, p.Choice.Window == 0),
			disabledButton("Later ›", discordgo.SecondaryButton, later, p.Choice.Window == p.Windows-1),
		))
	}
	submit := "Book"
	if p.Editing != nil {
		submit = "Save"
	}
	actions := []discordgo.MessageComponent{disabledButton(submit, discordgo.SuccessButton, state.as(actionSubmit), !p.Choice.Complete())}
	if !p.Ongoing {
		change := state.as(actionRespawnPage)
		change.Page = 0
		actions = append(actions, button("Change respawn", discordgo.SecondaryButton, change))
	}
	if p.Editing != nil {
		actions = append(actions, button("Back", discordgo.SecondaryButton, formAction{Kind: actionList}))
	}
	components = append(components, row(actions...))
	return formView{content: b.formatter.FormatTimeStep(p, status, now), components: components}
}

// wizardErrorView explains why a step could not show, with a way back.
func (b *Bot) wizardErrorView(err error, state formAction) formView {
	back := button("Back", discordgo.SecondaryButton, formAction{Kind: actionList})
	if !state.editing() {
		back = button("Choose a respawn", discordgo.SecondaryButton, formAction{Kind: actionRespawnPage})
	}
	return formView{content: b.formatter.FormatFormError(err), components: []discordgo.MessageComponent{row(back)}}
}
