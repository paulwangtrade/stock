package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

type stubCloseKline struct {
	bars []marketdata.Bar
	err  error

	lastCode   string
	lastPeriod string
	lastAdjust string
	lastEnd    time.Time
}

func (s *stubCloseKline) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	s.lastCode = code
	s.lastPeriod = period
	s.lastAdjust = adjust
	s.lastEnd = endTime
	if s.err != nil {
		return nil, s.err
	}
	out := make([]marketdata.Bar, len(s.bars))
	copy(out, s.bars)
	return out, nil
}

func TestCloseFillProvider_ReturnsDailyClose(t *testing.T) {
	day := time.Date(2026, 8, 5, 15, 0, 0, 0, time.Local)
	stub := &stubCloseKline{bars: []marketdata.Bar{
		{Time: day.AddDate(0, 0, -1), TimeText: "2026-08-04", Open: 9.0, Close: 9.5},
		{Time: day, TimeText: "2026-08-05", Open: 10.1, Close: 11.23},
	}}
	p := papertrading.CloseFillProvider{Klines: stub}

	q, ok := p.OpenQuote("sz000001", "2026-08-05")
	require.True(t, ok)
	require.InDelta(t, 11.23, q.Open, 1e-9)
	require.Equal(t, papertrading.PriceKindClose, q.PriceKind)
	require.Equal(t, marketdata.PeriodDay, stub.lastPeriod)
	require.Equal(t, marketdata.AdjustNone, stub.lastAdjust)
	require.Equal(t, "sz000001", stub.lastCode)
	require.Equal(t, "2026-08-05", stub.lastEnd.Format("2006-01-02"))
	// Must not return Open as fill.
	require.NotEqual(t, 10.1, q.Open)
}

func TestCloseFillProvider_CloseZero_Fails(t *testing.T) {
	day := time.Date(2026, 8, 5, 15, 0, 0, 0, time.Local)
	stub := &stubCloseKline{bars: []marketdata.Bar{
		{Time: day, TimeText: "2026-08-05", Open: 10.1, Close: 0},
	}}
	q, ok := papertrading.CloseFillProvider{Klines: stub}.OpenQuote("sz000001", "2026-08-05")
	require.False(t, ok)
	require.Equal(t, papertrading.Quote{}, q)
}

func TestCloseFillProvider_NoBar_Fails(t *testing.T) {
	stub := &stubCloseKline{bars: []marketdata.Bar{
		{Time: time.Date(2026, 8, 4, 15, 0, 0, 0, time.Local), TimeText: "2026-08-04", Open: 9, Close: 9.5},
	}}
	q, ok := papertrading.CloseFillProvider{Klines: stub}.OpenQuote("sz000001", "2026-08-05")
	require.False(t, ok)
	require.Equal(t, papertrading.Quote{}, q)
}

func TestCloseFillProvider_NoFallbackToOpen(t *testing.T) {
	// Wrong-day bar has Open; matching day missing → must fail, not use Open.
	stub := &stubCloseKline{bars: []marketdata.Bar{
		{Time: time.Date(2026, 8, 4, 15, 0, 0, 0, time.Local), TimeText: "2026-08-04", Open: 99.0, Close: 0},
	}}
	q, ok := papertrading.CloseFillProvider{Klines: stub}.OpenQuote("sz000001", "2026-08-05")
	require.False(t, ok)
	require.Equal(t, 0.0, q.Open)
}
