package decision

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/pretrade"
	"go-stock/backend/portfolio/score"
)

// Service loads read models and builds PortfolioDecisionSummary.
type Service struct {
	portfolio portfolio.Service
	intel     *intelligence.Service
	score     *score.Service
	pretrade  *pretrade.Service
	pools     poolLoader
	plans     planLoader
}

type poolLoader interface {
	GetLatestByTradeDate(tradeDate string) (*models.CandidatePool, error)
}

type planLoader interface {
	GetLatestByTradeDate(tradeDate string) (*models.TradePlan, error)
}

// NewService wires default readers.
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	return &Service{
		portfolio: ps,
		intel:     intelligence.NewService(ps),
		score:     score.NewService(ps),
		pretrade:  pretrade.NewService(ps),
		pools:     data.NewCandidatePoolRepo(),
		plans:     data.NewTradePlanRepo(),
	}
}

// Query selects trade_date / optional plan for PreTrade.
type Query struct {
	TradeDate string
	PlanID    uint
	AsOf      time.Time
}

// Build loads inputs and returns summary (read-only). Alias for Evaluate.
func (s *Service) Build(q Query) *PortfolioDecisionSummary {
	return s.Evaluate(q)
}

// Evaluate loads Snapshot / Intelligence / Scores / PreTrade / Candidate and Build.
func (s *Service) Evaluate(q Query) *PortfolioDecisionSummary {
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

	in := Inputs{
		TradeDate:                 td,
		AsOf:                      asOf,
		MissingOpportunityPackage: true, // G.1 not shipped; inline heuristic used
		MissingEfficiencyPackage:  true, // G.2 not shipped; inline heuristic used
	}

	snap, _ := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if snap != nil && snap.Found {
		in.AccountFound = true
		in.Equity = snap.TotalEquity
		in.Cash = snap.AvailableCash
		if in.Cash <= 0 {
			in.Cash = snap.Cash
		}
	}

	var intelBy map[string]intelligence.PositionIntelligenceView
	if s.intel != nil {
		bundle, _ := s.intel.Evaluate(intelligence.Query{AsOf: asOf, SkipQuotes: true})
		if bundle != nil {
			intelBy = map[string]intelligence.PositionIntelligenceView{}
			for _, p := range bundle.Positions {
				intelBy[normalize(p.StockCode)] = p
			}
		}
	}

	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			if p.Volume <= 0 {
				continue
			}
			row := HoldingRow{
				StockCode:   p.StockCode,
				StockName:   p.StockName,
				CapitalUsed: p.MarketValue,
				Weight:      p.Weight,
			}
			if iv, ok := intelBy[normalize(p.StockCode)]; ok {
				row.PositionStatus = iv.PositionStatus
				row.RiskLevel = iv.RiskLevel
				row.AttentionReason = iv.AttentionReason
			}
			if s.score != nil {
				sv := s.score.EvaluateByCode(score.Query{StockCode: p.StockCode, TradeDate: td, AsOf: asOf})
				if sv != nil && sv.TotalScore != nil {
					row.InvestmentScore = sv.TotalScore
				}
			}
			in.Holdings = append(in.Holdings, row)
		}
	}

	// Top candidate from pool by Score.
	if s.pools != nil {
		if pool, err := s.pools.GetLatestByTradeDate(td); err == nil && pool != nil && len(pool.Items) > 0 {
			top := pool.Items[0]
			for i := range pool.Items {
				if pool.Items[i].Rank > 0 && (top.Rank == 0 || pool.Items[i].Rank < top.Rank) {
					top = pool.Items[i]
				}
				if pool.Items[i].Score > top.Score {
					top = pool.Items[i]
				}
			}
			// Prefer rank 1 when available
			for i := range pool.Items {
				if pool.Items[i].Rank == 1 {
					top = pool.Items[i]
					break
				}
			}
			sc := top.Score * 100
			in.Candidate = CandidateRow{
				Present: true,
				Code:    top.StockCode,
				Name:    top.StockName,
				Score:   &sc,
			}
			// Prefer InvestmentScore total when available for same code
			if s.score != nil {
				sv := s.score.EvaluateByCode(score.Query{StockCode: top.StockCode, TradeDate: td, AsOf: asOf})
				if sv != nil && sv.TotalScore != nil {
					in.Candidate.Score = sv.TotalScore
				}
			}
		}
	}

	// PreTrade from plan_id or latest plan
	planID := q.PlanID
	if planID == 0 && s.plans != nil {
		if plan, err := s.plans.GetLatestByTradeDate(td); err == nil && plan != nil {
			planID = plan.ID
		}
	}
	if planID > 0 && s.pretrade != nil {
		if rep, err := s.pretrade.EvaluateByPlanID(planID); err == nil && rep != nil {
			in.PreTrade = PreTradeInput{
				Present:       true,
				Level:         rep.Level,
				CashEnough:    rep.CashCheck.CashEnough,
				Concentration: rep.ConcentrationCheck.Level,
			}
		}
	}

	return Build(in)
}

func normalize(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
