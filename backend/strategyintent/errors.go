package strategyintent

import "fmt"

// Write error codes for API mapping (B1-B).
const (
	CodeIntentImmutable   = "intent_immutable"
	CodeInvalidTransition = "invalid_transition"
	CodeDraftConflict     = "draft_conflict"
	CodeBadRequest        = "bad_request"
	CodeCreateNotAllowed  = "create_not_allowed"
)

// WriteError is a typed lifecycle write failure.
type WriteError struct {
	Code    string
	Message string
}

func (e WriteError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

func errBadRequest(msg string) error {
	return WriteError{Code: CodeBadRequest, Message: msg}
}

func errImmutable(status string) error {
	return WriteError{
		Code:    CodeIntentImmutable,
		Message: fmt.Sprintf("intent status %q is frozen; only draft is editable (approved changes require new intent — not implemented)", status),
	}
}

func errInvalidTransition(from, to string) error {
	return WriteError{
		Code:    CodeInvalidTransition,
		Message: fmt.Sprintf("cannot transition intent from %q to %q", from, to),
	}
}

func errDraftConflict(candidateID string) error {
	return WriteError{
		Code:    CodeDraftConflict,
		Message: fmt.Sprintf("candidate %s already has a draft or reviewing intent", candidateID),
	}
}

func errCreateNotAllowed(candidateID string) error {
	return WriteError{
		Code:    CodeCreateNotAllowed,
		Message: fmt.Sprintf("candidate %s already has an approved intent; create new intent is not implemented in B1-B", candidateID),
	}
}
