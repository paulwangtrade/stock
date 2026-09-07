package strategyschema

import (
	"strings"
	"time"
)

// CreateDraft creates a draft revision (and Definition if needed). source=manual only.
func CreateDraft(req CreateDraftRequest) (WriteResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return WriteResult{}, errBadRequest("name is required")
	}
	if req.RiskProfileRef.Mode == "" {
		req.RiskProfileRef.Mode = "inherit"
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		now := time.Now().UTC()
		strategyID := strings.TrimSpace(req.StrategyID)
		createdDef := false

		if strategyID == "" {
			slug := strings.TrimSpace(req.Slug)
			if slug == "" {
				slug = Slugify(name)
			}
			strategyID = strategyIDFromSlug(slug)
			if _, exists := f.Definitions[strategyID]; exists {
				return errBadRequest("strategy_id already exists: " + strategyID + "; pass strategy_id to add a draft on existing definition")
			}
			f.Definitions[strategyID] = Definition{
				StrategyID:      strategyID,
				SchemaVersion:   SchemaVersion,
				Name:            name,
				Description:     strings.TrimSpace(req.Description),
				Status:          DefinitionStatusDraftOnly,
				CurrentRevision: "",
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			createdDef = true
		}

		def, ok := f.Definitions[strategyID]
		if !ok {
			return ErrNotFound{ID: strategyID}
		}
		if def.Status == DefinitionStatusArchived {
			return errBadRequest("definition is archived; cannot create draft")
		}

		existing := revisionsFor(f, strategyID)
		if hasWorkingCopy(existing) {
			return errDraftConflict(strategyID)
		}

		revLabel := nextRevisionLabel(existing)
		parent := strings.TrimSpace(req.ParentRevision)

		rev := Revision{
			RevisionID:     revisionID(strategyID, revLabel),
			StrategyID:     strategyID,
			Revision:       revLabel,
			SchemaVersion:  SchemaVersion,
			Status:         RevisionStatusDraft,
			NameOverride:   strings.TrimSpace(req.NameOverride),
			RevisionNote:   strings.TrimSpace(req.RevisionNote),
			Universe:       req.Universe,
			Signals:        req.Signals,
			Filters:        req.Filters,
			Ranking:        req.Ranking,
			RiskProfileRef: req.RiskProfileRef,
			Parameters:     ParametersSpec{Knobs: req.Knobs},
			Source:         "manual",
			ParentRevision: parent,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// Fork: if parent set and request content empty-ish, copy parent body.
		if parent != "" {
			parentRev, found := findRevision(f, strategyID, parent)
			if !found {
				return ErrNotFound{ID: strategyID + "/" + parent}
			}
			if isEmptyContent(req) {
				rev.Universe = parentRev.Universe
				rev.Signals = parentRev.Signals
				rev.Filters = parentRev.Filters
				rev.Ranking = parentRev.Ranking
				rev.RiskProfileRef = parentRev.RiskProfileRef
				rev.Parameters.Knobs = cloneKnobs(parentRev.Parameters.Knobs)
				if rev.RevisionNote == "" {
					rev.RevisionNote = "fork of " + parent
				}
			}
		}

		ApplyContentHashes(&rev)
		f.Revisions[rev.RevisionID] = rev

		if !createdDef {
			// refresh name/description shell when provided on existing def
			if name != "" {
				def.Name = name
			}
			if strings.TrimSpace(req.Description) != "" {
				def.Description = strings.TrimSpace(req.Description)
			}
			def.UpdatedAt = now
			if def.Status != DefinitionStatusActive {
				def.Status = DefinitionStatusDraftOnly
			}
			f.Definitions[strategyID] = def
		}

		out = WriteResult{Definition: f.Definitions[strategyID], Revision: rev}
		return nil
	})
	return out, err
}

// UpdateDraft patches content of a draft revision only.
func UpdateDraft(strategyID, version string, req UpdateDraftRequest) (WriteResult, error) {
	strategyID = strings.TrimSpace(strategyID)
	version = strings.TrimSpace(version)
	if strategyID == "" || version == "" {
		return WriteResult{}, errBadRequest("strategy_id and revision are required")
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		def, ok := f.Definitions[strategyID]
		if !ok {
			return ErrNotFound{ID: strategyID}
		}
		rev, found := findRevision(f, strategyID, version)
		if !found {
			return ErrNotFound{ID: strategyID + "/" + version}
		}
		if rev.Status != RevisionStatusDraft {
			return errImmutable(rev.Status)
		}

		now := time.Now().UTC()
		if req.RevisionNote != nil {
			rev.RevisionNote = strings.TrimSpace(*req.RevisionNote)
		}
		if req.NameOverride != nil {
			rev.NameOverride = strings.TrimSpace(*req.NameOverride)
		}
		if req.Universe != nil {
			rev.Universe = *req.Universe
		}
		if req.Signals != nil {
			rev.Signals = *req.Signals
		}
		if req.Filters != nil {
			rev.Filters = *req.Filters
		}
		if req.Ranking != nil {
			rev.Ranking = *req.Ranking
		}
		if req.RiskProfileRef != nil {
			rev.RiskProfileRef = *req.RiskProfileRef
			if rev.RiskProfileRef.Mode == "" {
				rev.RiskProfileRef.Mode = "inherit"
			}
		}
		if req.ClearKnobs {
			rev.Parameters.Knobs = map[string]any{}
		} else if req.Knobs != nil {
			rev.Parameters.Knobs = req.Knobs
		}

		rev.UpdatedAt = now
		ApplyContentHashes(&rev)
		f.Revisions[rev.RevisionID] = rev

		def.UpdatedAt = now
		f.Definitions[strategyID] = def

		out = WriteResult{Definition: def, Revision: rev}
		return nil
	})
	return out, err
}

// SubmitReview transitions draft → reviewing after hash verification.
func SubmitReview(strategyID, version string) (WriteResult, error) {
	return transitionRevision(strategyID, version, RevisionStatusReviewing, func(rev *Revision) error {
		if err := minimalContentCheck(*rev); err != nil {
			return err
		}
		ApplyContentHashes(rev) // refresh before gate
		return VerifyContentHashes(*rev)
	})
}

// Activate transitions reviewing → active; retires previous active in the same mutate.
func Activate(strategyID, version string) (WriteResult, error) {
	strategyID = strings.TrimSpace(strategyID)
	version = strings.TrimSpace(version)
	if strategyID == "" || version == "" {
		return WriteResult{}, errBadRequest("strategy_id and revision are required")
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		def, ok := f.Definitions[strategyID]
		if !ok {
			return ErrNotFound{ID: strategyID}
		}
		if def.Status == DefinitionStatusArchived {
			return errBadRequest("definition is archived; cannot activate")
		}
		rev, found := findRevision(f, strategyID, version)
		if !found {
			return ErrNotFound{ID: strategyID + "/" + version}
		}
		if !CanTransitionRevision(rev.Status, RevisionStatusActive) {
			return errInvalidTransition(rev.Status, RevisionStatusActive)
		}
		if rev.Status != RevisionStatusReviewing {
			// CanTransition allows reviewing→active only; reinforce clear error for skip-level
			return errInvalidTransition(rev.Status, RevisionStatusActive)
		}
		ApplyContentHashes(&rev)
		if err := VerifyContentHashes(rev); err != nil {
			return err
		}
		if err := minimalContentCheck(rev); err != nil {
			return err
		}

		now := time.Now().UTC()
		// retire previous active
		for id, other := range f.Revisions {
			if other.StrategyID != strategyID || other.Status != RevisionStatusActive {
				continue
			}
			if other.RevisionID == rev.RevisionID {
				continue
			}
			other.Status = RevisionStatusRetired
			t := now
			other.RetiredAt = &t
			other.UpdatedAt = now
			f.Revisions[id] = other
		}

		rev.Status = RevisionStatusActive
		t := now
		rev.ActivatedAt = &t
		rev.UpdatedAt = now
		f.Revisions[rev.RevisionID] = rev

		def.Status = DefinitionStatusActive
		def.CurrentRevision = rev.Revision
		def.UpdatedAt = now
		f.Definitions[strategyID] = def

		out = WriteResult{Definition: def, Revision: rev}
		return nil
	})
	return out, err
}

func transitionRevision(strategyID, version, to string, before func(*Revision) error) (WriteResult, error) {
	strategyID = strings.TrimSpace(strategyID)
	version = strings.TrimSpace(version)
	if strategyID == "" || version == "" {
		return WriteResult{}, errBadRequest("strategy_id and revision are required")
	}

	var out WriteResult
	err := DefaultStore().Mutate(func(f *storeFile) error {
		def, ok := f.Definitions[strategyID]
		if !ok {
			return ErrNotFound{ID: strategyID}
		}
		rev, found := findRevision(f, strategyID, version)
		if !found {
			return ErrNotFound{ID: strategyID + "/" + version}
		}
		if !CanTransitionRevision(rev.Status, to) {
			return errInvalidTransition(rev.Status, to)
		}
		if before != nil {
			if err := before(&rev); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		rev.Status = to
		rev.UpdatedAt = now
		f.Revisions[rev.RevisionID] = rev
		def.UpdatedAt = now
		f.Definitions[strategyID] = def
		out = WriteResult{Definition: def, Revision: rev}
		return nil
	})
	return out, err
}

func revisionsFor(f *storeFile, strategyID string) []Revision {
	out := make([]Revision, 0)
	for _, r := range f.Revisions {
		if r.StrategyID == strategyID {
			out = append(out, r)
		}
	}
	return out
}

func findRevision(f *storeFile, strategyID, version string) (Revision, bool) {
	for _, r := range f.Revisions {
		if r.StrategyID == strategyID && r.Revision == version {
			return r, true
		}
	}
	return Revision{}, false
}

func hasWorkingCopy(revs []Revision) bool {
	for _, r := range revs {
		if r.Status == RevisionStatusDraft || r.Status == RevisionStatusReviewing {
			return true
		}
	}
	return false
}

func isEmptyContent(req CreateDraftRequest) bool {
	return req.Universe.Source == "" &&
		req.Signals.Kind == "" &&
		len(req.Signals.Rules) == 0 &&
		len(req.Filters.Rules) == 0 &&
		req.Ranking.TopN == 0 &&
		len(req.Knobs) == 0
}

func cloneKnobs(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func minimalContentCheck(r Revision) error {
	if strings.TrimSpace(r.Universe.Source) == "" && len(r.Universe.Include) == 0 {
		return errBadRequest("universe.source or universe.include is required before submit/activate")
	}
	if strings.TrimSpace(r.Signals.Kind) == "" {
		return errBadRequest("signals.kind is required before submit/activate")
	}
	if strings.TrimSpace(r.RiskProfileRef.Mode) == "" {
		return errBadRequest("risk_profile_ref.mode is required")
	}
	return nil
}
