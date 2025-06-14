package onlinecheck

import (
	"errors"
	"spot-assistant/internal/core/dto/summary"
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
		worlds:  map[string]string{"guild1": "Celesta"},
		players: make(map[string][]string),
		log:     log,
	}
	// when
	err := a.RefreshOnlinePlayers("guild1")
	// then
	assert.NoError(t, err)
	assert.Equal(t, []string{"Mariysz", "Asar Cham"}, a.players["Celesta"])
}

func TestRefreshOnlinePlayers_Error(t *testing.T) {
	// given
	mockAPI := &MockAPI{
		err: errors.New("API failure"),
	}
	log := zaptest.NewLogger(t).Sugar()

	a := &Adapter{
		api:     mockAPI,
		worlds:  map[string]string{"guild1": "Celesta"},
		players: make(map[string][]string),
		log:     log,
	}
	// when
	err := a.RefreshOnlinePlayers("guild1")
	// then
	assert.Error(t, err)
	assert.Equal(t, "API failure", err.Error())
}

func TestIsOnline(t *testing.T) {
	// given
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		worlds:  map[string]string{"guild1": "Celesta"},
		players: map[string][]string{"Celesta": {"Mariysz", "Asar Cham"}},
		log:     log,
	}

	// then
	assert.True(t, a.IsOnline("guild1", "Mariysz"))
	assert.True(t, a.IsOnline("guild1", "Mariysz / Irnas"))
	assert.True(t, a.IsOnline("guild1", "Irnas / Mariysz"))
	assert.True(t, a.IsOnline("guild1", "Asar Cham / Irnas"))
	assert.True(t, a.IsOnline("guild1", "Asar Cham / Mariysz"))
	assert.True(t, a.IsOnline("guild1", "Mariysz / Asar Cham"))
	assert.True(t, a.IsOnline("guild1", "  Mariysz  /   Irnas  "))       // test spaces
	assert.True(t, a.IsOnline("guild1", "Kai Ens / Mariysz / Miodoelo")) // more than two names, one online

	assert.False(t, a.IsOnline("guild1", "Irnas"))
	assert.False(t, a.IsOnline("guild1", "Kai Ens / Miodoelo")) // both offline
}

func TestIsOnline_CaseSensitivity(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		worlds:  map[string]string{"guild1": "Celesta"},
		players: map[string][]string{"Celesta": {"Mariysz", "Asar Cham"}},
		log:     log,
	}
	assert.True(t, a.IsOnline("guild1", "mariysz"))
	assert.True(t, a.IsOnline("guild1", "ASAR CHAM"))
	assert.True(t, a.IsOnline("guild1", "Mariysz"))
	assert.True(t, a.IsOnline("guild1", "Asar Cham"))
	assert.True(t, a.IsOnline("guild1", "mariysz / asar cham"))
	assert.True(t, a.IsOnline("guild1", "ASAR CHAM / MARIYSZ"))
	assert.True(t, a.IsOnline("guild1", "Mariysz / ASAR CHAM"))
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
		worlds map[string]string
		expect bool
	}{
		{"all valid", mockAPI, map[string]string{"guild1": "Celesta"}, true},
		{"nil api", nil, map[string]string{"guild1": "Celesta"}, false},
		{"empty worlds", mockAPI, map[string]string{}, false},
		{"empty api url", mockAPIEmptyURL, map[string]string{"guild1": "Celesta"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				api:    tt.api,
				worlds: tt.worlds,
				log:    log,
			}
			assert.Equal(t, tt.expect, a.IsConfigured())
		})
	}
}

func TestConfigureWorldName(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		worlds:  make(map[string]string),
		players: make(map[string][]string),
		log:     log,
	}
	guildID := "guild1"
	world := "Celesta"
	a.ConfigureWorldName(guildID, world)
	assert.Equal(t, world, a.worlds[guildID])
}

func TestPlayerStatus(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		worlds:  map[string]string{"guild1": "Celesta"},
		players: map[string][]string{"Celesta": {"Mariysz"}},
		log:     log,
	}
	assert.Equal(t, summary.Online, a.PlayerStatus("guild1", "Mariysz"))
	assert.Equal(t, summary.Offline, a.PlayerStatus("guild1", "Unknown"))
}

func TestTryRefresh(t *testing.T) {
	mockAPI := &MockAPI{
		players: []string{"Mariysz"},
	}
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		api:     mockAPI,
		worlds:  map[string]string{"guild1": "Celesta"},
		players: make(map[string][]string),
		log:     log,
	}
	a.TryRefresh("guild1")
	assert.Equal(t, []string{"Mariysz"}, a.players["Celesta"])
}
