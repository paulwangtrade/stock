package data

import (
	"testing"
)

func TestGetStockCodeRealTimeData_EmptyCodesNoError(t *testing.T) {
	api := NewStockDataApi()
	infos, err := api.GetStockCodeRealTimeData()
	if err != nil {
		t.Fatalf("empty codes should not error: %v", err)
	}
	if infos == nil || len(*infos) != 0 {
		t.Fatalf("empty codes should return empty slice, got %#v", infos)
	}
}

func TestGetStockCodeRealTimeData_EmptyVariadicSliceNoError(t *testing.T) {
	api := NewStockDataApi()
	infos, err := api.GetStockCodeRealTimeData([]string{}...)
	if err != nil {
		t.Fatalf("empty slice should not error: %v", err)
	}
	if infos == nil || len(*infos) != 0 {
		t.Fatalf("empty slice should return empty, got %#v", infos)
	}
}
