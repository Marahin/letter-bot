package guild

import (
	"spot-assistant/internal/core/dto/role"
)

type Guild struct {
	ID    string
	Name  string
	Roles []*role.Role

	InternalID int32 // Our internal database ID - only available when fetched from the database
}
