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
)

// formRequest is the member and the guild of a button or a form.
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
	g, err := b.gateway.Guild(i.GuildID)
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

func (b *Bot) handleComponent(i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	b.runFormAction(i, data.CustomID, data.Values, reservation.Form{})
}

func (b *Bot) handleModalSubmit(i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	values := modalValues(data)
	b.runFormAction(i, data.CustomID, nil, reservation.Form{Spot: values[inputSpot], StartAt: values[inputStart], EndAt: values[inputEnd]})
}

// modalValues maps the text input ids of a submitted form to their values.
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

func (b *Bot) runFormAction(i *discordgo.InteractionCreate, customID string, selected []string, form reservation.Form) {
	ctx := context.Background()
	action, err := parseFormAction(customID)
	if err != nil {
		b.log.With("custom_id", customID).Debugf("unknown custom id: %s", err)
		b.respondEphemeral(i, b.formatter.FormatFormOutdated())
		return
	}
	switch action.Kind {
	case actionBookForm, actionRetry:
		b.openBookForm(ctx, i, action)
	case actionEditForm, actionEditRetry:
		b.openEditForm(ctx, i, action)
	case actionMine:
		b.deferred(ctx, i, discordgo.InteractionResponseDeferredChannelMessageWithSource, func(req *formRequest) formView {
			return b.myReservationsView(ctx, req, "")
		})
	case actionBookSubmit:
		b.deferred(ctx, i, discordgo.InteractionResponseDeferredChannelMessageWithSource, func(req *formRequest) formView {
			return b.bookFormSubmitted(ctx, req, form)
		})
	default:
		b.deferred(ctx, i, discordgo.InteractionResponseDeferredMessageUpdate, func(req *formRequest) formView {
			return b.updateInPlace(ctx, req, action, selected, form)
		})
	}
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

func (b *Bot) updateInPlace(ctx context.Context, req *formRequest, action formAction, selected []string, form reservation.Form) formView {
	guildID := req.guild.ID
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
	case actionPick:
		spotID, ok := selectedID(selected)
		if !ok {
			return formView{content: b.formatter.FormatFormOutdated()}
		}
		return b.bookDraft(ctx, req, reservation.Draft{SpotID: spotID, StartAt: action.StartAt, EndAt: action.EndAt})
	case actionEditPick:
		spotID, ok := selectedID(selected)
		if !ok {
			return formView{content: b.formatter.FormatFormOutdated()}
		}
		draft := reservation.Draft{SpotID: spotID, StartAt: action.StartAt, EndAt: action.EndAt}
		outcome, err := b.forms.Edit(ctx, guildID, req.actor, action.ReservationID, draft)
		b.logFormError("edit", err)
		return b.editOutcome(ctx, req, action.ReservationID, outcome, err, formOf(outcome))
	case actionEditSubmit:
		outcome, err := b.forms.EditForm(ctx, guildID, req.actor, action.ReservationID, form)
		b.logFormError("edit", err)
		return b.editOutcome(ctx, req, action.ReservationID, outcome, err, form)
	default:
		b.log.Errorf("no handler for form action %d", action.Kind)
		return formView{content: b.formatter.FormatFormOutdated()}
	}
}

func (b *Bot) openBookForm(ctx context.Context, i *discordgo.InteractionCreate, action formAction) {
	req, reply := b.formContext(ctx, i)
	if req == nil {
		b.respondEphemeral(i, reply)
		return
	}
	if !req.actor.Caps.Reserve {
		b.respondEphemeral(i, b.formatter.FormatFormError(booking.ErrReserveNotAllowed))
		return
	}
	if err := b.interactionRespond(i, bookModal(retryForm(action)), discordgo.InteractionResponseModal); err != nil {
		b.log.Errorf("could not open the book form: %s", err)
	}
}

func (b *Bot) openEditForm(ctx context.Context, i *discordgo.InteractionCreate, action formAction) {
	req, reply := b.formContext(ctx, i)
	if req == nil {
		b.respondEphemeral(i, reply)
		return
	}
	r, err := b.forms.Editable(ctx, req.guild.ID, req.actor, action.ReservationID)
	if err != nil {
		b.logFormError("open edit form", err)
		b.respondEphemeral(i, b.formatter.FormatFormError(err))
		return
	}
	form := existingForm(r)
	if action.Kind == actionEditRetry {
		form = retryForm(action)
	}
	if err := b.interactionRespond(i, editModal(r.Reservation.ID, form), discordgo.InteractionResponseModal); err != nil {
		b.log.Errorf("could not open the edit form: %s", err)
	}
}

func (b *Bot) bookFormSubmitted(ctx context.Context, req *formRequest, form reservation.Form) formView {
	if !req.actor.Caps.Reserve {
		return formView{content: b.formatter.FormatFormError(booking.ErrReserveNotAllowed)}
	}
	outcome, err := b.forms.BookForm(ctx, req.guild.ID, req.actor, form)
	b.logFormError("book", err)
	return b.bookOutcomeView(outcome, err, form)
}

func (b *Bot) bookDraft(ctx context.Context, req *formRequest, draft reservation.Draft) formView {
	if !req.actor.Caps.Reserve {
		return formView{content: b.formatter.FormatFormError(booking.ErrReserveNotAllowed)}
	}
	outcome, err := b.forms.Book(ctx, req.guild.ID, req.actor, draft)
	b.logFormError("book", err)
	return b.bookOutcomeView(outcome, err, formOf(outcome))
}

func (b *Bot) editOutcome(ctx context.Context, req *formRequest, id int64, outcome *reservation.FormOutcome, err error, form reservation.Form) formView {
	page, listErr := b.forms.Mine(ctx, req.guild.ID, req.actor, listLimit)
	if listErr != nil {
		b.log.Errorf("could not list reservations: %s", listErr)
		return formView{content: b.formatter.FormatGenericError(listErr)}
	}
	return b.editOutcomeView(id, outcome, err, form, page, time.Now())
}

// MyReservations answers /reservations, after handleCommand deferred it.
func (b *Bot) MyReservations(i *discordgo.InteractionCreate) error {
	ctx := context.Background()
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

// formOf is the form behind an outcome, for a retry after a button.
func formOf(outcome *reservation.FormOutcome) reservation.Form {
	if outcome == nil {
		return reservation.Form{}
	}
	return outcomeForm(outcome)
}

func selectedID(selected []string) (int64, bool) {
	if len(selected) != 1 {
		return 0, false
	}
	id, err := strconv.ParseInt(selected[0], 10, 64)
	return id, err == nil && id > 0
}
