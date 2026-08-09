package license

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service loads/stores the active desktop license and validates it offline-first.
type Service interface {
	GetLicense(ctx context.Context) (*License, error)
	InstallLicense(ctx context.Context, lic *License) error
	ClearLicense(ctx context.Context) error
	Validate(ctx context.Context) (Result, error)
	// ActivateWithKey runs the simulated license-key validation flow, then installs on success.
	ActivateWithKey(ctx context.Context, key string) (KeyValidation, error)
}

// Store persists at most one active license blob (memory in C6).
type Store interface {
	Get(ctx context.Context) (*License, error)
	Save(ctx context.Context, lic *License) error
	Clear(ctx context.Context) error
}

// MemoryStore is a local in-process license store (no cloud / no forced online).
type MemoryStore struct {
	mu  sync.RWMutex
	lic *License
}

// NewMemoryStore constructs an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Get(_ context.Context) (*License, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lic == nil {
		return nil, nil
	}
	cp := *s.lic
	return &cp, nil
}

func (s *MemoryStore) Save(_ context.Context, lic *License) error {
	if lic == nil {
		return fmt.Errorf("license: nil license")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *lic
	s.lic = &cp
	return nil
}

func (s *MemoryStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lic = nil
	return nil
}

type service struct {
	store     Store
	validator LicenseValidator
	now       func() time.Time
}

// NewService returns a license Service with offline-first validation.
func NewService(store Store, validator LicenseValidator) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	if validator == nil {
		validator = NewLocalValidator("")
	}
	return &service{store: store, validator: validator, now: time.Now}
}

func (s *service) GetLicense(ctx context.Context) (*License, error) {
	return s.store.Get(ctx)
}

func (s *service) InstallLicense(ctx context.Context, lic *License) error {
	if lic == nil {
		return fmt.Errorf("license: nil license")
	}
	if strings.TrimSpace(lic.ID) == "" {
		return fmt.Errorf("license: id is required")
	}
	lic.Mode = NormalizeMode(lic.Mode)
	lic.Type = NormalizeType(lic.Type)
	return s.store.Save(ctx, lic)
}

func (s *service) ClearLicense(ctx context.Context) error {
	return s.store.Clear(ctx)
}

func (s *service) Validate(ctx context.Context) (Result, error) {
	lic, err := s.store.Get(ctx)
	if err != nil {
		return Result{
			PaperTradingAllowed: true,
			EffectiveType:       TypeFree,
			Status:              StatusInvalid,
			Reason:              err.Error(),
			CheckedAt:           s.now().UTC(),
		}, err
	}
	res := s.validator.Validate(lic, s.now().UTC())
	res.PaperTradingAllowed = true
	// Persist last status onto stored license when present.
	if lic != nil && res.License != nil {
		updated := *res.License
		_ = s.store.Save(ctx, &updated)
	}
	return res, nil
}

func (s *service) ActivateWithKey(ctx context.Context, key string) (KeyValidation, error) {
	kv := ValidateLicenseKey(key, s.now().UTC())
	if !kv.OK || kv.License == nil {
		return kv, nil
	}
	if err := s.InstallLicense(ctx, kv.License); err != nil {
		return KeyValidation{OK: false, Reason: err.Error(), RawHint: kv.RawHint}, err
	}
	res, err := s.Validate(ctx)
	if err != nil {
		return KeyValidation{OK: false, Reason: err.Error(), RawHint: kv.RawHint}, err
	}
	kv.License = res.License
	kv.Reason = res.Reason
	if res.Status != StatusValid && res.Status != StatusGrace {
		kv.OK = false
		kv.Reason = "installed but validation status=" + string(res.Status)
	}
	return kv, nil
}
