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
	"spot-assistant/internal/infrastructure/db/postgresql"
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

func (t *ReservationRepository) SelectOverlappingReservations(ctx context.Context, spotID int64, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error) {
	res, err := t.q.SelectOverlappingReservations(ctx, SelectOverlappingReservationsParams{
		StartAt: startAt,
		EndAt:   endAt,
		SpotID:  spotID,
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

func (t *ReservationRepository) SearchReservationsWithSpot(ctx context.Context, filter reservation.SearchFilter) ([]*reservation.ReservationWithSpot, error) {
	f := mapSearchFilter(filter)
	res, err := t.q.SearchReservationsWithSpot(ctx, SearchReservationsWithSpotParams{
		GuildID:         f.GuildID,
		SpotID:          f.SpotID,
		Author:          f.Author,
		AuthorDiscordID: f.AuthorDiscordID,
		FromAt:          f.FromAt,
		ToAt:            f.ToAt,
		Scope:           f.Scope,
		RowLimit:        int32(filter.Limit),
		RowOffset:       int32(filter.Offset),
	})
	if err != nil {
		return []*reservation.ReservationWithSpot{}, err
	}

	reservations := make([]*reservation.ReservationWithSpot, len(res))
	for i, row := range res {
		reservations[i] = mapReservationWithSpot(row.WebReservation, row.WebSpot)
	}

	return reservations, nil
}

func (t *ReservationRepository) CountReservations(ctx context.Context, filter reservation.SearchFilter) (int64, error) {
	return t.q.CountReservations(ctx, mapSearchFilter(filter))
}

func (t *ReservationRepository) SelectGuildReservationWithSpot(ctx context.Context, guildID string, id int64) (*reservation.ReservationWithSpot, error) {
	res, err := t.q.SelectGuildReservationWithSpot(ctx, SelectGuildReservationWithSpotParams{ID: id, GuildID: guildID})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return mapReservationWithSpot(res.WebReservation, res.WebSpot), nil
}

func (t *ReservationRepository) UpdateReservation(ctx context.Context, guildID string, r reservation.Reservation) error {
	return postgresql.RowsAffected(t.q.UpdateReservation(ctx, UpdateReservationParams{
		ID:              r.ID,
		GuildID:         guildID,
		SpotID:          r.SpotID,
		StartAt:         pgtype.Timestamptz{Time: r.StartAt, Valid: true},
		EndAt:           pgtype.Timestamptz{Time: r.EndAt, Valid: true},
		Author:          r.Author,
		AuthorDiscordID: r.AuthorDiscordID,
	}))
}

func (t *ReservationRepository) DeleteGuildReservation(ctx context.Context, guildID string, id int64) error {
	return postgresql.RowsAffected(t.q.DeleteGuildReservation(ctx, DeleteGuildReservationParams{ID: id, GuildID: guildID}))
}

func (t *ReservationRepository) SelectOverlappingReservationsBySpotID(ctx context.Context, guildID string, spotID int64, startAt time.Time, endAt time.Time, excludeID int64) ([]*reservation.Reservation, error) {
	res, err := t.q.SelectOverlappingReservationsBySpotID(ctx, SelectOverlappingReservationsBySpotIDParams{
		SpotID:    spotID,
		GuildID:   guildID,
		ExcludeID: excludeID,
		StartAt:   pgtype.Timestamptz{Time: startAt, Valid: true},
		EndAt:     pgtype.Timestamptz{Time: endAt, Valid: true},
	})
	if err != nil {
		return []*reservation.Reservation{}, err
	}

	reservations := make([]*reservation.Reservation, len(res))
	for i, row := range res {
		r := mapWebReservation(row)
		reservations[i] = &r
	}

	return reservations, nil
}

func (t *ReservationRepository) SelectKnownAuthors(ctx context.Context, guildID string, pattern string) ([]*reservation.KnownAuthor, error) {
	res, err := t.q.SelectKnownAuthors(ctx, SelectKnownAuthorsParams{GuildID: guildID, Pattern: pattern})
	if err != nil {
		return []*reservation.KnownAuthor{}, err
	}

	authors := make([]*reservation.KnownAuthor, len(res))
	for i, row := range res {
		authors[i] = &reservation.KnownAuthor{AuthorDiscordID: row.AuthorDiscordID, Author: row.Author}
	}

	return authors, nil
}

func mapSearchFilter(filter reservation.SearchFilter) CountReservationsParams {
	params := CountReservationsParams{
		GuildID: filter.GuildID,
		Scope:   string(reservation.ScopeAll),
	}
	if filter.Scope != "" {
		params.Scope = string(filter.Scope)
	}
	if filter.SpotID != nil {
		params.SpotID = pgtype.Int8{Int64: *filter.SpotID, Valid: true}
	}
	if filter.Author != "" {
		params.Author = pgtype.Text{String: filter.Author, Valid: true}
	}
	if filter.AuthorDiscordID != "" {
		params.AuthorDiscordID = pgtype.Text{String: filter.AuthorDiscordID, Valid: true}
	}
	if filter.From != nil {
		params.FromAt = pgtype.Timestamptz{Time: *filter.From, Valid: true}
	}
	if filter.To != nil {
		params.ToAt = pgtype.Timestamptz{Time: *filter.To, Valid: true}
	}

	return params
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
