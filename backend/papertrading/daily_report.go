package papertrading

import (
	"errors"
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"

	"gorm.io/gorm"
)

const DailyReportSourceSettlement = "settlement_auto"

// GenerateDailyReportResult summarizes daily report generation.
type GenerateDailyReportResult struct {
	Generated      bool   `json:"generated"`
	Skipped        bool   `json:"skipped"`
	ReportDate     string `json:"reportDate"`
	AccountID      uint   `json:"accountId"`
	PositionRows   int    `json:"positionRows"`
	Message        string `json:"message"`
}

// GenerateDailyReport freezes an immutable EOD snapshot after Settlement.
// Idempotent: UNIQUE(account_id, report_date) — second call skips without duplicating.
func GenerateDailyReport(reportDate string) (*GenerateDailyReportResult, error) {
	if reportDate == "" {
		reportDate = time.Now().Format("2006-01-02")
	}
	out := &GenerateDailyReportResult{ReportDate: reportDate}
	if !IsEnabled() {
		out.Skipped = true
		out.Message = "enablePaperTrading=false"
		return out, nil
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}

	acc, err := GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	if acc == nil {
		out.Skipped = true
		out.Message = "no paper_sim account"
		return out, nil
	}
	out.AccountID = acc.ID

	var existing PaperSimDailyReport
	err = db.Dao.Where("account_id = ? AND report_date = ?", acc.ID, reportDate).First(&existing).Error
	if err == nil {
		out.Skipped = true
		out.Message = "daily report already exists"
		logger.SugaredLogger.Infof("PaperDailyReport skipped_already account_id=%d report_date=%s", acc.ID, reportDate)
		return out, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Event aggregates for the report date.
	var orders []PaperSimOrder
	if err := db.Dao.Where("account_id = ? AND trade_date = ?", acc.ID, reportDate).Find(&orders).Error; err != nil {
		return nil, err
	}
	orderCount, filledCount, rejectedCount := 0, 0, 0
	turnover := 0.0
	planIDs := map[uint]struct{}{}
	for _, o := range orders {
		orderCount++
		if o.PlanID > 0 {
			planIDs[o.PlanID] = struct{}{}
		}
		switch o.Status {
		case OrderStatusFilled:
			filledCount++
			turnover += o.FilledPrice * float64(o.FilledVolume)
		case OrderStatusRejected:
			rejectedCount++
		}
	}

	// Fallback plan count from runs if no orders yet.
	planCount := len(planIDs)
	var run PaperSimRun
	runStatus := ""
	planID := uint(0)
	if err := db.Dao.Where("trade_date = ?", reportDate).Order("id desc").First(&run).Error; err == nil {
		runStatus = run.Status
		planID = run.PlanID
		if planCount == 0 && run.PlanID > 0 {
			planCount = 1
		}
	}

	positions, err := GetPositions(acc.ID)
	if err != nil {
		return nil, err
	}

	equity := acc.Equity
	if equity <= 0 {
		equity = acc.Cash + acc.MarketValue
	}
	maxSinglePct := 0.0
	maxSingleCode := ""
	grossExposure := 0.0
	if equity > 1e-9 {
		grossExposure = acc.MarketValue / equity
		for _, p := range positions {
			mv := p.MarkPrice * float64(p.TotalVolume)
			pct := mv / equity
			if pct > maxSinglePct {
				maxSinglePct = pct
				maxSingleCode = p.StockCode
			}
		}
	}

	now := time.Now()
	report := &PaperSimDailyReport{
		AccountID:            acc.ID,
		ReportDate:           reportDate,
		Cash:                 acc.Cash,
		MarketValue:          acc.MarketValue,
		Equity:               equity,
		FloatingPnl:          acc.UnrealizedPnl,
		PlanCount:            planCount,
		OrderCount:           orderCount,
		FilledCount:          filledCount,
		RejectedCount:        rejectedCount,
		Turnover:             turnover,
		MaxSinglePositionPct: maxSinglePct,
		MaxGrossExposurePct:  grossExposure,
		MaxSingleStockCode:   maxSingleCode,
		PositionCount:        len(positions),
		PlanID:               planID,
		RunStatus:            runStatus,
		Source:               DailyReportSourceSettlement,
		CreatedAt:            now,
	}

	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(report).Error; err != nil {
			// Race: unique conflict → treat as skip
			var raced PaperSimDailyReport
			if findErr := tx.Where("account_id = ? AND report_date = ?", acc.ID, reportDate).First(&raced).Error; findErr == nil {
				return errDailyReportExists
			}
			return err
		}
		for _, p := range positions {
			mv := p.MarkPrice * float64(p.TotalVolume)
			fpnl := (p.MarkPrice - p.AvgCost) * float64(p.TotalVolume)
			row := &PaperSimDailyPosition{
				AccountID:       acc.ID,
				ReportDate:      reportDate,
				Symbol:          p.StockCode,
				Volume:          p.TotalVolume,
				AvailableVolume: p.AvailableVolume,
				CostPrice:       p.AvgCost,
				ClosePrice:      p.MarkPrice,
				MarketValue:     mv,
				FloatingPnl:     fpnl,
				CreatedAt:       now,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errDailyReportExists) {
		out.Skipped = true
		out.Message = "daily report already exists"
		return out, nil
	}
	if err != nil {
		return nil, err
	}

	out.Generated = true
	out.PositionRows = len(positions)
	out.Message = fmt.Sprintf("daily report saved equity=%.2f positions=%d filled=%d", equity, len(positions), filledCount)
	logger.SugaredLogger.Infof(
		"PaperDailyReport generated account_id=%d report_date=%s equity=%.2f floating_pnl=%.2f filled=%d rejected=%d",
		acc.ID, reportDate, equity, acc.UnrealizedPnl, filledCount, rejectedCount,
	)
	return out, nil
}

var errDailyReportExists = errors.New("papertrading: daily report exists")

// ListDailyReports returns immutable daily reports in [from, to] (inclusive), newest first.
func ListDailyReports(from, to string, limit, offset int) ([]PaperSimDailyReport, int, error) {
	if limit <= 0 {
		limit = 60
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if db.Dao == nil {
		return nil, 0, fmt.Errorf("papertrading: db not initialized")
	}
	if !db.Dao.Migrator().HasTable(&PaperSimDailyReport{}) {
		return []PaperSimDailyReport{}, 0, nil
	}
	q := db.Dao.Model(&PaperSimDailyReport{})
	if from != "" {
		q = q.Where("report_date >= ?", from)
	}
	if to != "" {
		q = q.Where("report_date <= ?", to)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []PaperSimDailyReport
	if err := q.Order("report_date desc, id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, int(total), nil
}
