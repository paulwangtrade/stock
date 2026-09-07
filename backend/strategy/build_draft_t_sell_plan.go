package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// TSellDraftRequest is the input for BuildDraftTSellTradePlan (Phase13-D2-W2).
type TSellDraftRequest struct {
	TradeDate string
	StockCode string
	Quantity  int64
	Actor     string
	Reason    string
	// SourceSession optional: t_sell (default) | exit_review (Phase14-M1).
	SourceSession string
}

// BuildDraftTSellTradePlan creates a pure sell TradePlan draft from PositionState gate.
// Does not Approve, Freeze, or Execute.
func BuildDraftTSellTradePlan(req TSellDraftRequest) (*models.TradePlan, error) {
	tradeDate := strings.TrimSpace(req.TradeDate)
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return nil, fmt.Errorf("invalid trade_date %q: %w", tradeDate, err)
	}
	code := strings.TrimSpace(req.StockCode)
	if code == "" {
		return nil, ErrSellInvalidStock
	}

	gate, err := CheckSellPositionGate(code, req.Quantity, tradeDate, time.Now())
	if err != nil {
		return nil, err
	}

	repo := data.NewTradePlanRepo()
	version, err := repo.NextPlanVersion(tradeDate)
	if err != nil {
		return nil, err
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "T manual sell draft"
	}
	actor := strings.TrimSpace(req.Actor)
	sourceSession := resolveTSellDraftSourceSession(req.SourceSession)
	msg := fmt.Sprintf("%s draft symbol=%s qty=%d actor=%s", sourceSession, code, req.Quantity, actor)

	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		Side:                 "sell",
		EnableExecute:        false,
		Message:              msg,
		PlanVersion:          version,
		SourceSession:        sourceSession,
		PricingPolicyVersion: 1,
		PricingStage:         "t_sell_draft",
		MaxNames:             1,
	}
	item := models.TradePlanItem{
		TradeDate:    tradeDate,
		StockCode:    gate.Symbol,
		StockName:    gate.StockName,
		Side:         "sell",
		Status:       models.TradePlanItemPending,
		TargetVolume: req.Quantity,
		TargetAmount: 0,
		LimitPrice:   0,
		IntentStatus: morningIntentStatusPriced,
		Reason:       reason,
		RefPrice:     gate.AvgCost,
		RefSource:    "paper_sim_snapshot",
	}
	if err := repo.CreatePlanWithItems(plan, []models.TradePlanItem{item}); err != nil {
		return nil, err
	}

	logger.SugaredLogger.Infof(
		"BuildDraftTSellTradePlan date=%s planId=%d symbol=%s qty=%d version=%d status=%s",
		tradeDate, plan.ID, code, req.Quantity, plan.PlanVersion, plan.Status,
	)
	return plan, nil
}

func resolveTSellDraftSourceSession(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case models.TradePlanSourceExitReview:
		return models.TradePlanSourceExitReview
	default:
		return models.TradePlanSourceTSell
	}
}
