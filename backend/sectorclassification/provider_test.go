package sectorclassification_test

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
	"go-stock/backend/sectorclassification"

	"github.com/stretchr/testify/require"
)

func TestLookup_SymbolToSectorInfo(t *testing.T) {
	p := sectorclassification.StaticProvider{
		Sectors: map[string]string{"sz000001": "银行", "SZ000002": "地产"},
		Source:  sectorclassification.SourceFixture,
		Version: "fixture_v1",
	}
	info, ok := p.Lookup("SZ000001")
	require.True(t, ok)
	require.Equal(t, "银行", info.SectorName)
	require.Equal(t, sectorclassification.SourceFixture, info.Source)
	require.Equal(t, "fixture_v1", info.Version)

	_, ok = p.Lookup("sz999999")
	require.False(t, ok)
}

func TestBuild_WithIndustryData_SectorAvailableTrue(t *testing.T) {
	t.Parallel()
	snap := &portfolio.Snapshot{
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
	prov := sectorclassification.StaticProvider{
		Sectors: map[string]string{
			"sz000001": "银行",
			"sz000002": "银行",
		},
		Source:  sectorclassification.SourceFixture,
		Version: "fixture_v1",
	}
	by, ver, meta := sectorclassification.ForBuildInput(prov, portfoliorisk.HoldingCodes(snap))
	require.Equal(t, 2, meta.ResolvedCount)
	require.InDelta(t, 1.0, meta.Coverage, 1e-9)
	require.Equal(t, "fixture_v1", ver)

	maxSec := 0.25
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: by,
		IndustryTaxonomy: ver,
		Constraints: &portfoliolayer.ConstraintSet{
			Portfolio: portfoliolayer.PreferenceLayer{MaxSectorWeight: &maxSec},
		},
	})
	require.True(t, got.Sector.Available)
	require.Equal(t, "fixture_v1", got.Sector.Taxonomy)
	require.Len(t, got.Sector.SectorExposure, 1)
	require.Equal(t, "银行", got.Sector.SectorExposure[0].Sector)
	require.InDelta(t, 0.60, got.Sector.SectorExposure[0].Weight, 1e-9)
	require.NotNil(t, got.Sector.MaxSectorWeight)
	require.InDelta(t, 0.25, *got.Sector.MaxSectorWeight, 1e-9)

	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0,`)
	require.NotContains(t, string(raw), `"max_sector_weight":0}`)
}

func TestBuild_NoIndustryData_SectorAvailableFalse(t *testing.T) {
	t.Parallel()
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 1_000_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 100_000, Weight: 0.10},
		},
	}
	by, ver, meta := sectorclassification.ForBuildInput(
		sectorclassification.StaticProvider{Version: "empty"},
		portfoliorisk.HoldingCodes(snap),
	)
	require.Nil(t, by)
	require.Equal(t, "", ver)
	require.Equal(t, 0, meta.ResolvedCount)

	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: by,
		IndustryTaxonomy: ver,
	})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorUnavailable, got.Sector.Note)
	require.Nil(t, got.Sector.MaxSectorWeight)
	require.Empty(t, got.Sector.SectorExposure)

	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0,`)
	require.NotContains(t, string(raw), `"max_sector_weight":0}`)
	require.Contains(t, string(raw), `"available":false`)
}

func TestBuild_PartialMissing_SectorAvailableFalse(t *testing.T) {
	t.Parallel()
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 300_000, Weight: 0.30},
			{StockCode: "sz000002", Volume: 100, MarketValue: 300_000, Weight: 0.30},
		},
	}
	prov := sectorclassification.StaticProvider{
		Sectors: map[string]string{
			"sz000001": "银行",
			// sz000002 missing
		},
		Source:  sectorclassification.SourceInject,
		Version: "partial_v1",
	}
	by, ver, meta := sectorclassification.ForBuildInput(prov, portfoliorisk.HoldingCodes(snap))
	require.Equal(t, "partial_v1", ver)
	require.Equal(t, 1, meta.ResolvedCount)
	require.Contains(t, meta.MissingSymbols, "sz000002")
	require.Len(t, by, 1) // partial map returned; Build must fail-close

	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: by,
		IndustryTaxonomy: ver,
	})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorIncompleteCoverage, got.Sector.Note)
	require.Empty(t, got.Sector.SectorExposure)
	require.Nil(t, got.Sector.MaxSectorWeight)

	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0,`)
	require.NotContains(t, string(raw), `"max_sector_weight":0}`)
}

func TestRefuseSentinelSectors(t *testing.T) {
	p := sectorclassification.StaticProvider{
		Sectors: map[string]string{
			"sz000001": "unknown",
			"sz000002": "0",
			"sz000003": "银行",
		},
	}
	_, ok := p.Lookup("sz000001")
	require.False(t, ok)
	_, ok = p.Lookup("sz000002")
	require.False(t, ok)
	info, ok := p.Lookup("sz000003")
	require.True(t, ok)
	require.Equal(t, "银行", info.SectorName)
}

func TestPackageIsolation(t *testing.T) {
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
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err, e.Name())
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				require.NotEqual(t, bad, path, e.Name())
			}
		}
		text := string(src)
		require.NotContains(t, text, "CreatePlanWithItems")
		require.NotContains(t, text, "ExecutePlanItem")
		require.NotContains(t, text, "BuildDraftTradePlanFromCandidatePool")
	}
}

func TestStrategyBuyChain_NoSectorClassificationImport(t *testing.T) {
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
		require.NotContains(t, string(src), "sectorclassification", e.Name())
	}
}
