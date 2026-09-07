//go:build integration

package data

import (
	"testing"

	"go-stock/backend/db"
)

func initUnifiedBasicTestDB(t *testing.T) {
	t.Helper()
	db.Init("../../data/stock.db")
	if db.Dao == nil {
		t.Fatal("db.Dao is nil")
	}
}

func TestGetStockBasicByCode_Pipeline(t *testing.T) {
	initUnifiedBasicTestDB(t)
	api := NewStockDataApi()

	cases := []struct {
		in         string
		wantMarket string
		wantTable  string
	}{
		{"000001.SZ", MarketCN, "tushare_stock_basic"},
		{"00700.HK", MarketHK, "stock_base_info_hk"},
		{"AAPL.US", MarketUS, "stock_base_info_us"},
		{"sz300408", MarketCN, "tushare_stock_basic"},
		{"0.300408", MarketCN, "tushare_stock_basic"}, // 东财 secid
		{"1.600519", MarketCN, "tushare_stock_basic"},
	}

	t.Log("输入代码 -> market -> symbol -> ts_code/secucode -> name")
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			norm, err := NormalizeStockCode(tc.in)
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			u, err := api.GetStockBasicByCode(tc.in)
			if err != nil {
				t.Fatalf("GetStockBasicByCode(%s): %v", tc.in, err)
			}
			if u == nil {
				t.Fatal("result is nil")
			}
			t.Logf("%s -> %s -> %s -> %s/%s -> %s  [table=%s]",
				tc.in, u.Market, u.Symbol, u.TSCode, u.SecuCode, u.Name, u.SourceTable)

			if u.Market != tc.wantMarket {
				t.Fatalf("market=%s want %s", u.Market, tc.wantMarket)
			}
			if u.SourceTable != tc.wantTable {
				t.Fatalf("sourceTable=%s want %s", u.SourceTable, tc.wantTable)
			}
			if u.Name == "" || u.TSCode == "" || u.Symbol == "" {
				t.Fatalf("missing core fields: %+v", u)
			}
			_ = norm
		})
	}
}

func TestGetStockBasicListAndBySymbol(t *testing.T) {
	initUnifiedBasicTestDB(t)
	api := NewStockDataApi()

	cn := api.GetStockBasicList(MarketCN)
	if len(cn) == 0 || len(cn) > defaultStockBasicListLimit {
		t.Fatalf("GetStockBasicList(CN) len=%d", len(cn))
	}
	if cn[0].Market != MarketCN || cn[0].SourceTable != "tushare_stock_basic" {
		t.Fatalf("unexpected CN row: %+v", cn[0])
	}

	bySym := api.GetStockBasicBySymbol("000001")
	if len(bySym) == 0 {
		t.Fatal("GetStockBasicBySymbol(000001) empty")
	}
	t.Logf("BySymbol(000001) → market=%s name=%s table=%s", bySym[0].Market, bySym[0].Name, bySym[0].SourceTable)

	aapl := api.GetStockBasicBySymbol("AAPL")
	if len(aapl) == 0 {
		t.Fatal("GetStockBasicBySymbol(AAPL) empty")
	}
	if aapl[0].Market != MarketUS {
		t.Fatalf("AAPL market=%s", aapl[0].Market)
	}
}
