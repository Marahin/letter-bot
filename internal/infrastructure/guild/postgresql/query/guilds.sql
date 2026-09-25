-- name: SelectGuild :one
SELECT *
FROM guilds
WHERE guild_id = @guild_id;

-- name: SelectGuildsByIDs :many
SELECT *
FROM guilds
WHERE guild_id = ANY(@guild_ids::text[])
ORDER BY lower(name), guild_id;

-- name: SelectAllGuilds :many
SELECT *
FROM guilds
ORDER BY lower(name), guild_id;

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
SET resync_requested_at = NULL, synced_at = now()
WHERE guild_id = @guild_id;
