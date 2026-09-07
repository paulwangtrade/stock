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
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

  queries := []struct{ Title, SQL string; Args []any }{
    {"tushare 301677", `SELECT ts_code, symbol, name, market, list_status FROM tushare_stock_basic WHERE ts_code LIKE '%301677%' OR symbol='301677' OR name LIKE '%301677%'`, nil},
    {"tushare 600363", `SELECT ts_code, symbol, name, market, list_status FROM tushare_stock_basic WHERE ts_code LIKE '%600363%' OR symbol='600363'`, nil},
    {"followed 301677/600363", `SELECT stock_code, name, price, follow_price, is_del FROM followed_stock WHERE stock_code LIKE '%301677%' OR stock_code LIKE '%600363%'`, nil},
    {"stock_info", `SELECT * FROM stock_info WHERE CAST(code AS TEXT) LIKE '%301677%' OR CAST(code AS TEXT) LIKE '%600363%' OR CAST(stock_code AS TEXT) LIKE '%301677%' LIMIT 10`, nil},
    {"all_stock_info cols", `PRAGMA table_info(all_stock_info)`, nil},
  }
  // stock_info columns
  var siCols []struct{ Name string }
  _ = db.Raw(`PRAGMA table_info(stock_info)`).Scan(&siCols)
  fmt.Println("stock_info cols:", siCols)
  var asCols []struct{ Name string }
  _ = db.Raw(`PRAGMA table_info(all_stock_info)`).Scan(&asCols)
  fmt.Println("all_stock_info cols:", asCols)
  var gsCols []struct{ Name string }
  _ = db.Raw(`PRAGMA table_info(group_stock_info)`).Scan(&gsCols)
  fmt.Println("group_stock_info cols:", gsCols)

  searchTables := []string{"followed_stock", "tushare_stock_basic", "all_stock_info", "stock_info", "group_stock_info", "ai_recommend_stocks"}
  for _, t := range searchTables {
    var cols []struct{ Name string }
    _ = db.Raw(`PRAGMA table_info(`+t+`)`).Scan(&cols)
    if len(cols)==0 { continue }
    // build where on text-ish columns
    wheres := []string{}
    for _, c := range cols {
      ln := strings.ToLower(c.Name)
      if strings.Contains(ln, "code") || ln=="symbol" || ln=="name" || strings.Contains(ln,"ts_code") {
        wheres = append(wheres, fmt.Sprintf(`CAST("%s" AS TEXT) LIKE '%%301677%%' OR CAST("%s" AS TEXT) LIKE '%%600363%%'`, c.Name, c.Name))
      }
    }
    if len(wheres)==0 { continue }
    q := fmt.Sprintf(`SELECT * FROM "%s" WHERE %s LIMIT 8`, t, strings.Join(wheres, " OR "))
    rows, err := db.Raw(q).Rows()
    if err != nil { fmt.Println(t, "err", err); continue }
    cols2, _ := rows.Columns()
    n := 0
    for rows.Next() {
      n++
      vals := make([]any, len(cols2))
      ptrs := make([]any, len(cols2))
      for i := range vals { ptrs[i]=&vals[i] }
      _ = rows.Scan(ptrs...)
      parts := []string{}
      for i,c := range cols2 {
        v := vals[i]
        if b,ok := v.([]byte); ok { v=string(b) }
        s := fmt.Sprint(v)
        if s=="" || s=="0" || s=="<nil>" { continue }
        ln := strings.ToLower(c)
        if strings.Contains(ln,"code") || ln=="name" || ln=="symbol" || strings.Contains(ln,"price") || ln=="is_del" {
          parts = append(parts, fmt.Sprintf("%s=%v", c, v))
        }
      }
      fmt.Printf("%s: %s\n", t, strings.Join(parts, ", "))
    }
    rows.Close()
    if n==0 { fmt.Printf("%s: NO_MATCH\n", t) }
  }
  _ = queries

  // pool 30 parent
  type Pool struct{ ID uint; TradeDate, Status, Message, ConfigJSON string }
  var p Pool
  _ = db.Raw(`SELECT id, trade_date, status, message, config_json FROM candidate_pools WHERE id=30`).Scan(&p)
  fmt.Printf("pool30=%+v\n", p)

  // strategy run 50?
  var runCols []struct{ Name string }
  _ = db.Raw(`PRAGMA table_info(stock_strategy_runs)`).Scan(&runCols)
  fmt.Println("stock_strategy_runs cols", runCols)
}
