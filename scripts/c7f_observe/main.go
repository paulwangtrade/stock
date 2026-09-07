// Phase10-C.7-F read-only runtime snapshot (no product code changes).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
)

func main() {
	dsn := `D:\stock\build\bin\data\stock.db?mode=ro&_busy_timeout=8000`
	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}
	db.Init(dsn)
	tradeDate := time.Now().Format("2006-01-02")
	if len(os.Args) > 2 {
		tradeDate = os.Args[2]
	}

	out := map[string]any{
		"capturedAt": time.Now().Format(time.RFC3339),
		"tradeDate":  tradeDate,
	}

	var plans []models.TradePlan
	_ = db.Dao.Where("trade_date = ?", tradeDate).Order("id asc").Find(&plans).Error
	planViews := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		var items []models.TradePlanItem
		_ = db.Dao.Where("plan_id = ?", p.ID).Find(&items).Error
		itemViews := make([]map[string]any, 0, len(items))
		filled, pending, skipped, errored := 0, 0, 0, 0
		for _, it := range items {
			itemViews = append(itemViews, map[string]any{
				"id": it.ID, "code": it.StockCode, "status": it.Status,
				"orderId": it.OrderID, "fillId": it.FillID,
				"filledVol": it.FilledVolume, "filledPx": it.FilledPrice, "error": it.Error,
			})
			switch it.Status {
			case models.TradePlanItemFilled:
				filled++
			case models.TradePlanItemSkipped:
				skipped++
			case models.TradePlanItemError:
				errored++
			default:
				pending++
			}
		}
		planViews = append(planViews, map[string]any{
			"id": p.ID, "status": p.Status, "version": p.PlanVersion,
			"freezeAt": p.FreezeAt, "approvedAt": p.ApprovedAt, "executedAt": p.ExecutedAt,
			"message": p.Message, "itemCount": len(items),
			"itemFilled": filled, "itemPending": pending, "itemSkipped": skipped, "itemError": errored,
			"items": itemViews,
		})
	}
	out["plans"] = planViews

	var runs []papertrading.PaperSimRun
	_ = db.Dao.Where("trade_date = ?", tradeDate).Order("id asc").Find(&runs).Error
	runViews := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		runViews = append(runViews, map[string]any{
			"id": r.ID, "executionId": r.ExecutionID, "planId": r.PlanID,
			"status": r.Status, "trigger": r.Trigger, "actor": r.Actor,
			"orders": r.OrdersTotal, "filled": r.FilledCount, "reject": r.RejectCount,
			"message": r.Message, "startedAt": r.StartedAt, "finishedAt": r.FinishedAt,
		})
	}
	out["runs"] = runViews

	acc, _ := papertrading.GetDefaultAccount()
	if acc != nil {
		out["account"] = map[string]any{
			"id": acc.ID, "cash": acc.Cash, "equity": acc.Equity,
			"marketValue": acc.MarketValue, "unrealizedPnl": acc.UnrealizedPnl,
		}
		pos, _ := papertrading.GetPositions(acc.ID)
		posViews := make([]map[string]any, 0, len(pos))
		var locked, avail, total int64
		for _, p := range pos {
			posViews = append(posViews, map[string]any{
				"code": p.StockCode, "total": p.TotalVolume,
				"locked": p.LockedVolume, "available": p.AvailableVolume,
				"avgCost": p.AvgCost, "mark": p.MarkPrice,
			})
			locked += p.LockedVolume
			avail += p.AvailableVolume
			total += p.TotalVolume
		}
		out["positions"] = posViews
		out["positionTotals"] = map[string]any{"n": len(pos), "total": total, "locked": locked, "available": avail}
	}

	var orders, fills int64
	_ = db.Dao.Model(&papertrading.PaperSimOrder{}).Where("trade_date = ?", tradeDate).Count(&orders).Error
	_ = db.Dao.Table("paper_sim_fills AS f").
		Joins("JOIN paper_sim_orders AS o ON o.id = f.order_id").
		Where("o.trade_date = ?", tradeDate).Count(&fills).Error
	out["todayOrders"] = orders
	out["todayFills"] = fills

	var report papertrading.PaperSimDailyReport
	if err := db.Dao.Where("report_date = ?", tradeDate).First(&report).Error; err == nil {
		out["dailyReport"] = map[string]any{
			"equity": report.Equity, "cash": report.Cash, "marketValue": report.MarketValue,
			"floatingPnl": report.FloatingPnl, "filledCount": report.FilledCount,
			"positionCount": report.PositionCount, "runStatus": report.RunStatus, "source": report.Source,
		}
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	dir := `D:\stock\tmp\c7f_observation`
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(fmt.Sprintf("%s\\snap_%s.json", dir, time.Now().Format("150405")), b, 0o644)
	_ = os.WriteFile(fmt.Sprintf("%s\\latest.json", dir), b, 0o644)
}
