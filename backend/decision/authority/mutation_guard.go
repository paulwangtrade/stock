package authority

import (
	"go-stock/backend/decision/registry"
	"go-stock/backend/models"
)

// MutationResult consumer-side mutation check against sealed snapshot.
type MutationResult struct {
	DecisionID         string `json:"decisionId"`
	MutationDetected   bool   `json:"mutationDetected"`
	SealedHash         string `json:"sealedHash,omitempty"`
	ObservedHash       string `json:"observedHash,omitempty"`
	SealedActionCode   string `json:"sealedActionCode,omitempty"`
	ObservedActionCode string `json:"observedActionCode,omitempty"`
	Summary            string `json:"summary"`
}

// VerifyDecisionReadOnlyUsage compares a consumer-held Decision against the sealed snapshot.
// Read-only: does not write registry; does not touch Candidate Rank/Score.
// If Action.Code (or any semantic content) differs → mutation_detected.
func VerifyDecisionReadOnlyUsage(reg *registry.Registry, decisionID string, observed *models.QuantDecision) MutationResult {
	out := MutationResult{DecisionID: decisionID}
	if reg == nil {
		out.MutationDetected = true
		out.Summary = "mutation_detected: nil registry"
		return out
	}
	snap, ok := reg.GetSnapshot(decisionID)
	if !ok || snap.Decision == nil {
		out.MutationDetected = true
		out.Summary = "mutation_detected: sealed snapshot missing"
		return out
	}
	out.SealedHash = snap.Hash
	out.SealedActionCode = snap.Decision.Action.Code

	if observed == nil {
		out.MutationDetected = true
		out.Summary = "mutation_detected: nil observed decision"
		return out
	}
	out.ObservedActionCode = observed.Action.Code
	out.ObservedHash = registry.SemanticContentHash(observed)

	if out.SealedHash != "" && out.ObservedHash == out.SealedHash {
		out.MutationDetected = false
		out.Summary = "OK: read-only usage matches sealed snapshot"
		return out
	}
	out.MutationDetected = true
	if out.SealedActionCode != out.ObservedActionCode {
		out.Summary = "mutation_detected: Action.Code changed"
		return out
	}
	out.Summary = "mutation_detected: semantic content hash drift"
	return out
}
