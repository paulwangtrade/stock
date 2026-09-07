package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution/audit"
)

// FakeExecutionReportHandler Phase2-C PR4：假回报处理器（幂等 + PartialFill 累计）。
// 不调 FillPaperOrder、不写 paper_fills、不发 EventHub。
type FakeExecutionReportHandler struct {
	broker *RealBroker
}

func NewFakeExecutionReportHandler(broker *RealBroker) *FakeExecutionReportHandler {
	return &FakeExecutionReportHandler{broker: broker}
}

func (h *FakeExecutionReportHandler) beginReport(reportID string) bool {
	return h.broker.hasReportLocked(reportID)
}

func (h *FakeExecutionReportHandler) commitReport(reportID, execID, typ, clientID, localID string, payload any) error {
	return h.broker.recordReportLocked(reportID, execID, typ, clientID, localID, payload)
}

func (h *FakeExecutionReportHandler) claimExecLocked(execID string) bool {
	if execID == "" {
		return true
	}
	if _, ok := h.broker.seenExecIDs[execID]; ok {
		return false
	}
	if h.broker.store != nil {
		// DB 唯一约束为权威；预占内存避免同进程竞态
		h.broker.seenExecIDs[execID] = struct{}{}
		return true
	}
	h.broker.seenExecIDs[execID] = struct{}{}
	return true
}

func (h *FakeExecutionReportHandler) insertFillLocked(fill *data.RealStubFill) (bool, error) {
	if h.broker.store == nil {
		return true, nil // 纯内存：exec 已由 claimExecLocked 占用
	}
	return h.broker.store.InsertFillIfAbsent(fill)
}

// OnExecutionReport 统一回报入口（ACK/REJECT/TRADE/CANCEL）。
func (h *FakeExecutionReportHandler) OnExecutionReport(ctx context.Context, report ExecutionReport) error {
	switch strings.ToUpper(strings.TrimSpace(report.ReportType)) {
	case ExecReportTypeACK:
		return h.OnBrokerAck(ctx, BrokerAckReport{
			ReportID: report.ReportID, ExecID: report.ExecID,
			ClientOrderID: report.ClientOrderID, BrokerOrderID: report.BrokerOrderID,
			ExternalOrderID: report.ExternalOrderID, OccurredAt: report.OccurredAt,
		})
	case ExecReportTypeREJECT:
		return h.OnRejectReport(ctx, RejectReport{
			ReportID: report.ReportID, ExecID: report.ExecID,
			ClientOrderID: report.ClientOrderID, BrokerOrderID: report.BrokerOrderID,
			RejectCode: report.RejectCode, RejectReason: report.RejectReason,
			OccurredAt: report.OccurredAt,
		})
	case ExecReportTypeTRADE:
		return h.applyTradeReport(ctx, report)
	case ExecReportTypeCANCEL:
		return h.applyCancelReport(ctx, report)
	default:
		return fmt.Errorf("%w: unknown ReportType %q", ErrReportInvalidPayload, report.ReportType)
	}
}

func (h *FakeExecutionReportHandler) OnBrokerAck(ctx context.Context, report BrokerAckReport) error {
	_ = ctx
	if h == nil || h.broker == nil {
		return fmt.Errorf("execution: nil FakeExecutionReportHandler")
	}
	cid := strings.TrimSpace(report.ClientOrderID)
	if cid == "" {
		return ErrReportInvalidPayload
	}
	at := report.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}

	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	o, err := h.broker.orderByClientLocked(cid)
	if err != nil {
		return err
	}

	bid := strings.TrimSpace(report.BrokerOrderID)
	if bid == "" {
		if o.BrokerOrderID != "" {
			bid = o.BrokerOrderID
		} else {
			bid = newRealStubBrokerOrderID(cid)
		}
	}
	reportID := resolveReportID(report.ReportID, data.RealStubReportTypeAck, cid, bid)

	if h.beginReport(reportID) {
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeACK, report.ExecID, "duplicate_report", "noop", at, report, nil)
		return nil
	}

	// 语义幂等：已 ACK 且 broker_order_id 一致
	if o.Status == data.PaperOrderStatusPending &&
		(o.BrokerStatus == data.BrokerStatusWorking || o.BrokerStatus == data.BrokerStatusAccepted) &&
		o.BrokerOrderID == bid {
		_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeAck, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeACK, report.ExecID, "new", "noop", at, report, nil)
		return nil
	}
	if o.Status != data.PaperOrderStatusPending {
		return ErrReportInvalidTransition
	}
	if o.BrokerStatus != data.BrokerStatusPending && o.BrokerStatus != data.BrokerStatusAccepted && o.BrokerStatus != "" {
		// 已部分成交等：ACK 迟到则只补 broker_order_id
		if o.BrokerOrderID == "" {
			o.BrokerOrderID = bid
			h.broker.byBroker[bid] = o.ID
			_ = h.broker.persistLocked(o)
		}
		_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeAck, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeACK, report.ExecID, "new", "applied", at, report, nil)
		return nil
	}

	prevBID := o.BrokerOrderID
	o.BrokerOrderID = bid
	if prevBID != "" && prevBID != bid {
		delete(h.broker.byBroker, prevBID)
	}
	h.broker.byBroker[bid] = o.ID
	if ext := strings.TrimSpace(report.ExternalOrderID); ext != "" {
		o.ExternalOrderID = ext
	}
	// ACK：accepted → working（映射表 accepted/working）
	o.BrokerStatus = data.BrokerStatusWorking
	o.UpdatedAt = at
	if err := h.broker.persistLocked(o); err != nil {
		return err
	}
	if stub, ok := h.broker.adapter.(*broker.StubAdapter); ok && stub != nil {
		stub.BindBrokerOrderID(cid, bid)
	}
	h.broker.metrics.recordAccepted(at.Sub(o.CreatedAt))
	_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeAck, cid, o.ID, report)
	h.emitReportReceivedLocked(o, reportID, ExecReportTypeACK, report.ExecID, "new", "applied", at, report, nil)
	return nil
}

func (h *FakeExecutionReportHandler) OnRejectReport(ctx context.Context, report RejectReport) error {
	_ = ctx
	if h == nil || h.broker == nil {
		return fmt.Errorf("execution: nil FakeExecutionReportHandler")
	}
	cid := strings.TrimSpace(report.ClientOrderID)
	if cid == "" {
		return ErrReportInvalidPayload
	}
	key := strings.TrimSpace(report.BrokerOrderID)
	if key == "" {
		key = report.RejectCode
	}
	reportID := resolveReportID(report.ReportID, data.RealStubReportTypeReject, cid, key)
	at := report.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}

	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	if h.beginReport(reportID) {
		if o, err := h.broker.orderByClientLocked(cid); err == nil {
			h.emitReportReceivedLocked(o, reportID, ExecReportTypeREJECT, report.ExecID, "duplicate_report", "noop", at, report, nil)
		}
		return nil
	}

	o, err := h.broker.orderByClientLocked(cid)
	if err != nil {
		return err
	}
	if o.Status == data.PaperOrderStatusRejected {
		_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeReject, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeREJECT, report.ExecID, "new", "noop", at, report, nil)
		return nil
	}
	if o.Status != data.PaperOrderStatusPending {
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeREJECT, report.ExecID, "new", "rejected", at, report,
			map[string]any{"error": ErrReportInvalidTransition.Error()})
		return ErrReportInvalidTransition
	}
	if bid := strings.TrimSpace(report.BrokerOrderID); bid != "" {
		o.BrokerOrderID = bid
	}
	o.RejectCode = report.RejectCode
	if o.RejectCode == "" {
		o.RejectCode = data.PaperOrderRejectInternal
	}
	o.RejectReason = report.RejectReason
	rejectRemaining := o.FilledVolume > 0
	if rejectRemaining {
		// A broker reject after a partial fill rejects only the remaining
		// quantity. Preserve fills/position and close the remainder as cancelled.
		o.Status = data.PaperOrderStatusCancelled
		o.BrokerStatus = data.BrokerStatusCancelled
		o.LeavesQuantity = 0
	} else {
		o.Status = data.PaperOrderStatusRejected
		o.BrokerStatus = data.BrokerStatusRejected
		o.LeavesQuantity = 0
	}
	o.UpdatedAt = at
	if err := h.broker.persistLocked(o); err != nil {
		return err
	}
	if rejectRemaining {
		h.broker.metrics.recordCancel()
	} else {
		h.broker.metrics.recordRejected()
	}
	_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeReject, cid, o.ID, report)
	h.emitReportReceivedLocked(o, reportID, ExecReportTypeREJECT, report.ExecID, "new", "applied", at, report, nil)
	terminalExtra := map[string]any{
		"reject_code": o.RejectCode, "reject_reason": o.RejectReason,
	}
	if rejectRemaining {
		terminalExtra["reason"] = "reject_remaining"
	}
	h.emitOrderTerminalLocked(o, reportID, at, terminalExtra)
	return nil
}

func (h *FakeExecutionReportHandler) OnFilledReport(ctx context.Context, report FilledReport) error {
	return h.OnExecutionReport(ctx, report.toExecutionReport())
}

// applyCancelReport Cancel 回报：保留已成交，剩余撤单 → OMS cancelled。
func (h *FakeExecutionReportHandler) applyCancelReport(ctx context.Context, report ExecutionReport) error {
	_ = ctx
	if h == nil || h.broker == nil {
		return fmt.Errorf("execution: nil FakeExecutionReportHandler")
	}
	cid := strings.TrimSpace(report.ClientOrderID)
	bidKey := strings.TrimSpace(report.BrokerOrderID)
	at := report.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}

	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	var o *broker.TradeOrder
	var err error
	if cid != "" {
		o, err = h.broker.orderByClientLocked(cid)
	} else if bidKey != "" {
		o, err = h.broker.lookupLocked(bidKey)
	} else {
		return ErrReportInvalidPayload
	}
	if err != nil || o == nil {
		return ErrReportOrderNotFound
	}
	cid = o.ClientOrderID
	reportID := resolveReportID(report.ReportID, data.RealStubReportTypeCancel, cid, bidKey)

	if h.beginReport(reportID) {
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeCANCEL, report.ExecID, "duplicate_report", "noop", at, report, nil)
		return nil
	}
	if o.Status == data.PaperOrderStatusCancelled {
		_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeCancel, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeCANCEL, report.ExecID, "new", "noop", at, report, nil)
		return nil
	}
	if !realStubCancelReportApplicable(o) {
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeCANCEL, report.ExecID, "new", "rejected", at, report,
			map[string]any{"error": ErrReportInvalidTransition.Error()})
		return ErrReportInvalidTransition
	}
	applyRealStubCancel(o, at)
	if h.broker.adapter != nil {
		_ = h.broker.adapter.Cancel(ctx, cid)
	}
	if err := h.broker.persistLocked(o); err != nil {
		return err
	}
	h.broker.metrics.recordCancel()
	_ = h.commitReport(reportID, report.ExecID, data.RealStubReportTypeCancel, cid, o.ID, report)
	h.emitReportReceivedLocked(o, reportID, ExecReportTypeCANCEL, report.ExecID, "new", "applied", at, report, nil)
	h.emitOrderTerminalLocked(o, reportID, at, map[string]any{"reason": "cancel_confirmed"})
	return nil
}

// applyTradeReport PartialFill 累计：本地重算 filled/avg/leaves；OMS 仅在全成时 → filled。
func (h *FakeExecutionReportHandler) applyTradeReport(ctx context.Context, report ExecutionReport) error {
	if h == nil || h.broker == nil {
		return fmt.Errorf("execution: nil FakeExecutionReportHandler")
	}
	cid := strings.TrimSpace(report.ClientOrderID)
	execID := strings.TrimSpace(report.ExecID)
	if execID == "" {
		execID = fmt.Sprintf("synth|%s|%s|%d|%.4f",
			cid, strings.TrimSpace(report.BrokerOrderID), report.LastQty, report.LastPrice)
	}
	if cid == "" || report.LastPrice <= 0 || report.LastQty <= 0 {
		return ErrReportInvalidPayload
	}
	reportID := resolveReportID(report.ReportID, data.RealStubReportTypeTrade, cid, execID)
	at := report.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}

	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	if h.beginReport(reportID) {
		if o, err := h.broker.orderByClientLocked(cid); err == nil {
			h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "duplicate_report", "noop", at, report, nil)
		}
		return nil
	}
	if !h.claimExecLocked(execID) {
		_ = h.commitReport(reportID, execID, data.RealStubReportTypeTrade, cid, "", report)
		if o, err := h.broker.orderByClientLocked(cid); err == nil {
			h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "duplicate_exec", "noop", at, report, nil)
		}
		return nil
	}

	o, err := h.broker.orderByClientLocked(cid)
	if err != nil {
		return err
	}
	if o.Status == data.PaperOrderStatusFilled {
		_ = h.commitReport(reportID, execID, data.RealStubReportTypeTrade, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "new", "noop", at, report, nil)
		return nil
	}
	if o.Status != data.PaperOrderStatusPending {
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "new", "rejected", at, report,
			map[string]any{"error": ErrReportInvalidTransition.Error()})
		return ErrReportInvalidTransition
	}

	leaves := o.LeavesQuantity
	if leaves <= 0 {
		leaves = o.Volume - o.FilledVolume
	}
	if report.LastQty > leaves {
		return fmt.Errorf("%w: trade qty %d > leaves %d", ErrReportInvalidPayload, report.LastQty, leaves)
	}

	prevFilled := o.FilledVolume
	prevAvg := o.FilledPrice
	newFilled := prevFilled + report.LastQty
	var newAvg float64
	if newFilled > 0 {
		newAvg = (float64(prevFilled)*prevAvg + float64(report.LastQty)*report.LastPrice) / float64(newFilled)
	}
	newLeaves := o.Volume - newFilled
	if newLeaves < 0 {
		newLeaves = 0
	}

	fill := &data.RealStubFill{
		ClientOrderID: cid,
		LocalOrderID:  o.ID,
		ExecID:        execID,
		FillQty:       report.LastQty,
		FillPrice:     report.LastPrice,
		CumQty:        newFilled,
		AvgPrice:      newAvg,
		BrokerOrderID: strings.TrimSpace(report.BrokerOrderID),
		ReportID:      reportID,
		CreatedAt:     at,
	}
	inserted, err := h.insertFillLocked(fill)
	if err != nil {
		delete(h.broker.seenExecIDs, execID)
		return err
	}
	if !inserted {
		// DB 已有该 exec_id（重启后内存未命中又撞库）
		_ = h.commitReport(reportID, execID, data.RealStubReportTypeTrade, cid, o.ID, report)
		h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "duplicate_exec", "noop", at, report, nil)
		return nil
	}
	h.broker.appendFillLocked(fill)
	h.broker.metrics.recordFill(o.Side, report.LastQty, report.LastPrice)

	posDelta := report.LastQty
	if strings.ToLower(strings.TrimSpace(o.Side)) == "sell" {
		posDelta = -report.LastQty
	}

	// Position accounting（可选端口）：仅在本笔成交真正入账时调用；不改委托价量/Spec。
	if h.broker.accountant != nil {
		if aerr := h.broker.accountant.ApplyFill(ctx, FillEvent{
			AccountID:     o.AccountID,
			StockCode:     o.StockCode,
			StockName:     o.StockName,
			Side:          o.Side,
			Qty:           report.LastQty,
			Price:         report.LastPrice,
			ClientOrderID: cid,
			LocalOrderID:  o.ID,
			ExecID:        execID,
			OccurredAt:    at,
		}); aerr != nil {
			delete(h.broker.seenExecIDs, execID)
			return aerr
		}
	}

	if bid := strings.TrimSpace(report.BrokerOrderID); bid != "" {
		o.BrokerOrderID = bid
	}
	// Spec 投影 Price/Volume 保持不变；仅更新成交累计。
	o.FilledVolume = newFilled
	o.FilledPrice = newAvg
	o.LeavesQuantity = newLeaves
	o.UpdatedAt = at

	if newLeaves == 0 {
		o.Status = data.PaperOrderStatusFilled
		o.BrokerStatus = data.BrokerStatusFilled
		o.FilledAt = at
	} else {
		o.Status = data.PaperOrderStatusPending
		// 撤单请求在途时保留 cancel_pending：成交不撤销撤单意图，终态仍等 CANCEL 回报。
		if o.BrokerStatus != data.BrokerStatusCancelPending {
			o.BrokerStatus = data.BrokerStatusPartiallyFilled
		}
	}

	if err := h.broker.persistLocked(o); err != nil {
		return err
	}
	_ = h.commitReport(reportID, execID, data.RealStubReportTypeTrade, cid, o.ID, report)

	reportEventID := audit.IDReportReceived(reportID)
	h.emitReportReceivedLocked(o, reportID, ExecReportTypeTRADE, execID, "new", "applied", at, report, map[string]any{
		"last_qty": report.LastQty, "last_price": report.LastPrice,
	})
	h.broker.emitAuditLocked(audit.BuildFillApplied(
		data.ExecBackendRealStub, cid, cid, o.ID, o.AccountID, h.broker.specHashLocked(cid, o),
		reportID, execID, reportEventID,
		report.LastQty, report.LastPrice, newFilled, newAvg, newLeaves, posDelta,
		o.Status, o.BrokerStatus, at,
	))
	if o.Status == data.PaperOrderStatusFilled {
		h.emitOrderTerminalLocked(o, reportID, at, nil)
	}
	return nil
}

func (h *FakeExecutionReportHandler) emitReportReceivedLocked(
	o *broker.TradeOrder,
	reportID, reportType, execID, dedup, apply string,
	at time.Time,
	raw any,
	extra map[string]any,
) {
	if h == nil || h.broker == nil || o == nil {
		return
	}
	h.broker.emitAuditLocked(audit.BuildReportReceived(
		data.ExecBackendRealStub, o.ClientOrderID, o.ClientOrderID, o.ID, o.AccountID,
		h.broker.specHashLocked(o.ClientOrderID, o),
		reportID, reportType, execID, dedup, apply, at, raw, extra,
	))
}

func (h *FakeExecutionReportHandler) emitOrderTerminalLocked(
	o *broker.TradeOrder, causationID string, at time.Time, extra map[string]any,
) {
	if h == nil || h.broker == nil || o == nil {
		return
	}
	h.broker.emitAuditLocked(audit.BuildOrderTerminal(
		data.ExecBackendRealStub, o.ClientOrderID, o.ClientOrderID, o.ID, o.AccountID,
		h.broker.specHashLocked(o.ClientOrderID, o),
		o.Status, o.BrokerStatus, o.FilledVolume, o.LeavesQuantity, o.FilledPrice,
		causationID, at, extra,
	))
}

// applyCumulativeAvg 导出供测试校验公式（与 Handler 一致）。
func applyCumulativeAvg(prevFilled int64, prevAvg float64, lastQty int64, lastPrice float64) (newFilled int64, newAvg float64) {
	newFilled = prevFilled + lastQty
	if newFilled <= 0 {
		return 0, 0
	}
	newAvg = (float64(prevFilled)*prevAvg + float64(lastQty)*lastPrice) / float64(newFilled)
	return newFilled, newAvg
}

var _ ExecutionReportHandler = (*FakeExecutionReportHandler)(nil)

// 编译期确认 TradeOrder 含 LeavesQuantity。
var _ = broker.TradeOrder{}.LeavesQuantity
