package usagemetrics_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/featuregate"
	"go-stock/backend/usagemetrics"

	"github.com/stretchr/testify/require"
)

func TestAnalytics_FeatureUsageCount_TimeRange(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()
	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)

	_, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAdvancedRisk, EventType: usagemetrics.EventOpened, Timestamp: t1,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u2", Feature: featuregate.FeatureAdvancedRisk, EventType: usagemetrics.EventViewed, Timestamp: t2,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventOpened, Timestamp: t3,
	})
	require.NoError(t, err)

	n, err := svc.FeatureUsageCount(ctx, usagemetrics.AnalyticsFilter{
		Feature: featuregate.FeatureAdvancedRisk,
		From:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, 2, n)

	nOpened, err := svc.FeatureUsageCount(ctx, usagemetrics.AnalyticsFilter{
		Feature:   featuregate.FeatureAdvancedRisk,
		EventType: usagemetrics.EventOpened,
	})
	require.NoError(t, err)
	require.Equal(t, 1, nOpened)
}

func TestAnalytics_UserFeatureSummary_LastUsed(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()
	early := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	late := time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC)

	_, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "shell:pro", Feature: featuregate.FeatureAdvancedObservation,
		EventType: usagemetrics.EventOpened, Timestamp: early,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "shell:pro", Feature: featuregate.FeatureAdvancedObservation,
		EventType: usagemetrics.EventOpened, Timestamp: late,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "shell:pro", Feature: featuregate.FeatureAdvancedObservation,
		EventType: usagemetrics.EventViewed, Timestamp: late,
	})
	require.NoError(t, err)
	// Non-commercial feature should be excluded from UserFeatureSummary.
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "shell:pro", Feature: featuregate.FeatureRealtimeSignal,
		EventType: usagemetrics.EventOpened, Timestamp: late,
	})
	require.NoError(t, err)

	sum, err := svc.UserFeatureSummary(ctx, "shell:pro", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, sum, 2)

	var opened *usagemetrics.UsageSummary
	for i := range sum {
		if sum[i].EventType == usagemetrics.EventOpened {
			opened = &sum[i]
		}
		require.True(t, featuregate.IsCommercialFeature(sum[i].Feature))
	}
	require.NotNil(t, opened)
	require.Equal(t, 2, opened.Count)
	require.True(t, opened.LastUsed.Equal(late))
	require.Equal(t, "shell:pro", opened.UserID)
}

func TestAnalytics_SummarizeAnalytics_CommercialRollup(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()
	ts := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

	for _, uid := range []string{"a", "b"} {
		_, err := svc.Record(ctx, usagemetrics.RecordInput{
			UserID: uid, Feature: featuregate.FeatureBacktest,
			EventType: usagemetrics.EventOpened, Timestamp: ts,
		})
		require.NoError(t, err)
	}
	_, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "a", Feature: featuregate.FeatureMultiAccount,
		EventType: usagemetrics.EventOpened, Timestamp: ts,
	})
	require.NoError(t, err)

	rows, err := svc.SummarizeAnalytics(ctx, usagemetrics.AnalyticsFilter{CommercialOnly: true})
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	var backtest, multi *usagemetrics.UsageSummary
	for i := range rows {
		require.Empty(t, rows[i].UserID, "cross-user rollup clears user_id")
		if rows[i].Feature == featuregate.FeatureBacktest && rows[i].EventType == usagemetrics.EventOpened {
			backtest = &rows[i]
		}
		if rows[i].Feature == featuregate.FeatureMultiAccount {
			multi = &rows[i]
		}
	}
	require.NotNil(t, backtest)
	require.Equal(t, 2, backtest.Count)
	require.NotNil(t, multi)
	require.Equal(t, 1, multi.Count)
}

func TestAnalytics_TimeRangeQuery_ExcludesOutOfRange(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ctx := context.Background()
	in := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventOpened, Timestamp: in,
	})
	require.NoError(t, err)
	_, err = svc.Record(ctx, usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventOpened, Timestamp: out,
	})
	require.NoError(t, err)

	sum, err := svc.UserFeatureSummary(ctx, "u1",
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.Len(t, sum, 1)
	require.Equal(t, 1, sum[0].Count)
}

func TestCommercialFeatures_Catalog(t *testing.T) {
	feats := featuregate.CommercialFeatures()
	require.Contains(t, feats, featuregate.FeatureAIAnalysis)
	require.Contains(t, feats, featuregate.FeatureAdvancedRisk)
	require.Contains(t, feats, featuregate.FeatureAdvancedObservation)
	require.Contains(t, feats, featuregate.FeatureBacktest)
	require.Contains(t, feats, featuregate.FeatureMultiAccount)
	require.True(t, featuregate.IsKnown(featuregate.FeatureMultiAccount))
}
