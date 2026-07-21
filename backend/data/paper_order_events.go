package data

import (
	"encoding/json"
	"time"

	"go-stock/backend/db"

	"gorm.io/gorm"
)

// paperOrderEventPayload 审计 payload（最小观测字段集）。
type paperOrderEventPayload struct {
	OrderID          uint    `json:"orderId"`
	ClientOrderID    string  `json:"clientOrderId"`
	ExecBackend      string  `json:"execBackend"`
	BrokerOrderID    string  `json:"brokerOrderId"`
	ExternalOrderID  string  `json:"externalOrderId"`
	Status           string  `json:"status"`
	Side             string  `json:"side"`
	Symbol           string  `json:"symbol"`
	Price            float64 `json:"price"`
	Volume           int64   `json:"volume"`
	RejectCode       string  `json:"rejectCode"`
	RejectReason     string  `json:"rejectReason"`
	FillAttemptCount int     `json:"fillAttemptCount"`
}

func buildPaperOrderEventPayload(order *PaperOrder) string {
	if order == nil {
		return "{}"
	}
	p := paperOrderEventPayload{
		OrderID:          order.ID,
		ClientOrderID:    order.ClientOrderID,
		ExecBackend:      order.ExecBackend,
		BrokerOrderID:    order.BrokerOrderID,
		ExternalOrderID:  order.ExternalOrderID,
		Status:           order.Status,
		Side:             order.Side,
		Symbol:           order.StockCode,
		Price:            order.Price,
		Volume:           order.Volume,
		RejectCode:       order.RejectCode,
		RejectReason:     order.RejectReason,
		FillAttemptCount: order.FillAttemptCount,
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func writePaperOrderEvent(tx *gorm.DB, order *PaperOrder, eventType, code, message string, at time.Time) error {
	if tx == nil || order == nil || order.ID == 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	if len(message) > 500 {
		message = message[:500]
	}
	ev := PaperOrderEvent{
		OrderID:     order.ID,
		EventType:   eventType,
		Code:        code,
		Message:     message,
		PayloadJSON: buildPaperOrderEventPayload(order),
		CreatedAt:   at,
	}
	return tx.Create(&ev).Error
}

// recordPaperOrderSubmitted Submit 落库成功后写入 order_submitted。
func recordPaperOrderSubmitted(order *PaperOrder) error {
	if order == nil || order.ID == 0 {
		return nil
	}
	return writePaperOrderEvent(db.Dao, order, PaperOrderEventSubmitted, "", "order submitted", time.Now())
}

// recordPaperOrderFilled AutoFill 成交成功后写入 order_filled（Fill 事务已提交之后）。
func recordPaperOrderFilled(order *PaperOrder) error {
	if order == nil || order.ID == 0 {
		return nil
	}
	return writePaperOrderEvent(db.Dao, order, PaperOrderEventFilled, "", "order filled", time.Now())
}
