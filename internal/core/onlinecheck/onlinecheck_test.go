package onlinecheck

import (
	"errors"
	"testing"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/guildsworld"
	"spot-assistant/internal/core/dto/summary"

	cmap "github.com/orcaman/concurrent-map/v2"
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
		api:            mockAPI,
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
	// when
	err := a.RefreshOnlinePlayers("guild1")
	// then
	players, ok := a.players.Get("Celesta")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"Mariysz", "Asar Cham"}, players)
}

func TestRefreshOnlinePlayers_Error(t *testing.T) {
	// given
	mockAPI := &MockAPI{
		err: errors.New("API failure"),
	}
	log := zaptest.NewLogger(t).Sugar()

	a := &Adapter{
		api:            mockAPI,
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
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
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
	a.players.Set("Celesta", []string{"Mariysz", "Asar Cham"})

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

	// test missing world
	a2 := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a2.players.Set("Celesta", []string{"Mariysz", "Asar Cham"})
	assert.False(t, a2.IsOnline("guild1", "Mariysz"))

	// test missing players
	a3 := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a3.guildIdToWorld.Set("guild1", "Celesta")
	assert.False(t, a3.IsOnline("guild1", "Mariysz"))
}

func TestIsOnline_CaseSensitivity(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
	a.players.Set("Celesta", []string{"Mariysz", "Asar Cham"})
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
		expect bool
	}{
		{"all valid", mockAPI, true},
		{"nil api", nil, false},
		{"empty api url", mockAPIEmptyURL, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				api:            tt.api,
				guildIdToWorld: cmap.New[string](),
				players:        cmap.New[[]string](),
				log:            log,
			}
			assert.Equal(t, tt.expect, a.IsConfigured())
		})
	}
}

func TestConfigureWorldName(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	guildID := "guild1"
	world := "Celesta"
	a.ConfigureWorldName(guildID, world)
	val, ok := a.guildIdToWorld.Get(guildID)
	assert.True(t, ok)
	assert.Equal(t, world, val)
}

func TestPlayerStatus(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	mockAPI := &MockAPI{}
	a := &Adapter{
		api:            mockAPI, // ensure IsConfigured returns true
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
	a.players.Set("Celesta", []string{"Mariysz"})
	assert.Equal(t, summary.Online, a.PlayerStatus("guild1", "Mariysz"))
	assert.Equal(t, summary.Offline, a.PlayerStatus("guild1", "Unknown"))

	// not configured
	a2 := &Adapter{
		api:            nil, // IsConfigured returns false
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	assert.Equal(t, summary.Unknown, a2.PlayerStatus("guild1", "Mariysz"))

	// world missing
	a3 := &Adapter{
		api:            mockAPI,
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a3.guildIdToWorld.Set("guild1", "")
	assert.Equal(t, summary.Unknown, a3.PlayerStatus("guild1", "Mariysz"))
}

func TestTryRefresh(t *testing.T) {
	mockAPI := &MockAPI{
		players: []string{"Mariysz"},
	}
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		api:            mockAPI,
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.guildIdToWorld.Set("guild1", "Celesta")
	a.TryRefresh("guild1")
	players, ok := a.players.Get("Celesta")
	assert.True(t, ok)
	assert.Equal(t, []string{"Mariysz"}, players)
}

func TestIsOnline_KeyMisses(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()

	// guildIdToWorld key missing
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a.players.Set("Celesta", []string{"Mariysz"})
	assert.False(t, a.IsOnline("guild1", "Mariysz"))

	// world is empty string
	a2 := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a2.guildIdToWorld.Set("guild1", "")
	a2.players.Set("", []string{"Mariysz"})
	assert.False(t, a2.IsOnline("guild1", "Mariysz"))

	// players key missing
	a3 := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
	}
	a3.guildIdToWorld.Set("guild1", "Celesta")
	assert.False(t, a3.IsOnline("guild1", "Mariysz"))
}

func TestSetGuildWorld_Success(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	mockRepo := mocks.NewMockWorldNameRepository(t)
	mockRepo.On("UpsertGuildWorld", mocks.ContextMock, "guild1", "Celesta").Return(nil)
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  mockRepo,
	}
	guildID := "guild1"
	world := "Celesta"
	err := a.SetGuildWorld(guildID, world)
	assert.NoError(t, err)
	val, ok := a.guildIdToWorld.Get(guildID)
	assert.True(t, ok)
	assert.Equal(t, world, val)
}

func TestSetGuildWorld_Error(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	mockRepo := mocks.NewMockWorldNameRepository(t)
	mockRepo.On("UpsertGuildWorld", mocks.ContextMock, "guild1", "Celesta").Return(errors.New("something went wrong"))
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  mockRepo,
	}
	err := a.SetGuildWorld("guild1", "Celesta")
	assert.Error(t, err)
}

func TestSetGuildWorld_NilRepo(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  nil,
	}
	err := a.SetGuildWorld("guild1", "Celesta")
	assert.Error(t, err)
}

func TestConfigureWorldNameForGuild_Success(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	mockRepo := mocks.NewMockWorldNameRepository(t)
	mockRepo.On("SelectGuildWorld", mocks.ContextMock, "guild1").Return(&guildsworld.GuildsWorld{
		GuildID:   "guild1",
		WorldName: "Celesta",
	}, nil)
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  mockRepo,
	}
	err := a.ConfigureWorldNameForGuild("guild1")
	assert.NoError(t, err)
	val, ok := a.guildIdToWorld.Get("guild1")
	assert.True(t, ok)
	assert.Equal(t, "Celesta", val)
}

func TestConfigureWorldNameForGuild_Error(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	mockRepo := mocks.NewMockWorldNameRepository(t)
	mockRepo.On("SelectGuildWorld", mocks.ContextMock, "guild1").Return(nil, errors.New("db error"))
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  mockRepo,
	}
	err := a.ConfigureWorldNameForGuild("guild1")
	assert.Error(t, err)
}

func TestConfigureWorldNameForGuild_NilRepo(t *testing.T) {
	log := zaptest.NewLogger(t).Sugar()
	a := &Adapter{
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
		log:            log,
		worldNameRepo:  nil,
	}
	err := a.ConfigureWorldNameForGuild("guild1")
	assert.Error(t, err)
}
