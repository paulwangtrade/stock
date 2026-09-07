package rules_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfoliorisk"

	"github.com/stretchr/testify/require"
)

func ptr(v float64) *float64 { return &v }

func baseCtx(pol rules.Policy) rules.RuleContext {
	ret := 0.0
	return rules.RuleContext{
		Symbol:       "sz000001",
		Weight:       0.10,
		TotalQty:     100,
		Cost:         ptr(10),
		CurrentPrice: ptr(10),
		ReturnRate:   &ret,
		HoldingDays:  10,
		CanSell:      true,
		AvailableQty: 100,
		Policy:       pol,
	}
}

func TestDefaultPolicy_AllHold(t *testing.T) {
	got := rules.Evaluate(baseCtx(rules.DefaultPolicy()))
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)
	require.False(t, got.PersistSellPlans)
	require.True(t, got.NotSellTradePlan)
	require.True(t, got.NotExecution)
	require.True(t, got.NotBuyChain)
	require.Contains(t, got.Explanation, "HOLD")
}

func TestNoFacts_StayHold(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.EnableTenure = true
	pol.EnableTrend = true
	pol.EnableRisk = true
	pol.EnablePortfolioTighten = true
	pol.ReduceEnabled = true
	pol.TakeProfitReturn = 0.2
	pol.MaxLossReturn = -0.1
	pol.StaleHoldingDays = 60

	got := rules.Evaluate(rules.RuleContext{
		Symbol: "sz000001",
		Policy: pol,
		// no cost/price/return/trend/risk
	})
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)
}

func TestRule_PnLTakeProfit(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.TakeProfitReturn = 0.20

	ctx := baseCtx(pol)
	ctx.ReturnRate = ptr(0.25)
	ctx.CurrentPrice = ptr(12.5)
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.NotEmpty(t, got.RuleHits)
	require.Equal(t, rules.RulePnLTakeProfit, got.RuleHits[0].RuleID)
	require.Equal(t, rules.ReasonPnLTakeProfit, got.RuleHits[0].ReasonCode)
}

func TestRule_PnLLargeProfitProtect(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.LargeProfitProtect = 0.40

	ctx := baseCtx(pol)
	ctx.ReturnRate = ptr(0.45)
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	found := false
	for _, h := range got.RuleHits {
		if h.RuleID == rules.RulePnLLargeProtect {
			found = true
			require.Equal(t, rules.ReasonPnLLargeProtect, h.ReasonCode)
		}
	}
	require.True(t, found)
}

func TestRule_PnLMaxLoss(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.MaxLossReturn = -0.10

	ctx := baseCtx(pol)
	ctx.ReturnRate = ptr(-0.15)
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RulePnLMaxLoss, got.RuleHits[0].RuleID)
}

func TestRule_PnLMaxLoss_ExitWhenLongAndExitEnabled(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.ExitEnabled = true
	pol.MaxLossReturn = -0.10

	ctx := baseCtx(pol)
	ctx.ReturnRate = ptr(-0.20)
	ctx.PeriodState = papertrading.HoldingPeriodLong
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionExit, got.FinalAction)
}

func TestRule_PnL_MissingPrice_NoHit(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = true
	pol.TakeProfitReturn = 0.2
	pol.MaxLossReturn = -0.1

	got := rules.Evaluate(rules.RuleContext{
		Symbol: "sz000001",
		Weight: 0.2,
		Policy: pol,
	})
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)
}

func TestRule_TenureStale(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableTenure = true
	pol.ReduceEnabled = true
	pol.StaleHoldingDays = 60

	ctx := baseCtx(pol)
	ctx.HoldingDays = 90
	ctx.ReturnRate = ptr(0.0)
	ctx.ProfitState = papertrading.ProfitStateLoss
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RuleTenureStale, got.RuleHits[0].RuleID)
}

func TestRule_Trend_RequiresExplicitFact(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableTrend = true
	pol.ReduceEnabled = true

	// no TrendFact
	got := rules.Evaluate(baseCtx(pol))
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)

	// Available=false
	ctx := baseCtx(pol)
	ctx.Trend = &rules.TrendFact{Available: false, ThesisBroken: true}
	got = rules.Evaluate(ctx)
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)

	// Available=true + break
	ctx.Trend = &rules.TrendFact{Available: true, BelowMA: true}
	got = rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RuleTrendBreak, got.RuleHits[0].RuleID)
}

func TestRule_RiskNameOverCap(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	cap := 0.20

	ctx := baseCtx(pol)
	ctx.Weight = 0.35
	ctx.PortfolioRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true,
			CapSingle: &cap,
		},
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RuleRiskNameOverCap, got.RuleHits[0].RuleID)
}

func TestRule_Risk_Unavailable_NoHit(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true

	ctx := baseCtx(pol)
	ctx.Weight = 0.50
	ctx.PortfolioRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Concentration: portfoliorisk.ConcentrationBlock{Available: false},
		Exposure:      portfoliorisk.ExposureBlock{Available: false},
		Sector:        portfoliorisk.SectorBlock{Available: false},
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.Empty(t, got.RuleHits)
}

func TestRule_RiskGrossHot(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	zero := 0.0

	ctx := baseCtx(pol)
	ctx.Weight = 0.15
	ctx.PortfolioRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Exposure: portfoliorisk.ExposureBlock{
			Available:     true,
			HeadroomVsCap: &zero,
		},
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RuleRiskGrossHot, got.RuleHits[0].RuleID)
}

func TestRule_RiskSectorHot(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	maxSec := 0.25

	ctx := baseCtx(pol)
	ctx.Industry = "bank"
	ctx.PortfolioRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Sector: portfoliorisk.SectorBlock{
			Available:       true,
			MaxSectorWeight: &maxSec,
			SectorExposure:  []portfoliorisk.SectorWeight{{Sector: "bank", Weight: 0.40}},
		},
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RuleRiskSectorHot, got.RuleHits[0].RuleID)
}

func TestRule_PortfolioTighten(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePortfolioTighten = true
	pol.ReduceEnabled = true
	cap := 0.15

	ctx := baseCtx(pol)
	ctx.Weight = 0.22
	ctx.Tightened = &rules.TightenedConstraints{
		Available:       true,
		MaxSingleWeight: &cap,
		Source:          "suggest_tighten",
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionReduce, got.FinalAction)
	require.Equal(t, rules.RulePortfolioTighten, got.RuleHits[0].RuleID)
	require.Equal(t, rules.ReasonPortfolioTighten, got.RuleHits[0].ReasonCode)
}

func TestMerger_Conflict_ExitBeatsReduce(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.EnableRisk = true
	pol.ReduceEnabled = true
	pol.ExitEnabled = true
	pol.TakeProfitReturn = 0.10
	pol.MaxLossReturn = -0.05
	cap := 0.20

	ctx := baseCtx(pol)
	// take-profit would REDUCE; max-loss + LONG → EXIT candidate
	ctx.ReturnRate = ptr(-0.20) // also fails take-profit; max loss fires EXIT
	ctx.PeriodState = papertrading.HoldingPeriodLong
	ctx.Weight = 0.35
	ctx.PortfolioRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true,
			CapSingle: &cap,
		},
	}
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionExit, got.FinalAction)
	require.GreaterOrEqual(t, len(got.RuleHits), 2)
	require.NotEmpty(t, got.Explanation)
}

func TestMerger_ReduceGatedToHold(t *testing.T) {
	pol := rules.DefaultPolicy()
	pol.EngineEnabled = true
	pol.EnablePnL = true
	pol.ReduceEnabled = false // gate
	pol.TakeProfitReturn = 0.10

	ctx := baseCtx(pol)
	ctx.ReturnRate = ptr(0.30)
	got := rules.Evaluate(ctx)
	require.Equal(t, rules.ActionHold, got.FinalAction)
	require.NotEmpty(t, got.RuleHits) // hit exists but gated
	require.Contains(t, got.ReasonCodes, rules.ReasonRuleGated)
}

func TestPackageIsolation(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbiddenImports := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/risk",
		"go-stock/backend/models",
		"go-stock/backend/data",
	}
	forbiddenSrc := []string{
		"CreatePlanWithItems",
		"BuildDraftTSellTradePlan",
		"ExecutePlanItem",
		"FreezeTradePlan",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err, e.Name())
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err, e.Name())
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, e.Name())
			}
		}
	}
}
