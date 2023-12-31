package discord

import (
	"time"
)

// ChannelType is the type of a Channel
type ChannelType int

// Block contains known ChannelType values
const (
	ChannelTypeGuildText          ChannelType = 0
	ChannelTypeDM                 ChannelType = 1
	ChannelTypeGuildVoice         ChannelType = 2
	ChannelTypeGroupDM            ChannelType = 3
	ChannelTypeGuildCategory      ChannelType = 4
	ChannelTypeGuildNews          ChannelType = 5
	ChannelTypeGuildStore         ChannelType = 6
	ChannelTypeGuildNewsThread    ChannelType = 10
	ChannelTypeGuildPublicThread  ChannelType = 11
	ChannelTypeGuildPrivateThread ChannelType = 12
	ChannelTypeGuildStageVoice    ChannelType = 13
	ChannelTypeGuildForum         ChannelType = 15
)

type Guild struct {
	ID    string
	Name  string
	Roles []*Role
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Permissions int64  `json:"permissions,string"`
}

type Channel struct {
	ID   string
	Name string
	Type ChannelType
}

// A User stores all data for an individual Discord user.
type User struct {
	// The ID of the user.
	ID string `json:"id"`

	// The user's username.
	Username string `json:"username"`
}

// A Member stores user information for Guild members. A guild
// member represents a certain user's presence in a guild.
type Member struct {
	// ID of the user. MAY NOT BE PRESENT
	ID string

	// User name
	Username string

	// The nickname of the member, if they have one.
	Nick string `json:"nick"`

	// A list of IDs of the roles which are possessed by the member.
	Roles []string `json:"roles"`

	// Total permissions of the member in the channel, including overrides, returned when in the interaction object.
	Permissions int64 `json:"permissions,string"`
}

type Message struct {
	// The ID of the message.
	ID string `json:"id"`

	// The ID of the channel in which the message was sent.
	ChannelID string `json:"channel_id"`

	// The ID of the guild in which the message was sent.
	GuildID string `json:"guild_id,omitempty"`

	// The content of the message.
	Content string `json:"content"`

	// The time at which the messsage was sent.
	// CAUTION: this field may be removed in a
	// future API version; it is safer to calculate
	// the creation time via the ID.
	Timestamp time.Time `json:"timestamp"`

	// The time at which the last edit of the message
	// occurred, if it has been edited.
	EditedTimestamp *time.Time `json:"edited_timestamp"`

	// The author of the message. This is not guaranteed to be a
	// valid user (webhook-sent messages do not possess a full author).
	Author *User `json:"author"`

	// The webhook ID of the message, if it was generated by a webhook
	WebhookID string `json:"webhook_id"`

	// Member properties for this message's author,
	// contains only partial information
	Member *Member `json:"member"`
}
