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

// Status is how the region summaries of a guild stand, as the web shows them.
type Status string

const (
	StatusOff Status = "off"
	// StatusUnused: the setting is on, but no active respawn has a region, so
	// the guild keeps the single summary.
	StatusUnused Status = "unused"
	// StatusBlocked: Discord refused a channel, so the guild keeps the single
	// summary until BlockedUntil.
	StatusBlocked Status = "blocked"
	// StatusInUse: the setting is on, the guild is not blocked and at least one
	// active respawn has a region.
	StatusInUse Status = "in_use"
)

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
