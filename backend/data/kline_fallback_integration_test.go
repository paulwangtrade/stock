//go:build integration

package data

import (
	"testing"
)

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
