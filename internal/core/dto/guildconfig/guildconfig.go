package guildconfig

import "time"

// RoleKind names one of the rank lists of a guild.
type RoleKind string

const (
	RoleKindManage   RoleKind = "manage"
	RoleKindView     RoleKind = "view"
	RoleKindReserve  RoleKind = "reserve"
	RoleKindOverbook RoleKind = "overbook"
)

// RoleKinds lists every RoleKind in display order.
var RoleKinds = []RoleKind{RoleKindManage, RoleKindView, RoleKindReserve, RoleKindOverbook}

func (k RoleKind) Valid() bool {
	switch k {
	case RoleKindManage, RoleKindView, RoleKindReserve, RoleKindOverbook:
		return true
	}
	return false
}

// Config is the stored, per-guild configuration shared by the bot and the web.
type Config struct {
	GuildID           string
	Name              string
	Icon              string
	OwnerID           string
	BotPresent        bool
	Premium           bool
	PremiumForever    bool
	CommandChannelID  string
	SummaryChannelID  string
	ManageRoleIDs     []string
	ViewRoleIDs       []string
	ReserveRoleIDs    []string
	OverbookRoleIDs   []string
	ResyncRequestedAt *time.Time
	SyncedAt          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (c Config) IsPremium() bool {
	return c.Premium || c.PremiumForever
}

// RoleIDs returns the rank list of the given kind.
func (c Config) RoleIDs(kind RoleKind) []string {
	switch kind {
	case RoleKindManage:
		return c.ManageRoleIDs
	case RoleKindView:
		return c.ViewRoleIDs
	case RoleKindReserve:
		return c.ReserveRoleIDs
	case RoleKindOverbook:
		return c.OverbookRoleIDs
	}
	return nil
}
