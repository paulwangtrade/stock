// @Author spark
// @Date 2026/8/9
// @Desc MD-001 M1：Tencent fqkline → KlineService（daily_fq / daily_hk），只读委托

package adapter

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

type tencentDayKlineFetcher interface {
	GetHK_KLineData(stockCode string, kLineType string, days int64) *[]data.KLineData
	GetCommonKLineData(stockCode string, kLineType string, days int64) *[]data.KLineData
}

// TencentKlineAdapter bridges StockDataApi day K-line endpoints to KlineService.
// Period aliases:
//   marketdata.PeriodDailyHK → GetHK_KLineData
//   marketdata.PeriodDailyFQ → GetCommonKLineData
type TencentKlineAdapter struct {
	api tencentDayKlineFetcher
}

var _ marketdata.KlineService = (*TencentKlineAdapter)(nil)

func NewTencentKlineAdapter() *TencentKlineAdapter {
	return &TencentKlineAdapter{api: data.NewStockDataApi()}
}

func NewTencentKlineAdapterWith(api tencentDayKlineFetcher) *TencentKlineAdapter {
	return &TencentKlineAdapter{api: api}
}

func (a *TencentKlineAdapter) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	_ = endTime // day endpoints are days-count based; endTime unused (parity with App legacy)
	_ = adjust
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, marketdata.ErrEmptyCode
	}
	if limit <= 0 {
		return nil, marketdata.ErrInvalidLimit
	}
	if a == nil || a.api == nil {
		return nil, marketdata.ErrProviderUnavailable
	}

	p := strings.ToLower(strings.TrimSpace(period))
	var raw *[]data.KLineData
	switch p {
	case marketdata.PeriodDailyHK:
		raw = a.api.GetHK_KLineData(code, "day", int64(limit))
	case marketdata.PeriodDailyFQ, "":
		raw = a.api.GetCommonKLineData(code, "day", int64(limit))
	default:
		return nil, marketdata.ErrUnsupportedPeriod
	}
	if raw == nil || len(*raw) == 0 {
		return nil, marketdata.ErrNoData
	}

	req := marketdata.BarsRequest{
		Code:   code,
		Period: p,
		Adjust: marketdata.AdjustForward,
		Limit:  limit,
		End:    marketdata.LatestEndFlag,
	}
	bars := make([]marketdata.Bar, 0, len(*raw))
	for _, k := range *raw {
		bars = append(bars, klineToBar(req, k))
	}
	return bars, nil
}
