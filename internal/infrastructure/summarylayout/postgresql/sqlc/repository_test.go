package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/summarylayout"
)

var t0 = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func newMock(t *testing.T) (pgxmock.PgxPoolIface, *SummaryLayoutRepository) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock, NewSummaryLayoutRepository(mock)
}

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestGetLayout(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectQuery("FROM summary_layouts").WithArgs("g1").
		WillReturnRows(pgxmock.NewRows([]string{"guild_id", "category_id", "index_channel_id", "index_message_id", "index_hash", "blocked_until"}).
			AddRow("g1", "cat", "c-sum", "idx", "h", tsz(t0)))
	mock.ExpectQuery("FROM summary_layouts").WithArgs("g2").WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM summary_layouts").WithArgs("g3").WillReturnError(errors.New("boom"))

	// when
	found, err := repo.GetLayout(context.Background(), "g1")
	missing, missingErr := repo.GetLayout(context.Background(), "g2")
	_, failErr := repo.GetLayout(context.Background(), "g3")

	// then
	require.NoError(t, errors.Join(err, missingErr))
	until := t0
	assert.Equal(t, &summarylayout.Layout{GuildID: "g1", CategoryID: "cat", IndexChannelID: "c-sum", IndexMessageID: "idx", IndexHash: "h", BlockedUntil: &until}, found)
	assert.Equal(t, &summarylayout.Layout{GuildID: "g2"}, missing)
	assert.Error(t, failErr)
}

func TestSaveLayout(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectExec("INSERT INTO summary_layouts").WithArgs("g1", "cat", "", "", "", pgtype.Timestamptz{}).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// when
	err := repo.SaveLayout(context.Background(), summarylayout.Layout{GuildID: "g1", CategoryID: "cat"})

	// then
	assert.NoError(t, err)
}

func TestListRegionChannels(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectQuery("FROM summary_region_channels").WithArgs("g1").
		WillReturnRows(pgxmock.NewRows([]string{"guild_id", "region", "channel_id", "message_ids", "content_hash", "retired_at"}).
			AddRow("g1", "port-hope", "c-ph", []string{"m1"}, "h", pgtype.Timestamptz{}).
			AddRow("g1", "zao", "c-zao", nil, "", tsz(t0)))
	mock.ExpectQuery("FROM summary_region_channels").WithArgs("g2").WillReturnError(errors.New("boom"))

	// when
	rows, err := repo.ListRegionChannels(context.Background(), "g1")
	_, failErr := repo.ListRegionChannels(context.Background(), "g2")

	// then
	require.NoError(t, err)
	retired := t0
	assert.Equal(t, []summarylayout.RegionChannel{
		{GuildID: "g1", Region: "port-hope", ChannelID: "c-ph", MessageIDs: []string{"m1"}, ContentHash: "h"},
		{GuildID: "g1", Region: "zao", ChannelID: "c-zao", MessageIDs: []string{}, RetiredAt: &retired},
	}, rows)
	assert.Error(t, failErr)
}

func TestSaveAndDeleteRegionChannel(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectExec("INSERT INTO summary_region_channels").WithArgs("g1", "zao", "c-zao", []string{}, "", tsz(t0)).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("DELETE FROM summary_region_channels").WithArgs("g1", "zao").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	retired := t0

	// when
	saveErr := repo.SaveRegionChannel(context.Background(), summarylayout.RegionChannel{GuildID: "g1", Region: "zao", ChannelID: "c-zao", RetiredAt: &retired})
	deleteErr := repo.DeleteRegionChannel(context.Background(), "g1", "zao")

	// then
	assert.NoError(t, errors.Join(saveErr, deleteErr))
}
