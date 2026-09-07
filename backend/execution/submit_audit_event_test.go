package execution

import (
	"context"
	"encoding/json"
	"errors"
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

// Phase6.5.8.2.1 — Submit / Broker Response audit events via AuditSink (memory+db).
// Does not change OMS mapping / Freeze / Approve / Intent.

type failingAuditSink struct{}

func (failingAuditSink) Write(audit.Event) error {
	return errors.New("audit sink forced failure")
}

type panickingAuditSink struct{}

func (panickingAuditSink) Write(audit.Event) error {
	panic("audit sink panic")
}

func setupSubmitAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:submit_audit_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	return database
}

func attachSubmitAudit(t *testing.T, real *RealBroker) (*audit.MemorySink, *gorm.DB) {
	t.Helper()
	database := setupSubmitAuditDB(t)
	mem := audit.NewMemoryAuditSink()
	real.SetAuditEmitter(audit.NewPersistenceEmitter(mem, database))
	return mem, database
}

func findEvent(t *testing.T, events []audit.Event, typ string) audit.Event {
	t.Helper()
	for _, e := range events {
		if e.EventType == typ {
			return e
		}
	}
	t.Fatalf("event type %s not found", typ)
	return audit.Event{}
}

func TestSubmitAudit_AcceptedEvent(t *testing.T) {
	real := NewRealBroker()
	mem, database := attachSubmitAudit(t, real)
	specHash := audit.SpecFingerprint("sz000301", "buy", 12.3, 100)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000301", Side: "buy",
		Price: 12.3, Volume: 100, SpecHash: specHash,
	})
	require.NoError(t, err)
	require.NotNil(t, order)

	events := mem.Events()
	require.Equal(t, []string{audit.TypeSubmitAttempt, audit.TypeSubmitAccepted}, auditEventTypes(mem))

	attempt := events[0]
	accepted := events[1]
	require.Equal(t, audit.IDSubmitAttempt(order.ClientOrderID), attempt.EventID)
	require.Equal(t, audit.IDSubmitResult(order.ClientOrderID, SubmitOutcomeAccepted), accepted.EventID)
	require.Equal(t, specHash, attempt.SpecHash)
	require.Equal(t, specHash, accepted.SpecHash)
	require.Equal(t, order.ClientOrderID, accepted.Payload["broker_request_id"])
	require.Equal(t, SubmitOutcomeAccepted, accepted.Payload["submit_outcome"])
	require.NotNil(t, accepted.Payload["response"])

	var rows []models.AuditEvent
	require.NoError(t, database.Order("id asc").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, audit.TypeSubmitAccepted, rows[1].EventType)

	var decoded audit.Event
	require.NoError(t, json.Unmarshal([]byte(rows[1].PayloadJSON), &decoded))
	require.Equal(t, order.ClientOrderID, decoded.Payload["broker_request_id"])
	require.Equal(t, SubmitOutcomeAccepted, decoded.Payload["submit_outcome"])
	require.Contains(t, decoded.Payload, "response")
}

func TestSubmitAudit_RejectedEvent(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: "rejected", Message: "risk"},
	}
	real := NewRealBrokerWithAdapter(fake)
	mem, database := attachSubmitAudit(t, real)
	specHash := audit.SpecFingerprint("sz000302", "buy", 10, 50)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000302", Side: "buy", Price: 10, Volume: 50, SpecHash: specHash,
	})
	require.Error(t, err)
	require.NotNil(t, order)

	rejected := findEvent(t, mem.Events(), audit.TypeSubmitRejected)
	require.Equal(t, SubmitOutcomeRejected, rejected.Payload["submit_outcome"])
	require.Equal(t, order.ClientOrderID, rejected.Payload["broker_request_id"])
	require.Equal(t, specHash, rejected.SpecHash)
	resp, ok := rejected.Payload["response"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rejected", resp["status"])
	require.Equal(t, "risk", resp["message"])

	var n int64
	require.NoError(t, database.Model(&models.AuditEvent{}).
		Where("event_type = ?", audit.TypeSubmitRejected).Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestSubmitAudit_TimeoutAndUnknownEvents(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		outcome string
	}{
		{name: "timeout", status: "timeout", outcome: SubmitOutcomeTimeout},
		{name: "unknown", status: "unknown", outcome: SubmitOutcomeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &programmableAdapter{
				resp: &broker.SubmitResponse{Status: tc.status, Message: tc.name},
			}
			real := NewRealBrokerWithAdapter(fake)
			mem, database := attachSubmitAudit(t, real)
			specHash := audit.SpecFingerprint("sz000303", "buy", 9, 30)

			order, err := real.Submit(context.Background(), SubmitIntent{
				StockCode: "sz000303", Side: "buy", Price: 9, Volume: 30, SpecHash: specHash,
			})
			require.Error(t, err)
			require.NotNil(t, order)
			require.Equal(t, data.PaperOrderStatusPending, order.Status)

			timeoutEv := findEvent(t, mem.Events(), audit.TypeSubmitTimeout)
			require.Equal(t, tc.outcome, timeoutEv.Payload["submit_outcome"])
			require.Equal(t, order.ClientOrderID, timeoutEv.Payload["broker_request_id"])
			require.Equal(t, true, timeoutEv.Payload["reconciliation_required"])
			require.Equal(t, specHash, timeoutEv.SpecHash)
			require.Contains(t, timeoutEv.Payload, "response")

			var n int64
			require.NoError(t, database.Model(&models.AuditEvent{}).
				Where("event_id = ?", audit.IDSubmitResult(order.ClientOrderID, tc.outcome)).
				Count(&n).Error)
			require.Equal(t, int64(1), n)
		})
	}
}

func TestSubmitAudit_DuplicateEventID(t *testing.T) {
	real := NewRealBroker()
	mem, database := attachSubmitAudit(t, real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000304", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000304", "buy", 10, 100),
	})
	require.NoError(t, err)

	beforeMem := mem.Len()
	var beforeDB int64
	require.NoError(t, database.Model(&models.AuditEvent{}).Count(&beforeDB).Error)

	// Re-emit same Submit result event_id through the same emitter path.
	em := audit.NewPersistenceEmitter(mem, database)
	real.SetAuditEmitter(em)
	dup := audit.BuildSubmitResult(
		audit.TypeSubmitAccepted, data.ExecBackendRealStub,
		order.ClientOrderID, order.ClientOrderID, order.ID, order.AccountID,
		audit.SpecFingerprint("sz000304", "buy", 10, 100),
		SubmitOutcomeAccepted, order.Status, order.BrokerStatus,
		time.Now(), time.Now(), "", "",
		map[string]any{"status": "pending"},
	)
	// Force same event_id as first accepted event and write via DB sink directly + emitter.
	require.NoError(t, audit.NewDBAuditSink(database).Write(dup))
	ok, err := em.Emit(dup)
	require.NoError(t, err)
	require.True(t, ok) // new emitter seen-map; DB still idempotent

	var afterDB int64
	require.NoError(t, database.Model(&models.AuditEvent{}).Count(&afterDB).Error)
	require.Equal(t, beforeDB, afterDB, "duplicate event_id must not create a second DB row")

	var n int64
	require.NoError(t, database.Model(&models.AuditEvent{}).
		Where("event_id = ?", dup.EventID).Count(&n).Error)
	require.Equal(t, int64(1), n)
	_ = beforeMem
}

func TestSubmitAudit_SpecHashImmutable(t *testing.T) {
	real := NewRealBroker()
	mem, _ := attachSubmitAudit(t, real)
	specHash := audit.SpecFingerprint("sz000305", "buy", 11.1, 200)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000305", Side: "buy", Price: 11.1, Volume: 200, SpecHash: specHash,
	})
	require.NoError(t, err)

	for _, e := range mem.Events() {
		require.Equal(t, specHash, e.SpecHash)
		require.Equal(t, audit.SpecHashVersion, e.SpecHashVersion)
	}
	// Frozen Spec projections on order must remain submit inputs (not mutated by audit).
	require.InDelta(t, 11.1, order.Price, 1e-9)
	require.Equal(t, int64(200), order.Volume)
	auditRec, ok := real.LastSubmitAudit(order.ClientOrderID)
	require.True(t, ok)
	require.Equal(t, specHash, auditRec.SpecHash)
}

func TestSubmitAudit_WriteFailureDoesNotAffectSubmit(t *testing.T) {
	real := NewRealBroker()
	real.SetAuditEmitter(audit.NewEmitter(failingAuditSink{}))

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000306", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000306", "buy", 10, 100),
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)

	real2 := NewRealBroker()
	real2.SetAuditEmitter(audit.NewEmitter(panickingAuditSink{}))
	order2, err := real2.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000307", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000307", "buy", 10, 100),
	})
	require.NoError(t, err)
	require.NotNil(t, order2)
}
