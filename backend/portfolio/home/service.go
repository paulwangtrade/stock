package home

import (
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/readmodel"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
)

// Service loads existing read models and Assembles Investment Home.
type Service struct {
	portfolio portfolio.Service
	readmodel *readmodel.Service
	decision  *decision.Service
	summary   *summary.Service
	intel     *intelligence.Service
}

// NewService wires default readers. Does not alter trading packages.
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	intel := intelligence.NewService(ps)
	return &Service{
		portfolio: ps,
		readmodel: readmodel.NewService(ps),
		decision:  decision.NewService(ps),
		summary:   summary.NewService(ps, intel),
		intel:     intel,
	}
}

// Query selects the home trade_date.
type Query struct {
	TradeDate string
	AsOf      time.Time
}

// Build loads dependencies and Assembles (alias Evaluate).
func (s *Service) Build(q Query) *InvestmentHomeView {
	return s.Evaluate(q)
}

// Evaluate aggregates Dashboard + Decision + Daily Summary + Day Monitor + Daily Attention.
func (s *Service) Evaluate(q Query) *InvestmentHomeView {
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

	var snap *readmodel.View
	if s.readmodel != nil {
		snap, _ = s.readmodel.Evaluate(readmodel.Query{TradeDate: td, AsOf: asOf})
	}

	var dash *portfolio.PortfolioDashboardView
	if s.portfolio != nil {
		dash, _ = s.portfolio.Dashboard(portfolio.DashboardOptions{TradeDate: td, AsOf: asOf})
	}

	var dec *decision.PortfolioDecisionSummary
	if s.decision != nil {
		dec = s.decision.Build(decision.Query{TradeDate: td, AsOf: asOf})
	}

	var daily *summary.DailyInvestmentSummaryView
	if s.summary != nil {
		daily = s.summary.Evaluate(summary.Query{TradeDate: td, AsOf: asOf})
	}

	mon := tradingdaymonitor.Build(tradingdaymonitor.Options{TradeDate: td, AsOf: asOf})

	var intelBundle *intelligence.Bundle
	var intelErr error
	if s.intel != nil {
		intelBundle, intelErr = s.intel.Evaluate(intelligence.Query{AsOf: asOf, SkipQuotes: true})
	} else {
		intelErr = errString("position intelligence unavailable")
	}

	return Assemble(Inputs{
		TradeDate:    td,
		AsOf:         asOf,
		Snapshot:     snap,
		Dashboard:    dash,
		Decision:     dec,
		Daily:        daily,
		Monitor:      mon,
		Intelligence: intelBundle,
		IntelErr:     intelErr,
	})
}

type errString string

func (e errString) Error() string { return string(e) }
