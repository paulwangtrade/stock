package rules_test

import (
	"testing"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"

	"github.com/stretchr/testify/require"
)

func TestDecide_Chain_FactsToHoldingDecision(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.TakeProfitReturn = 0.20

	ret := 0.25
	facts := rules.BuildFacts(
		"sz000001", 0.10, 100, 100_000,
		ptr(10), ptr(12.5), &ret,
		20, true, 100,
		nil, nil, "", nil,
	)
	dec := rules.Decide(facts, pol)
	require.True(t, dec.SuggestOnly)
	require.Equal(t, rules.ActionReduce, dec.FinalAction)
	require.Equal(t, dec.FinalAction, dec.Action)
	require.NotEmpty(t, dec.RuleHits)
	require.NotEmpty(t, dec.ReasonCodes)
	require.NotEmpty(t, dec.Evidence)
	require.Equal(t, 1, dec.Evidence["rule_hit_count"])
	require.Contains(t, dec.Evidence, "return_rate")
	require.False(t, dec.PersistSellPlans)
	require.True(t, dec.NotSellTradePlan)
	require.True(t, dec.NotExecution)
	require.True(t, dec.NotBuyChain)
}

func TestDecide_NoFacts_Hold(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.EnableTenure = true
	pol.EnableTrend = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	pol.TakeProfitReturn = 0.2
	pol.MaxLossReturn = -0.1
	pol.StaleHoldingDays = 60

	facts := rules.BuildFacts("sz000001", 0, 0, 0, nil, nil, nil, 0, false, 0, nil, nil, "", nil)
	dec := rules.Decide(facts, pol)
	require.Equal(t, rules.ActionHold, dec.FinalAction)
	require.Equal(t, rules.ActionHold, dec.Action)
	require.True(t, dec.SuggestOnly)
	require.Empty(t, dec.RuleHits)
	require.Contains(t, dec.ReasonCodes, rules.ReasonDefaultHold)
}

func TestDecide_ConflictMerge_And_Risk(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	pol.ExitEnabled = true
	pol.MaxLossReturn = -0.05
	cap := 0.20

	ret := -0.20
	facts := rules.BuildFacts(
		"sz000001", 0.35, 100, 350_000,
		ptr(10), ptr(8), &ret,
		90, true, 100,
		nil,
		&portfoliorisk.PortfolioRiskSnapshot{
			Found: true,
			Concentration: portfoliorisk.ConcentrationBlock{
				Available: true,
				CapSingle: &cap,
			},
		},
		"", nil,
	)
	facts.PeriodState = "LONG_TERM"
	dec := rules.Decide(facts, pol)
	require.Equal(t, rules.ActionExit, dec.FinalAction) // max loss + long beats reduce
	require.GreaterOrEqual(t, len(dec.RuleHits), 2)
	require.NotEmpty(t, dec.ReasonCodes)
	require.True(t, dec.SuggestOnly)
}

func TestEvaluateRules_TrendRequiresFact(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableTrend = true
	pol.ReduceEnabled = true

	facts := rules.BuildFacts("sz000001", 0.1, 100, 1, ptr(1), ptr(1), ptr(0.0), 1, true, 100, nil, nil, "", nil)
	require.Empty(t, rules.EvaluateRules(facts, pol))

	facts.Trend = &rules.TrendFact{Available: true, ThesisBroken: true}
	hits := rules.EvaluateRules(facts, pol)
	require.Len(t, hits, 1)
	require.Equal(t, rules.RuleTrendBreak, hits[0].RuleID)
	require.NotEmpty(t, hits[0].Evidence)
}
