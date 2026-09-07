package diagnostic_test

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/diagnostic"

	"github.com/stretchr/testify/require"
)

func TestSanitizeTrading_DropsCashAndIDs(t *testing.T) {
	st := data.DailyTradingStatus{
		TradeDate:          "2026-08-22",
		IsWeekday:          true,
		EnablePaperOpenBuy: true,
		Candidate: data.DailyCandidateStatus{
			PoolID:  99,
			Status:  "READY",
			Count:   3,
			Source:  "morning_scan",
			Message: "600519 secret pool",
		},
		Plan: data.DailyPlanStatus{
			PlanID:       42,
			Status:       "ready",
			ItemCount:    2,
			Message:      "plan for 000001",
			ExecutingSince: "2026-08-22T09:30:00Z",
		},
		Risk: data.DailyRiskStatus{
			Status:  "pass",
			Summary: "accepted AAPL",
		},
		Execution: data.DailyExecutionStatus{
			Phase:   "ready",
			Ready:   true,
			Message: "executor path D:\\Users\\alice",
		},
		Paper: data.DailyPaperStatus{
			HasAccount:       true,
			AccountID:        1001,
			AccountCash:      123456.78,
			OrderCount:       1,
			FillCount:        0,
			FilledOrderCount: 0,
		},
		BlockReasons: []string{"candidate_pool_empty", "unknown_free_text", "trade_plan_missing"},
		Message:      "600519 blocked",
	}

	out := diagnostic.SanitizeTradingStatus(st)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	blob := strings.ToLower(string(raw))

	require.Equal(t, "2026-08-22", out.TradeDate)
	require.True(t, out.Paper.HasAccount)
	require.Equal(t, 1, out.Paper.OrderCount)
	require.NotContains(t, blob, "accountid")
	require.NotContains(t, blob, "account_cash")
	require.NotContains(t, blob, "accountcash")
	require.NotContains(t, blob, "600519")
	require.NotContains(t, blob, "000001")
	require.NotContains(t, blob, "poolid")
	require.NotContains(t, blob, "planid")
	require.Contains(t, out.BlockReasons, "candidate_pool_empty")
	require.Contains(t, out.BlockReasons, "trade_plan_missing")
	require.NotContains(t, out.BlockReasons, "unknown_free_text")
}

func TestProviderMode_NoWhitelistIDs(t *testing.T) {
	out := diagnostic.SnapshotProviderMode("controlled", "fixed_amount")
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	blob := string(raw)

	require.NotEmpty(t, out.Adoption)
	require.Equal(t, "controlled", out.LastPlanProviderMode)
	require.Equal(t, "fixed_amount", out.LastPlanDecisionProvider)
	require.NotContains(t, blob, "account_ids")
	require.NotContains(t, blob, "strategy_names")
}

func TestFrontendError_DropsStackAndAbsPath(t *testing.T) {
	t.Cleanup(diagnostic.ResetErrorRingForTest)
	diagnostic.ResetErrorRingForTest()

	diagnostic.RecordFrontendError(
		"App.vue",
		`fail at D:\Users\alice\go-stock\frontend\src\App.vue during render`,
		12,
	)

	report := diagnostic.CollectReport("test")
	require.NotEmpty(t, report.Errors)
	found := false
	for _, e := range report.Errors {
		if e.Source != diagnostic.SourceFrontend {
			continue
		}
		found = true
		require.NotContains(t, e.MessageSafe, "sk-live")
		require.NotContains(t, e.MessageSafe, `D:\Users\alice`)
		require.Equal(t, "App.vue", e.Page)
	}
	require.True(t, found)
}

func TestCollect_Diag2_NoSecrets(t *testing.T) {
	t.Cleanup(diagnostic.ResetLastErrorForTest)
	t.Cleanup(diagnostic.ResetErrorRingForTest)
	diagnostic.ResetLastErrorForTest()
	diagnostic.ResetErrorRingForTest()

	diagnostic.RecordError(diagnostic.CategoryConfig, "CFG_BAD", "bad config api_key=sk-test")
	diagnostic.RecordFrontendError("settings.vue", "render failed", 0)

	report := diagnostic.CollectReport("about_export")
	require.Equal(t, diagnostic.SchemaVersion, report.SchemaVersion)
	require.NotEmpty(t, report.Build.GOOS)
	require.NotEmpty(t, report.Migration.Status)
	require.NotNil(t, report.TradingStatus.TradeDate)
	require.NotEmpty(t, report.ProviderMode.Adoption)
	require.NotEmpty(t, report.ErrorSummary.Items)

	raw, err := json.Marshal(report)
	require.NoError(t, err)
	blob := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"sk-test", "account_cash", "accountid",
		"api_key=sk", "bearer ",
	} {
		require.NotContains(t, blob, forbidden)
	}
}
