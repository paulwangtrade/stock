package readmodel

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
)

// Query selects as-of / trade_date / display overlay.
type Query struct {
	TradeDate      string
	AsOf           time.Time
	IncludeDisplay bool
}

// Service loads Snapshot + lots (+ optional quotes) and Builds the read model.
type Service struct {
	portfolio portfolio.Service
	quotes    func() marketdata.QuoteService
}

// NewService wires default readers. nil portfolio → portfolio.NewService().
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	return &Service{
		portfolio: ps,
		quotes:    data.GetQuoteService,
	}
}

// SetQuoteServiceForTest injects QuoteService. Pass nil to restore default.
func (s *Service) SetQuoteServiceForTest(q marketdata.QuoteService) {
	if s == nil {
		return
	}
	if q == nil {
		s.quotes = data.GetQuoteService
		return
	}
	s.quotes = func() marketdata.QuoteService { return q }
}

// Evaluate is GET-time only: no INSERT, no mark_price write, no paper_* reads.
func (s *Service) Evaluate(q Query) (*View, error) {
	if s == nil {
		s = NewService(nil)
	}
	asOf := q.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	snap, err := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if err != nil && (snap == nil || !snap.Found) {
		if snap == nil {
			return emptyView(asOf, strings.TrimSpace(q.TradeDate)), err
		}
	}
	var buys, sells map[string][]positionstate.LotRecord
	if snap != nil && snap.Found {
		buys, sells = positionstate.LoadLotsByAccount(snap.AccountID)
	}
	var quoteMap map[string]marketdata.Quote
	if q.IncludeDisplay && snap != nil && snap.Found {
		quoteMap = fetchQuotesSafe(s.quotes, snap)
	}
	out := Build(Options{
		Snapshot:       snap,
		TradeDate:      q.TradeDate,
		AsOf:           asOf,
		IncludeDisplay: q.IncludeDisplay,
		Quotes:         quoteMap,
		BuyLotsByCode:  buys,
		SellLotsByCode: sells,
	})
	return out, nil
}

func fetchQuotesSafe(getter func() marketdata.QuoteService, snap *portfolio.Snapshot) map[string]marketdata.Quote {
	out := map[string]marketdata.Quote{}
	if getter == nil || snap == nil {
		return out
	}
	svc := getter()
	if svc == nil {
		return out
	}
	codes := make([]string, 0, len(snap.Positions))
	for _, p := range snap.Positions {
		c := strings.TrimSpace(p.StockCode)
		if c != "" {
			codes = append(codes, c)
		}
	}
	if len(codes) == 0 {
		return out
	}
	quotes, err := svc.GetQuotes(codes)
	if err != nil || len(quotes) == 0 {
		return out
	}
	for i := range quotes {
		q := quotes[i]
		c := strings.TrimSpace(q.Code)
		if c != "" {
			out[c] = q
		}
	}
	return out
}
