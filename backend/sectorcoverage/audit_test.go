package sectorcoverage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/sectorcoverage"
	"go-stock/backend/sectorprovider"
)

func TestAudit_PoolAndHoldingsCoverage(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
		"sz000002": "地产",
	}, sectorprovider.SourceFixture, "v1", sectorprovider.TaxonomyFixture)

	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		AsOf:     time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC),
		Holdings: []string{"sz000001", "sz000002"},
		Pool:     []string{"sz000001", "sz000858", "sh600000"},
	})
	require.Equal(t, sectorcoverage.SchemaVersion, rep.SchemaVersion)
	require.InDelta(t, 1.0, rep.HoldingsCoverage, 1e-9)
	require.InDelta(t, 1.0/3.0, rep.PoolCoverage, 1e-9)
	require.Equal(t, 2, rep.HoldingsResolved)
	require.Equal(t, 1, rep.PoolResolved)
	require.Contains(t, rep.PoolMissing, "sz000858")
	require.Contains(t, rep.IndustryClassificationMissing, "sz000858")
	require.True(t, rep.AllowSectorConstraint)
	require.True(t, rep.HoldingsComplete)
	require.Equal(t, 0, rep.UnknownCount)
	require.True(t, rep.NotTradePlan)
	require.True(t, rep.NotExecution)
}

func TestAudit_IncompleteHoldings_CannotEnableConstraint(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
	}, "", "v1", "")
	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		Holdings: []string{"sz000001", "sz000002"},
		Pool:     []string{"sz000001"},
	})
	require.False(t, rep.AllowSectorConstraint)
	require.False(t, rep.HoldingsComplete)
	require.InDelta(t, 0.5, rep.HoldingsCoverage, 1e-9)
	require.Contains(t, rep.HoldingsMissing, "sz000002")
	require.Contains(t, rep.IndustryClassificationMissing, "sz000002")
	require.Equal(t, sectorcoverage.NoteIncomplete, rep.Note)
}

func TestAudit_UnknownCount_BlocksConstraint(t *testing.T) {
	p := &sectorprovider.TableProvider{
		Entries: map[string]sectorprovider.TableEntry{
			"sz000001": {Industry: "银行", Sector: "银行"},
			"sz000002": {Industry: "unknown", Sector: "unknown"},
		},
		Source:   sectorprovider.SourceInject,
		Version:  "v1",
		Taxonomy: sectorprovider.TaxonomyFixture,
	}
	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		Holdings: []string{"sz000001", "sz000002"},
	})
	require.False(t, rep.AllowSectorConstraint)
	require.Equal(t, 1, rep.UnknownCount)
	require.Equal(t, sectorcoverage.NoteUnknownPresent, rep.Note)
	require.Contains(t, rep.HoldingsMissing, "sz000002")
	var found bool
	for _, g := range rep.ClassificationGaps {
		if g.Symbol == "sz000002" && g.Outcome == sectorcoverage.OutcomeUnknown {
			found = true
		}
	}
	require.True(t, found)
}

func TestAudit_EmptyHoldings_CannotEnable(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{"sz000001": "银行"}, "", "v1", "")
	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		Pool: []string{"sz000001"},
	})
	require.False(t, rep.AllowSectorConstraint)
	require.Equal(t, sectorcoverage.NoteEmptyHoldings, rep.Note)
	require.InDelta(t, 1.0, rep.PoolCoverage, 1e-9)
}

func TestAudit_WithPortfolioRisk_PartialCoverage(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
		// sz000002 missing
	}, "", "v1", "")
	maxSec := 0.25
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 200_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1000, Weight: 0.40, MarketValue: 400_000},
			{StockCode: "sz000002", Volume: 500, Weight: 0.20, MarketValue: 200_000},
		},
	}
	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		Holdings:            []string{"sz000001", "sz000002"},
		AttachPortfolioRisk: true,
		Snapshot:            snap,
		Constraints: &portfoliolayer.ConstraintSet{
			Portfolio: portfoliolayer.PreferenceLayer{MaxSectorWeight: &maxSec},
		},
	})
	require.True(t, rep.PortfolioRiskAttached)
	require.NotNil(t, rep.PortfolioRiskSectorAvailable)
	require.False(t, *rep.PortfolioRiskSectorAvailable)
	require.False(t, rep.AllowSectorConstraint)
}

func TestAudit_WithPortfolioRisk_FullCoverage(t *testing.T) {
	p := sectorprovider.NewTableFromIndustryMap(map[string]string{
		"sz000001": "银行",
		"sz000002": "银行",
	}, sectorprovider.SourceFixture, "v1", sectorprovider.TaxonomyFixture)
	snap := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1000, Weight: 0.30, MarketValue: 300_000},
			{StockCode: "sz000002", Volume: 800, Weight: 0.30, MarketValue: 300_000},
		},
	}
	rep := sectorcoverage.Audit(p, sectorcoverage.AuditInput{
		Holdings:            []string{"sz000001", "sz000002"},
		AttachPortfolioRisk: true,
		Snapshot:            snap,
	})
	require.True(t, rep.AllowSectorConstraint)
	require.NotNil(t, rep.PortfolioRiskSectorAvailable)
	require.True(t, *rep.PortfolioRiskSectorAvailable)
}
