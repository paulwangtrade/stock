// Package decisiontimeline builds a read-only evidence trail for one stock.
// It never writes TradePlan, never submits orders, and never treats an
// observation as an instruction. Entry, holding, and exit stay on separate lanes.
package decisiontimeline

import "errors"

const (
	Disclaimer = "回顾用，不是买卖指令"

	KindSignal         = "signal"
	KindWatch          = "watch"
	KindFollow         = "follow"
	KindPaperSim       = "paper_sim"
	KindExternalMirror = "external_mirror"
	KindOutcome        = "outcome"

	LaneObserve = "observe"
	LaneEntry   = "entry"
	LaneHolding = "holding"
	LaneExit    = "exit"

	SourceSignal         = "signal_snapshot"
	SourceWatchFollow    = "watch_follow"
	SourcePaperSim       = "paper_sim"
	SourceExternalMirror = "external_mirror"
	SourceOutcome        = "outcome_note"

	StatusOK          = "ok"
	StatusEmpty       = "empty"
	StatusUnavailable = "unavailable"
)

// ErrInvalidStockCode is returned when the query has no usable stock code.
var ErrInvalidStockCode = errors.New("decisiontimeline: stock code required")

// Event is one dated observation. It is not an order.
type Event struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Lane       string `json:"lane"`
	OccurredOn string `json:"occurred_on"`
	OccurredAt string `json:"occurred_at,omitempty"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Tag        string `json:"tag,omitempty"`
	Source     string `json:"source"`
	KlineDate  string `json:"kline_date,omitempty"`
}

// SourceStatus reports whether a read source contributed events.
type SourceStatus struct {
	Source  string `json:"source"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Count   int    `json:"count"`
}

// Timeline is the read model for one stock.
type Timeline struct {
	StockCode       string         `json:"stock_code"`
	StockName       string         `json:"stock_name,omitempty"`
	Disclaimer      string         `json:"disclaimer"`
	ObservationOnly bool           `json:"observation_only"`
	Events          []Event        `json:"events"`
	Sources         []SourceStatus `json:"sources"`
	GeneratedAt     string         `json:"generated_at"`
}

// Options limits how much snapshot history is scanned.
type Options struct {
	SnapshotLimit int
}
