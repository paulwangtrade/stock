// Package shadow provides Phase5-C TradePlanCandidate vs Existing TradePlan
// batch comparison (shadow only).
//
// DecisionSnapshot → Draft → TradePlanCandidate ──compare──> Existing TradePlan
//
// Boundaries:
//   - Does NOT modify plan builders or write TradePlan DB
//   - Does NOT connect live trading adapters / order submit paths
//   - Does NOT change Decision Schema or CandidatePool Rank/Score
package shadow
