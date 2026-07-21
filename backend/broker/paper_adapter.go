package broker

import (
	"context"
	"time"
)

// LegacyExecutionAdapter 将旧 Adapter 提升为 ExecutionAdapter。
type LegacyExecutionAdapter struct {
	Adapter
}

func NewLegacyExecutionAdapter(adapter Adapter) *LegacyExecutionAdapter {
	return &LegacyExecutionAdapter{Adapter: adapter}
}

func (a *LegacyExecutionAdapter) SubmitOrder(ctx context.Context, order TradeOrder) (*TradeOrder, error) {
	result, err := a.PlaceOrder(ctx, OrderRequest{
		StockCode: order.StockCode,
		StockName: order.StockName,
		Side:      order.Side,
		Price:     order.Price,
		Volume:    order.Volume,
		Reason:    order.Reason,
	})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	order.BrokerID = result.BrokerID
	order.Status = result.RawStatus
	if order.CreatedAt.IsZero() {
		order.CreatedAt = now
	}
	order.UpdatedAt = now
	return &order, nil
}

func (a *LegacyExecutionAdapter) QueryOrders(ctx context.Context, accountID string) ([]TradeOrder, error) {
	_ = ctx
	_ = accountID
	return []TradeOrder{}, nil
}

func (a *LegacyExecutionAdapter) QueryTradePositions(ctx context.Context, accountID string) ([]TradePosition, error) {
	positions, err := a.QueryPositions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]TradePosition, 0, len(positions))
	for _, position := range positions {
		result = append(result, TradePosition{
			AccountID: accountID,
			StockCode: position.StockCode,
			Volume:    position.Volume,
			Sellable:  position.Sellable,
			AvgCost:   position.AvgCost,
		})
	}
	return result, nil
}

func (a *LegacyExecutionAdapter) QueryAccount(ctx context.Context, accountID string) (*TradeAccount, error) {
	_ = ctx
	return &TradeAccount{ID: accountID}, nil
}

// PaperEventBridge 隔离模拟撮合实现与具体事件总线。
type PaperEventBridge struct {
	Hub    *EventHub
	Source string
}

func NewPaperEventBridge(hub *EventHub) *PaperEventBridge {
	if hub == nil {
		hub = DefaultHub
	}
	return &PaperEventBridge{Hub: hub, Source: "paper"}
}

func (b *PaperEventBridge) Publish(event TradingEvent) {
	if event.Source == "" {
		event.Source = b.Source
	}
	b.Hub.Publish(event)
}

func (b *PaperEventBridge) OrderSubmitted(order TradeOrder) {
	b.Publish(TradingEvent{
		Type:      EventOrderSubmitted,
		AccountID: order.AccountID,
		OrderID:   order.ID,
		Order:     &order,
	})
}

// OrderFilled 生命周期：Fill 事务成功提交后发布（与 EventFill 双发兼容）。
func (b *PaperEventBridge) OrderFilled(order TradeOrder) {
	b.Publish(TradingEvent{
		Type:      EventOrderFilled,
		AccountID: order.AccountID,
		OrderID:   order.ID,
		Order:     &order,
	})
}

// OrderRejected 生命周期：rejected 状态提交后发布。
func (b *PaperEventBridge) OrderRejected(order TradeOrder) {
	b.Publish(TradingEvent{
		Type:      EventOrderRejected,
		AccountID: order.AccountID,
		OrderID:   order.ID,
		Order:     &order,
	})
}

func (b *PaperEventBridge) Filled(fill TradeFill) {
	b.Publish(TradingEvent{
		Type:      EventFill,
		AccountID: fill.AccountID,
		OrderID:   fill.OrderID,
		Fill:      &fill,
	})
}

func (b *PaperEventBridge) PositionChanged(position TradePosition) {
	b.Publish(TradingEvent{
		Type:       EventPositionChanged,
		AccountID:  position.AccountID,
		Position:   &position,
		OccurredAt: position.UpdatedAt,
	})
}

func (b *PaperEventBridge) AccountChanged(account TradeAccount) {
	b.Publish(TradingEvent{
		Type:       EventAccountChanged,
		AccountID:  account.ID,
		Account:    &account,
		OccurredAt: account.UpdatedAt,
	})
}
