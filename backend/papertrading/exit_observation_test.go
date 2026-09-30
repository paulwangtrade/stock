package papertrading_test

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestProjectExitObservation_FailClosed(t *testing.T) {
	nan := math.NaN()
	cases := []papertrading.ExitObservationInput{
		{State: papertrading.ExitEvalStateNormal},
		{State: papertrading.ExitEvalStateWatch, ReasonCodes: []string{papertrading.ExitReasonTimeReview}},
		{State: papertrading.ExitEvalStateReviewRequired, UnrealizedReturn: &nan},
		{State: "SELL", UnrealizedReturn: ptrFloat(-0.2)},
		{State: "", UnrealizedReturn: ptrFloat(0.01)},
	}
	for _, in := range cases {
		obs := papertrading.ProjectExitObservation(in)
		require.Equal(t, papertrading.ExitObserveClassInsufficient, obs.Class)
		require.Equal(t, "数据不足", obs.Label)
		require.False(t, obs.SellIntentAllowed)
		require.False(t, obs.PersistSellPlans)
		require.False(t, obs.WritesTradePlan)
		require.True(t, obs.NotAnOrder)
		require.True(t, obs.NotLive)
		require.Equal(t, papertrading.ExitObservationDisclaimer, obs.Disclaimer)
		require.NotContains(t, obs.Reason, "立即卖出")
	}
}

func TestProjectExitObservation_StaleOverridesSevereState(t *testing.T) {
	obs := papertrading.ProjectExitObservation(papertrading.ExitObservationInput{
		State:            papertrading.ExitEvalStateReviewRequired,
		UnrealizedReturn: ptrFloat(-0.12),
		PriceStale:       true,
		Summary:          "持有5天，当前收益-12.0%，浮亏触及复评阈值，需要重新评估",
	})
	require.Equal(t, papertrading.ExitObserveClassInsufficient, obs.Class)
	require.Contains(t, obs.Reason, "数据过期")
	require.False(t, obs.SellIntentAllowed)
	require.False(t, obs.WritesTradePlan)
}

func TestProjectExitObservation_ProjectsExistingExitStates(t *testing.T) {
	hold := papertrading.ProjectExitObservation(papertrading.ExitObservationInput{
		State:            papertrading.ExitEvalStateNormal,
		UnrealizedReturn: ptrFloat(0.05),
		HoldingDays:      7,
		Summary:          "持有7天，当前收益+5.0%，暂无复评信号",
	})
	require.Equal(t, papertrading.ExitObserveClassHold, hold.Class)
	require.Equal(t, "持有观察", hold.Label)
	require.False(t, hold.SellIntentAllowed)
	require.Contains(t, hold.Reason, "暂无复评信号")

	reduce := papertrading.ProjectExitObservation(papertrading.ExitObservationInput{
		State:            papertrading.ExitEvalStateWatch,
		ReasonCodes:      []string{papertrading.ExitReasonLossReview},
		UnrealizedReturn: ptrFloat(-0.06),
		HoldingDays:      5,
		Summary:          "持有5天，当前收益-6.0%，浮亏进入关注区间，建议复核持仓假设",
	})
	require.Equal(t, papertrading.ExitObserveClassReduce, reduce.Class)
	require.Equal(t, "减仓观察", reduce.Label)
	require.True(t, reduce.SellIntentAllowed)
	require.False(t, reduce.PersistSellPlans)
	require.False(t, reduce.WritesTradePlan)

	flatten := papertrading.ProjectExitObservation(papertrading.ExitObservationInput{
		State:            papertrading.ExitEvalStateReviewRequired,
		ReasonCodes:      []string{papertrading.ExitReasonLossReview},
		UnrealizedReturn: ptrFloat(-0.12),
		HoldingDays:      5,
	})
	require.Equal(t, papertrading.ExitObserveClassFlatten, flatten.Class)
	require.Equal(t, "清仓观察", flatten.Label)
	require.True(t, flatten.SellIntentAllowed)
	require.Contains(t, flatten.Reason, "需要重新评估")
	require.NotContains(t, flatten.Reason, "SELL")
}

func TestExitEvaluation_AttachesObservationWithoutChangingState(t *testing.T) {
	asOf := time.Date(2026, 8, 9, 12, 0, 0, 0, time.Local)
	px := 10.0
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001", StockName: "平安银行",
			HoldingDays: 5, CurrentPrice: &px, UnrealizedReturn: ptrFloat(-0.12),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 7, PlanID: 1, PlanItemID: 1, HoldingDays: 5,
				CurrentPrice: &px, ReturnRate: ptrFloat(-0.12),
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, row.Evaluation.State)
	require.Equal(t, papertrading.ExitObserveClassFlatten, row.Observation.Class)
	require.Equal(t, "清仓观察", row.Observation.Label)
	require.False(t, row.Observation.WritesTradePlan)
	require.False(t, row.Observation.PersistSellPlans)

	holding.Holdings[0].UnrealizedReturn = nil
	holding.Holdings[0].Lots[0].ReturnRate = nil
	view = papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.Equal(t, papertrading.ExitObserveClassInsufficient, view.Holdings[0].Observation.Class)
	require.False(t, view.Holdings[0].Observation.SellIntentAllowed)
}

func TestExitEvaluation_StalePriceFailClosesObservation(t *testing.T) {
	asOf := time.Date(2026, 8, 9, 12, 0, 0, 0, time.Local)
	px := 9.0
	quote := asOf.Add(-48 * time.Hour)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600000", HoldingDays: 3,
			CurrentPrice: &px, UnrealizedReturn: ptrFloat(-0.2),
			QuoteTime: &quote,
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 3, HoldingDays: 3, CurrentPrice: &px, ReturnRate: ptrFloat(-0.2),
			}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	row := view.Holdings[0]
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, row.Evaluation.State, "stale price must not rewrite exit state")
	require.Equal(t, papertrading.ExitObserveClassInsufficient, row.Observation.Class)
	require.False(t, row.Observation.SellIntentAllowed)
}
