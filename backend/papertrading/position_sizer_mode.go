package papertrading

import "strings"

// Position sizer modes for paper_trading_mvp.json positionSizerMode.
// Missing / unknown values normalize to fixed_amount so old configs are unchanged.
const (
	PositionSizerModeFixedAmount    = "fixed_amount"
	PositionSizerModePortfolioAware = "portfolio_aware"
)

// NormalizePositionSizerMode maps config text to a supported mode.
// Empty / unknown → fixed_amount (existing production behavior).
func NormalizePositionSizerMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case PositionSizerModePortfolioAware:
		return PositionSizerModePortfolioAware
	default:
		return PositionSizerModeFixedAmount
	}
}
