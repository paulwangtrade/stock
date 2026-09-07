package decisionprovider

import (
	"math"
	"strings"

	"go-stock/backend/positionsizing"
	"go-stock/backend/selection"
)

// LegacyDecisionProvider wraps the production fixed-amount path:
// one scalar from FixedAmountSizer (or an injected UniformAmount) copied onto
// every ranked candidate. It does not Select, Filter, or persist.
type LegacyDecisionProvider struct {
	Sizer positionsizing.PositionSizer
}

// NewLegacyDecisionProvider returns a Legacy provider bound to FixedAmountSizer.
// A nil sizer uses a new FixedAmountSizer (not positionsizing.Default, so test
// Default swaps cannot silently turn this into an aware sizer).
func NewLegacyDecisionProvider(sizer positionsizing.PositionSizer) *LegacyDecisionProvider {
	if sizer == nil {
		sizer = &positionsizing.FixedAmountSizer{}
	}
	return &LegacyDecisionProvider{Sizer: sizer}
}

func (p *LegacyDecisionProvider) Name() string { return ProviderFixedAmount }

func (p *LegacyDecisionProvider) sizer() positionsizing.PositionSizer {
	if p != nil && p.Sizer != nil {
		return p.Sizer
	}
	return &positionsizing.FixedAmountSizer{}
}

// Decide copies one scalar onto ranked_candidates in order (G.3).
// UniformAmount>0 is the FilterPool compatibility injection (OFF amount argument).
func (p *LegacyDecisionProvider) Decide(ctx DecisionContext) (*DecisionEnvelope, error) {
	if ctx.Version.Provider != "" && ctx.Version.Provider != p.Name() {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "version.provider mismatch")
	}
	if ctx.Selection == nil {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "selection is required")
	}
	if ctx.DecisionTime.IsZero() {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "decision_time is required")
	}

	amount, err := p.resolveAmount(ctx)
	if err != nil {
		env := emptyEnvelope(ctx, p.Name())
		env.Error = err
		return env, err
	}

	ranked := ctx.Selection.RankedCandidates
	if ranked == nil {
		ranked = []selection.Candidate{}
	}
	limit := ctx.Selection.SelectionLimit
	lines := make([]DecisionLine, 0, len(ranked))
	for i, c := range ranked {
		sym := strings.ToLower(strings.TrimSpace(c.StockCode))
		if sym == "" {
			return failEnvelope(ctx, p.Name(), ErrCodeMissingSymbol, "empty symbol")
		}
		reason := strings.TrimSpace(c.Reason)
		lines = append(lines, DecisionLine{
			Symbol:       sym,
			TargetAmount: amount,
			Reason:       reason,
			Metadata: LineMeta{
				Rank:             c.Rank,
				Score:            c.Score,
				StockName:        strings.TrimSpace(c.StockName),
				Industry:         strings.TrimSpace(c.Industry),
				InAllocationSet:  i < limit,
				AllocationReason: AllocReasonFixedAmount,
				SourceProvider:   ProviderFixedAmount,
				CandidateReason:  reason,
			},
		})
	}

	ver := ctx.Version
	if ver.Provider == "" {
		ver.Provider = p.Name()
	}
	if ver.Contract == "" {
		ver.Contract = ContractG21
	}
	return &DecisionEnvelope{
		OK:             true,
		Provider:       p.Name(),
		DecisionTime:   ctx.DecisionTime,
		Version:        ver,
		SelectionLimit: limit,
		Lines:          lines,
		Rejected:       []RejectedLine{},
		Metadata:       EnvelopeMeta{UniformAmount: amount},
	}, nil
}

func (p *LegacyDecisionProvider) resolveAmount(ctx DecisionContext) (float64, *DecisionError) {
	if ctx.UniformAmount != 0 || math.IsNaN(ctx.UniformAmount) {
		if ctx.UniformAmount <= 0 || !isFinite(ctx.UniformAmount) {
			return 0, &DecisionError{Code: ErrCodeZeroAmount, Message: "uniform amount is not a positive finite number"}
		}
		return ctx.UniformAmount, nil
	}
	proposal := p.sizer().Propose(positionsizing.Request{})
	if proposal.Method != positionsizing.MethodFixedAmount {
		return 0, &DecisionError{Code: ErrCodeInvalidAllocation, Message: "sizer method is not fixed_amount"}
	}
	amount := proposal.PlannedAmount
	if amount <= 0 || !isFinite(amount) {
		return 0, &DecisionError{Code: ErrCodeZeroAmount, Message: "fixed amount sizer returned non-positive amount"}
	}
	return amount, nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func emptyEnvelope(ctx DecisionContext, provider string) *DecisionEnvelope {
	return &DecisionEnvelope{
		OK:           false,
		Provider:     provider,
		DecisionTime: ctx.DecisionTime,
		Version:      ctx.Version,
		Lines:        []DecisionLine{},
		Rejected:     []RejectedLine{},
	}
}

func failEnvelope(ctx DecisionContext, provider, code, msg string) (*DecisionEnvelope, error) {
	err := &DecisionError{Code: code, Message: msg}
	env := emptyEnvelope(ctx, provider)
	env.Error = err
	return env, err
}
