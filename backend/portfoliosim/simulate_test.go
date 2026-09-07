package portfoliosim

import (
	"encoding/json"
	"testing"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/risk"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func i(v int) *int { return &v }

func testSnap() *portfoliolayer.PortfolioSnapshot {
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
			{StockCode: "sz000001", Volume: 100, MarketValue: 200_000, Weight: 0.20},
		},
	})
}

func candResult(limit int, codes ...string) *selection.CandidateSelectionResult {
	ranked := make([]selection.Candidate, len(codes))
	for i, c := range codes {
		ranked[i] = selection.Candidate{StockCode: c, StockName: c, Rank: i + 1, Score: float64(100 - i)}
	}
	return &selection.CandidateSelectionResult{
		RankedCandidates: ranked,
		SelectionLimit:   limit,
	}
}

func TestSimulate_NotATradePlanAndCoreFields(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(3, "sz000002", "sz000003", "sz000004", "sz000005"),
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85), MaxSingleNamePct: f64(0.20)},
		},
	})
	require.True(t, got.NotATradePlan)
	require.True(t, got.RecordOnly)
	require.NotEmpty(t, got.Selected)
	require.NotEmpty(t, got.Waitlist)
	require.NotNil(t, got.Allocation)
	require.NotNil(t, got.PortfolioRiskSnapshot)
	require.NotNil(t, got.RiskConstraintTrace)
	require.NotNil(t, got.DecisionEnvelope)
	require.NotNil(t, got.LegacyCompare)
	require.Equal(t, FilterStatusCalled, got.PotentialFilter.Status)
	require.True(t, got.PotentialFilter.Ran)
	require.True(t, got.Evaluation.QuantityAlwaysZero)
	for _, item := range got.Allocation.Items {
		require.Equal(t, int64(0), item.TargetQuantity)
	}
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	s := string(raw)
	require.NotContains(t, s, `"plan_id"`)
	require.NotContains(t, s, `"freeze_at"`)
	require.NotContains(t, s, `"enable_execute"`)
	require.Contains(t, s, `"not_a_trade_plan":true`)
}

func TestSimulate_DeterministicStableJSON(t *testing.T) {
	t.Parallel()
	in := Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003", "sz000004"),
		Constraints: portfoliolayer.ConstraintSet{
			User: portfoliolayer.PreferenceLayer{MaxNewNames: i(2)},
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85), MaxSingleNamePct: f64(0.20)},
		},
		Risk: &tradingconfig.RiskView{MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20, MarketLevel: 3},
		DecisionTime: time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC),
	}
	a := Simulate(in)
	b := Simulate(in)
	require.Equal(t, codes(a.Selected), codes(b.Selected))
	require.Equal(t, codes(a.Waitlist), codes(b.Waitlist))
	require.Equal(t, a.Allocation.UniformAmount, b.Allocation.UniformAmount)
	require.Equal(t, a.Budget.AvailableCapital, b.Budget.AvailableCapital)
	require.Equal(t, pendingCodes(a), pendingCodes(b))
	require.Equal(t, skippedCodes(a), skippedCodes(b))
	require.Equal(t, a.PotentialFilter.RiskStatus, b.PotentialFilter.RiskStatus)
	require.Equal(t, a.Evaluation, b.Evaluation)
	require.NotNil(t, a.PortfolioRiskSnapshot)
	require.Equal(t, a.PortfolioRiskSnapshot.InputsFingerprint, b.PortfolioRiskSnapshot.InputsFingerprint)
	require.NotNil(t, a.RiskConstraintTrace)
	require.Equal(t, a.RiskConstraintTrace.InputsFingerprint, b.RiskConstraintTrace.InputsFingerprint)
	ja, err := json.Marshal(stableSimView(a))
	require.NoError(t, err)
	jb, err := json.Marshal(stableSimView(b))
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
}

func TestSimulate_ComparableWithLegacy(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:            testSnap(),
		Candidates:          candResult(2, "sz000002", "sz000003", "sz000004"),
		LegacyAmountPerName: 100_000,
		Constraints: portfoliolayer.ConstraintSet{
			User: portfoliolayer.PreferenceLayer{MaxNewNames: i(2)},
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85), MaxSingleNamePct: f64(0.20)},
		},
		Risk: &tradingconfig.RiskView{MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20},
	})
	require.NotNil(t, got.LegacyCompare)
	require.Equal(t, "fixed_amount", got.LegacyCompare.Method)
	require.Equal(t, len(got.Selected), got.LegacyCompare.SelectedCount)
	require.InDelta(t, 100_000*float64(len(got.Selected)), got.LegacyCompare.SumAllocationSet, 1e-9)
	require.NotNil(t, got.Allocation)
	require.Equal(t, "equal_weight", got.Allocation.Method)
	// Both sides cover the same selected universe size for comparison.
	require.Equal(t, got.LegacyCompare.SelectedCount, got.Evaluation.SelectedCount)
	require.NotNil(t, got.DecisionEnvelope)
	require.Equal(t, decisionprovider.ProviderPortfolioAllocation, got.DecisionEnvelope.Provider)
}

func TestSimulate_FilterRejectReasonsPreserved(t *testing.T) {
	t.Parallel()
	// Low cash, roomy gross headroom → PlanFilter skips with CASH_INSUFFICIENT.
	snap := portfoliolayer.FromLedger(&portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 20, 15, 30, 0, 0, time.Local),
		Found:         true,
		TotalEquity:   100_000,
		Cash:          500,
		AvailableCash: 500,
		MarketValue:   0,
		TotalExposure: 0,
	})
	got := Simulate(Input{
		Snapshot:   snap,
		Candidates: candResult(2, "sz000002", "sz000003"),
		Budget:     &portfoliolayer.AllocationBudget{AvailableCapital: 50_000, Binding: "injected"},
		Constraints: portfoliolayer.ConstraintSet{
			User: portfoliolayer.PreferenceLayer{MaxNewNames: i(2)},
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85), MaxSingleNamePct: f64(0.50)},
		},
		SkipRiskTighten: true,
	})
	require.True(t, got.PotentialFilter.Ran)
	require.Equal(t, FilterStatusCalled, got.PotentialFilter.Status)
	require.NotEmpty(t, got.PotentialFilter.Skipped)
	foundCash := false
	for _, sk := range got.PotentialFilter.Skipped {
		require.NotEmpty(t, sk.RiskCode, "skipped line must retain risk_code")
		require.Equal(t, "skipped", sk.Status)
		if sk.RiskCode == string(risk.ReasonCashInsufficient) {
			foundCash = true
		}
	}
	require.True(t, foundCash, "expected CASH_INSUFFICIENT among filter skip reasons")

	// Market block with injected positive budget → Filter retains MARKET_LEVEL_BLOCKED.
	blocked := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003"),
		Budget:     &portfoliolayer.AllocationBudget{AvailableCapital: 100_000, Binding: "injected"},
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{MarketLevel: 1, BlockNewEntries: true, MaxGrossExposurePct: f64(0.85)},
		},
		SkipRiskTighten: true,
	})
	require.Equal(t, "blocked", blocked.PotentialFilter.RiskStatus)
	require.Empty(t, blocked.PotentialFilter.Pending)
	require.NotEmpty(t, blocked.PotentialFilter.Skipped)
	for _, sk := range blocked.PotentialFilter.Skipped {
		require.Equal(t, string(risk.ReasonMarketLevelBlocked), sk.RiskCode)
	}
}

func TestSimulate_NoRealTradingObjects(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003"),
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85)},
		},
		Risk: &tradingconfig.RiskView{MaxGrossExposurePct: 0.85},
	})
	require.True(t, got.NotATradePlan)
	require.True(t, got.RecordOnly)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	s := string(raw)
	require.Contains(t, s, `"not_a_trade_plan":true`)
	require.NotContains(t, s, `"plan_id"`)
	require.NotContains(t, s, `"freeze_at"`)
	require.NotContains(t, s, `"enable_execute"`)
	require.NotContains(t, s, `"order_id"`)
	require.NotContains(t, s, `"broker_order"`)
	require.NotContains(t, s, `"trade_plan_id"`)
	if got.Allocation != nil {
		for _, it := range got.Allocation.Items {
			require.Equal(t, int64(0), it.TargetQuantity)
		}
	}
	if got.DecisionEnvelope != nil {
		require.NotEqual(t, "trade_plan", got.DecisionEnvelope.Provider)
	}
}

func skippedCodes(r *SimulatedPortfolioDecisionResult) []string {
	out := make([]string, 0, len(r.PotentialFilter.Skipped))
	for _, l := range r.PotentialFilter.Skipped {
		out = append(out, l.StockCode+":"+l.RiskCode)
	}
	return out
}

type stableSimPayload struct {
	Selected   []string
	Waitlist   []string
	Uniform    float64
	Capital    float64
	Pending    []string
	Skipped    []string
	RiskStatus string
	Eval       SimulatedDecisionMetrics
	RiskFP     string
	TraceFP    string
}

func stableSimView(r *SimulatedPortfolioDecisionResult) stableSimPayload {
	out := stableSimPayload{
		Selected: codes(r.Selected), Waitlist: codes(r.Waitlist),
		Pending: pendingCodes(r), Skipped: skippedCodes(r),
		RiskStatus: r.PotentialFilter.RiskStatus, Eval: r.Evaluation,
	}
	if r.Allocation != nil {
		out.Uniform = r.Allocation.UniformAmount
		out.Capital = r.Budget.AvailableCapital
	}
	if r.PortfolioRiskSnapshot != nil {
		out.RiskFP = r.PortfolioRiskSnapshot.InputsFingerprint
	}
	if r.RiskConstraintTrace != nil {
		out.TraceFP = r.RiskConstraintTrace.InputsFingerprint
	}
	return out
}


func TestSimulate_InjectedBudget(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003"),
		Constraints: portfoliolayer.ConstraintSet{
			User: portfoliolayer.PreferenceLayer{MaxNewNames: i(2)},
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85)},
		},
		Budget: &portfoliolayer.AllocationBudget{AvailableCapital: 200_000, Binding: "injected"},
	})
	require.Equal(t, "injected", got.Budget.Binding)
	require.InDelta(t, 100_000, got.Allocation.UniformAmount, 1e-6)
}

func TestSimulate_SkipFilter(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003"),
		Filter:     FilterCompatRequest{SkipCompatibilityCheck: true},
	})
	require.False(t, got.PotentialFilter.Ran)
	require.Equal(t, FilterStatusNotRun, got.PotentialFilter.Status)
	require.NotEmpty(t, got.Selected)
}

func TestSimulate_PotentialFilterMarketBlock(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot:   testSnap(),
		Candidates: candResult(2, "sz000002", "sz000003"),
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{
				MarketLevel:         1,
				BlockNewEntries:     true,
				MaxGrossExposurePct: f64(0.85),
			},
		},
	})
	require.True(t, got.PotentialFilter.Ran)
	require.Equal(t, "blocked", got.PotentialFilter.RiskStatus)
	require.Empty(t, got.PotentialFilter.Pending)
	require.NotEmpty(t, got.PotentialFilter.Skipped)
	require.Equal(t, 0, got.Evaluation.PotentialPendingCount)
}

func TestSimulate_DoesNotMutateSnapshot(t *testing.T) {
	t.Parallel()
	snap := testSnap()
	cash := snap.Cash
	_ = Simulate(Input{
		Snapshot:   snap,
		Candidates: candResult(1, "sz000002"),
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85)},
		},
	})
	require.Equal(t, cash, snap.Cash)
}

func TestSimulate_FallbackSelect(t *testing.T) {
	t.Parallel()
	got := Simulate(Input{
		Snapshot: testSnap(),
		RankedFallback: []selection.Candidate{
			{StockCode: "sz000009", Rank: 2, Score: 1},
			{StockCode: "sz000008", Rank: 1, Score: 9},
		},
		SelectionCtx: selection.SelectionContext{MaxSelectedNames: 1},
		Constraints: portfoliolayer.ConstraintSet{
			Risk: portfoliolayer.RiskLayer{MarketLevel: 3, MaxGrossExposurePct: f64(0.85)},
		},
	})
	require.Equal(t, "sz000008", got.Selection.RankedCandidates[0].StockCode)
	require.Equal(t, "sz000008", got.Selected[0].Candidate.StockCode)
}

func codes(picks []portfoliolayer.PortfolioPick) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Candidate.StockCode
	}
	return out
}

func pendingCodes(r *SimulatedPortfolioDecisionResult) []string {
	out := make([]string, 0, len(r.PotentialFilter.Pending))
	for _, l := range r.PotentialFilter.Pending {
		out = append(out, l.StockCode)
	}
	return out
}

func f64(v float64) *float64 { return &v }
