// Package draft provides TradePlan Draft Projection (Phase4-A) and
// Draft Lifecycle Isolation (Phase4-B).
//
// # DecisionSnapshot → TradePlanDraft → Draft Lifecycle Validation
//
// Boundaries (frozen):
//   - Does NOT connect Execution / PaperBroker / Broker / Order
//   - Does NOT call BuildTradePlan
//   - Does NOT mutate Decision, Candidate, Rank, or Score
//   - Decision.Action is Fact/Explanation only — NOT execution authorization
//   - AllowDraft must NEVER imply EnableExecute=true
package draft
