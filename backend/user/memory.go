package user

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultLocalUserID is the single-device local identity used before cloud login.
	DefaultLocalUserID = "local-device"
	// DefaultLocalDisplayName is the bootstrap display name for local users.
	DefaultLocalDisplayName = "Local User"
)

// MemoryStore is an in-process Store for C5 tests and early Shell wiring.
// It is not a cloud database and does not write migrations.
type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]*User
	profiles map[string]*UserProfile
}

// NewMemoryStore constructs an empty in-memory identity store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    make(map[string]*User),
		profiles: make(map[string]*UserProfile),
	}
}

func (s *MemoryStore) GetUser(_ context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *MemoryStore) SaveUser(_ context.Context, u *User) error {
	if u == nil || strings.TrimSpace(u.UserID) == "" {
		return fmt.Errorf("user: invalid user")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *u
	s.users[u.UserID] = &cp
	return nil
}

func (s *MemoryStore) GetProfile(_ context.Context, userID string) (*UserProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (s *MemoryStore) SaveProfile(_ context.Context, p *UserProfile) error {
	if p == nil || strings.TrimSpace(p.UserID) == "" {
		return fmt.Errorf("user: invalid profile")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *p
	s.profiles[p.UserID] = &cp
	return nil
}

// service implements Service against a Store.
type service struct {
	store Store
	now   func() time.Time
}

// NewService returns the default identity Service.
func NewService(store Store) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &service{store: store, now: time.Now}
}

func (s *service) EnsureLocalUser(ctx context.Context) (*User, error) {
	u, err := s.store.GetUser(ctx, DefaultLocalUserID)
	if err == nil {
		return u, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	now := s.now().UTC()
	u = &User{
		UserID:       DefaultLocalUserID,
		DisplayName:  DefaultLocalDisplayName,
		Kind:         KindLocal,
		AuthProvider: AuthProviderDevice,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	profile := &UserProfile{
		UserID:       u.UserID,
		DisplayName:  u.DisplayName,
		Preferences:  map[string]string{},
		Settings:     map[string]string{},
		UpdatedAt:    now,
	}
	if err := s.store.SaveProfile(ctx, profile); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *service) GetUser(ctx context.Context, userID string) (*User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user: userId is required")
	}
	return s.store.GetUser(ctx, userID)
}

func (s *service) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user: userId is required")
	}
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	p, err := s.store.GetProfile(ctx, userID)
	if err == nil {
		return p, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	p = &UserProfile{
		UserID:       u.UserID,
		DisplayName:  u.DisplayName,
		Preferences:  map[string]string{},
		Settings:     map[string]string{},
		UpdatedAt:    now,
	}
	if err := s.store.SaveProfile(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *service) UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (*UserProfile, error) {
	p, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if patch.DisplayName != nil {
		name := strings.TrimSpace(*patch.DisplayName)
		if name == "" {
			return nil, fmt.Errorf("user: displayName must not be empty")
		}
		p.DisplayName = name
	}
	if patch.Email != nil {
		p.Email = strings.TrimSpace(*patch.Email)
	}
	if patch.AvatarURL != nil {
		p.AvatarURL = strings.TrimSpace(*patch.AvatarURL)
	}
	if patch.Preferences != nil {
		if p.Preferences == nil {
			p.Preferences = map[string]string{}
		}
		for k, v := range patch.Preferences {
			p.Preferences[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	if patch.Settings != nil {
		if p.Settings == nil {
			p.Settings = map[string]string{}
		}
		for k, v := range patch.Settings {
			p.Settings[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	p.UpdatedAt = s.now().UTC()
	if err := s.store.SaveProfile(ctx, p); err != nil {
		return nil, err
	}
	// Keep User.DisplayName in sync when profile display name changes.
	if patch.DisplayName != nil {
		u, err := s.store.GetUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		u.DisplayName = p.DisplayName
		u.UpdatedAt = p.UpdatedAt
		if err := s.store.SaveUser(ctx, u); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// NewCloudUserStub builds a cloud Kind user domain object without contacting any cloud API.
// C5 forbids OAuth / WeChat / Google Login / cloud DB; this is for forward-compatible tests only.
func NewCloudUserStub(userID, displayName, externalSubject string) *User {
	now := time.Now().UTC()
	return &User{
		UserID:          userID,
		DisplayName:     displayName,
		Kind:            KindCloud,
		AuthProvider:    AuthProviderOIDC,
		ExternalSubject: externalSubject,
		Status:          StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}
