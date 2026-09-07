package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingcalendar"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	path := `D:/stock/build/bin/data/stock.db`
	gdb, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=15000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	db.Dao = gdb

	// Inspect latest enabled strategy run for target codes.
	api := data.NewStockStrategyApi()
	strat, err := api.GetFirstEnabled()
	if err != nil || strat == nil {
		fmt.Fprintln(os.Stderr, "no enabled strategy:", err)
		os.Exit(1)
	}
	run, err := api.GetLatestRun(strat.ID)
	if err != nil || run == nil {
		fmt.Fprintln(os.Stderr, "no latest run:", err)
		os.Exit(1)
	}
	fmt.Printf("strategy id=%d name=%q latest_run=%d created=%s\n", strat.ID, strat.Name, run.ID, run.CreatedAt.Format(time.RFC3339))

	var view models.StockStrategyRunView
	_ = json.Unmarshal([]byte(run.ResultJSON), &view)
	raw, _ := json.Marshal(view.DataList)
	var rows []map[string]any
	_ = json.Unmarshal(raw, &rows)
	has363, has677 := false, false
	for _, row := range rows {
		blob, _ := json.Marshal(row)
		s := string(blob)
		if strings.Contains(s, "600363") {
			has363 = true
			fmt.Println("run_row_600363:", nameKeys(row))
		}
		if strings.Contains(s, "301677") {
			has677 = true
			fmt.Println("run_row_301677:", nameKeys(row))
		}
	}
	fmt.Printf("run_has_600363=%v run_has_301677=%v dataList_len=%d\n", has363, has677, len(rows))
	if !has363 || !has677 {
		fmt.Fprintln(os.Stderr, "latest run missing target codes; cannot validate specific symbols via full generate")
		os.Exit(2)
	}

	sourceDate := time.Now().Format("2006-01-02")
	// Prefer a weekday source so next trading day is well-defined.
	if d, err := tradingcalendar.ParseDate(sourceDate); err == nil && !tradingcalendar.IsTradingDay(d) {
		// use previous trading day as source if today is weekend
		if prev, err := tradingcalendar.PrevTradingDayString(sourceDate); err == nil {
			sourceDate = prev
		}
	}
	fmt.Println("source_date=", sourceDate)

	res, err := strategy.RunAfterClosePlanWorkflow(sourceDate)
	if res != nil {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println("workflow_result=", string(b))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow err:", err)
		// still inspect if plan created
	}
	if res == nil || res.TradePlanID == 0 {
		fmt.Fprintln(os.Stderr, "no trade plan id")
		os.Exit(1)
	}

	type Item struct {
		ID        uint
		StockCode string
		StockName string
		Status    string
	}
	var items []Item
	if err := gdb.Raw(`SELECT id, stock_code, stock_name, status FROM trade_plan_items WHERE plan_id=? ORDER BY id`, res.TradePlanID).Scan(&items).Error; err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("==== trade_plan_items plan_id=", res.TradePlanID)
	ok363, ok677 := false, false
	for _, it := range items {
		fmt.Printf("  id=%d code=%q name=%q status=%s\n", it.ID, it.StockCode, it.StockName, it.Status)
		if it.StockCode == "sh600363" && strings.TrimSpace(it.StockName) != "" {
			ok363 = true
		}
		if it.StockCode == "sz301677" && strings.TrimSpace(it.StockName) != "" {
			ok677 = true
		}
	}

	var poolItems []Item
	_ = gdb.Raw(`SELECT id, stock_code, stock_name, '' as status FROM candidate_pool_items WHERE pool_id=? ORDER BY id`, res.CandidatePoolID).Scan(&poolItems)
	fmt.Println("==== candidate_pool_items pool_id=", res.CandidatePoolID)
	for _, it := range poolItems {
		fmt.Printf("  id=%d code=%q name=%q\n", it.ID, it.StockCode, it.StockName)
	}

	// Prove names came from persistence, not API enrich: DB columns non-empty.
	fmt.Printf("DB_sh600363_nonempty=%v DB_sz301677_nonempty=%v\n", ok363, ok677)
	if !ok363 || !ok677 {
		os.Exit(3)
	}
	fmt.Println("RUNTIME_VALIDATION_PASS=true")
}

func nameKeys(row map[string]any) string {
	parts := []string{}
	for _, k := range []string{"SECURITY_CODE", "SECURITY_SHORT_NAME", "SECURITY_NAME_ABBR", "security_short_name", "name"} {
		if v, ok := row[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
