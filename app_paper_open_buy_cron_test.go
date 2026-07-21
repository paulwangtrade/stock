package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaperOpenBuyCronTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestInitPaperOpenBuyJobs_RegistersReconcileCron(t *testing.T) {
	setupPaperOpenBuyCronTestDB(t)
	require.NoError(t, runSchemaMigrations())
	a := NewApp()
	t.Cleanup(func() { a.cron.Stop() })
	a.InitPaperOpenBuyJobs()

	keys := []string{
		"paper_daily_plan",
		"paper_open_prepare",
		"paper_open_buy",
		"paper_trade_plan_reconcile",
		"paper_trade_plan_reconcile_am",
		"paper_trade_plan_reconcile_pm",
	}
	for _, key := range keys {
		_, exists := a.getCronEntry(key)
		require.True(t, exists, "cron key %s should be registered", key)
	}
}

func TestInitPaperOpenBuyJobs_ReconcileCronIdempotent(t *testing.T) {
	setupPaperOpenBuyCronTestDB(t)
	require.NoError(t, runSchemaMigrations())
	a := NewApp()
	t.Cleanup(func() { a.cron.Stop() })
	a.InitPaperOpenBuyJobs()
	before := len(a.cron.Entries())
	a.InitPaperOpenBuyJobs()
	after := len(a.cron.Entries())
	require.Equal(t, before, after, "second InitPaperOpenBuyJobs should not add duplicate cron entries")
}

func TestInitPaperOpenBuyJobs_DailyPlanCronSpec920(t *testing.T) {
	require.Equal(t, "0 20 9 * * 1-5", paperDailyPlanCronSpec)

	setupPaperOpenBuyCronTestDB(t)
	require.NoError(t, runSchemaMigrations())
	a := NewApp()
	t.Cleanup(func() { a.cron.Stop() })
	a.InitPaperOpenBuyJobs()

	id, exists := a.getCronEntry("paper_daily_plan")
	require.True(t, exists)
	entry := a.cron.Entry(id)
	require.False(t, entry.Next.IsZero(), "9:20 entry should have a next schedule time")

	b, err := os.ReadFile(filepath.Join("app_paper_open_buy.go"))
	require.NoError(t, err)
	src := string(b)
	require.Contains(t, src, `paperDailyPlanCronSpec`)
	require.Contains(t, src, `RunMorningPlanPreparation`)
	require.NotContains(t, src, `strategy.RunDailyCandidateAndPlan("")`)
}

func TestRunPaperDailyPlanCronJob_AdoptFrozenMode(t *testing.T) {
	prev := morningPlanPreparationFn
	t.Cleanup(func() { morningPlanPreparationFn = prev })
	morningPlanPreparationFn = func(string) (*models.CandidatePool, *models.TradePlan, string, error) {
		return nil, &models.TradePlan{ID: 1, Status: models.TradePlanStatusReady}, strategy.MorningPlanModeAdoptFrozen, nil
	}
	mode, err := runPaperDailyPlanCronJob()
	require.NoError(t, err)
	require.Equal(t, strategy.MorningPlanModeAdoptFrozen, mode)
}

func TestRunPaperDailyPlanCronJob_BuildMorningMode(t *testing.T) {
	prev := morningPlanPreparationFn
	t.Cleanup(func() { morningPlanPreparationFn = prev })
	morningPlanPreparationFn = func(string) (*models.CandidatePool, *models.TradePlan, string, error) {
		return &models.CandidatePool{ID: 2}, &models.TradePlan{ID: 3, Status: models.TradePlanStatusReady}, strategy.MorningPlanModeBuildMorning, nil
	}
	mode, err := runPaperDailyPlanCronJob()
	require.NoError(t, err)
	require.Equal(t, strategy.MorningPlanModeBuildMorning, mode)
}

func TestPaperDailyPlanCron_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("app_paper_open_buy.go"))
	require.NoError(t, err)
	src := string(b)
	// 9:20 body must go through morning preparation, not legacy daily directly.
	require.Contains(t, src, "runPaperDailyPlanCronJob")
	require.Contains(t, src, "RunMorningPlanPreparation")
	require.True(t, strings.Contains(src, "TradingPreflightCheck"), "preflight gate must remain")
	require.Contains(t, src, `0 25 9 * * 1-5`)
	require.Contains(t, src, `0 30 9 * * 1-5`)
}
