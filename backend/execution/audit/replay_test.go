package audit_test

import (
	"testing"
	"time"

	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

func replaySpecHash() string {
	return audit.SpecFingerprint("sz000501", "buy", 10, 100)
}

func replayAttempt(specHash string) audit.Event {
	return audit.BuildSubmitAttempt(
		audit.BackendRealStub, "req-r1", "cid-r1", "7", "1", specHash,
		"sz000501", "buy", 10, 100, time.Now(),
	)
}

func replayFill(specHash, execID string, qty int64, price float64, cum, leaves int64, oms, brokerStatus string) audit.Event {
	return audit.BuildFillApplied(
		audit.BackendRealStub, "req-r1", "cid-r1", "7", "1", specHash,
		"rpt-"+execID, execID, "report_received:rpt-"+execID,
		qty, price, cum, price, leaves, qty,
		oms, brokerStatus, time.Now(),
	)
}

func TestReplayEvents_EventIDIdempotent(t *testing.T) {
	spec := replaySpecHash()
	fill := replayFill(spec, "E1", 100, 10, 100, 0, "filled", "filled")
	res := audit.ReplayEvents([]audit.Event{replayAttempt(spec), fill, fill})

	require.Equal(t, 1, res.FillCount())
	require.Equal(t, int64(100), res.FillQty)
	require.Equal(t, 1, res.EventsSkipped)
	require.False(t, res.HasDivergence())
}

func TestReplayEvents_ExecIDIdempotentAcrossEventIDs(t *testing.T) {
	spec := replaySpecHash()
	first := replayFill(spec, "E9", 40, 10, 40, 60, "pending", "partially_filled")
	second := first
	second.EventID = "fill_applied:E9:retransmit"
	second.ReportID = "rpt-retransmit"

	res := audit.ReplayEvents([]audit.Event{replayAttempt(spec), first, second})
	require.Equal(t, 1, res.FillCount())
	require.Equal(t, int64(40), res.FillQty)
	require.Equal(t, int64(40), res.PositionDelta)
}

func TestReplayEvents_SpecHashMismatchReportsDivergence(t *testing.T) {
	spec := replaySpecHash()
	tampered := replayFill("deadbeefdeadbeefdeadbeefdeadbeef", "E2", 100, 10, 100, 0, "filled", "filled")

	res := audit.ReplayEvents([]audit.Event{replayAttempt(spec), tampered})
	require.True(t, res.HasDivergence())
	require.Equal(t, audit.DivergenceSpecHashMismatch, res.Divergences[0].Kind)
	require.Equal(t, spec, res.SpecHash, "baseline spec_hash must stay the first observed Frozen Spec value")
}

func TestReplayEvents_MissingEventIDReportsDivergence(t *testing.T) {
	spec := replaySpecHash()
	broken := replayAttempt(spec)
	broken.EventID = ""

	res := audit.ReplayEvents([]audit.Event{broken})
	require.True(t, res.HasDivergence())
	require.Equal(t, audit.DivergenceMissingEventID, res.Divergences[0].Kind)
	require.Equal(t, 0, res.EventsApplied)
}

func TestMemoryCheckpointStore_SaveLoad(t *testing.T) {
	store := audit.NewMemoryCheckpointStore()
	_, ok := store.Load("scope-a")
	require.False(t, ok)

	require.NoError(t, store.Save(audit.Checkpoint{Scope: "scope-a", LastAuditID: 12, LastEventID: "e12"}))
	cp, ok := store.Load("scope-a")
	require.True(t, ok)
	require.Equal(t, uint(12), cp.LastAuditID)
	require.Equal(t, "e12", cp.LastEventID)
}
