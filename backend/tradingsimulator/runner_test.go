package tradingsimulator_test

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingsimulator"
	"go-stock/backend/tradingwindow"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSimDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:sim_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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
	require.NoError(t, data.EnsureTradePlanTables())
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
}

func TestSimulation_Normal(t *testing.T) {
	setupSimDB(t)
	res, err := tradingsimulator.RunTradingDaySimulation(tradingsimulator.SimulationRequest{
		Date: "2026-08-17", AutomationMode: "AUTO", Scenario: tradingsimulator.ScenarioNormal,
	})
	require.NoError(t, err)
	require.Equal(t, tradingsimulator.StatusPass, res.Overall, res.FailureReason)
	require.Equal(t, tradingsimulator.StatusPass, res.Morning.Status)
	require.Equal(t, tradingsimulator.StatusPass, res.Approve.Status)
	require.Equal(t, tradingsimulator.StatusPass, res.Freeze.Status)
	require.NotEqual(t, tradingsimulator.StatusFail, res.Execution.Status)
}

func TestSimulation_NoPlan(t *testing.T) {
	setupSimDB(t)
	res, err := tradingsimulator.RunTradingDaySimulation(tradingsimulator.SimulationRequest{
		Date: "2026-08-17", AutomationMode: "AUTO", Scenario: tradingsimulator.ScenarioNoPlan,
	})
	require.NoError(t, err)
	require.Equal(t, tradingsimulator.StatusPass, res.Overall, res.FailureReason)
	require.True(t, hasEvent(res, tradingsimulator.EventExecutionSkipped))
}

func TestSimulation_LateFreeze(t *testing.T) {
	setupSimDB(t)
	res, err := tradingsimulator.RunTradingDaySimulation(tradingsimulator.SimulationRequest{
		Date: "2026-08-17", AutomationMode: "AUTO", Scenario: tradingsimulator.ScenarioLateFreeze,
	})
	require.NoError(t, err)
	require.Equal(t, tradingsimulator.StatusPass, res.Overall, res.FailureReason)
	require.Equal(t, string(tradingwindow.StatusMissedOpenWindow), res.WindowStatus)
	require.Equal(t, tradingwindow.ReasonPlanFrozenAfterDeadline, res.WindowReason)
	require.NotEqual(t, tradingsimulator.StatusFail, res.Execution.Status, "manual RunExecution must remain allowed")
	require.False(t, tradingwindow.BlocksManualRunExecution(tradingwindow.PlanWindowResult{
		Status: tradingwindow.StatusMissedOpenWindow,
	}))
}

func TestSimulation_RiskBlock(t *testing.T) {
	setupSimDB(t)
	res, err := tradingsimulator.RunTradingDaySimulation(tradingsimulator.SimulationRequest{
		Date: "2026-08-17", AutomationMode: "AUTO", Scenario: tradingsimulator.ScenarioRiskBlock,
	})
	require.NoError(t, err)
	require.Equal(t, tradingsimulator.StatusPass, res.Overall, res.FailureReason)
	require.True(t, hasEvent(res, tradingsimulator.EventAutoApprovalBlocked))
}

func TestSimulation_ManualMode(t *testing.T) {
	setupSimDB(t)
	res, err := tradingsimulator.RunTradingDaySimulation(tradingsimulator.SimulationRequest{
		Date: "2026-08-17", AutomationMode: "MANUAL", Scenario: tradingsimulator.ScenarioManualMode,
	})
	require.NoError(t, err)
	require.Equal(t, tradingsimulator.StatusPass, res.Overall, res.FailureReason)
	require.Equal(t, "MANUAL", res.Mode)
	require.True(t, hasEvent(res, tradingsimulator.EventMaterializationSkipped) || hasEvent(res, tradingsimulator.EventAutoApprovalSkipped))
}

func TestClock_AdvanceForwardOnly(t *testing.T) {
	c := tradingsimulator.NewClock("2026-08-17", "09:25:00", time.FixedZone("CST", 8*3600))
	c.AdvanceTo("09:20:00")
	require.Equal(t, 9, c.Now().Hour())
	require.Equal(t, 25, c.Now().Minute())
	c.AdvanceTo("11:12:00")
	require.Equal(t, 11, c.Now().Hour())
	require.Equal(t, 12, c.Now().Minute())
}

func hasEvent(res *tradingsimulator.SimulationResult, name string) bool {
	for _, e := range res.Timeline {
		if e.Event == name {
			return true
		}
	}
	return false
}
