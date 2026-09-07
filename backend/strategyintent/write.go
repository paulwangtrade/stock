package strategyintent

import (
	"strings"
	"time"
)

// CreateDraft creates a draft intent for a candidate (manual only; B1-B).
func CreateDraft(req CreateDraftRequest) (WriteResult, error) {
	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" {
		return WriteResult{}, errBadRequest("candidate_id is required")
	}
	if strings.TrimSpace(req.Action.Verb) == "" {
		return WriteResult{}, errBadRequest("action.verb is required")
	}
	intentType := strings.TrimSpace(req.IntentType)
	if intentType == "" {
		intentType = IntentTypeManual
	}
	if intentType == IntentTypeAIDraft {
		return WriteResult{}, errBadRequest("ai_draft intent_type is not allowed in B1-B")
	}
	if !ValidIntentTypes[intentType] {
		return WriteResult{}, ValidationError{Code: CodeInvalidIntentType, Message: "invalid intent_type"}
	}

	schemaRef := req.StrategySchemaRef
	schemaRevision := strings.TrimSpace(req.SchemaRevision)
	normalizeSchemaRevision(&schemaRef, &schemaRevision)

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		id := IntentIDForCandidate(candidateID)
		if existing, ok := f.Intents[id]; ok {
			switch existing.Status {
			case StatusDraft, StatusReviewing:
				return errDraftConflict(candidateID)
			case StatusApproved:
				return errCreateNotAllowed(candidateID)
			case StatusExpired, StatusDiscarded:
				// allow recreate in same id slot (new intent document)
			default:
				return errBadRequest("intent already exists for candidate")
			}
		} else {
			for _, existing := range f.Intents {
				if existing.CandidateID != candidateID {
					continue
				}
				switch existing.Status {
				case StatusDraft, StatusReviewing:
					return errDraftConflict(candidateID)
				case StatusApproved:
					return errCreateNotAllowed(candidateID)
				}
			}
		}

		now := time.Now().UTC()
		in := Intent{
			ID:                id,
			SchemaVersion:     SchemaVersion,
			CandidateID:       candidateID,
			StrategySchemaRef: schemaRef,
			SchemaRevision:    schemaRevision,
			IntentType:        intentType,
			Conditions:        req.Conditions,
			Action:            req.Action,
			RiskConstraints:   req.RiskConstraints,
			Status:            StatusDraft,
			Summary:           strings.TrimSpace(req.Summary),
			ExplainRef:        strings.TrimSpace(req.ExplainRef),
			PromotedPoolID:    nil,
			TradePlanID:       nil,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := validateSchemaBinding(in.StrategySchemaRef, in.SchemaRevision, schemaBindingContext{}); err != nil {
			return err
		}
		if err := ValidateIntent(in); err != nil {
			return err
		}
		f.Intents[id] = in
		out = WriteResult{Intent: in}
		return nil
	})
	return out, err
}

// UpdateDraft patches content of a draft intent only.
func UpdateDraft(id string, req UpdateDraftRequest) (WriteResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WriteResult{}, errBadRequest("intent id is required")
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		in, ok := f.Intents[id]
		if !ok {
			return ErrNotFound{ID: id}
		}
		if in.Status != StatusDraft {
			return errImmutable(in.Status)
		}

		priorRef := in.StrategySchemaRef
		priorRevision := in.SchemaRevision

		now := time.Now().UTC()
		if req.ExplainRef != nil {
			in.ExplainRef = strings.TrimSpace(*req.ExplainRef)
		}
		if req.StrategySchemaRef != nil {
			in.StrategySchemaRef = *req.StrategySchemaRef
		}
		if req.SchemaRevision != nil {
			in.SchemaRevision = strings.TrimSpace(*req.SchemaRevision)
		}
		normalizeSchemaRevision(&in.StrategySchemaRef, &in.SchemaRevision)
		if req.IntentType != nil {
			t := strings.TrimSpace(*req.IntentType)
			if t == IntentTypeAIDraft {
				return errBadRequest("ai_draft intent_type is not allowed in B1-B")
			}
			if !ValidIntentTypes[t] {
				return ValidationError{Code: CodeInvalidIntentType, Message: "invalid intent_type"}
			}
			in.IntentType = t
		}
		if req.Summary != nil {
			in.Summary = strings.TrimSpace(*req.Summary)
		}
		if req.Conditions != nil {
			in.Conditions = *req.Conditions
		}
		if req.Action != nil {
			in.Action = *req.Action
		}
		if req.RiskConstraints != nil {
			in.RiskConstraints = *req.RiskConstraints
		}
		in.UpdatedAt = now
		in.PromotedPoolID = nil
		in.TradePlanID = nil

		if strings.TrimSpace(in.Action.Verb) == "" {
			return errBadRequest("action.verb is required")
		}
		if err := validateSchemaBinding(in.StrategySchemaRef, in.SchemaRevision, schemaBindingContext{
			priorRef:      &priorRef,
			priorRevision: priorRevision,
		}); err != nil {
			return err
		}
		if err := ValidateIntent(in); err != nil {
			return err
		}
		f.Intents[id] = in
		out = WriteResult{Intent: in}
		return nil
	})
	return out, err
}

// SubmitReview transitions draft → reviewing.
func SubmitReview(id string) (WriteResult, error) {
	return transitionIntent(id, StatusReviewing, nil)
}

// Approve transitions reviewing → approved (human review; B1-B).
func Approve(id string) (WriteResult, error) {
	return transitionIntent(id, StatusApproved, nil)
}

func transitionIntent(id, to string, before func(*Intent) error) (WriteResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WriteResult{}, errBadRequest("intent id is required")
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		in, ok := f.Intents[id]
		if !ok {
			return ErrNotFound{ID: id}
		}
		if !CanTransition(in.Status, to) {
			return errInvalidTransition(in.Status, to)
		}
		if before != nil {
			if err := before(&in); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		in.Status = to
		in.UpdatedAt = now
		in.PromotedPoolID = nil
		in.TradePlanID = nil
		if err := ValidateIntent(in); err != nil {
			return err
		}
		f.Intents[id] = in
		out = WriteResult{Intent: in}
		return nil
	})
	return out, err
}

func normalizeSchemaRevision(ref *SchemaRef, top *string) {
	if ref == nil || top == nil {
		return
	}
	rev := strings.TrimSpace(ref.Revision)
	if rev != "" {
		*top = rev
		ref.Revision = rev
	} else if strings.TrimSpace(*top) != "" {
		ref.Revision = strings.TrimSpace(*top)
	}
}
