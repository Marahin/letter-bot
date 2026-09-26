package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

// fakeDiscord serves the token endpoint at /oauth2/token and the API under /api.
type fakeDiscord struct {
	*httptest.Server
	tokenCalls atomic.Int32
	lastGrant  atomic.Value
	token      func(w http.ResponseWriter, form url.Values)
	api        http.HandlerFunc
}

func newFakeDiscord(t *testing.T) *fakeDiscord {
	f := &fakeDiscord{}
	f.token = func(w http.ResponseWriter, _ url.Values) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","token_type":"Bearer","expires_in":604800}`))
	}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			f.tokenCalls.Add(1)
			require.NoError(t, r.ParseForm())
			f.lastGrant.Store(r.PostForm.Get("grant_type"))
			f.token(w, r.PostForm)
			return
		}
		r.URL.Path = r.URL.Path[len("/api"):]
		f.api(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func newAdapter(t *testing.T, f *fakeDiscord, users ports.WebUserRepository) *Adapter {
	a := New("client-1", "secret-1", "http://localhost:8080/auth/callback", users)
	a.apiBase = f.URL + "/api"
	a.cfg.Endpoint.TokenURL = f.URL + "/oauth2/token"
	a.retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	a.maxRetryAfter = 50 * time.Millisecond
	return a
}

// validToken makes the user store hand out "tok", valid for an hour.
func validToken(t *testing.T) *mocks.MockWebUserRepository {
	users := mocks.NewMockWebUserRepository(t)
	expiry := time.Now().Add(time.Hour)
	users.EXPECT().AccessToken(mock.Anything, "u1").Return(&webuser.Token{AccessToken: "tok", RefreshToken: "rt", Expiry: &expiry}, nil).Maybe()
	return users
}

func TestAuthCodeURL_IncludesStateAndScopes(t *testing.T) {
	// given
	a := New("client-1", "secret", "http://localhost/auth/callback", mocks.NewMockWebUserRepository(t))

	// when
	got, err := url.Parse(a.AuthCodeURL("state-xyz"))

	// then
	require.NoError(t, err)
	q := got.Query()
	assert.Equal(t, "discord.com", got.Host)
	assert.Equal(t, "client-1", q.Get("client_id"))
	assert.Equal(t, "state-xyz", q.Get("state"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "identify guilds guilds.members.read", q.Get("scope"))
	assert.Equal(t, "http://localhost/auth/callback", q.Get("redirect_uri"))
}

func TestExchange_StoresUserThenToken(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/@me", r.URL.Path)
		assert.Equal(t, "Bearer at-new", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"id":"u1","username":"nyx","global_name":"Quiet Nyx","avatar":"av"}`))
	}
	users := mocks.NewMockWebUserRepository(t)
	stored := &webuser.User{DiscordUserID: "u1", Username: "nyx", GlobalName: "Quiet Nyx", Avatar: "av"}
	upsert := users.EXPECT().Upsert(mock.Anything, webuser.User{DiscordUserID: "u1", Username: "nyx", GlobalName: "Quiet Nyx", Avatar: "av"}).Return(stored, nil)
	users.EXPECT().SaveToken(mock.Anything, "u1", mock.MatchedBy(func(tok webuser.Token) bool {
		return tok.AccessToken == "at-new" && tok.RefreshToken == "rt-new" && tok.Expiry != nil && tok.Expiry.After(time.Now().Add(6*24*time.Hour))
	})).Return(nil).NotBefore(upsert.Call)
	a := newAdapter(t, f, users)

	// when
	user, err := a.Exchange(context.Background(), "code-1")

	// then
	require.NoError(t, err)
	assert.Equal(t, stored, user)
	assert.Equal(t, "authorization_code", f.lastGrant.Load())
}

func TestExchange_RefusedCodeIsUnauthorized(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.token = func(w http.ResponseWriter, _ url.Values) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}
	a := newAdapter(t, f, mocks.NewMockWebUserRepository(t))

	// when
	user, err := a.Exchange(context.Background(), "bad")

	// then
	assert.Nil(t, user)
	assert.ErrorIs(t, err, ports.ErrUnauthorized)
}

func TestExchange_TokenEndpointDownIsUnavailable(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.token = func(w http.ResponseWriter, _ url.Values) { w.WriteHeader(http.StatusBadGateway) }
	a := newAdapter(t, f, mocks.NewMockWebUserRepository(t))

	// when
	_, err := a.Exchange(context.Background(), "code")

	// then
	assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
}

func TestExchange_StoreErrors(t *testing.T) {
	me := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"id":"u1","username":"nyx"}`)) }
	t.Run("empty id", func(t *testing.T) {
		// given
		f := newFakeDiscord(t)
		f.api = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }
		a := newAdapter(t, f, mocks.NewMockWebUserRepository(t))

		// when
		_, err := a.Exchange(context.Background(), "code")

		// then
		assert.ErrorContains(t, err, "empty id")
	})
	t.Run("upsert", func(t *testing.T) {
		// given
		f := newFakeDiscord(t)
		f.api = me
		users := mocks.NewMockWebUserRepository(t)
		users.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil, assert.AnError)
		a := newAdapter(t, f, users)

		// when
		_, err := a.Exchange(context.Background(), "code")

		// then
		assert.ErrorIs(t, err, assert.AnError)
	})
	t.Run("save token", func(t *testing.T) {
		// given
		f := newFakeDiscord(t)
		f.api = me
		users := mocks.NewMockWebUserRepository(t)
		users.EXPECT().Upsert(mock.Anything, mock.Anything).Return(&webuser.User{DiscordUserID: "u1"}, nil)
		users.EXPECT().SaveToken(mock.Anything, "u1", mock.Anything).Return(assert.AnError)
		a := newAdapter(t, f, users)

		// when
		_, err := a.Exchange(context.Background(), "code")

		// then
		assert.ErrorIs(t, err, assert.AnError)
	})
	t.Run("fetch me", func(t *testing.T) {
		// given
		f := newFakeDiscord(t)
		f.api = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }
		a := newAdapter(t, f, mocks.NewMockWebUserRepository(t))

		// when
		_, err := a.Exchange(context.Background(), "code")

		// then
		assert.ErrorIs(t, err, ports.ErrUnauthorized)
	})
}

func TestUserGuilds_MapsAdminFlag(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/@me/guilds", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`[
			{"id":"1","name":"Owned","icon":"i1","owner":true,"permissions":"0"},
			{"id":"2","name":"AdminPerm","owner":false,"permissions":"8"},
			{"id":"3","name":"Member","owner":false,"permissions":2048},
			{"id":"4","name":"NoPerms","owner":false,"permissions":null}
		]`))
	}
	a := newAdapter(t, f, validToken(t))

	// when
	guilds, err := a.UserGuilds(context.Background(), "u1")

	// then
	require.NoError(t, err)
	require.Len(t, guilds, 4)
	assert.True(t, guilds[0].Admin)
	assert.Equal(t, "i1", guilds[0].Icon)
	assert.True(t, guilds[1].Admin)
	assert.False(t, guilds[2].Admin)
	assert.False(t, guilds[3].Admin)
}

func TestUserGuildMember(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/@me/guilds/806152499760201738/member", r.URL.Path)
		_, _ = w.Write([]byte(`{"nick":"Nyx","roles":["r1","r2"],"user":{"username":"nyx","global_name":"Quiet"}}`))
	}
	a := newAdapter(t, f, validToken(t))

	// when
	m, err := a.UserGuildMember(context.Background(), "u1", "806152499760201738")

	// then
	require.NoError(t, err)
	assert.Equal(t, "Nyx", m.Nick)
	assert.Equal(t, "Quiet", m.GlobalName)
	assert.Equal(t, "nyx", m.Username)
	assert.Equal(t, []string{"r1", "r2"}, m.RoleIDs)
}

func TestGet_StatusMapping(t *testing.T) {
	cases := map[string]struct {
		status   int
		expected error
		calls    int32
	}{
		"401 is unauthorized, not retried": {http.StatusUnauthorized, ports.ErrUnauthorized, 1},
		"404 is not found, not retried":    {http.StatusNotFound, ports.ErrNotFound, 1},
		"5xx is retried, then unavailable": {http.StatusInternalServerError, ports.ErrUpstreamUnavailable, 3},
		"429 is retried, then unavailable": {http.StatusTooManyRequests, ports.ErrUpstreamUnavailable, 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			var calls atomic.Int32
			f := newFakeDiscord(t)
			f.api = func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
			}
			a := newAdapter(t, f, validToken(t))

			// when
			_, err := a.UserGuildMember(context.Background(), "u1", "g")

			// then
			assert.ErrorIs(t, err, tc.expected)
			assert.Equal(t, tc.calls, calls.Load())
		})
	}
}

func TestGet_ForbiddenIsNeitherRetriedNorMapped(t *testing.T) {
	// given
	var calls atomic.Int32
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}
	a := newAdapter(t, f, validToken(t))

	// when
	_, err := a.UserGuilds(context.Background(), "u1")

	// then
	require.Error(t, err)
	assert.NotErrorIs(t, err, ports.ErrUpstreamUnavailable)
	assert.Equal(t, int32(1), calls.Load())
}

func TestGet_RetriesThenSucceeds(t *testing.T) {
	// given
	var calls atomic.Int32
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}
	a := newAdapter(t, f, validToken(t))

	// when
	guilds, err := a.UserGuilds(context.Background(), "u1")

	// then
	require.NoError(t, err)
	assert.Empty(t, guilds)
	assert.Equal(t, int32(2), calls.Load())
}

func TestGet_RetryAfterBeyondCapBailsImmediately(t *testing.T) {
	// given
	var calls atomic.Int32
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}
	a := newAdapter(t, f, validToken(t))

	// when
	_, err := a.UserGuilds(context.Background(), "u1")

	// then
	assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
	assert.Equal(t, int32(1), calls.Load())
}

func TestGet_MalformedBodyIsNotRetried(t *testing.T) {
	// given
	var calls atomic.Int32
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{not json`))
	}
	a := newAdapter(t, f, validToken(t))

	// when
	_, err := a.UserGuilds(context.Background(), "u1")

	// then
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
}

func TestGet_CancelledContext(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }
	a := newAdapter(t, f, validToken(t))
	a.retryDelays = []time.Duration{time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	// when
	_, err := a.UserGuilds(ctx, "u1")

	// then
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAccessToken_Refresh(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer at-new", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`[]`))
	}
	var refreshToken string
	f.token = func(w http.ResponseWriter, form url.Values) {
		refreshToken = form.Get("refresh_token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-new", "refresh_token": "rt-new", "token_type": "Bearer", "expires_in": 3600})
	}
	users := mocks.NewMockWebUserRepository(t)
	soon := time.Now().Add(30 * time.Second)
	users.EXPECT().AccessToken(mock.Anything, "u1").Return(&webuser.Token{AccessToken: "old", RefreshToken: "rt-old", Expiry: &soon}, nil)
	users.EXPECT().SaveToken(mock.Anything, "u1", mock.MatchedBy(func(tok webuser.Token) bool {
		return tok.AccessToken == "at-new" && tok.RefreshToken == "rt-new"
	})).Return(nil)
	a := newAdapter(t, f, users)

	// when
	_, err := a.UserGuilds(context.Background(), "u1")

	// then
	require.NoError(t, err)
	assert.Equal(t, "refresh_token", f.lastGrant.Load())
	assert.Equal(t, "rt-old", refreshToken)
}

func TestAccessToken_Failures(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cases := map[string]struct {
		token    *webuser.Token
		err      error
		refresh  func(w http.ResponseWriter, _ url.Values)
		save     error
		expected error
	}{
		"unknown user":              {err: ports.ErrNotFound, expected: ports.ErrUnauthorized},
		"store error":               {err: assert.AnError, expected: assert.AnError},
		"empty token":               {token: &webuser.Token{}, expected: ports.ErrUnauthorized},
		"expired, no refresh":       {token: &webuser.Token{AccessToken: "a", Expiry: &past}, expected: ports.ErrUnauthorized},
		"refresh token revoked":     {token: &webuser.Token{AccessToken: "a", RefreshToken: "r", Expiry: &past}, refresh: func(w http.ResponseWriter, _ url.Values) { w.WriteHeader(http.StatusBadRequest) }, expected: ports.ErrUnauthorized},
		"refreshed token not saved": {token: &webuser.Token{AccessToken: "a", RefreshToken: "r", Expiry: &past}, save: assert.AnError, expected: assert.AnError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newFakeDiscord(t)
			f.api = func(w http.ResponseWriter, _ *http.Request) { t.Error("the API must not be called") }
			if tc.refresh != nil {
				f.token = tc.refresh
			}
			users := mocks.NewMockWebUserRepository(t)
			users.EXPECT().AccessToken(mock.Anything, "u1").Return(tc.token, tc.err)
			if tc.save != nil {
				users.EXPECT().SaveToken(mock.Anything, "u1", mock.Anything).Return(tc.save)
			}
			a := newAdapter(t, f, users)

			// when
			_, errGuilds := a.UserGuilds(context.Background(), "u1")

			// then
			assert.ErrorIs(t, errGuilds, tc.expected)
		})
	}
}

func TestAccessToken_WithoutExpiryIsUsedAsIs(t *testing.T) {
	// given
	f := newFakeDiscord(t)
	f.api = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer forever", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"roles":[]}`))
	}
	users := mocks.NewMockWebUserRepository(t)
	users.EXPECT().AccessToken(mock.Anything, "u1").Return(&webuser.Token{AccessToken: "forever"}, nil)
	a := newAdapter(t, f, users)

	// when
	_, err := a.UserGuildMember(context.Background(), "u1", "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, int32(0), f.tokenCalls.Load())
}

func TestParseRetryAfter(t *testing.T) {
	assert.Equal(t, time.Duration(0), parseRetryAfter(""))
	assert.Equal(t, time.Duration(0), parseRetryAfter("soon"))
	assert.Equal(t, time.Duration(0), parseRetryAfter("-1"))
	assert.Equal(t, 1500*time.Millisecond, parseRetryAfter("1.5"))
}
