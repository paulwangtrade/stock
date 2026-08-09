package subscription

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/user"
)

// Service is the subscription domain port.
// It manages plan/status/period only — not payment, login, or FeatureGate branching.
type Service interface {
	// GetCurrentPlan returns the effective plan for user (expired → FREE).
	GetCurrentPlan(ctx context.Context, u *user.User) (Plan, error)

	// GetSubscription returns the stored row (may be expired); nil if none.
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)

	// AssignPlan manually sets subscription state (C3 stub; not a payment provider).
	AssignPlan(ctx context.Context, userID string, plan PlanCode, expiresAt time.Time) (*Subscription, error)

	// ResolveEntitlement builds a permission snapshot for the Entitlement layer.
	// Subscription itself does not call FeatureGate.Allow.
	ResolveEntitlement(ctx context.Context, u *user.User) (*Entitlement, error)

	// SyncToFeatureGate applies Subscription → backend/entitlement → FeatureGate.
	SyncToFeatureGate(ctx context.Context, u *user.User) error
}

// Store persists subscription rows (memory in C3).
type Store interface {
	GetByUserID(ctx context.Context, userID string) (*Subscription, error)
	Save(ctx context.Context, sub *Subscription) error
}

// MemoryStore is an in-process Store (no cloud billing DB).
type MemoryStore struct {
	mu   sync.RWMutex
	byID map[string]*Subscription // userID → subscription
}

// NewMemoryStore constructs an empty subscription store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: make(map[string]*Subscription)}
}

func (s *MemoryStore) GetByUserID(_ context.Context, userID string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.byID[userID]
	if !ok {
		return nil, nil
	}
	cp := *sub
	return &cp, nil
}

func (s *MemoryStore) Save(_ context.Context, sub *Subscription) error {
	if sub == nil || strings.TrimSpace(sub.UserID) == "" {
		return fmt.Errorf("subscription: invalid row")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *sub
	s.byID[sub.UserID] = &cp
	return nil
}

type service struct {
	store Store
	now   func() time.Time
}

// NewService returns the default subscription Service.
func NewService(store Store) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &service{store: store, now: time.Now}
}

func (s *service) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("subscription: userId is required")
	}
	return s.store.GetByUserID(ctx, userID)
}

func (s *service) GetCurrentPlan(ctx context.Context, u *user.User) (Plan, error) {
	if u == nil || strings.TrimSpace(u.UserID) == "" {
		return LookupPlan(PlanFREE), fmt.Errorf("subscription: user is required")
	}
	sub, err := s.store.GetByUserID(ctx, u.UserID)
	if err != nil {
		return LookupPlan(PlanFREE), err
	}
	now := s.now().UTC()
	code := PlanFREE
	if sub != nil {
		code = sub.EffectivePlanCode(now)
	}
	return LookupPlan(code), nil
}

func (s *service) ResolveEntitlement(ctx context.Context, u *user.User) (*Entitlement, error) {
	if u == nil || strings.TrimSpace(u.UserID) == "" {
		return nil, fmt.Errorf("subscription: user is required")
	}
	sub, err := s.store.GetByUserID(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if sub == nil {
		return ResolveEntitlementFromPlan(u.UserID, PlanFREE, "default_free"), nil
	}
	code := sub.EffectivePlanCode(now)
	source := "subscription"
	if code == PlanFREE {
		if NormalizePlanCode(sub.PlanCode) != PlanFREE {
			source = "expired_fallback"
		} else {
			source = "default_free"
		}
	}
	return ResolveEntitlementFromPlan(u.UserID, code, source), nil
}

// SyncToFeatureGate materializes Subscription → Entitlement domain → FeatureGate.
func (s *service) SyncToFeatureGate(ctx context.Context, u *user.User) error {
	ent, err := s.ResolveEntitlement(ctx, u)
	if err != nil {
		return err
	}
	return entitlement.Default().EnsureTierDefaults(ent.ToFeatureGateUser())
}

func (s *service) AssignPlan(ctx context.Context, userID string, plan PlanCode, expiresAt time.Time) (*Subscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("subscription: userId is required")
	}
	plan = NormalizePlanCode(plan)
	now := s.now().UTC()
	status := StatusActive
	if plan == PlanFREE {
		status = StatusNone
	}
	if !expiresAt.IsZero() && !expiresAt.After(now) {
		status = StatusExpired
	}
	sub := &Subscription{
		ID:        "sub-" + userID,
		UserID:    userID,
		PlanCode:  plan,
		Status:    status,
		StartsAt:  now,
		ExpiresAt: expiresAt.UTC(),
		UpdatedAt: now,
	}
	if plan == PlanFREE {
		sub.ExpiresAt = time.Time{}
		sub.Status = StatusNone
	}
	if err := s.store.Save(ctx, sub); err != nil {
		return nil, err
	}
	cp := *sub
	return &cp, nil
}
