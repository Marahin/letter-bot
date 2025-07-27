package bot

import "github.com/bwmarrin/discordgo"

func (b *Bot) baseEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         "https://tibialoot.com",
		Type:        discordgo.EmbedTypeRich,
		Title:       "TibiaLoot.com - Spot Assistant",
		Description: "Current and upcoming hunts. Times are in **Europe/Berlin**. \n :green_circle: **Online** \n :red_circle: **Offline**",
	}
}

func (b *Bot) newEmbed(
	title string,
	url string,
	description string,
	fields []*discordgo.MessageEmbedField,
	footer *discordgo.MessageEmbedFooter,
) *discordgo.MessageEmbed {
	embed := b.baseEmbed()

	embed.Fields = fields
	embed.Footer = footer

	return embed
}
