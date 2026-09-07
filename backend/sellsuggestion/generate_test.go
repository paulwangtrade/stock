package sellsuggestion_test

import (
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
	"go-stock/backend/sellsuggestion"

	"github.com/stretchr/testify/require"
)

func TestGenerate_DefaultDisabled(t *testing.T) {
	rep := sellsuggestion.Generate(sellsuggestion.Input{
		TradeDate: "2026-08-22",
		Options:   sellsuggestion.DefaultOptions(),
	})
	require.False(t, rep.Enabled)
	require.True(t, rep.Skipped)
	require.Equal(t, "sell_suggestion_disabled", rep.SkipReason)
	require.Empty(t, rep.Suggestions)
	require.True(t, rep.SuggestOnly)
	require.True(t, rep.NotASellTradePlan)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotBroker)
	require.False(t, rep.PersistSellPlans)
	require.True(t, sellsuggestion.DefaultEnabled == false)
}

func TestGenerate_REDUCE_And_EXIT(t *testing.T) {
	asOf := time.Date(2026, 8, 22, 15, 5, 0, 0, time.FixedZone("CST", 8*3600))
	pre := &holdingdecision.ActionView{
		InputsFingerprint: "test-fp",
		Decisions: []holdingdecision.ActionDecision{
			{
				Symbol: "sz000001", Action: holdingdecision.ActionReduce,
				Reason: "CONCENTRATION", ReasonCodes: []string{holdingdecision.ReasonConcentration},
				Summary: "trim overweight", Weight: 0.20,
				Reduce: &holdingdecision.ReduceHint{Fraction: f64(0.5)},
			},
			{
				Symbol: "sh600000", Action: holdingdecision.ActionExit,
				Reason: "EXIT_POLICY", ReasonCodes: []string{holdingdecision.ReasonExitPolicy},
				Summary: "exit observe", Weight: 0.10,
				Exit: &holdingdecision.ExitHint{Intent: holdingdecision.ExitIntentFlattenSellable},
			},
			{
				Symbol: "sz000002", Action: holdingdecision.ActionHold,
				Reason: "NONE", Summary: "hold", Weight: 0.05,
			},
		},
	}
	opt := sellsuggestion.DefaultOptions()
	opt.Enabled = true
	opt.AllocOptions.PreferWeightVsRatio = sellallocation.PreferRatioOnly
	opt.AllocOptions.DefaultReduceRatio = 0.5

	rep := sellsuggestion.Generate(sellsuggestion.Input{
		AsOf:                 asOf,
		TradeDate:            "2026-08-22",
		Equity:               1_000_000,
		PrecomputedDecisions: pre,
		Positions: map[string]sellallocation.CurrentPosition{
			"sz000001": {
				Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true,
				Weight: 0.20, MarketValue: 200_000, MarkPrice: 200,
			},
			"sh600000": {
				Symbol: "sh600000", TotalQty: 500, AvailableQty: 500, CanSell: true,
				Weight: 0.10, MarketValue: 100_000, MarkPrice: 200,
			},
			"sz000002": {
				Symbol: "sz000002", TotalQty: 200, AvailableQty: 200, CanSell: true,
				Weight: 0.05, MarketValue: 50_000, MarkPrice: 250,
			},
		},
		Options: opt,
	})
	require.False(t, rep.Skipped)
	require.Len(t, rep.Suggestions, 2) // HOLD excluded by default
	require.Equal(t, 1, rep.Summary.ReduceCount)
	require.Equal(t, 1, rep.Summary.ExitCount)

	by := map[string]sellsuggestion.SellSuggestion{}
	for _, s := range rep.Suggestions {
		by[s.Symbol] = s
		require.True(t, s.SuggestOnly)
		require.True(t, s.NotTradePlan)
		require.True(t, s.NotExecution)
		require.True(t, s.NotBroker)
		require.NotEmpty(t, s.Action)
		require.NotNil(t, s.CurrentPosition)
	}
	red := by["sz000001"]
	require.Equal(t, sellsuggestion.ActionReduce, red.Action)
	require.Equal(t, int64(500), red.SuggestSellQty)
	require.InDelta(t, 0.10, red.TargetWeight, 1e-6)
	require.Contains(t, red.RiskReason, holdingdecision.ReasonConcentration)
	require.Equal(t, int64(1000), red.CurrentPosition.TotalQty)

	ex := by["sh600000"]
	require.Equal(t, sellsuggestion.ActionExit, ex.Action)
	require.Equal(t, int64(500), ex.SuggestSellQty)
	require.InDelta(t, 0.0, ex.TargetWeight, 1e-9)
}

func TestGenerate_IncludeHold_And_RebalanceContrast(t *testing.T) {
	pre := &holdingdecision.ActionView{
		Decisions: []holdingdecision.ActionDecision{
			{
				Symbol: "sz000001", Action: holdingdecision.ActionReduce,
				Reason: "TRIM", ReasonCodes: []string{"OVERWEIGHT"},
				Summary: "reduce", Weight: 0.25,
				Reduce: &holdingdecision.ReduceHint{Fraction: f64(0.4)},
			},
			{
				Symbol: "sz000002", Action: holdingdecision.ActionHold,
				Reason: "HOLD", Weight: 0.10,
			},
		},
	}
	opt := sellsuggestion.DefaultOptions()
	opt.Enabled = true
	opt.IncludeHold = true
	opt.AttachRebalance = true
	opt.AllocOptions.PreferWeightVsRatio = sellallocation.PreferRatioOnly

	cur := rebalance.CurrentPortfolio{
		Found: true, Equity: 1_000_000,
		Positions: []rebalance.CurrentPortfolioPos{
			{Symbol: "sz000001", Qty: 1000, Weight: 0.25, MarketValue: 250_000},
			{Symbol: "sz000002", Qty: 400, Weight: 0.10, MarketValue: 100_000},
		},
	}
	tgt := rebalance.TargetPortfolio{
		EquityRef: 1_000_000,
		Positions: []rebalance.TargetPosition{
			{Symbol: "sz000001", TargetWeight: 0.10},
			{Symbol: "sz000002", TargetWeight: 0.10},
		},
	}

	rep := sellsuggestion.Generate(sellsuggestion.Input{
		TradeDate:            "2026-08-22",
		Equity:               1_000_000,
		PrecomputedDecisions: pre,
		Positions: map[string]sellallocation.CurrentPosition{
			"sz000001": {
				Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true,
				Weight: 0.25, MarkPrice: 250,
			},
			"sz000002": {
				Symbol: "sz000002", TotalQty: 400, AvailableQty: 400, CanSell: true,
				Weight: 0.10, MarkPrice: 250,
			},
		},
		SellTarget:       sellsuggestion.TargetFromRebalance(&tgt),
		RebalanceCurrent: &cur,
		RebalanceTarget:  &tgt,
		Options:          opt,
	})
	require.True(t, rep.RebalanceAttached)
	require.Len(t, rep.Suggestions, 2)
	var red sellsuggestion.SellSuggestion
	for _, s := range rep.Suggestions {
		if s.Symbol == "sz000001" {
			red = s
		}
	}
	require.Equal(t, sellsuggestion.ActionReduce, red.Action)
	require.Contains(t, red.RebalanceContrast, "rebalance:REDUCE")
	require.NotEmpty(t, red.RiskReason)
}

func TestGenerate_ObservePath_AllGatesOff_NoSells(t *testing.T) {
	opt := sellsuggestion.DefaultOptions()
	opt.Enabled = true
	// DecisionPolicy default: Reduce/Exit off → all HOLD → empty suggestions
	rep := sellsuggestion.Generate(sellsuggestion.Input{
		TradeDate: "2026-08-22",
		Equity:    1e6,
		Observation: holdingdecision.ObservationInput{
			TradeDate: "2026-08-22",
			Holdings: []holdingdecision.HoldingFact{
				{Symbol: "sz000001", Weight: 0.2, TotalQty: 1000, MarketValue: 200000},
			},
			PositionStates: map[string]holdingdecision.PositionStateFact{
				"sz000001": {Symbol: "sz000001", CanSell: true, TotalQty: 1000, AvailableQty: 1000},
			},
		},
		Positions: map[string]sellallocation.CurrentPosition{
			"sz000001": {Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true, Weight: 0.2},
		},
		Options: opt,
	})
	require.False(t, rep.Skipped)
	require.Empty(t, rep.Suggestions)
	require.NotEmpty(t, rep.DecisionFingerprint)
}

func f64(v float64) *float64 { return &v }
