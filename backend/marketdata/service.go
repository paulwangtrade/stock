// MarketDataService — unified read-only facade (Phase10-G1 / MD-001 M1).
// Does not replace QuoteService/KlineService; adapters compose them.
// Callers should migrate to this facade over time; UI must not know chromedp/providers.
//
// M1: no second-layer cache here — EastMoney kline_cache stays inside EastMoneyKLineApi.

package marketdata

import (
	"strings"
	"time"
)

// MarketDataService is the preferred application-facing market data boundary.
//
// Capability surface (provider-agnostic):
//   GetQuote / GetQuotes — realtime snapshots
//   GetBars / GetKline / GetHistory — OHLC series
//   GetMinute — intraday minute series
//
// Implementations must remain read-only: no orders, fills, or plan writes.
type MarketDataService interface {
	GetQuote(code string) (*Quote, error)
	GetQuotes(codes []string) ([]Quote, error)
	GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]Bar, error)
	GetKline(code string, period string, adjust string, limit int) ([]Bar, error)
	GetHistory(code string, period string, adjust string, from, to time.Time) ([]Bar, error)
	GetMinute(code string) (points []MinutePoint, asOfDate string, err error)
}

// CompositeMarketDataService delegates to Quote / primary Kline / optional secondary Kline / Minute.
type CompositeMarketDataService struct {
	Quotes QuoteService
	Klines KlineService
	// SecondaryKlines optional: PeriodDailyFQ / PeriodDailyHK (historical App Tencent day paths).
	SecondaryKlines KlineService
	Minutes         MinuteService
}

// NewCompositeMarketDataService wires quote + primary kline (backward compatible).
func NewCompositeMarketDataService(quotes QuoteService, klines KlineService) *CompositeMarketDataService {
	return &CompositeMarketDataService{Quotes: quotes, Klines: klines}
}

func (s *CompositeMarketDataService) GetQuote(code string) (*Quote, error) {
	if s == nil || s.Quotes == nil {
		return nil, ErrProviderUnavailable
	}
	return s.Quotes.GetQuote(code)
}

func (s *CompositeMarketDataService) GetQuotes(codes []string) ([]Quote, error) {
	if s == nil || s.Quotes == nil {
		return nil, ErrProviderUnavailable
	}
	return s.Quotes.GetQuotes(codes)
}

func (s *CompositeMarketDataService) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]Bar, error) {
	if s == nil {
		return nil, ErrProviderUnavailable
	}
	if isSecondaryDailyPeriod(period) {
		if s.SecondaryKlines == nil {
			return nil, ErrProviderUnavailable
		}
		return s.SecondaryKlines.GetBars(code, period, adjust, limit, endTime)
	}
	if s.Klines == nil {
		return nil, ErrProviderUnavailable
	}
	return s.Klines.GetBars(code, period, adjust, limit, endTime)
}

func (s *CompositeMarketDataService) GetKline(code string, period string, adjust string, limit int) ([]Bar, error) {
	return s.GetBars(code, period, adjust, limit, time.Time{})
}

func (s *CompositeMarketDataService) GetHistory(code string, period string, adjust string, from, to time.Time) ([]Bar, error) {
	if to.IsZero() {
		to = time.Now()
	}
	limit := 800
	bars, err := s.GetBars(code, period, adjust, limit, to)
	if err != nil {
		return nil, err
	}
	if from.IsZero() {
		return bars, nil
	}
	out := make([]Bar, 0, len(bars))
	for _, b := range bars {
		if b.Time.IsZero() || !b.Time.Before(from) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *CompositeMarketDataService) GetMinute(code string) ([]MinutePoint, string, error) {
	if s == nil || s.Minutes == nil {
		return nil, "", ErrProviderUnavailable
	}
	return s.Minutes.GetMinute(code)
}

func isSecondaryDailyPeriod(period string) bool {
	p := strings.ToLower(strings.TrimSpace(period))
	return p == PeriodDailyFQ || p == PeriodDailyHK
}

// Compile-time check.
var _ MarketDataService = (*CompositeMarketDataService)(nil)
