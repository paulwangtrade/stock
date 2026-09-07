package strategyintent

import (
	"fmt"
	"strings"
)

// ValidationError is a domain validation failure.
type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

const (
	CodeInvalidStatus            = "invalid_status"
	CodeInvalidIntentType        = "invalid_intent_type"
	CodeInvalidSchemaRef         = "invalid_schema_ref"
	CodeMissingField             = "missing_field"
	CodeForbiddenRevision        = "forbidden_revision_ref"
	CodeSchemaRevisionNotFound   = "schema_revision_not_found"
	CodeSchemaRevisionNotBindable = "schema_revision_not_bindable"
)

// ValidateIntent checks sint-1 shape and schema pin rules (B1-A).
func ValidateIntent(in Intent) error {
	if strings.TrimSpace(in.ID) == "" {
		return ValidationError{Code: CodeMissingField, Message: "id is required"}
	}
	if strings.TrimSpace(in.CandidateID) == "" {
		return ValidationError{Code: CodeMissingField, Message: "candidate_id is required"}
	}
	if !ValidStatuses[in.Status] {
		return ValidationError{Code: CodeInvalidStatus, Message: fmt.Sprintf("status %q is not a valid intent lifecycle value", in.Status)}
	}
	if !ValidIntentTypes[in.IntentType] {
		return ValidationError{Code: CodeInvalidIntentType, Message: fmt.Sprintf("intent_type %q is invalid", in.IntentType)}
	}
	if strings.TrimSpace(in.Action.Verb) == "" {
		return ValidationError{Code: CodeMissingField, Message: "action.verb is required"}
	}
	if err := ValidateSchemaRef(in.StrategySchemaRef, in.SchemaRevision); err != nil {
		return err
	}
	return nil
}

// ValidateSchemaRef enforces pinned strategy_id + revision (never "current" alone).
func ValidateSchemaRef(ref SchemaRef, schemaRevision string) error {
	if ref.Unbound {
		// unbound may omit strategy_id/revision; top-level schema_revision should be empty
		if strings.TrimSpace(ref.StrategyID) != "" || strings.TrimSpace(ref.Revision) != "" || strings.TrimSpace(schemaRevision) != "" {
			// allow optional note-only unbound; if any pin field set, must be fully consistent
			if strings.TrimSpace(ref.StrategyID) == "" || effectiveRevision(ref, schemaRevision) == "" {
				return ValidationError{
					Code:    CodeInvalidSchemaRef,
					Message: "partial schema pin on unbound intent: clear pins or set unbound=false with strategy_id+revision",
				}
			}
			return validatePinnedRevision(ref, schemaRevision)
		}
		return nil
	}

	// bound: must pin strategy_id + revision/version
	if strings.TrimSpace(ref.StrategyID) == "" {
		return ValidationError{
			Code:    CodeInvalidSchemaRef,
			Message: "strategy_schema_ref.strategy_id is required when unbound=false",
		}
	}
	return validatePinnedRevision(ref, schemaRevision)
}

func effectiveRevision(ref SchemaRef, schemaRevision string) string {
	r := strings.TrimSpace(ref.Revision)
	if r != "" {
		return r
	}
	return strings.TrimSpace(schemaRevision)
}

func validatePinnedRevision(ref SchemaRef, schemaRevision string) error {
	rev := strings.TrimSpace(ref.Revision)
	top := strings.TrimSpace(schemaRevision)

	if rev == "" && top == "" {
		return ValidationError{
			Code:    CodeInvalidSchemaRef,
			Message: "schema revision/version is required (strategy_schema_ref.revision or schema_revision); cannot reference current only",
		}
	}
	if rev == "" {
		rev = top
	}
	if top != "" && rev != top {
		return ValidationError{
			Code:    CodeInvalidSchemaRef,
			Message: fmt.Sprintf("schema_revision %q must match strategy_schema_ref.revision %q", top, rev),
		}
	}
	if isForbiddenRevisionLabel(rev) {
		return ValidationError{
			Code:    CodeForbiddenRevision,
			Message: fmt.Sprintf("revision %q is forbidden; Intent must pin an explicit Schema revision, not current", rev),
		}
	}
	if strings.TrimSpace(ref.StrategyID) == "" {
		return ValidationError{
			Code:    CodeInvalidSchemaRef,
			Message: "strategy_id is required with a pinned revision",
		}
	}
	return nil
}

func isForbiddenRevisionLabel(rev string) bool {
	s := strings.ToLower(strings.TrimSpace(rev))
	switch s {
	case "current", "latest", "head", "active_current", "definition_current":
		return true
	default:
		return false
	}
}
