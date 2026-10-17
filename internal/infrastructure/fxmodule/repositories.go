package fxmodule

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	experiencesqlc "spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	lootsqlc "spot-assistant/internal/infrastructure/loot/postgresql/sqlc"
	reservationsqlc "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotsqlc "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
	statssqlc "spot-assistant/internal/infrastructure/stats/postgresql/sqlc"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	worldnamesqlc "spot-assistant/internal/infrastructure/worldname/postgresql/sqlc"
)

// Repositories provides every PostgreSQL repository as its concrete type. The
// stats repository needs the time zone, so the web provides it.
var Repositories = fx.Provide(
	guildConfigRepository,
	guildChannelRepository,
	guildRoleRepository,
	worldNameRepository,
	spotRepository,
	webUserRepository,
	experienceRepository,
	reservationRepository,
	lootRepository,
	worldStatsRepository,
)

func guildConfigRepository(pool *pgxpool.Pool) *guildsqlc.GuildConfigRepository {
	return guildsqlc.NewGuildConfigRepository(pool)
}

func guildChannelRepository(pool *pgxpool.Pool) *guildsqlc.GuildChannelRepository {
	return guildsqlc.NewGuildChannelRepository(pool)
}

func guildRoleRepository(pool *pgxpool.Pool) *guildsqlc.GuildRoleRepository {
	return guildsqlc.NewGuildRoleRepository(pool)
}

func worldNameRepository(pool *pgxpool.Pool) *worldnamesqlc.WorldNameRepository {
	return worldnamesqlc.NewWorldNameRepository(pool)
}

func spotRepository(pool *pgxpool.Pool) *spotsqlc.SpotRepository {
	return spotsqlc.NewSpotRepository(pool)
}

func webUserRepository(pool *pgxpool.Pool) *webusersqlc.WebUserRepository {
	return webusersqlc.NewWebUserRepository(pool)
}

func experienceRepository(pool *pgxpool.Pool) *experiencesqlc.ExperienceRepository {
	return experiencesqlc.NewExperienceRepository(pool)
}

func reservationRepository(pool *pgxpool.Pool, log *zap.SugaredLogger) *reservationsqlc.ReservationRepository {
	return reservationsqlc.NewReservationRepository(pool).WithLogger(log)
}

func lootRepository(pool *pgxpool.Pool) *lootsqlc.LootRepository {
	return lootsqlc.NewLootRepository(pool)
}

func worldStatsRepository(pool *pgxpool.Pool) *statssqlc.WorldStatsRepository {
	return statssqlc.NewWorldStatsRepository(pool)
}
