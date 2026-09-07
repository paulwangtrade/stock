// Phase10-C.7-D TradePlan Lifecycle Runtime Harness (out-of-process).
// Uses an isolated SQLite file — never opens production build/bin/data/stock.db.
// Does not modify product source; only imports public packages / test hooks.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type caseResult struct {
	Name       string         `json:"name"`
	OK         bool           `json:"ok"`
	Error      string         `json:"error,omitempty"`
	PlanStatus string         `json:"planStatus"`
	Items      []itemSnap     `json:"items"`
	Orders     int64          `json:"orders"`
	Fills      int64          `json:"fills"`
	Positions  int64          `json:"positions"`
	Extras     map[string]any `json:"extras,omitempty"`
}

type itemSnap struct {
	Code   string  `json:"code"`
	Status string  `json:"status"`
	OrderID uint   `json:"orderId"`
	FillID uint    `json:"fillId"`
	Vol    int64   `json:"filledVolume"`
	Price  float64 `json:"filledPrice"`
	Error  string  `json:"error,omitempty"`
}

type harnessReport struct {
	Harness   string       `json:"harness"`
	DBPath    string       `json:"dbPath"`
	StartedAt string       `json:"startedAt"`
	Cases     []caseResult `json:"cases"`
	AllPass   bool         `json:"allPass"`
}

func main() {
	root := filepath.Clean(filepath.Join(".", "tmp", "c7d_lifecycle_harness"))
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	_ = os.RemoveAll(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		fail("mkdir harness root: %v", err)
	}
	dbPath := filepath.Join(root, "c7d_isolated.db")

	report := harnessReport{
		Harness:   "Phase10-C.7-D TradePlan Lifecycle Runtime Harness",
		DBPath:    dbPath,
		StartedAt: time.Now().Format(time.RFC3339),
		Cases:     make([]caseResult, 0, 4),
	}

	cases := []struct {
		name string
		fn   func(dbFile string) caseResult
	}{
		{"case1_all_filled_ready_to_done", runCase1},
		{"case2_partial_ready_to_partial", runCase2},
		{"case3_broker_error_ready_to_failed", runCase3},
		{"case4_duplicate_try_begin_blocked", runCase4},
	}

	allPass := true
	for i, c := range cases {
		file := filepath.Join(root, fmt.Sprintf("case%d.db", i+1))
		res := c.fn(file)
		res.Name = c.name
		report.Cases = append(report.Cases, res)
		if !res.OK {
			allPass = false
		}
		fmt.Fprintf(os.Stderr, "[C.7-D] %s ok=%v plan=%s orders=%d fills=%d positions=%d\n",
			res.Name, res.OK, res.PlanStatus, res.Orders, res.Fills, res.Positions)
	}
	report.AllPass = allPass

	outPath := filepath.Join(root, "results.json")
	b, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(outPath, b, 0o644)
	fmt.Println(string(b))

	if !allPass {
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

func openIsolatedDB(dbFile string) error {
	_ = os.Remove(dbFile)
	gdb, err := gorm.Open(sqlite.Open(dbFile+"?_busy_timeout=5000"), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)
	db.Dao = gdb
	if err := data.EnsureTradePlanTables(); err != nil {
		return err
	}
	if err := papertrading.EnsureSchema(db.Dao); err != nil {
		return err
	}
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	papertrading.SetRunForPlanHookForTest(nil)
	papertrading.SetBeforeTryBeginForTest(nil)
	return nil
}

func sessionANow() time.Time {
	return time.Date(2026, 8, 5, 10, 0, 0, 0, time.Local)
}

func seedFrozenPlan(tradeDate string, items []models.TradePlanItem) (*models.TradePlan, error) {
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		GeneratedAt:    now,
		Status:         models.TradePlanStatusReady,
		ApprovedAt:     &now,
		ApprovedBy:     "c7d-harness",
		FreezeAt:       &now,
		FreezeBy:       "c7d-harness",
		PlanVersion:    1,
		Side:           "buy",
		AmountPerStock: 100_000,
		MaxNames:       5,
	}
	if err := data.NewTradePlanRepo().CreatePlanWithItems(plan, items); err != nil {
		return nil, err
	}
	return data.NewTradePlanRepo().GetByID(plan.ID)
}

func buyItem(code, name string, vol int64) models.TradePlanItem {
	return models.TradePlanItem{
		StockCode: code, StockName: name, Side: "buy",
		Status: models.TradePlanItemPending, TargetVolume: vol, TargetAmount: 100_000, LimitPrice: 10,
	}
}

func snapshot(planID uint) (caseResult, error) {
	out := caseResult{Extras: map[string]any{}}
	got, err := data.NewTradePlanRepo().GetByID(planID)
	if err != nil {
		return out, err
	}
	out.PlanStatus = got.Status
	for _, it := range got.Items {
		out.Items = append(out.Items, itemSnap{
			Code: it.StockCode, Status: it.Status,
			OrderID: it.OrderID, FillID: it.FillID,
			Vol: it.FilledVolume, Price: it.FilledPrice, Error: it.Error,
		})
	}
	_ = db.Dao.Model(&papertrading.PaperSimOrder{}).Where("plan_id = ?", planID).Count(&out.Orders).Error
	_ = db.Dao.Model(&papertrading.PaperSimFill{}).Where("plan_id = ?", planID).Count(&out.Fills).Error
	var acc papertrading.PaperSimAccount
	if err := db.Dao.Where("name = ?", "paper_sim_default").First(&acc).Error; err == nil {
		_ = db.Dao.Model(&papertrading.PaperSimPosition{}).Where("account_id = ?", acc.ID).Count(&out.Positions).Error
		out.Extras["accountId"] = acc.ID
		out.Extras["cash"] = acc.Cash
	}
	if got.ExecutedAt != nil {
		out.Extras["executedAt"] = got.ExecutedAt.Format(time.RFC3339)
	}
	return out, nil
}

func assert(cond bool, msg string) error {
	if !cond {
		return errors.New(msg)
	}
	return nil
}

func runCase1(dbFile string) caseResult {
	res := caseResult{Name: "case1"}
	if err := openIsolatedDB(dbFile); err != nil {
		res.Error = err.Error()
		return res
	}
	plan, err := seedFrozenPlan("2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
		buyItem("sz000002", "万科A", 500),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if err := assert(plan.Status == models.TradePlanStatusReady, "precondition: ready"); err != nil {
		res.Error = err.Error()
		return res
	}

	exec, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0, LimitUp: 11},
			"sz000002": {Open: 8.0, LimitUp: 9},
		}},
		SkipWeekdayCheck: true, Now: sessionANow(),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	snap, err := snapshot(plan.ID)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res = snap
	res.Extras["runStatus"] = exec.Status
	res.Extras["filledCount"] = exec.FilledCount

	checks := []error{
		assert(exec.Status == papertrading.RunStatusCompleted, "run not completed"),
		assert(exec.FilledCount == 2, "filledCount!=2"),
		assert(res.PlanStatus == models.TradePlanStatusDone, "plan!=done"),
		assert(res.Orders == 2, "orders!=2"),
		assert(res.Fills == 2, "fills!=2"),
		assert(res.Positions == 2, "positions!=2"),
	}
	for _, it := range res.Items {
		checks = append(checks,
			assert(it.Status == models.TradePlanItemFilled, "item not filled: "+it.Code),
			assert(it.OrderID > 0, "item missing orderId: "+it.Code),
			assert(it.FillID > 0, "item missing fillId: "+it.Code),
			assert(it.Vol > 0, "item missing volume: "+it.Code),
		)
	}
	for _, e := range checks {
		if e != nil {
			res.Error = e.Error()
			res.OK = false
			return res
		}
	}
	res.OK = true
	return res
}

func runCase2(dbFile string) caseResult {
	res := caseResult{Name: "case2"}
	if err := openIsolatedDB(dbFile); err != nil {
		res.Error = err.Error()
		return res
	}
	plan, err := seedFrozenPlan("2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
		buyItem("sz000099", "无行情", 1000),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	exec, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0, LimitUp: 11},
		}},
		SkipWeekdayCheck: true, Now: sessionANow(),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	snap, err := snapshot(plan.ID)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res = snap
	res.Extras["runStatus"] = exec.Status
	res.Extras["filledCount"] = exec.FilledCount
	res.Extras["rejectCount"] = exec.RejectCount

	byCode := map[string]itemSnap{}
	for _, it := range res.Items {
		byCode[it.Code] = it
	}
	checks := []error{
		assert(exec.Status == papertrading.RunStatusCompletedWithRejects, "run status"),
		assert(exec.FilledCount == 1 && exec.RejectCount == 1, "fill/reject counts"),
		assert(res.PlanStatus == models.TradePlanStatusPartial, "plan!=partial"),
		assert(res.Orders == 2, "orders!=2"),
		assert(res.Fills == 1, "fills!=1"),
		assert(res.Positions == 1, "positions!=1"),
		assert(byCode["sz000001"].Status == models.TradePlanItemFilled, "sz000001 not filled"),
		assert(byCode["sz000099"].Status == models.TradePlanItemError, "sz000099 not error"),
		assert(byCode["sz000099"].Error == papertrading.RejectMissingOpenPrice, "reject reason"),
	}
	for _, e := range checks {
		if e != nil {
			res.Error = e.Error()
			res.OK = false
			return res
		}
	}
	res.OK = true
	return res
}

func runCase3(dbFile string) caseResult {
	res := caseResult{Name: "case3"}
	if err := openIsolatedDB(dbFile); err != nil {
		res.Error = err.Error()
		return res
	}
	defer papertrading.SetRunForPlanHookForTest(nil)

	plan, err := seedFrozenPlan("2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	papertrading.SetRunForPlanHookForTest(func() error {
		return errors.New("c7d simulated broker failure")
	})

	exec, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0},
		}},
		SkipWeekdayCheck: true, Now: sessionANow(),
	})
	if err == nil {
		res.Error = "expected broker error"
		return res
	}

	snap, errSnap := snapshot(plan.ID)
	if errSnap != nil {
		res.Error = errSnap.Error()
		return res
	}
	res = snap
	if exec != nil {
		res.Extras["runStatus"] = exec.Status
	}
	res.Extras["brokerError"] = err.Error()

	checks := []error{
		assert(res.PlanStatus == models.TradePlanStatusFailed, "plan!=failed"),
		assert(res.Orders == 0, "orders should be 0 on early broker fail"),
		assert(res.Fills == 0, "fills should be 0"),
		assert(res.Positions == 0, "positions should be 0"),
	}
	for _, e := range checks {
		if e != nil {
			res.Error = e.Error()
			res.OK = false
			return res
		}
	}
	res.OK = true
	return res
}

func runCase4(dbFile string) caseResult {
	res := caseResult{Name: "case4"}
	if err := openIsolatedDB(dbFile); err != nil {
		res.Error = err.Error()
		return res
	}
	defer papertrading.SetBeforeTryBeginForTest(nil)

	plan, err := seedFrozenPlan("2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	repo := data.NewTradePlanRepo()

	// Steal CAS before Job Begin — second TryBeginExecute inside Job must miss.
	papertrading.SetBeforeTryBeginForTest(func(planID uint) {
		ok, e := repo.TryBeginExecute(planID)
		if e != nil || !ok {
			panic(fmt.Sprintf("pre-hook begin failed ok=%v err=%v", ok, e))
		}
	})

	exec, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0},
		}},
		SkipWeekdayCheck: true, Now: sessionANow(),
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}

	snap, err := snapshot(plan.ID)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res = snap
	res.Extras["runStatus"] = exec.Status

	ok2, err2 := repo.TryBeginExecute(plan.ID)
	if err2 != nil {
		res.Error = err2.Error()
		res.OK = false
		return res
	}
	res.Extras["secondTryBeginOK"] = ok2

	checks := []error{
		assert(exec.Status == papertrading.RunStatusSkippedPlanLifecycle, "expected skipped_plan_lifecycle"),
		assert(res.PlanStatus == models.TradePlanStatusExecuting, "plan should stay executing after stolen begin"),
		assert(res.Orders == 0 && res.Fills == 0 && res.Positions == 0, "must not create ledger on CAS miss"),
		assert(!ok2, "second TryBeginExecute must be blocked"),
	}
	for _, e := range checks {
		if e != nil {
			res.Error = e.Error()
			res.OK = false
			return res
		}
	}
	res.OK = true
	return res
}
