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
where end_at >= now()
  AND guild_id = @guild_id
  AND author_discord_id = @author_discord_id
order by start_at asc;
-- name: SelectOverlappingReservations :many
SELECT web_reservation.id,
  web_reservation.author,
  web_reservation.author_discord_id,
  web_reservation.start_at,
  web_reservation.end_at,
  web_reservation.guild_id
FROM web_reservation
  INNER JOIN web_spot ON web_reservation.spot_id = web_spot.id
WHERE web_reservation.end_at >= now()
  AND tstzrange(@start_at, @end_at, '[]') && tstzrange(
    web_reservation.start_at,
    web_reservation.end_at,
    '[]'
  )
  AND lower(web_spot.name) = lower(@respawn)
  AND web_reservation.guild_id = @guild_id;
-- name: CreateReservation :exec
INSERT INTO web_reservation (
    author,
    author_discord_id,
    start_at,
    end_at,
    spot_id,
    created_at,
    guild_id
  )
VALUES ($1, $2, $3, $4, $5, now(), $6);
-- name: SelectReservationsWithSpots :many
select sqlc.embed(web_spot),
  sqlc.embed(web_reservation)
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where end_at >= now()
  AND guild_id = $1;
-- name: DeleteReservation :exec
DELETE FROM web_reservation
WHERE web_reservation.id = $1;
-- name: SelectAllReservationsWithSpotsBySpotNames :many
select sqlc.embed(web_spot),
       sqlc.embed(web_reservation)
from web_reservation
         inner join web_spot on web_reservation.spot_id = web_spot.id
where end_at >= now()
  AND guild_id = $1
  AND web_spot.name = ANY(@spot_names::text[]);