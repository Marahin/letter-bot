package channelshttp

import (
	"slices"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/infrastructure/web"
)

// channelOptions is a leading default entry (empty value), then the synced
// channels as "#name".
func channelOptions(channels []*discord.Channel, defaultLabel string) []web.ComboboxOption {
	opts := make([]web.ComboboxOption, 0, len(channels)+1)
	opts = append(opts, web.ComboboxOption{Value: "", Label: defaultLabel})
	for _, c := range channels {
		opts = append(opts, web.ComboboxOption{Value: c.ID, Label: "#" + c.Name})
	}
	return opts
}

// missing reports a stored channel that is no longer synced, e.g. deleted in Discord.
func missing(channels []*discord.Channel, id string) bool {
	return id != "" && !slices.ContainsFunc(channels, func(c *discord.Channel) bool { return c.ID == id })
}
