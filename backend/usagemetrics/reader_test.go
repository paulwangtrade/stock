package usagemetrics_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/featuregate"
	"go-stock/backend/usagemetrics"

	"github.com/stretchr/testify/require"
)

func seedCommercialEvents(t *testing.T, svc usagemetrics.Service) {
	t.Helper()
	ctx := context.Background()
	rows := []struct {
		user string
		feat featuregate.Feature
		ev   usagemetrics.EventType
		ts   time.Time
	}{
		{"u1", featuregate.FeatureAIAnalysis, usagemetrics.EventOpened, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)},
		{"u1", featuregate.FeatureAIAnalysis, usagemetrics.EventOpened, time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)},
		{"u1", featuregate.FeatureAIAnalysis, usagemetrics.EventViewed, time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)},
		{"u2", featuregate.FeatureAdvancedRisk, usagemetrics.EventOpened, time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)},
		{"u2", featuregate.FeatureAdvancedObservation, usagemetrics.EventOpened, time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)},
		{"u3", featuregate.FeatureBacktest, usagemetrics.EventExecuted, time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)},
		{"u3", featuregate.FeatureMultiAccount, usagemetrics.EventOpened, time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)},
	}
	for _, r := range rows {
		_, err := svc.Record(ctx, usagemetrics.RecordInput{
			UserID: r.user, Feature: r.feat, EventType: r.ev, Timestamp: r.ts,
		})
		require.NoError(t, err)
	}
}

func TestAnalyticsReader_EventAggregation(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	seedCommercialEvents(t, svc)
	reader := usagemetrics.NewAnalyticsReader(svc)
	ctx := context.Background()

	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	sum, err := reader.CommercialSummary(ctx, usagemetrics.AnalyticsFilter{From: from, To: to})
	require.NoError(t, err)
	require.NotEmpty(t, sum)

	byKey := map[string]usagemetrics.UsageSummary{}
	for _, row := range sum {
		require.NotEmpty(t, row.Feature)
		require.NotEmpty(t, row.EventType)
		require.Greater(t, row.Count, 0)
		require.False(t, row.LastUsed.IsZero())
		require.Equal(t, from.UTC().Format(time.RFC3339), row.TimeRange.From)
		require.Equal(t, to.UTC().Format(time.RFC3339), row.TimeRange.To)
		require.True(t, featuregate.IsCommercialFeature(row.Feature))
		byKey[string(row.Feature)+"|"+string(row.EventType)] = row
	}

	aiOpened := byKey["AIAnalysis|opened"]
	require.Equal(t, 2, aiOpened.Count)
	require.True(t, aiOpened.LastUsed.Equal(time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)))

	n, err := reader.FeatureCount(ctx, usagemetrics.AnalyticsFilter{
		Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventOpened, From: from, To: to,
	})
	require.NoError(t, err)
	require.Equal(t, 2, n)

	userSum, err := reader.UserSummary(ctx, "u1", from, to)
	require.NoError(t, err)
	require.Len(t, userSum, 2) // opened + viewed
	for _, row := range userSum {
		require.Equal(t, "u1", row.UserID)
		require.Equal(t, featuregate.FeatureAIAnalysis, row.Feature)
		require.Equal(t, from.UTC().Format(time.RFC3339), row.TimeRange.From)
	}
}

func TestAnalyticsReader_EmptyData(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	reader := usagemetrics.NewAnalyticsReader(svc)
	ctx := context.Background()

	sum, err := reader.CommercialSummary(ctx, usagemetrics.AnalyticsFilter{})
	require.NoError(t, err)
	require.Empty(t, sum)

	userSum, err := reader.UserSummary(ctx, "nobody", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Empty(t, userSum)

	n, err := reader.FeatureCount(ctx, usagemetrics.AnalyticsFilter{Feature: featuregate.FeatureBacktest})
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestAnalyticsReader_InvalidFeature(t *testing.T) {
	reader := usagemetrics.NewAnalyticsReader(usagemetrics.NewService(usagemetrics.NewMemoryStore()))
	ctx := context.Background()

	_, err := reader.FeatureCount(ctx, usagemetrics.AnalyticsFilter{Feature: featuregate.Feature("NotARealFeature")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid feature")

	_, err = reader.FeatureCount(ctx, usagemetrics.AnalyticsFilter{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "feature is required")

	_, err = reader.CommercialSummary(ctx, usagemetrics.AnalyticsFilter{Feature: featuregate.Feature("Nope")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid feature")

	// Known but non-commercial feature rejected on commercial summary filter.
	_, err = reader.CommercialSummary(ctx, usagemetrics.AnalyticsFilter{Feature: featuregate.FeatureRealtimeSignal})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in commercial catalog")
}

func TestAnalyticsReader_TimeRangeFilter(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	seedCommercialEvents(t, svc)
	reader := usagemetrics.NewAnalyticsReader(svc)
	ctx := context.Background()

	from := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) // exclusive → includes Aug3–4 only

	sum, err := reader.CommercialSummary(ctx, usagemetrics.AnalyticsFilter{From: from, To: to})
	require.NoError(t, err)

	total := 0
	for _, row := range sum {
		total += row.Count
		require.False(t, row.LastUsed.Before(from))
		require.True(t, row.LastUsed.Before(to))
		require.Equal(t, from.Format(time.RFC3339), row.TimeRange.From)
		require.Equal(t, to.Format(time.RFC3339), row.TimeRange.To)
	}
	// AdvancedRisk opened (Aug3) + AdvancedObservation opened (Aug4)
	require.Equal(t, 2, total)

	n, err := reader.FeatureCount(ctx, usagemetrics.AnalyticsFilter{
		Feature: featuregate.FeatureAIAnalysis, From: from, To: to,
	})
	require.NoError(t, err)
	require.Equal(t, 0, n, "AIAnalysis events are before Aug3")
}

func TestAnalyticsReader_Catalog(t *testing.T) {
	reader := usagemetrics.NewAnalyticsReader(nil)
	cat := reader.Catalog()
	require.Equal(t, featuregate.CommercialFeatures(), cat)
	require.Contains(t, cat, featuregate.FeatureAIAnalysis)
	require.Contains(t, cat, featuregate.FeatureAdvancedRisk)
	require.Contains(t, cat, featuregate.FeatureAdvancedObservation)
	require.Contains(t, cat, featuregate.FeatureBacktest)
	require.Contains(t, cat, featuregate.FeatureMultiAccount)
}

func TestSanitize_DropsStockCode(t *testing.T) {
	svc := usagemetrics.NewService(usagemetrics.NewMemoryStore())
	ev, err := svc.Record(context.Background(), usagemetrics.RecordInput{
		UserID: "u1", Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventOpened,
		Metadata: map[string]string{"source": "ui", "stock_code": "600000", "symbol": "SH600000"},
	})
	require.NoError(t, err)
	require.Equal(t, "ui", ev.Metadata["source"])
	_, ok := ev.Metadata["stock_code"]
	require.False(t, ok)
	_, ok = ev.Metadata["symbol"]
	require.False(t, ok)
}
