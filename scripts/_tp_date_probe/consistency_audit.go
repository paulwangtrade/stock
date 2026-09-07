package main

import (
	"fmt"
	"os"
	"strings"

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

	fmt.Println("==== 2099 / smoke residue")
	var n2099, nSmoke int
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plans WHERE trade_date LIKE '2099%'`).Scan(&n2099)
	_ = db.Raw(`SELECT COUNT(*) FROM candidate_pools WHERE message LIKE '%smoke%' OR trade_date LIKE '2099%'`).Scan(&nSmoke)
	fmt.Printf("trade_plans_2099=%d candidate_smoke_or_2099=%d\n", n2099, nSmoke)

	fmt.Println("==== recent plans (id>=28)")
	type Plan struct {
		ID uint
		TradeDate, Status, SourceSession string
		PlanVersion                      int
	}
	var plans []Plan
	_ = db.Raw(`SELECT id, trade_date, status, source_session, plan_version FROM trade_plans WHERE id>=28 ORDER BY id`).Scan(&plans)
	for _, p := range plans {
		fmt.Printf("plan id=%d td=%s status=%s src=%s v=%d\n", p.ID, p.TradeDate, p.Status, p.SourceSession, p.PlanVersion)
		type It struct {
			ID uint
			StockCode, StockName, TradeDate, Status string
		}
		var items []It
		_ = db.Raw(`SELECT id, stock_code, stock_name, trade_date, status FROM trade_plan_items WHERE plan_id=? ORDER BY id`, p.ID).Scan(&items)
		emptyName := 0
		for _, it := range items {
			if strings.TrimSpace(it.StockName) == "" {
				emptyName++
			}
			fmt.Printf("  item id=%d code=%q name=%q td=%s status=%s match_plan_td=%v\n",
				it.ID, it.StockCode, it.StockName, it.TradeDate, it.Status, it.TradeDate == p.TradeDate)
		}
		fmt.Printf("  empty_stock_name=%d / %d\n", emptyName, len(items))
	}

	fmt.Println("==== paper_sim_runs recent")
	type Run struct {
		ID, PlanID, AccountID uint
		TradeDate, Trigger, Actor, Status, ExecutionID string
	}
	var runs []Run
	_ = db.Raw(`SELECT id, plan_id, account_id, trade_date, trigger, actor, status, execution_id FROM paper_sim_runs ORDER BY id DESC LIMIT 15`).Scan(&runs)
	for _, r := range runs {
		fmt.Printf("run id=%d plan=%d td=%s trigger=%s actor=%s status=%s exec=%s\n",
			r.ID, r.PlanID, r.TradeDate, r.Trigger, r.Actor, r.Status, r.ExecutionID)
	}

	fmt.Println("==== join consistency: latest runs → orders/fills vs plan items")
	for _, r := range runs {
		if r.ID == 0 {
			continue
		}
		var planTD string
		_ = db.Raw(`SELECT trade_date FROM trade_plans WHERE id=?`, r.PlanID).Scan(&planTD)
		type Ord struct {
			ID uint
			StockCode, StockName, TradeDate, Status string
			PlanID, PlanItemID                      uint
		}
		var ords []Ord
		_ = db.Raw(`SELECT id, stock_code, stock_name, trade_date, status, plan_id, plan_item_id FROM paper_sim_orders WHERE plan_id=? AND trade_date=? ORDER BY id`, r.PlanID, r.TradeDate).Scan(&ords)
		fmt.Printf("-- run#%d plan=%d run_td=%s plan_td=%s orders=%d td_match=%v\n",
			r.ID, r.PlanID, r.TradeDate, planTD, len(ords), r.TradeDate == planTD || planTD == "")
		for _, o := range ords {
			var itemName, itemCode, itemTD string
			_ = db.Raw(`SELECT stock_code, stock_name, trade_date FROM trade_plan_items WHERE id=?`, o.PlanItemID).Scan(&struct {
				// placeholder
			}{})
			_ = db.Raw(`SELECT stock_code, stock_name, trade_date FROM trade_plan_items WHERE id=?`, o.PlanItemID).Row().Scan(&itemCode, &itemName, &itemTD)
			var fillN int
			var fillReason string
			_ = db.Raw(`SELECT COUNT(*), COALESCE(MAX(fill_reason),'') FROM paper_sim_fills WHERE order_id=?`, o.ID).Row().Scan(&fillN, &fillReason)
			nameOK := strings.TrimSpace(o.StockName) != "" || strings.TrimSpace(itemName) != ""
			codeOK := o.StockCode == itemCode || itemCode == ""
			fmt.Printf("   ord#%d code=%q name=%q item_code=%q item_name=%q fills=%d reason=%s codeOK=%v name_nonempty_either=%v\n",
				o.ID, o.StockCode, o.StockName, itemCode, itemName, fillN, fillReason, codeOK, nameOK)
		}
	}

	fmt.Println("==== paper_sim_positions sample")
	type Pos struct {
		ID, AccountID uint
		StockCode, StockName string
		Volume               int64
	}
	var pos []Pos
	_ = db.Raw(`SELECT id, account_id, stock_code, stock_name, volume FROM paper_sim_positions WHERE volume!=0 ORDER BY id DESC LIMIT 20`).Scan(&pos)
	for _, p := range pos {
		fmt.Printf("pos id=%d acct=%d code=%q name=%q vol=%d\n", p.ID, p.AccountID, p.StockCode, p.StockName, p.Volume)
	}

	fmt.Println("==== legacy baseline fills (known)")
	type Fill struct {
		ID, OrderID uint
		StockCode, FillReason string
	}
	var fills []Fill
	_ = db.Raw(`SELECT f.id, f.order_id, f.stock_code, f.fill_reason FROM paper_sim_fills f
		JOIN paper_sim_orders o ON o.id=f.order_id
		WHERE o.plan_id=26 OR f.id BETWEEN 6 AND 10
		ORDER BY f.id`).Scan(&fills)
	for _, f := range fills {
		fmt.Printf("legacy_fill id=%d ord=%d code=%s reason=%s\n", f.ID, f.OrderID, f.StockCode, f.FillReason)
	}

	fmt.Println("==== empty stock_name on recent plan items (id>=30)")
	var emptyRecent int
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE plan_id>=30 AND TRIM(COALESCE(stock_name,''))=''`).Scan(&emptyRecent)
	fmt.Printf("empty_name_plan_items_ge30=%d\n", emptyRecent)

	fmt.Println("==== plan32 (post SHORT_NAME fix) detail")
	var p32 []struct {
		StockCode, StockName, TradeDate string
	}
	_ = db.Raw(`SELECT stock_code, stock_name, trade_date FROM trade_plan_items WHERE plan_id=32`).Scan(&p32)
	for _, it := range p32 {
		fmt.Printf("plan32 %s name=%q td=%s\n", it.StockCode, it.StockName, it.TradeDate)
	}
}
