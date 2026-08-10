package tradingconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/risk"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestLegacyRiskAdapter_MatchesPaperOpenBuy(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnableRiskFilter:         true,
		PlanMarketLevel:          2,
		BlockNewEntriesOnDefense: true,
		MaxGrossExposurePct:      0.80,
		MaxSingleNamePct:         0.15,
		MaxDailyLossPct:          0.02,
		CurrentDailyPnlPct:       -0.01,
		OpenBuyAmountPerStock:    100_000,
	}))

	got := (&tradingconfig.LegacyRiskConfigAdapter{}).Load()
	legacy := data.GetPaperOpenBuyConfig()

	require.Equal(t, tradingconfig.SourceLegacyPaperConfig, got.Source)
	require.Equal(t, legacy.EnableRiskFilter, got.Enabled)
	require.Equal(t, legacy.PlanMarketLevel, got.MarketLevel)
	require.Equal(t, legacy.BlockNewEntriesOnDefense, got.BlockNewEntriesOnDefense)
	require.Equal(t, legacy.MaxGrossExposurePct, got.MaxGrossExposurePct)
	require.Equal(t, legacy.MaxSingleNamePct, got.MaxSingleNamePct)
	require.Equal(t, legacy.MaxDailyLossPct, got.MaxDailyLossPct)
	require.Equal(t, legacy.CurrentDailyPnlPct, got.CurrentDailyPnlPct)

	viaProvider := tradingconfig.Default().Risk()
	require.Equal(t, got, viaProvider)
}

func TestLegacyRiskAdapter_DefaultThresholdsMatchGetter(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, os.MkdirAll("data", 0o755))
	// Minimal file: getter applies Phase1.2 defaults.
	require.NoError(t, os.WriteFile(filepath.Join("data", "paper_open_buy.json"),
		[]byte(`{"openBuyAmountPerStock":100000}`), 0o644))
	data.ResetPaperOpenBuyConfigCache()

	legacy := data.GetPaperOpenBuyConfig()
	got := tradingconfig.Default().Risk()
	require.Equal(t, legacy.EnableRiskFilter, got.Enabled)
	require.Equal(t, legacy.PlanMarketLevel, got.MarketLevel)
	require.Equal(t, legacy.BlockNewEntriesOnDefense, got.BlockNewEntriesOnDefense)
	require.Equal(t, legacy.MaxGrossExposurePct, got.MaxGrossExposurePct)
	require.Equal(t, legacy.MaxSingleNamePct, got.MaxSingleNamePct)
	require.Equal(t, 0.85, got.MaxGrossExposurePct)
	require.Equal(t, 0.20, got.MaxSingleNamePct)
}

func TestProvider_RiskSourceIsLegacyNotTradingConfig(t *testing.T) {
	r := tradingconfig.Default().Resolve()
	require.Equal(t, tradingconfig.SourceLegacyPaperConfig, r.Risk.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Risk.Source)
}

// TestPlanFilter_IdenticalWithProviderVsDirectLegacy proves Risk PASS/FAIL and
// blocker codes are unchanged when thresholds come from Provider vs direct cfg.
func TestPlanFilter_IdenticalWithProviderVsDirectLegacy(t *testing.T) {
	legacy := data.GetPaperOpenBuyConfig()
	rv := tradingconfig.Default().Risk()

	require.Equal(t, legacy.EnableRiskFilter, rv.Enabled)
	require.Equal(t, legacy.PlanMarketLevel, rv.MarketLevel)
	require.Equal(t, legacy.BlockNewEntriesOnDefense, rv.BlockNewEntriesOnDefense)
	require.Equal(t, legacy.MaxGrossExposurePct, rv.MaxGrossExposurePct)
	require.Equal(t, legacy.MaxSingleNamePct, rv.MaxSingleNamePct)
	require.Equal(t, legacy.MaxDailyLossPct, rv.MaxDailyLossPct)
	require.Equal(t, legacy.CurrentDailyPnlPct, rv.CurrentDailyPnlPct)
	require.Equal(t, tradingconfig.SourceLegacyPaperConfig, rv.Source)

	cands := []risk.PlanCandidate{
		{StockCode: "sz000001", StockName: "平安银行", Rank: 1, TargetAmount: 100_000},
		{StockCode: "sh600519", StockName: "贵州茅台", Rank: 2, TargetAmount: 100_000},
	}

	legacyCtx := risk.PlanContext{
		Enabled:             legacy.EnableRiskFilter,
		MarketLevel:         legacy.PlanMarketLevel,
		BlockNewEntries:     legacy.BlockNewEntriesOnDefense,
		MaxGrossExposurePct: legacy.MaxGrossExposurePct,
		MaxSingleNamePct:    legacy.MaxSingleNamePct,
		MaxDailyLossPct:     legacy.MaxDailyLossPct,
		CurrentDailyPnlPct:  legacy.CurrentDailyPnlPct,
		AmountPerStock:      100_000,
		MaxNames:            5,
		Cash:                500_000,
		EquityBase:          500_000,
	}
	providerCtx := risk.PlanContext{
		Enabled:             rv.Enabled,
		MarketLevel:         rv.MarketLevel,
		BlockNewEntries:     rv.BlockNewEntriesOnDefense,
		MaxGrossExposurePct: rv.MaxGrossExposurePct,
		MaxSingleNamePct:    rv.MaxSingleNamePct,
		MaxDailyLossPct:     rv.MaxDailyLossPct,
		CurrentDailyPnlPct:  rv.CurrentDailyPnlPct,
		AmountPerStock:      100_000,
		MaxNames:            5,
		Cash:                500_000,
		EquityBase:          500_000,
	}

	assertPlanFilterIdentical(t, cands, legacyCtx, providerCtx)

	// Defense-blocked path: same mapping must produce identical blockers.
	legacyCtx.MarketLevel = 2
	legacyCtx.BlockNewEntries = true
	providerCtx.MarketLevel = 2
	providerCtx.BlockNewEntries = true
	assertPlanFilterIdentical(t, cands, legacyCtx, providerCtx)
}

func assertPlanFilterIdentical(t *testing.T, cands []risk.PlanCandidate, legacyCtx, providerCtx risk.PlanContext) {
	t.Helper()
	a := risk.PlanFilter(cands, legacyCtx)
	b := risk.PlanFilter(cands, providerCtx)
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.Equal(t, a.RiskStatus, b.RiskStatus)
	require.Equal(t, a.AcceptedCount, b.AcceptedCount)
	require.Equal(t, a.FilteredCount, b.FilteredCount)
	require.Equal(t, a.RiskSummary, b.RiskSummary)
	require.Equal(t, len(a.Items), len(b.Items))
	for i := range a.Items {
		require.Equal(t, a.Items[i].Candidate.StockCode, b.Items[i].Candidate.StockCode)
		require.Equal(t, a.Items[i].Status, b.Items[i].Status)
		require.Equal(t, a.Items[i].Allowed, b.Items[i].Allowed)
		require.Equal(t, a.Items[i].RiskCode, b.Items[i].RiskCode)
		require.Equal(t, a.Items[i].RiskMessage, b.Items[i].RiskMessage)
	}
}
