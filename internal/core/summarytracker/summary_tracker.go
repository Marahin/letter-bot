package summarytracker

import (
	"context"
	"time"
)

type MessageType string

const (
	MessageTypePreMessage MessageType = "pre_message"
	MessageTypeEmbed      MessageType = "embed"
)

type TrackedMessage struct {
	ID           int64
	GuildID      string
	ChannelID    string
	MessageID    string
	MessageType  MessageType
	MessageOrder int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Repository interface {
	GetMessages(ctx context.Context, guildID, channelID string) ([]*TrackedMessage, error)
	UpsertMessage(ctx context.Context, msg *TrackedMessage) error
	DeleteMessage(ctx context.Context, id int64) error
	DeleteAllForChannel(ctx context.Context, guildID, channelID string) error
	UpdateTimestamp(ctx context.Context, id int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetTrackedMessages retrieves all tracked messages for a channel
func (s *Service) GetTrackedMessages(
	ctx context.Context,
	guildID, channelID string,
) ([]*TrackedMessage, error) {
	return s.repo.GetMessages(ctx, guildID, channelID)
}

// TrackMessage saves or updates a tracked message
func (s *Service) TrackMessage(
	ctx context.Context,
	guildID, channelID, messageID string,
	messageType MessageType,
	messageOrder int,
) error {
	msg := &TrackedMessage{
		GuildID:      guildID,
		ChannelID:    channelID,
		MessageID:    messageID,
		MessageType:  messageType,
		MessageOrder: messageOrder,
	}
	return s.repo.UpsertMessage(ctx, msg)
}

// DeleteMessage removes a tracked message
func (s *Service) DeleteMessage(ctx context.Context, id int64) error {
	return s.repo.DeleteMessage(ctx, id)
}

// DeleteAllForChannel removes all tracked messages for a channel
func (s *Service) DeleteAllForChannel(ctx context.Context, guildID, channelID string) error {
	return s.repo.DeleteAllForChannel(ctx, guildID, channelID)
}

// UpdateTimestamp updates the updated_at timestamp for a message
func (s *Service) UpdateTimestamp(ctx context.Context, id int64) error {
	return s.repo.UpdateTimestamp(ctx, id)
}
