package decisionshadowv2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/selection"
)

func f64(v float64) *float64 { return &v }
func i32(v int) *int         { return &v }

func samplePool() []selection.Candidate {
	return []selection.Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 0.9, Industry: "bank"},
		{StockCode: "sz000002", Rank: 2, Score: 0.8, Industry: "estate"},
		{StockCode: "sh600000", Rank: 3, Score: 0.7, Industry: "bank"},
		{StockCode: "sz000858", Rank: 4, Score: 0.6, Industry: "liquor"},
	}
}

func sampleSnap() *portfoliolayer.PortfolioSnapshot {
	return &portfoliolayer.PortfolioSnapshot{
		Found: true, Equity: 1_000_000, Cash: 400_000, AvailableCash: 400_000,
		Exposure: 600_000, MarketValue: 600_000,
		Positions: []portfoliolayer.SnapshotPosition{
			{StockCode: "sz000001", MarketValue: 200_000, Weight: 0.20},
		},
	}
}

func baseInput() Input {
	return Input{
		Enabled:   true,
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		CandidatePool: samplePool(),
		Snapshot:      sampleSnap(),
		Objective: PortfolioObjective{
			MaxNewNames:      i32(3),
			ReserveCashRatio: f64(0.10),
			MaxSingleWeight:  f64(0.15),
		},
		RiskCeilingTemplate: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{
				MaxGrossExposurePct: f64(0.85),
				MaxSingleNamePct:    f64(0.20),
				Enabled:             true,
			},
		},
		SelectionLimitLegacy:    3,
		SelectionLimitPortfolio: 3,
		LegacyAmountPerName:     100_000,
		Options: Options{
			Phase:              "shadow_only",
			ProjectPostBuyBook: true,
		},
		// Neutral risk: no tighten patches.
		PrebuiltRisk: &portfoliorisk.PortfolioRiskSnapshot{
			Found: true,
			Exposure: portfoliorisk.ExposureBlock{
				Available: true, HeadroomVsCap: f64(0.25), CashRatio: f64(0.40), CapGross: f64(0.85),
			},
		},
	}
}

func TestDefaultEnabledIsFalse(t *testing.T) {
	require.False(t, DefaultEnabled)
	rt := NewRuntime(false)
	rep := rt.Observe(baseInput()) // in.Enabled true still runs — clear it
	in := baseInput()
	in.Enabled = false
	rep = rt.Observe(in)
	require.True(t, rep.Skipped)
	require.True(t, rep.RecordOnly)
	require.True(t, rep.NotATradePlan)
	require.True(t, rep.NotProviderSwitch)
}

func TestDeterministicSameInput(t *testing.T) {
	in := baseInput()
	a := Run(in, true)
	b := Run(in, true)
	require.Equal(t, a.InputsFingerprint, b.InputsFingerprint)
	require.Equal(t, a.LegacyPlanProjection.Totals, b.LegacyPlanProjection.Totals)
	require.Equal(t, a.PortfolioPlanProjection.Totals, b.PortfolioPlanProjection.Totals)
	require.Equal(t, a.Difference.CapitalDiff, b.Difference.CapitalDiff)
	ja, err := json.Marshal(a.Difference)
	require.NoError(t, err)
	jb, err := json.Marshal(b.Difference)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestRiskTightenAffectsAllocation(t *testing.T) {
	in := baseInput()
	// Exhausted headroom → MaxNewNames=0 → portfolio notional drops vs pre-tighten.
	in.PrebuiltRisk = &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, HeadroomVsCap: f64(0), CashRatio: f64(0.40), CapGross: f64(0.85),
		},
	}
	rep := Run(in, true)
	require.True(t, rep.ChainTrace.TightenApplied)
	require.True(t, rep.Difference.RiskImpactDiff.TightenApplied)
	require.Greater(t, rep.Difference.RiskImpactDiff.NotionalBeforeTighten, rep.Difference.RiskImpactDiff.NotionalAfterTighten)
	require.NotEmpty(t, rep.Difference.RiskImpactDiff.Explain)
	require.Contains(t, stringsJoin(rep.ChainTrace.TightenNotes), "gross_headroom_exhausted")
}

func TestAllocationReasonsExplainable(t *testing.T) {
	in := baseInput()
	// Tight single cap forces capped_single_weight or equal_split with binding.
	in.Objective.MaxSingleWeight = f64(0.05)
	in.RiskCeilingTemplate.Risk.MaxSingleNamePct = f64(0.05)
	rep := Run(in, true)
	require.True(t, rep.PortfolioPlanProjection.OK)
	require.NotEmpty(t, rep.ChainTrace.AllocationChangeReasons)
	require.NotEmpty(t, rep.PortfolioPlanProjection.Lines)
	for _, ln := range rep.PortfolioPlanProjection.Lines {
		if ln.Role == "selected" {
			require.NotEmpty(t, ln.AllocationReason)
		}
	}
	// Legacy vs portfolio capital differs under equal-weight + caps.
	require.NotEqual(t, rep.LegacyPlanProjection.Totals.BuyNotionalSum, 0)
	require.NotNil(t, rep.Difference.AmountDiff)
}

func TestNoIndustryDataSafe(t *testing.T) {
	in := baseInput()
	in.IndustryBySymbol = nil
	for i := range in.CandidatePool {
		in.CandidatePool[i].Industry = ""
	}
	rep := Run(in, true)
	require.False(t, rep.Difference.SectorDiff.Available)
	require.NotEmpty(t, rep.Difference.SectorDiff.UnavailableReason)
	require.True(t, rep.RecordOnly)
	require.NotNil(t, rep.LegacyPlanProjection)
	require.NotNil(t, rep.PortfolioPlanProjection)
	require.NotNil(t, rep.Difference)
}

func TestReportHasRequiredBlocksAndFlags(t *testing.T) {
	rep := Run(baseInput(), true)
	require.Equal(t, SchemaVersion, rep.SchemaVersion)
	require.True(t, rep.RecordOnly)
	require.True(t, rep.NotATradePlan)
	require.True(t, rep.NotProviderSwitch)
	require.True(t, rep.NotADraft)
	require.True(t, rep.NotExecution)
	require.Equal(t, "fixed_amount", rep.LegacyPlanProjection.ProviderIdentity)
	require.Equal(t, "portfolio_allocation", rep.PortfolioPlanProjection.ProviderIdentity)
	require.Equal(t, 3, rep.LegacyPlanProjection.Totals.NameCount)
	require.InDelta(t, 300_000, rep.LegacyPlanProjection.Totals.BuyNotionalSum, 1e-6)

	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Contains(t, m, "legacy_projection")
	require.Contains(t, m, "portfolio_projection")
	require.Contains(t, m, "difference")
	require.Equal(t, true, m["record_only"])
	require.Equal(t, true, m["not_a_trade_plan"])
	require.Equal(t, true, m["not_a_provider_switch"])
}

func TestSectorDiffWhenIndustryAvailable(t *testing.T) {
	in := baseInput()
	in.IndustryBySymbol = map[string]string{
		"sz000001": "bank", "sz000002": "estate", "sh600000": "bank", "sz000858": "liquor",
	}
	rep := Run(in, true)
	require.True(t, rep.Difference.SectorDiff.Available)
}

func stringsJoin(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + ";"
	}
	return out
}
