package onlinecheck

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

type MockAPI struct {
	players []string
	err     error
}

func (m *MockAPI) GetOnlinePlayerNames(world string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.players, nil
}

func (m *MockAPI) GetBaseURL() string {
	return "mock://baseurl"
}

func TestRefreshOnlinePlayers_Success(t *testing.T) {
	// given
	mockAPI := &MockAPI{
		players: []string{"Mariysz", "Asar Cham"},
	}
	log := zaptest.NewLogger(t).Sugar()

	a := &Adapter{
		api:     mockAPI,
		world:   "Celesta",
		log:     log,
		players: nil,
	}
	// when
	err := a.RefreshOnlinePlayers()
	// then
	assert.NoError(t, err)
	assert.Equal(t, []string{"Mariysz", "Asar Cham"}, a.players)
}

func TestRefreshOnlinePlayers_Error(t *testing.T) {
	// given
	mockAPI := &MockAPI{
		err: errors.New("API failure"),
	}
	log := zaptest.NewLogger(t).Sugar()

	a := &Adapter{
		api:   mockAPI,
		world: "Celesta",
		log:   log,
	}
	// when
	err := a.RefreshOnlinePlayers()
	// then
	assert.Error(t, err)
	assert.Equal(t, "API failure", err.Error())
}

func TestIsOnline(t *testing.T) {
	// given
	log := zaptest.NewLogger(t).Sugar()

	a := &Adapter{
		players: []string{"Mariysz", "Asar Cham"},
		log:     log,
	}

	// then
	assert.True(t, a.IsOnline("Mariysz"))
	assert.True(t, a.IsOnline("Mariysz / Irnas"))
	assert.True(t, a.IsOnline("Irnas / Mariysz"))
	assert.True(t, a.IsOnline("Asar Cham / Irnas"))
	assert.True(t, a.IsOnline("Asar Cham / Mariysz"))
	assert.True(t, a.IsOnline("Mariysz / Asar Cham"))
	assert.True(t, a.IsOnline("  Mariysz  /   Irnas  "))       // test spaces
	assert.True(t, a.IsOnline("Kai Ens / Mariysz / Miodoelo")) // more than two names, one online

	assert.False(t, a.IsOnline("Irnas"))
	assert.False(t, a.IsOnline("Kai Ens / Miodoelo")) // both offline
}

func TestIsOnline_CaseSensitivity(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		players: []string{"Mariysz", "Asar Cham"},
		log:     log,
	}
	assert.True(t, a.IsOnline("mariysz"))
	assert.True(t, a.IsOnline("ASAR CHAM"))
	assert.True(t, a.IsOnline("Mariysz"))
	assert.True(t, a.IsOnline("Asar Cham"))
	assert.True(t, a.IsOnline("mariysz / asar cham"))
	assert.True(t, a.IsOnline("ASAR CHAM / MARIYSZ"))
	assert.True(t, a.IsOnline("Mariysz / ASAR CHAM"))
}

type MockAPIEmptyURL struct {
	MockAPI
}

func (m *MockAPIEmptyURL) GetBaseURL() string {
	return ""
}

func TestIsConfigured(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()

	mockAPI := &MockAPI{players: []string{"A"}, err: nil}
	mockAPIEmptyURL := &MockAPIEmptyURL{}

	tests := []struct {
		name string
		api  interface {
			GetOnlinePlayerNames(string) ([]string, error)
			GetBaseURL() string
		}
		world  string
		expect bool
	}{
		{"all valid", mockAPI, "Celesta", true},
		{"nil api", nil, "Celesta", false},
		{"empty world", mockAPI, "", false},
		{"empty api url", mockAPIEmptyURL, "Celesta", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				api:   tt.api,
				world: tt.world,
				log:   log,
			}
			assert.Equal(t, tt.expect, a.IsConfigured())
		})
	}
}
