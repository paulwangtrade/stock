package strategyschema

import "fmt"

// Error codes for API mapping (B3-B).
const (
	CodeNotFound           = "not_found"
	CodeBadRequest         = "bad_request"
	CodeRevisionImmutable  = "revision_immutable"
	CodeInvalidTransition  = "invalid_transition"
	CodeDraftConflict      = "draft_conflict"
	CodeHashMismatch       = "hash_mismatch"
	CodeHashRequired       = "hash_required"
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
		Code:    CodeRevisionImmutable,
		Message: fmt.Sprintf("revision status %q is frozen; create a new revision to change content", status),
	}
}

func errInvalidTransition(from, to string) error {
	return WriteError{
		Code:    CodeInvalidTransition,
		Message: fmt.Sprintf("cannot transition revision from %q to %q", from, to),
	}
}

func errDraftConflict(strategyID string) error {
	return WriteError{
		Code:    CodeDraftConflict,
		Message: fmt.Sprintf("strategy %s already has a working draft or reviewing revision", strategyID),
	}
}

func errHashMismatch(kind string) error {
	return WriteError{
		Code:    CodeHashMismatch,
		Message: fmt.Sprintf("%s does not match revision content", kind),
	}
}

func errHashRequired(kind string) error {
	return WriteError{
		Code:    CodeHashRequired,
		Message: fmt.Sprintf("%s is required before submit/activate", kind),
	}
}
