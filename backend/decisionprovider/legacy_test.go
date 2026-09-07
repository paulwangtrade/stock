package decisionprovider

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/positionsizing"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func testCtx(ranked []selection.Candidate, limit int) DecisionContext {
	return DecisionContext{
		Selection: &selection.CandidateSelectionResult{
			RankedCandidates: ranked,
			SelectionLimit:   limit,
		},
		DecisionTime: time.Date(2026, 8, 20, 15, 30, 0, 0, time.FixedZone("CST", 8*3600)),
		Version:      DecisionVersion{Contract: ContractG21, Provider: ProviderFixedAmount},
	}
}

func TestLegacyDecide_UsesFixedAmountSizer(t *testing.T) {
	p := NewLegacyDecisionProvider(&positionsizing.FixedAmountSizer{})
	env, err := p.Decide(testCtx([]selection.Candidate{
		{StockCode: "sz000002", StockName: "B", Rank: 1, Score: 80, Reason: "r1"},
		{StockCode: "sz000001", StockName: "A", Rank: 2, Score: 70, Reason: "r2"},
	}, 1))
	require.NoError(t, err)
	require.True(t, env.OK)
	require.Equal(t, ProviderFixedAmount, env.Provider)
	require.Equal(t, p.Name(), env.Provider)
	require.Len(t, env.Lines, 2)
	want := (&positionsizing.FixedAmountSizer{}).Propose(positionsizing.Request{}).PlannedAmount
	require.Greater(t, want, 0.0)
	require.Equal(t, want, env.Metadata.UniformAmount)
	require.Equal(t, want, env.Lines[0].TargetAmount)
	require.Equal(t, want, env.Lines[1].TargetAmount)
	require.Equal(t, "sz000002", env.Lines[0].Symbol)
	require.Equal(t, "sz000001", env.Lines[1].Symbol)
	require.Equal(t, "r1", env.Lines[0].Reason)
	require.True(t, env.Lines[0].Metadata.InAllocationSet)
	require.False(t, env.Lines[1].Metadata.InAllocationSet)
	require.Equal(t, AllocReasonFixedAmount, env.Lines[0].Metadata.AllocationReason)
}

func TestLegacyDecide_UniformAmountOverrideSkipsSizer(t *testing.T) {
	p := NewLegacyDecisionProvider(&positionsizing.FixedAmountSizer{})
	ctx := testCtx([]selection.Candidate{
		{StockCode: "SH600000", Rank: 1, Reason: "pool"},
	}, 5)
	ctx.UniformAmount = 55_000
	env, err := p.Decide(ctx)
	require.NoError(t, err)
	require.Equal(t, 55_000.0, env.Lines[0].TargetAmount)
	require.Equal(t, "sh600000", env.Lines[0].Symbol)
	require.Equal(t, "pool", env.Lines[0].Reason)
}

func TestLegacyDecide_EmptyRankedOK(t *testing.T) {
	p := NewLegacyDecisionProvider(nil)
	env, err := p.Decide(testCtx(nil, 5))
	require.NoError(t, err)
	require.True(t, env.OK)
	require.Empty(t, env.Lines)
}

func TestLegacyDecide_RejectsZeroSizerAmount(t *testing.T) {
	p := NewLegacyDecisionProvider(stubSizer{amount: 0, method: positionsizing.MethodFixedAmount})
	_, err := p.Decide(testCtx([]selection.Candidate{{StockCode: "sz000001", Rank: 1}}, 1))
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeZeroAmount, de.Code)
}

func TestLegacyDecide_RejectsNonFixedMethod(t *testing.T) {
	p := NewLegacyDecisionProvider(stubSizer{amount: 100_000, method: positionsizing.MethodPortfolioAware})
	_, err := p.Decide(testCtx([]selection.Candidate{{StockCode: "sz000001", Rank: 1}}, 1))
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeInvalidAllocation, de.Code)
}

func TestLegacyDecide_RejectsNaNOverride(t *testing.T) {
	p := NewLegacyDecisionProvider(nil)
	ctx := testCtx([]selection.Candidate{{StockCode: "sz000001", Rank: 1}}, 1)
	ctx.UniformAmount = math.NaN()
	_, err := p.Decide(ctx)
	require.Error(t, err)
}

func TestLegacyDecide_MissingSymbol(t *testing.T) {
	p := NewLegacyDecisionProvider(nil)
	ctx := testCtx([]selection.Candidate{{StockCode: "  ", Rank: 1}}, 1)
	ctx.UniformAmount = 100_000
	_, err := p.Decide(ctx)
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeMissingSymbol, de.Code)
}

func TestLegacyDecide_ProviderMismatch(t *testing.T) {
	p := NewLegacyDecisionProvider(nil)
	ctx := testCtx([]selection.Candidate{{StockCode: "sz000001", Rank: 1}}, 1)
	ctx.Version.Provider = "portfolio_allocation"
	ctx.UniformAmount = 1
	_, err := p.Decide(ctx)
	require.Error(t, err)
	var de *DecisionError
	require.ErrorAs(t, err, &de)
	require.Equal(t, ErrCodeContextInvalid, de.Code)
}

func TestLegacyName_IsFixedAmount(t *testing.T) {
	require.Equal(t, tradingconfig.SizingMethodFixedAmount, ProviderFixedAmount)
	require.Equal(t, ProviderFixedAmount, NewLegacyDecisionProvider(nil).Name())
}

type stubSizer struct {
	amount float64
	method positionsizing.Method
}

func (s stubSizer) Propose(positionsizing.Request) positionsizing.SizingProposal {
	return positionsizing.SizingProposal{PlannedAmount: s.amount, Method: s.method}
}
