-- name: DeleteGuildChannels :exec
DELETE FROM guild_channels
WHERE guild_id = @guild_id;

-- name: InsertGuildChannels :exec
INSERT INTO guild_channels (guild_id, channel_id, name, type, parent_id, position)
SELECT @guild_id::text,
       unnest(@channel_ids::text[]),
       unnest(@names::text[]),
       unnest(@types::int[]),
       unnest(@parent_ids::text[]),
       unnest(@positions::int[]);

-- name: SelectGuildChannels :many
SELECT channel_id, name, type, parent_id, position
FROM guild_channels
WHERE guild_id = @guild_id
ORDER BY position, lower(name), channel_id;
