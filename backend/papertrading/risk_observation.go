// Risk Observation — read-only account risk view (Phase10-F.3).
package papertrading

import (
	"math"

	"go-stock/backend/db"
)

// RiskObservationView is account + position concentration (read-only).
type RiskObservationView struct {
	Enabled        bool                    `json:"enabled"`
	Quality        string                  `json:"quality"` // OK | UNKNOWN
	TotalAsset     *float64                `json:"total_asset"`
	Cash           *float64                `json:"cash"`
	PositionValue  *float64                `json:"position_value"`
	PositionRatio  *float64                `json:"position_ratio"`
	Positions      []RiskPositionRow       `json:"positions"`
	Concentration  *float64                `json:"concentration"` // max single-stock weight
	DataSourceNote string                  `json:"data_source_note"`
}

// RiskPositionRow is one name weight.
type RiskPositionRow struct {
	StockCode         string   `json:"stock_code"`
	StockName         string   `json:"stock_name"`
	MarketValue       float64  `json:"market_value"`
	SingleStockWeight *float64 `json:"single_stock_weight"`
}

// BuildRiskObservation projects cash/equity/weights from paper account (no invented cash).
func BuildRiskObservation() (*RiskObservationView, error) {
	out := &RiskObservationView{
		Enabled:        IsEnabled(),
		Quality:        "UNKNOWN",
		Positions:      []RiskPositionRow{},
		DataSourceNote: "Risk Observation · read-only; UNKNOWN when account/equity unavailable; no fabricated capital",
	}
	if !IsEnabled() {
		return out, nil
	}
	acc, err := GetDefaultAccount()
	if err != nil || acc == nil {
		return out, nil
	}
	cash := acc.Cash
	out.Cash = &cash

	var positions []PaperSimPosition
	if err := db.Dao.Where("account_id = ?", acc.ID).Find(&positions).Error; err != nil {
		return out, err
	}
	var posVal float64
	rows := make([]RiskPositionRow, 0, len(positions))
	for _, p := range positions {
		mv := p.MarkPrice * float64(p.TotalVolume)
		if mv < 0 || math.IsNaN(mv) {
			mv = 0
		}
		posVal += mv
		rows = append(rows, RiskPositionRow{
			StockCode:   p.StockCode,
			StockName:   p.StockName,
			MarketValue: mv,
		})
	}
	out.PositionValue = &posVal
	total := cash + posVal
	out.TotalAsset = &total
	if total > 0 {
		r := posVal / total
		out.PositionRatio = &r
	}
	var maxW float64
	for i := range rows {
		if posVal > 0 {
			w := rows[i].MarketValue / posVal
			rows[i].SingleStockWeight = &w
			if w > maxW {
				maxW = w
			}
		}
	}
	out.Positions = rows
	if len(rows) > 0 && posVal > 0 {
		out.Concentration = &maxW
	}
	out.Quality = "OK"
	return out, nil
}
