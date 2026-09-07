// Package providershadow is the G.12 / H3.1 DecisionProvider Shadow Runtime.
//
// AllocationShadowRuntime runs LegacyDecisionProvider and PortfolioDecisionProvider
// on the same DecisionContext:
//
//	Legacy  → fixed_amount envelope (write-chain identity; ChainEnvelope)
//	Portfolio → Selection + PortfolioRiskSnapshot + SuggestTighten + AllocationEngine
//	         → portfolio_allocation shadow envelope (observation only)
//
// ShadowComparisonRecord carries allocation_version, risk_constraint_trace,
// allocation_budget_summary, reserve_summary, risk_cut_summary, plus candidate /
// amount / allocation / risk-adjustment diffs on the nested report.
//
// It does not run plan-state risk filtering, persist TradePlan, Freeze, materialize,
// execute, fill, change the write-chain provider, or enable Controlled.
// DefaultEnabled is false. Portfolio failure is recorded; Legacy Draft path is untouched.
package providershadow
