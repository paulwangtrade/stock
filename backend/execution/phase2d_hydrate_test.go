package execution

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func TestPhase2D_Hydrate_FullRecoveryWithFills(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real1)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p2d-ack", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2d-t1", ExecID: "P2D-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10.0,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "p2d-cx", ClientOrderID: order.ClientOrderID,
	}))

	midFills, err := real1.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, midFills, 1)
	require.Equal(t, int64(40), midFills[0].FillQty)
	require.InDelta(t, 10.0, midFills[0].FillPrice, 1e-9)

	// 模拟重启 hydrate
	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)

	byClient, err := real2.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, byClient.Status)
	require.Equal(t, data.BrokerStatusCancelled, byClient.BrokerStatus)
	require.Equal(t, int64(40), byClient.FilledVolume)
	require.Equal(t, int64(0), byClient.LeavesQuantity)
	require.InDelta(t, 10.0, byClient.FilledPrice, 1e-9)
	require.NotEmpty(t, byClient.BrokerOrderID)
	require.Equal(t, data.ExecBackendRealStub, byClient.ExecBackend)

	byBroker, err := real2.QueryOrder(context.Background(), byClient.BrokerOrderID)
	require.NoError(t, err)
	require.Equal(t, order.ClientOrderID, byBroker.ClientOrderID)

	fills, err := real2.ListFills(context.Background(), byClient.BrokerOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "P2D-E1", fills[0].ExecID)
	require.Equal(t, int64(40), fills[0].FillQty)
	require.InDelta(t, 10.0, fills[0].FillPrice, 1e-9)
	require.Equal(t, order.ID, fills[0].LocalOrderID)
}

func TestPhase2D_Hydrate_InconsistentFilledRejectsStart(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real1)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p2d-bad-ack", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2d-bad-t", ExecID: "P2D-BAD",
		ClientOrderID: order.ClientOrderID, LastQty: 100, LastPrice: 10,
	}))

	// 篡改 filled_volume，与 fills 不一致
	require.NoError(t, db.Dao.Model(&data.RealStubOrder{}).
		Where("local_order_id = ?", order.ID).
		Update("filled_volume", 50).Error)

	real2, err := NewRealBrokerPersistent()
	require.Error(t, err)
	require.Nil(t, real2)
	require.True(t, errors.Is(err, ErrRealStubHydrateInconsistent))
}

func TestPhase2D_Hydrate_OrphanFillRejectsStart(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	_, err = real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	require.NoError(t, db.Dao.Create(&data.RealStubFill{
		ClientOrderID: "orphan-cid",
		LocalOrderID:  "missing-local",
		ExecID:        "ORPHAN-EXEC",
		FillQty:       10,
		FillPrice:     1,
	}).Error)

	real2, err := NewRealBrokerPersistent()
	require.Error(t, err)
	require.Nil(t, real2)
	require.True(t, errors.Is(err, ErrRealStubHydrateInconsistent))
}

func TestPhase2D_ListFills_SyncAfterTradeInProcess(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000004", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p2d-sync-ack", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2d-sync-t1", ExecID: "SYNC1",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 9.5,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2d-sync-t2", ExecID: "SYNC2",
		ClientOrderID: order.ClientOrderID, LastQty: 70, LastPrice: 10.5,
	}))

	fills, err := real.ListFills(context.Background(), order.ID)
	require.NoError(t, err)
	require.Len(t, fills, 2)
	require.Equal(t, int64(30), fills[0].FillQty)
	require.Equal(t, int64(70), fills[1].FillQty)
	require.InDelta(t, 9.5, fills[0].FillPrice, 1e-9)
	require.InDelta(t, 10.5, fills[1].FillPrice, 1e-9)
}
