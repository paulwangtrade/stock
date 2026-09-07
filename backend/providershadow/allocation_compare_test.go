package providershadow

import (
	"testing"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func TestBuildAllocationCompare_LegacyFixedVsPortfolioBudget(t *testing.T) {
	legacy := &decisionprovider.DecisionEnvelope{
		OK:       true,
		Provider: decisionprovider.ProviderFixedAmount,
		Metadata: decisionprovider.EnvelopeMeta{UniformAmount: 100_000},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "fixed_amount"}},
			{Symbol: "sz000002", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "fixed_amount"}},
			{Symbol: "sz000003", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: false, AllocationReason: "fixed_amount"}},
		},
	}
	port := &decisionprovider.DecisionEnvelope{
		OK:       true,
		Provider: decisionprovider.ProviderPortfolioAllocation,
		Metadata: decisionprovider.EnvelopeMeta{
			UniformAmount: 40_000,
			Budget: portfoliolayer.AllocationBudget{
				AvailableCash: 100_000, ReserveCash: 20_000, AvailableCapital: 80_000, Binding: "cash",
			},
		},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 40_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "equal_split"}},
			{Symbol: "sz000002", TargetAmount: 40_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "equal_split"}},
			{Symbol: "sz000003", TargetAmount: 40_000, Metadata: decisionprovider.LineMeta{InAllocationSet: false, AllocationReason: "equal_split"}},
		},
	}

	amountDiffs, _ := diffAmounts(legacy, port, []string{}, []string{}, []string{"sz000001", "sz000002", "sz000003"})
	got := BuildAllocationCompare(legacy, port, amountDiffs)
	require.NotNil(t, got)
	require.True(t, got.Comparable)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotAProviderSwitch)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.ChainProvider)

	require.False(t, got.BudgetDiff.LegacyHasBudget)
	require.Equal(t, "legacy_no_generation_budget", got.BudgetDiff.Note)
	require.Equal(t, 200_000.0, got.BudgetDiff.ImpliedLegacyNotional)
	require.Equal(t, 80_000.0, got.BudgetDiff.Portfolio.AvailableCapital)
	require.InDelta(t, -120_000.0, got.BudgetDiff.CapitalVsImpliedDelta, 1e-9)

	require.Equal(t, 0.0, got.ReserveDiff.LegacyReserve)
	require.Equal(t, 20_000.0, got.ReserveDiff.PortfolioReserve)
	require.Equal(t, 20_000.0, got.ReserveDiff.Delta)

	require.Equal(t, 100_000.0, got.UniformDiff.LegacyScalar)
	require.Equal(t, 40_000.0, got.UniformDiff.PortfolioUniform)
	require.InDelta(t, -60_000.0, got.UniformDiff.Delta, 1e-9)

	require.Equal(t, "copy_uniform", got.WaitlistAmountDiff.Mode)
	require.Equal(t, 1, got.WaitlistAmountDiff.LegacyWaitlistCount)
	require.Equal(t, 1, got.WaitlistAmountDiff.PortfolioWaitlistCount)
	require.Equal(t, 100_000.0, got.WaitlistAmountDiff.LegacyWaitlistAmount)
	require.Equal(t, 40_000.0, got.WaitlistAmountDiff.PortfolioWaitlistAmount)

	require.True(t, got.RiskCutDiff.LegacySizerIgnoresCaps)
	require.Equal(t, "cash", got.RiskCutDiff.PortfolioBinding)
	require.NotEmpty(t, got.AmountDiffs)
}

func TestBuildAllocationCompare_RiskCutAndAbsentNotZero(t *testing.T) {
	legacy := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderFixedAmount,
		Metadata: decisionprovider.EnvelopeMeta{UniformAmount: 100_000},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "fixed_amount"}},
			{Symbol: "sz000002", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "fixed_amount"}},
		},
	}
	port := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderPortfolioAllocation,
		Metadata: decisionprovider.EnvelopeMeta{
			UniformAmount: 30_000,
			Budget: portfoliolayer.AllocationBudget{
				AvailableCapital: 50_000, Binding: "gross", PolicyGrossPct: 0.5,
			},
		},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 30_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "capped_single_weight"}},
			{Symbol: "sz000002", TargetAmount: 0, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "below_min_order"}},
		},
	}
	report := DiffEnvelopes(legacy, port, g12Time(), nil)
	require.True(t, report.Comparable)
	require.NotNil(t, report.Allocation)
	require.True(t, report.Allocation.RiskCutDiff.SingleCapApplied)
	require.Equal(t, 1, report.Allocation.RiskCutDiff.MinOrderZeroCount)
	require.True(t, report.Allocation.RiskCutDiff.GrossHeadroomBinding)

	// absent ≠ 0: only_legacy amount presence stays absent on portfolio side when missing
	legacyOnly := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderFixedAmount,
		Metadata: decisionprovider.EnvelopeMeta{UniformAmount: 100_000},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true}},
			{Symbol: "sz000099", TargetAmount: 100_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true}},
		},
	}
	portCommon := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderPortfolioAllocation,
		Metadata: decisionprovider.EnvelopeMeta{UniformAmount: 50_000, Budget: portfoliolayer.AllocationBudget{AvailableCapital: 50_000, Binding: "cash"}},
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 50_000, Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "equal_split"}},
		},
	}
	r2 := DiffEnvelopes(legacyOnly, portCommon, g12Time(), []selection.Candidate{{StockCode: "sz000099"}})
	require.NotNil(t, r2.Allocation)
	found := false
	for _, d := range r2.Allocation.AmountDiffs {
		if d.Symbol == "sz000099" {
			found = true
			require.Equal(t, PresenceActual, d.Legacy.Presence)
			require.Equal(t, PresenceAbsent, d.Portfolio.Presence)
			require.Nil(t, d.Delta)
			require.Equal(t, KindAbsentVsActual, d.Kind)
		}
	}
	require.True(t, found)
}

func TestDiffEnvelopes_AllocationObserveOnlyNoChainSwitch(t *testing.T) {
	legacy := decisionprovider.NewLegacyDecisionProvider(nil)
	port := decisionprovider.NewPortfolioDecisionProvider()
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	snap := &portfoliolayer.PortfolioSnapshot{
		Found: true, Cash: 200_000, AvailableCash: 200_000, Equity: 200_000,
	}
	got := CompareProviders(g12Ctx(sel, 100_000, snap), true, legacy, port)
	require.NotNil(t, got.ChainEnvelope)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.ChainEnvelope.Provider)
	require.NotNil(t, got.Report)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.Report.ChainProvider)
	require.NotNil(t, got.Report.Allocation)
	require.True(t, got.Report.Allocation.NotAProviderSwitch)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.Report.Allocation.ChainProvider)
	// Allocation attachment must not rewrite chain lines.
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.ChainEnvelope.Provider)
}
