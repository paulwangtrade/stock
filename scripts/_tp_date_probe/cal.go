package main
import (
  "fmt"
  "go-stock/backend/tradingcalendar"
  "time"
)
func main() {
  for _, d := range []string{"2026-08-08","2026-08-07","2026-08-05","2099-01-03","2099-01-06"} {
    next, err := tradingcalendar.NextTradingDayString(d)
    fmt.Printf("%s -> %s err=%v\n", d, next, err)
  }
  fmt.Println("now", time.Now().Format("2006-01-02"))
}
