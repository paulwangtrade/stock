package execution

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPhase2E_Metrics_SubmitIncrementsPending(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)

	before := real.GetMetrics()
	require.Equal(t, int64(0), before.OrdersPending)

	_, err = real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	m := real.GetMetrics()
	require.Equal(t, before.OrdersPending+1, m.OrdersPending)
	require.Equal(t, int64(0), m.OrdersAccepted)
	require.Equal(t, int64(0), m.OrdersRejected)
	require.Equal(t, int64(0), m.FillsBuy)
	require.Equal(t, float64(0), m.FillNotional)
}

func TestPhase2E_Metrics_FillNotionalBySide(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	buy, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "m-ack-buy", ClientOrderID: buy.ClientOrderID,
		OccurredAt: time.Now(),
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "m-t-buy", ExecID: "M-BUY-1",
		ClientOrderID: buy.ClientOrderID, LastQty: 40, LastPrice: 10.5,
	}))

	sell, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "sell", Price: 20, Volume: 50,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "m-ack-sell", ClientOrderID: sell.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "m-t-sell", ExecID: "M-SELL-1",
		ClientOrderID: sell.ClientOrderID, LastQty: 50, LastPrice: 19.2,
	}))

	m := real.GetMetrics()
	require.Equal(t, int64(2), m.OrdersPending)
	require.Equal(t, int64(2), m.OrdersAccepted)
	require.Equal(t, int64(1), m.FillsBuy)
	require.Equal(t, int64(1), m.FillsSell)
	// 40*10.5 + 50*19.2 = 420 + 960 = 1380
	require.InDelta(t, 1380.0, m.FillNotional, 1e-9)
}

func TestPhase2E_Metrics_AckLatencyAndCancelReject(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	ackAt := order.CreatedAt.Add(50 * time.Millisecond)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "m-lat-ack", ClientOrderID: order.ClientOrderID,
		OccurredAt: ackAt,
	}))

	m := real.GetMetrics()
	require.Equal(t, int64(1), m.OrdersAccepted)
	require.Equal(t, int64(1), m.AckSamples)
	require.GreaterOrEqual(t, m.AvgSubmitToAck, 50*time.Millisecond)
	require.Less(t, m.AvgSubmitToAck, 200*time.Millisecond)

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "m-cx", ClientOrderID: order.ClientOrderID,
	}))
	m = real.GetMetrics()
	require.Equal(t, int64(1), m.Cancels)

	rej, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000004", Side: "buy", Price: 10, Volume: 10,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "m-rej", ClientOrderID: rej.ClientOrderID,
		RejectCode: "TEST",
	}))
	m = real.GetMetrics()
	require.Equal(t, int64(1), m.OrdersRejected)
	require.Equal(t, int64(2), m.OrdersPending)
}
