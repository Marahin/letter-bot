package summarylayout

import "time"

// Layout is what the bot made for the region summaries of a guild. The zero
// value means nothing yet.
type Layout struct {
	GuildID    string
	CategoryID string
	// IndexChannelID and IndexMessageID are the index of the region channels,
	// posted in the guild's summary channel.
	IndexChannelID string
	IndexMessageID string
	IndexHash      string
	// BlockedUntil is set when Discord refused a channel. Until then the guild
	// keeps the single summary.
	BlockedUntil *time.Time
}

// Blocked tells whether the region summaries wait at now.
func (l Layout) Blocked(now time.Time) bool {
	return l.BlockedUntil != nil && now.Before(*l.BlockedUntil)
}

// RegionChannel is a region channel the bot made or adopted, with the messages
// it posted there. ContentHash is "" when the messages must be written again.
type RegionChannel struct {
	GuildID     string
	Region      string
	ChannelID   string
	MessageIDs  []string
	ContentHash string
	// RetiredAt is set when the region was no longer needed; the channel is
	// deleted a day later.
	RetiredAt *time.Time
}

// Retired tells whether the channel waits for its deletion.
func (c RegionChannel) Retired() bool {
	return c.RetiredAt != nil
}
