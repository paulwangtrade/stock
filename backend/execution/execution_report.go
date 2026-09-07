package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Phase2-C PR4：Real Execution Report 支持 PartialFill 累计。
//
// ReportType：
//   ACK / REJECT / TRADE / CANCEL
// TRADE：可多次；本地重算 filled_volume / filled_price(avg) / leaves_quantity
// OMS：pending（含部分成交）| filled | rejected | cancelled
// broker_status：working | partially_filled | filled | rejected

var (
	ErrReportOrderNotFound     = errors.New("execution: report order not found")
	ErrReportInvalidTransition = errors.New("execution: report invalid transition")
	ErrReportInvalidPayload    = errors.New("execution: report invalid payload")
	ErrReportDuplicate         = errors.New("execution: duplicate report")
)

// ExecutionReport 统一回报信封（冻结字段集）。
type ExecutionReport struct {
	ReportType      string // ACK | REJECT | TRADE | CANCEL
	ReportID        string
	ExecID          string // TRADE 幂等主键之一；写入 RealStubFill.exec_id
	ClientOrderID   string
	BrokerOrderID   string
	ExternalOrderID string
	LastQty         int64   // 本次成交量
	LastPrice       float64 // 本次成交价
	CumQty          int64   // 累计成交量（快照）
	AvgPrice        float64 // 累计均价（快照）
	BrokerStatus    string
	RejectCode      string
	RejectReason    string
	OccurredAt      time.Time
}

// 冻结的回报类型常量。
const (
	ExecReportTypeACK    = "ACK"
	ExecReportTypeREJECT = "REJECT"
	ExecReportTypeTRADE  = "TRADE"
	ExecReportTypeCANCEL = "CANCEL"
)

// BrokerAckReport 兼容旧 ACK API。
type BrokerAckReport struct {
	ReportID        string
	ExecID          string
	ClientOrderID   string
	BrokerOrderID   string
	ExternalOrderID string
	OccurredAt      time.Time
}

// RejectReport 兼容旧拒单 API。
type RejectReport struct {
	ReportID      string
	ExecID        string
	ClientOrderID string
	BrokerOrderID string
	RejectCode    string
	RejectReason  string
	OccurredAt    time.Time
}

// FilledReport 兼容旧「单次全成」API（内部映射为 TRADE）。
type FilledReport struct {
	ReportID      string
	ExecID        string
	ClientOrderID string
	BrokerOrderID string
	FillPrice     float64
	FillQty       int64
	OccurredAt    time.Time
}

// ExecutionReportHandler 回报入站接口。
type ExecutionReportHandler interface {
	OnBrokerAck(ctx context.Context, report BrokerAckReport) error
	OnRejectReport(ctx context.Context, report RejectReport) error
	OnFilledReport(ctx context.Context, report FilledReport) error
	// OnExecutionReport 统一入口（ACK/REJECT/TRADE/CANCEL）。
	OnExecutionReport(ctx context.Context, report ExecutionReport) error
}

func resolveReportID(explicit, kind, clientOrderID, brokerOrExec string) string {
	if id := strings.TrimSpace(explicit); id != "" {
		return id
	}
	return fmt.Sprintf("%s|%s|%s", kind, strings.TrimSpace(clientOrderID), strings.TrimSpace(brokerOrExec))
}

func (r BrokerAckReport) toExecutionReport() ExecutionReport {
	return ExecutionReport{
		ReportType:      ExecReportTypeACK,
		ReportID:        r.ReportID,
		ExecID:          r.ExecID,
		ClientOrderID:   r.ClientOrderID,
		BrokerOrderID:   r.BrokerOrderID,
		ExternalOrderID: r.ExternalOrderID,
		OccurredAt:      r.OccurredAt,
	}
}

func (r RejectReport) toExecutionReport() ExecutionReport {
	return ExecutionReport{
		ReportType:    ExecReportTypeREJECT,
		ReportID:      r.ReportID,
		ExecID:        r.ExecID,
		ClientOrderID: r.ClientOrderID,
		BrokerOrderID: r.BrokerOrderID,
		RejectCode:    r.RejectCode,
		RejectReason:  r.RejectReason,
		OccurredAt:    r.OccurredAt,
	}
}

func (r FilledReport) toExecutionReport() ExecutionReport {
	return ExecutionReport{
		ReportType:    ExecReportTypeTRADE,
		ReportID:      r.ReportID,
		ExecID:        r.ExecID,
		ClientOrderID: r.ClientOrderID,
		BrokerOrderID: r.BrokerOrderID,
		LastQty:       r.FillQty,
		LastPrice:     r.FillPrice,
		CumQty:        r.FillQty,
		AvgPrice:      r.FillPrice,
		OccurredAt:    r.OccurredAt,
	}
}
