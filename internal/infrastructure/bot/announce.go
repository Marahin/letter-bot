package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
)

// BookingMade runs after every new booking but /book, which already replies in
// public. It announces the booking in the command channel of the guild, which
// is the guild's public record of bookings.
func (b *Bot) BookingMade(ctx context.Context, event book.Booked) {
	if err := b.announce(ctx, event); err != nil {
		b.log.With("guild.ID", event.GuildID, "spot.ID", event.SpotID, "source", event.Source).Warnw("could not announce the booking", "error", err)
	}
}

// BookingMadeAsync runs BookingMade in the background, so the caller is not held
// up. Shutdown cancels it and waits for it.
func (b *Bot) BookingMadeAsync(event book.Booked) {
	b.goBackground(func(ctx context.Context) { b.BookingMade(ctx, event) })
}

func (b *Bot) announce(ctx context.Context, event book.Booked) error {
	cfg, err := b.guildConfig(ctx, event.GuildID)
	if err != nil {
		return fmt.Errorf("load guild config: %w", err)
	}
	if !cfg.IsPremium() || !cfg.BotPresent {
		return nil
	}
	channel, err := b.announceChannel(cfg)
	if err != nil {
		return err
	}
	if event.SpotName == "" {
		sp, err := b.spots.SelectGuildSpotByID(ctx, event.GuildID, event.SpotID)
		if err != nil {
			return fmt.Errorf("select spot %d: %w", event.SpotID, err)
		}
		event.SpotName = sp.Name
	}
	event.StartAt, event.EndAt = event.StartAt.Local(), event.EndAt.Local()

	if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
		return err
	}
	_, err = b.writer.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Content: b.formatter.FormatBookingAnnouncement(event),
		// The mentions show who booked, without pinging anybody.
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	})
	if err != nil {
		return fmt.Errorf("send to channel %s: %w", channel.ID, err)
	}
	if b.metrics != nil {
		b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}
	return nil
}

// announceChannel is the configured command channel. When it is not set or not
// found, it is the legacy #letter channel.
func (b *Bot) announceChannel(cfg *guildconfig.Config) (*discord.Channel, error) {
	g := &guild.Guild{ID: cfg.GuildID, Name: cfg.Name}
	var errs []error
	if cfg.CommandChannelID != "" {
		channel, err := b.FindChannelByID(g, cfg.CommandChannelID)
		if err == nil {
			return channel, nil
		}
		errs = append(errs, err)
	}
	channel, err := b.FindChannelByName(g, discord.CommandChannel)
	if err == nil {
		return channel, nil
	}
	return nil, errors.Join(append(errs, err)...)
}
