package usagemetrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/featuregate"
)

// AnalyticsFilter configures Phase13-E commercial usage queries.
// Does not accept holdings, passwords, or trading payloads.
type AnalyticsFilter struct {
	UserID    string
	Feature   featuregate.Feature // empty = commercial catalog (or any if AllFeatures)
	EventType EventType           // empty = any
	From      time.Time
	To        time.Time
	// CommercialOnly when Feature is empty limits aggregation to CommercialFeatures().
	CommercialOnly bool
}

// FeatureUsageCount returns event count for one feature (optionally scoped by user / event / time).
func (s *service) FeatureUsageCount(ctx context.Context, filter AnalyticsFilter) (int, error) {
	if filter.Feature == "" {
		return 0, fmt.Errorf("usagemetrics: feature is required for FeatureUsageCount")
	}
	if !featuregate.IsKnown(filter.Feature) {
		return 0, fmt.Errorf("usagemetrics: invalid feature %q", filter.Feature)
	}
	q := QueryFilter{
		UserID:         strings.TrimSpace(filter.UserID),
		Feature:        filter.Feature,
		EventType:      filter.EventType,
		From:           filter.From,
		To:             filter.To,
		AllowEmptyUser: true,
	}
	return s.Count(ctx, q)
}

// UserFeatureSummary aggregates UsageSummary for one user (optional time range).
func (s *service) UserFeatureSummary(ctx context.Context, userID string, from, to time.Time) ([]UsageSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("usagemetrics: user_id is required for UserFeatureSummary")
	}
	return s.SummarizeAnalytics(ctx, AnalyticsFilter{
		UserID:         userID,
		From:           from,
		To:             to,
		CommercialOnly: true,
	})
}

// SummarizeAnalytics aggregates feature × event_type with count and last_used.
func (s *service) SummarizeAnalytics(ctx context.Context, filter AnalyticsFilter) ([]UsageSummary, error) {
	q := QueryFilter{
		UserID:         strings.TrimSpace(filter.UserID),
		Feature:        filter.Feature,
		EventType:      filter.EventType,
		From:           filter.From,
		To:             filter.To,
		AllowEmptyUser: true,
	}
	list, err := s.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	commercial := map[featuregate.Feature]struct{}{}
	if filter.CommercialOnly && filter.Feature == "" {
		for _, f := range featuregate.CommercialFeatures() {
			commercial[f] = struct{}{}
		}
	}

	type key struct {
		user string
		f    featuregate.Feature
		e    EventType
	}
	type agg struct {
		count int
		last  time.Time
	}
	bucket := map[key]*agg{}
	for _, ev := range list {
		if len(commercial) > 0 {
			if _, ok := commercial[ev.Feature]; !ok {
				continue
			}
		}
		k := key{user: ev.UserID, f: ev.Feature, e: ev.EventType}
		if filter.UserID == "" {
			k.user = "" // cross-user commercial rollup
		}
		a := bucket[k]
		if a == nil {
			a = &agg{}
			bucket[k] = a
		}
		a.count++
		if ev.Timestamp.After(a.last) {
			a.last = ev.Timestamp
		}
	}

	out := make([]UsageSummary, 0, len(bucket))
	tr := FormatTimeRange(filter.From, filter.To)
	for k, a := range bucket {
		out = append(out, UsageSummary{
			UserID:    k.user,
			Feature:   k.f,
			EventType: k.e,
			Count:     a.count,
			LastUsed:  a.last.UTC(),
			TimeRange: tr,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Feature != out[j].Feature {
			return out[i].Feature < out[j].Feature
		}
		if out[i].EventType != out[j].EventType {
			return out[i].EventType < out[j].EventType
		}
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}
