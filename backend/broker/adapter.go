package broker

import (
	"context"
	"sync"
	"time"
)

// OrderRequest 人工确认后的下单请求
type OrderRequest struct {
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	Side      string  `json:"side"` // buy / sell
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	Reason    string  `json:"reason"`
}

// OrderResult 下单结果
type OrderResult struct {
	OK        bool   `json:"ok"`
	BrokerID  string `json:"brokerId"`
	Message   string `json:"message"`
	RawStatus string `json:"rawStatus"`
}

// Position 券商持仓快照
type Position struct {
	StockCode string  `json:"stockCode"`
	Volume    int64   `json:"volume"`
	Sellable  int64   `json:"sellable"`
	AvgCost   float64 `json:"avgCost"`
}

// Adapter 券商适配器接口（Phase4：人工确认后送单，禁止无人值守全自动默认开启）
type Adapter interface {
	Name() string
	PlaceOrder(ctx context.Context, req OrderRequest) (*OrderResult, error)
	CancelOrder(ctx context.Context, brokerID string) error
	QueryPositions(ctx context.Context) ([]Position, error)
}

// ExecutionAdapter 在兼容旧 Adapter 的同时提供统一执行域模型。
type ExecutionAdapter interface {
	Adapter
	SubmitOrder(ctx context.Context, order TradeOrder) (*TradeOrder, error)
	QueryOrders(ctx context.Context, accountID string) ([]TradeOrder, error)
	QueryTradePositions(ctx context.Context, accountID string) ([]TradePosition, error)
	QueryAccount(ctx context.Context, accountID string) (*TradeAccount, error)
}

// ManualConfirmAdapter 默认适配器：不真正下单，只记录「已人工确认待送单」
type ManualConfirmAdapter struct{}

func (m *ManualConfirmAdapter) Name() string { return "manual-confirm" }

func (m *ManualConfirmAdapter) PlaceOrder(ctx context.Context, req OrderRequest) (*OrderResult, error) {
	_ = ctx
	return &OrderResult{
		OK:        true,
		BrokerID:  "MANUAL-" + req.StockCode,
		Message:   "已记录人工确认计划，尚未对接真实券商通道。请在券商客户端自行下单。",
		RawStatus: "manual_pending",
	}, nil
}

func (m *ManualConfirmAdapter) CancelOrder(ctx context.Context, brokerID string) error {
	_ = ctx
	_ = brokerID
	return nil
}

func (m *ManualConfirmAdapter) QueryPositions(ctx context.Context) ([]Position, error) {
	_ = ctx
	return []Position{}, nil
}

func (m *ManualConfirmAdapter) SubmitOrder(ctx context.Context, order TradeOrder) (*TradeOrder, error) {
	result, err := m.PlaceOrder(ctx, OrderRequest{
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

func (m *ManualConfirmAdapter) QueryOrders(ctx context.Context, accountID string) ([]TradeOrder, error) {
	_ = ctx
	_ = accountID
	return []TradeOrder{}, nil
}

func (m *ManualConfirmAdapter) QueryTradePositions(ctx context.Context, accountID string) ([]TradePosition, error) {
	positions, err := m.QueryPositions(ctx)
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

func (m *ManualConfirmAdapter) QueryAccount(ctx context.Context, accountID string) (*TradeAccount, error) {
	_ = ctx
	return &TradeAccount{ID: accountID}, nil
}

var (
	defaultAdapterMu sync.RWMutex
	defaultAdapter   Adapter = &ManualConfirmAdapter{}
)

func Default() Adapter {
	defaultAdapterMu.RLock()
	defer defaultAdapterMu.RUnlock()
	return defaultAdapter
}

// DefaultExecution 返回标准执行接口；旧 Adapter 会被无损包装。
func DefaultExecution() ExecutionAdapter {
	adapter := Default()
	if execution, ok := adapter.(ExecutionAdapter); ok {
		return execution
	}
	return NewLegacyExecutionAdapter(adapter)
}

func SetDefault(a Adapter) {
	if a != nil {
		defaultAdapterMu.Lock()
		defaultAdapter = a
		defaultAdapterMu.Unlock()
	}
}
