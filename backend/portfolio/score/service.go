package score

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
)

// Service loads candidate + holding context and builds InvestmentScoreView (read-only).
type Service struct {
	portfolio portfolio.Service
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
		pools:     data.NewCandidatePoolRepo(),
		plans:     data.NewTradePlanRepo(),
	}
}

// Query selects trade_date / as_of for a single stock_code.
type Query struct {
	StockCode string
	TradeDate string
	AsOf      time.Time
}

// EvaluateByCode builds score for one code. Never writes. Missing sides → null components.
func (s *Service) EvaluateByCode(q Query) *InvestmentScoreView {
	if s == nil {
		s = NewService(nil)
	}
	if s.portfolio == nil {
		s.portfolio = portfolio.NewService()
	}
	asOf := q.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	code := strings.TrimSpace(q.StockCode)
	td := strings.TrimSpace(q.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}

	in := Inputs{StockCode: code, TradeDate: td, AsOf: asOf}

	if s.pools != nil {
		if pool, err := s.pools.GetLatestByTradeDate(td); err == nil && pool != nil {
			if item := findPoolItem(pool.Items, code); item != nil {
				in.Candidate = CandidateInput{
					Present:     true,
					StockName:   item.StockName,
					Score:       item.Score,
					SignalScore: item.SignalScore,
					HasSignal:   item.SignalSnapshotID > 0 || strings.TrimSpace(item.SignalTag) != "" || item.SignalScore != 0,
					Rank:        item.Rank,
					PoolItemCount: pool.ItemCount,
				}
			}
		}
	}

	if s.plans != nil {
		if plan, err := s.plans.GetLatestByTradeDate(td); err == nil && plan != nil {
			for _, it := range plan.Items {
				if normalize(it.StockCode) == normalize(code) && strings.TrimSpace(it.RiskCode) != "" {
					in.Candidate.TradePlanRiskCode = it.RiskCode
					if !in.Candidate.Present {
						// Plan-only risk without pool row does not create a candidate subject;
						// RiskCode only applies when candidate already present.
					} else {
						break
					}
				}
			}
		}
	}

	snap, _ := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			if normalize(p.StockCode) != normalize(code) || p.Volume <= 0 {
				continue
			}
			var ret *float64
			if p.AvgCost > 0 {
				r := (p.MarkPrice - p.AvgCost) / p.AvgCost
				ret = &r
			}
			// Build intelligence labels without live quotes (SkipQuotes equivalent).
			bundle := intelligence.Build(intelligence.Options{
				AsOf:     asOf,
				Snapshot: snap,
			})
			riskLevel := ""
			posStatus := ""
			stratStatus := ""
			name := p.StockName
			if bundle != nil {
				for _, row := range bundle.Positions {
					if normalize(row.StockCode) == normalize(code) {
						riskLevel = row.RiskLevel
						posStatus = row.PositionStatus
						stratStatus = row.StrategyStatus
						if strings.TrimSpace(row.StockName) != "" {
							name = row.StockName
						}
						break
					}
				}
			}
			in.Holding = HoldingInput{
				Present:          true,
				StockName:        name,
				Weight:           p.Weight,
				UnrealizedReturn: ret,
				RiskLevel:        riskLevel,
				PositionStatus:   posStatus,
				StrategyStatus:   stratStatus,
			}
			break
		}
	}

	return Build(in)
}

func findPoolItem(items []models.CandidatePoolItem, code string) *models.CandidatePoolItem {
	want := normalize(code)
	for i := range items {
		if normalize(items[i].StockCode) == want {
			return &items[i]
		}
	}
	return nil
}

func normalize(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
