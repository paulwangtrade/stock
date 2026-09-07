package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func exitAsOf(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 8, 9, 12, 0, 0, 0, time.Local)
}

func ptrFloat(v float64) *float64 { return &v }

func TestExitEvaluation_NormalHolding(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		Enabled: true, AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600363", StockName: "联创光电",
			HoldingDays: 7, UnrealizedReturn: ptrFloat(0.05),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 1, PlanID: 5, PlanItemID: 12, HoldingDays: 7, ReturnRate: ptrFloat(0.05),
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Len(t, view.Holdings, 1)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateNormal, row.Evaluation.State)
	require.Empty(t, row.Evaluation.ReasonCodes)
	require.Len(t, row.Lots, 1)
	require.Equal(t, uint(1), row.Lots[0].FillID)
	require.Equal(t, papertrading.ExitEvalStateNormal, row.Lots[0].Evaluation.State)
}

func TestExitEvaluation_LongHolding_TimeReview(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600105", StockName: "永鼎股份",
			HoldingDays: 25, UnrealizedReturn: ptrFloat(0.10),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 5, PlanID: 5, PlanItemID: 21, HoldingDays: 25, ReturnRate: ptrFloat(0.10),
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateWatch, row.Evaluation.State)
	require.Contains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonTimeReview)
	require.NotContains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)
}

func TestExitEvaluation_LossReview(t *testing.T) {
	asOf := exitAsOf(t)
	// -6% → LOSS_REVIEW + WATCH
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001", StockName: "平安银行",
			HoldingDays: 5, UnrealizedReturn: ptrFloat(-0.06),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 7, PlanID: 1, PlanItemID: 1, HoldingDays: 5, ReturnRate: ptrFloat(-0.06),
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateWatch, row.Evaluation.State)
	require.Contains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)

	// -12% → REVIEW_REQUIRED
	holding.Holdings[0].UnrealizedReturn = ptrFloat(-0.12)
	holding.Holdings[0].Lots[0].ReturnRate = ptrFloat(-0.12)
	view = papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row = view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, row.Evaluation.State)
	require.Contains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)
}

func TestExitEvaluation_MultiLot_KeepPlanIDs(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600363", StockName: "联创光电",
			HoldingDays: 25, UnrealizedReturn: ptrFloat(0.02),
			Lots: []papertrading.HoldingEvalLotRow{
				{FillID: 10, PlanID: 32, PlanItemID: 1, HoldingDays: 25, ReturnRate: ptrFloat(0.05)},
				{FillID: 11, PlanID: 35, PlanItemID: 2, HoldingDays: 5, ReturnRate: ptrFloat(-0.01)},
			},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Len(t, view.Holdings, 1)
	require.Len(t, view.Holdings[0].Lots, 2, "must not collapse multi-plan lots")
	plans := map[uint]bool{}
	fills := map[uint]bool{}
	for _, lot := range view.Holdings[0].Lots {
		plans[lot.PlanID] = true
		fills[lot.FillID] = true
		require.NotZero(t, lot.FillID)
	}
	require.True(t, plans[32])
	require.True(t, plans[35])
	require.True(t, fills[10])
	require.True(t, fills[11])
	require.Contains(t, view.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonTimeReview)
}

func TestExitEvaluation_MissingPrice_NoFalseLoss(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001", StockName: "平安银行",
			HoldingDays: 3, UnrealizedReturn: nil,
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 9, PlanID: 1, PlanItemID: 1, HoldingDays: 3, ReturnRate: nil,
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateNormal, row.Evaluation.State)
	require.NotContains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)
	require.Nil(t, row.Lots[0].UnrealizedReturn)
}

func TestExitEvaluation_NoForgedLots(t *testing.T) {
	holding := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "x",
			Lots: []papertrading.HoldingEvalLotRow{
				{FillID: 0, PlanID: 1, HoldingDays: 30}, // invalid
				{FillID: 3, PlanID: 2, HoldingDays: 5, ReturnRate: ptrFloat(0)},
			},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Len(t, view.Holdings[0].Lots, 1)
	require.Equal(t, uint(3), view.Holdings[0].Lots[0].FillID)
}

func TestExitEvaluation_NoForbiddenActionCodes(t *testing.T) {
	holding := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "y", HoldingDays: 100, UnrealizedReturn: ptrFloat(-0.2),
			Lots: []papertrading.HoldingEvalLotRow{{FillID: 1, HoldingDays: 100, ReturnRate: ptrFloat(-0.2)}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	joined := stringsJoin(view.Holdings[0].Evaluation.ReasonCodes)
	require.NotContains(t, joined, "SELL")
	require.NotContains(t, joined, "EXIT_NOW")
	require.NotContains(t, joined, "FORCE_CLOSE")
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, view.Holdings[0].Evaluation.State)
}

func TestExitEvaluation_EntryContextPassthrough(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600363", StockName: "联创光电",
			HoldingDays: 7, UnrealizedReturn: ptrFloat(0.02),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 101, PlanID: 50, PlanItemID: 77, HoldingDays: 7, ReturnRate: ptrFloat(0.02),
			}},
		}},
	}
	ctx := map[uint]papertrading.ExitContext{
		101: {
			Entry: papertrading.ExitEntryContext{
				StrategyName: "momentum_v1",
				EntryReason:  "突破20日高",
				EntryRule:    "open_ge_ref",
				IntentStatus: "ready",
			},
			Plan: papertrading.ExitPlanContext{
				PlanID: 50, TradeDate: "2026-08-01", PlanStatus: "done",
			},
		},
	}
	view := papertrading.ProjectExitEvaluationWithContext(holding, papertrading.DefaultExitEvaluationPolicy, ctx)
	require.Len(t, view.Holdings[0].Lots, 1)
	lot := view.Holdings[0].Lots[0]
	require.Equal(t, "momentum_v1", lot.Context.Entry.StrategyName)
	require.Equal(t, "突破20日高", lot.Context.Entry.EntryReason)
	require.Equal(t, "open_ge_ref", lot.Context.Entry.EntryRule)
	require.Equal(t, "ready", lot.Context.Entry.IntentStatus)
	require.Equal(t, "done", lot.Context.Plan.PlanStatus)
	require.NotContains(t, lot.Evaluation.ReasonCodes, papertrading.ExitReasonPlanReview,
		"done is terminal but not abnormal for PLAN_REVIEW in D.2.5")
}

func TestExitEvaluation_PlanAbnormal_PlanReview(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001", HoldingDays: 3, UnrealizedReturn: ptrFloat(0.01),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 201, PlanID: 9, PlanItemID: 1, HoldingDays: 3, ReturnRate: ptrFloat(0.01),
			}},
		}},
	}
	for _, status := range []string{"superseded", "failed", "invalidated"} {
		ctx := map[uint]papertrading.ExitContext{
			201: {Plan: papertrading.ExitPlanContext{PlanID: 9, TradeDate: "2026-07-01", PlanStatus: status}},
		}
		view := papertrading.ProjectExitEvaluationWithContext(holding, papertrading.DefaultExitEvaluationPolicy, ctx)
		row := view.Holdings[0]
		require.Contains(t, row.Evaluation.ReasonCodes, papertrading.ExitReasonPlanReview, status)
		require.Contains(t, row.Lots[0].Evaluation.ReasonCodes, papertrading.ExitReasonPlanReview, status)
		require.Equal(t, papertrading.ExitEvalStateWatch, row.Evaluation.State, status)
		joined := stringsJoin(row.Evaluation.ReasonCodes)
		require.NotContains(t, joined, "SELL")
		require.NotContains(t, joined, "EXIT_NOW")
		require.NotContains(t, joined, "FORCE_CLOSE")
	}
}

func TestExitEvaluation_NormalHolding_NoFalsePlanReview(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600105", HoldingDays: 5, UnrealizedReturn: ptrFloat(0.03),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 301, PlanID: 12, PlanItemID: 4, HoldingDays: 5, ReturnRate: ptrFloat(0.03),
			}},
		}},
	}
	ctx := map[uint]papertrading.ExitContext{
		301: {
			Entry: papertrading.ExitEntryContext{StrategyName: "s", EntryReason: "r"},
			Plan:  papertrading.ExitPlanContext{PlanID: 12, TradeDate: "2026-08-05", PlanStatus: "ready"},
		},
	}
	view := papertrading.ProjectExitEvaluationWithContext(holding, papertrading.DefaultExitEvaluationPolicy, ctx)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateNormal, row.Evaluation.State)
	require.Empty(t, row.Evaluation.ReasonCodes)
	require.NotContains(t, row.Lots[0].Evaluation.ReasonCodes, papertrading.ExitReasonPlanReview)
}

func TestExitEvaluation_EmptyContext_NoPanic(t *testing.T) {
	holding := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "x", HoldingDays: 2, UnrealizedReturn: nil,
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 401, PlanID: 0, PlanItemID: 0, HoldingDays: 2, ReturnRate: nil,
			}},
		}},
	}
	require.NotPanics(t, func() {
		_ = papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	})
	require.NotPanics(t, func() {
		_ = papertrading.ProjectExitEvaluationWithContext(holding, papertrading.DefaultExitEvaluationPolicy, map[uint]papertrading.ExitContext{})
	})
	require.NotPanics(t, func() {
		_ = papertrading.LoadExitContextByFillID(nil)
		_ = papertrading.LoadExitContextByFillID(holding)
		_ = papertrading.BuildExitContextFromModels(0, nil, nil)
	})
	view := papertrading.ProjectExitEvaluationWithContext(holding, papertrading.DefaultExitEvaluationPolicy, nil)
	require.Equal(t, papertrading.ExitEvalStateNormal, view.Holdings[0].Evaluation.State)
	require.NotContains(t, view.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonPlanReview)
	lot := view.Holdings[0].Lots[0]
	require.Equal(t, "", lot.Context.Entry.StrategyName)
	require.Equal(t, "", lot.Context.Entry.EntryReason)
}

func stringsJoin(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + ","
	}
	return out
}
