package anchor

import (
	"strings"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

// RefSourceKlineClose is written when Resolve succeeds from daily K-line Close.
const RefSourceKlineClose = "kline_close"

const (
	klineCloseConfidence = 0.8
	klineCloseLookback   = 16
)

// KlineCloseAnchorProvider resolves after-close Intent ref_price from daily Close
// on Context.SourceDate via marketdata.KlineService (EastMoney + kline_cache).
//
// It never uses TradeDate, realtime quotes, or Open as ref_price.
type KlineCloseAnchorProvider struct {
	// Klines is optional; nil uses EastMoneyKlineAdapter (production default).
	Klines marketdata.KlineService
}

// Resolve implements Provider.
func (p KlineCloseAnchorProvider) Resolve(ctx Context) (Result, bool) {
	code := strings.TrimSpace(ctx.StockCode)
	sourceDate := strings.TrimSpace(ctx.SourceDate)
	if code == "" || sourceDate == "" {
		return Result{}, false
	}
	if _, err := time.ParseInLocation("2006-01-02", sourceDate, time.Local); err != nil {
		return Result{}, false
	}

	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", sourceDate+" 15:00:00", time.Local)
	if err != nil {
		return Result{}, false
	}

	svc := p.Klines
	if svc == nil {
		svc = defaultKlineService()
	}
	if svc == nil {
		return Result{}, false
	}

	bars, err := svc.GetBars(code, marketdata.PeriodDay, marketdata.AdjustNone, klineCloseLookback, endTime)
	if err != nil || len(bars) == 0 {
		return Result{}, false
	}

	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if barCalendarDay(b) != sourceDate {
			continue
		}
		// Exact SourceDate bar found: Close>0 only — never fall back to Open/High/Low.
		if b.Close <= 0 {
			return Result{}, false
		}
		return Result{
			RefPrice:   b.Close,
			RefSource:  RefSourceKlineClose,
			RefAsOf:    sourceDate,
			Confidence: klineCloseConfidence,
		}, true
	}
	return Result{}, false
}

func barCalendarDay(b marketdata.Bar) string {
	if !b.Time.IsZero() {
		return b.Time.Format("2006-01-02")
	}
	if t := marketdata.ParseBarTime(strings.TrimSpace(b.TimeText)); !t.IsZero() {
		return t.Format("2006-01-02")
	}
	s := strings.TrimSpace(b.TimeText)
	if len(s) >= 10 {
		cand := s[:10]
		if _, err := time.ParseInLocation("2006-01-02", cand, time.Local); err == nil {
			return cand
		}
	}
	return ""
}

func defaultKlineService() marketdata.KlineService {
	return adapter.NewEastMoneyKlineAdapter()
}
