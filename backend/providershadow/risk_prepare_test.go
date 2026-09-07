package providershadow

import (
	"testing"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestCompareProviders_DualProviderSameContext_RiskTrace(t *testing.T) {
	legacy := decisionprovider.NewLegacyDecisionProvider(nil)
	port := decisionprovider.NewPortfolioDecisionProvider()
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 80_000, AvailableCash: 80_000,
		TotalExposure: 920_000, PositionCount: 1,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1, MarketValue: 920_000, Weight: 0.92},
		},
	}
	snap := portfoliolayer.FromLedger(ledger)
	ctx := g12Ctx(sel, 100_000, snap)
	ctx.Constraints = portfoliolayer.ConstraintSet{
		Risk: portfoliolayer.RiskLayer{
			MaxGrossExposurePct: floatPtr(0.85),
			MaxSingleNamePct:    floatPtr(0.20),
		},
	}
	risk := &tradingconfig.RiskView{
		MarketLevel: 3, MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20,
	}
	riskBefore := *risk

	got := CompareProvidersPrepared(ctx, true, legacy, port, PrepareOptions{Risk: risk, Ledger: ledger})
	require.NotNil(t, got.ChainEnvelope)
	require.NotNil(t, got.PortfolioEnvelope)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.ChainEnvelope.Provider)
	require.Equal(t, decisionprovider.ProviderPortfolioAllocation, got.PortfolioEnvelope.Provider)
	require.True(t, got.ChainEnvelope.OK)
	require.NotNil(t, got.Report)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.Report.ChainProvider)
	require.True(t, got.Report.NotATradePlan)

	require.NotNil(t, got.RiskAdjustment)
	require.True(t, got.RiskAdjustment.RecordOnly)
	require.True(t, got.RiskAdjustment.NotATradePlan)
	require.True(t, got.RiskAdjustment.NotLegacyWriteChain)
	require.True(t, got.RiskAdjustment.NotRiskViewWrite)
	require.True(t, got.RiskAdjustment.NotFilterWrite)
	require.True(t, got.RiskAdjustment.NotExecutionWrite)
	require.NotEmpty(t, got.RiskAdjustment.InputsFingerprint)
	require.NotNil(t, got.RiskAdjustment.AllocationEngine)
	require.Equal(t, "equal_weight", got.RiskAdjustment.AllocationEngine.Method)
	require.Equal(t, riskBefore, *risk, "RiskView must remain unchanged")

	// Legacy write-chain identity: chain envelope stays fixed_amount amounts.
	for _, ln := range got.ChainEnvelope.Lines {
		if ln.Metadata.InAllocationSet {
			require.InDelta(t, 100_000.0, ln.TargetAmount, 1e-9)
		}
	}
}

func TestRuntime_RiskAdjustmentOnRecord_NoTradePlan(t *testing.T) {
	store := NewMemoryStore()
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 500_000, Cash: 200_000, AvailableCash: 200_000,
	}
	risk := &tradingconfig.RiskView{MaxGrossExposurePct: 0.90, MaxSingleNamePct: 0.25}
	rt := NewRuntime(Config{
		Enabled: true,
		Store:   store,
		Now:     g12Time,
		Risk:    risk,
	})
	sel := g12Sel("sz000002", "sz000003")
	ctx := g12Ctx(sel, 100_000, portfoliolayer.FromLedger(ledger))
	rec, err := rt.RunPrepared(ctx, "2026-08-21", PrepareOptions{Risk: risk, Ledger: ledger})
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.NotNil(t, rec.RiskAdjustment)
	require.NotNil(t, rec.Report)
	require.NotNil(t, rec.Report.RiskAdjustment)
	require.True(t, rec.Report.NotATradePlan)
	require.Equal(t, decisionprovider.ProviderFixedAmount, rec.Report.ChainProvider)
	require.NotNil(t, rec.RiskAdjustment.AllocationEngine)
}

func TestPreparePortfolioSide_DoesNotMutateBaseConstraints(t *testing.T) {
	sel := g12Sel("sz000002", "sz000003")
	ledger := &portfolio.Snapshot{
		Found: true, TotalEquity: 1_000_000, Cash: 50_000, AvailableCash: 50_000,
		TotalExposure: 950_000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 1, MarketValue: 950_000, Weight: 0.95},
		},
	}
	baseReserve := 0.05
	ctx := g12Ctx(sel, 100_000, portfoliolayer.FromLedger(ledger))
	ctx.Constraints = portfoliolayer.ConstraintSet{
		Portfolio: portfoliolayer.PreferenceLayer{ReserveCashRatio: &baseReserve},
		Risk:      portfoliolayer.RiskLayer{MaxGrossExposurePct: floatPtr(0.85)},
	}
	before := ctx.Constraints
	prep := PreparePortfolioSide(ctx, PrepareOptions{
		Risk:   &tradingconfig.RiskView{MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20},
		Ledger: ledger,
	})
	require.Equal(t, before.Portfolio.ReserveCashRatio, ctx.Constraints.Portfolio.ReserveCashRatio)
	require.InDelta(t, 0.05, *ctx.Constraints.Portfolio.ReserveCashRatio, 1e-9)
	require.NotNil(t, prep.Trace)
	require.NotNil(t, prep.RiskSnapshot)
	// Effective may tighten; base ctx.Constraints must stay as before.
	_ = prep.EffectiveConstraints
}

func floatPtr(v float64) *float64 { return &v }
