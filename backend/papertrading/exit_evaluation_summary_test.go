package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestBuildExitReviewSummary_Normal(t *testing.T) {
	r := -0.032
	s := papertrading.BuildExitReviewSummary(13, &r, papertrading.ExitEvalStateNormal, nil)
	require.Contains(t, s, "持有13天")
	require.Contains(t, s, "暂无复评信号")
	require.NotContains(t, s, "卖出")
}

func TestBuildExitReviewSummary_TimeReview(t *testing.T) {
	r := 0.01
	s := papertrading.BuildExitReviewSummary(25, &r, papertrading.ExitEvalStateWatch, []string{papertrading.ExitReasonTimeReview})
	require.Contains(t, s, "超过策略最大观察周期")
	require.NotContains(t, s, "清仓")
}

func TestBuildExitReviewSummary_AttachedToProjection(t *testing.T) {
	holding := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "x", HoldingDays: 25, UnrealizedReturn: ptrFloat(0.02),
			Lots: []papertrading.HoldingEvalLotRow{{FillID: 1, HoldingDays: 25, ReturnRate: ptrFloat(0.02)}},
		}},
	}
	view := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	require.NotEmpty(t, view.Holdings[0].Evaluation.Summary)
	require.Contains(t, view.Holdings[0].Evaluation.Summary, "重新评估")
	require.NotContains(t, view.Holdings[0].Evaluation.Summary, "SELL")
}
