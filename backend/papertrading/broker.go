package papertrading

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockname"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RunResult summarizes a single PaperBroker run over one Frozen Trade Plan.
type RunResult struct {
	Enabled        bool   `json:"enabled"`
	PlanID         uint   `json:"planId"`
	AccountID      uint   `json:"accountId"`
	OrdersTotal    int    `json:"ordersTotal"`
	FilledCount    int    `json:"filledCount"`
	RejectCount    int    `json:"rejectCount"`
	SkippedItems   int    `json:"skippedItems"`
	SkippedAlready int    `json:"skippedAlready"` // idempotent re-run hits
	Message        string `json:"message"`
}

// PaperBroker is the isolated Paper Trading MVP execution backend.
// It consumes a Frozen Trade Plan read-only and never writes trade_plans /
// trade_plan_items / production paper_* tables or touches Real Broker.
type PaperBroker struct {
	Price       PriceProvider
	FillSession ExecutionSession // A or B; empty → open/market_open semantics
}

// NewPaperBroker returns a broker with the given price source (nil → fail-closed).
func NewPaperBroker(price PriceProvider) *PaperBroker {
	if price == nil {
		price = MissingPriceProvider{}
	}
	return &PaperBroker{Price: price}
}

// NewPaperBrokerForSession returns a broker with session-aware fill_reason / reject codes.
func NewPaperBrokerForSession(price PriceProvider, session ExecutionSession) *PaperBroker {
	b := NewPaperBroker(price)
	b.FillSession = session
	return b
}

// RunForPlan simulates execution of a Frozen Trade Plan.
//
// Guards (fail-closed):
//   - feature flag off → no-op (no schema, no orders)
//   - plan not frozen → error, no orders
//
// It does NOT mutate the plan / items / lifecycle.
func (b *PaperBroker) RunForPlan(planID uint) (*RunResult, error) {
	if !IsEnabled() {
		logger.SugaredLogger.Infof("PaperTrading disabled plan_id=%d action=run result=skipped", planID)
		return &RunResult{Enabled: false, PlanID: planID, Message: "paper trading disabled"}, nil
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}

	plan, err := data.NewTradePlanRepo().GetByID(planID)
	if err != nil {
		return nil, fmt.Errorf("papertrading: load plan %d: %w", planID, err)
	}
	if !plan.IsFrozen() {
		return nil, fmt.Errorf("papertrading: plan %d not frozen (status=%s)", plan.ID, plan.Status)
	}

	acc, err := b.getOrCreateDefaultAccount()
	if err != nil {
		return nil, err
	}

	res := &RunResult{Enabled: true, PlanID: plan.ID, AccountID: acc.ID}
	for i := range plan.Items {
		item := plan.Items[i]
		if !strings.EqualFold(item.Side, "buy") || item.Status != models.TradePlanItemPending {
			res.SkippedItems++
			continue
		}
		order, filled, already := b.processItem(acc, plan, item)
		if already {
			res.SkippedAlready++
			_ = order
			continue
		}
		res.OrdersTotal++
		if filled {
			res.FilledCount++
		} else {
			res.RejectCount++
		}
	}

	if err := b.revalueAccount(acc.ID); err != nil {
		return nil, err
	}
	res.Message = fmt.Sprintf("orders=%d filled=%d rejected=%d skipped=%d already=%d",
		res.OrdersTotal, res.FilledCount, res.RejectCount, res.SkippedItems, res.SkippedAlready)
	logger.SugaredLogger.Infof("PaperTrading run completed plan_id=%d account_id=%d %s", plan.ID, acc.ID, res.Message)
	return res, nil
}

// processItem creates a submitted order then attempts an open-price fill.
// Returns (order, filled, alreadyExists). alreadyExists=true means the idempotency
// key (plan_id, plan_item_id, trade_date) already had an order — no duplicate created.
func (b *PaperBroker) processItem(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem) (*PaperSimOrder, bool, bool) {
	var existing PaperSimOrder
	err := db.Dao.Where("plan_id = ? AND plan_item_id = ? AND trade_date = ?",
		plan.ID, item.ID, plan.TradeDate).First(&existing).Error
	if err == nil {
		logger.SugaredLogger.Infof("PaperOrder skipped_already plan_id=%d plan_item_id=%d order_id=%d status=%s",
			plan.ID, item.ID, existing.ID, existing.Status)
		return &existing, existing.Status == OrderStatusFilled, true
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.SugaredLogger.Errorf("PaperTrading order lookup failed plan_id=%d item=%d err=%v", plan.ID, item.ID, err)
		return &PaperSimOrder{}, false, false
	}

	now := time.Now()
	order := &PaperSimOrder{
		AccountID:  acc.ID,
		PlanID:     plan.ID,
		PlanItemID: item.ID,
		TradeDate:  plan.TradeDate,
		StockCode:  item.StockCode,
		StockName:  item.StockName,
		Side:       "buy",
		OrderPrice: item.LimitPrice,
		Status:     OrderStatusSubmitted,
		OrderTime:  now,
	}
	if err := db.Dao.Create(order).Error; err != nil {
		// Race: another run inserted the unique key — treat as already.
		var raced PaperSimOrder
		if findErr := db.Dao.Where("plan_id = ? AND plan_item_id = ? AND trade_date = ?",
			plan.ID, item.ID, plan.TradeDate).First(&raced).Error; findErr == nil {
			return &raced, raced.Status == OrderStatusFilled, true
		}
		logger.SugaredLogger.Errorf("PaperTrading order create failed plan_id=%d symbol=%s err=%v", plan.ID, item.StockCode, err)
		return order, false, false
	}
	logger.SugaredLogger.Infof("PaperOrder created plan_id=%d order_id=%d symbol=%s side=buy status=submitted", plan.ID, order.ID, item.StockCode)

	quote, ok := b.Price.OpenQuote(item.StockCode, plan.TradeDate)
	missingReason := RejectMissingOpenPrice
	fillReason := FillReasonMarketOpen
	if b.FillSession == SessionB || quote.PriceKind == PriceKindClose {
		missingReason = RejectMissingClosePrice
		fillReason = FillReasonMarketClose
	}
	if !ok || quote.Open <= 0 {
		return b.reject(order, missingReason), false, false
	}
	if quote.LimitUp > 0 && quote.Open >= quote.LimitUp {
		return b.reject(order, RejectLimitUpUnavailable), false, false
	}

	qty := resolveBuyQuantity(item, quote.Open)
	if qty <= 0 {
		return b.reject(order, RejectInvalidQuantity), false, false
	}
	order.Quantity = qty

	fillPrice := quote.Open // NEVER use planned/limit price as fill
	fee := calcBuyFee(fillPrice, qty)
	cost := fillPrice*float64(qty) + fee
	if cost > acc.Cash+1e-9 {
		return b.reject(order, RejectInsufficientCash), false, false
	}

	if err := b.fillBuy(acc, plan, item, order, fillPrice, qty, fee, fillReason); err != nil {
		logger.SugaredLogger.Errorf("PaperTrading fill failed plan_id=%d order_id=%d err=%v", plan.ID, order.ID, err)
		return b.reject(order, RejectInvalidQuantity), false, false
	}
	return order, true, false
}

func (b *PaperBroker) reject(order *PaperSimOrder, reason string) *PaperSimOrder {
	order.Status = OrderStatusRejected
	order.RejectReason = reason
	order.UpdatedAt = time.Now()
	_ = db.Dao.Model(order).Select("status", "reject_reason", "quantity", "updated_at").Updates(order).Error
	logger.SugaredLogger.Infof("PaperOrder rejected plan_id=%d order_id=%d symbol=%s reason=%s", order.PlanID, order.ID, order.StockCode, reason)
	return order
}

// fillBuy applies a full buy fill in a single transaction: fill row, order update,
// position (T+1 locked), account cash.
func (b *PaperBroker) fillBuy(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem, order *PaperSimOrder, price float64, qty int64, fee float64, fillReason string) error {
	now := time.Now()
	if fillReason == "" {
		fillReason = FillReasonMarketOpen
	}
	stockName := resolvePaperSimStockName(plan, item)
	if strings.TrimSpace(stockName) == "" {
		stockName = stockname.UnknownName
	}
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		fill := &PaperSimFill{
			AccountID:  acc.ID,
			OrderID:    order.ID,
			PlanID:     plan.ID,
			PlanItemID: item.ID,
			StockCode:  item.StockCode,
			StockName:  stockName,
			Side:       "buy",
			Price:      price,
			Volume:     qty,
			Fee:        fee,
			FillReason: fillReason,
			FilledAt:   now,
		}
		if err := tx.Create(fill).Error; err != nil {
			return err
		}

		order.Status = OrderStatusFilled
		order.FilledPrice = price
		order.FilledVolume = qty
		order.Fee = fee
		order.UpdatedAt = now
		orderUpdates := map[string]any{
			"status":        OrderStatusFilled,
			"filled_price":  price,
			"filled_volume": qty,
			"fee":           fee,
			"quantity":      qty,
			"updated_at":    now,
		}
		if strings.TrimSpace(order.StockName) == "" {
			orderUpdates["stock_name"] = stockName
			order.StockName = stockName
		}
		if err := tx.Model(&PaperSimOrder{}).Where("id = ?", order.ID).
			Updates(orderUpdates).Error; err != nil {
			return err
		}

		// Position (T+1): today's buy is locked, not available.
		var pos PaperSimPosition
		err := tx.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			pos = PaperSimPosition{
				AccountID: acc.ID,
				StockCode: item.StockCode,
				StockName: stockName,
			}
		} else if err != nil {
			return err
		}
		if strings.TrimSpace(pos.StockName) == "" {
			pos.StockName = stockName
		}
		newTotal := pos.TotalVolume + qty
		if newTotal > 0 {
			pos.AvgCost = (pos.AvgCost*float64(pos.TotalVolume) + price*float64(qty)) / float64(newTotal)
		}
		pos.TotalVolume = newTotal
		pos.LockedVolume += qty
		pos.AvailableVolume = pos.TotalVolume - pos.LockedVolume
		if pos.AvailableVolume < 0 {
			pos.AvailableVolume = 0
		}
		pos.MarkPrice = price
		pos.UpdatedAt = now
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}, {Name: "stock_code"}},
			UpdateAll: true,
		}).Create(&pos).Error; err != nil {
			return err
		}

		if err := tx.Model(&PaperSimAccount{}).Where("id = ?", acc.ID).
			Update("cash", gorm.Expr("cash - ?", price*float64(qty)+fee)).Error; err != nil {
			return err
		}
		acc.Cash -= price*float64(qty) + fee
		return nil
	})
}

// SettleNewTradingDay moves all locked volume to available (T+1 next-day unlock).
func SettleNewTradingDay(accountID uint) error {
	if db.Dao == nil {
		return fmt.Errorf("papertrading: db not initialized")
	}
	return db.Dao.Model(&PaperSimPosition{}).
		Where("account_id = ?", accountID).
		Updates(map[string]any{
			"available_volume": gorm.Expr("total_volume"),
			"locked_volume":    0,
			"updated_at":       time.Now(),
		}).Error
}

func (b *PaperBroker) getOrCreateDefaultAccount() (*PaperSimAccount, error) {
	var acc PaperSimAccount
	err := db.Dao.Where("name = ?", "paper_sim_default").First(&acc).Error
	if err == nil {
		return &acc, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	initial := GetConfig().InitialCash
	acc = PaperSimAccount{
		Name:        "paper_sim_default",
		InitialCash: initial,
		Cash:        initial,
		Equity:      initial,
	}
	if err := db.Dao.Create(&acc).Error; err != nil {
		return nil, err
	}
	logger.SugaredLogger.Infof("PaperTrading account created account_id=%d initial_cash=%.0f", acc.ID, initial)
	return &acc, nil
}

// revalueAccount recomputes market value / equity / unrealized PnL from positions.
func (b *PaperBroker) revalueAccount(accountID uint) error {
	var acc PaperSimAccount
	if err := db.Dao.First(&acc, accountID).Error; err != nil {
		return err
	}
	var positions []PaperSimPosition
	if err := db.Dao.Where("account_id = ?", accountID).Find(&positions).Error; err != nil {
		return err
	}
	marketValue := 0.0
	unrealized := 0.0
	for _, p := range positions {
		mv := p.MarkPrice * float64(p.TotalVolume)
		marketValue += mv
		unrealized += (p.MarkPrice - p.AvgCost) * float64(p.TotalVolume)
	}
	acc.MarketValue = marketValue
	acc.UnrealizedPnl = unrealized
	acc.Equity = acc.Cash + marketValue
	acc.UpdatedAt = time.Now()
	return db.Dao.Model(&PaperSimAccount{}).Where("id = ?", accountID).
		Updates(map[string]any{
			"market_value":   acc.MarketValue,
			"unrealized_pnl": acc.UnrealizedPnl,
			"equity":         acc.Equity,
			"updated_at":     acc.UpdatedAt,
		}).Error
}

// resolveBuyQuantity prefers the frozen target_volume; otherwise derives lots from
// target_amount / open_price (floor to 100). Returns 0 if nothing valid.
func resolveBuyQuantity(item models.TradePlanItem, openPrice float64) int64 {
	if item.TargetVolume > 0 {
		return item.TargetVolume
	}
	if item.TargetAmount > 0 && openPrice > 0 {
		lots := int64(item.TargetAmount / openPrice / 100.0)
		return lots * 100
	}
	return 0
}

// calcBuyFee is a deterministic MVP fee model (0 by default; kept for future).
func calcBuyFee(_ float64, _ int64) float64 {
	return 0
}
