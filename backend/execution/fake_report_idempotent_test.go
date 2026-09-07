package execution

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func TestFakeReport_DuplicateAckNoReupdate(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	ack := BrokerAckReport{
		ReportID:      "ack-report-1",
		ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-1",
		OccurredAt:    time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, handler.OnBrokerAck(context.Background(), ack))
	got1, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	updated1 := got1.UpdatedAt

	ack.OccurredAt = time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC)
	require.NoError(t, handler.OnBrokerAck(context.Background(), ack))
	got2, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, updated1, got2.UpdatedAt, "重复 ACK 不得再次更新")
	require.Equal(t, "BRK-1", got2.BrokerOrderID)
	require.Equal(t, data.PaperOrderStatusPending, got2.Status)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubReportLedger{}).Where("report_id = ?", "ack-report-1").Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestFakeReport_DuplicateFillNoReprocess(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sh600000", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	fill := FilledReport{
		ReportID:      "fill-report-1",
		ExecID:        "EXEC-1",
		ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-F",
		FillPrice:     10.1,
		FillQty:       100,
	}
	require.NoError(t, handler.OnFilledReport(context.Background(), fill))
	got1, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got1.Status)
	filledAt := got1.FilledAt

	fill.FillPrice = 99 // 若重复处理会改价；幂等应忽略
	require.NoError(t, handler.OnFilledReport(context.Background(), fill))
	got2, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.InDelta(t, 10.1, got2.FilledPrice, 1e-9)
	require.Equal(t, filledAt, got2.FilledAt)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubReportLedger{}).Where("report_id = ?", "fill-report-1").Count(&n).Error)
	require.Equal(t, int64(1), n)
	var paperFills int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&paperFills).Error)
	require.Zero(t, paperFills)
}

func TestFakeReport_RestartRecoversQuery(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real1)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 12, Volume: 200,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnBrokerAck(context.Background(), BrokerAckReport{
		ReportID: "ack-r", ClientOrderID: order.ClientOrderID, BrokerOrderID: "BRK-R",
	}))
	require.NoError(t, handler.OnFilledReport(context.Background(), FilledReport{
		ReportID: "fill-r", ExecID: "EX-R", ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-R", FillPrice: 12, FillQty: 200,
	}))

	// 模拟重启：新 broker 从 DB hydrate
	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	got, err := real2.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, order.ClientOrderID, got.ClientOrderID)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, "BRK-R", got.BrokerOrderID)
	require.Equal(t, int64(200), got.FilledVolume)
	require.Equal(t, data.ExecBackendRealStub, got.ExecBackend)

	// 重复回报在重启后仍幂等
	h2 := NewFakeExecutionReportHandler(real2)
	require.NoError(t, h2.OnFilledReport(context.Background(), FilledReport{
		ReportID: "fill-r", ExecID: "EX-R", ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-R", FillPrice: 1, FillQty: 200,
	}))
	got2, err := real2.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.0, got2.FilledPrice, 1e-9)
}

func TestFakeReport_OnlyAffectsRealStubNotPaperOrders(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnFilledReport(context.Background(), FilledReport{
		ReportID: "f-only", ExecID: "E1", ClientOrderID: order.ClientOrderID,
		FillPrice: 10, FillQty: 100,
	}))

	var paperOrders, stubOrders int64
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&paperOrders).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubOrder{}).Count(&stubOrders).Error)
	require.Zero(t, paperOrders)
	require.Equal(t, int64(1), stubOrders)
}
