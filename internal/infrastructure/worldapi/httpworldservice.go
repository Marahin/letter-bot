package worldapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/world"
)

type HTTPWorldService struct {
	BaseURL string
	Client  *http.Client
	now     func() time.Time
}

func NewHTTPWorldService(baseURL string) *HTTPWorldService {
	return &HTTPWorldService{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *HTTPWorldService) GetOnlinePlayerNames(worldName string) ([]string, error) {
	url := fmt.Sprintf("%s/world/%s", h.BaseURL, worldName)

	resp, err := h.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	var data world.Response
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	names := collections.PoorMansMap(data.World.OnlinePlayers, func(p world.Player) string {
		return p.Name
	})

	return names, nil
}

func (h *HTTPWorldService) clock() time.Time {
	if h.now == nil {
		return time.Now()
	}
	return h.now()
}

func (h *HTTPWorldService) GetBaseURL() string {
	return h.BaseURL
}
