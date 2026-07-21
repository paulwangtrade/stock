package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// TradePlanRepo 交易计划持久化与执行状态 CAS。
type TradePlanRepo struct{}

func NewTradePlanRepo() *TradePlanRepo { return &TradePlanRepo{} }

func (r *TradePlanRepo) CreatePlanWithItems(plan *models.TradePlan, items []models.TradePlanItem) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	now := time.Now()
	if plan.GeneratedAt.IsZero() {
		plan.GeneratedAt = now
	}
	if plan.Status == "" {
		plan.Status = models.TradePlanStatusReady
	}
	if plan.Side == "" {
		plan.Side = "buy"
	}
	plan.CreatedAt = now
	plan.UpdatedAt = now

	return db.Dao.Transaction(func(tx *gorm.DB) error {
		// 同日旧 ready 计划作废，保证执行时最多一个 ready
		if plan.Status == models.TradePlanStatusReady && plan.TradeDate != "" {
			if err := tx.Model(&models.TradePlan{}).
				Where("trade_date = ? AND status = ?", plan.TradeDate, models.TradePlanStatusReady).
				Updates(map[string]any{
					"status":     models.TradePlanStatusSuperseded,
					"message":    "superseded by newer plan",
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(plan).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].PlanID = plan.ID
			items[i].TradeDate = plan.TradeDate
			if items[i].Side == "" {
				items[i].Side = plan.Side
			}
			if items[i].Status == "" {
				items[i].Status = models.TradePlanItemPending
			}
			items[i].CreatedAt = now
			items[i].UpdatedAt = now
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		plan.Items = items
		return nil
	})
}

func (r *TradePlanRepo) GetByID(id uint) (*models.TradePlan, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var plan models.TradePlan
	if err := db.Dao.First(&plan, id).Error; err != nil {
		return nil, err
	}
	items, err := r.loadItems(plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Items = items
	return &plan, nil
}

func (r *TradePlanRepo) GetLatestByTradeDate(tradeDate string) (*models.TradePlan, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var plan models.TradePlan
	err := db.Dao.Where("trade_date = ?", tradeDate).Order("id DESC").First(&plan).Error
	if err != nil {
		return nil, err
	}
	items, err := r.loadItems(plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Items = items
	return &plan, nil
}

// NextPlanVersion returns MAX(plan_version)+1 for tradeDate (append-only versioning).
// When no rows exist, returns 1. Legacy rows with plan_version=0 yield 1 on first draft.
func (r *TradePlanRepo) NextPlanVersion(tradeDate string) (int, error) {
	if db.Dao == nil {
		return 0, fmt.Errorf("数据库未初始化")
	}
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return 0, fmt.Errorf("trade date is required")
	}
	var maxVersion int
	err := db.Dao.Model(&models.TradePlan{}).
		Where("trade_date = ?", tradeDate).
		Select("COALESCE(MAX(plan_version), 0)").
		Scan(&maxVersion).Error
	if err != nil {
		return 0, err
	}
	return maxVersion + 1, nil
}

// GetReadyByTradeDate 9:30 执行入口：仅 status=ready。
func (r *TradePlanRepo) GetReadyByTradeDate(tradeDate string) (*models.TradePlan, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var plan models.TradePlan
	err := db.Dao.Where("trade_date = ? AND status = ?", tradeDate, models.TradePlanStatusReady).
		Order("id DESC").First(&plan).Error
	if err != nil {
		return nil, err
	}
	items, err := r.loadItems(plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Items = items
	return &plan, nil
}

func (r *TradePlanRepo) loadItems(planID uint) ([]models.TradePlanItem, error) {
	var items []models.TradePlanItem
	err := db.Dao.Where("plan_id = ?", planID).Order("priority ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *TradePlanRepo) MarkChecked(planID uint) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	now := time.Now()
	return db.Dao.Model(&models.TradePlan{}).Where("id = ?", planID).Updates(map[string]any{
		"checked_at": now,
		"updated_at": now,
	}).Error
}

// TryBeginExecute CAS: ready -> executing，防止 cron 重复买入。
func (r *TradePlanRepo) TryBeginExecute(planID uint) (bool, error) {
	if db.Dao == nil {
		return false, fmt.Errorf("数据库未初始化")
	}
	now := time.Now()
	res := db.Dao.Model(&models.TradePlan{}).
		Where("id = ? AND status = ?", planID, models.TradePlanStatusReady).
		Updates(map[string]any{
			"status":      models.TradePlanStatusExecuting,
			"executed_at": now,
			"updated_at":  now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// TradePlanExecutionSummary 计划执行聚合（对账 / reconcile 用）。
type TradePlanExecutionSummary struct {
	PlanID             uint `json:"planId"`
	ItemCount          int  `json:"itemCount"`
	PendingCount       int  `json:"pendingCount"`
	FilledCount        int  `json:"filledCount"`
	SkippedCount       int  `json:"skippedCount"`
	ErrorCount         int  `json:"errorCount"`
	EffectiveFilled    int  `json:"effectiveFilled"`
	StillPending       int  `json:"stillPending"`
	OrderFilledCount   int  `json:"orderFilledCount"`
	OrderRejectedCount int  `json:"orderRejectedCount"`
	OrderPendingCount  int  `json:"orderPendingCount"`
}

// FinishPlanCAS 终态迁移 CAS：仅 fromStatus 匹配时更新为 toStatus。
func (r *TradePlanRepo) FinishPlanCAS(planID uint, fromStatus, toStatus, message string) (bool, error) {
	if db.Dao == nil {
		return false, fmt.Errorf("数据库未初始化")
	}
	fromStatus = strings.TrimSpace(fromStatus)
	toStatus = strings.TrimSpace(toStatus)
	if planID == 0 || fromStatus == "" || toStatus == "" {
		return false, fmt.Errorf("invalid finish CAS args")
	}
	now := time.Now()
	res := db.Dao.Model(&models.TradePlan{}).
		Where("id = ? AND status = ?", planID, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"message":    message,
			"updated_at": now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// FinishPlan 兼容入口：executing → 终态（CAS）。
func (r *TradePlanRepo) FinishPlan(planID uint, status, message string) error {
	ok, err := r.FinishPlanCAS(planID, models.TradePlanStatusExecuting, status, message)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("finish plan CAS miss: planId=%d not executing", planID)
	}
	return nil
}

// ListExecutingPlans 列出 executed_at 早于 beforeTime 的 executing 计划。
func (r *TradePlanRepo) ListExecutingPlans(beforeTime time.Time) ([]models.TradePlan, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var plans []models.TradePlan
	err := db.Dao.Where("status = ? AND executed_at IS NOT NULL AND executed_at < ?",
		models.TradePlanStatusExecuting, beforeTime).
		Order("executed_at ASC").
		Find(&plans).Error
	return plans, err
}

// GetPlanExecutionSummary 聚合 item 与 paper_order 事实。
func (r *TradePlanRepo) GetPlanExecutionSummary(planID uint) (*TradePlanExecutionSummary, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	items, err := r.loadItems(planID)
	if err != nil {
		return nil, err
	}
	sum := &TradePlanExecutionSummary{PlanID: planID, ItemCount: len(items)}
	orderCache := map[uint]PaperOrder{}

	for _, it := range items {
		switch it.Status {
		case models.TradePlanItemPending, "":
			sum.PendingCount++
		case models.TradePlanItemFilled:
			sum.FilledCount++
		case models.TradePlanItemSkipped:
			sum.SkippedCount++
		case models.TradePlanItemError:
			sum.ErrorCount++
		}
		if it.OrderID == 0 {
			continue
		}
		order, ok := orderCache[it.OrderID]
		if !ok {
			if err := db.Dao.First(&order, it.OrderID).Error; err != nil {
				continue
			}
			orderCache[it.OrderID] = order
		}
		switch order.Status {
		case PaperOrderStatusFilled:
			sum.OrderFilledCount++
		case PaperOrderStatusRejected:
			sum.OrderRejectedCount++
		case PaperOrderStatusPending:
			sum.OrderPendingCount++
		}
	}

	for _, it := range items {
		if tradePlanItemEffectivelyFilled(it, orderCache) {
			sum.EffectiveFilled++
		} else if it.Status == models.TradePlanItemPending || it.Status == "" {
			sum.StillPending++
		}
	}
	return sum, nil
}

func tradePlanItemEffectivelyFilled(it models.TradePlanItem, orderCache map[uint]PaperOrder) bool {
	if it.OrderID > 0 {
		if order, ok := orderCache[it.OrderID]; ok {
			return order.Status == PaperOrderStatusFilled
		}
		if db.Dao != nil {
			var order PaperOrder
			if db.Dao.First(&order, it.OrderID).Error == nil && order.Status == PaperOrderStatusFilled {
				return true
			}
		}
	}
	return it.Status == models.TradePlanItemFilled
}

func (r *TradePlanRepo) UpdateItemExecution(item *models.TradePlanItem) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if item == nil || item.ID == 0 {
		return fmt.Errorf("invalid trade plan item")
	}
	item.UpdatedAt = time.Now()
	return db.Dao.Model(&models.TradePlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status":        item.Status,
		"error":         item.Error,
		"order_id":      item.OrderID,
		"fill_id":       item.FillID,
		"filled_price":  item.FilledPrice,
		"filled_volume": item.FilledVolume,
		"filled_fee":    item.FilledFee,
		"target_volume": item.TargetVolume,
		"stock_name":    item.StockName,
		"updated_at":    item.UpdatedAt,
	}).Error
}

// ErrNoReadyTradePlan 当日无 ready 计划。
var ErrNoReadyTradePlan = errors.New("no ready trade plan for trade date")

func IsNoReadyTradePlan(err error) bool {
	return errors.Is(err, ErrNoReadyTradePlan) || errors.Is(err, gorm.ErrRecordNotFound)
}

// EnsureTradePlanTables 供测试或延迟初始化（主路径走 main AutoMigrate）。
func EnsureTradePlanTables() error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return db.Dao.AutoMigrate(
		&models.CandidatePool{},
		&models.CandidatePoolItem{},
		&models.TradePlan{},
		&models.TradePlanItem{},
	)
}
