package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"spot-assistant/internal/common/errors"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

type DBTXWrapper interface {
	DBTX

	Begin(ctx context.Context) (pgx.Tx, error)
}

type ReservationRepository struct {
	q   *Queries
	db  DBTXWrapper
	log *zap.SugaredLogger
}

func NewReservationRepository(db DBTXWrapper) *ReservationRepository {
	return &ReservationRepository{
		q:  New(db),
		db: db,
	}
}

func (t *ReservationRepository) WithLogger(log *zap.SugaredLogger) *ReservationRepository {
	t.log = log.With(
		"layer", "infrastructure",
		"name", "ReservationRepository")

	return t
}

func (t *ReservationRepository) Find(ctx context.Context, id int64) (*reservation.Reservation, error) {
	res, err := t.q.SelectReservation(ctx, id)
	if err != nil {
		return nil, err
	}

	r := mapWebReservation(res)
	return &r, nil
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

	return mapReservationWithSpot(res.WebReservation, res.WebSpot), nil
}

func (t *ReservationRepository) SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectReservationsWithSpots(ctx, guildId)
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}

	reservationsWithSpots := make([]*reservation.ReservationWithSpot, len(res))
	for i, reservationWithSpotRow := range res {
		reservationsWithSpots[i] = mapReservationWithSpot(reservationWithSpotRow.WebReservation, reservationWithSpotRow.WebSpot)
	}

	return reservationsWithSpots, nil
}

func (t *ReservationRepository) SelectUpcomingReservationsWithSpotForSpot(ctx context.Context, guildId, spotName string) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectReservationsWithSpotsForSpot(ctx, SelectReservationsWithSpotsForSpotParams{
		GuildID: guildId,
		Lower:   spotName,
	})
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}
	reservationsWithSpots := make([]*reservation.ReservationWithSpot, len(res))
	for i, reservationWithSpotRow := range res {
		reservationsWithSpots[i] = mapReservationWithSpot(reservationWithSpotRow.WebReservation, reservationWithSpotRow.WebSpot)
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

func (t *ReservationRepository) CreateAndDeleteConflicting(ctx context.Context, member *member.Member, guild *guild.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.ClippedOrRemovedReservation, error) {
	modifiedConflicts := make([]*reservation.ClippedOrRemovedReservation, len(conflicts))
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return modifiedConflicts, err
	}
	defer errors.ExecuteAndIgnoreErrorF(tx.Rollback, ctx)
	qtx := t.q.WithTx(tx)

	for index, conflictingReservation := range conflicts {
		modifiedConflicts[index] = &reservation.ClippedOrRemovedReservation{
			Original: conflictingReservation,
			New:      []*reservation.Reservation{},
		}
		err = qtx.DeleteReservation(ctx, conflictingReservation.ID)
		if err != nil {
			return modifiedConflicts, err
		}

		if conflictingReservation.AuthorDiscordID != member.ID {
			createdLeftovers, err := t.createOverbookedLeftovers(ctx, qtx, conflictingReservation, spotId, startAt, endAt)
			if err != nil {
				return modifiedConflicts, err
			}

			for _, leftover := range createdLeftovers {
				modifiedConflicts[index].New = append(modifiedConflicts[index].New,
					&reservation.Reservation{
						ID:              leftover.ID,
						Author:          leftover.Author,
						CreatedAt:       leftover.CreatedAt.Time,
						StartAt:         leftover.StartAt.Time,
						EndAt:           leftover.EndAt.Time,
						SpotID:          leftover.SpotID,
						GuildID:         leftover.GuildID,
						AuthorDiscordID: leftover.AuthorDiscordID,
					},
				)
			}
		}
	}

	startAtInput := pgtype.Timestamptz{}
	err = startAtInput.Scan(startAt)
	if err != nil {
		return modifiedConflicts, err
	}

	endAtInput := pgtype.Timestamptz{}
	err = endAtInput.Scan(endAt)
	if err != nil {
		return modifiedConflicts, err
	}

	var author string
	if len(member.Nick) > 0 {
		author = member.Nick
	} else {
		author = member.Username
	}

	_, err = qtx.CreateReservation(ctx, CreateReservationParams{
		Author:          author,
		AuthorDiscordID: member.ID,
		StartAt:         startAtInput,
		EndAt:           endAtInput,
		SpotID:          spotId,
		GuildID:         guild.ID,
	})
	if err != nil {
		return modifiedConflicts, err
	}

	return modifiedConflicts, tx.Commit(ctx)
}

func (t *ReservationRepository) SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *guild.Guild, member *member.Member) ([]*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectUpcomingMemberReservationsWithSpots(ctx, SelectUpcomingMemberReservationsWithSpotsParams{
		GuildID:         guild.ID,
		AuthorDiscordID: member.ID,
	})
	if err != nil {
		return []*reservation.ReservationWithSpot{}, nil
	}

	reservations := make([]*reservation.ReservationWithSpot, len(res))
	for i, row := range res {
		reservations[i] = mapReservationWithSpot(row.WebReservation, row.WebSpot)
	}

	return reservations, nil
}

func (t *ReservationRepository) DeletePresentMemberReservation(ctx context.Context, g *guild.Guild, m *member.Member, reservationId int64) error {
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

// createOverbookedLeftovers creates up to two reservations from overbooked reservation leftovers.
// If overbooked reservation starts before new reservation, a reservation is created from overbooked reservation start time till new reservation start time.
// If overbooked reservation ends after new reservation, a reservation is created from new reservation end time till overbooked reservation end time.
func (t *ReservationRepository) createOverbookedLeftovers(
	ctx context.Context, qtx *Queries,
	overbookedReservation *reservation.Reservation, spotId int64,
	startAt time.Time, endAt time.Time) ([]WebReservation, error) {
	leftoverReservations := make([]WebReservation, 0, 2)
	if overbookedReservation.StartAt.Before(startAt) {
		// Create a reservation from overbooked reservation start time till new reservation start time
		startAtInput := pgtype.Timestamptz{}
		err := startAtInput.Scan(overbookedReservation.StartAt)
		if err != nil {
			return leftoverReservations, err
		}

		endAtInput := pgtype.Timestamptz{}
		err = endAtInput.Scan(startAt.Add(-1 * time.Minute))
		if err != nil {
			return leftoverReservations, err
		}

		newReservation, err := qtx.CreateReservation(ctx, CreateReservationParams{
			Author:          overbookedReservation.Author,
			AuthorDiscordID: overbookedReservation.AuthorDiscordID,
			StartAt:         startAtInput,
			EndAt:           endAtInput,
			SpotID:          spotId,
			GuildID:         overbookedReservation.GuildID,
		})
		if err != nil {
			return leftoverReservations, err
		}

		leftoverReservations = append(leftoverReservations, newReservation)
	}

	if overbookedReservation.EndAt.After(endAt) {
		// Create a reservation from overbooked reservation end time till overbooked reservation end time
		startAtInput := pgtype.Timestamptz{}
		err := startAtInput.Scan(endAt.Add(1 * time.Minute))
		if err != nil {
			return leftoverReservations, err
		}

		endAtInput := pgtype.Timestamptz{}
		err = endAtInput.Scan(overbookedReservation.EndAt)
		if err != nil {
			return leftoverReservations, err
		}

		newReservation, err := qtx.CreateReservation(ctx, CreateReservationParams{
			Author:          overbookedReservation.Author,
			AuthorDiscordID: overbookedReservation.AuthorDiscordID,
			StartAt:         startAtInput,
			EndAt:           endAtInput,
			SpotID:          spotId,
			GuildID:         overbookedReservation.GuildID,
		})
		if err != nil {
			return leftoverReservations, err
		}

		leftoverReservations = append(leftoverReservations, newReservation)
	}

	return leftoverReservations, nil
}

func mapWebReservation(res WebReservation) reservation.Reservation {
	return reservation.Reservation{
		ID:              res.ID,
		Author:          res.Author,
		CreatedAt:       res.CreatedAt.Time,
		StartAt:         res.StartAt.Time,
		EndAt:           res.EndAt.Time,
		SpotID:          res.SpotID,
		GuildID:         res.GuildID,
		AuthorDiscordID: res.AuthorDiscordID,
	}
}

func mapWebSpot(spot WebSpot) reservation.Spot {
	return reservation.Spot{
		ID:   spot.ID,
		Name: spot.Name,
	}
}

func mapReservationWithSpot(res WebReservation, spot WebSpot) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: mapWebReservation(res),
		Spot:        mapWebSpot(spot),
	}
}
