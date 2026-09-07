package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func explainAsOf(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 9, 7, 14, 0, 0, 0, time.Local)
}

func TestPositionEvaluationExplanation_SignalProfitTrendHold(t *testing.T) {
	asOf := explainAsOf(t)
	quote := asOf.Add(-30 * time.Minute)
	ret := 0.08
	pnl := 800.0
	cost := 10.0
	px := 10.8
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz300085",
		StockName:        "银之杰",
		HoldingDays:      5,
		AvgCost:          &cost,
		CurrentPrice:     &px,
		UnrealizedPnL:    &pnl,
		UnrealizedReturn: &ret,
		ProfitState:      papertrading.ProfitStateProfit,
		RiskState:        papertrading.RiskStateNormal,
		TrendState:       papertrading.TrendStateUp,
		QuoteTime:        &quote,
	}
	hint := papertrading.ExplanationSourceHint{
		SourceType:       papertrading.ExplanationSourceStrategy,
		SignalSnapshotID: 42,
		SignalTime:       "2026-09-01",
		SignalPrice:      10.2,
		SignalTag:        "强",
		SignalPresent:    true,
		StrategyName:     "ice_point",
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, hint, papertrading.ExplanationOptions{AsOf: asOf})

	require.Equal(t, "sz300085", ex.StockCode)
	require.Equal(t, papertrading.ExplanationSourceStrategy, ex.SourceType)
	require.True(t, ex.SignalContext.Present)
	require.Equal(t, uint(42), ex.SignalContext.SignalSnapshotID)
	require.InDelta(t, 10.2, ex.SignalContext.SignalPrice, 1e-9)
	require.Contains(t, ex.HoldReasons, papertrading.ExplainTagSignalActive)
	require.Contains(t, ex.HoldReasons, papertrading.ExplainTagTrendSupport)
	require.Contains(t, ex.HoldReasons, papertrading.ExplainTagProfitProtection)
	require.NotContains(t, ex.RiskHints, papertrading.ExplainTagNoSourceTrace)
	require.NotContains(t, ex.RiskHints, papertrading.ExplainTagPriceStale)
	require.NotContains(t, ex.RiskHints, papertrading.ExplainTagLossControl)
	require.Equal(t, papertrading.EvalFreshnessFresh, ex.Freshness.PriceStatus)
	require.Equal(t, papertrading.EvalFreshnessUnknown, ex.Freshness.KlineStatus)

	// Wire through ExitEval without changing exit state for a healthy lot.
	holding := &papertrading.HoldingEvalObservationView{
		Enabled: true, AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{stock},
	}
	papertrading.EnrichHoldingWithExplanations(holding, map[string]papertrading.ExplanationSourceHint{
		"sz300085": hint,
	}, papertrading.ExplanationOptions{AsOf: asOf})
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Len(t, view.Holdings, 1)
	require.NotNil(t, view.Holdings[0].Explanation)
	require.Contains(t, view.Holdings[0].Explanation.HoldReasons, papertrading.ExplainTagSignalActive)
	require.Equal(t, papertrading.ExitEvalStateNormal, view.Holdings[0].Evaluation.State)
}

func TestPositionEvaluationExplanation_NoSourceTrace(t *testing.T) {
	asOf := explainAsOf(t)
	ret := 0.02
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sh600000",
		HoldingDays:      3,
		UnrealizedReturn: &ret,
		ProfitState:      papertrading.ProfitStateProfit,
		RiskState:        papertrading.RiskStateNormal,
		TrendState:       papertrading.TrendStateUnknown,
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, papertrading.ExplanationSourceHint{}, papertrading.ExplanationOptions{AsOf: asOf})
	require.Equal(t, papertrading.ExplanationSourceUnknown, ex.SourceType)
	require.False(t, ex.SignalContext.Present)
	require.Contains(t, ex.RiskHints, papertrading.ExplainTagNoSourceTrace)
	require.NotContains(t, ex.HoldReasons, papertrading.ExplainTagSignalActive)
}

func TestPositionEvaluationExplanation_PriceStale(t *testing.T) {
	asOf := explainAsOf(t)
	staleQuote := asOf.Add(-30 * time.Hour)
	ret := 0.01
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz000001",
		HoldingDays:      2,
		UnrealizedReturn: &ret,
		ProfitState:      papertrading.ProfitStateProfit,
		RiskState:        papertrading.RiskStateNormal,
		QuoteTime:        &staleQuote,
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, papertrading.ExplanationSourceHint{
		SourceType: papertrading.ExplanationSourceManual,
	}, papertrading.ExplanationOptions{AsOf: asOf})
	require.Equal(t, papertrading.EvalFreshnessStale, ex.Freshness.PriceStatus)
	require.Contains(t, ex.RiskHints, papertrading.ExplainTagPriceStale)
	require.NotNil(t, ex.Freshness.PriceAgeSec)
	require.Greater(t, *ex.Freshness.PriceAgeSec, int64(24*3600))
}

func TestPositionEvaluationExplanation_LossControl(t *testing.T) {
	asOf := explainAsOf(t)
	quote := asOf.Add(-10 * time.Minute)
	ret := -0.12
	pnl := -1200.0
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz000002",
		HoldingDays:      8,
		UnrealizedReturn: &ret,
		UnrealizedPnL:    &pnl,
		ProfitState:      papertrading.ProfitStateLoss,
		RiskState:        papertrading.RiskStateDanger,
		QuoteTime:        &quote,
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, papertrading.ExplanationSourceHint{
		SourceType:    papertrading.ExplanationSourceStrategy,
		SignalPresent: true,
		SignalTime:    "2026-08-01", // expired vs max 20 days
		SignalPrice:   20,
	}, papertrading.ExplanationOptions{AsOf: asOf})
	require.Contains(t, ex.RiskHints, papertrading.ExplainTagLossControl)
	require.Contains(t, ex.RiskHints, papertrading.ExplainTagSignalExpired)
	require.NotContains(t, ex.HoldReasons, papertrading.ExplainTagSignalActive)
	require.NotContains(t, ex.HoldReasons, papertrading.ExplainTagProfitProtection)
	require.Equal(t, papertrading.EvalFreshnessFresh, ex.Freshness.PriceStatus)
}

func TestClassifyExplanationSourceType(t *testing.T) {
	require.Equal(t, papertrading.ExplanationSourceWatchlist,
		papertrading.ClassifyExplanationSourceType("watchlist", "", false))
	require.Equal(t, papertrading.ExplanationSourceManual,
		papertrading.ClassifyExplanationSourceType("exit_review", "", false))
	require.Equal(t, papertrading.ExplanationSourceStrategy,
		papertrading.ClassifyExplanationSourceType("after_close", "ice", false))
	require.Equal(t, papertrading.ExplanationSourceUnknown,
		papertrading.ClassifyExplanationSourceType("", "", false))
}

func TestClassifyEvaluationDataFreshness_KlineOptional(t *testing.T) {
	asOf := explainAsOf(t)
	kline := asOf.Add(-2 * time.Hour)
	quote := asOf.Add(-5 * time.Minute)
	fresh := papertrading.ClassifyEvaluationDataFreshness(&quote, &kline, papertrading.ExplanationOptions{AsOf: asOf})
	require.Equal(t, papertrading.EvalFreshnessFresh, fresh.PriceStatus)
	require.Equal(t, papertrading.EvalFreshnessFresh, fresh.KlineStatus)
	require.Equal(t, papertrading.EvalFreshnessFresh, fresh.Status)
}

func TestExplanation_NilAndEmptySafe(t *testing.T) {
	// Enrich / Load must not panic on nil or empty views.
	papertrading.EnrichHoldingWithExplanations(nil, nil, papertrading.ExplanationOptions{})
	require.Empty(t, papertrading.LoadExplanationSourceHints(nil))

	empty := &papertrading.HoldingEvalObservationView{Holdings: nil}
	papertrading.EnrichHoldingWithExplanations(empty, nil, papertrading.ExplanationOptions{})
	require.Empty(t, papertrading.LoadExplanationSourceHints(empty))
	require.Empty(t, empty.Holdings)

	asOf := explainAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001",
			Lots:      nil,
		}},
	}
	papertrading.EnrichHoldingWithExplanations(holding, map[string]papertrading.ExplanationSourceHint{}, papertrading.ExplanationOptions{AsOf: asOf})
	require.NotNil(t, holding.Holdings[0].Explanation)
	require.Contains(t, holding.Holdings[0].Explanation.RiskHints, papertrading.ExplainTagNoSourceTrace)
}

func TestExplanation_FreshnessUnknownWhenNoTimestamps(t *testing.T) {
	asOf := explainAsOf(t)
	fresh := papertrading.ClassifyEvaluationDataFreshness(nil, nil, papertrading.ExplanationOptions{AsOf: asOf})
	require.Equal(t, papertrading.EvalFreshnessUnknown, fresh.PriceStatus)
	require.Equal(t, papertrading.EvalFreshnessUnknown, fresh.KlineStatus)
	require.Equal(t, papertrading.EvalFreshnessUnknown, fresh.Status)
	require.Nil(t, fresh.PriceAgeSec)
}

func TestExplanation_SignalPresentWithoutParseableTime_NotExpired(t *testing.T) {
	asOf := explainAsOf(t)
	stock := papertrading.HoldingEvalStockRow{
		StockCode:   "sz300001",
		HoldingDays: 2,
		ProfitState: papertrading.ProfitStateBreakeven,
		RiskState:   papertrading.RiskStateNormal,
		TrendState:  papertrading.TrendStateUnknown,
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, papertrading.ExplanationSourceHint{
		SourceType:    papertrading.ExplanationSourceStrategy,
		SignalPresent: true,
		SignalPrice:   12.3,
		SignalTime:    "not-a-date",
	}, papertrading.ExplanationOptions{AsOf: asOf})
	require.True(t, ex.SignalContext.Present)
	require.Contains(t, ex.HoldReasons, papertrading.ExplainTagSignalActive)
	require.NotContains(t, ex.RiskHints, papertrading.ExplainTagSignalExpired)
}

func TestExplainTagLabelZH(t *testing.T) {
	require.Equal(t, "当前仍有有效信号", papertrading.ExplainTagLabelZH(papertrading.ExplainTagSignalActive))
	require.Equal(t, "无来源信息", papertrading.ExplainTagLabelZH(papertrading.ExplainTagNoSourceTrace))
	require.Equal(t, "CUSTOM", papertrading.ExplainTagLabelZH("CUSTOM"))
}

func TestProjectExitEvaluation_BuildsExplanationWhenMissing(t *testing.T) {
	asOf := explainAsOf(t)
	ret := 0.03
	holding := &papertrading.HoldingEvalObservationView{
		Enabled: true, AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600519", StockName: "茅台",
			HoldingDays: 3, UnrealizedReturn: &ret,
			ProfitState: papertrading.ProfitStateProfit,
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 1, PlanID: 9, PlanItemID: 1, HoldingDays: 3, ReturnRate: &ret,
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Len(t, view.Holdings, 1)
	require.NotNil(t, view.Holdings[0].Explanation)
	require.NotNil(t, view.Holdings[0].HealthScore)
	require.Equal(t, papertrading.ExitEvalStateNormal, view.Holdings[0].Evaluation.State)
}
