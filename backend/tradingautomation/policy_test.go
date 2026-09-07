package tradingautomation_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tradingautomation"
	"go-stock/backend/tradingwindow"

	"github.com/stretchr/testify/require"
)

func writeTestConfig(t *testing.T, mode string) {
	t.Helper()
	tradingautomation.ResetConfigCache()
	require.NoError(t, tradingautomation.SaveConfig(tradingautomation.Config{
		AutomationMode:  mode,
		MaterializeTime: "09:20",
		ApprovalTime:    "09:25",
		FreezeTime:      "09:29:00",
		FreezeDeadline:  "09:29:30",
	}))
	t.Cleanup(func() {
		tradingautomation.ResetConfigCache()
		_ = os.Remove(filepath.Join("data", "trading_automation.json"))
	})
}

var loc = time.FixedZone("CST", 8*3600)

func at(h, m, s int) time.Time {
	return time.Date(2026, 8, 14, h, m, s, 0, loc)
}

func TestConfig_DefaultManual(t *testing.T) {
	tradingautomation.ResetConfigCache()
	_ = os.Remove(filepath.Join("data", "trading_automation.json"))
	cfg := tradingautomation.GetConfig()
	require.Equal(t, tradingautomation.ModeManual, cfg.AutomationMode)
}

func TestRunMaterializeAutomation_Case1_AutoSuccess(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeAuto)
	tradingautomation.ResetLastSteps()
	orig := tradingautomation.SetMaterializeRunnerForTest(func(_ string, _ time.Time) (bool, string) {
		return true, ""
	})
	t.Cleanup(orig)

	res := tradingautomation.RunMaterializeAutomation("2026-08-14", at(9, 20, 0))
	require.True(t, res.OK)
	require.Equal(t, tradingautomation.OutcomeMaterializationAutoSuccess, res.Outcome)
}

func TestRunApprovalAutomation_Case4_ManualSkipped(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeManual)
	res := tradingautomation.RunApprovalAutomation("2026-08-14", at(9, 25, 0))
	require.Equal(t, tradingautomation.OutcomeAutoApprovalSkipped, res.Outcome)
}

func TestRunMaterializeAutomation_Case4_ManualSkipped(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeManual)
	res := tradingautomation.RunMaterializeAutomation("2026-08-14", at(9, 20, 0))
	require.Equal(t, tradingautomation.OutcomeMaterializationSkipped, res.Outcome)
}

func TestRunApprovalAutomation_Case3_RiskBlocked(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeAuto)
	tradingautomation.SetLookupPlanForTest(func(_ string) (*models.TradePlan, error) {
		return &models.TradePlan{
			ID: 1, TradeDate: "2026-08-14", Status: models.TradePlanStatusDraft,
			PricingPolicyVersion: 1, PricingStage: "morning_materialized",
		}, nil
	})
	t.Cleanup(func() { tradingautomation.ResetLookupPlanForTest() })

	t.Cleanup(tradingautomation.SetApproveRunnerForTest(func(_ uint, _ time.Time) (bool, string) {
		return false, "RISK_OR_READINESS_BLOCKED"
	}))

	res := tradingautomation.RunApprovalAutomation("2026-08-14", at(9, 25, 0))
	require.False(t, res.OK)
	require.Equal(t, tradingautomation.OutcomeAutoApprovalBlocked, res.Outcome)
}

func TestRunFreezeAutomation_Case5_AfterDeadline(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeAuto)
	tradingautomation.SetLookupPlanForTest(func(_ string) (*models.TradePlan, error) {
		at := at(9, 28, 0)
		return &models.TradePlan{
			ID: 2, TradeDate: "2026-08-14", Status: models.TradePlanStatusDraft,
			ApprovedAt: &at, PricingStage: "morning_materialized",
		}, nil
	})
	t.Cleanup(func() { tradingautomation.ResetLookupPlanForTest() })

	res := tradingautomation.RunFreezeAutomation("2026-08-14", at(9, 30, 0))
	require.False(t, res.OK)
	require.Equal(t, tradingautomation.OutcomeAutoFreezeAfterDeadline, res.Outcome)
}

func TestEvaluateMorningAutomation_FrozenPlanReady(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeAuto)
	freeze := at(9, 28, 0)
	plan := &models.TradePlan{
		ID: 3, TradeDate: "2026-08-14", Status: models.TradePlanStatusReady,
		PricingStage: "morning_materialized", FreezeAt: &freeze, ApprovedAt: &freeze,
	}
	obs := tradingautomation.EvaluateMorningAutomation("2026-08-14", plan, nil, nil, nil)
	require.Equal(t, tradingautomation.StepPass, obs.Materialization)
	require.Equal(t, tradingautomation.StepPass, obs.Approval)
	require.Equal(t, tradingautomation.StepPass, obs.Freeze)
}

func TestCase5_MissedOpenWindowAfterLateFreeze(t *testing.T) {
	freeze := at(11, 12, 0)
	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate: "2026-08-14", CurrentTime: freeze,
		PlanStatus: models.TradePlanStatusReady, IsFrozen: true, FrozenTime: &freeze,
	})
	require.Equal(t, tradingwindow.StatusMissedOpenWindow, window.Status)
	require.False(t, tradingwindow.BlocksManualRunExecution(window))
}

func TestRunApprovalAndFreeze_Case2_AutoSuccess(t *testing.T) {
	writeTestConfig(t, tradingautomation.ModeAuto)
	tradingautomation.ResetLastSteps()
	tradingautomation.SetLookupPlanForTest(func(_ string) (*models.TradePlan, error) {
		return &models.TradePlan{
			ID: 4, TradeDate: "2026-08-14", Status: models.TradePlanStatusDraft,
			PricingPolicyVersion: 1, PricingStage: "morning_materialized",
		}, nil
	})
	t.Cleanup(func() { tradingautomation.ResetLookupPlanForTest() })

	t.Cleanup(tradingautomation.SetApproveRunnerForTest(func(planID uint, _ time.Time) (bool, string) {
		require.Equal(t, uint(4), planID)
		return true, ""
	}))
	t.Cleanup(tradingautomation.SetFreezeRunnerForTest(func(planID uint, _ time.Time) (bool, string) {
		require.Equal(t, uint(4), planID)
		return true, ""
	}))

	appr := tradingautomation.RunApprovalAutomation("2026-08-14", at(9, 25, 0))
	require.True(t, appr.OK)
	freeze := tradingautomation.RunFreezeAutomation("2026-08-14", at(9, 29, 0))
	require.True(t, freeze.OK)
	require.Equal(t, tradingautomation.OutcomeAutoFreezeSuccess, freeze.Outcome)
}
