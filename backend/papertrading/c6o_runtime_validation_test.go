package papertrading_test

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

// Phase10-C.6-O: runtime validation — equity / ETF / CB fills + settlement invariants.
func TestPhase10C6O_TradingRule_RuntimeValidation(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	const (
		tradeDate   = "2026-08-14"
		initialCash = 1_000_000.0
		eqCode      = "sh600519"
		etfCode     = "sh510300"
		cbCode      = "sh113052"
		eqPx        = 100.0
		etfPx       = 4.0
		cbPx        = 110.0
		eqQty       = int64(100)
		etfQty      = int64(1000)
		cbQty       = int64(10)
	)

	items := []models.TradePlanItem{
		buyItem(eqCode, "贵州茅台", eqQty),
		buyItem(etfCode, "沪深300ETF", etfQty),
		buyItem(cbCode, "示例转债", cbQty),
	}
	// Distinct limit prices unused for fill; open prices come from provider.
	for i := range items {
		items[i].LimitPrice = 1
	}

	plan := seedFrozenPlan(t, tradeDate, items)
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
		eqCode:  {Open: eqPx, LimitUp: eqPx * 1.1},
		etfCode: {Open: etfPx},
		cbCode:  {Open: cbPx},
	}}
	broker := papertrading.NewPaperBroker(price)

	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 3, res.FilledCount)
	require.Equal(t, 0, res.RejectCount)
	require.Equal(t, 3, res.OrdersTotal)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.NotNil(t, acc)

	// --- No double-write: 3 orders, 3 fills ---
	var orderN, fillN int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&orderN).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fillN).Error)
	require.Equal(t, int64(3), orderN)
	require.Equal(t, int64(3), fillN)

	// Idempotent re-run must not create more fills/orders.
	res2, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, res2.OrdersTotal)
	require.Equal(t, 3, res2.SkippedAlready)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&orderN).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fillN).Error)
	require.Equal(t, int64(3), orderN)
	require.Equal(t, int64(3), fillN)

	pos, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, pos, 3)

	byCode := map[string]papertrading.PaperSimPosition{}
	for _, p := range pos {
		byCode[p.StockCode] = p
		require.Equal(t, p.TotalVolume, p.AvailableVolume+p.LockedVolume,
			"invariant available+locked==total code=%s", p.StockCode)
	}

	// 1) A-share equity: T1 locked
	eq := byCode[eqCode]
	require.Equal(t, eqQty, eq.TotalVolume)
	require.Equal(t, eqQty, eq.LockedVolume)
	require.Equal(t, int64(0), eq.AvailableVolume)
	require.InDelta(t, eqPx, eq.AvgCost, 1e-9)

	// 2) ETF: T0 available
	etf := byCode[etfCode]
	require.Equal(t, etfQty, etf.TotalVolume)
	require.Equal(t, int64(0), etf.LockedVolume)
	require.Equal(t, etfQty, etf.AvailableVolume)

	// 3) Convertible bond: T0 available
	cb := byCode[cbCode]
	require.Equal(t, cbQty, cb.TotalVolume)
	require.Equal(t, int64(0), cb.LockedVolume)
	require.Equal(t, cbQty, cb.AvailableVolume)

	// Cash: fee=0 → initial - sum(price*qty)
	spent := eqPx*float64(eqQty) + etfPx*float64(etfQty) + cbPx*float64(cbQty)
	wantCash := initialCash - spent
	require.InDelta(t, wantCash, acc.Cash, 1e-6, "cash after fills")

	cashBeforeSettle := acc.Cash
	lockedBefore := eq.LockedVolume
	availETFBefore := etf.AvailableVolume
	availCBBefore := cb.AvailableVolume

	// 5) Settlement: mark-to-market only — must NOT unlock / change volumes / change cash
	settle, err := papertrading.SettlementJob(tradeDate, price, false)
	require.NoError(t, err)
	require.NotNil(t, settle)
	require.GreaterOrEqual(t, settle.PositionsUpdated, 1)
	require.Equal(t, lockedBefore+0+0, settle.LockedVolumeTotal) // only equity still locked

	accAfter, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.InDelta(t, cashBeforeSettle, accAfter.Cash, 1e-9, "settlement must not touch cash")

	posAfter, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, posAfter, 3)
	after := map[string]papertrading.PaperSimPosition{}
	for _, p := range posAfter {
		after[p.StockCode] = p
		require.Equal(t, p.TotalVolume, p.AvailableVolume+p.LockedVolume)
	}
	require.Equal(t, lockedBefore, after[eqCode].LockedVolume, "settlement must not unlock equity")
	require.Equal(t, int64(0), after[eqCode].AvailableVolume)
	require.Equal(t, availETFBefore, after[etfCode].AvailableVolume)
	require.Equal(t, availCBBefore, after[cbCode].AvailableVolume)
	require.Equal(t, int64(0), after[etfCode].LockedVolume)
	require.Equal(t, int64(0), after[cbCode].LockedVolume)

	// Equity still T1-locked after settle; no extra fills.
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fillN).Error)
	require.Equal(t, int64(3), fillN)

	// Account equity ≈ cash + mark*volume (marks = open in this provider)
	mv := eqPx*float64(eqQty) + etfPx*float64(etfQty) + cbPx*float64(cbQty)
	require.InDelta(t, mv, settle.MarketValue, 1e-4)
	require.InDelta(t, accAfter.Cash+mv, settle.Equity, 1e-4)
}
