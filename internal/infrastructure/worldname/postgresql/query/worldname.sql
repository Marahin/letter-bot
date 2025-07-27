-- name: UpsertGuildWorld :exec
INSERT INTO guilds_world (guild_id, world_name, created_at, updated_at)
VALUES ($1, $2, now(), now())
ON CONFLICT (guild_id)
DO UPDATE SET world_name = EXCLUDED.world_name, updated_at = now();

-- name: SelectGuildWorld :one
SELECT id, guild_id, world_name
FROM guilds_world
WHERE guild_id = $1
LIMIT 1;
