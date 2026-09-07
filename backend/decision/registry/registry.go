package registry

import (
	"fmt"
	"sync"

	"go-stock/backend/models"
)

// Registry in-memory QuantDecision store keyed by DecisionID.
// Phase3-B: traceability. Phase3-C: immutable snapshots + conflict detection.
type Registry struct {
	mu         sync.RWMutex
	byID       map[string]*models.QuantDecision
	hashByID   map[string]string
	sealedAt   map[string]string
	byCodeAsOf map[string]string // code|asOf → first DecisionID
	conflicts  []Conflict
}

// New creates an empty registry.
func New() *Registry {
	return &Registry{
		byID:       map[string]*models.QuantDecision{},
		hashByID:   map[string]string{},
		sealedAt:   map[string]string{},
		byCodeAsOf: map[string]string{},
	}
}

// Put registers an immutable Decision snapshot (Phase3-C PutImmutable).
// Idempotent when content hash matches; refuses content overwrite for same ID.
// Does not touch Candidate Score/Rank, TradePlan, or Execution.
func (r *Registry) Put(d *models.QuantDecision) (string, error) {
	res, err := r.PutImmutable(d)
	if err != nil {
		return "", err
	}
	if res.Conflict != nil && res.Conflict.Kind == ConflictKindIDContent {
		// still return sealed id; caller may inspect Conflicts()
		return res.DecisionID, nil
	}
	return res.DecisionID, nil
}

// PutOrConflict is like Put but returns error when id_content_conflict occurs.
func (r *Registry) PutOrConflict(d *models.QuantDecision) (PutResult, error) {
	res, err := r.PutImmutable(d)
	if err != nil {
		return res, err
	}
	if res.Conflict != nil && res.Conflict.Kind == ConflictKindIDContent {
		return res, fmt.Errorf("registry: %s", res.Conflict.Summary)
	}
	return res, nil
}

// Get returns a clone of the registered Decision, if present.
func (r *Registry) Get(decisionID string) (*models.QuantDecision, bool) {
	if r == nil || decisionID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byID[decisionID]
	if !ok || d == nil {
		return nil, false
	}
	return cloneDecision(d), true
}

// Has reports whether decisionID is registered.
func (r *Registry) Has(decisionID string) bool {
	_, ok := r.Get(decisionID)
	return ok
}

// Count registered decisions.
func (r *Registry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// IDs returns registered DecisionIDs (unsorted).
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	return out
}
