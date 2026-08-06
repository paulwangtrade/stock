package papertrading

import "strings"

// Quote is the minimal open-auction/open-price view the MVP fill engine consumes.
// LimitUp/LimitDown of 0 means "unknown" and the corresponding ceiling check is skipped.
type Quote struct {
	Open      float64
	LimitUp   float64
	LimitDown float64
}

// PriceProvider yields the trade-day open price for a symbol.
// Implementations MUST NOT fabricate prices; a missing/invalid quote must
// surface as (Quote{}, false) so the broker can fail closed.
type PriceProvider interface {
	OpenQuote(symbol, tradeDate string) (Quote, bool)
}

// MissingPriceProvider always reports "no data" → every fill is rejected
// (missing_open_price). This is the safe default when no data source is wired,
// ensuring Paper Trading never invents fills in production.
type MissingPriceProvider struct{}

func (MissingPriceProvider) OpenQuote(_, _ string) (Quote, bool) { return Quote{}, false }

// StaticPriceProvider is an in-memory provider for tests / deterministic runs.
type StaticPriceProvider struct {
	Quotes map[string]Quote // key: normalized symbol
}

func (p StaticPriceProvider) OpenQuote(symbol, _ string) (Quote, bool) {
	if p.Quotes == nil {
		return Quote{}, false
	}
	q, ok := p.Quotes[normalizeSymbol(symbol)]
	if !ok || q.Open <= 0 {
		return Quote{}, false
	}
	return q, true
}

func normalizeSymbol(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
