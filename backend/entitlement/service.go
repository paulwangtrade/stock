package entitlement

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/featuregate"
)

// Service is the Entitlement application port.
type Service interface {
	featuregate.EntitlementChecker // HasFeature

	Grant(userID string, feature featuregate.Feature, expiresAt time.Time, source Source) (*Entitlement, error)
	Revoke(userID string, feature featuregate.Feature) error
	Get(userID string, feature featuregate.Feature) (*Entitlement, error)
	List(userID string) ([]Entitlement, error)
	// EnsureTierDefaults materializes catalog rows for user.Tier (Shell bootstrap).
	EnsureTierDefaults(user *featuregate.User) error
}

// Store persists entitlement rows (memory in C2; no schema migration).
type Store interface {
	Upsert(e *Entitlement) error
	Get(userID string, feature featuregate.Feature) (*Entitlement, error)
	ListByUser(userID string) ([]Entitlement, error)
	Delete(userID string, feature featuregate.Feature) error
}

// MemoryStore is an in-process store.
type MemoryStore struct {
	mu   sync.RWMutex
	rows map[string]*Entitlement // key: userID|feature
}

// NewMemoryStore constructs an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rows: make(map[string]*Entitlement)}
}

func rowKey(userID string, feature featuregate.Feature) string {
	return userID + "|" + string(feature)
}

func (s *MemoryStore) Upsert(e *Entitlement) error {
	if e == nil || strings.TrimSpace(e.UserID) == "" {
		return fmt.Errorf("entitlement: invalid row")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *e
	s.rows[rowKey(e.UserID, e.Feature)] = &cp
	return nil
}

func (s *MemoryStore) Get(userID string, feature featuregate.Feature) (*Entitlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.rows[rowKey(userID, feature)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (s *MemoryStore) ListByUser(userID string) ([]Entitlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entitlement, 0)
	prefix := userID + "|"
	for k, e := range s.rows {
		if strings.HasPrefix(k, prefix) {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (s *MemoryStore) Delete(userID string, feature featuregate.Feature) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, rowKey(userID, feature))
	return nil
}

type service struct {
	store Store
	now   func() time.Time
}

var defaultService Service

func init() {
	defaultService = NewService(NewMemoryStore())
	featuregate.RegisterEntitlementChecker(defaultService)
}

// Default returns the process-wide EntitlementService.
func Default() Service {
	return defaultService
}

// SetDefaultForTest replaces the process service and re-registers the FeatureGate checker.
func SetDefaultForTest(s Service) {
	if s == nil {
		s = NewService(NewMemoryStore())
	}
	defaultService = s
	featuregate.RegisterEntitlementChecker(s)
}

// NewService constructs an Entitlement Service.
func NewService(store Store) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &service{store: store, now: time.Now}
}

// HasFeature implements featuregate.EntitlementChecker.
func (s *service) HasFeature(user *featuregate.User, feature featuregate.Feature) bool {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return false
	}
	if !featuregate.IsKnown(feature) {
		return false
	}
	e, err := s.store.Get(user.ID, feature)
	if err != nil {
		return false // missing
	}
	return e.IsActive(s.now().UTC())
}

func (s *service) Grant(userID string, feature featuregate.Feature, expiresAt time.Time, source Source) (*Entitlement, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("entitlement: user is required")
	}
	if !featuregate.IsKnown(feature) {
		return nil, fmt.Errorf("entitlement: unknown feature %q", feature)
	}
	if source == "" {
		source = SourceManual
	}
	now := s.now().UTC()
	e := &Entitlement{
		UserID:    userID,
		Feature:   feature,
		Enabled:   true,
		ExpiresAt: expiresAt,
		Source:    source,
		UpdatedAt: now,
	}
	if !expiresAt.IsZero() {
		e.ExpiresAt = expiresAt.UTC()
	}
	if err := s.store.Upsert(e); err != nil {
		return nil, err
	}
	cp := *e
	return &cp, nil
}

func (s *service) Revoke(userID string, feature featuregate.Feature) error {
	return s.store.Delete(strings.TrimSpace(userID), feature)
}

func (s *service) Get(userID string, feature featuregate.Feature) (*Entitlement, error) {
	return s.store.Get(strings.TrimSpace(userID), feature)
}

func (s *service) List(userID string) ([]Entitlement, error) {
	return s.store.ListByUser(strings.TrimSpace(userID))
}

func (s *service) EnsureTierDefaults(user *featuregate.User) error {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return fmt.Errorf("entitlement: user is required")
	}
	tier := featuregate.NormalizeTier(user.Tier)
	for _, f := range featuregate.KnownFeatures() {
		if !featuregate.CatalogAllows(tier, f) {
			// Ensure disabled/absent for features not in catalog for this tier.
			_ = s.store.Delete(user.ID, f)
			continue
		}
		if _, err := s.Grant(user.ID, f, time.Time{}, SourceTierDefault); err != nil {
			return err
		}
	}
	return nil
}
