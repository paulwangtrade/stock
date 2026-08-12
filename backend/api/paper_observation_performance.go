package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/obsfeedback"
	"go-stock/backend/portfolioobs"
)

func (h *PaperObservationHandler) handleObservationPerformance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	horizon, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("horizon")))
	horizon = obsfeedback.NormalizeHorizon(horizon)
	benchmark := obsfeedback.NormalizeBenchmark(r.URL.Query().Get("benchmark"))

	records, err := obsfeedback.LoadRecords()
	if err != nil {
		records = []obsfeedback.Record{}
	}

	stockBars := map[string][]obsfeedback.Bar{}
	var benchBars []obsfeedback.Bar
	if len(records) > 0 {
		provider := performanceSeriesProvider()
		minDate := ""
		seen := map[string]struct{}{}
		for _, rec := range records {
			if rec.ObservationDate != "" && (minDate == "" || rec.ObservationDate < minDate) {
				minDate = rec.ObservationDate
			}
			seen[rec.Symbol] = struct{}{}
		}
		for symbol := range seen {
			bars, _ := provider.DailyBars(symbol, minDate, 80)
			stockBars[symbol] = bars
		}
		if code := obsfeedback.BenchmarkSymbol(benchmark); code != "" {
			benchBars, _ = provider.DailyBars(code, minDate, 80)
		}
	}

	outcomes := obsfeedback.EvaluateAll(records, horizon, stockBars, benchBars, benchmark)
	view := obsfeedback.AggregateMetrics(records, outcomes, horizon, benchmark)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":        0,
		"ok":          true,
		"disclaimer":  obsfeedback.Disclaimer,
		"performance": view,
	})
}

var performanceProviderForTest obsfeedback.SeriesProvider

func performanceSeriesProvider() obsfeedback.SeriesProvider {
	if performanceProviderForTest != nil {
		return performanceProviderForTest
	}
	return obsfeedback.NewKLineSeriesProvider()
}

// SetPerformanceSeriesProviderForTest injects bars for API tests.
func SetPerformanceSeriesProviderForTest(p obsfeedback.SeriesProvider) {
	performanceProviderForTest = p
}

// captureObservationRecords persists today's Decision snapshot outside paper_sim_*.
// Skipped under go test so portfolio observation tests stay isolated.
func captureObservationRecords(obs *portfolioobs.Observation) {
	if obsfeedback.RunningUnderTest() || obs == nil {
		return
	}
	recs := obsfeedback.CaptureFromObservation(obs)
	_ = obsfeedback.PersistCapture(recs)
}
