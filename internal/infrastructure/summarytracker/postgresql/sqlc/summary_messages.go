package sqlc

import (
	"context"

	"spot-assistant/internal/core/summarytracker"
)

type SummaryMessagesRepository struct {
	q *Queries
}

func NewSummaryMessagesRepository(db DBTX) *SummaryMessagesRepository {
	return &SummaryMessagesRepository{
		q: New(db),
	}
}

func (r *SummaryMessagesRepository) GetMessages(ctx context.Context, guildID, channelID string) ([]*summarytracker.TrackedMessage, error) {
	rows, err := r.q.GetSummaryMessages(ctx, GetSummaryMessagesParams{
		GuildID:   guildID,
		ChannelID: channelID,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]*summarytracker.TrackedMessage, len(rows))
	for i, row := range rows {
		messages[i] = &summarytracker.TrackedMessage{
			ID:           row.ID,
			GuildID:      row.GuildID,
			ChannelID:    row.ChannelID,
			MessageID:    row.MessageID,
			MessageType:  summarytracker.MessageType(row.MessageType),
			MessageOrder: int(row.MessageOrder),
			CreatedAt:    row.CreatedAt.Time,
			UpdatedAt:    row.UpdatedAt.Time,
		}
	}

	return messages, nil
}

func (r *SummaryMessagesRepository) UpsertMessage(ctx context.Context, msg *summarytracker.TrackedMessage) error {
	_, err := r.q.UpsertSummaryMessage(ctx, UpsertSummaryMessageParams{
		GuildID:      msg.GuildID,
		ChannelID:    msg.ChannelID,
		MessageID:    msg.MessageID,
		MessageType:  string(msg.MessageType),
		MessageOrder: int32(msg.MessageOrder),
	})
	return err
}

func (r *SummaryMessagesRepository) DeleteMessage(ctx context.Context, id int64) error {
	return r.q.DeleteSummaryMessage(ctx, id)
}

func (r *SummaryMessagesRepository) DeleteAllForChannel(ctx context.Context, guildID, channelID string) error {
	return r.q.DeleteSummaryMessagesForGuild(ctx, DeleteSummaryMessagesForGuildParams{
		GuildID:   guildID,
		ChannelID: channelID,
	})
}

func (r *SummaryMessagesRepository) UpdateTimestamp(ctx context.Context, id int64) error {
	return r.q.UpdateMessageTimestamp(ctx, id)
}
