-- name: SelectReservation :one
SELECT *
FROM web_reservation
WHERE id = @id
LIMIT 1;
-- name: SelectReservationWithSpot :one
SELECT sqlc.embed(reservations),
  sqlc.embed(spots)
FROM web_reservation reservations
  JOIN web_spot spots ON spots.id = reservations.spot_id
WHERE reservations.id = @id
  and reservations.guild_id = @guild_id
  AND reservations.author_discord_id = @author_discord_id
LIMIT 1;
-- name: DeletePresentMemberReservation :exec
DELETE FROM web_reservation
where web_reservation.guild_id = @guild_id
  AND web_reservation.author_discord_id = @author_discord_id
  AND web_reservation.id = @id
  AND web_reservation.end_at > now();
-- name: SelectUpcomingMemberReservationsWithSpots :many
select sqlc.embed(web_spot),
  sqlc.embed(web_reservation)
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where web_reservation.end_at >= now()
  AND web_reservation.guild_id = @guild_id
  AND web_reservation.author_discord_id = @author_discord_id
order by start_at asc;
-- name: SelectOverlappingReservations :many
SELECT web_reservation.id,
  web_reservation.author,
  web_reservation.author_discord_id,
  web_reservation.start_at,
  web_reservation.end_at,
  web_reservation.guild_id
FROM web_reservation
WHERE web_reservation.end_at >= now()
  AND tstzrange(@start_at, @end_at, '[]') && tstzrange(
    web_reservation.start_at,
    web_reservation.end_at,
    '[]'
  )
  AND web_reservation.spot_id = @spot_id
  AND web_reservation.guild_id = @guild_id;
-- name: CreateReservation :one
INSERT INTO web_reservation (
    author,
    author_discord_id,
    start_at,
    end_at,
    spot_id,
    created_at,
    guild_id
  )
VALUES ($1, $2, $3, $4, $5, now(), $6)
RETURNING *;
-- name: SelectReservationsWithSpots :many
select sqlc.embed(web_spot),
  sqlc.embed(web_reservation)
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where web_reservation.end_at >= now()
  AND web_reservation.guild_id = $1;
-- name: SelectReservationsWithSpotsForSpot :many
select sqlc.embed(web_spot),
  sqlc.embed(web_reservation)
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where web_reservation.end_at >= now()
  AND web_reservation.guild_id = $1
  AND lower(web_spot.name) = lower($2);
-- name: DeleteReservation :exec
DELETE FROM web_reservation
WHERE web_reservation.id = $1;
-- name: SearchReservationsWithSpot :many
SELECT sqlc.embed(web_reservation),
  sqlc.embed(web_spot)
FROM web_reservation
  INNER JOIN web_spot ON web_spot.id = web_reservation.spot_id
WHERE web_reservation.guild_id = @guild_id::text
  AND (sqlc.narg(spot_id)::bigint IS NULL OR web_reservation.spot_id = sqlc.narg(spot_id)::bigint)
  AND (sqlc.narg(author)::text IS NULL OR lower(web_reservation.author) LIKE '%' || lower(sqlc.narg(author)::text) || '%')
  AND (sqlc.narg(author_discord_id)::text IS NULL OR web_reservation.author_discord_id = sqlc.narg(author_discord_id)::text)
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR web_reservation.end_at >= sqlc.narg(from_at)::timestamptz)
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR web_reservation.start_at <= sqlc.narg(to_at)::timestamptz)
  AND (
    @scope::text = 'all'
    OR (@scope::text = 'upcoming' AND web_reservation.end_at >= now())
    OR (@scope::text = 'past' AND web_reservation.end_at < now())
  )
ORDER BY CASE WHEN @scope::text = 'upcoming' THEN web_reservation.start_at END ASC,
  web_reservation.start_at DESC,
  web_reservation.id DESC
LIMIT @row_limit::int OFFSET @row_offset::int;
-- name: CountReservations :one
SELECT count(*)
FROM web_reservation
WHERE web_reservation.guild_id = @guild_id::text
  AND (sqlc.narg(spot_id)::bigint IS NULL OR web_reservation.spot_id = sqlc.narg(spot_id)::bigint)
  AND (sqlc.narg(author)::text IS NULL OR lower(web_reservation.author) LIKE '%' || lower(sqlc.narg(author)::text) || '%')
  AND (sqlc.narg(author_discord_id)::text IS NULL OR web_reservation.author_discord_id = sqlc.narg(author_discord_id)::text)
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR web_reservation.end_at >= sqlc.narg(from_at)::timestamptz)
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR web_reservation.start_at <= sqlc.narg(to_at)::timestamptz)
  AND (
    @scope::text = 'all'
    OR (@scope::text = 'upcoming' AND web_reservation.end_at >= now())
    OR (@scope::text = 'past' AND web_reservation.end_at < now())
  );
-- name: SelectGuildReservationWithSpot :one
SELECT sqlc.embed(web_reservation),
  sqlc.embed(web_spot)
FROM web_reservation
  INNER JOIN web_spot ON web_spot.id = web_reservation.spot_id
WHERE web_reservation.id = @id
  AND web_reservation.guild_id = @guild_id::text
LIMIT 1;
-- name: UpdateReservation :execrows
UPDATE web_reservation
SET spot_id = @spot_id,
  start_at = @start_at,
  end_at = @end_at,
  author = @author,
  author_discord_id = @author_discord_id
WHERE id = @id
  AND guild_id = @guild_id::text;
-- name: DeleteGuildReservation :execrows
DELETE FROM web_reservation
WHERE id = @id
  AND guild_id = @guild_id::text;
-- name: SelectOverlappingReservationsBySpotID :many
SELECT id,
  author,
  created_at,
  start_at,
  end_at,
  spot_id,
  guild_id,
  author_discord_id
FROM web_reservation
WHERE spot_id = @spot_id
  AND guild_id = @guild_id::text
  AND id <> @exclude_id::bigint
  AND tstzrange(@start_at::timestamptz, @end_at::timestamptz, '[]') && tstzrange(start_at, end_at, '[]')
ORDER BY start_at;
-- name: SelectKnownAuthors :many
WITH matching AS (
  SELECT DISTINCT r.author_discord_id
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.author_discord_id <> ''
    AND lower(r.author) LIKE '%' || lower(@pattern::text) || '%'
)
SELECT m.author_discord_id::text AS author_discord_id,
  latest.author::text AS author
FROM matching m
  CROSS JOIN LATERAL (
    SELECT l.author
    FROM web_reservation l
    WHERE l.guild_id = @guild_id::text
      AND l.author_discord_id = m.author_discord_id
    ORDER BY l.end_at DESC
    LIMIT 1
  ) latest
ORDER BY lower(latest.author)
LIMIT 20;
