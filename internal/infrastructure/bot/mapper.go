package bot

import (
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/role"
	"strconv"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"

	"github.com/bwmarrin/discordgo"
)

const (
	EmojiOnline  = ":green_circle: "
	EmojiOffline = ":red_circle: "
)

func MapChannel(input *discordgo.Channel) *discord.Channel {
	return &discord.Channel{
		ID:       input.ID,
		Name:     input.Name,
		Type:     discord.ChannelType(input.Type),
		ParentID: input.ParentID,
		Position: input.Position,
	}
}

func mapRole(input *discordgo.Role) *role.Role {
	return &role.Role{
		ID:          input.ID,
		Name:        input.Name,
		Permissions: input.Permissions,
		Position:    input.Position,
		Color:       input.Color,
	}
}

func MapRoles(input []*discordgo.Role) []*role.Role {
	roles := make([]*role.Role, len(input))

	for i, r := range input {
		roles[i] = mapRole(r)
	}

	return roles
}

func MapGuild(input *discordgo.Guild) *guild.Guild {
	return &guild.Guild{
		Roles:   MapRoles(input.Roles),
		ID:      input.ID,
		Name:    input.Name,
		Icon:    input.Icon,
		OwnerID: input.OwnerID,
	}
}

func MapGuilds(input []*discordgo.Guild) []*guild.Guild {
	guilds := make([]*guild.Guild, len(input))
	for i, g := range input {
		guilds[i] = MapGuild(g)
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
		ID:          input.User.ID,
		Nick:        input.Nick,
		Username:    input.User.Username,
		Roles:       input.Roles,
		Permissions: input.Permissions,
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
	return collections.PoorMansMap(input, MapMessage)
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
	return collections.PoorMansMap(texts, MapStringToChoice)
}

func MapReservationWithSpotArrToChoice(input []*reservation.ReservationWithSpot) []*discordgo.ApplicationCommandOptionChoice {
	return collections.PoorMansMap(input, func(i *reservation.ReservationWithSpot) *discordgo.ApplicationCommandOptionChoice {
		return &discordgo.ApplicationCommandOptionChoice{
			Name:  i.Label(),
			Value: strconv.FormatInt(i.Reservation.ID, 10),
		}
	})
}

func MapOnlineStatus(status summary.OnlineStatus) string {
	switch status {
	case summary.Online:
		return EmojiOnline
	case summary.Offline:
		return EmojiOffline
	default:
		return ""
	}
}
