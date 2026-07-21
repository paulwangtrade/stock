package main

import (
	"fmt"
	"testing"

	"go-stock/backend/db"

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
