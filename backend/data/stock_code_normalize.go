package data

import (
	"fmt"
	"regexp"
	"strings"
)

// 统一市场枚举（只读查询层使用，不写库）
const (
	MarketCN = "CN"
	MarketHK = "HK"
	MarketUS = "US"
)

var (
	reDigitsOnly     = regexp.MustCompile(`^\d{1,6}$`)
	reCNPrefixed     = regexp.MustCompile(`(?i)^(sh|sz|bj)(\d{6})$`)
	reHKPrefixed     = regexp.MustCompile(`(?i)^hk(\d{1,5})$`)
	reUSPrefixed     = regexp.MustCompile(`(?i)^(?:gb_|us)([a-z0-9.\-]+)$`)
	reDottedExchange = regexp.MustCompile(`(?i)^([a-z0-9.\-]+)\.(sh|sz|bj|ss|hk|us)$`)
	// 东财 secid：1.600519(沪) / 0.000001(深/北) / 128.00700(港)
	reEastMoneySecID = regexp.MustCompile(`^(\d+)\.(\d{4,6})$`)
	reHasHan         = regexp.MustCompile(`\p{Han}`)
)

// NormalizedStockCode 编码归一结果（非持久化）。
type NormalizedStockCode struct {
	Input    string `json:"input"`
	Market   string `json:"market"`   // CN | HK | US
	Symbol   string `json:"symbol"`   // 000001 / 00700 / AAPL
	TSCode   string `json:"tsCode"`   // 000001.SZ / 00700.HK / AAPL.US
	SecuCode string `json:"secuCode"` // 与东财 SECUCODE 对齐（多数同 TSCode）
	SinaCode string `json:"sinaCode"` // sz000001 / hk00700 / gb_aapl
	EmSecID  string `json:"emSecId"`  // 东财 K 线 secid：1.600519
	Exchange string `json:"exchange"` // SSE/SZSE/BJSE/HKEX/US 等推断值
}

// NormalizeStockCode 将多种输入码统一为 market + symbol + 标准 ts_code。
// 支持：000001.SZ、00700.HK、AAPL.US、sz300408、300408、hk00700、gb_aapl、
// 东财 secid（1.600519 / 0.000001 / 128.00700）。中文名称与空串返回 error。
func NormalizeStockCode(code string) (NormalizedStockCode, error) {
	raw := strings.TrimSpace(code)
	if raw == "" {
		return NormalizedStockCode{}, fmt.Errorf("empty stock code")
	}
	input := raw
	c := strings.TrimSpace(raw)

	// 中文名称不是证券代码
	if reHasHan.MatchString(c) {
		return NormalizedStockCode{}, fmt.Errorf("not a stock code (looks like name): %s", input)
	}

	// 0) 东财 secid：必须在「交易所后缀」解析之前（避免与 000001.SZ 混淆；secid 左侧为市场号）
	if m := reEastMoneySecID.FindStringSubmatch(c); len(m) == 3 {
		marketNo, num := m[1], m[2]
		switch marketNo {
		case "1":
			symbol := padCNSymbol(num)
			return buildNormalized(input, MarketCN, symbol, "SH"), nil
		case "0":
			symbol := padCNSymbol(num)
			return buildNormalized(input, MarketCN, symbol, inferCNExchange(symbol)), nil
		case "128":
			symbol := padHKSymbol(num)
			return buildNormalized(input, MarketHK, symbol, "HK"), nil
		default:
			return NormalizedStockCode{}, fmt.Errorf("unsupported eastmoney secid market %s: %s", marketNo, input)
		}
	}

	// 1) 前缀形式：sz300408 / hk00700 / gb_aapl
	if m := reCNPrefixed.FindStringSubmatch(c); len(m) == 3 {
		ex := strings.ToUpper(m[1])
		if ex == "SS" {
			ex = "SH"
		}
		symbol := m[2]
		return buildNormalized(input, MarketCN, symbol, ex), nil
	}
	if m := reHKPrefixed.FindStringSubmatch(c); len(m) == 2 {
		symbol := padHKSymbol(m[1])
		return buildNormalized(input, MarketHK, symbol, "HK"), nil
	}
	if m := reUSPrefixed.FindStringSubmatch(c); len(m) == 2 {
		symbol := strings.ToUpper(m[1])
		return buildNormalized(input, MarketUS, symbol, "US"), nil
	}

	// 2) 点分形式：000001.SZ / 00700.HK / AAPL.US（大小写均可）
	if m := reDottedExchange.FindStringSubmatch(c); len(m) == 3 {
		left := m[1]
		ex := strings.ToUpper(m[2])
		if ex == "SS" {
			ex = "SH"
		}
		switch ex {
		case "SH", "SZ", "BJ":
			symbol := padCNSymbol(RemoveAllNonDigitChar(left))
			if symbol == "" {
				return NormalizedStockCode{}, fmt.Errorf("invalid CN code: %s", input)
			}
			return buildNormalized(input, MarketCN, symbol, ex), nil
		case "HK":
			symbol := padHKSymbol(RemoveAllNonDigitChar(left))
			if symbol == "" {
				return NormalizedStockCode{}, fmt.Errorf("invalid HK code: %s", input)
			}
			return buildNormalized(input, MarketHK, symbol, "HK"), nil
		case "US":
			symbol := strings.ToUpper(left)
			return buildNormalized(input, MarketUS, symbol, "US"), nil
		}
	}

	// 3) 纯数字：A 股 6 位，或港股 1–5 位（不带市场后缀）
	if reDigitsOnly.MatchString(c) {
		digits := c
		if len(digits) == 6 {
			ex := inferCNExchange(digits)
			return buildNormalized(input, MarketCN, digits, ex), nil
		}
		if len(digits) <= 5 {
			symbol := padHKSymbol(digits)
			return buildNormalized(input, MarketHK, symbol, "HK"), nil
		}
	}

	// 4) 纯美股 ticker（无后缀）
	if looksLikeUSTicker(c) {
		symbol := strings.ToUpper(c)
		return buildNormalized(input, MarketUS, symbol, "US"), nil
	}

	return NormalizedStockCode{}, fmt.Errorf("unsupported stock code: %s", input)
}

func buildNormalized(input, market, symbol, exchangeSuffix string) NormalizedStockCode {
	n := NormalizedStockCode{
		Input:  input,
		Market: market,
		Symbol: symbol,
	}
	switch market {
	case MarketCN:
		n.TSCode = symbol + "." + exchangeSuffix
		n.SecuCode = n.TSCode
		n.SinaCode = strings.ToLower(exchangeSuffix) + symbol
		n.Exchange = cnExchangeName(exchangeSuffix)
		n.EmSecID = cnEmSecID(symbol, exchangeSuffix)
	case MarketHK:
		n.TSCode = symbol + ".HK"
		n.SecuCode = n.TSCode
		n.SinaCode = "hk" + symbol
		n.Exchange = "HKEX"
		n.EmSecID = "128." + symbol
	case MarketUS:
		n.TSCode = symbol + ".US"
		n.SecuCode = n.TSCode
		n.SinaCode = "gb_" + strings.ToLower(symbol)
		n.Exchange = "US"
		// 美股东财 secid 规则因源而异，统一查询层暂不编造
		n.EmSecID = ""
	}
	return n
}

func cnEmSecID(symbol, exchangeSuffix string) string {
	switch strings.ToUpper(exchangeSuffix) {
	case "SH":
		return "1." + symbol
	case "SZ", "BJ":
		return "0." + symbol
	default:
		return "0." + symbol
	}
}

func padCNSymbol(digits string) string {
	digits = strings.TrimSpace(digits)
	if digits == "" {
		return ""
	}
	if len(digits) >= 6 {
		return digits[len(digits)-6:]
	}
	return strings.Repeat("0", 6-len(digits)) + digits
}

func padHKSymbol(digits string) string {
	digits = strings.TrimSpace(digits)
	if digits == "" {
		return ""
	}
	// 库内港股多为 5 位：00700
	if len(digits) >= 5 {
		return digits[len(digits)-5:]
	}
	return strings.Repeat("0", 5-len(digits)) + digits
}

func inferCNExchange(symbol6 string) string {
	if len(symbol6) < 1 {
		return "SZ"
	}
	switch symbol6[0] {
	case '6':
		return "SH"
	case '0', '3':
		return "SZ"
	case '4', '8', '9':
		return "BJ"
	default:
		return "SZ"
	}
}

func cnExchangeName(suffix string) string {
	switch strings.ToUpper(suffix) {
	case "SH":
		return "SSE"
	case "SZ":
		return "SZSE"
	case "BJ":
		return "BJSE"
	default:
		return strings.ToUpper(suffix)
	}
}

func looksLikeUSTicker(s string) bool {
	if s == "" || strings.Contains(s, ".") {
		return false
	}
	if reDigitsOnly.MatchString(s) {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}
