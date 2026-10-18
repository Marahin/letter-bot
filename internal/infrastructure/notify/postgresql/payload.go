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
	ChannelBooked         = "letter_booked"
)

var Channels = []string{ChannelSummaryRefresh, ChannelGuildResync, ChannelGuildConfig, ChannelOverbooked, ChannelBooked}

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

// reservation.Reservation has no json tags. Its Go field names are the wire
// format, so a bot and a web of different versions still read each other.
func (p OverbookedPayload) Encode() ([]byte, error) {
	return json.Marshal(p) //nolint:musttag // see above
}

func DecodeOverbookedPayload(data []byte) (OverbookedPayload, error) {
	var p OverbookedPayload
	if err := json.Unmarshal(data, &p); err != nil { //nolint:musttag // see Encode
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

// BookedPayload tells the bot to announce a reservation made in the web.
type BookedPayload struct {
	GuildID         string    `json:"guild_id"`
	SpotID          int64     `json:"spot_id"`
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	Author          string    `json:"author"`
	AuthorDiscordID string    `json:"author_discord_id"`
	ActorID         string    `json:"actor_id"`
	Overbooked      int       `json:"overbooked"`
}

func NewBookedPayload(event book.Booked) BookedPayload {
	return BookedPayload{
		GuildID:         event.GuildID,
		SpotID:          event.SpotID,
		StartAt:         event.StartAt,
		EndAt:           event.EndAt,
		Author:          event.Author,
		AuthorDiscordID: event.AuthorDiscordID,
		ActorID:         event.ActorID,
		Overbooked:      event.Overbooked,
	}
}

func (p BookedPayload) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func DecodeBookedPayload(data []byte) (BookedPayload, error) {
	var p BookedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.GuildID == "" || p.SpotID == 0 {
		return p, errors.New("booked payload needs guild_id and spot_id")
	}
	return p, nil
}

// Event is the announcement of the payload. Only the web sends it.
func (p BookedPayload) Event() book.Booked {
	return book.Booked{
		GuildID:         p.GuildID,
		SpotID:          p.SpotID,
		StartAt:         p.StartAt,
		EndAt:           p.EndAt,
		Author:          p.Author,
		AuthorDiscordID: p.AuthorDiscordID,
		ActorID:         p.ActorID,
		Overbooked:      p.Overbooked,
		Source:          book.SourceWeb,
	}
}
