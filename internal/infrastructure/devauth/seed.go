//go:build devauth

package devauth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

const (
	// ownerID owns both seeded servers: the dev-owner mock user.
	ownerID = "700000000000000005"
	// seedDays is how far back the past reservations go.
	seedDays = 30
)

// SpotNames are the respawns of the premium server.
var SpotNames = []string{"Hero Cave", "Dragon Lair", "Library", "Oramond Fury", "Roshamuul Prison"}

// seedAuthor is one reservation author: a Discord user with their characters, or
// free text that a manager typed (no Discord id).
type seedAuthor struct {
	text      string
	discordID string
}

var seedAuthors = []seedAuthor{
	{text: "Knight Alpha/Druid Beta", discordID: "700000000000000002"},
	{text: "Dev Manager", discordID: "700000000000000001"},
	{text: "Paladin Gamma"},
}

// Seed stores the two dev servers and, once, the premium server's respawns and
// reservations. It is idempotent: a second run only resets the settings.
func Seed(ctx context.Context, pool *pgxpool.Pool, configs ports.GuildConfigRepository, roles ports.GuildRoleRepository, now time.Time) error {
	for id, g := range map[string]struct {
		name    string
		premium bool
	}{
		PremiumGuildID: {name: "Letter E2E", premium: true},
		LockedGuildID:  {name: "Letter E2E Locked"},
	} {
		if _, err := configs.UpsertPresence(ctx, id, g.name, "", ownerID); err != nil {
			return fmt.Errorf("store guild %s: %w", id, err)
		}
		if err := configs.SetPremium(ctx, id, g.premium); err != nil {
			return fmt.Errorf("set premium of %s: %w", id, err)
		}
		if err := roles.Replace(ctx, id, []*role.Role{{ID: ManageRoleID, Name: "Managers", Position: 1, Color: 0xF97316}}); err != nil {
			return fmt.Errorf("store roles of %s: %w", id, err)
		}
		if err := configs.SetRoleIDs(ctx, id, guildconfig.RoleKindManage, []string{ManageRoleID}); err != nil {
			return fmt.Errorf("set manage rank of %s: %w", id, err)
		}
	}
	var seeded bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM web_reservation WHERE guild_id = $1)`, PremiumGuildID).Scan(&seeded); err != nil {
		return fmt.Errorf("check seeded reservations: %w", err)
	}
	if seeded {
		return nil
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return seedReservations(ctx, tx, now)
	})
}

func seedReservations(ctx context.Context, tx pgx.Tx, now time.Time) error {
	spotIDs := make([]int64, len(SpotNames))
	for i, name := range SpotNames {
		err := tx.QueryRow(ctx,
			`INSERT INTO web_spot (name, created_at, guild_id) VALUES ($1, $2, $3) RETURNING id`,
			name, now.AddDate(0, 0, -seedDays-1), PremiumGuildID).Scan(&spotIDs[i])
		if err != nil {
			return fmt.Errorf("insert spot %q: %w", name, err)
		}
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n := 0
	for d := seedDays; d >= 1; d-- {
		for slot, hour := range []int{18, 21} {
			start := day.AddDate(0, 0, -d).Add(time.Duration(hour) * time.Hour)
			author := seedAuthors[n%len(seedAuthors)]
			spot := spotIDs[(d+slot)%len(spotIDs)]
			id, err := insertReservation(ctx, tx, spot, author, start, start.Add(2*time.Hour))
			if err != nil {
				return err
			}
			if err := insertExperience(ctx, tx, id, author.text, n); err != nil {
				return err
			}
			n++
		}
	}
	// One reservation running now and one later today, on respawns no past one blocks.
	if _, err := insertReservation(ctx, tx, spotIDs[0], seedAuthors[0], now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
		return err
	}
	_, err := insertReservation(ctx, tx, spotIDs[1], seedAuthors[1], now.Add(2*time.Hour), now.Add(4*time.Hour))
	return err
}

func insertReservation(ctx context.Context, tx pgx.Tx, spotID int64, author seedAuthor, start, end time.Time) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx,
		`INSERT INTO web_reservation (author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		author.text, start.Add(-24*time.Hour), start, end, spotID, PremiumGuildID, author.discordID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert reservation: %w", err)
	}
	return id, nil
}

// insertExperience gives every fifth reservation "no data", like a character
// outside the world top 1000; the others a gain that varies by reservation.
func insertExperience(ctx context.Context, tx pgx.Tx, reservationID int64, author string, n int) error {
	for i, c := range experience.Characters(author) {
		var start, end, gain *int64
		status := experience.StatusNoData
		if n%5 != 4 {
			s := int64(400_000_000 + n*1_000_000)
			g := int64(1_500_000 + (n%7)*250_000 + i*100_000)
			e := s + g
			start, end, gain = &s, &e, &g
			status = experience.StatusOK
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO reservation_experience (reservation_id, character_key, character_name, start_experience, end_experience, gain, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			reservationID, c.Key, c.Name, start, end, gain, string(status))
		if err != nil {
			return fmt.Errorf("insert reservation experience: %w", err)
		}
	}
	return nil
}
