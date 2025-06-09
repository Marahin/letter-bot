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

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/role"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/common/collections"
	stringsHelper "spot-assistant/internal/common/strings"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/summary"
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

	return b.mgr.SessionForGuild(gID).ChannelMessagesBulkDelete(channel.ID, messageIds)
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

// SendChannelMessage sends a message to a channel in a guild.
func (b *Bot) SendChannelMessage(guild *guild.Guild, channel *discord.Channel, message string) error {
	gID, err := stringsHelper.StrToInt64(guild.ID)
	if err != nil {
		return err
	}

	dcSession := b.mgr.SessionForGuild(gID)
	_, err = dcSession.ChannelMessageSend(channel.ID, message)
	return err
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

	sum, err := b.summarySrv.PrepareSummary(reservationsWithSpots)
	if err != nil {
		return err
	}

	return b.SendLetterMessage(guild, summaryChannel, sum)
}

// SendLetterMessage sends a message to a guild channel,
// or in a DM if guild is nil.
func (b *Bot) SendLetterMessage(guild *guild.Guild, channel *discord.Channel, sum *summary.Summary) error {
	if len(sum.Ledger) == 0 {
		return fmt.Errorf("SendLetterMessage requires at least 1 ledger entry to be present")
	}

	// Do not allow for asynchronous modification
	// of the same channel - this leads to doubled summaries
	mutex, ok := b.channelLocks.Get(channel.ID)
	if !ok {
		mutex = &sync.RWMutex{}
		b.channelLocks.Set(channel.ID, mutex)
	}
	mutex.Lock()
	defer mutex.Unlock()

	// dcSession := b.mgr.SessionForGuild(gId)
	var dcSession *discordgo.Session
	if channel.Type == discord.ChannelTypeDM {
		dcSession = b.mgr.SessionForDM()
	} else {
		// Grab a session for this guild
		gID, err := stringsHelper.StrToInt64(guild.ID)
		if err != nil {
			return fmt.Errorf("could not parse guild ID: %w", err)
		}

		dcSession = b.mgr.SessionForGuild(gID)
	}

	// Transfrom into lines of text describing reservation
	fields := collections.PoorMansMap(sum.Ledger, func(el summary.LedgerEntry) *discordgo.MessageEmbedField {
		writtenReservations := strings.Builder{}

		for _, booking := range el.Bookings {
			writtenReservations.WriteString(
				fmt.Sprintf(
					"%s**%s** - **%s** %s\n",
					booking.Status,
					booking.StartAt.Format("15:04"),
					booking.EndAt.Format("15:04"),
					booking.Author,
				),
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
		err := b.CleanChannel(guild, channel)
		if err != nil {
			return err
		}
	}

	if sum.PreMessage != "" {
		_, err := dcSession.ChannelMessageSend(channel.ID, sum.PreMessage)
		if err != nil {
			return err
		}
	}

	_, err := dcSession.ChannelFileSend(channel.ID, "spots.png", bytes.NewReader(sum.Chart))
	if err != nil {
		return err
	}

	// It seems that discord applies the same validation to 1 embed and to bulk sent embeds,
	// without treating them as separate messages. Because of that, we're gonna need to send embeds 1 by 1.
	// _, err = dcSession.ChannelMessageSendEmbeds(channel.ID, embeds)
	// if err != nil {
	// 	return err
	// }
	for _, embed := range embeds {
		_, err = dcSession.ChannelMessageSendEmbed(channel.ID, embed)
		if err != nil {
			b.log.Errorf("something went wrong when sending embed: %s", err)
		}
	}

	return err
}

func (b *Bot) SendDM(member *member.Member, message string) error {
	channel, err := b.OpenDM(member)
	if err != nil {
		return err
	}

	_, err = b.mgr.SessionForDM().ChannelMessageSend(
		channel.ID,
		message)

	return err
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

	_, err = session.ApplicationCommandBulkOverwrite(session.State.User.ID, guild.ID, commands)
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
