package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution/audit"
)

// Submit outcome（同步四态；不进 OMS enum）。
const (
	SubmitOutcomeAccepted = "accepted"
	SubmitOutcomeRejected = "rejected"
	SubmitOutcomeTimeout  = "timeout"
	SubmitOutcomeUnknown  = "unknown"
)

var (
	// ErrBrokerRejected 通道明确拒绝（订单已保留，OMS=rejected）。
	ErrBrokerRejected = errors.New("execution: broker rejected")
	// ErrBrokerTimeout 同步确认超时（订单已保留，OMS=pending, broker_status=timeout）。
	ErrBrokerTimeout = errors.New("execution: broker timeout")
	// ErrBrokerUnknown 发送结果不明（订单已保留，OMS=pending, broker_status=unknown）。
	ErrBrokerUnknown = errors.New("execution: broker unknown")
)

// BrokerSubmitAudit Phase6.5.7.4.4.1：Submit 出站审计（内存，无 migration）。
type BrokerSubmitAudit struct {
	BrokerRequestID string
	SubmitTime      time.Time
	ResponseTime    time.Time
	SpecHash        string
	Outcome         string
	LocalOrderID    string
	ClientOrderID   string
	OMSStatus       string
	BrokerStatus    string
}

func deriveIntentSpecHash(intent SubmitIntent) string {
	if h := strings.TrimSpace(intent.SpecHash); h != "" {
		return h
	}
	return audit.SpecFingerprint(intent.StockCode, intent.Side, intent.Price, intent.Volume)
}

func mapSubmitResponseStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SubmitOutcomeAccepted, "pending", "":
		// Stub 现网返回 pending 表示「已受理、待 ACK」→ 同步 accepted。
		return SubmitOutcomeAccepted
	case SubmitOutcomeRejected:
		return SubmitOutcomeRejected
	case SubmitOutcomeTimeout:
		return SubmitOutcomeTimeout
	case SubmitOutcomeUnknown:
		return SubmitOutcomeUnknown
	default:
		return SubmitOutcomeUnknown
	}
}

func classifyAdapterSubmitError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return SubmitOutcomeTimeout
	}
	if errors.Is(err, ErrBrokerTimeout) {
		return SubmitOutcomeTimeout
	}
	if errors.Is(err, ErrBrokerRejected) {
		return SubmitOutcomeRejected
	}
	if errors.Is(err, ErrBrokerUnknown) {
		return SubmitOutcomeUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return SubmitOutcomeTimeout
	case strings.Contains(msg, "reject"):
		return SubmitOutcomeRejected
	default:
		return SubmitOutcomeUnknown
	}
}

// applySubmitOutcome maps adapter response/error → OMS + broker_status.
// Does not mutate TradePlan / Intent / Frozen Spec / price / volume.
func applySubmitOutcome(order *broker.TradeOrder, resp *broker.SubmitResponse, adapterErr error) (outcome string, retErr error) {
	if order == nil {
		return SubmitOutcomeUnknown, fmt.Errorf("execution: nil order for submit outcome")
	}
	at := time.Now()
	order.UpdatedAt = at

	switch {
	case adapterErr != nil:
		outcome = classifyAdapterSubmitError(adapterErr)
	case resp != nil:
		outcome = mapSubmitResponseStatus(resp.Status)
		if outcome == SubmitOutcomeRejected {
			if msg := strings.TrimSpace(resp.Message); msg != "" {
				order.RejectReason = msg
			}
		}
		// accepted 同步路径不写 broker_order_id（等 ACK）；其它态亦不伪造。
	default:
		outcome = SubmitOutcomeUnknown
	}

	switch outcome {
	case SubmitOutcomeAccepted:
		order.Status = data.PaperOrderStatusPending
		order.BrokerStatus = data.BrokerStatusAccepted
		return outcome, nil
	case SubmitOutcomeRejected:
		order.Status = data.PaperOrderStatusRejected
		order.BrokerStatus = data.BrokerStatusRejected
		order.LeavesQuantity = 0
		if order.RejectCode == "" {
			order.RejectCode = data.PaperOrderRejectInternal
		}
		if adapterErr != nil {
			return outcome, fmt.Errorf("%w: %v", ErrBrokerRejected, adapterErr)
		}
		return outcome, ErrBrokerRejected
	case SubmitOutcomeTimeout:
		order.Status = data.PaperOrderStatusPending
		order.BrokerStatus = data.BrokerStatusTimeout
		if adapterErr != nil {
			return outcome, fmt.Errorf("%w: %v", ErrBrokerTimeout, adapterErr)
		}
		return outcome, ErrBrokerTimeout
	default:
		order.Status = data.PaperOrderStatusPending
		order.BrokerStatus = data.BrokerStatusUnknown
		if adapterErr != nil {
			return outcome, fmt.Errorf("%w: %v", ErrBrokerUnknown, adapterErr)
		}
		return outcome, ErrBrokerUnknown
	}
}
