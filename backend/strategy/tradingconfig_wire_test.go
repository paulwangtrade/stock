package strategy_test

import (
	"os"
	"strings"
	"testing"

	"go-stock/backend/positionsizing"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestBuildTradePlan_UsesProviderForEnableExecute(t *testing.T) {
	raw, err := os.ReadFile("build_trade_plan.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "tradingconfig.Default().EnablePaperOpenBuy()")
	require.NotContains(t, src, "GetPaperOpenBuyConfig()")
	require.NotContains(t, src, "cfg.EnablePaperOpenBuy")
}

func TestFreezeTradePlan_UsesProviderForEnableExecute(t *testing.T) {
	raw, err := os.ReadFile("freeze_trade_plan.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "tradingconfig.Default().EnablePaperOpenBuy()")
	require.NotContains(t, src, "GetPaperOpenBuyConfig()")
}

func TestBuildDraftTradePlan_UsesPositionSizerForAmount(t *testing.T) {
	raw, err := os.ReadFile("build_draft_trade_plan.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "resolvePlanAmountViaSizerForDraft")
	require.NotContains(t, src, "GetPaperOpenBuyConfig()")
	require.NotContains(t, src, "cfg.OpenBuyAmountPerStock")
}

func TestBuildTradePlan_UsesPositionSizerForAmount(t *testing.T) {
	raw, err := os.ReadFile("build_trade_plan.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "resolvePlanAmountViaSizer()")
	require.Equal(t, 0, strings.Count(src, "OpenBuyAmountPerStock()"))
	require.Equal(t, 0, strings.Count(src, "cfg.OpenBuyAmountPerStock"))
}

func TestPlanAmountSizing_UsesPositionsizingPackage(t *testing.T) {
	raw, err := os.ReadFile("plan_amount_sizing.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "positionsizing.ProposeDefault")
	require.Contains(t, src, "positionsizing.ForMode")
	require.Contains(t, src, "positionsizing.LogApplied")
	require.Contains(t, src, "ErrNoBudget")
	require.NotContains(t, src, "GetPaperOpenBuyConfig")
	require.NotContains(t, src, "db.Dao")
}

func TestTradingConfigSources_AreLegacyOnly(t *testing.T) {
	r := tradingconfig.Default().Resolve()
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, r.Position.Source)
	require.Equal(t, tradingconfig.SourceLegacyAfterClose, r.Workflow.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperConfig, r.Risk.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, r.Execution.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperMVP, r.PaperMVP.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Position.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Workflow.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Risk.Source)
}

func TestPlanRiskBridge_UsesTradingConfigProviderForRisk(t *testing.T) {
	raw, err := os.ReadFile("plan_risk_bridge.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "tradingconfig.Default().Risk()")
	require.Contains(t, src, "tradingconfig.LogRiskInitialized()")
	require.Contains(t, src, "portfolio.NewService().Snapshot")
	require.NotContains(t, src, "NewPaperTradingApi()")
	require.NotContains(t, src, "cfg.EnableRiskFilter")
	require.NotContains(t, src, "cfg.MaxSingleNamePct")
	require.NotContains(t, src, "GetPaperOpenBuyConfig()")
}

func TestMorningMaterialize_UsesTradingConfigProviderForRiskLimits(t *testing.T) {
	raw, err := os.ReadFile("morning_position_materialize.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "tradingconfig.Default().Risk()")
	require.Contains(t, src, "rv.MaxGrossExposurePct")
	require.Contains(t, src, "rv.MaxSingleNamePct")
	require.Contains(t, src, "portfolio.NewService().Snapshot")
	require.NotContains(t, src, "NewPaperTradingApi()")
	require.NotContains(t, src, "cfg.MaxGrossExposurePct")
	require.NotContains(t, src, "cfg.MaxSingleNamePct")
}

func TestResolvePlanAmount_MatchesProviderFixedAmount(t *testing.T) {
	positionsizing.ResetLogAppliedForTest()
	proposal := positionsizing.ProposeDefault(positionsizing.Request{})
	require.Equal(t, positionsizing.MethodFixedAmount, proposal.Method)
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, proposal.Source)
	require.Equal(t, tradingconfig.Default().OpenBuyAmountPerStock(), proposal.PlannedAmount)
}
