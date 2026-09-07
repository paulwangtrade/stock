// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A2-3-1：Watchlist/Monitor 行情经 QuoteService；Quote→StockInfo 仅市场字段

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

var errQuoteServiceUninitialized = errors.New("QuoteService 未初始化")

// quoteBatchService is the minimal read surface used by App watchlist/realtime bridges.
// MarketDataService and QuoteService both satisfy it.
type quoteBatchService interface {
	GetQuotes(codes []string) ([]marketdata.Quote, error)
}

// quoteToStockInfo 将 marketdata.Quote 转为 StockInfo 行情字段（供缓存与 Follow 叠加）。
// 禁止映射 cost / position / profit / follow / alarm / order / plan。
func quoteToStockInfo(q marketdata.Quote) data.StockInfo {
	return data.StockInfo{
		Code:          strings.TrimSpace(q.Code),
		Name:          strings.TrimSpace(q.Name),
		Price:         formatQuoteFloat(q.Price),
		Open:          formatQuoteFloat(q.Open),
		PreClose:      formatQuoteFloat(q.PreClose),
		High:          formatQuoteFloat(q.High),
		Low:           formatQuoteFloat(q.Low),
		Volume:        formatQuoteFloat(q.Volume),
		Amount:        formatQuoteFloat(q.Amount),
		ChangePercent: q.ChangePercent,
		ChangePrice:   q.ChangeValue,
		Bid:           formatQuoteFloat(q.Bid),
		Ask:           formatQuoteFloat(q.Ask),
		Market:        strings.TrimSpace(q.Market),
		Date:          strings.TrimSpace(q.Date),
		Time:          strings.TrimSpace(q.Time),
	}
}

func formatQuoteFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// fetchRealtimeStockInfos 经 QuoteService/MarketDataService 批量取行情并映射为 StockInfo（可注入 fake，无 DB）。
func fetchRealtimeStockInfos(svc quoteBatchService, codes []string) ([]data.StockInfo, error) {
	if svc == nil {
		return nil, errQuoteServiceUninitialized
	}
	if len(codes) == 0 {
		return nil, nil
	}
	quotes, err := svc.GetQuotes(codes)
	if err != nil {
		return nil, err
	}
	out := make([]data.StockInfo, 0, len(quotes))
	for _, q := range quotes {
		out = append(out, quoteToStockInfo(q))
	}
	return out, nil
}

func quoteServiceFetchError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errQuoteServiceUninitialized) {
		return err
	}
	return fmt.Errorf("QuoteService.GetQuotes: %w", err)
}
