package bot

import (
	"fmt"

	"spot-assistant/internal/common/strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleCommand(i *discordgo.InteractionCreate) {
	name := i.ApplicationCommandData().Name
	isAutocomplete := i.Type == discordgo.InteractionApplicationCommandAutocomplete
	log := b.log.With("interaction_name", name, "isAutocomplete", isAutocomplete)

	if isAutocomplete {
		if err := b.handleAutocomplete(i); err != nil {
			log.Error(err)
		}
		return
	}

    // metrics: count non-autocomplete slash command invocations
    if b.metrics != nil {
        var guildName string
        if gID, convErr := strings.StrToInt64(i.GuildID); convErr == nil {
            if g, err := b.GetGuild(gID); err == nil && g != nil {
                guildName = g.Name
            }
        }
        b.metrics.IncSlashCommand(i.GuildID, guildName, name)
    }

	// Send deferred response for slash commands
	if err := b.interactionRespond(i, &discordgo.InteractionResponseData{}, discordgo.InteractionResponseDeferredChannelMessageWithSource); err != nil {
		b.log.Error(fmt.Errorf("could not send a deferred response: %w", err))
		return
	}

	err := b.handleSlash(i)

	if err != nil {
		log.Error(err)
            if b.metrics != nil {
                var guildName string
                if gID, convErr := strings.StrToInt64(i.GuildID); convErr == nil {
                    if g, err := b.GetGuild(gID); err == nil && g != nil {
                        guildName = g.Name
                    }
                }
                b.metrics.IncCommandError(i.GuildID, guildName, name)
            }
		webhookParams := &discordgo.WebhookParams{Content: b.formatter.FormatGenericError(err)}
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

// duplicate helpers removed

func (b *Bot) handleSlash(i *discordgo.InteractionCreate) error {
	switch i.ApplicationCommandData().Name {
	case "book":
		return b.Book(i)
	case "unbook":
		return b.Unbook(i)
	case "summary":
		return b.PrivateSummary(i)
	case "world-set":
		return b.SetWorld(i)
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
			Name:        "summary",
			Description: "Request a summary snapshot",
			Type:        discordgo.ChatApplicationCommand,
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
