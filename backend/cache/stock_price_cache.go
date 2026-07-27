package cache

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data"
)

const (
	// DefaultStockPriceTTL 行情缓存默认 TTL（秒级行情足够，且不拖慢 cron）。
	DefaultStockPriceTTL = 5 * time.Second
	minStockPriceTTL     = 100 * time.Millisecond
	maxStockPriceTTL     = 60 * time.Second
)

// StockPrice 缓存中的行情快照（与 data.StockInfo 对齐，便于透明复用）。
type StockPrice = data.StockInfo

type stockPriceEntry struct {
	price     *StockPrice
	updatedAt time.Time
}

// StockPriceCache 进程内行情缓存：按代码存快照，支持 TTL 与批量读写。
type StockPriceCache struct {
	mu        sync.RWMutex
	data      map[string]*stockPriceEntry
	ttl       time.Duration
	updatedAt time.Time // 最近一次成功写入时间（任意代码）
}

// FollowRealtimePriceCache 供自选实时行情（GetFollowRealtimeList）使用的全局实例。
var FollowRealtimePriceCache = NewStockPriceCache(ttlFromEnvOrDefault())

func ttlFromEnvOrDefault() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GOSTOCK_PRICE_CACHE_TTL_SEC"))
	if raw == "" {
		return DefaultStockPriceTTL
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return DefaultStockPriceTTL
	}
	d := time.Duration(sec) * time.Second
	return clampTTL(d)
}

func clampTTL(d time.Duration) time.Duration {
	if d < minStockPriceTTL {
		return minStockPriceTTL
	}
	if d > maxStockPriceTTL {
		return maxStockPriceTTL
	}
	return d
}

func NewStockPriceCache(ttl time.Duration) *StockPriceCache {
	if ttl <= 0 {
		ttl = DefaultStockPriceTTL
	}
	return &StockPriceCache{
		data: make(map[string]*stockPriceEntry),
		ttl:  clampTTL(ttl),
	}
}

// SetTTL 运行时调整 TTL（可配置）。
func (c *StockPriceCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl <= 0 {
		ttl = DefaultStockPriceTTL
	}
	c.ttl = clampTTL(ttl)
}

// TTL 返回当前 TTL。
func (c *StockPriceCache) TTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ttl
}

// NormalizeStockCode 统一缓存 key（与行情接口返回的 code 风格对齐）。
func NormalizeStockCode(code string) string {
	c := strings.TrimSpace(strings.ToLower(code))
	if strings.HasPrefix(c, "us") {
		c = "gb_" + strings.TrimPrefix(c, "us")
	}
	return c
}

func (c *StockPriceCache) clonePrice(p *StockPrice) *StockPrice {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// Get 返回未过期行情；过期或不存在返回 false。
func (c *StockPriceCache) Get(code string) (*StockPrice, bool) {
	key := NormalizeStockCode(code)
	if key == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	ent, ok := c.data[key]
	if !ok || ent == nil || ent.price == nil {
		return nil, false
	}
	if c.ttl > 0 && time.Since(ent.updatedAt) > c.ttl {
		return nil, false
	}
	return c.clonePrice(ent.price), true
}

// Set 写入单条行情。
func (c *StockPriceCache) Set(code string, price *StockPrice) {
	if price == nil {
		return
	}
	key := NormalizeStockCode(code)
	if key == "" {
		key = NormalizeStockCode(price.Code)
	}
	if key == "" {
		return
	}
	cp := c.clonePrice(price)
	cp.Code = key
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = &stockPriceEntry{price: cp, updatedAt: now}
	c.updatedAt = now
}

// GetBatch 返回仍新鲜的命中项（key 为规范化 code）。
func (c *StockPriceCache) GetBatch(codes []string) map[string]*StockPrice {
	out := make(map[string]*StockPrice)
	if len(codes) == 0 {
		return out
	}
	now := time.Now()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, code := range codes {
		key := NormalizeStockCode(code)
		if key == "" {
			continue
		}
		ent, ok := c.data[key]
		if !ok || ent == nil || ent.price == nil {
			continue
		}
		if c.ttl > 0 && now.Sub(ent.updatedAt) > c.ttl {
			continue
		}
		out[key] = c.clonePrice(ent.price)
	}
	return out
}

// SetBatch 批量写入。
func (c *StockPriceCache) SetBatch(prices map[string]*StockPrice) {
	if len(prices) == 0 {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for code, price := range prices {
		if price == nil {
			continue
		}
		key := NormalizeStockCode(code)
		if key == "" {
			key = NormalizeStockCode(price.Code)
		}
		if key == "" {
			continue
		}
		cp := c.clonePrice(price)
		cp.Code = key
		c.data[key] = &stockPriceEntry{price: cp, updatedAt: now}
	}
	c.updatedAt = now
}

// SetBatchFromSlice 从行情切片写入缓存。
func (c *StockPriceCache) SetBatchFromSlice(list []StockPrice) {
	if len(list) == 0 {
		return
	}
	m := make(map[string]*StockPrice, len(list))
	for i := range list {
		p := list[i]
		key := NormalizeStockCode(p.Code)
		if key == "" {
			continue
		}
		cp := p
		m[key] = &cp
	}
	c.SetBatch(m)
}

// IsStale 判断缓存整体是否过期（无数据，或最近写入已超过 TTL）。
func (c *StockPriceCache) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.data) == 0 || c.updatedAt.IsZero() {
		return true
	}
	if c.ttl <= 0 {
		return false
	}
	return time.Since(c.updatedAt) > c.ttl
}

// Partition 将 codes 拆成命中 map 与未命中列表（规范化后去重）。
func (c *StockPriceCache) Partition(codes []string) (hits map[string]*StockPrice, misses []string) {
	hits = c.GetBatch(codes)
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		key := NormalizeStockCode(code)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if _, ok := hits[key]; !ok {
			misses = append(misses, key)
		}
	}
	return hits, misses
}

// Clear 清空缓存（测试用）。
func (c *StockPriceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*stockPriceEntry)
	c.updatedAt = time.Time{}
}

// Len 当前条目数（含可能已逻辑过期但仍驻留的项）。
func (c *StockPriceCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
