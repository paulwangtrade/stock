package provenance

import (
	"errors"
	"sort"
	"time"

	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/tradeplanorigin"
)

// ErrNotFound is returned when the stock has no open position in the snapshot.
var ErrNotFound = errors.New("provenance: position not found")

// EvaluateOptions selects as_of for the provenance view.
type EvaluateOptions struct {
	AsOf time.Time
}

// Evaluate builds a read-only provenance view for one stock code.
func Evaluate(stockCode string, opts EvaluateOptions) (*View, error) {
	code := normalizeCode(stockCode)
	if code == "" {
		return nil, ErrNotFound
	}

	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if err != nil {
		return nil, err
	}
	if snap == nil || !snap.Found {
		return nil, ErrNotFound
	}

	var pos *portfolio.Position
	for i := range snap.Positions {
		if normalizeCode(snap.Positions[i].StockCode) == code && snap.Positions[i].Volume > 0 {
			pos = &snap.Positions[i]
			break
		}
	}
	if pos == nil {
		return nil, ErrNotFound
	}

	attr, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{StockCode: code})
	if err != nil {
		return nil, err
	}

	var attrRow *papertrading.PositionAttributionRow
	for i := range attr.Positions {
		if normalizeCode(attr.Positions[i].StockCode) == code {
			attrRow = &attr.Positions[i]
			break
		}
	}

	view := &View{
		StockCode: code,
		StockName: pos.StockName,
		Position: PositionBlock{
			Quantity:      float64(pos.Volume),
			AvgCost:       pos.AvgCost,
			MarkPrice:     pos.MarkPrice,
			UnrealizedPnl: pos.UnrealizedPnL,
		},
		Trades:         []TradeBlock{},
		Origins:        []OriginBlock{},
		AsOf:           asOf,
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}

	if attrRow != nil {
		view.Trades = mapTrades(attrRow.Lots)
		view.Origins = buildOrigins(attrRow.Lots, code)
		view.Reconcile = mapReconcile(attrRow.Reconcile)
	} else {
		view.Reconcile = &ReconcileBlock{
			Status:           papertrading.ReconcileStatusUnattributed,
			PositionVolume:   pos.Volume,
			AttributedVolume: 0,
		}
	}

	return view, nil
}

func normalizeCode(raw string) string {
	return opportunity.NormalizeReadStockCode(raw)
}

func mapTrades(lots []papertrading.PositionLotDTO) []TradeBlock {
	trades := make([]TradeBlock, 0, len(lots))
	for _, lot := range lots {
		trades = append(trades, TradeBlock{
			PlanID:     lot.PlanID,
			PlanItemID: lot.PlanItemID,
			FillID:     lot.FillID,
			FillPrice:  lot.FillPrice,
			FillVolume: lot.Volume,
			FilledAt:   lot.FilledAt,
			TradeDate:  lot.TradeDate,
		})
	}
	sort.Slice(trades, func(i, j int) bool {
		return tradeLess(trades[i], trades[j])
	})
	return trades
}

func tradeLess(a, b TradeBlock) bool {
	if a.FilledAt != nil && b.FilledAt != nil {
		return a.FilledAt.After(*b.FilledAt)
	}
	if a.FilledAt != nil {
		return true
	}
	if b.FilledAt != nil {
		return false
	}
	return a.TradeDate > b.TradeDate
}

func mapReconcile(rec papertrading.AttributionReconcile) *ReconcileBlock {
	return &ReconcileBlock{
		Status:           rec.Status,
		PositionVolume:   rec.PositionVolume,
		AttributedVolume: rec.AttributedVolume,
	}
}

func buildOrigins(lots []papertrading.PositionLotDTO, code string) []OriginBlock {
	cache := map[uint][]tradeplanorigin.ItemOrigin{}
	planOrder := []uint{}
	seen := map[uint]bool{}

	for _, lot := range lots {
		if lot.PlanID == 0 || seen[lot.PlanID] {
			continue
		}
		seen[lot.PlanID] = true
		planOrder = append(planOrder, lot.PlanID)
		items, err := tradeplanorigin.ProjectPlanOrigin(lot.PlanID)
		if err != nil {
			cache[lot.PlanID] = nil
		} else {
			cache[lot.PlanID] = items
		}
	}

	origins := make([]OriginBlock, 0, len(planOrder))
	for _, planID := range planOrder {
		if ob := mapOriginBlock(planID, cache[planID], code); ob != nil {
			origins = append(origins, *ob)
		}
	}
	sort.Slice(origins, func(i, j int) bool { return origins[i].PlanID < origins[j].PlanID })
	return origins
}

func mapOriginBlock(planID uint, items []tradeplanorigin.ItemOrigin, code string) *OriginBlock {
	for _, item := range items {
		if normalizeCode(item.StockCode) != code {
			continue
		}
		sigPresent := originFieldPresent(item.SignalTag) ||
			originFieldPresent(item.SignalTime) ||
			originFieldPresent(item.SignalPrice) ||
			item.SignalSnapshotID > 0
		reasonPresent := originFieldPresent(item.SourceReason) || originFieldPresent(item.SelectionReason)
		ob := &OriginBlock{
			PlanID: planID,
			Signal: SignalBlock{Present: sigPresent},
			Reason: ReasonBlock{Present: reasonPresent},
		}
		if originFieldPresent(item.StrategyName) {
			ob.Strategy = item.StrategyName
		}
		if originFieldPresent(item.SignalTag) {
			ob.Signal.Tag = item.SignalTag
		}
		if originFieldPresent(item.SignalTime) {
			ob.Signal.Time = item.SignalTime
		}
		if originFieldPresent(item.SignalPrice) {
			ob.Signal.Price = item.SignalPrice
		}
		if item.SignalSnapshotID > 0 {
			ob.Signal.SnapshotID = item.SignalSnapshotID
		}
		if originFieldPresent(item.SourceReason) {
			ob.Reason.Source = item.SourceReason
		}
		if originFieldPresent(item.SelectionReason) {
			ob.Reason.Selection = item.SelectionReason
		}
		if originFieldPresent(item.Score) {
			ob.Score = item.Score
		}
		return ob
	}
	return nil
}

func originFieldPresent(v string) bool {
	return v != "" && v != tradeplanorigin.Missing
}
