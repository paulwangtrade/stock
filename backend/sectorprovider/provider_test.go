package sectorprovider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go-stock/backend/sectorprovider"
)

func TestLookup_SymbolIndustrySectorSourceVersion(t *testing.T) {
	p := &sectorprovider.TableProvider{
		Entries: map[string]sectorprovider.TableEntry{
			"sz000001": {Industry: "银行", Sector: "金融", Board: "银行"},
		},
		Source:   sectorprovider.SourceFixture,
		Version:  "fix-v1",
		Taxonomy: sectorprovider.TaxonomyFixture,
	}
	got, ok := p.Lookup("SZ000001")
	require.True(t, ok)
	require.Equal(t, "sz000001", got.Symbol)
	require.Equal(t, "银行", got.Industry)
	require.Equal(t, "金融", got.Sector)
	require.Equal(t, "银行", got.Board)
	require.Equal(t, sectorprovider.SourceFixture, got.Source)
	require.Equal(t, "fix-v1", got.Version)
}

func TestSentinelUnknownNotAcceptedAsZero(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "unknown",
		"sz000002": "0",
		"sz000003": "银行",
	}, sectorprovider.SourceInject, "v1", "")
	_, ok := p.Lookup("sz000001")
	require.False(t, ok)
	_, ok = p.Lookup("sz000002")
	require.False(t, ok)
	got, ok := p.Lookup("sz000003")
	require.True(t, ok)
	require.Equal(t, "银行", got.EffectiveSector())

	by, err := p.Resolve([]string{"sz000001", "sz000002", "sz000003"})
	require.NoError(t, err)
	require.Len(t, by, 1)
	m := sectorprovider.IndustryBySymbol(by)
	require.NotContains(t, m, "sz000001")
	require.Equal(t, "银行", m["sz000003"])
}

func TestCoverageReport_IncompleteCannotEnableConstraint(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
	}, "", "v1", "")
	rep := sectorprovider.EvaluateCoverage(p, sectorprovider.CoverageInput{
		Holdings:   []string{"sz000001", "sz000002"},
		Candidates: []string{"sz000858"},
	})
	require.False(t, rep.AllowSectorConstraint)
	require.False(t, rep.HoldingsComplete)
	require.InDelta(t, 0.5, rep.HoldingsCoverage, 1e-9)
	require.Contains(t, rep.HoldingsMissing, "sz000002")
	require.Equal(t, "holdings_coverage_incomplete", rep.Note)
	// missing must not be represented as a zero-weight sector in the report
	require.NotContains(t, rep.Note, "weight=0")
}

func TestCoverageReport_FullHoldingsAllowsConstraint(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
		"sz000002": "地产",
	}, sectorprovider.SourceManual, "v2", sectorprovider.TaxonomyManual)
	rep := sectorprovider.EvaluateCoverage(p, sectorprovider.CoverageInput{
		Holdings:   []string{"sz000001", "sz000002"},
		Candidates: []string{"sz000858"}, // missing candidate OK
	})
	require.True(t, rep.AllowSectorConstraint)
	require.True(t, rep.HoldingsComplete)
	require.InDelta(t, 1.0, rep.HoldingsCoverage, 1e-9)
	require.Contains(t, rep.CandidatesMissing, "sz000858")
	require.Equal(t, sectorprovider.SourceManual, rep.Source)
}

func TestCoverageReport_EmptyHoldingsCannotEnable(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{"sz000001": "银行"}, "", "v1", "")
	rep := sectorprovider.EvaluateCoverage(p, sectorprovider.CoverageInput{})
	require.False(t, rep.AllowSectorConstraint)
	require.Equal(t, "empty_holdings_cannot_enable_sector_constraint", rep.Note)
}

func TestFuncProvider_IndustryToSector(t *testing.T) {
	p := &sectorprovider.FuncProvider{
		Fn: func(symbol string) (industry, board string) {
			if symbol == "sz000001" {
				return "银行", "银行板块"
			}
			return "", ""
		},
		Source:   sectorprovider.SourceCache,
		Version:  "cache-1",
		Taxonomy: sectorprovider.TaxonomyEastmoneyIndustry,
	}
	got, ok := p.Lookup("sz000001")
	require.True(t, ok)
	require.Equal(t, "银行", got.Industry)
	require.Equal(t, "银行", got.Sector) // board not mixed into sector
	require.Equal(t, "银行板块", got.Board)
	_, ok = p.Lookup("sz999999")
	require.False(t, ok)
}

func TestForBuildInput_PartialMapStillReturned(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
	}, "", "v1", "")
	m, meta, err := sectorprovider.ForBuildInput(p, []string{"sz000001", "sz000002"})
	require.NoError(t, err)
	require.Equal(t, "银行", m["sz000001"])
	require.NotContains(t, m, "sz000002")
	require.Equal(t, "v1", meta.Version)
	// Gate still false via CoverageReport
	cov := sectorprovider.EvaluateCoverage(p, sectorprovider.CoverageInput{
		Holdings: []string{"sz000001", "sz000002"},
	})
	require.False(t, cov.AllowSectorConstraint)
}
