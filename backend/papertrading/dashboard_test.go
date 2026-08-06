package papertrading_test

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestGetDashboardToday_AndPositions_AndRuns(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05}}}
	_, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true,
	})
	require.NoError(t, err)

	today, err := papertrading.GetDashboardToday(plan.TradeDate)
	require.NoError(t, err)
	require.True(t, today.Enabled)
	require.Equal(t, 1, today.PlanCount)
	require.Equal(t, 1, today.FilledCount)
	require.InDelta(t, 10.05*1000, today.FilledAmount, 1e-6)
	require.Contains(t, today.DataSourceNote, "paper_sim_")

	pos, err := papertrading.GetDashboardPositions()
	require.NoError(t, err)
	require.Len(t, pos.Positions, 1)
	require.Equal(t, int64(1000), pos.Positions[0].TotalVolume)
	require.Equal(t, int64(0), pos.Positions[0].AvailableVolume)
	require.Equal(t, int64(1000), pos.Positions[0].LockedVolume)
	require.True(t, pos.Positions[0].T1Locked)
	require.InDelta(t, 10.05*1000, pos.Positions[0].MarketValue, 1e-6)

	runs, err := papertrading.ListRuns(plan.TradeDate, 50, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, runs.Total, 1)
	require.Equal(t, papertrading.TriggerCron, runs.Runs[0].Trigger)
}

func TestGetDashboardToday_FlagOff(t *testing.T) {
	setupTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})
	today, err := papertrading.GetDashboardToday("2026-07-30")
	require.NoError(t, err)
	require.False(t, today.Enabled)
	require.Equal(t, papertrading.RunStatusSkippedDisabled, today.RunStatus)
}
