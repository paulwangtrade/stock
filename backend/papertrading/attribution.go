// Position Attribution View — read-only projection (Phase10).
//
// Does not alter paper_sim_positions schema, Broker, Gateway, or fill writers.
// Attribution keys come from paper_sim_fills → orders → plans/items (never item.order_id/fill_id).

package papertrading

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// Reconcile status values for attribution rows.
const (
	ReconcileStatusMatched      = "matched"
	ReconcileStatusUnattributed = "unattributed"
	ReconcileStatusSurplusFills = "surplus_fills"
)

// AttributionOptions filters BuildPositionAttribution.
type AttributionOptions struct {
	StockCode string // optional; empty = all positions
}

// PositionAttributionView is the account-level attribution response.
type PositionAttributionView struct {
	Enabled              bool                      `json:"enabled"`
	AccountID            uint                      `json:"account_id,omitempty"`
	AsOf                 time.Time                 `json:"as_of"`
	ReconcileAllMatched  bool                      `json:"reconcile_all_matched"`
	Positions            []PositionAttributionRow  `json:"positions"`
	DataSourceNote       string                    `json:"data_source_note"`
}

// PositionAttributionRow is one net position with lot breakdown.
type PositionAttributionRow struct {
	StockCode     string               `json:"stock_code"`
	StockName     string               `json:"stock_name"`
	TotalVolume   int64                `json:"total_volume"`
	CurrentPrice  float64              `json:"current_price"`
	PnL           float64              `json:"pnl"`
	Lots          []PositionLotDTO     `json:"lots"`
	Reconcile     AttributionReconcile `json:"reconcile"`
	Unattributed  *UnattributedQty     `json:"unattributed,omitempty"`
	SourceSummary string               `json:"source_summary,omitempty"`
	PlanIDs       []uint               `json:"plan_ids,omitempty"`
}

// PositionLotDTO is one buy-fill projected lot (never a forged fill).
type PositionLotDTO struct {
	PlanID       uint       `json:"plan_id"`
	PlanItemID   uint       `json:"plan_item_id"`
	OrderID      uint       `json:"order_id"`
	FillID       uint       `json:"fill_id"`
	FillPrice    float64    `json:"fill_price"`
	Volume       int64      `json:"volume"`
	CostAmount   float64    `json:"cost_amount"`
	TradeDate    string     `json:"trade_date,omitempty"`
	StrategyName string     `json:"strategy_name,omitempty"`
	FilledAt     *time.Time `json:"filled_at,omitempty"`
}

// AttributionReconcile compares position qty vs attributed fill qty.
type AttributionReconcile struct {
	PositionVolume     int64  `json:"position_volume"`
	AttributedVolume   int64  `json:"attributed_volume"`
	UnattributedVolume int64  `json:"unattributed_volume"`
	SurplusFillVolume  int64  `json:"surplus_fill_volume,omitempty"`
	Status             string `json:"status"`
}

// UnattributedQty is position qty not covered by buy fills (not a fake fill_id).
type UnattributedQty struct {
	Volume     int64  `json:"volume"`
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message"`
}

const attributionDataSourceNote = "Position Attribution · read-only projection from paper_sim_fills; positions schema unchanged"

// BuildPositionAttribution builds the read-only attribution view for the default paper account.
func BuildPositionAttribution(opts AttributionOptions) (*PositionAttributionView, error) {
	out := &PositionAttributionView{
		Enabled:        IsEnabled(),
		AsOf:           time.Now(),
		Positions:      []PositionAttributionRow{},
		DataSourceNote: attributionDataSourceNote,
	}
	if db.Dao == nil {
		return out, fmt.Errorf("papertrading: db not initialized")
	}
	acc, err := GetDefaultAccount()
	if err != nil {
		return out, err
	}
	if acc == nil {
		out.ReconcileAllMatched = true
		return out, nil
	}
	out.AccountID = acc.ID
	return buildPositionAttributionForAccount(db.Dao, acc.ID, opts, out)
}

func buildPositionAttributionForAccount(
	gdb *gorm.DB,
	accountID uint,
	opts AttributionOptions,
	out *PositionAttributionView,
) (*PositionAttributionView, error) {
	if out == nil {
		out = &PositionAttributionView{
			Enabled:        IsEnabled(),
			AsOf:           time.Now(),
			Positions:      []PositionAttributionRow{},
			DataSourceNote: attributionDataSourceNote,
		}
	}
	out.AccountID = accountID

	filterCode := strings.ToLower(strings.TrimSpace(opts.StockCode))

	var positions []PaperSimPosition
	pq := gdb.Where("account_id = ?", accountID)
	if filterCode != "" {
		pq = pq.Where("LOWER(TRIM(stock_code)) = ?", filterCode)
	}
	if err := pq.Order("stock_code asc").Find(&positions).Error; err != nil {
		return out, err
	}

	var fills []PaperSimFill
	fq := gdb.Where("account_id = ?", accountID).
		Where("(side = '' OR LOWER(side) = ?)", "buy")
	if filterCode != "" {
		fq = fq.Where("LOWER(TRIM(stock_code)) = ?", filterCode)
	}
	if err := fq.Order("filled_at asc, id asc").Find(&fills).Error; err != nil {
		return out, err
	}

	orderIDs := uniqueUint(len(fills), func(i int) uint { return fills[i].OrderID })
	itemIDs := uniqueUint(len(fills), func(i int) uint { return fills[i].PlanItemID })
	planIDs := uniqueUint(len(fills), func(i int) uint { return fills[i].PlanID })

	ordersByID := map[uint]PaperSimOrder{}
	if len(orderIDs) > 0 {
		var orders []PaperSimOrder
		if err := gdb.Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			return out, err
		}
		for _, o := range orders {
			ordersByID[o.ID] = o
		}
	}

	itemsByID := map[uint]models.TradePlanItem{}
	if len(itemIDs) > 0 {
		var items []models.TradePlanItem
		if err := gdb.Where("id IN ?", itemIDs).Find(&items).Error; err != nil {
			return out, err
		}
		for _, it := range items {
			itemsByID[it.ID] = it
		}
	}

	plansByID := map[uint]models.TradePlan{}
	if len(planIDs) > 0 {
		var plans []models.TradePlan
		if err := gdb.Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
			return out, err
		}
		for _, p := range plans {
			plansByID[p.ID] = p
		}
	}

	fillsByCode := map[string][]PaperSimFill{}
	for _, f := range fills {
		code := normalizeAttrCode(f.StockCode)
		if code == "" {
			continue
		}
		fillsByCode[code] = append(fillsByCode[code], f)
	}

	allMatched := true
	rows := make([]PositionAttributionRow, 0, len(positions))
	for _, pos := range positions {
		code := normalizeAttrCode(pos.StockCode)
		codeFills := fillsByCode[code]
		row := projectAttributionRow(pos, codeFills, ordersByID, itemsByID, plansByID)
		if row.Reconcile.Status != ReconcileStatusMatched {
			allMatched = false
		}
		rows = append(rows, row)
	}
	out.Positions = rows
	out.ReconcileAllMatched = allMatched
	return out, nil
}

func projectAttributionRow(
	pos PaperSimPosition,
	fills []PaperSimFill,
	ordersByID map[uint]PaperSimOrder,
	itemsByID map[uint]models.TradePlanItem,
	plansByID map[uint]models.TradePlan,
) PositionAttributionRow {
	lots := make([]PositionLotDTO, 0, len(fills))
	var attributed int64
	planIDSet := map[uint]bool{}

	for _, f := range fills {
		vol := f.Volume
		if vol < 0 {
			vol = 0
		}
		attributed += vol
		cost := f.Price * float64(vol)
		filledAt := f.FilledAt
		lot := PositionLotDTO{
			PlanID:     f.PlanID,
			PlanItemID: f.PlanItemID,
			OrderID:    f.OrderID,
			FillID:     f.ID,
			FillPrice:  f.Price,
			Volume:     vol,
			CostAmount: cost,
			FilledAt:   &filledAt,
		}
		if o, ok := ordersByID[f.OrderID]; ok {
			lot.TradeDate = o.TradeDate
		} else if p, ok := plansByID[f.PlanID]; ok {
			lot.TradeDate = p.TradeDate
		}
		if it, ok := itemsByID[f.PlanItemID]; ok {
			lot.StrategyName = it.StrategyName
		}
		if f.PlanID > 0 {
			planIDSet[f.PlanID] = true
		}
		lots = append(lots, lot)
	}

	posVol := pos.TotalVolume
	unattr := posVol - attributed
	surplus := int64(0)
	if unattr < 0 {
		surplus = -unattr
		unattr = 0
	}

	rec := AttributionReconcile{
		PositionVolume:     posVol,
		AttributedVolume:   attributed,
		UnattributedVolume: unattr,
		SurplusFillVolume:  surplus,
		Status:             ReconcileStatusMatched,
	}
	if unattr > 0 {
		rec.Status = ReconcileStatusUnattributed
	} else if surplus > 0 {
		rec.Status = ReconcileStatusSurplusFills
	}

	planIDs := make([]uint, 0, len(planIDSet))
	for id := range planIDSet {
		planIDs = append(planIDs, id)
	}
	sort.Slice(planIDs, func(i, j int) bool { return planIDs[i] < planIDs[j] })

	current := pos.MarkPrice
	pnl := (current - pos.AvgCost) * float64(posVol)

	row := PositionAttributionRow{
		StockCode:     pos.StockCode,
		StockName:     pos.StockName,
		TotalVolume:   posVol,
		CurrentPrice:  current,
		PnL:           pnl,
		Lots:          lots,
		Reconcile:     rec,
		PlanIDs:       planIDs,
		SourceSummary: buildSourceSummary(planIDs, len(lots), unattr),
	}
	if unattr > 0 {
		row.Unattributed = &UnattributedQty{
			Volume:     unattr,
			ReasonCode: "POSITION_GT_FILLS",
			Message:    fmt.Sprintf("position_volume=%d exceeds attributed buy fills=%d", posVol, attributed),
		}
	}
	return row
}

func buildSourceSummary(planIDs []uint, lotCount int, unattr int64) string {
	var b strings.Builder
	if lotCount == 0 && unattr > 0 {
		fmt.Fprintf(&b, "unattributed %d", unattr)
		return b.String()
	}
	fmt.Fprintf(&b, "%d lots", lotCount)
	switch len(planIDs) {
	case 0:
		if lotCount > 0 {
			b.WriteString(" · plan unknown")
		}
	case 1:
		fmt.Fprintf(&b, " · plan #%d", planIDs[0])
	default:
		b.WriteString(" · plans")
		for i, id := range planIDs {
			if i == 0 {
				fmt.Fprintf(&b, " #%d", id)
			} else {
				fmt.Fprintf(&b, ",#%d", id)
			}
		}
	}
	if unattr > 0 {
		fmt.Fprintf(&b, " · +unattributed %d", unattr)
	}
	return b.String()
}

func normalizeAttrCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func uniqueUint(n int, at func(int) uint) []uint {
	seen := map[uint]bool{}
	out := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		id := at(i)
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
