package portfolio

import (
	"fmt"
	"math"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
)

// DashboardOptions selects account + business day for the read model.
type DashboardOptions struct {
	AccountID   uint
	AccountName string
	TradeDate   string // YYYY-MM-DD; empty → as_of local date
	AsOf        time.Time
}

// Dashboard builds PortfolioDashboardView (read-only). Missing account → Found=false, no INSERT.
func (s *dbService) Dashboard(opts DashboardOptions) (*PortfolioDashboardView, error) {
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	tradeDate := strings.TrimSpace(opts.TradeDate)
	if tradeDate == "" {
		tradeDate = asOf.Format("2006-01-02")
	}
	name := strings.TrimSpace(opts.AccountName)
	if name == "" {
		name = defaultAccountName
	}

	empty := emptyDashboard(asOf, tradeDate)

	if db.Dao == nil {
		return empty, fmt.Errorf("portfolio: db not initialized")
	}

	acc, err := loadAccount(opts.AccountID, name)
	if err != nil {
		return empty, err
	}
	if acc == nil {
		return empty, nil
	}

	positions, err := papertrading.GetPositions(acc.ID)
	if err != nil {
		return emptyDashboard(asOf, tradeDate), err
	}

	snap := ProjectSnapshot(asOf, acc, positions)
	prevEquity, prevOK := loadPriorDailyEquity(acc.ID, tradeDate)
	fills, err := loadTodayFills(acc.ID, tradeDate)
	if err != nil {
		return emptyDashboard(asOf, tradeDate), err
	}

	return ProjectDashboard(asOf, tradeDate, snap, prevEquity, prevOK, fills), nil
}

// ProjectDashboard assembles the view from already-loaded read data (pure; no DB I/O).
func ProjectDashboard(
	asOf time.Time,
	tradeDate string,
	snap *Snapshot,
	priorDailyEquity float64,
	priorDailyOK bool,
	fills []papertrading.PaperSimFill,
) *PortfolioDashboardView {
	if asOf.IsZero() {
		asOf = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = asOf.Format("2006-01-02")
	}
	out := emptyDashboard(asOf, tradeDate)
	if snap == nil || !snap.Found {
		return out
	}

	out.Found = true
	out.Summary = PortfolioSummary{
		Equity:        snap.TotalEquity,
		Cash:          snap.Cash,
		MarketValue:   snap.MarketValue,
		PositionCount: snap.PositionCount,
		DailyPnLBasis: DailyPnLBasisUnavailable,
	}
	if priorDailyOK {
		d := snap.TotalEquity - priorDailyEquity
		out.Summary.DailyPnL = &d
		out.Summary.DailyPnLBasis = DailyPnLBasisReportDelta
	}

	rows := make([]PositionView, 0, len(snap.Positions))
	var maxW float64
	for _, p := range snap.Positions {
		rows = append(rows, PositionView{
			StockCode:     p.StockCode,
			StockName:     p.StockName,
			Quantity:      p.Volume,
			AvgCost:       p.AvgCost,
			MarketPrice:   p.MarkPrice,
			UnrealizedPnL: p.UnrealizedPnL,
		})
		if p.Weight > maxW {
			maxW = p.Weight
		}
	}
	out.Positions = rows
	out.Risk = RiskView{
		MaxPositionRatio:      maxW,
		IndustryConcentration: nil, // paper_sim has no industry attribution
		RiskLevel:             classifyRiskLevel(snap.PositionCount, maxW),
	}

	tradeFills := make([]TradeFillView, 0, len(fills))
	for _, f := range fills {
		tradeFills = append(tradeFills, TradeFillView{
			FillID:     f.ID,
			OrderID:    f.OrderID,
			PlanID:     f.PlanID,
			StockCode:  strings.TrimSpace(f.StockCode),
			StockName:  strings.TrimSpace(f.StockName),
			Side:       strings.TrimSpace(f.Side),
			Price:      f.Price,
			Volume:     f.Volume,
			Fee:        f.Fee,
			FillReason: strings.TrimSpace(f.FillReason),
			FilledAt:   f.FilledAt,
		})
	}
	out.Trades = TradeHistoryView{TradeDate: tradeDate, Fills: tradeFills}
	return out
}

func emptyDashboard(asOf time.Time, tradeDate string) *PortfolioDashboardView {
	return &PortfolioDashboardView{
		TradeDate: tradeDate,
		AsOf:      asOf,
		Found:     false,
		Summary: PortfolioSummary{
			DailyPnLBasis: DailyPnLBasisUnavailable,
		},
		Positions:      []PositionView{},
		Risk:           RiskView{RiskLevel: RiskLevelUnknown},
		Trades:         TradeHistoryView{TradeDate: tradeDate, Fills: []TradeFillView{}},
		DataSourceNote: dashboardDataSourceNote,
		Disclaimer:     dashboardDisclaimer,
	}
}

func classifyRiskLevel(positionCount int, maxPositionRatio float64) string {
	if positionCount <= 0 || math.IsNaN(maxPositionRatio) {
		return RiskLevelLow
	}
	switch {
	case maxPositionRatio >= 0.40:
		return RiskLevelHigh
	case maxPositionRatio >= 0.25:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// loadPriorDailyEquity reads the latest settlement daily_report before tradeDate (read-only).
func loadPriorDailyEquity(accountID uint, tradeDate string) (equity float64, ok bool) {
	if db.Dao == nil || accountID == 0 || strings.TrimSpace(tradeDate) == "" {
		return 0, false
	}
	if !db.Dao.Migrator().HasTable(&papertrading.PaperSimDailyReport{}) {
		return 0, false
	}
	var row papertrading.PaperSimDailyReport
	err := db.Dao.Where("account_id = ? AND report_date < ?", accountID, tradeDate).
		Order("report_date desc, id desc").
		First(&row).Error
	if err != nil {
		return 0, false
	}
	return row.Equity, true
}

// loadTodayFills loads fills for orders on tradeDate for the account (read-only).
func loadTodayFills(accountID uint, tradeDate string) ([]papertrading.PaperSimFill, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("portfolio: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&papertrading.PaperSimOrder{}) ||
		!db.Dao.Migrator().HasTable(&papertrading.PaperSimFill{}) {
		return []papertrading.PaperSimFill{}, nil
	}
	var orderIDs []uint
	if err := db.Dao.Model(&papertrading.PaperSimOrder{}).
		Where("account_id = ? AND trade_date = ?", accountID, tradeDate).
		Pluck("id", &orderIDs).Error; err != nil {
		return nil, err
	}
	if len(orderIDs) == 0 {
		return []papertrading.PaperSimFill{}, nil
	}
	var fills []papertrading.PaperSimFill
	if err := db.Dao.Where("account_id = ? AND order_id IN ?", accountID, orderIDs).
		Order("id asc").Find(&fills).Error; err != nil {
		return nil, err
	}
	return fills, nil
}
