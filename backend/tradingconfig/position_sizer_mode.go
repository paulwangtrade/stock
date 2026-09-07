package tradingconfig

import "strings"

// NormalizePositionSizerMode maps config text to a supported Draft sizer mode.
// Empty / unknown → fixed_amount so old JSON files keep current amounts.
func NormalizePositionSizerMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case PositionSizerModePortfolioAware:
		return PositionSizerModePortfolioAware
	default:
		return PositionSizerModeFixedAmount
	}
}

// IsPortfolioAwareSizerMode reports whether Draft should inject a Portfolio Snapshot.
func IsPortfolioAwareSizerMode(s string) bool {
	return NormalizePositionSizerMode(s) == PositionSizerModePortfolioAware
}
