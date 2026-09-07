package candidate

import (
	"fmt"
	"strings"

	"go-stock/backend/decision/authority"
	"go-stock/backend/tradeplan/draft"
)

// ValidationResult candidate validation outcome.
type ValidationResult struct {
	OK      bool     `json:"ok"`
	Codes   []string `json:"codes,omitempty"`
	Message string   `json:"message"`
}

// ValidateTradePlanCandidate checks provenance + Executable isolation against source draft.
func ValidateTradePlanCandidate(c TradePlanCandidate, d draft.TradePlanDraft) ValidationResult {
	var codes []string

	if c.Executable {
		codes = append(codes, CodeCandidateExecuteForbidden)
	}
	// force isolation observation
	c.Executable = false

	srcHash := d.SourceSnapshotHash
	if srcHash == "" {
		srcHash = d.SnapshotHash
	}
	if c.SourceDecisionID == "" || c.SourceDecisionID != d.SourceDecisionID {
		codes = append(codes, CodeSourceInvalid)
	}
	if c.SourceSnapshotHash == "" || c.SourceSnapshotHash != srcHash {
		codes = append(codes, CodeSourceInvalid)
	}

	auth := VerifyCandidateAuthority()
	if !auth.OK {
		codes = append(codes, CodeAuthorityDenied)
	}

	if len(codes) > 0 {
		return ValidationResult{OK: false, Codes: codes, Message: "REJECTED: " + strings.Join(codes, ",")}
	}
	return ValidationResult{OK: true, Message: "OK: candidate source bound; Executable=false"}
}

// VerifyCandidateIntegrity detects mutation of sealed candidate fields.
func VerifyCandidateIntegrity(c TradePlanCandidate) ValidationResult {
	if c.Executable {
		return ValidationResult{
			OK: false, Codes: []string{CodeCandidateExecuteForbidden},
			Message: CodeCandidateExecuteForbidden,
		}
	}
	got := ComputeCandidateHash(c)
	if c.CandidateHash == "" || got != c.CandidateHash {
		return ValidationResult{
			OK: false, Codes: []string{CodeCandidateMutationDetected},
			Message: CodeCandidateMutationDetected,
		}
	}
	return ValidationResult{OK: true, Message: "OK: candidate integrity intact"}
}

// AuthorityCheckResult shadow candidate authority.
type AuthorityCheckResult struct {
	OK                  bool   `json:"ok"`
	CanReadDecision     bool   `json:"canReadDecision"`
	CanCreateCandidate  bool   `json:"canCreateCandidate"`
	CanModifyDecision   bool   `json:"canModifyDecision"`
	CanAuthorizeExecute bool   `json:"canAuthorizeExecute"`
	Message             string `json:"message"`
}

// VerifyCandidateAuthority aligns with TRADEPLAN_CANDIDATE_SHADOW matrix.
func VerifyCandidateAuthority() AuthorityCheckResult {
	read := authority.Authorize(authority.RoleTradePlanCandidateShadow)
	perm := authority.PermissionFor(authority.RoleTradePlanCandidateShadow)
	createTrade := authority.AuthorizeCreateTrade(authority.RoleTradePlanCandidateShadow)

	ok := read.Allowed && perm.ReadDecision && perm.CreateCandidate &&
		!perm.WriteDecision && !perm.ModifyAction && !perm.CreateTrade && !createTrade.Allowed

	msg := "OK: may read Decision & create Candidate shadow; cannot modify Decision/Action; cannot Execute"
	if !ok {
		msg = "DENIED: candidate shadow authority violated"
	}
	return AuthorityCheckResult{
		OK:                  ok,
		CanReadDecision:     perm.ReadDecision && read.Allowed,
		CanCreateCandidate:  perm.CreateCandidate,
		CanModifyDecision:   perm.WriteDecision || perm.ModifyAction,
		CanAuthorizeExecute: createTrade.Allowed || perm.CreateTrade,
		Message:             msg,
	}
}

// RejectExecutableInjection returns candidate_execute_forbidden if Executable was forced true.
func RejectExecutableInjection(c *TradePlanCandidate) error {
	if c == nil {
		return fmt.Errorf("%s: nil candidate", CodeCandidateExecuteForbidden)
	}
	if c.Executable {
		c.Executable = false
		return fmt.Errorf("%s", CodeCandidateExecuteForbidden)
	}
	return nil
}
