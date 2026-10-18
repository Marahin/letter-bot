package bot

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/dto/summarylayout"
	"spot-assistant/internal/core/regions"
	coresummary "spot-assistant/internal/core/summary"
)

const (
	// channelOpGap is the least time before a channel create or delete. Discord
	// limits channel creation per guild more than it says.
	channelOpGap = 5 * time.Second
	// retiredChannelTTL keeps an unneeded region channel, with a notice, before
	// it is deleted, so a toggle switched back on finds it again.
	retiredChannelTTL = 24 * time.Hour
	// blockedRetry is how long a guild keeps the single summary after Discord
	// refused a channel.
	blockedRetry = time.Hour
	auditReason  = "Letter region summaries"
)

// SummaryPassHook runs at the start of each summary pass of a premium guild the
// bot is in, before any summary is written. Its error is logged and the pass
// goes on.
type SummaryPassHook func(ctx context.Context, guildID string) error

// reconcileSummaries is the summary pass of one guild. Every trigger reads the
// whole guild from the database, but Discord is written only where the
// rendered content changed. A pass makes at most one channel create or delete
// and asks for another pass after channelOpGap, so guilds take turns.
func (b *Bot) reconcileSummaries(ctx context.Context, guildID string) reconcileResult {
	if b.layouts == nil {
		return b.reconcileLegacy(ctx, guildID)
	}
	cfg, err := b.guildConfig(ctx, guildID)
	if err != nil {
		return reconcileResult{Err: err}
	}
	// A guild without the bot or premium keeps whatever is there.
	if !cfg.BotPresent || !cfg.IsPremium() {
		return reconcileResult{}
	}
	b.runPassHooks(ctx, guildID)

	p, err := b.newRegionPass(ctx, cfg)
	if err != nil {
		return reconcileResult{Err: err}
	}
	var res reconcileResult
	if len(p.desired) == 0 {
		res = p.legacy()
	} else {
		res = p.regions()
	}
	b.log.With("guild.ID", guildID, "writes", p.writes, "regions", len(p.desired)).Debug("summary reconciled")
	return res
}

func (b *Bot) runPassHooks(ctx context.Context, guildID string) {
	for _, hook := range b.passHooks {
		if err := hook(ctx, guildID); err != nil {
			b.log.With("guild.ID", guildID).Warnf("summary pass hook failed: %s", err)
		}
	}
}

// regionPass is the state of one summary pass of a guild.
type regionPass struct {
	b       *Bot
	ctx     context.Context
	cfg     *guildconfig.Config
	now     time.Time
	layout  summarylayout.Layout
	rows    map[string]*summarylayout.RegionChannel
	desired []summary.RegionSummary
	next    time.Time
	writes  int
	// channels is the guild's channel list, read on first use.
	channels *channelView
}

func (b *Bot) newRegionPass(ctx context.Context, cfg *guildconfig.Config) (*regionPass, error) {
	guildID := cfg.GuildID
	layout, err := b.layouts.GetLayout(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("get summary layout: %w", err)
	}
	stored, err := b.layouts.ListRegionChannels(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("list region channels: %w", err)
	}
	p := &regionPass{
		b:        b,
		ctx:      ctx,
		cfg:      cfg,
		now:      b.clock(),
		layout:   *layout,
		rows:     make(map[string]*summarylayout.RegionChannel, len(stored)),
		channels: &channelView{b: b, guildID: guildID},
	}
	p.layout.GuildID = guildID
	for i := range stored {
		p.rows[stored[i].Region] = &stored[i]
	}
	if !cfg.RegionSummaries || p.layout.Blocked(p.now) {
		return p, nil
	}

	spots, err := b.spots.SelectGuildSpots(ctx, guildID, false)
	if err != nil {
		return nil, fmt.Errorf("select spots: %w", err)
	}
	reservations, err := b.reservationRepo.SelectUpcomingReservationsWithSpot(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("select upcoming reservations: %w", err)
	}
	if b.metrics != nil {
		b.metrics.SetUpcomingReservations(guildID, cfg.Name, len(reservations))
	}
	p.next, _ = coresummary.NextRefresh(reservations, p.now)
	p.desired = b.summarySrv.PrepareRegionSummaries(spots, reservations)
	return p, nil
}

// legacy posts the single summary and retires the region channels.
func (p *regionPass) legacy() reconcileResult {
	next, err := p.b.PostLegacySummary(p.ctx, p.cfg)
	if err != nil {
		return reconcileResult{Next: next, Err: err}
	}
	// The single summary cleaned the channel, the index with it.
	if p.layout.IndexMessageID != "" {
		p.layout.IndexChannelID, p.layout.IndexMessageID, p.layout.IndexHash = "", "", ""
		if err := p.saveLayout(); err != nil {
			return reconcileResult{Next: next, Err: err}
		}
	}
	again, err := p.retire(nil)
	if p.cfg.RegionSummaries && p.layout.Blocked(p.now) {
		again = minPositive(again, p.layout.BlockedUntil.Sub(p.now))
	}
	return reconcileResult{Next: next, Again: again, Err: err}
}

func (p *regionPass) regions() reconcileResult {
	done, res := p.ensureCategory()
	if !done {
		return res
	}
	done, res = p.ensureChannels()
	if !done {
		return res
	}
	if res, ok := p.writeRegions(); !ok {
		return res
	}
	if err := p.writeIndex(); err != nil {
		return reconcileResult{Next: p.next, Err: err}
	}
	keep := make(map[string]bool, len(p.desired))
	for _, rs := range p.desired {
		keep[rs.Region.Key] = true
	}
	again, err := p.retire(keep)
	return reconcileResult{Next: p.next, Again: again, Err: err}
}

// ensureCategory finds or makes the category. done is false when the pass ends here.
func (p *regionPass) ensureCategory() (done bool, res reconcileResult) {
	if p.layout.CategoryID != "" {
		ok, err := p.channels.has(p.layout.CategoryID)
		if err != nil {
			return false, reconcileResult{Err: err}
		}
		if ok {
			return true, reconcileResult{}
		}
	}
	found, ok, err := p.channels.find(regions.CategoryName, discordgo.ChannelTypeGuildCategory, "")
	if err != nil {
		return false, reconcileResult{Err: err}
	}
	if ok {
		p.layout.CategoryID = found.ID
		if err := p.saveLayout(); err != nil {
			return false, reconcileResult{Err: err}
		}
		return true, reconcileResult{}
	}
	created, err := p.createChannel(discordgo.GuildChannelCreateData{
		Name: regions.CategoryName,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if errors.Is(err, errSummaryBlocked) {
		return false, p.block(err)
	}
	if err != nil {
		return false, reconcileResult{Err: err}
	}
	p.layout.CategoryID = created.ID
	return false, p.again(p.saveLayout())
}

// ensureChannels gives every desired region a channel: its stored one, one of
// its name under the category, or a new one. It makes at most one per pass.
func (p *regionPass) ensureChannels() (done bool, res reconcileResult) {
	for _, rs := range p.desired {
		key := rs.Region.Key
		row := p.rows[key]
		if row != nil && row.Retired() {
			row.RetiredAt, row.ContentHash = nil, ""
			if err := p.saveRow(row); err != nil {
				return false, reconcileResult{Err: err}
			}
		}
		if row != nil {
			ok, err := p.channels.has(row.ChannelID)
			if err != nil {
				return false, reconcileResult{Err: err}
			}
			if ok {
				continue
			}
		}

		name := regions.ChannelName(rs.Region)
		found, ok, err := p.channels.find(name, discordgo.ChannelTypeGuildText, p.layout.CategoryID)
		if err != nil {
			return false, reconcileResult{Err: err}
		}
		if ok {
			if err := p.saveRow(&summarylayout.RegionChannel{GuildID: p.cfg.GuildID, Region: key, ChannelID: found.ID}); err != nil {
				return false, reconcileResult{Err: err}
			}
			continue
		}
		created, err := p.createChannel(discordgo.GuildChannelCreateData{
			Name:     name,
			Type:     discordgo.ChannelTypeGuildText,
			Topic:    fmt.Sprintf("Letter: upcoming hunts in %s. Managed by the bot.", rs.Region.Name),
			ParentID: p.layout.CategoryID,
		})
		if errors.Is(err, errSummaryBlocked) {
			return false, p.block(err)
		}
		if err != nil {
			return false, reconcileResult{Err: err}
		}
		return false, p.again(p.saveRow(&summarylayout.RegionChannel{GuildID: p.cfg.GuildID, Region: key, ChannelID: created.ID}))
	}
	return true, reconcileResult{}
}

// writeRegions edits the messages of each region whose content changed. ok is
// false when the pass ends here.
func (p *regionPass) writeRegions() (reconcileResult, bool) {
	for _, rs := range p.desired {
		row := p.rows[rs.Region.Key]
		msgs := regionMessages(rs, p.b.webBaseURL)
		hash := payloadHash(msgs)
		if hash != "" && row.ContentHash == hash {
			continue
		}
		stampUpdated(msgs, p.now)
		ids, err := p.writeMessages(row.ChannelID, row.MessageIDs, msgs)
		if isDiscordCode(err, discordgo.ErrCodeUnknownChannel) {
			// Deleted by hand: the next pass makes it again.
			return p.again(p.deleteRow(row)), false
		}
		row.MessageIDs = ids
		row.ContentHash = hash
		if err != nil {
			row.ContentHash = ""
		}
		if saveErr := p.saveRow(row); saveErr != nil || err != nil {
			return reconcileResult{Next: p.next, Err: errors.Join(err, saveErr)}, false
		}
	}
	return reconcileResult{}, true
}

// writeMessages edits the stored messages in order, sends the missing ones and
// deletes the surplus. It returns the ids now in the channel, also on an error.
func (p *regionPass) writeMessages(channelID string, stored []string, msgs []*discordgo.MessageSend) ([]string, error) {
	ids := make([]string, 0, len(msgs))
	keep := func() []string { return append(ids, stored[min(len(ids), len(stored)):]...) }
	for i, m := range msgs {
		if i < len(stored) {
			if err := p.wait(messageOpGap); err != nil {
				return keep(), err
			}
			_, err := p.b.writer.ChannelMessageEditComplex(messageEdit(channelID, stored[i], m))
			if err == nil {
				ids = append(ids, stored[i])
				continue
			}
			if !isDiscordCode(err, discordgo.ErrCodeUnknownMessage) {
				return keep(), err
			}
		}
		if err := p.wait(messageOpGap); err != nil {
			return keep(), err
		}
		sent, err := p.b.writer.ChannelMessageSendComplex(channelID, m)
		if err != nil {
			return keep(), err
		}
		ids = append(ids, sent.ID)
	}
	for i := len(msgs); i < len(stored); i++ {
		if err := p.wait(messageOpGap); err != nil {
			return append(ids, stored[i:]...), err
		}
		if err := p.b.writer.ChannelMessageDelete(channelID, stored[i]); err != nil && !isDiscordCode(err, discordgo.ErrCodeUnknownMessage) {
			return append(ids, stored[i:]...), err
		}
	}
	return ids, nil
}

func messageEdit(channelID, messageID string, m *discordgo.MessageSend) *discordgo.MessageEdit {
	content := m.Content
	components := m.Components
	if components == nil {
		// An empty list removes the buttons the message had before.
		components = []discordgo.MessageComponent{}
	}
	return &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Content:    &content,
		Embeds:     m.Embeds,
		Components: components,
	}
}

// writeIndex keeps the links to the region channels in the guild's summary
// channel. A guild without one gets no index.
func (p *regionPass) writeIndex() error {
	g := &guild.Guild{ID: p.cfg.GuildID, Name: p.cfg.Name}
	channel, err := p.b.summaryChannel(g, p.cfg)
	if err != nil {
		p.b.log.With("guild.ID", g.ID).Debugf("no summary channel for the region index: %s", err)
		return nil
	}
	msg := indexMessage(p.desired, p.rows, p.b.webBaseURL)
	hash := payloadHash([]*discordgo.MessageSend{msg})
	sameChannel := p.layout.IndexChannelID == channel.ID && p.layout.IndexMessageID != ""
	if sameChannel && hash != "" && p.layout.IndexHash == hash {
		return nil
	}
	if sameChannel {
		if err := p.wait(messageOpGap); err != nil {
			return err
		}
		_, err := p.b.writer.ChannelMessageEditComplex(messageEdit(channel.ID, p.layout.IndexMessageID, msg))
		if err == nil {
			p.layout.IndexHash = hash
			return p.saveLayout()
		}
		if !isDiscordCode(err, discordgo.ErrCodeUnknownMessage) {
			return err
		}
	}

	unlock := p.b.lockChannel(channel.ID)
	defer unlock()
	// The single summary goes, so a switch back to it posts it again.
	p.b.forgetLegacyChannel(g.ID, channel.ID)
	if err := p.b.CleanChannel(p.ctx, channel); err != nil {
		return err
	}
	if err := p.wait(messageOpGap); err != nil {
		return err
	}
	sent, err := p.b.writer.ChannelMessageSendComplex(channel.ID, msg)
	if err != nil {
		return err
	}
	if p.b.metrics != nil {
		p.b.metrics.IncMessagesSent(channel.ID, channel.Name)
	}
	p.layout.IndexChannelID, p.layout.IndexMessageID, p.layout.IndexHash = channel.ID, sent.ID, hash
	return p.saveLayout()
}

// retire handles the stored channels of regions not in keep: first a notice,
// then, a day later, the delete. The category goes when it is empty and no
// region channel is left. It returns when to look again.
func (p *regionPass) retire(keep map[string]bool) (time.Duration, error) {
	var again time.Duration
	keys := make([]string, 0, len(p.rows))
	for key := range p.rows {
		if !keep[key] {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		row := p.rows[key]
		if !row.Retired() {
			if err := p.retireRow(row); err != nil {
				return again, err
			}
		}
		if row.RetiredAt == nil {
			continue
		}
		if left := retiredChannelTTL - p.now.Sub(*row.RetiredAt); left > 0 {
			again = minPositive(again, left)
			continue
		}
		if err := p.deleteChannel(row.ChannelID); err != nil {
			return again, err
		}
		return channelOpGap, p.deleteRow(row)
	}
	if len(keep) > 0 || len(p.rows) > 0 || p.layout.CategoryID == "" {
		return again, nil
	}
	return again, p.deleteCategory()
}

func (p *regionPass) retireRow(row *summarylayout.RegionChannel) error {
	ids, err := p.writeMessages(row.ChannelID, row.MessageIDs, []*discordgo.MessageSend{retiredMessage()})
	if isDiscordCode(err, discordgo.ErrCodeUnknownChannel) {
		return p.deleteRow(row)
	}
	if err != nil {
		return err
	}
	at := p.now
	row.MessageIDs, row.ContentHash, row.RetiredAt = ids, "", &at
	return p.saveRow(row)
}

// deleteCategory deletes the stored category when no channel is left in it.
// One with the guild's own channels in it is left to the guild.
func (p *regionPass) deleteCategory() error {
	children, found, err := p.channels.children(p.layout.CategoryID)
	if err != nil {
		return err
	}
	if found && children == 0 {
		if err := p.deleteChannel(p.layout.CategoryID); err != nil {
			return err
		}
	}
	p.layout.CategoryID = ""
	return p.saveLayout()
}

var errSummaryBlocked = errors.New("discord refused the summary channel")

// createChannel retries without the permission overwrites when Discord refuses
// them. A refusal for permissions or the channel limit is errSummaryBlocked.
func (p *regionPass) createChannel(data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	data.PermissionOverwrites = p.b.summaryOverwrites(p.cfg.GuildID)
	for {
		if err := p.wait(channelOpGap); err != nil {
			return nil, err
		}
		created, err := p.b.writer.GuildChannelCreateComplex(p.cfg.GuildID, data, discordgo.WithAuditLogReason(auditReason))
		if err == nil {
			return created, nil
		}
		if isDiscordCode(err, discordgo.ErrCodeMissingPermissions) && len(data.PermissionOverwrites) > 0 {
			data.PermissionOverwrites = nil
			continue
		}
		if isDiscordCode(err, discordgo.ErrCodeMissingPermissions) || isDiscordCode(err, discordgo.ErrCodeMissingAccess) ||
			isDiscordCode(err, discordgo.ErrCodeMaximumNumberOfGuildChannelsReached) {
			return nil, fmt.Errorf("%w: %w", errSummaryBlocked, err)
		}
		return nil, err
	}
}

func (p *regionPass) deleteChannel(channelID string) error {
	if err := p.wait(channelOpGap); err != nil {
		return err
	}
	_, err := p.b.writer.ChannelDelete(channelID, discordgo.WithAuditLogReason(auditReason))
	if err != nil && !isDiscordCode(err, discordgo.ErrCodeUnknownChannel) {
		return err
	}
	return nil
}

// block keeps the guild on the single summary for a while.
func (p *regionPass) block(cause error) reconcileResult {
	p.b.log.With("guild.ID", p.cfg.GuildID).Warnf("region summaries blocked for %s: %s", blockedRetry, cause)
	until := p.now.Add(blockedRetry)
	p.layout.BlockedUntil = &until
	if err := p.saveLayout(); err != nil {
		return reconcileResult{Err: err}
	}
	return p.legacy()
}

func (p *regionPass) wait(gap time.Duration) error {
	if err := p.b.pacer.Wait(p.ctx, gap); err != nil {
		return err
	}
	p.writes++
	return nil
}

func (p *regionPass) saveLayout() error {
	return p.b.layouts.SaveLayout(p.ctx, p.layout)
}

func (p *regionPass) saveRow(row *summarylayout.RegionChannel) error {
	p.rows[row.Region] = row
	return p.b.layouts.SaveRegionChannel(p.ctx, *row)
}

func (p *regionPass) deleteRow(row *summarylayout.RegionChannel) error {
	delete(p.rows, row.Region)
	return p.b.layouts.DeleteRegionChannel(p.ctx, row.GuildID, row.Region)
}

// again asks for the next pass after a channel was made.
func (p *regionPass) again(err error) reconcileResult {
	return reconcileResult{Next: p.next, Again: channelOpGap, Err: err}
}

func minPositive(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	return min(a, b)
}

// summaryOverwrites keep the summary channels for the bot: members read them
// and use the buttons. Without the bot's user id the channels get no overwrites.
func (b *Bot) summaryOverwrites(guildID string) []*discordgo.PermissionOverwrite {
	userID := b.botUserID()
	if userID == "" {
		return nil
	}
	return []*discordgo.PermissionOverwrite{
		{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionSendMessages | discordgo.PermissionAddReactions},
		{ID: userID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks | discordgo.PermissionReadMessageHistory},
	}
}

func (b *Bot) botUserID() string {
	if b.mgr == nil || b.mgr.Gateway == nil || b.mgr.Gateway.State == nil {
		return ""
	}
	state := b.mgr.Gateway.State
	state.RLock()
	defer state.RUnlock()
	if state.User == nil {
		return ""
	}
	return state.User.ID
}

// channelView reads the guild's channels from the gateway cache. Before it
// reports a channel missing it asks the API, because the cache can lag behind a
// channel the bot just made, and a wrong "missing" makes a duplicate.
type channelView struct {
	b       *Bot
	guildID string
	list    []*discordgo.Channel
	cached  bool
	fresh   bool
}

func (v *channelView) lookup(match func(*discordgo.Channel) bool) (*discordgo.Channel, bool, error) {
	if !v.cached {
		v.cached = true
		if channels, err := v.b.gateway.StateChannels(v.guildID); err == nil {
			v.list = channels
		}
	}
	if i := slices.IndexFunc(v.list, match); i >= 0 {
		return v.list[i], true, nil
	}
	if err := v.refresh(); err != nil {
		return nil, false, err
	}
	if i := slices.IndexFunc(v.list, match); i >= 0 {
		return v.list[i], true, nil
	}
	return nil, false, nil
}

// refresh reads the channels from the API, once per pass.
func (v *channelView) refresh() error {
	if v.fresh {
		return nil
	}
	channels, err := v.b.writer.GuildChannels(v.guildID)
	if err != nil {
		return fmt.Errorf("list guild channels: %w", err)
	}
	v.list, v.cached, v.fresh = channels, true, true
	return nil
}

func (v *channelView) has(id string) (bool, error) {
	_, found, err := v.lookup(func(c *discordgo.Channel) bool { return c.ID == id })
	return found, err
}

// find returns the channel of the name and type, in parentID when it is set.
func (v *channelView) find(name string, kind discordgo.ChannelType, parentID string) (*discordgo.Channel, bool, error) {
	return v.lookup(func(c *discordgo.Channel) bool {
		return c.Name == name && c.Type == kind && (parentID == "" || c.ParentID == parentID)
	})
}

// children counts the channels in the category from the API: a child the
// cache lacks would make the bot delete a category in use.
func (v *channelView) children(parentID string) (n int, found bool, err error) {
	if err := v.refresh(); err != nil {
		return 0, false, err
	}
	for _, c := range v.list {
		found = found || c.ID == parentID
		if c.ParentID == parentID {
			n++
		}
	}
	return n, found, nil
}
