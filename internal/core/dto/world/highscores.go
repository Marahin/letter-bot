package world

import "time"

// HighscorePage is one page of a world's experience highscores.
type HighscorePage struct {
	Entries []HighscoreEntry
	// ObservedAt is when tibia.com computed the values on the page.
	ObservedAt time.Time
	// TotalPages is the page count of the whole list.
	TotalPages int
}

type HighscoreEntry struct {
	Name     string
	Vocation string
	Level    int
	Value    int64
}
