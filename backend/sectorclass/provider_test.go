package sectorclass_test

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
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/sectorclass"

	"github.com/stretchr/testify/require"
)

func TestClassify_EmptyInput_EmptyMap(t *testing.T) {
	t.Parallel()
	p := sectorclass.StaticMap{IndustryBySymbol: map[string]string{"sz000001": "银行"}}
	res, err := sectorclass.Classify(p, sectorclass.ClassifyInput{})
	require.NoError(t, err)
	require.Nil(t, res.IndustryBySymbol)

	by, tax := sectorclass.ForBuildInput(p, sectorclass.ClassifyInput{})
	require.Nil(t, by)
	require.Empty(t, tax)

	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         foundSnap(),
		IndustryBySymbol: by,
		IndustryTaxonomy: tax,
	})
	require.False(t, got.Sector.Available)
}

func TestClassify_PartialHoldings_BuildSectorFalse(t *testing.T) {
	t.Parallel()
	p := sectorclass.StaticMap{
		IndustryBySymbol: map[string]string{"sz000001": "银行"},
		Taxonomy:         "fixture",
	}
	res, err := sectorclass.Classify(p, sectorclass.ClassifyInput{
		Holdings:   []string{"sz000001", "sz000002"},
		Candidates: []string{"sz000003"},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"sz000001": "银行"}, res.IndustryBySymbol)
	require.False(t, res.Meta.HoldingsComplete)
	require.Contains(t, res.Meta.MissingSymbols, "sz000002")

	snap := twoPosSnap()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: res.IndustryBySymbol,
		IndustryTaxonomy: res.Meta.Taxonomy,
	})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorIncompleteCoverage, got.Sector.Note)
	require.Nil(t, got.Sector.MaxSectorWeight)
	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0,`)
	require.NotContains(t, string(raw), `"max_sector_weight":0}`)
	require.NotContains(t, string(raw), `"sector":"unknown"`)
}

func TestClassify_FullHoldingsCoverage_BuildSectorTrue(t *testing.T) {
	t.Parallel()
	p := sectorclass.StaticMap{
		IndustryBySymbol: map[string]string{
			"SZ000001": "银行",
			"sz000002": "银行",
			"sz000099": "可选候选",
		},
		Taxonomy: "fixture_v1",
		Source:   sectorclass.SourceFixture,
	}
	by, tax := sectorclass.ForBuildInput(p, sectorclass.ClassifyInput{
		Holdings:   []string{"sz000001", "sz000002"},
		Candidates: []string{"sz000099"},
	})
	require.NotNil(t, by)
	require.Equal(t, "fixture_v1", tax)
	require.Equal(t, "银行", by["sz000001"])
	require.Equal(t, "可选候选", by["sz000099"])

	maxSec := 0.25
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         twoPosSnap(),
		IndustryBySymbol: by,
		IndustryTaxonomy: tax,
		Constraints: &portfoliolayer.ConstraintSet{
			Portfolio: portfoliolayer.PreferenceLayer{MaxSectorWeight: &maxSec},
		},
	})
	require.True(t, got.Sector.Available)
	require.Equal(t, "fixture_v1", got.Sector.Taxonomy)
	require.Len(t, got.Sector.SectorExposure, 1)
	require.InDelta(t, 0.60, got.Sector.SectorExposure[0].Weight, 1e-9)
	require.NotNil(t, got.Sector.MaxSectorWeight)
	require.InDelta(t, 0.25, *got.Sector.MaxSectorWeight, 1e-9)
}

func TestNormalize_RejectsUnknownSentinel(t *testing.T) {
	t.Parallel()
	got := sectorclass.Normalize(map[string]string{
		"sz000001": "unknown",
		"sz000002": "0",
		"sz000003": "银行",
	})
	require.Equal(t, map[string]string{"sz000003": "银行"}, got)
}

func TestPackage_NoTradeChainImports(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbidden := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/risk",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/allocationengine",
		"go-stock/backend/decisionprovider",
		"go-stock/backend/portfoliorisk",
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
		require.NotContains(t, text, "CreatePlanWithItems")
		require.NotContains(t, text, "ExecutePlanItem")
		require.NotContains(t, text, "FreezeTradePlan")
		require.NotContains(t, text, "risk.PlanFilter")
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				require.NotEqual(t, bad, path, e.Name())
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

func twoPosSnap() *portfolio.Snapshot {
	return &portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		Found:         true,
		TotalEquity:   1_000_000,
		Cash:          400_000,
		TotalExposure: 600_000,
		PositionCount: 2,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 300_000, Weight: 0.30},
			{StockCode: "sz000002", Volume: 100, MarketValue: 300_000, Weight: 0.30},
		},
	}
}
