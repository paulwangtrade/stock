// 批量恢复自选：写入 followed_stock，关注时间来自运行日志首次扫描记录
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/duke-git/lancet/v2/convertor"
)

type item struct {
	Num  string
	Time string
}

var items = []item{
	{"300489", "2026-06-15 16:51:37"},
	{"300548", "2026-06-15 16:51:34"},
	{"300570", "2026-06-15 16:51:34"},
	{"300620", "2026-06-15 16:51:39"},
	{"300726", "2026-06-15 16:51:33"},
	{"301176", "2026-06-15 16:51:37"},
	{"301196", "2026-06-15 16:51:39"},
	{"301217", "2026-06-15 16:51:37"},
	{"301362", "2026-06-15 16:51:39"},
	{"301373", "2026-06-15 16:51:34"},
	{"301387", "2026-06-15 16:51:39"},
	{"301526", "2026-06-15 16:51:33"},
	{"301566", "2026-06-15 16:51:33"},
	{"688090", "2026-06-15 16:51:37"},
	{"688167", "2026-06-15 16:51:36"},
	{"688170", "2026-06-15 16:51:36"},
	{"688603", "2026-06-15 16:51:36"},
	{"688757", "2026-06-15 16:51:36"},
	{"688813", "2026-06-15 16:51:34"},
	{"920126", "2026-06-15 16:51:33"},
}

func toStockCode(num string) string {
	if strings.HasPrefix(num, "688") {
		return "sh" + num
	}
	if strings.HasPrefix(num, "920") {
		return "bj" + num
	}
	return "sz" + num
}

func lookupName(num string) string {
	var name string
	db.Dao.Table("tushare_stock_basic").Where("symbol = ?", num).Pluck("name", &name)
	return strings.TrimSpace(name)
}

func parseTime(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return time.Now()
	}
	return t
}

func main() {
	dbPath := `D:\stock\build\bin\data\stock.db`
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	db.Init(dbPath + "?_busy_timeout=10000&_journal_mode=WAL")
	api := data.NewStockDataApi()

	ok, skip, fail := 0, 0, 0
	for i, it := range items {
		code := toStockCode(it.Num)
		followTime := parseTime(it.Time)
		sort := int64(i + 1)

		var existing data.FollowedStock
		res := db.Dao.Unscoped().Where("stock_code = ?", code).First(&existing)
		if res.Error == nil && existing.IsDel == 0 {
			fmt.Printf("SKIP already active: %s %s\n", code, existing.Name)
			skip++
			continue
		}

		name := lookupName(it.Num)
		price := 0.0
		if infos, err := api.GetStockCodeRealTimeData(code); err == nil && infos != nil && len(*infos) > 0 {
			if name == "" {
				name = (*infos)[0].Name
			}
			price, _ = convertor.ToFloat((*infos)[0].Price)
		}
		if name == "" {
			fmt.Printf("FAIL no name: %s\n", code)
			fail++
			continue
		}

		followPrice := api.LookupFollowBaselinePrice(code, followTime)
		if followPrice <= 0 && price > 0 {
			followPrice = price
		}

		row := data.FollowedStock{
			StockCode:          code,
			Name:               name,
			Price:              price,
			FollowPrice:        followPrice,
			Time:               followTime,
			Sort:               sort,
			AlarmChangePercent: 3,
			AlarmPrice:         price + 1,
			IsDel:              0,
		}

		if res.Error == nil {
			if err := db.Dao.Unscoped().Where("stock_code = ?", code).Updates(map[string]interface{}{
				"name":                 name,
				"price":                price,
				"follow_price":         followPrice,
				"time":                 followTime,
				"sort":                 sort,
				"alarm_change_percent": 3,
				"alarm_price":          price + 1,
				"is_del":               0,
			}).Error; err != nil {
				fmt.Printf("FAIL update %s: %v\n", code, err)
				fail++
				continue
			}
		} else {
			if err := db.Dao.Create(&row).Error; err != nil {
				fmt.Printf("FAIL create %s: %v\n", code, err)
				fail++
				continue
			}
		}
		fmt.Printf("OK %s %s sort=%d time=%s followPrice=%.2f\n", code, name, sort, followTime.Format("2006-01-02 15:04:05"), followPrice)
		ok++
	}

	fmt.Printf("\nDone: ok=%d skip=%d fail=%d\n", ok, skip, fail)
	var active int64
	db.Dao.Model(&data.FollowedStock{}).Where("is_del = 0").Count(&active)
	fmt.Printf("Active watchlist: %d\n", active)
}
