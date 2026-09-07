package data

import (
	"testing"
)

func TestNormalizeStockCode(t *testing.T) {
	tests := []struct {
		in     string
		market string
		symbol string
		tsCode string
		sina   string
	}{
		{"000001.SZ", MarketCN, "000001", "000001.SZ", "sz000001"},
		{"00700.HK", MarketHK, "00700", "00700.HK", "hk00700"},
		{"AAPL.US", MarketUS, "AAPL", "AAPL.US", "gb_aapl"},
		{"sz300408", MarketCN, "300408", "300408.SZ", "sz300408"},
		{"300408", MarketCN, "300408", "300408.SZ", "sz300408"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			n, err := NormalizeStockCode(tt.in)
			if err != nil {
				t.Fatalf("NormalizeStockCode(%q): %v", tt.in, err)
			}
			if n.Market != tt.market || n.Symbol != tt.symbol || n.TSCode != tt.tsCode || n.SinaCode != tt.sina {
				t.Fatalf("got market=%s symbol=%s ts=%s sina=%s; want %s %s %s %s",
					n.Market, n.Symbol, n.TSCode, n.SinaCode, tt.market, tt.symbol, tt.tsCode, tt.sina)
			}
			t.Logf("input=%s → market=%s symbol=%s tsCode=%s sina=%s", tt.in, n.Market, n.Symbol, n.TSCode, n.SinaCode)
		})
	}
}
