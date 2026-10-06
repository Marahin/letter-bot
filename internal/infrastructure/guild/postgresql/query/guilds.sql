-- name: SelectGuild :one
SELECT *
FROM guilds
WHERE guild_id = @guild_id;

-- name: SelectGuildsByIDs :many
SELECT *
FROM guilds
WHERE guild_id = ANY(@guild_ids::text[])
ORDER BY lower(name), guild_id;

-- name: SelectPresentGuildsByIDs :many
SELECT *
FROM guilds
WHERE guild_id = ANY(@guild_ids::text[])
  AND bot_present
ORDER BY lower(name), guild_id;

-- name: SelectAllGuilds :many
SELECT *
FROM guilds
ORDER BY bot_present DESC, lower(name), guild_id;

-- Never touches the admin-set columns (premium, channels, ranks).
-- name: UpsertGuildPresence :one
INSERT INTO guilds (guild_id, name, icon, owner_id, bot_present)
VALUES (@guild_id, @name, @icon, @owner_id, true)
ON CONFLICT (guild_id) DO UPDATE
SET name = EXCLUDED.name,
    icon = EXCLUDED.icon,
    owner_id = EXCLUDED.owner_id,
    bot_present = true,
    updated_at = now()
RETURNING *;

-- name: SetGuildBotPresent :execrows
UPDATE guilds
SET bot_present = @bot_present, updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildPremium :execrows
UPDATE guilds
SET premium = @premium, updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildChannels :execrows
UPDATE guilds
SET command_channel_id = @command_channel_id,
    summary_channel_id = @summary_channel_id,
    updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildManageRoleIDs :execrows
UPDATE guilds
SET manage_role_ids = @role_ids::text[], updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildViewRoleIDs :execrows
UPDATE guilds
SET view_role_ids = @role_ids::text[], updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildReserveRoleIDs :execrows
UPDATE guilds
SET reserve_role_ids = @role_ids::text[], updated_at = now()
WHERE guild_id = @guild_id;

-- name: SetGuildOverbookRoleIDs :execrows
UPDATE guilds
SET overbook_role_ids = @role_ids::text[], updated_at = now()
WHERE guild_id = @guild_id;

-- name: RequestGuildResync :execrows
UPDATE guilds
SET resync_requested_at = now()
WHERE guild_id = @guild_id;

-- name: SelectResyncRequestedGuildIDs :many
SELECT guild_id
FROM guilds
WHERE resync_requested_at IS NOT NULL
  AND bot_present
ORDER BY resync_requested_at;

-- name: MarkGuildSynced :execrows
UPDATE guilds
SET resync_requested_at = CASE WHEN resync_requested_at <= @started_at::timestamptz THEN NULL ELSE resync_requested_at END,
    synced_at = now()
WHERE guild_id = @guild_id;

-- Discord routes a guild to shard (guild_id >> 22) % shard_count. The CASE keeps
-- a non-snowflake id away from the bigint cast.
-- name: MarkGuildsAbsentExcept :exec
UPDATE guilds
SET bot_present = false, updated_at = now()
WHERE bot_present
  AND updated_at < @ready_at::timestamptz
  AND NOT (guild_id = ANY(@present_ids::text[]))
  AND CASE WHEN guild_id ~ '^[0-9]{1,19}$'
    THEN (guild_id::bigint >> 22) % @shard_count::bigint = @shard_id::bigint
    ELSE false
  END;
