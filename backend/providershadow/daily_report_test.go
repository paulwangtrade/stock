package providershadow

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/decisionprovider"

	"github.com/stretchr/testify/require"
)

func TestBuildPortfolioShadowDailyReport_Empty(t *testing.T) {
	t.Parallel()
	got := BuildPortfolioShadowDailyReport(nil, "2026-08-21")
	require.Equal(t, DailyReportSchema, got.SchemaVersion)
	require.Equal(t, "2026-08-21", got.TradeDate)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotAProviderSwitch)
	require.Zero(t, got.RecordCount)
	require.Empty(t, got.OnlyLegacyStocks)
	require.Empty(t, got.OnlyPortfolioStocks)
}

func TestBuildPortfolioShadowDailyReport_AveragesAndUnions(t *testing.T) {
	t.Parallel()
	d1 := 0.0
	d2 := -20000.0
	recs := []ShadowComparisonRecord{
		{
			TradeDate:  "2026-08-21",
			Comparable: true,
			LegacySummary: EnvelopeRef{
				OK: true, Provider: decisionprovider.ProviderFixedAmount, LineCount: 3,
			},
			PortfolioSummary: EnvelopeRef{
				OK: true, Provider: decisionprovider.ProviderPortfolioAllocation, LineCount: 2,
			},
			Report: &ProviderComparisonReport{
				Comparable: true,
				Legacy:     EnvelopeRef{OK: true, LineCount: 3},
				Portfolio:  EnvelopeRef{OK: true, LineCount: 2},
				Symbols: SymbolDiff{
					OnlyLegacy:    []string{"sz000001"},
					OnlyPortfolio: []string{},
					Common:        []string{"sz000002", "sz000003"},
				},
				Amounts: []AmountDiff{
					{Symbol: "sz000002", Kind: KindBothActual, Delta: &d1},
					{Symbol: "sz000003", Kind: KindBothActual, Delta: &d2},
				},
				Roles: []RoleDiff{
					{Symbol: "sz000001", LegacyRole: RoleSelected, PortfolioRole: RoleWaitlist, Pair: "selected_to_waitlist"},
				},
				Reasons: []ReasonDiff{
					{Symbol: "sz000002", Layer: "decision", LegacyText: "fixed_amount", PortfolioText: "equal_split"},
					{Symbol: "sz000003", Layer: "decision", LegacyText: "fixed_amount", PortfolioText: "below_min_order"},
				},
			},
		},
		{
			TradeDate:  "2026-08-21",
			Comparable: true,
			LegacySummary: EnvelopeRef{
				OK: true, LineCount: 1,
			},
			PortfolioSummary: EnvelopeRef{
				OK: true, LineCount: 1,
			},
			Report: &ProviderComparisonReport{
				Comparable: true,
				Legacy:     EnvelopeRef{LineCount: 1},
				Portfolio:  EnvelopeRef{LineCount: 1},
				Symbols: SymbolDiff{
					OnlyLegacy:    []string{},
					OnlyPortfolio: []string{"sz000099"},
					Common:        []string{"sz000002"},
				},
				Amounts: []AmountDiff{},
				Roles: []RoleDiff{
					{Symbol: "sz000004", LegacyRole: RoleSelected, PortfolioRole: RoleRejected, Pair: "selected_to_rejected"},
				},
				Reasons: []ReasonDiff{
					{Symbol: "sz000002", Layer: "decision", LegacyText: "fixed_amount", PortfolioText: "equal_split"},
				},
			},
		},
		{
			TradeDate:     "2026-08-20",
			Comparable:    true,
			LegacySummary: EnvelopeRef{LineCount: 99},
			Report: &ProviderComparisonReport{
				Comparable: true,
				Symbols:    SymbolDiff{OnlyLegacy: []string{"ignore"}},
			},
		},
		{
			TradeDate:  "2026-08-21",
			Comparable: false,
			Failure:    ShadowFailure{PortfolioFailed: true, ErrorReason: "boom"},
			Report: &ProviderComparisonReport{
				Comparable:         false,
				IncomparableReason: IncomparablePortfolioFailed,
			},
		},
	}

	got := BuildPortfolioShadowDailyReport(recs, "2026-08-21")
	require.Equal(t, 3, got.RecordCount)
	require.Equal(t, 2, got.ComparableCount)
	require.Equal(t, 1, got.IncomparableCount)
	require.InDelta(t, 2.0, got.AvgLegacyStockCount, 1e-9)    // (3+1)/2
	require.InDelta(t, 1.5, got.AvgPortfolioStockCount, 1e-9) // (2+1)/2
	require.Equal(t, 2, got.AmountDiffSampleCount)
	require.InDelta(t, -10000.0, got.AvgAmountDelta, 1e-9) // (0-20000)/2
	require.InDelta(t, 10000.0, got.AvgAbsAmountDelta, 1e-9)
	require.Equal(t, []string{"sz000001"}, got.OnlyLegacyStocks)
	require.Equal(t, []string{"sz000099"}, got.OnlyPortfolioStocks)
	require.NotEmpty(t, got.TopConstraintImpacts)
	require.NotEmpty(t, got.AllocationChangeReasons)

	var foundEqualSplit, foundBelowMin, foundRoleWait bool
	for _, c := range got.AllocationChangeReasons {
		if c.Reason == "decision:fixed_amount→equal_split" {
			foundEqualSplit = true
			require.Equal(t, 2, c.Count)
		}
	}
	for _, c := range got.TopConstraintImpacts {
		if c.Reason == "alloc:below_min_order" {
			foundBelowMin = true
		}
		if c.Reason == "role:selected_to_waitlist" {
			foundRoleWait = true
		}
	}
	require.True(t, foundEqualSplit)
	require.True(t, foundBelowMin)
	require.True(t, foundRoleWait)
}

func TestAnalyzeStore_FiltersTradeDate(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	require.NoError(t, store.Append(ShadowComparisonRecord{
		RunID:            "a",
		TradeDate:        "2026-08-21",
		Comparable:       true,
		RecordedAt:       time.Now(),
		LegacySummary:    EnvelopeRef{LineCount: 2},
		PortfolioSummary: EnvelopeRef{LineCount: 2},
		Report: &ProviderComparisonReport{
			Comparable: true,
			Legacy:     EnvelopeRef{LineCount: 2},
			Portfolio:  EnvelopeRef{LineCount: 2},
			Symbols:    emptySymbolDiff(),
		},
	}))
	require.NoError(t, store.Append(ShadowComparisonRecord{
		RunID:      "b",
		TradeDate:  "2026-08-22",
		Comparable: true,
		Report: &ProviderComparisonReport{
			Comparable: true,
			Symbols:    emptySymbolDiff(),
		},
	}))

	got := AnalyzeStore(store, "2026-08-21")
	require.Equal(t, 1, got.RecordCount)
	require.Equal(t, 1, got.ComparableCount)
	require.InDelta(t, 2.0, got.AvgLegacyStockCount, 1e-9)
}

func TestDailyReport_SourceHasNoWriteChainHooks(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("daily_report.go")
	require.NoError(t, err)
	text := string(src)
	require.NotContains(t, text, "CreatePlanWithItems")
	require.NotContains(t, text, "NewPortfolioDecisionProvider")
	require.NotContains(t, text, "provider_mode")
	require.Contains(t, text, "NotAProviderSwitch")
}
