package execution

import (
	"context"
	"errors"
	"fmt"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"
)

// Phase6.5.7.3.2.1 Partial Fill / Reject / Cancel scenario engine (Paper only).
// Extends 3.1 lifecycle with Partial / Rejected / Cancelled. Never writes Spec/Intent/TradePlan.

const (
	PaperLogicPartial   = "Partial"
	PaperLogicRejected  = "Rejected"
	PaperLogicCancelled = "Cancelled"
)

// PaperSimReject reasons (stored as reject_code; no migration).
const (
	RejectReasonPrice     = data.PaperSimRejectReasonPrice
	RejectReasonLiquidity = data.PaperSimRejectReasonLiquidity
	RejectReasonBroker    = data.PaperSimRejectReasonBroker
)

// paperScenarioBroker is PaperBroker surface for scenario simulation.
type paperScenarioBroker interface {
	Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error)
	FillQty(ctx context.Context, orderID string, fillPrice float64, fillQty int64) error
	RejectSim(ctx context.Context, orderID, reason, message string) error
	Cancel(ctx context.Context, orderID string) error
	QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error)
}

// PaperScenarioEngine drives Submitted → Partial/Filled/Rejected/Cancelled on Paper.
type PaperScenarioEngine struct {
	broker paperScenarioBroker
}

func NewPaperScenarioEngine(b paperScenarioBroker) *PaperScenarioEngine {
	if b == nil {
		b = NewPaperBroker(nil)
	}
	return &PaperScenarioEngine{broker: b}
}

// PaperScenarioSnap is a runtime view after each step (does not mutate Spec).
type PaperScenarioSnap struct {
	LogicStatus  string
	OMSStatus    string
	Order        *broker.TradeOrder
	FilledQty    int64
	RemainingQty int64
	LastFill     PaperFillModel
	LimitPrice   float64 // Spec snapshot
	TargetVol    int64   // Spec snapshot
}

func deriveLogicStatus(order *broker.TradeOrder) string {
	if order == nil {
		return PaperLogicCreated
	}
	switch order.Status {
	case data.PaperOrderStatusRejected:
		return PaperLogicRejected
	case data.PaperOrderStatusCancelled:
		return PaperLogicCancelled
	case data.PaperOrderStatusFilled:
		return PaperLogicFilled
	case data.PaperOrderStatusPending:
		if order.FilledVolume > 0 && order.FilledVolume < order.Volume {
			return PaperLogicPartial
		}
		return PaperLogicSubmitted
	default:
		return order.Status
	}
}

func snapFromOrder(order *broker.TradeOrder, limit float64, target int64, last PaperFillModel) *PaperScenarioSnap {
	filled := int64(0)
	vol := target
	if order != nil {
		filled = order.FilledVolume
		vol = order.Volume
	}
	remaining := vol - filled
	if remaining < 0 {
		remaining = 0
	}
	return &PaperScenarioSnap{
		LogicStatus:  deriveLogicStatus(order),
		OMSStatus:    orderStatusOrEmpty(order),
		Order:        order,
		FilledQty:    filled,
		RemainingQty: remaining,
		LastFill:     last,
		LimitPrice:   limit,
		TargetVol:    target,
	}
}

func orderStatusOrEmpty(order *broker.TradeOrder) string {
	if order == nil {
		return ""
	}
	return order.Status
}

// SubmitFromSpec creates a pending Paper order from Frozen Spec (no fill).
func (e *PaperScenarioEngine) SubmitFromSpec(ctx context.Context, item models.TradePlanItem, accountID uint, reason string) (*PaperScenarioSnap, error) {
	if e == nil || e.broker == nil {
		return nil, errors.New("paper scenario: nil engine")
	}
	limit := item.LimitPrice
	vol := item.TargetVolume
	side := normalizePaperSide(item.Side)
	if limit <= 0 {
		return nil, errors.New("paper scenario: frozen limit_price missing or invalid")
	}
	if vol < 100 {
		return nil, fmt.Errorf("paper scenario: frozen target_volume < 100 (vol=%d)", vol)
	}
	order, err := e.broker.Submit(ctx, SubmitIntent{
		AccountID:   accountID,
		StockCode:   item.StockCode,
		StockName:   item.StockName,
		Side:        side,
		Price:       limit,
		Volume:      vol,
		Reason:      reason,
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    false,
	})
	if err != nil {
		return snapFromOrder(order, limit, vol, PaperFillModel{}), err
	}
	return snapFromOrder(order, limit, vol, PaperFillModel{}), nil
}

// PartialFill applies one fill leg. fill_qty < remaining → Partial; remaining→0 → Filled.
// limitSnap/targetSnap are Spec read-only snapshots for the result view (never written back).
func (e *PaperScenarioEngine) PartialFill(ctx context.Context, orderID string, fillPrice float64, fillQty int64, limitSnap float64, targetSnap int64) (*PaperScenarioSnap, error) {
	if e == nil || e.broker == nil {
		return nil, errors.New("paper scenario: nil engine")
	}
	if fillQty <= 0 {
		return nil, errors.New("paper scenario: fill_qty must be > 0")
	}
	before, err := e.broker.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if before.Status == data.PaperOrderStatusFilled {
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), errors.New("paper scenario: filled order cannot be rolled back or re-filled")
	}
	if before.Status == data.PaperOrderStatusCancelled {
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), errors.New("paper scenario: cancelled order cannot be filled")
	}
	if before.Status == data.PaperOrderStatusRejected {
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), errors.New("paper scenario: rejected order cannot be filled")
	}
	remaining := before.Volume - before.FilledVolume
	if fillQty > remaining {
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), fmt.Errorf("paper scenario: fill_qty %d > remaining %d", fillQty, remaining)
	}

	if err := e.broker.FillQty(ctx, orderID, fillPrice, fillQty); err != nil {
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), err
	}
	after, qerr := e.broker.QueryOrder(ctx, orderID)
	if qerr != nil {
		return nil, qerr
	}
	last := PaperFillModel{
		FillPrice:  fillPrice,
		FillQty:    fillQty,
		Slippage:   ComputeSlippage(before.Side, limitSnap, fillPrice),
		Commission: after.Fee - before.Fee,
	}
	if last.Commission < 0 {
		last.Commission = 0
	}
	return snapFromOrder(after, limitSnap, targetSnap, last), nil
}

// RejectSim rejects a zero-fill pending order. Position unchanged.
func (e *PaperScenarioEngine) RejectSim(ctx context.Context, orderID, reason, message string, limitSnap float64, targetSnap int64) (*PaperScenarioSnap, error) {
	if e == nil || e.broker == nil {
		return nil, errors.New("paper scenario: nil engine")
	}
	if err := e.broker.RejectSim(ctx, orderID, reason, message); err != nil {
		before, _ := e.broker.QueryOrder(ctx, orderID)
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), err
	}
	after, err := e.broker.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return snapFromOrder(after, limitSnap, targetSnap, PaperFillModel{}), nil
}

// Cancel cancels a pending order (including partial). Further Fill is rejected.
func (e *PaperScenarioEngine) Cancel(ctx context.Context, orderID string, limitSnap float64, targetSnap int64) (*PaperScenarioSnap, error) {
	if e == nil || e.broker == nil {
		return nil, errors.New("paper scenario: nil engine")
	}
	if err := e.broker.Cancel(ctx, orderID); err != nil {
		before, _ := e.broker.QueryOrder(ctx, orderID)
		return snapFromOrder(before, limitSnap, targetSnap, PaperFillModel{}), err
	}
	after, err := e.broker.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return snapFromOrder(after, limitSnap, targetSnap, PaperFillModel{}), nil
}
