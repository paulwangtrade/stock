package signalattribution

import (
	"sort"

	"go-stock/backend/tradingcalendar"
)

// closesThroughAsOf returns contiguous session closes ending at asOf, oldest first.
// A calendar gap stops the walk. Bars after asOf are ignored.
func closesThroughAsOf(asOf string, byDate map[string]float64, cal tradingcalendar.Calendar) []float64 {
	if asOf == "" {
		return nil
	}
	if _, ok := byDate[asOf]; !ok {
		return nil
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		if d <= asOf {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	idx := -1
	for i, d := range dates {
		if d == asOf {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	start := idx
	for i := idx; i > 0; i-- {
		if !gapTrusted(dates[i-1], dates[i], cal) {
			break
		}
		start = i - 1
	}
	out := make([]float64, 0, idx-start+1)
	for _, d := range dates[start : idx+1] {
		out = append(out, byDate[d])
	}
	return out
}

func asOfFeatures(asOf string, hit HitInput, byDate map[string]float64, cal tradingcalendar.Calendar) AsOfFeatures {
	f := AsOfFeatures{
		Industry: hit.Industry,
		Market:   hit.Market,
		Tag:      hit.Tag,
	}
	if hit.HasVolumeRatio {
		f.VolumeRatio = floatPtr(hit.VolumeRatio)
	}
	if hit.HasRSI {
		f.RSI = floatPtr(hit.RSI)
	}
	closes := closesThroughAsOf(asOf, byDate, cal)
	n := len(closes)
	if n >= 21 {
		base := closes[n-1-20]
		last := closes[n-1]
		if base > 0 {
			r := (last - base) / base
			f.Prior20Return = floatPtr(r)
		}
	}
	if n >= 20 {
		var sum float64
		for _, c := range closes[n-20:] {
			sum += c
		}
		ma := sum / 20
		last := closes[n-1]
		if ma > 0 {
			f.DistMA20 = floatPtr((last - ma) / ma)
		}
	}
	return f
}
