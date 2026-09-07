package riskallocshadow_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/riskallocshadow"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func floatPtr(v float64) *float64 { return &v }

func baseRisk() *tradingconfig.RiskView {
	return &tradingconfig.RiskView{
		MarketLevel: 3, MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20,
	}
}

func baseConstraints() portfoliolayer.ConstraintSet {
	return portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.85),
			MaxSingleNamePct:    floatPtr(0.20),
			MarketLevel:         3,
		},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 100},
	}
}

func sel2() allocationengine.SelectionResult {
	return allocationengine.SelectionResult{
		Selected: []string{"sz000002", "sz000003"},
		Waitlist: []string{"sz000004"},
	}
}

func TestBuild_RecordOnlyFlags(t *testing.T) {
	t.Parallel()
	ledger := &portfolio.Snapshot{Found: true, TotalEquity: 1_000_000, Cash: 400_000, TotalExposure: 200_000}
	got := riskallocshadow.Build(riskallocshadow.Input{
		Ledger: ledger, Risk: baseRisk(), Constraints: baseConstraints(), Selection: sel2(),
	})
	require.True(t, got.RecordOnly)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotRiskViewWrite)
	require.True(t, got.NotFilterWrite)
	require.True(t, got.NotExecutionWrite)
	require.True(t, got.TightenOnlyApplied)
	require.Equal(t, "fixed_amount", got.Legacy.Method)
	require.Equal(t, "equal_weight", got.RiskAdjustedPortfolio.Method)
}

func TestBuild_HighExposure_ReducesNewBuyBudget(t *testing.T) {
	t.Parallel()
	// Gross ~90% vs 85% cap → headroom exhausted → tighten reserve / max names → capital shrinks.
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 100_000, AvailableCash: 100_000,
		TotalExposure: 900_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1, MarketValue: 900_000, Weight: 0.90},
		},
	}
	riskBefore := *baseRisk()
	cons := baseConstraints()
	execBefore := cons.Execution

	got := riskallocshadow.Build(riskallocshadow.Input{
		Ledger: ledger, Risk: baseRisk(), Constraints: cons, Selection: sel2(),
		LegacyAmountPerName: 100_000, TradeDate: "2026-08-21",
	})
	require.True(t, got.RiskFound)
	require.LessOrEqual(t, got.RiskAdjustedBudget.AvailableCapital, got.OriginalBudget.AvailableCapital+1e-9)
	require.True(t,
		got.RiskAdjustedBudget.AvailableCapital < got.OriginalBudget.AvailableCapital-1e-6 ||
			got.MaxNamesChange.After < got.MaxNamesChange.Before ||
			got.ReserveChange.After > got.ReserveChange.Before+1e-6,
		"high exposure must shrink buy budget and/or max names / raise reserve",
	)
	require.Less(t, got.RiskAdjustedPortfolio.SumAllocationSet, got.Legacy.SumAllocationSet)
	require.Equal(t, riskBefore, *baseRisk())
	require.Equal(t, execBefore, cons.Execution)
}

func TestBuild_LowCash_RaisesReserve(t *testing.T) {
	t.Parallel()
	// Cash ratio 10% < 20% floor → SuggestTighten raises reserve preference to 0.15.
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 100_000, AvailableCash: 100_000,
		TotalExposure: 400_000, // headroom still positive vs 0.85
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1, MarketValue: 400_000, Weight: 0.40},
		},
	}
	got := riskallocshadow.Build(riskallocshadow.Input{
		Ledger: ledger, Risk: baseRisk(), Constraints: baseConstraints(), Selection: sel2(),
	})
	require.Greater(t, got.ReserveChange.After, got.ReserveChange.Before+1e-6)
	require.Greater(t, got.ReserveChange.Delta, 0.0)
	require.LessOrEqual(t, got.RiskAdjustedBudget.AvailableCapital, got.OriginalBudget.AvailableCapital+1e-9)
	found := false
	for _, n := range got.TightenNotes {
		if n.Reason == "cash_ratio_low" {
			found = true
			break
		}
	}
	require.True(t, found, "expected cash_ratio_low tighten note")
}

func TestBuild_SectorUnavailable_NoSectorLimit(t *testing.T) {
	t.Parallel()
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 400_000, TotalExposure: 200_000,
	}
	got := riskallocshadow.Build(riskallocshadow.Input{
		Ledger: ledger, Risk: baseRisk(), Constraints: baseConstraints(), Selection: sel2(),
	})
	require.False(t, got.SectorChange.Available)
	require.False(t, got.SectorChange.Applied)
	require.Equal(t, "sector_unavailable_no_sector_limit", got.SectorChange.Note)
	for _, n := range got.TightenNotes {
		require.NotEqual(t, "sector_over_cap", n.Reason)
		require.NotEqual(t, "max_sector_weight", n.Field)
	}
}

func TestBuild_RiskCannotBeOverriddenByPreference(t *testing.T) {
	t.Parallel()
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 500_000, TotalExposure: 100_000,
	}
	// Risk ceiling 0.70; Preference tries to "widen" to 0.99 — Resolve must keep ≤ 0.70.
	wide := 0.99
	cons := portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.70),
			MaxSingleNamePct:    floatPtr(0.15),
		},
		Portfolio: portfoliolayer.PreferenceLayer{
			MaxGrossExposurePct: &wide,
			MaxSingleWeight:     floatPtr(0.50),
		},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 100},
	}
	riskView := &tradingconfig.RiskView{MaxGrossExposurePct: 0.70, MaxSingleNamePct: 0.15}
	riskBefore := *riskView

	got := riskallocshadow.Build(riskallocshadow.Input{
		Ledger: ledger, Risk: riskView, Constraints: cons, Selection: sel2(),
	})
	require.True(t, got.RiskCeilingPreserved)
	require.InDelta(t, 0.70, got.RiskGrossCeilingAfter, 1e-9)
	require.LessOrEqual(t, got.RiskGrossCeilingAfter, 0.70+1e-12)
	require.InDelta(t, 0.15, got.RiskSingleCeilingAfter, 1e-9)
	require.Equal(t, riskBefore, *riskView)

	// ApplyTightenOnly must not copy evil Risk/Execution from a suggestion.
	evil := portfoliolayer.ConstraintSet{
		Risk:      portfoliolayer.RiskLayer{MaxGrossExposurePct: floatPtr(0.99), BlockNewEntries: true, MarketLevel: 9},
		Execution: portfoliolayer.ExecutionLayer{MinLot: 1},
		Portfolio: portfoliolayer.PreferenceLayer{MaxGrossExposurePct: floatPtr(0.99)},
	}
	merged := portfoliorisk.ApplyTightenOnly(cons, evil)
	require.InDelta(t, 0.70, *merged.Risk.MaxGrossExposurePct, 1e-9)
	require.Equal(t, int64(100), merged.Execution.MinLot)
	require.False(t, merged.Risk.BlockNewEntries)
	resolved := merged.Resolve()
	require.InDelta(t, 0.70, resolved.MaxGrossExposurePct, 1e-9)
}

func TestPackage_Isolation(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbiddenImports := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/models",
		"go-stock/backend/risk",
		"go-stock/backend/papertrading",
		"go-stock/backend/decisionprovider",
		"go-stock/backend/tradingautomation",
	}
	forbiddenSrc := []string{
		"CreatePlanWithItems",
		"FreezeTradePlan",
		"NewTradePlanRepo",
		"ExecutePlanItem",
		"BuildDraftTradePlan",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err)
		text := string(src)
		require.NotContains(t, text, "PlanFilter", e.Name())
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, path, "%s imports %s", e.Name(), bad)
			}
		}
	}
}
