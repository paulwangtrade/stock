package providershadow

import (
	"go-stock/backend/decisionprovider"
)

// AllocationShadowRuntime is the H3.1 Portfolio Allocation Shadow entry.
// Independent from production Draft and from any write-chain provider switch.
// Default Enabled is false (inherits DefaultEnabled).
//
// Same DecisionContext: LegacyDecisionProvider + PortfolioDecisionProvider.
// Portfolio path: Selection, PortfolioRiskSnapshot, SuggestTighten, AllocationEngine.
// ChainEnvelope is always Legacy; Portfolio failure is recorded only.
type AllocationShadowRuntime struct {
	inner *Runtime
}

// NewAllocationShadowRuntime builds an isolated Allocation Shadow runtime.
// cfg.Enabled defaults false when omitted; never auto-enables write-chain modes.
func NewAllocationShadowRuntime(cfg Config) *AllocationShadowRuntime {
	if cfg.Legacy == nil {
		cfg.Legacy = decisionprovider.NewLegacyDecisionProvider(nil)
	}
	if cfg.Portfolio == nil {
		cfg.Portfolio = decisionprovider.NewPortfolioDecisionProvider()
	}
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore()
	}
	return &AllocationShadowRuntime{inner: NewRuntime(cfg)}
}

// Enabled reports whether observation is on. Production default remains false.
func (r *AllocationShadowRuntime) Enabled() bool {
	return r != nil && r.inner != nil && r.inner.Enabled()
}

// Observe is the optional Draft-path sidecar (trigger after_close_draft only).
// Never persists TradePlan; never switches provider; Portfolio errors are swallowed into the record.
func (r *AllocationShadowRuntime) Observe(in ObserveInput) *ShadowComparisonRecord {
	if r == nil || r.inner == nil {
		return nil
	}
	return r.inner.Observe(in)
}

// Run compares Legacy fixed_amount vs Portfolio portfolio_allocation on ctx.
// Returns ChainEnvelope=Legacy for write-chain continuity; Portfolio side is observation only.
func (r *AllocationShadowRuntime) Run(ctx decisionprovider.DecisionContext, tradeDate string) (*ShadowComparisonRecord, error) {
	if r == nil || r.inner == nil {
		return nil, nil
	}
	return r.inner.Run(ctx, tradeDate)
}

// RunPrepared is Run with RiskView / Ledger for H0.2 PortfolioRiskSnapshot.
func (r *AllocationShadowRuntime) RunPrepared(ctx decisionprovider.DecisionContext, tradeDate string, prep PrepareOptions) (*ShadowComparisonRecord, error) {
	if r == nil || r.inner == nil {
		return nil, nil
	}
	return r.inner.RunPrepared(ctx, tradeDate, prep)
}

// Store exposes the observation sink (tests / offline report).
func (r *AllocationShadowRuntime) Store() RecordStore {
	if r == nil || r.inner == nil {
		return nil
	}
	return r.inner.Store()
}

// Inner exposes the underlying G.12 Runtime (tests).
func (r *AllocationShadowRuntime) Inner() *Runtime {
	if r == nil {
		return nil
	}
	return r.inner
}
