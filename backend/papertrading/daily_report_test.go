package papertrading_test

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestGenerateDailyReport_Idempotent_AfterSettlement(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05}}}
	_, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true, Now: sessionANow(),
	})
	require.NoError(t, err)

	settle, err := papertrading.SettlementJob(plan.TradeDate, price, false)
	require.NoError(t, err)
	require.True(t, settle.Enabled)

	// Settlement already generates the report; second generate must skip.
	r2, err := papertrading.GenerateDailyReport(plan.TradeDate)
	require.NoError(t, err)
	require.True(t, r2.Skipped)
	require.False(t, r2.Generated)

	var count int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimDailyReport{}).
		Where("report_date = ?", plan.TradeDate).Count(&count).Error)
	require.Equal(t, int64(1), count)

	var posCount int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimDailyPosition{}).
		Where("report_date = ?", plan.TradeDate).Count(&posCount).Error)
	require.Equal(t, int64(1), posCount)

	rows, total, err := papertrading.ListDailyReports("2026-07-01", "2026-07-31", 60, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, plan.TradeDate, rows[0].ReportDate)
	require.Equal(t, 1, rows[0].FilledCount)
	require.InDelta(t, 10.05*1000, rows[0].Turnover, 1e-6)
	require.Greater(t, rows[0].MaxGrossExposurePct, 0.0)
}

func TestGenerateDailyReport_FlagOff(t *testing.T) {
	setupTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})
	r, err := papertrading.GenerateDailyReport("2026-07-30")
	require.NoError(t, err)
	require.True(t, r.Skipped)
}
