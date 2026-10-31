package summary

import (
	"math"
	"sort"

	"spot-assistant/internal/common/collections"
	dto "spot-assistant/internal/core/dto/summary"
)

// maxChartRespawns defines the amount of respawns isolated on the chart.
// Any n-th where n is > maxChartRespawns is aggregated into "Other".
const maxChartRespawns = 14

// Image file binary
type ChartImage []byte

// MapToLegendValues maps any string-keyed float64-valued map to []dto.LegendValue,
// where each element holds legend (key) and amount of elements of the map for this
// given key. If there are many results, it will truncate to maxChartRespawns.
func (a *Adapter) mapToLegendValues(m map[string]float64) []dto.LegendValue {
	result := make([]dto.LegendValue, len(m))
	index := 0
	for key, val := range m {
		result[index] = dto.LegendValue{Legend: key, Value: val}
		index++
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Value > result[j].Value
	})

	truncatedResultLength := int(math.Min(
		float64(len(m)), float64(maxChartRespawns),
	))
	truncatedResult := make([]dto.LegendValue, 0, truncatedResultLength+1)

	// Container for optional "Other" results, if any
	var otherResults *dto.LegendValue
	for i, res := range result {
		// Top maxChartRespawns results go in. Any results that would
		// exceed the limit will be aggregated into "Other" results container
		if i < truncatedResultLength {
			truncatedResult = append(truncatedResult, res)

			continue
		}

		if otherResults == nil {
			otherResults = &dto.LegendValue{
				Legend: "Other",
			}
		}

		otherResults.Value += res.Value
	}

	// Other results exist, so add them
	if otherResults != nil {
		truncatedResult = append(truncatedResult, *otherResults)
	}

	return truncatedResult
}

// NewChart a chart image and an optional error generated for
// a set of dto.LegendValue s.
func (a *Adapter) newChart(lvs []dto.LegendValue) (ChartImage, error) {
	values := collections.PoorMansMap(lvs, func(lv dto.LegendValue) float64 {
		return lv.Value
	})
	legend := collections.PoorMansMap(lvs, func(lv dto.LegendValue) string {
		return lv.Legend
	})

	img, err := a.service.NewChart(values, legend)
	if err != nil {
		return nil, err
	}

	return img, nil
}
