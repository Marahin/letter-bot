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
	Character   characterEnvelope  `json:"character"`
	Information characterStatusBox `json:"information"`
}

type characterEnvelope struct {
	Character characterData `json:"character"`
}

type characterData struct {
	Name              string         `json:"name"`
	Sex               string         `json:"sex"`
	Vocation          string         `json:"vocation"`
	Level             int            `json:"level"`
	AchievementPoints int            `json:"achievement_points"`
	World             string         `json:"world"`
	Residence         string         `json:"residence"`
	Guild             characterGuild `json:"guild"`
	LastLogin         *time.Time     `json:"last_login"`
	AccountStatus     string         `json:"account_status"`
}

type characterGuild struct {
	Name string `json:"name"`
	Rank string `json:"rank"`
}

type characterStatusBox struct {
	Status responseStatus `json:"status"`
}

type responseStatus struct {
	HTTPCode int `json:"http_code"`
}

// highscoresResponse is one page of /v4/highscores/{world}/{category}/{vocation}/{page}.
type highscoresResponse struct {
	Highscores  highscores           `json:"highscores"`
	Information highscoresScrapeInfo `json:"information"`
}

type highscores struct {
	// HighscoreAge is how old the tibia.com highscore data is, in minutes.
	HighscoreAge  int             `json:"highscore_age"`
	HighscoreList []highscoreRow  `json:"highscore_list"`
	HighscorePage highscoresPager `json:"highscore_page"`
}

type highscoreRow struct {
	Name     string `json:"name"`
	Vocation string `json:"vocation"`
	Level    int    `json:"level"`
	Value    int64  `json:"value"`
}

type highscoresPager struct {
	TotalPages int `json:"total_pages"`
}

type highscoresScrapeInfo struct {
	// Timestamp is when TibiaData scraped tibia.com; a cached response can be older than the request.
	Timestamp time.Time `json:"timestamp"`
}

// GetHighscoresPage reads /highscores/{world}/experience/all/{page}.
func (h *HTTPWorldService) GetHighscoresPage(ctx context.Context, worldName string, page int) (*world.HighscorePage, error) {
	var data highscoresResponse
	path := fmt.Sprintf("/highscores/%s/experience/all/%d", url.PathEscape(worldName), page)
	requested := h.clock()
	status, err := h.getJSON(ctx, path, &data)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, statusError(status)
	}
	out := &world.HighscorePage{
		Entries:    make([]world.HighscoreEntry, len(data.Highscores.HighscoreList)),
		ObservedAt: observedAt(data.Information.Timestamp, data.Highscores.HighscoreAge, requested),
		TotalPages: data.Highscores.HighscorePage.TotalPages,
	}
	for i, e := range data.Highscores.HighscoreList {
		out.Entries[i] = world.HighscoreEntry{Name: e.Name, Vocation: e.Vocation, Level: e.Level, Value: e.Value}
	}
	return out, nil
}

// observedAt is the scrape time (the request time when it is missing or in the future) minus the
// highscore age.
func observedAt(scraped time.Time, ageMinutes int, requested time.Time) time.Time {
	if scraped.IsZero() || scraped.After(requested) {
		scraped = requested
	}
	return scraped.Add(-time.Duration(max(0, ageMinutes)) * time.Minute)
}

// GetCharacter reads /character/{name}.
func (h *HTTPWorldService) GetCharacter(ctx context.Context, name string) (*character.Character, error) {
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
func (h *HTTPWorldService) getJSON(ctx context.Context, path string, out any) (int, error) {
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
