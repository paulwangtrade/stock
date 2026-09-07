package papertrading

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/instrument"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockname"
	"go-stock/backend/tradingconfig"
	"go-stock/backend/tradingrule"

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
// It consumes a Frozen Trade Plan (ready or Job-begun executing) and writes
// paper_sim_* plus trade_plan_items execution fields (Phase10-C.7-A).
// It never writes trade_plans.status (Job owns Begin/Finish) or production paper_* /
// Real Broker. fillBuy must not call UpdateItemExecution.
type PaperBroker struct {
	Price       PriceProvider
	FillSession ExecutionSession // A or B; empty → open/market_open semantics
	// Rules resolves Availability for fillBuy (Phase10-C.6-N). Nil → StaticResolver.
	Rules tradingrule.Resolver
}

// runForPlanHookForTest, when set, runs at the start of RunForPlan (tests only).
// Non-nil error short-circuits the broker (simulates broker failure after Job Begin).
var runForPlanHookForTest func() error

// SetRunForPlanHookForTest installs a RunForPlan pre-hook (nil clears). Tests only.
func SetRunForPlanHookForTest(fn func() error) {
	runForPlanHookForTest = fn
}

// NewPaperBroker returns a broker with the given price source (nil → fail-closed).
func NewPaperBroker(price PriceProvider) *PaperBroker {
	if price == nil {
		price = MissingPriceProvider{}
	}
	return &PaperBroker{Price: price, Rules: tradingrule.NewStaticResolver()}
}

// NewPaperBrokerForSession returns a broker with session-aware fill_reason / reject codes.
func NewPaperBrokerForSession(price PriceProvider, session ExecutionSession) *PaperBroker {
	b := NewPaperBroker(price)
	b.FillSession = session
	return b
}

func (b *PaperBroker) ruleResolver() tradingrule.Resolver {
	if b != nil && b.Rules != nil {
		return b.Rules
	}
	return tradingrule.NewStaticResolver()
}

// RunForPlan simulates execution of a Frozen Trade Plan.
//
// Guards (fail-closed):
//   - feature flag off → no-op (no schema, no orders)
//   - plan not frozen-ready and not Job-begun executing → error, no orders
//
// Item execution fields are written via UpdateItemExecution after processItem.
// Plan status lifecycle (ready→executing→terminal) is owned by paperTradingJob.
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
	if runForPlanHookForTest != nil {
		if err := runForPlanHookForTest(); err != nil {
			return nil, err
		}
	}

	plan, err := data.NewTradePlanRepo().GetByID(planID)
	if err != nil {
		return nil, fmt.Errorf("papertrading: load plan %d: %w", planID, err)
	}
	if err := allowPaperBrokerPlan(plan); err != nil {
		return nil, err
	}

	acc, err := b.getOrCreateDefaultAccount()
	if err != nil {
		return nil, err
	}

	res := &RunResult{Enabled: true, PlanID: plan.ID, AccountID: acc.ID}
	for i := range plan.Items {
		item := plan.Items[i]
		side := strings.ToLower(strings.TrimSpace(item.Side))
		if side != "buy" && side != "sell" {
			res.SkippedItems++
			continue
		}
		// Pending: full process. Filled/error: idempotent order lookup only (no new fills).
		// Keeps RunForPlan re-run SkippedAlready semantics after C.7 item writeback.
		if item.Status == models.TradePlanItemPending {
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
			continue
		}
		if item.Status == models.TradePlanItemFilled || item.Status == models.TradePlanItemError {
			var existing PaperSimOrder
			err := db.Dao.Where("plan_id = ? AND plan_item_id = ? AND trade_date = ?",
				plan.ID, item.ID, plan.TradeDate).First(&existing).Error
			if err == nil {
				res.SkippedAlready++
				continue
			}
			res.SkippedItems++
			continue
		}
		res.SkippedItems++
	}

	if err := b.revalueAccount(acc.ID); err != nil {
		return nil, err
	}
	res.Message = fmt.Sprintf("orders=%d filled=%d rejected=%d skipped=%d already=%d",
		res.OrdersTotal, res.FilledCount, res.RejectCount, res.SkippedItems, res.SkippedAlready)
	logger.SugaredLogger.Infof("PaperTrading run completed plan_id=%d account_id=%d %s", plan.ID, acc.ID, res.Message)
	return res, nil
}

// allowPaperBrokerPlan accepts frozen-ready (direct broker / tests) or executing
// after Job TryBeginExecute. Does not change models.RequireFrozenReadyTradePlan.
func allowPaperBrokerPlan(plan *models.TradePlan) error {
	if plan == nil {
		return fmt.Errorf("papertrading: execution blocked reason=PLAN_NIL trade plan is nil")
	}
	if plan.Status == models.TradePlanStatusExecuting {
		if plan.FreezeAt == nil || plan.FreezeAt.IsZero() {
			return fmt.Errorf("papertrading: execution blocked reason=PLAN_NOT_FROZEN executing plan missing FreezeAt planId=%d", plan.ID)
		}
		if plan.ApprovedAt == nil || plan.ApprovedAt.IsZero() {
			return fmt.Errorf("papertrading: execution blocked reason=PLAN_NOT_APPROVED executing plan missing ApprovedAt planId=%d", plan.ID)
		}
		return nil
	}
	guard := models.RequireFrozenReadyTradePlan(plan)
	if !guard.Allowed {
		return fmt.Errorf("papertrading: execution blocked reason=%s %s", guard.Reason, guard.Message)
	}
	return nil
}

// processItem creates a submitted order then attempts an open-price fill.
// Returns (order, filled, alreadyExists). alreadyExists=true means the idempotency
// key (plan_id, plan_item_id, trade_date) already had an order — no duplicate created.
// After every exit path, trade_plan_items execution fields are synced (not inside fillBuy).
func (b *PaperBroker) processItem(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem) (*PaperSimOrder, bool, bool) {
	order, filled, already := b.processItemCore(acc, plan, item)
	syncTradePlanItemAfterProcess(item, order, filled, already)
	return order, filled, already
}

func (b *PaperBroker) processItemCore(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem) (*PaperSimOrder, bool, bool) {
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

	side := strings.ToLower(strings.TrimSpace(item.Side))
	if side == "sell" {
		return b.processSellItemCore(acc, plan, item)
	}
	return b.processBuyItemCore(acc, plan, item)
}

func (b *PaperBroker) processBuyItemCore(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem) (*PaperSimOrder, bool, bool) {
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
	// Phase12-M2.3.2: QuantityPolicy Validate-only gate (Flag default off → no-op).
	// Does not normalize qty; illegal lots → reject. Generation stays in Policy/Materialize.
	if tradingrule.EnableQuantityPolicy() {
		gate := tradingrule.ApplyBrokerBuyQuantityGate(item.StockCode, qty)
		if !gate.Accepted {
			logger.SugaredLogger.Infof(
				"PaperOrder quantity_policy_reject plan_id=%d symbol=%s qty=%d reason=%s rule=%s suggested=%d",
				plan.ID, item.StockCode, qty, gate.Reason, gate.RuleKey, gate.SuggestedQty,
			)
			return b.reject(order, RejectInvalidQuantity), false, false
		}
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

func (b *PaperBroker) processSellItemCore(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem) (*PaperSimOrder, bool, bool) {
	now := time.Now()
	order := &PaperSimOrder{
		AccountID:  acc.ID,
		PlanID:     plan.ID,
		PlanItemID: item.ID,
		TradeDate:  plan.TradeDate,
		StockCode:  item.StockCode,
		StockName:  item.StockName,
		Side:       "sell",
		OrderPrice: item.LimitPrice,
		Status:     OrderStatusSubmitted,
		OrderTime:  now,
	}
	if err := db.Dao.Create(order).Error; err != nil {
		var raced PaperSimOrder
		if findErr := db.Dao.Where("plan_id = ? AND plan_item_id = ? AND trade_date = ?",
			plan.ID, item.ID, plan.TradeDate).First(&raced).Error; findErr == nil {
			return &raced, raced.Status == OrderStatusFilled, true
		}
		logger.SugaredLogger.Errorf("PaperTrading order create failed plan_id=%d symbol=%s err=%v", plan.ID, item.StockCode, err)
		return order, false, false
	}
	logger.SugaredLogger.Infof("PaperOrder created plan_id=%d order_id=%d symbol=%s side=sell status=submitted", plan.ID, order.ID, item.StockCode)

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

	qty := resolveSellQuantity(item)
	if qty <= 0 {
		return b.reject(order, RejectInvalidQuantity), false, false
	}
	order.Quantity = qty

	var pos PaperSimPosition
	err := db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || pos.TotalVolume <= 0 {
		return b.reject(order, RejectNoPosition), false, false
	}
	if err != nil {
		logger.SugaredLogger.Errorf("PaperTrading position lookup failed plan_id=%d symbol=%s err=%v", plan.ID, item.StockCode, err)
		return b.reject(order, RejectInvalidQuantity), false, false
	}
	if qty > pos.AvailableVolume {
		return b.reject(order, RejectInsufficientAvailable), false, false
	}

	fillPrice := quote.Open
	fee := calcSellFee(fillPrice, qty)

	if err := b.fillSell(acc, plan, item, order, fillPrice, qty, fee, fillReason); err != nil {
		logger.SugaredLogger.Errorf("PaperTrading sell fill failed plan_id=%d order_id=%d err=%v", plan.ID, order.ID, err)
		return b.reject(order, RejectInvalidQuantity), false, false
	}
	return order, true, false
}

// syncTradePlanItemAfterProcess writes item execution outcome (Phase10-C.7-A).
// Never called from fillBuy.
func syncTradePlanItemAfterProcess(item models.TradePlanItem, order *PaperSimOrder, filled, already bool) {
	if item.ID == 0 {
		return
	}
	it := item // copy; do not mutate Frozen Spec fields
	if order != nil && order.ID > 0 {
		it.OrderID = order.ID
		if strings.TrimSpace(order.StockName) != "" {
			it.StockName = order.StockName
		}
	}
	switch {
	case filled && order != nil && order.ID > 0:
		it.Status = models.TradePlanItemFilled
		it.Error = ""
		it.FilledPrice = order.FilledPrice
		it.FilledVolume = order.FilledVolume
		it.FilledFee = order.Fee
		it.FillID = findSimFillIDByOrder(order.ID)
	case order != nil && order.ID > 0 && order.Status == OrderStatusRejected:
		it.Status = models.TradePlanItemError
		it.Error = strings.TrimSpace(order.RejectReason)
		if it.Error == "" {
			it.Error = "rejected"
		}
		it.FilledPrice = 0
		it.FilledVolume = 0
		it.FilledFee = 0
		it.FillID = 0
	default:
		it.Status = models.TradePlanItemError
		if already {
			it.Error = "order exists but not filled"
		} else {
			it.Error = "order process failed"
		}
		it.FilledPrice = 0
		it.FilledVolume = 0
		it.FilledFee = 0
		it.FillID = 0
	}
	if err := data.NewTradePlanRepo().UpdateItemExecution(&it); err != nil {
		logger.SugaredLogger.Errorf("PaperTrading UpdateItemExecution failed item_id=%d err=%v", item.ID, err)
	}
}

func findSimFillIDByOrder(orderID uint) uint {
	if orderID == 0 || db.Dao == nil {
		return 0
	}
	var fill PaperSimFill
	if err := db.Dao.Where("order_id = ?", orderID).Order("id DESC").First(&fill).Error; err != nil {
		return 0
	}
	return fill.ID
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
// position (Availability from TradingRule), account cash.
func (b *PaperBroker) fillBuy(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem, order *PaperSimOrder, price float64, qty int64, fee float64, fillReason string) error {
	now := time.Now()
	if fillReason == "" {
		fillReason = FillReasonMarketOpen
	}
	stockName := resolvePaperSimStockName(plan, item)
	if strings.TrimSpace(stockName) == "" {
		stockName = stockname.UnknownName
	}
	asOf := strings.TrimSpace(plan.TradeDate)
	lockedDelta, availableDelta := b.buyAvailabilityDeltas(item.StockCode, asOf, qty)

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

		// Position: Availability Policy (C.6-N) — T1 locked+=qty, T0 available+=qty.
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
		pos.LockedVolume += lockedDelta
		pos.AvailableVolume += availableDelta
		if pos.AvailableVolume < 0 {
			pos.AvailableVolume = 0
		}
		if pos.LockedVolume < 0 {
			pos.LockedVolume = 0
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

// fillSell applies a full sell fill in a single transaction: fill row, order update,
// position decrement (available + total), account cash. avg_cost is unchanged.
func (b *PaperBroker) fillSell(acc *PaperSimAccount, plan *models.TradePlan, item models.TradePlanItem, order *PaperSimOrder, price float64, qty int64, fee float64, fillReason string) error {
	now := time.Now()
	if fillReason == "" {
		fillReason = FillReasonMarketOpen
	}
	stockName := resolvePaperSimStockName(plan, item)
	if strings.TrimSpace(stockName) == "" {
		stockName = stockname.UnknownName
	}

	return db.Dao.Transaction(func(tx *gorm.DB) error {
		var pos PaperSimPosition
		err := tx.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || pos.TotalVolume <= 0 {
			return fmt.Errorf("papertrading: no position for sell")
		}
		if err != nil {
			return err
		}
		if qty > pos.AvailableVolume {
			return fmt.Errorf("papertrading: insufficient available for sell")
		}

		fill := &PaperSimFill{
			AccountID:  acc.ID,
			OrderID:    order.ID,
			PlanID:     plan.ID,
			PlanItemID: item.ID,
			StockCode:  item.StockCode,
			StockName:  stockName,
			Side:       "sell",
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

		newTotal := pos.TotalVolume - qty
		if newTotal < 0 {
			newTotal = 0
		}
		newAvail := pos.AvailableVolume - qty
		if newAvail < 0 {
			newAvail = 0
		}
		pos.TotalVolume = newTotal
		pos.AvailableVolume = newAvail
		pos.MarkPrice = price
		pos.UpdatedAt = now
		if err := tx.Model(&PaperSimPosition{}).Where("id = ?", pos.ID).Updates(map[string]any{
			"total_volume":     pos.TotalVolume,
			"available_volume": pos.AvailableVolume,
			"mark_price":       pos.MarkPrice,
			"updated_at":       pos.UpdatedAt,
		}).Error; err != nil {
			return err
		}

		proceeds := price*float64(qty) - fee
		if err := tx.Model(&PaperSimAccount{}).Where("id = ?", acc.ID).
			Update("cash", gorm.Expr("cash + ?", proceeds)).Error; err != nil {
			return err
		}
		acc.Cash += proceeds
		return nil
	})
}

// buyAvailabilityDeltas classifies the code, resolves TradingRule, and splits qty.
// Classify/Resolve failure → fail-closed CN Equity T1 (all locked).
func (b *PaperBroker) buyAvailabilityDeltas(stockCode, asOf string, qty int64) (lockedDelta, availableDelta int64) {
	policy := tradingrule.SellableT1
	id, err := instrument.Classify(stockCode)
	if err == nil {
		if profile, rerr := b.ruleResolver().Resolve(id, asOf); rerr == nil {
			policy = profile.Availability.SellablePolicy
		}
	}
	return tradingrule.BuyFillDeltas(qty, policy)
}

// SettleNewTradingDay moves all locked volume to available (T+1 next-day unlock).
// Test / explicit whole-account helper only. Production PositionUnlockJob MUST NOT
// call this: it ignores fill trade_date and would unlock same-day buys (T+0).
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
	initial := tradingconfig.Default().PaperInitialCash()
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

// resolveBuyQuantity resolves buy qty from Frozen Spec.
//
//	Always: TargetVolume > 0 → trust Spec (caller may Validate).
//	Flag OFF: if volume missing, legacy floor(amount/open/100)*100 fallback.
//	Flag ON:  no quantity calculation — missing/zero volume → 0 (reject upstream).
//
// Does not Normalize, does not mutate TradePlan / target_volume.
func resolveBuyQuantity(item models.TradePlanItem, openPrice float64) int64 {
	if item.TargetVolume > 0 {
		return item.TargetVolume
	}
	if tradingrule.EnableQuantityPolicy() {
		return 0
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

// resolveSellQuantity resolves sell qty from Frozen Spec (TargetVolume only).
func resolveSellQuantity(item models.TradePlanItem) int64 {
	return item.TargetVolume
}

// calcSellFee is a deterministic MVP fee model (0 by default; kept for future).
func calcSellFee(_ float64, _ int64) float64 {
	return 0
}
