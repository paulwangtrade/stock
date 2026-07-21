package data

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func seedHealthOrder(t *testing.T, o PaperOrder) PaperOrder {
	t.Helper()
	now := time.Now()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = o.CreatedAt
	}
	if o.ClientOrderID == "" {
		o.ClientOrderID = newPaperClientOrderID()
	}
	if o.ExecBackend == "" {
		o.ExecBackend = PaperExecBackendPaper
	}
	require.NoError(t, db.Dao.Create(&o).Error)
	require.NotZero(t, o.ID)
	return o
}

func TestGetPaperOrderHealth_FilledCounts(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000001", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 100,
		FilledPrice: 10, FilledVol: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000002", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 11, Volume: 200,
		FilledPrice: 11, FilledVol: 200, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 2, rep.Counts.Submitted)
	require.Equal(t, 2, rep.Counts.Filled)
	require.Equal(t, 0, rep.Counts.Rejected)
	require.Equal(t, 0, rep.Counts.Pending)
	require.InDelta(t, 1.0, rep.Rates.FillRate, 1e-9)
	require.InDelta(t, 0.0, rep.Rates.RejectRate, 1e-9)
	require.InDelta(t, 0.0, rep.Rates.PendingRatio, 1e-9)
}

func TestGetPaperOrderHealth_RejectBreakdown(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "a", Side: "buy", Status: PaperOrderStatusRejected,
		RejectCode: PaperOrderRejectCashInsufficient, RejectReason: "cash",
		Price: 1, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "b", Side: "buy", Status: PaperOrderStatusRejected,
		RejectCode: PaperOrderRejectCashInsufficient, RejectReason: "cash2",
		Price: 1, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "c", Side: "sell", Status: PaperOrderStatusRejected,
		RejectCode: PaperOrderRejectPositionInsufficient, RejectReason: "pos",
		Price: 1, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "d", Side: "buy", Status: PaperOrderStatusRejected,
		RejectCode: "", RejectReason: "missing code",
		Price: 1, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 4, rep.Counts.Rejected)

	byCode := map[string]PaperOrderRejectStat{}
	for _, s := range rep.RejectBreakdown {
		byCode[s.Code] = s
	}
	require.Equal(t, 2, byCode[PaperOrderRejectCashInsufficient].Count)
	require.InDelta(t, 0.5, byCode[PaperOrderRejectCashInsufficient].Percentage, 1e-9)
	require.Equal(t, 1, byCode[PaperOrderRejectPositionInsufficient].Count)
	require.Equal(t, 1, byCode[PaperOrderRejectEmptyCode].Count)
}

func TestGetPaperOrderHealth_IOCOrphanDetected(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sh600000", Side: "buy",
		Status: PaperOrderStatusPending, ExecMode: PaperOrderExecModeIOCAutofill,
		Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 1, rep.Counts.Pending)
	require.Len(t, rep.Orphans, 1)
	require.Equal(t, o.ID, rep.Orphans[0].OrderID)
	require.Equal(t, PaperOrderOrphanIOC, rep.Orphans[0].OrphanKind)
	require.Equal(t, "sh600000", rep.Orphans[0].Symbol)
	require.NotEmpty(t, rep.Orphans[0].Hint)
}

func TestGetPaperOrderHealth_RestingPendingNotOrphan(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000001", Side: "buy",
		Status: PaperOrderStatusPending, ExecMode: PaperOrderExecModeResting,
		Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 1, rep.Counts.Pending)
	require.Empty(t, rep.Orphans)
	require.InDelta(t, 1.0, rep.Rates.PendingRatio, 1e-9)
}

func TestGetPaperOrderHealth_Rates(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	// 2 filled, 2 rejected, 1 pending resting => submitted=5
	for i := 0; i < 2; i++ {
		seedHealthOrder(t, PaperOrder{
			AccountID: 1, StockCode: "f", Side: "buy", Status: PaperOrderStatusFilled,
			Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
		})
	}
	for i := 0; i < 2; i++ {
		seedHealthOrder(t, PaperOrder{
			AccountID: 1, StockCode: "r", Side: "buy", Status: PaperOrderStatusRejected,
			RejectCode: PaperOrderRejectInternal, Price: 10, Volume: 100,
			CreatedAt: today, UpdatedAt: today,
		})
	}
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "p", Side: "buy", Status: PaperOrderStatusPending,
		ExecMode: PaperOrderExecModeResting, Price: 10, Volume: 100,
		CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 5, rep.Counts.Submitted)
	require.Equal(t, 2, rep.Counts.Filled)
	require.Equal(t, 2, rep.Counts.Rejected)
	require.Equal(t, 1, rep.Counts.Pending)
	require.InDelta(t, 0.5, rep.Rates.FillRate, 1e-9)
	require.InDelta(t, 0.5, rep.Rates.RejectRate, 1e-9)
	require.InDelta(t, 0.2, rep.Rates.PendingRatio, 1e-9)
	require.Empty(t, rep.Orphans)
}

func TestClassifyPaperOrderOrphan_ProcessingStuck(t *testing.T) {
	now := time.Now()
	o := PaperOrder{
		ID: 9, StockCode: "x", Side: "buy", Status: PaperOrderStatusProcessing,
		CreatedAt: now.Add(-2 * time.Minute),
	}
	orphan, ok := classifyPaperOrderOrphan(o, now)
	require.True(t, ok)
	require.Equal(t, PaperOrderOrphanProcessing, orphan.OrphanKind)
}

func TestClassifyPaperOrderOrphan_PendingTimeout(t *testing.T) {
	now := time.Now()
	o := PaperOrder{
		ID: 10, StockCode: "y", Side: "buy", Status: PaperOrderStatusPending,
		ExecMode: "", CreatedAt: now.Add(-25 * time.Hour),
	}
	orphan, ok := classifyPaperOrderOrphan(o, now)
	require.True(t, ok)
	require.Equal(t, PaperOrderOrphanPendingTO, orphan.OrphanKind)
}

func seedOrderEvent(t *testing.T, orderID uint, eventType string) {
	t.Helper()
	require.NoError(t, db.Dao.Create(&PaperOrderEvent{
		OrderID: orderID, EventType: eventType, CreatedAt: time.Now(),
		PayloadJSON: fmt.Sprintf(`{"orderId":%d,"symbol":"sz000001"}`, orderID),
	}).Error)
}

func seedPaperFill(t *testing.T, orderID uint, code string) {
	t.Helper()
	require.NoError(t, db.Dao.Create(&PaperFill{
		AccountID: 1, OrderID: orderID, StockCode: code, Side: "buy",
		Price: 10, Volume: 100, FilledAt: time.Now(),
	}).Error)
}

func gapIssues(gaps []PaperOrderLifecycleGap) []string {
	out := make([]string, 0, len(gaps))
	for _, g := range gaps {
		out = append(out, g.Issue)
	}
	return out
}

func TestGetPaperOrderHealth_CancelledCounts(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000010", Side: "buy",
		Status: PaperOrderStatusCancelled, Price: 10, Volume: 100,
		CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000011", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 100,
		FilledPrice: 10, FilledVol: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000012", Side: "buy",
		Status: PaperOrderStatusRejected, RejectCode: PaperOrderRejectCashInsufficient,
		Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 1, rep.Counts.Cancelled)
	require.Equal(t, 1, rep.Counts.Filled)
	require.Equal(t, 1, rep.Counts.Rejected)
	require.Equal(t, 3, rep.Counts.Submitted)
}

func TestLifecycleGaps_NormalCancelledNoGap(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000013", Side: "buy",
		Status: PaperOrderStatusCancelled, Price: 10, Volume: 100,
		CreatedAt: today, UpdatedAt: today,
	})
	seedOrderEvent(t, o.ID, PaperOrderEventSubmitted)
	seedOrderEvent(t, o.ID, PaperOrderEventCancelled)

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Equal(t, 1, rep.Counts.Cancelled)
	require.Empty(t, rep.LifecycleGaps)
	require.Empty(t, rep.Orphans)
}

func TestLifecycleGaps_NormalFilledNoGap(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000001", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 100,
		FilledPrice: 10, FilledVol: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedOrderEvent(t, o.ID, PaperOrderEventSubmitted)
	seedOrderEvent(t, o.ID, PaperOrderEventFilled)
	seedPaperFill(t, o.ID, "sz000001")

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Empty(t, rep.LifecycleGaps)
}

func TestLifecycleGaps_FilledMissingEvent(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000001", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 100,
		FilledPrice: 10, FilledVol: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedPaperFill(t, o.ID, "sz000001")

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Contains(t, gapIssues(rep.LifecycleGaps), PaperOrderGapFilledMissingEvent)
}

func TestLifecycleGaps_FilledMissingPaperFill(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000002", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 100,
		FilledPrice: 10, FilledVol: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedOrderEvent(t, o.ID, PaperOrderEventFilled)

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Contains(t, gapIssues(rep.LifecycleGaps), PaperOrderGapFilledMissingFill)
}

func TestLifecycleGaps_RejectedMissingEvent(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000003", Side: "buy",
		Status: PaperOrderStatusRejected, RejectCode: PaperOrderRejectCashInsufficient,
		Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Contains(t, gapIssues(rep.LifecycleGaps), PaperOrderGapRejectedMissingEvent)
}

func TestLifecycleGaps_NormalPendingNoGap(t *testing.T) {
	setupPaperTradingTestDB(t)
	today := time.Now()
	o := seedHealthOrder(t, PaperOrder{
		AccountID: 1, StockCode: "sz000004", Side: "buy",
		Status: PaperOrderStatusPending, ExecMode: PaperOrderExecModeResting,
		Price: 10, Volume: 100, CreatedAt: today, UpdatedAt: today,
	})
	seedOrderEvent(t, o.ID, PaperOrderEventSubmitted)

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Empty(t, rep.LifecycleGaps)
	require.Empty(t, rep.Orphans)
}

func TestLifecycleGaps_OrphanEvent(t *testing.T) {
	setupPaperTradingTestDB(t)
	orphanID := uint(999001)
	require.NoError(t, db.Dao.Create(&PaperOrderEvent{
		OrderID: orphanID, EventType: PaperOrderEventSubmitted, CreatedAt: time.Now(),
		PayloadJSON: `{"orderId":999001,"symbol":"orphan"}`,
	}).Error)

	rep, err := GetPaperOrderHealth("")
	require.NoError(t, err)
	require.Contains(t, gapIssues(rep.LifecycleGaps), PaperOrderGapOrphanEvent)
	found := false
	for _, g := range rep.LifecycleGaps {
		if g.Issue == PaperOrderGapOrphanEvent && g.OrderID == orphanID {
			found = true
			require.Equal(t, "orphan", g.Symbol)
			require.NotEmpty(t, g.Detail)
		}
	}
	require.True(t, found)
}
