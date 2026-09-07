// Package outcome assembles read-only OutcomeProjection views (Phase16-D2).
// It does not write DB, modify PaperBroker, or participate in TradePlan generation.
package outcome

import (
	"time"

	"go-stock/backend/opportunity/projection"
)

const (
	OutcomeStatusNoTrade = "NO_TRADE"
	OutcomeStatusOpen    = "OPEN"
	OutcomeStatusClosed  = "CLOSED"

	QualityComplete = "complete"
	QualityPartial  = "partial"

	SourceTypeRoundTripFIFO = "round_trip_fifo"
	SourceTypeNoTrade       = "no_trade"
	FIFOPolicyAccountV1     = "account_default_v1"
)

// OutcomeProjection is the read-only trade result for one opportunity leg.
type OutcomeProjection struct {
	OutcomeID     string `json:"outcome_id"`
	OpportunityID string `json:"opportunity_id"`
	StockCode     string `json:"stock_code"`
	StockName     string `json:"stock_name,omitempty"`
	OutcomeStatus string `json:"outcome_status"`

	Signal      projection.SignalBlock      `json:"signal"`
	Opportunity projection.OpportunityBlock `json:"opportunity"`
	Decision    projection.DecisionBlock    `json:"decision"`
	Entry       EntryBlock                  `json:"entry"`
	Exit        ExitBlock                   `json:"exit"`
	Performance PerformanceBlock            `json:"performance"`
	Metadata    MetadataBlock               `json:"metadata"`
}

// EntryBlock describes the buy fill leg.
type EntryBlock struct {
	Present       bool    `json:"present"`
	BuyFillID     uint    `json:"buy_fill_id,omitempty"`
	BuyPlanID     uint    `json:"buy_plan_id,omitempty"`
	BuyPlanItemID uint    `json:"buy_plan_item_id,omitempty"`
	EntryPrice    float64 `json:"entry_price,omitempty"`
	EntryQty      int64   `json:"entry_qty,omitempty"`
	EntryFee      float64 `json:"entry_fee,omitempty"`
	EntryDate     string  `json:"entry_date,omitempty"`
}

// ExitBlock describes the sell fill leg (absent when OPEN / NO_TRADE).
type ExitBlock struct {
	Present        bool    `json:"present"`
	SellFillID     uint    `json:"sell_fill_id,omitempty"`
	SellPlanID     uint    `json:"sell_plan_id,omitempty"`
	SellPlanItemID uint    `json:"sell_plan_item_id,omitempty"`
	ExitPrice      float64 `json:"exit_price,omitempty"`
	ExitQty        int64   `json:"exit_qty,omitempty"`
	ExitFee        float64 `json:"exit_fee,omitempty"`
	ExitDate       string  `json:"exit_date,omitempty"`
	ExitChannel    string  `json:"exit_channel,omitempty"`
	ExitReasonText string  `json:"exit_reason_text,omitempty"`
}

// PerformanceBlock holds MVP realized / open metrics.
type PerformanceBlock struct {
	EntryPrice         float64  `json:"entry_price,omitempty"`
	ExitPrice          *float64 `json:"exit_price,omitempty"`
	Quantity           int64    `json:"quantity,omitempty"`
	RealizedReturnPct  *float64 `json:"realized_return_pct,omitempty"`
	HoldingDays        int      `json:"holding_days,omitempty"`
}

// MetadataBlock aggregates outcome provenance.
type MetadataBlock struct {
	SourceType string    `json:"source_type"`
	Quality    string    `json:"quality"`
	Missing    []string  `json:"missing,omitempty"`
	FIFOPolicy string    `json:"fifo_policy,omitempty"`
	AsOf       time.Time `json:"as_of"`
}

// ProjectOptions selects read inputs.
type ProjectOptions struct {
	StockCode      string
	TradeDate      string
	IncludeNoTrade bool
	Status         string // filter OPEN / CLOSED / NO_TRADE
	Limit          int
	AsOf           time.Time
}
