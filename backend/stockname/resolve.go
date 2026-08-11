// Package stockname resolves a display name for a stock code.
// It is shared by PaperBroker, observation enrich, and maintenance repair jobs.
// It does not import maint or papertrading (Broker must not depend on maint).
package stockname

import (
	"errors"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

const (
	SourceTradePlan     = "trade_plan"
	SourceCandidatePool = "candidate_pool"
	SourceUniverse      = "universe"
	SourceQuote         = "quote"
	SourceFollowed      = "followed"
	SourceTushare       = "tushare"
	SourceUnknown       = "unknown"

	// UnknownName is the non-empty sentinel when no source yields a name.
	UnknownName = "未知名称"
)

// ErrUnresolved is returned when every lookup misses (Name is still UnknownName).
var ErrUnresolved = errors.New("stock name unresolved")

// Hint narrows lookups (optional).
type Hint struct {
	PlanID     uint
	PoolID     uint
	PlanItemID uint
}

// Result is the resolver outcome. Name is never empty.
type Result struct {
	Name   string
	Source string
	Err    error
}

// UniverseLookup optional cache (e.g. strategy universe). Nil → skipped.
var UniverseLookup func(code string) string

// QuoteLookup optional quote name. Nil → data.GetQuoteService if available.
var QuoteLookup func(code string) string

type resolveFunc func(gdb *gorm.DB, symbol string, hint Hint, logUnresolved bool) Result

var testOverride resolveFunc

// SetResolveForTest injects a resolver (unit tests). Pass nil to restore.
func SetResolveForTest(fn func(symbol string, hint Hint) Result) {
	if fn == nil {
		testOverride = nil
		return
	}
	testOverride = func(_ *gorm.DB, symbol string, hint Hint, _ bool) Result {
		return fn(symbol, hint)
	}
}

// Resolve uses db.Dao and no hint. Name is never "".
func Resolve(symbol string) Result {
	return ResolveWithHint(symbol, Hint{})
}

// ResolveWithHint uses db.Dao.
func ResolveWithHint(symbol string, hint Hint) Result {
	return resolve(db.Dao, symbol, hint, true)
}

// ResolveWithDB is the injectable/DB entry used by repair jobs and tests.
func ResolveWithDB(gdb *gorm.DB, symbol string, hint Hint) Result {
	return resolve(gdb, symbol, hint, false)
}

func resolve(gdb *gorm.DB, symbol string, hint Hint, logUnresolved bool) Result {
	if testOverride != nil {
		out := testOverride(gdb, symbol, hint, logUnresolved)
		return ensureName(out)
	}
	code := strings.TrimSpace(symbol)
	if code == "" {
		return unresolved(code, logUnresolved)
	}
	keys := CodeKeys(code)

	if n := lookupPlanItem(gdb, hint, keys); n != "" {
		return Result{Name: n, Source: SourceTradePlan}
	}
	if n := lookupCandidatePool(gdb, hint, keys); n != "" {
		return Result{Name: n, Source: SourceCandidatePool}
	}
	if n := lookupUniverse(code, keys); n != "" {
		return Result{Name: n, Source: SourceUniverse}
	}
	if n := lookupQuote(code); n != "" {
		return Result{Name: n, Source: SourceQuote}
	}
	if n := lookupFollowed(gdb, keys); n != "" {
		return Result{Name: n, Source: SourceFollowed}
	}
	if n := lookupTushare(gdb, code, keys); n != "" {
		return Result{Name: n, Source: SourceTushare}
	}
	return unresolved(code, logUnresolved)
}

func ensureName(r Result) Result {
	if strings.TrimSpace(r.Name) == "" {
		r.Name = UnknownName
		if r.Source == "" {
			r.Source = SourceUnknown
		}
		if r.Err == nil {
			r.Err = ErrUnresolved
		}
	}
	return r
}

func unresolved(symbol string, logUnresolved bool) Result {
	if logUnresolved && logger.SugaredLogger != nil {
		logger.SugaredLogger.Errorf(
			"StockNameResolver unresolved symbol=%s source=%s name=%s",
			symbol, SourceUnknown, UnknownName,
		)
	}
	return Result{Name: UnknownName, Source: SourceUnknown, Err: ErrUnresolved}
}

func lookupPlanItem(gdb *gorm.DB, hint Hint, keys []string) string {
	if gdb == nil || len(keys) == 0 {
		return ""
	}
	if hint.PlanItemID > 0 {
		var it models.TradePlanItem
		if err := gdb.Select("id", "stock_name").First(&it, hint.PlanItemID).Error; err == nil {
			if n := strings.TrimSpace(it.StockName); n != "" && n != UnknownName {
				return n
			}
		}
	}
	named := func(extra func(*gorm.DB) *gorm.DB) string {
		var items []models.TradePlanItem
		q := gdb.Model(&models.TradePlanItem{}).
			Select("id", "plan_id", "stock_code", "stock_name").
			Where("stock_name IS NOT NULL AND TRIM(stock_name) != '' AND TRIM(stock_name) != ?", UnknownName).
			Where("LOWER(TRIM(stock_code)) IN ?", keys)
		if extra != nil {
			q = extra(q)
		}
		if err := q.Order("id DESC").Limit(5).Find(&items).Error; err != nil {
			return ""
		}
		for _, it := range items {
			if n := strings.TrimSpace(it.StockName); n != "" {
				return n
			}
		}
		return ""
	}
	if hint.PlanID > 0 {
		if n := named(func(q *gorm.DB) *gorm.DB {
			return q.Where("plan_id = ?", hint.PlanID)
		}); n != "" {
			return n
		}
	}
	return named(nil)
}

func lookupCandidatePool(gdb *gorm.DB, hint Hint, keys []string) string {
	if gdb == nil || len(keys) == 0 {
		return ""
	}
	named := func(extra func(*gorm.DB) *gorm.DB) string {
		var items []models.CandidatePoolItem
		q := gdb.Model(&models.CandidatePoolItem{}).
			Select("id", "pool_id", "stock_code", "stock_name").
			Where("stock_name IS NOT NULL AND TRIM(stock_name) != ''").
			Where("LOWER(TRIM(stock_code)) IN ?", keys)
		if extra != nil {
			q = extra(q)
		}
		if err := q.Order("id DESC").Limit(5).Find(&items).Error; err != nil {
			return ""
		}
		for _, it := range items {
			if n := strings.TrimSpace(it.StockName); n != "" {
				return n
			}
		}
		return ""
	}
	if hint.PoolID > 0 {
		if n := named(func(q *gorm.DB) *gorm.DB {
			return q.Where("pool_id = ?", hint.PoolID)
		}); n != "" {
			return n
		}
	}
	return named(nil)
}

func lookupUniverse(code string, keys []string) string {
	if UniverseLookup == nil {
		return ""
	}
	if n := strings.TrimSpace(UniverseLookup(code)); n != "" {
		return n
	}
	for _, k := range keys {
		if n := strings.TrimSpace(UniverseLookup(k)); n != "" {
			return n
		}
	}
	return ""
}

func lookupQuote(code string) string {
	fn := QuoteLookup
	if fn == nil {
		fn = defaultQuoteLookup
	}
	if fn == nil {
		return ""
	}
	return strings.TrimSpace(fn(code))
}

func defaultQuoteLookup(code string) string {
	svc := data.GetQuoteService()
	if svc == nil {
		return ""
	}
	q, err := svc.GetQuote(code)
	if err != nil || q == nil {
		return ""
	}
	return strings.TrimSpace(q.Name)
}

func lookupFollowed(gdb *gorm.DB, keys []string) string {
	if gdb == nil || len(keys) == 0 {
		return ""
	}
	var rows []data.FollowedStock
	if err := gdb.Table("followed_stock").
		Select("stock_code", "name").
		Where("name IS NOT NULL AND TRIM(name) != ''").
		Where("LOWER(TRIM(stock_code)) IN ?", keys).
		Limit(5).
		Find(&rows).Error; err != nil {
		return ""
	}
	for _, r := range rows {
		if n := strings.TrimSpace(r.Name); n != "" {
			return n
		}
	}
	return ""
}

func lookupTushare(gdb *gorm.DB, code string, keys []string) string {
	if gdb == nil {
		return ""
	}
	symbols := make([]string, 0, 4)
	seen := map[string]bool{}
	if norm, err := data.NormalizeStockCode(code); err == nil {
		if s := strings.TrimSpace(norm.Symbol); s != "" {
			symbols = append(symbols, s)
			seen[s] = true
		}
	}
	for _, k := range keys {
		if norm, err := data.NormalizeStockCode(k); err == nil {
			s := strings.TrimSpace(norm.Symbol)
			if s != "" && !seen[s] {
				seen[s] = true
				symbols = append(symbols, s)
			}
		}
	}
	if len(symbols) == 0 {
		return ""
	}
	var basics []data.StockBasic
	if err := gdb.Model(&data.StockBasic{}).
		Select("symbol", "name").
		Where("symbol IN ?", symbols).
		Find(&basics).Error; err != nil {
		return ""
	}
	bySym := map[string]string{}
	for _, b := range basics {
		sym := strings.TrimSpace(b.Symbol)
		name := strings.TrimSpace(b.Name)
		if sym != "" && name != "" {
			bySym[sym] = name
		}
	}
	for _, s := range symbols {
		if n := bySym[s]; n != "" {
			return n
		}
	}
	return ""
}
