package main
import (
  "encoding/json"
  "fmt"
  "os"
  "strings"
  "github.com/glebarez/sqlite"
  "gorm.io/gorm"
  "gorm.io/gorm/logger"
  "go-stock/backend/api"
  "go-stock/backend/data"
  "go-stock/backend/db"
)
func main() {
  gdb, err := gorm.Open(sqlite.Open(`D:/stock/build/bin/data/stock.db?mode=ro&_busy_timeout=8000`), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
  db.Dao = gdb

  var run struct{ ID uint; ResultJSON string; Message string; StockCount int }
  _ = gdb.Raw(`SELECT id, result_json, message, stock_count FROM stock_strategy_runs WHERE id=50`).Scan(&run)
  fmt.Printf("run50 id=%d count=%d msg=%s jsonLen=%d\n", run.ID, run.StockCount, run.Message, len(run.ResultJSON))

  // parse loosely
  var view map[string]any
  if err := json.Unmarshal([]byte(run.ResultJSON), &view); err != nil {
    fmt.Println("unmarshal view err", err)
  } else {
    raw, _ := json.Marshal(view["dataList"])
    if raw == nil { raw, _ = json.Marshal(view["DataList"]) }
    var rows []map[string]any
    _ = json.Unmarshal(raw, &rows)
    fmt.Println("dataList len", len(rows))
    for _, row := range rows {
      blob, _ := json.Marshal(row)
      s := string(blob)
      if strings.Contains(s, "301677") || strings.Contains(s, "600363") {
        // print name-related keys
        keys := []string{}
        for k, v := range row {
          lk := strings.ToLower(k)
          if strings.Contains(lk, "name") || strings.Contains(lk, "code") || strings.Contains(lk, "secu") {
            keys = append(keys, fmt.Sprintf("%s=%v", k, v))
          }
        }
        fmt.Println("ROW:", strings.Join(keys, ", "))
      }
    }
  }

  // tushare presence
  for _, sym := range []string{"301677", "600363"} {
    var n int
    gdb.Raw(`SELECT COUNT(*) FROM tushare_stock_basic WHERE symbol=?`, sym).Scan(&n)
    var name string
    gdb.Raw(`SELECT name FROM tushare_stock_basic WHERE symbol=? LIMIT 1`, sym).Scan(&name)
    fmt.Printf("tushare symbol=%s count=%d name=%q\n", sym, n, name)
  }
  for _, secu := range []string{"301677.SZ", "600363.SH"} {
    var name string
    gdb.Raw(`SELECT sec_uri_tynameabbr FROM all_stock_info WHERE secucode=? LIMIT 1`, secu).Scan(&name)
    fmt.Printf("all_stock_info %s nameabbr=%q\n", secu, name)
  }

  // simulate enrich via handler path: call lookup through package - use data Normalize + same query
  // Use unexported via writing equivalent
  codes := []string{"sz301677", "sh600363"}
  // replicate defaultStockNameLookup by importing through a tiny HTTP? Instead duplicate Normalize
  for _, c := range codes {
    norm, err := data.NormalizeStockCode(c)
    fmt.Printf("normalize %s -> %+v err=%v\n", c, norm, err)
  }

  // Check StockBasic model table name
  var basics []data.StockBasic
  gdb.Model(&data.StockBasic{}).Select("symbol","name","ts_code").Where("symbol IN ?", []string{"301677","600363"}).Find(&basics)
  fmt.Printf("StockBasic model rows=%+v\n", basics)

  _ = api.NewTradePlansHandler
}
