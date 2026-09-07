package main
import (
  "fmt"
  "os"
  "github.com/glebarez/sqlite"
  "gorm.io/gorm"
  "gorm.io/gorm/logger"
  "go-stock/backend/data"
  "go-stock/backend/db"
)
func main() {
  path := `D:/stock/build/bin/data/stock.db`
  gdb, err := gorm.Open(sqlite.Open(path+"?mode=ro&_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
  db.Dao = gdb
  repo := data.NewTradePlanRepo()
  for _, d := range []string{"2026-08-08", "2026-08-07", "2026-08-10"} {
    view, err := repo.GetUpcomingTradePlan(d)
    if err != nil {
      fmt.Printf("upcoming(%s) err=%v\n", d, err)
      continue
    }
    fmt.Printf("upcoming(%s) id=%d trade_date=%s status=%s src=%s gen=%v\n", d, view.ID, view.TradeDate, view.Status, view.SourceSession, view.GeneratedAt)
  }
  var n int
  gdb.Raw(`SELECT COUNT(*) FROM trade_plans WHERE id IN (22,23) OR trade_date LIKE '2099%'`).Scan(&n)
  fmt.Println("left_smoke_or_2099=", n)
  var pools int
  gdb.Raw(`SELECT COUNT(*) FROM candidate_pools WHERE id IN (21,22) OR message LIKE '%smoke%'`).Scan(&pools)
  fmt.Println("left_smoke_pools=", pools)
}
