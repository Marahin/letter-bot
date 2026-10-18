package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
	corequeue "spot-assistant/internal/core/queue"
	"spot-assistant/internal/core/reservationforms"
	"spot-assistant/internal/ports"
)

const (
	commandQueue      = "queue"
	commandQueueLeave = "queue-leave"
	// queueLookupTimeout bounds the read before an offer click is answered:
	// a refusal must be the first reply, within Discord's 3 seconds.
	queueLookupTimeout = 2 * time.Second
)

// sendQueueOffers is the summary pass hook of the queue. Running in the pass
// keeps the offers paced with the other summary writes, through one session.
func (b *Bot) sendQueueOffers(ctx context.Context, guildID string) error {
	due, err := b.queue.DueOffers(ctx, guildID)
	if len(due) == 0 {
		return err
	}
	errs := []error{err}
	cfg, cfgErr := b.guildConfig(ctx, guildID)
	for _, e := range due {
		sendErr := cfgErr
		if sendErr == nil {
			sendErr = b.sendQueueOffer(ctx, cfg, e)
		}
		if sendErr == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("send offer %d: %w", e.ID, sendErr))
		if err := b.queue.Unclaim(ctx, e.ID); err != nil {
			errs = append(errs, fmt.Errorf("unclaim offer %d: %w", e.ID, err))
		}
	}
	return errors.Join(errs...)
}

// sendQueueOffer sends the offer by DM. A member who does not take DMs from
// the server gets it with a mention in the command channel.
func (b *Bot) sendQueueOffer(ctx context.Context, cfg *guildconfig.Config, e queue.Entry) error {
	if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
		return err
	}
	dm, err := b.writer.UserChannelCreate(e.UserID)
	if err != nil {
		return fmt.Errorf("open DM: %w", err)
	}
	if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
		return err
	}
	_, err = b.writer.ChannelMessageSendComplex(dm.ID, b.queueOfferMessage(e, cfg.Name, false))
	if err == nil {
		return nil
	}
	if !isDiscordCode(err, discordgo.ErrCodeCannotSendMessagesToThisUser) {
		return fmt.Errorf("send DM: %w", err)
	}
	channel, chErr := b.announceChannel(cfg)
	if chErr != nil {
		return fmt.Errorf("DMs are closed and no command channel was found: %w", chErr)
	}
	if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
		return err
	}
	if _, err := b.writer.ChannelMessageSendComplex(channel.ID, b.queueOfferMessage(e, "", true)); err != nil {
		return fmt.Errorf("send to channel %s: %w", channel.ID, err)
	}
	return nil
}

func (b *Bot) queueOfferMessage(e queue.Entry, guildName string, mention bool) *discordgo.MessageSend {
	content := b.formatter.FormatQueueOffer(e, guildName, offerDeadline(e))
	allowed := &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
	if mention {
		content = "<@" + e.UserID + "> " + content
		allowed.Users = []string{e.UserID}
	}
	return &discordgo.MessageSend{Content: content, Components: offerComponents(e), AllowedMentions: allowed}
}

func offerDeadline(e queue.Entry) time.Time {
	if e.OfferedAt == nil {
		return time.Time{}
	}
	return e.OfferedAt.Add(corequeue.OfferTTL).Local()
}

func offerComponents(e queue.Entry) []discordgo.MessageComponent {
	var offeredAt time.Time
	if e.OfferedAt != nil {
		offeredAt = *e.OfferedAt
	}
	return []discordgo.MessageComponent{row(
		button("Book", discordgo.SuccessButton, formAction{Kind: actionQueueBook, QueueID: e.ID, StartAt: offeredAt}),
		cancelOfferButton(e.ID, offeredAt),
	)}
}

func cancelOfferButton(entryID int64, offeredAt time.Time) plainButton {
	return button("Cancel", discordgo.SecondaryButton, formAction{Kind: actionQueueCancel, QueueID: entryID, StartAt: offeredAt})
}

// queueOfferClick answers Book and Cancel of an offer. The click may come from
// a DM, which has no guild and no member: the guild comes from the entry, the
// member from the API.
func (b *Bot) queueOfferClick(ctx context.Context, i *discordgo.InteractionCreate, action formAction) {
	userID := interactionUserID(i)
	lookup, cancel := context.WithTimeout(ctx, queueLookupTimeout)
	e, err := b.queue.Entry(lookup, action.QueueID)
	cancel()
	switch {
	case errors.Is(err, ports.ErrNotFound):
		b.respondEphemeral(i, b.formatter.FormatQueueGone())
		return
	case err != nil:
		b.log.With("queue.ID", action.QueueID).Errorf("could not load the queue entry: %s", err)
		b.respondEphemeral(i, b.formatter.FormatGenericError(errors.New("could not load the queue, please try again")))
		return
	case e.UserID != userID:
		b.respondEphemeral(i, b.formatter.FormatFormError(corequeue.ErrOfferNotYours))
		return
	}
	if err := b.interactionRespond(i, nil, discordgo.InteractionResponseDeferredMessageUpdate); err != nil {
		b.log.Errorf("could not defer the interaction: %s", err)
		return
	}
	var view formView
	if action.Kind == actionQueueCancel {
		view = b.queueCancelView(ctx, e, action.StartAt)
	} else {
		view = b.queueBookView(ctx, i, e, action.StartAt)
	}
	if _, err := b.gateway.InteractionResponseEdit(i.Interaction, view.edit()); err != nil {
		b.log.Errorf("could not edit the interaction response: %s", err)
	}
}

func (b *Bot) queueCancelView(ctx context.Context, e *queue.Entry, offeredAt time.Time) formView {
	err := b.queue.Leave(ctx, e.UserID, e.ID)
	if err != nil && !errors.Is(err, ports.ErrNotFound) {
		b.log.With("queue.ID", e.ID).Errorf("could not leave the queue: %s", err)
		return formView{
			content:    b.formatter.FormatGenericError(errors.New("could not leave the queue, please try again")),
			components: []discordgo.MessageComponent{row(cancelOfferButton(e.ID, offeredAt))},
		}
	}
	return formView{content: b.formatter.FormatQueueLeft(e.SpotName)}
}

func (b *Bot) queueBookView(ctx context.Context, i *discordgo.InteractionCreate, e *queue.Entry, offeredAt time.Time) formView {
	now := time.Now()
	if err := b.queue.CheckOffer(e, e.UserID, offeredAt); err != nil {
		return formView{content: b.formatter.FormatQueueOfferExpired(*e, now)}
	}
	keepCancel := []discordgo.MessageComponent{row(cancelOfferButton(e.ID, offeredAt))}
	actor, reply := b.queueActor(ctx, i, e)
	if reply != "" {
		return formView{content: reply, components: keepCancel}
	}
	outcome, err := b.queue.Book(ctx, actor, e.ID, offeredAt)
	b.logFormError("queue book", err)
	switch {
	case err == nil:
		return formView{content: b.formatter.FormatFormBooked(outcome)}
	case errors.Is(err, corequeue.ErrOfferExpired):
		return formView{content: b.formatter.FormatQueueOfferExpired(*e, now)}
	case errors.Is(err, ports.ErrNotFound):
		return formView{content: b.formatter.FormatQueueGone()}
	case errors.Is(err, booking.ErrInsufficientPermissions) && outcome != nil && len(outcome.Conflicts) > 0:
		// The offer has no Overbook button; the wizard has.
		outcome.CanOverbook = false
		return formView{content: b.formatter.FormatFormConflicts(outcome), components: keepCancel}
	}
	return formView{content: b.formatter.FormatFormError(err), components: keepCancel}
}

// queueActor resolves the clicking member's rights in the entry's guild, as
// formContext does for a click in the guild.
func (b *Bot) queueActor(ctx context.Context, i *discordgo.InteractionCreate, e *queue.Entry) (reservation.Actor, string) {
	log := b.log.With("guild.ID", e.GuildID)
	cfg, err := b.guildConfig(ctx, e.GuildID)
	if err != nil {
		log.Errorf("could not load guild config: %s", err)
		return reservation.Actor{}, b.formatter.FormatGenericError(errors.New("could not load the server settings, please try again"))
	}
	if !cfg.IsPremium() {
		return reservation.Actor{}, notPremiumMessage(b.webBaseURL)
	}
	g, err := b.formGuild(e.GuildID)
	if err != nil {
		log.Errorf("could not fetch guild: %s", err)
		return reservation.Actor{}, b.formatter.FormatGenericError(errors.New("could not load the server, please try again"))
	}
	gld := MapGuild(g)
	if i.GuildID == e.GuildID && i.Member != nil && i.Member.User != nil {
		return memberActor(cfg, gld, MapMember(i.Member)), ""
	}
	dm, err := b.gateway.GuildMember(e.GuildID, e.UserID)
	if err != nil {
		log.Warnf("could not fetch the queued member: %s", err)
		return reservation.Actor{}, b.formatter.FormatGenericError(errors.New("could not find you on the server, please try again"))
	}
	m := MapMember(dm)
	// A member from the API has no permissions field: only interactions carry it.
	m.Permissions = rolePermissions(gld, m.Roles)
	return memberActor(cfg, gld, m), ""
}

// rolePermissions are the server-wide permissions of the roles, @everyone (the
// role with the guild's id) included.
func rolePermissions(g *guild.Guild, roleIDs []string) int64 {
	held := make(map[string]bool, len(roleIDs)+1)
	held[g.ID] = true
	for _, id := range roleIDs {
		held[id] = true
	}
	var perms int64
	for _, r := range g.Roles {
		if r != nil && held[r.ID] {
			perms |= r.Permissions
		}
	}
	return perms
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (b *Bot) joinQueue(ctx context.Context, req *formRequest, spotID int64, length time.Duration) formView {
	outcome, err := b.queue.Join(ctx, req.guild.ID, req.actor, spotID, length)
	if err != nil {
		b.logFormError("queue", err)
		return formView{content: b.formatter.FormatFormError(err)}
	}
	if outcome.FreeNow {
		return formView{content: b.formatter.FormatQueueOffer(outcome.Entry, "", offerDeadline(outcome.Entry)), components: offerComponents(outcome.Entry)}
	}
	return formView{
		content: b.formatter.FormatQueueJoined(outcome.Entry, time.Now()),
		components: []discordgo.MessageComponent{row(
			button("Leave queue", discordgo.SecondaryButton, formAction{Kind: actionQueueLeave, QueueID: outcome.Entry.ID}),
		)},
	}
}

func (b *Bot) queueLeaveView(ctx context.Context, userID string, entryID int64) formView {
	e, err := b.queue.Entry(ctx, entryID)
	if err == nil && e.UserID != userID {
		err = ports.ErrNotFound
	}
	if err == nil {
		err = b.queue.Leave(ctx, userID, entryID)
	}
	switch {
	case err == nil:
		return formView{content: b.formatter.FormatQueueLeft(e.SpotName)}
	case errors.Is(err, ports.ErrNotFound):
		return formView{content: b.formatter.FormatQueueGone()}
	}
	b.log.With("queue.ID", entryID).Errorf("could not leave the queue: %s", err)
	return formView{content: b.formatter.FormatGenericError(err)}
}

// QueueJoin answers /queue, after handleCommand deferred it.
func (b *Bot) QueueJoin(i *discordgo.InteractionCreate) error {
	ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
	defer cancel()
	req, reply := b.formContext(ctx, i)
	view := formView{content: reply}
	if req != nil {
		view = b.queueJoinCommand(ctx, req, i.ApplicationCommandData().Options)
	}
	_, err := b.gateway.InteractionResponseEdit(i.Interaction, view.edit())
	return err
}

func (b *Bot) queueJoinCommand(ctx context.Context, req *formRequest, options []*discordgo.ApplicationCommandInteractionDataOption) formView {
	var name string
	var length time.Duration
	for _, o := range options {
		switch o.Name {
		case "respawn":
			name = o.StringValue()
		case "length":
			length = time.Duration(o.IntValue()) * time.Minute
		}
	}
	sp, err := b.forms.FindRespawn(ctx, req.guild.ID, name)
	var ambiguous *reservationforms.AmbiguousSpotError
	switch {
	case errors.As(err, &ambiguous):
		return formView{content: b.formatter.FormatQueueAmbiguous(ambiguous.Query)}
	case err != nil:
		b.logFormError("queue", err)
		return formView{content: b.formatter.FormatFormError(err)}
	}
	return b.joinQueue(ctx, req, sp.ID, length)
}

// QueueLeave answers /queue-leave, after handleCommand deferred it. The option
// is an entry id from the autocomplete, or a respawn name typed by hand.
func (b *Bot) QueueLeave(i *discordgo.InteractionCreate) error {
	ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
	defer cancel()
	var value string
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == "respawn" {
			value = o.StringValue()
		}
	}
	content := b.queueLeaveCommand(ctx, i.GuildID, interactionUserID(i), value)
	_, err := b.gateway.InteractionResponseEdit(i.Interaction, formView{content: content}.edit())
	return err
}

func (b *Bot) queueLeaveCommand(ctx context.Context, guildID, userID, value string) string {
	mine, err := b.queue.Mine(ctx, guildID, userID)
	if err != nil {
		b.log.With("guild.ID", guildID).Errorf("could not list the queues: %s", err)
		return b.formatter.FormatGenericError(err)
	}
	e := matchEntry(mine, value)
	if e == nil {
		return b.formatter.FormatQueueNotQueued(mine)
	}
	if err := b.queue.Leave(ctx, userID, e.ID); err != nil && !errors.Is(err, ports.ErrNotFound) {
		b.log.With("queue.ID", e.ID).Errorf("could not leave the queue: %s", err)
		return b.formatter.FormatGenericError(err)
	}
	return b.formatter.FormatQueueLeft(e.SpotName)
}

func matchEntry(entries []queue.Entry, value string) *queue.Entry {
	value = strings.TrimSpace(value)
	for i := range entries {
		if strconv.FormatInt(entries[i].ID, 10) == value || strings.EqualFold(entries[i].SpotName, value) {
			return &entries[i]
		}
	}
	return nil
}

// QueueAutocomplete suggests the respawns for /queue.
func (b *Bot) QueueAutocomplete(i *discordgo.InteractionCreate) error {
	var filter string
	for _, o := range i.ApplicationCommandData().Options {
		if o.Focused && o.Name == "respawn" {
			filter = o.StringValue()
		}
	}
	names, err := b.eventHandler.OnBookAutocomplete(book.BookAutocompleteRequest{GuildID: i.GuildID, Field: book.BookAutocompleteSpot, Value: filter})
	if err != nil {
		return err
	}
	return b.interactionRespond(i, &discordgo.InteractionResponseData{Choices: MapStringArrToChoice(names)}, discordgo.InteractionApplicationCommandAutocompleteResult)
}

// QueueLeaveAutocomplete suggests the respawns the member waits for.
func (b *Bot) QueueLeaveAutocomplete(i *discordgo.InteractionCreate) error {
	var filter string
	for _, o := range i.ApplicationCommandData().Options {
		if o.Focused {
			filter = strings.ToLower(o.StringValue())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), queueLookupTimeout)
	defer cancel()
	mine, err := b.queue.Mine(ctx, i.GuildID, interactionUserID(i))
	if err != nil {
		return err
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(mine))
	for _, e := range mine {
		if !strings.Contains(strings.ToLower(e.SpotName), filter) {
			continue
		}
		label := fmt.Sprintf("%s (%s)", e.SpotName, b.formatter.FormatLength(e.Length))
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: cut(label, maxOptionLength), Value: strconv.FormatInt(e.ID, 10)})
	}
	return b.interactionRespond(i, &discordgo.InteractionResponseData{Choices: choices}, discordgo.InteractionApplicationCommandAutocompleteResult)
}

// queueCommands are the slash commands of the queue.
func (b *Bot) queueCommands() []*discordgo.ApplicationCommand {
	lengths := make([]*discordgo.ApplicationCommandOptionChoice, 0, int(corequeue.MaxLength/corequeue.LengthStep))
	for d := corequeue.MinLength; d <= corequeue.MaxLength; d += corequeue.LengthStep {
		lengths = append(lengths, &discordgo.ApplicationCommandOptionChoice{Name: b.formatter.FormatLength(d), Value: int(d / time.Minute)})
	}
	return []*discordgo.ApplicationCommand{
		{
			Name:        commandQueue,
			Description: "Wait for a taken respawn and get a DM when it is free",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "respawn",
					Description:  "Name of the respawn",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "length",
					Description: "How long you want to hunt (default 2 h)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Choices:     lengths,
				},
			},
		},
		{
			Name:        commandQueueLeave,
			Description: "Stop waiting for a respawn",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "respawn",
					Description:  "The respawn you wait for",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
	}
}

// queueable tells whether the time step offers the Queue button.
func (b *Bot) queueable(p *reservation.TimePicker) bool {
	return b.queue != nil && p.TakenNow && p.Editing == nil
}
