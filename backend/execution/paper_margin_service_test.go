package execution

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/risk"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) (*PaperMarginService, *gorm.DB) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigratePaperMargin(database); err != nil {
		t.Fatal(err)
	}
	return NewPaperMarginService(database), database
}

func TestSubmitMarginBuyAndRejectWithoutCredit(t *testing.T) {
	service, database := newTestService(t)
	ctx := context.Background()
	account, _, err := service.ensureAccount(database, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.UpsertBorrowPool(ctx, data.PaperBorrowPool{
		StockCode:             "SZ000001",
		AvailableQuantity:     1000,
		CollateralRate:        0.7,
		FinanceMarginRatio:    0.5,
		SecuritiesMarginRatio: 0.5,
		Enabled:               true,
	})
	if err != nil {
		t.Fatal(err)
	}

	order, decision, err := service.Submit(ctx, SubmitRequest{
		AccountID: account.ID, Kind: data.PaperMarginOrderMarginBuy,
		StockCode: "SZ000001", Price: 10, Volume: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || order.Status != "filled" {
		t.Fatalf("unexpected order/decision: %+v %+v", order, decision)
	}
	var debt data.PaperFinanceLiability
	if err = database.Where("account_id = ? AND stock_code = ?", account.ID, "SZ000001").First(&debt).Error; err != nil {
		t.Fatal(err)
	}
	if debt.Principal != 1000 || debt.Quantity != 100 {
		t.Fatalf("unexpected financing debt: %+v", debt)
	}

	var margin data.PaperMarginAccount
	if err = database.Where("account_id = ?", account.ID).First(&margin).Error; err != nil {
		t.Fatal(err)
	}
	margin.FinanceCreditLimit = 1000
	if err = database.Save(&margin).Error; err != nil {
		t.Fatal(err)
	}
	rejected, rejectedDecision, err := service.Submit(ctx, SubmitRequest{
		AccountID: account.ID, Kind: data.PaperMarginOrderMarginBuy,
		StockCode: "SZ000001", Price: 10, Volume: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "rejected" || rejectedDecision.Code != risk.ReasonFinanceCreditExceeded {
		t.Fatalf("unexpected rejection: %+v %+v", rejected, rejectedDecision)
	}
}

func TestAccrueInterestACT365(t *testing.T) {
	service, database := newTestService(t)
	ctx := context.Background()
	account, margin, err := service.ensureAccount(database, 0)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(10 * 24 * time.Hour)
	margin.LastAccruedAt = &from
	margin.FinanceAnnualRate = 0.08
	margin.SecuritiesAnnualRate = 0.10
	if err = database.Save(margin).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Create(&data.PaperFinanceLiability{
		AccountID: account.ID, StockCode: "SZ000001", Principal: 36500,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Create(&data.PaperSecuritiesLiability{
		AccountID: account.ID, StockCode: "SZ000002", Quantity: 100, AvgPrice: 10,
	}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := service.AccrueInterest(ctx, account.ID, to)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result.FinanceInterest-80) > 1e-9 {
		t.Fatalf("finance interest = %v, want 80", result.FinanceInterest)
	}
	wantFee := 1000.0 * 0.10 * 10 / 365
	if math.Abs(result.SecuritiesFee-wantFee) > 1e-9 {
		t.Fatalf("securities fee = %v, want %v", result.SecuritiesFee, wantFee)
	}
}

func TestSnapshotMarginAvailableAndExposure(t *testing.T) {
	service, database := newTestService(t)
	account, _, err := service.ensureAccount(database, 0)
	if err != nil {
		t.Fatal(err)
	}
	account.Cash = 100
	if err = database.Save(account).Error; err != nil {
		t.Fatal(err)
	}
	records := []any{
		&data.PaperPosition{AccountID: account.ID, StockCode: "SZ000001", Volume: 100, AvgCost: 9},
		&data.PaperFinanceLiability{AccountID: account.ID, StockCode: "SZ000001", Principal: 200},
		&data.PaperSecuritiesLiability{AccountID: account.ID, StockCode: "SZ000002", Quantity: 50, AvgPrice: 7},
		&data.PaperBorrowPool{StockCode: "SZ000001", CollateralRate: 0.7, FinanceMarginRatio: 0.5, SecuritiesMarginRatio: 0.5, Enabled: true},
		&data.PaperBorrowPool{StockCode: "SZ000002", CollateralRate: 0.7, FinanceMarginRatio: 0.5, SecuritiesMarginRatio: 0.6, Enabled: true},
	}
	for _, record := range records {
		if err = database.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := service.Snapshot(context.Background(), account.ID, []PositionMark{
		{StockCode: "SZ000001", Price: 10},
		{StockCode: "SZ000002", Price: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Metrics.MarginAvailable != 460 {
		t.Fatalf("margin available = %v, want 460", snapshot.Metrics.MarginAvailable)
	}
	if snapshot.Metrics.NetExposure != 600 || snapshot.Metrics.GrossExposure != 1400 {
		t.Fatalf("unexpected exposure: %+v", snapshot.Metrics)
	}
}

func TestMaintenanceRiskScan(t *testing.T) {
	service, database := newTestService(t)
	ctx := context.Background()
	account, margin, err := service.ensureAccount(database, 0)
	if err != nil {
		t.Fatal(err)
	}
	account.Cash = 100
	margin.WarningRatio = 1.5
	margin.CloseoutRatio = 1.3
	if err = database.Save(account).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Save(margin).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Create(&data.PaperFinanceLiability{
		AccountID: account.ID, StockCode: "SZ000001", Principal: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	events, err := service.ScanRisk(ctx, account.ID, nil, time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ReasonCode != string(risk.ReasonCloseoutTriggered) {
		t.Fatalf("unexpected events: %+v", events)
	}
}
