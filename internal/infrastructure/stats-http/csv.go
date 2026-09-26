package statshttp

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"
)

// csv writes every row of a stats table. Figures are plain (no grouping, dot decimals) so a
// spreadsheet reads them; a figure without experience data is an empty cell, not 0.
func (h *Handlers) csv(w http.ResponseWriter, r *http.Request, kind tableKind, p pageView, rows []tableRow) {
	ctx := r.Context()
	name := "stats-" + string(kind)
	if n := len(p.Range.Days); n > 0 {
		name += "-" + p.Range.Days[0].Format(time.DateOnly) + "_" + p.Range.Days[n-1].Format(time.DateOnly)
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	out := csv.NewWriter(w)
	header := []string{nameHeader(ctx, kind)}
	for _, c := range figureColumns(ctx) {
		header = append(header, c.Label)
	}
	records := [][]string{header}
	for _, row := range rows {
		exp, expPerHour := "", ""
		if v := row.ExpTotal(); v != nil {
			exp = strconv.FormatInt(*v, 10)
		}
		if v := row.ExpPerHour(); v != nil {
			expPerHour = strconv.FormatFloat(*v, 'f', 0, 64)
		}
		records = append(records, []string{
			row.Name,
			strconv.FormatInt(row.Reservations, 10),
			strconv.FormatFloat(row.Hours(), 'f', 2, 64),
			exp,
			expPerHour,
		})
	}
	if err := out.WriteAll(records); err != nil {
		h.D.Log.Warnw("write stats csv", "error", err)
	}
}
