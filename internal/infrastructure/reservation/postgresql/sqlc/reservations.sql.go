// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.21.0
// source: reservations.sql

package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createReservation = `-- name: CreateReservation :one
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
RETURNING id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id
`

type CreateReservationParams struct {
	Author          string
	AuthorDiscordID string
	StartAt         pgtype.Timestamptz
	EndAt           pgtype.Timestamptz
	SpotID          int64
	GuildID         string
}

func (q *Queries) CreateReservation(ctx context.Context, arg CreateReservationParams) (WebReservation, error) {
	row := q.db.QueryRow(ctx, createReservation,
		arg.Author,
		arg.AuthorDiscordID,
		arg.StartAt,
		arg.EndAt,
		arg.SpotID,
		arg.GuildID,
	)
	var i WebReservation
	err := row.Scan(
		&i.ID,
		&i.Author,
		&i.CreatedAt,
		&i.StartAt,
		&i.EndAt,
		&i.SpotID,
		&i.GuildID,
		&i.AuthorDiscordID,
	)
	return i, err
}

const deletePresentMemberReservation = `-- name: DeletePresentMemberReservation :exec
DELETE FROM web_reservation
where web_reservation.guild_id = $1
  AND web_reservation.author_discord_id = $2
  AND web_reservation.id = $3
  AND web_reservation.end_at > now()
`

type DeletePresentMemberReservationParams struct {
	GuildID         string
	AuthorDiscordID string
	ID              int64
}

func (q *Queries) DeletePresentMemberReservation(ctx context.Context, arg DeletePresentMemberReservationParams) error {
	_, err := q.db.Exec(ctx, deletePresentMemberReservation, arg.GuildID, arg.AuthorDiscordID, arg.ID)
	return err
}

const deleteReservation = `-- name: DeleteReservation :exec
DELETE FROM web_reservation
WHERE web_reservation.id = $1
`

func (q *Queries) DeleteReservation(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, deleteReservation, id)
	return err
}

const selectOverlappingReservations = `-- name: SelectOverlappingReservations :many
SELECT web_reservation.id,
  web_reservation.author,
  web_reservation.author_discord_id,
  web_reservation.start_at,
  web_reservation.end_at,
  web_reservation.guild_id
FROM web_reservation
  INNER JOIN web_spot ON web_reservation.spot_id = web_spot.id
WHERE web_reservation.end_at >= now()
  AND tstzrange($1, $2, '[]') && tstzrange(
    web_reservation.start_at,
    web_reservation.end_at,
    '[]'
  )
  AND lower(web_spot.name) = lower($3)
  AND web_reservation.guild_id = $4
`

type SelectOverlappingReservationsParams struct {
	StartAt interface{}
	EndAt   interface{}
	Respawn string
	GuildID string
}

type SelectOverlappingReservationsRow struct {
	ID              int64
	Author          string
	AuthorDiscordID string
	StartAt         pgtype.Timestamptz
	EndAt           pgtype.Timestamptz
	GuildID         string
}

func (q *Queries) SelectOverlappingReservations(ctx context.Context, arg SelectOverlappingReservationsParams) ([]SelectOverlappingReservationsRow, error) {
	rows, err := q.db.Query(ctx, selectOverlappingReservations,
		arg.StartAt,
		arg.EndAt,
		arg.Respawn,
		arg.GuildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SelectOverlappingReservationsRow
	for rows.Next() {
		var i SelectOverlappingReservationsRow
		if err := rows.Scan(
			&i.ID,
			&i.Author,
			&i.AuthorDiscordID,
			&i.StartAt,
			&i.EndAt,
			&i.GuildID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const selectReservation = `-- name: SelectReservation :one
SELECT id, author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id
FROM web_reservation
WHERE id = $1
LIMIT 1
`

func (q *Queries) SelectReservation(ctx context.Context, id int64) (WebReservation, error) {
	row := q.db.QueryRow(ctx, selectReservation, id)
	var i WebReservation
	err := row.Scan(
		&i.ID,
		&i.Author,
		&i.CreatedAt,
		&i.StartAt,
		&i.EndAt,
		&i.SpotID,
		&i.GuildID,
		&i.AuthorDiscordID,
	)
	return i, err
}

const selectReservationWithSpot = `-- name: SelectReservationWithSpot :one
SELECT reservations.id, reservations.author, reservations.created_at, reservations.start_at, reservations.end_at, reservations.spot_id, reservations.guild_id, reservations.author_discord_id,
  spots.id, spots.name, spots.created_at
FROM web_reservation reservations
  JOIN web_spot spots ON spots.id = reservations.spot_id
WHERE reservations.id = $1
  and reservations.guild_id = $2
  AND reservations.author_discord_id = $3
LIMIT 1
`

type SelectReservationWithSpotParams struct {
	ID              int64
	GuildID         string
	AuthorDiscordID string
}

type SelectReservationWithSpotRow struct {
	WebReservation WebReservation
	WebSpot        WebSpot
}

func (q *Queries) SelectReservationWithSpot(ctx context.Context, arg SelectReservationWithSpotParams) (SelectReservationWithSpotRow, error) {
	row := q.db.QueryRow(ctx, selectReservationWithSpot, arg.ID, arg.GuildID, arg.AuthorDiscordID)
	var i SelectReservationWithSpotRow
	err := row.Scan(
		&i.WebReservation.ID,
		&i.WebReservation.Author,
		&i.WebReservation.CreatedAt,
		&i.WebReservation.StartAt,
		&i.WebReservation.EndAt,
		&i.WebReservation.SpotID,
		&i.WebReservation.GuildID,
		&i.WebReservation.AuthorDiscordID,
		&i.WebSpot.ID,
		&i.WebSpot.Name,
		&i.WebSpot.CreatedAt,
	)
	return i, err
}

const selectReservationsWithSpots = `-- name: SelectReservationsWithSpots :many
select web_spot.id, web_spot.name, web_spot.created_at,
  web_reservation.id, web_reservation.author, web_reservation.created_at, web_reservation.start_at, web_reservation.end_at, web_reservation.spot_id, web_reservation.guild_id, web_reservation.author_discord_id
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where end_at >= now()
  AND guild_id = $1
`

type SelectReservationsWithSpotsRow struct {
	WebSpot        WebSpot
	WebReservation WebReservation
}

func (q *Queries) SelectReservationsWithSpots(ctx context.Context, guildID string) ([]SelectReservationsWithSpotsRow, error) {
	rows, err := q.db.Query(ctx, selectReservationsWithSpots, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SelectReservationsWithSpotsRow
	for rows.Next() {
		var i SelectReservationsWithSpotsRow
		if err := rows.Scan(
			&i.WebSpot.ID,
			&i.WebSpot.Name,
			&i.WebSpot.CreatedAt,
			&i.WebReservation.ID,
			&i.WebReservation.Author,
			&i.WebReservation.CreatedAt,
			&i.WebReservation.StartAt,
			&i.WebReservation.EndAt,
			&i.WebReservation.SpotID,
			&i.WebReservation.GuildID,
			&i.WebReservation.AuthorDiscordID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const selectUpcomingMemberReservationsWithSpots = `-- name: SelectUpcomingMemberReservationsWithSpots :many
select web_spot.id, web_spot.name, web_spot.created_at,
  web_reservation.id, web_reservation.author, web_reservation.created_at, web_reservation.start_at, web_reservation.end_at, web_reservation.spot_id, web_reservation.guild_id, web_reservation.author_discord_id
from web_reservation
  inner join web_spot on web_reservation.spot_id = web_spot.id
where end_at >= now()
  AND guild_id = $1
  AND author_discord_id = $2
order by start_at asc
`

type SelectUpcomingMemberReservationsWithSpotsParams struct {
	GuildID         string
	AuthorDiscordID string
}

type SelectUpcomingMemberReservationsWithSpotsRow struct {
	WebSpot        WebSpot
	WebReservation WebReservation
}

func (q *Queries) SelectUpcomingMemberReservationsWithSpots(ctx context.Context, arg SelectUpcomingMemberReservationsWithSpotsParams) ([]SelectUpcomingMemberReservationsWithSpotsRow, error) {
	rows, err := q.db.Query(ctx, selectUpcomingMemberReservationsWithSpots, arg.GuildID, arg.AuthorDiscordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SelectUpcomingMemberReservationsWithSpotsRow
	for rows.Next() {
		var i SelectUpcomingMemberReservationsWithSpotsRow
		if err := rows.Scan(
			&i.WebSpot.ID,
			&i.WebSpot.Name,
			&i.WebSpot.CreatedAt,
			&i.WebReservation.ID,
			&i.WebReservation.Author,
			&i.WebReservation.CreatedAt,
			&i.WebReservation.StartAt,
			&i.WebReservation.EndAt,
			&i.WebReservation.SpotID,
			&i.WebReservation.GuildID,
			&i.WebReservation.AuthorDiscordID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
