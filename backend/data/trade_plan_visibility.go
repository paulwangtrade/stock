package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"
	"go-stock/backend/tradingwindow"

	"gorm.io/gorm"
)

// ErrNoUpcomingTradePlan 表示 today 及之后无可展示的 Frozen/draft 计划。
var ErrNoUpcomingTradePlan = errors.New("no upcoming trade plan")

// IsNoUpcomingTradePlan 判断是否为「无 upcoming」语义（含 record not found）。
func IsNoUpcomingTradePlan(err error) bool {
	return errors.Is(err, ErrNoUpcomingTradePlan) || errors.Is(err, gorm.ErrRecordNotFound)
}

// TradePlanVisibilityView 只读可视化 DTO（不直接暴露 models.TradePlan）。
type TradePlanVisibilityView struct {
	ID            uint       `json:"id"`
	TradeDate     string     `json:"tradeDate"`
	PlanVersion   int        `json:"planVersion"`
	Status        string     `json:"status"`
	SourceSession string     `json:"sourceSession"`
	GeneratedAt   *time.Time `json:"generatedAt,omitempty"`
	PoolID        uint       `json:"poolId"`
	RiskPassed    bool       `json:"riskPassed"`
	RiskReasons   []string   `json:"riskReasons"`
	ApprovedAt    *time.Time `json:"approvedAt,omitempty"`
	ApprovedBy    string     `json:"approvedBy"`
	FreezeAt      *time.Time `json:"freezeAt,omitempty"`
	FreezeBy      string     `json:"freezeBy"`
	FreezeReason  string     `json:"freezeReason"`
	IsFrozen      bool       `json:"isFrozen"`
	WindowStatus  string     `json:"windowStatus"`
	WindowReason  string     `json:"windowReason"`
	OpenWindowStart string   `json:"openWindowStart"`
	OpenWindowEnd   string   `json:"openWindowEnd"`
	FreezeDeadline  string   `json:"freezeDeadline"`
	Items         []TradePlanVisibilityItemView `json:"items"`
}

// TradePlanVisibilityItemView 计划明细只读视图。
// Execution Preview 字段（RefPrice/LimitPrice/TargetVolume/EntryRule/IntentStatus）
// 从 trade_plan_items 只读投影；AfterClose 阶段 LimitPrice/TargetVolume 为 0 属正常。
type TradePlanVisibilityItemView struct {
	ID            uint    `json:"id"` // Phase13-D: read-only for strategy explanation wiring
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	Side          string  `json:"side"`
	Priority      int     `json:"priority"`
	TargetAmount  float64 `json:"targetAmount"`
	Status        string  `json:"status"`
	Score         float64 `json:"score"`
	RiskCode      string  `json:"riskCode"`
	RiskMessage   string  `json:"riskMessage"`
	StrategyName  string  `json:"strategyName"`
	RefPrice      float64 `json:"refPrice"`
	LimitPrice    float64 `json:"limitPrice"`
	TargetVolume  int64   `json:"targetVolume"`
	EntryRule     string  `json:"entryRule"`
	IntentStatus  string  `json:"intentStatus"`
	Reason        string  `json:"reason"`
}

// GetUpcomingTradePlan 只读查询即将交易的计划（可视化用，不改变任何交易状态）。
//
// 输入 today：源日期（通常为「今天」），筛选 trade_date >= today。
// 优先：Frozen（status=ready AND freeze_at IS NOT NULL）；
// 否则：最新 draft。
// 排序：trade_date ASC, plan_version DESC, id DESC。
func (r *TradePlanRepo) GetUpcomingTradePlan(today string) (*TradePlanVisibilityView, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	today = strings.TrimSpace(today)
	if today == "" {
		return nil, fmt.Errorf("today/source date is required")
	}

	plan, err := r.findUpcomingFrozen(today)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if plan == nil {
		plan, err = r.findUpcomingDraft(today)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNoUpcomingTradePlan
			}
			return nil, err
		}
	}

	items, err := r.loadItems(plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Items = items
	return toTradePlanVisibilityView(plan), nil
}

// GetTradePlanVisibilityByID loads one plan by primary key for read-only UI (exact id; no Frozen priority).
func (r *TradePlanRepo) GetTradePlanVisibilityByID(id uint) (*TradePlanVisibilityView, error) {
	if id == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	plan, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	return toTradePlanVisibilityView(plan), nil
}

func (r *TradePlanRepo) findUpcomingFrozen(today string) (*models.TradePlan, error) {
	var plan models.TradePlan
	err := db.Dao.Where(
		"trade_date >= ? AND status = ? AND freeze_at IS NOT NULL",
		today, models.TradePlanStatusReady,
	).Order("trade_date ASC, plan_version DESC, id DESC").First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *TradePlanRepo) findUpcomingDraft(today string) (*models.TradePlan, error) {
	var plan models.TradePlan
	err := db.Dao.Where(
		"trade_date >= ? AND status = ?",
		today, models.TradePlanStatusDraft,
	).Order("trade_date ASC, plan_version DESC, id DESC").First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func toTradePlanVisibilityView(plan *models.TradePlan) *TradePlanVisibilityView {
	if plan == nil {
		return nil
	}
	view := &TradePlanVisibilityView{
		ID:            plan.ID,
		TradeDate:     plan.TradeDate,
		PlanVersion:   plan.PlanVersion,
		Status:        plan.Status,
		SourceSession: plan.SourceSession,
		GeneratedAt:   generatedAtPtr(plan.GeneratedAt),
		PoolID:        plan.PoolID,
		RiskPassed:    visibilityRiskPassed(plan.RiskStatus),
		RiskReasons:   visibilityRiskReasons(plan),
		ApprovedAt:    plan.ApprovedAt,
		ApprovedBy:    plan.ApprovedBy,
		FreezeAt:      plan.FreezeAt,
		FreezeBy:      plan.FreezeBy,
		FreezeReason:  plan.FreezeReason,
		IsFrozen:      plan.IsFrozen(),
		Items:         make([]TradePlanVisibilityItemView, 0, len(plan.Items)),
	}
	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   plan.TradeDate,
		CurrentTime: time.Now(),
		PlanStatus:  plan.Status,
		IsFrozen:    plan.IsFrozen(),
		FrozenTime:  plan.FreezeAt,
	})
	view.WindowStatus = string(window.Status)
	view.WindowReason = window.Reason
	view.OpenWindowStart = window.OpenWindowStart
	view.OpenWindowEnd = window.OpenWindowEnd
	view.FreezeDeadline = window.FreezeDeadline

	for _, it := range plan.Items {
		view.Items = append(view.Items, TradePlanVisibilityItemView{
			ID:           it.ID,
			StockCode:    it.StockCode,
			StockName:    it.StockName,
			Side:         it.Side,
			Priority:     it.Priority,
			TargetAmount: it.TargetAmount,
			Status:       it.Status,
			Score:        it.Score,
			RiskCode:     it.RiskCode,
			RiskMessage:  it.RiskMessage,
			StrategyName: it.StrategyName,
			RefPrice:     it.RefPrice,
			LimitPrice:   it.LimitPrice,
			TargetVolume: it.TargetVolume,
			EntryRule:    it.EntryRule,
			IntentStatus: it.IntentStatus,
			Reason:       it.Reason,
		})
	}
	return view
}

// generatedAtPtr 返回非零生成时间的指针（只读观测，不改任何生成逻辑）。
func generatedAtPtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func visibilityRiskPassed(riskStatus string) bool {
	switch strings.TrimSpace(riskStatus) {
	case risk.PlanRiskStatusPassed, risk.PlanRiskStatusBypassed:
		return true
	default:
		return false
	}
}

// visibilityRiskReasons 从已持久化的 Risk 字段只读拼装（不重跑 PlanFilter）。
func visibilityRiskReasons(plan *models.TradePlan) []string {
	if plan == nil {
		return nil
	}
	reasons := make([]string, 0)
	seen := map[string]bool{}
	for _, it := range plan.Items {
		code := strings.TrimSpace(it.RiskCode)
		msg := strings.TrimSpace(it.RiskMessage)
		line := code
		if msg != "" {
			if line != "" {
				line = code + ": " + msg
			} else {
				line = msg
			}
		}
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		reasons = append(reasons, line)
	}
	summary := strings.TrimSpace(plan.RiskSummary)
	if len(reasons) == 0 && summary != "" {
		switch strings.TrimSpace(plan.RiskStatus) {
		case risk.PlanRiskStatusBlocked, risk.PlanRiskStatusPartial:
			reasons = append(reasons, summary)
		}
	}
	return reasons
}
