package data

import (
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
