package main
import (
  "fmt"
  "github.com/glebarez/sqlite"
  "gorm.io/gorm"
  "gorm.io/gorm/logger"
)
func main() {
  db, _ := gorm.Open(sqlite.Open(`D:/stock/build/bin/data/stock.db?mode=ro`), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
  for _, sym := range []string{"301677","600363"} {
    var n int; var name string
    db.Raw(`SELECT COUNT(*) FROM tushare_stock_basic WHERE symbol=?`, sym).Scan(&n)
    db.Raw(`SELECT name FROM tushare_stock_basic WHERE symbol=? LIMIT 1`, sym).Scan(&name)
    fmt.Printf("tushare %s count=%d name=%q\n", sym, n, name)
  }
  var items []struct{ StockCode, StockName string }
  db.Raw(`SELECT stock_code, stock_name FROM trade_plan_items WHERE plan_id=32`).Scan(&items)
  fmt.Printf("plan32=%+v\n", items)
}
