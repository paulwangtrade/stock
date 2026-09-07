package audit_test

import (
	"testing"
	"time"

	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

func TestEmitter_IdempotentByEventID(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewEmitter(mem)

	ev := audit.BuildSubmitAttempt(
		audit.BackendRealStub, "real_x", "real_x", "1", "1",
		audit.SpecFingerprint("sz000001", "buy", 10, 100),
		"sz000001", "buy", 10, 100, time.Now(),
	)
	ok, err := em.Emit(ev)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = em.Emit(ev)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, 1, mem.Len())
	require.True(t, em.Seen(ev.EventID))
}

func TestSpecFingerprint_StableAndIndependentOfFillPrice(t *testing.T) {
	a := audit.SpecFingerprint("sz000001", "buy", 10.0, 100)
	b := audit.SpecFingerprint("sz000001", "BUY", 10.0, 100)
	require.Equal(t, a, b)
	require.Len(t, a, 32)
	// Fill price must not be used in fingerprint — callers pass Spec limit only.
	require.NotEqual(t, a, audit.SpecFingerprint("sz000001", "buy", 10.5, 100))
}

func TestEventID_StableKeys(t *testing.T) {
	require.Equal(t, "submit_attempt:real_1", audit.IDSubmitAttempt("real_1"))
	require.Equal(t, "submit_result:real_1:accepted", audit.IDSubmitResult("real_1", "accepted"))
	require.Equal(t, "report_received:r1", audit.IDReportReceived("r1"))
	require.Equal(t, "fill_applied:e1", audit.IDFillApplied("e1"))
	require.Equal(t, "order_terminal:9:filled:r1", audit.IDOrderTerminal("9", "filled", "r1"))
}

func TestMultiSink_MemoryAndLog(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewDefaultEmitter(mem)
	ev := audit.NewBase(audit.TypeSubmitAccepted, audit.SourceExecution, audit.BackendRealStub, "req", "hash", time.Now())
	ev.EventID = "test-multi-1"
	ok, err := em.Emit(ev)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, mem.Len())
}
