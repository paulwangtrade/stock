package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAfterCloseCronTestDB(t *testing.T) {
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

func withTempAfterCloseConfig(t *testing.T, enabled bool) {
	t.Helper()
	dir := t.TempDir()
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		data.ResetAfterClosePlanConfigCache()
	})
	data.ResetAfterClosePlanConfigCache()
	require.NoError(t, data.SaveAfterClosePlanConfig(data.AfterClosePlanConfig{
		AfterClosePlanEnabled: enabled,
	}))
}

func TestInitAfterClosePlanJobs_RegistersCron(t *testing.T) {
	setupAfterCloseCronTestDB(t)
	require.NoError(t, runSchemaMigrations())
	a := NewApp()
	t.Cleanup(func() { a.cron.Stop() })

	a.InitAfterClosePlanJobs()
	_, exists := a.getCronEntry(afterClosePlanCronKey)
	require.True(t, exists, "cron key %s should be registered", afterClosePlanCronKey)

	before := len(a.cron.Entries())
	a.InitAfterClosePlanJobs()
	after := len(a.cron.Entries())
	require.Equal(t, before, after, "second InitAfterClosePlanJobs should not duplicate cron entries")
}

func TestRunAfterClosePlanWorkflowJob_DisabledDoesNotRun(t *testing.T) {
	withTempAfterCloseConfig(t, false)

	var calls atomic.Int32
	prev := afterCloseWorkflowRunner
	afterCloseWorkflowRunner = func(sourceDate string) (*strategy.AfterCloseWorkflowResult, error) {
		calls.Add(1)
		return &strategy.AfterCloseWorkflowResult{OK: true, RiskPassed: true}, nil
	}
	t.Cleanup(func() { afterCloseWorkflowRunner = prev })

	runAfterClosePlanWorkflowJob()
	require.Equal(t, int32(0), calls.Load())
}

func TestRunAfterClosePlanWorkflowJob_EnabledCallsWorkflow(t *testing.T) {
	withTempAfterCloseConfig(t, true)

	var calls atomic.Int32
	var gotSource string
	prev := afterCloseWorkflowRunner
	afterCloseWorkflowRunner = func(sourceDate string) (*strategy.AfterCloseWorkflowResult, error) {
		calls.Add(1)
		gotSource = sourceDate
		return &strategy.AfterCloseWorkflowResult{
			OK:              true,
			SourceDate:      "2026-07-24",
			TradeDate:       "2026-07-27",
			CandidatePoolID: 11,
			TradePlanID:     21,
			PlanVersion:     1,
			RiskPassed:      true,
		}, nil
	}
	t.Cleanup(func() { afterCloseWorkflowRunner = prev })

	runAfterClosePlanWorkflowJob()
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, "", gotSource)
}

func TestAfterCloseCron_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("app_after_close_workflow.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"ApproveTradePlan(",
		"FreezeTradePlan(",
		"RunDailyCandidateAndPlan(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunPaperOpenPrepare(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("after-close cron must not reference %s", token)
		}
	}
	require.Contains(t, src, "RunAfterClosePlanWorkflow")
	require.Contains(t, src, afterClosePlanCronSpec)
	require.Contains(t, src, "TradingPreflightCheck")
}
