package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// InteractionCreate is the entry point of slash commands, buttons, selects and forms.
func (b *Bot) InteractionCreate(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	tStart := time.Now()

	switch i.Type {
	case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
		b.handleCommand(i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(i)
	default:
		b.log.With("type", i.Type).Debug("interaction type not handled")
		return
	}

	b.log.With("duration", time.Since(tStart), "type", i.Type).Debug("interaction handled")
}
