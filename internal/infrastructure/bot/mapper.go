package bot

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/util"
)

func MapChannel(input *discordgo.Channel) *discord.Channel {
	return &discord.Channel{
		ID:   input.ID,
		Name: input.Name,
	}
}

func mapRole(input *discordgo.Role) *discord.Role {
	return &discord.Role{
		ID:          input.ID,
		Name:        input.Name,
		Permissions: input.Permissions,
	}
}

func MapRoles(input []*discordgo.Role) []*discord.Role {
	roles := make([]*discord.Role, len(input))

	for i, role := range input {
		roles[i] = mapRole(role)
	}

	return roles
}

func MapGuild(input *discordgo.Guild) *discord.Guild {
	return &discord.Guild{
		Roles: MapRoles(input.Roles),
		ID:    input.ID,
		Name:  input.Name,
	}
}

func MapGuilds(input []*discordgo.Guild) []*discord.Guild {
	guilds := make([]*discord.Guild, len(input))
	for i, guild := range input {
		guilds[i] = MapGuild(guild)
	}

	return guilds
}

func MapUser(input *discordgo.User) *discord.User {
	if input == nil {
		logrus.Error("MapUser got nil on input: ", input)
		return nil
	}

	return &discord.User{
		ID:       input.ID,
		Username: input.Username,
	}
}

func MapMember(input *discordgo.Member) *discord.Member {
	if input == nil {
		return nil
	}

	return &discord.Member{
		ID:       input.User.ID,
		Nick:     input.Nick,
		Username: input.User.Username,
		Roles:    input.Roles,
	}
}

func MapMessage(input *discordgo.Message) *discord.Message {
	return &discord.Message{
		ID:              input.ID,
		ChannelID:       input.ChannelID,
		Content:         input.Content,
		Timestamp:       input.Timestamp,
		EditedTimestamp: input.EditedTimestamp,
		Member:          MapMember(input.Member),
	}
}

func MapMessages(input []*discordgo.Message) []*discord.Message {
	return util.PoorMansMap(input, func(el *discordgo.Message) *discord.Message {
		return MapMessage(el)
	})
}

func MapFooter(text string) *discordgo.MessageEmbedFooter {
	return &discordgo.MessageEmbedFooter{
		Text: text,
	}
}

func MapStringToChoice(text string) *discordgo.ApplicationCommandOptionChoice {
	return &discordgo.ApplicationCommandOptionChoice{
		Name:  text,
		Value: text,
	}
}

func MapStringArrToChoice(texts []string) []*discordgo.ApplicationCommandOptionChoice {
	return util.PoorMansMap(texts, func(t string) *discordgo.ApplicationCommandOptionChoice {
		return MapStringToChoice(t)
	})
}

func MapUnbookAutocompleteChoices(input []*reservation.ReservationWithSpot) []*discordgo.ApplicationCommandOptionChoice {
	return util.PoorMansMap(input, func(i *reservation.ReservationWithSpot) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s - %s %s", i.StartAt.Format(util.DC_LONG_TIME_FORMAT), i.EndAt.Format(util.DC_LONG_TIME_FORMAT), i.Spot.Name),
			Value: strconv.FormatInt(i.Reservation.ID, 10),
		}
	})
}
