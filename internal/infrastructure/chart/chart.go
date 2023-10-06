package chart

import "github.com/vicanso/go-charts/v2"

type Adapter struct{}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) NewChart(values []float64, legend []string) ([]byte, error) {
	f := false
	p, err := charts.PieRender(
		values,
		charts.TitleOptionFunc(charts.TitleOption{
			Text: "Current reservations",
			Left: charts.PositionCenter,
		}),
		charts.PaddingOptionFunc(charts.Box{
			Top:    20,
			Left:   20,
			Right:  20,
			Bottom: 20,
		}),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: &f,
			Data: legend,
		}),
		charts.PieSeriesShowLabel(),
		charts.ThemeOptionFunc(charts.ThemeDark),
	)
	if err != nil {
		return nil, err
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, err
	}

	return buf, nil
}
