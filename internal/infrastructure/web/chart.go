package web

import (
	"math"
	"strconv"
	"strings"
)

// Chart (chart.templ) is the app's only chart. The props below configure it and
// the helpers here do all of its geometry, so the template stays a dumb
// projection of already-computed percentages.
//
// #124: the plot SVG is preserveAspectRatio="none" (that is what makes the
// geometry span its column), which scales x and y independently, so ANY glyph
// inside it is stretched by containerWidth/viewBoxWidth. Every coordinate here is
// therefore a plain percentage of the plot box: the SVG holds only the
// polyline/area in a 0..100 square, and axis labels, gridlines, markers and hover
// targets are HTML boxes positioned over the same box at their natural font size.

// ChartKind selects the shape Chart draws.
type ChartKind int

const (
	// ChartLine is one or more trend lines over a shared x-axis.
	ChartLine ChartKind = iota
	// ChartBar is one labelled column per item.
	ChartBar
	// ChartSparkline is the tiny fixed-size inline trend, no axes or labels.
	ChartSparkline
)

// chartSurface is the page background, used as the fill behind a point marker
// and as the ring around the emphasised endpoint so it reads over the line.
const chartSurface = "#0A0A0B"

// ChartSeries is one line on a line chart (or the values of a sparkline). Name,
// when set, prefixes each point's hover tooltip. Tips, when set, replaces the
// composed tooltip text per point (index-aligned with Vals).
type ChartSeries struct {
	Name  string
	Color string // resolved #rrggbb
	Vals  []float64
	Tips  []string
}

// ChartBarItem is one bar on a bar chart.
type ChartBarItem struct {
	Name  string
	Color string
	Val   float64
}

// ChartProps configures Chart.
type ChartProps struct {
	Kind ChartKind
	// Series carries the lines (ChartLine) or the single value set of a
	// ChartSparkline; Bars carries the columns of a ChartBar.
	Series []ChartSeries
	Bars   []ChartBarItem
	// XLabels is one label per point, thinned out when crowded and reused in the
	// default tooltip text.
	XLabels []string
	// Height is the plot height in px (ChartLine/ChartBar); 0 means 200.
	Height int
	// Ticks is the number of y-axis gridline intervals; 0 means 4.
	Ticks int
	// Area softly fills under the line of a single-series ChartLine.
	Area bool
	// ZeroBase pulls the axis floor down to zero so a trend reads against a
	// stable baseline instead of its own minimum.
	ZeroBase bool
	// Decimals is the precision of tooltip values and bar figures.
	Decimals int
	// FormatY formats the y tick labels; nil derives the precision from the span.
	FormatY func(float64) string
	// EdgeXLabels labels only the first and last point (a sparkline-sized trend).
	EdgeXLabels bool
	// MaxXLabels caps how many x labels a long range thins down to; 0 means 8.
	// Lower it on a chart that sits in a narrow column.
	MaxXLabels int
	// Tip is the whole-chart tooltip text (ChartSparkline).
	Tip string
	// Label names the chart for screen readers.
	Label string
	// Class adds classes to the <figure>.
	Class string
}

// chartYTick is one y-axis gridline and its label.
type chartYTick struct {
	TopPct string
	Text   string
}

// chartXTick is one x-axis label. AnchorX is its horizontal translate (a CSS
// value, not a Tailwind class: Tailwind scans only .templ/_templ.go, so a class
// name computed here would be purged).
type chartXTick struct {
	LeftPct string
	Text    string
	AnchorX string
}

// style positions an x-axis label over the point it labels.
func (t chartXTick) style() string {
	return "left:" + t.LeftPct + "%;transform:translateX(" + t.AnchorX + ")"
}

// chartPoint is one plotted point: an HTML dot plus its hover target.
type chartPoint struct {
	LeftPct string
	TopPct  string
	Color   string
	Last    bool
	Tip     string
}

// chartLine is one rendered series: the SVG geometry plus its HTML markers.
type chartLine struct {
	Color   string
	Points  string // polyline points
	Area    string // polygon points; empty when the area is not filled
	Markers []chartPoint
}

// chartBar is one rendered column.
type chartBar struct {
	Name      string
	Color     string
	Value     string
	Tip       string
	HeightPct string
}

// chartSpark is the fixed 92x26 inline sparkline: it lives in its own box at its
// own size, so the default (uniform) preserveAspectRatio applies and nothing is
// distorted. It carries no text at all.
type chartSpark struct {
	Points string
	DotX   string
	DotY   string
	Color  string
	Tip    string
	Label  string
}

// chartPct formats a percentage for a style attribute or an SVG coordinate.
func chartPct(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// chartRange returns the padded [lo, hi] axis span for a set of values. With
// zeroBase the floor is pulled down to zero. A flat series is padded by a tenth
// of its magnitude (at least 1) so the range never has zero width.
func chartRange(vals []float64, zeroBase bool) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range vals {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if math.IsInf(lo, 1) { // no values
		return 0, 1
	}
	signed := lo < 0
	if zeroBase && lo > 0 {
		lo = 0
	}
	pad := (hi - lo) * 0.12
	if pad == 0 {
		pad = math.Max(math.Abs(hi)*0.1, 1)
	}
	hi += pad
	// Padding must not invent a negative axis under non-negative data, but data
	// that is genuinely signed keeps its negative floor: clamping that away would
	// put the point below the plot box instead of on it.
	if lo -= pad; lo < 0 && !signed {
		lo = 0
	}
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

// chartXPct is a point's horizontal position in the plot box, as a percentage. A
// lone point centres (an edge-hugging single dot reads as clipped).
func chartXPct(i, n int) float64 {
	if n <= 1 {
		return 50
	}
	return float64(i) / float64(n-1) * 100
}

// chartYPct is a value's vertical position in the plot box, as a percentage
// measured from the top (SVG and CSS both grow downwards).
func chartYPct(v, lo, hi float64) float64 {
	if hi == lo {
		return 50
	}
	return (1 - (v-lo)/(hi-lo)) * 100
}

// chartTicks returns the n+1 tick values spanning [lo, hi] inclusive.
func chartTicks(lo, hi float64, n int) []float64 {
	if n < 1 {
		n = 1
	}
	out := make([]float64, n+1)
	for t := 0; t <= n; t++ {
		out[t] = lo + float64(t)/float64(n)*(hi-lo)
	}
	return out
}

// chartAxisDecimals picks tick-label precision from the axis span, so a narrow
// range doesn't collapse into duplicate integers and a wide one doesn't carry
// noise decimals.
func chartAxisDecimals(lo, hi float64) int {
	switch span := hi - lo; {
	case span < 1:
		return 2
	case span < 10:
		return 1
	default:
		return 0
	}
}

// chartLabelStep is the stride between labelled x points, thinning them out to at
// most maxLabels so a month-long range doesn't crowd into an unreadable smear.
func chartLabelStep(n, maxLabels int) int {
	if maxLabels < 1 {
		maxLabels = 1
	}
	if n <= maxLabels {
		return 1
	}
	return int(math.Ceil(float64(n) / float64(maxLabels)))
}

// chartTip composes a point's hover text, "<series> · <label> · <value>",
// omitting the empty parts. Not HTML-escaped: templ escapes the attribute.
func chartTip(name, label string, v float64, decimals int) string {
	var b strings.Builder
	if name != "" {
		b.WriteString(name + " · ")
	}
	if label != "" {
		b.WriteString(label + " · ")
	}
	b.WriteString(strconv.FormatFloat(v, 'f', decimals, 64))
	return b.String()
}

// empty reports whether there is nothing to draw, so Chart renders no markup at
// all (callers gate their surrounding card on their own data).
func (p ChartProps) empty() bool {
	if p.Kind == ChartBar {
		return len(p.Bars) == 0
	}
	return p.points() == 0
}

// height is the plot height in px.
func (p ChartProps) height() int {
	if p.Height > 0 {
		return p.Height
	}
	return 200
}

// heightStyle is the plot box's inline height.
func (p ChartProps) heightStyle() string {
	return "height:" + strconv.Itoa(p.height()) + "px"
}

// maxXLabels is the cap on labelled x positions.
func (p ChartProps) maxXLabels() int {
	if p.MaxXLabels > 0 {
		return p.MaxXLabels
	}
	return 8
}

// ticks is the number of y-axis gridline intervals.
func (p ChartProps) ticks() int {
	if p.Ticks > 0 {
		return p.Ticks
	}
	return 4
}

// points is the longest series' length: the shared x-axis resolution, so series
// of different lengths still line up on the same days.
func (p ChartProps) points() int {
	n := 0
	for _, s := range p.Series {
		if len(s.Vals) > n {
			n = len(s.Vals)
		}
	}
	return n
}

// rng is the axis span: the padded value range for lines, zero-to-headroom for
// bars (a bar's length must be readable against a zero floor).
func (p ChartProps) rng() (lo, hi float64) {
	if p.Kind == ChartBar {
		return 0, p.barMax()
	}
	all := make([]float64, 0, p.points()*len(p.Series))
	for _, s := range p.Series {
		all = append(all, s.Vals...)
	}
	return chartRange(all, p.ZeroBase)
}

// barMax is the bar chart's axis ceiling: the tallest bar plus headroom for its
// value figure.
func (p ChartProps) barMax() float64 {
	highest := 0.0
	for _, it := range p.Bars {
		highest = math.Max(highest, it.Val)
	}
	if highest *= 1.15; highest == 0 {
		return 1
	}
	return highest
}

// yTicks are the gridlines and their labels, top to bottom. Label precision is
// derived from the same range that places the gridlines, so the two cannot drift.
func (p ChartProps) yTicks() []chartYTick {
	lo, hi := p.rng()
	dec := chartAxisDecimals(lo, hi)
	vals := chartTicks(lo, hi, p.ticks())
	out := make([]chartYTick, len(vals))
	for i, v := range vals {
		text := strconv.FormatFloat(v, 'f', dec, 64)
		if p.FormatY != nil {
			text = p.FormatY(v)
		}
		out[i] = chartYTick{TopPct: chartPct(chartYPct(v, lo, hi)), Text: text}
	}
	return out
}

// xTicks are the labelled x positions. They are picked from the last point
// backwards so the most recent day is always labelled, whatever the stride.
func (p ChartProps) xTicks() []chartXTick {
	n := p.points()
	if n == 0 || len(p.XLabels) == 0 {
		return nil
	}
	var idx []int
	if p.EdgeXLabels {
		idx = []int{0}
		if n > 1 {
			idx = append(idx, n-1)
		}
	} else {
		step := chartLabelStep(n, p.maxXLabels())
		for i := n - 1; i >= 0; i -= step {
			idx = append([]int{i}, idx...)
		}
	}
	out := make([]chartXTick, 0, len(idx))
	for _, i := range idx {
		if i >= len(p.XLabels) {
			continue
		}
		out = append(out, chartXTick{
			LeftPct: chartPct(chartXPct(i, n)),
			Text:    p.XLabels[i],
			AnchorX: chartXAnchor(i, n),
		})
	}
	return out
}

// chartXAnchor keeps the first and last labels inside the plot box (a centred
// label at 0% or 100% would hang half of itself outside it).
func chartXAnchor(i, n int) string {
	switch {
	case n <= 1:
		return "-50%"
	case i == 0:
		return "0"
	case i == n-1:
		return "-100%"
	default:
		return "-50%"
	}
}

// lines projects the series onto SVG geometry plus HTML markers.
func (p ChartProps) lines() []chartLine {
	lo, hi := p.rng()
	n := p.points()
	out := make([]chartLine, 0, len(p.Series))
	for _, s := range p.Series {
		if len(s.Vals) == 0 {
			continue
		}
		pts := make([]string, len(s.Vals))
		markers := make([]chartPoint, len(s.Vals))
		for i, v := range s.Vals {
			x, y := chartPct(chartXPct(i, n)), chartPct(chartYPct(v, lo, hi))
			pts[i] = x + "," + y
			markers[i] = chartPoint{
				LeftPct: x,
				TopPct:  y,
				Color:   s.Color,
				Last:    i == len(s.Vals)-1,
				Tip:     p.tipAt(s, i, v),
			}
		}
		l := chartLine{Color: s.Color, Points: strings.Join(pts, " "), Markers: markers}
		if p.Area && len(p.Series) == 1 {
			floor := chartPct(chartYPct(lo, lo, hi))
			l.Area = chartPct(chartXPct(0, n)) + "," + floor + " " + l.Points + " " +
				chartPct(chartXPct(len(s.Vals)-1, n)) + "," + floor
		}
		out = append(out, l)
	}
	return out
}

// tipAt is a point's hover text: the caller's own when it supplied one, else the
// composed "<series> · <label> · <value>".
func (p ChartProps) tipAt(s ChartSeries, i int, v float64) string {
	if i < len(s.Tips) {
		return s.Tips[i]
	}
	var label string
	if i < len(p.XLabels) {
		label = p.XLabels[i]
	}
	return chartTip(s.Name, label, v, p.Decimals)
}

// bars projects the items onto percent-height HTML columns.
func (p ChartProps) bars() []chartBar {
	lo, hi := p.rng()
	out := make([]chartBar, len(p.Bars))
	for i, it := range p.Bars {
		h := 100 - chartYPct(it.Val, lo, hi)
		if h < 0 {
			h = 0
		}
		out[i] = chartBar{
			Name:      it.Name,
			Color:     it.Color,
			Value:     strconv.FormatFloat(it.Val, 'f', p.Decimals, 64),
			Tip:       chartTip(it.Name, "", it.Val, p.Decimals),
			HeightPct: chartPct(h),
		}
	}
	return out
}

// spark projects the first series onto the sparkline's own fixed 92x26 box.
func (p ChartProps) spark() chartSpark {
	if len(p.Series) == 0 || len(p.Series[0].Vals) == 0 {
		return chartSpark{}
	}
	const w, h, pad = 92.0, 26.0, 3.0
	s := p.Series[0]
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range s.Vals {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	x := func(i int) string {
		return chartPct(pad + chartXPct(i, len(s.Vals))/100*(w-2*pad))
	}
	y := func(v float64) string {
		return chartPct(pad + chartYPct(v, lo, hi)/100*(h-2*pad))
	}
	pts := make([]string, len(s.Vals))
	for i, v := range s.Vals {
		pts[i] = x(i) + "," + y(v)
	}
	last := len(s.Vals) - 1
	return chartSpark{
		Points: strings.Join(pts, " "),
		DotX:   x(last),
		DotY:   y(s.Vals[last]),
		Color:  s.Color,
		Tip:    p.Tip,
		Label:  p.Label,
	}
}

// dotStyle positions and paints a point marker: a small hollow dot on the plot
// surface, the endpoint filled with the series colour and ringed in the surface
// colour so it stays legible where the line crosses it.
func (m chartPoint) dotStyle() string {
	size, fill, ring := "6px", chartSurface, m.Color
	if m.Last {
		size, fill, ring = "9px", m.Color, chartSurface
	}
	return "left:" + m.LeftPct + "%;top:" + m.TopPct + "%" +
		";width:" + size + ";height:" + size +
		";background:" + fill + ";border:1.5px solid " + ring
}

// tipStyle positions a point's hover target.
func (m chartPoint) tipStyle() string {
	return "left:" + m.LeftPct + "%;top:" + m.TopPct + "%"
}

// fillStyle sizes and paints a bar.
func (b chartBar) fillStyle() string {
	return "height:" + b.HeightPct + "%;background:" + b.Color
}
