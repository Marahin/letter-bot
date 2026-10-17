package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/dto/summary"
	coresummary "spot-assistant/internal/core/summary"
)

// StartTicking starts the 2-minute tick. Every shard's Ready calls it, also on a
// reconnect, so only the first call starts it.
func (b *Bot) StartTicking() {
	b.ticking.Do(func() {
		b.log.Info("ticker started")
		go b.ticker()
	})
}

func (b *Bot) ticker() {
	ticker := time.NewTicker(2 * time.Minute)

	for {
		select {
		case <-b.quit:
			b.log.Warn("shutting down bot ticker")
			ticker.Stop()
			return
		case <-ticker.C:
			b.Tick()
		}
	}
}

func (b *Bot) SendDMOverbookedNotification(m *member.Member, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) error {
	return b.SendDM(m, b.formatter.FormatOverbookedMemberNotification(m, request, res))
}

// CleanChannel deletes the last 100 messages of the channel.
func (b *Bot) CleanChannel(ctx context.Context, channel *discord.Channel) error {
	messages, err := b.writer.ChannelMessages(channel.ID, 100, "", "", "")
	if err != nil {
		return err
	}
	ids := collections.PoorMansMap(messages, func(msg *discordgo.Message) string { return msg.ID })

	young, old := splitByAge(ids, time.Now())
	deleted := 0
	defer func() {
		if deleted > 0 {
			b.metrics.AddMessagesDeleted(channel.ID, channel.Name, deleted)
		}
	}()
	if len(young) > 0 {
		if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
			return err
		}
		if err := b.writer.ChannelMessagesBulkDelete(channel.ID, young); err != nil {
			return err
		}
		deleted += len(young)
	}
	for _, id := range old {
		if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
			return err
		}
		if err := b.writer.ChannelMessageDelete(channel.ID, id); err != nil && !isDiscordCode(err, discordgo.ErrCodeUnknownMessage) {
			return err
		}
		deleted++
	}
	return nil
}

// bulkDeleteMaxAge stays under the 14 days after which Discord refuses a bulk delete.
const bulkDeleteMaxAge = 13 * 24 * time.Hour

// splitByAge splits message ids into those a bulk delete accepts and those it
// refuses (code 50034). A summary in a quiet guild can be that old.
func splitByAge(ids []string, now time.Time) (young, old []string) {
	for _, id := range ids {
		created, err := discordgo.SnowflakeTimestamp(id)
		if err == nil && now.Sub(created) < bulkDeleteMaxAge {
			young = append(young, id)
			continue
		}
		old = append(old, id)
	}
	return young, old
}

func isDiscordCode(err error, code int) bool {
	var restErr *discordgo.RESTError
	return errors.As(err, &restErr) && restErr.Message != nil && restErr.Message.Code == code
}

// EnsureChannel creates the legacy channels that the guild settings leave empty and that do not exist.
func (b *Bot) EnsureChannel(g *guild.Guild, cfg *guildconfig.Config) error {
	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return err
	}

	for _, name := range legacyChannelsToCreate(cfg, channels) {
		if _, err := b.mgr.Gateway.GuildChannelCreate(g.ID, name, discordgo.ChannelTypeGuildText); err != nil {
			return err
		}
	}

	return nil
}

var errChannelNotFound = errors.New("channel not found")

func (b *Bot) FindChannelByID(g *guild.Guild, channelID string) (*discord.Channel, error) {
	channel, err := b.findChannel(g.ID, func(c *discordgo.Channel) bool { return c.ID == channelID })
	if errors.Is(err, errChannelNotFound) {
		return nil, fmt.Errorf("channel with id '%s' not found in guild '%s'", channelID, g.Name)
	}
	if err != nil {
		return nil, err
	}
	return MapChannel(channel), nil
}

func (b *Bot) FindChannelByName(g *guild.Guild, channelName string) (*discord.Channel, error) {
	channel, err := b.findChannel(g.ID, func(c *discordgo.Channel) bool { return c.Name == channelName })
	if errors.Is(err, errChannelNotFound) {
		return nil, fmt.Errorf("channel '%s' not found in guild '%s'", channelName, g.Name)
	}
	if err != nil {
		return nil, err
	}
	return MapChannel(channel), nil
}

// findChannel looks in the gateway cache first, because Discord rate-limits
// REST reads too. It asks REST only when the cache does not have the channel.
func (b *Bot) findChannel(guildID string, match func(*discordgo.Channel) bool) (*discordgo.Channel, error) {
	if cached, err := b.gateway.StateChannels(guildID); err == nil {
		if c, i := collections.PoorMansFind(cached, match); i != -1 {
			return c, nil
		}
	}
	channels, err := b.writer.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("error when fetching guild channels: %w", err)
	}
	if c, i := collections.PoorMansFind(channels, match); i != -1 {
		return c, nil
	}
	return nil, errChannelNotFound
}

func (b *Bot) EnsureRoles(g *guild.Guild) error {
	gld, err := b.mgr.Gateway.Guild(g.ID)
	if err != nil {
		return fmt.Errorf("error when fetching guild: %w", err)
	}

	roles, err := b.GetRoles(g)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r.Name == discord.PrivilegedRole {
			return nil
		}
	}

	_, err = b.mgr.Gateway.GuildRoleCreate(gld.ID, &discordgo.RoleParams{Name: discord.PrivilegedRole})
	if err != nil {
		return fmt.Errorf("error when creating a postman role: %w", err)
	}

	return nil
}

func (b *Bot) GetGuilds() []*guild.Guild {
	b.mgr.RLock()
	defer b.mgr.RUnlock()

	guilds := make([]*discordgo.Guild, 0)
	for _, shard := range b.mgr.Shards {
		for _, poorGuild := range shard.Session.State.Guilds {
			g, err := shard.Session.Guild(poorGuild.ID)
			if err != nil {
				b.log.With("guild.ID", poorGuild.ID).Errorf("could not download guild data: %s", err)

				continue
			}

			guilds = append(guilds, g)
		}
	}

	return MapGuilds(guilds)
}

// stateGuildIDs returns the ids of the guilds in the gateway cache of every shard.
func (b *Bot) stateGuildIDs() []string {
	b.mgr.RLock()
	defer b.mgr.RUnlock()

	var ids []string
	for _, shard := range b.mgr.Shards {
		if shard.Session == nil || shard.Session.State == nil {
			continue
		}
		shard.Session.State.RLock()
		for _, g := range shard.Session.State.Guilds {
			ids = append(ids, g.ID)
		}
		shard.Session.State.RUnlock()
	}
	return ids
}

func (b *Bot) GetGuild(id int64) (*guild.Guild, error) {
	g, err := b.mgr.SessionForGuild(id).Guild(strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}

	return MapGuild(g), nil
}

// reconcileLegacy is the summary pass of a guild in the single-summary layout.
func (b *Bot) reconcileLegacy(ctx context.Context, guildID string) reconcileResult {
	cfg, err := b.guildConfig(ctx, guildID)
	if err != nil {
		return reconcileResult{Err: err}
	}
	if !cfg.BotPresent {
		return reconcileResult{}
	}
	next, err := b.PostLegacySummary(ctx, cfg)
	return reconcileResult{Next: next, Err: err}
}

// PostLegacySummary posts the summary of a premium guild in its summary channel,
// unless the bot already posted the same content there. It returns the next
// start or end of a reservation in the summary, or zero when there is none.
func (b *Bot) PostLegacySummary(ctx context.Context, cfg *guildconfig.Config) (time.Time, error) {
	if !cfg.IsPremium() {
		return time.Time{}, nil
	}
	g := &guild.Guild{ID: cfg.GuildID, Name: cfg.Name}
	channel, err := b.summaryChannel(g, cfg)
	if err != nil {
		return time.Time{}, err
	}

	reservations, err := b.reservationRepo.SelectUpcomingReservationsWithSpot(ctx, g.ID)
	if err != nil {
		return time.Time{}, err
	}
	if b.metrics != nil {
		b.metrics.SetUpcomingReservations(g.ID, g.Name, len(reservations))
	}
	next, _ := coresummary.NextRefresh(reservations, time.Now())

	key := g.ID + "/" + channel.ID
	hash := b.summarySrv.Ledger(reservations).Hash()
	if posted, ok := b.legacyHashes.Get(key); ok && posted == hash {
		return next, nil
	}

	if len(reservations) == 0 {
		err = b.sendEmptyLetter(ctx, channel)
	} else {
		var sum *summary.Summary
		sum, err = b.summarySrv.PrepareSummary(reservations)
		if err == nil {
			hash = sum.Ledger.Hash()
			err = b.sendLetter(ctx, channel, sum)
		}
	}
	if err != nil {
		return next, err
	}
	b.legacyHashes.Set(key, hash)
	return next, nil
}

// forgetLegacyChannel drops what the bot knows of a summary channel and reports
// whether the bot had posted there.
func (b *Bot) forgetLegacyChannel(guildID, channelID string) bool {
	_, ok := b.legacyHashes.Pop(guildID + "/" + channelID)
	return ok
}

// SendLetterMessage sends a message to a guild channel,
// or in a DM if guild is nil.
func (b *Bot) SendLetterMessage(g *guild.Guild, channel *discord.Channel, sum *summary.Summary) error {
	return b.sendLetter(context.Background(), channel, sum)
}

// letterSender is the part of a session that sends a summary.
type letterSender interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelFileSend(channelID, name string, r io.Reader, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// sendLetter paces the writes to a guild channel. A DM is sent at once.
func (b *Bot) sendLetter(ctx context.Context, channel *discord.Channel, sum *summary.Summary) error {
	if len(sum.Ledger) == 0 {
		return errors.New("SendLetterMessage requires at least 1 ledger entry to be present")
	}

	unlock := b.lockChannel(channel.ID)
	defer unlock()

	var dcSession letterSender = b.writer
	pace := func() error { return b.pacer.Wait(ctx, messageOpGap) }
	if channel.Type == discord.ChannelTypeDM {
		dcSession = b.mgr.SessionForDM()
		pace = func() error { return nil }
	}

	// Transform into lines of text describing reservation
	fields := collections.PoorMansMap(sum.Ledger, func(el summary.LedgerEntry) *discordgo.MessageEmbedField {
		writtenReservations := strings.Builder{}

		for _, booking := range el.Bookings {
			statusStr := MapOnlineStatus(booking.Status)
			fmt.Fprintf(&writtenReservations,
				"%s**%s** - **%s** %s\n",
				statusStr,
				booking.StartAt.Format("15:04"),
				booking.EndAt.Format("15:04"),
				booking.Author,
			)
		}

		value := writtenReservations.String()
		name := fmt.Sprintf("**`%s`**", el.Spot)

		return &discordgo.MessageEmbedField{
			Name:   name,
			Value:  value,
			Inline: true,
		}
	})
	footer := MapFooter(sum.Footer)

	// Discord seems to have a limit of embeds per message
	// this means we should limit ourselves to send maximum 20 embeds
	// per message; and continue sending messages until we're done
	batchLimit := int(math.Min(13.0, float64(len(fields))))
	batches := collections.PoorMansPartition(fields, batchLimit)
	embeds := collections.PoorMansMap(batches, func(batch []*discordgo.MessageEmbedField) *discordgo.MessageEmbed {
		return b.newEmbed(sum.Title, sum.URL, sum.Description, batch, footer)
	})

	if channel.Type != discord.ChannelTypeDM {
		if err := b.CleanChannel(ctx, channel); err != nil {
			return err
		}
	}

	if sum.PreMessage != "" {
		if err := pace(); err != nil {
			return err
		}
		if _, err := dcSession.ChannelMessageSend(channel.ID, sum.PreMessage); err != nil {
			return err
		}
		b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	if err := pace(); err != nil {
		return err
	}
	_, err := dcSession.ChannelFileSend(channel.ID, "spots.png", bytes.NewReader(sum.Chart))
	if err != nil {
		return err
	}
	defer b.metrics.IncMessagesSent(channel.ID, channel.Name)

	// It seems that discord applies the same validation to 1 embed and to bulk sent embeds,
	// without treating them as separate messages. Because of that, we're gonna need to send embeds 1 by 1.
	// _, err = dcSession.ChannelMessageSendEmbeds(channel.ID, embeds)
	// if err != nil {
	// 	return err
	// }
	// The other embeds are still sent. The error makes the summary worker post
	// the whole summary again later.
	var embedErrs []error
	for idx, embed := range embeds {
		if err := pace(); err != nil {
			return err
		}
		if channel.Type != discord.ChannelTypeDM && idx == len(embeds)-1 {
			_, err = dcSession.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: summaryComponents(b.webBaseURL),
			})
		} else {
			_, err = dcSession.ChannelMessageSendEmbed(channel.ID, embed)
		}
		if err != nil {
			embedErrs = append(embedErrs, err)
			continue
		}

		b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	return errors.Join(embedErrs...)
}

// lockChannel serializes the summaries of one channel; two at once double the summary.
func (b *Bot) lockChannel(channelID string) func() {
	mutex := b.channelLocks.Upsert(channelID, nil, func(exists bool, current, _ *sync.RWMutex) *sync.RWMutex {
		if exists {
			return current
		}
		return &sync.RWMutex{}
	})
	mutex.Lock()
	return mutex.Unlock
}

// sendEmptyLetter replaces the summary of a guild with no upcoming hunts, so the
// channel keeps the buttons.
func (b *Bot) sendEmptyLetter(ctx context.Context, channel *discord.Channel) error {
	unlock := b.lockChannel(channel.ID)
	defer unlock()

	if err := b.CleanChannel(ctx, channel); err != nil {
		return err
	}
	if err := b.pacer.Wait(ctx, messageOpGap); err != nil {
		return err
	}
	_, err := b.writer.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{b.emptyEmbed()},
		Components: summaryComponents(b.webBaseURL),
	})
	if err != nil {
		return err
	}
	b.metrics.IncMessagesSent(channel.ID, channel.Name)
	return nil
}

func (b *Bot) SendDM(m *member.Member, message string) error {
	channel, err := b.OpenDM(m)
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForDM().ChannelMessageSend(
		channel.ID,
		message)

	if err != nil {
		return err
	}
	defer b.metrics.IncMessagesSent(channel.ID, m.Username)

	return nil
}

func (b *Bot) GetMemberByGuildAndID(g *guild.Guild, memberID string) (*member.Member, error) {
	gID, err := stringsHelper.StrToInt64(g.ID)
	if err != nil {
		return nil, err
	}

	m, err := b.mgr.SessionForGuild(gID).GuildMember(g.ID, memberID)
	if err != nil {
		return nil, err
	}

	return MapMember(m), nil
}

func (b *Bot) RegisterCommands(g *guild.Guild) error {
	gID, err := stringsHelper.StrToInt64(g.ID)
	if err != nil {
		return err
	}

	session := b.mgr.SessionForGuild(gID)

	// In the past, we've been using global commands for all guilds.
	// This is a legacy code that should be removed in the future.
	globalCmds, err := session.ApplicationCommands(session.State.User.ID, "")
	if err != nil {
		return err
	}
	for _, cmd := range globalCmds {
		b.log.With("guild_name", g.Name, "cmd.ID", cmd.ID, "cmd.Name", cmd.Name).Warn("removing command")
		err = session.ApplicationCommandDelete(session.State.User.ID, "", cmd.ID)
		if err != nil {
			b.log.Error("could not delete command: %s", err)
		}
	}

	// This is the way to register and unregister commands now.
	guildCmds, err := session.ApplicationCommands(session.State.User.ID, g.ID)
	if err != nil {
		return err
	}
	for _, cmd := range guildCmds {
		b.log.With("guild_name", g.Name, "cmd.ID", cmd.ID, "cmd.Name", cmd.Name).Warn("removing command")
		err = session.ApplicationCommandDelete(session.State.User.ID, g.ID, cmd.ID)
		if err != nil {
			b.log.Error("could not delete command: %s", err)
		}
	}

	_, err = session.ApplicationCommandBulkOverwrite(session.State.User.ID, g.ID, b.getCommands())
	return err
}

func (b *Bot) GetRoles(g *guild.Guild) ([]*role.Role, error) {
	roles, err := b.mgr.Gateway.GuildRoles(g.ID)
	if err != nil {
		return []*role.Role{}, fmt.Errorf("error when fetching guild roles: %w", err)
	}

	return MapRoles(roles), nil
}

func (b *Bot) OpenDM(m *member.Member) (*discord.Channel, error) {
	sess := b.mgr.SessionForDM()
	channel, err := sess.UserChannelCreate(m.ID)
	if err != nil {
		return nil, err
	}

	return MapChannel(channel), nil
}
