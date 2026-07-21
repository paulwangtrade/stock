package data

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
)

const (
	PaperOrderOrphanIOC        = "IOC_ORPHAN"
	PaperOrderOrphanProcessing = "PROCESSING_STUCK"
	PaperOrderOrphanPendingTO  = "PENDING_TIMEOUT"
	PaperOrderRejectEmptyCode  = "EMPTY_CODE"

	// OBS-2 生命周期完整性缺口
	PaperOrderGapFilledMissingEvent    = "FILLED_MISSING_EVENT"
	PaperOrderGapFilledMissingFill     = "FILLED_MISSING_FILL"
	PaperOrderGapRejectedMissingEvent  = "REJECTED_MISSING_EVENT"
	PaperOrderGapPendingMissingSubmit  = "PENDING_MISSING_SUBMITTED"
	PaperOrderGapOrphanEvent           = "ORPHAN_EVENT"

	defaultProcessingStuckAge = 60 * time.Second
	defaultPendingTimeoutAge  = 24 * time.Hour
)

// PaperOrderHealthCounts 订单状态计数。
// submitted = 当日订单总数（落库即视为已提交）。
type PaperOrderHealthCounts struct {
	Submitted  int `json:"submitted"`
	Pending    int `json:"pending"`
	Filled     int `json:"filled"`
	Rejected   int `json:"rejected"`
	Cancelled  int `json:"cancelled"`
	Processing int `json:"processing"`
}

// PaperOrderHealthRates 成交/拒绝/挂单比例。
type PaperOrderHealthRates struct {
	FillRate     float64 `json:"fillRate"`
	RejectRate   float64 `json:"rejectRate"`
	PendingRatio float64 `json:"pendingRatio"`
}

// PaperOrderRejectStat reject_code 聚合。
type PaperOrderRejectStat struct {
	Code       string  `json:"code"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// PaperOrderOrphan 孤儿订单诊断项（只读）。
type PaperOrderOrphan struct {
	OrderID    uint    `json:"orderId"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Price      float64 `json:"price"`
	Volume     int64   `json:"volume"`
	ExecMode   string  `json:"execMode"`
	AgeSeconds int64   `json:"age"`
	OrphanKind string  `json:"orphanKind"`
	Hint       string  `json:"hint"`
}

// PaperOrderLifecycleGap 订单/事件链路完整性缺口（只读）。
type PaperOrderLifecycleGap struct {
	OrderID uint   `json:"orderId"`
	Symbol  string `json:"symbol"`
	Issue   string `json:"issue"`
	Detail  string `json:"detail"`
}

// PaperOrderHealthReport 数据库侧健康报告（OBS-1 counts/orphans + OBS-2 lifecycleGaps）。
type PaperOrderHealthReport struct {
	TradeDate       string                    `json:"tradeDate"`
	Counts          PaperOrderHealthCounts    `json:"counts"`
	Rates           PaperOrderHealthRates     `json:"rates"`
	RejectBreakdown []PaperOrderRejectStat    `json:"rejectBreakdown"`
	Orphans         []PaperOrderOrphan        `json:"orphans"`
	LifecycleGaps   []PaperOrderLifecycleGap  `json:"lifecycleGaps"`
	Message         string                    `json:"message"`
}

// GetPaperOrderHealth 按订单 created_at 日历日聚合只读健康报告；date 空则今日。
func GetPaperOrderHealth(date string) (*PaperOrderHealthReport, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	date = strings.TrimSpace(date)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	now := time.Now()

	var orders []PaperOrder
	// SQLite / GORM：按本地日期字符串比较 created_at 前缀
	err := db.Dao.Where("strftime('%Y-%m-%d', created_at) = ?", date).
		Order("id ASC").Find(&orders).Error
	if err != nil {
		return nil, err
	}

	report := &PaperOrderHealthReport{
		TradeDate:       date,
		RejectBreakdown: []PaperOrderRejectStat{},
		Orphans:         []PaperOrderOrphan{},
		LifecycleGaps:   []PaperOrderLifecycleGap{},
	}
	report.Counts.Submitted = len(orders)

	rejectByCode := map[string]int{}
	orderIDs := make([]uint, 0, len(orders))
	orderByID := map[uint]PaperOrder{}
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
		orderByID[o.ID] = o
		switch o.Status {
		case PaperOrderStatusPending:
			report.Counts.Pending++
		case PaperOrderStatusFilled:
			report.Counts.Filled++
		case PaperOrderStatusRejected:
			report.Counts.Rejected++
			code := strings.TrimSpace(o.RejectCode)
			if code == "" {
				code = PaperOrderRejectEmptyCode
			}
			rejectByCode[code]++
		case PaperOrderStatusCancelled:
			report.Counts.Cancelled++
		case PaperOrderStatusProcessing:
			report.Counts.Processing++
		}

		if orphan, ok := classifyPaperOrderOrphan(o, now); ok {
			report.Orphans = append(report.Orphans, orphan)
		}
	}

	eventsByOrder, fillOrderIDs, err := loadLifecycleAuditIndexes(orderIDs, date)
	if err != nil {
		return nil, err
	}
	report.LifecycleGaps = collectLifecycleGaps(orders, eventsByOrder, fillOrderIDs)
	orphanGaps, err := collectOrphanEventGaps(date, orderByID)
	if err != nil {
		return nil, err
	}
	report.LifecycleGaps = append(report.LifecycleGaps, orphanGaps...)

	report.Rates = calcPaperOrderHealthRates(report.Counts)
	report.RejectBreakdown = buildRejectBreakdown(rejectByCode, report.Counts.Rejected)
	report.Message = fmt.Sprintf(
		"date=%s submitted=%d filled=%d rejected=%d pending=%d orphans=%d gaps=%d",
		date, report.Counts.Submitted, report.Counts.Filled, report.Counts.Rejected,
		report.Counts.Pending, len(report.Orphans), len(report.LifecycleGaps),
	)
	return report, nil
}

// loadLifecycleAuditIndexes 只读加载当日订单相关 events / fills 索引。
func loadLifecycleAuditIndexes(orderIDs []uint, date string) (map[uint]map[string]bool, map[uint]bool, error) {
	eventsByOrder := map[uint]map[string]bool{}
	fillOrderIDs := map[uint]bool{}
	if len(orderIDs) == 0 {
		return eventsByOrder, fillOrderIDs, nil
	}

	var events []PaperOrderEvent
	if err := db.Dao.Where("order_id IN ?", orderIDs).Find(&events).Error; err != nil {
		return nil, nil, err
	}
	for _, ev := range events {
		if eventsByOrder[ev.OrderID] == nil {
			eventsByOrder[ev.OrderID] = map[string]bool{}
		}
		eventsByOrder[ev.OrderID][ev.EventType] = true
	}

	var fills []PaperFill
	if err := db.Dao.Where("order_id IN ?", orderIDs).Find(&fills).Error; err != nil {
		return nil, nil, err
	}
	for _, f := range fills {
		fillOrderIDs[f.OrderID] = true
	}
	_ = date
	return eventsByOrder, fillOrderIDs, nil
}

func collectLifecycleGaps(
	orders []PaperOrder,
	eventsByOrder map[uint]map[string]bool,
	fillOrderIDs map[uint]bool,
) []PaperOrderLifecycleGap {
	gaps := make([]PaperOrderLifecycleGap, 0)
	hasEvent := func(orderID uint, typ string) bool {
		m := eventsByOrder[orderID]
		return m != nil && m[typ]
	}
	for _, o := range orders {
		switch o.Status {
		case PaperOrderStatusFilled:
			if !hasEvent(o.ID, PaperOrderEventFilled) {
				gaps = append(gaps, PaperOrderLifecycleGap{
					OrderID: o.ID, Symbol: o.StockCode,
					Issue:  PaperOrderGapFilledMissingEvent,
					Detail: "status=filled 但缺少 order_filled 审计事件",
				})
			}
			if !fillOrderIDs[o.ID] {
				gaps = append(gaps, PaperOrderLifecycleGap{
					OrderID: o.ID, Symbol: o.StockCode,
					Issue:  PaperOrderGapFilledMissingFill,
					Detail: "status=filled 但缺少 paper_fills 成交行",
				})
			}
		case PaperOrderStatusRejected:
			if !hasEvent(o.ID, PaperOrderEventRejected) {
				gaps = append(gaps, PaperOrderLifecycleGap{
					OrderID: o.ID, Symbol: o.StockCode,
					Issue:  PaperOrderGapRejectedMissingEvent,
					Detail: "status=rejected 但缺少 order_rejected 审计事件",
				})
			}
		case PaperOrderStatusPending:
			if !hasEvent(o.ID, PaperOrderEventSubmitted) {
				gaps = append(gaps, PaperOrderLifecycleGap{
					OrderID: o.ID, Symbol: o.StockCode,
					Issue:  PaperOrderGapPendingMissingSubmit,
					Detail: "status=pending 但缺少 order_submitted 审计事件",
				})
			}
		}
	}
	return gaps
}

// collectOrphanEventGaps 当日事件指向不存在的订单。
func collectOrphanEventGaps(date string, dayOrders map[uint]PaperOrder) ([]PaperOrderLifecycleGap, error) {
	var events []PaperOrderEvent
	err := db.Dao.Where("strftime('%Y-%m-%d', created_at) = ?", date).
		Order("id ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	// 收集事件中的 order_id，批量查是否存在（含非当日订单）
	idSet := map[uint]bool{}
	ids := make([]uint, 0)
	for _, ev := range events {
		if ev.OrderID == 0 || idSet[ev.OrderID] {
			continue
		}
		idSet[ev.OrderID] = true
		ids = append(ids, ev.OrderID)
	}
	existing := map[uint]bool{}
	if len(ids) > 0 {
		var rows []PaperOrder
		if err := db.Dao.Select("id").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			existing[r.ID] = true
		}
	}

	seenOrphan := map[uint]bool{}
	gaps := make([]PaperOrderLifecycleGap, 0)
	for _, ev := range events {
		if ev.OrderID == 0 || existing[ev.OrderID] || seenOrphan[ev.OrderID] {
			continue
		}
		// 当日订单表里也没有（dayOrders 仅为辅助；权威是 existing）
		if _, inDay := dayOrders[ev.OrderID]; inDay {
			continue
		}
		seenOrphan[ev.OrderID] = true
		symbol := symbolFromEventPayload(ev.PayloadJSON)
		gaps = append(gaps, PaperOrderLifecycleGap{
			OrderID: ev.OrderID,
			Symbol:  symbol,
			Issue:   PaperOrderGapOrphanEvent,
			Detail:  fmt.Sprintf("存在 %s 事件但 paper_orders 无对应订单", ev.EventType),
		})
	}
	return gaps, nil
}

func symbolFromEventPayload(payloadJSON string) string {
	if strings.TrimSpace(payloadJSON) == "" {
		return ""
	}
	var p struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return ""
	}
	return p.Symbol
}

func calcPaperOrderHealthRates(c PaperOrderHealthCounts) PaperOrderHealthRates {
	r := PaperOrderHealthRates{}
	decided := c.Filled + c.Rejected
	if decided > 0 {
		r.FillRate = float64(c.Filled) / float64(decided)
		r.RejectRate = float64(c.Rejected) / float64(decided)
	}
	if c.Submitted > 0 {
		r.PendingRatio = float64(c.Pending) / float64(c.Submitted)
	}
	return r
}

func buildRejectBreakdown(byCode map[string]int, rejectedTotal int) []PaperOrderRejectStat {
	order := []string{
		PaperOrderRejectCashInsufficient,
		PaperOrderRejectPositionInsufficient,
		PaperOrderRejectInvalidOrder,
		PaperOrderRejectInternal,
		PaperOrderRejectEmptyCode,
	}
	seen := map[string]bool{}
	out := make([]PaperOrderRejectStat, 0, len(byCode))
	appendStat := func(code string) {
		cnt := byCode[code]
		if cnt == 0 || seen[code] {
			return
		}
		seen[code] = true
		pct := 0.0
		if rejectedTotal > 0 {
			pct = float64(cnt) / float64(rejectedTotal)
		}
		out = append(out, PaperOrderRejectStat{Code: code, Count: cnt, Percentage: pct})
	}
	for _, code := range order {
		appendStat(code)
	}
	extras := make([]string, 0)
	for code := range byCode {
		if !seen[code] {
			extras = append(extras, code)
		}
	}
	sort.Strings(extras)
	for _, code := range extras {
		appendStat(code)
	}
	return out
}

func classifyPaperOrderOrphan(o PaperOrder, now time.Time) (PaperOrderOrphan, bool) {
	age := now.Sub(o.CreatedAt)
	if age < 0 {
		age = 0
	}
	exec := strings.TrimSpace(o.ExecMode)

	base := PaperOrderOrphan{
		OrderID:    o.ID,
		Symbol:     o.StockCode,
		Side:       o.Side,
		Price:      o.Price,
		Volume:     o.Volume,
		ExecMode:   exec,
		AgeSeconds: int64(age.Seconds()),
	}

	if o.Status == PaperOrderStatusProcessing && age >= defaultProcessingStuckAge {
		base.OrphanKind = PaperOrderOrphanProcessing
		base.Hint = "processing 超过 60s，可能 Fill 事务中断残留"
		return base, true
	}

	if o.Status != PaperOrderStatusPending {
		return PaperOrderOrphan{}, false
	}

	// resting pending：不判故障
	if exec == PaperOrderExecModeResting {
		return PaperOrderOrphan{}, false
	}

	if exec == PaperOrderExecModeIOCAutofill {
		base.OrphanKind = PaperOrderOrphanIOC
		base.Hint = "ioc_autofill 订单仍为 pending，AutoFill 失败本应 rejected"
		return base, true
	}

	if age >= defaultPendingTimeoutAge {
		base.OrphanKind = PaperOrderOrphanPendingTO
		base.Hint = "pending 超过 24h"
		return base, true
	}

	return PaperOrderOrphan{}, false
}
