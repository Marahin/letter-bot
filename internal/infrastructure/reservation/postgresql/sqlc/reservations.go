package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

type ReservationRepository struct {
	q   *Queries
	db  DBTXWrapper
	log *logrus.Entry
}

func NewReservationRepository(db DBTXWrapper) *ReservationRepository {
	return &ReservationRepository{
		q:   New(db),
		db:  db,
		log: logrus.WithFields(logrus.Fields{"type": "infra", "name": "ReservationRepository"}),
	}
}

func (t *ReservationRepository) Find(ctx context.Context, id int64) (*reservation.Reservation, error) {
	res, err := t.q.SelectReservation(ctx, id)
	if err != nil {
		return nil, err
	}

	return &reservation.Reservation{
		ID:              res.ID,
		Author:          res.Author,
		CreatedAt:       res.CreatedAt.Time,
		StartAt:         res.StartAt.Time,
		EndAt:           res.EndAt.Time,
		SpotID:          res.SpotID,
		GuildID:         res.GuildID,
		AuthorDiscordID: res.AuthorDiscordID,
	}, nil
}

func (t *ReservationRepository) FindReservationWithSpot(ctx context.Context, id int64, guildID, authorDiscordID string) (*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectReservationWithSpot(ctx, SelectReservationWithSpotParams{
		ID:              id,
		GuildID:         guildID,
		AuthorDiscordID: authorDiscordID,
	})

	if err != nil {
		return nil, err
	}

	return &reservation.ReservationWithSpot{
		Spot: reservation.Spot{
			ID:   res.WebSpot.ID,
			Name: res.WebSpot.Name,
		},
		Reservation: reservation.Reservation{
			ID:              res.WebReservation.ID,
			Author:          res.WebReservation.Author,
			AuthorDiscordID: res.WebReservation.AuthorDiscordID,
			CreatedAt:       res.WebReservation.CreatedAt.Time,
			StartAt:         res.WebReservation.StartAt.Time,
			EndAt:           res.WebReservation.EndAt.Time,
			SpotID:          res.WebReservation.SpotID,
			GuildID:         res.WebReservation.GuildID,
		},
	}, nil
}

func (t *ReservationRepository) SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectReservationsWithSpots(ctx, guildId)
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}

	reservationsWithSpots := make([]*reservation.ReservationWithSpot, len(res))
	for i, reservationWithSpotRow := range res {
		mappedRes := &reservation.ReservationWithSpot{
			Reservation: reservation.Reservation{
				ID:              reservationWithSpotRow.WebReservation.ID,
				Author:          reservationWithSpotRow.WebReservation.Author,
				CreatedAt:       reservationWithSpotRow.WebReservation.CreatedAt.Time,
				StartAt:         reservationWithSpotRow.WebReservation.StartAt.Time,
				EndAt:           reservationWithSpotRow.WebReservation.EndAt.Time,
				SpotID:          reservationWithSpotRow.WebReservation.SpotID,
				GuildID:         reservationWithSpotRow.WebReservation.GuildID,
				AuthorDiscordID: reservationWithSpotRow.WebReservation.AuthorDiscordID,
			},
			Spot: reservation.Spot{
				ID:   reservationWithSpotRow.WebSpot.ID,
				Name: reservationWithSpotRow.WebSpot.Name,
			},
		}

		reservationsWithSpots[i] = mappedRes
	}

	return reservationsWithSpots, nil
}

func (t *ReservationRepository) SelectOverlappingReservations(ctx context.Context, spot string, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error) {
	res, err := t.q.SelectOverlappingReservations(ctx, SelectOverlappingReservationsParams{
		StartAt: startAt,
		EndAt:   endAt,
		Respawn: spot,
		GuildID: guildId,
	})
	if err != nil {
		return []*reservation.Reservation{}, err
	}

	reservations := make([]*reservation.Reservation, len(res))
	for i, row := range res {
		reservations[i] = &reservation.Reservation{
			ID:              row.ID,
			Author:          row.Author,
			AuthorDiscordID: row.AuthorDiscordID,
			StartAt:         row.StartAt.Time,
			EndAt:           row.EndAt.Time,
			GuildID:         row.GuildID,
		}
	}

	return reservations, nil
}

func (t *ReservationRepository) CreateAndDeleteConflicting(ctx context.Context, member *discord.Member, guild *discord.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.Reservation, error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return []*reservation.Reservation{}, err
	}
	defer errors.ExecuteAndIgnoreErrorF(tx.Rollback, ctx)
	qtx := t.q.WithTx(tx)

	for _, reservation := range conflicts {
		err = qtx.DeleteReservation(ctx, reservation.ID)
		if err != nil {
			return conflicts, err
		}

		if reservation.AuthorDiscordID != member.ID {
			err = t.createOverbookedLeftovers(ctx, qtx, reservation, spotId, startAt, endAt)
			if err != nil {
				return conflicts, err
			}
		}
	}

	startAtInput := pgtype.Timestamptz{}
	err = startAtInput.Scan(startAt)
	if err != nil {
		return []*reservation.Reservation{}, err
	}

	endAtInput := pgtype.Timestamptz{}
	err = endAtInput.Scan(endAt)
	if err != nil {
		return []*reservation.Reservation{}, err
	}

	var author string
	if len(member.Nick) > 0 {
		author = member.Nick
	} else {
		author = member.Username
	}

	err = qtx.CreateReservation(ctx, CreateReservationParams{
		Author:          author,
		AuthorDiscordID: member.ID,
		StartAt:         startAtInput,
		EndAt:           endAtInput,
		SpotID:          spotId,
		GuildID:         guild.ID,
	})
	if err != nil {
		return []*reservation.Reservation{}, err
	}

	return conflicts, tx.Commit(ctx)
}

func (t *ReservationRepository) SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *discord.Guild, member *discord.Member) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectUpcomingMemberReservationsWithSpots(ctx, SelectUpcomingMemberReservationsWithSpotsParams{
		GuildID:         guild.ID,
		AuthorDiscordID: member.ID,
	})
	if err != nil {
		return []*reservation.ReservationWithSpot{}, nil
	}

	reservations := make([]*reservation.ReservationWithSpot, len(res))
	for i, row := range res {
		reservations[i] = &reservation.ReservationWithSpot{
			Spot: reservation.Spot{
				ID:   row.WebSpot.ID,
				Name: row.WebSpot.Name,
			},
			Reservation: reservation.Reservation{
				ID:              row.WebReservation.ID,
				Author:          row.WebReservation.Author,
				AuthorDiscordID: row.WebReservation.AuthorDiscordID,
				CreatedAt:       row.WebReservation.CreatedAt.Time,
				StartAt:         row.WebReservation.StartAt.Time,
				EndAt:           row.WebReservation.EndAt.Time,
				SpotID:          row.WebReservation.SpotID,
				GuildID:         row.WebReservation.GuildID,
			},
		}
	}

	return reservations, nil
}

func (t *ReservationRepository) SelectAllReservationsWithSpotsBySpotNames(ctx context.Context, guildId string, spotNames []string) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectAllReservationsWithSpotsBySpotNames(ctx, SelectAllReservationsWithSpotsBySpotNamesParams{
		GuildID:   guildId,
		SpotNames: spotNames,
	})
	if err != nil {
		return nil, err
	}

	reservations := make([]*reservation.ReservationWithSpot, len(res))
	for i, row := range res {
		reservations[i] = &reservation.ReservationWithSpot{
			Spot: reservation.Spot{
				ID:   row.WebSpot.ID,
				Name: row.WebSpot.Name,
			},
			Reservation: reservation.Reservation{
				ID:              row.WebReservation.ID,
				Author:          row.WebReservation.Author,
				AuthorDiscordID: row.WebReservation.AuthorDiscordID,
				CreatedAt:       row.WebReservation.CreatedAt.Time,
				StartAt:         row.WebReservation.StartAt.Time,
				EndAt:           row.WebReservation.EndAt.Time,
				SpotID:          row.WebReservation.SpotID,
				GuildID:         row.WebReservation.GuildID,
			},
		}
	}

	return reservations, nil
}

func (t *ReservationRepository) DeletePresentMemberReservation(ctx context.Context, g *discord.Guild, m *discord.Member, reservationId int64) error {
	err := t.q.DeletePresentMemberReservation(ctx, DeletePresentMemberReservationParams{
		GuildID:         g.ID,
		AuthorDiscordID: m.ID,
		ID:              reservationId,
	})
	if err != nil {
		return err
	}

	return nil
}

func (t *ReservationRepository) createOverbookedLeftovers(ctx context.Context, qtx *Queries, overbookedReservation *reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) error {
	if overbookedReservation.StartAt.Before(startAt) {
		// Create a reservation from overbooked reservation start time till new reservation start time
		startAtInput := pgtype.Timestamptz{}
		err := startAtInput.Scan(overbookedReservation.StartAt)
		if err != nil {
			return err
		}

		endAtInput := pgtype.Timestamptz{}
		err = endAtInput.Scan(startAt.Add(-1 * time.Minute))
		if err != nil {
			return err
		}

		err = qtx.CreateReservation(ctx, CreateReservationParams{
			Author:          overbookedReservation.Author,
			AuthorDiscordID: overbookedReservation.AuthorDiscordID,
			StartAt:         startAtInput,
			EndAt:           endAtInput,
			SpotID:          spotId,
			GuildID:         overbookedReservation.GuildID,
		})

		if err != nil {
			return err
		}
	}

	if overbookedReservation.EndAt.After(endAt) {
		// Create a reservation from overbooked reservation end time till overbooked reservation end time
		startAtInput := pgtype.Timestamptz{}
		err := startAtInput.Scan(endAt.Add(1 * time.Minute))
		if err != nil {
			return err
		}

		endAtInput := pgtype.Timestamptz{}
		err = endAtInput.Scan(overbookedReservation.EndAt)
		if err != nil {
			return err
		}

		err = qtx.CreateReservation(ctx, CreateReservationParams{
			Author:          overbookedReservation.Author,
			AuthorDiscordID: overbookedReservation.AuthorDiscordID,
			StartAt:         startAtInput,
			EndAt:           endAtInput,
			SpotID:          spotId,
			GuildID:         overbookedReservation.GuildID,
		})

		return err
	}

	return nil
}
