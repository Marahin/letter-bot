package bot

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"

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
			b.log.Warning("shutting down bot ticker")
			ticker.Stop()
			return
		case <-ticker.C:
			b.Tick()
		}
	}
}

func (b *Bot) ChannelMessages(g *discord.Guild, ch *discord.Channel, limit int) ([]*discord.Message, error) {
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

func (b *Bot) CleanChannel(g *discord.Guild, channel *discord.Channel) error {
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

func (b *Bot) EnsureChannel(guild *discord.Guild) error {
	log := logrus.WithFields(logrus.Fields{"type": "infra"})
	letterSummaryChannelFound := false
	letterChannelFound := false
	defer log.WithFields(logrus.Fields{"summaryFound": letterSummaryChannelFound, "letterFound": letterChannelFound})

	g, err := b.mgr.Gateway.Guild(guild.ID)
	if err != nil {

		return err
	}

	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return err
	}

	for _, ch := range channels {
		if ch.Name == "letter-summary" {
			letterSummaryChannelFound = true
		}

		if ch.Name == "letter" {
			letterChannelFound = true
		}
	}

	if !letterSummaryChannelFound {
		_, err := b.mgr.Gateway.GuildChannelCreate(g.ID, "letter-summary", discordgo.ChannelTypeGuildText)
		if err != nil {
			return err
		}
	}

	if !letterChannelFound {
		_, err := b.mgr.Gateway.GuildChannelCreate(g.ID, "letter", discordgo.ChannelTypeGuildText)
		if err != nil {

			return err
		}
	}

	return nil
}

func (b *Bot) FindChannel(g *discord.Guild, channelName string) (*discord.Channel, error) {
	channels, err := b.mgr.Gateway.GuildChannels(g.ID)
	if err != nil {
		return nil, err
	}

	for _, channel := range channels {
		if channel.Name == channelName {
			return MapChannel(channel), nil
		}
	}

	return nil, errors.New("channel not found")
}

func (b *Bot) EnsureRoles(g *discord.Guild) error {
	guild, err := b.mgr.Gateway.Guild(g.ID)
	if err != nil {
		return fmt.Errorf("error when fetching guild: %s", err)
	}

	roles, err := b.GetRoles(g)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role.Name == ROLE {
			return nil
		}
	}

	_, err = b.mgr.Gateway.GuildRoleCreate(guild.ID, &discordgo.RoleParams{Name: "Postman"})
	if err != nil {
		return fmt.Errorf("error when creating a postman role: %s", err)
	}

	return nil
}

func (b *Bot) GetGuilds() []*discord.Guild {
	b.mgr.RLock()
	defer b.mgr.RUnlock()

	guilds := make([]*discordgo.Guild, 0)
	for _, shard := range b.mgr.Shards {
		for _, poorGuild := range shard.Session.State.Guilds {
			guild, err := shard.Session.Guild(poorGuild.ID)
			if err != nil {
				b.log.WithFields(logrus.Fields{"guild.ID": guild.ID}).Errorf("could not download guild data: %s", err)

				continue
			}

			guilds = append(guilds, shard.Session.State.Guilds...)
		}
	}

	return MapGuilds(guilds)
}

func (b *Bot) GetGuild(id int64) (*discord.Guild, error) {
	guild, err := b.mgr.SessionForGuild(id).Guild(strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}

	return MapGuild(guild), nil
}

func (b *Bot) SendLetterMessage(guild *discord.Guild, channel *discord.Channel, sum *summary.Summary) error {
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
					"**%s** - **%s** %s\n",
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

func (b *Bot) GetMember(guild *discord.Guild, memberID string) (*discord.Member, error) {
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

func (b *Bot) RegisterCommands(guild *discord.Guild) error {
	gID, err := stringsHelper.StrToInt64(guild.ID)
	if err != nil {
		return err
	}

	session := b.mgr.SessionForGuild(gID)
	_, err = session.ApplicationCommandBulkOverwrite(session.State.User.ID, guild.ID, commands)
	return err
}

func (b *Bot) GetRoles(g *discord.Guild) ([]*discord.Role, error) {
	roles, err := b.mgr.Gateway.GuildRoles(g.ID)
	if err != nil {
		return []*discord.Role{}, fmt.Errorf("error when fetching guild roles: %s", err)
	}

	return MapRoles(roles), nil
}

func (b *Bot) MemberHasRole(g *discord.Guild, m *discord.Member, targetRoleName string) bool {
	roles, err := b.GetRoles(g)
	if err != nil {
		b.log.Errorf("error occured when getting roles: %s", err)

		return false
	}

	targetRole, _ := collections.PoorMansFind(roles, func(r *discord.Role) bool {
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

func (b *Bot) OpenDM(m *discord.Member) (*discord.Channel, error) {
	sess := b.mgr.SessionForDM()
	channel, err := sess.UserChannelCreate(m.ID)
	if err != nil {
		return nil, err
	}

	return MapChannel(channel), nil
}
