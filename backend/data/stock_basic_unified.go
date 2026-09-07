package data

import (
	"fmt"
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

const defaultStockBasicListLimit = 500

// UnifiedStockBasic 股票基础信息统一视图（仅 data 层返回模型，非数据库表）。
type UnifiedStockBasic struct {
	ID          string `json:"id"`          // 逻辑键：{market}:{tsCode}
	Market      string `json:"market"`      // CN | HK | US
	Symbol      string `json:"symbol"`
	TSCode      string `json:"tsCode"`
	SecuCode    string `json:"secuCode"`
	SinaCode    string `json:"sinaCode"`
	Name        string `json:"name"`
	Exchange    string `json:"exchange"`
	ListStatus  string `json:"listStatus,omitempty"`
	Industry    string `json:"industry,omitempty"`
	BKCode      string `json:"bkCode,omitempty"`
	BKName      string `json:"bkName,omitempty"`
	SourceTable string `json:"sourceTable"`
	SourceID    uint   `json:"sourceId"`
}

// GetStockBasicByCode 按任意常见编码查询一条统一基础信息。
func (receiver StockDataApi) GetStockBasicByCode(code string) (*UnifiedStockBasic, error) {
	norm, err := NormalizeStockCode(code)
	if err != nil {
		return nil, err
	}
	switch norm.Market {
	case MarketCN:
		return receiver.lookupCNBasic(norm)
	case MarketHK:
		return receiver.lookupHKBasic(norm)
	case MarketUS:
		return receiver.lookupUSBasic(norm)
	default:
		return nil, fmt.Errorf("unsupported market: %s", norm.Market)
	}
}

// GetStockBasicList 按市场返回基础列表；market 为空则按 CN→HK→US 拼接。
// 默认最多 defaultStockBasicListLimit 条，避免一次拉满美股全表。
func (receiver StockDataApi) GetStockBasicList(market string) []UnifiedStockBasic {
	return receiver.GetStockBasicListLimit(market, defaultStockBasicListLimit)
}

// GetStockBasicListLimit 带上限的市场列表查询。
func (receiver StockDataApi) GetStockBasicListLimit(market string, limit int) []UnifiedStockBasic {
	if limit <= 0 {
		limit = defaultStockBasicListLimit
	}
	m := strings.ToUpper(strings.TrimSpace(market))
	out := make([]UnifiedStockBasic, 0, limit)
	switch m {
	case "", "*":
		remain := limit
		for _, part := range []string{MarketCN, MarketHK, MarketUS} {
			if remain <= 0 {
				break
			}
			chunk := receiver.listByMarket(part, remain)
			out = append(out, chunk...)
			remain = limit - len(out)
		}
	case MarketCN, "A", "ASHARE", "CN_A":
		out = receiver.listByMarket(MarketCN, limit)
	case MarketHK:
		out = receiver.listByMarket(MarketHK, limit)
	case MarketUS:
		out = receiver.listByMarket(MarketUS, limit)
	default:
		// 无法识别市场时返回空，避免误扫全库
		return []UnifiedStockBasic{}
	}
	return out
}

// GetStockBasicBySymbol 按纯交易代码查询（可能多市场；优先精确命中）。
func (receiver StockDataApi) GetStockBasicBySymbol(symbol string) []UnifiedStockBasic {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil
	}
	// 若带市场后缀，走 ByCode 单条路径
	if strings.Contains(symbol, ".") || reCNPrefixed.MatchString(symbol) || reHKPrefixed.MatchString(symbol) || reUSPrefixed.MatchString(symbol) {
		if one, err := receiver.GetStockBasicByCode(symbol); err == nil && one != nil {
			return []UnifiedStockBasic{*one}
		}
	}

	out := make([]UnifiedStockBasic, 0, 3)
	normGuess, err := NormalizeStockCode(symbol)
	if err == nil {
		if one, e := receiver.GetStockBasicByCode(normGuess.TSCode); e == nil && one != nil {
			out = append(out, *one)
			return out
		}
	}

	// 回退：各市场按 symbol / code 模糊精确查
	digits := RemoveAllNonDigitChar(symbol)
	upper := strings.ToUpper(symbol)

	if digits != "" {
		var cn StockBasic
		if err := db.Dao.Model(&StockBasic{}).
			Where("symbol = ? OR ts_code LIKE ?", padCNSymbol(digits), "%"+padCNSymbol(digits)+"%").
			Order("list_status = 'L' DESC").
			First(&cn).Error; err == nil && cn.ID > 0 {
			out = append(out, mapCNBasic(cn))
		}
		hkSym := padHKSymbol(digits)
		var hk models.StockInfoHK
		if err := db.Dao.Model(&models.StockInfoHK{}).
			Where("code = ? OR code = ?", hkSym+".HK", "0"+hkSym+".HK").
			First(&hk).Error; err == nil && hk.ID > 0 {
			out = append(out, mapHKBasic(hk))
		}
	}

	var us models.StockInfoUS
	if err := db.Dao.Model(&models.StockInfoUS{}).
		Where("code = ? OR code = ? OR UPPER(code) = ?", upper+".US", "US"+upper, upper).
		First(&us).Error; err == nil && us.ID > 0 {
		out = append(out, mapUSBasic(us))
	}

	return out
}

func (receiver StockDataApi) listByMarket(market string, limit int) []UnifiedStockBasic {
	out := make([]UnifiedStockBasic, 0, limit)
	switch market {
	case MarketCN:
		var rows []StockBasic
		db.Dao.Model(&StockBasic{}).Order("ts_code asc").Limit(limit).Find(&rows)
		for _, r := range rows {
			out = append(out, mapCNBasic(r))
		}
	case MarketHK:
		var rows []models.StockInfoHK
		db.Dao.Model(&models.StockInfoHK{}).Order("code asc").Limit(limit).Find(&rows)
		for _, r := range rows {
			out = append(out, mapHKBasic(r))
		}
	case MarketUS:
		var rows []models.StockInfoUS
		db.Dao.Model(&models.StockInfoUS{}).Order("code asc").Limit(limit).Find(&rows)
		for _, r := range rows {
			out = append(out, mapUSBasic(r))
		}
	}
	return out
}

func (receiver StockDataApi) lookupCNBasic(norm NormalizedStockCode) (*UnifiedStockBasic, error) {
	var row StockBasic
	q := db.Dao.Model(&StockBasic{}).Where(
		"ts_code = ? OR ts_code = ? OR symbol = ?",
		norm.TSCode, strings.ToLower(norm.TSCode), norm.Symbol,
	)
	if err := q.Order("list_status = 'L' DESC").First(&row).Error; err != nil {
		return nil, fmt.Errorf("CN stock not found for %s (table=tushare_stock_basic): %w", norm.Input, err)
	}
	u := mapCNBasic(row)
	return &u, nil
}

func (receiver StockDataApi) lookupHKBasic(norm NormalizedStockCode) (*UnifiedStockBasic, error) {
	candidates := []string{
		norm.TSCode,
		norm.Symbol + ".HK",
		strings.TrimLeft(norm.Symbol, "0") + ".HK",
	}
	var row models.StockInfoHK
	if err := db.Dao.Model(&models.StockInfoHK{}).Where("code IN ?", candidates).First(&row).Error; err != nil {
		// 再试：code 以 symbol 结尾
		if err2 := db.Dao.Model(&models.StockInfoHK{}).
			Where("code LIKE ?", "%"+norm.Symbol+".HK").
			First(&row).Error; err2 != nil {
			return nil, fmt.Errorf("HK stock not found for %s (table=stock_base_info_hk): %w", norm.Input, err)
		}
	}
	u := mapHKBasic(row)
	return &u, nil
}

func (receiver StockDataApi) lookupUSBasic(norm NormalizedStockCode) (*UnifiedStockBasic, error) {
	candidates := []string{
		norm.TSCode,
		norm.Symbol + ".US",
		"US" + norm.Symbol,
		norm.Symbol,
	}
	var row models.StockInfoUS
	if err := db.Dao.Model(&models.StockInfoUS{}).Where("code IN ?", candidates).First(&row).Error; err != nil {
		upper := strings.ToUpper(norm.Symbol)
		if err2 := db.Dao.Model(&models.StockInfoUS{}).
			Where("UPPER(code) IN ?", []string{upper + ".US", "US" + upper, upper}).
			First(&row).Error; err2 != nil {
			return nil, fmt.Errorf("US stock not found for %s (table=stock_base_info_us): %w", norm.Input, err)
		}
	}
	u := mapUSBasic(row)
	return &u, nil
}

func mapCNBasic(row StockBasic) UnifiedStockBasic {
	exSuffix := "SZ"
	if parts := strings.Split(row.TsCode, "."); len(parts) == 2 {
		exSuffix = strings.ToUpper(parts[1])
	} else if row.Exchange != "" {
		switch strings.ToUpper(row.Exchange) {
		case "SSE", "SH":
			exSuffix = "SH"
		case "SZSE", "SZ":
			exSuffix = "SZ"
		case "BSE", "BJSE", "BJ":
			exSuffix = "BJ"
		}
	}
	symbol := row.Symbol
	if symbol == "" {
		symbol = RemoveAllNonDigitChar(row.TsCode)
	}
	norm := buildNormalized(row.TsCode, MarketCN, padCNSymbol(symbol), exSuffix)
	exchange := row.Exchange
	if exchange == "" {
		exchange = norm.Exchange
	}
	return UnifiedStockBasic{
		ID:          MarketCN + ":" + norm.TSCode,
		Market:      MarketCN,
		Symbol:      norm.Symbol,
		TSCode:      firstNonEmpty(row.TsCode, norm.TSCode),
		SecuCode:    firstNonEmpty(row.TsCode, norm.SecuCode),
		SinaCode:    norm.SinaCode,
		Name:        row.Name,
		Exchange:    exchange,
		ListStatus:  row.ListStatus,
		Industry:    firstNonEmpty(row.Industry, row.BKName),
		BKCode:      row.BKCode,
		BKName:      row.BKName,
		SourceTable: "tushare_stock_basic",
		SourceID:    row.ID,
	}
}

func mapHKBasic(row models.StockInfoHK) UnifiedStockBasic {
	norm, err := NormalizeStockCode(row.Code)
	if err != nil {
		digits := RemoveAllNonDigitChar(row.Code)
		norm = buildNormalized(row.Code, MarketHK, padHKSymbol(digits), "HK")
	}
	return UnifiedStockBasic{
		ID:          MarketHK + ":" + norm.TSCode,
		Market:      MarketHK,
		Symbol:      norm.Symbol,
		TSCode:      firstNonEmpty(row.Code, norm.TSCode),
		SecuCode:    firstNonEmpty(row.Code, norm.SecuCode),
		SinaCode:    norm.SinaCode,
		Name:        row.Name,
		Exchange:    "HKEX",
		Industry:    row.BKName,
		BKCode:      row.BKCode,
		BKName:      row.BKName,
		SourceTable: "stock_base_info_hk",
		SourceID:    row.ID,
	}
}

func mapUSBasic(row models.StockInfoUS) UnifiedStockBasic {
	code := row.Code
	norm, err := NormalizeStockCode(code)
	if err != nil {
		sym := strings.TrimSuffix(strings.TrimSuffix(strings.ToUpper(code), ".US"), "US")
		sym = strings.TrimPrefix(sym, "US")
		norm = buildNormalized(code, MarketUS, sym, "US")
	}
	exchange := row.Exchange
	if exchange == "" {
		exchange = "US"
	}
	return UnifiedStockBasic{
		ID:          MarketUS + ":" + norm.TSCode,
		Market:      MarketUS,
		Symbol:      norm.Symbol,
		TSCode:      firstNonEmpty(ensureUSTsCode(code), norm.TSCode),
		SecuCode:    firstNonEmpty(ensureUSTsCode(code), norm.SecuCode),
		SinaCode:    norm.SinaCode,
		Name:        row.Name,
		Exchange:    exchange,
		Industry:    row.BKName,
		BKCode:      row.BKCode,
		BKName:      row.BKName,
		SourceTable: "stock_base_info_us",
		SourceID:    row.ID,
	}
}

func ensureUSTsCode(code string) string {
	c := strings.TrimSpace(code)
	upper := strings.ToUpper(c)
	if strings.HasSuffix(upper, ".US") {
		return upper
	}
	if strings.HasPrefix(upper, "US") && !strings.Contains(upper, ".") {
		return strings.TrimPrefix(upper, "US") + ".US"
	}
	if looksLikeUSTicker(c) {
		return upper + ".US"
	}
	return c
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
