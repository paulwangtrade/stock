package main

import (
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, _ := gorm.Open(sqlite.Open(`D:\stock\build\bin\data\stock.db`), &gorm.Config{})
	codes := []string{
		"300489", "300548", "300570", "300620", "300726",
		"301176", "301196", "301217", "301362", "301373", "301387", "301526", "301566",
		"688090", "688167", "688170", "688603", "688757", "688813", "920126",
	}
	fmt.Println("=== followed_stock (all is_del) ===")
	for _, c := range codes {
		prefix := "sz"
		if strings.HasPrefix(c, "688") || strings.HasPrefix(c, "920") {
			if strings.HasPrefix(c, "688") {
				prefix = "sh"
			} else {
				prefix = "bj"
			}
		}
		code := prefix + c
		_ = code
		var rows []map[string]interface{}
		db.Table("followed_stock").Where("stock_code like ?", "%"+c+"%").Find(&rows)
		for _, r := range rows {
			fmt.Printf("%v\n", r)
		}
	}
	fmt.Println("\n=== names from tushare_stock_basic ===")
	for _, c := range codes {
		var name string
		db.Table("tushare_stock_basic").Where("symbol = ?", c).Pluck("name", &name)
		if name == "" {
			db.Table("stock_base_info_us").Where("symbol = ?", c).Pluck("name", &name)
		}
		fmt.Printf("%s -> %s\n", c, name)
	}
}
