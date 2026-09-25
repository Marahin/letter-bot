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
ORDER BY name
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
WHERE id = @id
  AND guild_id = @guild_id::text;

-- name: CountSpotReservations :one
SELECT count(*)
FROM web_reservation
WHERE spot_id = @spot_id
  AND guild_id = @guild_id::text;

-- name: InsertSpotsIgnoreDuplicates :execrows
INSERT INTO web_spot (guild_id, name, created_at)
SELECT @guild_id::text, unnest(@names::text[]), now()
ON CONFLICT (guild_id, lower(name)) WHERE archived_at IS NULL DO NOTHING;
