package pretrade

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/tradingconfig"
)

// Service loads plan + portfolio snapshot and builds the report (read-only).
type Service struct {
	plans     planLoader
	portfolio portfolio.Service
	riskView  func() tradingconfig.RiskView
}

type planLoader interface {
	GetByID(id uint) (*models.TradePlan, error)
}

// NewService wires default DB readers.
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	return &Service{
		plans:     data.NewTradePlanRepo(),
		portfolio: ps,
		riskView:  func() tradingconfig.RiskView { return tradingconfig.Default().Risk() },
	}
}

// EvaluateByPlanID loads plan and account context then Build.
func (s *Service) EvaluateByPlanID(planID uint) (*PreTradeRiskResult, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	if s == nil {
		s = NewService(nil)
	}
	if s.plans == nil {
		s.plans = data.NewTradePlanRepo()
	}
	if s.portfolio == nil {
		s.portfolio = portfolio.NewService()
	}

	plan, err := s.plans.GetByID(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("plan not found")
	}

	snap, _ := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: time.Now()})
	account := AccountInput{}
	positions := []PositionInput{}
	if snap != nil && snap.Found {
		account.Cash = snap.AvailableCash
		if account.Cash <= 0 {
			account.Cash = snap.Cash
		}
		account.Equity = snap.TotalEquity
		for _, p := range snap.Positions {
			positions = append(positions, PositionInput{
				StockCode:   p.StockCode,
				StockName:   p.StockName,
				TotalVolume: p.Volume,
			})
		}
	}

	rv := tradingconfig.RiskView{}
	if s.riskView != nil {
		rv = s.riskView()
	}

	return Build(Inputs{
		PlanID:    plan.ID,
		TradeDate: plan.TradeDate,
		AsOf:      time.Now(),
		Lines:     mapPlanLines(plan.Items),
		Account:   account,
		Positions: positions,
		RiskSnapshot: RiskSnapshot{
			PlanRiskStatus:      strings.TrimSpace(plan.RiskStatus),
			MarketLevel:         plan.MarketLevel,
			RiskSummary:         strings.TrimSpace(plan.RiskSummary),
			MaxSingleNamePct:    rv.MaxSingleNamePct,
			MaxGrossExposurePct: rv.MaxGrossExposurePct,
			RiskFilterEnabled:   rv.Enabled,
		},
	}), nil
}

func mapPlanLines(items []models.TradePlanItem) []PlanLine {
	out := make([]PlanLine, 0, len(items))
	for _, it := range items {
		out = append(out, PlanLine{
			StockCode:    it.StockCode,
			StockName:    it.StockName,
			Side:         it.Side,
			TargetAmount: it.TargetAmount,
			TargetVolume: it.TargetVolume,
			LimitPrice:   it.LimitPrice,
			RefPrice:     it.RefPrice,
			OpenRefPrice: it.OpenRefPrice,
			Status:       it.Status,
			IntentStatus: it.IntentStatus,
		})
	}
	return out
}
