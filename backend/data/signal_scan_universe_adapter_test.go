package data

import (
	"testing"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestAdaptUniverseCodeToStockInfo(t *testing.T) {
	info := AdaptUniverseCodeToStockInfo("sz300274", "阳光电源", "电气设备")
	require.Equal(t, "300274.SZ", info.SECUCODE)
	require.Equal(t, "300274", info.SECURITYCODE)
	require.Equal(t, "阳光电源", info.SECURITYNAMEABBR)
	require.Equal(t, "电气设备", info.INDUSTRY)
}

func TestAdaptStrategyRunRowToStockInfo_NLFields(t *testing.T) {
	row := map[string]any{
		"SECURITY_CODE":       "300274",
		"SECURITY_SHORT_NAME": "阳光电源",
		"MARKET_SHORT_NAME":   "SZ",
		"NEWEST_PRICE":        "86.40",
		"CHG":                 "-3.28",
		"VOLUME":              "5416.91万",
		"TURNOVER_RATE":       "3.41",
		"QRR":                 "0.66",
	}
	info, err := AdaptStrategyRunRowToStockInfo(row)
	require.NoError(t, err)
	require.Equal(t, "300274.SZ", info.SECUCODE)
	require.Equal(t, "300274", info.SECURITYCODE)
	require.Equal(t, "阳光电源", info.SECURITYNAMEABBR)
	require.Equal(t, "86.40", info.NEWPRICE)
	require.Equal(t, "-3.28", info.CHANGERATE)
}

func TestStrategyKeyAndUniverseIDHelpers(t *testing.T) {
	require.Equal(t, "stock_strategy:4", StrategyKeyFromID(4))
	require.Equal(t, "run:82", UniverseIDFromRun(82))
	require.Equal(t, "follow:2026-09-02", UniverseIDFromFollow("2026-09-02"))
}

func TestFormatUniverseSnapshotMessage(t *testing.T) {
	msg := formatUniverseSnapshotMessage(&models.UniverseSignalSnapshotConfig{
		Scope:         models.SignalScanScopeUniverse,
		StrategyID:    4,
		StrategyRunID: 82,
		UniverseID:    "run:82",
	})
	require.Contains(t, msg, "universeId=run:82")
	require.Contains(t, msg, "strategyId=4")
	require.Contains(t, msg, "strategyRunId=82")
	require.Contains(t, msg, "scope=universe")
}
