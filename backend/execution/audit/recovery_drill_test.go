package audit_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Phase6.5.8.4.2 — Recovery Drill Harness acceptance (P1–P5).
// Isolated from trading paths: no Execution / Order / Fill / Position schema edits.

func setupDrillDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:recovery_drill_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	require.NoError(t, db.MigrateRecoveryCheckpoints(database))
	return database
}

func drillLifecycle(t *testing.T, cid, symbol, req string) (spec string, prefix, tail []audit.Event) {
	t.Helper()
	spec = audit.SpecFingerprint(symbol, "buy", 10, 100)
	now := time.Now().UTC()
	prefix = []audit.Event{
		audit.BuildSubmitAttempt(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			symbol, "buy", 10, 100, now,
		),
		audit.BuildSubmitResult(
			audit.TypeSubmitAccepted, audit.BackendRealStub, req, cid, "1", "1", spec,
			"accepted", "pending", "accepted", now, now.Add(time.Second), "", "", map[string]any{"status": "pending"},
		),
		audit.BuildReportReceived(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-1", "TRADE", req+"-E1", "new", "applied", now.Add(2*time.Second), map[string]any{},
			map[string]any{"last_qty": 40, "last_price": 10.0},
		),
		audit.BuildFillApplied(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-1", req+"-E1", "report_received:"+req+"-tr-1",
			40, 10.0, 40, 10.0, 60, 40,
			"pending", "partially_filled", now.Add(2*time.Second),
		),
	}
	tail = []audit.Event{
		audit.BuildReportReceived(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-2", "TRADE", req+"-E2", "new", "applied", now.Add(3*time.Second), map[string]any{},
			map[string]any{"last_qty": 60, "last_price": 10.1},
		),
		audit.BuildFillApplied(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-2", req+"-E2", "report_received:"+req+"-tr-2",
			60, 10.1, 100, 10.06, 0, 60,
			"filled", "filled", now.Add(3*time.Second),
		),
		audit.BuildOrderTerminal(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			"filled", "filled", 100, 0, 10.06, req+"-tr-2", now.Add(3*time.Second), nil,
		),
	}
	return spec, prefix, tail
}

func TestRecoveryDrill_P1_CheckpointIncrementalEqualsFull(t *testing.T) {
	h := audit.NewRecoveryDrillHarness(setupDrillDB(t))
	spec, prefix, tail := drillLifecycle(t, "cid-drill-1", "sz000801", "req-drill-1")
	scope := audit.ReplayScope{ClientOrderID: "cid-drill-1"}

	report, err := h.RunCheckpointIncrementalDrill(scope, prefix, tail)
	require.NoError(t, err)
	require.False(t, report.HasDivergence(), "divergences=%+v", report.Divergences)

	require.Equal(t, spec, report.FullReplay.SpecHash)
	require.Equal(t, report.FullReplay.Order, report.IncrementalReplay.Order)
	require.Equal(t, report.FullReplay.FillQty, report.IncrementalReplay.FillQty)
	require.Equal(t, report.FullReplay.PositionDelta, report.IncrementalReplay.PositionDelta)
	require.Equal(t, int64(100), report.IncrementalReplay.FillQty)
	require.True(t, report.IncrementalReplay.Order.Terminal)
	require.InDelta(t, 10.0, report.IncrementalReplay.Order.Price, 1e-9)
	require.Equal(t, int64(100), report.IncrementalReplay.Order.Volume)

	runtime, ok := h.RuntimeSnapshot("cid-drill-1")
	require.True(t, ok)
	require.Empty(t, audit.CompareReplayToRuntime(report.IncrementalReplay, runtime))
	require.NotEmpty(t, report.Checkpoint.SnapshotHash)
}

func TestRecoveryDrill_P2_CheckpointCorruptionDetected(t *testing.T) {
	h := audit.NewRecoveryDrillHarness(setupDrillDB(t))
	_, prefix, tail := drillLifecycle(t, "cid-drill-2", "sz000802", "req-drill-2")
	scope := audit.ReplayScope{ClientOrderID: "cid-drill-2"}

	require.NoError(t, h.AppendAuditEvents(prefix))
	_, err := h.CreateCheckpoint(scope)
	require.NoError(t, err)
	require.NoError(t, h.CorruptCheckpoint(scope))

	_, err = h.LoadCheckpointVerified(scope)
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))

	require.NoError(t, h.AppendAuditEvents(tail))
	_, err = h.ReplayIncremental(scope)
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))

	// Full replay still works (read-only audit path); no auto-repair of checkpoint.
	full, err := h.ReplayFull(scope)
	require.NoError(t, err)
	require.Equal(t, int64(100), full.FillQty)

	var rows []models.RecoveryCheckpoint
	require.NoError(t, h.DB().Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "drill-corrupt-deadbeef", rows[0].SnapshotHash)
}

func TestRecoveryDrill_P3_AuditWindowGapReportsDivergence(t *testing.T) {
	h := audit.NewRecoveryDrillHarness(setupDrillDB(t))
	_, prefix, tail := drillLifecycle(t, "cid-drill-3", "sz000803", "req-drill-3")
	scope := audit.ReplayScope{ClientOrderID: "cid-drill-3"}

	require.NoError(t, h.AppendAuditEvents(prefix))
	partial, err := h.CreateCheckpoint(scope)
	require.NoError(t, err)
	afterID := partial.Checkpoint.LastAuditID

	require.NoError(t, h.AppendAuditEvents(tail))
	var highWatermark uint
	require.NoError(t, h.DB().Model(&models.AuditEvent{}).
		Where("client_order_id = ?", "cid-drill-3").
		Select("MAX(id)").Scan(&highWatermark).Error)
	require.Greater(t, highWatermark, afterID)

	expectedTail := int64(len(tail))
	gapsBefore := h.CheckAuditWindowExpectedCount(scope, afterID, highWatermark, expectedTail)
	require.Empty(t, gapsBefore, "window should be complete before fault injection")

	deleted, err := h.DeleteAuditWindow("cid-drill-3", afterID, highWatermark)
	require.NoError(t, err)
	require.Equal(t, expectedTail, deleted)

	gaps := h.CheckAuditWindowExpectedCount(scope, afterID, highWatermark, expectedTail)
	require.NotEmpty(t, gaps)
	require.Equal(t, audit.DivergenceAuditWindowGap, gaps[0].Kind)

	emptyGaps := h.CheckAuditWindow(scope, afterID, highWatermark)
	require.NotEmpty(t, emptyGaps)
	require.Equal(t, audit.DivergenceAuditWindowGap, emptyGaps[0].Kind)

	// Incremental from checkpoint cannot see deleted tail → diverges from runtime mirror.
	incremental, err := h.ReplayIncremental(scope)
	require.NoError(t, err)
	require.Equal(t, int64(40), incremental.FillQty)
	require.False(t, incremental.Order.Terminal)

	runtime, ok := h.RuntimeSnapshot("cid-drill-3")
	require.True(t, ok)
	require.Equal(t, int64(100), runtime.FillQty)
	runtimeDivs := audit.CompareReplayToRuntime(incremental, runtime)
	require.NotEmpty(t, runtimeDivs)
	require.Equal(t, audit.DivergenceRuntimeMismatch, runtimeDivs[0].Kind)
}

func TestRecoveryDrill_P4_MultiScopeCheckpointIsolation(t *testing.T) {
	h := audit.NewRecoveryDrillHarness(setupDrillDB(t))
	_, prefixA, tailA := drillLifecycle(t, "cid-drill-a", "sz000804", "req-drill-a")
	_, prefixB, tailB := drillLifecycle(t, "cid-drill-b", "sz000805", "req-drill-b")
	// Second order uses local_order_id "2" in payloads for clarity.
	for i := range prefixB {
		prefixB[i].LocalOrderID = "2"
		if prefixB[i].Payload != nil {
			prefixB[i].Payload["order_id"] = "2"
		}
	}
	for i := range tailB {
		tailB[i].LocalOrderID = "2"
		if tailB[i].Payload != nil {
			tailB[i].Payload["order_id"] = "2"
		}
	}

	scopeA := audit.ReplayScope{ClientOrderID: "cid-drill-a"}
	scopeB := audit.ReplayScope{ClientOrderID: "cid-drill-b"}

	require.NoError(t, h.AppendAuditEvents(prefixA))
	require.NoError(t, h.AppendAuditEvents(prefixB))
	_, err := h.CreateCheckpoint(scopeA)
	require.NoError(t, err)
	_, err = h.CreateCheckpoint(scopeB)
	require.NoError(t, err)

	cpA, err := h.LoadCheckpointVerified(scopeA)
	require.NoError(t, err)
	cpB, err := h.LoadCheckpointVerified(scopeB)
	require.NoError(t, err)
	require.NotEqual(t, cpA.Scope, cpB.Scope)
	require.NotEqual(t, cpA.SnapshotHash, cpB.SnapshotHash)

	require.NoError(t, h.CorruptCheckpoint(scopeA))
	_, err = h.LoadCheckpointVerified(scopeA)
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))

	// Scope B remains intact and independently replayable.
	_, err = h.LoadCheckpointVerified(scopeB)
	require.NoError(t, err)
	require.NoError(t, h.AppendAuditEvents(tailB))
	incrB, err := h.ReplayIncremental(scopeB)
	require.NoError(t, err)
	require.Equal(t, "sz000805", incrB.Order.StockCode)
	require.Equal(t, int64(100), incrB.FillQty)

	require.NoError(t, h.AppendAuditEvents(tailA))
	fullA, err := h.ReplayFull(scopeA)
	require.NoError(t, err)
	require.Equal(t, "sz000804", fullA.Order.StockCode)
	require.Equal(t, int64(100), fullA.FillQty)
	require.NotEqual(t, fullA.Order.StockCode, incrB.Order.StockCode)

	var count int64
	require.NoError(t, h.DB().Model(&models.RecoveryCheckpoint{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestRecoveryDrill_P5_RepeatedReplayIdempotent(t *testing.T) {
	h := audit.NewRecoveryDrillHarness(setupDrillDB(t))
	_, prefix, tail := drillLifecycle(t, "cid-drill-5", "sz000806", "req-drill-5")
	scope := audit.ReplayScope{ClientOrderID: "cid-drill-5"}

	report, err := h.RunCheckpointIncrementalDrill(scope, prefix, tail)
	require.NoError(t, err)
	require.False(t, report.HasDivergence())

	first := report.IncrementalReplay
	second, err := h.ReplayIncremental(scope)
	require.NoError(t, err)
	third, err := h.ReplayIncremental(scope)
	require.NoError(t, err)

	require.Empty(t, audit.CompareReplayEqual(first, second))
	require.Empty(t, audit.CompareReplayEqual(second, third))
	require.Equal(t, first.Order, third.Order)
	require.Equal(t, first.FillQty, third.FillQty)
	require.Equal(t, first.PositionDelta, third.PositionDelta)
	require.Equal(t, first.Fills, third.Fills)

	// Re-append the same L0 events (event_id idempotent) must not change fold output.
	require.NoError(t, h.AppendAuditEvents(append(append([]audit.Event{}, prefix...), tail...)))
	afterDup, err := h.ReplayFull(scope)
	require.NoError(t, err)
	require.Empty(t, audit.CompareReplayEqual(first, afterDup))

	var eventRows int64
	require.NoError(t, h.DB().Model(&models.AuditEvent{}).
		Where("client_order_id = ?", "cid-drill-5").Count(&eventRows).Error)
	require.Equal(t, int64(len(prefix)+len(tail)), eventRows)
}
