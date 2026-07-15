package data

import (
	"testing"
)

func TestEastMoneyToTencentCode(t *testing.T) {
	cases := map[string]string{
		"600280.SH": "sh600280",
		"000001.SZ": "sz000001",
		"601101.SH": "sh601101",
	}
	for in, want := range cases {
		if got := EastMoneyToTencentCode(in); got != want {
			t.Errorf("%s => %q, want %q", in, got, want)
		}
	}
}

func TestFillKLineDerivedFields(t *testing.T) {
	rows := []KLineData{
		{Day: "2026-06-04", Close: "30.00", Open: "29.50", High: "30.50", Low: "29.00", Volume: "1000"},
		{Day: "2026-06-05", Close: "35.50", Open: "31.45", High: "36.39", Low: "30.71", Volume: "13056763"},
	}
	fillKLineDerivedFields(&rows)
	last := rows[1]
	if isKLineFieldEmpty(last.ChangePercent) {
		t.Fatalf("changePercent empty: %q", last.ChangePercent)
	}
	if isKLineFieldEmpty(last.ChangeValue) {
		t.Fatalf("changeValue empty: %q", last.ChangeValue)
	}
	if isKLineFieldEmpty(last.Amplitude) {
		t.Fatalf("amplitude empty: %q", last.Amplitude)
	}
	if isKLineFieldEmpty(last.Amount) {
		t.Fatalf("amount empty: %q", last.Amount)
	}
}

func TestGetKLineDataBeforeTencentFallback(t *testing.T) {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	k := api.GetKLineDataBefore("600280.SH", "101", "", 30, "20500101")
	if k == nil || len(*k) == 0 {
		t.Fatal("expected K-line data for 600280.SH via tencent fallback")
	}
	t.Logf("got %d bars, last day=%s close=%s", len(*k), (*k)[len(*k)-1].Day, (*k)[len(*k)-1].Close)
}

func TestGetKLineDataBefore002955SZ(t *testing.T) {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	k := api.GetKLineDataBefore("002955.SZ", "101", "", 30, "20500101")
	if k == nil || len(*k) == 0 {
		t.Fatal("expected K-line data for 002955.SZ")
	}
	t.Logf("got %d bars, last day=%s close=%s", len(*k), (*k)[len(*k)-1].Day, (*k)[len(*k)-1].Close)
}

func TestGetKLineDataBefore002955SZMinute(t *testing.T) {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	for _, klt := range []string{"1", "5", "15", "30", "60"} {
		k := api.GetKLineDataBefore("002955.SZ", klt, "", 60, "20500101")
		if k == nil || len(*k) == 0 {
			t.Fatalf("expected minute K-line for 002955.SZ klt=%s", klt)
		}
		t.Logf("klt=%s got %d bars last=%s close=%s", klt, len(*k), (*k)[len(*k)-1].Day, (*k)[len(*k)-1].Close)
	}
}

func TestGetKLineDataBeforeSinaFallbackBJ(t *testing.T) {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	k := api.GetKLineDataBefore("920036.BJ", "101", "", 120, "20500101")
	if k == nil || len(*k) < 20 {
		t.Fatalf("expected >=20 daily bars for 920036.BJ, got %d", len(*k))
	}
	t.Logf("got %d bars, first=%s last=%s", len(*k), (*k)[0].Day, (*k)[len(*k)-1].Day)
}
