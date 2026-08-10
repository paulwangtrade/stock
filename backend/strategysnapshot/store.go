package strategysnapshot

import (
	"fmt"
	"sync"
)

// Store persists snapshots and plan references (memory in Phase11-G; no schema migration).
type Store interface {
	Save(s *StrategySnapshot) error
	Get(snapshotID string) (*StrategySnapshot, error)
	ListByPlan(planID uint) ([]StrategySnapshot, error)
	SavePlanRef(ref *PlanReference) error
	GetPlanRef(planID uint) (*PlanReference, error)
}

// ErrNotFound is returned when a snapshot or plan reference is missing.
var ErrNotFound = fmt.Errorf("strategysnapshot: not found")

// MemoryStore is an in-process JSON-friendly store.
type MemoryStore struct {
	mu    sync.RWMutex
	byID  map[string]*StrategySnapshot
	byPlan map[uint][]string // planID → snapshot IDs (order preserved)
	refs  map[uint]*PlanReference
}

// NewMemoryStore constructs an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID:   make(map[string]*StrategySnapshot),
		byPlan: make(map[uint][]string),
		refs:   make(map[uint]*PlanReference),
	}
}

func (m *MemoryStore) Save(s *StrategySnapshot) error {
	if s == nil || s.SnapshotID == "" {
		return fmt.Errorf("strategysnapshot: invalid snapshot")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	m.byID[s.SnapshotID] = &cp
	ids := m.byPlan[s.PlanID]
	for _, id := range ids {
		if id == s.SnapshotID {
			return nil
		}
	}
	m.byPlan[s.PlanID] = append(ids, s.SnapshotID)
	return nil
}

func (m *MemoryStore) Get(snapshotID string) (*StrategySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byID[snapshotID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *MemoryStore) ListByPlan(planID uint) ([]StrategySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.byPlan[planID]
	out := make([]StrategySnapshot, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.byID[id]; ok {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *MemoryStore) SavePlanRef(ref *PlanReference) error {
	if ref == nil || ref.PlanID == 0 {
		return fmt.Errorf("strategysnapshot: invalid plan reference")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *ref
	if ref.ItemSnapshotIDs != nil {
		cp.ItemSnapshotIDs = make(map[uint]string, len(ref.ItemSnapshotIDs))
		for k, v := range ref.ItemSnapshotIDs {
			cp.ItemSnapshotIDs[k] = v
		}
	}
	m.refs[ref.PlanID] = &cp
	return nil
}

func (m *MemoryStore) GetPlanRef(planID uint) (*PlanReference, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ref, ok := m.refs[planID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *ref
	if ref.ItemSnapshotIDs != nil {
		cp.ItemSnapshotIDs = make(map[uint]string, len(ref.ItemSnapshotIDs))
		for k, v := range ref.ItemSnapshotIDs {
			cp.ItemSnapshotIDs[k] = v
		}
	}
	return &cp, nil
}
