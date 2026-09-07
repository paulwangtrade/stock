package execution

import (
	"context"
	"errors"
	"fmt"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"
)

// Phase6.5.7.3.1 Paper Fill Engine MVP.
// Frozen Spec → Paper Order → full Fill (optional slippage) → Position.
// Does NOT write TradePlan Spec / Intent; does NOT touch Real Broker / Strategy / Approve / Freeze.

// Paper logical lifecycle (maps to OMS pending|filled; Created is pre-persist).
const (
	PaperLogicCreated   = "Created"
	PaperLogicSubmitted = "Submitted"
	PaperLogicFilled    = "Filled"
)

// PaperFillModel is the runtime fill result (no migration / no new DB columns).
// Slippage is derived or scenario-injected; never written back to limit_price.
type PaperFillModel struct {
	FillPrice  float64 `json:"fillPrice"`
	FillQty    int64   `json:"fillQty"`
	Slippage   float64 `json:"slippage"`   // buy: fill-limit; sell: limit-fill
	Commission float64 `json:"commission"` // maps to PaperFill.Fee / Order.Fee
}

// PaperFillOpts configures MVP full-fill simulation.
type PaperFillOpts struct {
	AccountID uint
	// Slippage is absolute price units (adverse when >0):
	//   buy:  fill_price = limit_price + Slippage
	//   sell: fill_price = limit_price - Slippage
	Slippage float64
	Reason   string
}

// PaperFillResult is the closed-loop outcome of one Spec-driven full fill.
type PaperFillResult struct {
	LogicStatus string // Created → Submitted → Filled (final)
	OMSStatus   string // pending | filled (compat)
	Order       *broker.TradeOrder
	Fill        PaperFillModel
	LimitPrice  float64 // Spec snapshot (read-only)
	TargetVol   int64   // Spec snapshot (read-only)
	Side        string
}

// paperFillBroker is the minimal Port+Fill surface used by the engine.
type paperFillBroker interface {
	Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error)
	Fill(ctx context.Context, orderID string, fillPrice float64) error
	QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error)
}

// PaperFillEngine runs Spec → Order → Fill → (accounting via PaperBroker).
type PaperFillEngine struct {
	broker paperFillBroker
}

func NewPaperFillEngine(b paperFillBroker) *PaperFillEngine {
	if b == nil {
		b = NewPaperBroker(nil)
	}
	return &PaperFillEngine{broker: b}
}

// ComputeFillPrice applies configurable absolute slippage to Frozen limit_price.
func ComputeFillPrice(side string, limitPrice, slippage float64) (float64, error) {
	if limitPrice <= 0 {
		return 0, errors.New("paper fill: invalid limit_price")
	}
	if slippage < 0 {
		return 0, errors.New("paper fill: slippage must be >= 0")
	}
	side = normalizePaperSide(side)
	switch side {
	case "buy":
		return limitPrice + slippage, nil
	case "sell":
		p := limitPrice - slippage
		if p <= 0 {
			return 0, fmt.Errorf("paper fill: fill_price <= 0 after slippage (limit=%.4f slip=%.4f)", limitPrice, slippage)
		}
		return p, nil
	default:
		return 0, fmt.Errorf("paper fill: invalid side %q", side)
	}
}

// ComputeSlippage returns signed adverse slippage vs Spec limit.
func ComputeSlippage(side string, limitPrice, fillPrice float64) float64 {
	side = normalizePaperSide(side)
	if side == "sell" {
		return limitPrice - fillPrice
	}
	return fillPrice - limitPrice
}

func normalizePaperSide(side string) string {
	if side == "" {
		return "buy"
	}
	return side
}

// MapPaperLogicToOMS keeps OMS four-state compatibility (Created has no row yet).
func MapPaperLogicToOMS(logic string) string {
	switch logic {
	case PaperLogicSubmitted:
		return data.PaperOrderStatusPending
	case PaperLogicFilled:
		return data.PaperOrderStatusFilled
	default:
		return ""
	}
}

// ExecuteFullFillFromSpec consumes Frozen Spec, submits a Paper order at limit_price,
// then fully fills at limit±slippage. Never mutates the TradePlanItem Spec fields.
func (e *PaperFillEngine) ExecuteFullFillFromSpec(ctx context.Context, item models.TradePlanItem, opts PaperFillOpts) (*PaperFillResult, error) {
	if e == nil || e.broker == nil {
		return nil, errors.New("paper fill: nil engine")
	}
	limit := item.LimitPrice
	vol := item.TargetVolume
	side := normalizePaperSide(item.Side)
	if limit <= 0 {
		return nil, errors.New("paper fill: frozen limit_price missing or invalid")
	}
	if vol < 100 {
		return nil, fmt.Errorf("paper fill: frozen target_volume < 100 (vol=%d)", vol)
	}

	fillPrice, err := ComputeFillPrice(side, limit, opts.Slippage)
	if err != nil {
		return nil, err
	}
	fillQty := vol // MVP: full fill only

	out := &PaperFillResult{
		LogicStatus: PaperLogicCreated,
		OMSStatus:   MapPaperLogicToOMS(PaperLogicCreated),
		LimitPrice:  limit,
		TargetVol:   vol,
		Side:        side,
		Fill: PaperFillModel{
			FillPrice: fillPrice,
			FillQty:   fillQty,
			Slippage:  ComputeSlippage(side, limit, fillPrice),
		},
	}

	intent := SubmitIntent{
		AccountID:   opts.AccountID,
		StockCode:   item.StockCode,
		StockName:   item.StockName,
		Side:        side,
		Price:       limit, // order price = Frozen Spec; fill may differ
		Volume:      vol,
		Reason:      opts.Reason,
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    false, // explicit Fill step so slippage can apply
	}
	order, serr := e.broker.Submit(ctx, intent)
	if serr != nil {
		return out, serr
	}
	if order == nil {
		return out, errors.New("paper fill: nil order after submit")
	}
	out.Order = order
	out.LogicStatus = PaperLogicSubmitted
	out.OMSStatus = MapPaperLogicToOMS(PaperLogicSubmitted)

	if ferr := e.broker.Fill(ctx, order.ID, fillPrice); ferr != nil {
		return out, ferr
	}
	filled, qerr := e.broker.QueryOrder(ctx, order.ID)
	if qerr != nil {
		return out, qerr
	}
	out.Order = filled
	out.LogicStatus = PaperLogicFilled
	out.OMSStatus = MapPaperLogicToOMS(PaperLogicFilled)
	out.Fill.Commission = filled.Fee
	if filled.FilledPrice > 0 {
		out.Fill.FillPrice = filled.FilledPrice
	}
	if filled.FilledVolume > 0 {
		out.Fill.FillQty = filled.FilledVolume
	}
	out.Fill.Slippage = ComputeSlippage(side, limit, out.Fill.FillPrice)

	// Intentionally does NOT call UpdateItemExecution / touch TradePlan Spec.
	return out, nil
}
