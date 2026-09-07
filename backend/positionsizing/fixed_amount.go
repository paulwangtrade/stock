package positionsizing

import (
	"sync"

	"go-stock/backend/logger"
	"go-stock/backend/tradingconfig"
)

// FixedAmountSizer reproduces the legacy per-name fixed budget:
// planned_amount = TradingConfig.OpenBuyAmountPerStock() (legacy_paper_open_buy → 100000 default).
// It does not perform percent, risk-budget, or score-weighted sizing.
type FixedAmountSizer struct {
	Provider tradingconfig.Provider
}

// Propose returns the fixed-amount proposal (behavior identical to pre-MVP OpenBuyAmountPerStock).
// Request.Portfolio / Score / Rank are ignored — this slice does not change buy amounts.
func (s *FixedAmountSizer) Propose(_ Request) SizingProposal {
	p := s.provider()
	pos := p.Resolve().Position
	amount := p.OpenBuyAmountPerStock()
	return SizingProposal{
		PlannedAmount: amount,
		PlannedVolume: 0,
		Method:        MethodFixedAmount,
		Source:        pos.Source,
		AmountReason: &AmountReason{
			Method:      string(MethodFixedAmount),
			FinalAmount: amount,
			RawAmount:   amount,
			CappedBy:    []string{capFixed},
		},
	}
}

func (s *FixedAmountSizer) provider() tradingconfig.Provider {
	if s != nil && s.Provider != nil {
		return s.Provider
	}
	return tradingconfig.Default()
}

var (
	defaultSizer PositionSizer = &FixedAmountSizer{}
	logMu        sync.Mutex
	loggedOnce   bool
)

// Default returns the process-wide PositionSizer (ProposeDefault / TEMP builders).
// Always FixedAmountSizer so missing config and existing tests keep 100000.
// Draft Builder selects PortfolioAwareSizer via ForMode when positionSizerMode=portfolio_aware.
func Default() PositionSizer {
	return defaultSizer
}

// ForMode returns a sizer for paper_trading.positionSizerMode.
// Unknown / empty / fixed_amount → FixedAmountSizer. Does not query the database.
func ForMode(mode string) PositionSizer {
	if tradingconfig.IsPortfolioAwareSizerMode(mode) {
		return &PortfolioAwareSizer{}
	}
	return &FixedAmountSizer{}
}

// SetDefaultSizer replaces the process sizer (tests only).
func SetDefaultSizer(s PositionSizer) {
	if s == nil {
		defaultSizer = &FixedAmountSizer{}
		return
	}
	defaultSizer = s
}

// ResetDefaultSizer restores FixedAmountSizer (tests).
func ResetDefaultSizer() {
	defaultSizer = &FixedAmountSizer{}
}

// ProposeDefault is a convenience wrapper around Default().Propose.
func ProposeDefault(req Request) SizingProposal {
	return Default().Propose(req)
}

// PlannedAmount returns the MVP fixed planned_amount via the default sizer.
func PlannedAmount() float64 {
	return ProposeDefault(Request{}).PlannedAmount
}

// LogApplied emits an observability line for the applied proposal.
// once=true logs only the first application in-process (reduces AfterClose noise);
// once=false always logs (tests / explicit call sites).
func LogApplied(proposal SizingProposal, once bool) {
	if once {
		logMu.Lock()
		if loggedOnce {
			logMu.Unlock()
			return
		}
		loggedOnce = true
		logMu.Unlock()
	}
	if proposal.AmountReason != nil {
		logger.SugaredLogger.Infof(
			"PositionSizing applied method=%s source=%s planned_amount=%.0f planned_volume=%d capped_by=%v budget=%.0f raw=%.0f final=%.0f",
			proposal.Method,
			proposal.Source,
			proposal.PlannedAmount,
			proposal.PlannedVolume,
			proposal.AmountReason.CappedBy,
			proposal.AmountReason.Budget,
			proposal.AmountReason.RawAmount,
			proposal.AmountReason.FinalAmount,
		)
		return
	}
	logger.SugaredLogger.Infof(
		"PositionSizing applied method=%s source=%s planned_amount=%.0f planned_volume=%d",
		proposal.Method,
		proposal.Source,
		proposal.PlannedAmount,
		proposal.PlannedVolume,
	)
}

// ResetLogAppliedForTest clears the one-shot log gate (tests).
func ResetLogAppliedForTest() {
	logMu.Lock()
	loggedOnce = false
	logMu.Unlock()
}
