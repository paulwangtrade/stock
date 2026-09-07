// Package candidate provides Phase5-B TradePlanCandidate Shadow Projection.
//
// TradePlanDraft → DraftToTradePlanCandidate → TradePlanCandidate
//
// TradePlanCandidate is NOT models.TradePlan:
//   - evaluable plan shadow
//   - comparable object
//   - shadow output only (never a live trade plan)
//
// Boundaries:
//   - Does NOT modify TradePlan / invoke plan builders / write TradePlan DB
//   - Does NOT connect order submit paths or live trading adapters
//   - Does NOT change Decision Schema or CandidatePool Rank/Score
//   - Executable is always false; Action.Code → IntentKind only
package candidate
