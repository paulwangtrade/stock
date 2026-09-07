package holdingdecision_test

import (
	"testing"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfoliorisk"

	"github.com/stretchr/testify/require"
)

func TestObserve_Default_StillHold_WithoutRuleEngine(t *testing.T) {
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.3, TotalQty: 100}},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong, ReturnRate: ptr(-0.2)},
		},
		Policy: holdingdecision.DefaultActionPolicy(),
	})
	require.False(t, got.RuleEngineUsed)
	require.Equal(t, holdingdecision.ActionSchemaVersion, got.SchemaVersion)
	require.Equal(t, 1, got.ByAction[holdingdecision.ActionHold])
	require.True(t, got.SuggestOnly)
}

func TestObserve_RuleEngine_SingleHit_TakeProfit(t *testing.T) {
	pol := holdingdecision.DefaultActionPolicy()
	pol.UseRuleEngine = true
	pol.ReduceEnabled = true
	pol.RulePolicy = holdingdecision.RuleEnginePolicy{
		EnablePnL: true, TakeProfitReturn: 0.20,
	}
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.1, TotalQty: 100, MarketValue: 100000}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 100, AvailableQty: 100},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: ptr(0.25), CurrentPrice: ptr(12.5)},
		},
		Policy: pol,
	})
	require.True(t, got.RuleEngineUsed)
	require.Equal(t, holdingdecision.ActionSchemaVersionH12, got.SchemaVersion)
	require.Equal(t, 1, got.ByAction[holdingdecision.ActionReduce])
	require.NotEmpty(t, got.Decisions[0].FiredRules)
	require.NotEmpty(t, got.Decisions[0].ConflictResolution)
	require.True(t, got.Decisions[0].SuggestOnly)
	require.False(t, got.Decisions[0].PersistSellPlans)
}

func TestObserve_RuleEngine_Conflict_ExitBeatsReduce(t *testing.T) {
	pol := holdingdecision.DefaultActionPolicy()
	pol.UseRuleEngine = true
	pol.ReduceEnabled = true
	pol.ExitEnabled = true
	pol.RulePolicy = holdingdecision.RuleEnginePolicy{
		EnablePnL: true, EnableRisk: true, MaxLossReturn: -0.05,
	}
	cap := 0.20
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.35, TotalQty: 100, MarketValue: 350000}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, AvailableQty: 100, TotalQty: 100},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {
				ReturnRate: ptr(-0.20), CurrentPrice: ptr(8),
				PeriodState: papertrading.HoldingPeriodLong,
				RiskState:   papertrading.RiskStateDanger,
			},
		},
		PortfolioRisk: &portfoliorisk.PortfolioRiskSnapshot{
			Found: true,
			Concentration: portfoliorisk.ConcentrationBlock{
				Available: true, CapSingle: &cap,
			},
		},
		Policy: pol,
	})
	require.Equal(t, holdingdecision.ActionExit, got.Decisions[0].Action)
	require.GreaterOrEqual(t, len(got.Decisions[0].FiredRules), 2)
	require.GreaterOrEqual(t, len(got.Decisions[0].ReasonCodes), 2)
	require.NotEmpty(t, got.Decisions[0].ConflictResolution)
	joined := ""
	for _, s := range got.Decisions[0].ConflictResolution {
		joined += s
	}
	require.Contains(t, joined, "final_action=EXIT")
}

func TestObserve_RuleEngine_NoTrendFact_NoMisjudge(t *testing.T) {
	pol := holdingdecision.DefaultActionPolicy()
	pol.UseRuleEngine = true
	pol.ReduceEnabled = true
	pol.RulePolicy = holdingdecision.RuleEnginePolicy{EnableTrend: true}
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.1, TotalQty: 100}},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {ReturnRate: ptr(-0.5)}, // must NOT imply trend
		},
		Policy: pol,
	})
	require.Equal(t, holdingdecision.ActionHold, got.Decisions[0].Action)
	require.Empty(t, got.Decisions[0].FiredRules)
}

func TestObserve_RuleEngine_NoRiskSnapshot_Safe(t *testing.T) {
	pol := holdingdecision.DefaultActionPolicy()
	pol.UseRuleEngine = true
	pol.ReduceEnabled = true
	pol.RulePolicy = holdingdecision.RuleEnginePolicy{EnableRisk: true}
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.5, TotalQty: 100}},
		Policy:   pol,
		// no PortfolioRisk
	})
	require.Equal(t, holdingdecision.ActionHold, got.Decisions[0].Action)
	require.True(t, got.NotSellTradePlan)
	require.True(t, got.NotExecution)
	require.True(t, got.NotBuyChain)
}

func TestObserve_RuleEngine_ExplicitTrend(t *testing.T) {
	pol := holdingdecision.DefaultActionPolicy()
	pol.UseRuleEngine = true
	pol.ReduceEnabled = true
	pol.RulePolicy = holdingdecision.RuleEnginePolicy{EnableTrend: true}
	got := holdingdecision.Observe(holdingdecision.ObservationInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.1, TotalQty: 100}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, AvailableQty: 100, TotalQty: 100},
		},
		TrendFacts: map[string]holdingdecision.TrendFactInput{
			"sz000001": {Available: true, BelowMA: true},
		},
		Policy: pol,
	})
	require.Equal(t, holdingdecision.ActionReduce, got.Decisions[0].Action)
}
