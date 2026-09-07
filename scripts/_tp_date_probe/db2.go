package main
import (
  "fmt"
  "os"
  "github.com/glebarez/sqlite"
  "gorm.io/gorm"
  "gorm.io/gorm/logger"
)
func main() {
  db, err := gorm.Open(sqlite.Open(`D:/stock/build/bin/data/stock.db?mode=ro`), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
  type P struct{ ID uint; TradeDate, Status, GeneratedAt, CreatedAt, ConfigJSON, Message string; ItemCount int }
  var p P
  db.Raw(`SELECT id, trade_date, status, CAST(generated_at AS TEXT) generated_at, CAST(created_at AS TEXT) created_at, config_json, message, item_count FROM candidate_pools WHERE id=21`).Scan(&p)
  fmt.Printf("pool21=%+v\n", p)
  type I struct{ ID uint; StockCode, StockName, Side string; Score float64; Status string }
  var items []I
  db.Raw(`SELECT id, stock_code, stock_name, side, score, status FROM trade_plan_items WHERE plan_id=22`).Scan(&items)
  fmt.Printf("plan22 items=%+v\n", items)
  // recent ready plans for context
  type R struct{ ID uint; TradeDate, Status, SourceSession, GeneratedAt, FreezeAt string; PlanVersion int }
  var rs []R
  db.Raw(`SELECT id, trade_date, plan_version, status, source_session, CAST(generated_at AS TEXT) generated_at, CAST(freeze_at AS TEXT) freeze_at FROM trade_plans WHERE status='ready' ORDER BY trade_date DESC, id DESC LIMIT 10`).Scan(&rs)
  for _, r := range rs { fmt.Printf("ready %+v\n", r) }
}
