package summary

import (
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/tradingdaymonitor"
)

// Service loads Dashboard + Monitor + Intelligence and builds the daily summary (read-only).
type Service struct {
	portfolio portfolio.Service
	intel     *intelligence.Service
}

// NewService wires default readers.
func NewService(ps portfolio.Service, intel *intelligence.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	if intel == nil {
		intel = intelligence.NewService(ps)
	}
	return &Service{portfolio: ps, intel: intel}
}

// Query selects trade_date for the digest.
type Query struct {
	TradeDate string
	AsOf      time.Time
}

// Evaluate loads read models and builds summary. Intelligence errors degrade, do not fail.
func (s *Service) Evaluate(q Query) *DailyInvestmentSummaryView {
	asOf := q.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(q.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}

	dash, _ := s.portfolio.Dashboard(portfolio.DashboardOptions{TradeDate: td, AsOf: asOf})
	mon := tradingdaymonitor.Build(tradingdaymonitor.Options{TradeDate: td, AsOf: asOf})

	var intelBundle *intelligence.Bundle
	var intelErr error
	if s.intel != nil {
		intelBundle, intelErr = s.intel.Evaluate(intelligence.Query{AsOf: asOf, SkipQuotes: true})
	} else {
		intelErr = errIntelUnavailable
	}

	return Build(Inputs{
		TradeDate:    td,
		AsOf:         asOf,
		Dashboard:    dash,
		Monitor:      mon,
		Intelligence: intelBundle,
		IntelErr:     intelErr,
	})
}

var errIntelUnavailable = errString("position intelligence unavailable")

type errString string

func (e errString) Error() string { return string(e) }
