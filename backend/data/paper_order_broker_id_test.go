package data

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPaperOrderBrokerIDs_SubmitLeavesBrokerIDsEmpty(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		StockName: "平安银行",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotEmpty(t, order.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, order.ExecBackend)
	require.Empty(t, order.BrokerOrderID, "Paper 不得伪造 broker_order_id")
	require.Empty(t, order.ExternalOrderID, "Paper 不得伪造 external_order_id")
	require.Equal(t, PaperBrokerStatusPaper, order.BrokerStatus)
	require.Equal(t, PaperOrderStatusFilled, order.Status)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.NotEmpty(t, dbOrder.ClientOrderID)
	require.Empty(t, dbOrder.BrokerOrderID)
	require.Empty(t, dbOrder.ExternalOrderID)
	require.Equal(t, PaperBrokerStatusPaper, dbOrder.BrokerStatus)
}

func TestPaperOrderBrokerIDs_TradeOrderMapping(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh600000",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)

	mapped := toTradeOrder(*order)
	require.Equal(t, order.ClientOrderID, mapped.ClientOrderID)
	require.Equal(t, order.ExecBackend, mapped.ExecBackend)
	require.Equal(t, order.BrokerOrderID, mapped.BrokerOrderID)
	require.Equal(t, order.ExternalOrderID, mapped.ExternalOrderID)
	require.Equal(t, order.BrokerStatus, mapped.BrokerStatus)
	require.Empty(t, mapped.BrokerOrderID)
	require.Empty(t, mapped.ExternalOrderID)
	require.Equal(t, PaperBrokerStatusPaper, mapped.BrokerStatus)
}

func TestPaperOrderBrokerIDs_EventPayloadIncludesFields(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000002",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)

	var events []PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ?", order.ID).Order("id ASC").Find(&events).Error)
	require.GreaterOrEqual(t, len(events), 2)

	for _, ev := range events {
		if ev.EventType != PaperOrderEventSubmitted && ev.EventType != PaperOrderEventFilled {
			continue
		}
		var payload paperOrderEventPayload
		require.NoError(t, json.Unmarshal([]byte(ev.PayloadJSON), &payload))
		require.Equal(t, order.ClientOrderID, payload.ClientOrderID)
		require.Equal(t, PaperExecBackendPaper, payload.ExecBackend)
		require.Empty(t, payload.BrokerOrderID)
		require.Empty(t, payload.ExternalOrderID)
		// 字段必须出现在 JSON 中（即使为空字符串）
		require.Contains(t, ev.PayloadJSON, `"clientOrderId"`)
		require.Contains(t, ev.PayloadJSON, `"execBackend"`)
		require.Contains(t, ev.PayloadJSON, `"brokerOrderId"`)
		require.Contains(t, ev.PayloadJSON, `"externalOrderId"`)
	}
}

func TestMigratePaperTrading_BackfillsBrokerStatusForLegacyRows(t *testing.T) {
	dsn := fmt.Sprintf("file:phase2b_pr2_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	// 模拟旧表：无 broker_* 列
	require.NoError(t, testDB.Exec(`
		CREATE TABLE paper_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER,
			stock_code TEXT,
			stock_name TEXT,
			side TEXT,
			status TEXT,
			price REAL,
			volume INTEGER,
			client_order_id TEXT,
			exec_backend TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error)
	require.NoError(t, testDB.Exec(`
		INSERT INTO paper_orders (account_id, stock_code, side, status, price, volume, client_order_id, exec_backend, created_at, updated_at)
		VALUES (1, 'sz000001', 'buy', 'filled', 10, 100, 'paper_legacy_cid', 'paper', ?, ?)
	`, time.Now(), time.Now()).Error)

	require.NoError(t, MigratePaperTrading(testDB))

	var order PaperOrder
	require.NoError(t, testDB.First(&order).Error)
	require.Equal(t, "paper_legacy_cid", order.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, order.ExecBackend)
	require.Empty(t, order.BrokerOrderID)
	require.Empty(t, order.ExternalOrderID)
	require.Equal(t, PaperBrokerStatusPaper, order.BrokerStatus)
}
