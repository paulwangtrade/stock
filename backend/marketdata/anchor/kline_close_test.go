package anchor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/marketdata"

	"github.com/stretchr/testify/require"
)

type stubKlineService struct {
	bars []marketdata.Bar
	err  error

	lastCode   string
	lastPeriod string
	lastAdjust string
	lastLimit  int
	lastEnd    time.Time
}

func (s *stubKlineService) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	s.lastCode = code
	s.lastPeriod = period
	s.lastAdjust = adjust
	s.lastLimit = limit
	s.lastEnd = endTime
	if s.err != nil {
		return nil, s.err
	}
	out := make([]marketdata.Bar, len(s.bars))
	copy(out, s.bars)
	return out, nil
}

func TestKlineClose_HitExactSourceDate(t *testing.T) {
	day := time.Date(2026, 7, 27, 15, 0, 0, 0, time.Local)
	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: day.AddDate(0, 0, -1), TimeText: "2026-07-26", Open: 9, Close: 9.5},
		{Time: day, TimeText: "2026-07-27", Open: 10.1, Close: 10.5},
	}}
	p := KlineCloseAnchorProvider{Klines: stub}

	r, ok := p.Resolve(Context{
		StockCode:  "sz000001",
		TradeDate:  "2026-07-28", // must not be used as bar day
		SourceDate: "2026-07-27",
	})
	require.True(t, ok)
	require.True(t, Valid(r))
	require.InDelta(t, 10.5, r.RefPrice, 1e-9)
	require.Equal(t, RefSourceKlineClose, r.RefSource)
	require.Equal(t, "2026-07-27", r.RefAsOf)
	require.Equal(t, marketdata.PeriodDay, stub.lastPeriod)
	require.Equal(t, marketdata.AdjustNone, stub.lastAdjust)
	require.Equal(t, "sz000001", stub.lastCode)
	// TradeDate must not become endTime calendar day.
	require.Equal(t, "2026-07-27", stub.lastEnd.Format("2006-01-02"))
}

func TestKlineClose_WrongDayMiss(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: time.Date(2026, 7, 26, 15, 0, 0, 0, time.Local), TimeText: "2026-07-26", Close: 9.9},
	}}
	_, ok := KlineCloseAnchorProvider{Klines: stub}.Resolve(Context{
		StockCode: "sz000001", SourceDate: "2026-07-27", TradeDate: "2026-07-28",
	})
	require.False(t, ok)
}

func TestKlineClose_EmptySourceDateMiss(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: time.Date(2026, 7, 27, 15, 0, 0, 0, time.Local), Close: 10},
	}}
	_, ok := KlineCloseAnchorProvider{Klines: stub}.Resolve(Context{
		StockCode: "sz000001", TradeDate: "2026-07-28", SourceDate: "",
	})
	require.False(t, ok)
}

func TestKlineClose_CloseZeroIgnoresOpen(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{
		{
			Time: time.Date(2026, 7, 27, 15, 0, 0, 0, time.Local),
			TimeText: "2026-07-27", Open: 11.2, High: 12, Low: 10, Close: 0,
		},
	}}
	_, ok := KlineCloseAnchorProvider{Klines: stub}.Resolve(Context{
		StockCode: "sz000001", SourceDate: "2026-07-27",
	})
	require.False(t, ok, "must not use Open when Close<=0")
}

func TestKlineClose_NeverUsesOpenOrRealtime(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("kline_close.go"))
	require.NoError(t, err)
	src := string(body)
	for _, bad := range []string{
		"OpenQuote", "RealtimeOpen", "GetStockCodeRealTimeData", "MarkPrice",
		"QuoteService", "GetQuote",
	} {
		require.NotContains(t, src, bad, "KlineClose must not use %q for ref_price", bad)
	}
	// Must not assign Open into RefPrice.
	require.NotContains(t, src, "RefPrice:   b.Open")
	require.NotContains(t, src, "RefPrice: b.Open")
}

func TestChain_KlineHitPreferredOverFollowed(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000001", 1.0, 1.0, time.Time{}) // would be wrong price if used

	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: time.Date(2026, 7, 27, 15, 0, 0, 0, time.Local), Close: 22.2},
	}}
	chain := Chain{Sources: []Provider{
		KlineCloseAnchorProvider{Klines: stub},
		FollowedStockAnchorProvider{},
	}}
	r, ok := chain.Resolve(Context{StockCode: "sz000001", SourceDate: "2026-07-27", TradeDate: "2026-07-28"})
	require.True(t, ok)
	require.InDelta(t, 22.2, r.RefPrice, 1e-9)
	require.Equal(t, RefSourceKlineClose, r.RefSource)
}

func TestChain_KlineMissFallsBackToFollowed(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000001", 8.8, 0, time.Date(2026, 7, 20, 0, 0, 0, 0, time.Local))

	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: time.Date(2026, 7, 26, 15, 0, 0, 0, time.Local), Close: 99}, // wrong day
	}}
	chain := Chain{Sources: []Provider{
		KlineCloseAnchorProvider{Klines: stub},
		FollowedStockAnchorProvider{},
	}}
	r, ok := chain.Resolve(Context{
		StockCode: "sz000001", SourceDate: "2026-07-27", TradeDate: "2026-07-28",
		PoolSource: PoolSourceStrategyRun,
	})
	require.True(t, ok)
	require.InDelta(t, 8.8, r.RefPrice, 1e-9)
	require.Equal(t, RefSourceStrategySnapshot, r.RefSource)
}

func TestChain_BothMissSoftFail(t *testing.T) {
	setupFollowedTestDB(t)
	// no followed row for this code
	stub := &stubKlineService{err: marketdata.ErrNoData}
	chain := Chain{Sources: []Provider{
		KlineCloseAnchorProvider{Klines: stub},
		FollowedStockAnchorProvider{},
	}}
	_, ok := chain.Resolve(Context{StockCode: "sz999999", SourceDate: "2026-07-27", TradeDate: "2026-07-28"})
	require.False(t, ok)
}

func TestDefaultProvider_KlineThenFollowed(t *testing.T) {
	p, ok := DefaultProvider().(Chain)
	require.True(t, ok)
	require.Len(t, p.Sources, 2)
	_, isKline := p.Sources[0].(KlineCloseAnchorProvider)
	_, isFollowed := p.Sources[1].(FollowedStockAnchorProvider)
	require.True(t, isKline, "Sources[0] must be KlineCloseAnchorProvider")
	require.True(t, isFollowed, "Sources[1] must be FollowedStockAnchorProvider")
}

// TestKlineHit_YieldsSelectableAnchor mirrors populate's ok&&Valid gate for selected Intent.
func TestKlineHit_YieldsSelectableAnchor(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{
		{Time: time.Date(2026, 7, 27, 15, 0, 0, 0, time.Local), Close: 15.3},
	}}
	r, ok := KlineCloseAnchorProvider{Klines: stub}.Resolve(Context{
		StockCode: "sh600000", SourceDate: "2026-07-27",
	})
	require.True(t, ok)
	require.True(t, Valid(r), "populate writes selected only when Valid(Result)")
	require.Greater(t, r.RefPrice, 0.0)
	require.Equal(t, RefSourceKlineClose, r.RefSource)
}
