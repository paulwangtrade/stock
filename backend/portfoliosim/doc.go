// Package portfoliosim is the Phase12-F.8 controlled simulation harness (enhanced).
//
// Sandbox flow:
//
//	Selection → PortfolioRiskSnapshot → SuggestTighten → ApplyTightenOnly →
//	PortfolioDecisionProvider / AllocationEngine → PlanFilter (read-only)
//
// Independent of the production trading chain: no Draft, no trade_plans writes,
// no Execution, no Materialization. Result always has not_a_trade_plan=true.
package portfoliosim
