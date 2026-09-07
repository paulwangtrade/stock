package portfoliorisk_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestBuild_Deterministic_SameInput(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	snap := &portfolio.Snapshot{
		AsOf:          asOf,
		AccountID:     7,
		Found:         true,
		TotalEquity:   1_000_000,
		Cash:          400_000,
		TotalExposure: 600_000,
		PositionCount: 3,
		Positions: []portfolio.Position{
			{StockCode: "sz000003", Volume: 100, MarketValue: 100_000, Weight: 0.10},
			{StockCode: "sz000001", Volume: 100, MarketValue: 300_000, Weight: 0.30},
			{StockCode: "sz000002", Volume: 100, MarketValue: 200_000, Weight: 0.20},
		},
	}
	risk := &tradingconfig.RiskView{
		MarketLevel:              3,
		BlockNewEntriesOnDefense: false,
		MaxGrossExposurePct:      0.85,
		MaxSingleNamePct:         0.20,
	}
	gross := 0.80
	single := 0.15
	constraints := &portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: &gross,
			MaxSingleNamePct:    &single,
			MarketLevel:         3,
		},
	}
	in := portfoliorisk.BuildInput{
		Snapshot:    snap,
		TradeDate:   "2026-08-21",
		Risk:        risk,
		Constraints: constraints,
		PositionStates: []positionstate.PositionStateView{
			{Symbol: "sz000001", CanSell: false, TotalQty: 100, AvailableQty: 0, State: positionstate.S1NewLocked},
			{Symbol: "sz000002", CanSell: true, TotalQty: 100, AvailableQty: 100, State: positionstate.S2Available},
		},
	}
	a := portfoliorisk.Build(in)
	b := portfoliorisk.Build(in)
	require.Equal(t, a.InputsFingerprint, b.InputsFingerprint)
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))

	require.True(t, a.Found)
	require.True(t, a.RecordOnly)
	require.True(t, a.NotATradePlan)
	require.True(t, a.NotPlanFilter)
	require.True(t, a.NotExecutionRisk)
	require.True(t, a.Exposure.Available)
	require.InDelta(t, 0.60, *a.Exposure.GrossExposure, 1e-9)
	require.InDelta(t, 0.40, *a.Exposure.CashRatio, 1e-9)
	require.InDelta(t, 0.20, *a.Exposure.HeadroomVsCap, 1e-9) // tighter 0.80 - 0.60
	require.True(t, a.Concentration.Available)
	require.InDelta(t, 0.30, *a.Concentration.Top1Weight, 1e-9)
	require.InDelta(t, 0.60, *a.Concentration.Top5Weight, 1e-9)
	require.InDelta(t, 0.15, *a.Concentration.CapSingle, 1e-9)
	require.Equal(t, "configured_level_3", a.Market.MarketRegime)
	require.Equal(t, 3, a.Market.MarketLevel)
	require.True(t, a.PositionLiquidity.Available)
	require.Equal(t, 1, a.PositionLiquidity.SellableNameCount)
	require.Equal(t, 1, a.PositionLiquidity.LockedNameCount)
}

func TestBuild_EmptyIndustry_SafeClose_NoFakeZero(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{Snapshot: foundSnap()})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorUnavailable, got.Sector.Note)
	require.NotNil(t, got.Sector.SectorExposure)
	require.Empty(t, got.Sector.SectorExposure)
	require.Nil(t, got.Sector.MaxSectorWeight)

	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0`)
	require.Contains(t, string(raw), `"available":false`)
	require.NotContains(t, string(raw), `"sector_exposure":[{"sector":""`)
}

func TestBuild_EmptyCorrelation_SafeClose_NullNotZero(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{Snapshot: foundSnap()})
	require.False(t, got.Correlation.Available)
	require.Nil(t, got.Correlation.CorrelationRisk)
	require.Equal(t, portfoliorisk.NoteCorrelationUnavailable, got.Correlation.Note)
	require.False(t, got.Theme.Available)
	require.Empty(t, got.Theme.ThemeExposure)

	raw, err := json.Marshal(got.Correlation)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"correlation_risk":null`)
	require.NotContains(t, string(raw), `"correlation_risk":0`)
}

func TestBuild_MarketLevelProjectionOnly(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot: foundSnap(),
		Risk: &tradingconfig.RiskView{
			MarketLevel:              2,
			BlockNewEntriesOnDefense: true,
		},
	})
	require.Equal(t, "configured_level_2", got.Market.MarketRegime)
	require.Equal(t, portfoliorisk.MarketSourceRiskView, got.Market.Source)
	require.True(t, got.Market.BlockNewEntries)
	require.Contains(t, got.Market.Note, "projected")
}

func TestBuild_NotFound_ClosesBlocks(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot: &portfolio.Snapshot{Found: false, AccountName: "x"},
	})
	require.False(t, got.Found)
	require.False(t, got.Exposure.Available)
	require.False(t, got.Concentration.Available)
	require.False(t, got.Sector.Available)
	require.Nil(t, got.Exposure.GrossExposure)
	require.Nil(t, got.Concentration.Top1Weight)
	require.Nil(t, got.Correlation.CorrelationRisk)
}

func TestPackage_ForbiddenWriteChainRefs(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbiddenImports := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/risk",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/decisionprovider",
		"go-stock/backend/providershadow",
	}
	forbiddenSrc := []string{
		"PlanFilter(",
		"risk.PlanFilter",
		"CreatePlanWithItems",
		"ExecutePlanItem",
		"NewTradePlanRepo",
		"FreezeTradePlan",
		"TradePlan{",
		"broker.TradeOrder",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(wd, e.Name())
		src, err := os.ReadFile(path)
		require.NoError(t, err, e.Name())
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, "%s imports %s", e.Name(), bad)
			}
		}
	}
}

func foundSnap() *portfolio.Snapshot {
	return &portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		Found:         true,
		TotalEquity:   500_000,
		Cash:          200_000,
		TotalExposure: 300_000,
		PositionCount: 1,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 300_000, Weight: 0.6},
		},
	}
}
