package main

import (
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := gorm.Open(sqlite.Open(`D:/stock/build/bin/data/stock.db?mode=ro&_busy_timeout=8000`), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var n int
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_positions`).Scan(&n)
	fmt.Println("positions_count=", n)
	type pos struct {
		ID          uint
		AccountID   uint
		StockCode   string
		StockName   string
		TotalVolume int64
		AvgCost     float64
	}
	var ps []pos
	_ = db.Raw(`SELECT id, account_id, stock_code, COALESCE(stock_name,'') AS stock_name, total_volume, avg_cost FROM paper_sim_positions ORDER BY id`).Scan(&ps)
	for _, p := range ps {
		fmt.Printf("pos id=%d acct=%d code=%q name=%q vol=%d avg=%.4f\n", p.ID, p.AccountID, p.StockCode, p.StockName, p.TotalVolume, p.AvgCost)
	}
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders WHERE side!='buy'`).Scan(&n)
	fmt.Println("non_buy_orders=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_fills`).Scan(&n)
	fmt.Println("fills_total=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders`).Scan(&n)
	fmt.Println("orders_total=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_runs`).Scan(&n)
	fmt.Println("runs_total=", n)
	// name empty on plan items >=28
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE plan_id>=28 AND (stock_name IS NULL OR TRIM(stock_name)='')`).Scan(&n)
	fmt.Println("empty_name_plan_items_ge28=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE plan_id IN (32,33) AND (stock_name IS NULL OR TRIM(stock_name)='')`).Scan(&n)
	fmt.Println("empty_name_plan_32_33=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE plan_id=31 AND (stock_name IS NULL OR TRIM(stock_name)='')`).Scan(&n)
	fmt.Println("empty_name_plan_31=", n)
}
