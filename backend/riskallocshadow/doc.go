// Package riskallocshadow is the H0.2 + H.3 joint Shadow model:
//
//	PortfolioRiskSnapshot → SuggestTighten → ApplyTightenOnly → AllocationEngine
//
// It compares Legacy fixed_amount allocation vs Risk-adjusted Portfolio allocation.
// record_only: does not write TradePlan, RiskView, or Execution.
// Preference may only tighten; Risk ceilings cannot be widened.
package riskallocshadow
