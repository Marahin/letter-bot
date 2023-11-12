package bot

import (
	"strconv"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/reservation"
)

func TestMapChannel(t *testing.T) {
	// given
	assert := assert.New(t)
	channel := &discordgo.Channel{
		ID:   "channel-id",
		Name: "channel-name",
	}

	// when
	res := MapChannel(channel)

	// assert
	assert.NotNil(res)
	assert.Equal(channel.Name, res.Name)
	assert.Equal(channel.ID, res.ID)
}

func TestMapRoles(t *testing.T) {
	// given
	assert := assert.New(t)
	roles := []*discordgo.Role{
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
	}

	// when
	res := MapRoles(roles)

	// assert
	assert.Len(res, len(roles))
	for index, resRole := range res {
		assert.Equal(roles[index].ID, resRole.ID)
		assert.Equal(roles[index].Name, resRole.Name)
		assert.Equal(roles[index].Permissions, resRole.Permissions)
	}
}

func TestMapGuild(t *testing.T) {
	// given
	assert := assert.New(t)
	guild := &discordgo.Guild{
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
	}

	// when
	res := MapGuild(guild)

	// res
	assert.NotNil(res)
	assert.Equal(guild.ID, res.ID)
	assert.Equal(guild.Name, res.Name)
	for index, gRole := range guild.Roles {
		expectedRole := guild.Roles[index]

		assert.Equal(expectedRole.ID, gRole.ID)
		assert.Equal(expectedRole.Name, gRole.Name)
		assert.Equal(expectedRole.Permissions, gRole.Permissions)
	}
}

func TestMapGuilds(t *testing.T) {
	// given
	assert := assert.New(t)
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
	assert.Len(resGuilds, 2)
	for index, res := range resGuilds {
		guild := guilds[index]

		assert.Equal(guild.ID, res.ID)
		assert.Equal(guild.Name, res.Name)
		for index, gRole := range guild.Roles {
			expectedRole := guild.Roles[index]

			assert.Equal(expectedRole.ID, gRole.ID)
			assert.Equal(expectedRole.Name, gRole.Name)
			assert.Equal(expectedRole.Permissions, gRole.Permissions)
		}
	}
}

func TestMapUser(t *testing.T) {
	// given
	assert := assert.New(t)
	user := &discordgo.User{
		ID:       "test-user-id",
		Username: "test-user-username",
	}

	// when
	res := MapUser(user)

	// assert
	assert.NotNil(res)
	assert.Equal(user.ID, res.ID)
	assert.Equal(user.Username, res.Username)
}

func TestMapUserIfNil(t *testing.T) {
	// given
	assert := assert.New(t)

	// when
	res := MapUser(nil)

	// assert
	assert.Nil(res)
}

func TestMapMember(t *testing.T) {
	// given
	assert := assert.New(t)
	member := &discordgo.Member{
		Nick:  "test-member-nick",
		Roles: []string{"test-member-role1", "test-member-role2"},
		User: &discordgo.User{
			ID:       "test-member-user-id",
			Username: "test-member-user-username",
		},
	}

	// when
	res := MapMember(member)

	// assert
	assert.NotNil(res)
	assert.Equal(member.User.ID, res.ID)
	assert.Equal(member.Nick, res.Nick)
	assert.Equal(member.User.Username, res.Username)
	assert.Equal(member.Roles, res.Roles)
}

func TestMapMemberIfNil(t *testing.T) {
	// given
	assert := assert.New(t)

	// when
	res := MapMember(nil)

	// assert
	assert.Nil(res)
}

func TestMapMessage(t *testing.T) {
	// given
	assert := assert.New(t)
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
	assert.NotNil(res)
	assert.Equal(msg.ID, res.ID)
	assert.Equal(msg.ChannelID, res.ChannelID)
	assert.Equal(msg.Content, res.Content)
	assert.Equal(msg.Timestamp, res.Timestamp)
	assert.Equal(msg.EditedTimestamp, res.EditedTimestamp)
	assert.NotNil(res.Member)
}

func TestMapFooter(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "test footer"

	// when
	res := MapFooter(input)

	// assert
	assert.NotNil(res)
	assert.Equal(input, res.Text)
}

func TestMapStringToChoice(t *testing.T) {
	// given
	assert := assert.New(t)
	input := "test-choice"

	// when
	res := MapStringToChoice(input)

	// assert
	assert.NotNil(res)
	assert.Equal(input, res.Name)
	assert.Equal(input, res.Value)
}

func TestMapStringArrToChoice(t *testing.T) {
	// given
	assert := assert.New(t)
	input := []string{"test-choice-1", "test-choice-2"}

	// when
	res := MapStringArrToChoice(input)

	// assert
	assert.Len(res, len(input))
	for index, choice := range res {
		assert.Equal(input[index], choice.Name)
		assert.Equal(input[index], choice.Value)
	}
}

func TestMapReservationWithSpotArrToChoice(t *testing.T) {
	// given
	assert := assert.New(t)
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
	assert.Len(res, len(input))
	result := res[0]
	assert.Equal("2023-08-10 16:00 - 2023-08-10 18:00 test-spot", result.Name)
	assert.Equal(strconv.FormatInt(input[0].Reservation.ID, 10), result.Value)
}
