package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func tSuitAsOf(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 9, 7, 14, 30, 0, 0, time.Local)
}

func TestHoldingTSuitability_NormalSellable(t *testing.T) {
	asOf := tSuitAsOf(t)
	out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{
		StockCode:        "sh600000",
		CanSell:          true,
		AvailableQty:     1000,
		Freshness:        papertrading.EvalFreshnessFresh,
		VolatilityStatus: papertrading.VolatilityNormal,
		HealthGrade:      papertrading.HealthGradeA,
		AsOf:             asOf,
	})
	require.Equal(t, papertrading.TSuitabilitySuitable, out.Level)
	require.True(t, out.CanSell)
	require.Equal(t, papertrading.EvalFreshnessFresh, out.Freshness)
	require.Equal(t, papertrading.VolatilityNormal, out.VolatilityStatus)
	require.Equal(t, papertrading.HealthGradeA, out.HealthGrade)
	require.Empty(t, out.Reasons)
}

func TestHoldingTSuitability_T1Locked(t *testing.T) {
	out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{
		StockCode:        "sz000001",
		CanSell:          false,
		AvailableQty:     0,
		LockedQty:        1000,
		Freshness:        papertrading.EvalFreshnessFresh,
		VolatilityStatus: papertrading.VolatilityActive,
		HealthGrade:      papertrading.HealthGradeA,
		AsOf:             tSuitAsOf(t),
	})
	require.Equal(t, papertrading.TSuitabilityUnsuitable, out.Level)
	require.Contains(t, out.Reasons, papertrading.TSuitReasonLockedPosition)
	require.False(t, out.CanSell)
}

func TestHoldingTSuitability_PriceStale(t *testing.T) {
	out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{
		StockCode:        "sh600519",
		CanSell:          true,
		AvailableQty:     500,
		Freshness:        papertrading.EvalFreshnessStale,
		VolatilityStatus: papertrading.VolatilityActive,
		HealthGrade:      papertrading.HealthGradeB,
		AsOf:             tSuitAsOf(t),
	})
	require.Equal(t, papertrading.TSuitabilityUnsuitable, out.Level)
	require.Contains(t, out.Reasons, papertrading.TSuitReasonPriceStale)
}

func TestHoldingTSuitability_LowVolatility(t *testing.T) {
	out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{
		StockCode:        "sz000002",
		CanSell:          true,
		AvailableQty:     800,
		Freshness:        papertrading.EvalFreshnessFresh,
		VolatilityStatus: papertrading.VolatilityLow,
		HealthGrade:      papertrading.HealthGradeA,
		AsOf:             tSuitAsOf(t),
	})
	require.Equal(t, papertrading.TSuitabilityCaution, out.Level)
	require.Contains(t, out.Reasons, papertrading.TSuitReasonLowVolatility)
}

func TestHoldingTSuitability_HealthD_IsCautionNotUnsuitable(t *testing.T) {
	out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{
		StockCode:        "sh601318",
		CanSell:          true,
		AvailableQty:     200,
		Freshness:        papertrading.EvalFreshnessFresh,
		VolatilityStatus: papertrading.VolatilityActive,
		HealthGrade:      papertrading.HealthGradeD,
		AsOf:             tSuitAsOf(t),
	})
	require.Equal(t, papertrading.TSuitabilityCaution, out.Level)
	require.Contains(t, out.Reasons, papertrading.TSuitReasonHealthRisk)
	require.NotEqual(t, papertrading.TSuitabilityUnsuitable, out.Level)
}

func TestHoldingTSuitability_NilSafeZeroInput(t *testing.T) {
	require.NotPanics(t, func() {
		out := papertrading.BuildHoldingTSuitability(papertrading.HoldingTSuitabilityInput{})
		require.Equal(t, papertrading.TSuitabilityUnsuitable, out.Level) // !canSell
		require.NotNil(t, out.Reasons)
	})
	require.NotPanics(t, func() {
		papertrading.EnrichHoldingWithTSuitability(nil, nil)
	})
	require.NotPanics(t, func() {
		_ = papertrading.BuildHoldingTSuitabilityFromStock(nil, papertrading.HoldingTSuitabilityPositionGate{}, "", time.Time{})
	})
}

func TestClassifyVolatilityFromBars(t *testing.T) {
	require.Equal(t, papertrading.VolatilityUnknown, papertrading.ClassifyVolatilityFromBars(nil))

	low := make([]papertrading.AmplitudeBar, 12)
	for i := range low {
		low[i] = papertrading.AmplitudeBar{High: 10.02, Low: 10.00, Close: 10.01} // ~0.2%
	}
	require.Equal(t, papertrading.VolatilityLow, papertrading.ClassifyVolatilityFromBars(low))

	active := make([]papertrading.AmplitudeBar, 12)
	for i := range active {
		active[i] = papertrading.AmplitudeBar{High: 10.20, Low: 10.00, Close: 10.10} // ~2%
	}
	require.Equal(t, papertrading.VolatilityActive, papertrading.ClassifyVolatilityFromBars(active))
}

func TestEnrichHoldingWithTSuitability_UsesGateHook(t *testing.T) {
	asOf := tSuitAsOf(t)
	orig := papertrading.SetLoadPositionGatesHookForTest(func(accountID uint) map[string]papertrading.HoldingTSuitabilityPositionGate {
		return map[string]papertrading.HoldingTSuitabilityPositionGate{
			"sh600000": {CanSell: true, AvailableQty: 1000, LockedQty: 0},
		}
	})
	t.Cleanup(orig)

	holding := &papertrading.HoldingEvalObservationView{
		Enabled: true,
		AsOf:    asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600000",
			Explanation: &papertrading.PositionEvaluationExplanation{
				StockCode: "sh600000",
				Freshness: papertrading.EvaluationDataFreshness{Status: papertrading.EvalFreshnessFresh},
			},
			HealthScore: &papertrading.HoldingHealthScore{Grade: papertrading.HealthGradeB},
		}},
	}
	papertrading.EnrichHoldingWithTSuitability(holding, map[string]string{
		"sh600000": papertrading.VolatilityNormal,
	})
	require.NotNil(t, holding.Holdings[0].TSuitability)
	require.Equal(t, papertrading.TSuitabilitySuitable, holding.Holdings[0].TSuitability.Level)
}
