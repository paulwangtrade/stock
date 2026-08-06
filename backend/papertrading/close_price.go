package papertrading

import (
	"strings"
	"sync"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

const closeFillLookback = 16

// CloseFillProvider resolves the trade-date daily K-line Close for Session B fills.
// It never falls back to Open/High/Low/realtime. Missing/invalid close → (Quote{}, false).
//
// Does not use TradePlan Anchor (anchor.KlineCloseAnchorProvider): Fill uses trade_date
// as the execution calendar day, not Intent source_date.
type CloseFillProvider struct {
	// Klines is optional; nil uses EastMoneyKlineAdapter (or test override).
	Klines marketdata.KlineService
}

var (
	closeKlineMu sync.RWMutex
	closeKlineFn func() marketdata.KlineService
)

// SetCloseKlineServiceForTest overrides the default KlineService for CloseFillProvider.
// Pass nil to restore.
func SetCloseKlineServiceForTest(fn func() marketdata.KlineService) {
	closeKlineMu.Lock()
	defer closeKlineMu.Unlock()
	closeKlineFn = fn
}

func defaultCloseKlineService() marketdata.KlineService {
	closeKlineMu.RLock()
	fn := closeKlineFn
	closeKlineMu.RUnlock()
	if fn != nil {
		if svc := fn(); svc != nil {
			return svc
		}
	}
	return adapter.NewEastMoneyKlineAdapter()
}

// OpenQuote implements PriceProvider. On success Quote.Open holds the Close price
// (compat with existing Broker) and PriceKind is PriceKindClose.
func (p CloseFillProvider) OpenQuote(symbol, tradeDate string) (Quote, bool) {
	symbol = strings.TrimSpace(symbol)
	tradeDate = strings.TrimSpace(tradeDate)
	if symbol == "" || tradeDate == "" {
		return Quote{}, false
	}
	closePx, ok := resolveDailyClose(p.klines(), symbol, tradeDate)
	if !ok || closePx <= 0 {
		return Quote{}, false
	}
	return Quote{Open: closePx, PriceKind: PriceKindClose}, true
}

func (p CloseFillProvider) klines() marketdata.KlineService {
	if p.Klines != nil {
		return p.Klines
	}
	return defaultCloseKlineService()
}

// resolveDailyClose loads daily bars ending on tradeDate and returns Close for that calendar day only.
// Never falls back to Open/High/Low.
func resolveDailyClose(svc marketdata.KlineService, symbol, tradeDate string) (float64, bool) {
	if svc == nil {
		return 0, false
	}
	if _, err := time.ParseInLocation("2006-01-02", tradeDate, time.Local); err != nil {
		return 0, false
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", tradeDate+" 15:00:00", time.Local)
	if err != nil {
		return 0, false
	}
	bars, err := svc.GetBars(symbol, marketdata.PeriodDay, marketdata.AdjustNone, closeFillLookback, endTime)
	if err != nil || len(bars) == 0 {
		return 0, false
	}
	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if fillBarCalendarDay(b) != tradeDate {
			continue
		}
		if b.Close <= 0 {
			return 0, false
		}
		return b.Close, true
	}
	return 0, false
}

func fillBarCalendarDay(b marketdata.Bar) string {
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
