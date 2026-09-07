package providershadow

import (
	"testing"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func g12Time() time.Time {
	return time.Date(2026, 8, 20, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))
}

func g12Sel(codes ...string) *selection.CandidateSelectionResult {
	ranked := make([]selection.Candidate, len(codes))
	for i, c := range codes {
		ranked[i] = selection.Candidate{StockCode: c, StockName: c, Rank: i + 1, Score: float64(100 - i), Reason: "research"}
	}
	return &selection.CandidateSelectionResult{RankedCandidates: ranked, SelectionLimit: 2}
}

func g12Ctx(sel *selection.CandidateSelectionResult, amount float64, snap *portfoliolayer.PortfolioSnapshot) decisionprovider.DecisionContext {
	return decisionprovider.DecisionContext{
		Selection:     sel,
		DecisionTime:  g12Time(),
		UniformAmount: amount,
		Snapshot:      snap,
		Version: decisionprovider.DecisionVersion{
			Contract:   decisionprovider.ContractG21,
			Selection:  SelectionVersionE61,
			Constraint: ConstraintVersionF21,
			Allocation: decisionprovider.AllocVersionF1Equal,
		},
	}
}

func TestCompareProviders_DisabledSkipsPortfolioAndReport(t *testing.T) {
	legacy := decisionprovider.NewLegacyDecisionProvider(nil)
	port := &countingProvider{inner: decisionprovider.NewPortfolioDecisionProvider()}
	sel := g12Sel("sz000002", "sz000003", "sz000004")
	got := CompareProviders(g12Ctx(sel, 100_000, nil), false, legacy, port)
	require.NotNil(t, got.ChainEnvelope)
	require.True(t, got.ChainEnvelope.OK)
	require.Equal(t, decisionprovider.ProviderFixedAmount, got.ChainEnvelope.Provider)
	require.Nil(t, got.Report)
	require.Equal(t, 0, port.calls)
}

func TestDiffEnvelopes_PortfolioFailedDoesNotExpandFakeEmptyBuys(t *testing.T) {
	legacy := decisionprovider.NewLegacyDecisionProvider(nil)
	sel := g12Sel("sz000002", "sz000003")
	env, err := legacy.Decide(g12Ctx(sel, 100_000, nil))
	require.NoError(t, err)
	fail := &decisionprovider.DecisionEnvelope{
		OK:       false,
		Provider: decisionprovider.ProviderPortfolioAllocation,
		Error:    &decisionprovider.DecisionError{Code: decisionprovider.ErrCodeZeroAmount, Message: "allocation set has no positive target_amount"},
		Lines:    []decisionprovider.DecisionLine{},
	}
	report := DiffEnvelopes(env, fail, g12Time(), sel.RankedCandidates)
	require.False(t, report.Comparable)
	require.Equal(t, IncomparablePortfolioFailed, report.IncomparableReason)
	require.Empty(t, report.Symbols.OnlyLegacy)
	require.Empty(t, report.Amounts)
	require.Empty(t, report.Roles)
	require.Equal(t, IncomparablePortfolioFailed, report.Metrics.SkippedBecause)
	require.Equal(t, decisionprovider.ProviderFixedAmount, report.ChainProvider)
}

func TestDiffEnvelopes_CommonAndAmountPresence(t *testing.T) {
	legacy := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderFixedAmount,
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 100_000, Reason: "research", Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "fixed_amount", CandidateReason: "research"}},
			{Symbol: "sz000002", TargetAmount: 100_000, Reason: "research", Metadata: decisionprovider.LineMeta{InAllocationSet: false, AllocationReason: "fixed_amount", CandidateReason: "research"}},
		},
	}
	port := &decisionprovider.DecisionEnvelope{
		OK: true, Provider: decisionprovider.ProviderPortfolioAllocation,
		Lines: []decisionprovider.DecisionLine{
			{Symbol: "sz000001", TargetAmount: 80_000, Reason: "equal_split", Metadata: decisionprovider.LineMeta{InAllocationSet: true, AllocationReason: "equal_split", CandidateReason: "research"}},
		},
		Rejected: []decisionprovider.RejectedLine{{Symbol: "sz000002", Reason: "already_holding"}},
	}
	report := DiffEnvelopes(legacy, port, g12Time(), []selection.Candidate{
		{StockCode: "sz000001", Rank: 1},
		{StockCode: "sz000002", Rank: 2},
	})
	require.True(t, report.Comparable)
	require.Equal(t, []string{"sz000002"}, report.Symbols.OnlyLegacy)
	require.Equal(t, []string{"sz000001"}, report.Symbols.Common)
	require.Empty(t, report.Symbols.OnlyPortfolio)
	require.GreaterOrEqual(t, report.Metrics.AmountDiffCount, 1)
	require.GreaterOrEqual(t, report.Metrics.RoleDiffCount, 1)
}

type countingProvider struct {
	inner decisionprovider.DecisionProvider
	calls int
}

func (p *countingProvider) Name() string { return p.inner.Name() }
func (p *countingProvider) Decide(ctx decisionprovider.DecisionContext) (*decisionprovider.DecisionEnvelope, error) {
	p.calls++
	return p.inner.Decide(ctx)
}
