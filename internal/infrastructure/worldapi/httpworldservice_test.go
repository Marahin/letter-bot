package worldapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dto "spot-assistant/internal/core/dto/world"
)

func TestGetOnlinePlayerNames_Success(t *testing.T) {
	// given
	mockResponse := dto.Response{
		World: dto.World{
			OnlinePlayers: []dto.Player{
				{Name: "Mariysz"},
				{Name: "Mariysz Monk"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/world/Celesta", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewHttpWorldService(server.URL)

	// when
	names, err := service.GetOnlinePlayerNames("Celesta")

	// then
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"Mariysz", "Mariysz Monk"}, names)
}

func TestGetOnlinePlayerNames_EmptyList(t *testing.T) {
	// given
	mockResponse := dto.Response{
		World: dto.World{
			OnlinePlayers: []dto.Player{},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewHttpWorldService(server.URL)
	// when
	names, err := service.GetOnlinePlayerNames("Celesta")
	// then
	require.NoError(t, err)
	require.Empty(t, names)
}

func TestGetOnlinePlayerNames_Non200Status(t *testing.T) {
	// given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	service := NewHttpWorldService(server.URL)
	// when
	names, err := service.GetOnlinePlayerNames("Celesta")
	// then
	require.Error(t, err)
	require.Nil(t, names)
}

func TestGetOnlinePlayerNames_InvalidJSON(t *testing.T) {
	// given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	service := NewHttpWorldService(server.URL)
	// when
	names, err := service.GetOnlinePlayerNames("Celesta")
	// then
	require.Error(t, err)
	require.Nil(t, names)
}

func TestGetOnlinePlayerNames_HttpRequestFails(t *testing.T) {
	// given
	service := &HttpWorldService{
		BaseURL: "http://tibiacomnodot",
		Client:  &http.Client{},
	}
	// when
	names, err := service.GetOnlinePlayerNames("")

	// then
	require.Error(t, err)
	require.Contains(t, err.Error(), "error making GET request")
	require.Nil(t, names)
}

func TestGetOnlinePlayerNames_Timeout(t *testing.T) {
	// given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewHttpWorldService(server.URL)
	// set a short timeout for this test
	service.Client.Timeout = 50 * time.Millisecond

	// when
	start := time.Now()
	names, err := service.GetOnlinePlayerNames("Celesta")
	elapsed := time.Since(start)

	// then
	require.Error(t, err)
	require.Nil(t, names)
	require.Less(t, elapsed, 200*time.Millisecond, "should timeout before server responds")
}
