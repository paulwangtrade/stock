package rebalance_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/rebalance"

	"github.com/stretchr/testify/require"
)

func TestCalculate_CurrentEqualsTarget_NoAction(t *testing.T) {
	cur := rebalance.CurrentPortfolio{
		Found:  true,
		Equity: 1_000_000,
		Cash:   400_000,
		Positions: []rebalance.CurrentPortfolioPos{
			{Symbol: "sz000001", Qty: 100, MarketValue: 300_000, Weight: 0.30},
			{Symbol: "sz000002", Qty: 100, MarketValue: 300_000, Weight: 0.30},
		},
	}
	tgt := rebalance.TargetPortfolio{
		EquityRef: 1_000_000,
		Positions: []rebalance.TargetPosition{
			{Symbol: "sz000001", TargetWeight: 0.30},
			{Symbol: "sz000002", TargetWeight: 0.30},
		},
	}
	got := rebalance.Calculate(rebalance.RebalanceInput{
		Current: cur,
		Target:  tgt,
		Options: rebalance.DefaultCalculateOptions(),
	})
	require.True(t, got.SuggestOnly)
	require.Equal(t, rebalance.PhaseSuggestOnly, got.Phase)
	require.Empty(t, got.Buys)
	require.Empty(t, got.Reduces)
	require.Empty(t, got.Exits)
	require.True(t, got.NotTradePlan)
	require.True(t, got.NotExecution)
	require.True(t, got.NotHoldingsMutate)
}

func TestCalculate_Overweight_ProducesReduce(t *testing.T) {
	got := rebalance.Calculate(rebalance.RebalanceInput{
		Current: rebalance.CurrentPortfolio{
			Found: true, Equity: 1_000_000, Cash: 200_000,
			Positions: []rebalance.CurrentPortfolioPos{
				{Symbol: "sz000001", Qty: 100, MarketValue: 400_000, Weight: 0.40},
			},
		},
		Target: rebalance.TargetPortfolio{
			EquityRef: 1_000_000,
			Positions: []rebalance.TargetPosition{
				{Symbol: "sz000001", TargetWeight: 0.20},
			},
		},
		Options: rebalance.DefaultCalculateOptions(),
	})
	require.Len(t, got.Reduces, 1)
	require.Empty(t, got.Buys)
	require.Empty(t, got.Exits)
	require.InDelta(t, 0.20, got.Reduces[0].DeltaWeight, 1e-9)
	require.Equal(t, rebalance.ReasonOverweight, got.Reduces[0].Reason)
}

func TestCalculate_MissingTargetName_ProducesBuy(t *testing.T) {
	got := rebalance.Calculate(rebalance.RebalanceInput{
		Current: rebalance.CurrentPortfolio{
			Found: true, Equity: 1_000_000, Cash: 500_000, Exposure: 200_000,
			Positions: []rebalance.CurrentPortfolioPos{
				{Symbol: "sz000001", Qty: 100, MarketValue: 200_000, Weight: 0.20},
			},
		},
		Target: rebalance.TargetPortfolio{
			EquityRef: 1_000_000,
			Positions: []rebalance.TargetPosition{
				{Symbol: "sz000001", TargetWeight: 0.20},
				{Symbol: "sz000002", TargetWeight: 0.20}, // new
			},
		},
		Resolved: allocationengine.ResolvedConstraints{
			MaxGrossExposurePct: 0.95,
			MaxSingleWeight:     0.30,
			ReserveCashRatio:    0.05,
		},
		Options: rebalance.DefaultCalculateOptions(),
	})
	require.NotEmpty(t, got.Buys)
	found := false
	for _, b := range got.Buys {
		if b.Symbol == "sz000002" {
			found = true
			require.Equal(t, rebalance.ReasonNewName, b.Reason)
			require.Greater(t, b.DeltaNotional, 0.0)
		}
	}
	require.True(t, found)
}

func TestCalculate_OrphanCurrent_ProducesExit(t *testing.T) {
	got := rebalance.Calculate(rebalance.RebalanceInput{
		Current: rebalance.CurrentPortfolio{
			Found: true, Equity: 1_000_000,
			Positions: []rebalance.CurrentPortfolioPos{
				{Symbol: "sz000001", Qty: 100, MarketValue: 300_000, Weight: 0.30},
			},
		},
		Target: rebalance.TargetPortfolio{
			EquityRef: 1_000_000,
			Positions: []rebalance.TargetPosition{}, // orphan
		},
		Options: rebalance.DefaultCalculateOptions(),
	})
	require.Len(t, got.Exits, 1)
	require.Equal(t, "sz000001", got.Exits[0].Symbol)
	require.Contains(t, got.Exits[0].ReasonCodes, rebalance.ReasonOrphan)
}

func TestCalculate_RiskTighten_ReducesBuys(t *testing.T) {
	base := rebalance.RebalanceInput{
		Current: rebalance.CurrentPortfolio{
			Found: true, Equity: 1_000_000, Cash: 800_000, Exposure: 100_000,
			Positions: nil,
		},
		Target: rebalance.TargetPortfolio{
			EquityRef: 1_000_000,
			Positions: []rebalance.TargetPosition{
				{Symbol: "sz000001", TargetWeight: 0.40},
			},
		},
		Options: rebalance.DefaultCalculateOptions(),
	}
	loose := base
	loose.Resolved = allocationengine.ResolvedConstraints{
		MaxGrossExposurePct: 0.95,
		MaxSingleWeight:     0.50,
		ReserveCashRatio:    0,
	}
	tight := base
	tight.Resolved = allocationengine.ResolvedConstraints{
		MaxGrossExposurePct: 0.95,
		MaxSingleWeight:     0.15, // tighter single cap
		ReserveCashRatio:    0,
	}
	a := rebalance.Calculate(loose)
	b := rebalance.Calculate(tight)
	require.Len(t, a.Buys, 1)
	require.Len(t, b.Buys, 1)
	require.Greater(t, a.Buys[0].DeltaNotional, b.Buys[0].DeltaNotional)
	require.Greater(t, b.RiskImpact.BuyNotionalCut, 0.0)
	require.Equal(t, rebalance.ReasonSingleCap, b.Buys[0].RiskBinding)
}

func TestCalculate_DeterministicFingerprint(t *testing.T) {
	in := rebalance.RebalanceInput{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Current: rebalance.CurrentPortfolio{
			Found: true, Equity: 1_000_000, Cash: 400_000,
			Positions: []rebalance.CurrentPortfolioPos{
				{Symbol: "sz000002", Weight: 0.10, MarketValue: 100_000, Qty: 10},
				{Symbol: "sz000001", Weight: 0.20, MarketValue: 200_000, Qty: 20},
			},
		},
		Target: rebalance.TargetPortfolio{
			EquityRef: 1_000_000,
			Positions: []rebalance.TargetPosition{
				{Symbol: "sz000001", TargetWeight: 0.25},
				{Symbol: "sz000003", TargetWeight: 0.10},
			},
		},
		Resolved: allocationengine.ResolvedConstraints{MaxSingleWeight: 0.30, MaxGrossExposurePct: 0.9},
		Options:  rebalance.DefaultCalculateOptions(),
	}
	a := rebalance.Calculate(in)
	b := rebalance.Calculate(in)
	require.Equal(t, a.InputsFingerprint, b.InputsFingerprint)
	require.NotEmpty(t, a.InputsFingerprint)
	require.Equal(t, len(a.Buys), len(b.Buys))
}

func TestCurrentFromSnapshot_And_TargetFromAllocation(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 500_000, TotalExposure: 500_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 500_000, Weight: 0.50},
		},
	}
	cur := rebalance.CurrentPortfolioFromSnapshot(snap)
	require.True(t, cur.Found)
	require.Equal(t, rebalance.SourceSnapshot, cur.Source)

	alloc := &allocationengine.AllocationResult{
		Allocated: []allocationengine.NameAllocation{
			{StockCode: "sz000001", TargetAmount: 300_000, InAllocationSet: true, AllocationReason: "equal_split"},
			{StockCode: "sz000002", TargetAmount: 200_000, InAllocationSet: true},
		},
	}
	tgt := rebalance.TargetFromAllocation(alloc, snap.TotalEquity)
	require.Len(t, tgt.Positions, 2)
	require.Equal(t, rebalance.SourcePortfolioAllocation, tgt.Positions[0].Source)

	got := rebalance.Calculate(rebalance.RebalanceInput{
		Current:  cur,
		Target:   tgt,
		Resolved: allocationengine.ResolvedConstraints{MaxSingleWeight: 0.40, MaxGrossExposurePct: 0.9},
		Options:  rebalance.DefaultCalculateOptions(),
		PortfolioRisk: &portfoliorisk.PortfolioRiskSnapshot{
			Found: true,
			Concentration: portfoliorisk.ConcentrationBlock{Available: false},
			Exposure:      portfoliorisk.ExposureBlock{Available: false},
		},
	})
	require.True(t, got.SuggestOnly)
	// 0.50 → 0.30 REDUCE; new 000002 BUY
	require.NotEmpty(t, got.Reduces)
	require.NotEmpty(t, got.Buys)
}

func TestPackageIsolation_CalculateFiles(t *testing.T) {
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
		"BuildDraftTradePlanFromCandidatePool",
		"ExecutePlanItem",
		"FreezeTradePlan",
	}
	fset := token.NewFileSet()
	for _, name := range []string{"calculate.go", "h4_types.go", "from_sources.go"} {
		src, err := os.ReadFile(filepath.Join(wd, name))
		require.NoError(t, err, name)
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, name)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		require.NoError(t, err, name)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, name)
			}
		}
	}
}
