package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestClassifyFill_Case1_LegacyExact_Plan26Run2(t *testing.T) {
	filledAt := time.Date(2026, 8, 5, 15, 14, 20, 0, time.Local)
	fill := papertrading.PaperSimFill{
		ID:         6,
		PlanID:     26,
		OrderID:    6,
		StockCode:  "sh603986",
		Price:      358,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   filledAt,
	}

	lbl := papertrading.ClassifyFill(2, 26, "2026-08-05", fill)
	require.Equal(t, papertrading.SessionB, lbl.DerivedSession)
	require.Equal(t, papertrading.SessionSourceDerived, lbl.SessionSource)
	require.True(t, lbl.ExcludedBaseline)
	require.Equal(t, papertrading.QualityLegacyBaseline, lbl.QualityTag)
	require.True(t, lbl.LegacyExactMatch)

	bundle := papertrading.BuildObservationDayBundle(papertrading.ObservationBundleInput{
		Run: &papertrading.PaperSimRun{
			ID: 2, PlanID: 26, TradeDate: "2026-08-05",
			Trigger: papertrading.TriggerManual, Actor: "verify:b1-paper",
			StartedAt: time.Date(2026, 8, 5, 15, 14, 19, 0, time.Local),
		},
		Fills: []papertrading.PaperSimFill{
			{ID: 6, PlanID: 26, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt},
			{ID: 7, PlanID: 26, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt.Add(time.Second)},
			{ID: 8, PlanID: 26, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt.Add(2 * time.Second)},
			{ID: 9, PlanID: 26, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt.Add(3 * time.Second)},
			{ID: 10, PlanID: 26, FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt.Add(4 * time.Second)},
		},
		PositionCount: 5,
	})
	require.Equal(t, uint(2), bundle.RunID)
	require.Equal(t, uint(26), bundle.PlanID)
	require.Equal(t, "2026-08-05", bundle.TradeDate)
	require.Equal(t, papertrading.SessionB, bundle.DerivedSession)
	require.Equal(t, papertrading.SessionSourceDerived, bundle.SessionSource)
	require.Equal(t, 5, bundle.FillCount)
	require.Equal(t, 5, bundle.PositionCount)
	require.True(t, bundle.ExcludedBaseline)
	require.Equal(t, papertrading.QualityLegacyBaseline, bundle.QualityTag)
	require.Equal(t, 5, bundle.FillReasonSummary[papertrading.FillReasonMarketOpen])
}

func TestClassifyFill_Case2_PostCutover_B_MarketOpen_Anomaly(t *testing.T) {
	// After C.2-C cutover: B + market_open must NOT be silent legacy.
	filledAt := time.Date(2026, 8, 6, 15, 10, 0, 0, time.Local)
	require.False(t, filledAt.Before(papertrading.C2CCutoverLocal))

	fill := papertrading.PaperSimFill{
		ID:         100,
		PlanID:     99,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   filledAt,
	}
	lbl := papertrading.ClassifyFill(50, 99, "2026-08-06", fill)
	require.Equal(t, papertrading.SessionB, lbl.DerivedSession)
	require.False(t, lbl.ExcludedBaseline)
	require.Equal(t, papertrading.QualityAnomaly, lbl.QualityTag)
	require.False(t, lbl.LegacyExactMatch)
	require.False(t, lbl.LegacyHeuristic)
}

func TestClassifyFill_Case3_B_MarketClose_OK(t *testing.T) {
	filledAt := time.Date(2026, 8, 6, 15, 10, 0, 0, time.Local)
	fill := papertrading.PaperSimFill{
		ID:         101,
		PlanID:     40,
		FillReason: papertrading.FillReasonMarketClose,
		FilledAt:   filledAt,
		Price:      12.34,
	}
	lbl := papertrading.ClassifyFill(10, 40, "2026-08-06", fill)
	require.Equal(t, papertrading.SessionB, lbl.DerivedSession)
	require.Equal(t, papertrading.PriceModeClose, lbl.PriceMode)
	require.Equal(t, papertrading.QualityOK, lbl.QualityTag)
	require.False(t, lbl.ExcludedBaseline)

	bundle := papertrading.BuildObservationDayBundle(papertrading.ObservationBundleInput{
		Run: &papertrading.PaperSimRun{
			ID: 10, PlanID: 40, TradeDate: "2026-08-06",
			StartedAt: filledAt,
		},
		Fills: []papertrading.PaperSimFill{fill},
	})
	require.Equal(t, papertrading.SessionB, bundle.DerivedSession)
	require.Equal(t, papertrading.PriceModeClose, bundle.PriceMode)
	require.Equal(t, papertrading.QualityOK, bundle.QualityTag)
}

func TestClassifyFill_Case4_A_MarketOpen_OK(t *testing.T) {
	filledAt := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)
	fill := papertrading.PaperSimFill{
		ID:         102,
		PlanID:     41,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   filledAt,
	}
	lbl := papertrading.ClassifyFill(11, 41, "2026-08-06", fill)
	require.Equal(t, papertrading.SessionA, lbl.DerivedSession)
	require.Equal(t, papertrading.PriceModeRealtime, lbl.PriceMode)
	require.Equal(t, papertrading.QualityOK, lbl.QualityTag)
	require.False(t, lbl.ExcludedBaseline)
}

func TestClassifyFill_PreCutoverHeuristic_B_Open_WithoutExactID(t *testing.T) {
	// Same policy bug before cutover, but not the exact fill id list → still LEGACY via heuristic.
	filledAt := time.Date(2026, 8, 5, 15, 20, 0, 0, time.Local)
	fill := papertrading.PaperSimFill{
		ID:         999,
		PlanID:     77,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   filledAt,
	}
	lbl := papertrading.ClassifyFill(88, 77, "2026-08-05", fill)
	require.Equal(t, papertrading.SessionB, lbl.DerivedSession)
	require.True(t, lbl.ExcludedBaseline)
	require.Equal(t, papertrading.QualityLegacyBaseline, lbl.QualityTag)
	require.True(t, lbl.LegacyHeuristic)
	require.False(t, lbl.LegacyExactMatch)
}

func TestIsExactLegacyFill_OnlyKnownIDs(t *testing.T) {
	require.True(t, papertrading.IsExactLegacyFill(2, 26, "2026-08-05", 6))
	require.True(t, papertrading.IsExactLegacyFill(2, 26, "2026-08-05", 10))
	require.False(t, papertrading.IsExactLegacyFill(2, 26, "2026-08-05", 11))
	require.False(t, papertrading.IsExactLegacyFill(2, 26, "2026-08-06", 6)) // wrong date
	require.False(t, papertrading.IsExactLegacyFill(3, 26, "2026-08-05", 6))
}
