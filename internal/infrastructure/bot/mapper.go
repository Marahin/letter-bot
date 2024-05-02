package bot

import (
	"fmt"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/role"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

func MapChannel(input *discordgo.Channel) *discord.Channel {
	return &discord.Channel{
		ID:   input.ID,
		Name: input.Name,
		Type: discord.ChannelType(input.Type),
	}
}

func mapRole(input *discordgo.Role) *role.Role {
	return &role.Role{
		ID:          input.ID,
		Name:        input.Name,
		Permissions: input.Permissions,
	}
}

func MapRoles(input []*discordgo.Role) []*role.Role {
	roles := make([]*role.Role, len(input))

	for i, role := range input {
		roles[i] = mapRole(role)
	}

	return roles
}

func MapGuild(input *discordgo.Guild) *guild.Guild {
	return &guild.Guild{
		Roles: MapRoles(input.Roles),
		ID:    input.ID,
		Name:  input.Name,
	}
}

func MapGuilds(input []*discordgo.Guild) []*guild.Guild {
	guilds := make([]*guild.Guild, len(input))
	for i, guild := range input {
		guilds[i] = MapGuild(guild)
	}

	return guilds
}

func MapUser(input *discordgo.User) *discord.User {
	if input == nil {
		return nil
	}

	return &discord.User{
		ID:       input.ID,
		Username: input.Username,
	}
}

func MapMember(input *discordgo.Member) *member.Member {
	if input == nil {
		return nil
	}

	return &member.Member{
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
	return collections.PoorMansMap(input, func(el *discordgo.Message) *discord.Message {
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
	return collections.PoorMansMap(texts, func(t string) *discordgo.ApplicationCommandOptionChoice {
		return MapStringToChoice(t)
	})
}

func MapReservationWithSpotArrToChoice(input []*reservation.ReservationWithSpot) []*discordgo.ApplicationCommandOptionChoice {
	return collections.PoorMansMap(input, func(i *reservation.ReservationWithSpot) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s - %s %s", i.StartAt.Format(strings.DcLongTimeFormat), i.EndAt.Format(strings.DcLongTimeFormat), i.Spot.Name),
			Value: strconv.FormatInt(i.Reservation.ID, 10),
		}
	})
}
