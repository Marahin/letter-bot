package factories

import "spot-assistant/internal/core/dto/member"

// CreateMember creates a sample member.
func CreateMember() *member.Member {
	return &member.Member{
		ID:          "sample-member-id",
		Username:    "sample-member-username",
		Nick:        "sample-member-nick",
		Roles:       nil,
		Permissions: 0,
	}
}
