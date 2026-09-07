package intelligence

import (
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
)

// Service builds Position Intelligence from portfolio Snapshot (read-only).
type Service struct {
	portfolio portfolio.Service
	quotes    func() marketdata.QuoteService
}

// NewService returns the default reader. nil portfolio → portfolio.NewService().
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	return &Service{
		portfolio: ps,
		quotes:    data.GetQuoteService,
	}
}

// SetQuoteServiceForTest injects quotes (nil restores GetQuoteService).
func (s *Service) SetQuoteServiceForTest(q marketdata.QuoteService) {
	if q == nil {
		s.quotes = data.GetQuoteService
		return
	}
	s.quotes = func() marketdata.QuoteService { return q }
}

// Query options for HTTP / tests.
type Query struct {
	AsOf          time.Time
	ExtraCodes    []string
	StrategyHints map[string]string
	SkipQuotes    bool
}

// Evaluate loads Snapshot and builds intelligence (never writes).
func (s *Service) Evaluate(q Query) (*Bundle, error) {
	snap, err := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: q.AsOf})
	if err != nil && (snap == nil || !snap.Found) {
		// Still return a soft bundle when possible; only hard-fail if snap nil.
		if snap == nil {
			return Build(Options{
				AsOf:          q.AsOf,
				ExtraCodes:    q.ExtraCodes,
				StrategyHints: q.StrategyHints,
			}), err
		}
	}
	var qs marketdata.QuoteService
	if !q.SkipQuotes && s.quotes != nil {
		qs = s.quotes()
	}
	var buys, sells map[string][]positionstate.LotRecord
	if snap != nil && snap.Found {
		buys, sells = positionstate.LoadLotsByAccount(snap.AccountID)
	}
	return Build(Options{
		AsOf:           q.AsOf,
		Snapshot:       snap,
		Quotes:         qs,
		StrategyHints:  q.StrategyHints,
		ExtraCodes:     q.ExtraCodes,
		BuyLotsByCode:  buys,
		SellLotsByCode: sells,
	}), nil
}
