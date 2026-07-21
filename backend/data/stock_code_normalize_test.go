package data

import (
	"fmt"
	"strings"
	"testing"
)

// TestNormalizeStockCode_Table 完整转换表：正常样例 + 异常输入。
// 运行：go test ./backend/data -run TestNormalizeStockCode_Table -v
func TestNormalizeStockCode_Table(t *testing.T) {
	type row struct {
		category string
		input    string
		wantOK   bool
		market   string
		symbol   string
		tsCode   string
		sina     string
		emSecID  string
		errPart  string // 失败时错误信息应包含
	}

	rows := []row{
		// —— 标准点分 ——
		{"标准点分", "000001.SZ", true, MarketCN, "000001", "000001.SZ", "sz000001", "0.000001", ""},
		{"标准点分", "600519.SH", true, MarketCN, "600519", "600519.SH", "sh600519", "1.600519", ""},
		{"标准点分", "00700.HK", true, MarketHK, "00700", "00700.HK", "hk00700", "128.00700", ""},
		{"标准点分", "AAPL.US", true, MarketUS, "AAPL", "AAPL.US", "gb_aapl", "", ""},

		// —— 小写代码 ——
		{"小写代码", "000001.sz", true, MarketCN, "000001", "000001.SZ", "sz000001", "0.000001", ""},
		{"小写代码", "aapl.us", true, MarketUS, "AAPL", "AAPL.US", "gb_aapl", "", ""},
		{"小写代码", "sz300408", true, MarketCN, "300408", "300408.SZ", "sz300408", "0.300408", ""},
		{"小写代码", "hk00700", true, MarketHK, "00700", "00700.HK", "hk00700", "128.00700", ""},
		{"小写代码", "gb_aapl", true, MarketUS, "AAPL", "AAPL.US", "gb_aapl", "", ""},

		// —— 新浪前缀 / 自选格式 ——
		{"新浪前缀", "SH600519", true, MarketCN, "600519", "600519.SH", "sh600519", "1.600519", ""},
		{"新浪前缀", "sz000001", true, MarketCN, "000001", "000001.SZ", "sz000001", "0.000001", ""},

		// —— 东财 secid ——
		{"东财secid", "1.600519", true, MarketCN, "600519", "600519.SH", "sh600519", "1.600519", ""},
		{"东财secid", "0.000001", true, MarketCN, "000001", "000001.SZ", "sz000001", "0.000001", ""},
		{"东财secid", "0.300408", true, MarketCN, "300408", "300408.SZ", "sz300408", "0.300408", ""},
		{"东财secid", "128.00700", true, MarketHK, "00700", "00700.HK", "hk00700", "128.00700", ""},

		// —— 不带市场后缀 ——
		{"无后缀", "300408", true, MarketCN, "300408", "300408.SZ", "sz300408", "0.300408", ""},
		{"无后缀", "600519", true, MarketCN, "600519", "600519.SH", "sh600519", "1.600519", ""},
		{"无后缀", "000001", true, MarketCN, "000001", "000001.SZ", "sz000001", "0.000001", ""},
		{"无后缀", "700", true, MarketHK, "00700", "00700.HK", "hk00700", "128.00700", ""},
		{"无后缀", "AAPL", true, MarketUS, "AAPL", "AAPL.US", "gb_aapl", "", ""},
		{"无后缀", "aapl", true, MarketUS, "AAPL", "AAPL.US", "gb_aapl", "", ""},

		// —— 异常：空字符串 ——
		{"异常-空串", "", false, "", "", "", "", "", "empty"},
		{"异常-空串", "   ", false, "", "", "", "", "", "empty"},

		// —— 异常：中文名称 ——
		{"异常-中文", "平安银行", false, "", "", "", "", "", "name"},
		{"异常-中文", "腾讯控股", false, "", "", "", "", "", "name"},
		{"异常-中文", "苹果", false, "", "", "", "", "", "name"},

		// —— 异常：无法识别 ——
		{"异常-非法", "!!!", false, "", "", "", "", "", "unsupported"},
		{"异常-非法", "90.BK0475", false, "", "", "", "", "", "unsupported"}, // 板块 secid 非个股
	}

	t.Log("===== stock_code_normalize 转换表 =====")
	t.Logf("%-12s | %-14s | %-4s | %-8s | %-12s | %-12s | %-12s | %-10s",
		"类别", "输入", "OK", "market", "symbol", "ts_code", "sina", "em_secid")
	t.Log(strings.Repeat("-", 110))

	for _, r := range rows {
		name := r.category + "/" + r.input
		if r.input == "" {
			name = r.category + "/<empty>"
		}
		t.Run(name, func(t *testing.T) {
			n, err := NormalizeStockCode(r.input)
			ok := err == nil
			if ok != r.wantOK {
				t.Fatalf("input=%q wantOK=%v got err=%v", r.input, r.wantOK, err)
			}
			if !r.wantOK {
				if r.errPart != "" && (err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(r.errPart))) {
					t.Fatalf("input=%q err=%v want contains %q", r.input, err, r.errPart)
				}
				t.Logf("%-12s | %-14q | ERR  | %v", r.category, r.input, err)
				return
			}
			if n.Market != r.market || n.Symbol != r.symbol || n.TSCode != r.tsCode || n.SinaCode != r.sina {
				t.Fatalf("input=%q got market=%s symbol=%s ts=%s sina=%s; want %s %s %s %s",
					r.input, n.Market, n.Symbol, n.TSCode, n.SinaCode, r.market, r.symbol, r.tsCode, r.sina)
			}
			if r.emSecID != "" && n.EmSecID != r.emSecID {
				t.Fatalf("input=%q emSecID=%s want %s", r.input, n.EmSecID, r.emSecID)
			}
			if n.SecuCode != n.TSCode {
				t.Fatalf("secuCode should equal tsCode, got %s vs %s", n.SecuCode, n.TSCode)
			}
			line := fmt.Sprintf("%-12s | %-14s | OK   | %-6s | %-8s | %-12s | %-12s | %-10s",
				r.category, r.input, n.Market, n.Symbol, n.TSCode, n.SinaCode, n.EmSecID)
			t.Log(line)
		})
	}
}
