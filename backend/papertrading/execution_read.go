// Execution Read Path — unified read-only access to execution facts (Phase10-H.2 / LEG-1).
// Primary: paper_sim_* ; Fallback: legacy paper_orders / paper_fills (never deleted).
// Does not write orders, fills, settlement, or plan status.

package papertrading

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Read-path source tags (Observation / Summary envelopes).
const (
	ExecutionReadSourcePaperSim       = "paper_sim"
	ExecutionReadSourceLegacyFallback = "paper_orders_fallback"
)

// ExecutionOrderFilter selects orders for ListOrders.
type ExecutionOrderFilter struct {
	TradeDate string
	PlanID    uint
	AccountID uint
}

// ExecutionFillFilter selects fills for ListFills.
type ExecutionFillFilter struct {
	TradeDate string
	PlanID    uint
	OrderID   uint
}

// ExecutionOrderRecord is a normalized read model (sim or legacy).
type ExecutionOrderRecord struct {
	ID           uint      `json:"id"`
	PlanID       uint      `json:"plan_id,omitempty"`
	PlanItemID   uint      `json:"plan_item_id,omitempty"`
	TradeDate    string    `json:"trade_date,omitempty"`
	StockCode    string    `json:"stock_code"`
	Side         string    `json:"side"`
	Status       string    `json:"status"`
	Quantity     int64     `json:"quantity"`
	FilledVolume int64     `json:"filled_volume"`
	FilledPrice  float64   `json:"filled_price"`
	OrderTime    time.Time `json:"order_time,omitempty"`
	Source       string    `json:"source"`
}

// ExecutionFillRecord is a normalized fill read model.
type ExecutionFillRecord struct {
	ID         uint      `json:"id"`
	OrderID    uint      `json:"order_id"`
	PlanID     uint      `json:"plan_id,omitempty"`
	StockCode  string    `json:"stock_code"`
	Side       string    `json:"side"`
	Price      float64   `json:"price"`
	Volume     int64     `json:"volume"`
	FilledAt   time.Time `json:"filled_at"`
	Source     string    `json:"source"`
}

// PlanExecutionSummaryView mirrors data.TradePlanExecutionSummary for reconcile / API mapping.
type PlanExecutionSummaryView struct {
	PlanID             uint   `json:"plan_id"`
	ItemCount          int    `json:"item_count"`
	PendingCount       int    `json:"pending_count"`
	FilledCount        int    `json:"filled_count"`
	SkippedCount       int    `json:"skipped_count"`
	ErrorCount         int    `json:"error_count"`
	EffectiveFilled    int    `json:"effective_filled"`
	StillPending       int    `json:"still_pending"`
	OrderFilledCount   int    `json:"order_filled_count"`
	OrderRejectedCount int    `json:"order_rejected_count"`
	OrderPendingCount  int    `json:"order_pending_count"`
	DataSource         string `json:"data_source"`
}

// ExecutionReadService is the sole Observation/Summary read facade for execution facts.
type ExecutionReadService interface {
	ListOrders(filter ExecutionOrderFilter) ([]ExecutionOrderRecord, string, error)
	ListFills(filter ExecutionFillFilter) ([]ExecutionFillRecord, string, error)
	BuildExecutionSummary(tradeDate string) (*ExecutionSummaryView, error)
	BuildPlanExecutionSummary(planID uint) (*PlanExecutionSummaryView, error)
	GetObservationMetrics(tradeDate string) (*ObservationMetrics, error)
}

// DefaultExecutionReadService returns the package singleton (sim-primary).
func DefaultExecutionReadService() ExecutionReadService {
	return defaultExecutionReader
}

var defaultExecutionReader ExecutionReadService = &SimExecutionReadService{}

// SetExecutionReadServiceForTest swaps the default reader (tests). Pass nil to restore.
func SetExecutionReadServiceForTest(svc ExecutionReadService) {
	if svc == nil {
		defaultExecutionReader = &SimExecutionReadService{}
		return
	}
	defaultExecutionReader = svc
}

// SimExecutionReadService reads paper_sim_* first; falls back to legacy paper_* tables.
type SimExecutionReadService struct{}

var _ ExecutionReadService = (*SimExecutionReadService)(nil)

func (s *SimExecutionReadService) ListOrders(filter ExecutionOrderFilter) ([]ExecutionOrderRecord, string, error) {
	if db.Dao == nil {
		return nil, "", fmt.Errorf("papertrading: db not initialized")
	}
	orders, err := s.listSimOrders(filter)
	if err != nil {
		return nil, "", err
	}
	if len(orders) > 0 {
		return orders, ExecutionReadSourcePaperSim, nil
	}
	legacy, lerr := s.listLegacyOrders(filter)
	if lerr != nil {
		return nil, ExecutionReadSourcePaperSim, lerr
	}
	if len(legacy) > 0 {
		return legacy, ExecutionReadSourceLegacyFallback, nil
	}
	return orders, ExecutionReadSourcePaperSim, nil
}

func (s *SimExecutionReadService) ListFills(filter ExecutionFillFilter) ([]ExecutionFillRecord, string, error) {
	if db.Dao == nil {
		return nil, "", fmt.Errorf("papertrading: db not initialized")
	}
	fills, err := s.listSimFills(filter)
	if err != nil {
		return nil, "", err
	}
	if len(fills) > 0 {
		return fills, ExecutionReadSourcePaperSim, nil
	}
	legacy, lerr := s.listLegacyFills(filter)
	if lerr != nil {
		return nil, ExecutionReadSourcePaperSim, lerr
	}
	if len(legacy) > 0 {
		return legacy, ExecutionReadSourceLegacyFallback, nil
	}
	return fills, ExecutionReadSourcePaperSim, nil
}

func (s *SimExecutionReadService) BuildExecutionSummary(tradeDate string) (*ExecutionSummaryView, error) {
	out := &ExecutionSummaryView{
		Enabled:        IsEnabled(),
		TradeDate:      strings.TrimSpace(tradeDate),
		AvgSlippage:    nil,
		DataSourceNote: "Execution Summary · read-only via ExecutionReadService; avg_slippage null without benchmark",
	}
	if !IsEnabled() {
		return out, nil
	}
	acc, err := GetDefaultAccount()
	if err != nil || acc == nil {
		return out, nil
	}
	recs, source, err := s.ListOrders(ExecutionOrderFilter{TradeDate: out.TradeDate, AccountID: acc.ID})
	if err != nil {
		return out, err
	}
	// Account-scoped sim may be empty while other accounts exist — also try plan-agnostic sim by date.
	if len(recs) == 0 && out.TradeDate != "" {
		recs, source, err = s.ListOrders(ExecutionOrderFilter{TradeDate: out.TradeDate})
		if err != nil {
			return out, err
		}
	}
	out.TotalOrders = len(recs)
	for _, o := range recs {
		st := strings.ToLower(strings.TrimSpace(o.Status))
		switch st {
		case strings.ToLower(OrderStatusFilled), "filled":
			out.FilledOrders++
		case strings.ToLower(OrderStatusRejected), "failed", "error", "cancelled", "rejected":
			out.FailedOrders++
		}
	}
	if out.TotalOrders > 0 {
		out.FillRate = float64(out.FilledOrders) / float64(out.TotalOrders)
	}
	out.DataSourceNote = fmt.Sprintf(
		"Execution Summary · source=%s; read-only; avg_slippage null without benchmark; does not alter fill prices",
		source,
	)
	return out, nil
}

func (s *SimExecutionReadService) BuildPlanExecutionSummary(planID uint) (*PlanExecutionSummaryView, error) {
	out := &PlanExecutionSummaryView{PlanID: planID, DataSource: ExecutionReadSourcePaperSim}
	if planID == 0 || db.Dao == nil {
		return out, nil
	}
	var items []models.TradePlanItem
	if err := db.Dao.Where("plan_id = ?", planID).Order("id asc").Find(&items).Error; err != nil {
		return out, err
	}
	out.ItemCount = len(items)

	simByItem := map[uint]ExecutionOrderRecord{}
	simOrders, _, err := s.ListOrders(ExecutionOrderFilter{PlanID: planID})
	if err != nil {
		return out, err
	}
	usedSim := false
	for _, o := range simOrders {
		if o.Source == ExecutionReadSourcePaperSim || o.PlanItemID > 0 {
			usedSim = true
		}
		if o.PlanItemID > 0 {
			simByItem[o.PlanItemID] = o
		}
	}

	legacyByID := map[uint]string{} // orderID → status
	needLegacy := false
	for _, it := range items {
		if _, ok := simByItem[it.ID]; !ok && it.OrderID > 0 {
			needLegacy = true
		}
	}
	if needLegacy || (!usedSim && len(simOrders) == 0) {
		for _, it := range items {
			if it.OrderID == 0 {
				continue
			}
			if _, ok := legacyByID[it.OrderID]; ok {
				continue
			}
			st, ok := lookupLegacyOrderStatus(it.OrderID)
			if ok {
				legacyByID[it.OrderID] = st
			}
		}
		if len(legacyByID) > 0 && !usedSim {
			out.DataSource = ExecutionReadSourceLegacyFallback
		} else if len(legacyByID) > 0 && usedSim {
			out.DataSource = ExecutionReadSourcePaperSim + "+legacy_partial"
		}
	}

	orderStatusForItem := func(it models.TradePlanItem) (status string, ok bool) {
		if o, hit := simByItem[it.ID]; hit {
			return o.Status, true
		}
		if it.OrderID > 0 {
			if st, hit := legacyByID[it.OrderID]; hit {
				return st, true
			}
		}
		return "", false
	}

	for _, it := range items {
		switch it.Status {
		case models.TradePlanItemPending, "":
			out.PendingCount++
		case models.TradePlanItemFilled:
			out.FilledCount++
		case models.TradePlanItemSkipped:
			out.SkippedCount++
		case models.TradePlanItemError:
			out.ErrorCount++
		}
		st, ok := orderStatusForItem(it)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(st)) {
		case "filled":
			out.OrderFilledCount++
		case "rejected", "cancelled":
			out.OrderRejectedCount++
		case "pending", "created", "submitted", "processing":
			out.OrderPendingCount++
		}
	}

	for _, it := range items {
		if planItemEffectivelyFilled(it, orderStatusForItem) {
			out.EffectiveFilled++
		} else if it.Status == models.TradePlanItemPending || it.Status == "" {
			out.StillPending++
		}
	}
	return out, nil
}

func planItemEffectivelyFilled(it models.TradePlanItem, statusFn func(models.TradePlanItem) (string, bool)) bool {
	if st, ok := statusFn(it); ok && strings.EqualFold(strings.TrimSpace(st), "filled") {
		return true
	}
	return it.Status == models.TradePlanItemFilled
}

func (s *SimExecutionReadService) GetObservationMetrics(tradeDate string) (*ObservationMetrics, error) {
	return GetObservationMetrics(tradeDate)
}

// --- sim loaders ---

func (s *SimExecutionReadService) listSimOrders(filter ExecutionOrderFilter) ([]ExecutionOrderRecord, error) {
	if !db.Dao.Migrator().HasTable(&PaperSimOrder{}) {
		return nil, nil
	}
	q := db.Dao.Model(&PaperSimOrder{})
	if filter.AccountID > 0 {
		q = q.Where("account_id = ?", filter.AccountID)
	}
	if filter.PlanID > 0 {
		q = q.Where("plan_id = ?", filter.PlanID)
	}
	if td := strings.TrimSpace(filter.TradeDate); td != "" {
		q = q.Where("trade_date = ?", td)
	}
	var rows []PaperSimOrder
	if err := q.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ExecutionOrderRecord, 0, len(rows))
	for _, o := range rows {
		out = append(out, ExecutionOrderRecord{
			ID: o.ID, PlanID: o.PlanID, PlanItemID: o.PlanItemID, TradeDate: o.TradeDate,
			StockCode: o.StockCode, Side: o.Side, Status: o.Status,
			Quantity: o.Quantity, FilledVolume: o.FilledVolume, FilledPrice: o.FilledPrice,
			OrderTime: o.OrderTime, Source: ExecutionReadSourcePaperSim,
		})
	}
	return out, nil
}

func (s *SimExecutionReadService) listSimFills(filter ExecutionFillFilter) ([]ExecutionFillRecord, error) {
	if !db.Dao.Migrator().HasTable(&PaperSimFill{}) {
		return nil, nil
	}
	q := db.Dao.Model(&PaperSimFill{})
	if filter.PlanID > 0 {
		q = q.Where("plan_id = ?", filter.PlanID)
	}
	if filter.OrderID > 0 {
		q = q.Where("order_id = ?", filter.OrderID)
	}
	if td := strings.TrimSpace(filter.TradeDate); td != "" && db.Dao.Migrator().HasTable(&PaperSimOrder{}) {
		var rows []PaperSimFill
		err := db.Dao.Table("paper_sim_fills AS f").
			Select("f.*").
			Joins("JOIN paper_sim_orders AS o ON o.id = f.order_id").
			Where("o.trade_date = ?", td).
			Order("f.id asc").
			Find(&rows).Error
		if err != nil {
			return nil, err
		}
		return mapSimFills(rows), nil
	}
	var rows []PaperSimFill
	if err := q.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return mapSimFills(rows), nil
}

func mapSimFills(rows []PaperSimFill) []ExecutionFillRecord {
	out := make([]ExecutionFillRecord, 0, len(rows))
	for _, f := range rows {
		out = append(out, ExecutionFillRecord{
			ID: f.ID, OrderID: f.OrderID, PlanID: f.PlanID, StockCode: f.StockCode,
			Side: f.Side, Price: f.Price, Volume: f.Volume, FilledAt: f.FilledAt,
			Source: ExecutionReadSourcePaperSim,
		})
	}
	return out
}

// --- legacy fallback (table scan; no import of data.PaperOrder to keep boundary clear) ---

type legacyOrderRow struct {
	ID         uint      `gorm:"column:id"`
	StockCode  string    `gorm:"column:stock_code"`
	Side       string    `gorm:"column:side"`
	Status     string    `gorm:"column:status"`
	Volume     int64     `gorm:"column:volume"`
	FilledVol  int64     `gorm:"column:filled_vol"`
	FilledPrice float64  `gorm:"column:filled_price"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

type legacyFillRow struct {
	ID        uint      `gorm:"column:id"`
	OrderID   uint      `gorm:"column:order_id"`
	StockCode string    `gorm:"column:stock_code"`
	Side      string    `gorm:"column:side"`
	Price     float64   `gorm:"column:price"`
	Volume    int64     `gorm:"column:volume"`
	FilledAt  time.Time `gorm:"column:filled_at"`
}

func (s *SimExecutionReadService) listLegacyOrders(filter ExecutionOrderFilter) ([]ExecutionOrderRecord, error) {
	if !db.Dao.Migrator().HasTable("paper_orders") {
		return nil, nil
	}
	// Legacy orders lack plan_id; plan-scoped queries cannot use legacy as primary.
	if filter.PlanID > 0 {
		return nil, nil
	}
	q := db.Dao.Table("paper_orders")
	if td := strings.TrimSpace(filter.TradeDate); td != "" {
		q = q.Where("date(created_at) = ?", td)
	}
	var rows []legacyOrderRow
	if err := q.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ExecutionOrderRecord, 0, len(rows))
	for _, o := range rows {
		out = append(out, ExecutionOrderRecord{
			ID: o.ID, StockCode: o.StockCode, Side: o.Side, Status: o.Status,
			Quantity: o.Volume, FilledVolume: o.FilledVol, FilledPrice: o.FilledPrice,
			OrderTime: o.CreatedAt, TradeDate: strings.TrimSpace(filter.TradeDate),
			Source: ExecutionReadSourceLegacyFallback,
		})
	}
	return out, nil
}

func (s *SimExecutionReadService) listLegacyFills(filter ExecutionFillFilter) ([]ExecutionFillRecord, error) {
	if !db.Dao.Migrator().HasTable("paper_fills") {
		return nil, nil
	}
	if filter.PlanID > 0 {
		return nil, nil
	}
	q := db.Dao.Table("paper_fills")
	if filter.OrderID > 0 {
		q = q.Where("order_id = ?", filter.OrderID)
	}
	if td := strings.TrimSpace(filter.TradeDate); td != "" {
		q = q.Where("date(filled_at) = ?", td)
	}
	var rows []legacyFillRow
	if err := q.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ExecutionFillRecord, 0, len(rows))
	for _, f := range rows {
		out = append(out, ExecutionFillRecord{
			ID: f.ID, OrderID: f.OrderID, StockCode: f.StockCode, Side: f.Side,
			Price: f.Price, Volume: f.Volume, FilledAt: f.FilledAt,
			Source: ExecutionReadSourceLegacyFallback,
		})
	}
	return out, nil
}

func lookupLegacyOrderStatus(orderID uint) (string, bool) {
	if orderID == 0 || db.Dao == nil || !db.Dao.Migrator().HasTable("paper_orders") {
		return "", false
	}
	var row legacyOrderRow
	if err := db.Dao.Table("paper_orders").Select("id, status").Where("id = ?", orderID).First(&row).Error; err != nil {
		return "", false
	}
	return row.Status, true
}
