package adapter_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"

	"github.com/stretchr/testify/require"
)

type fakeTencentDay struct {
	hkCalls     int
	commonCalls int
	hk          []data.KLineData
	common      []data.KLineData
}

func (f *fakeTencentDay) GetHK_KLineData(stockCode string, kLineType string, days int64) *[]data.KLineData {
	f.hkCalls++
	out := append([]data.KLineData(nil), f.hk...)
	return &out
}
func (f *fakeTencentDay) GetCommonKLineData(stockCode string, kLineType string, days int64) *[]data.KLineData {
	f.commonCalls++
	out := append([]data.KLineData(nil), f.common...)
	return &out
}

func TestTencentKlineAdapter_RoutesHKAndFQ(t *testing.T) {
	fake := &fakeTencentDay{
		hk:     []data.KLineData{{Day: "2026-08-01", Open: "1", Close: "2", High: "3", Low: "0.5", Volume: "10"}},
		common: []data.KLineData{{Day: "2026-08-02", Open: "4", Close: "5", High: "6", Low: "3", Volume: "20"}},
	}
	a := adapter.NewTencentKlineAdapterWith(fake)

	hk, err := a.GetBars("hk00700", marketdata.PeriodDailyHK, "", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, hk, 1)
	require.Equal(t, 2.0, hk[0].Close)
	require.Equal(t, 1, fake.hkCalls)

	fq, err := a.GetBars("sz000001", marketdata.PeriodDailyFQ, "", 10, time.Time{})
	require.NoError(t, err)
	require.Len(t, fq, 1)
	require.Equal(t, 5.0, fq[0].Close)
	require.Equal(t, 1, fake.commonCalls)
}

type fakeMinute struct {
	points []data.MinuteData
	date   string
}

func (f fakeMinute) GetStockMinutePriceData(stockCode string) (*[]data.MinuteData, string) {
	out := append([]data.MinuteData(nil), f.points...)
	return &out, f.date
}

func TestTencentMinuteAdapter_MapsPoints(t *testing.T) {
	a := adapter.NewTencentMinuteAdapterWith(fakeMinute{
		points: []data.MinuteData{{Time: "09:31", Price: 10.1, Volume: 1, Amount: 2}},
		date:   "20260809",
	})
	pts, date, err := a.GetMinute("sz000001")
	require.NoError(t, err)
	require.Equal(t, "20260809", date)
	require.Len(t, pts, 1)
	require.InDelta(t, 10.1, pts[0].Price, 1e-9)
}

func TestTencentMinuteAdapter_Empty(t *testing.T) {
	a := adapter.NewTencentMinuteAdapterWith(fakeMinute{})
	_, _, err := a.GetMinute("sz000001")
	require.ErrorIs(t, err, marketdata.ErrNoData)
}
