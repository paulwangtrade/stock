package draft

import (
	"fmt"
	"strings"

	"go-stock/backend/decision/authority"
	"go-stock/backend/decision/registry"
)

// ValidationResult outcome of ValidateTradePlanDraft.
type ValidationResult struct {
	OK      bool        `json:"ok"`
	Status  DraftStatus `json:"status"`
	Codes   []string    `json:"codes,omitempty"`
	Message string      `json:"message"`
}

// AuthorityCheckResult VerifyDraftAuthority outcome.
type AuthorityCheckResult struct {
	OK                  bool   `json:"ok"`
	CanReadDecision     bool   `json:"canReadDecision"`
	CanModifyDecision   bool   `json:"canModifyDecision"`
	CanAuthorizeExecute bool   `json:"canAuthorizeExecute"`
	Message             string `json:"message"`
}

// ValidateTradePlanDraft runs Phase4-B lifecycle validation against sealed snapshot.
// Updates draft Status to VALIDATED or REJECTED. Never mutates Decision / Candidate.
func ValidateTradePlanDraft(draft *TradePlanDraft, snap registry.Snapshot) ValidationResult {
	if draft == nil {
		return ValidationResult{OK: false, Status: DraftRejected, Codes: []string{CodeSourceMismatch}, Message: "nil draft"}
	}
	var codes []string

	if draft.EnableExecute {
		codes = append(codes, CodeExecuteForbidden)
	}
	// Force isolation regardless of input
	draft.EnableExecute = false

	if snap.Decision == nil || strings.TrimSpace(snap.DecisionID) == "" {
		codes = append(codes, CodeSourceMismatch)
	} else if draft.SourceDecisionID != snap.DecisionID {
		codes = append(codes, CodeSourceMismatch)
	}

	snapHash := snap.Hash
	draftHashRef := draft.SourceSnapshotHash
	if draftHashRef == "" {
		draftHashRef = draft.SnapshotHash
	}
	if snapHash == "" || draftHashRef != snapHash {
		codes = append(codes, CodeSnapshotChanged)
	} else if snap.Decision != nil && registry.SemanticContentHash(snap.Decision) != snapHash {
		codes = append(codes, CodeSnapshotChanged)
	}

	if auth := VerifyDraftAuthority(); !auth.OK {
		codes = append(codes, CodeAuthorityDenied)
	}

	if len(codes) > 0 {
		msg := "REJECTED: " + strings.Join(codes, ",")
		draft.AdvanceToRejected(codes, msg)
		return ValidationResult{OK: false, Status: DraftRejected, Codes: codes, Message: msg}
	}

	msg := "VALIDATED: source bound; execute isolated"
	draft.AdvanceToValidated(msg)
	return ValidationResult{OK: true, Status: DraftValidated, Message: msg}
}

// ValidateDraftSource Phase4-A compat wrapper (error if not OK).
func ValidateDraftSource(draft *TradePlanDraft, snap registry.Snapshot) error {
	res := ValidateTradePlanDraft(draft, snap)
	if !res.OK {
		return fmt.Errorf("draft: %s", res.Message)
	}
	return nil
}

// VerifyDraftIntegrity recomputes DraftHash over sealed fields; detects payload mutation.
func VerifyDraftIntegrity(draft *TradePlanDraft) ValidationResult {
	if draft == nil {
		return ValidationResult{
			OK: false, Status: DraftRejected,
			Codes: []string{CodeDraftMutationDetected}, Message: "draft_mutation_detected: nil draft",
		}
	}
	got := ComputeDraftHash(draft)
	if draft.DraftHash == "" || got != draft.DraftHash {
		return ValidationResult{
			OK: false, Status: draft.Status,
			Codes: []string{CodeDraftMutationDetected}, Message: "draft_mutation_detected",
		}
	}
	if draft.EnableExecute {
		return ValidationResult{
			OK: false, Status: draft.Status,
			Codes: []string{CodeExecuteForbidden}, Message: "execute_forbidden",
		}
	}
	return ValidationResult{OK: true, Status: draft.Status, Message: "OK: draft integrity intact"}
}

// VerifyDraftImmutable Phase4-A compat.
func VerifyDraftImmutable(draft *TradePlanDraft) error {
	res := VerifyDraftIntegrity(draft)
	if !res.OK {
		return fmt.Errorf("draft: %s", res.Message)
	}
	return nil
}

// VerifyDraftAuthority aligns with decision/authority TRADEPLAN_DRAFT_FUTURE matrix.
func VerifyDraftAuthority() AuthorityCheckResult {
	read := authority.Authorize(authority.RoleTradePlanDraftFuture)
	perm := authority.PermissionFor(authority.RoleTradePlanDraftFuture)
	create := authority.AuthorizeCreateTrade(authority.RoleTradePlanDraftFuture)

	ok := read.Allowed && perm.ReadDecision &&
		!perm.WriteDecision && !perm.ModifyAction && !perm.CreateTrade &&
		!create.Allowed

	msg := "OK: draft may read Decision; cannot modify Decision/Action; cannot authorize Execution"
	if !ok {
		msg = "DENIED: draft authority matrix violated"
	}
	return AuthorityCheckResult{
		OK:                  ok,
		CanReadDecision:     perm.ReadDecision && read.Allowed,
		CanModifyDecision:   perm.WriteDecision || perm.ModifyAction,
		CanAuthorizeExecute: create.Allowed || perm.CreateTrade,
		Message:             msg,
	}
}
