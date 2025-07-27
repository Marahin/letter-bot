package guildsworld

import (
	"time"
)

type GuildsWorld struct {
	ID        int64
	GuildID   string
	WorldName string
	CreatedAt time.Time
}
