package statshttp

import (
	"encoding/csv"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/stats"
)

// csv writes the loaded rows of a server's stats table.
func (h *Handlers) csv(w http.ResponseWriter, r *http.Request, kind tableKind, p pageView, rows []tableRow) {
	ctx := r.Context()
	h.writeCSV(w, "stats-"+string(kind)+rangeSuffix(p.Range), nameHeader(ctx, kind), guildColumns(ctx), rows)
}

// writeCSV writes rows as a CSV download. Figures are plain (no grouping, dot decimals) so a
// spreadsheet reads them; a figure without data is an empty cell, not 0.
func (h *Handlers) writeCSV(w http.ResponseWriter, filename, name string, cols []column, rows []tableRow) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
	header := make([]string, 0, 1+len(cols))
	header = append(header, name)
	for _, c := range cols {
		header = append(header, c.Label)
	}
	records := make([][]string, 0, 1+len(rows))
	records = append(records, header)
	for _, row := range rows {
		record := make([]string, 0, 1+len(cols))
		record = append(record, csvText(row.Name))
		for _, c := range cols {
			record = append(record, csvFigure(c.Key, row))
		}
		records = append(records, record)
	}
	if err := csv.NewWriter(w).WriteAll(records); err != nil {
		h.D.Log.Warnw("write stats csv", "error", err)
	}
}

func csvFigure(key stats.SortKey, row tableRow) string {
	whole := func(v *int64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatInt(*v, 10)
	}
	perHour := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(math.Round(*v), 'f', 0, 64)
	}
	switch key {
	case stats.SortWorlds:
		return strconv.Itoa(row.Worlds)
	case stats.SortReservations:
		return strconv.FormatInt(row.Reservations, 10)
	case stats.SortHours:
		return strconv.FormatFloat(row.Hours(), 'f', 2, 64)
	case stats.SortExp:
		return whole(row.ExpTotal())
	case stats.SortExpPerHour:
		return perHour(row.ExpPerHour())
	case stats.SortProfit:
		return whole(row.ProfitTotal())
	case stats.SortProfitPerHour:
		return perHour(row.ProfitPerHour())
	default:
		return ""
	}
}

// rangeSuffix is "-<first day>_<last day>" for a file name.
func rangeSuffix(rng stats.Range) string {
	n := len(rng.Days)
	if n == 0 {
		return ""
	}
	return "-" + rng.Days[0].Format(time.DateOnly) + "_" + rng.Days[n-1].Format(time.DateOnly)
}

// csvText defuses a spreadsheet formula: author names are free text, and a leading = + - @ (or a
// tab / carriage return) makes Excel and LibreOffice evaluate the cell.
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
