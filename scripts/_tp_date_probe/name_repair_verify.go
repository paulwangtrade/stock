package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := gorm.Open(sqlite.Open(`D:/stock/build/bin/data/stock.db?_busy_timeout=10000`), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	var empty, named, total int64
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items`).Scan(&total).Error
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE stock_name IS NULL OR TRIM(stock_name)=''`).Scan(&empty).Error
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE stock_name IS NOT NULL AND TRIM(stock_name)!=''`).Scan(&named).Error
	fmt.Printf("total=%d empty=%d named=%d\n", total, empty, named)

	type sample struct {
		ID   uint
		Code string
		Name string
	}
	var samples []sample
	_ = db.Raw(`SELECT id, stock_code as code, stock_name as name FROM trade_plan_items WHERE id IN (1,32,33,139,140) ORDER BY id`).Scan(&samples).Error
	for _, s := range samples {
		fmt.Printf("id=%d code=%s name=%q\n", s.ID, s.Code, s.Name)
	}

	var row struct {
		LimitPrice   float64
		TargetVolume int64
		Status       string
	}
	_ = db.Raw(`SELECT limit_price, target_volume, status FROM trade_plan_items WHERE id=1`).Scan(&row).Error
	fmt.Printf("item1_untouched limit=%v vol=%d status=%s\n", row.LimitPrice, row.TargetVolume, row.Status)

	// plan 32/33 named items should remain
	var p32empty int64
	_ = db.Raw(`SELECT COUNT(*) FROM trade_plan_items WHERE plan_id IN (32,33) AND (stock_name IS NULL OR TRIM(stock_name)='')`).Scan(&p32empty).Error
	fmt.Printf("plan_32_33_empty=%d\n", p32empty)

	// source distribution from apply log is separate; count distinct plans still empty
	var plans []uint
	_ = db.Raw(`SELECT DISTINCT plan_id FROM trade_plan_items WHERE stock_name IS NULL OR TRIM(stock_name)=''`).Scan(&plans).Error
	fmt.Printf("remaining_empty_plans=%v\n", plans)
}
