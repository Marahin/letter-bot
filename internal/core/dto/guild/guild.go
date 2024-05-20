package guild

import (
	"spot-assistant/internal/core/dto/role"
)

type Guild struct {
	ID    string
	Name  string
	Roles []*role.Role
}
