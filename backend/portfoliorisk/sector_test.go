package portfoliorisk_test

import (
	"encoding/json"
	"testing"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/sectorclass"

	"github.com/stretchr/testify/require"
)

func TestBuild_FullCoverage_ViaSectorclass_AvailableTrue(t *testing.T) {
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
	maxSec := 0.25
	prov := sectorclass.StaticMap{
		IndustryBySymbol: map[string]string{
			"sz000001": "银行",
			"sz000002": "银行",
		},
		Taxonomy: "fixture_v1",
	}
	by, tax := sectorclass.ForBuildInput(prov, sectorclass.ClassifyInput{
		Holdings:   portfoliorisk.HoldingCodes(snap),
		Candidates: []string{"sz000099"},
	})
	require.Len(t, by, 2) // candidate missing from table → omitted

	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: by,
		IndustryTaxonomy: tax,
		Constraints: &portfoliolayer.ConstraintSet{
			Portfolio: portfoliolayer.PreferenceLayer{MaxSectorWeight: &maxSec},
		},
	})
	require.True(t, got.Sector.Available)
	require.Equal(t, "fixture_v1", got.Sector.Taxonomy)
	require.Len(t, got.Sector.SectorExposure, 1)
	require.Equal(t, "银行", got.Sector.SectorExposure[0].Sector)
	require.InDelta(t, 0.60, got.Sector.SectorExposure[0].Weight, 1e-9)
	require.Equal(t, 2, got.Sector.SectorExposure[0].NameCount)
	require.NotNil(t, got.Sector.MaxSectorWeight)
	require.InDelta(t, 0.25, *got.Sector.MaxSectorWeight, 1e-9)
}

func TestBuild_PartialCoverage_SectorUnavailable(t *testing.T) {
	t.Parallel()
	snap := foundSnap() // sz000001 only
	by, tax := sectorclass.ForBuildInput(sectorclass.StaticMap{
		IndustryBySymbol: map[string]string{"sz000002": "地产"},
		Taxonomy:         "fixture",
	}, sectorclass.ClassifyInput{
		Holdings:   portfoliorisk.HoldingCodes(snap),
		Candidates: nil,
	})
	// Provider returns nil/empty when holding code unresolved
	require.Nil(t, by)

	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot: snap,
		IndustryBySymbol: map[string]string{
			"sz000002": "地产", // inject partial map directly
		},
		IndustryTaxonomy: tax,
	})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorIncompleteCoverage, got.Sector.Note)
	require.Empty(t, got.Sector.SectorExposure)
	require.Nil(t, got.Sector.MaxSectorWeight)

	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight":0,`)
	require.NotContains(t, string(raw), `"max_sector_weight":0}`)
	require.Contains(t, string(raw), `"available":false`)
}

func TestBuild_EmptyIndustryInput_SectorFalse(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{Snapshot: foundSnap()})
	require.False(t, got.Sector.Available)
	require.Equal(t, portfoliorisk.NoteSectorUnavailable, got.Sector.Note)
	require.Nil(t, got.Sector.MaxSectorWeight)
}

func TestBuild_FullCoverage_NoMaxSectorCap_OmitsPointer(t *testing.T) {
	t.Parallel()
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot: foundSnap(),
		IndustryBySymbol: map[string]string{
			"sz000001": "银行",
		},
	})
	require.True(t, got.Sector.Available)
	require.Nil(t, got.Sector.MaxSectorWeight)
	require.Len(t, got.Sector.SectorExposure, 1)
	raw, err := json.Marshal(got.Sector)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"max_sector_weight"`)
}

func TestBuild_EmptyBook_WithMap_SectorAvailableEmptyExposure(t *testing.T) {
	t.Parallel()
	snap := &portfolio.Snapshot{
		AsOf:        time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		Found:       true,
		TotalEquity: 100_000,
		Cash:        100_000,
	}
	got := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: map[string]string{"sz000001": "银行"},
	})
	require.True(t, got.Sector.Available)
	require.Empty(t, got.Sector.SectorExposure)
	require.Nil(t, got.Sector.MaxSectorWeight)
}

func TestBuild_IndustryFingerprintChanges(t *testing.T) {
	t.Parallel()
	snap := foundSnap()
	a := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: map[string]string{"sz000001": "银行"},
	})
	b := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: map[string]string{"sz000001": "地产"},
	})
	require.NotEqual(t, a.InputsFingerprint, b.InputsFingerprint)
}

func TestSuggestTighten_SectorOverCap_UsesBuildIndustry(t *testing.T) {
	t.Parallel()
	snap := &portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		Found:         true,
		TotalEquity:   1_000_000,
		Cash:          200_000,
		TotalExposure: 800_000,
		PositionCount: 1,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 100, MarketValue: 400_000, Weight: 0.40},
		},
	}
	maxSec := 0.25
	riskSnap := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:         snap,
		IndustryBySymbol: map[string]string{"sz000001": "银行"},
		Constraints: &portfoliolayer.ConstraintSet{
			Portfolio: portfoliolayer.PreferenceLayer{MaxSectorWeight: &maxSec},
		},
	})
	require.True(t, riskSnap.Sector.Available)
	got := portfoliorisk.SuggestTighten(riskSnap, portfoliolayer.ConstraintSet{})
	require.NotNil(t, got.ConstraintSet.Portfolio.MaxSectorWeight)
	require.InDelta(t, 0.25, *got.ConstraintSet.Portfolio.MaxSectorWeight, 1e-9)
}
