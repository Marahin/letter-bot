-- name: UpsertQueueEntry :one
INSERT INTO spot_queue_entries (guild_id, spot_id, user_id, length_minutes, expires_at)
VALUES (@guild_id, @spot_id, @user_id, @length_minutes, @expires_at)
ON CONFLICT (guild_id, spot_id, user_id) DO UPDATE
SET length_minutes = EXCLUDED.length_minutes,
    expires_at = EXCLUDED.expires_at,
    offered_at = NULL
RETURNING id, guild_id, spot_id, user_id, length_minutes, created_at, expires_at, offered_at;

-- name: CountUserQueueEntries :one
SELECT count(*)
FROM spot_queue_entries
WHERE guild_id = @guild_id
  AND user_id = @user_id
  AND spot_id <> @except_spot_id
  AND expires_at > @now;

-- name: SelectQueueEntry :one
SELECT q.id, q.guild_id, q.spot_id, q.user_id, q.length_minutes, q.created_at, q.expires_at, q.offered_at,
  s.name AS spot_name
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.id = @id;

-- name: SelectUserQueueEntries :many
SELECT q.id, q.guild_id, q.spot_id, q.user_id, q.length_minutes, q.created_at, q.expires_at, q.offered_at,
  s.name AS spot_name
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
WHERE guild_id = @guild_id
  AND expires_at <= @now;

-- name: SelectGuildQueueCandidates :many
-- The live entries of active respawns. free uses the inclusive overlap of
-- SelectOverlappingReservations, so a free respawn can be booked.
SELECT q.id, q.guild_id, q.spot_id, q.user_id, q.length_minutes, q.created_at, q.expires_at, q.offered_at,
  s.name AS spot_name,
  NOT EXISTS (
    SELECT 1
    FROM web_reservation r
    WHERE r.guild_id = q.guild_id
      AND r.spot_id = q.spot_id
      AND tstzrange(sqlc.arg(start_at)::timestamptz, sqlc.arg(start_at)::timestamptz + make_interval(0, 0, 0, 0, 0, q.length_minutes), '[]')
        && tstzrange(r.start_at, r.end_at, '[]')
  ) AS free
FROM spot_queue_entries q
JOIN web_spot s ON s.id = q.spot_id
WHERE q.guild_id = @guild_id
  AND q.expires_at > @now
  AND s.archived_at IS NULL
ORDER BY q.created_at, q.id;

-- name: ClaimQueueOffer :execrows
UPDATE spot_queue_entries
SET offered_at = @offered_at
WHERE id = @id
  AND offered_at IS NULL;

-- name: ResetQueueOffers :exec
UPDATE spot_queue_entries
SET offered_at = NULL
WHERE id = ANY(@ids::bigint[]);
