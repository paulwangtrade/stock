package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Phase6.5.8.3 — Audit Replay Recovery acceptance.
//
// Drives real runtime (RealBroker + ExecutionReportConsumer), then rebuilds
// Order / Fill set / Position delta purely from persisted audit_events and
// compares against the live runtime. Replay is read-only.

func setupReplayRecoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:audit_replay_recovery_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	return database
}

func newReplayRecoveryFixture(t *testing.T) (*RealBroker, *ExecutionReportConsumer, *gorm.DB, *InMemoryPositionAccountant) {
	t.Helper()
	database := setupReplayRecoveryDB(t)
	real := NewRealBroker()
	real.SetAuditEmitter(audit.NewPersistenceEmitter(audit.NewMemoryAuditSink(), database))
	acc := NewInMemoryPositionAccountant()
	real.SetPositionAccountant(acc)
	return real, NewExecutionReportConsumer(real, acc), database, acc
}

func replayAll(t *testing.T, database *gorm.DB) audit.ReplayResult {
	t.Helper()
	res, err := audit.NewReplayer(database).Replay(audit.ReplayScope{})
	require.NoError(t, err)
	return res
}

func requireReplayEqualsRuntime(
	t *testing.T,
	replayed audit.ReplayResult,
	runtimeOrder *broker.TradeOrder,
	runtimeFillQty int64,
	runtimeFillCount int,
	runtimePositionDelta int64,
) {
	t.Helper()
	require.NotNil(t, runtimeOrder)
	require.Equal(t, runtimeOrder.StockCode, replayed.Order.StockCode)
	require.Equal(t, runtimeOrder.Side, replayed.Order.Side)
	require.InDelta(t, runtimeOrder.Price, replayed.Order.Price, 1e-9)
	require.Equal(t, runtimeOrder.Volume, replayed.Order.Volume)
	require.Equal(t, runtimeOrder.Status, replayed.Order.Status)
	require.Equal(t, runtimeOrder.BrokerStatus, replayed.Order.BrokerStatus)
	require.Equal(t, runtimeOrder.FilledVolume, replayed.Order.FilledVolume)
	require.InDelta(t, runtimeOrder.FilledPrice, replayed.Order.FilledPrice, 1e-9)
	require.Equal(t, runtimeOrder.LeavesQuantity, replayed.Order.LeavesQuantity)
	require.Equal(t, runtimeOrder.RejectCode, replayed.Order.RejectCode)
	require.Equal(t, runtimeOrder.RejectReason, replayed.Order.RejectReason)
	require.Equal(t, runtimeFillQty, replayed.FillQty)
	require.Equal(t, runtimeFillCount, replayed.FillCount())
	require.Equal(t, runtimePositionDelta, replayed.PositionDelta)
	require.False(t, replayed.HasDivergence(), "unexpected divergence: %+v", replayed.Divergences)
}

func runtimeFillTotals(t *testing.T, real *RealBroker, cid string) (int64, int) {
	t.Helper()
	fills, err := real.ListFills(context.Background(), cid)
	require.NoError(t, err)
	var qty int64
	for _, f := range fills {
		qty += f.FillQty
	}
	return qty, len(fills)
}

func TestAuditReplayRecovery_P1_FullLifecycle(t *testing.T) {
	real, consumer, database, acc := newReplayRecoveryFixture(t)
	specHash := audit.SpecFingerprint("sz000601", "buy", 10.0, 100)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000601", Side: "buy",
		Price: 10.0, Volume: 100, SpecHash: specHash,
	})
	require.NoError(t, err)
	cid := order.ClientOrderID

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "r1-ack", ClientOrderID: cid,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r1-tr", ExecID: "R1E1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.25,
	}))

	runtimeOrder, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	fillQty, fillCount := runtimeFillTotals(t, real, cid)
	pos, ok := acc.Position("1", "sz000601")
	require.True(t, ok)

	replayed := replayAll(t, database)
	requireReplayEqualsRuntime(t, replayed, runtimeOrder, fillQty, fillCount, pos.Volume)
	require.Equal(t, specHash, replayed.SpecHash)
	require.Equal(t, data.PaperOrderStatusFilled, replayed.Order.Status)
	require.True(t, replayed.Order.Terminal)
	require.Equal(t, cid, replayed.Order.ClientOrderID)

	// Frozen Spec projection stays at limit price / target volume despite 10.25 fill.
	require.InDelta(t, 10.0, replayed.Order.Price, 1e-9)
	require.Equal(t, int64(100), replayed.Order.Volume)
}

func TestAuditReplayRecovery_P2_PartialFill(t *testing.T) {
	real, consumer, database, acc := newReplayRecoveryFixture(t)
	specHash := audit.SpecFingerprint("sz000602", "buy", 10.0, 100)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000602", Side: "buy",
		Price: 10.0, Volume: 100, SpecHash: specHash,
	})
	require.NoError(t, err)
	cid := order.ClientOrderID

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r2-a", ExecID: "R2E1",
		ClientOrderID: cid, LastQty: 40, LastPrice: 10.0,
	}))

	partialOrder, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	partialQty, partialCount := runtimeFillTotals(t, real, cid)
	partialPos, ok := acc.Position("1", "sz000602")
	require.True(t, ok)

	partialReplay := replayAll(t, database)
	requireReplayEqualsRuntime(t, partialReplay, partialOrder, partialQty, partialCount, partialPos.Volume)
	require.Equal(t, int64(40), partialReplay.FillQty)
	require.Equal(t, int64(60), partialReplay.Order.LeavesQuantity)
	require.False(t, partialReplay.Order.Terminal)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r2-b", ExecID: "R2E2",
		ClientOrderID: cid, LastQty: 60, LastPrice: 10.1,
	}))

	finalOrder, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	finalQty, finalCount := runtimeFillTotals(t, real, cid)
	finalPos, ok := acc.Position("1", "sz000602")
	require.True(t, ok)

	finalReplay := replayAll(t, database)
	requireReplayEqualsRuntime(t, finalReplay, finalOrder, finalQty, finalCount, finalPos.Volume)
	require.Equal(t, 2, finalReplay.FillCount())
	require.Equal(t, int64(100), finalReplay.FillQty)
	require.Equal(t, int64(0), finalReplay.Order.LeavesQuantity)
	require.True(t, finalReplay.Order.Terminal)

	// Checkpoint + tail events must equal a full replay (snapshot restore, not cursor-only).
	replayer := audit.NewReplayer(database)
	first, err := replayer.Replay(audit.ReplayScope{Limit: partialReplay.EventsApplied})
	require.NoError(t, err)
	require.Greater(t, first.Checkpoint.LastAuditID, uint(0))
	require.NotEmpty(t, first.Checkpoint.SnapshotHash)
	merged, err := replayer.ReplayIncremental(audit.ReplayScope{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, merged.Checkpoint.LastAuditID, first.Checkpoint.LastAuditID)
	require.Equal(t, finalReplay.Order, merged.Order)
	require.Equal(t, finalReplay.FillQty, merged.FillQty)
	require.Equal(t, finalReplay.PositionDelta, merged.PositionDelta)
}

func TestAuditReplayRecovery_P3_DuplicateEvents(t *testing.T) {
	real, consumer, database, acc := newReplayRecoveryFixture(t)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000603", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000603", "buy", 10, 100),
	})
	require.NoError(t, err)
	cid := order.ClientOrderID

	report := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r3-tr", ExecID: "R3E1",
		ClientOrderID: cid, LastQty: 40, LastPrice: 10,
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), report))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), report))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r3-tr-2", ExecID: "R3E1",
		ClientOrderID: cid, LastQty: 40, LastPrice: 10,
	}))

	runtimeOrder, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	fillQty, fillCount := runtimeFillTotals(t, real, cid)
	pos, ok := acc.Position("1", "sz000603")
	require.True(t, ok)

	replayed := replayAll(t, database)
	requireReplayEqualsRuntime(t, replayed, runtimeOrder, fillQty, fillCount, pos.Volume)
	require.Equal(t, 1, replayed.FillCount())
	require.Equal(t, int64(40), replayed.PositionDelta)

	// Replaying the same rows twice must yield an identical projection.
	again := replayAll(t, database)
	require.Equal(t, replayed.Order, again.Order)
	require.Equal(t, replayed.FillQty, again.FillQty)
	require.Equal(t, replayed.PositionDelta, again.PositionDelta)
}

func TestAuditReplayRecovery_P4_SubmitRejected(t *testing.T) {
	database := setupReplayRecoveryDB(t)
	real := NewRealBrokerWithAdapter(&programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeRejected, Message: "risk reject"},
	})
	real.SetAuditEmitter(audit.NewPersistenceEmitter(audit.NewMemoryAuditSink(), database))

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000604", Side: "buy", Price: 12, Volume: 200,
		SpecHash: audit.SpecFingerprint("sz000604", "buy", 12, 200),
	})
	require.ErrorIs(t, err, ErrBrokerRejected)
	require.NotNil(t, order)

	replayed := replayAll(t, database)
	requireReplayEqualsRuntime(t, replayed, order, 0, 0, 0)
	require.Equal(t, data.PaperOrderStatusRejected, replayed.Order.Status)
	require.Equal(t, data.BrokerStatusRejected, replayed.Order.BrokerStatus)
	require.Equal(t, SubmitOutcomeRejected, replayed.SubmitOutcome)
	require.True(t, replayed.Order.Terminal)
}

func TestAuditReplayRecovery_P5_SubmitTimeoutUnknown(t *testing.T) {
	database := setupReplayRecoveryDB(t)
	real := NewRealBrokerWithAdapter(&programmableAdapter{err: context.DeadlineExceeded})
	real.SetAuditEmitter(audit.NewPersistenceEmitter(audit.NewMemoryAuditSink(), database))

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000605", Side: "buy", Price: 9.5, Volume: 50,
		SpecHash: audit.SpecFingerprint("sz000605", "buy", 9.5, 50),
	})
	require.ErrorIs(t, err, ErrBrokerTimeout)
	require.NotNil(t, order)

	replayed := replayAll(t, database)
	requireReplayEqualsRuntime(t, replayed, order, 0, 0, 0)
	require.Equal(t, data.PaperOrderStatusPending, replayed.Order.Status)
	require.Equal(t, data.BrokerStatusTimeout, replayed.Order.BrokerStatus)
	require.Equal(t, SubmitOutcomeTimeout, replayed.SubmitOutcome)
	require.False(t, replayed.Order.Terminal, "timeout stays open pending reconciliation")
}

func TestAuditReplayRecovery_P6_SpecHashMismatchDivergence(t *testing.T) {
	real, consumer, database, _ := newReplayRecoveryFixture(t)
	specHash := audit.SpecFingerprint("sz000606", "buy", 10.0, 100)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000606", Side: "buy",
		Price: 10.0, Volume: 100, SpecHash: specHash,
	})
	require.NoError(t, err)
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r6-tr", ExecID: "R6E1",
		ClientOrderID: order.ClientOrderID, LastQty: 100, LastPrice: 10.0,
	}))

	clean := replayAll(t, database)
	require.False(t, clean.HasDivergence())

	// Inject a foreign-spec event into the audit stream only (execution tables untouched).
	tampered := audit.BuildFillApplied(
		audit.BackendRealStub, order.ClientOrderID, order.ClientOrderID, "1", "1",
		"tampered-spec-hash", "r6-forged", "R6E9", "",
		10, 10.0, 110, 10.0, 0, 10,
		data.PaperOrderStatusFilled, data.BrokerStatusFilled, time.Now(),
	)
	require.NoError(t, audit.NewDBAuditSink(database).Write(tampered))

	diverged := replayAll(t, database)
	require.True(t, diverged.HasDivergence())
	require.Equal(t, audit.DivergenceSpecHashMismatch, diverged.Divergences[0].Kind)
	require.Equal(t, audit.IDFillApplied("R6E9"), diverged.Divergences[0].EventID)
	require.Equal(t, specHash, diverged.SpecHash,
		"Frozen Spec baseline must remain the first observed spec_hash")

	// Replay never rewrites the audit trail nor the Frozen Spec projection.
	var rows int64
	require.NoError(t, database.Model(&models.AuditEvent{}).Count(&rows).Error)
	require.Equal(t, int64(clean.EventsApplied+1), rows)
	require.InDelta(t, 10.0, diverged.Order.Price, 1e-9)
	require.Equal(t, int64(100), diverged.Order.Volume)

	runtimeOrder, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.InDelta(t, 10.0, runtimeOrder.Price, 1e-9)
	require.Equal(t, int64(100), runtimeOrder.Volume)
	require.Equal(t, int64(100), runtimeOrder.FilledVolume)
}

func TestAuditReplayRecovery_ScopeFilterAndCheckpointCursor(t *testing.T) {
	real, consumer, database, _ := newReplayRecoveryFixture(t)
	first, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000607", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000607", "buy", 10, 100),
	})
	require.NoError(t, err)
	second, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000608", Side: "buy", Price: 20, Volume: 50,
		SpecHash: audit.SpecFingerprint("sz000608", "buy", 20, 50),
	})
	require.NoError(t, err)
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r7-tr", ExecID: "R7E1",
		ClientOrderID: second.ClientOrderID, LastQty: 50, LastPrice: 20,
	}))

	replayer := audit.NewReplayer(database)
	scope := audit.ReplayScope{ClientOrderID: first.ClientOrderID}
	scoped, err := replayer.Replay(scope)
	require.NoError(t, err)
	require.Equal(t, "sz000607", scoped.Order.StockCode)
	require.Equal(t, 0, scoped.FillCount())

	cp, ok := replayer.Checkpoint(scope)
	require.True(t, ok)
	require.Equal(t, scoped.Checkpoint.LastAuditID, cp.LastAuditID)
	require.Equal(t, scoped.Checkpoint.LastEventID, cp.LastEventID)
	require.NotEmpty(t, cp.SnapshotHash)

	// Nothing new for this scope → incremental restores snapshot; cursor holds.
	again, err := replayer.ReplayIncremental(scope)
	require.NoError(t, err)
	require.Equal(t, scoped.Order, again.Order)
	require.Equal(t, scoped.FillQty, again.FillQty)
	require.Equal(t, cp.LastAuditID, again.Checkpoint.LastAuditID)
}
