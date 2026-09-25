package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

type WebUserRepository struct {
	q *Queries
}

func NewWebUserRepository(db DBTX) *WebUserRepository {
	return &WebUserRepository{q: New(db)}
}

func (r *WebUserRepository) Upsert(ctx context.Context, user webuser.User) (*webuser.User, error) {
	res, err := r.q.UpsertWebUser(ctx, UpsertWebUserParams{
		DiscordUserID: user.DiscordUserID,
		Username:      user.Username,
		GlobalName:    user.GlobalName,
		Avatar:        user.Avatar,
	})
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return &webuser.User{
		DiscordUserID: res.DiscordUserID,
		Username:      res.Username,
		GlobalName:    res.GlobalName,
		Avatar:        res.Avatar,
		CreatedAt:     res.CreatedAt.Time,
		UpdatedAt:     res.UpdatedAt.Time,
	}, nil
}

func (r *WebUserRepository) Get(ctx context.Context, discordUserID string) (*webuser.User, error) {
	res, err := r.q.SelectWebUser(ctx, discordUserID)
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	return &webuser.User{
		DiscordUserID: res.DiscordUserID,
		Username:      res.Username,
		GlobalName:    res.GlobalName,
		Avatar:        res.Avatar,
		CreatedAt:     res.CreatedAt.Time,
		UpdatedAt:     res.UpdatedAt.Time,
	}, nil
}

func (r *WebUserRepository) SaveToken(ctx context.Context, discordUserID string, token webuser.Token) error {
	expiry := pgtype.Timestamptz{}
	if token.Expiry != nil {
		expiry = pgtype.Timestamptz{Time: *token.Expiry, Valid: true}
	}

	return postgresql.RowsAffected(r.q.SaveWebUserToken(ctx, SaveWebUserTokenParams{
		AccessToken:   token.AccessToken,
		RefreshToken:  token.RefreshToken,
		TokenExpiry:   expiry,
		DiscordUserID: discordUserID,
	}))
}

func (r *WebUserRepository) AccessToken(ctx context.Context, discordUserID string) (*webuser.Token, error) {
	res, err := r.q.SelectWebUserToken(ctx, discordUserID)
	if err != nil {
		return nil, postgresql.MapError(err)
	}

	token := &webuser.Token{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken}
	if res.TokenExpiry.Valid {
		expiry := res.TokenExpiry.Time
		token.Expiry = &expiry
	}

	return token, nil
}
