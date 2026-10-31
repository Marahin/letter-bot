package postgresql

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/book"
)

// TestListener_ConnectsThenReceivesABooking runs against a real database. Set
// LETTER_TEST_DATABASE_URL to run it.
func TestListener_ConnectsThenReceivesABooking(t *testing.T) {
	dsn := os.Getenv("LETTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LETTER_TEST_DATABASE_URL not set")
	}

	// given
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := mocks.NewMockNotifyHandler(t)
	connected := make(chan struct{})
	booked := make(chan book.Booked, 1)
	handler.On("OnListenerConnected", mock.Anything).Run(func(mock.Arguments) { close(connected) }).Once()
	handler.On("OnBooked", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		event, _ := args.Get(1).(book.Booked)
		booked <- event
	}).Once()
	done := make(chan struct{})
	go func() {
		NewListener(dsn, zap.NewNop().Sugar()).Listen(ctx, handler)
		close(done)
	}()
	select {
	case <-connected:
	case <-time.After(10 * time.Second):
		t.Fatal("the listener did not connect")
	}
	start := time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC)

	// when
	err = NewNotifier(pool).Booked(context.Background(), book.Booked{GuildID: "it-notify", SpotID: 4, StartAt: start, EndAt: start.Add(time.Hour)})
	require.NoError(t, err)

	// then
	select {
	case got := <-booked:
		require.Equal(t, "it-notify", got.GuildID)
		require.Equal(t, book.SourceWeb, got.Source)
		require.True(t, start.Equal(got.StartAt))
	case <-time.After(10 * time.Second):
		t.Fatal("the booking did not arrive")
	}
	cancel()
	<-done
}
