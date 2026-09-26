package worldapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/ports"
)

func serveFixture(t *testing.T, wantPath, fixture string, status int) *HttpWorldService {
	t.Helper()
	body := []byte{}
	if fixture != "" {
		var err error
		body, err = os.ReadFile("testdata/" + fixture)
		require.NoError(t, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, wantPath, r.URL.EscapedPath())
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return NewHttpWorldService(server.URL + "/v4")
}

func TestGetHighscoresPage_DecodesTibiaDataFixture(t *testing.T) {
	// given
	service := serveFixture(t, "/v4/highscores/Celesta/experience/all/1", "highscores_celesta_experience_1.json", http.StatusOK)

	// when
	page, err := service.GetHighscoresPage(context.Background(), "Celesta", 1)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Celesta", page.Highscores.World)
	assert.Equal(t, 24, page.Highscores.HighscoreAge)
	assert.Equal(t, 20, page.Highscores.HighscorePage.TotalPages)
	assert.Equal(t, 1000, page.Highscores.HighscorePage.TotalRecords)
	require.Len(t, page.Highscores.HighscoreList, 50)
	first := page.Highscores.HighscoreList[0]
	assert.Equal(t, "Elder Reno", first.Name)
	assert.Equal(t, 2744, first.Level)
	assert.Equal(t, int64(343769339496), first.Value)
	assert.Equal(t, "Elder Druid", first.Vocation)
	assert.Equal(t, time.Date(2026, 9, 26, 1, 4, 15, 0, time.UTC), page.Information.Timestamp)
}

func TestGetHighscoresPage_EscapesWorld(t *testing.T) {
	// given
	service := serveFixture(t, "/v4/highscores/Some%20World/experience/all/3", "highscores_celesta_experience_1.json", http.StatusOK)

	// when
	_, err := service.GetHighscoresPage(context.Background(), "Some World", 3)

	// then
	assert.NoError(t, err)
}

func TestGetHighscoresPage_Errors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		fixture     string
		upstreamErr bool
	}{
		{name: "server error", status: http.StatusBadGateway, upstreamErr: true},
		{name: "rate limited", status: http.StatusTooManyRequests, upstreamErr: true},
		{name: "bad request", status: http.StatusBadRequest},
		{name: "unknown world", status: http.StatusNotFound, fixture: "highscores_celesta_experience_1.json"},
		{name: "invalid json", status: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			service := serveFixture(t, "/v4/highscores/Celesta/experience/all/1", tt.fixture, tt.status)

			// when
			page, err := service.GetHighscoresPage(context.Background(), "Celesta", 1)

			// then
			require.Error(t, err)
			assert.Nil(t, page)
			assert.Equal(t, tt.upstreamErr, errors.Is(err, ports.ErrUpstreamUnavailable))
		})
	}
}

func TestGetHighscoresPage_TransportErrorIsUpstreamUnavailable(t *testing.T) {
	// given
	service := NewHttpWorldService("http://127.0.0.1:1/v4")

	// when
	_, err := service.GetHighscoresPage(context.Background(), "Celesta", 1)

	// then
	assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
}

func TestGetCharacter_DecodesTibiaDataFixture(t *testing.T) {
	// given
	service := serveFixture(t, "/v4/character/Elder%20Reno", "character_elder_reno.json", http.StatusOK)

	// when
	c, err := service.GetCharacter(context.Background(), "Elder Reno")

	// then
	require.NoError(t, err)
	assert.Equal(t, "Elder Reno", c.Name)
	assert.Equal(t, 2744, c.Level)
	assert.Equal(t, "Elder Druid", c.Vocation)
	assert.Equal(t, "Celesta", c.World)
	assert.Equal(t, "male", c.Sex)
	assert.Equal(t, "Premium Account", c.AccountStatus)
	assert.Equal(t, "Refugees", c.GuildName)
	assert.Equal(t, "Leader", c.GuildRank)
	assert.Equal(t, "Ankrahmun", c.Residence)
	assert.Equal(t, 734, c.AchievementPoints)
	require.NotNil(t, c.LastLogin)
	assert.Equal(t, time.Date(2026, 9, 26, 0, 43, 27, 0, time.UTC), *c.LastLogin)
}

func TestGetCharacter_NotFound(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "404 with json", status: http.StatusNotFound, body: `{"information":{"status":{"http_code":404,"error":20001,"message":"could not find character"}}}`},
		{name: "404 without json", status: http.StatusNotFound, body: "not found"},
		{name: "200 with empty character", status: http.StatusOK, body: `{"character":{"character":{"name":""}},"information":{"status":{"http_code":200}}}`},
		{name: "200 with status 404", status: http.StatusOK, body: `{"information":{"status":{"http_code":404}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)
			service := NewHttpWorldService(server.URL)

			// when
			c, err := service.GetCharacter(context.Background(), "Nobody Here")

			// then
			assert.Nil(t, c)
			assert.ErrorIs(t, err, ports.ErrCharacterNotFound)
			assert.ErrorIs(t, err, ports.ErrNotFound)
		})
	}
}

func TestGetCharacter_UpstreamErrors(t *testing.T) {
	// given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	service := NewHttpWorldService(server.URL)
	broken := serveFixture(t, "/v4/character/X", "", http.StatusOK)

	// when
	_, err := service.GetCharacter(context.Background(), "Elder Reno")
	_, decodeErr := broken.GetCharacter(context.Background(), "X")

	// then
	assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
	assert.NotErrorIs(t, err, ports.ErrNotFound)
	assert.Error(t, decodeErr)
	assert.NotErrorIs(t, decodeErr, ports.ErrNotFound)
}
