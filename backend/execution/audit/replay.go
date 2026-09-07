package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"gorm.io/gorm"

	"go-stock/backend/models"
)

// Phase6.5.8.3 Audit Replay Recovery.
//
// Rebuilds Order runtime state / Fill set / Position delta from the append-only
// audit_events stream. Read-only: it never writes execution tables, never touches
// Frozen Spec, Intent, TradePlan, Freeze or Approve.

// Report types carried in REPORT_RECEIVED payload (mirrors execution constants;
// duplicated here to keep the audit package free of execution imports).
const (
	replayReportACK    = "ACK"
	replayReportREJECT = "REJECT"
	replayReportTRADE  = "TRADE"
	replayReportCANCEL = "CANCEL"
)

// OMS/broker status literals used when an event only implies a transition.
const (
	replayStatusPending = "pending"
	replayBrokerWorking = "working"
)

// Divergence kinds.
const (
	DivergenceSpecHashMismatch = "spec_hash_mismatch"
	DivergenceMissingEventID   = "missing_event_id"
	DivergenceDecodeFailed     = "decode_failed"
	DivergenceMissingExecID    = "missing_exec_id"
)

// Divergence records a replay inconsistency that must be surfaced, never silently fixed.
type Divergence struct {
	Kind      string `json:"kind"`
	EventID   string `json:"eventId"`
	EventType string `json:"eventType"`
	Detail    string `json:"detail"`
}

// ReplayOrder is the Order runtime state rebuilt from events.
type ReplayOrder struct {
	LocalOrderID   string  `json:"localOrderId"`
	ClientOrderID  string  `json:"clientOrderId"`
	AccountID      string  `json:"accountId"`
	StockCode      string  `json:"stockCode"`
	Side           string  `json:"side"`
	Price          float64 `json:"price"`  // Frozen Spec limit_price projection (never mutated by fills)
	Volume         int64   `json:"volume"` // Frozen Spec target_volume projection
	Status         string  `json:"status"`
	BrokerStatus   string  `json:"brokerStatus"`
	FilledVolume   int64   `json:"filledVolume"`
	FilledPrice    float64 `json:"filledPrice"`
	LeavesQuantity int64   `json:"leavesQuantity"`
	RejectCode     string  `json:"rejectCode"`
	RejectReason   string  `json:"rejectReason"`
	Terminal       bool    `json:"terminal"`
}

// ReplayFill is one reconstructed fill fact (exec_id unique).
type ReplayFill struct {
	ExecID   string  `json:"execId"`
	ReportID string  `json:"reportId"`
	Qty      int64   `json:"qty"`
	Price    float64 `json:"price"`
}

// Checkpoint is the replay cursor plus optional durable snapshot fields.
// Memory stores may carry only the cursor; PersistentCheckpointStore requires
// SnapshotJSON + SnapshotHash so incremental resume can restore full fold state.
type Checkpoint struct {
	Scope                 string `json:"scope"`
	LastAuditID           uint   `json:"lastAuditId"`
	LastEventID           string `json:"lastEventId"`
	EventsSeen            int    `json:"eventsSeen"`
	SubmitOutcom          string `json:"submitOutcome,omitempty"`
	ReplayContractVersion string `json:"replayContractVersion,omitempty"`
	SnapshotJSON          string `json:"snapshotJson,omitempty"`
	SnapshotHash          string `json:"snapshotHash,omitempty"`
	FillCount             int    `json:"fillCount,omitempty"`
	DivergenceCount       int    `json:"divergenceCount,omitempty"`
	Version               int    `json:"version,omitempty"`
}

// ReplayResult is the rebuilt runtime view plus divergence report.
type ReplayResult struct {
	Order         ReplayOrder  `json:"order"`
	Fills         []ReplayFill `json:"fills"`
	FillQty       int64        `json:"fillQty"`
	PositionDelta int64        `json:"positionDelta"`
	SpecHash      string       `json:"specHash"`
	SubmitOutcome string       `json:"submitOutcome"`
	EventsApplied int          `json:"eventsApplied"`
	EventsSkipped int          `json:"eventsSkipped"`
	Divergences   []Divergence `json:"divergences"`
	Checkpoint    Checkpoint   `json:"checkpoint"`
}

// FillCount returns the number of distinct exec_id fills.
func (r ReplayResult) FillCount() int { return len(r.Fills) }

// HasDivergence reports whether any inconsistency was detected.
func (r ReplayResult) HasDivergence() bool { return len(r.Divergences) > 0 }

// ReplayEvents rebuilds runtime state from an ordered event slice.
// Idempotent by event_id (duplicates skipped) and by exec_id (fills counted once).
// spec_hash drift is reported as divergence rather than silently applied.
func ReplayEvents(events []Event) ReplayResult {
	state := newReplayState()
	for _, ev := range events {
		state.apply(ev, 0)
	}
	return state.result()
}

type replayState struct {
	out         ReplayResult
	seenEventID map[string]struct{}
	seenExecID  map[string]struct{}
}

func newReplayState() *replayState {
	return &replayState{
		out:         ReplayResult{Fills: make([]ReplayFill, 0, 4)},
		seenEventID: make(map[string]struct{}),
		seenExecID:  make(map[string]struct{}),
	}
}

func (s *replayState) result() ReplayResult {
	out := s.out
	out.Checkpoint.EventsSeen = out.EventsApplied
	out.Checkpoint.SubmitOutcom = out.SubmitOutcome
	return out
}

func (s *replayState) addDivergence(kind string, ev Event, detail string) {
	s.out.Divergences = append(s.out.Divergences, Divergence{
		Kind: kind, EventID: ev.EventID, EventType: ev.EventType, Detail: detail,
	})
}

// apply folds one event into the state. auditID is the audit_events row id (0 for in-memory).
func (s *replayState) apply(ev Event, auditID uint) {
	if strings.TrimSpace(ev.EventID) == "" {
		s.out.EventsSkipped++
		s.addDivergence(DivergenceMissingEventID, ev, "event without event_id cannot be replayed")
		return
	}
	if _, dup := s.seenEventID[ev.EventID]; dup {
		s.out.EventsSkipped++
		return
	}
	s.seenEventID[ev.EventID] = struct{}{}

	if s.out.SpecHash == "" {
		s.out.SpecHash = ev.SpecHash
	} else if ev.SpecHash != s.out.SpecHash {
		s.addDivergence(DivergenceSpecHashMismatch, ev,
			fmt.Sprintf("spec_hash %q != replay baseline %q", ev.SpecHash, s.out.SpecHash))
	}

	s.out.EventsApplied++
	if auditID > s.out.Checkpoint.LastAuditID {
		s.out.Checkpoint.LastAuditID = auditID
	}
	s.out.Checkpoint.LastEventID = ev.EventID

	s.applyIdentity(ev)

	switch ev.EventType {
	case TypeSubmitAttempt:
		// Frozen Spec projection: only source of limit_price / target_volume.
		s.out.Order.StockCode = payloadStr(ev.Payload, "symbol")
		s.out.Order.Side = payloadStr(ev.Payload, "side")
		s.out.Order.Price = payloadF64(ev.Payload, "limit_price")
		s.out.Order.Volume = payloadI64(ev.Payload, "target_volume")
		s.out.Order.LeavesQuantity = s.out.Order.Volume

	case TypeSubmitAccepted, TypeSubmitTimeout:
		s.out.SubmitOutcome = payloadStr(ev.Payload, "submit_outcome")
		s.out.Order.Status = payloadStr(ev.Payload, "oms_status")
		s.out.Order.BrokerStatus = payloadStr(ev.Payload, "broker_status")

	case TypeSubmitRejected:
		s.out.SubmitOutcome = payloadStr(ev.Payload, "submit_outcome")
		s.out.Order.Status = payloadStr(ev.Payload, "oms_status")
		s.out.Order.BrokerStatus = payloadStr(ev.Payload, "broker_status")
		s.out.Order.RejectCode = payloadStr(ev.Payload, "reject_code")
		s.out.Order.RejectReason = payloadStr(ev.Payload, "reject_reason")

	case TypeReportReceived:
		if payloadStr(ev.Payload, "apply_result") != "applied" {
			return
		}
		if payloadStr(ev.Payload, "report_type") == replayReportACK {
			s.out.Order.Status = replayStatusPending
			s.out.Order.BrokerStatus = replayBrokerWorking
		}

	case TypeFillApplied:
		execID := strings.TrimSpace(ev.ExecID)
		if execID == "" {
			execID = payloadStr(ev.Payload, "exec_id")
		}
		if execID == "" {
			s.addDivergence(DivergenceMissingExecID, ev, "FILL_APPLIED without exec_id")
			return
		}
		if _, dup := s.seenExecID[execID]; dup {
			return
		}
		s.seenExecID[execID] = struct{}{}

		qty := payloadI64(ev.Payload, "fill_qty")
		s.out.Fills = append(s.out.Fills, ReplayFill{
			ExecID:   execID,
			ReportID: payloadStr(ev.Payload, "report_id"),
			Qty:      qty,
			Price:    payloadF64(ev.Payload, "fill_price"),
		})
		s.out.FillQty += qty
		s.out.PositionDelta += payloadI64(ev.Payload, "position_delta")
		s.out.Order.FilledVolume = payloadI64(ev.Payload, "cum_qty")
		s.out.Order.FilledPrice = payloadF64(ev.Payload, "avg_price")
		s.out.Order.LeavesQuantity = payloadI64(ev.Payload, "leaves_quantity")
		s.out.Order.Status = payloadStr(ev.Payload, "oms_status_after")
		s.out.Order.BrokerStatus = payloadStr(ev.Payload, "broker_status_after")

	case TypeOrderTerminal:
		s.out.Order.Terminal = true
		s.out.Order.Status = payloadStr(ev.Payload, "terminal_status")
		s.out.Order.BrokerStatus = payloadStr(ev.Payload, "broker_status")
		s.out.Order.FilledVolume = payloadI64(ev.Payload, "filled_volume")
		s.out.Order.FilledPrice = payloadF64(ev.Payload, "avg_fill_price")
		s.out.Order.LeavesQuantity = payloadI64(ev.Payload, "leaves_quantity")
		if v := payloadStr(ev.Payload, "reject_code"); v != "" {
			s.out.Order.RejectCode = v
		}
		if v := payloadStr(ev.Payload, "reject_reason"); v != "" {
			s.out.Order.RejectReason = v
		}
	}
}

func (s *replayState) applyIdentity(ev Event) {
	if v := strings.TrimSpace(ev.LocalOrderID); v != "" {
		s.out.Order.LocalOrderID = v
	}
	if v := strings.TrimSpace(ev.ClientOrderID); v != "" {
		s.out.Order.ClientOrderID = v
	}
	if v := strings.TrimSpace(ev.AccountID); v != "" {
		s.out.Order.AccountID = v
	}
}

// ---------- checkpoint store ----------

// CheckpointStore persists replay cursors (and optionally full snapshots).
type CheckpointStore interface {
	Load(scope string) (Checkpoint, bool)
	Save(cp Checkpoint) error
}

// VerifiedCheckpointLoader distinguishes missing checkpoints from corrupted ones.
type VerifiedCheckpointLoader interface {
	LoadVerified(scope string) (Checkpoint, error)
}

// MemoryCheckpointStore is a process-local cursor store.
type MemoryCheckpointStore struct {
	mu     sync.Mutex
	cursor map[string]Checkpoint
}

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{cursor: make(map[string]Checkpoint)}
}

func (s *MemoryCheckpointStore) Load(scope string) (Checkpoint, bool) {
	if s == nil {
		return Checkpoint{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.cursor[scope]
	return cp, ok
}

func (s *MemoryCheckpointStore) Save(cp Checkpoint) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursor[cp.Scope] = cp
	return nil
}

// ---------- DB-backed replayer ----------

// ReplayScope selects the audit_events slice to replay.
type ReplayScope struct {
	// BrokerRequestID / ClientOrderID / LocalOrderID are optional filters (AND semantics).
	BrokerRequestID string
	ClientOrderID   string
	LocalOrderID    string
	// AfterAuditID replays strictly after this row id (incremental replay).
	AfterAuditID uint
	// UntilAuditID when >0 includes only rows with id <= UntilAuditID (inclusive high watermark).
	UntilAuditID uint
	// Limit caps rows read (0 = unlimited).
	Limit int
}

func (s ReplayScope) key() string {
	switch {
	case strings.TrimSpace(s.BrokerRequestID) != "":
		return "broker_request_id:" + strings.TrimSpace(s.BrokerRequestID)
	case strings.TrimSpace(s.ClientOrderID) != "":
		return "client_order_id:" + strings.TrimSpace(s.ClientOrderID)
	case strings.TrimSpace(s.LocalOrderID) != "":
		return "order_id:" + strings.TrimSpace(s.LocalOrderID)
	default:
		return "global"
	}
}

// Replayer reads audit_events and rebuilds runtime state. Read-only by construction.
type Replayer struct {
	db          *gorm.DB
	checkpoints CheckpointStore
}

func NewReplayer(database *gorm.DB) *Replayer {
	return &Replayer{db: database, checkpoints: NewMemoryCheckpointStore()}
}

// WithCheckpointStore swaps the cursor store (memory by default).
func (r *Replayer) WithCheckpointStore(store CheckpointStore) *Replayer {
	if r != nil && store != nil {
		r.checkpoints = store
	}
	return r
}

// Checkpoint returns the stored cursor for a scope.
func (r *Replayer) Checkpoint(scope ReplayScope) (Checkpoint, bool) {
	if r == nil || r.checkpoints == nil {
		return Checkpoint{}, false
	}
	return r.checkpoints.Load(scope.key())
}

// Replay reads events in append order and rebuilds Order / Fills / Position delta.
// It performs no writes to execution tables and does not mutate Frozen Spec.
func (r *Replayer) Replay(scope ReplayScope) (ReplayResult, error) {
	return r.replay(scope, nil)
}

// ReplayIncremental continues from a stored checkpoint for the scope.
//
// Behavior:
//   - No checkpoint → same as Replay (legacy full fold from AfterAuditID / start).
//   - Checkpoint with intact snapshot → restore fold state, then apply only new rows.
//   - Checkpoint with cursor only (no snapshot) → legacy AfterAuditID tail fold.
//   - Corrupted persistent snapshot → error (never silently repaired).
func (r *Replayer) ReplayIncremental(scope ReplayScope) (ReplayResult, error) {
	cp, ok, err := r.loadCheckpointForIncremental(scope)
	if err != nil {
		return ReplayResult{}, err
	}
	if !ok {
		return r.Replay(scope)
	}
	scope.AfterAuditID = cp.LastAuditID
	if strings.TrimSpace(cp.SnapshotJSON) == "" {
		// Cursor-only legacy path (pre-snapshot memory checkpoints).
		return r.Replay(scope)
	}
	if err := VerifyCheckpointIntegrity(cp); err != nil {
		return ReplayResult{}, err
	}
	return r.replay(scope, &cp)
}

func (r *Replayer) loadCheckpointForIncremental(scope ReplayScope) (Checkpoint, bool, error) {
	if r == nil || r.checkpoints == nil {
		return Checkpoint{}, false, nil
	}
	key := scope.key()
	if loader, ok := r.checkpoints.(VerifiedCheckpointLoader); ok {
		cp, err := loader.LoadVerified(key)
		if err != nil {
			if errors.Is(err, ErrCheckpointMissing) {
				return Checkpoint{}, false, nil
			}
			return Checkpoint{}, false, err
		}
		return cp, true, nil
	}
	cp, ok := r.checkpoints.Load(key)
	return cp, ok, nil
}

func (r *Replayer) replay(scope ReplayScope, baseline *Checkpoint) (ReplayResult, error) {
	if r == nil || r.db == nil {
		return ReplayResult{}, fmt.Errorf("audit replay: 数据库未初始化")
	}

	query := r.db.Model(&models.AuditEvent{}).Order("id ASC")
	if v := strings.TrimSpace(scope.BrokerRequestID); v != "" {
		query = query.Where("broker_request_id = ?", v)
	}
	if v := strings.TrimSpace(scope.ClientOrderID); v != "" {
		query = query.Where("client_order_id = ?", v)
	}
	if v := strings.TrimSpace(scope.LocalOrderID); v != "" {
		query = query.Where("order_id = ?", v)
	}
	if scope.AfterAuditID > 0 {
		query = query.Where("id > ?", scope.AfterAuditID)
	}
	if scope.UntilAuditID > 0 {
		query = query.Where("id <= ?", scope.UntilAuditID)
	}
	if scope.Limit > 0 {
		query = query.Limit(scope.Limit)
	}

	var rows []models.AuditEvent
	if err := query.Find(&rows).Error; err != nil {
		return ReplayResult{}, fmt.Errorf("audit replay: read audit_events: %w", err)
	}

	state := newReplayState()
	if baseline != nil {
		snap, err := DecodeSnapshot(baseline.SnapshotJSON, baseline.SnapshotHash)
		if err != nil {
			return ReplayResult{}, err
		}
		state = newReplayStateFromSnapshot(snap, *baseline)
	}

	for _, row := range rows {
		ev, err := rowToEvent(row)
		if err != nil {
			state.out.EventsSkipped++
			state.addDivergence(DivergenceDecodeFailed,
				Event{EventID: row.EventID, EventType: row.EventType}, err.Error())
			continue
		}
		state.apply(ev, row.ID)
	}

	out := state.result()
	out.Checkpoint.Scope = scope.key()
	if out.Checkpoint.LastAuditID < scope.AfterAuditID {
		out.Checkpoint.LastAuditID = scope.AfterAuditID
	}
	if baseline != nil && out.Checkpoint.LastAuditID < baseline.LastAuditID {
		out.Checkpoint.LastAuditID = baseline.LastAuditID
		out.Checkpoint.LastEventID = baseline.LastEventID
	}

	attached, err := AttachSnapshot(out.Checkpoint, out)
	if err != nil {
		return out, fmt.Errorf("audit replay: attach snapshot: %w", err)
	}
	out.Checkpoint = attached
	if r.checkpoints != nil {
		if saveErr := r.checkpoints.Save(out.Checkpoint); saveErr != nil {
			// Persistence failure must not invent business repairs; surface to caller.
			return out, fmt.Errorf("audit replay: save checkpoint: %w", saveErr)
		}
	}
	return out, nil
}

func newReplayStateFromSnapshot(snap ReplaySnapshot, cp Checkpoint) *replayState {
	out := ResultFromSnapshot(snap)
	out.Checkpoint = Checkpoint{
		Scope:                 cp.Scope,
		LastAuditID:           cp.LastAuditID,
		LastEventID:           cp.LastEventID,
		EventsSeen:            snap.EventsApplied,
		SubmitOutcom:          snap.SubmitOutcome,
		ReplayContractVersion: ReplayContractVersion,
		Version:               cp.Version,
	}
	state := &replayState{
		out:         out,
		seenEventID: make(map[string]struct{}, 8),
		seenExecID:  make(map[string]struct{}, len(snap.Fills)),
	}
	// Seed exec_id idempotency from restored fills so retransmits in the tail
	// window cannot double-count. Historical event_ids are outside the window.
	for _, fill := range snap.Fills {
		if id := strings.TrimSpace(fill.ExecID); id != "" {
			state.seenExecID[id] = struct{}{}
		}
	}
	return state
}

func rowToEvent(row models.AuditEvent) (Event, error) {
	var ev Event
	if strings.TrimSpace(row.PayloadJSON) != "" {
		if err := json.Unmarshal([]byte(row.PayloadJSON), &ev); err != nil {
			return Event{}, fmt.Errorf("decode payload_json: %w", err)
		}
	}
	if ev.EventID == "" {
		ev.EventID = row.EventID
	}
	if ev.EventType == "" {
		ev.EventType = row.EventType
	}
	if ev.SpecHash == "" {
		ev.SpecHash = row.SpecHash
	}
	if ev.ReportID == "" {
		ev.ReportID = row.ReportID
	}
	if ev.ExecID == "" {
		ev.ExecID = row.ExecID
	}
	if ev.ClientOrderID == "" {
		ev.ClientOrderID = row.ClientOrderID
	}
	if ev.AccountID == "" {
		ev.AccountID = row.AccountID
	}
	if ev.LocalOrderID == "" && row.OrderID != nil {
		ev.LocalOrderID = *row.OrderID
	}
	return ev, nil
}

func payloadStr(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return v
}

func payloadI64(payload map[string]any, key string) int64 {
	switch v := payload[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

func payloadF64(payload map[string]any, key string) float64 {
	switch v := payload[key].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
