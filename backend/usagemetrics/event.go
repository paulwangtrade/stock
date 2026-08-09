// Package usagemetrics records Shell / FeatureGate feature usage for commercial insight.
//
// C4: local UsageEvent capture + query. Integration: FeatureGate / UI entry only.
// Forbidden: trading-core instrumentation, cloud sync, holdings, passwords, PII.
package usagemetrics

import (
	"time"

	"go-stock/backend/featuregate"
)

// EventType is a feature-usage lifecycle verb (Shell / UI).
type EventType string

const (
	EventOpened           EventType = "opened"
	EventViewed           EventType = "viewed" // Phase13-D product UI panel content viewed
	EventExecuted         EventType = "executed"
	EventCompleted        EventType = "completed"
	EventContextGenerated EventType = "context_generated" // Phase13-C AI Assistant Context Builder
)

// IsKnownEventType reports whether event_type is in the C4 catalog.
func IsKnownEventType(t EventType) bool {
	switch t {
	case EventOpened, EventViewed, EventExecuted, EventCompleted, EventContextGenerated:
		return true
	default:
		return false
	}
}

// KnownEventTypes returns the C4 event_type catalog.
func KnownEventTypes() []EventType {
	return []EventType{EventOpened, EventViewed, EventExecuted, EventCompleted, EventContextGenerated}
}

// UsageEvent is one local product-usage observation.
type UsageEvent struct {
	ID        string              `json:"id,omitempty"`
	UserID    string              `json:"user_id"`
	Feature   featuregate.Feature `json:"feature"`
	EventType EventType           `json:"event_type"`
	Timestamp time.Time           `json:"timestamp"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

// QueryFilter narrows List / Count / Analytics queries.
type QueryFilter struct {
	UserID    string
	Feature   featuregate.Feature // empty = any
	EventType EventType           // empty = any
	From      time.Time           // zero = no lower bound (inclusive)
	To        time.Time           // zero = no upper bound (exclusive if set)
	Limit     int
	// AllowEmptyUser permits Query/Count without user_id (commercial aggregate only).
	AllowEmptyUser bool
}

// UsageSummary is the Phase13-E commercial analytics read-model row.
type UsageSummary struct {
	UserID    string              `json:"user_id,omitempty"`
	Feature   featuregate.Feature `json:"feature"`
	EventType EventType           `json:"event_type"`
	Count     int                 `json:"count"`
	LastUsed  time.Time           `json:"last_used"`
	TimeRange TimeRange           `json:"time_range"`
}

// TimeRange describes the query window applied to a UsageSummary row.
type TimeRange struct {
	From string `json:"from,omitempty"` // RFC3339 or empty = unbounded
	To   string `json:"to,omitempty"`   // RFC3339 exclusive upper bound; empty = unbounded
}

// CountSummary is kept as an alias shape for C4 callers; prefer UsageSummary.
type CountSummary = UsageSummary

// FormatTimeRange builds a TimeRange JSON object from filter bounds.
func FormatTimeRange(from, to time.Time) TimeRange {
	tr := TimeRange{}
	if !from.IsZero() {
		tr.From = from.UTC().Format(time.RFC3339)
	}
	if !to.IsZero() {
		tr.To = to.UTC().Format(time.RFC3339)
	}
	return tr
}