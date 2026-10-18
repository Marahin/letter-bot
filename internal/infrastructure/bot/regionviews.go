package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"

	"spot-assistant/internal/core/dto/region"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/dto/summarylayout"
)

const (
	regionEmbedColor = 0xF97316
	// Discord allows 10 embeds and 6000 characters per message; the margin
	// covers the timestamp and what Discord counts differently.
	maxEmbedsPerMessage = 10
	maxMessageChars     = 5800
	maxEmbedTitle       = 256
	maxRegionBookings   = 20
	maxRespawnListChars = 1000
	regionFooter        = "powered by TibiaLoot.com"
	regionLead          = "Current and upcoming hunts. Times are in **Europe/Berlin**."
	onlineLegend        = EmojiOnline + "**Online**  " + EmojiOffline + "**Offline**"
)

// regionMessages renders the summary of one region channel: a header, then one
// embed per booked respawn, packed into as few messages as fit. The last
// message carries the buttons.
func regionMessages(rs summary.RegionSummary, webBaseURL string) []*discordgo.MessageSend {
	embeds := make([]*discordgo.MessageEmbed, 0, 1+len(rs.Ledger))
	embeds = append(embeds, regionHeader(rs))
	for _, entry := range rs.Ledger {
		embeds = append(embeds, spotEmbed(entry))
	}
	msgs := packEmbeds(embeds)
	msgs[len(msgs)-1].Components = regionComponents(rs.Region, webBaseURL)
	return msgs
}

func regionHeader(rs summary.RegionSummary) *discordgo.MessageEmbed {
	var desc strings.Builder
	desc.WriteString(regionLead)
	if len(rs.Ledger) == 0 {
		fmt.Fprintf(&desc, "\n\nNo upcoming hunts in %s.", rs.Region.Name)
	} else {
		desc.WriteString("\n" + onlineLegend)
	}
	if len(rs.Spots) > 0 {
		desc.WriteString("\n\nRespawns: " + cutList(rs.Spots, maxRespawnListChars))
	}
	return &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       cut(rs.Region.Name, maxEmbedTitle),
		Description: desc.String(),
		Color:       regionEmbedColor,
		Footer:      &discordgo.MessageEmbedFooter{Text: regionFooter},
	}
}

func spotEmbed(entry summary.LedgerEntry) *discordgo.MessageEmbed {
	var desc strings.Builder
	for i, b := range entry.Bookings {
		if i == maxRegionBookings {
			fmt.Fprintf(&desc, "+%d more", len(entry.Bookings)-maxRegionBookings)
			break
		}
		fmt.Fprintf(&desc, "%s**%s** - **%s** %s\n", MapOnlineStatus(b.Status), b.StartAt.Format("15:04"), b.EndAt.Format("15:04"), b.Author)
	}
	embed := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       cut(entry.Spot, maxEmbedTitle),
		Description: strings.TrimSuffix(desc.String(), "\n"),
		Color:       regionEmbedColor,
	}
	if entry.ImageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: entry.ImageURL}
	}
	return embed
}

// cutList joins the names and stops before limit characters.
func cutList(names []string, limit int) string {
	var out strings.Builder
	for i, name := range names {
		sep := ""
		if i > 0 {
			sep = ", "
		}
		if utf8.RuneCountInString(out.String()+sep+name) > limit {
			fmt.Fprintf(&out, " and %d more", len(names)-i)
			break
		}
		out.WriteString(sep + name)
	}
	return out.String()
}

func packEmbeds(embeds []*discordgo.MessageEmbed) []*discordgo.MessageSend {
	msgs := []*discordgo.MessageSend{{}}
	chars := 0
	for _, e := range embeds {
		last := msgs[len(msgs)-1]
		size := embedChars(e)
		if len(last.Embeds) > 0 && (len(last.Embeds) == maxEmbedsPerMessage || chars+size > maxMessageChars) {
			last = &discordgo.MessageSend{}
			msgs = append(msgs, last)
			chars = 0
		}
		last.Embeds = append(last.Embeds, e)
		chars += size
	}
	return msgs
}

// embedChars counts what Discord counts towards the 6000 characters of a message.
func embedChars(e *discordgo.MessageEmbed) int {
	n := utf8.RuneCountInString(e.Title) + utf8.RuneCountInString(e.Description)
	if e.Footer != nil {
		n += utf8.RuneCountInString(e.Footer.Text)
	}
	if e.Author != nil {
		n += utf8.RuneCountInString(e.Author.Name)
	}
	for _, f := range e.Fields {
		n += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	return n
}

func regionComponents(r region.Region, webBaseURL string) []discordgo.MessageComponent {
	buttons := []discordgo.MessageComponent{
		button("Book in "+r.Name, discordgo.PrimaryButton, formAction{Kind: actionBookRegion, Region: r.Key}),
		button("My reservations", discordgo.SecondaryButton, formAction{Kind: actionMine}),
	}
	if webBaseURL != "" {
		buttons = append(buttons, linkButton("Open panel", webBaseURL))
	}
	return []discordgo.MessageComponent{row(buttons...)}
}

// payloadHash identifies what the messages show. Hash before stampUpdated, so
// the time alone never causes an edit.
func payloadHash(msgs []*discordgo.MessageSend) string {
	body, err := json.Marshal(msgs)
	if err != nil {
		// Never cached: the messages are written again on the next pass.
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// dropThumbnails removes the respawn images and reports whether there were any.
func dropThumbnails(msgs []*discordgo.MessageSend) bool {
	dropped := false
	for _, m := range msgs {
		for _, e := range m.Embeds {
			dropped = dropped || e.Thumbnail != nil
			e.Thumbnail = nil
		}
	}
	return dropped
}

// stampUpdated shows when the bot last wrote the messages.
func stampUpdated(msgs []*discordgo.MessageSend, now time.Time) {
	if len(msgs) > 0 && len(msgs[0].Embeds) > 0 {
		msgs[0].Embeds[0].Timestamp = now.Format(time.RFC3339)
	}
}

// indexMessage links the region channels from the guild's summary channel,
// with the buttons of the whole guild.
func indexMessage(desired []summary.RegionSummary, rows map[string]*summarylayout.RegionChannel, webBaseURL string) *discordgo.MessageSend {
	var desc strings.Builder
	desc.WriteString("Hunts per region, each in its own channel. Times are in **Europe/Berlin**.\n")
	for _, rs := range desired {
		row, ok := rows[rs.Region.Key]
		if !ok {
			continue
		}
		hunts := 0
		for _, entry := range rs.Ledger {
			hunts += len(entry.Bookings)
		}
		fmt.Fprintf(&desc, "\n<#%s> %s", row.ChannelID, upcomingHunts(hunts))
	}
	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Type:        discordgo.EmbedTypeRich,
			URL:         "https://tibialoot.com",
			Title:       "TibiaLoot.com - Spot Assistant",
			Description: desc.String(),
			Color:       regionEmbedColor,
			Footer:      &discordgo.MessageEmbedFooter{Text: regionFooter},
		}},
		Components: summaryComponents(webBaseURL),
	}
}

func upcomingHunts(n int) string {
	switch n {
	case 0:
		return "no upcoming hunts"
	case 1:
		return "1 upcoming hunt"
	}
	return fmt.Sprintf("%d upcoming hunts", n)
}

// retiredMessage replaces the summary of a region channel the bot no longer needs.
func retiredMessage() *discordgo.MessageSend {
	return &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{{
		Type:        discordgo.EmbedTypeRich,
		Description: "This channel is no longer used. The bot deletes it within a day.",
		Color:       regionEmbedColor,
	}}}
}
