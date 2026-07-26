// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A1 marketdata 接口 / Legacy Adapter 单元测试（全部使用 fake，不发网络请求）

package marketdata_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

// fakeKlineApi 记录调用参数并返回固定数据，用于验证 Adapter 的委托与映射。
type fakeKlineApi struct {
	calls  int
	code   string
	period string
	adjust string
	limit  int
	end    string
	result *[]data.KLineData
}

func (f *fakeKlineApi) GetKLineDataBefore(stockCode, kLineType, adjustFlag string, limit int, end string) *[]data.KLineData {
	f.calls++
	f.code, f.period, f.adjust, f.limit, f.end = stockCode, kLineType, adjustFlag, limit, end
	return f.result
}

type fakeQuoteApi struct {
	calls  int
	codes  []string
	result *[]data.StockInfo
	err    error
}

func (f *fakeQuoteApi) GetStockCodeRealTimeData(stockCodes ...string) (*[]data.StockInfo, error) {
	f.calls++
	f.codes = append([]string{}, stockCodes...)
	return f.result, f.err
}

func TestNormalizePeriodAndAdjust(t *testing.T) {
	periodCases := map[string]string{
		"":      marketdata.PeriodDay,
		"day":   marketdata.PeriodDay,
		"1d":    marketdata.PeriodDay,
		" 101 ": marketdata.PeriodDay,
		"WEEK":  marketdata.PeriodWeek,
		"5m":    marketdata.Period5Min,
		"60":    marketdata.Period60Min,
	}
	for in, want := range periodCases {
		got, err := marketdata.NormalizePeriod(in)
		if err != nil {
			t.Fatalf("NormalizePeriod(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizePeriod(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := marketdata.NormalizePeriod("tick"); !errors.Is(err, marketdata.ErrUnsupportedPeriod) {
		t.Fatalf("NormalizePeriod(tick) error = %v, want ErrUnsupportedPeriod", err)
	}

	adjustCases := map[string]string{
		"":     marketdata.AdjustNone,
		"none": marketdata.AdjustNone,
		"QFQ":  marketdata.AdjustForward,
		"hfq":  marketdata.AdjustBackward,
	}
	for in, want := range adjustCases {
		got, err := marketdata.NormalizeAdjust(in)
		if err != nil {
			t.Fatalf("NormalizeAdjust(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizeAdjust(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := marketdata.NormalizeAdjust("split"); !errors.Is(err, marketdata.ErrUnsupportedAdjust) {
		t.Fatalf("NormalizeAdjust(split) error = %v, want ErrUnsupportedAdjust", err)
	}
}

func TestNormalizeBarsRequest(t *testing.T) {
	end := time.Date(2026, 7, 24, 14, 30, 0, 0, time.Local)

	req, err := marketdata.NormalizeBarsRequest(" sh600000 ", "day", "qfq", 30, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Code != "sh600000" || req.Period != marketdata.PeriodDay || req.Adjust != marketdata.AdjustForward {
		t.Fatalf("unexpected normalized request: %+v", req)
	}
	if req.End != "20260724" {
		t.Fatalf("End = %q, want 20260724", req.End)
	}

	minReq, err := marketdata.NormalizeBarsRequest("sh600000", "5m", "", 10, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if minReq.End != "20260724143000" {
		t.Fatalf("minute End = %q, want 20260724143000", minReq.End)
	}

	latest, err := marketdata.NormalizeBarsRequest("sh600000", "", "", 5, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest.End != marketdata.LatestEndFlag {
		t.Fatalf("zero endTime End = %q, want %q", latest.End, marketdata.LatestEndFlag)
	}

	if _, err := marketdata.NormalizeBarsRequest("  ", "day", "", 10, time.Time{}); !errors.Is(err, marketdata.ErrEmptyCode) {
		t.Fatalf("empty code error = %v, want ErrEmptyCode", err)
	}
	if _, err := marketdata.NormalizeBarsRequest("sh600000", "day", "", 0, time.Time{}); !errors.Is(err, marketdata.ErrInvalidLimit) {
		t.Fatalf("zero limit error = %v, want ErrInvalidLimit", err)
	}
}

func TestEastMoneyKlineAdapterDelegatesLegacyApi(t *testing.T) {
	fake := &fakeKlineApi{result: &[]data.KLineData{
		{
			Day: "2026-07-23", Open: "10.10", Close: "10.50", High: "10.80", Low: "10.00",
			Volume: "123456", Amount: "1234567.89", Amplitude: "7.92",
			ChangePercent: "3.96", ChangeValue: "0.40", TurnoverRate: "1.23",
		},
		{Day: "2026-07-24", Open: "10.50", Close: "-", High: "", Low: "null", Volume: "0"},
	}}

	var svc marketdata.KlineService = adapter.NewEastMoneyKlineAdapterWith(fake)
	bars, err := svc.GetBars("sh600000", "day", "qfq", 2, time.Time{})
	if err != nil {
		t.Fatalf("GetBars error: %v", err)
	}

	if fake.calls != 1 {
		t.Fatalf("legacy api calls = %d, want 1", fake.calls)
	}
	if fake.code != "sh600000" || fake.period != marketdata.PeriodDay || fake.adjust != marketdata.AdjustForward {
		t.Fatalf("legacy api args = (%s,%s,%s), want (sh600000,101,qfq)", fake.code, fake.period, fake.adjust)
	}
	if fake.limit != 2 || fake.end != marketdata.LatestEndFlag {
		t.Fatalf("legacy api limit/end = (%d,%s), want (2,%s)", fake.limit, fake.end, marketdata.LatestEndFlag)
	}

	if len(bars) != 2 {
		t.Fatalf("bars len = %d, want 2", len(bars))
	}
	first := bars[0]
	if first.Code != "sh600000" || first.Period != marketdata.PeriodDay || first.Adjust != marketdata.AdjustForward {
		t.Fatalf("bar meta not propagated: %+v", first)
	}
	if first.Open != 10.10 || first.Close != 10.50 || first.High != 10.80 || first.Low != 10.00 {
		t.Fatalf("bar ohlc mismatch: %+v", first)
	}
	if first.Volume != 123456 || first.Amount != 1234567.89 || first.TurnoverRate != 1.23 {
		t.Fatalf("bar volume/amount mismatch: %+v", first)
	}
	if first.TimeText != "2026-07-23" || first.Time.Format("2006-01-02") != "2026-07-23" {
		t.Fatalf("bar time mismatch: text=%q time=%v", first.TimeText, first.Time)
	}
	if second := bars[1]; second.Close != 0 || second.High != 0 || second.Low != 0 {
		t.Fatalf("invalid upstream fields should map to 0: %+v", second)
	}
}

func TestEastMoneyKlineAdapterErrorHandling(t *testing.T) {
	empty := &fakeKlineApi{result: &[]data.KLineData{}}
	if _, err := adapter.NewEastMoneyKlineAdapterWith(empty).GetBars("sh600000", "day", "", 5, time.Time{}); !errors.Is(err, marketdata.ErrNoData) {
		t.Fatalf("empty result error = %v, want ErrNoData", err)
	}

	nilResult := &fakeKlineApi{result: nil}
	if _, err := adapter.NewEastMoneyKlineAdapterWith(nilResult).GetBars("sh600000", "day", "", 5, time.Time{}); !errors.Is(err, marketdata.ErrNoData) {
		t.Fatalf("nil result error = %v, want ErrNoData", err)
	}

	if _, err := adapter.NewEastMoneyKlineAdapterWith(nil).GetBars("sh600000", "day", "", 5, time.Time{}); !errors.Is(err, marketdata.ErrProviderUnavailable) {
		t.Fatalf("nil provider error = %v, want ErrProviderUnavailable", err)
	}

	guard := &fakeKlineApi{result: &[]data.KLineData{{Day: "2026-07-24"}}}
	if _, err := adapter.NewEastMoneyKlineAdapterWith(guard).GetBars("", "day", "", 5, time.Time{}); !errors.Is(err, marketdata.ErrEmptyCode) {
		t.Fatalf("empty code error = %v, want ErrEmptyCode", err)
	}
	if guard.calls != 0 {
		t.Fatalf("legacy api should not be called on invalid input, calls = %d", guard.calls)
	}
}

func TestLegacyQuoteAdapterDelegatesLegacyApi(t *testing.T) {
	fake := &fakeQuoteApi{result: &[]data.StockInfo{
		{
			Code: "sh600000", Name: "浦发银行", Price: "10.50", Open: "10.10", PreClose: "10.10",
			High: "10.80", Low: "10.00", Volume: "12345600", Amount: "129000000",
			Bid: "10.49", Ask: "10.51", Market: "sh", Date: "2026-07-24", Time: "15:00:00",
			ChangePercent: 3.96, ChangePrice: 0.40,
			CostPrice:     9.80, CostVolume: 1000, Profit: 7.14, ProfitAmount: 700,
		},
	}}

	var svc marketdata.QuoteService = adapter.NewLegacyQuoteAdapterWith(fake)
	quotes, err := svc.GetQuotes([]string{" sh600000 ", "sh600000", ""})
	if err != nil {
		t.Fatalf("GetQuotes error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("legacy api calls = %d, want 1", fake.calls)
	}
	if len(fake.codes) != 1 || fake.codes[0] != "sh600000" {
		t.Fatalf("legacy api codes = %v, want [sh600000]", fake.codes)
	}
	if len(quotes) != 1 {
		t.Fatalf("quotes len = %d, want 1", len(quotes))
	}
	q := quotes[0]
	if q.Code != "sh600000" || q.Name != "浦发银行" || q.Price != 10.50 || q.PreClose != 10.10 {
		t.Fatalf("quote mapping mismatch: %+v", q)
	}
	if q.ChangePercent != 3.96 || q.ChangeValue != 0.40 || q.Bid != 10.49 || q.Ask != 10.51 {
		t.Fatalf("quote derived fields mismatch: %+v", q)
	}
	if q.FetchedAt.IsZero() {
		t.Fatal("FetchedAt should be set by adapter")
	}

	single, err := svc.GetQuote("600000")
	if err != nil {
		t.Fatalf("GetQuote error: %v", err)
	}
	if single == nil || single.Code != "sh600000" {
		t.Fatalf("GetQuote result mismatch: %+v", single)
	}
}

func TestLegacyQuoteAdapterErrorHandling(t *testing.T) {
	if _, err := adapter.NewLegacyQuoteAdapterWith(&fakeQuoteApi{}).GetQuote("  "); !errors.Is(err, marketdata.ErrEmptyCode) {
		t.Fatalf("empty code error = %v, want ErrEmptyCode", err)
	}
	if _, err := adapter.NewLegacyQuoteAdapterWith(&fakeQuoteApi{}).GetQuotes([]string{"", "  "}); !errors.Is(err, marketdata.ErrEmptyCodes) {
		t.Fatalf("empty codes error = %v, want ErrEmptyCodes", err)
	}
	if _, err := adapter.NewLegacyQuoteAdapterWith(&fakeQuoteApi{result: &[]data.StockInfo{}}).GetQuotes([]string{"sh600000"}); !errors.Is(err, marketdata.ErrNoData) {
		t.Fatalf("empty result error = %v, want ErrNoData", err)
	}
	if _, err := adapter.NewLegacyQuoteAdapterWith(nil).GetQuotes([]string{"sh600000"}); !errors.Is(err, marketdata.ErrProviderUnavailable) {
		t.Fatalf("nil provider error = %v, want ErrProviderUnavailable", err)
	}

	upstream := errors.New("upstream boom")
	if _, err := adapter.NewLegacyQuoteAdapterWith(&fakeQuoteApi{err: upstream}).GetQuotes([]string{"sh600000"}); !errors.Is(err, upstream) {
		t.Fatalf("upstream error should be propagated, got %v", err)
	}

	miss := &fakeQuoteApi{result: &[]data.StockInfo{{Code: "sz000001", Price: "12.00"}}}
	if _, err := adapter.NewLegacyQuoteAdapterWith(miss).GetQuote("sh600000"); !errors.Is(err, marketdata.ErrNoData) {
		t.Fatalf("code miss error = %v, want ErrNoData", err)
	}
}

// TestModelsStayMarketDataOnly 守护领域边界：模型里出现交易域字段应立即失败，
// 避免后续 Phase 把 TradePlan / Risk / Position / Order 字段渗透进只读行情层。
func TestModelsStayMarketDataOnly(t *testing.T) {
	forbidden := []string{
		"cost", "profit", "position", "order", "plan", "risk",
		"alarm", "follow", "qty", "quantity", "stoploss", "takeprofit", "account",
	}
	for _, model := range []reflect.Type{
		reflect.TypeOf(marketdata.Bar{}),
		reflect.TypeOf(marketdata.Quote{}),
	} {
		for i := 0; i < model.NumField(); i++ {
			name := strings.ToLower(model.Field(i).Name)
			for _, bad := range forbidden {
				if strings.Contains(name, bad) {
					t.Fatalf("%s.%s looks like a trading-domain field (%q); marketdata models must stay market-data only",
						model.Name(), model.Field(i).Name, bad)
				}
			}
		}
	}
}
