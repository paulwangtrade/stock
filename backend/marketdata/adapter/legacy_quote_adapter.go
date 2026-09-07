// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 QuoteService Legacy Adapter：委托 StockDataApi 实时行情，只读

package adapter

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

// legacyRealtimeFetcher 旧实时行情实现的最小方法集（data.StockDataApi 已满足）。
type legacyRealtimeFetcher interface {
	GetStockCodeRealTimeData(stockCodes ...string) (*[]data.StockInfo, error)
}

// LegacyQuoteAdapter marketdata.QuoteService 的旧实现桥接（纯委托、只读）。
//
// 不引入共享缓存 / TTL 策略（属 Phase7-A2），也不接触 Paper Trading、Execution、Position。
type LegacyQuoteAdapter struct {
	api legacyRealtimeFetcher
}

var _ marketdata.QuoteService = (*LegacyQuoteAdapter)(nil)

// NewLegacyQuoteAdapter 使用现有 StockDataApi 创建 Adapter。
func NewLegacyQuoteAdapter() *LegacyQuoteAdapter {
	return &LegacyQuoteAdapter{api: data.NewStockDataApi()}
}

// NewLegacyQuoteAdapterWith 注入自定义 fetcher，供测试或未来替换 provider 使用。
func NewLegacyQuoteAdapterWith(api legacyRealtimeFetcher) *LegacyQuoteAdapter {
	return &LegacyQuoteAdapter{api: api}
}

// GetQuote 实现 marketdata.QuoteService：单只标的行情快照。
func (a *LegacyQuoteAdapter) GetQuote(code string) (*marketdata.Quote, error) {
	c, err := marketdata.NormalizeCode(code)
	if err != nil {
		return nil, err
	}
	quotes, err := a.GetQuotes([]string{c})
	if err != nil {
		return nil, err
	}
	if q := marketdata.FindQuote(quotes, c); q != nil {
		out := *q
		return &out, nil
	}
	return nil, marketdata.ErrNoData
}

// GetQuotes 实现 marketdata.QuoteService：批量行情快照，顺序由上游返回决定。
func (a *LegacyQuoteAdapter) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	normalized, err := marketdata.NormalizeCodes(codes)
	if err != nil {
		return nil, err
	}
	if a == nil || a.api == nil {
		return nil, marketdata.ErrProviderUnavailable
	}

	raw, err := a.api.GetStockCodeRealTimeData(normalized...)
	if err != nil {
		return nil, err
	}
	if raw == nil || len(*raw) == 0 {
		return nil, marketdata.ErrNoData
	}

	fetchedAt := time.Now()
	quotes := make([]marketdata.Quote, 0, len(*raw))
	for _, info := range *raw {
		quotes = append(quotes, stockInfoToQuote(info, fetchedAt))
	}
	return quotes, nil
}

// stockInfoToQuote 只映射市场数据字段：持仓成本、盈亏、关注价、报警阈值等一律不进入本层。
// Bid/Ask prefer 竞买/竞卖，空则回退买一/卖一（B1P/A1P），对齐 App RealTimePrice 级联语义。
func stockInfoToQuote(info data.StockInfo, fetchedAt time.Time) marketdata.Quote {
	bid := parseFloat(info.Bid)
	if bid == 0 {
		bid = parseFloat(info.B1P)
	}
	ask := parseFloat(info.Ask)
	if ask == 0 {
		ask = parseFloat(info.A1P)
	}
	return marketdata.Quote{
		Code:          strings.TrimSpace(info.Code),
		Name:          strings.TrimSpace(info.Name),
		Price:         parseFloat(info.Price),
		Open:          parseFloat(info.Open),
		PreClose:      parseFloat(info.PreClose),
		High:          parseFloat(info.High),
		Low:           parseFloat(info.Low),
		Volume:        parseFloat(info.Volume),
		Amount:        parseFloat(info.Amount),
		ChangePercent: info.ChangePercent,
		ChangeValue:   info.ChangePrice,
		Bid:           bid,
		Ask:           ask,
		Market:        strings.TrimSpace(info.Market),
		Date:          strings.TrimSpace(info.Date),
		Time:          strings.TrimSpace(info.Time),
		FetchedAt:     fetchedAt,
	}
}
