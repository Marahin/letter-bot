package bot

import (
	"bytes"
	"context"
	"fmt"
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
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/summarytracker"
)

// Starts internal ticker, that will trigger bot's emission
// of Tick event. Essentially we're just spawning a background
// worker.
func (b *Bot) StartTicking() {
	b.log.Info("ticker started")

	go b.ticker()
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

func (b *Bot) SendDMOverbookedNotification(member *member.Member, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) error {
	return b.SendDM(member, b.formatter.FormatOverbookedMemberNotification(member, request, res))
}

func (b *Bot) ChannelMessages(g *guild.Guild, ch *discord.Channel, limit int) ([]*discord.Message, error) {
	gID, err := stringsHelper.StrToInt64(g.ID)
	if err != nil {
		return []*discord.Message{}, err
	}

	msgs, err := b.mgr.SessionForGuild(gID).ChannelMessages(ch.ID, limit, "", "", "")
	if err != nil {
		return []*discord.Message{}, err
	}

	return MapMessages(msgs), nil
}

func (b *Bot) CleanChannel(g *guild.Guild, channel *discord.Channel) error {
	gID, err := stringsHelper.StrToInt64(g.ID)
	if err != nil {
		return err
	}

	messages, err := b.ChannelMessages(g, channel, 100)
	if err != nil {
		return err
	}

	messageIds := collections.PoorMansMap(messages, func(msg *discord.Message) string {
		return msg.ID
	})

	err = b.mgr.SessionForGuild(gID).ChannelMessagesBulkDelete(channel.ID, messageIds)
	if err != nil {
		return err
	}

	defer b.metrics.AddMessagesDeleted(channel.ID, channel.Name, len(messages))

	return nil
}

func (b *Bot) EnsureChannel(guild *guild.Guild) error {
	letterSummaryChannelFound := false
	letterChannelFound := false

	g, err := b.mgr.Gateway.Guild(guild.ID)
	if err != nil {

		return err
	}

	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return err
	}

	for _, ch := range channels {
		if ch.Name == discord.SummaryChannel {
			letterSummaryChannelFound = true
		}

		if ch.Name == discord.CommandChannel {
			letterChannelFound = true
		}
	}

	if !letterSummaryChannelFound {
		_, err := b.mgr.Gateway.GuildChannelCreate(g.ID, discord.SummaryChannel, discordgo.ChannelTypeGuildText)
		if err != nil {
			return err
		}
	}

	if !letterChannelFound {
		_, err := b.mgr.Gateway.GuildChannelCreate(g.ID, discord.CommandChannel, discordgo.ChannelTypeGuildText)
		if err != nil {

			return err
		}
	}

	return nil
}

func (b *Bot) FindChannelById(g *guild.Guild, channelId string) (*discord.Channel, error) {
	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return nil, fmt.Errorf("error when fetching guild channels: %s", err)
	}

	channel, _ := collections.PoorMansFind(channels, func(channel *discordgo.Channel) bool {
		return channel.ID == channelId
	})

	if channel != nil {
		return MapChannel(channel), nil
	}

	return nil, fmt.Errorf("channel with id '%s' not found in guild '%s'", channelId, g.Name)
}

func (b *Bot) FindChannelByName(g *guild.Guild, channelName string) (*discord.Channel, error) {
	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return nil, fmt.Errorf("error when fetching guild channels: %s", err)
	}

	for _, channel := range channels {
		if channel.Name == channelName {
			return MapChannel(channel), nil
		}
	}

	return nil, fmt.Errorf("channel '%s' not found in guild '%s'", channelName, g.Name)
}

func (b *Bot) EnsureRoles(g *guild.Guild) error {
	guild, err := b.mgr.Gateway.Guild(g.ID)
	if err != nil {
		return fmt.Errorf("error when fetching guild: %s", err)
	}

	roles, err := b.GetRoles(g)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role.Name == discord.PrivilegedRole {
			return nil
		}
	}

	_, err = b.mgr.Gateway.GuildRoleCreate(guild.ID, &discordgo.RoleParams{Name: discord.PrivilegedRole})
	if err != nil {
		return fmt.Errorf("error when creating a postman role: %s", err)
	}

	return nil
}

func (b *Bot) GetGuilds() []*guild.Guild {
	b.mgr.RLock()
	defer b.mgr.RUnlock()

	guilds := make([]*discordgo.Guild, 0)
	for _, shard := range b.mgr.Shards {
		for _, poorGuild := range shard.Session.State.Guilds {
			guild, err := shard.Session.Guild(poorGuild.ID)
			if err != nil {
				b.log.With("guild.ID", guild.ID).Errorf("could not download guild data: %s", err)

				continue
			}

			guilds = append(guilds, shard.Session.State.Guilds...)
		}
	}

	return MapGuilds(guilds)
}

func (b *Bot) GetGuild(id int64) (*guild.Guild, error) {
	guild, err := b.mgr.SessionForGuild(id).Guild(strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}

	return MapGuild(guild), nil
}

func (b *Bot) TryUpdateGuildLetter(guild *guild.Guild) {
	err := b.UpdateGuildLetter(guild)
	if err != nil {
		b.log.Errorf("could not update guild letter: %s", err)
	}
}

func (b *Bot) UpdateGuildLetter(guild *guild.Guild) error {
	summaryChannel, err := b.FindChannelByName(guild, discord.SummaryChannel)
	if err != nil {
		return err
	}

	reservationsWithSpots, err := b.reservationRepo.SelectUpcomingReservationsWithSpot(context.Background(), guild.ID)
	if err != nil {
		return err
	}

	// metrics: update gauge of upcoming reservations
	if b.metrics != nil {
		b.metrics.SetUpcomingReservations(guild.ID, guild.Name, len(reservationsWithSpots))
	}

	sum, err := b.summarySrv.PrepareSummary(reservationsWithSpots)
	if err != nil {
		return err
	}

	return b.SendLetterMessageGuildChannel(guild, summaryChannel, sum)
}

func (b *Bot) SendDM(member *member.Member, message string) error {
	channel, err := b.OpenDM(member)
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForDM().ChannelMessageSend(
		channel.ID,
		message)

	if err != nil {
		return err
	}
	defer b.metrics.IncMessagesSent(channel.ID, member.Username)

	return nil
}

func (b *Bot) GetMemberByGuildAndId(guild *guild.Guild, memberID string) (*member.Member, error) {
	gID, err := stringsHelper.StrToInt64(guild.ID)
	if err != nil {
		return nil, err
	}

	member, err := b.mgr.SessionForGuild(gID).GuildMember(guild.ID, memberID)
	if err != nil {
		return nil, err
	}

	return MapMember(member), nil
}

func (b *Bot) RegisterCommands(guild *guild.Guild) error {
	gID, err := stringsHelper.StrToInt64(guild.ID)
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
		b.log.With("guild_name", guild.Name, "cmd.ID", cmd.ID, "cmd.Name", cmd.Name).Warn("removing command")
		err = session.ApplicationCommandDelete(session.State.User.ID, "", cmd.ID)
		if err != nil {
			b.log.Error("could not delete command: %s", err)
		}
	}

	// This is the way to register and unregister commands now.
	guildCmds, err := session.ApplicationCommands(session.State.User.ID, guild.ID)
	if err != nil {
		return err
	}
	for _, cmd := range guildCmds {
		b.log.With("guild_name", guild.Name, "cmd.ID", cmd.ID, "cmd.Name", cmd.Name).Warn("removing command")
		err = session.ApplicationCommandDelete(session.State.User.ID, guild.ID, cmd.ID)
		if err != nil {
			b.log.Error("could not delete command: %s", err)
		}
	}

	_, err = session.ApplicationCommandBulkOverwrite(session.State.User.ID, guild.ID, b.getCommands())
	return err
}

func (b *Bot) GetRoles(g *guild.Guild) ([]*role.Role, error) {
	roles, err := b.mgr.Gateway.GuildRoles(g.ID)
	if err != nil {
		return []*role.Role{}, fmt.Errorf("error when fetching guild roles: %s", err)
	}

	return MapRoles(roles), nil
}

func (b *Bot) MemberHasRole(g *guild.Guild, m *member.Member, targetRoleName string) bool {
	roles, err := b.GetRoles(g)
	if err != nil {
		b.log.Errorf("error occured when getting roles: %s", err)

		return false
	}

	targetRole, _ := collections.PoorMansFind(roles, func(r *role.Role) bool {
		return r.Name == targetRoleName
	})

	if targetRole == nil {
		return false
	}

	for _, memberRole := range m.Roles {
		if memberRole == targetRole.ID {
			return true
		}
	}

	return false
}

func (b *Bot) OpenDM(m *member.Member) (*discord.Channel, error) {
	sess := b.mgr.SessionForDM()
	channel, err := sess.UserChannelCreate(m.ID)
	if err != nil {
		return nil, err
	}

	return MapChannel(channel), nil
}

func (b *Bot) createEmbedsFromSummary(sum *summary.Summary) []*discordgo.MessageEmbed {
	fields := collections.PoorMansMap(sum.Ledger, func(el summary.LedgerEntry) *discordgo.MessageEmbedField {
		writtenReservations := strings.Builder{}
		for _, booking := range el.Bookings {
			statusStr := MapOnlineStatus(booking.Status)
			fmt.Fprintf(&writtenReservations, "%s**%s** - **%s** %s\n",
				statusStr,
				booking.StartAt.Format("15:04"),
				booking.EndAt.Format("15:04"),
				booking.Author)
		}
		return &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("**`%s`**", el.Spot),
			Value:  writtenReservations.String(),
			Inline: true,
		}
	})

	batchLimit := int(math.Min(13.0, float64(len(fields))))
	batches := collections.PoorMansPartition(fields, batchLimit)
	footer := MapFooter(sum.Footer)

	return collections.PoorMansMap(batches, func(batch []*discordgo.MessageEmbedField) *discordgo.MessageEmbed {
		return b.newEmbed(sum.Title, sum.URL, sum.Description, batch, footer)
	})
}

func (b *Bot) SendLetterMessageDM(channel *discord.Channel, sum *summary.Summary) error {
	if len(sum.Ledger) == 0 {
		return fmt.Errorf("SendLetterMessageDM requires at least 1 ledger entry to be present")
	}

	b.log.Debugf("Sending DM summary to channel %s (%d ledger entries)", channel.ID, len(sum.Ledger))

	mutex, ok := b.channelLocks.Get(channel.ID)
	if !ok {
		mutex = &sync.RWMutex{}
		b.channelLocks.Set(channel.ID, mutex)
	}
	mutex.Lock()
	defer mutex.Unlock()

	dcSession := b.mgr.SessionForDM()
	embeds := b.createEmbedsFromSummary(sum)

	if sum.PreMessage != "" {
		if _, err := dcSession.ChannelMessageSend(channel.ID, sum.PreMessage); err != nil {
			return err
		}
		defer b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	if _, err := dcSession.ChannelFileSend(channel.ID, "spots.png", bytes.NewReader(sum.Chart)); err != nil {
		return err
	}
	defer b.metrics.IncMessagesSent(channel.ID, channel.Name)

	for _, embed := range embeds {
		if _, err := dcSession.ChannelMessageSendEmbed(channel.ID, embed); err != nil {
			b.log.Errorf("failed to send embed: %s", err)
		}
		defer b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	return nil
}

func (b *Bot) SendLetterMessageGuildChannel(guild *guild.Guild, channel *discord.Channel, sum *summary.Summary) error {
	if len(sum.Ledger) == 0 {
		return fmt.Errorf("SendLetterMessageGuildChannel requires at least 1 ledger entry to be present")
	}

	b.log.Debugf("Sending guild summary to channel %s in guild %s (%d ledger entries)", channel.ID, guild.ID, len(sum.Ledger))

	gID, err := stringsHelper.StrToInt64(guild.ID)
	if err != nil {
		return fmt.Errorf("could not parse guild ID: %w", err)
	}

	dcSession := b.mgr.SessionForGuild(gID)
	embeds := b.createEmbedsFromSummary(sum)
	ctx := context.Background()

	trackedMsgs, err := b.summaryTrackerSrv.GetTrackedMessages(ctx, guild.ID, channel.ID)
	if err != nil {
		b.log.Warnf("Failed to get tracked messages for channel %s, falling back to fresh send: %v", channel.ID, err)
		return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
	}

	var existingPreMsg *summarytracker.TrackedMessage
	existingEmbeds := make([]*summarytracker.TrackedMessage, 0)
	for _, msg := range trackedMsgs {
		switch msg.MessageType {
		case "pre_message":
			existingPreMsg = msg
		case "embed":
			existingEmbeds = append(existingEmbeds, msg)
		}
	}

	if sum.PreMessage != "" {
		if existingPreMsg != nil {
			if _, err := dcSession.ChannelMessageEdit(channel.ID, existingPreMsg.MessageID, sum.PreMessage); err != nil {
				b.log.Warnf("Failed to edit pre-message %s, falling back to fresh send: %v", existingPreMsg.MessageID, err)
				return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
			}
			b.summaryTrackerSrv.UpdateTimestamp(ctx, existingPreMsg.ID)
		} else {
			b.log.Warnf("Pre-message required but not tracked, falling back to fresh send")
			return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
		}
	}

	return b.processEmbedUpdates(ctx, guild, channel, sum, dcSession, embeds, existingEmbeds)
}

func (b *Bot) processEmbedUpdates(ctx context.Context, guild *guild.Guild, channel *discord.Channel, sum *summary.Summary, dcSession *discordgo.Session, embeds []*discordgo.MessageEmbed, existingEmbeds []*summarytracker.TrackedMessage) error {
	commonLen := min(len(existingEmbeds), len(embeds))

	b.log.Debugf("Editing %d embeds, adding %d new, deleting %d old",
		commonLen,
		max(0, len(embeds)-commonLen),
		max(0, len(existingEmbeds)-commonLen))

	// 1. Update existing messages (where both slices have an element)
	for i := range commonLen {
		edit := discordgo.NewMessageEdit(channel.ID, existingEmbeds[i].MessageID)
		edit.SetEmbeds([]*discordgo.MessageEmbed{embeds[i]})

		if _, err := dcSession.ChannelMessageEditComplex(edit); err != nil {
			b.log.Warnf("Failed to edit embed %d (msg %s), falling back to fresh send: %v", i, existingEmbeds[i].MessageID, err)
			return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
		} else {
			b.summaryTrackerSrv.UpdateTimestamp(ctx, existingEmbeds[i].ID)
		}
	}

	// 2. Add new embeds (if embeds > existingEmbeds)
	for i := commonLen; i < len(embeds); i++ {
		msg, err := dcSession.ChannelMessageSendEmbed(channel.ID, embeds[i])
		if err != nil {
			b.log.Warnf("Failed to send new embed %d, falling back to fresh send: %v", i, err)
			return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
		}
		b.summaryTrackerSrv.TrackMessage(ctx, guild.ID, channel.ID, msg.ID, "embed", i)
		defer b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	// 3. Delete extra embeds (if existingEmbeds > embeds)
	for i := commonLen; i < len(existingEmbeds); i++ {
		if err := dcSession.ChannelMessageDelete(channel.ID, existingEmbeds[i].MessageID); err != nil {
			b.log.Warnf("Failed to delete old embed %d (msg %s), falling back to fresh send: %v", i, existingEmbeds[i].MessageID, err)
			return b.sendFreshMessages(guild, channel, sum, dcSession, embeds)
		}
		b.summaryTrackerSrv.DeleteMessage(ctx, existingEmbeds[i].ID)
	}

	b.log.Infof("Successfully updated summary in channel %s (edited %d, added %d, deleted %d embeds)",
		channel.ID, commonLen, max(0, len(embeds)-commonLen), max(0, len(existingEmbeds)-commonLen))
	return nil
}

func (b *Bot) sendFreshMessages(guild *guild.Guild, channel *discord.Channel, sum *summary.Summary, dcSession *discordgo.Session, embeds []*discordgo.MessageEmbed) error {
	ctx := context.Background()

	b.log.Infof("Sending fresh messages to channel %s (clean slate)", channel.ID)

	if err := b.CleanChannel(guild, channel); err != nil {
		return err
	}

	b.summaryTrackerSrv.DeleteAllForChannel(ctx, guild.ID, channel.ID)

	if sum.PreMessage != "" {
		msg, err := dcSession.ChannelMessageSend(channel.ID, sum.PreMessage)
		if err != nil {
			b.log.Errorf("failed to send pre-message: %s", err)
			return err
		}
		b.summaryTrackerSrv.TrackMessage(ctx, guild.ID, channel.ID, msg.ID, "pre_message", 0)
		b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	for order, embed := range embeds {
		msg, err := dcSession.ChannelMessageSendEmbed(channel.ID, embed)
		if err != nil {
			b.log.Errorf("failed to send embed: %s", err)
			return err
		}
		b.summaryTrackerSrv.TrackMessage(ctx, guild.ID, channel.ID, msg.ID, "embed", order)
		b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}

	return nil
}
