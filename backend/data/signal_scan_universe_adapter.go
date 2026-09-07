package data

import (
	"fmt"
	"strings"

	"go-stock/backend/models"
)

// AdaptUniverseCodeToStockInfo maps a sina A-share code (+ optional name/industry)
// into StockInfo so prepareStockBars / stockQuoteRowMap can run unchanged.
// Strategy-run NL rows use SECURITY_CODE / SECURITY_SHORT_NAME / NEWEST_PRICE;
// this adapter synthesizes SECUCODE / SECURITY_CODE / SECURITY_NAME_ABBR.
func AdaptUniverseCodeToStockInfo(sinaCode, name, industry string) models.StockInfo {
	sinaCode = strings.ToLower(strings.TrimSpace(sinaCode))
	name = strings.TrimSpace(name)
	industry = strings.TrimSpace(industry)

	info := models.StockInfo{
		SECURITYNAMEABBR: name,
		INDUSTRY:         industry,
	}
	n, err := NormalizeStockCode(sinaCode)
	if err != nil || n.Market != MarketCN {
		// Best-effort fallback: keep raw code for prepareStockBars filter.
		info.SECUCODE = sinaCode
		info.SECURITYCODE = stripMarketPrefix(sinaCode)
		return info
	}
	info.SECUCODE = n.SecuCode // e.g. 300274.SZ
	info.SECURITYCODE = n.Symbol
	info.MARKET = n.Exchange
	if info.SECURITYNAMEABBR == "" {
		info.SECURITYNAMEABBR = n.Symbol
	}
	return info
}

// AdaptStrategyRunRowToStockInfo adapts a raw eastmoney NL / technical dataList row.
func AdaptStrategyRunRowToStockInfo(row map[string]any) (models.StockInfo, error) {
	if row == nil {
		return models.StockInfo{}, fmt.Errorf("nil row")
	}
	codeRaw := firstMapString(row,
		"SECUCODE", "secucode",
		"SECURITY_CODE", "security_code",
		"stockCode", "StockCode", "code", "Code",
	)
	if codeRaw == "" {
		return models.StockInfo{}, fmt.Errorf("missing stock code")
	}
	n, err := NormalizeStockCode(codeRaw)
	if err != nil {
		return models.StockInfo{}, err
	}
	if n.Market != MarketCN {
		return models.StockInfo{}, fmt.Errorf("not CN: %s", codeRaw)
	}
	sina := strings.ToLower(strings.TrimSpace(n.SinaCode))
	if !IsAShareSinaCode(sina) {
		return models.StockInfo{}, fmt.Errorf("invalid sina: %s", sina)
	}
	name := firstMapString(row,
		"SECURITY_NAME_ABBR", "security_name_abbr",
		"SECURITY_SHORT_NAME", "security_short_name",
		"stockName", "StockName", "name", "Name",
	)
	industry := firstMapString(row, "INDUSTRY", "industry")
	info := AdaptUniverseCodeToStockInfo(sina, name, industry)

	// Optional quote overlays from NL field names → StockInfo quote fields.
	if v := firstMapString(row, "NEW_PRICE", "NEWEST_PRICE", "newest_price"); v != "" {
		info.NEWPRICE = v
	}
	if v := firstMapString(row, "CHANGE_RATE", "CHG", "PCHG"); v != "" {
		info.CHANGERATE = v
	}
	if v := firstMapString(row, "VOLUME"); v != "" {
		info.VOLUME = v
	}
	if v := firstMapString(row, "TURNOVERRATE", "TURNOVER_RATE"); v != "" {
		info.TURNOVERRATE = v
	}
	if v := firstMapString(row, "VOLUME_RATIO", "QRR"); v != "" {
		info.VOLUMERATIO = v
	}
	if v := firstMapString(row, "HIGH_PRICE", "PEAK_PRICE"); v != "" {
		info.HIGHPRICE = v
	}
	if v := firstMapString(row, "LOW_PRICE", "BOTTOM_PRICE"); v != "" {
		info.LOWPRICE = v
	}
	if mkt := firstMapString(row, "MARKET", "MARKET_SHORT_NAME"); mkt != "" {
		info.MARKET = mkt
	}
	return info, nil
}

func stripMarketPrefix(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	for _, p := range []string{"sh", "sz", "bj"} {
		if strings.HasPrefix(c, p) && len(c) > len(p) {
			return c[len(p):]
		}
	}
	return c
}

func firstMapString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}
