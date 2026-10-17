package toolshttp

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/loot"
	"spot-assistant/internal/core/lootcalc"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web/webtest"
)

const sample = `Session data: From 2026-09-25, 20:14:39 to 2026-09-25, 21:27:43
Session: 01:13h
Loot Type: Leader
Loot: 1,904,159
Supplies: 1,010,086
Balance: 894,073
Marahin
    Loot: 87,536
    Supplies: 67,484
    Balance: 20,052
    Damage: 1,750,214
    Healing: 355,923
Iscarlott
    Loot: 0
    Supplies: 414,235
    Balance: -414,235
    Damage: 200
    Healing: 1,589,899
Killer Potato (Leader)
    Loot: 1,816,623
    Supplies: 528,367
    Balance: 1,288,256
    Damage: 4,043,228
    Healing: 398,456
`

func anonymous(t *testing.T) http.Handler {
	t.Helper()
	d, m := webtest.NewDeps(t)
	m.Loot.EXPECT().Record(mock.Anything, mock.Anything, mock.Anything).Return(loot.Outcome{Status: loot.StatusUnmatched}, nil).Maybe()
	return webtest.Handler(d, Register)
}

// recording serves the calculator with Record answering out and err.
func recording(t *testing.T, out loot.Outcome, err error) (http.Handler, webtest.Mocks) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	m.Loot.EXPECT().Record(mock.Anything, mock.Anything, mock.Anything).Return(out, err).Maybe()
	return webtest.Handler(d, Register), m
}

func htmx(r *http.Request) *http.Request {
	r.Header.Set("HX-Request", "true")
	return r
}

func TestHandleLootCalculator_SignedOutTopBar(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, webtest.Get("/tools/loot-calculator", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Loot Calculator - TibiaLoot.com</title>", "Loot Calculator</h1>",
		`id="loot-main"`, `hx-post="/tools/loot-calculator"`, `name="session"`,
		`aria-describedby="loot-session-help"`, "Calculate split",
		`data-loot-history`, "Stored in this browser only.",
		"/assets/loot-calculator.js", "data-letter-strings",
		"/login", // the signed-out top bar
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, "role=\"alert\"")
	assert.NotContains(t, body, "<aside")
}

func TestHandleLootCalculator_SignedInSidebar(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")

	// when
	rec := webtest.Serve(h, webtest.Get("/tools/loot-calculator", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<aside")
	assert.Regexp(t, `href="/tools/loot-calculator"\s+aria-current="page"`, body)
	assert.Contains(t, body, "user-u1")
}

func TestHandleLootCalculator_Polish(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, webtest.Get("/tools/loot-calculator?lang=pl", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Kalkulator lootu</h1>")
	assert.Contains(t, rec.Body.String(), "Podziel loot")
}

func TestHandleLootCalculate_HtmxResult(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}}, nil)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "<html", "htmx gets the main column only")
	for _, want := range []string{
		`id="loot-main"`, "Transfers", "New session",
		"894.073", "298.024", ">3<", "01:13h",
		"277.972 gp", "712.259 gp",
		`data-copy="transfer 277972 to Marahin"`,
		`data-copy="transfer 712259 to Iscarlott"`,
		`aria-label="Copy “transfer” for Iscarlott"`,
		`aria-live="polite"`,
		"Copy for Discord", "Copy for TeamSpeak",
		"−414.235", "text-rose-400", "text-emerald-400", "Leader",
		`id="loot-entry"`,
	} {
		assert.Contains(t, body, want)
	}
	assert.Contains(t, html.UnescapeString(body), `data-copy="**Killer Potato** should give **277.972 gp** to **Marahin**.`)
	assert.Contains(t, html.UnescapeString(body), "[color=green]894.073 gp[/color]")

	// players are listed by balance, highest first
	rows := regexp.MustCompile(`<th scope="row"[^>]*>([^<]+)`).FindAllStringSubmatch(body, -1)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"Killer Potato", "Marahin", "Iscarlott"}, []string{strings.TrimSpace(rows[0][1]), strings.TrimSpace(rows[1][1]), strings.TrimSpace(rows[2][1])})

	entry := regexp.MustCompile(`(?s)<script id="loot-entry" type="application/json">(.*?)</script>`).FindStringSubmatch(body)
	require.Len(t, entry, 2)
	var got historyEntry
	require.NoError(t, json.Unmarshal([]byte(entry[1]), &got))
	assert.Equal(t, historyEntry{
		Key:   "2026-09-25, 20:14:39|2026-09-25, 21:27:43|Marahin|Iscarlott|Killer Potato|894073",
		Label: "2026-09-25, 20:14 → 21:27",
		Names: []string{"Marahin", "Iscarlott", "Killer Potato"},
		Total: 894073,
		Text:  sample,
		Count: true,
	}, got)
}

func TestHandleLootCalculate_HtmxErrors(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "empty", text: "   ", want: "Paste the Party Hunt Analyser text first."},
		{name: "no players", text: "Loot: 1\nBalance: 1", want: "No player found."},
		{name: "missing fields", text: "Iscarlott\n    Loot: 0\n    Supplies: 414,235\n    Balance: -414,235\n", want: "Player “Iscarlott” has no Damage and Healing lines."},
		{name: "malformed", text: "A\n    Balance: 1x\n", want: "Line 2 has a value that is not a number: “Balance: 1x”."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			h := anonymous(t)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {tt.text}}, nil)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			body := html.UnescapeString(rec.Body.String())
			assert.Contains(t, body, `role="alert"`)
			assert.Contains(t, body, `aria-invalid="true"`)
			assert.Contains(t, body, `aria-describedby="loot-session-help loot-session-error"`)
			assert.Contains(t, body, tt.want)
			assert.Contains(t, body, strings.TrimSpace(tt.text), "the pasted text stays in the field")
		})
	}
}

func TestHandleLootCalculate_PlainPostRendersPage(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}}, nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, `data-copy="transfer 712259 to Iscarlott"`)
	assert.Contains(t, body, `data-loot-history`)
}

func TestHandleLootCalculate_TooLong(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {strings.Repeat("x", MaxSessionBytes)}}, nil)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "The text is too long.")
}

func TestHandleLootCalculate_MalformedBody(t *testing.T) {
	// given
	h := anonymous(t)
	r := webtest.Post("/tools/loot-calculator", nil, nil)
	r.Body = io.NopCloser(strings.NewReader("session=%zz"))

	// when
	rec := webtest.Serve(h, r)

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleLootCalculate_CrossOriginRefused(t *testing.T) {
	// given
	h := anonymous(t)
	r := webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}}, nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")

	// when
	rec := webtest.Serve(h, r)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleLootCalculate_EvenSplit(t *testing.T) {
	// given
	h := anonymous(t)
	text := "A\n    Loot: 1\n    Supplies: 1\n    Balance: 50\n    Damage: 1\n    Healing: 1\n" +
		"B\n    Loot: 1\n    Supplies: 1\n    Balance: 50\n    Damage: 1\n    Healing: 1\n"

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {text}}, nil)))

	// then
	body := rec.Body.String()
	assert.Contains(t, body, "Nobody has to send gold")
	assert.Contains(t, body, "—", "no header means no duration")
}

func TestSessionLabel(t *testing.T) {
	tests := []struct {
		name string
		s    lootcalc.Session
		want string
	}{
		{name: "same day", s: lootcalc.Session{From: "2026-09-25, 20:14:39", To: "2026-09-25, 21:27:43"}, want: "2026-09-25, 20:14 → 21:27"},
		{name: "past midnight", s: lootcalc.Session{From: "2026-09-25, 23:14:39", To: "2026-09-26, 01:27:43"}, want: "2026-09-25, 23:14 → 2026-09-26, 01:27"},
		{name: "no header", s: lootcalc.Session{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sessionLabel(tt.s))
		})
	}
}

func TestErrorText(t *testing.T) {
	// given
	ctx := i18n.WithLocale(context.Background(), i18n.Match("pl"))

	// when / then
	assert.Equal(t, "Gracz „A” nie ma wierszy Loot, Damage i Healing. Wklej cały tekst z okna Party Hunt Analyser.",
		errorText(ctx, &lootcalc.ParseError{Err: lootcalc.ErrMissingFields, Player: "A", Missing: []string{"Loot", "Damage", "Healing"}}))
	assert.Equal(t, "Gracz „A” nie ma wierszy Healing. Wklej cały tekst z okna Party Hunt Analyser.",
		errorText(ctx, &lootcalc.ParseError{Err: lootcalc.ErrMissingFields, Player: "A", Missing: []string{"Healing"}}))
	assert.Contains(t, errorText(ctx, errors.New("other")), "Nie znaleziono graczy")
}

func TestLandingCalculator_StartsFromTheLandingPage(t *testing.T) {
	// given
	var out strings.Builder

	// when
	err := LandingCalculator().Render(context.Background(), &out)

	// then
	require.NoError(t, err)
	body := out.String()
	for _, want := range []string{
		`id="loot-main"`, `hx-post="/tools/loot-calculator"`,
		`name="from" value="/"`, `rows="10"`, "data-loot-autofocus",
		`data-loot-from="/"`, "/assets/loot-calculator.js",
	} {
		assert.Contains(t, body, want)
	}
}

func TestHandleLootCalculate_NewCalculationReturnsToTheStartPage(t *testing.T) {
	cases := map[string]string{
		"/":                      `href="/"`,
		"/tools/loot-calculator": `href="/tools/loot-calculator"`,
		"//evil.example":         `href="/tools/loot-calculator"`,
		"":                       `href="/tools/loot-calculator"`,
	}
	for from, want := range cases {
		t.Run(from, func(t *testing.T) {
			// given
			h := anonymous(t)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}, "from": {from}}, nil)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), want+` data-loot-new`)
		})
	}
}

func TestHandleLootCalculator_PageDefaults(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, webtest.Get("/tools/loot-calculator", nil))

	// then
	body := rec.Body.String()
	assert.Contains(t, body, `rows="17"`)
	assert.Contains(t, body, `name="from" value="/tools/loot-calculator"`)
	assert.Contains(t, body, "data-loot-autofocus")
}

func TestHandleLootCalculate_StatsNotices(t *testing.T) {
	match := &loot.Candidate{ReservationID: 9, GuildID: "g1", GuildName: "Celesta Community", SpotID: 7, SpotName: "Hero Cave"}
	tests := []struct {
		name string
		out  loot.Outcome
		err  error
		want []string
	}{
		{name: "counted", out: loot.Outcome{Status: loot.StatusCounted, Match: match}, want: []string{
			`data-loot-stats="counted"`, "Counted in the public stats of Hero Cave on Celesta Community.", `href="/servers/g1/stats/spots/7"`, "View the respawn stats",
		}},
		{name: "unticked", out: loot.Outcome{Status: loot.StatusUnticked}, want: []string{`data-loot-stats="unticked"`, "the box is unticked"}},
		{name: "no time", out: loot.Outcome{Status: loot.StatusNoTime}, want: []string{`data-loot-stats="no_time"`, "the text has no session time"}},
		{name: "future", out: loot.Outcome{Status: loot.StatusFuture}, want: []string{`data-loot-stats="future"`, "ends in the future"}},
		{name: "out of range", out: loot.Outcome{Status: loot.StatusOutOfRange}, want: []string{`data-loot-stats="out_of_range"`, "has figures no hunt reaches"}},
		{name: "unmatched", out: loot.Outcome{Status: loot.StatusUnmatched}, want: []string{`data-loot-stats="unmatched"`, "no reservation matches it"}},
		{name: "outmatched", out: loot.Outcome{Status: loot.StatusOutmatched, Match: match}, want: []string{
			`data-loot-stats="outmatched"`, "the reservation of Hero Cave on Celesta Community already has its author's session",
		}},
		{name: "failed", err: errors.New("db down"), want: []string{`data-loot-stats="failed"`, "could not be saved", `data-copy="transfer 712259 to Iscarlott"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			h, _ := recording(t, tt.out, tt.err)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}, "count_field": {"1"}, "count": {"1"}}, nil)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			body := html.UnescapeString(rec.Body.String())
			assert.Contains(t, body, `role="status"`)
			for _, want := range tt.want {
				assert.Contains(t, body, want)
			}
		})
	}
}

func TestHandleLootCalculate_PassesTheSessionAndTheChoice(t *testing.T) {
	tests := []struct {
		name  string
		form  url.Values
		count bool
	}{
		{name: "ticked", form: url.Values{"count_field": {"1"}, "count": {"1"}}, count: true},
		{name: "unticked", form: url.Values{"count_field": {"1"}}, count: false},
		{name: "a form without the box counts", form: url.Values{}, count: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			var got loot.Submission
			m.Loot.EXPECT().Record(mock.Anything, mock.Anything, tt.count).RunAndReturn(func(_ context.Context, s loot.Submission, _ bool) (loot.Outcome, error) {
				got = s
				return loot.Outcome{Status: loot.StatusUnmatched}, nil
			}).Once()
			form := tt.form
			form.Set("session", sample)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", form, nil)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "2026-09-25, 20:14:39", got.From)
			assert.Empty(t, got.SubmittedBy, "signed out")
			require.Len(t, got.Members, 3)
			assert.Equal(t, loot.Member{Name: "Killer Potato", Leader: true, Loot: 1816623, Supplies: 528367, Balance: 1288256, Damage: 4043228, Healing: 398456}, got.Members[2])
			entry := regexp.MustCompile(`(?s)<script id="loot-entry" type="application/json">(.*?)</script>`).FindStringSubmatch(rec.Body.String())
			require.Len(t, entry, 2)
			var saved historyEntry
			require.NoError(t, json.Unmarshal([]byte(entry[1]), &saved))
			assert.Equal(t, tt.count, saved.Count, "the history keeps the choice")
		})
	}
}

func TestHandleLootCalculate_SubmitsAsTheSignedInUser(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	var got loot.Submission
	m.Loot.EXPECT().Record(mock.Anything, mock.Anything, true).RunAndReturn(func(_ context.Context, s loot.Submission, _ bool) (loot.Outcome, error) {
		got = s
		return loot.Outcome{Status: loot.StatusUnmatched}, nil
	}).Once()

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "u1", got.SubmittedBy)
}

func TestHandleLootCalculate_ParseErrorKeepsTheChoiceAndRecordsNothing(t *testing.T) {
	// given
	d, _ := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/tools/loot-calculator", url.Values{"session": {"   "}, "count_field": {"1"}}, nil)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `name="count" value="1"`)
	assert.NotRegexp(t, `name="count" value="1"\s+checked`, body)
}

func TestHandleLootCalculator_CountBoxIsTickedByDefault(t *testing.T) {
	// given
	h := anonymous(t)

	// when
	rec := webtest.Serve(h, webtest.Get("/tools/loot-calculator", nil))

	// then
	body := rec.Body.String()
	assert.Contains(t, body, `name="count_field" value="1"`)
	assert.Regexp(t, `name="count" value="1"\s+checked`, body)
	assert.Contains(t, body, `aria-describedby="loot-count-note"`)
	assert.Contains(t, body, "Count this session in public stats")
	assert.Contains(t, body, "not the pasted text")
}

func TestHandleLootCalculate_PlainPostShowsTheNotice(t *testing.T) {
	// given
	h, _ := recording(t, loot.Outcome{Status: loot.StatusNoTime}, nil)

	// when
	rec := webtest.Serve(h, webtest.Post("/tools/loot-calculator", url.Values{"session": {sample}}, nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html")
	assert.Contains(t, rec.Body.String(), `data-loot-stats="no_time"`)
}

func TestLandingCalculator_HasTheCountBox(t *testing.T) {
	// given
	var out strings.Builder

	// when
	err := LandingCalculator().Render(context.Background(), &out)

	// then
	require.NoError(t, err)
	assert.Regexp(t, `name="count" value="1"\s+checked`, out.String())
}
