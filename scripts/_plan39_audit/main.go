package main

import (
	"encoding/json"
	"fmt"
	"os"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	gdb, err := gorm.Open(sqlite.Open(`D:\stock\build\bin\data\stock.db?mode=ro&_busy_timeout=5000`), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	db.Dao = gdb

	for _, id := range []uint{39, 40} {
		var p models.TradePlan
		if err := db.Dao.First(&p, id).Error; err != nil {
			fmt.Fprintf(os.Stderr, "plan %d: %v\n", id, err)
			continue
		}
		var items []models.TradePlanItem
		_ = db.Dao.Where("plan_id = ?", id).Order("id asc").Find(&items).Error
		itemOut := make([]map[string]any, 0, len(items))
		for _, it := range items {
			itemOut = append(itemOut, map[string]any{
				"id": it.ID, "code": it.StockCode, "status": it.Status, "side": it.Side,
				"limitPrice": it.LimitPrice, "targetVolume": it.TargetVolume, "targetAmount": it.TargetAmount,
				"intentStatus": it.IntentStatus, "openRefPrice": it.OpenRefPrice,
				"refPrice": it.RefPrice, "entryRule": it.EntryRule,
				"pricedAt": it.PricedAt, "pricedBy": it.PricedBy,
				"orderId": it.OrderID, "fillId": it.FillID,
			})
		}
		guard := models.RequireFrozenReadyTradePlan(&p)
		out := map[string]any{
			"id": p.ID, "tradeDate": p.TradeDate, "status": p.Status, "planVersion": p.PlanVersion,
			"message": p.Message, "side": p.Side, "amountPerStock": p.AmountPerStock,
			"freezeAt": p.FreezeAt, "freezeBy": p.FreezeBy,
			"approvedAt": p.ApprovedAt, "approvedBy": p.ApprovedBy,
			"executedAt": p.ExecutedAt, "isFrozen": p.IsFrozen(),
			"enableExecute": p.EnableExecute, "pricingStage": p.PricingStage,
			"guardAllowed": guard.Allowed, "guardReason": guard.Reason, "guardMessage": guard.Message,
			"items": itemOut,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Printf("===== plan %d =====\n%s\n", id, string(b))
		_ = os.WriteFile(fmt.Sprintf(`D:\stock\tmp\plan%d_materialization_audit.json`, id), b, 0o644)
	}

	var frozenIDs []uint
	_ = db.Dao.Model(&models.TradePlan{}).
		Where("trade_date = ? AND status = ? AND freeze_at IS NOT NULL AND approved_at IS NOT NULL", "2026-08-14", "ready").
		Pluck("id", &frozenIDs)
	fmt.Printf("frozenReadyOn2026-08-14=%v\n", frozenIDs)
}
