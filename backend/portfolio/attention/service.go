package attention

import (
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
)

// Service loads read models and builds DailyAttentionView.
type Service struct {
	decision *decision.Service
	intel    *intelligence.Service
	summary  *summary.Service
}

// NewService wires default readers. Does not alter trading packages.
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	intel := intelligence.NewService(ps)
	return &Service{
		decision: decision.NewService(ps),
		intel:    intel,
		summary:  summary.NewService(ps, intel),
	}
}

// Query selects trade_date for the attention center.
type Query struct {
	TradeDate string
	AsOf      time.Time
}

// BuildDailyAttention loads dependencies and Builds (alias Evaluate).
func (s *Service) BuildDailyAttention(tradeDate string) *DailyAttentionView {
	return s.Evaluate(Query{TradeDate: tradeDate, AsOf: time.Now()})
}

// Build is Evaluate alias for home-style callers.
func (s *Service) Build(q Query) *DailyAttentionView {
	return s.Evaluate(q)
}

// Evaluate aggregates Decision + Intelligence + Daily Summary + Monitor (read-only).
// Snapshot / Investment Score are consumed via Decision.Service (no recompute here).
func (s *Service) Evaluate(q Query) *DailyAttentionView {
	if s == nil {
		s = NewService(nil)
	}
	asOf := q.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(q.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}

	in := Inputs{TradeDate: td, AsOf: asOf}

	if s.decision != nil {
		in.Decision = s.decision.Build(decision.Query{TradeDate: td, AsOf: asOf})
	} else {
		in.DecisionErr = errString("decision service unavailable")
	}

	if s.intel != nil {
		bundle, err := s.intel.Evaluate(intelligence.Query{AsOf: asOf, SkipQuotes: true})
		in.Intelligence = bundle
		in.IntelErr = err
	} else {
		in.IntelErr = errString("position intelligence unavailable")
	}

	if s.summary != nil {
		in.Daily = s.summary.Evaluate(summary.Query{TradeDate: td, AsOf: asOf})
	} else {
		in.DailyErr = errString("daily summary unavailable")
	}

	in.Monitor = tradingdaymonitor.Build(tradingdaymonitor.Options{TradeDate: td, AsOf: asOf})

	return Build(in)
}

type errString string

func (e errString) Error() string { return string(e) }
