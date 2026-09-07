// Package registry provides QuantDecision Registry, Traceability (Phase3-B),
// and Snapshot Integrity (Phase3-C).
//
// CandidatePoolItem.decisionId → Registry → sealed QuantDecision snapshot
//
// Boundaries (frozen):
//   - Query / bind-trace / immutable snapshot / conflict detection only
//   - Does NOT connect TradePlan / Execution
//   - Does NOT mutate CandidatePoolItem.Score / Rank
//   - Does NOT switch Go producer into BuildCandidatePool
package registry
