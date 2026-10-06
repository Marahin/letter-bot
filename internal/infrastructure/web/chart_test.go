package web

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderChart renders the atom to markup, for the "what actually reaches the
// browser" assertions (the regression guard for #124 is one of them).
func renderChart(t *testing.T, p ChartProps) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, Chart(p).Render(context.Background(), &sb))
	return sb.String()
}

func TestChartRange_PadsAndZeroBases(t *testing.T) {
	// given a rising series with zeroBase, then the floor is pulled to zero and
	// the ceiling gets headroom above the maximum.
	lo, hi := chartRange([]float64{1, 2, 1.5}, true)
	assert.Equal(t, 0.0, lo)
	assert.Greater(t, hi, 2.0)

	// without zeroBase a series well above zero still gets a padded floor, but
	// the padding can't push it below zero.
	lo, hi = chartRange([]float64{100, 200}, false)
	assert.Greater(t, lo, 0.0)
	assert.Less(t, lo, 100.0)
	assert.Greater(t, hi, 200.0)
}

func TestChartRange_FlatSeriesDoesNotDivideByZero(t *testing.T) {
	// given every value identical, then the span still has width, padded in
	// proportion to the magnitude (a price series near 500 must not get a ±1 span).
	lo, hi := chartRange([]float64{500, 500}, false)
	assert.Greater(t, hi-lo, 1.0)
	assert.Equal(t, 450.0, lo)
	assert.Equal(t, 550.0, hi)

	// an all-zero series (a stat nobody scored) falls back to a unit span.
	lo, hi = chartRange([]float64{0, 0, 0}, true)
	assert.Equal(t, 0.0, lo)
	assert.Equal(t, 1.0, hi)
}

func TestChartRange_NoValues(t *testing.T) {
	lo, hi := chartRange(nil, true)
	assert.Equal(t, 0.0, lo)
	assert.Equal(t, 1.0, hi)
}

func TestChartXPct_SinglePointCentres(t *testing.T) {
	// a lone point centres rather than hugging an edge, where it would read as clipped.
	assert.Equal(t, 50.0, chartXPct(0, 1))
	assert.Equal(t, 50.0, chartXPct(0, 0))
	// otherwise points spread across the full plot width.
	assert.Equal(t, 0.0, chartXPct(0, 3))
	assert.Equal(t, 50.0, chartXPct(1, 3))
	assert.Equal(t, 100.0, chartXPct(2, 3))
}

func TestChartYPct_InvertsAndSurvivesAFlatSpan(t *testing.T) {
	// percentages grow downwards, so the axis ceiling is 0%.
	assert.Equal(t, 0.0, chartYPct(10, 0, 10))
	assert.Equal(t, 100.0, chartYPct(0, 0, 10))
	assert.Equal(t, 50.0, chartYPct(5, 0, 10))
	// a zero-width span centres instead of producing NaN.
	assert.Equal(t, 50.0, chartYPct(3, 3, 3))
}

func TestChartTicks(t *testing.T) {
	// n intervals yield n+1 tick values spanning the range inclusively.
	assert.Equal(t, []float64{0, 0.5, 1}, chartTicks(0, 1, 2))
	assert.Len(t, chartTicks(0, 8, 4), 5)
	// a nonsensical interval count still yields a usable pair.
	assert.Equal(t, []float64{0, 4}, chartTicks(0, 4, 0))
}

func TestChartAxisDecimals(t *testing.T) {
	// narrow spans keep decimals so ticks don't collapse into duplicates.
	assert.Equal(t, 2, chartAxisDecimals(0, 0.4))
	assert.Equal(t, 1, chartAxisDecimals(0, 2.24))
	assert.Equal(t, 0, chartAxisDecimals(0, 18))
}

func TestChartLabelStep(t *testing.T) {
	// up to eight points every one is labelled; beyond that they thin out.
	assert.Equal(t, 1, chartLabelStep(3, 8))
	assert.Equal(t, 1, chartLabelStep(8, 8))
	assert.Equal(t, 2, chartLabelStep(9, 8))
	assert.Equal(t, 4, chartLabelStep(31, 8))
	// a narrower column asks for fewer labels, so the stride grows.
	assert.Equal(t, 8, chartLabelStep(31, 4))
	assert.Equal(t, 31, chartLabelStep(31, 0))
}

func TestChartTip_Composition(t *testing.T) {
	// given a series name and a day label, then both prefix the value.
	assert.Equal(t, "Alpha · 01 Jun · 1.00", chartTip("Alpha", "01 Jun", 1, 2))
	// empty parts are omitted.
	assert.Equal(t, "01 Jun · 16", chartTip("", "01 Jun", 16, 0))
	assert.Equal(t, "1.50", chartTip("", "", 1.5, 2))
	// no HTML escaping here: templ escapes the attribute it lands in.
	assert.Equal(t, "A<b> · 1.50", chartTip("A<b>", "", 1.5, 2))
}

func TestChartXTicks_LabelsTheLastPointAndAnchorsTheEdges(t *testing.T) {
	labels := make([]string, 31)
	for i := range labels {
		labels[i] = "d" + string(rune('a'+i%26))
	}
	ticks := ChartProps{Series: []ChartSeries{{Vals: make([]float64, 31)}}, XLabels: labels}.xTicks()

	require.NotEmpty(t, ticks)
	// the most recent day is always labelled and right-anchored so it stays inside
	// the plot box.
	last := ticks[len(ticks)-1]
	assert.Equal(t, "100.000", last.LeftPct)
	assert.Equal(t, "transform:translateX(-100%)", strings.SplitN(last.style(), ";", 2)[1], "the last label is right-anchored so it stays inside the plot")
	// and 31 days thin out to a readable handful.
	assert.LessOrEqual(t, len(ticks), 9)
}

func TestChartXTicks_EdgeOnly(t *testing.T) {
	p := ChartProps{
		Series:      []ChartSeries{{Vals: []float64{1, 2, 3, 4}}},
		XLabels:     []string{"Jul 15", "Jul 16", "Jul 17", "Jul 18"},
		EdgeXLabels: true,
	}
	ticks := p.xTicks()
	require.Len(t, ticks, 2)
	assert.Equal(t, "Jul 15", ticks[0].Text)
	assert.Equal(t, "left:0.000%;transform:translateX(0)", ticks[0].style())
	assert.Equal(t, "Jul 18", ticks[1].Text)

	// no labels at all means no label band.
	assert.Empty(t, ChartProps{Series: []ChartSeries{{Vals: []float64{1}}}}.xTicks())
}

func TestChartProps_Defaults(t *testing.T) {
	p := ChartProps{}
	assert.Equal(t, 200, p.height())
	assert.Equal(t, "height:200px", p.heightStyle())
	assert.Equal(t, 4, p.ticks())
	assert.Equal(t, 130, ChartProps{Height: 130}.height())
	assert.Equal(t, 2, ChartProps{Ticks: 2}.ticks())
}

func TestChartProps_TipAtPrefersTheCallersOwnText(t *testing.T) {
	p := ChartProps{XLabels: []string{"01 Jun"}, Decimals: 2}
	s := ChartSeries{Name: "KDA", Tips: []string{"Jul 15 12:00 · 1,500,000"}}
	assert.Equal(t, "Jul 15 12:00 · 1,500,000", p.tipAt(s, 0, 1500000))
	// beyond the supplied tips it falls back to the composed text.
	assert.Equal(t, "KDA · 2.00", p.tipAt(s, 1, 2))
}

// TestChart_Line_PutsNoTextInTheSVG is the #124 regression guard: the plot SVG is
// non-uniformly scaled, so any glyph inside it would be stretched by the
// container width. Every label must be an HTML box instead.
func TestChart_Line_PutsNoTextInTheSVG(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:     ChartLine,
		Series:   []ChartSeries{{Name: "Alpha", Color: "#F97316", Vals: []float64{1, 2, 1.5}}},
		XLabels:  []string{"01 Jun", "02 Jun", "03 Jun"},
		Area:     true,
		Decimals: 2,
		ZeroBase: true,
		Label:    "Average KDA per squad",
	})

	assert.NotContains(t, out, "<text", "no glyphs inside the stretched plot SVG")
	assert.NotContains(t, out, "font-size=", "no SVG font attributes at all")
	// the geometry is still SVG, in a normalised percent box with a stroke that
	// doesn't thicken with the container.
	assert.Contains(t, out, `viewBox="0 0 100 100"`)
	assert.Contains(t, out, `preserveAspectRatio="none"`)
	assert.Contains(t, out, `vector-effect="non-scaling-stroke"`)
	assert.Contains(t, out, "<polyline")
	assert.Contains(t, out, "<polygon", "Area fills under a single series")
	assert.Contains(t, out, "#F97316")
	// labels are HTML text nodes at their natural font size.
	assert.Contains(t, out, "data-chart-label")
	assert.Contains(t, out, ">01 Jun<")
	assert.Contains(t, out, ">03 Jun<")
	// one hover target per point, and no SVG circles to be squashed into ellipses.
	assert.Equal(t, 3, strings.Count(out, "data-chart-tip="))
	assert.NotContains(t, out, "<circle")
	assert.Contains(t, out, "Alpha · 01 Jun · 1.00")
	assert.Contains(t, out, "<figcaption")
}

func TestChart_Line_EscapesSeriesNames(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:     ChartLine,
		Series:   []ChartSeries{{Name: "A<b>", Color: "#2ecc71", Vals: []float64{1}}},
		Decimals: 2,
	})
	assert.NotContains(t, out, "A<b>", "templ escapes the attribute; the value is never pre-escaped")
	assert.Contains(t, out, "A&lt;b&gt; · 1.00")
}

func TestChart_Line_MultiSeriesShareOneXAxis(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind: ChartLine,
		Series: []ChartSeries{
			{Name: "Alpha", Color: "#F97316", Vals: []float64{1, 2, 3}},
			{Name: "Bravo", Color: "#38BDF8", Vals: []float64{3, 2}},
		},
		XLabels:  []string{"01 Jun", "02 Jun", "03 Jun"},
		Area:     true,
		Decimals: 2,
	})
	assert.Equal(t, 2, strings.Count(out, "<polyline"))
	assert.NotContains(t, out, "<polygon", "the area fill is for a single series only")
	// the shorter series still uses the shared three-point x-axis, so day 2 lands
	// halfway across rather than at the right edge.
	assert.Contains(t, out, "Bravo · 02 Jun · 2.00")
	assert.NotContains(t, out, "NaN")
}

func TestChart_Bar_IsPureHTML(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:     ChartBar,
		Bars:     []ChartBarItem{{Name: "A<b>", Color: "#2ecc71", Val: 1.5}},
		Height:   220,
		Decimals: 2,
		Label:    "Average KDA per squad",
	})

	assert.NotContains(t, out, "<svg", "bars need no SVG, so nothing can be distorted")
	assert.NotContains(t, out, "A<b>", "the squad name is escaped by templ")
	assert.Contains(t, out, "A&lt;b&gt;")
	assert.Contains(t, out, ">1.50<", "the value figure sits above the bar")
	assert.Contains(t, out, "background:#2ecc71")
	assert.Equal(t, 1, strings.Count(out, "data-chart-tip="))
	assert.Contains(t, out, "data-chart-label")
}

func TestChart_Bar_HeightsAreProportionalWithHeadroom(t *testing.T) {
	bars := ChartProps{Kind: ChartBar, Bars: []ChartBarItem{
		{Name: "Alpha", Val: 2},
		{Name: "Bravo", Val: 1},
		{Name: "Charlie", Val: 0},
	}}.bars()

	require.Len(t, bars, 3)
	// the tallest bar keeps headroom for its value figure, the rest scale against it.
	assert.Equal(t, "86.957", bars[0].HeightPct)
	assert.Equal(t, "43.478", bars[1].HeightPct)
	assert.Equal(t, "0.000", bars[2].HeightPct)
}

func TestChart_Sparkline_IsFixedSizeAndUndistorted(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:   ChartSparkline,
		Series: []ChartSeries{{Color: "#FB923C", Vals: []float64{0.5, 1.2, 0.9}}},
		Tip:    "latest 0.90",
		Label:  "KDA trend",
	})

	assert.Contains(t, out, `width="92" height="26"`, "its own box at its own size")
	assert.NotContains(t, out, `preserveAspectRatio="none"`, "the uniform default keeps it square")
	assert.NotContains(t, out, "<text")
	assert.Contains(t, out, "<polyline")
	assert.Contains(t, out, "#FB923C")
	assert.Equal(t, 1, strings.Count(out, "data-chart-tip="), "one whole-chart hover target")
	assert.Contains(t, out, "latest 0.90")
}

func TestChart_Sparkline_FlatSeriesDoesNotProduceNaN(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:   ChartSparkline,
		Series: []ChartSeries{{Color: "#FB923C", Vals: []float64{0, 0}}},
	})
	assert.NotContains(t, out, "NaN")
	assert.Contains(t, out, "<polyline")
}

func TestChart_Empty_RendersNothing(t *testing.T) {
	// callers gate their surrounding card on their own data, so the atom must be
	// silent when there is nothing to draw.
	assert.Empty(t, renderChart(t, ChartProps{Kind: ChartLine}))
	assert.Empty(t, renderChart(t, ChartProps{Kind: ChartLine, Series: []ChartSeries{{Color: "#fff"}}}))
	assert.Empty(t, renderChart(t, ChartProps{Kind: ChartBar}))
	assert.Empty(t, renderChart(t, ChartProps{Kind: ChartSparkline}))
}

func TestChart_FormatY_UsedForTicks(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:    ChartLine,
		Series:  []ChartSeries{{Color: "#F59E0B", Vals: []float64{12500, 1400000}}},
		Ticks:   2,
		FormatY: func(v float64) string { return "≈" + compactAxisStub(v) },
	})
	assert.Equal(t, 3, strings.Count(out, "≈"), "one label per tick, all through FormatY")
}

// compactAxisStub stands in for a caller's own axis formatter (admin-http's
// compactPrice) so the atom's test stays free of that package.
func compactAxisStub(v float64) string {
	if v >= 1000 {
		return "big"
	}
	return "small"
}

func TestChart_Class_ReachesTheFigure(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind:   ChartLine,
		Series: []ChartSeries{{Color: "#fff", Vals: []float64{1, 2}}},
		Class:  "mt-4",
	})
	assert.Contains(t, out, "mt-4")
	assert.Contains(t, out, "<figure data-chart")
}

func TestChartXTicks_SinglePointAndShortLabelList(t *testing.T) {
	// a lone labelled point centres.
	ticks := ChartProps{Series: []ChartSeries{{Vals: []float64{2}}}, XLabels: []string{"01 Jun"}}.xTicks()
	require.Len(t, ticks, 1)
	assert.Equal(t, "left:50.000%;transform:translateX(-50%)", ticks[0].style())

	// fewer labels than points: the unlabelled tail is skipped, not indexed past it.
	short := ChartProps{Series: []ChartSeries{{Vals: []float64{1, 2, 3}}}, XLabels: []string{"01 Jun"}}
	shortTicks := short.xTicks()
	require.Len(t, shortTicks, 1)
	assert.Equal(t, "01 Jun", shortTicks[0].Text)

	// a caller in a narrow column asks for fewer labels.
	narrow := ChartProps{
		Series:     []ChartSeries{{Vals: make([]float64, 31)}},
		XLabels:    make([]string, 31),
		MaxXLabels: 4,
	}
	assert.Equal(t, 4, narrow.maxXLabels())
	assert.LessOrEqual(t, len(narrow.xTicks()), 4)
}

func TestChartProps_BarsClampNegativeValues(t *testing.T) {
	// a negative figure can't draw a bar below the floor; it flattens to zero.
	bars := ChartProps{Kind: ChartBar, Bars: []ChartBarItem{{Name: "Alpha", Val: -3}}}.bars()
	require.Len(t, bars, 1)
	assert.Equal(t, "0.000", bars[0].HeightPct)
}

func TestChartProps_SparkIsEmptyWithoutValues(t *testing.T) {
	assert.Empty(t, ChartProps{Kind: ChartSparkline}.spark().Points)
	assert.Empty(t, ChartProps{Kind: ChartSparkline, Series: []ChartSeries{{}}}.spark().Points)
}

func TestChartRange_AllNegativeStillHasWidth(t *testing.T) {
	// given a flat all-negative series, when the range is taken, then it keeps its
	// own signed span rather than collapsing onto a zero floor.
	lo, hi := chartRange([]float64{-5, -5}, false)
	assert.Equal(t, -6.0, lo)
	assert.Equal(t, -4.0, hi)
}

func TestChartRange_ZeroFloorOnlyForNonNegativeData(t *testing.T) {
	// given non-negative data, when padding would push the floor below zero, then
	// it is clamped: an axis reading below zero for a count is nonsense.
	lo, _ := chartRange([]float64{0, 10}, false)
	assert.Equal(t, 0.0, lo)

	// given genuinely signed data, when the range is taken, then the floor stays
	// negative so the lowest point lands inside the plot box, not below it.
	lo, hi := chartRange([]float64{-5, -1, 3}, false)
	assert.Negative(t, lo)
	assert.LessOrEqual(t, chartYPct(-5, lo, hi), 100.0,
		"a signed low point must sit within the 0-100%% plot box")
}

func TestChart_Line_KeepsSignedPointsInsideThePlot(t *testing.T) {
	// #124 follow-up: a clamped floor drew the point at 226% of the plot height,
	// i.e. outside the figure entirely (overflow-visible does not clip it).
	out := renderChart(t, ChartProps{
		Kind:    ChartLine,
		Series:  []ChartSeries{{Name: "Net", Color: "#F97316", Vals: []float64{-5, -1, 3}}},
		XLabels: []string{"a", "b", "c"},
	})
	for _, coord := range regexp.MustCompile(`\d+\.\d+,(-?\d+\.\d+)`).FindAllStringSubmatch(out, -1) {
		y, err := strconv.ParseFloat(coord[1], 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, y, 0.0, "point above the plot box in %q", coord[0])
		assert.LessOrEqual(t, y, 100.0, "point below the plot box in %q", coord[0])
	}
}

func TestChart_Line_SkipsAnEmptySeries(t *testing.T) {
	out := renderChart(t, ChartProps{
		Kind: ChartLine,
		Series: []ChartSeries{
			{Name: "Alpha", Color: "#F97316", Vals: []float64{1, 2}},
			{Name: "Empty", Color: "#38BDF8"}, // a squad with no recorded days
		},
		Decimals: 2,
	})
	assert.Equal(t, 1, strings.Count(out, "<polyline"))
	assert.NotContains(t, out, "Empty ·")
}

func TestChart_Line_PartialLastDashesTheLastSegment(t *testing.T) {
	// given
	p := ChartProps{
		Kind:        ChartLine,
		Series:      []ChartSeries{{Color: "#F97316", Vals: []float64{4, 6, 1}}},
		Area:        true,
		PartialLast: true,
	}

	// when
	lines := p.lines()
	out := renderChart(t, p)

	// then
	require.Len(t, lines, 1)
	assert.Equal(t, 2, strings.Count(lines[0].Points, ","), "the solid line stops one point early")
	assert.Equal(t, 2, strings.Count(lines[0].Dashed, ","), "the dashed segment joins the last two points")
	assert.Equal(t, 5, strings.Count(lines[0].Area, ","), "the area still covers every point")
	assert.True(t, lines[0].Markers[2].Partial)
	assert.Contains(t, out, "data-chart-partial")
	assert.Contains(t, out, `stroke-dasharray="4 4"`)
	assert.Contains(t, lines[0].Markers[2].dotStyle(), "background:"+chartSurface, "a partial point is hollow")
}

func TestChart_Line_PartialLastNeedsTwoPoints(t *testing.T) {
	// given
	p := ChartProps{Kind: ChartLine, Series: []ChartSeries{{Color: "#F97316", Vals: []float64{4}}}, PartialLast: true}

	// when
	out := renderChart(t, p)

	// then
	assert.NotContains(t, out, "data-chart-partial")
}

func TestChart_HBar_ListsNamesBesideBars(t *testing.T) {
	// given
	p := ChartProps{
		Kind: ChartHBar,
		Bars: []ChartBarItem{
			{Name: "Grim Reaper Yalahar -1 (long name)", Color: "#FB923C", Val: 10, Text: "10,0 h", Href: "/servers/1/stats/spots/4"},
			{Name: "Hero Cave", Color: "#FB923C", Val: 5},
		},
		Decimals: 1,
		Label:    "Busiest respawns",
	}

	// when
	out := renderChart(t, p)
	bars := p.bars()

	// then
	assert.Contains(t, out, ">Grim Reaper Yalahar -1 (long name)<")
	assert.Contains(t, out, `href="/servers/1/stats/spots/4"`)
	assert.Contains(t, out, ">10,0 h<", "the caller's text replaces the figure")
	assert.Contains(t, out, ">5.0<")
	assert.Contains(t, out, "Hero Cave · 5.0")
	assert.NotContains(t, out, "<svg")
	assert.Equal(t, "width:"+bars[0].HeightPct+"%;background:#FB923C", bars[0].hFillStyle())
	assert.Greater(t, bars[0].HeightPct, bars[1].HeightPct)
}
