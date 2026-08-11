// Holding Evaluation quote overlay — GET-time mark-to-market (Phase10-C.5-B.1).
// Read-only: never writes paper_sim_positions.mark_price.

package papertrading

import (
	"strings"
	"time"

	"go-stock/backend/marketdata"
)

// Holding Evaluation quote_source values (Observation DTO).
const (
	QuoteSourceTencent      = "tencent"
	QuoteSourcePositionMark = "position_mark"
)

// holdingEvalQuoteFetcher is injectable for tests; defaults to the shared QuoteService.
var holdingEvalQuoteFetcher = func() marketdata.QuoteService {
	return observationQuoteFetcher()
}

// SetHoldingEvalQuoteServiceForTest injects QuoteService for Holding Evaluation tests.
// Pass nil to restore the default (observation / data.GetQuoteService).
func SetHoldingEvalQuoteServiceForTest(svc marketdata.QuoteService) {
	if svc == nil {
		holdingEvalQuoteFetcher = func() marketdata.QuoteService {
			return observationQuoteFetcher()
		}
		return
	}
	holdingEvalQuoteFetcher = func() marketdata.QuoteService { return svc }
}

// quoteOverlayPriceProvider prefers QuoteService snapshots, then position MarkPrice.
type quoteOverlayPriceProvider struct {
	quotes   map[string]marketdata.Quote
	fallback MarketPriceProvider
}

// NewQuoteOverlayPriceProvider batches GetQuotes for attributed codes.
// svc nil / GetQuotes error / missing code → fallback position_mark. No DB writes.
func NewQuoteOverlayPriceProvider(attr *PositionAttributionView, svc marketdata.QuoteService) MarketPriceProvider {
	codes := holdingEvalSymbols(attr)
	return &quoteOverlayPriceProvider{
		quotes:   fetchObservationQuotes(svc, codes),
		fallback: NewAttributionMarkPriceProviderStrict(attr),
	}
}

func holdingEvalSymbols(attr *PositionAttributionView) []string {
	if attr == nil {
		return nil
	}
	out := make([]string, 0, len(attr.Positions))
	seen := map[string]bool{}
	for _, row := range attr.Positions {
		code := strings.TrimSpace(row.StockCode)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

// MarkPrice implements MarketPriceProvider.
func (p *quoteOverlayPriceProvider) MarkPrice(stockCode string, asOf time.Time) (MarkQuote, bool) {
	if p == nil {
		return MarkQuote{Quality: "missing", AsOf: asOf, Source: QuoteSourcePositionMark}, false
	}
	if q := findObservationQuote(p.quotes, stockCode); q != nil {
		px := q.Price
		if px <= 0 {
			px = q.Open
		}
		if px > 0 {
			ts := asOf
			if !q.FetchedAt.IsZero() {
				ts = q.FetchedAt
			}
			return MarkQuote{Price: px, AsOf: ts, Source: QuoteSourceTencent, Quality: "ok"}, true
		}
	}
	if p.fallback != nil {
		return p.fallback.MarkPrice(stockCode, asOf)
	}
	return MarkQuote{Quality: "missing", AsOf: asOf, Source: QuoteSourcePositionMark}, false
}
