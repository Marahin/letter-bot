// Package oauth adapts Discord's OAuth2 endpoints to ports.OAuthPort.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

const (
	apiBase     = "https://discord.com/api"
	authURL     = "https://discord.com/oauth2/authorize"
	tokenURL    = "https://discord.com/api/oauth2/token"
	permAdmin   = 0x8
	httpTimeout = 10 * time.Second
	// refreshMargin refreshes a token shortly before Discord would refuse it.
	refreshMargin = time.Minute
)

// Scopes are the OAuth scopes the panel asks for. guilds.members.read gives the
// user's roles in a guild without the privileged Server Members intent on the bot.
var Scopes = []string{"identify", "guilds", "guilds.members.read"}

// These run synchronously inside a request handler, so they stay short.
var defaultRetryDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond}

// A longer Retry-After (often a global rate limit) is not waited for inline.
const defaultMaxRetryAfter = 2 * time.Second

// Adapter implements ports.OAuthPort. It owns the tokens: Exchange stores them
// and every API call reads (and, when due, refreshes) them from the user store.
type Adapter struct {
	cfg     *oauth2.Config
	client  *http.Client
	apiBase string
	users   ports.WebUserRepository
	now     func() time.Time
	// refreshes holds one refresh per user: Discord rotates the refresh token, so
	// a second concurrent refresh with the old one gets invalid_grant.
	refreshes singleflight.Group

	retryDelays   []time.Duration
	maxRetryAfter time.Duration
}

func New(clientID, clientSecret, redirectURL string, users ports.WebUserRepository) *Adapter {
	return &Adapter{
		apiBase: apiBase,
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:   authURL,
				TokenURL:  tokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		client:        &http.Client{Timeout: httpTimeout},
		users:         users,
		now:           time.Now,
		retryDelays:   defaultRetryDelays,
		maxRetryAfter: defaultMaxRetryAfter,
	}
}

func (a *Adapter) AuthCodeURL(state string) string {
	return a.cfg.AuthCodeURL(state)
}

func (a *Adapter) Exchange(ctx context.Context, code string) (*webuser.User, error) {
	token, err := a.cfg.Exchange(a.clientContext(ctx), code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", tokenError(err))
	}

	var me struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Avatar     string `json:"avatar"`
	}
	if err := a.get(ctx, token.AccessToken, "/users/@me", &me); err != nil {
		return nil, fmt.Errorf("fetch current user: %w", err)
	}
	if me.ID == "" {
		return nil, errors.New("fetch current user: empty id")
	}

	// The user row must exist before the token can be stored on it.
	user, err := a.users.Upsert(ctx, webuser.User{DiscordUserID: me.ID, Username: me.Username, GlobalName: me.GlobalName, Avatar: me.Avatar})
	if err != nil {
		return nil, fmt.Errorf("store user: %w", err)
	}
	if err := a.users.SaveToken(ctx, me.ID, toStored(token)); err != nil {
		return nil, fmt.Errorf("store oauth token: %w", err)
	}
	return user, nil
}

func (a *Adapter) UserGuilds(ctx context.Context, userID string) ([]access.UserGuild, error) {
	accessToken, err := a.accessToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	var guilds []struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Icon        string   `json:"icon"`
		Owner       bool     `json:"owner"`
		Permissions permBits `json:"permissions"`
	}
	if err := a.get(ctx, accessToken, "/users/@me/guilds", &guilds); err != nil {
		return nil, fmt.Errorf("fetch guilds: %w", err)
	}

	out := make([]access.UserGuild, 0, len(guilds))
	for _, g := range guilds {
		out = append(out, access.UserGuild{
			ID:    g.ID,
			Name:  g.Name,
			Icon:  g.Icon,
			Admin: g.Owner || int64(g.Permissions)&permAdmin != 0,
		})
	}
	return out, nil
}

func (a *Adapter) UserGuildMember(ctx context.Context, userID, guildID string) (*access.GuildMember, error) {
	accessToken, err := a.accessToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	var m struct {
		Nick  string   `json:"nick"`
		Roles []string `json:"roles"`
		User  struct {
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
		} `json:"user"`
	}
	if err := a.get(ctx, accessToken, "/users/@me/guilds/"+url.PathEscape(guildID)+"/member", &m); err != nil {
		return nil, fmt.Errorf("fetch guild member: %w", err)
	}
	return &access.GuildMember{Nick: m.Nick, GlobalName: m.User.GlobalName, Username: m.User.Username, RoleIDs: m.Roles}, nil
}

// accessToken loads the user's token and refreshes it when it is about to expire.
func (a *Adapter) accessToken(ctx context.Context, userID string) (string, error) {
	stored, err := a.storedToken(ctx, userID)
	if err != nil {
		return "", err
	}
	if !a.due(stored) {
		return stored.AccessToken, nil
	}
	fresh, err, _ := a.refreshes.Do(userID, func() (any, error) {
		// Shared by every waiter, so one cancelled request must not fail the others.
		return a.refresh(context.WithoutCancel(ctx), userID)
	})
	if err != nil {
		return "", err
	}
	return fresh.(string), nil
}

func (a *Adapter) storedToken(ctx context.Context, userID string) (*webuser.Token, error) {
	stored, err := a.users.AccessToken(ctx, userID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, ports.ErrUnauthorized
	}
	if err != nil {
		return nil, fmt.Errorf("load oauth token: %w", err)
	}
	if stored.AccessToken == "" {
		return nil, ports.ErrUnauthorized
	}
	return stored, nil
}

func (a *Adapter) due(t *webuser.Token) bool {
	return t.Expiry != nil && !a.now().Add(refreshMargin).Before(*t.Expiry)
}

// refresh re-reads the stored token first: a refresh that finished between our
// read and winning the flight has already rotated it.
func (a *Adapter) refresh(ctx context.Context, userID string) (string, error) {
	stored, err := a.storedToken(ctx, userID)
	if err != nil {
		return "", err
	}
	if !a.due(stored) {
		return stored.AccessToken, nil
	}
	if stored.RefreshToken == "" {
		return "", ports.ErrUnauthorized
	}

	// An expired Expiry makes the token source refresh.
	fresh, err := a.cfg.TokenSource(a.clientContext(ctx), &oauth2.Token{
		RefreshToken: stored.RefreshToken,
		Expiry:       a.now().Add(-time.Second),
	}).Token()
	if err != nil {
		return "", fmt.Errorf("refresh oauth token: %w", tokenError(err))
	}
	if err := a.users.SaveToken(ctx, userID, toStored(fresh)); err != nil {
		return "", fmt.Errorf("store refreshed oauth token: %w", err)
	}
	return fresh.AccessToken, nil
}

func (a *Adapter) clientContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, a.client)
}

func toStored(t *oauth2.Token) webuser.Token {
	stored := webuser.Token{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken}
	if !t.Expiry.IsZero() {
		expiry := t.Expiry
		stored.Expiry = &expiry
	}
	return stored
}

// tokenError maps a token endpoint failure. A 4xx (invalid_grant, a revoked
// refresh token) means the user must sign in again. The error text of
// oauth2.RetrieveError carries the response body, which never holds a token.
func tokenError(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && re.Response != nil {
		if re.Response.StatusCode >= 400 && re.Response.StatusCode < 500 && re.Response.StatusCode != http.StatusTooManyRequests {
			return fmt.Errorf("%w: token endpoint returned %d", ports.ErrUnauthorized, re.Response.StatusCode)
		}
		return fmt.Errorf("%w: token endpoint returned %d", ports.ErrUpstreamUnavailable, re.Response.StatusCode)
	}
	return fmt.Errorf("%w: %w", ports.ErrUpstreamUnavailable, err)
}

// permBits is a Discord permission bitfield. The API has sent it both as a JSON
// string and as a bare number, so both are accepted.
type permBits int64

func (p *permBits) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*p = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parse permissions %q: %w", s, err)
	}
	*p = permBits(v)
	return nil
}

// get performs a GET against the Discord API. Transient failures (429, 5xx,
// transport errors) are retried on a short backoff and then wrap
// ports.ErrUpstreamUnavailable. A 401 wraps ports.ErrUnauthorized and a 404 wraps
// ports.ErrNotFound; neither is retried.
func (a *Adapter) get(ctx context.Context, accessToken, path string, dst any) error {
	for attempt := 0; ; attempt++ {
		err := a.getOnce(ctx, accessToken, path, dst)
		if err == nil {
			return nil
		}

		var ae *apiError
		isAPIError := errors.As(err, &ae)
		switch {
		case isAPIError && ae.status == http.StatusUnauthorized:
			return fmt.Errorf("%w: %w", ports.ErrUnauthorized, err)
		case isAPIError && ae.status == http.StatusNotFound:
			return fmt.Errorf("%w: %w", ports.ErrNotFound, err)
		case ctx.Err() != nil:
			return ctx.Err()
		}

		transient := retryable(err)
		if !transient {
			return err
		}
		if attempt >= len(a.retryDelays) {
			return fmt.Errorf("%w: %w", ports.ErrUpstreamUnavailable, err)
		}
		delay := a.retryDelays[attempt]
		if isAPIError && ae.retryAfter > 0 {
			if ae.retryAfter > a.maxRetryAfter {
				return fmt.Errorf("%w: %w", ports.ErrUpstreamUnavailable, err)
			}
			delay = ae.retryAfter
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (a *Adapter) getOnce(ctx context.Context, accessToken, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &apiError{
			status:     resp.StatusCode,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			msg:        fmt.Sprintf("discord API %s returned %d", path, resp.StatusCode),
		}
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

type apiError struct {
	status     int
	retryAfter time.Duration
	msg        string
}

func (e *apiError) Error() string { return e.msg }

func retryable(err error) bool {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.status == http.StatusTooManyRequests || ae.status >= 500
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return false
	}
	return true
}

// parseRetryAfter reads Discord's Retry-After header (seconds), 0 when absent or malformed.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	secs, err := strconv.ParseFloat(v, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}
