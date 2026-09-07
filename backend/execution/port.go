package execution

import (
	"context"
	"errors"
	"fmt"

	"go-stock/backend/broker"
	"go-stock/backend/execution/safetygate"
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
	// SpecHash optional Frozen Spec fingerprint for broker submit audit (no schema write).
	// When empty, RealBroker derives a hash from symbol|side|price|volume.
	SpecHash string
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
	// Plan enables Safety Gate freeze check (TradePlan open-buy path).
	Plan *models.TradePlan
	// SkipSafetyFrozen skips freeze check (Manual/Research / ad-hoc tests).
	// Spec integrity + no runtime override still apply.
	SkipSafetyFrozen bool
}

// ExecutionService 编排：TradePlanItem → SafetyGate → PreTradeCheck → ExecutionPort。
type ExecutionService struct {
	port     ExecutionPort
	loadSnap preTradeSnapshotLoader // 可测注入；nil 用默认 DB 快照
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

// ExecutePlanItem 将计划项转为 SubmitIntent，经 Safety Gate + PreTradeCheck 后调用 Port。
// Gate 失败不调用 Broker；不修改 Frozen Spec。
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

	// Proposed submit values: opts when set, else Spec.
	// If Spec is valid and opts diverge, Gate blocks before Port.Submit.
	gatePrice := item.LimitPrice
	gateVol := item.TargetVolume
	if opts.Price > 0 {
		gatePrice = opts.Price
	}
	if opts.Volume > 0 {
		gateVol = opts.Volume
	}

	spec := safetygate.SpecFromItem(item)
	skipFrozen := opts.SkipSafetyFrozen || opts.Plan == nil
	gate := safetygate.ValidateBrokerSubmit(spec, safetygate.Context{
		Plan:            opts.Plan,
		SubmitPrice:     gatePrice,
		SubmitVolume:    gateVol,
		SkipFrozenCheck: skipFrozen,
	})
	if !gate.Allowed {
		return nil, fmt.Errorf("%w", gate.Error())
	}

	// Intent uses Frozen Spec when present (no silent runtime override).
	price := item.LimitPrice
	if price <= 0 {
		price = opts.Price
	}
	vol := item.TargetVolume
	if vol <= 0 {
		vol = opts.Volume
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
