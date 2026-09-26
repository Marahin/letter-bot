package worldapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/core/dto/world"
	"spot-assistant/internal/ports"
)

type characterResponse struct {
	Character struct {
		Character struct {
			Name              string `json:"name"`
			Sex               string `json:"sex"`
			Vocation          string `json:"vocation"`
			Level             int    `json:"level"`
			AchievementPoints int    `json:"achievement_points"`
			World             string `json:"world"`
			Residence         string `json:"residence"`
			Guild             struct {
				Name string `json:"name"`
				Rank string `json:"rank"`
			} `json:"guild"`
			LastLogin     *time.Time `json:"last_login"`
			AccountStatus string     `json:"account_status"`
		} `json:"character"`
	} `json:"character"`
	Information struct {
		Status struct {
			HTTPCode int `json:"http_code"`
		} `json:"status"`
	} `json:"information"`
}

// GetHighscoresPage reads /highscores/{world}/experience/all/{page}.
func (h *HttpWorldService) GetHighscoresPage(ctx context.Context, worldName string, page int) (*world.HighscoresResponse, error) {
	var data world.HighscoresResponse
	path := fmt.Sprintf("/highscores/%s/experience/all/%d", url.PathEscape(worldName), page)
	status, err := h.getJSON(ctx, path, &data)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, statusError(status)
	}
	return &data, nil
}

// GetCharacter reads /character/{name}.
func (h *HttpWorldService) GetCharacter(ctx context.Context, name string) (*character.Character, error) {
	var data characterResponse
	status, err := h.getJSON(ctx, "/character/"+url.PathEscape(name), &data)
	if err != nil && status != http.StatusNotFound {
		return nil, err
	}
	// TibiaData answers an unknown name with 404 (or, in older v4 releases, 200 and an empty character).
	c := data.Character.Character
	if status == http.StatusNotFound || data.Information.Status.HTTPCode == http.StatusNotFound || (status == http.StatusOK && c.Name == "") {
		return nil, fmt.Errorf("%q: %w", name, ports.ErrCharacterNotFound)
	}
	if status != http.StatusOK {
		return nil, statusError(status)
	}

	return &character.Character{
		Name:              c.Name,
		Level:             c.Level,
		Vocation:          c.Vocation,
		World:             c.World,
		Sex:               c.Sex,
		AccountStatus:     c.AccountStatus,
		GuildName:         c.Guild.Name,
		GuildRank:         c.Guild.Rank,
		LastLogin:         c.LastLogin,
		Residence:         c.Residence,
		AchievementPoints: c.AchievementPoints,
	}, nil
}

// getJSON decodes a 200 or 404 body into out and returns the HTTP status.
func (h *HttpWorldService) getJSON(ctx context.Context, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+path, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.Client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %w: %w", path, ports.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("decode %s: %w", path, err)
	}
	return resp.StatusCode, nil
}

func statusError(status int) error {
	if status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return fmt.Errorf("unexpected HTTP status %d: %w", status, ports.ErrUpstreamUnavailable)
	}
	return fmt.Errorf("unexpected HTTP status %d", status)
}
