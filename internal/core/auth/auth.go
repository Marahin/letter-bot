// Package auth is the Discord sign-in flow of the web panel.
package auth

import (
	"context"
	"errors"
	"fmt"

	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

// Service implements ports.AuthService.
type Service struct {
	oauth ports.OAuthPort
	users ports.WebUserRepository
}

func New(oauth ports.OAuthPort, users ports.WebUserRepository) *Service {
	return &Service{oauth: oauth, users: users}
}

// LoginURL returns the Discord authorize URL for the given CSRF state.
func (s *Service) LoginURL(state string) string {
	return s.oauth.AuthCodeURL(state)
}

// Complete exchanges the OAuth code. The adapter stores the user and the tokens.
func (s *Service) Complete(ctx context.Context, code string) (*webuser.User, error) {
	user, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth code: %w", err)
	}
	return user, nil
}

func (s *Service) User(ctx context.Context, userID string) (*webuser.User, error) {
	return s.users.Get(ctx, userID)
}

// Logout clears the stored token, so it is no longer used on the user's behalf.
// The user's other sessions must sign in again once their cached Discord data expires.
func (s *Service) Logout(ctx context.Context, userID string) error {
	err := s.users.SaveToken(ctx, userID, webuser.Token{})
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	return err
}
