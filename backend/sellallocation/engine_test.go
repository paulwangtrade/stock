package sellallocation_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/sellallocation"

	"github.com/stretchr/testify/require"
)

func TestEXIT_FlattenAllAvailable(t *testing.T) {
	dec := rules.HoldingDecision{
		Symbol: "sz000001", FinalAction: rules.ActionExit, Action: rules.ActionExit,
		ReasonCodes: []string{"PNL_MAX_LOSS"}, Explanation: "exit observe",
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true,
		Weight: 0.20, MarketValue: 200_000, MarkPrice: 200,
	}
	got := sellallocation.AllocateFromDecision(dec, pos, nil, nil, 1_000_000, sellallocation.DefaultOptions())
	require.Equal(t, sellallocation.ActionExit, got.Action)
	require.Equal(t, int64(1000), got.SuggestedSellQty) // full available, lot OK
	require.Equal(t, int64(1000), got.CurrentQty)
	require.Equal(t, int64(1000), got.AvailableQty)
	require.InDelta(t, 0.20, got.CurrentWeight, 1e-9)
	require.InDelta(t, 0.0, got.TargetWeight, 1e-9)
	require.Contains(t, got.ReasonCodes, "PNL_MAX_LOSS")
	require.True(t, got.SuggestOnly)
	require.True(t, got.NotTradePlan)
	require.True(t, got.NotExecution)
	require.True(t, got.NotBuyChain)
}

func TestREDUCE_PartialSell(t *testing.T) {
	opt := sellallocation.DefaultOptions()
	opt.DefaultReduceRatio = 0.5
	opt.PreferWeightVsRatio = sellallocation.PreferRatioOnly
	intent := sellallocation.SellIntent{
		Symbol: "sz000001", Action: sellallocation.ActionReduce,
		TargetReduceRatio: 0.5, TargetPositionWeight: 0.10,
		Reason: "TRIM", ReasonCodes: []string{"OVERWEIGHT"},
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true,
		Weight: 0.20, MarkPrice: 100,
	}
	got := sellallocation.Allocate(sellallocation.Input{
		Intent: intent, Position: pos, Equity: 1_000_000, Options: opt,
	})
	require.Equal(t, sellallocation.ActionReduce, got.Action)
	require.Equal(t, int64(500), got.SuggestedSellQty)
	require.InDelta(t, 0.20, got.CurrentWeight, 1e-9)
	require.InDelta(t, 0.10, got.TargetWeight, 1e-9)
	require.True(t, got.ExecutableHint)
}

func TestT1_CannotSell(t *testing.T) {
	intent := sellallocation.SellIntent{
		Symbol: "sz000001", Action: sellallocation.ActionExit,
		TargetReduceRatio: 1, Reason: "EXIT",
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 0, LockedQty: 1000,
		CanSell: false, Weight: 0.15,
	}
	got := sellallocation.Allocate(sellallocation.Input{Intent: intent, Position: pos, Equity: 1e6, Options: sellallocation.DefaultOptions()})
	require.Equal(t, sellallocation.ActionExit, got.Action) // keep why-sell
	require.Equal(t, int64(0), got.SuggestedSellQty)        // locked qty not suggested
	require.False(t, got.ExecutableHint)
	require.Equal(t, sellallocation.BindingT1, got.Binding)
	require.Contains(t, got.ReasonCodes, sellallocation.ReasonT1Locked)
}

func TestLot_FloorTo100(t *testing.T) {
	opt := sellallocation.DefaultOptions()
	opt.LotSize = 100
	opt.PreferWeightVsRatio = sellallocation.PreferRatioOnly
	intent := sellallocation.SellIntent{
		Symbol: "sz000001", Action: sellallocation.ActionReduce,
		TargetReduceRatio: 0.33, TargetPositionWeight: 0.1, Reason: "TRIM",
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true, Weight: 0.2, MarkPrice: 10,
	}
	got := sellallocation.Allocate(sellallocation.Input{Intent: intent, Position: pos, Equity: 1e6, Options: opt})
	require.Equal(t, int64(300), got.SuggestedSellQty) // 330 → 300
	require.Contains(t, got.ReasonCodes, sellallocation.ReasonLotRound)
}

func TestNoRiskSnapshot_Safe(t *testing.T) {
	opt := sellallocation.DefaultOptions()
	opt.RiskBoostEnabled = true // even with boost on, nil risk must not panic
	intent := sellallocation.SellIntent{
		Symbol: "sz000001", Action: sellallocation.ActionReduce,
		TargetReduceRatio: 0.5, TargetPositionWeight: 0.1, Reason: "TRIM",
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true, Weight: 0.2, MarkPrice: 10,
	}
	got := sellallocation.Allocate(sellallocation.Input{
		Intent: intent, Position: pos, PortfolioRisk: nil, Equity: 1e6, Options: opt,
	})
	require.Equal(t, sellallocation.ActionReduce, got.Action)
	require.Greater(t, got.SuggestedSellQty, int64(0))
	require.NotContains(t, got.ReasonCodes, sellallocation.ReasonRiskBoost)
}

func TestRiskBoost_IncreasesReduce(t *testing.T) {
	baseOpt := sellallocation.DefaultOptions()
	baseOpt.PreferWeightVsRatio = sellallocation.PreferRatioOnly
	baseOpt.DefaultReduceRatio = 0.3
	cap := 0.15
	risk := &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, CapSingle: &cap,
		},
	}
	intent := sellallocation.SellIntent{
		Symbol: "sz000001", Action: sellallocation.ActionReduce,
		TargetReduceRatio: 0.3, TargetPositionWeight: 0.25, Reason: "OVER",
	}
	pos := sellallocation.CurrentPosition{
		Symbol: "sz000001", TotalQty: 1000, AvailableQty: 1000, CanSell: true,
		Weight: 0.35, MarkPrice: 100,
	}
	noBoost := sellallocation.Allocate(sellallocation.Input{
		Intent: intent, Position: pos, PortfolioRisk: risk, Equity: 1e6, Options: baseOpt,
	})
	boostOpt := baseOpt
	boostOpt.RiskBoostEnabled = true
	withBoost := sellallocation.Allocate(sellallocation.Input{
		Intent: intent, Position: pos, PortfolioRisk: risk, Equity: 1e6, Options: boostOpt,
	})
	require.Greater(t, withBoost.SuggestedSellQty, noBoost.SuggestedSellQty)
	require.Contains(t, withBoost.ReasonCodes, sellallocation.ReasonRiskBoost)
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
		"go-stock/backend/allocationengine",
	}
	forbiddenSrc := []string{
		"CreatePlanWithItems",
		"BuildDraftTSellTradePlan",
		"ExecutePlanItem",
		"BuildDraftTradePlanFromCandidatePool",
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
