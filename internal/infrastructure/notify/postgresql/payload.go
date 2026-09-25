package postgresql

import (
	"encoding/json"
	"errors"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

// NOTIFY channels from the web to the bot. The payload of the first three is a guild id.
const (
	ChannelSummaryRefresh = "letter_summary_refresh"
	ChannelGuildResync    = "letter_guild_resync"
	ChannelGuildConfig    = "letter_guild_config"
	ChannelOverbooked     = "letter_overbooked"
)

var Channels = []string{ChannelSummaryRefresh, ChannelGuildResync, ChannelGuildConfig, ChannelOverbooked}

// OverbookedPayload tells the bot to send the overbooked DM for a reservation made in the web.
type OverbookedPayload struct {
	GuildID        string                     `json:"guild_id"`
	MemberID       string                     `json:"member_id"`
	MemberNick     string                     `json:"member_nick"`
	MemberUsername string                     `json:"member_username"`
	Spot           string                     `json:"spot"`
	StartAt        time.Time                  `json:"start_at"`
	EndAt          time.Time                  `json:"end_at"`
	Original       *reservation.Reservation   `json:"original"`
	New            []*reservation.Reservation `json:"new"`
}

func NewOverbookedPayload(request book.BookRequest, res *reservation.ClippedOrRemovedReservation) OverbookedPayload {
	p := OverbookedPayload{
		Spot:     request.Spot,
		StartAt:  request.StartAt,
		EndAt:    request.EndAt,
		Original: res.Original,
		New:      res.New,
	}
	if request.Guild != nil {
		p.GuildID = request.Guild.ID
	}
	if request.Member != nil {
		p.MemberID = request.Member.ID
		p.MemberNick = request.Member.Nick
		p.MemberUsername = request.Member.Username
	}
	return p
}

func (p OverbookedPayload) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func DecodeOverbookedPayload(data []byte) (OverbookedPayload, error) {
	var p OverbookedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.GuildID == "" || p.Original == nil {
		return p, errors.New("overbooked payload needs guild_id and original")
	}
	return p, nil
}

func (p OverbookedPayload) BookRequest() book.BookRequest {
	return book.BookRequest{
		Guild:    &guild.Guild{ID: p.GuildID},
		Member:   &member.Member{ID: p.MemberID, Nick: p.MemberNick, Username: p.MemberUsername},
		Spot:     p.Spot,
		StartAt:  p.StartAt,
		EndAt:    p.EndAt,
		Overbook: true,
	}
}

func (p OverbookedPayload) Reservation() *reservation.ClippedOrRemovedReservation {
	return &reservation.ClippedOrRemovedReservation{Original: p.Original, New: p.New}
}
