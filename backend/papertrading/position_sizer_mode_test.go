package papertrading_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/papertrading"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestNormalizePositionSizerMode_DefaultsFixedAmount(t *testing.T) {
	require.Equal(t, papertrading.PositionSizerModeFixedAmount, papertrading.NormalizePositionSizerMode(""))
	require.Equal(t, papertrading.PositionSizerModeFixedAmount, papertrading.NormalizePositionSizerMode("unknown"))
	require.Equal(t, papertrading.PositionSizerModeFixedAmount, papertrading.NormalizePositionSizerMode("FIXED_AMOUNT"))
	require.Equal(t, papertrading.PositionSizerModePortfolioAware, papertrading.NormalizePositionSizerMode("portfolio_aware"))
	require.Equal(t, papertrading.PositionSizerModePortfolioAware, papertrading.NormalizePositionSizerMode(" Portfolio_Aware "))
}

func TestGetConfig_MissingPositionSizerModeKeepsFixedAmount(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(prev)
		papertrading.ResetConfigCache()
	})
	papertrading.ResetConfigCache()

	require.NoError(t, os.MkdirAll("data", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("data", "paper_trading_mvp.json"),
		[]byte(`{"enablePaperTrading":true}`), 0o644))

	cfg := papertrading.GetConfig()
	require.True(t, cfg.EnablePaperTrading)
	require.Equal(t, papertrading.PositionSizerModeFixedAmount, cfg.PositionSizerMode)
	require.Equal(t, tradingconfig.PositionSizerModeFixedAmount, tradingconfig.Default().PositionSizerMode())
}
