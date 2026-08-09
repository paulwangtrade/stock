package usagemetrics_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/usagemetrics"

	"github.com/stretchr/testify/require"
)

func TestEventCreate(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()

	ev, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID:    "u1",
		Feature:   featuregate.FeatureAdvancedRisk,
		EventType: usagemetrics.EventOpened,
		Metadata:  map[string]string{"source": "ui"},
	})
	require.NoError(t, err)
	require.Equal(t, "u1", ev.UserID)
	require.Equal(t, featuregate.FeatureAdvancedRisk, ev.Feature)
	require.Equal(t, usagemetrics.EventOpened, ev.EventType)
	require.False(t, ev.Timestamp.IsZero())
	require.Equal(t, "ui", ev.Metadata["source"])

	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAdvancedRisk, EventType: usagemetrics.EventExecuted,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAdvancedRisk, EventType: usagemetrics.EventCompleted,
	})
	require.NoError(t, err)
}

func TestQuery(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()
	_, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAdvancedRisk, EventType: usagemetrics.EventOpened,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureBacktest, EventType: usagemetrics.EventExecuted,
	})
	require.NoError(t, err)

	all, err := svc.Query(ctx, usagemetrics.QueryFilter{UserID: "u1", Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 2)

	risk, err := svc.Query(ctx, usagemetrics.QueryFilter{
		UserID: "u1", Feature: featuregate.FeatureAdvancedRisk,
	})
	require.NoError(t, err)
	require.Len(t, risk, 1)
	require.Equal(t, usagemetrics.EventOpened, risk[0].EventType)

	n, err := svc.Count(ctx, usagemetrics.QueryFilter{
		UserID: "u1", EventType: usagemetrics.EventExecuted,
	})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	sum, err := svc.Summarize(ctx, "u1")
	require.NoError(t, err)
	require.Len(t, sum, 2)
}

func TestInvalidFeature(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	_, err := svc.Record(context.Background(), usagemetrics.RecordInput{
		UserID:    "u1",
		Feature:   featuregate.Feature("NotARealFeature"),
		EventType: usagemetrics.EventOpened,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid feature")
}

func TestInvalidEventType(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	_, err := svc.Record(context.Background(), usagemetrics.RecordInput{
		UserID:    "u1",
		Feature:   featuregate.FeatureAdvancedRisk,
		EventType: usagemetrics.EventType("clicked"),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid event_type")
}

func TestSanitize_DropsHoldingsAndPassword(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ev, err := svc.Record(context.Background(), usagemetrics.RecordInput{
		UserID:    "u1",
		Feature:   featuregate.FeatureAIAnalysis,
		EventType: usagemetrics.EventOpened,
		Metadata: map[string]string{
			"source":   "ui",
			"holding":  "600000",
			"password": "secret",
			"email":    "a@b.c",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "ui", ev.Metadata["source"])
	_, ok := ev.Metadata["holding"]
	require.False(t, ok)
	_, ok = ev.Metadata["password"]
	require.False(t, ok)
	_, ok = ev.Metadata["email"]
	require.False(t, ok)
}

func TestRecordFeatureEvent_RequiresGateAllow(t *testing.T) {
	entSvc := entitlement.NewService(entitlement.NewMemoryStore())
	entitlement.SetDefaultForTest(entSvc)
	t.Cleanup(func() { entitlement.SetDefaultForTest(nil) })

	user := &featuregate.User{ID: "gate-u", Tier: featuregate.TierPro}
	_, err := usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedRisk, usagemetrics.EventOpened, nil)
	require.Error(t, err) // no entitlement rows yet

	require.NoError(t, entSvc.EnsureTierDefaults(user))
	ev, err := usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedRisk, usagemetrics.EventOpened, map[string]string{"source": "gate"})
	require.NoError(t, err)
	require.Equal(t, featuregate.FeatureAdvancedRisk, ev.Feature)
	require.Equal(t, usagemetrics.EventOpened, ev.EventType)
}

func TestRecord_CustomTimestamp(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ts := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	ev, err := svc.Record(context.Background(), usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureBacktest, EventType: usagemetrics.EventCompleted, Timestamp: ts,
	})
	require.NoError(t, err)
	require.True(t, ev.Timestamp.Equal(ts))
}
