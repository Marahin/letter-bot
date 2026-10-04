package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/core/worlds"
	"spot-assistant/internal/ports"
)

/*
*
System events that are initialized by Discord.
*/

func (b *Bot) GuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	log := b.log.With("event", "GuildCreate", "guild_name", g.Name, "g.ID", g.ID)
	log.Info("guild created")
	gld := MapGuild(g.Guild)
	ctx := context.Background()

	cfg := b.storePresence(ctx, gld)

	err := b.RegisterCommands(gld)
	if err != nil {
		log.Errorf("could not overwrite commands: %s", err)

		return
	}

	if err := b.syncer.Sync(ctx, gld.ID); err != nil {
		log.Errorf("could not sync guild channels and roles: %s", err)
	}

	if err := b.onlineCheckService.ConfigureWorldNameForGuild(gld.ID); err != nil {
		log.Errorf("ConfigureWorldNameForGuild failed for guild %s: %v", gld.ID, err)
	}

	if cfg == nil || !cfg.IsPremium() {
		log.Info("guild is not premium, the bot stays inactive")

		return
	}

	if err := b.setupPremiumGuild(gld, cfg); err != nil {
		log.Error(err)

		return
	}

	go b.onlineCheckService.TryRefresh(gld.ID)
	go b.TryUpdateGuildLetter(gld)
	b.eventHandler.OnGuildCreate(MapGuild(g.Guild))
}

// storePresence stores the guild and returns its configuration. When the write
// fails it falls back to the stored configuration, so a premium guild stays active.
// It returns nil only when neither works.
func (b *Bot) storePresence(ctx context.Context, g *guild.Guild) *guildconfig.Config {
	log := b.log.With("guild.ID", g.ID)
	cfg, err := b.guildConfigs.UpsertPresence(ctx, g.ID, g.Name, g.Icon, g.OwnerID)
	if err == nil {
		return cfg
	}
	log.Errorf("could not store guild presence: %s", err)
	cfg, err = b.guildConfig(ctx, g.ID)
	if err != nil {
		log.Errorf("could not load guild config: %s", err)
		return nil
	}
	return cfg
}

func (b *Bot) GuildUpdate(s *discordgo.Session, g *discordgo.GuildUpdate) {
	if _, err := b.guildConfigs.UpsertPresence(context.Background(), g.ID, g.Name, g.Icon, g.OwnerID); err != nil {
		b.log.With("event", "GuildUpdate", "g.ID", g.ID).Errorf("could not store guild presence: %s", err)
	}
}

func (b *Bot) GuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	// Unavailable means a Discord outage, not that the bot left the guild.
	if g.Unavailable {
		return
	}
	if err := b.guildConfigs.SetBotPresent(context.Background(), g.ID, false); err != nil && !errors.Is(err, ports.ErrNotFound) {
		b.log.With("event", "GuildDelete", "g.ID", g.ID).Errorf("could not mark the bot absent: %s", err)
	}
}

func (b *Bot) ChannelCreate(s *discordgo.Session, c *discordgo.ChannelCreate) {
	b.scheduleSync(c.GuildID)
}

func (b *Bot) ChannelUpdate(s *discordgo.Session, c *discordgo.ChannelUpdate) {
	b.scheduleSync(c.GuildID)
}

func (b *Bot) ChannelDelete(s *discordgo.Session, c *discordgo.ChannelDelete) {
	b.scheduleSync(c.GuildID)
}

func (b *Bot) GuildRoleCreate(s *discordgo.Session, r *discordgo.GuildRoleCreate) {
	b.scheduleSync(r.GuildID)
}

func (b *Bot) GuildRoleUpdate(s *discordgo.Session, r *discordgo.GuildRoleUpdate) {
	b.scheduleSync(r.GuildID)
}

func (b *Bot) GuildRoleDelete(s *discordgo.Session, r *discordgo.GuildRoleDelete) {
	b.scheduleSync(r.GuildID)
}

func (b *Bot) scheduleSync(guildID string) {
	if guildID == "" {
		return
	}
	b.syncer.Schedule(guildID)
}

func (b *Bot) Ready(s *discordgo.Session, r *discordgo.Ready) {
	readyAt := time.Now()
	b.markAbsentGuilds(r, readyAt)
	for _, g := range s.State.Guilds {
		if err := b.onlineCheckService.ConfigureWorldNameForGuild(g.ID); err != nil {
			b.log.Errorf("ConfigureWorldNameForGuild failed for guild %s: %v", g.ID, err)
		}
	}
	b.StartTicking()

	b.eventHandler.OnReady()
}

// markAbsentGuilds clears bot_present for the guilds that removed the bot while it
// was offline. Each shard's Ready lists only the guilds of that shard. A guild
// that joined after readyAt is kept, because its GuildCreate may run first.
func (b *Bot) markAbsentGuilds(r *discordgo.Ready, readyAt time.Time) {
	shardID, shardCount := 0, 1
	if r.Shard != nil && r.Shard[1] > 0 {
		shardID, shardCount = r.Shard[0], r.Shard[1]
	}
	ids := make([]string, 0, len(r.Guilds))
	for _, g := range r.Guilds {
		ids = append(ids, g.ID)
	}
	if err := b.guildConfigs.MarkAbsentExcept(context.Background(), shardID, shardCount, ids, readyAt); err != nil {
		b.log.With("event", "Ready", "shard", shardID).Errorf("could not mark absent guilds: %s", err)
	}
}

// InteractionCreate this is the entry point when a slash command is invoked.
func (b *Bot) InteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tStart := time.Now()

	b.handleCommand(i)

	b.log.With("duration", time.Since(tStart)).Debug("interaction handled")
}

func (b *Bot) Tick() {
	defer b.metrics.IncTicks()
	defer b.eventHandler.OnTick()

	ctx := context.Background()
	b.log.Info("About to refresh online players")
	guilds := b.GetGuilds()
	ids := collections.PoorMansMap(guilds, func(g *guild.Guild) string { return g.ID })
	cfgs, err := b.guildConfigsByID(ctx, ids)
	if err != nil {
		b.log.Errorf("could not load guild configs, skipping summaries: %s", err)
	}
	for _, g := range guilds {
		cfg, ok := cfgs[g.ID]
		if !ok || !cfg.IsPremium() {
			continue
		}
		go b.onlineCheckService.TryRefresh(g.ID)
		go func() {
			if err := b.updateGuildLetterWithConfig(g, cfg); err != nil {
				b.log.Errorf("could not update guild letter: %s", err)
			}
		}()
	}

	b.processResyncRequests(ctx)
}

func (b *Bot) processResyncRequests(ctx context.Context) {
	guildIDs, err := b.guildConfigs.ListResyncRequested(ctx)
	if err != nil {
		b.log.Errorf("could not list guild resync requests: %s", err)
		return
	}
	for _, guildID := range guildIDs {
		if err := b.syncer.Sync(ctx, guildID); err != nil {
			b.log.With("guild.ID", guildID).Errorf("could not sync guild: %s", err)
		}
	}
}

func (b *Bot) Book(i *discordgo.InteractionCreate, cfg *guildconfig.Config) error {
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
	default:
		return errors.New("book command requires 3 arguments")
	}

	// metrics: track overbook flag usage
	if overbook && b.metrics != nil {
		var guildName string
		if g, err := b.GetGuild(gID); err == nil && g != nil {
			guildName = g.Name
		}
		b.metrics.IncOverbook(i.GuildID, guildName)
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

	g, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	member := MapMember(i.Member)
	caps := permission.Resolve(*cfg, memberSubject(g, member))
	if !caps.Reserve {
		return booking.ErrReserveNotAllowed
	}
	request := book.BookRequest{
		Member:         member,
		Guild:          g,
		Spot:           i.ApplicationCommandData().Options[0].StringValue(),
		StartAt:        startAt,
		EndAt:          endAt,
		HasPermissions: caps.Overbook,
		Overbook:       overbook,
	}

	tStart := time.Now()
	response, err := b.eventHandler.OnBook(request)
	bookLog := b.log.With("duration", time.Since(tStart), "error", err)
	var message string
	if err != nil {
		message = b.formatter.FormatBookError(response, err)
	} else {
		go b.TryUpdateGuildLetter(g)
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
		GuildID: i.GuildID,
		Field:   book.BookAutocompleteFocus(index),
		Value:   selectedOption.StringValue(),
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

	reservationID, err := stringsHelper.StrToInt64(i.ApplicationCommandData().Options[0].StringValue())
	if err != nil {
		return fmt.Errorf("could not parse reservation id: %v", reservationID)
	}

	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return fmt.Errorf("could not parse guild id: %v", i.GuildID)
	}

	g, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	res, err := b.eventHandler.OnUnbook(book.UnbookRequest{
		Member:        MapMember(i.Member),
		Guild:         g,
		ReservationID: reservationID,
	})
	if err != nil {
		return err
	}

	go b.TryUpdateGuildLetter(g)

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
		return errors.New("none of the options were selected for autocompletion")
	}

	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}

	g, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	request := book.UnbookAutocompleteRequest{
		Guild:  g,
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
	gID, err := stringsHelper.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}

	uID, err := stringsHelper.StrToInt64(i.Member.User.ID)
	if err != nil {
		return err
	}

	var spotName string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "respawn" {
			spotName = opt.StringValue()
			break
		}
	}
	err = b.eventHandler.OnPrivateSummary(summary.PrivateSummaryRequest{
		GuildID:  gID,
		UserID:   uID,
		SpotName: spotName,
	})
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{Content: "Check your DM!"})
	return err
}

func (b *Bot) SummaryAutocomplete(i *discordgo.InteractionCreate) error {
	var spotFilter string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Focused && opt.Name == "respawn" {
			spotFilter = opt.StringValue()
			break
		}
	}

	response, err := b.eventHandler.OnBookAutocomplete(book.BookAutocompleteRequest{
		GuildID: i.GuildID,
		Field:   book.BookAutocompleteSpot,
		Value:   spotFilter,
	})
	if err != nil {
		return err
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(response))
	for _, v := range response {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: v, Value: v})
	}

	return b.interactionRespond(i, &discordgo.InteractionResponseData{Choices: choices}, discordgo.InteractionApplicationCommandAutocompleteResult)
}

func (b *Bot) SetWorld(i *discordgo.InteractionCreate) error {
	guildID := i.GuildID
	userID := i.Member.User.ID

	g, err := b.mgr.Gateway.Guild(guildID)
	if err != nil {
		return fmt.Errorf("could not fetch guild: %w", err)
	}
	if g.OwnerID != userID {
		return errors.New("only the server owner can use this command")
	}

	world := ""
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "world" {
			world = opt.StringValue()
			break
		}
	}
	if world == "" {
		return errors.New("world name is required")
	}

	// Validate world against the allowed list
	existingWorld, idx := collections.PoorMansFind(worlds.Worlds, func(w string) bool {
		return strings.EqualFold(w, world)
	})
	if idx == -1 {
		return fmt.Errorf("invalid world name: %s, please select a valid Tibia world", world)
	}
	world = existingWorld

	err = b.onlineCheckService.SetGuildWorld(guildID, world)
	if err != nil {
		return fmt.Errorf("failed to save world name: %w", err)
	}

	// Configure the online checker with the new world name
	err = b.onlineCheckService.ConfigureWorldNameForGuild(guildID)
	if err != nil {
		b.log.Errorf("ConfigureWorldNameForGuild failed for guild %s: %v", guildID, err)
	}

	gID, err := stringsHelper.StrToInt64(guildID)
	if err != nil {
		return fmt.Errorf("could not parse guild id: %v", guildID)
	}
	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Tibia world for this server set to: **%s**", world),
	})
	return err
}

func (b *Bot) SetWorldAutocomplete(i *discordgo.InteractionCreate) error {
	var userInput string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Focused && opt.Name == "world" {
			userInput = opt.StringValue()
			break
		}
	}

	var filtered []*discordgo.ApplicationCommandOptionChoice
	for _, w := range worlds.Worlds {
		if userInput == "" || strings.Contains(strings.ToLower(w), strings.ToLower(userInput)) {
			filtered = append(filtered, &discordgo.ApplicationCommandOptionChoice{Name: w, Value: w})
		}
		if len(filtered) >= 25 { // Discord max choices
			break
		}
	}

	return b.interactionRespond(i, &discordgo.InteractionResponseData{
		Choices: filtered,
	}, discordgo.InteractionApplicationCommandAutocompleteResult)
}
