package data

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/marketdata"
)

type stubKlineService struct {
	bars []marketdata.Bar
	err  error
	last struct {
		code, period, adjust string
		limit                int
		end                  time.Time
	}
}

func (s *stubKlineService) GetBars(code string, period string, adjust string, limit int, endTime time.Time) ([]marketdata.Bar, error) {
	s.last.code, s.last.period, s.last.adjust, s.last.limit, s.last.end = code, period, adjust, limit, endTime
	if s.err != nil {
		return nil, s.err
	}
	return s.bars, nil
}

func TestEastMoneyKLineSectionWithService_UsesKlineService(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{{
		Code: "000001.SZ", Period: "101", TimeText: "2026-07-24",
		Open: 10.1, High: 10.8, Low: 10.0, Close: 10.5,
		Volume: 123456, Amount: 1e6, ChangePercent: 1.2, ChangeValue: 0.1, Amplitude: 2.0, TurnoverRate: 0.5,
	}}}
	api := NewEastMoneyKLineApi(&SettingConfig{})
	out := EastMoneyKLineSectionWithService(api, stub, "000001.SZ", "day", "", 60)
	if stub.last.code != "000001.SZ" || stub.last.period != "101" || stub.last.limit != 60 {
		t.Fatalf("unexpected GetBars args: %+v", stub.last)
	}
	if !stub.last.end.IsZero() {
		t.Fatalf("endTime should be zero for latest bars, got %v", stub.last.end)
	}
	if !strings.Contains(out, "000001.SZ") || !strings.Contains(out, "2026-07-24") {
		t.Fatalf("markdown missing expected content: %s", out)
	}
	if !strings.Contains(out, "10.5") {
		t.Fatalf("markdown missing close price: %s", out)
	}
}

func TestEastMoneyKLineSectionWithService_QfqDay(t *testing.T) {
	stub := &stubKlineService{bars: []marketdata.Bar{{TimeText: "2026-07-24", Close: 1}}}
	api := NewEastMoneyKLineApi(&SettingConfig{})
	_ = EastMoneyKLineSectionWithService(api, stub, "600000.SH", "101", "forward", 30)
	if stub.last.adjust != "qfq" {
		t.Fatalf("day+non-qfq/hfq adjust should normalize to qfq for legacy parity, got %q", stub.last.adjust)
	}
}

func TestEastMoneyKLineSectionWithService_NoService(t *testing.T) {
	api := NewEastMoneyKLineApi(&SettingConfig{})
	out := EastMoneyKLineSectionWithService(api, nil, "000001.SZ", "day", "", 10)
	if !strings.Contains(out, "KlineService 未初始化") {
		t.Fatalf("expected uninitialized service message, got %s", out)
	}
}

func TestEastMoneyKLineSectionWithService_NoData(t *testing.T) {
	stub := &stubKlineService{err: marketdata.ErrNoData}
	api := NewEastMoneyKLineApi(&SettingConfig{})
	out := EastMoneyKLineSectionWithService(api, stub, "000001.SZ", "day", "", 10)
	if !strings.Contains(out, "未获取到 K 线数据") {
		t.Fatalf("expected no-data message, got %s", out)
	}
}

func TestResolveEastMoneyKLineAdjust(t *testing.T) {
	if got := resolveEastMoneyKLineAdjust("101", "hfq"); got != "hfq" {
		t.Fatalf("hfq: got %q", got)
	}
	if got := resolveEastMoneyKLineAdjust("101", "x"); got != "qfq" {
		t.Fatalf("invalid day adjust -> qfq: got %q", got)
	}
	if got := resolveEastMoneyKLineAdjust("5", "qfq"); got != "qfq" {
		t.Fatalf("minute keeps adjust: got %q", got)
	}
}
