package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestObservationMetrics_Case1_LegacyPlan26_ExcludedFromBOpen(t *testing.T) {
	filledAt := time.Date(2026, 8, 5, 15, 14, 20, 0, time.Local)
	run := papertrading.PaperSimRun{
		ID: 2, PlanID: 26, TradeDate: "2026-08-05",
		StartedAt: time.Date(2026, 8, 5, 15, 14, 19, 0, time.Local),
	}
	fills := []papertrading.PaperSimFill{}
	for id := uint(6); id <= 10; id++ {
		fills = append(fills, papertrading.PaperSimFill{
			ID: id, PlanID: 26, OrderID: id,
			FillReason: papertrading.FillReasonMarketOpen,
			FilledAt:   filledAt.Add(time.Duration(id) * time.Millisecond),
		})
	}
	td := map[uint]string{}
	for _, f := range fills {
		td[f.ID] = "2026-08-05"
	}
	labels := papertrading.LabelFillsForMetrics(fills, []papertrading.PaperSimRun{run}, td)
	m := papertrading.AggregateObservationMetrics([]papertrading.PaperSimRun{run}, labels)

	require.Equal(t, 1, m.TotalRuns)
	require.Equal(t, 1, m.SessionDistribution.SessionB)
	require.Greater(t, m.Legacy.LegacyBaselineCount, 0)
	require.Equal(t, 5, m.Legacy.ExcludedFillCount)
	require.Equal(t, 5, m.Quality.LegacyCount)
	// Legacy B+open must NOT inflate compliance violation counters.
	require.Equal(t, 0, m.FillPolicy.BWindowOpenFills)
	require.Equal(t, 0, m.FillPolicy.BWindowTotal)
	require.Equal(t, 5, m.FillPolicy.MarketOpenFills) // raw count still visible
	require.InDelta(t, 1.0, m.PricePolicyCompliance, 1e-9)
}

func TestObservationMetrics_Case2_PostCutover_B_Open_Anomaly(t *testing.T) {
	filledAt := time.Date(2026, 8, 6, 15, 10, 0, 0, time.Local)
	run := papertrading.PaperSimRun{
		ID: 50, PlanID: 99, TradeDate: "2026-08-06", StartedAt: filledAt,
	}
	fill := papertrading.PaperSimFill{
		ID: 100, PlanID: 99, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt,
	}
	labels := papertrading.LabelFillsForMetrics(
		[]papertrading.PaperSimFill{fill},
		[]papertrading.PaperSimRun{run},
		map[uint]string{100: "2026-08-06"},
	)
	m := papertrading.AggregateObservationMetrics([]papertrading.PaperSimRun{run}, labels)

	require.Equal(t, 1, m.FillPolicy.BWindowOpenFills)
	require.Equal(t, 1, m.FillPolicy.BWindowViolation)
	require.Equal(t, 1, m.Quality.AnomalyCount)
	require.Equal(t, 0, m.Legacy.ExcludedFillCount)
	require.InDelta(t, 0.0, m.PricePolicyCompliance, 1e-9)
}

func TestObservationMetrics_Case3_B_Close(t *testing.T) {
	filledAt := time.Date(2026, 8, 6, 15, 10, 0, 0, time.Local)
	run := papertrading.PaperSimRun{
		ID: 10, PlanID: 40, TradeDate: "2026-08-06", StartedAt: filledAt,
	}
	fill := papertrading.PaperSimFill{
		ID: 101, PlanID: 40, FillReason: papertrading.FillReasonMarketClose, FilledAt: filledAt,
	}
	labels := papertrading.LabelFillsForMetrics(
		[]papertrading.PaperSimFill{fill},
		[]papertrading.PaperSimRun{run},
		map[uint]string{101: "2026-08-06"},
	)
	m := papertrading.AggregateObservationMetrics([]papertrading.PaperSimRun{run}, labels)

	require.Equal(t, 1, m.FillPolicy.BWindowCloseFills)
	require.Equal(t, 1, m.FillPolicy.BWindowCompliant)
	require.Equal(t, 0, m.FillPolicy.BWindowOpenFills)
	require.Equal(t, 1, m.Quality.OKCount)
	require.InDelta(t, 1.0, m.PricePolicyCompliance, 1e-9)
}

func TestObservationMetrics_Case4_A_Open_NoBImpact(t *testing.T) {
	filledAt := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)
	run := papertrading.PaperSimRun{
		ID: 11, PlanID: 41, TradeDate: "2026-08-06", StartedAt: filledAt,
	}
	fill := papertrading.PaperSimFill{
		ID: 102, PlanID: 41, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt,
	}
	labels := papertrading.LabelFillsForMetrics(
		[]papertrading.PaperSimFill{fill},
		[]papertrading.PaperSimRun{run},
		map[uint]string{102: "2026-08-06"},
	)
	m := papertrading.AggregateObservationMetrics([]papertrading.PaperSimRun{run}, labels)

	require.Equal(t, 1, m.SessionDistribution.SessionA)
	require.Equal(t, 1, m.FillPolicy.MarketOpenFills)
	require.Equal(t, 0, m.FillPolicy.BWindowTotal)
	require.Equal(t, 0, m.FillPolicy.BWindowOpenFills)
	require.Equal(t, 1, m.Quality.OKCount)
	require.InDelta(t, 1.0, m.PricePolicyCompliance, 1e-9)
}
