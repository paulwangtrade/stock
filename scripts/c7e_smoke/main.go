// Temporary C.7-E smoke: RO open production DB + observation HTTP (AssetServer not curlable).
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
)

func main() {
	dsn := `D:\stock\build\bin\data\stock.db?mode=ro&_busy_timeout=5000`
	db.Init(dsn)

	var plans, items, accounts, positions, orders, fills int64
	_ = db.Dao.Model(&models.TradePlan{}).Count(&plans).Error
	_ = db.Dao.Model(&models.TradePlanItem{}).Count(&items).Error
	_ = db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&accounts).Error
	_ = db.Dao.Model(&papertrading.PaperSimPosition{}).Count(&positions).Error
	_ = db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&orders).Error
	_ = db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fills).Error

	var latest models.TradePlan
	_ = db.Dao.Order("id desc").First(&latest).Error

	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/portfolio?target=identity", nil)
	mux.ServeHTTP(rec, req)

	out := map[string]any{
		"db_ok":            db.Dao != nil,
		"trade_plans":      plans,
		"trade_plan_items": items,
		"paper_sim_accounts": accounts,
		"paper_sim_positions": positions,
		"paper_sim_orders": orders,
		"paper_sim_fills":   fills,
		"latest_plan": map[string]any{
			"id": latest.ID, "tradeDate": latest.TradeDate, "status": latest.Status,
		},
		"portfolio_http": rec.Code,
	}
	var body map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &body) == nil {
		out["portfolio_ok"] = body["ok"]
		if p, ok := body["portfolio"].(map[string]any); ok {
			if acc, ok := p["account"].(map[string]any); ok {
				out["portfolio_equity"] = acc["equity"]
				out["portfolio_n"] = acc["n"]
			}
		}
	} else {
		out["portfolio_body_prefix"] = string(rec.Body.Bytes())
		if len(rec.Body.Bytes()) > 200 {
			out["portfolio_body_prefix"] = string(rec.Body.Bytes()[:200])
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	_ = os.WriteFile(`D:\stock\tmp\c7e_smoke.json`, b, 0o644)
}
