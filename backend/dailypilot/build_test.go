package dailypilot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/controlledmonitor"
	"go-stock/backend/dailypilot"
	"go-stock/backend/portfoliohistory"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/providershadow"

	"github.com/stretchr/testify/require"
)

func TestBuildDailyPilotReport_DefaultOffSkipped(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		TradeDate: "2026-08-22",
	})
	require.True(t, rep.Skipped)
	require.False(t, rep.Enabled)
	require.Equal(t, dailypilot.SchemaVersion, rep.SchemaVersion)
	require.Equal(t, verdictUnset(t), rep.HumanVerdict.Status)
	require.False(t, rep.HumanVerdict.NextDateApproved)
}

func TestBuildDailyPilotReport_AllSourcesMissingStillEmits(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
	})
	require.False(t, rep.Skipped)
	require.Contains(t, rep.DataGaps, "shadow_records_missing")
	require.Contains(t, rep.DataGaps, "validation_missing")
	require.Contains(t, rep.DataGaps, "controlled_monitor_missing")
	require.Contains(t, rep.DataGaps, "portfolio_history_missing")
	require.Equal(t, 0, rep.LegacyVsPortfolio.ComparableCount)
	require.Equal(t, 0, rep.LegacyVsPortfolio.Amount.SampleCount)
	require.Contains(t, rep.KnownGaps, "write_chain_may_omit_tighten")
}

func TestBuildDailyPilotReport_ScopeExpandedNoPlaintextIDs(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
		PolicySnapshotClose: dailypilot.PolicySnapshotInput{
			Adoption:      "controlled",
			AccountIDs:    []uint{1001, 1002},
			StrategyNames: []string{"alpha", "beta"},
			TradeDates:    []string{"2026-08-22", "2026-08-23", "2026-08-24"},
		},
	})
	require.True(t, rep.Scope.ScopeExpanded)
	require.Equal(t, 2, rep.Scope.AccountWhitelistCount)
	require.NotEmpty(t, rep.Scope.AccountIDHash8)
	require.NotContains(t, string(mustJSON(t, rep)), "1001")
	require.NotContains(t, string(mustJSON(t, rep)), `"alpha"`)
	require.Contains(t, rep.Failure.HumanWatchCodes, "scope_expanded")
}

func TestBuildDailyPilotReport_ValidationSkippedNoFakeZeroDelta(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
		Validation: &portfoliovalidation.PortfolioValidationReport{
			Enabled: true,
			Skipped: true,
		},
	})
	require.True(t, rep.LegacyVsPortfolio.ValidationOverlay.Skipped)
	require.True(t, rep.RiskConstraint.Validation.Skipped)
	raw := string(mustJSON(t, rep))
	require.NotContains(t, raw, `"notional_delta":0`)
}

func TestBuildDailyPilotReport_ShadowFailureDoesNotSwitchProvider(t *testing.T) {
	rec := providershadow.ShadowComparisonRecord{
		TradeDate:  "2026-08-22",
		Comparable: false,
		Failure: providershadow.ShadowFailure{
			PortfolioFailed: true,
			ErrorReason:     providershadow.ErrRuntimeTimeout,
		},
		IncomparableReason: providershadow.IncomparablePortfolioFailed,
	}
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:       true,
		TradeDate:     "2026-08-22",
		ShadowRecords: []providershadow.ShadowComparisonRecord{rec, rec},
	})
	require.Equal(t, 2, rep.Failure.Shadow.PortfolioFailedCount)
	require.Contains(t, rep.Failure.HumanWatchCodes, "shadow_fail_ge_2")
	require.True(t, rep.NotProviderSwitch)
}

func TestBuildDailyPilotReport_ControlledMonitorOverlay(t *testing.T) {
	monitor := controlledmonitor.Observe(controlledmonitor.Input{
		Enabled:   true,
		TradeDate: "2026-08-22",
		Shadow: &providershadow.ShadowComparisonRecord{
			TradeDate:  "2026-08-22",
			Comparable: true,
			LegacySummary: providershadow.EnvelopeRef{OK: true, LineCount: 2},
			PortfolioSummary: providershadow.EnvelopeRef{OK: true, LineCount: 3},
		},
	})
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:           true,
		TradeDate:         "2026-08-22",
		ControlledMonitor: monitor,
	})
	require.True(t, rep.LegacyVsPortfolio.MonitorOverlay.Present)
	require.Equal(t, controlledmonitor.OutcomeOK, rep.LegacyVsPortfolio.MonitorOverlay.OutcomeCode)
}

func TestBuildDailyPilotReport_PortfolioHistoryShape(t *testing.T) {
	cash := 0.25
	view := &portfoliohistory.PortfolioHistoryView{
		Days: []portfoliohistory.DailyRecord{
			{
				TradeDate: "2026-08-22",
				Risk: portfoliohistory.RiskSnapshotSummary{
					Present: true,
					Found:   true,
					CashRatio: &cash,
					NameCount: 5,
				},
			},
		},
	}
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:          true,
		TradeDate:        "2026-08-22",
		PortfolioHistory: view,
	})
	require.True(t, rep.Allocation.HistoryShape.Available)
	require.NotNil(t, rep.Allocation.HistoryShape.CashRatio)
	require.Equal(t, 5, rep.Allocation.HistoryShape.NameCount)
}

func TestWriteReportFile(t *testing.T) {
	dir := t.TempDir()
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
	})
	path, err := dailypilot.WriteReportFile(dir, rep)
	require.NoError(t, err)
	require.FileExists(t, path)
	require.Equal(t, filepath.Join(dir, "DAILY_PILOT_20260822.json"), path)
}

func TestBuildDailyPilotReport_RollbackScopeChanged(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{
		Enabled:   true,
		TradeDate: "2026-08-22",
		PolicySnapshotOpen: dailypilot.PolicySnapshotInput{
			Adoption:      "off",
			AccountIDs:    []uint{1},
			StrategyNames: []string{"s"},
			TradeDates:    []string{"2026-08-22"},
		},
		PolicySnapshotClose: dailypilot.PolicySnapshotInput{
			Adoption:      "controlled",
			AccountIDs:    []uint{1, 2},
			StrategyNames: []string{"s"},
			TradeDates:    []string{"2026-08-22"},
		},
	})
	require.Equal(t, "scope_changed", rep.Rollback.Transition)
	require.Equal(t, "investigate_scope", rep.Rollback.RecommendedHumanAction)
	require.True(t, rep.Rollback.ScopeDelta.Expanded)
}

func mustJSON(t *testing.T, rep *dailypilot.DailyPilotReport) []byte {
	t.Helper()
	raw, err := dailypilot.ToJSON(rep)
	require.NoError(t, err)
	return raw
}

func verdictUnset(t *testing.T) string {
	t.Helper()
	return "unset"
}

func TestIsolation_NoWriteChainOrProviderSwitch(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	forbidden := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/controlledadoption",
	}
	forbiddenSrc := []string{
		"SetActive(",
		"ResetActive(",
		"CreatePlanWithItems",
		"BuildDraftTradePlan",
		"FreezeTradePlan",
	}
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err)
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		for _, imp := range forbidden {
			require.NotContains(t, text, imp, e.Name())
		}
	}
}

func TestBuildDailyPilotReport_JSONSnakeCase(t *testing.T) {
	rep := dailypilot.BuildDailyPilotReport(dailypilot.DailyPilotInput{Enabled: true, TradeDate: "2026-08-22"})
	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	blob := string(raw)
	require.Contains(t, blob, `"schema_version"`)
	require.Contains(t, blob, `"legacy_vs_portfolio"`)
	require.Contains(t, blob, `"human_verdict"`)
}
