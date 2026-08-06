package papertrading

import (
	"strconv"
	"strings"
	"sync"

	"go-stock/backend/data"
)

// quoteFetcher abstracts realtime quote lookup (injectable for tests).
type quoteFetcher func(codes ...string) (*[]data.StockInfo, error)

var (
	quoteFetchMu sync.RWMutex
	quoteFetch   quoteFetcher = defaultRealtimeQuoteFetch
)

func defaultRealtimeQuoteFetch(codes ...string) (*[]data.StockInfo, error) {
	return data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
}

// SetQuoteFetcherForTest replaces the realtime fetcher (tests). Pass nil to restore.
func SetQuoteFetcherForTest(fn quoteFetcher) {
	quoteFetchMu.Lock()
	defer quoteFetchMu.Unlock()
	if fn == nil {
		quoteFetch = defaultRealtimeQuoteFetch
	} else {
		quoteFetch = fn
	}
}

// RealtimeOpenPriceProvider reads today's open from live quotes.
// It NEVER falls back to planned/limit price — missing/invalid open → (Quote{}, false).
type RealtimeOpenPriceProvider struct{}

func (RealtimeOpenPriceProvider) OpenQuote(symbol, _ string) (Quote, bool) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return Quote{}, false
	}
	quoteFetchMu.RLock()
	fn := quoteFetch
	quoteFetchMu.RUnlock()
	if fn == nil {
		return Quote{}, false
	}
	infos, err := fn(symbol)
	if err != nil || infos == nil || len(*infos) == 0 {
		return Quote{}, false
	}
	info := (*infos)[0]
	open, ok := parsePositiveFloat(info.Open)
	if !ok {
		return Quote{}, false
	}
	q := Quote{Open: open}
	// Limit up/down not always present on StockInfo; leave 0 → broker skips ceiling check.
	return q, true
}

// MarkPrice returns the latest tradeable price for EOD mark-to-market.
// Prefers current Price; falls back to Open. Never invents a price.
func (RealtimeOpenPriceProvider) MarkPrice(symbol string) (float64, bool) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return 0, false
	}
	quoteFetchMu.RLock()
	fn := quoteFetch
	quoteFetchMu.RUnlock()
	if fn == nil {
		return 0, false
	}
	infos, err := fn(symbol)
	if err != nil || infos == nil || len(*infos) == 0 {
		return 0, false
	}
	info := (*infos)[0]
	if p, ok := parsePositiveFloat(info.Price); ok {
		return p, true
	}
	return parsePositiveFloat(info.Open)
}

// DefaultOpenPriceProvider is the production fill price source (open only).
func DefaultOpenPriceProvider() PriceProvider {
	return RealtimeOpenPriceProvider{}
}

func parsePositiveFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
