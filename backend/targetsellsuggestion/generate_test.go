package targetsellsuggestion_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfoliotarget"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
	"go-stock/backend/targetsellsuggestion"
)

func TestGenerate_DefaultDisabled(t *testing.T) {
	rep := targetsellsuggestion.Generate(targetsellsuggestion.Input{
		TradeDate: "2026-08-22",
		Options:   targetsellsuggestion.DefaultOptions(),
	})
	require.False(t, rep.Enabled)
	require.True(t, rep.Skipped)
	require.Empty(t, rep.Suggestions)
	require.True(t, rep.SuggestOnly)
	require.True(t, rep.NotASellTradePlan)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotHoldingsMutate)
	require.False(t, targetsellsuggestion.DefaultEnabled)
}

func TestGenerate_OverweightReduce(t *testing.T) {
	opt := targetsellsuggestion.DefaultOptions()
	opt.Enabled = true
	opt.MinDeltaWeight = 0.01
	opt.AllocOptions.PreferWeightVsRatio = sellallocation.PreferWeightOnly
	opt.AllocOptions.LotSize = 100

	tgt := &portfoliotarget.TargetPortfolio{
		TargetPositions: []portfoliotarget.TargetPosition{
			{Symbol: "sz000001", TargetWeight: 0.10},
			{Symbol: "sz000002", TargetWeight: 0.10},
		},
		TargetCashRatio: 0.80,
	}

	rep := targetsellsuggestion.Generate(targetsellsuggestion.Input{
		AsOf:      time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-22",
		Equity:    1_000_000,
		Cash:      500_000,
		Holdings: []targetsellsuggestion.HoldingPosition{
			{
				Symbol: "sz000001", Qty: 1000, AvailableQty: 1000, CanSell: true,
				Weight: 0.25, MarketValue: 250_000, MarkPrice: 250,
			},
			{
				Symbol: "sz000002", Qty: 400, AvailableQty: 400, CanSell: true,
				Weight: 0.10, MarketValue: 100_000, MarkPrice: 250,
			},
		},
		Target:  tgt,
		Options: opt,
	})
	require.False(t, rep.Skipped)
	require.GreaterOrEqual(t, rep.Summary.ReduceCount+rep.Summary.ExitCount, 1)

	by := map[string]targetsellsuggestion.TargetSellSuggestion{}
	for _, s := range rep.Suggestions {
		by[s.Symbol] = s
		require.True(t, s.SuggestOnly)
		require.True(t, s.NotSellTradePlan)
		require.True(t, s.NotExecution)
		require.True(t, s.NotHoldingsMutate)
		require.GreaterOrEqual(t, s.WeightGap, 0.0)
	}
	red, ok := by["sz000001"]
	require.True(t, ok)
	require.InDelta(t, 0.25, red.CurrentWeight, 1e-6)
	require.InDelta(t, 0.10, red.TargetWeight, 1e-6)
	require.InDelta(t, 0.15, red.WeightGap, 1e-6)
	require.Equal(t, targetsellsuggestion.ActionReduce, red.Action)
	require.Greater(t, red.SuggestQty, int64(0))
	require.NotEmpty(t, red.Reason)
	// sz000002 at target → no sell row by default
	_, has2 := by["sz000002"]
	require.False(t, has2)
}

func TestGenerate_OrphanExit(t *testing.T) {
	opt := targetsellsuggestion.DefaultOptions()
	opt.Enabled = true
	opt.MinDeltaWeight = 0.01
	opt.AllocOptions.LotSize = 100
	opt.RebalanceOptions.OrphanPolicy = rebalance.OrphanExit

	tgt := &portfoliotarget.TargetPortfolio{
		TargetPositions: []portfoliotarget.TargetPosition{
			{Symbol: "sz000002", TargetWeight: 0.20},
		},
	}

	rep := targetsellsuggestion.Generate(targetsellsuggestion.Input{
		TradeDate: "2026-08-22",
		Equity:    1_000_000,
		Holdings: []targetsellsuggestion.HoldingPosition{
			{
				Symbol: "sz000001", Qty: 500, AvailableQty: 500, CanSell: true,
				Weight: 0.20, MarketValue: 200_000, MarkPrice: 400,
			},
		},
		Target:  tgt,
		Options: opt,
	})
	require.GreaterOrEqual(t, rep.Summary.ExitCount, 1)
	var exit targetsellsuggestion.TargetSellSuggestion
	for _, s := range rep.Suggestions {
		if s.Symbol == "sz000001" {
			exit = s
		}
	}
	require.Equal(t, targetsellsuggestion.ActionExit, exit.Action)
	require.InDelta(t, 0.0, exit.TargetWeight, 1e-9)
	require.InDelta(t, 0.20, exit.WeightGap, 1e-6)
	require.Equal(t, int64(500), exit.SuggestQty)
}

func TestGenerate_DoesNotMutateHoldingsInput(t *testing.T) {
	opt := targetsellsuggestion.DefaultOptions()
	opt.Enabled = true
	holdings := []targetsellsuggestion.HoldingPosition{
		{Symbol: "sz000001", Qty: 1000, AvailableQty: 1000, CanSell: true, Weight: 0.30, MarketValue: 300_000, MarkPrice: 300},
	}
	origQty := holdings[0].Qty
	tgt := &portfoliotarget.TargetPortfolio{
		TargetPositions: []portfoliotarget.TargetPosition{{Symbol: "sz000001", TargetWeight: 0.10}},
	}
	_ = targetsellsuggestion.Generate(targetsellsuggestion.Input{
		Equity: 1_000_000, Holdings: holdings, Target: tgt, Options: opt,
	})
	require.Equal(t, origQty, holdings[0].Qty)
	require.Equal(t, int64(1000), holdings[0].AvailableQty)
}
