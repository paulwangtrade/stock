package usagemetrics

import (
	"context"
	"fmt"

	"go-stock/backend/featuregate"
)

// RecordFeatureEvent records a UI / FeatureGate entry usage event when access is allowed.
// Trading Engine must not call this; Shell / FeatureGate wrappers may.
func RecordFeatureEvent(ctx context.Context, user *featuregate.User, feature featuregate.Feature, eventType EventType, metadata map[string]string) (*UsageEvent, error) {
	if user == nil {
		return nil, fmt.Errorf("usagemetrics: user is required")
	}
	if !featuregate.Allow(user, feature) {
		return nil, fmt.Errorf("usagemetrics: feature %s not allowed for user", feature)
	}
	return Default().Record(ctx, RecordInput{
		UserID:    user.ID,
		Feature:   feature,
		EventType: eventType,
		Metadata:  metadata,
	})
}
