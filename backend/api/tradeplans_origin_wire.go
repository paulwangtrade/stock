package api

import "go-stock/backend/tradeplanorigin"

// tradePlanOriginItemWire is the Phase16-G0 HTTP contract for one origin row.
// Flat fields are kept for existing UI clients; nested blocks carry present/omitempty semantics.
type tradePlanOriginItemWire struct {
	StockCode        string             `json:"stock_code"`
	PlanID           string             `json:"plan_id"`
	SignalTime       string             `json:"signal_time,omitempty"`
	SignalPrice      string             `json:"signal_price,omitempty"`
	SignalTag        string             `json:"signal_tag,omitempty"`
	SignalSnapshotID uint               `json:"signal_snapshot_id,omitempty"`
	SourceReason     string             `json:"source_reason,omitempty"`
	SelectionReason  string             `json:"selection_reason,omitempty"`
	StrategyName     string             `json:"strategy_name,omitempty"`
	Score            string             `json:"score,omitempty"`
	Signal           originSignalWire   `json:"signal"`
	Reason           originReasonWire   `json:"reason"`
	Strategy         originStrategyWire `json:"strategy"`
}

type originSignalWire struct {
	Present          bool   `json:"present"`
	SignalTime       string `json:"signal_time,omitempty"`
	SignalPrice      string `json:"signal_price,omitempty"`
	SignalTag        string `json:"signal_tag,omitempty"`
	SignalSnapshotID uint   `json:"signal_snapshot_id,omitempty"`
}

type originReasonWire struct {
	Present         bool   `json:"present"`
	SourceReason    string `json:"source_reason,omitempty"`
	SelectionReason string `json:"selection_reason,omitempty"`
}

type originStrategyWire struct {
	Present      bool   `json:"present"`
	StrategyName string `json:"strategy_name,omitempty"`
	Score        string `json:"score,omitempty"`
}

func wireOriginItems(items []tradeplanorigin.ItemOrigin) []tradePlanOriginItemWire {
	out := make([]tradePlanOriginItemWire, 0, len(items))
	for _, item := range items {
		out = append(out, wireOriginItem(item))
	}
	return out
}

func wireOriginItem(item tradeplanorigin.ItemOrigin) tradePlanOriginItemWire {
	w := tradePlanOriginItemWire{
		StockCode: item.StockCode,
		PlanID:    item.PlanID,
	}

	sigPresent := originFieldPresent(item.SignalTag) ||
		originFieldPresent(item.SignalTime) ||
		originFieldPresent(item.SignalPrice) ||
		item.SignalSnapshotID > 0
	w.Signal = originSignalWire{Present: sigPresent}
	if originFieldPresent(item.SignalTag) {
		w.SignalTag = item.SignalTag
		w.Signal.SignalTag = item.SignalTag
	}
	if originFieldPresent(item.SignalTime) {
		w.SignalTime = item.SignalTime
		w.Signal.SignalTime = item.SignalTime
	}
	if originFieldPresent(item.SignalPrice) {
		w.SignalPrice = item.SignalPrice
		w.Signal.SignalPrice = item.SignalPrice
	}
	if item.SignalSnapshotID > 0 {
		w.SignalSnapshotID = item.SignalSnapshotID
		w.Signal.SignalSnapshotID = item.SignalSnapshotID
	}

	reasonPresent := originFieldPresent(item.SourceReason) || originFieldPresent(item.SelectionReason)
	w.Reason = originReasonWire{Present: reasonPresent}
	if originFieldPresent(item.SourceReason) {
		w.SourceReason = item.SourceReason
		w.Reason.SourceReason = item.SourceReason
	}
	if originFieldPresent(item.SelectionReason) {
		w.SelectionReason = item.SelectionReason
		w.Reason.SelectionReason = item.SelectionReason
	}

	strategyPresent := originFieldPresent(item.StrategyName) || originFieldPresent(item.Score)
	w.Strategy = originStrategyWire{Present: strategyPresent}
	if originFieldPresent(item.StrategyName) {
		w.StrategyName = item.StrategyName
		w.Strategy.StrategyName = item.StrategyName
	}
	if originFieldPresent(item.Score) {
		w.Score = item.Score
		w.Strategy.Score = item.Score
	}

	return w
}

func originFieldPresent(v string) bool {
	return v != "" && v != tradeplanorigin.Missing
}
