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
	dsn := `D:/stock/build/bin/data/stock.db?mode=ro&_busy_timeout=8000`
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var n int64
	fmt.Println("==== counts")
	for _, t := range []string{
		"trade_plans", "trade_plan_items", "candidate_pools",
		"paper_sim_runs", "paper_sim_orders", "paper_sim_fills", "paper_sim_positions",
	} {
		_ = db.Raw("SELECT COUNT(*) FROM " + t).Scan(&n)
		fmt.Printf("%s=%d\n", t, n)
	}

	fmt.Println("==== trade_plans integrity")
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plans WHERE pool_id IS NULL OR pool_id=0`).Scan(&n)
	fmt.Println("plans_missing_pool_id=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plans p LEFT JOIN candidate_pools c ON c.id=p.pool_id WHERE p.pool_id>0 AND c.id IS NULL`).Scan(&n)
	fmt.Println("plans_orphan_pool_fk=", n)

	type planRow struct {
		ID, PoolID, PlanVersion                                         uint
		TradeDate, Status, SourceSession, RiskStatus                    string
		FreezeAt, ApprovedAt                                            *string
		HasFreeze, HasApprove                                           int
	}
	var plans []planRow
	_ = db.Raw(`
SELECT id, pool_id, plan_version, trade_date, status, COALESCE(source_session,'') source_session,
       COALESCE(risk_status,'') risk_status,
       CASE WHEN freeze_at IS NOT NULL AND TRIM(CAST(freeze_at AS TEXT))!='' THEN 1 ELSE 0 END has_freeze,
       CASE WHEN approved_at IS NOT NULL AND TRIM(CAST(approved_at AS TEXT))!='' THEN 1 ELSE 0 END has_approve,
       CAST(freeze_at AS TEXT) freeze_at, CAST(approved_at AS TEXT) approved_at
FROM trade_plans ORDER BY id`).Scan(&plans)
	readyNoFreeze, draftWithFreeze, readyNoApprove, frozenOK := 0, 0, 0, 0
	for _, p := range plans {
		fmt.Printf("plan id=%d pool=%d td=%s status=%s src=%s v=%d approve=%d freeze=%d\n",
			p.ID, p.PoolID, p.TradeDate, p.Status, p.SourceSession, p.PlanVersion, p.HasApprove, p.HasFreeze)
		if p.Status == "ready" && p.HasFreeze == 0 {
			readyNoFreeze++
		}
		if p.Status == "draft" && p.HasFreeze == 1 {
			draftWithFreeze++
		}
		if p.Status == "ready" && p.HasFreeze == 1 && p.HasApprove == 0 {
			readyNoApprove++
		}
		if p.Status == "ready" && p.HasFreeze == 1 {
			frozenOK++
		}
	}
	fmt.Printf("ready_without_freeze=%d draft_with_freeze=%d ready_frozen_without_approve=%d frozen_ready_ok=%d\n",
		readyNoFreeze, draftWithFreeze, readyNoApprove, frozenOK)

	fmt.Println("==== trade_plan_items integrity")
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items`).Scan(&n)
	fmt.Println("items_total=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE stock_code IS NULL OR TRIM(stock_code)=''`).Scan(&n)
	fmt.Println("items_empty_stock_code=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE stock_name IS NULL OR TRIM(stock_name)=''`).Scan(&n)
	fmt.Println("items_empty_stock_name=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items i LEFT JOIN trade_plans p ON p.id=i.plan_id WHERE p.id IS NULL`).Scan(&n)
	fmt.Println("items_orphan_plan_fk=", n)

	type emptyName struct {
		PlanID uint
		Cnt    int64
	}
	var empties []emptyName
	_ = db.Raw(`
SELECT plan_id, COUNT(*) cnt FROM trade_plan_items
WHERE stock_name IS NULL OR TRIM(stock_name)=''
GROUP BY plan_id ORDER BY plan_id`).Scan(&empties)
	for _, e := range empties {
		fmt.Printf("empty_name plan_id=%d items=%d\n", e.PlanID, e.Cnt)
	}

	fmt.Println("==== paper_sim integrity")
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders o WHERE NOT EXISTS (
SELECT 1 FROM paper_sim_runs r WHERE r.plan_id=o.plan_id AND r.trade_date=o.trade_date)`).Scan(&n)
	fmt.Println("orders_without_matching_run=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_fills f LEFT JOIN paper_sim_orders o ON o.id=f.order_id WHERE o.id IS NULL`).Scan(&n)
	fmt.Println("fills_orphan_order_fk=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders o WHERE NOT EXISTS (SELECT 1 FROM paper_sim_fills f WHERE f.order_id=o.id) AND o.status='filled'`).Scan(&n)
	fmt.Println("filled_orders_without_fill=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders o LEFT JOIN trade_plans p ON p.id=o.plan_id WHERE p.id IS NULL`).Scan(&n)
	fmt.Println("orders_orphan_plan_fk=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders o LEFT JOIN trade_plan_items i ON i.id=o.plan_item_id WHERE i.id IS NULL`).Scan(&n)
	fmt.Println("orders_orphan_item_fk=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_fills f LEFT JOIN trade_plans p ON p.id=f.plan_id WHERE p.id IS NULL`).Scan(&n)
	fmt.Println("fills_orphan_plan_fk=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_fills f LEFT JOIN trade_plan_items i ON i.id=f.plan_item_id WHERE i.id IS NULL`).Scan(&n)
	fmt.Println("fills_orphan_item_fk=", n)
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_runs r LEFT JOIN trade_plans p ON p.id=r.plan_id WHERE p.id IS NULL`).Scan(&n)
	fmt.Println("runs_orphan_plan_fk=", n)

	// plan_id/item_id mismatch between fill and order
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_fills f JOIN paper_sim_orders o ON o.id=f.order_id
WHERE f.plan_id!=o.plan_id OR f.plan_item_id!=o.plan_item_id`).Scan(&n)
	fmt.Println("fill_order_plan_item_mismatch=", n)

	// order vs item stock_code
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_orders o JOIN trade_plan_items i ON i.id=o.plan_item_id
WHERE LOWER(TRIM(o.stock_code))!=LOWER(TRIM(i.stock_code))`).Scan(&n)
	fmt.Println("order_item_stock_code_mismatch=", n)

	fmt.Println("==== position vs fills derivation")
	type pos struct {
		StockCode string
		Vol       int64
	}
	var positions []pos
	_ = db.Raw(`SELECT stock_code, total_volume AS vol FROM paper_sim_positions ORDER BY stock_code`).Scan(&positions)
	type fillAgg struct {
		StockCode string
		Vol       int64
	}
	var fillAggs []fillAgg
	_ = db.Raw(`SELECT LOWER(TRIM(stock_code)) stock_code, SUM(volume) vol FROM paper_sim_fills
WHERE LOWER(TRIM(side))='buy' OR side='' OR side IS NULL
GROUP BY LOWER(TRIM(stock_code))`).Scan(&fillAggs)
	fillMap := map[string]int64{}
	for _, f := range fillAggs {
		fillMap[strings.ToLower(strings.TrimSpace(f.StockCode))] = f.Vol
	}
	posOnly, volMismatch, okMatch := 0, 0, 0
	posMap := map[string]int64{}
	for _, p := range positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		posMap[code] = p.Vol
		fv, ok := fillMap[code]
		if !ok {
			posOnly++
			fmt.Printf("ORPHAN_POS code=%s pos_vol=%d no_buy_fills\n", code, p.Vol)
			continue
		}
		if fv != p.Vol {
			volMismatch++
			fmt.Printf("POS_FILL_VOL_MISMATCH code=%s pos=%d fills_buy=%d\n", code, p.Vol, fv)
		} else {
			okMatch++
			fmt.Printf("pos_ok code=%s vol=%d\n", code, p.Vol)
		}
	}
	fillOnly := 0
	for code, fv := range fillMap {
		if _, ok := posMap[code]; !ok {
			fillOnly++
			fmt.Printf("ORPHAN_FILL_AGG code=%s buy_vol=%d no_position\n", code, fv)
		}
	}
	fmt.Printf("pos_fill_match=%d vol_mismatch=%d pos_without_fills=%d fills_without_pos=%d\n",
		okMatch, volMismatch, posOnly, fillOnly)

	fmt.Println("==== orphan detail lists")
	type idRow struct {
		ID uint
	}
	var ids []idRow
	_ = db.Raw(`SELECT o.id FROM paper_sim_orders o WHERE NOT EXISTS (
SELECT 1 FROM paper_sim_runs r WHERE r.plan_id=o.plan_id AND r.trade_date=o.trade_date)`).Scan(&ids)
	fmt.Print("orphan_orders_no_run ids=")
	for i, r := range ids {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print(r.ID)
	}
	fmt.Println()
	ids = nil
	_ = db.Raw(`SELECT f.id FROM paper_sim_fills f LEFT JOIN paper_sim_orders o ON o.id=f.order_id WHERE o.id IS NULL`).Scan(&ids)
	fmt.Print("orphan_fills_no_order ids=")
	for i, r := range ids {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print(r.ID)
	}
	fmt.Println()

	// runs without orders
	_ = db.Raw(`SELECT COUNT(*) FROM paper_sim_runs r WHERE NOT EXISTS (
SELECT 1 FROM paper_sim_orders o WHERE o.plan_id=r.plan_id AND o.trade_date=r.trade_date)`).Scan(&n)
	fmt.Println("runs_without_orders=", n)
	var runIDs []idRow
	_ = db.Raw(`SELECT r.id FROM paper_sim_runs r WHERE NOT EXISTS (
SELECT 1 FROM paper_sim_orders o WHERE o.plan_id=r.plan_id AND o.trade_date=r.trade_date)`).Scan(&runIDs)
	fmt.Print("runs_without_orders ids=")
	for i, r := range runIDs {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print(r.ID)
	}
	fmt.Println()

	// item order_id/fill_id populated?
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE order_id>0 OR fill_id>0`).Scan(&n)
	fmt.Println("items_with_order_or_fill_backref=", n)

	fmt.Println("==== done")
}
