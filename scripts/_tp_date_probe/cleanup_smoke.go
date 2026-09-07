package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := `D:/stock/build/bin/data/stock.db`
	if len(os.Args) > 1 && os.Args[1] == "cleanup" {
		runCleanup(dsn)
		return
	}
	runProbe(dsn)
}

func open(dsn string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return db
}

func runProbe(dsn string) {
	db := open(dsn + "?mode=ro&_busy_timeout=5000")
	type n struct{ N int }
	count := func(sql string, args ...any) int {
		var x n
		_ = db.Raw(sql, args...).Scan(&x)
		return x.N
	}
	fmt.Println("==== plan 22/23 refs")
	tables := []string{}
	_ = db.Raw(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`).Scan(&tables)
	for _, t := range tables {
		// find columns mentioning plan
		var cols []struct{ Name string }
		_ = db.Raw(`PRAGMA table_info(` + t + `)`).Scan(&cols)
		var planCols []string
		for _, c := range cols {
			ln := strings.ToLower(c.Name)
			if strings.Contains(ln, "plan_id") || ln == "trade_plan_id" {
				planCols = append(planCols, c.Name)
			}
		}
		for _, c := range planCols {
			n22 := count(fmt.Sprintf(`SELECT COUNT(*) AS n FROM "%s" WHERE "%s"=22`, t, c))
			n23 := count(fmt.Sprintf(`SELECT COUNT(*) AS n FROM "%s" WHERE "%s"=23`, t, c))
			if n22+n23 > 0 {
				fmt.Printf("%s.%s plan22=%d plan23=%d\n", t, c, n22, n23)
			}
		}
	}
	fmt.Println("==== pools 21/22")
	type P struct {
		ID uint
		TradeDate, Status, Message, ConfigJSON string
	}
	var pools []P
	_ = db.Raw(`SELECT id, trade_date, status, message, config_json FROM candidate_pools WHERE id IN (21,22)`).Scan(&pools)
	for _, p := range pools {
		fmt.Printf("%+v\n", p)
	}
	fmt.Println("==== upcoming draft >= 2026-08-08")
	type R struct {
		ID uint
		TradeDate, Status, SourceSession, GeneratedAt string
	}
	var rows []R
	_ = db.Raw(`SELECT id, trade_date, status, source_session, CAST(generated_at AS TEXT) generated_at
		FROM trade_plans WHERE trade_date >= '2026-08-08' AND status='draft'
		ORDER BY trade_date ASC, plan_version DESC, id DESC`).Scan(&rows)
	for _, r := range rows {
		fmt.Printf("%+v\n", r)
	}
}

func runCleanup(dsn string) {
	db := open(dsn + "?_busy_timeout=10000")
	tx := db.Begin()
	if tx.Error != nil {
		fmt.Fprintln(os.Stderr, tx.Error)
		os.Exit(1)
	}
	type n struct{ N int64 }
	del := func(sql string, args ...any) int64 {
		res := tx.Exec(sql, args...)
		if res.Error != nil {
			tx.Rollback()
			fmt.Fprintln(os.Stderr, "DEL ERR", sql, res.Error)
			os.Exit(1)
		}
		return res.RowsAffected
	}
	// Related child tables for plan 22 + 23 (B.0.5 / B.1 smoke) and their pools.
	planIDs := []any{22, 23}
	poolIDs := []any{21, 22}

	report := map[string]int64{}
	report["trade_plan_items_22_23"] = del(`DELETE FROM trade_plan_items WHERE plan_id IN (?,?)`, planIDs...)
	// execution intents / readiness if present
	for _, t := range []string{
		"execution_intents",
		"trade_plan_execution_intents",
		"paper_sim_orders",
		"paper_sim_fills",
		"paper_sim_runs",
		"audit_events",
	} {
		var exists n
		_ = tx.Raw(`SELECT COUNT(*) AS n FROM sqlite_master WHERE type='table' AND name=?`, t).Scan(&exists)
		if exists.N == 0 {
			continue
		}
		var cols []struct{ Name string }
		_ = tx.Raw(`PRAGMA table_info(` + t + `)`).Scan(&cols)
		col := ""
		for _, c := range cols {
			ln := strings.ToLower(c.Name)
			if ln == "plan_id" || ln == "trade_plan_id" {
				col = c.Name
				break
			}
		}
		if col == "" {
			continue
		}
		report[t] = del(fmt.Sprintf(`DELETE FROM "%s" WHERE "%s" IN (?,?)`, t, col), planIDs...)
	}
	report["trade_plans_22_23"] = del(`DELETE FROM trade_plans WHERE id IN (?,?)`, planIDs...)
	// candidate pool items then pools
	var hasPoolItems n
	_ = tx.Raw(`SELECT COUNT(*) AS n FROM sqlite_master WHERE type='table' AND name='candidate_pool_items'`).Scan(&hasPoolItems)
	if hasPoolItems.N > 0 {
		report["candidate_pool_items_21_22"] = del(`DELETE FROM candidate_pool_items WHERE pool_id IN (?,?)`, poolIDs...)
	}
	report["candidate_pools_21_22"] = del(`DELETE FROM candidate_pools WHERE id IN (?,?)`, poolIDs...)

	if err := tx.Commit().Error; err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("CLEANUP_OK", time.Now().Format(time.RFC3339))
	for k, v := range report {
		fmt.Printf("  %s rows=%d\n", k, v)
	}

	// post verify
	var left n
	_ = db.Raw(`SELECT COUNT(*) AS n FROM trade_plans WHERE trade_date LIKE '2099%'`).Scan(&left)
	fmt.Println("remaining_2099_plans=", left.N)
	var upcoming []struct {
		ID        uint
		TradeDate string
		Status    string
	}
	_ = db.Raw(`SELECT id, trade_date, status FROM trade_plans
		WHERE trade_date >= '2026-08-08' AND status='ready' AND freeze_at IS NOT NULL
		ORDER BY trade_date ASC, plan_version DESC, id DESC LIMIT 3`).Scan(&upcoming)
	fmt.Println("upcoming_frozen=", upcoming)
	_ = db.Raw(`SELECT id, trade_date, status FROM trade_plans
		WHERE trade_date >= '2026-08-08' AND status='draft'
		ORDER BY trade_date ASC, plan_version DESC, id DESC LIMIT 5`).Scan(&upcoming)
	fmt.Println("upcoming_draft=", upcoming)
	// also check what upcoming would pick for today-like dates
	for _, d := range []string{"2026-08-08", "2026-08-07", "2026-08-10"} {
		var fr, dr struct {
			ID        uint
			TradeDate string
			Status    string
		}
		_ = db.Raw(`SELECT id, trade_date, status FROM trade_plans
			WHERE trade_date >= ? AND status='ready' AND freeze_at IS NOT NULL
			ORDER BY trade_date ASC, plan_version DESC, id DESC LIMIT 1`, d).Scan(&fr)
		_ = db.Raw(`SELECT id, trade_date, status FROM trade_plans
			WHERE trade_date >= ? AND status='draft'
			ORDER BY trade_date ASC, plan_version DESC, id DESC LIMIT 1`, d).Scan(&dr)
		fmt.Printf("pick today=%s frozen={%d %s} draft={%d %s}\n", d, fr.ID, fr.TradeDate, dr.ID, dr.TradeDate)
	}
}
