package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func healthAsOf(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 9, 7, 14, 0, 0, 0, time.Local)
}

func TestHoldingHealthScore_SignalProfitTrend_HighGrade(t *testing.T) {
	asOf := healthAsOf(t)
	quote := asOf.Add(-20 * time.Minute)
	ret := 0.08
	pnl := 800.0
	cost := 10.0
	px := 10.8
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz300085",
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
		SignalPresent:    true,
		StrategyName:     "ice_point",
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, hint, papertrading.ExplanationOptions{AsOf: asOf})
	hs := papertrading.BuildHoldingHealthScore(ex, &stock)

	require.GreaterOrEqual(t, hs.Score, 80)
	require.Contains(t, []string{papertrading.HealthGradeA, papertrading.HealthGradeB}, hs.Grade)
	require.Contains(t, hs.SupportingFactors, papertrading.ExplainTagSignalActive)
	require.Contains(t, hs.SupportingFactors, papertrading.ExplainTagTrendSupport)
	require.Contains(t, hs.SupportingFactors, papertrading.HealthFactorProfit)
	require.NotNil(t, hs.Explanation)
	require.Equal(t, "健康持有", hs.GradeLabel)

	// ExitEval must surface score without changing NORMAL for healthy lot.
	holding := &papertrading.HoldingEvalObservationView{
		Enabled: true, AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{stock},
	}
	papertrading.EnrichHoldingWithExplanations(holding, map[string]papertrading.ExplanationSourceHint{
		"sz300085": hint,
	}, papertrading.ExplanationOptions{AsOf: asOf})
	papertrading.EnrichHoldingWithHealthScores(holding)
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.NotNil(t, view.Holdings[0].HealthScore)
	require.GreaterOrEqual(t, view.Holdings[0].HealthScore.Score, 80)
	require.Equal(t, papertrading.ExitEvalStateNormal, view.Holdings[0].Evaluation.State)
}

func TestHoldingHealthScore_LossAndExpiredSignal(t *testing.T) {
	asOf := healthAsOf(t)
	quote := asOf.Add(-10 * time.Minute)
	ret := -0.12
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz000002",
		HoldingDays:      8,
		UnrealizedReturn: &ret,
		ProfitState:      papertrading.ProfitStateLoss,
		RiskState:        papertrading.RiskStateDanger,
		QuoteTime:        &quote,
	}
	ex := papertrading.BuildPositionEvaluationExplanation(stock, papertrading.ExplanationSourceHint{
		SourceType:    papertrading.ExplanationSourceStrategy,
		SignalPresent: true,
		SignalTime:    "2026-08-01",
		SignalPrice:   20,
	}, papertrading.ExplanationOptions{AsOf: asOf})
	hs := papertrading.BuildHoldingHealthScore(ex, &stock)

	require.Less(t, hs.Score, 50)
	require.Contains(t, hs.RiskFactors, papertrading.ExplainTagLossControl)
	require.Contains(t, hs.RiskFactors, papertrading.ExplainTagSignalExpired)
	require.Contains(t, []string{papertrading.HealthGradeC, papertrading.HealthGradeD}, hs.Grade)
}

func TestHoldingHealthScore_NoSourceTrace(t *testing.T) {
	asOf := healthAsOf(t)
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
	hs := papertrading.BuildHoldingHealthScore(ex, &stock)

	require.Contains(t, hs.RiskFactors, papertrading.ExplainTagNoSourceTrace)
	// base 50 + profit 10 + protection 10 - no_source 10 = 60
	require.Less(t, hs.Score, 80)
	require.Equal(t, 60, hs.Score)
	require.Equal(t, papertrading.HealthGradeB, hs.Grade)
}

func TestHoldingHealthScore_PriceStale(t *testing.T) {
	asOf := healthAsOf(t)
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
	hs := papertrading.BuildHoldingHealthScore(ex, &stock)

	require.Contains(t, hs.RiskFactors, papertrading.ExplainTagPriceStale)
	// base 50 + profit 10 + protection 10 - stale 10 = 60
	require.Equal(t, 60, hs.Score)
	baseline := papertrading.BuildHoldingHealthScore(
		papertrading.BuildPositionEvaluationExplanation(
			papertrading.HoldingEvalStockRow{
				StockCode: "sz000001", HoldingDays: 2,
				UnrealizedReturn: &ret, ProfitState: papertrading.ProfitStateProfit,
				QuoteTime: func() *time.Time { t := asOf.Add(-10 * time.Minute); return &t }(),
			},
			papertrading.ExplanationSourceHint{SourceType: papertrading.ExplanationSourceManual},
			papertrading.ExplanationOptions{AsOf: asOf},
		),
		&papertrading.HoldingEvalStockRow{
			StockCode: "sz000001", UnrealizedReturn: &ret, ProfitState: papertrading.ProfitStateProfit,
		},
	)
	require.Greater(t, baseline.Score, hs.Score)
}

func TestMapHoldingHealthGrade(t *testing.T) {
	g, l := papertrading.MapHoldingHealthGrade(85)
	require.Equal(t, papertrading.HealthGradeA, g)
	require.Equal(t, "健康持有", l)
	g, _ = papertrading.MapHoldingHealthGrade(70)
	require.Equal(t, papertrading.HealthGradeB, g)
	g, _ = papertrading.MapHoldingHealthGrade(45)
	require.Equal(t, papertrading.HealthGradeC, g)
	g, l = papertrading.MapHoldingHealthGrade(10)
	require.Equal(t, papertrading.HealthGradeD, g)
	require.Equal(t, "风险较高", l)
}

func TestHoldingHealthScore_NilStockAndNilEnrich(t *testing.T) {
	papertrading.EnrichHoldingWithHealthScores(nil)

	asOf := healthAsOf(t)
	ex := papertrading.PositionEvaluationExplanation{
		StockCode:      "sz000001",
		EvaluationTime: asOf,
		SourceType:     papertrading.ExplanationSourceManual,
		HoldReasons:    []string{papertrading.ExplainTagProfitProtection},
		RiskHints:      []string{},
	}
	hs := papertrading.BuildHoldingHealthScore(ex, nil)
	require.Equal(t, "sz000001", hs.StockCode)
	require.Equal(t, 60, hs.Score) // 50 + PROFIT_PROTECTION 10; no PROFIT without stock/pnl
	require.Contains(t, hs.SupportingFactors, papertrading.ExplainTagProfitProtection)
}

func TestHoldingHealthScore_ScoreClampTo100(t *testing.T) {
	asOf := healthAsOf(t)
	ret := 0.10
	stock := papertrading.HoldingEvalStockRow{
		StockCode:        "sz300085",
		UnrealizedReturn: &ret,
		ProfitState:      papertrading.ProfitStateProfit,
	}
	ex := papertrading.PositionEvaluationExplanation{
		StockCode:      "sz300085",
		EvaluationTime: asOf,
		HoldReasons: []string{
			papertrading.ExplainTagSignalActive,
			papertrading.ExplainTagTrendSupport,
			papertrading.ExplainTagProfitProtection,
		},
	}
	hs := papertrading.BuildHoldingHealthScore(ex, &stock)
	// 50+10+10+15+15+10 = 110 → clamp 100
	require.Equal(t, 100, hs.Score)
	require.Equal(t, papertrading.HealthGradeA, hs.Grade)
}

func TestHoldingHealthScore_ScoreClampToZero(t *testing.T) {
	asOf := healthAsOf(t)
	ex := papertrading.PositionEvaluationExplanation{
		StockCode:      "sz000002",
		EvaluationTime: asOf,
		RiskHints: []string{
			papertrading.ExplainTagSignalExpired,
			papertrading.ExplainTagLossControl,
			papertrading.ExplainTagPriceStale,
			papertrading.ExplainTagNoSourceTrace,
		},
	}
	hs := papertrading.BuildHoldingHealthScore(ex, nil)
	// 50-15-15-10-10 = 0
	require.Equal(t, 0, hs.Score)
	require.Equal(t, papertrading.HealthGradeD, hs.Grade)
}

func TestEnrichHoldingWithHealthScores_EmptyLotsRow(t *testing.T) {
	asOf := healthAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600000",
			Lots:      []papertrading.HoldingEvalLotRow{},
		}},
	}
	papertrading.EnrichHoldingWithHealthScores(holding)
	require.NotNil(t, holding.Holdings[0].Explanation)
	require.NotNil(t, holding.Holdings[0].HealthScore)
	require.Contains(t, holding.Holdings[0].HealthScore.RiskFactors, papertrading.ExplainTagNoSourceTrace)
}
