-- name: SelectSummaryLayout :one
SELECT guild_id, category_id, index_channel_id, index_message_id, index_hash, blocked_until
FROM summary_layouts
WHERE guild_id = @guild_id;

-- name: UpsertSummaryLayout :exec
INSERT INTO summary_layouts (guild_id, category_id, index_channel_id, index_message_id, index_hash, blocked_until)
VALUES (@guild_id, @category_id, @index_channel_id, @index_message_id, @index_hash, sqlc.narg(blocked_until)::timestamptz)
ON CONFLICT (guild_id) DO UPDATE
SET category_id = EXCLUDED.category_id,
    index_channel_id = EXCLUDED.index_channel_id,
    index_message_id = EXCLUDED.index_message_id,
    index_hash = EXCLUDED.index_hash,
    blocked_until = EXCLUDED.blocked_until,
    updated_at = now();

-- name: SelectRegionChannels :many
SELECT guild_id, region, channel_id, message_ids, content_hash, retired_at
FROM summary_region_channels
WHERE guild_id = @guild_id
ORDER BY region;

-- name: UpsertRegionChannel :exec
INSERT INTO summary_region_channels (guild_id, region, channel_id, message_ids, content_hash, retired_at)
VALUES (@guild_id, @region, @channel_id, @message_ids::text[], @content_hash, sqlc.narg(retired_at)::timestamptz)
ON CONFLICT (guild_id, region) DO UPDATE
SET channel_id = EXCLUDED.channel_id,
    message_ids = EXCLUDED.message_ids,
    content_hash = EXCLUDED.content_hash,
    retired_at = EXCLUDED.retired_at,
    updated_at = now();

-- name: DeleteRegionChannel :exec
DELETE FROM summary_region_channels
WHERE guild_id = @guild_id
  AND region = @region;
