package execution

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/logger"
)

// ErrRealStubHydrateInconsistent hydrate 一致性校验失败（拒绝启动 Real 持久化端口）。
var ErrRealStubHydrateInconsistent = errors.New("execution: real stub hydrate inconsistent")

const hydrateAvgEpsilon = 1e-6

// validateHydrateConsistency 校验订单与 fills 一致性；失败打 ERROR 并返回 ErrRealStubHydrateInconsistent。
func validateHydrateConsistency(orders map[string]*broker.TradeOrder, fillsByOrder map[string][]data.RealStubFill, allFills []data.RealStubFill) error {
	orderIDs := make(map[string]struct{}, len(orders))
	for id, o := range orders {
		if o == nil {
			continue
		}
		orderIDs[id] = struct{}{}
		if err := validateOneHydratedOrder(o, fillsByOrder[id]); err != nil {
			logger.SugaredLogger.Errorf("real stub hydrate inconsistent: %v", err)
			return fmt.Errorf("%w: %v", ErrRealStubHydrateInconsistent, err)
		}
	}
	for _, f := range allFills {
		lid := strings.TrimSpace(f.LocalOrderID)
		if lid == "" {
			err := fmt.Errorf("orphan fill exec_id=%s: empty local_order_id", f.ExecID)
			logger.SugaredLogger.Errorf("real stub hydrate inconsistent: %v", err)
			return fmt.Errorf("%w: %v", ErrRealStubHydrateInconsistent, err)
		}
		if _, ok := orderIDs[lid]; !ok {
			err := fmt.Errorf("orphan fill exec_id=%s local_order_id=%s: order missing", f.ExecID, lid)
			logger.SugaredLogger.Errorf("real stub hydrate inconsistent: %v", err)
			return fmt.Errorf("%w: %v", ErrRealStubHydrateInconsistent, err)
		}
	}
	return nil
}

func validateOneHydratedOrder(o *broker.TradeOrder, fills []data.RealStubFill) error {
	if o == nil {
		return fmt.Errorf("nil order")
	}
	if strings.TrimSpace(o.ClientOrderID) == "" {
		return fmt.Errorf("order %s: empty client_order_id", o.ID)
	}

	var sumQty int64
	var sumNotional float64
	for _, f := range fills {
		sumQty += f.FillQty
		sumNotional += float64(f.FillQty) * f.FillPrice
	}
	if o.FilledVolume != sumQty {
		return fmt.Errorf("order %s: filled_volume=%d != sum(fill_qty)=%d", o.ID, o.FilledVolume, sumQty)
	}
	if sumQty > 0 {
		wantAvg := sumNotional / float64(sumQty)
		if math.Abs(o.FilledPrice-wantAvg) > hydrateAvgEpsilon {
			return fmt.Errorf("order %s: filled_price=%.8f != avg=%.8f", o.ID, o.FilledPrice, wantAvg)
		}
	} else if o.FilledPrice != 0 {
		return fmt.Errorf("order %s: filled_price=%.8f but no fills", o.ID, o.FilledPrice)
	}

	switch o.Status {
	case data.PaperOrderStatusFilled:
		if o.LeavesQuantity != 0 {
			return fmt.Errorf("order %s: filled but leaves=%d", o.ID, o.LeavesQuantity)
		}
		if o.FilledVolume != o.Volume {
			return fmt.Errorf("order %s: filled but filled_volume=%d != volume=%d", o.ID, o.FilledVolume, o.Volume)
		}
		if o.BrokerStatus != data.BrokerStatusFilled {
			return fmt.Errorf("order %s: OMS=filled but broker_status=%s", o.ID, o.BrokerStatus)
		}
	case data.PaperOrderStatusCancelled:
		if o.LeavesQuantity != 0 {
			return fmt.Errorf("order %s: cancelled but leaves=%d", o.ID, o.LeavesQuantity)
		}
		if o.FilledVolume < 0 || o.FilledVolume > o.Volume {
			return fmt.Errorf("order %s: cancelled filled_volume=%d out of range", o.ID, o.FilledVolume)
		}
		if o.BrokerStatus != data.BrokerStatusCancelled {
			return fmt.Errorf("order %s: OMS=cancelled but broker_status=%s", o.ID, o.BrokerStatus)
		}
	case data.PaperOrderStatusRejected:
		if o.FilledVolume != 0 || len(fills) != 0 {
			return fmt.Errorf("order %s: rejected but has fills", o.ID)
		}
		if o.BrokerStatus != data.BrokerStatusRejected {
			return fmt.Errorf("order %s: OMS=rejected but broker_status=%s", o.ID, o.BrokerStatus)
		}
	case data.PaperOrderStatusPending:
		wantLeaves := o.Volume - o.FilledVolume
		if wantLeaves < 0 {
			return fmt.Errorf("order %s: filled_volume=%d > volume=%d", o.ID, o.FilledVolume, o.Volume)
		}
		if o.LeavesQuantity != wantLeaves {
			return fmt.Errorf("order %s: leaves=%d != volume-filled=%d", o.ID, o.LeavesQuantity, wantLeaves)
		}
		switch o.BrokerStatus {
		case data.BrokerStatusPending, data.BrokerStatusAccepted, data.BrokerStatusWorking,
			data.BrokerStatusPartiallyFilled, data.BrokerStatusCancelPending,
			data.BrokerStatusTimeout, data.BrokerStatusUnknown:
			// ok（timeout/unknown/cancel_pending 仅 L2；OMS 仍 pending）
		default:
			return fmt.Errorf("order %s: OMS=pending but broker_status=%s", o.ID, o.BrokerStatus)
		}
		// 部分成交细态：cancel_pending 优先保留（撤单请求在途仍可能有成交）。
		if o.FilledVolume > 0 && o.FilledVolume < o.Volume &&
			o.BrokerStatus != data.BrokerStatusPartiallyFilled &&
			o.BrokerStatus != data.BrokerStatusCancelPending {
			return fmt.Errorf("order %s: partial fill but broker_status=%s", o.ID, o.BrokerStatus)
		}
	default:
		return fmt.Errorf("order %s: unknown OMS status %q", o.ID, o.Status)
	}
	return nil
}
