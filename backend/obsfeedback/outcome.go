package obsfeedback

import (
	"sort"
	"strings"
)

// EvaluateOutcome computes T+N future return vs optional benchmark.
// Missing future price → INSUFFICIENT/PENDING, never a fake win/loss.
// Missing benchmark → still EVALUATED with nil alpha (does not fail).
func EvaluateOutcome(rec Record, horizon int, stock []Bar, benchmark []Bar, benchmarkID string) Outcome {
	horizon = NormalizeHorizon(horizon)
	out := Outcome{
		ObservationID:   rec.ObservationID,
		Horizon:         horizon,
		Status:          OutcomeInsufficient,
		ObservationDate: rec.ObservationDate,
		BenchmarkID:     NormalizeBenchmark(benchmarkID),
	}
	if out.BenchmarkID == BenchmarkNone {
		out.BenchmarkID = ""
	}
	if rec.MarketPrice == nil || *rec.MarketPrice <= 0 {
		out.Note = "missing or invalid market_price"
		return out
	}
	obsPx := *rec.MarketPrice
	future, window, ok := nthTradingDayAfter(stock, rec.ObservationDate, horizon)
	if !ok || future.Close <= 0 {
		if len(filterAfter(stock, rec.ObservationDate)) == 0 {
			out.Status = OutcomePending
			out.Note = "no future kline yet"
		} else {
			out.Note = "insufficient future kline for horizon"
		}
		return out
	}
	ret := (future.Close - obsPx) / obsPx
	out.Status = OutcomeEvaluated
	out.FutureDate = future.Date
	out.FuturePrice = fptr(future.Close)
	out.FutureReturn = fptr(ret)
	if dd, profit, okHL := windowHighLow(window, obsPx); okHL {
		out.MaxDrawdown = fptr(dd)
		out.MaxProfit = fptr(profit)
	}
	if out.BenchmarkID == "" || len(benchmark) == 0 {
		if out.BenchmarkID != "" && len(benchmark) == 0 {
			out.Note = "benchmark kline missing; absolute return only"
		}
		return out
	}
	benchRet := benchmarkReturn(benchmark, rec.ObservationDate, future.Date)
	if benchRet == nil {
		out.Note = "benchmark kline missing; absolute return only"
		return out
	}
	out.BenchmarkReturn = benchRet
	out.Alpha = fptr(ret - *benchRet)
	return out
}

func filterAfter(bars []Bar, obsDate string) []Bar {
	obsDate = datePrefix(obsDate)
	out := make([]Bar, 0, len(bars))
	for _, b := range bars {
		d := datePrefix(b.Date)
		if d == "" || b.Close <= 0 {
			continue
		}
		if d > obsDate {
			b.Date = d
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func nthTradingDayAfter(bars []Bar, obsDate string, n int) (Bar, []Bar, bool) {
	after := filterAfter(bars, obsDate)
	if n <= 0 || len(after) < n {
		return Bar{}, after, false
	}
	return after[n-1], after[:n], true
}

func windowHighLow(window []Bar, obsPx float64) (drawdown, profit float64, ok bool) {
	if obsPx <= 0 || len(window) == 0 {
		return 0, 0, false
	}
	minLow, maxHigh := 0.0, 0.0
	first := true
	for _, b := range window {
		low, high := b.Low, b.High
		if low <= 0 {
			low = b.Close
		}
		if high <= 0 {
			high = b.Close
		}
		if low <= 0 || high <= 0 {
			continue
		}
		if first {
			minLow, maxHigh, first = low, high, false
			continue
		}
		if low < minLow {
			minLow = low
		}
		if high > maxHigh {
			maxHigh = high
		}
	}
	if first {
		return 0, 0, false
	}
	return minLow/obsPx - 1, maxHigh/obsPx - 1, true
}

func benchmarkReturn(bars []Bar, obsDate, futureDate string) *float64 {
	obsDate = datePrefix(obsDate)
	futureDate = datePrefix(futureDate)
	var start, end *Bar
	sorted := make([]Bar, 0, len(bars))
	for _, b := range bars {
		d := datePrefix(b.Date)
		if d == "" || b.Close <= 0 {
			continue
		}
		b.Date = d
		sorted = append(sorted, b)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	for i := range sorted {
		if sorted[i].Date <= obsDate {
			start = &sorted[i]
		}
		if sorted[i].Date == futureDate {
			end = &sorted[i]
		}
	}
	if start == nil || end == nil || start.Close <= 0 {
		return nil
	}
	v := (end.Close - start.Close) / start.Close
	return &v
}

// ParseKLineDay normalizes kline day strings to YYYY-MM-DD.
func ParseKLineDay(day string) string {
	day = strings.TrimSpace(day)
	day = strings.ReplaceAll(day, "/", "-")
	if len(day) >= 10 {
		return datePrefix(day[:10])
	}
	return ""
}
