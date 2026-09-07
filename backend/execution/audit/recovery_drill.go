package audit

import (
	"fmt"
	"strings"

	"go-stock/backend/models"

	"gorm.io/gorm"
)

// Phase6.5.8.4.2 Recovery Drill Harness.
//
// Acceptance-only orchestration around PersistentCheckpointStore + Replayer.
// It never writes Order / Fill / Position / TradePlan / Frozen Spec tables and
// never auto-repairs production business state.

// Drill divergence kinds (report-only; no repair actions).
const (
	DivergenceAuditWindowGap       = "audit_window_gap"
	DivergenceRuntimeMismatch      = "runtime_mismatch"
	DivergenceCheckpointCorrupt    = "checkpoint_corrupt"
	DivergenceScopeCrossTalk       = "scope_cross_talk"
	DivergenceReplayNonIdempotent  = "replay_non_idempotent"
)

// RuntimeSnapshot is a read-only capture of Order / Fill / Position facts used
// for drill comparison. Callers may populate it from RealBroker queries or from
// a drill-local mirror; the harness never mutates the source system.
type RuntimeSnapshot struct {
	ClientOrderID  string
	LocalOrderID   string
	AccountID      string
	StockCode      string
	Side           string
	Price          float64 // Frozen Spec projection
	Volume         int64   // Frozen Spec projection
	Status         string
	BrokerStatus   string
	FilledVolume   int64
	FilledPrice    float64
	LeavesQuantity int64
	FillQty        int64
	FillCount      int
	PositionDelta  int64
	Terminal       bool
	RejectCode     string
	RejectReason   string
}

// DrillReport is the immutable evidence produced by one drill step or scenario.
type DrillReport struct {
	Scope             string       `json:"scope"`
	Checkpoint        Checkpoint   `json:"checkpoint"`
	FullReplay        ReplayResult `json:"fullReplay"`
	IncrementalReplay ReplayResult `json:"incrementalReplay"`
	Runtime           RuntimeSnapshot `json:"runtime"`
	Divergences       []Divergence `json:"divergences"`
	Notes             []string     `json:"notes,omitempty"`
}

// HasDivergence reports whether the drill found any inconsistency.
func (r DrillReport) HasDivergence() bool { return len(r.Divergences) > 0 }

// RecoveryDrillHarness isolates recovery acceptance flows from trading paths.
type RecoveryDrillHarness struct {
	db         *gorm.DB
	store      *PersistentCheckpointStore
	sink       *DBAuditSink
	mirror     map[string]RuntimeSnapshot // client_order_id → simulated runtime
	seenEvent  map[string]struct{}
	seenExecID map[string]struct{}
}

// NewRecoveryDrillHarness prepares an isolated drill environment.
// database must already have audit_events + audit_recovery_checkpoints migrated.
func NewRecoveryDrillHarness(database *gorm.DB) *RecoveryDrillHarness {
	return &RecoveryDrillHarness{
		db:         database,
		store:      NewPersistentCheckpointStore(database),
		sink:       NewDBAuditSink(database),
		mirror:     make(map[string]RuntimeSnapshot),
		seenEvent:  make(map[string]struct{}),
		seenExecID: make(map[string]struct{}),
	}
}

// Store exposes the persistent checkpoint store (read/verify/corrupt-in-test).
func (h *RecoveryDrillHarness) Store() *PersistentCheckpointStore {
	if h == nil {
		return nil
	}
	return h.store
}

// DB returns the drill database (test isolation only).
func (h *RecoveryDrillHarness) DB() *gorm.DB {
	if h == nil {
		return nil
	}
	return h.db
}

// AppendAuditEvents writes L0 events through DBAuditSink (append-only).
func (h *RecoveryDrillHarness) AppendAuditEvents(events []Event) error {
	if h == nil || h.sink == nil {
		return fmt.Errorf("recovery drill: harness not initialized")
	}
	for _, ev := range events {
		if err := h.sink.Write(ev); err != nil {
			return err
		}
		h.applyMirror(ev)
	}
	return nil
}

// SetRuntimeSnapshot overrides the drill-local runtime mirror for a client order.
// Used when an external test captured RealBroker state; never writes business tables.
func (h *RecoveryDrillHarness) SetRuntimeSnapshot(snap RuntimeSnapshot) {
	if h == nil {
		return
	}
	if h.mirror == nil {
		h.mirror = make(map[string]RuntimeSnapshot)
	}
	key := strings.TrimSpace(snap.ClientOrderID)
	if key == "" {
		key = strings.TrimSpace(snap.LocalOrderID)
	}
	h.mirror[key] = snap
}

// RuntimeSnapshot returns the drill-local mirror for a client order id.
func (h *RecoveryDrillHarness) RuntimeSnapshot(clientOrderID string) (RuntimeSnapshot, bool) {
	if h == nil || h.mirror == nil {
		return RuntimeSnapshot{}, false
	}
	snap, ok := h.mirror[strings.TrimSpace(clientOrderID)]
	return snap, ok
}

// CreateCheckpoint runs a scoped Replay and persists the checkpoint+snapshot.
func (h *RecoveryDrillHarness) CreateCheckpoint(scope ReplayScope) (ReplayResult, error) {
	if h == nil || h.db == nil {
		return ReplayResult{}, fmt.Errorf("recovery drill: harness not initialized")
	}
	replayer := NewReplayer(h.db).WithCheckpointStore(h.store)
	return replayer.Replay(scope)
}

// LoadCheckpointVerified loads and verifies a persisted checkpoint for the scope.
func (h *RecoveryDrillHarness) LoadCheckpointVerified(scope ReplayScope) (Checkpoint, error) {
	if h == nil || h.store == nil {
		return Checkpoint{}, ErrCheckpointMissing
	}
	return h.store.LoadVerified(scope.key())
}

// CorruptCheckpoint tampers snapshot_hash in the drill DB only (acceptance fault injection).
func (h *RecoveryDrillHarness) CorruptCheckpoint(scope ReplayScope) error {
	if h == nil || h.db == nil {
		return fmt.Errorf("recovery drill: harness not initialized")
	}
	res := h.db.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", scope.key()).
		Update("snapshot_hash", "drill-corrupt-deadbeef")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCheckpointMissing
	}
	return nil
}

// DeleteAuditWindow removes audit_events rows in (afterID, throughID] for a client order.
// Drill-only fault injection to simulate a missing audit window; never used in production paths.
func (h *RecoveryDrillHarness) DeleteAuditWindow(clientOrderID string, afterID, throughID uint) (int64, error) {
	if h == nil || h.db == nil {
		return 0, fmt.Errorf("recovery drill: harness not initialized")
	}
	query := h.db.Where("client_order_id = ? AND id > ?", strings.TrimSpace(clientOrderID), afterID)
	if throughID > 0 {
		query = query.Where("id <= ?", throughID)
	}
	res := query.Delete(&models.AuditEvent{})
	return res.RowsAffected, res.Error
}

// ReplayFull folds all matching audit_events without using a prior checkpoint baseline.
func (h *RecoveryDrillHarness) ReplayFull(scope ReplayScope) (ReplayResult, error) {
	if h == nil || h.db == nil {
		return ReplayResult{}, fmt.Errorf("recovery drill: harness not initialized")
	}
	// Fresh memory store so this call does not disturb persistent checkpoints.
	return NewReplayer(h.db).Replay(scope)
}

// ReplayIncremental restores from PersistentCheckpointStore then folds the audit tail.
func (h *RecoveryDrillHarness) ReplayIncremental(scope ReplayScope) (ReplayResult, error) {
	if h == nil || h.db == nil {
		return ReplayResult{}, fmt.Errorf("recovery drill: harness not initialized")
	}
	return NewReplayer(h.db).WithCheckpointStore(h.store).ReplayIncremental(scope)
}

// CheckAuditWindow reports divergence when the contiguous id window after a checkpoint
// is missing rows relative to the expected high watermark.
//
// Expected continuity: every id in (afterID, highWatermark] that matches the scope
// filter should exist. When rows were deleted / never written, missing ids are reported.
func (h *RecoveryDrillHarness) CheckAuditWindow(scope ReplayScope, afterID, highWatermark uint) []Divergence {
	var out []Divergence
	if h == nil || h.db == nil {
		out = append(out, Divergence{
			Kind:   DivergenceAuditWindowGap,
			Detail: "harness database unavailable",
		})
		return out
	}
	if highWatermark == 0 || highWatermark <= afterID {
		return out
	}

	query := h.db.Model(&models.AuditEvent{}).
		Select("id").
		Where("id > ? AND id <= ?", afterID, highWatermark).
		Order("id ASC")
	if v := strings.TrimSpace(scope.ClientOrderID); v != "" {
		query = query.Where("client_order_id = ?", v)
	}
	if v := strings.TrimSpace(scope.BrokerRequestID); v != "" {
		query = query.Where("broker_request_id = ?", v)
	}
	if v := strings.TrimSpace(scope.LocalOrderID); v != "" {
		query = query.Where("order_id = ?", v)
	}

	var ids []uint
	if err := query.Find(&ids).Error; err != nil {
		out = append(out, Divergence{
			Kind:   DivergenceAuditWindowGap,
			Detail: fmt.Sprintf("audit window query failed: %v", err),
		})
		return out
	}

	present := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		present[id] = struct{}{}
	}

	// Scope-filtered windows may be sparse across global ids. Detect gaps only among
	// ids that previously existed for this scope by comparing expected count from
	// watermark metadata: if the caller recorded expectedCount, use that; otherwise
	// detect holes between min and max observed ids inside the window.
	if len(ids) == 0 {
		out = append(out, Divergence{
			Kind: DivergenceAuditWindowGap,
			Detail: fmt.Sprintf(
				"audit window empty for scope %s after_id=%d high_watermark=%d",
				scope.key(), afterID, highWatermark,
			),
		})
		return out
	}

	for id := ids[0]; id <= ids[len(ids)-1]; id++ {
		if _, ok := present[id]; ok {
			continue
		}
		var count int64
		if err := h.db.Model(&models.AuditEvent{}).Where("id = ?", id).Count(&count).Error; err != nil {
			out = append(out, Divergence{
				Kind:   DivergenceAuditWindowGap,
				Detail: fmt.Sprintf("count id=%d failed: %v", id, err),
			})
			continue
		}
		if count == 0 {
			out = append(out, Divergence{
				Kind: DivergenceAuditWindowGap,
				Detail: fmt.Sprintf(
					"missing audit_events.id=%d in window (%d, %d] for %s",
					id, afterID, highWatermark, scope.key(),
				),
			})
		}
	}
	return out
}

// CheckAuditWindowExpectedCount reports divergence when the number of rows in
// (afterID, highWatermark] for the scope does not match expectedCount.
func (h *RecoveryDrillHarness) CheckAuditWindowExpectedCount(
	scope ReplayScope, afterID, highWatermark uint, expectedCount int64,
) []Divergence {
	var out []Divergence
	if h == nil || h.db == nil {
		return []Divergence{{Kind: DivergenceAuditWindowGap, Detail: "harness database unavailable"}}
	}
	query := h.db.Model(&models.AuditEvent{}).Where("id > ?", afterID)
	if highWatermark > 0 {
		query = query.Where("id <= ?", highWatermark)
	}
	if v := strings.TrimSpace(scope.ClientOrderID); v != "" {
		query = query.Where("client_order_id = ?", v)
	}
	if v := strings.TrimSpace(scope.BrokerRequestID); v != "" {
		query = query.Where("broker_request_id = ?", v)
	}
	if v := strings.TrimSpace(scope.LocalOrderID); v != "" {
		query = query.Where("order_id = ?", v)
	}
	var actual int64
	if err := query.Count(&actual).Error; err != nil {
		return []Divergence{{Kind: DivergenceAuditWindowGap, Detail: err.Error()}}
	}
	if actual != expectedCount {
		out = append(out, Divergence{
			Kind: DivergenceAuditWindowGap,
			Detail: fmt.Sprintf(
				"audit window row count mismatch for %s: expected=%d actual=%d after_id=%d high_watermark=%d",
				scope.key(), expectedCount, actual, afterID, highWatermark,
			),
		})
	}
	return out
}

// CompareReplayEqual reports divergences when two replay results disagree on
// Order / Fill / Position fold outputs.
func CompareReplayEqual(a, b ReplayResult) []Divergence {
	var out []Divergence
	if a.Order != b.Order {
		out = append(out, Divergence{
			Kind:   DivergenceRuntimeMismatch,
			Detail: fmt.Sprintf("order mismatch: full=%+v incremental=%+v", a.Order, b.Order),
		})
	}
	if a.FillQty != b.FillQty || a.FillCount() != b.FillCount() {
		out = append(out, Divergence{
			Kind: DivergenceRuntimeMismatch,
			Detail: fmt.Sprintf("fill mismatch: full_qty=%d full_count=%d incr_qty=%d incr_count=%d",
				a.FillQty, a.FillCount(), b.FillQty, b.FillCount()),
		})
	}
	if a.PositionDelta != b.PositionDelta {
		out = append(out, Divergence{
			Kind: DivergenceRuntimeMismatch,
			Detail: fmt.Sprintf("position_delta mismatch: full=%d incremental=%d",
				a.PositionDelta, b.PositionDelta),
		})
	}
	if a.SpecHash != b.SpecHash {
		out = append(out, Divergence{
			Kind:   DivergenceSpecHashMismatch,
			Detail: fmt.Sprintf("spec_hash mismatch: full=%q incremental=%q", a.SpecHash, b.SpecHash),
		})
	}
	return out
}

// CompareReplayToRuntime reports divergences between a replay fold and a
// read-only runtime snapshot. Does not modify runtime.
func CompareReplayToRuntime(replayed ReplayResult, runtime RuntimeSnapshot) []Divergence {
	var out []Divergence
	add := func(field string, want, got any) {
		out = append(out, Divergence{
			Kind:   DivergenceRuntimeMismatch,
			Detail: fmt.Sprintf("%s: runtime=%v replay=%v", field, want, got),
		})
	}
	if runtime.StockCode != "" && runtime.StockCode != replayed.Order.StockCode {
		add("stock_code", runtime.StockCode, replayed.Order.StockCode)
	}
	if runtime.Side != "" && runtime.Side != replayed.Order.Side {
		add("side", runtime.Side, replayed.Order.Side)
	}
	if runtime.Price != 0 && absFloat(runtime.Price-replayed.Order.Price) > 1e-9 {
		add("price", runtime.Price, replayed.Order.Price)
	}
	if runtime.Volume != 0 && runtime.Volume != replayed.Order.Volume {
		add("volume", runtime.Volume, replayed.Order.Volume)
	}
	if runtime.Status != "" && runtime.Status != replayed.Order.Status {
		add("status", runtime.Status, replayed.Order.Status)
	}
	if runtime.BrokerStatus != "" && runtime.BrokerStatus != replayed.Order.BrokerStatus {
		add("broker_status", runtime.BrokerStatus, replayed.Order.BrokerStatus)
	}
	if runtime.FilledVolume != replayed.Order.FilledVolume {
		add("filled_volume", runtime.FilledVolume, replayed.Order.FilledVolume)
	}
	if absFloat(runtime.FilledPrice-replayed.Order.FilledPrice) > 1e-9 {
		add("filled_price", runtime.FilledPrice, replayed.Order.FilledPrice)
	}
	if runtime.LeavesQuantity != replayed.Order.LeavesQuantity {
		add("leaves_quantity", runtime.LeavesQuantity, replayed.Order.LeavesQuantity)
	}
	if runtime.FillQty != replayed.FillQty {
		add("fill_qty", runtime.FillQty, replayed.FillQty)
	}
	if runtime.FillCount != replayed.FillCount() {
		add("fill_count", runtime.FillCount, replayed.FillCount())
	}
	if runtime.PositionDelta != replayed.PositionDelta {
		add("position_delta", runtime.PositionDelta, replayed.PositionDelta)
	}
	if runtime.Terminal != replayed.Order.Terminal {
		add("terminal", runtime.Terminal, replayed.Order.Terminal)
	}
	return out
}

// RunCheckpointIncrementalDrill executes the standard drill:
// create checkpoint → append events → incremental replay → compare to full replay
// and optional runtime mirror.
func (h *RecoveryDrillHarness) RunCheckpointIncrementalDrill(
	scope ReplayScope,
	prefix []Event,
	tail []Event,
) (DrillReport, error) {
	report := DrillReport{Scope: scope.key()}
	if err := h.AppendAuditEvents(prefix); err != nil {
		return report, err
	}
	if _, err := h.CreateCheckpoint(scope); err != nil {
		return report, err
	}
	cp, err := h.LoadCheckpointVerified(scope)
	if err != nil {
		return report, err
	}
	report.Checkpoint = cp

	if err := h.AppendAuditEvents(tail); err != nil {
		return report, err
	}

	incremental, err := h.ReplayIncremental(scope)
	if err != nil {
		return report, err
	}
	report.IncrementalReplay = incremental

	full, err := h.ReplayFull(scope)
	if err != nil {
		return report, err
	}
	report.FullReplay = full
	report.Divergences = append(report.Divergences, CompareReplayEqual(full, incremental)...)

	if runtime, ok := h.RuntimeSnapshot(scope.ClientOrderID); ok {
		report.Runtime = runtime
		report.Divergences = append(report.Divergences, CompareReplayToRuntime(incremental, runtime)...)
	}
	return report, nil
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (h *RecoveryDrillHarness) applyMirror(ev Event) {
	if h.mirror == nil {
		h.mirror = make(map[string]RuntimeSnapshot)
	}
	if h.seenEvent == nil {
		h.seenEvent = make(map[string]struct{})
	}
	if h.seenExecID == nil {
		h.seenExecID = make(map[string]struct{})
	}
	if id := strings.TrimSpace(ev.EventID); id != "" {
		if _, dup := h.seenEvent[id]; dup {
			return
		}
		h.seenEvent[id] = struct{}{}
	}

	cid := strings.TrimSpace(ev.ClientOrderID)
	if cid == "" {
		return
	}
	snap := h.mirror[cid]
	snap.ClientOrderID = cid
	if v := strings.TrimSpace(ev.LocalOrderID); v != "" {
		snap.LocalOrderID = v
	}
	if v := strings.TrimSpace(ev.AccountID); v != "" {
		snap.AccountID = v
	}

	switch ev.EventType {
	case TypeSubmitAttempt:
		snap.StockCode = payloadStr(ev.Payload, "symbol")
		snap.Side = payloadStr(ev.Payload, "side")
		snap.Price = payloadF64(ev.Payload, "limit_price")
		snap.Volume = payloadI64(ev.Payload, "target_volume")
		snap.LeavesQuantity = snap.Volume
	case TypeSubmitAccepted, TypeSubmitTimeout:
		snap.Status = payloadStr(ev.Payload, "oms_status")
		snap.BrokerStatus = payloadStr(ev.Payload, "broker_status")
	case TypeSubmitRejected:
		snap.Status = payloadStr(ev.Payload, "oms_status")
		snap.BrokerStatus = payloadStr(ev.Payload, "broker_status")
		snap.RejectCode = payloadStr(ev.Payload, "reject_code")
		snap.RejectReason = payloadStr(ev.Payload, "reject_reason")
		snap.Terminal = true
	case TypeFillApplied:
		execID := strings.TrimSpace(ev.ExecID)
		if execID == "" {
			execID = payloadStr(ev.Payload, "exec_id")
		}
		if execID != "" {
			if _, dup := h.seenExecID[execID]; dup {
				h.mirror[cid] = snap
				return
			}
			h.seenExecID[execID] = struct{}{}
		}
		qty := payloadI64(ev.Payload, "fill_qty")
		snap.FillQty += qty
		snap.FillCount++
		snap.PositionDelta += payloadI64(ev.Payload, "position_delta")
		snap.FilledVolume = payloadI64(ev.Payload, "cum_qty")
		snap.FilledPrice = payloadF64(ev.Payload, "avg_price")
		snap.LeavesQuantity = payloadI64(ev.Payload, "leaves_quantity")
		snap.Status = payloadStr(ev.Payload, "oms_status_after")
		snap.BrokerStatus = payloadStr(ev.Payload, "broker_status_after")
	case TypeOrderTerminal:
		snap.Terminal = true
		snap.Status = payloadStr(ev.Payload, "terminal_status")
		snap.BrokerStatus = payloadStr(ev.Payload, "broker_status")
		snap.FilledVolume = payloadI64(ev.Payload, "filled_volume")
		snap.FilledPrice = payloadF64(ev.Payload, "avg_fill_price")
		snap.LeavesQuantity = payloadI64(ev.Payload, "leaves_quantity")
	}
	h.mirror[cid] = snap
}
