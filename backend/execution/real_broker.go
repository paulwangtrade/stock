package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
)

// RealBroker Phase2–4：Real OMS + 持久化；券商通道经 BrokerAdapter（默认 StubAdapter）。
// 不写 paper_orders / paper_fills；不发 EventHub；不调 FillPaperOrder。
type RealBroker struct {
	mu           sync.Mutex
	seq          uint64
	orders       map[string]*broker.TradeOrder  // local id → order
	byClient     map[string]string              // client_order_id → id
	byBroker     map[string]string              // broker_order_id → id
	fillsByOrder map[string][]data.RealStubFill // local_order_id → fills
	seenReports  map[string]struct{}
	seenExecIDs  map[string]struct{}
	audits       map[string]BrokerSubmitAudit // client_order_id → last submit audit
	store        realStubStore
	metrics      realBrokerMetricStore
	adapter      broker.BrokerAdapter
	accountant   PositionAccountant // optional; TRADE fill → position accounting（无则跳过）
	auditEmitter *audit.Emitter     // optional; L0 audit events（memory/log only）
}

// SetPositionAccountant 注入持仓会计端口（可选；nil 表示不做会计）。
func (b *RealBroker) SetPositionAccountant(acc PositionAccountant) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.accountant = acc
}

// SetAuditEmitter 注入 L0 Audit Emitter（可选；nil 表示不发审计事件）。
func (b *RealBroker) SetAuditEmitter(em *audit.Emitter) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.auditEmitter = em
}

func (b *RealBroker) emitAuditLocked(ev audit.Event) {
	// Audit is side-car: never panic, never alter Submit/Cancel return path.
	defer func() { _ = recover() }()
	if b == nil || b.auditEmitter == nil {
		return
	}
	_, _ = b.auditEmitter.Emit(ev)
}

func (b *RealBroker) specHashLocked(clientOrderID string, o *broker.TradeOrder) string {
	if a, ok := b.audits[strings.TrimSpace(clientOrderID)]; ok {
		if h := strings.TrimSpace(a.SpecHash); h != "" {
			return h
		}
	}
	if o != nil {
		return audit.SpecFingerprint(o.StockCode, o.Side, o.Price, o.Volume)
	}
	return ""
}

func newRealBrokerWith(adapter broker.BrokerAdapter, store realStubStore) *RealBroker {
	if adapter == nil {
		adapter = broker.NewConnectedStubAdapter()
	}
	return &RealBroker{
		orders:       make(map[string]*broker.TradeOrder),
		byClient:     make(map[string]string),
		byBroker:     make(map[string]string),
		fillsByOrder: make(map[string][]data.RealStubFill),
		seenReports:  make(map[string]struct{}),
		seenExecIDs:  make(map[string]struct{}),
		audits:       make(map[string]BrokerSubmitAudit),
		store:        store,
		adapter:      adapter,
	}
}

func NewRealBroker() *RealBroker {
	return newRealBrokerWith(broker.NewConnectedStubAdapter(), nil)
}

// NewRealBrokerWithAdapter 注入自定义 BrokerAdapter（测试 / 未来真通道）。
func NewRealBrokerWithAdapter(adapter broker.BrokerAdapter) *RealBroker {
	return newRealBrokerWith(adapter, nil)
}

func NewRealBrokerPersistent() (*RealBroker, error) {
	b := newRealBrokerWith(broker.NewConnectedStubAdapter(), newGormRealStubStore())
	if err := b.hydrateLocked(); err != nil {
		return nil, err
	}
	// Side-car Submit audit (memory+log+db). Missing table / write errors degrade only.
	b.SetAuditEmitter(audit.NewPersistenceEmitter(nil, db.Dao))
	return b, nil
}

// Adapter 返回当前券商适配器（只读）。
func (b *RealBroker) Adapter() broker.BrokerAdapter {
	if b == nil {
		return nil
	}
	return b.adapter
}

func (b *RealBroker) hydrateLocked() error {
	if b.store == nil {
		return nil
	}
	rows, err := b.store.ListOrders()
	if err != nil {
		return err
	}
	var maxSeq uint64
	for i := range rows {
		ord := rows[i]
		ptr := new(broker.TradeOrder)
		*ptr = ord
		if ptr.LeavesQuantity == 0 && ptr.Volume > 0 && ptr.Status == data.PaperOrderStatusPending {
			ptr.LeavesQuantity = ptr.Volume - ptr.FilledVolume
			if ptr.LeavesQuantity < 0 {
				ptr.LeavesQuantity = 0
			}
		}
		b.indexOrderLocked(ptr)
		b.restoreAdapterLocked(ptr)
		if id, err := strconv.ParseUint(ptr.ID, 10, 64); err == nil && id > maxSeq {
			maxSeq = id
		}
	}
	b.seq = maxSeq

	fills, err := b.store.ListFills()
	if err != nil {
		return err
	}
	b.fillsByOrder = make(map[string][]data.RealStubFill)
	b.seenExecIDs = make(map[string]struct{})
	for _, f := range fills {
		lid := strings.TrimSpace(f.LocalOrderID)
		b.fillsByOrder[lid] = append(b.fillsByOrder[lid], f)
		if eid := strings.TrimSpace(f.ExecID); eid != "" {
			b.seenExecIDs[eid] = struct{}{}
		}
	}

	ids, err := b.store.ListReportIDs()
	if err != nil {
		return err
	}
	for _, id := range ids {
		b.seenReports[id] = struct{}{}
	}

	if err := validateHydrateConsistency(b.orders, b.fillsByOrder, fills); err != nil {
		return err
	}
	return nil
}

func (b *RealBroker) restoreAdapterLocked(o *broker.TradeOrder) {
	if b == nil || o == nil {
		return
	}
	stub, ok := b.adapter.(*broker.StubAdapter)
	if !ok || stub == nil {
		return
	}
	status := o.BrokerStatus
	if status == "" {
		status = o.Status
	}
	// Stub 通道词汇：同步 L2（accepted/timeout/unknown）在 ACK 前等价 pending；
	// cancel_pending 未确认，通道侧仍视为在场委托。
	switch status {
	case data.BrokerStatusAccepted, data.BrokerStatusTimeout, data.BrokerStatusUnknown,
		data.BrokerStatusCancelPending:
		status = broker.StubStatusPending
	}
	stub.Restore(broker.SubmitRequest{
		ClientOrderID: o.ClientOrderID,
		AccountID:     o.AccountID,
		StockCode:     o.StockCode,
		StockName:     o.StockName,
		Side:          o.Side,
		Price:         o.Price,
		Volume:        o.Volume,
	}, status, o.BrokerOrderID, o.FilledVolume, o.LeavesQuantity, o.FilledPrice)
}

// ListFills 按 local / client / broker order id 查询成交列表（内存，hydrate 或 TRADE 后可用）。
func (b *RealBroker) ListFills(ctx context.Context, orderKey string) ([]data.RealStubFill, error) {
	_ = ctx
	if b == nil {
		return nil, fmt.Errorf("execution: nil RealBroker")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	o, err := b.lookupLocked(orderKey)
	if err != nil {
		return nil, err
	}
	src := b.fillsByOrder[o.ID]
	out := make([]data.RealStubFill, len(src))
	copy(out, src)
	return out, nil
}

func (b *RealBroker) appendFillLocked(fill *data.RealStubFill) {
	if b == nil || fill == nil {
		return
	}
	lid := strings.TrimSpace(fill.LocalOrderID)
	if lid == "" {
		return
	}
	b.fillsByOrder[lid] = append(b.fillsByOrder[lid], *fill)
}

func (b *RealBroker) indexOrderLocked(o *broker.TradeOrder) {
	if o == nil {
		return
	}
	b.orders[o.ID] = o
	if o.ClientOrderID != "" {
		b.byClient[o.ClientOrderID] = o.ID
	}
	if o.BrokerOrderID != "" {
		b.byBroker[o.BrokerOrderID] = o.ID
	}
}

func (b *RealBroker) hasReportLocked(reportID string) bool {
	if reportID == "" {
		return false
	}
	_, ok := b.seenReports[reportID]
	return ok
}

func (b *RealBroker) recordReportLocked(reportID, execID, typ, clientID, localID string, payload any) error {
	if reportID == "" {
		return nil
	}
	b.seenReports[reportID] = struct{}{}
	if b.store == nil {
		return nil
	}
	return b.store.RecordReport(reportID, execID, typ, clientID, localID, payload)
}

func newRealClientOrderID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%s%d", data.RealClientOrderIDPrefix, time.Now().UnixNano())
	}
	return data.RealClientOrderIDPrefix + hex.EncodeToString(buf[:])
}

// newRealStubBrokerOrderID 委托 Stub 生成模拟券商委托号。
func newRealStubBrokerOrderID(clientOrderID string) string {
	return broker.NewStubBrokerOrderID(clientOrderID)
}

func (b *RealBroker) persistLocked(o *broker.TradeOrder) error {
	if b.store == nil || o == nil {
		return nil
	}
	return b.store.UpsertOrder(o)
}

// Submit：OMS 建单 → BrokerAdapter.Submit → 消费 SubmitResponse（accepted/rejected/timeout/unknown）。
// Adapter error 不删除已建订单；按 outcome 落 OMS/broker_status。不改价量、不写 TradePlan/Intent/Spec。
func (b *RealBroker) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	if b == nil {
		return nil, fmt.Errorf("execution: nil RealBroker")
	}
	if b.adapter == nil {
		return nil, fmt.Errorf("execution: nil BrokerAdapter")
	}
	symbol := strings.TrimSpace(intent.StockCode)
	if symbol == "" || intent.Price <= 0 || intent.Volume <= 0 {
		return nil, fmt.Errorf("execution: invalid real submit intent")
	}
	side := strings.ToLower(strings.TrimSpace(intent.Side))
	if side == "" {
		side = "buy"
	}
	if side != "buy" && side != "sell" {
		return nil, fmt.Errorf("execution: side must be buy or sell")
	}

	now := time.Now()
	specHash := deriveIntentSpecHash(intent)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := strconv.FormatUint(b.seq, 10)
	accID := ""
	if intent.AccountID != 0 {
		accID = strconv.FormatUint(uint64(intent.AccountID), 10)
	}
	clientID := newRealClientOrderID()
	order := &broker.TradeOrder{
		ID:             id,
		AccountID:      accID,
		StockCode:      symbol,
		Symbol:         symbol,
		StockName:      intent.StockName,
		Side:           side,
		Status:         data.PaperOrderStatusPending,
		Price:          intent.Price,
		Volume:         intent.Volume,
		FilledPrice:    0,
		FilledVolume:   0,
		LeavesQuantity: intent.Volume,
		Reason:         intent.Reason,
		StrategyTag:    intent.StrategyTag,
		ClientOrderID:  clientID,
		ExecBackend:    data.ExecBackendRealStub,
		BrokerOrderID:  "",
		BrokerStatus:   data.BrokerStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = intent.AutoFill

	b.indexOrderLocked(order)
	if err := b.persistLocked(order); err != nil {
		delete(b.orders, id)
		delete(b.byClient, order.ClientOrderID)
		return nil, err
	}

	submitAt := time.Now()
	// Submit 前：SUBMIT_ATTEMPT（Spec 投影价量；不改 Frozen Spec）。
	b.emitAuditLocked(audit.BuildSubmitAttempt(
		data.ExecBackendRealStub, clientID, clientID, id, accID, specHash,
		symbol, side, intent.Price, intent.Volume, submitAt,
	))

	req := &broker.SubmitRequest{
		ClientOrderID: clientID,
		AccountID:     accID,
		StockCode:     symbol,
		StockName:     intent.StockName,
		Side:          side,
		Price:         intent.Price,
		Volume:        intent.Volume,
	}
	// 释放锁外调用适配器会有竞态；适配器自身有锁，此处在 OMS 锁内调用 stub（快速内存路径）。
	resp, adapterErr := b.adapter.Submit(ctx, req)
	responseAt := time.Now()

	outcome, mapErr := applySubmitOutcome(order, resp, adapterErr)
	if err := b.persistLocked(order); err != nil {
		return nil, err
	}

	submitAudit := BrokerSubmitAudit{
		BrokerRequestID: clientID,
		SubmitTime:      submitAt,
		ResponseTime:    responseAt,
		SpecHash:        specHash,
		Outcome:         outcome,
		LocalOrderID:    id,
		ClientOrderID:   clientID,
		OMSStatus:       order.Status,
		BrokerStatus:    order.BrokerStatus,
	}
	b.audits[clientID] = submitAudit

	// Broker Response → Submit 结果事件（timeout 类型承载 timeout|unknown outcome）。
	resultType := audit.TypeSubmitAccepted
	switch outcome {
	case SubmitOutcomeRejected:
		resultType = audit.TypeSubmitRejected
	case SubmitOutcomeTimeout, SubmitOutcomeUnknown:
		resultType = audit.TypeSubmitTimeout
	}
	b.emitAuditLocked(audit.BuildSubmitResult(
		resultType, data.ExecBackendRealStub, clientID, clientID, id, accID, specHash,
		outcome, order.Status, order.BrokerStatus, submitAt, responseAt,
		order.RejectCode, order.RejectReason,
		submitResponsePayload(resp, adapterErr),
	))
	if outcome == SubmitOutcomeRejected {
		b.emitAuditLocked(audit.BuildOrderTerminal(
			data.ExecBackendRealStub, clientID, clientID, id, accID, specHash,
			data.PaperOrderStatusRejected, order.BrokerStatus,
			order.FilledVolume, order.LeavesQuantity, order.FilledPrice,
			audit.IDSubmitResult(clientID, outcome), responseAt,
			map[string]any{"reject_code": order.RejectCode, "reject_reason": order.RejectReason},
		))
	}

	cp := *order
	switch outcome {
	case SubmitOutcomeAccepted:
		b.metrics.recordSubmitPending()
		return &cp, nil
	case SubmitOutcomeRejected:
		b.metrics.recordRejected()
		return &cp, mapErr
	default:
		// timeout / unknown：OMS 仍 pending，计入 submit pending 观测。
		b.metrics.recordSubmitPending()
		return &cp, mapErr
	}
}

// LastSubmitAudit 返回最近一次 Submit 审计快照（按 client_order_id；无 migration）。
func (b *RealBroker) LastSubmitAudit(clientOrderID string) (BrokerSubmitAudit, bool) {
	if b == nil {
		return BrokerSubmitAudit{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.audits[strings.TrimSpace(clientOrderID)]
	return a, ok
}

// GetMetrics 返回 RealBroker 基础监控快照（atomic 累计，并发安全）。
func (b *RealBroker) GetMetrics() RealBrokerMetrics {
	if b == nil {
		return RealBrokerMetrics{}
	}
	return b.metrics.snapshot()
}

func (b *RealBroker) orderByClientLocked(clientOrderID string) (*broker.TradeOrder, error) {
	id, ok := b.byClient[clientOrderID]
	if !ok {
		return nil, ErrReportOrderNotFound
	}
	o, ok := b.orders[id]
	if !ok || o == nil {
		return nil, ErrReportOrderNotFound
	}
	return o, nil
}

func (b *RealBroker) lookupLocked(key string) (*broker.TradeOrder, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("execution: empty order key")
	}
	if o, ok := b.orders[key]; ok && o != nil {
		return o, nil
	}
	if id, ok := b.byClient[key]; ok {
		if o, ok := b.orders[id]; ok && o != nil {
			return o, nil
		}
	}
	if id, ok := b.byBroker[key]; ok {
		if o, ok := b.orders[id]; ok && o != nil {
			return o, nil
		}
	}
	return nil, fmt.Errorf("execution: real order %q not found", key)
}

// QueryOrder 支持 local id / client_order_id / broker_order_id。
func (b *RealBroker) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	if b == nil {
		return nil, fmt.Errorf("execution: nil RealBroker")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	o, err := b.lookupLocked(orderID)
	if err != nil {
		return nil, err
	}
	cp := *o
	return &cp, nil
}

// realStubCancellable 撤单「请求」资格：OMS pending 且通道侧仍在场。
// cancel_pending 已在撤单途中，重复请求不再受理（ErrOrderNotCancellable）。
func realStubCancellable(o *broker.TradeOrder) bool {
	if o == nil || o.Status != data.PaperOrderStatusPending {
		return false
	}
	switch o.BrokerStatus {
	case data.BrokerStatusPending, data.BrokerStatusAccepted, data.BrokerStatusWorking,
		data.BrokerStatusPartiallyFilled, data.BrokerStatusTimeout, data.BrokerStatusUnknown, "":
		return true
	default:
		return false
	}
}

// realStubCancelReportApplicable CANCEL 回报可施加的前置态：可撤 + 已在 cancel_pending。
func realStubCancelReportApplicable(o *broker.TradeOrder) bool {
	if o == nil {
		return false
	}
	if o.Status == data.PaperOrderStatusPending && o.BrokerStatus == data.BrokerStatusCancelPending {
		return true
	}
	return realStubCancellable(o)
}

// applyRealStubCancelRequest 撤单请求：OMS 保持 pending，仅置通道细态 cancel_pending。
// 不动 LeavesQuantity —— 撤单未确认前剩余量仍可能成交。
func applyRealStubCancelRequest(o *broker.TradeOrder, at time.Time) {
	o.BrokerStatus = data.BrokerStatusCancelPending
	o.UpdatedAt = at
}

func applyRealStubCancel(o *broker.TradeOrder, at time.Time) {
	o.Status = data.PaperOrderStatusCancelled
	o.BrokerStatus = data.BrokerStatusCancelled
	o.LeavesQuantity = 0 // 剩余撤销；保留 FilledVolume
	o.UpdatedAt = at
}

// Cancel：通知 BrokerAdapter 后仅登记撤单请求（broker_status=cancel_pending）。
// OMS 终态由 CANCEL 回报施加；请求本身不是事实终态，故不发 ORDER_TERMINAL。不改 Paper Cancel。
func (b *RealBroker) Cancel(ctx context.Context, orderID string) error {
	if b == nil {
		return fmt.Errorf("execution: nil RealBroker")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	o, err := b.lookupLocked(orderID)
	if err != nil || !realStubCancellable(o) {
		return data.ErrOrderNotCancellable
	}
	if b.adapter != nil {
		if err := b.adapter.Cancel(ctx, o.ClientOrderID); err != nil {
			return err
		}
	}
	applyRealStubCancelRequest(o, time.Now())
	return b.persistLocked(o)
}

// submitResponsePayload builds payload.response for SUBMIT_* result events (audit only).
func submitResponsePayload(resp *broker.SubmitResponse, adapterErr error) map[string]any {
	out := map[string]any{}
	if resp != nil {
		out["status"] = resp.Status
		out["message"] = resp.Message
		out["broker_order_id"] = resp.BrokerOrderID
		out["client_order_id"] = resp.ClientOrderID
	}
	if adapterErr != nil {
		out["adapter_error"] = adapterErr.Error()
	}
	return out
}

var _ ExecutionPort = (*RealBroker)(nil)
