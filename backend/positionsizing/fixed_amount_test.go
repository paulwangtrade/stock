package positionsizing_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/portfolio"
	"go-stock/backend/positionsizing"
	"go-stock/backend/tradingconfig"

	"github.com/stretchr/testify/require"
)

func TestFixedAmountSizer_MatchesTradingConfigAndLegacy(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		OpenBuyAmountPerStock: 100_000,
	}))

	got := (&positionsizing.FixedAmountSizer{}).Propose(positionsizing.Request{})
	require.Equal(t, positionsizing.MethodFixedAmount, got.Method)
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, got.Source)
	require.Equal(t, float64(100_000), got.PlannedAmount)
	require.Equal(t, int64(0), got.PlannedVolume)

	viaProvider := tradingconfig.Default().OpenBuyAmountPerStock()
	legacy := data.GetPaperOpenBuyConfig().OpenBuyAmountPerStock
	if legacy <= 0 {
		legacy = 100_000
	}
	require.Equal(t, viaProvider, got.PlannedAmount)
	require.Equal(t, legacy, got.PlannedAmount)
}

func TestFixedAmountSizer_CustomAmountStillViaProvider(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		OpenBuyAmountPerStock: 88_000,
	}))

	got := positionsizing.ProposeDefault(positionsizing.Request{StockCode: "sz000001", Score: 0.99})
	require.Equal(t, float64(88_000), got.PlannedAmount)
	// Score must not affect MVP fixed_amount.
	require.Equal(t, positionsizing.MethodFixedAmount, got.Method)
}

func TestFixedAmountSizer_InvalidFallsBackTo100000(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, os.MkdirAll("data", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("data", "paper_open_buy.json"),
		[]byte(`{"openBuyAmountPerStock":0}`), 0o644))
	data.ResetPaperOpenBuyConfigCache()

	got := positionsizing.PlannedAmount()
	require.Equal(t, float64(100_000), got)
	require.Equal(t, tradingconfig.Default().OpenBuyAmountPerStock(), got)
}

func TestDefaultSizer_IsFixedAmountOnly(t *testing.T) {
	p := positionsizing.Default().Propose(positionsizing.Request{})
	require.Equal(t, positionsizing.MethodFixedAmount, p.Method)
	require.NotEqual(t, "fixed_fractional", string(p.Method))
	require.NotEqual(t, "risk_budget", string(p.Method))
	require.Equal(t, tradingconfig.SourceLegacyPaperOpenBuy, p.Source)
}

func TestFixedAmountSizer_IgnoresPortfolioSnapshot(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	data.ResetPaperOpenBuyConfigCache()
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)

	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		OpenBuyAmountPerStock: 100_000,
	}))

	withPort := (&positionsizing.FixedAmountSizer{}).Propose(positionsizing.Request{
		Score: 99,
		Rank:  1,
		Portfolio: &portfolio.Snapshot{
			Found:         true,
			Cash:          1,
			TotalEquity:   2_000_000,
			MarketValue:   1_200_000,
			PositionCount: 10,
			TotalExposure: 1_200_000,
		},
	})
	without := (&positionsizing.FixedAmountSizer{}).Propose(positionsizing.Request{})
	require.Equal(t, without.PlannedAmount, withPort.PlannedAmount)
	require.Equal(t, float64(100_000), withPort.PlannedAmount)
	require.Equal(t, positionsizing.MethodFixedAmount, withPort.Method)
}
