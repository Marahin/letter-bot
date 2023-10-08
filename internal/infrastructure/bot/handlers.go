package bot

import (
	"fmt"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/util"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// System events
// Events that are sent by discord itself

func (b *Bot) GuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	b.log.Debug("GuildCreate")

	defer b.eventHandler.OnGuildCreate(b, MapGuild(g.Guild))
}

func (b *Bot) Ready(s *discordgo.Session, r *discordgo.Ready) {
	b.log.Debug("Ready")

	defer b.eventHandler.OnReady(b)
}

// When a slash command is invoked, this is the entry point.
func (b *Bot) InteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.log.Debug("InteractionCreate")
	tStart := time.Now()

	b.handleCommand(i)

	b.log.WithFields(logrus.Fields{"time": time.Since(tStart)}).Info("interaction handled")
}

// Service events
// Events that are sent by tickers or our custom integrations,
// such as commands.

func (b *Bot) Tick() {
	b.log.Debug("Tick")

	defer b.eventHandler.OnTick(b)
}

func (b *Bot) Book(i *discordgo.InteractionCreate) error {
	b.log.Debug("Book")
	tNow := time.Now()
	gID, err := util.StrToInt64(i.GuildID)
	if err != nil {
		return err
	}
	dcSession := b.mgr.SessionForGuild(gID)

	// Flag parsing
	overbook := false
	switch len(i.ApplicationCommandData().Options) {
	case 4:
		if i.ApplicationCommandData().Options[3].StringValue() == "true" {
			overbook = true
		}
	case 3:
		break
	default:
		return fmt.Errorf("Book command requires 3 arguments")
	}

	startAt, err := time.Parse(util.DC_TIME_FORMAT, i.ApplicationCommandData().Options[1].StringValue())
	if err != nil {
		return err
	}
	startAt = time.Date(
		tNow.Year(), tNow.Month(), tNow.Day(), startAt.Hour(), startAt.Minute(), 0, 0, tNow.Location())

	endAt, err := time.Parse(util.DC_TIME_FORMAT, i.ApplicationCommandData().Options[2].StringValue())
	if err != nil {
		return err
	}
	endAt = time.Date(
		tNow.Year(), tNow.Month(), tNow.Day(), endAt.Hour(), endAt.Minute(), 0, 0, tNow.Location())

	if startAt.Before(tNow) {
		b.log.Warning("moving startAt to next day, as it's already past the starting point")
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
		Member:   member,
		Guild:    guild,
		Spot:     i.ApplicationCommandData().Options[0].StringValue(),
		StartAt:  startAt,
		EndAt:    endAt,
		Overbook: overbook,
	}

	response, err := b.eventHandler.OnBook(b, request)

	message := strings.Builder{}
	if err != nil {
		message.WriteString("I'm sorry, but something went wrong. If you require support, join TibiaLoot.com Discord: https://discord.gg/F4YKgsnzmc \n")

		message.WriteString(fmt.Sprintf("Error message:\n```%s```\n", err))
	} else {
		message.WriteString(fmt.Sprintf(
			"<@!%s> booked **%s** between %s and %s.\n\n",
			member.ID,
			response.Spot,
			response.StartAt.Format("2006-01-02 15:04"),
			response.EndAt.Format("2006-01-02 15:04"),
		))
	}
	haveWeOverbooked := err == nil

	if len(response.ConflictingReservations) > 0 {
		message.WriteString("Following reservations are conflicting")
		if err == nil {
			message.WriteString(" **and have been removed**")
		}
		message.WriteString(":\n\n")

		for _, res := range response.ConflictingReservations {
			var author string
			switch haveWeOverbooked { // We notify users on overbooks only
			case true:
				author = fmt.Sprintf("<@!%s>", res.AuthorDiscordID) // Mention user profile by ID
			case false:
				member, err = b.GetMember(guild, res.AuthorDiscordID)
				if err == nil {
					author = member.Nick
					if len(author) == 0 {
						author = member.Username
					}
				} else {
					author = res.Author
				}
				author = fmt.Sprintf("**%s**", author)
			}

			message.WriteString(fmt.Sprintf(
				"* %s %s - %s\n",
				author,
				res.StartAt.Format("2006-01-02 15:04"),
				res.EndAt.Format("2006-01-02 15:04"),
			))
		}
	}

	_, err = dcSession.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: message.String(),
	})
	return err
}

func (b *Bot) BookAutocomplete(i *discordgo.InteractionCreate) error {
	selectedOption, index := util.PoorMansFind(i.ApplicationCommandData().Options,
		func(o *discordgo.ApplicationCommandInteractionDataOption) bool {
			return o.Focused
		})
	if index == -1 {
		return fmt.Errorf("none of the options were selected for autocompletion")
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
		return fmt.Errorf("you must select a reservation to unbook")
	}

	reservationId, err := util.StrToInt64(i.ApplicationCommandData().Options[0].StringValue())
	if err != nil {
		return fmt.Errorf("could not parse reservation id: %v", reservationId)
	}

	gID, err := util.StrToInt64(i.GuildID)
	if err != nil {
		return fmt.Errorf("could not parse guild id: %v", i.GuildID)
	}

	guild, err := b.GetGuild(gID)
	if err != nil {
		return err
	}

	res, err := b.eventHandler.OnUnbook(b, book.UnbookRequest{
		Member:        MapMember(i.Member),
		Guild:         guild,
		ReservationID: reservationId,
	})
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForGuild(gID).FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("%s (%s - %s) reservation has been cancelled.", res.Spot.Name, res.StartAt.Format(util.DC_LONG_TIME_FORMAT), res.EndAt.Format(util.DC_LONG_TIME_FORMAT)),
	})
	return err
}

func (b *Bot) UnbookAutocomplete(i *discordgo.InteractionCreate) error {
	selectedOption, index := util.PoorMansFind(i.ApplicationCommandData().Options,
		func(o *discordgo.ApplicationCommandInteractionDataOption) bool {
			return o.Focused
		})
	if index == -1 {
		return fmt.Errorf("none of the options were selected for autocompletion")
	}
	fieldType := book.UnbookAutocompleteFocus(index)

	gID, err := util.StrToInt64(i.GuildID)
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
		Field:  fieldType,
		Value:  selectedOption.StringValue(),
	}

	response, err := b.eventHandler.OnUnbookAutocomplete(request)
	if err != nil {
		return err
	}

	responseData := &discordgo.InteractionResponseData{
		Choices: MapUnbookAutocompleteChoices(response.Choices),
	}

	return b.interactionRespond(i, responseData, discordgo.InteractionApplicationCommandAutocompleteResult)
}
