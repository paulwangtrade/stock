package papertrading_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

type stubObsQuoteService struct {
	quotes []marketdata.Quote
	err    error
	calls  int
}

func (s *stubObsQuoteService) GetQuote(code string) (*marketdata.Quote, error) {
	qs, err := s.GetQuotes([]string{code})
	if err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, marketdata.ErrNoData
	}
	out := qs[0]
	return &out, nil
}

func (s *stubObsQuoteService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.quotes, nil
}

func TestBuildObservationPositionRows_FallbackPersistedMark(t *testing.T) {
	pos := []papertrading.PaperSimPosition{{
		StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.05, MarkPrice: 10.05,
	}}

	// nil service
	rows := papertrading.BuildObservationPositionRows(pos, nil)
	require.Len(t, rows, 1)
	require.Equal(t, papertrading.QuoteSourcePersisted, rows[0].QuoteSource)
	require.InDelta(t, 10.05, rows[0].DisplayPrice, 1e-9)
	require.InDelta(t, 10.05, rows[0].PersistedMarkPrice, 1e-9)
	require.InDelta(t, 10.05, rows[0].MarkPrice, 1e-9)
	require.InDelta(t, 0, rows[0].UnrealizedPnl, 1e-9)

	// error from service
	stub := &stubObsQuoteService{err: errors.New("upstream down")}
	rows = papertrading.BuildObservationPositionRows(pos, stub)
	require.Equal(t, 1, stub.calls)
	require.Equal(t, papertrading.QuoteSourcePersisted, rows[0].QuoteSource)
	require.InDelta(t, 10.05, rows[0].DisplayPrice, 1e-9)
}

func TestBuildObservationPositionRows_LiveQuoteAndPnL(t *testing.T) {
	fetched := time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local)
	stub := &stubObsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Name: "平安银行", Price: 11.0, Open: 10.5,
		FetchedAt: fetched,
	}}}
	pos := []papertrading.PaperSimPosition{{
		StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.0, MarkPrice: 10.0,
	}}
	rows := papertrading.BuildObservationPositionRows(pos, stub)
	require.Len(t, rows, 1)
	r := rows[0]
	require.Equal(t, papertrading.QuoteSourceLive, r.QuoteSource)
	require.InDelta(t, 11.0, r.DisplayPrice, 1e-9)
	require.InDelta(t, 11.0, r.MarkPrice, 1e-9)
	require.InDelta(t, 10.0, r.PersistedMarkPrice, 1e-9)
	require.InDelta(t, 11000.0, r.MarketValue, 1e-6)
	require.InDelta(t, 1000.0, r.UnrealizedPnl, 1e-6)
	require.InDelta(t, 0.1, r.ReturnRate, 1e-9)
	require.NotNil(t, r.QuoteUpdatedAt)
	require.True(t, r.QuoteUpdatedAt.Equal(fetched))
}

func TestBuildObservationPositionRows_OpenFallback(t *testing.T) {
	stub := &stubObsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Open: 10.8, Price: 0,
	}}}
	pos := []papertrading.PaperSimPosition{{
		StockCode: "sz000001", TotalVolume: 100, AvgCost: 10.0, MarkPrice: 10.0,
	}}
	rows := papertrading.BuildObservationPositionRows(pos, stub)
	require.Equal(t, papertrading.QuoteSourceOpenFallback, rows[0].QuoteSource)
	require.InDelta(t, 10.8, rows[0].DisplayPrice, 1e-9)
	require.InDelta(t, 80.0, rows[0].UnrealizedPnl, 1e-6)
}

func TestBuildObservationPositionRows_ReadOnlyNoMutation(t *testing.T) {
	updatedAt := time.Date(2026, 7, 27, 12, 30, 0, 0, time.Local)
	pos := []papertrading.PaperSimPosition{{
		StockCode:       "sz000001",
		StockName:       "平安银行",
		TotalVolume:     1000,
		AvailableVolume: 1000,
		LockedVolume:    0,
		AvgCost:         10.0,
		MarkPrice:       10.0,
		UpdatedAt:       updatedAt,
	}}
	before := pos[0]

	stub := &stubObsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 12.0, FetchedAt: updatedAt.Add(5 * time.Second),
	}}}
	rows := papertrading.BuildObservationPositionRows(pos, stub)
	require.Len(t, rows, 1)
	require.InDelta(t, 12.0, rows[0].DisplayPrice, 1e-9)
	require.InDelta(t, 10.0, rows[0].PersistedMarkPrice, 1e-9)
	require.InDelta(t, 2000.0, rows[0].UnrealizedPnl, 1e-6)

	// Read-only guarantee: overlay computes DTO only; input persisted position data remains unchanged.
	require.Equal(t, before.StockCode, pos[0].StockCode)
	require.Equal(t, before.StockName, pos[0].StockName)
	require.Equal(t, before.TotalVolume, pos[0].TotalVolume)
	require.Equal(t, before.AvailableVolume, pos[0].AvailableVolume)
	require.Equal(t, before.LockedVolume, pos[0].LockedVolume)
	require.InDelta(t, before.AvgCost, pos[0].AvgCost, 1e-9)
	require.InDelta(t, before.MarkPrice, pos[0].MarkPrice, 1e-9)
	require.Equal(t, before.UpdatedAt.UnixNano(), pos[0].UpdatedAt.UnixNano())
}
