package strategy

import "testing"

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
