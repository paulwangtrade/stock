package usagemetrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/featuregate"
)

// AnalyticsReader is the Phase13-E Usage Analytics read model.
//
// Architecture: UsageMetrics (events) → AnalyticsReader → Product API / Dashboard.
// Read-only; must not write trading state or accept sensitive payloads.
type AnalyticsReader interface {
	// FeatureCount returns usage count for one feature (optional event/time/user).
	FeatureCount(ctx context.Context, filter AnalyticsFilter) (int, error)
	// UserSummary returns commercial UsageSummary rows for one user.
	UserSummary(ctx context.Context, userID string, from, to time.Time) ([]UsageSummary, error)
	// CommercialSummary aggregates commercial features (AIAnalysis / AdvancedRisk / …).
	CommercialSummary(ctx context.Context, filter AnalyticsFilter) ([]UsageSummary, error)
	// Catalog returns features included in commercial analytics.
	Catalog() []featuregate.Feature
}

type analyticsReader struct {
	svc Service
}

// NewAnalyticsReader wraps a UsageMetrics Service as a commercial read model.
func NewAnalyticsReader(svc Service) AnalyticsReader {
	if svc == nil {
		svc = Default()
	}
	return &analyticsReader{svc: svc}
}

var (
	defaultReaderMu sync.RWMutex
	defaultReader   AnalyticsReader
)

// DefaultAnalyticsReader returns the process-wide commercial analytics reader (lazy).
// Lazy init avoids package init ordering races with Default() Service.
func DefaultAnalyticsReader() AnalyticsReader {
	defaultReaderMu.RLock()
	r := defaultReader
	defaultReaderMu.RUnlock()
	if r != nil {
		return r
	}
	defaultReaderMu.Lock()
	defer defaultReaderMu.Unlock()
	if defaultReader == nil {
		defaultReader = NewAnalyticsReader(Default())
	}
	return defaultReader
}

// SetDefaultAnalyticsReader replaces the process reader (tests).
func SetDefaultAnalyticsReader(r AnalyticsReader) {
	defaultReaderMu.Lock()
	defer defaultReaderMu.Unlock()
	if r == nil {
		defaultReader = NewAnalyticsReader(Default())
		return
	}
	defaultReader = r
}

func (r *analyticsReader) Catalog() []featuregate.Feature {
	return featuregate.CommercialFeatures()
}

func (r *analyticsReader) FeatureCount(ctx context.Context, filter AnalyticsFilter) (int, error) {
	feat := filter.Feature
	if feat == "" {
		return 0, fmt.Errorf("usagemetrics: feature is required")
	}
	if !featuregate.IsKnown(feat) {
		return 0, fmt.Errorf("usagemetrics: invalid feature %q", feat)
	}
	if r.svc == nil {
		return 0, fmt.Errorf("usagemetrics: analytics service is nil")
	}
	return r.svc.FeatureUsageCount(ctx, filter)
}

func (r *analyticsReader) UserSummary(ctx context.Context, userID string, from, to time.Time) ([]UsageSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("usagemetrics: user_id is required")
	}
	if r.svc == nil {
		return nil, fmt.Errorf("usagemetrics: analytics service is nil")
	}
	return r.svc.UserFeatureSummary(ctx, userID, from, to)
}

func (r *analyticsReader) CommercialSummary(ctx context.Context, filter AnalyticsFilter) ([]UsageSummary, error) {
	if filter.Feature != "" {
		if !featuregate.IsKnown(filter.Feature) {
			return nil, fmt.Errorf("usagemetrics: invalid feature %q", filter.Feature)
		}
		if !featuregate.IsCommercialFeature(filter.Feature) {
			return nil, fmt.Errorf("usagemetrics: feature %q is not in commercial catalog", filter.Feature)
		}
	}
	if filter.EventType != "" && !IsKnownEventType(filter.EventType) {
		return nil, fmt.Errorf("usagemetrics: invalid event_type %q", filter.EventType)
	}
	if r.svc == nil {
		return nil, fmt.Errorf("usagemetrics: analytics service is nil")
	}
	filter.CommercialOnly = true
	return r.svc.SummarizeAnalytics(ctx, filter)
}
