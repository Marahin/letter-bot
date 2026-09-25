package spot

import "time"

type Spot struct {
	Name       string
	ID         int64
	CreatedAt  time.Time
	GuildID    string
	ArchivedAt *time.Time
}

func (s Spot) IsArchived() bool {
	return s.ArchivedAt != nil
}
