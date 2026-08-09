package usagemetrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/featuregate"
)

// RecordInput is the caller-facing payload for Record (FeatureGate / UI).
type RecordInput struct {
	UserID    string
	Feature   featuregate.Feature
	EventType EventType
	Timestamp time.Time // zero → now
	Metadata  map[string]string
}

// Service captures and queries local usage metrics.
type Service interface {
	Record(ctx context.Context, in RecordInput) (*UsageEvent, error)
	Query(ctx context.Context, filter QueryFilter) ([]UsageEvent, error)
	Count(ctx context.Context, filter QueryFilter) (int, error)
	Summarize(ctx context.Context, userID string) ([]UsageSummary, error)
	// Phase13-E commercial analytics
	FeatureUsageCount(ctx context.Context, filter AnalyticsFilter) (int, error)
	UserFeatureSummary(ctx context.Context, userID string, from, to time.Time) ([]UsageSummary, error)
	SummarizeAnalytics(ctx context.Context, filter AnalyticsFilter) ([]UsageSummary, error)
}

// Store is the persistence port (memory in C4; no cloud sync).
type Store interface {
	Append(ctx context.Context, ev *UsageEvent) error
	Query(ctx context.Context, filter QueryFilter) ([]UsageEvent, error)
}

// MemoryStore is an in-process event log (local only).
type MemoryStore struct {
	mu     sync.RWMutex
	events []UsageEvent
}

// NewMemoryStore constructs an empty local store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{events: make([]UsageEvent, 0, 64)}
}

func (s *MemoryStore) Append(_ context.Context, ev *UsageEvent) error {
	if ev == nil {
		return fmt.Errorf("usagemetrics: nil event")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cloneEvent(*ev)
	s.events = append(s.events, cp)
	return nil
}

func (s *MemoryStore) Query(_ context.Context, filter QueryFilter) ([]UsageEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UsageEvent, 0)
	fromOK := !filter.From.IsZero()
	toOK := !filter.To.IsZero()
	from := filter.From.UTC()
	to := filter.To.UTC()
	for i := len(s.events) - 1; i >= 0; i-- {
		ev := s.events[i]
		if filter.UserID != "" && ev.UserID != filter.UserID {
			continue
		}
		if filter.Feature != "" && ev.Feature != filter.Feature {
			continue
		}
		if filter.EventType != "" && ev.EventType != filter.EventType {
			continue
		}
		ts := ev.Timestamp.UTC()
		if fromOK && ts.Before(from) {
			continue
		}
		if toOK && !ts.Before(to) {
			// To is exclusive upper bound
			continue
		}
		out = append(out, cloneEvent(ev))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func cloneEvent(ev UsageEvent) UsageEvent {
	cp := ev
	if len(ev.Metadata) > 0 {
		cp.Metadata = make(map[string]string, len(ev.Metadata))
		for k, v := range ev.Metadata {
			cp.Metadata[k] = v
		}
	}
	return cp
}

type service struct {
	store Store
	now   func() time.Time
	seq   atomic.Uint64
}

var defaultService Service

func init() {
	defaultService = NewService(NewMemoryStore())
}

// Default returns the process-wide usage metrics service.
func Default() Service {
	return defaultService
}

// NewService returns a local usage metrics Service.
func NewService(store Store) Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &service{store: store, now: time.Now}
}

func (s *service) Record(ctx context.Context, in RecordInput) (*UsageEvent, error) {
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		return nil, fmt.Errorf("usagemetrics: user_id is required")
	}
	if !featuregate.IsKnown(in.Feature) {
		return nil, fmt.Errorf("usagemetrics: invalid feature %q", in.Feature)
	}
	if !IsKnownEventType(in.EventType) {
		return nil, fmt.Errorf("usagemetrics: invalid event_type %q", in.EventType)
	}
	ts := in.Timestamp
	if ts.IsZero() {
		ts = s.now().UTC()
	} else {
		ts = ts.UTC()
	}
	ev := &UsageEvent{
		ID:        fmt.Sprintf("ue-%d", s.seq.Add(1)),
		UserID:    userID,
		Feature:   in.Feature,
		EventType: in.EventType,
		Timestamp: ts,
		Metadata:  SanitizeMetadata(in.Metadata),
	}
	if err := s.store.Append(ctx, ev); err != nil {
		return nil, err
	}
	cp := cloneEvent(*ev)
	return &cp, nil
}

func (s *service) Query(ctx context.Context, filter QueryFilter) ([]UsageEvent, error) {
	if strings.TrimSpace(filter.UserID) == "" && !filter.AllowEmptyUser {
		return nil, fmt.Errorf("usagemetrics: user_id is required for query")
	}
	if filter.Limit < 0 {
		filter.Limit = 0
	}
	return s.store.Query(ctx, filter)
}

func (s *service) Count(ctx context.Context, filter QueryFilter) (int, error) {
	// Count ignores Limit.
	filter.Limit = 0
	list, err := s.Query(ctx, filter)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

func (s *service) Summarize(ctx context.Context, userID string) ([]UsageSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("usagemetrics: user_id is required")
	}
	// Full known-feature summary (C4); commercial subset → UserFeatureSummary.
	return s.SummarizeAnalytics(ctx, AnalyticsFilter{UserID: userID})
}
