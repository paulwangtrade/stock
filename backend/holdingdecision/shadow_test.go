package holdingdecision_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestHoldingShadow_DefaultDisabled(t *testing.T) {
	holdingdecision.ResetDefaultHoldingShadow()
	t.Cleanup(holdingdecision.ResetDefaultHoldingShadow)
	rt := holdingdecision.DefaultHoldingShadow()
	require.False(t, rt.Enabled())
	rep := rt.Run(holdingdecision.ShadowInput{
		Holdings: []holdingdecision.HoldingFact{{Symbol: "sz000001", Weight: 0.2, TotalQty: 100, MarketValue: 200_000}},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {Symbol: "sz000001", CanSell: true, TotalQty: 100, AvailableQty: 100},
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {Symbol: "sz000001", RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong},
		},
		Policy: &holdingdecision.ActionPolicy{ReduceEnabled: true, ExitEnabled: true, DefaultReduceFraction: 0.5},
	})
	require.NotNil(t, rep)
	require.False(t, rep.Enabled)
	require.True(t, rep.Skipped)
	require.Equal(t, 0, rep.HoldCount+rep.ReduceCount+rep.ExitCount)
	require.True(t, rep.NotSellTradePlan)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotPositionWrite)
	require.True(t, rep.NotTradePlanWrite)
	require.False(t, rep.PersistSellPlans)
}

func TestHoldingShadow_Enabled_BuildsReport(t *testing.T) {
	rt := holdingdecision.DefaultShadow()
	rt.SetEnabled(true)
	rt.SetPolicy(holdingdecision.ActionPolicy{
		ReduceEnabled: true, ExitEnabled: true, DefaultReduceFraction: 0.5, ConcentrationCap: 0.20,
	})
	cap := 0.20
	ret := -0.12
	price := 10.0
	rep := rt.Run(holdingdecision.ShadowInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Holdings: []holdingdecision.HoldingFact{
			{Symbol: "sz000001", Weight: 0.10, TotalQty: 100, MarketValue: 100_000}, // HOLD
			{Symbol: "sz000002", Weight: 0.35, TotalQty: 200, MarketValue: 350_000}, // REDUCE concentration
			{Symbol: "sz000003", Weight: 0.15, TotalQty: 300, MarketValue: 150_000}, // EXIT danger+long
		},
		PositionStates: map[string]holdingdecision.PositionStateFact{
			"sz000001": {CanSell: true, TotalQty: 100, AvailableQty: 100},
			"sz000002": {CanSell: true, TotalQty: 200, AvailableQty: 200},
			"sz000003": {CanSell: false, TotalQty: 300, AvailableQty: 0}, // EXIT but not executable
		},
		Evaluations: map[string]holdingdecision.EvalFact{
			"sz000001": {CurrentPrice: &price, ReturnRate: floatPtr(0.02), RiskState: papertrading.RiskStateNormal, PeriodState: papertrading.HoldingPeriodMid},
			"sz000002": {CurrentPrice: &price, ReturnRate: floatPtr(0.01), RiskState: papertrading.RiskStateNormal, PeriodState: papertrading.HoldingPeriodMid},
			"sz000003": {CurrentPrice: &price, ReturnRate: &ret, RiskState: papertrading.RiskStateDanger, PeriodState: papertrading.HoldingPeriodLong},
		},
		RiskSnapshot: &holdingdecision.RiskSnapshotFact{Found: true, MaxSingleNamePct: &cap, GrossExposure: floatPtr(0.6), Top1Weight: floatPtr(0.35)},
	})
	require.True(t, rep.Enabled)
	require.False(t, rep.Skipped)
	require.Equal(t, holdingdecision.ShadowSchemaVersion, rep.SchemaVersion)
	require.Equal(t, 1, rep.HoldCount)
	require.Equal(t, 1, rep.ReduceCount)
	require.Equal(t, 1, rep.ExitCount)
	require.NotEmpty(t, rep.ReasonStats)
	require.True(t, rep.RiskSources.RiskSnapshotFound)
	require.GreaterOrEqual(t, rep.RiskSources.DangerCount, 1)
	require.GreaterOrEqual(t, rep.RiskSources.ConcentrationHits, 1)
	require.InDelta(t, 0.10, rep.PortfolioImpact.HoldWeightSum, 1e-9)
	require.InDelta(t, 0.35, rep.PortfolioImpact.ReduceWeightSum, 1e-9)
	require.InDelta(t, 0.15, rep.PortfolioImpact.ExitWeightSum, 1e-9)
	require.Equal(t, 1, rep.PortfolioImpact.ExecutableReduceOrExit)    // reduce executable
	require.Equal(t, 1, rep.PortfolioImpact.NonExecutableReduceOrExit) // exit locked
	require.False(t, rep.PersistSellPlans)
	require.True(t, rep.NotSellTradePlan)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotPositionWrite)
	require.True(t, rep.NotTradePlanWrite)
	require.Len(t, rt.Reports(), 1)
}

func TestHoldingShadow_PackageIsolation(t *testing.T) {
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
		"UpdateItemMorning",
	}
	fset := token.NewFileSet()
	for _, name := range []string{"shadow.go"} {
		src, err := os.ReadFile(filepath.Join(wd, name))
		require.NoError(t, err)
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, name)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p)
			}
		}
	}
}

func TestStrategyBuyChain_StillNoHoldingShadowImport(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "strategy")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, e.Name()))
		require.NoError(t, err, e.Name())
		require.NotContains(t, string(src), "holdingdecision.DefaultHoldingShadow", e.Name())
		require.NotContains(t, string(src), "HoldingDecisionShadowReport", e.Name())
	}
}

func floatPtr(v float64) *float64 { return &v }
