//go:build integration

package data

import (
	"testing"
)

func TestFetchEastMoneyKlineViaChromedp(t *testing.T) {
	bs, err := fetchEastMoneyCookiesViaChromedp("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe", 30, "https://quote.eastmoney.com/")

	if err != nil {
		t.Errorf("fetchEastMoneyCookiesViaChromedp() error = %v", err)
	}
	t.Log(string(bs))
}
