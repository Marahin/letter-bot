package worldapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"spot-assistant/internal/core/dto/world"
)

type HttpWorldService struct {
	BaseURL string
	Client  *http.Client
}

func NewHttpWorldService(baseURL string) *HttpWorldService {
	return &HttpWorldService{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

func (h *HttpWorldService) GetOnlinePlayerNames(worldName string) ([]string, error) {
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

	names := make([]string, len(data.World.OnlinePlayers))
	for i, p := range data.World.OnlinePlayers {
		names[i] = p.Name
	}

	return names, nil
}

func (h *HttpWorldService) GetBaseURL() string {
	return h.BaseURL
}
