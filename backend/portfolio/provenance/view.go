package provenance

import "time"

const (
	dataSourceNote = "Portfolio Provenance · read-only projection from snapshot + attribution + trade plan origin; no schema writes"
	disclaimer     = "Paper 模拟持仓溯源（只读）。不代表真实券商流水。"
)

// View is the GET-time provenance projection for one holding.
type View struct {
	StockCode      string          `json:"stock_code"`
	StockName      string          `json:"stock_name,omitempty"`
	Position       PositionBlock   `json:"position"`
	Trades         []TradeBlock    `json:"trades"`
	Origins        []OriginBlock   `json:"origins"`
	Reconcile      *ReconcileBlock `json:"reconcile,omitempty"`
	AsOf           time.Time       `json:"as_of"`
	DataSourceNote string          `json:"data_source_note,omitempty"`
	Disclaimer     string          `json:"disclaimer,omitempty"`
}

// PositionBlock mirrors portfolio snapshot quantity / cost for the holding.
type PositionBlock struct {
	Quantity      float64 `json:"quantity"`
	AvgCost       float64 `json:"avg_cost"`
	MarkPrice     float64 `json:"mark_price,omitempty"`
	UnrealizedPnl float64 `json:"unrealized_pnl,omitempty"`
}

// TradeBlock is one buy fill attributed to a trade plan.
type TradeBlock struct {
	PlanID     uint       `json:"plan_id"`
	PlanItemID uint       `json:"plan_item_id,omitempty"`
	FillID     uint       `json:"fill_id,omitempty"`
	FillPrice  float64    `json:"fill_price"`
	FillVolume int64      `json:"fill_volume,omitempty"`
	FilledAt   *time.Time `json:"filled_at,omitempty"`
	TradeDate  string     `json:"trade_date,omitempty"`
}

// OriginBlock is the strategy / signal / reason provenance for one plan.
type OriginBlock struct {
	PlanID   uint        `json:"plan_id"`
	Strategy string      `json:"strategy,omitempty"`
	Signal   SignalBlock `json:"signal"`
	Reason   ReasonBlock `json:"reason"`
	Score    string      `json:"score,omitempty"`
}

// SignalBlock maps trade plan origin signal fields.
type SignalBlock struct {
	Present    bool   `json:"present"`
	Tag        string `json:"tag,omitempty"`
	Time       string `json:"time,omitempty"`
	Price      string `json:"price,omitempty"`
	SnapshotID uint   `json:"snapshot_id,omitempty"`
}

// ReasonBlock maps trade plan origin reason fields.
type ReasonBlock struct {
	Present   bool   `json:"present"`
	Source    string `json:"source,omitempty"`
	Selection string `json:"selection,omitempty"`
	Item      string `json:"item,omitempty"`
}

// ReconcileBlock compares position qty vs attributed fill qty.
type ReconcileBlock struct {
	Status           string `json:"status"`
	PositionVolume   int64  `json:"position_volume"`
	AttributedVolume int64  `json:"attributed_volume"`
}
