// @Author spark
// @Date 2026/8/9
// @Desc MD-001 M1：Tencent minute → MinuteService，只读委托

package adapter

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

type tencentMinuteFetcher interface {
	GetStockMinutePriceData(stockCode string) (*[]data.MinuteData, string)
}

// TencentMinuteAdapter bridges StockDataApi minute endpoint to MinuteService.
type TencentMinuteAdapter struct {
	api tencentMinuteFetcher
}

var _ marketdata.MinuteService = (*TencentMinuteAdapter)(nil)

func NewTencentMinuteAdapter() *TencentMinuteAdapter {
	return &TencentMinuteAdapter{api: data.NewStockDataApi()}
}

func NewTencentMinuteAdapterWith(api tencentMinuteFetcher) *TencentMinuteAdapter {
	return &TencentMinuteAdapter{api: api}
}

func (a *TencentMinuteAdapter) GetMinute(code string) ([]marketdata.MinutePoint, string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, "", marketdata.ErrEmptyCode
	}
	if a == nil || a.api == nil {
		return nil, "", marketdata.ErrProviderUnavailable
	}
	raw, date := a.api.GetStockMinutePriceData(code)
	if raw == nil || len(*raw) == 0 {
		return nil, strings.TrimSpace(date), marketdata.ErrNoData
	}
	out := make([]marketdata.MinutePoint, 0, len(*raw))
	for _, m := range *raw {
		out = append(out, marketdata.MinutePoint{
			Time:   m.Time,
			Price:  m.Price,
			Volume: m.Volume,
			Amount: m.Amount,
		})
	}
	return out, strings.TrimSpace(date), nil
}
