package providershadow

import (
	"testing"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestAllocationShadowRuntime_DefaultOff(t *testing.T) {
	rt := NewAllocationShadowRuntime(Config{})
	require.False(t, DefaultEnabled)
	require.False(t, rt.Enabled())
	rec, err := rt.Run(g12Ctx(g12Sel("sz000001"), 100_000, nil), "2026-08-21")
	require.NoError(t, err)
	require.Nil(t, rec)
}

func TestAllocationShadowRuntime_DualProvider_RecordFields(t *testing.T) {
	store := NewMemoryStore()
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 400_000, AvailableCash: 400_000,
		TotalExposure: 200_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000099", Volume: 1, MarketValue: 200_000, Weight: 0.20},
		},
	}
	risk := &tradingconfig.RiskView{MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20, MarketLevel: 3}
	rt := NewAllocationShadowRuntime(Config{
		Enabled: true,
		Store:   store,
		Now:     g12Time,
		Risk:    risk,
	})
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	ctx := g12Ctx(sel, 100_000, portfoliolayer.FromLedger(ledger))
	ctx.Constraints = portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.85),
			MaxSingleNamePct:    floatPtr(0.20),
		},
	}

	rec, err := rt.RunPrepared(ctx, "2026-08-21", PrepareOptions{Risk: risk, Ledger: ledger})
	require.NoError(t, err)
	require.NotNil(t, rec)

	// versions
	require.Equal(t, decisionprovider.AllocVersionF1Equal, rec.AllocationVersion)
	require.Equal(t, ComparatorVersionH31, rec.ComparatorVersion)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.LegacyProviderVersion)
	require.Equal(t, decisionprovider.ProviderPortfolioAllocation, rec.PortfolioProviderVersion)

	// H3.1 record summaries
	require.NotNil(t, rec.RiskConstraintTrace)
	require.True(t, rec.RiskConstraintTrace.RecordOnly)
	require.True(t, rec.RiskConstraintTrace.NotLegacyWriteChain)
	require.NotNil(t, rec.RiskConstraintTrace.AllocationEngine)
	require.NotNil(t, rec.AllocationBudgetSummary)
	require.False(t, rec.AllocationBudgetSummary.LegacyHasBudget)
	require.NotNil(t, rec.ReserveSummary)
	require.Equal(t, 0.0, rec.ReserveSummary.LegacyReserve)
	require.NotNil(t, rec.RiskCutSummary)
	require.True(t, rec.RiskCutSummary.LegacySizerIgnoresCaps)

	require.NotNil(t, rec.Report)
	require.True(t, rec.Report.RecordOnly)
	require.True(t, rec.Report.NotATradePlan)
	require.True(t, rec.Report.NotAProviderSwitch)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.Report.ChainProvider)

	require.True(t, rec.Comparable, "portfolio should succeed with ample cash headroom")
	require.NotNil(t, rec.Report.Allocation)
	require.True(t, rec.Report.Allocation.NotAProviderSwitch)
	require.NotNil(t, rec.Report.RiskAdjustment)
	require.NotEmpty(t, rec.Report.Symbols.Common)

	require.Len(t, store.List(), 1)
}

func TestAllocationShadowRuntime_PortfolioFail_LegacyChainIntact(t *testing.T) {
	store := NewMemoryStore()
	rt := NewAllocationShadowRuntime(Config{
		Enabled:   true,
		Store:     store,
		Now:       g12Time,
		Portfolio: failPortfolio{code: decisionprovider.ErrCodeZeroAmount, msg: "alloc fail"},
	})
	sel := g12Sel("sz000002", "sz000003")
	ctx := g12Ctx(sel, 100_000, portfoliolayer.FromLedger(nil))

	// Direct compare: chain stays Legacy even when Portfolio fails.
	cmp := CompareProviders(ctx, true, decisionprovider.NewLegacyDecisionProvider(nil), failPortfolio{
		code: decisionprovider.ErrCodeZeroAmount, msg: "alloc fail",
	})
	require.NotNil(t, cmp.ChainEnvelope)
	require.True(t, cmp.ChainEnvelope.OK)
	require.Equal(t, decisionprovider.ProviderFixedAmount, cmp.ChainEnvelope.Provider)
	require.NotNil(t, cmp.PortfolioEnvelope)
	require.False(t, cmp.PortfolioEnvelope.OK)
	require.NotNil(t, cmp.Report)
	require.False(t, cmp.Report.Comparable)
	require.Equal(t, IncomparablePortfolioFailed, cmp.Report.IncomparableReason)
	require.True(t, cmp.Report.NotAProviderSwitch)
	require.True(t, cmp.Report.RecordOnly)
	require.True(t, cmp.Report.NotATradePlan)

	rec, err := rt.Run(ctx, "2026-08-21")
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.True(t, rec.Failure.PortfolioFailed)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.LegacySummary.Provider)
}

func TestAllocationShadowRuntime_ObserveWrongTriggerNoop(t *testing.T) {
	rt := NewAllocationShadowRuntime(Config{Enabled: true, Store: NewMemoryStore(), Now: g12Time})
	got := rt.Observe(ObserveInput{
		Trigger:       "morning_freeze",
		Selection:     g12Sel("sz000001"),
		UniformAmount: 100_000,
		DecisionTime:  g12Time(),
	})
	require.Nil(t, got)
}

func TestBuildAllocationShadowDailyReport_Flags(t *testing.T) {
	// daily report remains observe-only (no provider switch)
	got := BuildPortfolioShadowDailyReport(nil, "2026-08-21")
	require.True(t, got.RecordOnly)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotAProviderSwitch)
}
