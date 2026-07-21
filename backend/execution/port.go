package execution

import (
	"context"
	"errors"

	"go-stock/backend/broker"
	"go-stock/backend/models"
)

// ErrCancelNotImplemented Phase2-A：Cancel 仅预留，不实现。
var ErrCancelNotImplemented = errors.New("execution: cancel not implemented")

// SubmitIntent 执行域报单意图（由 TradePlanItem + 运行时价量组装）。
type SubmitIntent struct {
	AccountID   uint
	StockCode   string
	StockName   string
	Side        string
	Price       float64
	Volume      int64
	Reason      string
	StrategyTag string
	AutoFill    bool
}

// ExecutionPort Paper/Real 统一执行端口（Phase2-A 只定义 + Paper 实现，不切流）。
type ExecutionPort interface {
	Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error)
	QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error)
	Cancel(ctx context.Context, orderID string) error
}

// ExecutePlanItemOpts 编排层运行时参数（行情价/数量等，不改 TradePlan 生成逻辑）。
type ExecutePlanItemOpts struct {
	AccountID   uint
	StockName   string
	Price       float64
	Volume      int64
	Reason      string
	StrategyTag string
	AutoFill    bool
}

// ExecutionService 编排：TradePlanItem → PreTradeCheck → ExecutionPort → 订单结果。
type ExecutionService struct {
	port       ExecutionPort
	loadSnap   preTradeSnapshotLoader // 可测注入；nil 用默认 DB 快照
}

func NewExecutionService(port ExecutionPort) *ExecutionService {
	return &ExecutionService{port: port}
}

// withSnapshotLoader 测试用。
func (s *ExecutionService) withSnapshotLoader(load preTradeSnapshotLoader) *ExecutionService {
	if s == nil {
		return nil
	}
	s.loadSnap = load
	return s
}

// ExecutePlanItem 将计划项转为 SubmitIntent，经 PreTradeCheck 后调用 Port。
func (s *ExecutionService) ExecutePlanItem(ctx context.Context, item models.TradePlanItem, opts ExecutePlanItemOpts) (*broker.TradeOrder, error) {
	if s == nil || s.port == nil {
		return nil, errors.New("execution: nil ExecutionService or port")
	}
	side := item.Side
	if side == "" {
		side = "buy"
	}
	name := opts.StockName
	if name == "" {
		name = item.StockName
	}
	reason := opts.Reason
	if reason == "" {
		reason = item.Reason
	}
	tag := opts.StrategyTag
	if tag == "" {
		tag = item.StrategyName
	}
	price := opts.Price
	if price <= 0 {
		price = item.LimitPrice
	}
	vol := opts.Volume
	if vol <= 0 {
		vol = item.TargetVolume
	}
	intent := SubmitIntent{
		AccountID:   opts.AccountID,
		StockCode:   item.StockCode,
		StockName:   name,
		Side:        side,
		Price:       price,
		Volume:      vol,
		Reason:      reason,
		StrategyTag: tag,
		AutoFill:    opts.AutoFill,
	}
	if err := PreTradeCheck(intent, s.loadSnap); err != nil {
		return nil, err
	}
	return s.port.Submit(ctx, intent)
}

// Cancel 委托 ExecutionPort.Cancel（Paper：pending→cancelled）。
func (s *ExecutionService) Cancel(ctx context.Context, orderID string) error {
	if s == nil || s.port == nil {
		return errors.New("execution: nil ExecutionService or port")
	}
	return s.port.Cancel(ctx, orderID)
}
