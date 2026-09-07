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
	codes := []string{"sz301677", "sh600363", "301677", "600363"}

	fmt.Println("==== trade_plan_items schema")
	var cols []struct{ Name, Type string }
	_ = db.Raw(`PRAGMA table_info(trade_plan_items)`).Scan(&cols)
	for _, c := range cols {
		fmt.Printf("  %s %s\n", c.Name, c.Type)
	}

	fmt.Println("==== trade_plan_items for codes")
	type Item struct {
		ID, PlanID uint
		StockCode, StockName, Side, Status string
		TradeDate string
	}
	var items []Item
	_ = db.Raw(`SELECT id, plan_id, stock_code, stock_name, side, status, trade_date
		FROM trade_plan_items
		WHERE stock_code IN (?,?) OR stock_code LIKE '%301677%' OR stock_code LIKE '%600363%'
		ORDER BY plan_id DESC, id DESC LIMIT 40`, "sz301677", "sh600363").Scan(&items)
	for _, it := range items {
		fmt.Printf("item id=%d plan=%d code=%q name=%q side=%s status=%s td=%s\n",
			it.ID, it.PlanID, it.StockCode, it.StockName, it.Side, it.Status, it.TradeDate)
	}

	fmt.Println("==== latest plans containing either code")
	type P struct {
		ID uint
		TradeDate, Status, SourceSession string
		PlanVersion int
	}
	var plans []P
	_ = db.Raw(`SELECT DISTINCT p.id, p.trade_date, p.status, p.source_session, p.plan_version
		FROM trade_plans p
		JOIN trade_plan_items i ON i.plan_id=p.id
		WHERE i.stock_code IN (?,?)
		ORDER BY p.id DESC LIMIT 20`, "sz301677", "sh600363").Scan(&plans)
	for _, p := range plans {
		fmt.Printf("plan id=%d td=%s status=%s src=%s v=%d\n", p.ID, p.TradeDate, p.Status, p.SourceSession, p.PlanVersion)
		var its []Item
		_ = db.Raw(`SELECT id, plan_id, stock_code, stock_name, side, status, trade_date FROM trade_plan_items WHERE plan_id=? ORDER BY id`, p.ID).Scan(&its)
		for _, it := range its {
			fmt.Printf("  -> code=%q name=%q\n", it.StockCode, it.StockName)
		}
	}

	fmt.Println("==== candidate_pool_items")
	var ccols []struct{ Name string }
	_ = db.Raw(`PRAGMA table_info(candidate_pool_items)`).Scan(&ccols)
	names := make([]string, 0, len(ccols))
	for _, c := range ccols {
		names = append(names, c.Name)
	}
	fmt.Println("cols:", strings.Join(names, ","))
	type CI struct {
		ID, PoolID uint
		StockCode, StockName, Name string
	}
	// try flexible
	var cis []map[string]any
	rows, err := db.Raw(`SELECT * FROM candidate_pool_items WHERE stock_code IN (?,?) OR stock_code LIKE '%301677%' OR stock_code LIKE '%600363%' ORDER BY id DESC LIMIT 20`, "sz301677", "sh600363").Rows()
	if err == nil {
		defer rows.Close()
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			_ = rows.Scan(ptrs...)
			m := map[string]any{}
			for i, c := range cols {
				switch v := vals[i].(type) {
				case []byte:
					m[c] = string(v)
				default:
					m[c] = v
				}
			}
			cis = append(cis, m)
			fmt.Println(m)
		}
	} else {
		fmt.Println("err", err)
	}
	_ = cis

	// stock master tables
	fmt.Println("==== tables matching stock/basic/follow")
	var tables []string
	_ = db.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%stock%' OR name LIKE '%basic%' OR name LIKE '%follow%' OR name LIKE '%symbol%') ORDER BY name`).Scan(&tables)
	fmt.Println(tables)

	for _, t := range tables {
		var tcols []struct{ Name string }
		_ = db.Raw(`PRAGMA table_info(` + t + `)`).Scan(&tcols)
		codeCol, nameCol := "", ""
		for _, c := range tcols {
			ln := strings.ToLower(c.Name)
			if codeCol == "" && (ln == "stock_code" || ln == "code" || ln == "ts_code" || ln == "symbol") {
				codeCol = c.Name
			}
			if nameCol == "" && (ln == "name" || ln == "stock_name" || ln == "stockname") {
				nameCol = c.Name
			}
		}
		if codeCol == "" {
			continue
		}
		q := fmt.Sprintf(`SELECT * FROM "%s" WHERE CAST("%s" AS TEXT) IN ('sz301677','sh600363','301677','600363','301677.SZ','600363.SH') OR CAST("%s" AS TEXT) LIKE '%%301677%%' OR CAST("%s" AS TEXT) LIKE '%%600363%%' LIMIT 5`, t, codeCol, codeCol, codeCol)
		r2, err := db.Raw(q).Rows()
		if err != nil {
			continue
		}
		cols2, _ := r2.Columns()
		found := 0
		for r2.Next() {
			found++
			vals := make([]any, len(cols2))
			ptrs := make([]any, len(cols2))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			_ = r2.Scan(ptrs...)
			parts := []string{}
			for i, c := range cols2 {
				v := vals[i]
				if b, ok := v.([]byte); ok {
					v = string(b)
				}
				if c == codeCol || c == nameCol || strings.Contains(strings.ToLower(c), "name") || strings.Contains(strings.ToLower(c), "code") {
					parts = append(parts, fmt.Sprintf("%s=%v", c, v))
				}
			}
			fmt.Printf("%s: %s\n", t, strings.Join(parts, ", "))
		}
		r2.Close()
		if found == 0 {
			// silent
		}
	}
	_ = codes
}
