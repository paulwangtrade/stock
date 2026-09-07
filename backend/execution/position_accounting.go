package execution

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Phase6.5.7.5.1：持仓会计端口（Consumer TRADE 写界之一）。
// 只消费成交事实做净持仓/均价核算；不触碰 TradePlan / Intent / Frozen Spec / 委托价量。

// FillEvent 单笔成交入账事件（由 ExecutionReport TRADE 派生）。
type FillEvent struct {
	AccountID     string
	StockCode     string
	StockName     string
	Side          string // buy | sell
	Qty           int64
	Price         float64
	ClientOrderID string
	LocalOrderID  string
	ExecID        string
	OccurredAt    time.Time
}

// PositionAccountant 持仓会计端口；实现方负责幂等外的净头寸核算。
type PositionAccountant interface {
	ApplyFill(ctx context.Context, ev FillEvent) error
}

// AccountedPosition 内存持仓快照。
type AccountedPosition struct {
	AccountID string
	StockCode string
	StockName string
	Volume    int64   // 净持仓（买入累加，卖出减少）
	AvgCost   float64 // 买入加权成本；卖出不改成本
	Realized  float64 // 已实现盈亏（卖出 (price-avg)*qty）
}

// InMemoryPositionAccountant 进程内持仓会计（无 DB / 无 migration；供 Consumer 与测试使用）。
type InMemoryPositionAccountant struct {
	mu        sync.Mutex
	positions map[string]*AccountedPosition
}

// NewInMemoryPositionAccountant 创建空的内存会计器。
func NewInMemoryPositionAccountant() *InMemoryPositionAccountant {
	return &InMemoryPositionAccountant{positions: make(map[string]*AccountedPosition)}
}

func positionKey(accountID, stockCode string) string {
	return strings.TrimSpace(accountID) + "|" + strings.TrimSpace(stockCode)
}

// ApplyFill 按 buy/sell 更新净头寸与均价；不修改任何委托/计划字段。
func (a *InMemoryPositionAccountant) ApplyFill(ctx context.Context, ev FillEvent) error {
	_ = ctx
	if a == nil {
		return fmt.Errorf("execution: nil position accountant")
	}
	if ev.Qty <= 0 || ev.Price <= 0 {
		return fmt.Errorf("execution: invalid fill event qty=%d price=%.4f", ev.Qty, ev.Price)
	}
	side := strings.ToLower(strings.TrimSpace(ev.Side))
	if side == "" {
		side = "buy"
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	key := positionKey(ev.AccountID, ev.StockCode)
	pos := a.positions[key]
	if pos == nil {
		pos = &AccountedPosition{
			AccountID: strings.TrimSpace(ev.AccountID),
			StockCode: strings.TrimSpace(ev.StockCode),
			StockName: ev.StockName,
		}
		a.positions[key] = pos
	}
	if pos.StockName == "" && ev.StockName != "" {
		pos.StockName = ev.StockName
	}

	switch side {
	case "sell":
		if ev.Qty > pos.Volume {
			return fmt.Errorf("execution: sell qty %d exceeds position %d for %s", ev.Qty, pos.Volume, key)
		}
		pos.Realized += (ev.Price - pos.AvgCost) * float64(ev.Qty)
		pos.Volume -= ev.Qty
		if pos.Volume == 0 {
			pos.AvgCost = 0
		}
	default: // buy
		newVol := pos.Volume + ev.Qty
		pos.AvgCost = (pos.AvgCost*float64(pos.Volume) + ev.Price*float64(ev.Qty)) / float64(newVol)
		pos.Volume = newVol
	}
	return nil
}

// Position 返回持仓快照（值拷贝）；用于观测与测试。
func (a *InMemoryPositionAccountant) Position(accountID, stockCode string) (AccountedPosition, bool) {
	if a == nil {
		return AccountedPosition{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	pos, ok := a.positions[positionKey(accountID, stockCode)]
	if !ok || pos == nil {
		return AccountedPosition{}, false
	}
	return *pos, true
}
