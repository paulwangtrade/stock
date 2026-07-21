package data

import (
	"math"
	"testing"
)

func TestIsAShareSinaCode(t *testing.T) {
	ok := []string{"sz000001", "sh600519", "SZ000001"}
	for _, c := range ok {
		if !IsAShareSinaCode(c) {
			t.Fatalf("want ok: %s", c)
		}
	}
	bad := []string{"", "000001.SZ", "hk00700", "sz00001", "gb_aapl", "sh60051a"}
	for _, c := range bad {
		if IsAShareSinaCode(c) {
			t.Fatalf("want reject: %s", c)
		}
	}
}

func TestCalcOpenBuyVolume(t *testing.T) {
	// 100000 / 10.5 ≈ 9523 → 9500
	v := calcOpenBuyVolume(100000, 10.5)
	if v != 9500 {
		t.Fatalf("vol=%d want 9500", v)
	}
	if calcOpenBuyVolume(1000, 50) != 0 { // 20 股不足一手
		t.Fatalf("expect 0 for tiny amount")
	}
	if calcOpenBuyVolume(100000, 0) != 0 {
		t.Fatal("zero price")
	}
	_ = math.Floor
}
