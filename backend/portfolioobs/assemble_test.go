package portfolioobs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/rebalance"

	"github.com/stretchr/testify/require"
)

func ptr(f float64) *float64 { return &f }

func TestAssemble_Empty(t *testing.T) {
	snap := &portfolio.Snapshot{Found: true, Cash: 1_000_000, TotalEquity: 1_000_000}
	eval := &papertrading.HoldingEvalObservationView{}
	dec := holdingdecision.EvaluateObservation(eval, holdingdecision.DefaultPolicy())
	cur := rebalance.CurrentFromSnapshot(snap)
	tgt := rebalance.BuildObservationTarget(cur, rebalance.ObservationTargetOptions{Identity: true})
	diff := rebalance.Diff(cur, tgt, rebalance.DefaultPolicy())
	obs := Assemble(snap, eval, dec, diff, tgt, nil, time.Date(2026, 8, 12, 8, 0, 0, 0, time.Local))
	require.Equal(t, Disclaimer, obs.Disclaimer)
	require.Equal(t, actionNone, obs.Action)
	require.Equal(t, 0, obs.Decision.NormalCount)
	require.Equal(t, 0, obs.Rebalance.KeepCount)
	require.Empty(t, obs.Positions)
	require.InDelta(t, 1_000_000.0, obs.Account.Cash, 1e-9)
}

func TestAssemble_JoinsEvalDecisionDiff(t *testing.T) {
	rGood, rBad := 0.10, -0.12
	pxGood, pxBad := 11.0, 8.8
	snap := &portfolio.Snapshot{
		Found: true, Cash: 800_000, MarketValue: 200_000, TotalEquity: 1_000_000,
		TotalExposure: 200_000, PositionCount: 2,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", StockName: "平安银行", Volume: 1000, MarketValue: 110000, Weight: 0.11},
			{StockCode: "sz000002", StockName: "万科A", Volume: 1000, MarketValue: 88000, Weight: 0.088},
		},
	}
	eval := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{
			{
				StockCode: "sz000001", StockName: "平安银行", AvgCost: ptr(10), CurrentPrice: &pxGood,
				UnrealizedPnL: ptr(1000), UnrealizedReturn: &rGood, RiskState: papertrading.RiskStateNormal,
				ProfitState: papertrading.ProfitStateProfit, HoldingPeriodState: papertrading.HoldingPeriodMid,
			},
			{
				StockCode: "sz000002", StockName: "万科A", AvgCost: ptr(10), CurrentPrice: &pxBad,
				UnrealizedPnL: ptr(-1200), UnrealizedReturn: &rBad, RiskState: papertrading.RiskStateDanger,
				ProfitState: papertrading.ProfitStateLoss, HoldingPeriodState: papertrading.HoldingPeriodMid,
			},
		},
	}
	dec := holdingdecision.EvaluateObservation(eval, holdingdecision.DefaultPolicy())
	cur := rebalance.CurrentFromSnapshot(snap)
	rebalance.AttachDecisions(cur, map[string]string{
		"sz000001": holdingdecision.StateHoldNormal,
		"sz000002": holdingdecision.StateHoldReview,
	})
	tgt := rebalance.BuildObservationTarget(cur, rebalance.ObservationTargetOptions{DropSymbols: []string{"sz000002"}})
	diff := rebalance.Diff(cur, tgt, rebalance.DefaultPolicy())
	obs := Assemble(snap, eval, dec, diff, tgt, []string{"eval_ok"}, time.Now())
	require.Equal(t, 1, obs.Decision.ReviewCount)
	require.GreaterOrEqual(t, obs.Decision.NormalCount, 1)
	require.Equal(t, 1, obs.Rebalance.RemoveCount)
	require.True(t, obs.Rebalance.Available)
	require.Len(t, obs.Positions, 2)
	var a, b *PositionRow
	for i := range obs.Positions {
		if obs.Positions[i].Symbol == "sz000001" {
			a = &obs.Positions[i]
		}
		if obs.Positions[i].Symbol == "sz000002" {
			b = &obs.Positions[i]
		}
	}
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.Equal(t, holdingdecision.StateHoldReview, b.DecisionState)
	require.Equal(t, rebalance.ActionRemove, b.RebalanceAction)
	require.Equal(t, actionNone, b.Action)
	require.InDelta(t, -0.12, *b.Return, 1e-9)
	raw, err := json.Marshal(obs)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
	require.Contains(t, string(raw), Disclaimer)
}
