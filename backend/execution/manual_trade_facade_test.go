package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupManualFacadeTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:manual_facade_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureManualTradeIntentTables())
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func mustConfirmedManualIntent(t *testing.T, side string, patch *models.ManualTradeIntent) *models.ManualTradeIntent {
	t.Helper()
	repo := data.NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{
		Symbol:    "sh600036",
		StockName: "招商银行",
		Side:      side,
		Price:     10,
		Volume:    100,
		Reason:    "模拟执行台",
	}
	if patch != nil {
		if patch.Symbol != "" {
			intent.Symbol = patch.Symbol
		}
		if patch.Price > 0 {
			intent.Price = patch.Price
		}
		if patch.Volume > 0 {
			intent.Volume = patch.Volume
		}
		if patch.AccountID > 0 {
			intent.AccountID = patch.AccountID
		}
		if patch.OrderKind != "" {
			intent.OrderKind = patch.OrderKind
		}
	}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil))
	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	return got
}

func sellReadyLoader(cash float64, code string, sellable int64) preTradeSnapshotLoader {
	return func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{
			Cash: cash,
			Positions: map[string]preTradePosition{
				code: {Volume: sellable, Sellable: sellable},
			},
		}, nil
	}
}

func TestManualTradeFacade_ConfirmedBuyExecutes(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", nil)

	port := &stubPort{
		order: &broker.TradeOrder{
			ID: "101", ClientOrderID: "cli-buy-1", Status: data.PaperOrderStatusFilled,
			StockCode: intent.Symbol, StrategyTag: models.ManualTradeStrategyTag,
		},
	}
	svc := NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewManualTradeFacade(svc, nil)

	require.NoError(t, facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID))
	require.Equal(t, 1, port.calls)
	require.Equal(t, "buy", port.lastIntent.Side)
	require.Equal(t, models.ManualTradeStrategyTag, port.lastIntent.StrategyTag)
	require.Contains(t, port.lastIntent.Reason, "manual_source=paper_trading_panel")
	require.Contains(t, port.lastIntent.Reason, "order_kind=normal")
	require.NotContains(t, port.lastIntent.Reason, "research_source")
	require.NotContains(t, port.lastIntent.Reason, "snapshot_id")
	require.NotContains(t, port.lastIntent.Reason, "signal_tag")
}

func TestManualTradeFacade_ConfirmedSellExecutes(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "sell", nil)

	port := &stubPort{
		order: &broker.TradeOrder{
			ID: "102", ClientOrderID: "cli-sell-1", Status: data.PaperOrderStatusFilled,
			StockCode: intent.Symbol, Side: "sell",
		},
	}
	svc := NewExecutionService(port).withSnapshotLoader(sellReadyLoader(1_000_000, intent.Symbol, 500))
	facade := NewManualTradeFacade(svc, nil)

	require.NoError(t, facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID))
	require.Equal(t, 1, port.calls)
	require.Equal(t, "sell", port.lastIntent.Side)
	require.Equal(t, models.ManualTradeStrategyTag, port.lastIntent.StrategyTag)
}

func TestManualTradeFacade_DraftNotExecutable(t *testing.T) {
	setupManualFacadeTestDB(t)
	repo := data.NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{Symbol: "sz000001", Price: 10, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent))

	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)

	err := facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrManualTradeIntentNotExecutable))
	require.Equal(t, 0, port.calls)
}

func TestManualTradeFacade_RejectedNotExecutable(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", nil)
	repo := data.NewManualTradeIntentRepo()
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusRejected, &data.ManualTradeIntentStatusPatch{
		ErrorCode: "pre", ErrorMessage: "x",
	}))

	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)
	err := facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrManualTradeIntentNotExecutable))
	require.Equal(t, 0, port.calls)
}

func TestManualTradeFacade_SuccessWritesOrderIDs(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", nil)
	port := &stubPort{
		order: &broker.TradeOrder{ID: "501", ClientOrderID: "cli-m-501", Status: data.PaperOrderStatusFilled},
	}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)
	require.NoError(t, facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID))

	got, err := data.NewManualTradeIntentRepo().GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusSubmitted, got.Status)
	require.Equal(t, "cli-m-501", got.ClientOrderID)
	require.Equal(t, uint(501), got.OrderID)
}

func TestManualTradeFacade_FailureWritesErrorCode(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", nil)
	port := &stubPort{
		err: preTradeReject(data.PaperOrderRejectCashInsufficient, "cash insufficient from port"),
	}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)
	err := facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID)
	require.Error(t, err)
	require.Equal(t, 1, port.calls)

	got, err := data.NewManualTradeIntentRepo().GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusRejected, got.Status)
	require.Equal(t, data.PaperOrderRejectCashInsufficient, got.ErrorCode)
	require.Contains(t, got.ErrorMessage, "cash insufficient")
}

func TestManualTradeFacade_ReasonHasNoResearchFields(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", nil)
	port := &stubPort{order: &broker.TradeOrder{ID: "9", ClientOrderID: "c"}}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)
	require.NoError(t, facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID))
	require.NotContains(t, port.lastIntent.Reason, "candidate_snapshot")
	require.NotContains(t, port.lastIntent.Reason, "research_source")
	require.NotContains(t, port.lastIntent.Reason, "signal_score")
	require.NotContains(t, port.lastIntent.Reason, "signal_tag")
}

func TestManualTradeFacade_UnsupportedMarginKind(t *testing.T) {
	setupManualFacadeTestDB(t)
	intent := mustConfirmedManualIntent(t, "buy", &models.ManualTradeIntent{OrderKind: models.ManualTradeOrderKindMarginBuy})
	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	facade := NewManualTradeFacade(NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000)), nil)
	err := facade.ConfirmAndExecuteManualTrade(context.Background(), intent.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrUnsupportedManualTradeOrderKind))
	require.Equal(t, 0, port.calls)
}

func TestManualTradeFacade_SourceHasNoSubmitPaperOrder(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "manual_trade_facade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)
	require.NotContains(t, body, "SubmitPaperOrder")
	require.Contains(t, body, "ExecutePlanItem(")
}
