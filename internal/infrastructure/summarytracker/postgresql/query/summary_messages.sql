-- name: GetSummaryMessages :many
SELECT * FROM summary_messages
WHERE guild_id = $1 AND channel_id = $2
ORDER BY message_order ASC;

-- name: UpsertSummaryMessage :one
INSERT INTO summary_messages (
  guild_id, channel_id, message_id, message_type, message_order, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, now(), now())
ON CONFLICT (guild_id, channel_id, message_type, message_order)
DO UPDATE SET
  message_id = EXCLUDED.message_id,
  updated_at = now()
RETURNING *;

-- name: DeleteSummaryMessagesForGuild :exec
DELETE FROM summary_messages
WHERE guild_id = $1 AND channel_id = $2;

-- name: DeleteSummaryMessage :exec
DELETE FROM summary_messages
WHERE id = $1;

-- name: UpdateMessageTimestamp :exec
UPDATE summary_messages
SET updated_at = now()
WHERE id = $1;
