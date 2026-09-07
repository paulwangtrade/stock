package strategy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingcalendar"
)

var (
	ErrWatchlistDraftInvalidStock  = errors.New("watchlist draft: invalid stock_code")
	ErrWatchlistDraftNotWatching   = errors.New("watchlist draft: opportunity is not currently WATCH")
	ErrWatchlistDraftPlanExists    = errors.New("watchlist draft: trade plan already exists for stock")
	ErrWatchlistDraftMissingFields = errors.New("watchlist draft: opportunity_id and scan_batch_key required")
)

// WatchlistDraftRequest is the input for BuildDraftWatchlistTradePlan (Phase16.26-C2.3.2-B1).
type WatchlistDraftRequest struct {
	AccountID     uint
	TradeDate     string
	StockCode     string
	StockName     string
	OpportunityID string
	ScanBatchKey  string
	Actor         string
}

// WatchlistDraftConflict carries the existing representative plan when create is rejected.
type WatchlistDraftConflict struct {
	PlanID            uint
	TradePlanStatus   string
	TradeDate         string
	ExistingPlanStatus string
}

func (e *WatchlistDraftConflict) Error() string {
	if e == nil {
		return ErrWatchlistDraftPlanExists.Error()
	}
	return fmt.Sprintf("%s: plan_id=%d status=%s", ErrWatchlistDraftPlanExists.Error(), e.PlanID, e.ExistingPlanStatus)
}

func (e *WatchlistDraftConflict) Unwrap() error { return ErrWatchlistDraftPlanExists }

// BuildDraftWatchlistTradePlan creates a single-name buy TradePlan draft from a watching opportunity.
// Does not Approve, Freeze, Materialize, or Execute. Does not set prices or volumes.
func BuildDraftWatchlistTradePlan(req WatchlistDraftRequest) (*models.TradePlan, error) {
	code := opportunity.NormalizeStockCode(req.StockCode)
	if code == "" {
		return nil, ErrWatchlistDraftInvalidStock
	}
	oid := strings.TrimSpace(req.OpportunityID)
	batch := strings.TrimSpace(req.ScanBatchKey)
	if oid == "" || batch == "" {
		return nil, ErrWatchlistDraftMissingFields
	}

	accountID := req.AccountID
	if accountID == 0 {
		acc, err := papertrading.GetDefaultAccount()
		if err != nil {
			return nil, err
		}
		accountID = acc.ID
	}

	if err := opportunity.EnsureSchema(db.Dao); err != nil {
		return nil, err
	}
	if err := assertOpportunityStillWatching(accountID, oid); err != nil {
		return nil, err
	}

	if plan, ok := opportunity.FindRepresentativeTradePlanForCode(code); ok {
		return nil, &WatchlistDraftConflict{
			PlanID:             plan.ID,
			TradePlanStatus:    mapWatchlistConflictStatus(plan.Status),
			TradeDate:          plan.TradeDate,
			ExistingPlanStatus: plan.Status,
		}
	}

	tradeDate := strings.TrimSpace(req.TradeDate)
	if tradeDate == "" {
		today := time.Now().Format("2006-01-02")
		next, err := tradingcalendar.NextTradingDayString(today)
		if err != nil {
			tradeDate = today
		} else {
			tradeDate = next
		}
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return nil, fmt.Errorf("invalid trade_date %q: %w", tradeDate, err)
	}

	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}

	repo := data.NewTradePlanRepo()
	version, err := repo.NextPlanVersion(tradeDate)
	if err != nil {
		return nil, err
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "ui:watched-opportunities"
	}
	stockName := strings.TrimSpace(req.StockName)
	reason := fmt.Sprintf("watchlist · opportunity_id=%s", oid)
	msg := fmt.Sprintf(
		"watchlist draft symbol=%s opportunity_id=%s batch=%s actor=%s",
		code, oid, batch, actor,
	)

	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		GeneratedAt:    time.Now(),
		Status:         models.TradePlanStatusDraft,
		Side:           "buy",
		AmountPerStock: amount,
		MaxNames:       1,
		EnableExecute:  false,
		Message:        msg,
		PlanVersion:    version,
		SourceSession:  models.TradePlanSourceWatchlist,
		PricingStage:   "watchlist_draft",
	}
	models.StampLegacyBuyDraftProviderMetadata(plan)

	item := models.TradePlanItem{
		TradeDate:    tradeDate,
		StockCode:    code,
		StockName:    stockName,
		Side:         "buy",
		Status:       models.TradePlanItemPending,
		TargetAmount: amount,
		TargetVolume: 0,
		LimitPrice:   0,
		Reason:       reason,
	}
	if err := repo.CreatePlanWithItems(plan, []models.TradePlanItem{item}); err != nil {
		return nil, err
	}

	logger.SugaredLogger.Infof(
		"BuildDraftWatchlistTradePlan date=%s planId=%d symbol=%s amount=%.0f version=%d status=%s",
		tradeDate, plan.ID, code, amount, plan.PlanVersion, plan.Status,
	)
	return plan, nil
}

func assertOpportunityStillWatching(accountID uint, opportunityID string) error {
	var rows []opportunity.UserOpportunityAction
	err := db.Dao.Where("account_id = ? AND opportunity_id = ?", accountID, opportunityID).
		Order("created_at DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return err
	}
	if len(rows) == 0 || rows[0].Action != opportunity.ActionWatch {
		return ErrWatchlistDraftNotWatching
	}
	return nil
}

func mapWatchlistConflictStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case models.TradePlanStatusDraft:
		return opportunity.WatchlistTradePlanDraft
	case models.TradePlanStatusReady:
		return opportunity.WatchlistTradePlanReady
	default:
		return opportunity.WatchlistTradePlanExecuted
	}
}
