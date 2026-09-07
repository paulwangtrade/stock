package strategy

import (
	"errors"
	"fmt"
	"sync"

	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/positionsizing"
	"go-stock/backend/tradingconfig"
)

// ErrNoBudget means portfolio-aware sizing produced a non-positive amount.
// Draft Builder must return this without calling FilterPool (which would
// rewrite amount<=0 into 100000).
var ErrNoBudget = errors.New("no_budget")

var (
	draftSnapMu   sync.Mutex
	draftSnapStub draftSnapshotStub
)

type draftSnapshotStub struct {
	active bool
	snap   *portfolio.Snapshot
	err    error
}

// SetDraftPortfolioSnapshotForTest injects a Snapshot so tests do not hit paper_sim.
func SetDraftPortfolioSnapshotForTest(snap *portfolio.Snapshot, err error) {
	draftSnapMu.Lock()
	defer draftSnapMu.Unlock()
	draftSnapStub = draftSnapshotStub{active: true, snap: snap, err: err}
}

// ResetDraftPortfolioSnapshotForTest clears the test Snapshot stub.
func ResetDraftPortfolioSnapshotForTest() {
	draftSnapMu.Lock()
	defer draftSnapMu.Unlock()
	draftSnapStub = draftSnapshotStub{}
}

// resolvePlanAmountViaSizer returns the Trade Plan planned_amount via PositionSizer.
// Phase6.5-D MVP / default mode: FixedAmountSizer → TradingConfig Provider → legacy_paper_open_buy
// (result identical to former OpenBuyAmountPerStock / 100000 default).
func resolvePlanAmountViaSizer() float64 {
	tradingconfig.LogInitialized()
	proposal := positionsizing.ProposeDefault(positionsizing.Request{})
	positionsizing.LogApplied(proposal, true)
	return proposal.PlannedAmount
}

// resolvePlanAmountViaSizerForDraft selects fixed_amount vs portfolio_aware.
// portfolio_aware: Builder loads Snapshot and injects it — sizer does not query DB.
// Returns ErrNoBudget when the computed amount is <=0 so callers skip FilterPool.
func resolvePlanAmountViaSizerForDraft(pool *models.CandidatePool) (float64, error) {
	tradingconfig.LogInitialized()
	mode := tradingconfig.Default().PositionSizerMode()
	if !tradingconfig.IsPortfolioAwareSizerMode(mode) {
		return resolvePlanAmountViaSizer(), nil
	}

	n := 0
	if pool != nil {
		n = len(pool.Items)
		if n > defaultMaxPlanNames {
			n = defaultMaxPlanNames
		}
	}

	snap, err := loadDraftPortfolioSnapshot()
	if err != nil {
		return 0, fmt.Errorf("%w: snapshot: %v", ErrNoBudget, err)
	}

	pa := tradingconfig.Default().Resolve().PaperMVP.PortfolioAware
	pct := pa.MaxExposure
	if pct <= 0 {
		pct = tradingconfig.Default().Risk().MaxGrossExposurePct
	}

	proposal := positionsizing.ForMode(mode).Propose(positionsizing.Request{
		Portfolio:               snap,
		CandidateCount:          n,
		MaxGrossExposurePct:     pct,
		MaxSinglePositionWeight: pa.MaxSinglePositionWeight,
		ReserveCashRatio:        pa.ReserveCashRatio,
		MinOrderAmount:          pa.MinOrderAmount,
	})
	positionsizing.LogApplied(proposal, true)
	if proposal.PlannedAmount <= 0 {
		return 0, fmt.Errorf("%w: insufficient_allocation planned_amount<=0 method=%s candidates=%d", ErrNoBudget, proposal.Method, n)
	}
	return proposal.PlannedAmount, nil
}

func loadDraftPortfolioSnapshot() (*portfolio.Snapshot, error) {
	draftSnapMu.Lock()
	stub := draftSnapStub
	draftSnapMu.Unlock()
	if stub.active {
		return stub.snap, stub.err
	}
	return portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
}
