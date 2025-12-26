-- name: SelectAllSpots :many
SELECT
    id,
    name,
    created_at
FROM
    web_spot;

-- name: SelectSpotByName :one
SELECT id, name, created_at
FROM web_spot
WHERE lower(name) = lower(@name)
LIMIT 1;

-- name: SelectSpotsByNameCaseInsensitiveLike :many
SELECT id, name, created_at
FROM web_spot
WHERE lower(name) LIKE '%' || lower(@name_pattern) || '%'
ORDER BY name
LIMIT 15;