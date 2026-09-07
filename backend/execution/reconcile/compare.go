package reconcile

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func (o Options) withDefaults() Options {
	if o.CancelPendingTimeout <= 0 {
		o.CancelPendingTimeout = DefaultCancelPendingTimeout
	}
	if o.UnknownTimeout <= 0 {
		o.UnknownTimeout = DefaultUnknownTimeout
	}
	if o.AvgPriceEpsilon <= 0 {
		o.AvgPriceEpsilon = DefaultAvgPriceEpsilon
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	return o
}

// Compare is a pure function: LocalOrderSnapshot ⊕ BrokerSnapshot → ReconcileResult.
// It only reports differences; it never mutates inputs or runtime state.
func Compare(local LocalOrderSnapshot, broker BrokerSnapshot, opt Options) ReconcileResult {
	opt = opt.withDefaults()
	var divs []Divergence

	if broker.Missing {
		divs = append(divs, Divergence{
			Kind:        DivMissingOrder,
			Detail:      "broker query returned no order",
			LocalValue:  local.OrderID,
			BrokerValue: "",
		})
	} else {
		localBS := normalizeStatus(local.BrokerStatus)
		brokerBS := normalizeStatus(broker.BrokerStatus)
		if brokerBS == "" {
			brokerBS = normalizeStatus(broker.Status)
		}
		if localBS != brokerBS {
			divs = append(divs, Divergence{
				Kind:        DivStatusMismatch,
				Detail:      fmt.Sprintf("local_broker_status=%s broker_broker_status=%s", localBS, brokerBS),
				LocalValue:  localBS,
				BrokerValue: brokerBS,
			})
		}
		if local.FilledQty != broker.FilledQty {
			divs = append(divs, Divergence{
				Kind:        DivFilledQtyMismatch,
				Detail:      fmt.Sprintf("local_filled_qty=%d broker_filled_qty=%d", local.FilledQty, broker.FilledQty),
				LocalValue:  fmt.Sprintf("%d", local.FilledQty),
				BrokerValue: fmt.Sprintf("%d", broker.FilledQty),
			})
		}
		if local.FilledQty > 0 && broker.FilledQty > 0 &&
			math.Abs(local.AvgPrice-broker.AvgPrice) > opt.AvgPriceEpsilon {
			divs = append(divs, Divergence{
				Kind:        DivAvgPriceMismatch,
				Detail:      fmt.Sprintf("local_avg_price=%.8f broker_avg_price=%.8f", local.AvgPrice, broker.AvgPrice),
				LocalValue:  fmt.Sprintf("%.8f", local.AvgPrice),
				BrokerValue: fmt.Sprintf("%.8f", broker.AvgPrice),
			})
		}
	}

	age := ageSince(local.UpdatedAt, opt.Now)
	localBS := normalizeStatus(local.BrokerStatus)
	switch localBS {
	case "cancel_pending":
		if age >= opt.CancelPendingTimeout {
			divs = append(divs, Divergence{
				Kind:       DivCancelPendingTimeout,
				Detail:     fmt.Sprintf("age=%s threshold=%s", age, opt.CancelPendingTimeout),
				LocalValue: localBS,
			})
		}
	case "timeout", "unknown":
		if age >= opt.UnknownTimeout {
			divs = append(divs, Divergence{
				Kind:       DivUnknownTimeout,
				Detail:     fmt.Sprintf("local_broker_status=%s age=%s threshold=%s", localBS, age, opt.UnknownTimeout),
				LocalValue: localBS,
			})
		}
	}

	return classify(divs)
}

func classify(divs []Divergence) ReconcileResult {
	if len(divs) == 0 {
		return ReconcileResult{
			Consistent:     true,
			Classification: ClassConsistent,
			Divergences:    nil,
		}
	}
	hasStale := false
	hasMissing := false
	for _, d := range divs {
		switch d.Kind {
		case DivCancelPendingTimeout, DivUnknownTimeout:
			hasStale = true
		case DivMissingOrder:
			hasMissing = true
		}
	}
	class := ClassDivergent
	switch {
	case hasStale:
		class = ClassStale
	case hasMissing:
		class = ClassUnverifiable
	}
	return ReconcileResult{
		Consistent:     false,
		Classification: class,
		Divergences:    divs,
	}
}

func normalizeStatus(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func ageSince(updatedAt, now time.Time) time.Duration {
	if updatedAt.IsZero() || now.IsZero() {
		return 0
	}
	age := now.Sub(updatedAt)
	if age < 0 {
		return 0
	}
	return age
}

// NormalizeChannelStatus maps a raw BrokerAdapter.OrderStatus.Status into an
// observation broker_status vocabulary. It does not write OMS state.
func NormalizeChannelStatus(channelStatus string) string {
	switch normalizeStatus(channelStatus) {
	case "pending":
		return "pending"
	case "accepted":
		return "accepted"
	case "working":
		return "working"
	case "partially_filled", "partial":
		return "partially_filled"
	case "filled":
		return "filled"
	case "cancelled", "canceled":
		return "cancelled"
	case "rejected":
		return "rejected"
	case "timeout":
		return "timeout"
	case "unknown", "":
		return "unknown"
	default:
		return "unknown"
	}
}
