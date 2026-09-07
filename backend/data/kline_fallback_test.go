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
