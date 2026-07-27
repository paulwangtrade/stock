package cache

import (
	"testing"
	"time"

	"go-stock/backend/data"

	"github.com/stretchr/testify/require"
)

func TestStockPriceCache_GetSetTTL(t *testing.T) {
	c := NewStockPriceCache(120 * time.Millisecond)
	_, ok := c.Get("sh600000")
	require.False(t, ok)
	require.True(t, c.IsStale())

	c.Set("SH600000", &data.StockInfo{Code: "sh600000", Name: "浦发", Price: "10.1"})
	got, ok := c.Get("sh600000")
	require.True(t, ok)
	require.Equal(t, "10.1", got.Price)
	require.False(t, c.IsStale())

	// 规范化：us -> gb_
	c.Set("usAAPL", &data.StockInfo{Code: "gb_aapl", Name: "AAPL", Price: "1"})
	got, ok = c.Get("usaapl")
	require.True(t, ok)
	require.Equal(t, "gb_aapl", got.Code)

	time.Sleep(200 * time.Millisecond)
	_, ok = c.Get("sh600000")
	require.False(t, ok)
	require.True(t, c.IsStale())
}

func TestStockPriceCache_GetBatchPartition(t *testing.T) {
	c := NewStockPriceCache(time.Second)
	c.SetBatch(map[string]*data.StockInfo{
		"sh600000": {Code: "sh600000", Price: "1"},
		"sz000001": {Code: "sz000001", Price: "2"},
	})

	hits := c.GetBatch([]string{"sh600000", "hk00700", "sz000001"})
	require.Len(t, hits, 2)
	require.Equal(t, "1", hits["sh600000"].Price)

	partHits, misses := c.Partition([]string{"sh600000", "hk00700", "sz000001", "sh600000"})
	require.Len(t, partHits, 2)
	require.Equal(t, []string{"hk00700"}, misses)
}

func TestStockPriceCache_SetTTLConfigurable(t *testing.T) {
	c := NewStockPriceCache(time.Second)
	c.SetTTL(2 * time.Second)
	require.Equal(t, 2*time.Second, c.TTL())
	c.SetTTL(0)
	require.Equal(t, DefaultStockPriceTTL, c.TTL())
}

func TestStockPriceCache_CloneIsolation(t *testing.T) {
	c := NewStockPriceCache(time.Second)
	src := &data.StockInfo{Code: "sh600000", Price: "9"}
	c.Set("sh600000", src)
	src.Price = "hack"
	got, ok := c.Get("sh600000")
	require.True(t, ok)
	require.Equal(t, "9", got.Price)
	got.Price = "mut"
	got2, _ := c.Get("sh600000")
	require.Equal(t, "9", got2.Price)
}
