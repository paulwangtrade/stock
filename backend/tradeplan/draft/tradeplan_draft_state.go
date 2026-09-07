package draft

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// AdvanceToValidated marks a successfully validated draft (lifecycle only).
func (d *TradePlanDraft) AdvanceToValidated(msg string) {
	if d == nil {
		return
	}
	d.Status = DraftValidated
	d.Validation = DraftValidation{OK: true, Message: msg}
	d.Message = msg
	d.EnableExecute = false
	d.DraftHash = ComputeDraftHash(d)
}

// AdvanceToRejected marks rejection with reason codes.
func (d *TradePlanDraft) AdvanceToRejected(codes []string, msg string) {
	if d == nil {
		return
	}
	d.Status = DraftRejected
	d.Validation = DraftValidation{OK: false, Codes: append([]string{}, codes...), Message: msg}
	d.Message = msg
	d.EnableExecute = false
	d.DraftHash = ComputeDraftHash(d)
}

// MarkExpired marks draft expired (e.g. source snapshot gone). Does not touch Decision.
func (d *TradePlanDraft) MarkExpired(msg string) {
	if d == nil {
		return
	}
	d.Status = DraftExpired
	d.Validation = DraftValidation{OK: false, Codes: []string{CodeSnapshotChanged}, Message: msg}
	d.Message = msg
	d.EnableExecute = false
	d.DraftHash = ComputeDraftHash(d)
}

// CanValidate reports whether lifecycle allows validation attempt.
func (d *TradePlanDraft) CanValidate() bool {
	if d == nil {
		return false
	}
	return d.Status == DraftCreated || d.Status == DraftRejected
}

func newDraftID(sourceDecisionID string) string {
	sum := sha256.Sum256([]byte(sourceDecisionID))
	return fmt.Sprintf("tpd:%s", hex.EncodeToString(sum[:8]))
}
