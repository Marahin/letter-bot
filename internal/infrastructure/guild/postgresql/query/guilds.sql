-- name: CreateGuild :one
INSERT INTO guilds (
                    guild_id,
                    guild_name
  )
VALUES ($1, $2)
RETURNING *;
-- name: SelectGuilds :many
select sqlc.embed(guilds)
from guilds;