package strategysnapshot

import (
	"errors"
	"fmt"
	"strings"
)

// LayeredStore reads DB first, then memory; writes memory cache then DB.
type LayeredStore struct {
	db     Store
	memory Store
}

// NewLayeredStore requires a primary (usually SQLite) and optional memory fallback.
func NewLayeredStore(primary, memory Store) *LayeredStore {
	if memory == nil {
		memory = NewMemoryStore()
	}
	return &LayeredStore{db: primary, memory: memory}
}

func (l *LayeredStore) Save(s *StrategySnapshot) error {
	if l == nil {
		return fmt.Errorf("strategysnapshot: layered store nil")
	}
	if l.memory != nil {
		_ = l.memory.Save(s)
	}
	if l.db == nil {
		return fmt.Errorf("strategysnapshot: primary store nil")
	}
	return l.db.Save(s)
}

func (l *LayeredStore) Get(snapshotID string) (*StrategySnapshot, error) {
	if l == nil {
		return nil, ErrNotFound
	}
	if l.db != nil {
		s, err := l.db.Get(snapshotID)
		if err == nil {
			return s, nil
		}
		if !isStoreNotFound(err) {
			if l.memory != nil {
				if ms, merr := l.memory.Get(snapshotID); merr == nil {
					return ms, nil
				}
			}
			return nil, err
		}
	}
	if l.memory != nil {
		return l.memory.Get(snapshotID)
	}
	return nil, ErrNotFound
}

func (l *LayeredStore) ListByPlan(planID uint) ([]StrategySnapshot, error) {
	if l == nil {
		return nil, nil
	}
	if l.db != nil {
		list, err := l.db.ListByPlan(planID)
		if err == nil && len(list) > 0 {
			return list, nil
		}
		if err != nil && !isStoreNotFound(err) {
			if l.memory != nil {
				return l.memory.ListByPlan(planID)
			}
			return nil, err
		}
	}
	if l.memory != nil {
		return l.memory.ListByPlan(planID)
	}
	return nil, nil
}

func (l *LayeredStore) SavePlanRef(ref *PlanReference) error {
	if l == nil {
		return fmt.Errorf("strategysnapshot: layered store nil")
	}
	if l.memory != nil {
		_ = l.memory.SavePlanRef(ref)
	}
	if l.db == nil {
		return fmt.Errorf("strategysnapshot: primary store nil")
	}
	return l.db.SavePlanRef(ref)
}

func (l *LayeredStore) GetPlanRef(planID uint) (*PlanReference, error) {
	if l == nil {
		return nil, ErrNotFound
	}
	if l.db != nil {
		ref, err := l.db.GetPlanRef(planID)
		if err == nil {
			return ref, nil
		}
		if !isStoreNotFound(err) {
			if l.memory != nil {
				if mr, merr := l.memory.GetPlanRef(planID); merr == nil {
					return mr, nil
				}
			}
			return nil, err
		}
	}
	if l.memory != nil {
		return l.memory.GetPlanRef(planID)
	}
	return nil, ErrNotFound
}

func isStoreNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}
