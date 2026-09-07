package data

import (
	"time"

	"go-stock/backend/db"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Phase2-C PR1：订单身份与 OMS 状态语义冻结（仅常量/注释；不改 Submit/Fill/Cancel）。
//
// OMS status（PaperOrder.Status / TradeOrder.Status）—— Paper 仅允许：
//   pending | filled | rejected | cancelled
// 禁止将券商细态 accepted / working / partially_filled 写入 OMS status。
//
// 身份字段约定：
//   client_order_id  —— 本地幂等键；Paper 格式 paper_<32hex>
//   exec_backend     —— paper | real_xxx
//   broker_order_id  —— 券商委托号；Paper 必须为空（不伪造）
//   external_order_id—— 外部通道单号；Paper 必须为空
//   broker_status    —— 通道/券商细态；Paper 固定为 paper
//
// FilledVol / FilledPrice 语义约定：
//   Paper：全成时写入；FilledVol=成交量，FilledPrice=成交价
//   Real ：累计语义 —— FilledVol=cum qty，FilledPrice=avg fill price（多 Fill 时由后续 PR 推进）
// ExecutionReport（Real）：LastQty/LastPrice=本笔；CumQty/AvgPrice=累计（券商回报字段）
// ---------------------------------------------------------------------------

const (
	PaperOrderStatusPending   = "pending"
	PaperOrderStatusFilled    = "filled"
	PaperOrderStatusRejected  = "rejected"
	PaperOrderStatusCancelled = "cancelled"

	// PaperOrderStatusProcessing 内部事务 CAS 用，禁止作为业务终态对外语义。
	PaperOrderStatusProcessing = "processing"
)

// PaperOrder 执行模式。
const (
	PaperOrderExecModeIOCAutofill = "ioc_autofill" // 提交后立即撮合，失败应 rejected
	PaperOrderExecModeResting     = "resting"      // 挂单等待（本期不实现连续撮合）
)

// 执行后端与 ClientOrderID 格式（身份冻结）。
const (
	// PaperExecBackendPaper 本地纸面执行后端标识（未来可扩展 real_xxx）。
	PaperExecBackendPaper = "paper"
	// ExecBackendRealStub Phase2-C PR2 骨架后端（未接真券商；仅验证 Port 可替换）。
	ExecBackendRealStub = "real_stub"

	// PaperClientOrderIDPrefix Paper 本地 ClientOrderID 固定前缀。
	PaperClientOrderIDPrefix = "paper_"
	// PaperClientOrderIDHexLen 前缀后十六进制长度（16 字节随机 → 32 hex）。
	PaperClientOrderIDHexLen = 32
	// RealClientOrderIDPrefix Real 骨架 ClientOrderID 前缀（与 paper_ 区分）。
	RealClientOrderIDPrefix = "real_"
	// RealStubBrokerOrderIDPrefix ACK 时生成的模拟券商委托号前缀。
	RealStubBrokerOrderIDPrefix = "real_stub_"
)

// broker_status 取值：Paper 仅用 paper；RealStub 生命周期细态（不写入 OMS status）。
const (
	// PaperBrokerStatusPaper 纸面通道占位（非券商回报；不伪造 broker/external id）。
	PaperBrokerStatusPaper = "paper"

	BrokerStatusPending         = "pending" // Submit 后、ACK 前（同步 accepted 后常用 accepted）
	BrokerStatusAccepted        = "accepted"
	BrokerStatusWorking         = "working"
	BrokerStatusPartiallyFilled = "partially_filled"
	BrokerStatusFilled          = "filled"
	// BrokerStatusCancelPending 撤单请求已送达通道、尚未收到 CANCEL 回报；OMS 仍 pending（非终态）。
	BrokerStatusCancelPending = "cancel_pending"
	BrokerStatusCancelled     = "cancelled"
	BrokerStatusRejected      = "rejected"
	// BrokerStatusTimeout / BrokerStatusUnknown：同步 Submit 不确定结果（仅 L2，不进 OMS enum）。
	BrokerStatusTimeout = "timeout"
	BrokerStatusUnknown = "unknown"
)

// PaperOMSStatuses Paper 允许的 OMS 业务态（不含 accepted / processing）。
var PaperOMSStatuses = []string{
	PaperOrderStatusPending,
	PaperOrderStatusFilled,
	PaperOrderStatusRejected,
	PaperOrderStatusCancelled,
}

// IsPaperOMSStatus 是否为冻结的 Paper OMS status。
func IsPaperOMSStatus(status string) bool {
	switch status {
	case PaperOrderStatusPending, PaperOrderStatusFilled, PaperOrderStatusRejected, PaperOrderStatusCancelled:
		return true
	default:
		return false
	}
}

// IsPaperClientOrderIDFormat 校验 Paper ClientOrderID：paper_ + 32 位小写 hex。
func IsPaperClientOrderIDFormat(id string) bool {
	if len(id) != len(PaperClientOrderIDPrefix)+PaperClientOrderIDHexLen {
		return false
	}
	if id[:len(PaperClientOrderIDPrefix)] != PaperClientOrderIDPrefix {
		return false
	}
	for i := len(PaperClientOrderIDPrefix); i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// PaperOrder 拒单码（与风控 ReasonCode 语义对齐，供观测）。
const (
	PaperOrderRejectCashInsufficient     = "CASH_INSUFFICIENT"
	PaperOrderRejectPositionInsufficient = "POSITION_INSUFFICIENT"
	PaperOrderRejectInvalidOrder         = "INVALID_ORDER"
	PaperOrderRejectInternal             = "INTERNAL"

	// Phase6.5.7.3.2.1 scenario reject reasons (stored in reject_code; no migration).
	PaperSimRejectReasonPrice     = "price"
	PaperSimRejectReasonLiquidity = "liquidity"
	PaperSimRejectReasonBroker    = "broker"
)

// PaperOrderEventType 订单生命周期事件类型。
const (
	PaperOrderEventSubmitted = "order_submitted"
	PaperOrderEventRejected  = "order_rejected"
	PaperOrderEventCancelled = "order_cancelled"
	PaperOrderEventFilled    = "order_filled"
	PaperOrderEventAttempt   = "fill_attempt"
)

// PaperOrderEvent 订单生命周期观测事件。
type PaperOrderEvent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrderID     uint      `gorm:"index:idx_paper_order_event_order_time,priority:1;index;not null" json:"orderId"`
	EventType   string    `gorm:"size:32;index;not null" json:"eventType"`
	Code        string    `gorm:"size:64" json:"code"`
	Message     string    `gorm:"size:500" json:"message"`
	PayloadJSON string    `gorm:"type:text" json:"payloadJson"`
	CreatedAt   time.Time `gorm:"index:idx_paper_order_event_order_time,priority:2" json:"createdAt"`
}

func (PaperOrderEvent) TableName() string { return "paper_order_events" }

// markPaperOrderRejectedAfterAutofillFail Fill 事务已回滚后，独立事务将 pending→rejected。
// rejected 状态与审计提交后，再向 EventHub 发布 OrderRejected。
func markPaperOrderRejectedAfterAutofillFail(orderID uint, fillErr error) error {
	if db.Dao == nil || orderID == 0 || fillErr == nil {
		return nil
	}
	code := classifyFillRejectCode(fillErr)
	if code == "" {
		code = PaperOrderRejectInternal
	}
	reason := fillErr.Error()
	if len(reason) > 500 {
		reason = reason[:500]
	}
	now := time.Now()
	var rejected *PaperOrder
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&PaperOrder{}).
			Where("id = ? AND status = ?", orderID, PaperOrderStatusPending).
			Updates(map[string]any{
				"status":             PaperOrderStatusRejected,
				"reject_code":        code,
				"reject_reason":      reason,
				"fill_attempt_count": gorm.Expr("fill_attempt_count + 1"),
				"updated_at":         now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 已被成交/他途改写则不强改
			return nil
		}
		var order PaperOrder
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if err := writePaperOrderEvent(tx, &order, PaperOrderEventRejected, code, reason, now); err != nil {
			return err
		}
		rejected = &order
		return nil
	})
	if err != nil {
		return err
	}
	if rejected != nil {
		paperTradingEvents.OrderRejected(toTradeOrder(*rejected))
	}
	return nil
}
