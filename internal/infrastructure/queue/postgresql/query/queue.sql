-- name: UpsertQueueEntry :one
INSERT INTO spot_queue_entries (guild_id, spot_id, user_id, length_minutes, expires_at)
VALUES (@guild_id, @spot_id, @user_id, @length_minutes, @expires_at)
ON CONFLICT (guild_id, spot_id, user_id) DO UPDATE
SET length_minutes = EXCLUDED.length_minutes,
    expires_at = EXCLUDED.expires_at,
    offered_at = NULL
RETURNING *;

-- name: CountUserQueueEntries :one
SELECT count(*)
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.guild_id = @guild_id
  AND q.user_id = @user_id
  AND q.spot_id <> @except_spot_id
  AND q.expires_at > @now
  AND s.archived_at IS NULL;

-- name: SelectQueueEntry :one
SELECT sqlc.embed(q), s.name AS spot_name
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.id = @id;

-- name: SelectUserQueueEntries :many
SELECT sqlc.embed(q), s.name AS spot_name
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.guild_id = @guild_id
  AND q.user_id = @user_id
  AND q.expires_at > @now
ORDER BY q.created_at, q.id;

-- name: DeleteQueueEntry :execrows
DELETE FROM spot_queue_entries
WHERE id = @id
  AND user_id = @user_id;

-- name: DeleteUserSpotQueueEntry :execrows
DELETE FROM spot_queue_entries
WHERE guild_id = @guild_id
  AND user_id = @user_id
  AND spot_id = @spot_id;

-- name: DeleteExpiredQueueEntries :execrows
DELETE FROM spot_queue_entries
WHERE expires_at <= @now;

-- name: SelectDueQueueEntries :many
SELECT sqlc.embed(q), s.name AS spot_name
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.guild_id = @guild_id
  AND q.expires_at > @now
  AND q.offered_at IS NULL
  AND s.archived_at IS NULL
  AND spot_queue_free(q.guild_id, q.spot_id, q.length_minutes, sqlc.arg(start_at)::timestamptz)
ORDER BY q.created_at, q.id;

-- name: ClaimQueueOffer :execrows
-- The respawn may have been taken since the caller read it, so "free" is checked again.
UPDATE spot_queue_entries
SET offered_at = @offered_at
WHERE id = @id
  AND offered_at IS NULL
  AND expires_at > @offered_at
  AND spot_queue_free(guild_id, spot_id, length_minutes, sqlc.arg(start_at)::timestamptz);

-- name: ResetTakenQueueOffers :execrows
-- One statement, so an offer claimed after the caller's last read stays.
UPDATE spot_queue_entries
SET offered_at = NULL
WHERE guild_id = @guild_id
  AND offered_at IS NOT NULL
  AND NOT spot_queue_free(guild_id, spot_id, length_minutes, sqlc.arg(start_at)::timestamptz);

-- name: ResetQueueOffer :exec
UPDATE spot_queue_entries
SET offered_at = NULL
WHERE id = @id;
