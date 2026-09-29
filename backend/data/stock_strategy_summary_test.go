package data

import (
	"strings"
	"testing"

	"go-stock/backend/models"
)

func TestTechnicalSummaryIncludesComparisonPackFlags(t *testing.T) {
	api := NewStockStrategyApi()
	breakout := &models.StockStrategy{
		QueryType: "technical",
		QueryJSON: `{"BREAK_THROUGH":true,"LONG_AVG_ARRAY":true,"MACD_GOLDEN_FORK":true}`,
	}
	text := api.SummaryText(breakout)
	for _, want := range []string{"放量突破", "均线多头", "MACD金叉"} {
		if !strings.Contains(text, want) {
			t.Fatalf("breakout summary %q missing %s", text, want)
		}
	}

	pullback := &models.StockStrategy{
		QueryType: "technical",
		QueryJSON: `{"LONG_AVG_ARRAY":true,"DOWN_NARROW_VOLUME":true,"BREAKUP_MA_5DAYS":true}`,
	}
	text = api.SummaryText(pullback)
	for _, want := range []string{"均线多头", "下跌无量", "向上突破5日均线"} {
		if !strings.Contains(text, want) {
			t.Fatalf("pullback summary %q missing %s", text, want)
		}
	}

	funds := &models.StockStrategy{
		QueryType: "technical",
		QueryJSON: `{"LOW_FUNDS_INFLOW":true,"KDJ_GOLDEN_FORK":true}`,
	}
	text = api.SummaryText(funds)
	for _, want := range []string{"低位资金流入", "KDJ金叉"} {
		if !strings.Contains(text, want) {
			t.Fatalf("funds summary %q missing %s", text, want)
		}
	}
}
