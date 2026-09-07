package decisionprovider

import (
	"testing"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func portfolioCtx(ranked []selection.Candidate, limit int, snap *portfoliolayer.PortfolioSnapshot, cons portfoliolayer.ConstraintSet) DecisionContext {
	return DecisionContext{
		Selection: &selection.CandidateSelectionResult{
			RankedCandidates: ranked,
			SelectionLimit:   limit,
		},
		DecisionTime: time.Date(2026, 8, 20, 15, 30, 0, 0, time.FixedZone("CST", 8*3600)),
		Version: DecisionVersion{
			Contract:   ContractG21,
			Provider:   ProviderPortfolioAllocation,
			Allocation: AllocVersionF1Equal,
		},
		Snapshot:    snap,
		Constraints: cons,
	}
}

func g11Snap() *portfoliolayer.PortfolioSnapshot {
	return portfoliolayer.FromLedger(&portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 20, 15, 30, 0, 0, time.Local),
		AccountID:     1,
		Found:         true,
		TotalEquity:   1_000_000,
		Cash:          800_000,
		AvailableCash: 800_000,
		MarketValue:   200_000,
		TotalExposure: 200_000,
		PositionCount: 1,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", StockName: "Ping An", Volume: 100, MarketValue: 200_000, Weight: 0.20},
		},
	})
}

func g11Ranked(codes ...string) []selection.Candidate {
	out := make([]selection.Candidate, len(codes))
	for i, c := range codes {
		out[i] = selection.Candidate{StockCode: c, StockName: c, Rank: i + 1, Score: float64(100 - i), Reason: "research"}
	}
	return out
}

func TestPortfolioDecide_ImplementsDecisionProvider(t *testing.T) {
	var _ DecisionProvider = NewPortfolioDecisionProvider()
	require.Equal(t, ProviderPortfolioAllocation, NewPortfolioDecisionProvider().Name())
}

func TestPortfolioDecide_Deterministic(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002", "sz000003", "sz000004"), 2, g11Snap(), portfoliolayer.ConstraintSet{})
	a, err := p.Decide(ctx)
	require.NoError(t, err)
	b, err := p.Decide(ctx)
	require.NoError(t, err)
	require.True(t, a.OK)
	require.Equal(t, a.Provider, b.Provider)
	require.Equal(t, len(a.Lines), len(b.Lines))
	for i := range a.Lines {
		require.Equal(t, a.Lines[i].Symbol, b.Lines[i].Symbol)
		require.Equal(t, a.Lines[i].TargetAmount, b.Lines[i].TargetAmount)
		require.Equal(t, a.Lines[i].Reason, b.Lines[i].Reason)
		require.Equal(t, a.Lines[i].Metadata.InAllocationSet, b.Lines[i].Metadata.InAllocationSet)
	}
	require.Equal(t, a.Rejected, b.Rejected)
}

func TestPortfolioDecide_AmountsLegalAndCompleteSymbols(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002", "sz000003", "sz000004"), 2, g11Snap(), portfoliolayer.ConstraintSet{})
	env, err := p.Decide(ctx)
	require.NoError(t, err)
	require.True(t, env.OK)
	require.Len(t, env.Lines, 3)
	seen := map[string]struct{}{}
	selectedN := 0
	for _, line := range env.Lines {
		require.NotEmpty(t, line.Symbol)
		require.True(t, line.TargetAmount >= 0)
		require.True(t, isFinite(line.TargetAmount))
		if line.Metadata.InAllocationSet {
			require.Greater(t, line.TargetAmount, 0.0)
			selectedN++
		}
		_, dup := seen[line.Symbol]
		require.False(t, dup)
		seen[line.Symbol] = struct{}{}
		require.Equal(t, ProviderPortfolioAllocation, line.Metadata.SourceProvider)
		require.Equal(t, "research", line.Metadata.CandidateReason)
	}
	require.Equal(t, 2, selectedN)
	require.Equal(t, "sz000002", env.Lines[0].Symbol)
	require.Equal(t, "sz000003", env.Lines[1].Symbol)
	require.Equal(t, "sz000004", env.Lines[2].Symbol)
	require.True(t, env.Lines[0].Metadata.InAllocationSet)
	require.False(t, env.Lines[2].Metadata.InAllocationSet)
	require.Empty(t, env.Rejected)
}

func TestPortfolioDecide_RejectedNotInEnvelopeLines(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	skip := true
	ctx := portfolioCtx(g11Ranked("sz000001", "sz000002"), 5, g11Snap(), portfoliolayer.ConstraintSet{
		User: portfoliolayer.PreferenceLayer{SkipAlreadyHolding: &skip},
	})
	env, err := p.Decide(ctx)
	require.NoError(t, err)
	require.True(t, env.OK)
	require.Len(t, env.Rejected, 1)
	require.Equal(t, "sz000001", env.Rejected[0].Symbol)
	require.Equal(t, portfoliolayer.ReasonAlreadyHolding, env.Rejected[0].Reason)
	for _, line := range env.Lines {
		require.NotEqual(t, "sz000001", line.Symbol)
	}
	require.Equal(t, "sz000002", env.Lines[0].Symbol)
}

func TestPortfolioDecide_ZeroBudgetOnAllocationSetFailsClosed(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002", "sz000003"), 2, g11Snap(), portfoliolayer.ConstraintSet{})
	ctx.Budget = &portfoliolayer.AllocationBudget{AvailableCapital: 0}
	_, err := p.Decide(ctx)
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeZeroAmount, de.Code)
}

func TestPortfolioDecide_DoesNotFallbackToFixedAmount(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002"), 1, g11Snap(), portfoliolayer.ConstraintSet{})
	ctx.Budget = &portfoliolayer.AllocationBudget{AvailableCapital: 0}
	env, err := p.Decide(ctx)
	require.Error(t, err)
	require.False(t, env.OK)
	require.Empty(t, env.Lines)
	require.NotEqual(t, 100_000.0, env.Metadata.UniformAmount)
}

func TestPortfolioDecide_NoSnapshotWaitlistZeroIsNotFakeTenThousand(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002", "sz000003"), 2, portfoliolayer.FromLedger(nil), portfoliolayer.ConstraintSet{})
	env, err := p.Decide(ctx)
	require.NoError(t, err)
	require.True(t, env.OK)
	require.Len(t, env.Lines, 2)
	for _, line := range env.Lines {
		require.Equal(t, 0.0, line.TargetAmount)
		require.False(t, line.Metadata.InAllocationSet)
		require.NotEqual(t, 100_000.0, line.TargetAmount)
	}
	require.Empty(t, env.Rejected)
}

func TestPortfolioDecide_InjectedBudget(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002", "sz000003"), 2, g11Snap(), portfoliolayer.ConstraintSet{})
	ctx.Budget = &portfoliolayer.AllocationBudget{AvailableCapital: 50_000, Binding: "cash"}
	env, err := p.Decide(ctx)
	require.NoError(t, err)
	require.Equal(t, 25_000.0, env.Lines[0].TargetAmount)
	require.Equal(t, 25_000.0, env.Lines[1].TargetAmount)
	require.Equal(t, 25_000.0, env.Metadata.UniformAmount)
}

func TestPortfolioDecide_RejectsProviderMismatch(t *testing.T) {
	p := NewPortfolioDecisionProvider()
	ctx := portfolioCtx(g11Ranked("sz000002"), 1, g11Snap(), portfoliolayer.ConstraintSet{})
	ctx.Version.Provider = ProviderFixedAmount
	_, err := p.Decide(ctx)
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeContextInvalid, de.Code)
}
