package strategy

import (
	"encoding/json"
	"testing"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestIsSTName(t *testing.T) {
	if !isSTName("ST示例") || !isSTName("*ST示例") {
		t.Fatal("should detect ST prefix")
	}
	if isSTName("平安银行") {
		t.Fatal("平安银行 should not be ST")
	}
}

func TestNormalizeTradeDate(t *testing.T) {
	if normalizeTradeDate("2026-07-17") != "2026-07-17" {
		t.Fatal("keep valid date")
	}
	if normalizeTradeDate("") == "" {
		t.Fatal("empty should fallback today")
	}
}

func parseRunFromDataList(t *testing.T, dataList []map[string]any) []UniverseCandidate {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"dataList": dataList,
	})
	require.NoError(t, err)
	strat := &models.StockStrategy{ID: 1, Name: "test-strategy"}
	run := &models.StockStrategyRun{ID: 50, ResultJSON: string(payload)}
	return parseStrategyRunItems(strat, run)
}

func TestParseStrategyRunItems_LegacyAbbrStillWorks(t *testing.T) {
	items := parseRunFromDataList(t, []map[string]any{
		{
			"SECURITY_CODE":      "600036",
			"SECURITY_NAME_ABBR": "招商银行",
		},
	})
	require.Len(t, items, 1)
	require.Equal(t, "sh600036", items[0].StockCode)
	require.Equal(t, "招商银行", items[0].StockName)
}

func TestParseStrategyRunItems_ShortNameOnly(t *testing.T) {
	// Mirrors live strategy run shape (no SECURITY_NAME_ABBR); e.g. sz301677 / 欣兴工具.
	items := parseRunFromDataList(t, []map[string]any{
		{
			"SECURITY_CODE":        "301677",
			"MARKET_SHORT_NAME":    "SZ",
			"SECURITY_SHORT_NAME":  "欣兴工具",
		},
		{
			"SECURITY_CODE":       "600363",
			"MARKET_SHORT_NAME":   "SH",
			"SECURITY_SHORT_NAME": "联创光电",
		},
	})
	require.Len(t, items, 2)
	byCode := map[string]string{}
	for _, it := range items {
		byCode[it.StockCode] = it.StockName
	}
	require.Equal(t, "欣兴工具", byCode["sz301677"])
	require.Equal(t, "联创光电", byCode["sh600363"])
}

func TestParseStrategyRunItems_AbbrPreferredOverShort(t *testing.T) {
	items := parseRunFromDataList(t, []map[string]any{
		{
			"SECURITY_CODE":        "600036",
			"SECURITY_NAME_ABBR":   "招商银行",
			"SECURITY_SHORT_NAME":  "招行",
		},
	})
	require.Len(t, items, 1)
	require.Equal(t, "招商银行", items[0].StockName)
}

func TestParseStrategyRunItems_LowerShortNameKey(t *testing.T) {
	items := parseRunFromDataList(t, []map[string]any{
		{
			"security_code":       "000001",
			"security_short_name": "平安银行",
		},
	})
	require.Len(t, items, 1)
	require.Equal(t, "sz000001", items[0].StockCode)
	require.Equal(t, "平安银行", items[0].StockName)
}

func TestParseStrategyRunItems_ShortNameSTSkipped(t *testing.T) {
	items := parseRunFromDataList(t, []map[string]any{
		{
			"SECURITY_CODE":       "000002",
			"SECURITY_SHORT_NAME": "ST示例",
		},
		{
			"SECURITY_CODE":       "600000",
			"SECURITY_SHORT_NAME": "浦发银行",
		},
	})
	require.Len(t, items, 1)
	require.Equal(t, "sh600000", items[0].StockCode)
	require.Equal(t, "浦发银行", items[0].StockName)
}
