package positionstate

import (
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/portfolio"
	"go-stock/backend/tradingcalendar"
)

// Service loads Snapshot + fill lots and builds PositionState Bundle (read-mostly).
type Service struct {
	portfolio portfolio.Service
}

// NewService wires portfolio reader.
func NewService(ps portfolio.Service) *Service {
	if ps == nil {
		ps = portfolio.NewService()
	}
	return &Service{portfolio: ps}
}

// Query selects as-of / trade_date.
type Query struct {
	TradeDate string
	AsOf      time.Time
}

// Evaluate builds PositionStateView list from live Snapshot + buy/sell fill lots.
func (s *Service) Evaluate(q Query) *Bundle {
	if s == nil {
		s = NewService(nil)
	}
	asOf := q.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(q.TradeDate)
	if td == "" {
		td = tradingcalendar.FormatDate(asOf)
	}
	out := &Bundle{
		TradeDate:      td,
		AsOf:           asOf,
		Positions:      []PositionStateView{},
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}
	snap, _ := s.portfolio.Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	if snap == nil || !snap.Found {
		return out
	}
	buys, sells := loadLotsByCode(snap.AccountID)
	for _, p := range snap.Positions {
		if p.Volume <= 0 {
			continue
		}
		code := strings.TrimSpace(p.StockCode)
		locked := p.LockedVolume
		in := SnapshotInput{
			Symbol:       code,
			TotalQty:     p.Volume,
			AvailableQty: p.AvailableVolume,
			LockedQty:    &locked,
			BuyRecords:   buys[normalize(code)],
			SellRecords:  sells[normalize(code)],
			TradeDate:    td,
			CurrentDate:  td,
		}
		out.Positions = append(out.Positions, Calculate(in))
	}
	return out
}

func normalize(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func loadLotsByCode(accountID uint) (buys, sells map[string][]LotRecord) {
	buys = map[string][]LotRecord{}
	sells = map[string][]LotRecord{}
	if accountID == 0 || db.Dao == nil {
		return buys, sells
	}
	type row struct {
		StockCode string
		TradeDate string
		Side      string
		Quantity  int64
	}
	var rows []row
	// Join fills → orders for trade_date / side (read-only).
	// Phase12-M0: fill canonical qty is physical column paper_sim_fills.volume (NOT quantity).
	_ = db.Dao.Table("paper_sim_fills AS f").
		Select(fillLotQtySelectSQL).
		Joins("JOIN paper_sim_orders o ON o.id = f.order_id").
		Where("o.account_id = ?", accountID).
		Scan(&rows).Error
	for _, r := range rows {
		code := normalize(r.StockCode)
		if code == "" || r.Quantity <= 0 {
			continue
		}
		lot := LotRecord{TradeDate: strings.TrimSpace(r.TradeDate), Quantity: r.Quantity, Side: strings.ToUpper(strings.TrimSpace(r.Side))}
		switch lot.Side {
		case "SELL":
			sells[code] = append(sells[code], lot)
		default:
			lot.Side = "BUY"
			buys[code] = append(buys[code], lot)
		}
	}
	return buys, sells
}

// LoadLotsByAccount is used by Intelligence / Home to feed Calculate (read-only).
func LoadLotsByAccount(accountID uint) (buys, sells map[string][]LotRecord) {
	return loadLotsByCode(accountID)
}
