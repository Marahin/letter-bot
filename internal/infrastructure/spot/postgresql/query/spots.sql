-- name: SelectGuildSpots :many
SELECT id, name, created_at, guild_id, archived_at
FROM web_spot
WHERE guild_id = @guild_id::text
  AND (@include_archived::boolean OR archived_at IS NULL)
ORDER BY lower(name), id;

-- name: SelectGuildSpotByName :one
SELECT id, name, created_at, guild_id, archived_at
FROM web_spot
WHERE guild_id = @guild_id::text
  AND archived_at IS NULL
  AND lower(name) = lower(@name)
LIMIT 1;

-- name: SelectGuildSpotsLike :many
SELECT id, name, created_at, guild_id, archived_at
FROM web_spot
WHERE guild_id = @guild_id::text
  AND archived_at IS NULL
  AND lower(name) LIKE '%' || lower(@name_pattern) || '%'
ORDER BY lower(name), id
LIMIT 15;

-- name: SelectGuildSpotByID :one
SELECT id, name, created_at, guild_id, archived_at
FROM web_spot
WHERE id = @id
  AND guild_id = @guild_id::text
LIMIT 1;

-- name: InsertSpot :one
INSERT INTO web_spot (name, created_at, guild_id)
VALUES (@name, now(), @guild_id::text)
RETURNING id, name, created_at, guild_id, archived_at;

-- name: RenameSpot :execrows
UPDATE web_spot
SET name = @name
WHERE id = @id
  AND guild_id = @guild_id::text;

-- name: ArchiveSpot :execrows
UPDATE web_spot
SET archived_at = now()
WHERE id = @id
  AND guild_id = @guild_id::text
  AND archived_at IS NULL;

-- name: RestoreSpot :execrows
UPDATE web_spot
SET archived_at = NULL
WHERE id = @id
  AND guild_id = @guild_id::text
  AND archived_at IS NOT NULL;

-- name: DeleteSpot :execrows
DELETE FROM web_spot
WHERE web_spot.id = @id
  AND web_spot.guild_id = @guild_id::text
  AND NOT EXISTS (SELECT 1 FROM web_reservation r WHERE r.spot_id = web_spot.id);

-- One tab of the respawn list. The counts take every reservation that points at
-- the spot, whatever its guild_id, because any of them blocks DeleteSpot.
-- name: SelectGuildSpotList :many
SELECT s.id, s.name, s.created_at, s.guild_id, s.archived_at, c.total, c.upcoming
FROM web_spot s
  CROSS JOIN LATERAL (
    SELECT count(*) AS total,
      count(*) FILTER (WHERE r.end_at >= now()) AS upcoming
    FROM web_reservation r
    WHERE r.spot_id = s.id
  ) c
WHERE s.guild_id = @guild_id::text
  AND (s.archived_at IS NOT NULL) = @archived::boolean
  AND lower(s.name) LIKE '%' || lower(@name_pattern::text) || '%'
ORDER BY lower(s.name), s.id;

-- name: CountGuildSpots :one
SELECT count(*) FILTER (WHERE archived_at IS NULL) AS active,
  count(*) FILTER (WHERE archived_at IS NOT NULL) AS archived
FROM web_spot
WHERE guild_id = @guild_id::text;

-- The single-spot variant of the SelectGuildSpotList counts.
-- name: SelectSpotReservationCounts :one
SELECT count(*) AS total,
  count(*) FILTER (WHERE r.end_at >= now()) AS upcoming
FROM web_reservation r
  INNER JOIN web_spot s ON s.id = r.spot_id
WHERE s.id = @spot_id
  AND s.guild_id = @guild_id::text;

-- name: InsertSpotsIgnoreDuplicates :execrows
INSERT INTO web_spot (guild_id, name, created_at)
SELECT @guild_id::text, unnest(@names::text[]), now()
ON CONFLICT (guild_id, lower(name)) WHERE archived_at IS NULL DO NOTHING;

-- The respawns a member (or anyone, with a NULL author) booked most since a time.
-- name: SelectTopGuildSpots :many
SELECT s.id, s.name, s.created_at, s.guild_id, s.archived_at,
  count(r.id) AS bookings,
  max(r.start_at)::timestamptz AS last_start_at
FROM web_reservation r
  INNER JOIN web_spot s ON s.id = r.spot_id
WHERE r.guild_id = @guild_id::text
  AND s.guild_id = @guild_id::text
  AND s.archived_at IS NULL
  AND r.start_at >= @since::timestamptz
  AND (sqlc.narg(author_discord_id)::text IS NULL OR r.author_discord_id = sqlc.narg(author_discord_id)::text)
GROUP BY s.id, s.name, s.created_at, s.guild_id, s.archived_at
ORDER BY bookings DESC, last_start_at DESC, s.id
LIMIT @row_limit::int;
