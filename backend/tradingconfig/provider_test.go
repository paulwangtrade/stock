package tradingconfig_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestLegacyAdapter_PositionMatchesPaperOpenBuy(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    false,
		OpenBuyAmountPerStock: 123_456,
	}))

	got := (&tradingconfig.LegacyAdapter{}).Load()
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, got.Position.Source)
	require.Equal(t, tradingconfig.SizingMethodFixedAmount, got.Position.SizingMethod)
	require.Equal(t, float64(123_456), got.Position.MaxPositionAmount)

	legacy := data.GetPaperOpenBuyConfig()
	amount := legacy.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	require.Equal(t, amount, got.Position.MaxPositionAmount)
}

func TestLegacyAdapter_PositionDefaultWhenInvalid(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, os.MkdirAll("data", 0o755))
	// Write raw zero amount; SavePaperOpenBuyConfig would coerce before write.
	raw := []byte(`{"openBuyAmountPerStock":0,"enablePaperOpenBuy":false}`)
	require.NoError(t, os.WriteFile(filepath.Join("data", "paper_open_buy.json"), raw, 0o644))
	data.ResetPaperOpenBuyConfigCache()

	got := tradingconfig.Default().OpenBuyAmountPerStock()
	require.Equal(t, float64(tradingconfig.DefaultFixedAmount), got)

	legacy := data.GetPaperOpenBuyConfig()
	require.Equal(t, legacy.OpenBuyAmountPerStock, got)
}

func TestLegacyAdapter_AfterCloseMatchesLegacy(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetAfterClosePlanConfigCache()
	t.Cleanup(data.ResetAfterClosePlanConfigCache)

	require.NoError(t, os.MkdirAll("data", 0o755))
	raw, _ := json.MarshalIndent(map[string]any{"after_close_plan_enabled": true}, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join("data", "after_close_plan.json"), raw, 0o644))
	data.ResetAfterClosePlanConfigCache()

	got := tradingconfig.Default().AfterCloseEnabled()
	require.True(t, got)
	require.Equal(t, data.IsAfterClosePlanEnabled(), got)

	r := tradingconfig.Default().Resolve()
	require.Equal(t, tradingconfig.SourceLegacyAfterClose, r.Workflow.Source)
	require.True(t, r.Workflow.AfterCloseEnabled)
}

func TestProvider_MatchesLegacyGetters(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	data.ResetAfterClosePlanConfigCache()
	t.Cleanup(func() {
		data.ResetPaperOpenBuyConfigCache()
		data.ResetAfterClosePlanConfigCache()
	})

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		OpenBuyAmountPerStock: 100_000,
	}))
	require.NoError(t, data.SaveAfterClosePlanConfig(data.AfterClosePlanConfig{
		AfterClosePlanEnabled: false,
	}))

	p := tradingconfig.Default()
	legacyAmt := data.GetPaperOpenBuyConfig().OpenBuyAmountPerStock
	if legacyAmt <= 0 {
		legacyAmt = 100_000
	}
	require.Equal(t, legacyAmt, p.OpenBuyAmountPerStock())
	require.Equal(t, data.IsAfterClosePlanEnabled(), p.AfterCloseEnabled())
}

func TestProvider_SourceTags(t *testing.T) {
	r := (&tradingconfig.LegacyAdapter{}).Load()
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, r.Position.Source)
	require.Equal(t, tradingconfig.SourceLegacyAfterClose, r.Workflow.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperConfig, r.Risk.Source)
	require.Equal(t, tradingconfig.SourceLegacyAutomation, r.Automation.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, r.Execution.Source)
	require.Equal(t, tradingconfig.SourceLegacyPaperMVP, r.PaperMVP.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Position.Source)
	require.NotEqual(t, tradingconfig.SourceTradingConfig, r.Risk.Source)
}

func TestProvider_EnablePaperOpenBuyMatchesLegacy(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 50_000,
	}))

	p := tradingconfig.Default()
	require.True(t, p.EnablePaperOpenBuy())
	require.Equal(t, data.GetPaperOpenBuyConfig().EnablePaperOpenBuy, p.EnablePaperOpenBuy())
	require.Equal(t, float64(50_000), p.OpenBuyAmountPerStock())
}

func TestProvider_PaperMVPMatchesJSON(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	require.NoError(t, os.MkdirAll("data", 0o755))
	raw := []byte(`{"enablePaperTrading":true,"initialCash":2000000,"fillMode":"B"}`)
	require.NoError(t, os.WriteFile(filepath.Join("data", "paper_trading_mvp.json"), raw, 0o644))

	// Direct JSON path (no papertrading loader registration in this package test).
	r := (&tradingconfig.LegacyAdapter{}).Load()
	require.Equal(t, tradingconfig.SourceLegacyPaperMVP, r.PaperMVP.Source)
	require.True(t, r.PaperMVP.EnablePaperTrading)
	require.Equal(t, float64(2_000_000), r.PaperMVP.InitialCash)
	require.Equal(t, "B", r.PaperMVP.FillMode)
	require.True(t, tradingconfig.Default().PaperTradingEnabled())
	require.Equal(t, "B", tradingconfig.Default().FillMode())
}
