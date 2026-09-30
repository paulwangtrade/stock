package exitwatch_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/exitwatch"
	"go-stock/backend/externalmirror"
	"go-stock/backend/marketdata"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIsolation_MirrorAdapterSources(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)

	forbiddenImports := map[string][]string{
		"external_mirror.go": {
			"go-stock/backend/papertrading",
			"go-stock/backend/models",
			"go-stock/backend/strategy",
			"go-stock/backend/execution",
			"go-stock/backend/data",
			"gorm.io/gorm",
		},
		"project.go": {
			"go-stock/backend/papertrading",
			"go-stock/backend/externalmirror",
			"go-stock/backend/models",
			"go-stock/backend/data",
			"gorm.io/gorm",
		},
	}
	forbiddenText := []string{
		"AutoMigrate",
		".Create(",
		".Save(",
		".Delete(",
		"models.TradePlan",
		"trade_plans",
		"trade_plan_items",
		"paper_sim_fills",
		"paper_sim_positions",
		"paper_sim_orders",
		"BuildDraft",
		"PaperBroker",
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, name))
		require.NoError(t, err)
		text := string(src)
		for _, bad := range forbiddenText {
			require.NotContains(t, text, bad, name)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		require.NoError(t, err)
		if blocked, ok := forbiddenImports[name]; ok {
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range blocked {
					require.NotEqual(t, bad, p, name)
				}
			}
		}
	}
}

type countQuotes struct {
	byCode map[string]marketdata.Quote
}

func (c countQuotes) GetQuote(code string) (*marketdata.Quote, error) {
	q, ok := c.byCode[strings.ToLower(code)]
	if !ok {
		return nil, nil
	}
	cp := q
	return &cp, nil
}

func (c countQuotes) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	out := make([]marketdata.Quote, 0, len(codes))
	for _, code := range codes {
		if q, ok := c.byCode[strings.ToLower(code)]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

func TestBuild_MirrorDoesNotWritePaperSimOrTradePlan(t *testing.T) {
	orig := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = orig
		_ = sqlDB.Close()
		externalmirror.SetNowForTest(nil)
		externalmirror.SetNameResolverForTest(nil)
	})

	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, externalmirror.EnsureSchema(db.Dao))
	externalmirror.SetNowForTest(func() time.Time { return time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local) })
	externalmirror.SetNameResolverForTest(func(string) string { return "平安" })

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 100_000, Cash: 80_000, Equity: 100_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	sim := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发银行",
		TotalVolume: 100, AvailableVolume: 100, AvgCost: 10, MarkPrice: 10,
	}
	require.NoError(t, db.Dao.Create(&sim).Error)
	plan := &models.TradePlan{TradeDate: "2026-09-30", Status: "draft", Side: "buy"}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{PlanID: plan.ID, StockCode: "sh600000", StockName: "浦发银行", TargetAmount: 1000}
	require.NoError(t, db.Dao.Create(item).Error)

	_, err = externalmirror.Create(externalmirror.Input{
		StockCode: "000001", Quantity: f64(500), CostPrice: f64(10), EntryDate: "2026-09-01",
	})
	require.NoError(t, err)

	before := tableCounts(t)
	var writes int
	require.NoError(t, db.Dao.Callback().Create().Before("gorm:create").Register("exitwatch_count_create", func(tx *gorm.DB) {
		writes++
	}))
	require.NoError(t, db.Dao.Callback().Update().Before("gorm:update").Register("exitwatch_count_update", func(tx *gorm.DB) {
		writes++
	}))
	require.NoError(t, db.Dao.Callback().Delete().Before("gorm:delete").Register("exitwatch_count_delete", func(tx *gorm.DB) {
		writes++
	}))
	t.Cleanup(func() {
		db.Dao.Callback().Create().Remove("exitwatch_count_create")
		db.Dao.Callback().Update().Remove("exitwatch_count_update")
		db.Dao.Callback().Delete().Remove("exitwatch_count_delete")
	})

	asOf := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	view := exitwatch.Build(exitwatch.Options{
		AsOf: asOf,
		Bar:  "2026-09-30",
		QuoteService: countQuotes{byCode: map[string]marketdata.Quote{
			"sz000001": {Code: "sz000001", Price: 8, FetchedAt: asOf.Add(-time.Minute)},
		}},
	})
	require.Zero(t, writes)
	require.Equal(t, before, tableCounts(t))

	var simRows []papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Find(&simRows).Error)
	require.Len(t, simRows, 1)
	require.Equal(t, "sh600000", simRows[0].StockCode)
	require.Equal(t, int64(100), simRows[0].TotalVolume)

	var mirrorHit *exitwatch.Item
	for i := range view.Items {
		it := &view.Items[i]
		require.True(t, it.NotAnOrder)
		if it.StockCode == "sz000001" {
			require.Equal(t, exitwatch.SourceExternalMirror, it.Source)
			require.False(t, it.SellIntentAllowed)
			mirrorHit = it
		}
		if it.Source == exitwatch.SourceExternalMirror {
			require.False(t, it.SellIntentAllowed)
		}
	}
	require.NotNil(t, mirrorHit)
	require.Equal(t, exitwatch.ClassFlatten, mirrorHit.Class)
	require.Contains(t, mirrorHit.ReasonCodes, exitwatch.ReasonLoss)
	require.NotContains(t, mirrorHit.ReasonCodes, exitwatch.ReasonPlan)
	require.Contains(t, mirrorHit.DedupKey, "external_mirror|")
	require.Contains(t, mirrorHit.PolicyRef, "@v")
	require.False(t, mirrorHit.AsOf.IsZero())
	require.False(t, view.WritesTradePlan)
	require.False(t, view.WritesPaperSim)
	require.False(t, view.AutoSell)
	require.False(t, view.PersistTrailing)
	require.False(t, view.NewLedger)
}

func f64(v float64) *float64 { return &v }

func tableCounts(t *testing.T) map[string]int64 {
	t.Helper()
	tables := []string{
		"paper_sim_accounts",
		"paper_sim_positions",
		"paper_sim_orders",
		"paper_sim_fills",
		"trade_plans",
		"trade_plan_items",
		"external_mirror_holdings",
	}
	out := map[string]int64{}
	for _, name := range tables {
		var n int64
		require.NoError(t, db.Dao.Table(name).Count(&n).Error)
		out[name] = n
	}
	return out
}
