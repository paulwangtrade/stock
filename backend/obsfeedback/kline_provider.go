package obsfeedback

import (
	"strconv"
	"strings"

	"go-stock/backend/data"
)

// KLineSeriesProvider adapts EastMoney daily kline (with tencent fallback) for outcomes.
type KLineSeriesProvider struct {
	API *data.EastMoneyKLineApi
}

func NewKLineSeriesProvider() *KLineSeriesProvider {
	return &KLineSeriesProvider{API: data.NewEastMoneyKLineApi(nil)}
}

func (p *KLineSeriesProvider) DailyBars(symbol string, fromDate string, limit int) ([]Bar, error) {
	if p == nil || p.API == nil {
		return nil, nil
	}
	if limit < 8 {
		limit = 40
	}
	raw := p.API.GetKLineData(symbol, "101", "", limit)
	if raw == nil {
		return nil, nil
	}
	fromDate = datePrefix(fromDate)
	out := make([]Bar, 0, len(*raw))
	for _, k := range *raw {
		d := ParseKLineDay(k.Day)
		if d == "" {
			continue
		}
		closePx, err1 := strconv.ParseFloat(strings.TrimSpace(k.Close), 64)
		if err1 != nil || closePx <= 0 {
			continue
		}
		high, _ := strconv.ParseFloat(strings.TrimSpace(k.High), 64)
		low, _ := strconv.ParseFloat(strings.TrimSpace(k.Low), 64)
		if fromDate != "" && d < fromDate {
			continue
		}
		out = append(out, Bar{Date: d, Close: closePx, High: high, Low: low})
	}
	return out, nil
}

// MapSeriesProvider is an in-memory SeriesProvider for tests / API fakes.
type MapSeriesProvider struct {
	Bars map[string][]Bar
}

func (p MapSeriesProvider) DailyBars(symbol string, fromDate string, limit int) ([]Bar, error) {
	if p.Bars == nil {
		return nil, nil
	}
	return p.Bars[symbol], nil
}
