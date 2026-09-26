package world

import "time"

// HighscoresResponse is one page of TibiaData /v4/highscores/{world}/{category}/{vocation}/{page}.
type HighscoresResponse struct {
	Highscores  Highscores  `json:"highscores"`
	Information Information `json:"information"`
}

type Highscores struct {
	World    string `json:"world"`
	Category string `json:"category"`
	// HighscoreAge is how old the tibia.com highscore data is, in minutes.
	HighscoreAge  int              `json:"highscore_age"`
	HighscoreList []HighscoreEntry `json:"highscore_list"`
	HighscorePage HighscorePage    `json:"highscore_page"`
}

type HighscoreEntry struct {
	Rank     int    `json:"rank"`
	Name     string `json:"name"`
	Vocation string `json:"vocation"`
	World    string `json:"world"`
	Level    int    `json:"level"`
	Value    int64  `json:"value"`
}

type HighscorePage struct {
	CurrentPage  int `json:"current_page"`
	TotalPages   int `json:"total_pages"`
	TotalRecords int `json:"total_records"`
}

// Information.Timestamp is when TibiaData scraped tibia.com. A cached response can be older than the request.
type Information struct {
	Timestamp time.Time `json:"timestamp"`
}
