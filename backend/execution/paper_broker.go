package execution

import (
	"context"
	"fmt"
	"strconv"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"

	// Wire QuantityPolicy Submit hooks (Validate-only when Flag ON).
	_ "go-stock/backend/tradingrule"
)

// paperTradingAPI PaperBroker 对现有纸面交易 API 的最小依赖（便于单测注入）。
type paperTradingAPI interface {
	SubmitPaperOrder(req data.PaperSubmitOrderReq) (*data.PaperOrder, error)
	FillPaperOrder(orderID uint, fillPrice float64) error
	FillPaperOrderQty(orderID uint, fillPrice float64, fillQty int64) error
	RejectPaperOrderSim(orderID uint, reason, message string) error
	CancelPaperOrder(orderID uint) error
}

// PaperBroker 包装现有 SubmitPaperOrder / FillPaperOrder，不复制撮合与会计逻辑。
// SubmitPaperOrder is a Paper accounting primitive; UI/API/Façade must not call it directly.
type PaperBroker struct {
	api paperTradingAPI
}

func NewPaperBroker(api paperTradingAPI) *PaperBroker {
	if api == nil {
		api = data.NewPaperTradingApi()
	}
	return &PaperBroker{api: api}
}

// Submit 委托 SubmitPaperOrder（含 AutoFill→Fill 既有路径）。
func (b *PaperBroker) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	_ = ctx
	if b == nil || b.api == nil {
		return nil, fmt.Errorf("execution: nil PaperBroker")
	}
	order, err := b.api.SubmitPaperOrder(data.PaperSubmitOrderReq{
		AccountID:   intent.AccountID,
		StockCode:   intent.StockCode,
		StockName:   intent.StockName,
		Side:        intent.Side,
		Price:       intent.Price,
		Volume:      intent.Volume,
		Reason:      intent.Reason,
		StrategyTag: intent.StrategyTag,
		AutoFill:    intent.AutoFill,
	})
	if order == nil {
		return nil, err
	}
	out := mapPaperOrderToTradeOrder(*order)
	return &out, err
}

// Fill 显式包装 FillPaperOrder（手动成交路径；IOC 仍由 Submit AutoFill 覆盖）。
func (b *PaperBroker) Fill(ctx context.Context, orderID string, fillPrice float64) error {
	_ = ctx
	if b == nil || b.api == nil {
		return fmt.Errorf("execution: nil PaperBroker")
	}
	id, err := strconv.ParseUint(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("execution: invalid order id %q: %w", orderID, err)
	}
	return b.api.FillPaperOrder(uint(id), fillPrice)
}

// FillQty wraps FillPaperOrderQty (partial or full remaining).
func (b *PaperBroker) FillQty(ctx context.Context, orderID string, fillPrice float64, fillQty int64) error {
	_ = ctx
	if b == nil || b.api == nil {
		return fmt.Errorf("execution: nil PaperBroker")
	}
	id, err := strconv.ParseUint(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("execution: invalid order id %q: %w", orderID, err)
	}
	return b.api.FillPaperOrderQty(uint(id), fillPrice, fillQty)
}

// RejectSim marks pending zero-fill order rejected (price|liquidity|broker).
func (b *PaperBroker) RejectSim(ctx context.Context, orderID, reason, message string) error {
	_ = ctx
	if b == nil || b.api == nil {
		return fmt.Errorf("execution: nil PaperBroker")
	}
	id, err := strconv.ParseUint(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("execution: invalid order id %q: %w", orderID, err)
	}
	return b.api.RejectPaperOrderSim(uint(id), reason, message)
}

// QueryOrder 按本地纸面订单 ID 查询（只读包装，不改状态机）。
func (b *PaperBroker) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	id, err := strconv.ParseUint(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("execution: invalid order id %q: %w", orderID, err)
	}
	data.EnsurePaperTradingTables()
	var order data.PaperOrder
	if err := db.Dao.First(&order, uint(id)).Error; err != nil {
		return nil, err
	}
	out := mapPaperOrderToTradeOrder(order)
	return &out, nil
}

// Cancel 委托 CancelPaperOrder：仅 pending→cancelled（CAS）；不扩展 EventHub。
func (b *PaperBroker) Cancel(ctx context.Context, orderID string) error {
	_ = ctx
	if b == nil || b.api == nil {
		return fmt.Errorf("execution: nil PaperBroker")
	}
	id, err := strconv.ParseUint(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("execution: invalid order id %q: %w", orderID, err)
	}
	return b.api.CancelPaperOrder(uint(id))
}

func mapPaperOrderToTradeOrder(order data.PaperOrder) broker.TradeOrder {
	tradeOrder := broker.TradeOrder{
		ID:               strconv.FormatUint(uint64(order.ID), 10),
		AccountID:        strconv.FormatUint(uint64(order.AccountID), 10),
		StockCode:        order.StockCode,
		Symbol:           order.StockCode,
		StockName:        order.StockName,
		Side:             order.Side,
		Status:           order.Status,
		Price:            order.Price,
		Volume:           order.Volume,
		FilledPrice:      order.FilledPrice,
		FilledVolume:     order.FilledVol,
		Fee:              order.Fee,
		Reason:           order.Reason,
		StrategyTag:      order.StrategyTag,
		RejectCode:       order.RejectCode,
		RejectReason:     order.RejectReason,
		FillAttemptCount: order.FillAttemptCount,
		ClientOrderID:    order.ClientOrderID,
		ExecBackend:      order.ExecBackend,
		BrokerOrderID:    order.BrokerOrderID,
		ExternalOrderID:  order.ExternalOrderID,
		BrokerStatus:     order.BrokerStatus,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
	}
	if order.FilledAt != nil {
		tradeOrder.FilledAt = *order.FilledAt
	}
	return tradeOrder
}

// 编译期断言：PaperBroker 实现 ExecutionPort。
var _ ExecutionPort = (*PaperBroker)(nil)
