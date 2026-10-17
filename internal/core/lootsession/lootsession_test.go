package lootsession

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/loot"
)

var (
	berlin, _ = time.LoadLocation("Europe/Berlin")
	errBoom   = errors.New("boom")
	// The session of sub: 20:00 to 21:00 on 2026-09-25.
	start = time.Date(2026, 9, 25, 20, 0, 0, 0, berlin)
	end   = start.Add(time.Hour)
)

func sub() loot.Submission {
	return loot.Submission{
		From: "2026-09-25, 20:00:00",
		To:   "2026-09-25, 21:00:00",
		Members: []loot.Member{
			{Name: "Marahin", Loot: 100, Supplies: 40, Balance: 60, Damage: 7, Healing: 8},
			{Name: " Killer Potato ", Leader: true, Loot: 200, Supplies: 50, Balance: 150, Damage: 9, Healing: 10},
		},
	}
}

func newService(t *testing.T) (*Service, *mocks.MockLootSessionRepository) {
	repo := mocks.NewMockLootSessionRepository(t)
	s := New(repo, berlin)
	s.now = func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, berlin) }
	return s, repo
}

func candidate(id int64, from, to time.Time) loot.Candidate {
	return loot.Candidate{ReservationID: id, GuildID: "g", GuildName: "Guild", SpotID: 7, SpotName: "Hero Cave", Author: "Marahin", AuthorDiscordID: "111", StartAt: from, EndAt: to}
}

func TestRecord_UntickedWritesNothing(t *testing.T) {
	// given
	s, _ := newService(t)

	// when
	out, err := s.Record(context.Background(), sub(), false)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.Outcome{Status: loot.StatusUnticked}, out)
}

func TestRecord_WithoutSessionTimeWritesNothing(t *testing.T) {
	for name, edit := range map[string]func(*loot.Submission){
		"no header": func(s *loot.Submission) { s.From, s.To = "", "" },
		"inverted":  func(s *loot.Submission) { s.From, s.To = s.To, s.From },
	} {
		t.Run(name, func(t *testing.T) {
			// given
			s, _ := newService(t)
			in := sub()
			edit(&in)

			// when
			out, err := s.Record(context.Background(), in, true)

			// then
			require.NoError(t, err)
			assert.Equal(t, loot.StatusNoTime, out.Status)
		})
	}
}

func TestRecord_FutureSessionWritesNothing(t *testing.T) {
	// given
	s, _ := newService(t)
	s.now = func() time.Time { return end.Add(-FutureSlack - time.Second) }

	// when
	out, err := s.Record(context.Background(), sub(), true)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.StatusFuture, out.Status)
}

func TestRecord_SessionEndingWithinTheSlackIsStored(t *testing.T) {
	// given
	s, repo := newService(t)
	s.now = func() time.Time { return end.Add(-FutureSlack) }
	repo.EXPECT().SaveSession(mock.Anything, mock.Anything).Return(1, nil)
	repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	// when
	out, err := s.Record(context.Background(), sub(), true)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.StatusUnmatched, out.Status)
}

func TestRecord_CountsTheMatchedReservation(t *testing.T) {
	// given
	s, repo := newService(t)
	var saved loot.Session
	repo.EXPECT().SaveSession(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ls loot.Session) (int64, error) {
		saved = ls
		return 42, nil
	})
	booked := candidate(9, start.Add(-30*time.Minute), end.Add(30*time.Minute))
	repo.EXPECT().MatchCandidates(mock.Anything, start, end, []string{"marahin", "killer potato"}, booking.MaximumReservationLength).
		Return([]loot.Candidate{booked}, nil)
	repo.EXPECT().Pick(mock.Anything, loot.Attribution{SessionID: 42, ReservationID: 9, OverlapSeconds: 3600}).Return(true, nil)

	// when
	out, err := s.Record(context.Background(), sub(), true)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.StatusCounted, out.Status)
	require.NotNil(t, out.Match)
	assert.Equal(t, booked, *out.Match)
	assert.True(t, start.Equal(saved.StartedAt))
	assert.Equal(t, time.Hour, saved.Duration)
	assert.Equal(t, int64(300), saved.Loot)
	assert.Equal(t, int64(90), saved.Supplies)
	assert.Equal(t, int64(210), saved.Balance)
	assert.Equal(t, int64(16), saved.Damage)
	assert.Equal(t, int64(18), saved.Healing)
	require.Len(t, saved.Members, 2)
	assert.Equal(t, "Killer Potato", saved.Members[1].Name)
	assert.Equal(t, "killer potato", saved.Members[1].Key)
	assert.Equal(t, 1, saved.Members[1].Position)
	assert.True(t, saved.Members[1].Leader)
	assert.Len(t, saved.Hash, 32)
}

func TestRecord_TheAuthorsOwnSessionIsByAuthor(t *testing.T) {
	tests := map[string]struct {
		submitter string
		byAuthor  bool
	}{
		"the author":   {submitter: "111", byAuthor: true},
		"someone else": {submitter: "222"},
		"signed out":   {submitter: ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			s, repo := newService(t)
			in := sub()
			in.SubmittedBy = tt.submitter
			var saved loot.Session
			repo.EXPECT().SaveSession(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ls loot.Session) (int64, error) {
				saved = ls
				return 42, nil
			})
			repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]loot.Candidate{candidate(9, start, end)}, nil)
			repo.EXPECT().Pick(mock.Anything, loot.Attribution{SessionID: 42, ReservationID: 9, OverlapSeconds: 3600, ByAuthor: tt.byAuthor}).Return(true, nil)

			// when
			out, err := s.Record(context.Background(), in, true)

			// then
			require.NoError(t, err)
			assert.Equal(t, loot.StatusCounted, out.Status)
			assert.Equal(t, tt.submitter, saved.SubmittedBy)
		})
	}
}

func TestRecord_OutOfRangeWritesNothing(t *testing.T) {
	tests := map[string]func(*loot.Submission){
		"shorter than a minute": func(s *loot.Submission) { s.To = "2026-09-25, 20:00:59" },
		"longer than a reservation": func(s *loot.Submission) {
			s.From, s.To = "2026-09-25, 17:59:59", "2026-09-25, 21:00:00"
		},
		"no party member": func(s *loot.Submission) { s.Members = nil },
		"more than ten members": func(s *loot.Submission) {
			for len(s.Members) <= MaxPartySize {
				s.Members = append(s.Members, loot.Member{Name: "Extra " + strconv.Itoa(len(s.Members))})
			}
		},
		"no member with a name": func(s *loot.Submission) {
			s.Members = []loot.Member{{Name: "  ", Balance: 1}}
		},
		"loot too large":        func(s *loot.Submission) { s.Members[0].Loot = MaxFigure + 1 },
		"supplies too large":    func(s *loot.Submission) { s.Members[0].Supplies = MaxFigure + 1 },
		"balance too large":     func(s *loot.Submission) { s.Members[1].Balance = MaxFigure + 1 },
		"balance too small":     func(s *loot.Submission) { s.Members[1].Balance = -MaxFigure - 1 },
		"damage too large":      func(s *loot.Submission) { s.Members[0].Damage = MaxFigure + 1 },
		"healing too large":     func(s *loot.Submission) { s.Members[0].Healing = MaxFigure + 1 },
		"healing far too small": func(s *loot.Submission) { s.Members[0].Healing = math.MinInt64 },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			s, _ := newService(t)
			in := sub()
			edit(&in)

			// when
			out, err := s.Record(context.Background(), in, true)

			// then
			require.NoError(t, err)
			assert.Equal(t, loot.Outcome{Status: loot.StatusOutOfRange}, out)
		})
	}
}

func TestRecord_TheBoundsAreInclusive(t *testing.T) {
	tests := map[string]func(*loot.Submission){
		"one minute": func(s *loot.Submission) { s.To = "2026-09-25, 20:01:00" },
		"as long as a reservation": func(s *loot.Submission) {
			s.From, s.To = "2026-09-25, 18:00:00", "2026-09-25, 21:00:00"
		},
		"ten members": func(s *loot.Submission) {
			for len(s.Members) < MaxPartySize {
				s.Members = append(s.Members, loot.Member{Name: "Extra " + strconv.Itoa(len(s.Members))})
			}
		},
		"the largest figures": func(s *loot.Submission) {
			s.Members[0].Loot, s.Members[0].Balance = MaxFigure, -MaxFigure
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			s, repo := newService(t)
			in := sub()
			edit(&in)
			repo.EXPECT().SaveSession(mock.Anything, mock.Anything).Return(42, nil)
			repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

			// when
			out, err := s.Record(context.Background(), in, true)

			// then
			require.NoError(t, err)
			assert.Equal(t, loot.StatusUnmatched, out.Status)
		})
	}
}

func TestRecord_OutmatchedKeepsTheMatch(t *testing.T) {
	// given
	s, repo := newService(t)
	booked := candidate(9, start, end)
	repo.EXPECT().SaveSession(mock.Anything, mock.Anything).Return(42, nil)
	repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]loot.Candidate{booked}, nil)
	repo.EXPECT().Pick(mock.Anything, loot.Attribution{SessionID: 42, ReservationID: 9, OverlapSeconds: 3600}).Return(false, nil)

	// when
	out, err := s.Record(context.Background(), sub(), true)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.StatusOutmatched, out.Status)
	assert.Equal(t, int64(9), out.Match.ReservationID)
}

func TestRecord_Unmatched(t *testing.T) {
	// given
	s, repo := newService(t)
	repo.EXPECT().SaveSession(mock.Anything, mock.Anything).Return(42, nil)
	// 29m59s of a one-hour session is less than half.
	short := candidate(9, end.Add(-30*time.Minute+time.Second), end.Add(time.Hour))
	repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]loot.Candidate{short}, nil)

	// when
	out, err := s.Record(context.Background(), sub(), true)

	// then
	require.NoError(t, err)
	assert.Equal(t, loot.Outcome{Status: loot.StatusUnmatched}, out)
}

func TestRecord_RepositoryErrors(t *testing.T) {
	for i, step := range []string{"save", "match", "pick"} {
		t.Run(step, func(t *testing.T) {
			// given
			s, repo := newService(t)
			fail := func(n int) error {
				if n == i {
					return errBoom
				}
				return nil
			}
			repo.EXPECT().SaveSession(mock.Anything, mock.Anything).Return(42, fail(0))
			if i >= 1 {
				repo.EXPECT().MatchCandidates(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return([]loot.Candidate{candidate(9, start, end)}, fail(1))
			}
			if i >= 2 {
				repo.EXPECT().Pick(mock.Anything, mock.Anything).Return(false, fail(2))
			}

			// when
			_, err := s.Record(context.Background(), sub(), true)

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestPick(t *testing.T) {
	tests := []struct {
		name       string
		candidates []loot.Candidate
		want       int64
		overlap    time.Duration
	}{
		{name: "none"},
		{name: "exactly half counts", candidates: []loot.Candidate{candidate(1, start.Add(30*time.Minute), end.Add(time.Hour))}, want: 1, overlap: 30 * time.Minute},
		{name: "touching does not count", candidates: []loot.Candidate{candidate(1, end, end.Add(time.Hour))}},
		{
			name: "largest overlap wins",
			candidates: []loot.Candidate{
				candidate(1, start.Add(-time.Hour), start.Add(40*time.Minute)),
				candidate(2, start.Add(10*time.Minute), end.Add(time.Hour)),
			},
			want: 2, overlap: 50 * time.Minute,
		},
		{
			name: "equal overlap: the latest start",
			candidates: []loot.Candidate{
				candidate(3, start.Add(-time.Hour), end),
				candidate(1, start, end.Add(time.Hour)),
			},
			want: 1, overlap: time.Hour,
		},
		{
			name: "equal overlap and start: the larger id",
			candidates: []loot.Candidate{
				candidate(5, start, end),
				candidate(8, start, end),
				candidate(6, start, end),
			},
			want: 8, overlap: time.Hour,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, overlap := pick(tt.candidates, start, end)

			// then
			if tt.want == 0 {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.ReservationID)
			assert.Equal(t, tt.overlap, overlap)
		})
	}
}

func TestNewSession_DuplicateKeysKeepTheFirstLine(t *testing.T) {
	// given
	in := sub()
	in.Members = append(in.Members, loot.Member{Name: "marahin", Balance: 1000})

	// when
	s := NewSession(in, start, end)

	// then
	require.Len(t, s.Members, 2)
	assert.Equal(t, "Marahin", s.Members[0].Name)
	assert.Equal(t, int64(60), s.Members[0].Balance)
	assert.Equal(t, int64(1210), s.Balance, "the party figures cover every line, as the split does")
	assert.Equal(t, 3, s.PartySize)
}

func TestHash_IgnoresPasteOrderAndChangesWithAnyFigure(t *testing.T) {
	// given
	base := NewSession(sub(), start, end)
	reversed := sub()
	reversed.Members[0], reversed.Members[1] = reversed.Members[1], reversed.Members[0]

	// then
	assert.Equal(t, base.Hash, NewSession(reversed, start, end).Hash)
	assert.NotEqual(t, base.Hash, NewSession(sub(), start, end.Add(time.Second)).Hash)
	assert.NotEqual(t, base.Hash, NewSession(sub(), start.Add(-time.Second), end).Hash)
	edits := []func(*loot.Member){
		func(m *loot.Member) { m.Name = "Other" },
		func(m *loot.Member) { m.Leader = !m.Leader },
		func(m *loot.Member) { m.Loot++ },
		func(m *loot.Member) { m.Supplies++ },
		func(m *loot.Member) { m.Balance++ },
		func(m *loot.Member) { m.Damage++ },
		func(m *loot.Member) { m.Healing++ },
	}
	for i, edit := range edits {
		changed := sub()
		edit(&changed.Members[0])
		assert.NotEqual(t, base.Hash, NewSession(changed, start, end).Hash, "edit %d", i)
	}
}
