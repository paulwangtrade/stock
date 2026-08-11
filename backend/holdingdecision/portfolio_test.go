package holdingdecision

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"

	"github.com/stretchr/testify/require"
)

func mv(v float64) *float64 { return &v }

func TestBuildPortfolioObservation_Empty(t *testing.T) {
	snap := &portfolio.Snapshot{
		AsOf: time.Date(2026, 8, 11, 15, 0, 0, 0, time.Local), Found: false, Cash: 0,
	}
	eval := &papertrading.HoldingEvalObservationView{Holdings: nil}
	out := BuildPortfolioObservation(snap, eval, EvaluateObservation(eval, DefaultPolicy()))
	require.Equal(t, 0, out.PositionCount)
	require.Equal(t, 0, out.EvalHoldingCount)
	require.Equal(t, 0, out.DecisionCounts.HoldNormalCount)
	require.Equal(t, 0, out.DecisionCounts.HoldWatchCount)
	require.Equal(t, 0, out.DecisionCounts.HoldReviewCount)
	require.Equal(t, 0, out.DecisionCounts.ExitCandidateCount)
	require.Equal(t, StateHoldNormal, out.PortfolioDecisionState)
	require.Equal(t, ReasonNone, out.PortfolioDecisionReason)
	require.Equal(t, actionNone, out.Action)
	require.Empty(t, out.Holdings)
	assertPortfolioNoSell(t, out)
}

func TestBuildPortfolioObservation_NormalBook(t *testing.T) {
	r := 0.08
	px := 12.0
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stockMV("sz000001", &r, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, 100000, &px),
			stockMV("sh600000", &r, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, 100000, &px),
		},
	}
	snap := &portfolio.Snapshot{
		Found: true, Cash: 800000, MarketValue: 200000, TotalEquity: 1_000_000,
		TotalExposure: 200000, PositionCount: 2, AvailableCash: 800000,
	}
	out := BuildPortfolioObservation(snap, eval, EvaluateObservation(eval, DefaultPolicy()))
	require.Equal(t, 2, out.PositionCount)
	require.Equal(t, 2, out.DecisionCounts.HoldNormalCount)
	require.Equal(t, 0, out.DecisionCounts.HoldWatchCount)
	require.Equal(t, 0, out.DecisionCounts.HoldReviewCount)
	require.Equal(t, StateHoldNormal, out.PortfolioDecisionState)
	require.InDelta(t, 1.0, out.DecisionMarketValue.HoldNormal/out.DecisionMarketValue.Total, 1e-9)
	require.Equal(t, 800000.0, out.Account.Cash)
	assertPortfolioNoSell(t, out)
}

func TestBuildPortfolioObservation_IncludesWatch(t *testing.T) {
	good, bad := 0.05, -0.06
	px := 10.0
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stockMV("sz000001", &good, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, 100000, &px),
			stockMV("sz000002", &bad, papertrading.RiskStateWatch, papertrading.ProfitStateLoss, 50000, &px),
		},
	}
	out := BuildPortfolioObservation(&portfolio.Snapshot{Found: true, PositionCount: 2}, eval, EvaluateObservation(eval, DefaultPolicy()))
	require.Equal(t, 1, out.DecisionCounts.HoldNormalCount)
	require.Equal(t, 1, out.DecisionCounts.HoldWatchCount)
	require.Equal(t, 0, out.DecisionCounts.HoldReviewCount)
	require.Equal(t, StateHoldWatch, out.PortfolioDecisionState)
	require.Equal(t, ReasonRiskIncrease, out.PortfolioDecisionReason)
	require.Equal(t, 1, out.RiskDistribution.WatchCount)
	require.InDelta(t, 50000.0/150000.0, out.DecisionMarketValue.HoldWatchWeight, 1e-9)
	assertPortfolioNoSell(t, out)
}

func TestBuildPortfolioObservation_IncludesReview(t *testing.T) {
	good, ugly := 0.04, -0.12
	px := 10.0
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stockMV("sz000001", &good, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, 90000, &px),
			stockMV("sz000002", &ugly, papertrading.RiskStateDanger, papertrading.ProfitStateLoss, 10000, &px),
		},
	}
	out := BuildPortfolioObservation(&portfolio.Snapshot{Found: true, PositionCount: 2, MarketValue: 100000}, eval, EvaluateObservation(eval, DefaultPolicy()))
	require.Equal(t, 1, out.DecisionCounts.HoldNormalCount)
	require.Equal(t, 1, out.DecisionCounts.HoldReviewCount)
	require.Equal(t, 0, out.DecisionCounts.ExitCandidateCount)
	require.Equal(t, StateHoldReview, out.PortfolioDecisionState)
	require.Equal(t, ReasonRiskMaterial, out.PortfolioDecisionReason)
	require.Equal(t, 1, out.RiskDistribution.DangerCount)
	require.InDelta(t, 0.10, out.DecisionMarketValue.HoldReviewWeight, 1e-9)
	require.Equal(t, "evaluation_overlay", out.DecisionMarketValue.Basis)
	assertPortfolioNoSell(t, out)
}

func TestBuildPortfolioObservation_DoesNotMutateInputsOrEmitSell(t *testing.T) {
	r := 0.02
	px := 11.0
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			stockMV("sz000001", &r, papertrading.RiskStateNormal, papertrading.ProfitStateProfit, 100000, &px),
		},
	}
	snap := &portfolio.Snapshot{Found: true, Cash: 123456, PositionCount: 1}
	_ = BuildPortfolioObservation(snap, eval, EvaluateObservation(eval, DefaultPolicy()))
	require.Equal(t, 123456.0, snap.Cash)
	require.Equal(t, papertrading.RiskStateNormal, eval.Holdings[0].RiskState)
}

func stockMV(code string, ret *float64, risk, profit string, market float64, price *float64) papertrading.HoldingEvalStockRow {
	row := stock(code, ret, risk, profit, papertrading.HoldingPeriodMid, price, nil)
	row.MarketValue = mv(market)
	return row
}

func assertPortfolioNoSell(t *testing.T, s *PortfolioSummary) {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, "EXIT_NOW")
	require.NotContains(t, up, "FORCE_CLOSE")
	require.NotContains(t, up, "SELL_APPROVED")
	require.NotContains(t, up, `"REBALANCE"`)
	require.Equal(t, actionNone, s.Action)
	for _, h := range s.Holdings {
		require.Equal(t, actionNone, h.Action)
	}
}
