package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"
)

/*
*
System events that are initialized by Discord.
*/

func (b *Bot) GuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	b.log.With("event", "GuildCreate", "guild_name", g.Name, "g.ID", g.ID).Info("guild created")
	guild := MapGuild(g.Guild)
	// Register commands
	err := b.RegisterCommands(guild)
	if err != nil {
		b.log.Errorf("could not overwrite commands: %s", err)

		return
	}
	//
	err = b.EnsureChannel(guild)
	if err != nil {
		b.log.Errorf("could not ensure channels: %s", err)

		return
	}
	//
	err = b.EnsureRoles(guild)
	if err != nil {
		b.log.Errorf("could not ensure roles: %s", err)

		return
	}
	b.ConfigureWorldNameForGuild(guild.ID)
	go b.TryUpdateGuildLetter(guild)
	defer b.eventHandler.OnGuildCreate(MapGuild(g.Guild))
}

func (b *Bot) Ready(s *discordgo.Session, r *discordgo.Ready) {
	b.log.Debug("Ready")
	for _, g := range s.State.Guilds {
		b.ConfigureWorldNameForGuild(g.ID)
	}
	b.StartTicking()

	defer b.eventHandler.OnReady()
}

// InteractionCreate this is the entry point when a slash command is invoked.
func (b *Bot) InteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.log.Debug("InteractionCreate")
	tStart := time.Now()

	b.handleCommand(i)

	b.log.With("duration", time.Since(tStart)).Debug("interaction handled")
}

func (b *Bot) Tick() {
	b.log.Debug("Tick")
	b.log.Info("About to refresh online players")
	guilds := b.GetGuilds()
	for _, guild := range guilds {
		guild := guild
		b.ConfigureWorldNameForGuild(guild.ID)
		go b.onlineCheckService.TryRefresh(guild.ID)
		go b.TryUpdateGuildLetter(guild)
	}

	defer b.eventHandler.OnTick()
}

func (b *Bot) Book(i *discordgo.InteractionCreate) error {
	b.log.Info("Book")
	interaction := i.Interaction
	tNow := time.Now()
	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}
	dcSession := b.mgr.SessionForGuild(gID)

	// Flag parsing
	overbook := false
	switch len(i.ApplicationCommandData().Options) {
	case 4:
		overbook = i.ApplicationCommandData().Options[3].StringValue() == "true"
	case 3:
		break
	default:
		return errors.New("book command requires 3 arguments")
	}

	startAtStr := sanitizeTimeFormat(i.ApplicationCommandData().Options[1].StringValue())
	startAt, err := time.Parse(stringsHelper.DcTimeFormat, startAtStr)
	if err != nil {
		return err
	}
	startAt = time.Date(
		tNow.Year(), tNow.Month(), tNow.Day(), startAt.Hour(), startAt.Minute(), 0, 0, tNow.Location())

	endAtStr := sanitizeTimeFormat(i.ApplicationCommandData().Options[2].StringValue())
	endAt, err := time.Parse(stringsHelper.DcTimeFormat, endAtStr)
	if err != nil {
		return err
	}
	endAt = time.Date(
		tNow.Year(), tNow.Month(), tNow.Day(), endAt.Hour(), endAt.Minute(), 0, 0, tNow.Location())

	if startAt.Before(tNow) {
		startAt = startAt.Add(24 * time.Hour)
		endAt = endAt.Add(24 * time.Hour)
	}

	if startAt.After(endAt) {
		endAt = endAt.Add(24 * time.Hour)
	}

	guild, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	member := MapMember(i.Member)
	request := book.BookRequest{
		Member:         member,
		Guild:          guild,
		Spot:           i.ApplicationCommandData().Options[0].StringValue(),
		StartAt:        startAt,
		EndAt:          endAt,
		HasPermissions: b.MemberHasRole(guild, member, discord.PrivilegedRole),
		Overbook:       overbook,
	}

	tStart := time.Now()
	response, err := b.eventHandler.OnBook(request)
	bookLog := b.log.With("duration", time.Since(tStart), "error", err)
	var message string
	if err != nil {
		message = b.formatter.FormatBookError(response, err)
	} else {
		go b.TryUpdateGuildLetter(guild)
		message = b.formatter.FormatBookResponse(response)
	}

	bookLog.Info("booking request handled")
	_, err = dcSession.FollowupMessageCreate(interaction, false, &discordgo.WebhookParams{
		Content: message,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
	})
	return err
}

func (b *Bot) BookAutocomplete(i *discordgo.InteractionCreate) error {
	selectedOption, index := collections.PoorMansFind(i.ApplicationCommandData().Options,
		func(o *discordgo.ApplicationCommandInteractionDataOption) bool {
			return o.Focused
		})
	if index == -1 {
		return errors.New("none of the options were selected for autocompletion")
	}

	response, err := b.eventHandler.OnBookAutocomplete(book.BookAutocompleteRequest{
		Field: book.BookAutocompleteFocus(index),
		Value: selectedOption.StringValue(),
	})
	if err != nil {
		return err
	}

	responseData := &discordgo.InteractionResponseData{
		Choices: MapStringArrToChoice(response),
	}
	return b.interactionRespond(i, responseData, discordgo.InteractionApplicationCommandAutocompleteResult)
}

func (b *Bot) Unbook(i *discordgo.InteractionCreate) error {
	if len(i.ApplicationCommandData().Options) < 1 {
		return errors.New("you must select a reservation to unbook")
	}

	reservationId, err := stringsHelper.StrToInt64(i.ApplicationCommandData().Options[0].StringValue())
	if err != nil {
		return fmt.Errorf("could not parse reservation id: %v", reservationId)
	}

	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return fmt.Errorf("could not parse guild id: %v", i.GuildID)
	}

	guild, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	res, err := b.eventHandler.OnUnbook(book.UnbookRequest{
		Member:        MapMember(i.Member),
		Guild:         guild,
		ReservationID: reservationId,
	})
	if err != nil {
		return err
	}

	go b.TryUpdateGuildLetter(guild)

	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: b.formatter.FormatUnbookResponse(res),
	})
	return err
}

func (b *Bot) UnbookAutocomplete(i *discordgo.InteractionCreate) error {
	selectedOption, index := collections.PoorMansFind(i.ApplicationCommandData().Options,
		func(o *discordgo.ApplicationCommandInteractionDataOption) bool {
			return o.Focused
		})
	if index == -1 {
		return fmt.Errorf("none of the options were selected for autocompletion")
	}

	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}

	guild, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	request := book.UnbookAutocompleteRequest{
		Guild:  guild,
		Member: MapMember(i.Member),
		Value:  selectedOption.StringValue(),
	}

	response, err := b.eventHandler.OnUnbookAutocomplete(request)
	if err != nil {
		return err
	}

	responseData := &discordgo.InteractionResponseData{
		Choices: MapReservationWithSpotArrToChoice(response.Choices),
	}

	return b.interactionRespond(i, responseData, discordgo.InteractionApplicationCommandAutocompleteResult)
}

func (b *Bot) PrivateSummary(i *discordgo.InteractionCreate) error {
	b.log.Debug("PrivateSummary")

	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}

	uID, err := stringsHelper.StrToInt64(i.Member.User.ID)
	if err != nil {
		return err
	}

	err = b.eventHandler.OnPrivateSummary(summary.PrivateSummaryRequest{
		GuildID: gID,
		UserID:  uID,
	})
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{Content: "Check your DM!"})
	return err
}

func (b *Bot) SetWorld(i *discordgo.InteractionCreate) error {
	guildID := i.GuildID
	userID := i.Member.User.ID

	// Fetch guild to check owner
	guild, err := b.mgr.Gateway.Guild(guildID)
	if err != nil {
		return fmt.Errorf("could not fetch guild: %w", err)
	}
	if guild.OwnerID != userID {
		return fmt.Errorf("only the server owner can use this command")
	}

	world := ""
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "world" {
			world = opt.StringValue()
		}
	}
	if world == "" {
		return fmt.Errorf("world name is required")
	}

	// Save or update the world name for this guild in the database
	err = b.SetGuildWorld(guildID, world)
	if err != nil {
		return fmt.Errorf("failed to save world name: %w", err)
	}

	// Configure the online checker with the new world name
	b.ConfigureWorldNameForGuild(guildID)

	gID, err := stringsHelper.StrToInt64(guildID)
	if err != nil {
		return fmt.Errorf("could not parse guild id: %v", guildID)
	}
	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Tibia world for this server set to: **%s**", world),
	})
	return err
}

func (b *Bot) ConfigureWorldNameForGuild(guildID string) {
	guildWorld, err := b.worldNameRepo.SelectGuildWorld(context.Background(), guildID)
	if err == nil && guildWorld != nil && guildWorld.WorldName != "" {
		b.onlineCheckService.ConfigureWorldName(guildID, guildWorld.WorldName)
	}
}
