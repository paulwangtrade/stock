package marketdata_test

import (
	"testing"
	"time"

	"go-stock/backend/marketdata"

	"github.com/stretchr/testify/require"
)

type stubQuote struct{}

func (stubQuote) GetQuote(code string) (*marketdata.Quote, error) {
	return &marketdata.Quote{Code: code, Price: 10}, nil
}
func (stubQuote) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	out := make([]marketdata.Quote, 0, len(codes))
	for _, c := range codes {
		out = append(out, marketdata.Quote{Code: c, Price: 1})
	}
	return out, nil
}

type stubKline struct {
	calls int
	lastP string
}

func (s *stubKline) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	s.calls++
	s.lastP = period
	t0 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	t1 := time.Date(2026, 8, 5, 0, 0, 0, 0, time.Local)
	return []marketdata.Bar{
		{Code: code, Period: period, Adjust: adjust, Time: t0, Close: 1},
		{Code: code, Period: period, Adjust: adjust, Time: t1, Close: 2},
	}, nil
}

type stubMinute struct{}

func (stubMinute) GetMinute(code string) ([]marketdata.MinutePoint, string, error) {
	return []marketdata.MinutePoint{{Time: "09:30", Price: 9.9}}, "20260809", nil
}

func TestCompositeMarketDataService_Facade(t *testing.T) {
	primary := &stubKline{}
	svc := marketdata.NewCompositeMarketDataService(stubQuote{}, primary)
	q, err := svc.GetQuote("sz000001")
	require.NoError(t, err)
	require.Equal(t, 10.0, q.Price)

	bars, err := svc.GetKline("sz000001", "day", "qfq", 10)
	require.NoError(t, err)
	require.Len(t, bars, 2)

	from := time.Date(2026, 8, 3, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 8, 9, 0, 0, 0, 0, time.Local)
	hist, err := svc.GetHistory("sz000001", "day", "qfq", from, to)
	require.NoError(t, err)
	require.Len(t, hist, 1)
	require.Equal(t, 2.0, hist[0].Close)
}

func TestCompositeMarketDataService_GetBars_RoutesSecondary(t *testing.T) {
	primary := &stubKline{}
	secondary := &stubKline{}
	svc := &marketdata.CompositeMarketDataService{
		Quotes:          stubQuote{},
		Klines:          primary,
		SecondaryKlines: secondary,
	}
	_, err := svc.GetBars("sz000001", marketdata.PeriodDailyFQ, "", 5, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 1, secondary.calls)
	require.Equal(t, 0, primary.calls)

	_, err = svc.GetBars("hk00700", marketdata.PeriodDailyHK, "", 5, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 2, secondary.calls)

	_, err = svc.GetBars("sz000001", marketdata.PeriodDay, "", 5, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 1, primary.calls)
}

func TestCompositeMarketDataService_GetMinute(t *testing.T) {
	svc := &marketdata.CompositeMarketDataService{
		Quotes:  stubQuote{},
		Klines:  &stubKline{},
		Minutes: stubMinute{},
	}
	pts, date, err := svc.GetMinute("sz000001")
	require.NoError(t, err)
	require.Equal(t, "20260809", date)
	require.Len(t, pts, 1)
}

func TestCompositeMarketDataService_NilProvider(t *testing.T) {
	svc := marketdata.NewCompositeMarketDataService(nil, nil)
	_, err := svc.GetQuote("x")
	require.ErrorIs(t, err, marketdata.ErrProviderUnavailable)

	_, err = svc.GetBars("x", marketdata.PeriodDailyFQ, "", 1, time.Time{})
	require.ErrorIs(t, err, marketdata.ErrProviderUnavailable)

	_, _, err = svc.GetMinute("x")
	require.ErrorIs(t, err, marketdata.ErrProviderUnavailable)
}

type errQuote struct{ err error }

func (e errQuote) GetQuote(code string) (*marketdata.Quote, error) { return nil, e.err }
func (e errQuote) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	return nil, e.err
}

type errKline struct{ err error }

func (e errKline) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	return nil, e.err
}

type errMinute struct{ err error }

func (e errMinute) GetMinute(code string) ([]marketdata.MinutePoint, string, error) {
	return nil, "", e.err
}

func TestCompositeMarketDataService_ErrorPropagation(t *testing.T) {
	svc := &marketdata.CompositeMarketDataService{
		Quotes:  errQuote{err: marketdata.ErrNoData},
		Klines:  errKline{err: marketdata.ErrNoData},
		Minutes: errMinute{err: marketdata.ErrNoData},
	}
	_, err := svc.GetQuote("sz000001")
	require.ErrorIs(t, err, marketdata.ErrNoData)
	_, err = svc.GetBars("sz000001", marketdata.PeriodDay, "", 1, time.Time{})
	require.ErrorIs(t, err, marketdata.ErrNoData)
	_, _, err = svc.GetMinute("sz000001")
	require.ErrorIs(t, err, marketdata.ErrNoData)
}

func TestCompositeMarketDataService_SecondaryFallbackUnavailable(t *testing.T) {
	// primary present; secondary missing → daily_fq must not silently use primary
	svc := &marketdata.CompositeMarketDataService{
		Quotes: stubQuote{},
		Klines: &stubKline{},
	}
	_, err := svc.GetBars("sz000001", marketdata.PeriodDailyFQ, "", 1, time.Time{})
	require.ErrorIs(t, err, marketdata.ErrProviderUnavailable)
}

func TestParseEndTime(t *testing.T) {
	require.True(t, marketdata.ParseEndTime("").IsZero())
	require.True(t, marketdata.ParseEndTime(marketdata.LatestEndFlag).IsZero())
	d := marketdata.ParseEndTime("20260801")
	require.Equal(t, 2026, d.Year())
	require.Equal(t, time.August, d.Month())
	require.Equal(t, 1, d.Day())
}
