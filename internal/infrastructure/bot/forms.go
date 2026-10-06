package bot

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/core/reservationforms"
)

const interactionTimeout = 10 * time.Second

type formRequest struct {
	guild *guild.Guild
	actor reservation.Actor
}

// formContext resolves the clicking member's rights fresh, so a custom id never
// carries a right. reply is the refusal to show when the request cannot go on.
func (b *Bot) formContext(ctx context.Context, i *discordgo.InteractionCreate) (req *formRequest, reply string) {
	if i.GuildID == "" || i.Member == nil || i.Member.User == nil {
		return nil, b.formatter.FormatFormNoGuild()
	}
	cfg, err := b.guildConfig(ctx, i.GuildID)
	if err != nil {
		b.log.With("guild.ID", i.GuildID).Errorf("could not load guild config: %s", err)
		return nil, b.formatter.FormatGenericError(errors.New("could not load the server settings, please try again"))
	}
	if !cfg.IsPremium() {
		return nil, notPremiumMessage(b.webBaseURL)
	}
	g, err := b.formGuild(i.GuildID)
	if err != nil {
		b.log.With("guild.ID", i.GuildID).Errorf("could not fetch guild: %s", err)
		return nil, b.formatter.FormatGenericError(errors.New("could not load the server, please try again"))
	}
	gld := MapGuild(g)
	m := MapMember(i.Member)
	name := m.Nick
	if name == "" {
		name = m.Username
	}
	return &formRequest{
		guild: gld,
		actor: reservation.Actor{UserID: m.ID, Name: name, Caps: permission.Resolve(*cfg, memberSubject(gld, m))},
	}, ""
}

// formGuild prefers the gateway cache: a REST call before a modal opens eats into
// Discord's 3 second limit.
func (b *Bot) formGuild(guildID string) (*discordgo.Guild, error) {
	g, err := b.gateway.StateGuild(guildID)
	if err == nil {
		return g, nil
	}
	b.log.With("guild.ID", guildID).Debugf("guild not in the gateway state, asking the API: %s", err)
	return b.gateway.Guild(guildID)
}

func (b *Bot) handleComponent(i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	b.runFormAction(i, data.CustomID, data.Values, "")
}

func (b *Bot) handleModalSubmit(i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	b.runFormAction(i, data.CustomID, nil, modalValues(data)[inputQuery])
}

func modalValues(data discordgo.ModalSubmitInteractionData) map[string]string {
	values := map[string]string{}
	for _, component := range data.Components {
		r, ok := component.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, inner := range r.Components {
			if input, ok := inner.(*discordgo.TextInput); ok {
				values[input.CustomID] = input.Value
			}
		}
	}
	return values
}

// runFormAction answers a click, a select or a form. query is the text of the
// search form.
func (b *Bot) runFormAction(i *discordgo.InteractionCreate, customID string, selected []string, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
	defer cancel()
	action, err := parseFormAction(customID)
	if err != nil {
		b.log.With("custom_id", customID).Debugf("unknown custom id: %s", err)
		b.respondEphemeral(i, b.formatter.FormatFormOutdated())
		return
	}
	switch action.Kind {
	case actionSearch, actionEditSearch:
		b.openSearch(ctx, i, action)
	case actionMine:
		b.deferred(ctx, i, openKind(i), func(req *formRequest) formView {
			return b.myReservationsView(ctx, req, "")
		})
	case actionBook:
		b.deferred(ctx, i, openKind(i), func(req *formRequest) formView {
			return b.respawnStep(ctx, req, formAction{}, "")
		})
	default:
		b.deferred(ctx, i, discordgo.InteractionResponseDeferredMessageUpdate, func(req *formRequest) formView {
			return b.updateInPlace(ctx, req, action, selected, query)
		})
	}
}

// openKind answers a click in the public summary with a new ephemeral message,
// and a click in an ephemeral message in place, so the member keeps one message.
func openKind(i *discordgo.InteractionCreate) discordgo.InteractionResponseType {
	if i.Message != nil && i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		return discordgo.InteractionResponseDeferredMessageUpdate
	}
	return discordgo.InteractionResponseDeferredChannelMessageWithSource
}

// deferred acknowledges the interaction first, so a slow database does not miss
// Discord's 3 second limit, and then edits the reply with the view.
func (b *Bot) deferred(ctx context.Context, i *discordgo.InteractionCreate, kind discordgo.InteractionResponseType, render func(req *formRequest) formView) {
	var data *discordgo.InteractionResponseData
	if kind == discordgo.InteractionResponseDeferredChannelMessageWithSource {
		data = &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}
	}
	if err := b.interactionRespond(i, data, kind); err != nil {
		b.log.Errorf("could not defer the interaction: %s", err)
		return
	}
	req, reply := b.formContext(ctx, i)
	view := formView{content: reply}
	if req != nil {
		view = render(req)
	}
	if _, err := b.gateway.InteractionResponseEdit(i.Interaction, view.edit()); err != nil {
		b.log.Errorf("could not edit the interaction response: %s", err)
	}
}

func (b *Bot) updateInPlace(ctx context.Context, req *formRequest, action formAction, selected []string, query string) formView {
	guildID := req.guild.ID
	if !action.editing() && isBookingStep(action.Kind) && !req.actor.Caps.Reserve {
		return formView{content: b.formatter.FormatFormError(booking.ErrReserveNotAllowed)}
	}
	switch action.Kind {
	case actionList:
		return b.myReservationsView(ctx, req, "")
	case actionCancel:
		r, err := b.forms.Cancellable(ctx, guildID, req.actor, action.ReservationID)
		if err != nil {
			return b.myReservationsView(ctx, req, b.formatter.FormatFormError(err))
		}
		return b.confirmCancelView(r)
	case actionCancelConfirm:
		r, err := b.forms.Cancel(ctx, guildID, req.actor, action.ReservationID)
		if err != nil {
			return b.myReservationsView(ctx, req, b.formatter.FormatFormError(err))
		}
		return b.myReservationsView(ctx, req, b.formatter.FormatCancelled(r))
	case actionOverbook:
		if b.metrics != nil {
			b.metrics.IncOverbook(guildID, req.guild.Name)
		}
		draft := reservation.Draft{SpotID: action.SpotID, StartAt: action.StartAt, EndAt: action.EndAt, Overbook: true}
		return b.bookDraft(ctx, req, draft)
	case actionEdit:
		return b.timeStep(ctx, req, formAction{ReservationID: action.ReservationID, Window: reservation.AutoWindow})
	case actionRespawnPage, actionEditRespawnPage:
		return b.respawnStep(ctx, req, action, "")
	case actionRespawnPick, actionEditRespawnPick:
		spotID, ok := selectedID(selected)
		if !ok {
			return formView{content: b.formatter.FormatFormOutdated()}
		}
		action.SpotID, action.Window = spotID, reservation.AutoWindow
		return b.timeStep(ctx, req, action)
	case actionSearchSubmit, actionEditSearchSubmit:
		return b.searchResult(ctx, req, action, query)
	case actionStartPick, actionEditStartPick:
		now, startAt, ok := selectedStart(selected)
		if !ok {
			return formView{content: b.formatter.FormatFormOutdated()}
		}
		action.Now, action.StartAt = now, startAt
		return b.timeStep(ctx, req, action)
	case actionLengthPick, actionEditLengthPick:
		length, ok := selectedLength(selected)
		if !ok {
			return formView{content: b.formatter.FormatFormOutdated()}
		}
		action.Length = length
		return b.timeStep(ctx, req, action)
	case actionWindow, actionEditWindow:
		return b.timeStep(ctx, req, action)
	case actionSubmit:
		outcome, err := b.forms.BookChoice(ctx, guildID, req.actor, action.choice())
		b.logFormError("book", err)
		return b.bookOutcomeView(outcome, err, retryState(action))
	case actionEditSubmit:
		outcome, err := b.forms.EditChoice(ctx, guildID, req.actor, action.choice())
		b.logFormError("edit", err)
		return b.editOutcome(ctx, req, outcome, err, retryState(action))
	default:
		b.log.Errorf("no handler for form action %d", action.Kind)
		return formView{content: b.formatter.FormatFormOutdated()}
	}
}

// isBookingStep tells the wizard actions of a booking, which need the reserve right.
func isBookingStep(kind formActionKind) bool {
	_, ok := editTwin[kind]
	return ok
}

func retryState(action formAction) formAction {
	action.Window = reservation.AutoWindow
	return action
}

func (b *Bot) respawnStep(ctx context.Context, req *formRequest, state formAction, status string) formView {
	if !state.editing() && !req.actor.Caps.Reserve {
		return formView{content: b.formatter.FormatFormError(booking.ErrReserveNotAllowed)}
	}
	picker, err := b.forms.RespawnPicker(ctx, req.guild.ID, req.actor, state.Page)
	if err != nil {
		b.log.Errorf("could not list respawns: %s", err)
		return formView{content: b.formatter.FormatGenericError(err)}
	}
	return b.respawnStepView(picker, state, status)
}

func (b *Bot) timeStep(ctx context.Context, req *formRequest, state formAction) formView {
	picker, err := b.forms.TimePicker(ctx, req.guild.ID, req.actor, state.choice())
	if err != nil {
		b.logFormError("time picker", err)
		return b.wizardErrorView(err, state)
	}
	return b.timeStepView(picker, "", time.Now())
}

func (b *Bot) searchResult(ctx context.Context, req *formRequest, state formAction, query string) formView {
	sp, err := b.forms.FindRespawn(ctx, req.guild.ID, query)
	var ambiguous *reservationforms.AmbiguousSpotError
	switch {
	case err == nil:
		state.SpotID, state.Window = sp.ID, reservation.AutoWindow
		return b.timeStep(ctx, req, state)
	case errors.As(err, &ambiguous):
		return b.matchesView(ambiguous, state)
	case errors.Is(err, booking.ErrSpotNotFound):
		state.Page = 0
		return b.respawnStep(ctx, req, state, b.formatter.FormatFormError(err))
	}
	b.log.Errorf("could not search respawns: %s", err)
	return b.wizardErrorView(err, state)
}

// openSearch opens the search form; a form must be the first reply to a click.
func (b *Bot) openSearch(ctx context.Context, i *discordgo.InteractionCreate, action formAction) {
	req, reply := b.formContext(ctx, i)
	if req == nil {
		b.respondEphemeral(i, reply)
		return
	}
	if !action.editing() && !req.actor.Caps.Reserve {
		b.respondEphemeral(i, b.formatter.FormatFormError(booking.ErrReserveNotAllowed))
		return
	}
	if err := b.interactionRespond(i, searchModal(action.as(actionSearchSubmit)), discordgo.InteractionResponseModal); err != nil {
		b.log.Errorf("could not open the search form: %s", err)
	}
}

func (b *Bot) bookDraft(ctx context.Context, req *formRequest, draft reservation.Draft) formView {
	if !req.actor.Caps.Reserve {
		return formView{content: b.formatter.FormatFormError(booking.ErrReserveNotAllowed)}
	}
	outcome, err := b.forms.Book(ctx, req.guild.ID, req.actor, draft)
	b.logFormError("book", err)
	retry := formAction{SpotID: draft.SpotID, StartAt: draft.StartAt, Length: draft.EndAt.Sub(draft.StartAt), Window: reservation.AutoWindow}
	return b.bookOutcomeView(outcome, err, retry)
}

func (b *Bot) editOutcome(ctx context.Context, req *formRequest, outcome *reservation.FormOutcome, err error, retry formAction) formView {
	page, listErr := b.forms.Mine(ctx, req.guild.ID, req.actor, listLimit)
	if listErr != nil {
		b.log.Errorf("could not list reservations: %s", listErr)
		page = nil
	}
	return b.editOutcomeView(outcome, err, retry, page, time.Now())
}

// MyReservations answers /reservations, after handleCommand deferred it.
func (b *Bot) MyReservations(i *discordgo.InteractionCreate) error {
	ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
	defer cancel()
	req, reply := b.formContext(ctx, i)
	view := formView{content: reply}
	if req != nil {
		view = b.myReservationsView(ctx, req, "")
	}
	_, err := b.gateway.InteractionResponseEdit(i.Interaction, view.edit())
	return err
}

func (b *Bot) myReservationsView(ctx context.Context, req *formRequest, status string) formView {
	page, err := b.forms.Mine(ctx, req.guild.ID, req.actor, listLimit)
	if err != nil {
		b.log.Errorf("could not list reservations: %s", err)
		return formView{content: b.formatter.FormatGenericError(err)}
	}
	return b.listView(page, status, time.Now())
}

func (b *Bot) logFormError(what string, err error) {
	if err != nil {
		b.log.With("form", what).Debugf("form refused: %s", err)
	}
}

func selectedStart(selected []string) (bool, time.Time, bool) {
	if len(selected) != 1 {
		return false, time.Time{}, false
	}
	now, startAt, err := parseChoice(selected[0])
	return now, startAt, err == nil && (now || !startAt.IsZero())
}

func selectedLength(selected []string) (time.Duration, bool) {
	if len(selected) != 1 {
		return 0, false
	}
	minutes, err := parseSmall(selected[0], maxLengthMin)
	return time.Duration(minutes) * time.Minute, err == nil && minutes > 0
}

func selectedID(selected []string) (int64, bool) {
	if len(selected) != 1 {
		return 0, false
	}
	id, err := strconv.ParseInt(selected[0], 10, 64)
	return id, err == nil && id > 0
}
