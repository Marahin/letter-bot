//go:build devauth

package devauth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

const (
	// ownerID owns the mock users' servers: the dev-owner mock user.
	ownerID = "700000000000000005"
	// alliesOwnerID owns AlliesGuildID; it is no mock user.
	alliesOwnerID = "700000000000000099"
	// seedDays is how far back the past reservations go.
	seedDays = 30
	// alliesDays is how far back the reservations of AlliesGuildID go.
	alliesDays = 10
	// lootHashPrefix marks the seeded loot sessions, whose hash is not a real one.
	lootHashPrefix = "devauth-seed:"
)

// The worlds of the premium servers.
const (
	PremiumWorld = "Celesta"
	AlliesWorld  = "Antica"
)

// SpotNames are the respawns of the premium server.
var SpotNames = []string{"Hero Cave", "Dragon Lair", "Library", "Oramond Fury", "Roshamuul Prison"}

// AlliesSpotNames are the respawns of AlliesGuildID. "Hero Cave" is booked on both worlds.
var AlliesSpotNames = []string{"Hero Cave", "Banuta"}

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

var alliesAuthors = []seedAuthor{
	{text: "Sorcerer Echo/Knight Foxtrot"},
	{text: "Druid Golf"},
}

// lootGuest is a party member in every seeded session who is in no author text.
const lootGuest = "Sorcerer Delta"

// Seed stores the dev servers and, once each, the premium servers' respawns, reservations and
// loot sessions. It is idempotent: a second run only resets the settings.
func Seed(ctx context.Context, pool *pgxpool.Pool, configs ports.GuildConfigRepository, roles ports.GuildRoleRepository, now time.Time) error {
	for id, g := range map[string]struct {
		name    string
		owner   string
		premium bool
	}{
		PremiumGuildID: {name: "Letter E2E", owner: ownerID, premium: true},
		LockedGuildID:  {name: "Letter E2E Locked", owner: ownerID},
		AlliesGuildID:  {name: "Letter E2E Allies", owner: alliesOwnerID, premium: true},
	} {
		if _, err := configs.UpsertPresence(ctx, id, g.name, "", g.owner); err != nil {
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
	for id, world := range map[string]string{PremiumGuildID: PremiumWorld, AlliesGuildID: AlliesWorld} {
		if _, err := pool.Exec(ctx, `INSERT INTO guilds_world (guild_id, world_name) VALUES ($1, $2) ON CONFLICT (guild_id) DO NOTHING`, id, world); err != nil {
			return fmt.Errorf("set world of %s: %w", id, err)
		}
	}
	for _, s := range []struct {
		guildID string
		seed    func(pgx.Tx) error
	}{
		{PremiumGuildID, func(tx pgx.Tx) error { return seedReservations(ctx, tx, now) }},
		{AlliesGuildID, func(tx pgx.Tx) error { return seedAllies(ctx, tx, now) }},
	} {
		var seeded bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM web_reservation WHERE guild_id = $1)`, s.guildID).Scan(&seeded); err != nil {
			return fmt.Errorf("check seeded reservations: %w", err)
		}
		if seeded {
			continue
		}
		if err := pgx.BeginFunc(ctx, pool, s.seed); err != nil {
			return err
		}
	}
	return seedLoot(ctx, pool, now)
}

func seedReservations(ctx context.Context, tx pgx.Tx, now time.Time) error {
	spotIDs, err := insertSpots(ctx, tx, PremiumGuildID, SpotNames, now.AddDate(0, 0, -seedDays-1))
	if err != nil {
		return err
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n := 0
	for d := seedDays; d >= 1; d-- {
		for slot, hour := range []int{18, 21} {
			start := day.AddDate(0, 0, -d).Add(time.Duration(hour) * time.Hour)
			author := seedAuthors[n%len(seedAuthors)]
			spot := spotIDs[(d+slot)%len(spotIDs)]
			id, err := insertReservation(ctx, tx, PremiumGuildID, spot, author, start, start.Add(2*time.Hour))
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
	if _, err := insertReservation(ctx, tx, PremiumGuildID, spotIDs[0], seedAuthors[0], now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
		return err
	}
	_, err = insertReservation(ctx, tx, PremiumGuildID, spotIDs[1], seedAuthors[1], now.Add(2*time.Hour), now.Add(4*time.Hour))
	return err
}

// seedAllies books one evening hunt a day on AlliesGuildID, without experience data.
func seedAllies(ctx context.Context, tx pgx.Tx, now time.Time) error {
	spotIDs, err := insertSpots(ctx, tx, AlliesGuildID, AlliesSpotNames, now.AddDate(0, 0, -alliesDays-1))
	if err != nil {
		return err
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for d := alliesDays; d >= 1; d-- {
		start := day.AddDate(0, 0, -d).Add(19 * time.Hour)
		author := alliesAuthors[d%len(alliesAuthors)]
		if _, err := insertReservation(ctx, tx, AlliesGuildID, spotIDs[d%len(spotIDs)], author, start, start.Add(2*time.Hour)); err != nil {
			return err
		}
	}
	return nil
}

func insertSpots(ctx context.Context, tx pgx.Tx, guildID string, names []string, created time.Time) ([]int64, error) {
	ids := make([]int64, len(names))
	for i, name := range names {
		err := tx.QueryRow(ctx,
			`INSERT INTO web_spot (name, created_at, guild_id) VALUES ($1, $2, $3) RETURNING id`,
			name, created, guildID).Scan(&ids[i])
		if err != nil {
			return nil, fmt.Errorf("insert spot %q: %w", name, err)
		}
	}
	return ids, nil
}

func insertReservation(ctx context.Context, tx pgx.Tx, guildID string, spotID int64, author seedAuthor, start, end time.Time) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx,
		`INSERT INTO web_reservation (author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		author.text, start.Add(-24*time.Hour), start, end, spotID, guildID, author.discordID).Scan(&id)
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

// seedLoot gives two of every three ended reservations of the premium servers a loot session,
// as if the party pasted it in the Loot Calculator, once: a server with a session is skipped.
func seedLoot(ctx context.Context, pool *pgxpool.Pool, now time.Time) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.id, r.guild_id, r.author, r.start_at, r.end_at
			FROM web_reservation r
			WHERE r.guild_id = ANY($1) AND r.end_at <= $2
			  AND NOT EXISTS (SELECT 1 FROM reservation_loot rl JOIN web_reservation o ON o.id = rl.reservation_id WHERE o.guild_id = r.guild_id)
			ORDER BY r.start_at, r.id`, []string{PremiumGuildID, AlliesGuildID}, now)
		if err != nil {
			return fmt.Errorf("list seeded reservations: %w", err)
		}
		type booked struct {
			id         int64
			guildID    string
			author     string
			start, end time.Time
		}
		var all []booked
		for rows.Next() {
			var b booked
			if err := rows.Scan(&b.id, &b.guildID, &b.author, &b.start, &b.end); err != nil {
				return fmt.Errorf("scan seeded reservation: %w", err)
			}
			all = append(all, b)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("list seeded reservations: %w", err)
		}
		// Counted per server: a shared counter would skip the same server's rows every time.
		counts := map[string]int{}
		for _, b := range all {
			n := counts[b.guildID]
			counts[b.guildID]++
			if n%3 == 2 {
				continue
			}
			if err := insertLoot(ctx, tx, b.id, b.author, b.start.Add(10*time.Minute), b.end.Add(-5*time.Minute), n); err != nil {
				return err
			}
		}
		return nil
	})
}

// insertLoot stores one session of the author's characters and lootGuest, attributed to the
// reservation. Every seventh session is a loss.
func insertLoot(ctx context.Context, tx pgx.Tx, reservationID int64, author string, from, to time.Time, n int) error {
	characters := experience.Characters(author)
	names := make([]string, 0, len(characters)+1)
	for _, c := range characters {
		names = append(names, c.Name)
	}
	names = append(names, lootGuest)
	balance := int64(300_000 + (n%5)*150_000)
	if n%7 == 3 {
		balance = -120_000
	}
	supplies := int64(200_000 + (n%3)*50_000)
	members := int64(len(names))
	duration := int(to.Sub(from) / time.Second)
	var sessionID int64
	err := tx.QueryRow(ctx,
		`INSERT INTO loot_sessions (content_hash, started_at, ended_at, duration_seconds, members, loot, supplies, balance, damage, healing)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		[]byte(lootHashPrefix+strconv.FormatInt(reservationID, 10)), from, to, duration, members,
		(balance+supplies)/members*members, supplies/members*members, balance/members*members, members*900_000, members*150_000).Scan(&sessionID)
	if err != nil {
		return fmt.Errorf("insert loot session: %w", err)
	}
	for i, name := range names {
		_, err := tx.Exec(ctx,
			`INSERT INTO loot_session_members (session_id, character_key, character_name, position, leader, loot, supplies, balance, damage, healing)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 900000, 150000)`,
			sessionID, experience.CharacterKey(name), name, i, i == 0, (balance+supplies)/members, supplies/members, balance/members)
		if err != nil {
			return fmt.Errorf("insert loot session member: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO reservation_loot (reservation_id, session_id, overlap_seconds) VALUES ($1, $2, $3)`, reservationID, sessionID, duration)
	if err != nil {
		return fmt.Errorf("pick loot session: %w", err)
	}
	return nil
}
