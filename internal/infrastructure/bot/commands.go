package bot

import (
	"fmt"

	"spot-assistant/internal/common/strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleCommand(i *discordgo.InteractionCreate) {
	var err error
	name := i.ApplicationCommandData().Name
	isAutocomplete := i.Type == discordgo.InteractionApplicationCommandAutocomplete
	log := b.log.With("interaction_name", name, "isAutocomplete", isAutocomplete)

	if !isAutocomplete {
		err = b.interactionRespond(i, &discordgo.InteractionResponseData{}, discordgo.InteractionResponseDeferredChannelMessageWithSource)
		if err != nil {
			b.log.Error(fmt.Errorf("could not send a deferred response: %w", err))

			return
		}
	}

	switch name {
	case "book":
		if isAutocomplete {
			err = b.BookAutocomplete(i)
		} else {
			err = b.Book(i)
		}
	case "unbook":
		if isAutocomplete {
			err = b.UnbookAutocomplete(i)
		} else {
			err = b.Unbook(i)
		}
	case "summary":
		err = b.PrivateSummary(i)
	default:
		err = fmt.Errorf("missing handler for command: %s", name)
	}

	if err != nil {
		log.Error(err)

		if !isAutocomplete {
			webhookParams := &discordgo.WebhookParams{
				Content: b.formatter.FormatGenericError(err),
			}

			gID, err := strings.StrToInt64(i.GuildID)
			if err != nil {
				b.log.Errorf("could not translate guildID: %s", err)
				return
			}

			dcSession := b.mgr.SessionForGuild(gID)
			_, err = dcSession.FollowupMessageCreate(i.Interaction, false, webhookParams)

			if err != nil {
				b.log.Errorf("could not respond with an error message: %s", err)
			}
		}
	}
}

var commands = []*discordgo.ApplicationCommand{
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
				Description:  "An hour the hunt shall end (e.g. 17:20)",
				Type:         discordgo.ApplicationCommandOptionBoolean,
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
