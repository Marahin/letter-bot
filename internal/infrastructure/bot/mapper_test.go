package bot

import (
	"strconv"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func TestMapChannel(t *testing.T) {
	// given
	is := assert.New(t)
	channel := &discordgo.Channel{
		ID:       "channel-id",
		Name:     "channel-name",
		ParentID: "category-id",
		Position: 3,
	}

	// when
	res := MapChannel(channel)

	// assert
	is.NotNil(res)
	is.Equal(channel.Name, res.Name)
	is.Equal(channel.ID, res.ID)
	is.Equal(channel.ParentID, res.ParentID)
	is.Equal(channel.Position, res.Position)
}

func TestMapRoles(t *testing.T) {
	// given
	is := assert.New(t)
	roles := []*discordgo.Role{
		{
			ID:          "test-role-id",
			Name:        "test-role-name",
			Permissions: 12345,
			Position:    2,
			Color:       0xff0000,
		},
		{
			ID:          "test-role-id-2",
			Name:        "test-role-name-2",
			Permissions: 654321,
		},
	}

	// when
	res := MapRoles(roles)

	// assert
	is.Len(res, len(roles))
	for index, resRole := range res {
		is.Equal(roles[index].ID, resRole.ID)
		is.Equal(roles[index].Name, resRole.Name)
		is.Equal(roles[index].Permissions, resRole.Permissions)
		is.Equal(roles[index].Position, resRole.Position)
		is.Equal(roles[index].Color, resRole.Color)
	}
}

func TestMapGuild(t *testing.T) {
	// given
	is := assert.New(t)
	guild := &discordgo.Guild{
		ID:      "test-guild-id",
		Name:    "test-guild-name",
		Icon:    "test-guild-icon",
		OwnerID: "test-owner-id",
		Roles: []*discordgo.Role{
			{
				ID:          "test-role-id",
				Name:        "test-role-name",
				Permissions: 12345,
			},
			{
				ID:          "test-role-id-2",
				Name:        "test-role-name-2",
				Permissions: 654321,
			},
		},
	}

	// when
	res := MapGuild(guild)

	// res
	is.NotNil(res)
	is.Equal(guild.ID, res.ID)
	is.Equal(guild.Name, res.Name)
	is.Equal(guild.Icon, res.Icon)
	is.Equal(guild.OwnerID, res.OwnerID)
	for index, gRole := range guild.Roles {
		expectedRole := guild.Roles[index]

		is.Equal(expectedRole.ID, gRole.ID)
		is.Equal(expectedRole.Name, gRole.Name)
		is.Equal(expectedRole.Permissions, gRole.Permissions)
	}
}

func TestMapGuilds(t *testing.T) {
	// given
	is := assert.New(t)
	guilds := []*discordgo.Guild{
		{
			ID:   "test-guild-id",
			Name: "test-guild-name",
			Roles: []*discordgo.Role{
				{
					ID:          "test-role-id",
					Name:        "test-role-name",
					Permissions: 12345,
				},
				{
					ID:          "test-role-id-2",
					Name:        "test-role-name-2",
					Permissions: 654321,
				},
			},
		},
		{

			ID:   "test-guild-id-2",
			Name: "test-guild-name-2",
			Roles: []*discordgo.Role{
				{
					ID:          "test-role-id-2",
					Name:        "test-role-name-2",
					Permissions: 32323,
				},
				{
					ID:          "test-role-id-2-2",
					Name:        "test-role-name-2",
					Permissions: 6556564321,
				},
			},
		},
	}

	// when
	resGuilds := MapGuilds(guilds)

	// res
	is.Len(resGuilds, 2)
	for index, res := range resGuilds {
		guild := guilds[index]

		is.Equal(guild.ID, res.ID)
		is.Equal(guild.Name, res.Name)
		for index, gRole := range guild.Roles {
			expectedRole := guild.Roles[index]

			is.Equal(expectedRole.ID, gRole.ID)
			is.Equal(expectedRole.Name, gRole.Name)
			is.Equal(expectedRole.Permissions, gRole.Permissions)
		}
	}
}

func TestMapUser(t *testing.T) {
	// given
	is := assert.New(t)
	user := &discordgo.User{
		ID:       "test-user-id",
		Username: "test-user-username",
	}

	// when
	res := MapUser(user)

	// assert
	is.NotNil(res)
	is.Equal(user.ID, res.ID)
	is.Equal(user.Username, res.Username)
}

func TestMapUserIfNil(t *testing.T) {
	// given
	is := assert.New(t)

	// when
	res := MapUser(nil)

	// assert
	is.Nil(res)
}

func TestMapMember(t *testing.T) {
	// given
	is := assert.New(t)
	member := &discordgo.Member{
		Nick:        "test-member-nick",
		Roles:       []string{"test-member-role1", "test-member-role2"},
		Permissions: discordgo.PermissionAdministrator,
		User: &discordgo.User{
			ID:       "test-member-user-id",
			Username: "test-member-user-username",
		},
	}

	// when
	res := MapMember(member)

	// assert
	is.NotNil(res)
	is.Equal(member.User.ID, res.ID)
	is.Equal(member.Nick, res.Nick)
	is.Equal(member.User.Username, res.Username)
	is.Equal(member.Roles, res.Roles)
	is.Equal(member.Permissions, res.Permissions)
}

func TestMapMemberIfNil(t *testing.T) {
	// given
	is := assert.New(t)

	// when
	res := MapMember(nil)

	// assert
	is.Nil(res)
}

func TestMapMessage(t *testing.T) {
	// given
	is := assert.New(t)
	msg := &discordgo.Message{
		ID:              "test-message-id",
		ChannelID:       "test-message-channel-id",
		Content:         "test-message-content",
		Timestamp:       time.Now(),
		EditedTimestamp: nil,
		Member: &discordgo.Member{
			Nick:  "test-member-nick",
			Roles: []string{"test-member-role1", "test-member-role2"},
			User: &discordgo.User{
				ID:       "test-member-user-id",
				Username: "test-member-user-username",
			},
		},
	}

	// when
	res := MapMessage(msg)

	// assert
	is.NotNil(res)
	is.Equal(msg.ID, res.ID)
	is.Equal(msg.ChannelID, res.ChannelID)
	is.Equal(msg.Content, res.Content)
	is.Equal(msg.Timestamp, res.Timestamp)
	is.Equal(msg.EditedTimestamp, res.EditedTimestamp)
	is.NotNil(res.Member)
}

func TestMapFooter(t *testing.T) {
	// given
	is := assert.New(t)
	input := "test footer"

	// when
	res := MapFooter(input)

	// assert
	is.NotNil(res)
	is.Equal(input, res.Text)
}

func TestMapStringToChoice(t *testing.T) {
	// given
	is := assert.New(t)
	input := "test-choice"

	// when
	res := MapStringToChoice(input)

	// assert
	is.NotNil(res)
	is.Equal(input, res.Name)
	is.Equal(input, res.Value)
}

func TestMapStringArrToChoice(t *testing.T) {
	// given
	is := assert.New(t)
	input := []string{"test-choice-1", "test-choice-2"}

	// when
	res := MapStringArrToChoice(input)

	// assert
	is.Len(res, len(input))
	for index, choice := range res {
		is.Equal(input[index], choice.Name)
		is.Equal(input[index], choice.Value)
	}
}

func TestMapReservationWithSpotArrToChoice(t *testing.T) {
	// given
	is := assert.New(t)
	startAt := time.Date(2023, 8, 10, 16, 0, 0, 0, time.Now().Location())
	endAt := time.Date(2023, 8, 10, 18, 0, 0, 0, time.Now().Location())
	input := []*reservation.ReservationWithSpot{
		{
			Reservation: reservation.Reservation{
				ID:      1,
				StartAt: startAt,
				EndAt:   endAt,
			},
			Spot: reservation.Spot{
				Name: "test-spot",
			},
		},
	}

	// when
	res := MapReservationWithSpotArrToChoice(input)

	// assert
	is.Len(res, len(input))
	result := res[0]
	is.Equal("2023-08-10 16:00 - 2023-08-10 18:00 test-spot", result.Name)
	is.Equal(strconv.FormatInt(input[0].Reservation.ID, 10), result.Value)
}

func TestMapOnlineStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   summary.OnlineStatus
		expected string
	}{
		{"Online", summary.Online, EmojiOnline},
		{"Offline", summary.Offline, EmojiOffline},
		{"Unknown", summary.Unknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapOnlineStatus(tt.status)
			is := assert.New(t)
			is.Equal(tt.expected, got)
		})
	}
}
