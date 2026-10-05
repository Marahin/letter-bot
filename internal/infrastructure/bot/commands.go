package bot

import (
	"context"
	"errors"
	"fmt"

	"spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/guildconfig"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleCommand(i *discordgo.InteractionCreate) {
	name := i.ApplicationCommandData().Name
	isAutocomplete := i.Type == discordgo.InteractionApplicationCommandAutocomplete
	log := b.log.With("interaction_name", name, "isAutocomplete", isAutocomplete)

	cfg, err := b.guildConfig(context.Background(), i.GuildID)
	if err != nil {
		log.Errorf("could not load guild config: %s", err)
		if !isAutocomplete {
			b.respondEphemeral(i, b.formatter.FormatGenericError(errors.New("could not load the server settings, please try again")))
		}
		return
	}

	if isAutocomplete {
		if err := b.respondAutocomplete(i, cfg); err != nil {
			log.Error(err)
		}
		return
	}

	if reply := commandGateReply(cfg, name, i.ChannelID, b.webBaseURL); reply != "" {
		b.respondEphemeral(i, reply)
		return
	}

	// metrics: count non-autocomplete slash command invocations
	if b.metrics != nil {
		b.metrics.IncSlashCommand(i.GuildID, b.guildName(i.GuildID), name)
	}

	deferData := &discordgo.InteractionResponseData{}
	if name == "reservations" {
		deferData.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := b.interactionRespond(i, deferData, discordgo.InteractionResponseDeferredChannelMessageWithSource); err != nil {
		b.log.Error(fmt.Errorf("could not send a deferred response: %w", err))
		return
	}

	err = b.handleSlash(i, cfg)

	if err != nil {
		log.Error(err)
		if b.metrics != nil {
			b.metrics.IncCommandError(i.GuildID, b.guildName(i.GuildID), name)
		}
		webhookParams := &discordgo.WebhookParams{Content: b.formatter.FormatGenericError(err), Flags: deferData.Flags}
		gID, convErr := strings.StrToInt64(i.GuildID)
		if convErr != nil {
			b.log.Errorf("could not translate guildID: %s", convErr)
			return
		}
		dcSession := b.mgr.SessionForGuild(gID)
		if _, respErr := dcSession.FollowupMessageCreate(i.Interaction, false, webhookParams); respErr != nil {
			b.log.Errorf("could not respond with an error message: %s", respErr)
		}
	}
}

func (b *Bot) guildName(guildID string) string {
	gID, err := strings.StrToInt64(guildID)
	if err != nil {
		return ""
	}
	g, err := b.GetGuild(gID)
	if err != nil || g == nil {
		return ""
	}
	return g.Name
}

func (b *Bot) respondAutocomplete(i *discordgo.InteractionCreate, cfg *guildconfig.Config) error {
	if !cfg.IsPremium() {
		return b.interactionRespond(i, &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}}, discordgo.InteractionApplicationCommandAutocompleteResult)
	}
	return b.handleAutocomplete(i)
}

// duplicate helpers removed

func (b *Bot) respondEphemeral(i *discordgo.InteractionCreate, content string) {
	err := b.interactionRespond(i, &discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	}, discordgo.InteractionResponseChannelMessageWithSource)
	if err != nil {
		b.log.Errorf("could not send an ephemeral response: %s", err)
	}
}

func (b *Bot) handleSlash(i *discordgo.InteractionCreate, cfg *guildconfig.Config) error {
	switch i.ApplicationCommandData().Name {
	case "book":
		return b.Book(i, cfg)
	case "unbook":
		return b.Unbook(i)
	case "summary":
		return b.PrivateSummary(i)
	case "world-set":
		return b.SetWorld(i)
	case "reservations":
		return b.MyReservations(i)
	default:
		return fmt.Errorf("missing handler for command: %s", i.ApplicationCommandData().Name)
	}
}

func (b *Bot) handleAutocomplete(i *discordgo.InteractionCreate) error {
	switch i.ApplicationCommandData().Name {
	case "book":
		return b.BookAutocomplete(i)
	case "unbook":
		return b.UnbookAutocomplete(i)
	case "world-set":
		return b.SetWorldAutocomplete(i)
	case "summary":
		return b.SummaryAutocomplete(i)
	default:
		return fmt.Errorf("missing handler for command: %s", i.ApplicationCommandData().Name)
	}
}

func (b *Bot) getCommands() []*discordgo.ApplicationCommand {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "book",
			Description: "Book a respawn",
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
					Name:         "start-at",
					Description:  "An hour the hunt shall start (e.g. 15:20)",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "end-at",
					Description:  "An hour the hunt shall end (e.g. 17:20)",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "overbook",
					Description:  "Should try to overbook existing reservations",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     false,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "unbook",
			Description: "Cancel a respawn booking",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "reservation",
					Description:  "Reservation to be cancelled",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "reservations",
			Description: "Show, edit and cancel your upcoming reservations",
			Type:        discordgo.ChatApplicationCommand,
		},
		{
			Name:        "summary",
			Description: "Request a summary snapshot (optionally for a specific respawn)",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "respawn",
					Description:  "Name of the respawn (optional)",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     false,
					Autocomplete: true,
				},
			},
		},
	}
	// it's intentionally not 'set-world' as it would appear alphabetically higher than summary and unbook - purely for UX - its only used once and only by owner
	// only register world-set if onlineCheckService is configured
	if b.onlineCheckService != nil && b.onlineCheckService.IsConfigured() {
		commands = append(commands, &discordgo.ApplicationCommand{
			Name:        "world-set",
			Description: "Set the Tibia world for this server (owner only)",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "world",
					Description:  "Tibia world name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		})
	}

	return commands
}
