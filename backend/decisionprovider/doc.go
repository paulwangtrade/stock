// Package decisionprovider is the G.2 DecisionProvider seam (Selection → envelope).
//
// G.10 extracts LegacyDecisionProvider; G.11 adds PortfolioDecisionProvider.
// G.12 runs Portfolio only inside providershadow (bypass). Production write
// chain remains Legacy → plan-state risk filtering → TradePlan.
package decisionprovider
